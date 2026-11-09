package evm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/contract"
	gethbackend "github.com/vexo-network/vexo-consensus/modules/evm/backend/geth"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "evm"

const authNamespace = "auth"
const bankNamespace = "bank"

// ReceiptNamespace holds receipts, receipt locations, logs and blob sidecars.
// They are written with their block but are not consensus state: the app hash
// does not cover the namespace and no transaction outcome reads it, so a node
// may prune it below its retained height without diverging from the chain.
// The first receipt stored for a transaction hash stays; a later one with the
// same hash (possible only for transactions without replay protection) does
// not replace it.
const ReceiptNamespace = "evm_receipts"

// MaxLogBlockRange is the most blocks one logs query may cover.
const MaxLogBlockRange = 10_000

// MaxLogQueryResults is the most logs one logs query returns as its result. A query that matches
// more returns MaxLogQueryResults+1 logs, so the caller can tell that the result was cut off.
const MaxLogQueryResults = 10_000

// MaxLogTopics is the number of topic positions an EVM log has, and MaxLogTopicOptions the most
// alternatives one position of a logs query may accept.
const (
	MaxLogTopics       = 4
	MaxLogTopicOptions = 32
)

// defaultFeeCollector is the bank account credited with EVM priority fees when
// the policy does not name another one.
const defaultFeeCollector types.Address = "fee_collector"

// coinbaseAlias is the block coinbase every execution runs under. The bank
// fee collector is a named account that has no 20-byte EVM address, so the
// executor pays tips to this alias and sweeps whatever balance it ends up
// holding into the configured fee collector inside the same write batch.
const coinbaseAlias types.Address = "0x0000000000000000000000000000000000000fee"

var (
	ErrInvalidEVMTx          = errors.New("invalid EVM transaction")
	ErrInvalidEVMQuery       = errors.New("invalid EVM query")
	ErrInvalidEVMGasEstimate = errors.New("invalid EVM gas estimate")
	ErrStoreMissing          = errors.New("EVM module store is required")
	ErrVMRegistryEmpty       = errors.New("EVM VM registry is required")
	ErrVMBackendUnavailable  = errors.New("EVM VM backend is unavailable")
	ErrAtomicStoreRequired   = errors.New("EVM module requires an atomic batch store")
	ErrNonceMismatch         = errors.New("EVM transaction nonce does not match the account nonce")
)

type Module struct {
	registry *contract.Registry
	policy   Policy
	initErr  error
	roots    *stateRootCache
}

type Policy struct {
	EVMChainID               uint64
	EVMForkPreset            string
	GethChainConfigJSON      string
	AllowUnprotectedLegacyTx bool
	MaxBlobSidecarBlobs      uint64
	MaxBlobSidecarBytes      uint64
	// FeeCollector is the bank account credited with EVM priority fees.
	FeeCollector types.Address
	// RPCGasCap bounds the gas of one read-only execution (evm/call query). It is a node-local
	// limit and never applies to block execution.
	RPCGasCap uint64
	// MaxTxGas bounds the gas of one transaction's execution. It is the chain's
	// execution.max_gas, a consensus parameter: every validator executes a block under the
	// same bound, whatever its own RPC limits.
	MaxTxGas uint64
}

type Receipt struct {
	TxHash          string `json:"tx_hash"`
	Height          uint64 `json:"height"`
	Status          uint32 `json:"status"`
	From            string `json:"from"`
	To              string `json:"to,omitempty"`
	ContractAddress string `json:"contract_address,omitempty"`
	VM              string `json:"vm"`
	GasUsed         uint64 `json:"gas_used"`
	Error           string `json:"error,omitempty"`
	Output          string `json:"output,omitempty"`
	Logs            []Log  `json:"logs,omitempty"`
	StateDiff       any    `json:"state_diff,omitempty"`
}

type ReceiptIndex struct {
	TxHash  string `json:"tx_hash"`
	Height  uint64 `json:"height"`
	TxIndex uint64 `json:"tx_index"`
}

type BlobSidecarRecord struct {
	TxHash  string                      `json:"tx_hash"`
	Sidecar ethcompat.BlobSidecarBundle `json:"sidecar"`
}

type Log struct {
	Address         string            `json:"address"`
	Topics          []string          `json:"topics,omitempty"`
	Data            string            `json:"data,omitempty"`
	BlockNumber     uint64            `json:"block_number,omitempty"`
	TransactionHash string            `json:"transaction_hash,omitempty"`
	LogIndex        uint64            `json:"log_index,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
}

type BlockContextRecord struct {
	Height       uint64     `json:"height"`
	TimeUnixNano int64      `json:"time_unix_nano,omitempty"`
	AppHash      types.Hash `json:"app_hash,omitempty"`
	BaseFee      uint64     `json:"base_fee,omitempty"`
	BlobBaseFee  uint64     `json:"blob_base_fee,omitempty"`
}

type CallRequest struct {
	VM                        string                       `json:"vm"`
	From                      string                       `json:"from"`
	To                        string                       `json:"to"`
	Method                    string                       `json:"method"`
	Input                     string                       `json:"input,omitempty"`
	GasLimit                  uint64                       `json:"gas_limit,omitempty"`
	Value                     uint64                       `json:"value,omitempty"`
	ValueWei                  string                       `json:"value_wei,omitempty"`
	ValueHex                  string                       `json:"value_hex,omitempty"`
	Height                    uint64                       `json:"height,omitempty"`
	GasPrice                  uint64                       `json:"gas_price,omitempty"`
	GasPriceHex               string                       `json:"gas_price_hex,omitempty"`
	MaxFeePerGas              uint64                       `json:"max_fee_per_gas,omitempty"`
	MaxFeePerGasHex           string                       `json:"max_fee_per_gas_hex,omitempty"`
	MaxPriorityFeePerGas      uint64                       `json:"max_priority_fee_per_gas,omitempty"`
	MaxPriorityFeePerGasHex   string                       `json:"max_priority_fee_per_gas_hex,omitempty"`
	BaseFee                   uint64                       `json:"base_fee,omitempty"`
	BlobBaseFee               uint64                       `json:"blob_base_fee,omitempty"`
	BlobHashes                []string                     `json:"blob_hashes,omitempty"`
	Nonce                     uint64                       `json:"nonce,omitempty"`
	AccessList                []contract.AccessListEntry   `json:"access_list,omitempty"`
	StateOverrides            map[string]CallStateOverride `json:"state_overrides,omitempty"`
	BlockOverride             CallBlockOverride            `json:"block_override,omitempty"`
	SetCodeAuthorizationsJSON string                       `json:"set_code_authorizations_json,omitempty"`
	// Trace asks for a bounded opcode trace in CallResponse.VMTrace. Only the
	// debug and trace RPC methods set it.
	Trace bool `json:"trace,omitempty"`
}

// LogsRequest bounds a logs query to the inclusive block range
// [FromBlock, ToBlock]. A zero ToBlock, or one past the latest block, means
// the latest block. Topics filters like eth_getLogs: position i accepts any of
// Topics[i] (compared case-insensitively), and an empty position accepts every
// topic. Limit is the most logs the query returns; zero, or anything above
// MaxLogQueryResults+1, means MaxLogQueryResults+1. The query stops reading
// once it holds Limit matching logs.
type LogsRequest struct {
	FromBlock uint64     `json:"from_block,omitempty"`
	ToBlock   uint64     `json:"to_block,omitempty"`
	Topics    [][]string `json:"topics,omitempty"`
	Limit     uint64     `json:"limit,omitempty"`
}

type CallResponse struct {
	Output     string                     `json:"output,omitempty"`
	GasUsed    uint64                     `json:"gas_used,omitempty"`
	Failed     bool                       `json:"failed,omitempty"`
	Error      string                     `json:"error,omitempty"`
	AccessList []contract.AccessListEntry `json:"access_list,omitempty"`
	StateDiff  any                        `json:"state_diff,omitempty"`
	VMTrace    any                        `json:"vm_trace,omitempty"`
}

type CallStateOverride struct {
	Balance   string            `json:"balance,omitempty"`
	Nonce     *uint64           `json:"nonce,omitempty"`
	Code      string            `json:"code,omitempty"`
	State     map[string]string `json:"state,omitempty"`
	StateDiff map[string]string `json:"stateDiff,omitempty"`
}

type CallBlockOverride struct {
	Number      uint64 `json:"number,omitempty"`
	Timestamp   uint64 `json:"timestamp,omitempty"`
	GasLimit    uint64 `json:"gas_limit,omitempty"`
	BaseFee     uint64 `json:"base_fee,omitempty"`
	BlobBaseFee uint64 `json:"blob_base_fee,omitempty"`
}

type ProofRequest struct {
	Address     string   `json:"address"`
	StorageKeys []string `json:"storage_keys,omitempty"`
	Height      uint64   `json:"height,omitempty"`
}

type StateRootRequest struct {
	Height uint64 `json:"height,omitempty"`
}

type AccountStateRequest struct {
	Height uint64 `json:"height,omitempty"`
}

type StateBackendInfo struct {
	Name                    string   `json:"name"`
	Version                 string   `json:"version"`
	ExecutionBackend        string   `json:"execution_backend"`
	GethModule              string   `json:"geth_module"`
	GethVersion             string   `json:"geth_version"`
	GethChecksum            string   `json:"geth_checksum,omitempty"`
	GethReplace             string   `json:"geth_replace,omitempty"`
	LinkedVMs               []string `json:"linked_vms,omitempty"`
	AccountNamespace        string   `json:"account_namespace"`
	AuthNamespace           string   `json:"auth_namespace"`
	NativeBalanceNamespace  string   `json:"native_balance_namespace"`
	NativeBalanceKeyFormat  string   `json:"native_balance_key_format"`
	NativeBalanceBits       uint64   `json:"native_balance_bits"`
	NativeCoinSharedWithEVM bool     `json:"native_coin_shared_with_evm"`
	HistoricalSnapshots     bool     `json:"historical_snapshots"`
	StateRootProofs         bool     `json:"state_root_proofs"`
	StorageProofs           bool     `json:"storage_proofs"`
	StrictStateRootCapable  bool     `json:"strict_state_root_capable"`
	GethLibraryBoundary     string   `json:"geth_library_boundary"`
}

func NewModule() Module {
	registry := contract.NewRegistry()
	if err := registry.Register(gethbackend.New()); err != nil {
		return Module{policy: DefaultPolicy(), roots: newStateRootCache(), initErr: fmt.Errorf("%w: register geth EVM backend: %v", ErrVMBackendUnavailable, err)}
	}
	return Module{registry: registry, policy: DefaultPolicy(), roots: newStateRootCache()}
}

func DefaultPolicy() Policy {
	return Policy{
		EVMForkPreset:       gethbackend.DefaultForkPreset,
		MaxBlobSidecarBlobs: 6,
		MaxBlobSidecarBytes: 2 * 1024 * 1024,
		FeeCollector:        defaultFeeCollector,
		RPCGasCap:           config.DefaultRPCGasCap,
		MaxTxGas:            config.DefaultMaxGas,
	}
}

func NewModuleWithPolicy(policy Policy) (Module, error) {
	if policy.EVMForkPreset == "" {
		policy.EVMForkPreset = gethbackend.DefaultForkPreset
	}
	if policy.MaxBlobSidecarBlobs == 0 {
		policy.MaxBlobSidecarBlobs = DefaultPolicy().MaxBlobSidecarBlobs
	}
	if policy.MaxBlobSidecarBytes == 0 {
		policy.MaxBlobSidecarBytes = DefaultPolicy().MaxBlobSidecarBytes
	}
	if policy.FeeCollector == "" {
		policy.FeeCollector = DefaultPolicy().FeeCollector
	}
	if policy.RPCGasCap == 0 {
		policy.RPCGasCap = DefaultPolicy().RPCGasCap
	}
	if policy.MaxTxGas == 0 {
		policy.MaxTxGas = DefaultPolicy().MaxTxGas
	}
	vm, err := gethbackend.NewWithChainConfigPresetJSON(policy.EVMForkPreset, policy.GethChainConfigJSON, policy.EVMChainID)
	if err != nil {
		return Module{}, err
	}
	registry := contract.NewRegistry()
	if err := registry.Register(vm); err != nil {
		return Module{}, err
	}
	return Module{registry: registry, policy: policy, roots: newStateRootCache()}, nil
}

func NewModuleWithRegistry(registry *contract.Registry) Module {
	return Module{registry: registry, policy: DefaultPolicy(), roots: newStateRootCache()}
}

func (module Module) CloneModule() vexoapp.Module {
	return Module{registry: module.registry, policy: module.policy, initErr: module.initErr, roots: newStateRootCache()}
}

func (module Module) Name() string { return ModuleName }

func (module Module) ReplayNamespaces() []string {
	return []string{ModuleName, bankNamespace, authNamespace}
}

func (Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	if ctx.Store == nil {
		return nil
	}
	for rawKey, rawValue := range genesis {
		if !strings.HasPrefix(rawKey, ModuleName+":code:") {
			continue
		}
		address := strings.TrimPrefix(rawKey, ModuleName+":code:")
		if address == "" {
			return ErrInvalidEVMTx
		}
		if err := ctx.Store.Set(ctx.GoContext(), ModuleName, codeKey(types.Address(address)), rawValue); err != nil {
			return err
		}
	}
	return nil
}

func (Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	if ctx.Store == nil {
		return nil
	}
	height := header.Height
	if height == 0 {
		height = ctx.Height
	}
	if height == 0 {
		return nil
	}
	record := BlockContextRecord{
		Height:       uint64(height),
		TimeUnixNano: header.TimeUnixNano,
		AppHash:      header.AppHash,
		BaseFee:      ctx.BaseFee,
		BlobBaseFee:  ctx.BlobBaseFee,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return ctx.Store.Set(ctx.GoContext(), ModuleName, latestBlockContextKey(), encoded)
}

func latestBlockContextKey() []byte {
	return []byte("meta/latest_block_context")
}

func (Module) EndBlock(ctx vexoapp.Context) error {
	if ctx.Store == nil || ctx.Height == 0 {
		return nil
	}
	return persistReceiptIndexes(ctx.GoContext(), ctx.Store, ctx.TxResults)
}

func (Module) Prune(ctx vexoapp.Context, retainFrom types.Height) error {
	if ctx.Store == nil || retainFrom == 0 {
		return nil
	}
	writes, err := evmHistoryPruneWrites(ctx.GoContext(), ctx.Store, uint64(retainFrom))
	if err != nil {
		return err
	}
	return applyKVWrites(ctx.GoContext(), ctx.Store, writes)
}

func (module Module) validateEthereumRawTx(ctx vexoapp.Context, tx types.Tx) error {
	if !ethcompat.IsEthereumTx(tx) {
		return nil
	}
	chainID := ctx.EVMChainID
	if chainID == 0 {
		chainID = ethcompat.ChainNumericID(ctx.ChainID)
	}
	options := ethcompat.DecodeOptions{
		ChainID:                chainID,
		BaseFee:                ctx.BaseFee,
		BlobBaseFee:            ctx.BlobBaseFee,
		AllowUnprotectedLegacy: module.policy.AllowUnprotectedLegacyTx,
	}
	return ethcompat.ValidateCanonicalTxForExecution(tx, options)
}

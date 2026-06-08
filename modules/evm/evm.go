package evm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/events"
	gethbackend "github.com/vexo-network/vexo-consensus/modules/evm/backend/geth"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"golang.org/x/crypto/sha3"
)

const ModuleName = "evm"

const ethereumStateSnapshotNamespace = "evm_ethstate"

const (
	callGasCost   uint64 = 21_000
	deployGasCost uint64 = 53_000
)

var (
	ErrInvalidEVMTx    = errors.New("invalid EVM transaction")
	ErrInvalidEVMQuery = errors.New("invalid EVM query")
	ErrStoreMissing    = errors.New("EVM module store is required")
	ErrVMRegistryEmpty = errors.New("EVM VM registry is required")
)

type Module struct {
	registry *contract.Registry
	policy   Policy
}

type Policy struct {
	EVMChainID               uint64
	GethChainConfigJSON      string
	AllowUnprotectedLegacyTx bool
	MaxBlobSidecarBlobs      uint64
	MaxBlobSidecarBytes      uint64
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
	VMTrace         any    `json:"vm_trace,omitempty"`
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
	ValueHex                  string                       `json:"value_hex,omitempty"`
	Height                    uint64                       `json:"height,omitempty"`
	GasPrice                  uint64                       `json:"gas_price,omitempty"`
	MaxFeePerGas              uint64                       `json:"max_fee_per_gas,omitempty"`
	MaxPriorityFeePerGas      uint64                       `json:"max_priority_fee_per_gas,omitempty"`
	BaseFee                   uint64                       `json:"base_fee,omitempty"`
	BlobBaseFee               uint64                       `json:"blob_base_fee,omitempty"`
	BlobHashes                []string                     `json:"blob_hashes,omitempty"`
	Nonce                     uint64                       `json:"nonce,omitempty"`
	AccessList                []contract.AccessListEntry   `json:"access_list,omitempty"`
	StateOverrides            map[string]CallStateOverride `json:"state_overrides,omitempty"`
	BlockOverride             CallBlockOverride            `json:"block_override,omitempty"`
	SetCodeAuthorizationsJSON string                       `json:"set_code_authorizations_json,omitempty"`
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

type ethereumStateSnapshotMeta struct {
	Height    uint64 `json:"height"`
	StateRoot string `json:"state_root"`
}

func NewModule() Module {
	registry := contract.NewRegistry()
	_ = registry.Register(gethbackend.New())
	return Module{registry: registry, policy: DefaultPolicy()}
}

func DefaultPolicy() Policy {
	return Policy{
		MaxBlobSidecarBlobs: 6,
		MaxBlobSidecarBytes: 2 * 1024 * 1024,
	}
}

func NewModuleWithPolicy(policy Policy) (Module, error) {
	if policy.MaxBlobSidecarBlobs == 0 {
		policy.MaxBlobSidecarBlobs = DefaultPolicy().MaxBlobSidecarBlobs
	}
	if policy.MaxBlobSidecarBytes == 0 {
		policy.MaxBlobSidecarBytes = DefaultPolicy().MaxBlobSidecarBytes
	}
	vm, err := gethbackend.NewWithChainConfigJSON(policy.GethChainConfigJSON, policy.EVMChainID)
	if err != nil {
		return Module{}, err
	}
	registry := contract.NewRegistry()
	if err := registry.Register(vm); err != nil {
		return Module{}, err
	}
	return Module{registry: registry, policy: policy}, nil
}

func NewModuleWithRegistry(registry *contract.Registry) Module {
	return Module{registry: registry, policy: DefaultPolicy()}
}

func (module Module) CloneModule() vexoapp.Module {
	return Module{registry: module.registry, policy: module.policy}
}

func (module Module) Name() string { return ModuleName }

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

func (module Module) ValidateTx(ctx vexoapp.Context, tx types.Tx) error {
	if err := module.validateEthereumRawTx(ctx, tx); err != nil {
		return err
	}
	_, err := module.blobSidecarBundleFromTx(tx)
	return err
}

func (module Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: ErrStoreMissing.Error()}
	}
	if module.registry == nil {
		return types.Result{Code: 1, Log: ErrVMRegistryEmpty.Error()}
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return types.Result{Code: 2, Log: ErrInvalidEVMTx.Error()}
	}
	switch canonical.Action {
	case "call":
		if err := module.validateEthereumRawTx(ctx, tx); err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		return module.deliverCall(ctx, tx, canonical.Args)
	case "deploy":
		return module.deliverDeploy(ctx, tx, canonical.Args)
	case "eth_deploy":
		if err := module.validateEthereumRawTx(ctx, tx); err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		return module.deliverEthereumDeploy(ctx, tx, canonical.Args)
	default:
		return types.Result{Code: 2, Log: ErrInvalidEVMTx.Error()}
	}
}

func (Module) EndBlock(ctx vexoapp.Context) error {
	if ctx.Store == nil || ctx.Height == 0 {
		return nil
	}
	if err := persistEthereumStateSnapshot(ctx.GoContext(), ctx.Store, uint64(ctx.Height)); err != nil {
		return err
	}
	return persistReceiptIndexes(ctx.GoContext(), ctx.Store, ctx.TxResults)
}

func (Module) Prune(ctx vexoapp.Context, retainFrom types.Height) error {
	if ctx.Store == nil || retainFrom == 0 {
		return nil
	}
	prefixStore, ok := ctx.Store.(vexostore.PrefixKVStore)
	if !ok {
		return nil
	}
	pairs, err := prefixStore.ExportPrefix(ctx.GoContext(), ethereumStateSnapshotNamespace, nil)
	if err != nil {
		return err
	}
	writes := make([]vexostore.KVWrite, 0)
	for _, pair := range pairs {
		height, ok := ethereumStateSnapshotHeightFromKey(pair.Key)
		if !ok || height >= uint64(retainFrom) {
			continue
		}
		writes = append(writes, vexostore.KVWrite{
			Namespace: ethereumStateSnapshotNamespace,
			Key:       append([]byte(nil), pair.Key...),
			Delete:    true,
		})
	}
	pruneWrites, err := evmHistoryPruneWrites(ctx.GoContext(), ctx.Store, uint64(retainFrom))
	if err != nil {
		return err
	}
	writes = append(writes, pruneWrites...)
	return applyKVWrites(ctx.GoContext(), ctx.Store, writes)
}

func (module Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return 0, ErrInvalidEVMTx
	}
	switch canonical.Action {
	case "call":
		invocation, err := callInvocationFromArgs(canonical.Args)
		if err != nil {
			return 0, err
		}
		if ctx.Store != nil {
			state := evmStateReader{store: ctx.Store}
			code, err := state.Code(ctx.GoContext(), invocation.Contract)
			if err != nil {
				return 0, err
			}
			invocation.Code = code
			invocation.State = state
		}
		invocation.ReadOnly = false
		invocation.BlockNumber = uint64(ctx.Height)
		invocation.Timestamp = headerUnixSeconds(ctx.Header)
		invocation.BlockGasLimit = ctx.GasLimit()
		invocation.GasPrice = txGasPrice(tx)
		invocation.GasFeeCap = txGasFeeCap(tx)
		invocation.GasTipCap = txGasTipCap(tx)
		invocation.BaseFee = txExecutionBaseFee(ctx, tx)
		invocation.BlobBaseFee = txExecutionBlobBaseFee(ctx, tx)
		invocation.BlobGasFeeCap = txBlobGasFeeCap(tx)
		invocation.BlobHashes = txBlobHashes(tx)
		invocation.Coinbase = types.Address("fee_collector")
		invocation.Nonce = txNonce(tx)
		invocation.EthereumTx = ethcompat.IsEthereumTx(tx)
		invocation.RawEthereumTx = txRawEthereum(tx)
		return module.estimateInvocationGas(ctx, invocation, callGasCost)
	case "deploy", "eth_deploy":
		invocation, err := module.deployInvocationForEstimate(ctx, canonical.Action, canonical.Args, tx)
		if err != nil {
			return 0, err
		}
		return module.estimateInvocationGas(ctx, invocation, deployGasCost)
	default:
		return 0, ErrInvalidEVMTx
	}
}

func (module Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if len(req.Path) == 0 {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	if req.Path[0] == "call" {
		return module.queryCall(ctx, req.Data)
	}
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStoreMissing.Error()}
	}
	switch req.Path[0] {
	case "eth_state_root":
		return queryEthereumStateRoot(ctx, req.Data)
	case "eth_proof":
		return queryEthereumProof(ctx, req.Data)
	case "account":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		height, err := accountStateRequestHeight(req.Data)
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		var account ethcompat.AccountState
		found := false
		if height > 0 {
			account, found, err = ethereumAccountAtHeight(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]), height)
			if errors.Is(err, vexostore.ErrKeyNotFound) {
				return vexoapp.QueryResponse{Code: 3, Log: "Ethereum state snapshot not found"}
			}
			if err != nil {
				return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
			}
		} else {
			account, found, err = ethereumAccountFromCurrentState(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]))
			if err != nil {
				return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
			}
		}
		if !found {
			account = ethcompat.AccountState{Address: canonicalAddressKey(types.Address(req.Path[1])), Storage: map[string][]byte{}}
		}
		encoded, err := json.Marshal(map[string]any{
			"address":     canonicalAddressKey(types.Address(account.Address)),
			"balance":     account.Balance,
			"balance_hex": accountBalanceHex(account),
			"nonce":       account.Nonce,
			"code":        hex.EncodeToString(account.Code),
		})
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: encoded}
	case "receipt":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		return queryJSON(ctx, receiptKey(req.Path[1]))
	case "receipt_index":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		return queryJSON(ctx, receiptIndexKey(req.Path[1]))
	case "blob_sidecar":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		return queryJSON(ctx, blobSidecarKey(req.Path[1]))
	case "blob_sidecar_by_hash":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		txHash, err := ctx.Store.Get(ctx.GoContext(), ModuleName, blobSidecarHashIndexKey(req.Path[1]))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return vexoapp.QueryResponse{Code: 3, Log: "EVM blob sidecar not found"}
		}
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		return queryJSON(ctx, blobSidecarKey(string(txHash)))
	case "code":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		height, err := accountStateRequestHeight(req.Data)
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		if height > 0 {
			account, found, err := ethereumAccountAtHeight(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]), height)
			if errors.Is(err, vexostore.ErrKeyNotFound) {
				return vexoapp.QueryResponse{Code: 3, Log: "Ethereum state snapshot not found"}
			}
			if err != nil {
				return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
			}
			if !found || len(account.Code) == 0 {
				return vexoapp.QueryResponse{Code: 3, Log: "EVM code not found"}
			}
			encoded, _ := json.Marshal(map[string]string{"address": req.Path[1], "code": hex.EncodeToString(account.Code)})
			return vexoapp.QueryResponse{Value: encoded}
		}
		code, err := ctx.Store.Get(ctx.GoContext(), ModuleName, codeKey(types.Address(req.Path[1])))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return vexoapp.QueryResponse{Code: 3, Log: "EVM code not found"}
		}
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		encoded, _ := json.Marshal(map[string]string{"address": req.Path[1], "code": hex.EncodeToString(code)})
		return vexoapp.QueryResponse{Value: encoded}
	case "storage":
		if len(req.Path) != 3 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		height, err := accountStateRequestHeight(req.Data)
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		if height > 0 {
			account, found, err := ethereumAccountAtHeight(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]), height)
			if errors.Is(err, vexostore.ErrKeyNotFound) {
				return vexoapp.QueryResponse{Code: 3, Log: "Ethereum state snapshot not found"}
			}
			if err != nil {
				return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
			}
			value := []byte(nil)
			if found && account.Storage != nil {
				value = account.Storage[strings.TrimPrefix(normalizeSlot(req.Path[2]), "0x")]
			}
			if len(value) == 0 {
				return vexoapp.QueryResponse{Code: 3, Log: "EVM storage not found"}
			}
			encoded, _ := json.Marshal(map[string]string{
				"address": req.Path[1],
				"slot":    req.Path[2],
				"value":   "0x" + hex.EncodeToString(value),
			})
			return vexoapp.QueryResponse{Value: encoded}
		}
		value, err := ctx.Store.Get(ctx.GoContext(), ModuleName, storageKey(types.Address(req.Path[1]), req.Path[2]))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return vexoapp.QueryResponse{Code: 3, Log: "EVM storage not found"}
		}
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		encoded, _ := json.Marshal(map[string]string{
			"address": req.Path[1],
			"slot":    req.Path[2],
			"value":   "0x" + hex.EncodeToString(value),
		})
		return vexoapp.QueryResponse{Value: encoded}
	case "logs":
		if len(req.Path) == 1 {
			return queryLogs(ctx, globalLogPrefix(), globalLogsKey())
		}
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		address := types.Address(req.Path[1])
		return queryLogs(ctx, addressLogPrefix(address), logsKey(address))
	default:
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
}

func (module Module) Events(ctx vexoapp.Context, tx types.Tx, result types.Result) []events.Event {
	if result.Code != 0 {
		return nil
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return nil
	}
	attributes := []events.Attribute{{Key: "module", Value: ModuleName, Index: true}, {Key: "action", Value: canonical.Action, Index: true}}
	if len(result.Data) > 0 {
		var receipt Receipt
		if err := json.Unmarshal(result.Data, &receipt); err == nil {
			attributes = append(attributes,
				events.Attribute{Key: "evm_tx_hash", Value: receipt.TxHash, Index: true},
				events.Attribute{Key: "evm_from", Value: receipt.From, Index: true},
			)
			if receipt.To != "" {
				attributes = append(attributes, events.Attribute{Key: "evm_to", Value: receipt.To, Index: true})
			}
			if receipt.ContractAddress != "" {
				attributes = append(attributes, events.Attribute{Key: "evm_contract", Value: receipt.ContractAddress, Index: true})
			}
		}
	}
	return []events.Event{{Type: "evm_" + canonical.Action, Attributes: attributes}}
}

func (module Module) deliverCall(ctx vexoapp.Context, tx types.Tx, args []string) types.Result {
	if _, err := module.blobSidecarBundleFromTx(tx); err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	invocation, err := callInvocationFromArgs(args)
	if err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	code, err := loadCode(ctx.GoContext(), ctx.Store, invocation.Contract)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	invocation.Code = code
	invocation.State = evmStateReader{store: ctx.Store}
	invocation.BlockNumber = uint64(ctx.Height)
	invocation.Timestamp = headerUnixSeconds(ctx.Header)
	invocation.BlockGasLimit = ctx.GasLimit()
	invocation.GasPrice = txGasPrice(tx)
	invocation.GasFeeCap = txGasFeeCap(tx)
	invocation.GasTipCap = txGasTipCap(tx)
	invocation.BaseFee = txExecutionBaseFee(ctx, tx)
	invocation.BlobBaseFee = txExecutionBlobBaseFee(ctx, tx)
	invocation.BlobGasFeeCap = txBlobGasFeeCap(tx)
	invocation.BlobHashes = txBlobHashes(tx)
	invocation.Coinbase = types.Address("fee_collector")
	invocation.AccessList = accessListFromTx(tx)
	invocation.Nonce = txNonce(tx)
	invocation.EthereumTx = ethcompat.IsEthereumTx(tx)
	invocation.RawEthereumTx = txRawEthereum(tx)
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := ctx.ConsumeGas(result.GasUsed); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if err := persistExecutionResult(ctx.GoContext(), ctx.Store, invocation.Contract, result); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	receipt := receiptFromResult(tx, ctx.Height, invocation, "", result)
	if err := persistReceipt(ctx.GoContext(), ctx.Store, receipt); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := module.persistBlobSidecar(ctx.GoContext(), ctx.Store, tx); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	return types.Result{Data: encoded, GasUsed: result.GasUsed}
}

func (module Module) deliverDeploy(ctx vexoapp.Context, tx types.Tx, args []string) types.Result {
	if len(args) != 4 && len(args) != 5 {
		return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
	}
	code, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
	if err != nil || len(code) == 0 {
		return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
	}
	value := new(big.Int)
	if len(args) == 5 {
		value, err = parseAmountBig(args[4])
		if err != nil {
			return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
		}
	}
	salt := create2Salt(args[3])
	contractAddress := createAddress(types.Address(args[1]), code, salt)
	invocation := contract.Invocation{
		VM:            args[0],
		Caller:        types.Address(args[1]),
		Contract:      contractAddress,
		Method:        "deploy",
		Input:         code,
		GasLimit:      txGasLimit(tx, ctx.GasLimit()),
		Value:         uint64Amount(value),
		ValueBig:      value,
		Salt:          salt[:],
		State:         evmStateReader{store: ctx.Store},
		BlockNumber:   uint64(ctx.Height),
		Timestamp:     headerUnixSeconds(ctx.Header),
		BlockGasLimit: ctx.GasLimit(),
		GasPrice:      txGasPrice(tx),
		GasFeeCap:     txGasFeeCap(tx),
		GasTipCap:     txGasTipCap(tx),
		BaseFee:       txExecutionBaseFee(ctx, tx),
		BlobBaseFee:   txExecutionBlobBaseFee(ctx, tx),
		BlobGasFeeCap: txBlobGasFeeCap(tx),
		BlobHashes:    txBlobHashes(tx),
		Coinbase:      types.Address("fee_collector"),
		AccessList:    accessListFromTx(tx),
		Nonce:         txNonce(tx),
		EthereumTx:    ethcompat.IsEthereumTx(tx),
		RawEthereumTx: txRawEthereum(tx),
	}
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := ctx.ConsumeGas(result.GasUsed); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if !result.Failed {
		deployedCode := result.DeployedCode
		if len(deployedCode) == 0 {
			deployedCode = code
		}
		if err := ctx.Store.Set(ctx.GoContext(), ModuleName, codeKey(contractAddress), deployedCode); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
	}
	if err := persistExecutionResult(ctx.GoContext(), ctx.Store, invocation.Contract, result); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	receipt := receiptFromResult(tx, ctx.Height, invocation, string(contractAddress), result)
	if err := persistReceipt(ctx.GoContext(), ctx.Store, receipt); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	return types.Result{Data: encoded, GasUsed: result.GasUsed}
}

func (module Module) deliverEthereumDeploy(ctx vexoapp.Context, tx types.Tx, args []string) types.Result {
	if _, err := module.blobSidecarBundleFromTx(tx); err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	if len(args) != 5 {
		return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
	}
	code, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
	if err != nil || len(code) == 0 {
		return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
	}
	nonce, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
	}
	value, err := parseAmountBig(args[4])
	if err != nil {
		return types.Result{Code: 3, Log: ErrInvalidEVMTx.Error()}
	}
	contractAddress := createLegacyAddress(types.Address(args[1]), nonce)
	invocation := contract.Invocation{
		VM:            args[0],
		Caller:        types.Address(args[1]),
		Contract:      contractAddress,
		Method:        "deploy",
		Input:         code,
		GasLimit:      txGasLimit(tx, ctx.GasLimit()),
		Value:         uint64Amount(value),
		ValueBig:      value,
		State:         evmStateReader{store: ctx.Store},
		BlockNumber:   uint64(ctx.Height),
		Timestamp:     headerUnixSeconds(ctx.Header),
		BlockGasLimit: ctx.GasLimit(),
		GasPrice:      txGasPrice(tx),
		GasFeeCap:     txGasFeeCap(tx),
		GasTipCap:     txGasTipCap(tx),
		BaseFee:       txExecutionBaseFee(ctx, tx),
		BlobBaseFee:   txExecutionBlobBaseFee(ctx, tx),
		BlobGasFeeCap: txBlobGasFeeCap(tx),
		BlobHashes:    txBlobHashes(tx),
		Coinbase:      types.Address("fee_collector"),
		AccessList:    accessListFromTx(tx),
		Nonce:         nonce,
		EthereumTx:    ethcompat.IsEthereumTx(tx),
		RawEthereumTx: txRawEthereum(tx),
	}
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := ctx.ConsumeGas(result.GasUsed); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if !result.Failed {
		deployedCode := result.DeployedCode
		if len(deployedCode) == 0 {
			deployedCode = code
		}
		if err := ctx.Store.Set(ctx.GoContext(), ModuleName, codeKey(contractAddress), deployedCode); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
	}
	if err := persistExecutionResult(ctx.GoContext(), ctx.Store, invocation.Contract, result); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	receipt := receiptFromResult(tx, ctx.Height, invocation, string(contractAddress), result)
	if err := persistReceipt(ctx.GoContext(), ctx.Store, receipt); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := module.persistBlobSidecar(ctx.GoContext(), ctx.Store, tx); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	return types.Result{Data: encoded, GasUsed: result.GasUsed}
}

func (module Module) queryCall(ctx vexoapp.Context, data []byte) vexoapp.QueryResponse {
	if module.registry == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrVMRegistryEmpty.Error()}
	}
	var request CallRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
	}
	input, err := hex.DecodeString(strings.TrimPrefix(request.Input, "0x"))
	if err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	blockNumber := uint64(ctx.Height)
	if request.Height > 0 {
		blockNumber = request.Height
	}
	if request.BlockOverride.Number > 0 {
		blockNumber = request.BlockOverride.Number
	}
	state, err := stateReaderForCall(ctx, request.Height)
	if err != nil {
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return vexoapp.QueryResponse{Code: 3, Log: "Ethereum state snapshot not found"}
		}
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	state = newOverrideStateReader(state, request.StateOverrides)
	var code []byte
	if ctx.Store != nil {
		code, err = state.Code(ctx.GoContext(), types.Address(request.To))
		if err != nil && request.Method != "deploy" {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		if request.Method == "deploy" {
			code = nil
		}
	}
	callValue, err := callRequestValue(request)
	if err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	timestamp := headerUnixSeconds(ctx.Header)
	if request.BlockOverride.Timestamp > 0 {
		timestamp = request.BlockOverride.Timestamp
	}
	blockGasLimit := ctx.GasLimit()
	if request.BlockOverride.GasLimit > 0 {
		blockGasLimit = request.BlockOverride.GasLimit
	}
	baseFee := request.BaseFee
	if request.BlockOverride.BaseFee > 0 {
		baseFee = request.BlockOverride.BaseFee
	}
	blobBaseFee := request.BlobBaseFee
	if request.BlockOverride.BlobBaseFee > 0 {
		blobBaseFee = request.BlockOverride.BlobBaseFee
	}
	result, err := module.registry.Execute(ctx.GoContext(), contract.Invocation{
		VM:                        request.VM,
		Caller:                    types.Address(request.From),
		Contract:                  types.Address(request.To),
		Method:                    request.Method,
		Input:                     input,
		GasLimit:                  request.GasLimit,
		Value:                     uint64Amount(callValue),
		ValueBig:                  callValue,
		Code:                      code,
		State:                     state,
		ReadOnly:                  false,
		BlockNumber:               blockNumber,
		Timestamp:                 timestamp,
		BlockGasLimit:             blockGasLimit,
		GasPrice:                  request.GasPrice,
		GasFeeCap:                 request.MaxFeePerGas,
		GasTipCap:                 request.MaxPriorityFeePerGas,
		BaseFee:                   baseFee,
		BlobBaseFee:               blobBaseFee,
		BlobHashes:                parseBlobHashes(request.BlobHashes),
		Coinbase:                  types.Address("fee_collector"),
		AccessList:                append([]contract.AccessListEntry(nil), request.AccessList...),
		Nonce:                     request.Nonce,
		EthereumSimulation:        true,
		SetCodeAuthorizationsJSON: request.SetCodeAuthorizationsJSON,
	})
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(CallResponse{
		Output:     "0x" + hex.EncodeToString(result.Output),
		GasUsed:    result.GasUsed,
		Failed:     result.Failed,
		Error:      result.Error,
		AccessList: append([]contract.AccessListEntry(nil), result.AccessList...),
		StateDiff:  stateDiffFromResult(result, types.Address(request.To)),
		VMTrace:    result.VMTrace,
	})
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func callRequestValue(request CallRequest) (*big.Int, error) {
	if request.ValueHex == "" {
		return new(big.Int).SetUint64(request.Value), nil
	}
	value, ok := new(big.Int).SetString(strings.TrimPrefix(request.ValueHex, "0x"), 16)
	if !ok || value.Sign() < 0 || value.BitLen() > 256 {
		return nil, ErrInvalidEVMQuery
	}
	return value, nil
}

func (module Module) estimateInvocationGas(ctx vexoapp.Context, invocation contract.Invocation, fallback uint64) (uint64, error) {
	if module.registry == nil {
		return 0, ErrVMRegistryEmpty
	}
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return 0, err
	}
	if result.GasUsed == 0 {
		return fallback, nil
	}
	return result.GasUsed, nil
}

func (module Module) deployInvocationForEstimate(ctx vexoapp.Context, action string, args []string, tx types.Tx) (contract.Invocation, error) {
	switch action {
	case "deploy":
		if len(args) != 4 && len(args) != 5 {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
		code, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
		if err != nil || len(code) == 0 {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
		value := new(big.Int)
		if len(args) == 5 {
			value, err = parseAmountBig(args[4])
			if err != nil {
				return contract.Invocation{}, ErrInvalidEVMTx
			}
		}
		salt := create2Salt(args[3])
		return module.prepareDeployInvocation(ctx, tx, args[0], types.Address(args[1]), createAddress(types.Address(args[1]), code, salt), code, value, salt[:]), nil
	case "eth_deploy":
		if len(args) != 5 {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
		code, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
		if err != nil || len(code) == 0 {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
		nonce, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
		value, err := parseAmountBig(args[4])
		if err != nil {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
		return module.prepareDeployInvocation(ctx, tx, args[0], types.Address(args[1]), createLegacyAddress(types.Address(args[1]), nonce), code, value, nil), nil
	default:
		return contract.Invocation{}, ErrInvalidEVMTx
	}
}

func (Module) prepareDeployInvocation(ctx vexoapp.Context, tx types.Tx, vm string, caller types.Address, contractAddress types.Address, code []byte, value *big.Int, salt []byte) contract.Invocation {
	return contract.Invocation{
		VM:            vm,
		Caller:        caller,
		Contract:      contractAddress,
		Method:        "deploy",
		Input:         code,
		GasLimit:      txGasLimit(tx, ctx.GasLimit()),
		Value:         uint64Amount(value),
		ValueBig:      value,
		Salt:          append([]byte(nil), salt...),
		State:         evmStateReader{store: ctx.Store},
		BlockNumber:   uint64(ctx.Height),
		Timestamp:     headerUnixSeconds(ctx.Header),
		BlockGasLimit: ctx.GasLimit(),
		GasPrice:      txGasPrice(tx),
		GasFeeCap:     txGasFeeCap(tx),
		GasTipCap:     txGasTipCap(tx),
		BaseFee:       txExecutionBaseFee(ctx, tx),
		BlobBaseFee:   txExecutionBlobBaseFee(ctx, tx),
		BlobGasFeeCap: txBlobGasFeeCap(tx),
		BlobHashes:    txBlobHashes(tx),
		Coinbase:      types.Address("fee_collector"),
		AccessList:    accessListFromTx(tx),
		Nonce:         txNonce(tx),
		EthereumTx:    ethcompat.IsEthereumTx(tx),
		RawEthereumTx: txRawEthereum(tx),
	}
}

func accessListFromTx(tx types.Tx) []contract.AccessListEntry {
	encoded, found := vexoapp.TxTag(tx, ethcompat.TagAccessList)
	if !found || encoded == "" {
		return nil
	}
	entries, err := ethcompat.DecodeAccessList(encoded)
	if err != nil {
		return nil
	}
	return entries
}

func callInvocationFromArgs(args []string) (contract.Invocation, error) {
	if len(args) != 6 && len(args) != 7 {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	input, err := hex.DecodeString(strings.TrimPrefix(args[4], "0x"))
	if err != nil {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	gasLimit, err := strconv.ParseUint(args[5], 10, 64)
	if err != nil {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	value := new(big.Int)
	if len(args) == 7 {
		value, err = parseAmountBig(args[6])
		if err != nil {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
	}
	invocation := contract.Invocation{
		VM:       args[0],
		Caller:   types.Address(args[1]),
		Contract: types.Address(args[2]),
		Method:   args[3],
		Input:    input,
		GasLimit: gasLimit,
		Value:    uint64Amount(value),
		ValueBig: value,
	}
	if invocation.VM == "" || invocation.Caller == "" || invocation.Contract == "" || invocation.Method == "" {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	return invocation, nil
}

func parseAmountBig(raw string) (*big.Int, error) {
	if raw == "" {
		return new(big.Int), nil
	}
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok || value.Sign() < 0 || value.BitLen() > 256 {
		return nil, ErrInvalidEVMTx
	}
	return value, nil
}

func uint64Amount(value *big.Int) uint64 {
	if value == nil || !value.IsUint64() {
		return 0
	}
	return value.Uint64()
}

func receiptFromResult(tx types.Tx, height types.Height, invocation contract.Invocation, contractAddress string, result contract.Result) Receipt {
	receipt := Receipt{
		TxHash:          txHash(tx),
		Height:          uint64(height),
		Status:          1,
		From:            string(invocation.Caller),
		ContractAddress: contractAddress,
		VM:              invocation.VM,
		GasUsed:         result.GasUsed,
		Output:          "0x" + hex.EncodeToString(result.Output),
		Logs:            make([]Log, 0, len(result.Logs)),
		StateDiff:       stateDiffFromResult(result, types.Address(receiptTargetAddress(invocation, contractAddress))),
		VMTrace:         result.VMTrace,
	}
	if result.Failed {
		receipt.Status = 0
		receipt.Error = result.Error
	}
	if contractAddress == "" {
		receipt.To = string(invocation.Contract)
	}
	for index, log := range result.Logs {
		receipt.Logs = append(receipt.Logs, Log{
			Address:         string(log.Address),
			Topics:          append([]string(nil), log.Topics...),
			Data:            "0x" + hex.EncodeToString(log.Data),
			BlockNumber:     uint64(height),
			TransactionHash: txHash(tx),
			LogIndex:        uint64(index),
			Meta:            cloneMap(log.Meta),
		})
	}
	return receipt
}

func receiptTargetAddress(invocation contract.Invocation, contractAddress string) string {
	if contractAddress != "" {
		return string(contractAddress)
	}
	return string(invocation.Contract)
}

func stateDiffFromResult(result contract.Result, defaultAddress types.Address) any {
	diff := make(map[string]map[string]any)
	account := func(address types.Address) map[string]any {
		if address == "" {
			address = defaultAddress
		}
		if address == "" {
			return nil
		}
		key := canonicalAddressKey(address)
		item := diff[key]
		if item == nil {
			item = make(map[string]any)
			diff[key] = item
		}
		return item
	}
	for _, write := range result.BalanceWrites {
		entry := account(write.Address)
		if entry != nil {
			entry["balance"] = map[string]any{"to": balanceWriteHex(write)}
		}
	}
	for _, write := range result.NonceWrites {
		entry := account(write.Address)
		if entry != nil {
			entry["nonce"] = map[string]any{"to": hexQuantityLocal(write.Nonce)}
		}
	}
	for _, write := range result.CodeWrites {
		entry := account(write.Address)
		if entry == nil {
			continue
		}
		if write.Delete {
			entry["code"] = map[string]any{"delete": true}
			continue
		}
		entry["code"] = map[string]any{"to": "0x" + hex.EncodeToString(write.Code)}
	}
	for _, write := range result.StorageWrites {
		if write.Slot == "" {
			continue
		}
		entry := account(write.Address)
		if entry == nil {
			continue
		}
		storage, _ := entry["storage"].(map[string]any)
		if storage == nil {
			storage = make(map[string]any)
			entry["storage"] = storage
		}
		if write.Delete {
			storage[normalizeSlot(write.Slot)] = map[string]any{"delete": true}
			continue
		}
		storage[normalizeSlot(write.Slot)] = map[string]any{"to": "0x" + hex.EncodeToString(write.Value)}
	}
	for _, deletion := range result.AccountDeletions {
		entry := account(deletion.Address)
		if entry != nil {
			entry["delete"] = true
		}
	}
	if len(diff) == 0 {
		return nil
	}
	return diff
}

func hexQuantityLocal(value uint64) string {
	if value == 0 {
		return "0x0"
	}
	return "0x" + strconv.FormatUint(value, 16)
}

func balanceWriteHex(write contract.BalanceWrite) string {
	if write.BalanceBig != nil {
		return hexQuantityBigLocal(write.BalanceBig)
	}
	return hexQuantityLocal(write.Balance)
}

func hexQuantityBigLocal(value *big.Int) string {
	if value == nil || value.Sign() == 0 {
		return "0x0"
	}
	return "0x" + value.Text(16)
}

func persistReceipt(ctx context.Context, store vexoapp.StateStore, receipt Receipt) error {
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if err := store.Set(ctx, ModuleName, receiptKey(receipt.TxHash), encoded); err != nil {
		return err
	}
	for _, log := range receipt.Logs {
		if err := persistLog(ctx, store, log); err != nil {
			return err
		}
	}
	return nil
}

func blobSidecarBundleFromTx(tx types.Tx) (ethcompat.BlobSidecarBundle, error) {
	return Module{policy: DefaultPolicy()}.blobSidecarBundleFromTx(tx)
}

func (module Module) blobSidecarBundleFromTx(tx types.Tx) (ethcompat.BlobSidecarBundle, error) {
	expectedHashes := txBlobHashStrings(tx)
	encoded, found := vexoapp.TxTag(tx, ethcompat.TagBlobSidecar)
	if !found || encoded == "" {
		if len(expectedHashes) > 0 {
			return ethcompat.BlobSidecarBundle{}, ethcompat.ErrInvalidBlobSidecar
		}
		return ethcompat.BlobSidecarBundle{}, nil
	}
	bundle, err := ethcompat.DecodeBlobSidecarBundle(encoded)
	if err != nil {
		return ethcompat.BlobSidecarBundle{}, err
	}
	if !sameBlobHashes(expectedHashes, bundle.BlobHashes) {
		return ethcompat.BlobSidecarBundle{}, ethcompat.ErrInvalidBlobSidecar
	}
	if err := module.validateBlobSidecarLimits(encoded, bundle); err != nil {
		return ethcompat.BlobSidecarBundle{}, err
	}
	return bundle, nil
}

func persistBlobSidecar(ctx context.Context, store vexoapp.StateStore, tx types.Tx) error {
	return Module{policy: DefaultPolicy()}.persistBlobSidecar(ctx, store, tx)
}

func (module Module) persistBlobSidecar(ctx context.Context, store vexoapp.StateStore, tx types.Tx) error {
	bundle, err := module.blobSidecarBundleFromTx(tx)
	if err != nil {
		return err
	}
	if len(bundle.BlobHashes) == 0 {
		return nil
	}
	record := BlobSidecarRecord{TxHash: txHash(tx), Sidecar: bundle}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	writes := []vexostore.KVWrite{{
		Namespace: ModuleName,
		Key:       blobSidecarKey(record.TxHash),
		Value:     encoded,
	}}
	for _, blobHash := range bundle.BlobHashes {
		writes = append(writes, vexostore.KVWrite{
			Namespace: ModuleName,
			Key:       blobSidecarHashIndexKey(blobHash),
			Value:     []byte(record.TxHash),
		})
	}
	if batchStore, ok := store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func (module Module) validateBlobSidecarLimits(encoded string, bundle ethcompat.BlobSidecarBundle) error {
	policy := module.policy
	if policy.MaxBlobSidecarBlobs == 0 {
		policy.MaxBlobSidecarBlobs = DefaultPolicy().MaxBlobSidecarBlobs
	}
	if policy.MaxBlobSidecarBytes == 0 {
		policy.MaxBlobSidecarBytes = DefaultPolicy().MaxBlobSidecarBytes
	}
	if uint64(len(bundle.BlobHashes)) > policy.MaxBlobSidecarBlobs {
		return ethcompat.ErrInvalidBlobSidecar
	}
	if uint64(len(encoded)) > policy.MaxBlobSidecarBytes {
		return ethcompat.ErrInvalidBlobSidecar
	}
	return nil
}

func persistReceiptIndexes(ctx context.Context, store vexoapp.StateStore, results []types.Result) error {
	writes := make([]vexostore.KVWrite, 0)
	for index, result := range results {
		receipt, ok := receiptFromTxResult(result)
		if !ok || receipt.TxHash == "" || receipt.Height == 0 {
			continue
		}
		encoded, err := json.Marshal(ReceiptIndex{TxHash: receipt.TxHash, Height: receipt.Height, TxIndex: uint64(index)})
		if err != nil {
			return err
		}
		writes = append(writes, vexostore.KVWrite{
			Namespace: ModuleName,
			Key:       receiptIndexKey(receipt.TxHash),
			Value:     encoded,
		})
	}
	if len(writes) == 0 {
		return nil
	}
	if batchStore, ok := store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func receiptFromTxResult(result types.Result) (Receipt, bool) {
	if len(result.Data) == 0 {
		return Receipt{}, false
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil || receipt.TxHash == "" {
		return Receipt{}, false
	}
	return receipt, true
}

func persistExecutionResult(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, result contract.Result) error {
	writes, err := executionResultWrites(ctx, store, defaultAddress, result)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, writes)
}

func executionResultWrites(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, result contract.Result) ([]vexostore.KVWrite, error) {
	writes := make([]vexostore.KVWrite, 0, len(result.CodeWrites)+len(result.StorageWrites)+len(result.BalanceWrites)+len(result.NonceWrites)+len(result.AccountDeletions)*3)
	codeWrites, err := codeKVWrites(result.CodeWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, codeWrites...)
	storageWrites, err := storageKVWrites(defaultAddress, result.StorageWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, storageWrites...)
	balanceWrites, err := balanceKVWrites(result.BalanceWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, balanceWrites...)
	nonceWrites, err := nonceKVWrites(result.NonceWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, nonceWrites...)
	deletionWrites, err := accountDeletionKVWrites(ctx, store, result.AccountDeletions)
	if err != nil {
		return nil, err
	}
	writes = append(writes, deletionWrites...)
	return writes, nil
}

func persistCodeWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.CodeWrite) error {
	kvWrites, err := codeKVWrites(writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func codeKVWrites(writes []contract.CodeWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		if write.Delete {
			kvWrites = append(kvWrites, deleteWrite(ModuleName, codeKey(write.Address)))
			continue
		}
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: ModuleName, Key: codeKey(write.Address), Value: append([]byte(nil), write.Code...)})
	}
	return kvWrites, nil
}

func persistAccountDeletions(ctx context.Context, store vexoapp.StateStore, deletions []contract.AccountDeletion) error {
	writes, err := accountDeletionKVWrites(ctx, store, deletions)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, writes)
}

func accountDeletionKVWrites(ctx context.Context, store vexoapp.StateStore, deletions []contract.AccountDeletion) ([]vexostore.KVWrite, error) {
	writes := make([]vexostore.KVWrite, 0, len(deletions)*3)
	for _, deletion := range deletions {
		if deletion.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		writes = append(writes,
			deleteWrite(ModuleName, codeKey(deletion.Address)),
			deleteWrite("bank", evmBankKey(deletion.Address)),
			deleteWrite("auth", evmNonceKey(deletion.Address)),
		)
		prefixStore, ok := store.(vexostore.PrefixKVStore)
		if !ok {
			continue
		}
		pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, storageAccountPrefix(deletion.Address))
		if err != nil {
			return nil, err
		}
		for _, pair := range pairs {
			writes = append(writes, deleteWrite(ModuleName, pair.Key))
		}
	}
	return writes, nil
}

func persistStorageWrites(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, writes []contract.StorageWrite) error {
	kvWrites, err := storageKVWrites(defaultAddress, writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func storageKVWrites(defaultAddress types.Address, writes []contract.StorageWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		address := write.Address
		if address == "" {
			address = defaultAddress
		}
		if address == "" || write.Slot == "" {
			return nil, ErrInvalidEVMTx
		}
		key := storageKey(address, write.Slot)
		if write.Delete {
			kvWrites = append(kvWrites, deleteWrite(ModuleName, key))
			continue
		}
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: ModuleName, Key: key, Value: append([]byte(nil), write.Value...)})
	}
	return kvWrites, nil
}

func persistBalanceWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.BalanceWrite) error {
	kvWrites, err := balanceKVWrites(writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func balanceKVWrites(writes []contract.BalanceWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		encoded, err := encodeEthereumBalance(write)
		if err != nil {
			return nil, err
		}
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: "bank", Key: evmBankKey(write.Address), Value: encoded})
	}
	return kvWrites, nil
}

func encodeEthereumBalance(write contract.BalanceWrite) ([]byte, error) {
	if write.BalanceBig != nil {
		if write.BalanceBig.Sign() < 0 || write.BalanceBig.BitLen() > 256 {
			return nil, ErrInvalidEVMTx
		}
		encoded := trimLeftZero(write.BalanceBig.FillBytes(make([]byte, 32)))
		if len(encoded) == 0 {
			return []byte{0}, nil
		}
		return encoded, nil
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], write.Balance)
	return encoded[:], nil
}

func decodeEthereumBalance(value []byte) (uint64, string, error) {
	if len(value) == 0 {
		return 0, "", nil
	}
	if len(value) > 32 {
		return 0, "", ErrInvalidEVMTx
	}
	balance := new(big.Int).SetBytes(value)
	if !balance.IsUint64() {
		return 0, hexQuantityBigLocal(balance), nil
	}
	return balance.Uint64(), "", nil
}

func accountBalanceHex(account ethcompat.AccountState) string {
	if account.BalanceHex != "" {
		return account.BalanceHex
	}
	return hexQuantityLocal(account.Balance)
}

func persistNonceWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.NonceWrite) error {
	kvWrites, err := nonceKVWrites(writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func nonceKVWrites(writes []contract.NonceWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], write.Nonce)
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: "auth", Key: evmNonceKey(write.Address), Value: append([]byte(nil), encoded[:]...)})
	}
	return kvWrites, nil
}

func persistLog(ctx context.Context, store vexoapp.StateStore, log Log) error {
	if log.Address == "" {
		return nil
	}
	encoded, err := json.Marshal(log)
	if err != nil {
		return err
	}
	if err := store.Set(ctx, ModuleName, globalLogKey(log), encoded); err != nil {
		return err
	}
	return store.Set(ctx, ModuleName, addressLogKey(log), encoded)
}

func queryJSON(ctx vexoapp.Context, key []byte) vexoapp.QueryResponse {
	value, err := ctx.Store.Get(ctx.GoContext(), ModuleName, key)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return vexoapp.QueryResponse{Code: 3, Log: "EVM state not found"}
	}
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: append([]byte(nil), value...)}
}

func queryLogs(ctx vexoapp.Context, prefix []byte, legacyKey []byte) vexoapp.QueryResponse {
	if prefixStore, ok := ctx.Store.(vexostore.PrefixKVStore); ok {
		pairs, err := prefixStore.ExportPrefix(ctx.GoContext(), ModuleName, prefix)
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		if len(pairs) > 0 {
			logs := make([]Log, 0, len(pairs))
			for _, pair := range pairs {
				var log Log
				if err := json.Unmarshal(pair.Value, &log); err != nil {
					return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
				}
				logs = append(logs, log)
			}
			encoded, err := json.Marshal(logs)
			if err != nil {
				return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
			}
			return vexoapp.QueryResponse{Value: encoded}
		}
	}
	value, err := ctx.Store.Get(ctx.GoContext(), ModuleName, legacyKey)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return vexoapp.QueryResponse{Value: []byte("[]")}
	}
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: append([]byte(nil), value...)}
}

func evmHistoryPruneWrites(ctx context.Context, store vexoapp.StateStore, retainFrom uint64) ([]vexostore.KVWrite, error) {
	prefixStore, ok := store.(vexostore.PrefixKVStore)
	if !ok {
		return nil, nil
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte("receipt_index/"))
	if err != nil {
		return nil, err
	}
	writes := make([]vexostore.KVWrite, 0)
	for _, pair := range pairs {
		var index ReceiptIndex
		if err := json.Unmarshal(pair.Value, &index); err != nil {
			return nil, err
		}
		if index.Height == 0 || index.Height >= retainFrom || index.TxHash == "" {
			continue
		}
		receipt, err := loadReceiptForPrune(ctx, store, index.TxHash)
		if err != nil {
			return nil, err
		}
		writes = append(writes,
			deleteWrite(ModuleName, append([]byte(nil), pair.Key...)),
			deleteWrite(ModuleName, receiptKey(index.TxHash)),
		)
		for _, log := range receipt.Logs {
			writes = append(writes,
				deleteWrite(ModuleName, globalLogKey(log)),
				deleteWrite(ModuleName, addressLogKey(log)),
			)
		}
		blobRecord, found, err := loadBlobSidecarForPrune(ctx, store, index.TxHash)
		if err != nil {
			return nil, err
		}
		if found {
			writes = append(writes, deleteWrite(ModuleName, blobSidecarKey(index.TxHash)))
			for _, blobHash := range blobRecord.Sidecar.BlobHashes {
				writes = append(writes, deleteWrite(ModuleName, blobSidecarHashIndexKey(blobHash)))
			}
		}
	}
	return writes, nil
}

func loadReceiptForPrune(ctx context.Context, store vexoapp.StateStore, txHash string) (Receipt, error) {
	value, err := store.Get(ctx, ModuleName, receiptKey(txHash))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return Receipt{TxHash: txHash}, nil
	}
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func loadBlobSidecarForPrune(ctx context.Context, store vexoapp.StateStore, txHash string) (BlobSidecarRecord, bool, error) {
	value, err := store.Get(ctx, ModuleName, blobSidecarKey(txHash))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return BlobSidecarRecord{}, false, nil
	}
	if err != nil {
		return BlobSidecarRecord{}, false, err
	}
	var record BlobSidecarRecord
	if err := json.Unmarshal(value, &record); err != nil {
		return BlobSidecarRecord{}, false, err
	}
	return record, true, nil
}

func applyKVWrites(ctx context.Context, store vexoapp.StateStore, writes []vexostore.KVWrite) error {
	if len(writes) == 0 {
		return nil
	}
	if batchStore, ok := store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if write.Delete {
			if err := store.Delete(ctx, write.Namespace, write.Key); err != nil {
				return err
			}
			continue
		}
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func deleteWrite(namespace string, key []byte) vexostore.KVWrite {
	return vexostore.KVWrite{Namespace: namespace, Key: append([]byte(nil), key...), Delete: true}
}

func queryEthereumStateRoot(ctx vexoapp.Context, data []byte) vexoapp.QueryResponse {
	var request StateRootRequest
	if len(data) > 0 {
		if err := json.Unmarshal(data, &request); err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
	}
	if request.Height > 0 {
		meta, err := ethereumStateSnapshotMetaAt(ctx.GoContext(), ctx.Store, request.Height)
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return vexoapp.QueryResponse{Code: 3, Log: "Ethereum state snapshot not found"}
		}
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		encoded, err := json.Marshal(map[string]string{"state_root": meta.StateRoot})
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: encoded}
	}
	accounts, err := ethereumAccountsFromStore(ctx.GoContext(), ctx.Store)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	root, err := ethcompat.StateRoot(accounts)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(map[string]string{"state_root": root})
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func queryEthereumProof(ctx vexoapp.Context, data []byte) vexoapp.QueryResponse {
	var request ProofRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
	}
	if request.Address == "" {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	accounts, err := ethereumAccountsForProof(ctx.GoContext(), ctx.Store, request.Height)
	if err != nil {
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return vexoapp.QueryResponse{Code: 3, Log: "Ethereum state snapshot not found"}
		}
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	proof, err := ethcompat.GetProof(accounts, request.Address, request.StorageKeys)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func accountStateRequestHeight(data []byte) (uint64, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return 0, nil
	}
	var request AccountStateRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return 0, err
	}
	return request.Height, nil
}

func ethereumAccountAtHeight(ctx context.Context, stateStore vexoapp.StateStore, address types.Address, height uint64) (ethcompat.AccountState, bool, error) {
	accounts, err := ethereumAccountsForProof(ctx, stateStore, height)
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	target := canonicalAddressKey(address)
	for _, account := range accounts {
		if canonicalAddressKey(types.Address(account.Address)) == target {
			if account.Storage == nil {
				account.Storage = map[string][]byte{}
			}
			return account, true, nil
		}
	}
	return ethcompat.AccountState{}, false, nil
}

func ethereumAccountFromCurrentState(ctx context.Context, stateStore vexoapp.StateStore, address types.Address) (ethcompat.AccountState, bool, error) {
	accounts, err := ethereumAccountsFromStore(ctx, stateStore)
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	target := canonicalAddressKey(address)
	for _, account := range accounts {
		if canonicalAddressKey(types.Address(account.Address)) == target {
			if account.Storage == nil {
				account.Storage = map[string][]byte{}
			}
			return account, true, nil
		}
	}
	return ethcompat.AccountState{}, false, nil
}

func persistEthereumStateSnapshot(ctx context.Context, stateStore vexoapp.StateStore, height uint64) error {
	accounts, err := ethereumAccountsFromStore(ctx, stateStore)
	if err != nil {
		return err
	}
	root, err := ethcompat.StateRoot(accounts)
	if err != nil {
		return err
	}
	meta := ethereumStateSnapshotMeta{Height: height, StateRoot: root}
	encodedMeta, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := stateStore.Set(ctx, ethereumStateSnapshotNamespace, ethereumStateSnapshotMetaKey(height), encodedMeta); err != nil {
		return err
	}
	for _, account := range accounts {
		encodedAccount, err := json.Marshal(account)
		if err != nil {
			return err
		}
		if err := stateStore.Set(ctx, ethereumStateSnapshotNamespace, ethereumStateSnapshotAccountKey(height, types.Address(account.Address)), encodedAccount); err != nil {
			return err
		}
	}
	return nil
}

func ethereumAccountsForProof(ctx context.Context, stateStore vexoapp.StateStore, height uint64) ([]ethcompat.AccountState, error) {
	if height == 0 {
		return ethereumAccountsFromStore(ctx, stateStore)
	}
	return ethereumAccountsFromSnapshot(ctx, stateStore, height)
}

func ethereumStateSnapshotMetaAt(ctx context.Context, stateStore vexoapp.StateStore, height uint64) (ethereumStateSnapshotMeta, error) {
	if stateStore == nil {
		return ethereumStateSnapshotMeta{}, ErrStoreMissing
	}
	value, err := stateStore.Get(ctx, ethereumStateSnapshotNamespace, ethereumStateSnapshotMetaKey(height))
	if err != nil {
		return ethereumStateSnapshotMeta{}, err
	}
	var meta ethereumStateSnapshotMeta
	if err := json.Unmarshal(value, &meta); err != nil {
		return ethereumStateSnapshotMeta{}, err
	}
	if meta.Height != height || meta.StateRoot == "" {
		return ethereumStateSnapshotMeta{}, ErrInvalidEVMQuery
	}
	return meta, nil
}

func ethereumAccountsFromSnapshot(ctx context.Context, stateStore vexoapp.StateStore, height uint64) ([]ethcompat.AccountState, error) {
	if _, err := ethereumStateSnapshotMetaAt(ctx, stateStore, height); err != nil {
		return nil, err
	}
	prefixStore, ok := stateStore.(vexostore.PrefixKVStore)
	if !ok {
		return nil, ErrStoreMissing
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ethereumStateSnapshotNamespace, ethereumStateSnapshotAccountPrefix(height))
	if err != nil {
		return nil, err
	}
	accounts := make([]ethcompat.AccountState, 0, len(pairs))
	for _, pair := range pairs {
		var account ethcompat.AccountState
		if err := json.Unmarshal(pair.Value, &account); err != nil {
			return nil, err
		}
		if account.Storage == nil {
			account.Storage = map[string][]byte{}
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func ethereumAccountsFromStore(ctx context.Context, stateStore vexoapp.StateStore) ([]ethcompat.AccountState, error) {
	prefixStore, ok := stateStore.(vexostore.PrefixKVStore)
	if !ok {
		return nil, ErrStoreMissing
	}
	accounts := map[string]*ethcompat.AccountState{}
	accountFor := func(address types.Address) *ethcompat.AccountState {
		key := canonicalAddressKey(address)
		account := accounts[key]
		if account == nil {
			account = &ethcompat.AccountState{Address: key, Storage: map[string][]byte{}}
			accounts[key] = account
		}
		return account
	}
	balances, err := prefixStore.ExportPrefix(ctx, "bank", nil)
	if err != nil {
		return nil, err
	}
	for _, pair := range balances {
		address := types.Address(string(pair.Key))
		if !strings.HasPrefix(string(address), "0x") || len(pair.Value) == 0 {
			continue
		}
		balance, balanceHex, err := decodeEthereumBalance(pair.Value)
		if err != nil {
			return nil, err
		}
		account := accountFor(address)
		account.Balance = balance
		account.BalanceHex = balanceHex
	}
	nonces, err := prefixStore.ExportPrefix(ctx, "auth", []byte("nonce/"))
	if err != nil {
		return nil, err
	}
	for _, pair := range nonces {
		address := types.Address(strings.TrimPrefix(string(pair.Key), "nonce/"))
		if !strings.HasPrefix(string(address), "0x") {
			continue
		}
		if len(pair.Value) != 8 {
			return nil, ErrInvalidEVMTx
		}
		accountFor(address).Nonce = binary.BigEndian.Uint64(pair.Value)
	}
	codes, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte("code/"))
	if err != nil {
		return nil, err
	}
	for _, pair := range codes {
		address := types.Address(strings.TrimPrefix(string(pair.Key), "code/"))
		if !strings.HasPrefix(string(address), "0x") {
			continue
		}
		accountFor(address).Code = append([]byte(nil), pair.Value...)
	}
	storage, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte("storage/"))
	if err != nil {
		return nil, err
	}
	for _, pair := range storage {
		address, slot, ok := ethereumStorageKeyParts(pair.Key)
		if !ok {
			continue
		}
		accountFor(address).Storage[slot] = append([]byte(nil), pair.Value...)
	}
	result := make([]ethcompat.AccountState, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, *account)
	}
	return result, nil
}

func ethereumStorageKeyParts(key []byte) (types.Address, string, bool) {
	trimmed := strings.TrimPrefix(string(key), "storage/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "0x") || parts[1] == "" {
		return "", "", false
	}
	return types.Address(parts[0]), parts[1], true
}

func ethereumStateSnapshotMetaKey(height uint64) []byte {
	return []byte(fmt.Sprintf("%020d/meta", height))
}

func ethereumStateSnapshotAccountPrefix(height uint64) []byte {
	return []byte(fmt.Sprintf("%020d/accounts/", height))
}

func ethereumStateSnapshotAccountKey(height uint64, address types.Address) []byte {
	return append(ethereumStateSnapshotAccountPrefix(height), []byte(canonicalAddressKey(address))...)
}

func ethereumStateSnapshotHeightFromKey(key []byte) (uint64, bool) {
	raw := string(key)
	if len(raw) < 21 || raw[20] != '/' {
		return 0, false
	}
	height, err := strconv.ParseUint(raw[:20], 10, 64)
	if err != nil || height == 0 {
		return 0, false
	}
	return height, true
}

func createAddress(caller types.Address, code []byte, salt [32]byte) types.Address {
	codeHash := keccak256(code)
	seed := append([]byte{0xff}, addressBytes(caller)...)
	seed = append(seed, salt[:]...)
	seed = append(seed, codeHash...)
	final := keccak256(seed)
	return types.Address("0x" + hex.EncodeToString(final[12:]))
}

func createLegacyAddress(caller types.Address, nonce uint64) types.Address {
	final := keccak256(rlpLegacyCreateAddress(addressBytes(caller), nonce))
	return types.Address("0x" + hex.EncodeToString(final[12:]))
}

func rlpLegacyCreateAddress(address []byte, nonce uint64) []byte {
	address = rightMost20(address)
	noncePayload := rlpEncodeUint(nonce)
	payloadLen := 1 + len(address) + len(noncePayload)
	encoded := make([]byte, 0, 1+payloadLen)
	encoded = append(encoded, byte(0xc0+payloadLen))
	encoded = append(encoded, byte(0x80+len(address)))
	encoded = append(encoded, address...)
	encoded = append(encoded, noncePayload...)
	return encoded
}

func rlpEncodeUint(value uint64) []byte {
	if value == 0 {
		return []byte{0x80}
	}
	raw := make([]byte, 8)
	for index := len(raw) - 1; index >= 0; index-- {
		raw[index] = byte(value)
		value >>= 8
	}
	raw = trimLeftZero(raw)
	if len(raw) == 1 && raw[0] < 0x80 {
		return raw
	}
	encoded := make([]byte, 0, 1+len(raw))
	encoded = append(encoded, byte(0x80+len(raw)))
	encoded = append(encoded, raw...)
	return encoded
}

func trimLeftZero(value []byte) []byte {
	for len(value) > 0 && value[0] == 0 {
		value = value[1:]
	}
	return value
}

func txHash(tx types.Tx) string {
	if hash, found := vexoapp.TxTag(tx, ethcompat.TagHash); found && hash != "" {
		return hash
	}
	hash := sha256.Sum256(tx)
	return "0x" + hex.EncodeToString(hash[:])
}

func receiptKey(hash string) []byte {
	return []byte("receipts/" + strings.TrimPrefix(hash, "0x"))
}

func receiptIndexKey(hash string) []byte {
	return []byte("receipt_index/" + strings.TrimPrefix(hash, "0x"))
}

func blobSidecarKey(hash string) []byte {
	return []byte("blob_sidecars/by_tx/" + strings.ToLower(strings.TrimPrefix(hash, "0x")))
}

func blobSidecarHashIndexKey(hash string) []byte {
	return []byte("blob_sidecars/by_blob/" + strings.ToLower(strings.TrimPrefix(hash, "0x")))
}

func codeKey(address types.Address) []byte {
	return []byte("code/" + canonicalAddressKey(address))
}

func logsKey(address types.Address) []byte {
	return []byte("logs/" + string(address))
}

func globalLogsKey() []byte {
	return []byte("logs")
}

func globalLogPrefix() []byte {
	return []byte("logs/by_height/")
}

func addressLogPrefix(address types.Address) []byte {
	return []byte("logs/by_address/" + string(address) + "/")
}

func globalLogKey(log Log) []byte {
	return append(globalLogPrefix(), []byte(logOrderKey(log))...)
}

func addressLogKey(log Log) []byte {
	return append(addressLogPrefix(types.Address(log.Address)), []byte(logOrderKey(log))...)
}

func logOrderKey(log Log) string {
	return fmt.Sprintf("%020d/%s/%020d", log.BlockNumber, strings.TrimPrefix(log.TransactionHash, "0x"), log.LogIndex)
}

func storageKey(address types.Address, slot string) []byte {
	return []byte("storage/" + canonicalAddressKey(address) + "/" + strings.TrimPrefix(normalizeSlot(slot), "0x"))
}

func storageAccountPrefix(address types.Address) []byte {
	return []byte("storage/" + canonicalAddressKey(address) + "/")
}

func headerUnixSeconds(header types.Header) uint64 {
	if header.TimeUnixNano <= 0 {
		return 0
	}
	return uint64(header.TimeUnixNano / 1_000_000_000)
}

func loadCode(ctx context.Context, store vexoapp.StateStore, address types.Address) ([]byte, error) {
	if store == nil {
		return nil, ErrStoreMissing
	}
	code, err := store.Get(ctx, ModuleName, codeKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, ErrInvalidEVMTx
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), code...), nil
}

type evmStateReader struct {
	store vexoapp.StateStore
}

type callStateReader interface {
	contract.StateReader
	contract.BalanceBigReader
	contract.BalanceReader
	contract.NonceReader
	contract.BlockHashReader
}

func stateReaderForCall(ctx vexoapp.Context, height uint64) (callStateReader, error) {
	if ctx.Store == nil {
		return evmStateReader{}, nil
	}
	if height == 0 {
		return evmStateReader{store: ctx.Store}, nil
	}
	accounts, err := ethereumAccountsForProof(ctx.GoContext(), ctx.Store, height)
	if err != nil {
		return nil, err
	}
	return newEVMSnapshotStateReader(ctx.Store, accounts), nil
}

type overrideStateReader struct {
	base     callStateReader
	accounts map[string]CallStateOverride
}

func newOverrideStateReader(base callStateReader, overrides map[string]CallStateOverride) callStateReader {
	if len(overrides) == 0 {
		return base
	}
	normalized := make(map[string]CallStateOverride, len(overrides))
	for address, override := range overrides {
		if len(override.State) > 0 {
			state := make(map[string]string, len(override.State))
			for slot, value := range override.State {
				state[normalizeSlot(slot)] = value
			}
			override.State = state
		}
		if len(override.StateDiff) > 0 {
			stateDiff := make(map[string]string, len(override.StateDiff))
			for slot, value := range override.StateDiff {
				stateDiff[normalizeSlot(slot)] = value
			}
			override.StateDiff = stateDiff
		}
		normalized[canonicalAddressKey(types.Address(address))] = override
	}
	return overrideStateReader{base: base, accounts: normalized}
}

func (reader overrideStateReader) override(address types.Address) (CallStateOverride, bool) {
	override, found := reader.accounts[canonicalAddressKey(address)]
	return override, found
}

func (reader overrideStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	if override, found := reader.override(address); found && override.Code != "" {
		return decodeOverrideBytes(override.Code)
	}
	return reader.base.Code(ctx, address)
}

func (reader overrideStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	override, found := reader.override(address)
	if !found {
		return reader.base.Storage(ctx, address, slot)
	}
	normalized := normalizeSlot(slot)
	if override.State != nil {
		return decodeOverrideBytes(override.State[normalized])
	}
	if override.StateDiff != nil {
		if value, ok := override.StateDiff[normalized]; ok {
			return decodeOverrideBytes(value)
		}
	}
	return reader.base.Storage(ctx, address, slot)
}

func (reader overrideStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	balance, err := reader.BalanceBig(ctx, address)
	if err != nil {
		return 0, err
	}
	if balance == nil || balance.Sign() == 0 {
		return 0, nil
	}
	if !balance.IsUint64() {
		return 0, ErrInvalidEVMTx
	}
	return balance.Uint64(), nil
}

func (reader overrideStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	if override, found := reader.override(address); found && override.Balance != "" {
		return parseOverrideBig(override.Balance)
	}
	return reader.base.BalanceBig(ctx, address)
}

func (reader overrideStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	if override, found := reader.override(address); found && override.Nonce != nil {
		return *override.Nonce, nil
	}
	return reader.base.Nonce(ctx, address)
}

func (reader overrideStateReader) BlockHash(ctx context.Context, height uint64) (types.Hash, error) {
	return reader.base.BlockHash(ctx, height)
}

func decodeOverrideBytes(value string) ([]byte, error) {
	if value == "" || value == "0x" {
		return nil, nil
	}
	clean := strings.TrimPrefix(value, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil {
		return nil, ErrInvalidEVMQuery
	}
	return decoded, nil
}

func parseOverrideBig(value string) (*big.Int, error) {
	if value == "" {
		return new(big.Int), nil
	}
	base := 10
	clean := value
	if strings.HasPrefix(value, "0x") {
		base = 16
		clean = strings.TrimPrefix(value, "0x")
	}
	parsed, ok := new(big.Int).SetString(clean, base)
	if !ok || parsed.Sign() < 0 || parsed.BitLen() > 256 {
		return nil, ErrInvalidEVMQuery
	}
	return parsed, nil
}

func (reader evmStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	if reader.store == nil {
		return nil, ErrStoreMissing
	}
	code, err := reader.store.Get(ctx, ModuleName, codeKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), code...), nil
}

type evmSnapshotStateReader struct {
	store    vexoapp.StateStore
	accounts map[string]ethcompat.AccountState
}

func newEVMSnapshotStateReader(stateStore vexoapp.StateStore, accounts []ethcompat.AccountState) evmSnapshotStateReader {
	reader := evmSnapshotStateReader{store: stateStore, accounts: make(map[string]ethcompat.AccountState, len(accounts))}
	for _, account := range accounts {
		if account.Storage == nil {
			account.Storage = map[string][]byte{}
		}
		reader.accounts[canonicalAddressKey(types.Address(account.Address))] = account
	}
	return reader
}

func (reader evmSnapshotStateReader) account(address types.Address) ethcompat.AccountState {
	return reader.accounts[canonicalAddressKey(address)]
}

func (reader evmSnapshotStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	return append([]byte(nil), reader.account(address).Code...), nil
}

func (reader evmSnapshotStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	account := reader.account(address)
	if account.Storage == nil {
		return nil, nil
	}
	value := account.Storage[normalizeSlot(slot)]
	if len(value) == 0 {
		value = account.Storage[slot]
	}
	return append([]byte(nil), value...), nil
}

func (reader evmSnapshotStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	balance, err := reader.BalanceBig(ctx, address)
	if err != nil {
		return 0, err
	}
	if balance == nil || balance.Sign() == 0 {
		return 0, nil
	}
	if !balance.IsUint64() {
		return 0, ErrInvalidEVMTx
	}
	return balance.Uint64(), nil
}

func (reader evmSnapshotStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	account := reader.account(address)
	if account.BalanceHex != "" {
		value, ok := new(big.Int).SetString(strings.TrimPrefix(account.BalanceHex, "0x"), 16)
		if !ok || value.Sign() < 0 || value.BitLen() > 256 {
			return nil, ErrInvalidEVMTx
		}
		return value, nil
	}
	return new(big.Int).SetUint64(account.Balance), nil
}

func (reader evmSnapshotStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	return reader.account(address).Nonce, nil
}

func (reader evmSnapshotStateReader) BlockHash(ctx context.Context, height uint64) (types.Hash, error) {
	return evmStateReader{store: reader.store}.BlockHash(ctx, height)
}

func (reader evmStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	if reader.store == nil {
		return nil, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, ModuleName, storageKey(address, slot))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func (reader evmStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	balance, err := reader.BalanceBig(ctx, address)
	if err != nil {
		return 0, err
	}
	if balance == nil || balance.Sign() == 0 {
		return 0, nil
	}
	if !balance.IsUint64() {
		return 0, ErrInvalidEVMTx
	}
	return balance.Uint64(), nil
}

func (reader evmStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	if reader.store == nil {
		return nil, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, "bank", evmBankKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = reader.store.Get(ctx, "bank", []byte(address))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return new(big.Int), nil
		}
	}
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return new(big.Int), nil
	}
	if len(value) > 32 {
		return nil, ErrInvalidEVMTx
	}
	return new(big.Int).SetBytes(value), nil
}

func (reader evmStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	if reader.store == nil {
		return 0, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, "auth", evmNonceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, ErrInvalidEVMTx
	}
	return binary.BigEndian.Uint64(value), nil
}

func (reader evmStateReader) BlockHash(ctx context.Context, height uint64) (types.Hash, error) {
	var zero types.Hash
	if reader.store == nil {
		return zero, ErrStoreMissing
	}
	blockStore, ok := reader.store.(interface {
		BlockByHeight(context.Context, types.Height) (vexostore.BlockRecord, error)
	})
	if !ok {
		return zero, nil
	}
	record, err := blockStore.BlockByHeight(ctx, types.Height(height))
	if errors.Is(err, vexostore.ErrBlockNotFound) {
		return zero, nil
	}
	if err != nil {
		return zero, err
	}
	return record.Hash, nil
}

func txGasPrice(tx types.Tx) uint64 {
	if gasPrice, found := vexoapp.TxUintTag(tx, ethcompat.TagGasPrice); found {
		return gasPrice
	}
	meta := vexoapp.ParseTxMeta(tx)
	if meta.Gas > 0 && meta.Fee > 0 {
		return meta.Fee / meta.Gas
	}
	return 0
}

func txGasFeeCap(tx types.Tx) uint64 {
	if feeCap, found := vexoapp.TxUintTag(tx, ethcompat.TagMaxFeePerGas); found {
		return feeCap
	}
	return txGasPrice(tx)
}

func txGasTipCap(tx types.Tx) uint64 {
	if tipCap, found := vexoapp.TxUintTag(tx, ethcompat.TagMaxPriorityFeePerGas); found {
		return tipCap
	}
	return txGasPrice(tx)
}

func txBlobGasFeeCap(tx types.Tx) uint64 {
	if feeCap, found := vexoapp.TxUintTag(tx, ethcompat.TagBlobGasFeeCap); found {
		return feeCap
	}
	return 0
}

func txNonce(tx types.Tx) uint64 {
	if nonce, found := vexoapp.TxUintTag(tx, "nonce"); found {
		return nonce
	}
	return 0
}

func txRawEthereum(tx types.Tx) string {
	raw, _ := vexoapp.TxTag(tx, ethcompat.TagRaw)
	return raw
}

func txGasLimit(tx types.Tx, fallback uint64) uint64 {
	if !ethcompat.IsEthereumTx(tx) {
		return fallback
	}
	if gas, found := vexoapp.TxUintTag(tx, "gas"); found && gas > 0 {
		return gas
	}
	if gas, found := vexoapp.TxUintTag(tx, "gas_limit"); found && gas > 0 {
		return gas
	}
	return fallback
}

func txBaseFee(tx types.Tx) uint64 {
	if baseFee, found := vexoapp.TxUintTag(tx, ethcompat.TagBaseFee); found {
		return baseFee
	}
	return 0
}

func txExecutionBaseFee(ctx vexoapp.Context, tx types.Tx) uint64 {
	if ethcompat.IsEthereumTx(tx) && ctx.BaseFee > 0 {
		return ctx.BaseFee
	}
	if baseFee := txBaseFee(tx); baseFee > 0 {
		return baseFee
	}
	return ctx.BaseFee
}

func txBlobBaseFee(tx types.Tx) uint64 {
	if blobBaseFee, found := vexoapp.TxUintTag(tx, ethcompat.TagBlobBaseFee); found {
		return blobBaseFee
	}
	return 0
}

func txExecutionBlobBaseFee(ctx vexoapp.Context, tx types.Tx) uint64 {
	if ethcompat.IsEthereumTx(tx) && ctx.BlobBaseFee > 0 {
		return ctx.BlobBaseFee
	}
	if blobBaseFee := txBlobBaseFee(tx); blobBaseFee > 0 {
		return blobBaseFee
	}
	return ctx.BlobBaseFee
}

func txBlobHashes(tx types.Tx) []types.Hash {
	return parseBlobHashes(txBlobHashStrings(tx))
}

func txBlobHashStrings(tx types.Tx) []string {
	encoded, found := vexoapp.TxTag(tx, ethcompat.TagBlobHashes)
	if !found || encoded == "" {
		return nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var hashes []string
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return nil
	}
	out := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		if decoded, err := hex.DecodeString(strings.TrimPrefix(hash, "0x")); err == nil && len(decoded) == len(types.Hash{}) {
			out = append(out, "0x"+strings.ToLower(hex.EncodeToString(decoded)))
		}
	}
	return out
}

func sameBlobHashes(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !strings.EqualFold(left[index], right[index]) {
			return false
		}
	}
	return true
}

func parseBlobHashes(raw []string) []types.Hash {
	if len(raw) == 0 {
		return nil
	}
	hashes := make([]types.Hash, 0, len(raw))
	for _, value := range raw {
		decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
		if err != nil || len(decoded) != len(types.Hash{}) {
			return nil
		}
		var hash types.Hash
		copy(hash[:], decoded)
		hashes = append(hashes, hash)
	}
	return hashes
}

func evmBankKey(address types.Address) []byte {
	return []byte(canonicalAddressKey(address))
}

func evmNonceKey(address types.Address) []byte {
	return []byte("nonce/" + canonicalAddressKey(address))
}

func create2Salt(raw string) [32]byte {
	clean := strings.TrimPrefix(raw, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err == nil && len(decoded) <= 32 {
		var salt [32]byte
		copy(salt[32-len(decoded):], decoded)
		return salt
	}
	var salt [32]byte
	copy(salt[:], keccak256([]byte(raw)))
	return salt
}

func addressBytes(address types.Address) []byte {
	clean := strings.TrimPrefix(string(address), "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err == nil && len(decoded) <= 20 {
		out := make([]byte, 20)
		copy(out[20-len(decoded):], decoded)
		return out
	}
	hash := keccak256([]byte(address))
	return hash[12:]
}

func rightMost20(raw []byte) []byte {
	if len(raw) > 20 {
		return raw[len(raw)-20:]
	}
	if len(raw) == 20 {
		return raw
	}
	padded := make([]byte, 20)
	copy(padded[20-len(raw):], raw)
	return padded
}

func canonicalAddressKey(address types.Address) string {
	raw := string(address)
	clean := strings.TrimPrefix(raw, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 20 {
		return raw
	}
	padded := make([]byte, 20)
	copy(padded[20-len(decoded):], decoded)
	return "0x" + hex.EncodeToString(padded)
}

func normalizeSlot(slot string) string {
	clean := strings.TrimPrefix(slot, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 32 {
		decoded = keccak256([]byte(slot))
	}
	padded := make([]byte, 32)
	copy(padded[32-len(decoded):], decoded)
	return "0x" + hex.EncodeToString(padded)
}

func keccak256(values ...[]byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	for _, value := range values {
		_, _ = hasher.Write(value)
	}
	return hasher.Sum(nil)
}

func cloneMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func validateEthereumRawTx(ctx vexoapp.Context, tx types.Tx) error {
	return Module{policy: DefaultPolicy()}.validateEthereumRawTx(ctx, tx)
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
	if ctx.BaseFee > 0 || ctx.BlobBaseFee > 0 {
		return ethcompat.ValidateCanonicalTxForExecution(tx, options)
	}
	return ethcompat.ValidateCanonicalTxWithOptions(tx, options)
}

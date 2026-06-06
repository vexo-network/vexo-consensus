package evm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	Output          string `json:"output,omitempty"`
	Logs            []Log  `json:"logs,omitempty"`
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

type CallRequest struct {
	VM       string `json:"vm"`
	From     string `json:"from"`
	To       string `json:"to"`
	Method   string `json:"method"`
	Input    string `json:"input,omitempty"`
	GasLimit uint64 `json:"gas_limit,omitempty"`
	Value    uint64 `json:"value,omitempty"`
}

type CallResponse struct {
	Output  string `json:"output,omitempty"`
	GasUsed uint64 `json:"gas_used,omitempty"`
}

type ProofRequest struct {
	Address     string   `json:"address"`
	StorageKeys []string `json:"storage_keys,omitempty"`
	Height      uint64   `json:"height,omitempty"`
}

type StateRootRequest struct {
	Height uint64 `json:"height,omitempty"`
}

type ethereumStateSnapshotMeta struct {
	Height    uint64 `json:"height"`
	StateRoot string `json:"state_root"`
}

func NewModule() Module {
	registry := contract.NewRegistry()
	_ = registry.Register(gethbackend.New())
	return Module{registry: registry}
}

func NewModuleWithRegistry(registry *contract.Registry) Module {
	return Module{registry: registry}
}

func (module Module) CloneModule() vexoapp.Module {
	return Module{registry: module.registry}
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

func (Module) BeginBlock(ctx vexoapp.Context, header types.Header) error { return nil }

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
		if err := validateEthereumRawTx(ctx, tx); err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		if err := ctx.ConsumeGas(callGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		return module.deliverCall(ctx, tx, canonical.Args)
	case "deploy":
		if err := ctx.ConsumeGas(deployGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		return module.deliverDeploy(ctx, tx, canonical.Args)
	case "eth_deploy":
		if err := validateEthereumRawTx(ctx, tx); err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		if err := ctx.ConsumeGas(deployGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
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
	return persistEthereumStateSnapshot(ctx.GoContext(), ctx.Store, uint64(ctx.Height))
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
	if len(writes) == 0 {
		return nil
	}
	if batchStore, ok := ctx.Store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx.GoContext(), writes)
	}
	for _, write := range writes {
		if err := ctx.Store.Delete(ctx.GoContext(), write.Namespace, write.Key); err != nil {
			return err
		}
	}
	return nil
}

func (Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return 0, ErrInvalidEVMTx
	}
	switch canonical.Action {
	case "call":
		return callGasCost, nil
	case "deploy", "eth_deploy":
		return deployGasCost, nil
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
	case "receipt":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		return queryJSON(ctx, receiptKey(req.Path[1]))
	case "code":
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
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
	invocation.BaseFee = txBaseFee(tx)
	invocation.Coinbase = types.Address("fee_collector")
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := persistStorageWrites(ctx.GoContext(), ctx.Store, invocation.Contract, result.StorageWrites); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := persistBalanceWrites(ctx.GoContext(), ctx.Store, result.BalanceWrites); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	receipt := receiptFromResult(tx, ctx.Height, invocation, "", result)
	if err := persistReceipt(ctx.GoContext(), ctx.Store, receipt); err != nil {
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
	value := uint64(0)
	if len(args) == 5 {
		value, err = strconv.ParseUint(args[4], 10, 64)
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
		GasLimit:      ctx.GasLimit(),
		Value:         value,
		Salt:          salt[:],
		State:         evmStateReader{store: ctx.Store},
		BlockNumber:   uint64(ctx.Height),
		Timestamp:     headerUnixSeconds(ctx.Header),
		BlockGasLimit: ctx.GasLimit(),
		GasPrice:      txGasPrice(tx),
		BaseFee:       txBaseFee(tx),
		Coinbase:      types.Address("fee_collector"),
	}
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	deployedCode := result.DeployedCode
	if len(deployedCode) == 0 {
		deployedCode = code
	}
	if err := ctx.Store.Set(ctx.GoContext(), ModuleName, codeKey(contractAddress), deployedCode); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := persistStorageWrites(ctx.GoContext(), ctx.Store, invocation.Contract, result.StorageWrites); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := persistBalanceWrites(ctx.GoContext(), ctx.Store, result.BalanceWrites); err != nil {
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
	value, err := strconv.ParseUint(args[4], 10, 64)
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
		GasLimit:      ctx.GasLimit(),
		Value:         value,
		State:         evmStateReader{store: ctx.Store},
		BlockNumber:   uint64(ctx.Height),
		Timestamp:     headerUnixSeconds(ctx.Header),
		BlockGasLimit: ctx.GasLimit(),
		GasPrice:      txGasPrice(tx),
		BaseFee:       txBaseFee(tx),
		Coinbase:      types.Address("fee_collector"),
	}
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	deployedCode := result.DeployedCode
	if len(deployedCode) == 0 {
		deployedCode = code
	}
	if err := ctx.Store.Set(ctx.GoContext(), ModuleName, codeKey(contractAddress), deployedCode); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := persistStorageWrites(ctx.GoContext(), ctx.Store, invocation.Contract, result.StorageWrites); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := persistBalanceWrites(ctx.GoContext(), ctx.Store, result.BalanceWrites); err != nil {
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
	var code []byte
	var state contract.StateReader
	if ctx.Store != nil {
		var err error
		code, err = loadCode(ctx.GoContext(), ctx.Store, types.Address(request.To))
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		state = evmStateReader{store: ctx.Store}
	}
	result, err := module.registry.Execute(ctx.GoContext(), contract.Invocation{
		VM:            request.VM,
		Caller:        types.Address(request.From),
		Contract:      types.Address(request.To),
		Method:        request.Method,
		Input:         input,
		GasLimit:      request.GasLimit,
		Value:         request.Value,
		Code:          code,
		State:         state,
		ReadOnly:      true,
		BlockNumber:   uint64(ctx.Height),
		Timestamp:     headerUnixSeconds(ctx.Header),
		BlockGasLimit: ctx.GasLimit(),
		Coinbase:      types.Address("fee_collector"),
	})
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(CallResponse{Output: "0x" + hex.EncodeToString(result.Output), GasUsed: result.GasUsed})
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
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
	value := uint64(0)
	if len(args) == 7 {
		value, err = strconv.ParseUint(args[6], 10, 64)
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
		Value:    value,
	}
	if invocation.VM == "" || invocation.Caller == "" || invocation.Contract == "" || invocation.Method == "" {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	return invocation, nil
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

func persistStorageWrites(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, writes []contract.StorageWrite) error {
	for _, write := range writes {
		address := write.Address
		if address == "" {
			address = defaultAddress
		}
		if address == "" || write.Slot == "" {
			return ErrInvalidEVMTx
		}
		key := storageKey(address, write.Slot)
		if write.Delete {
			if err := store.Delete(ctx, ModuleName, key); err != nil {
				return err
			}
			continue
		}
		if err := store.Set(ctx, ModuleName, key, append([]byte(nil), write.Value...)); err != nil {
			return err
		}
	}
	return nil
}

func persistBalanceWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.BalanceWrite) error {
	for _, write := range writes {
		if write.Address == "" {
			return ErrInvalidEVMTx
		}
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], write.Balance)
		if err := store.Set(ctx, "bank", evmBankKey(write.Address), encoded[:]); err != nil {
			return err
		}
	}
	return nil
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
		if len(pair.Value) != 8 {
			return nil, ErrInvalidEVMTx
		}
		accountFor(address).Balance = binary.BigEndian.Uint64(pair.Value)
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
	if reader.store == nil {
		return 0, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, "bank", evmBankKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = reader.store.Get(ctx, "bank", []byte(address))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return 0, nil
		}
	}
	if err != nil {
		return 0, err
	}
	if len(value) == 0 {
		return 0, nil
	}
	if len(value) != 8 {
		return 0, ErrInvalidEVMTx
	}
	return binary.BigEndian.Uint64(value), nil
}

func (reader evmStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	if reader.store == nil {
		return 0, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, "auth", []byte("nonce/"+string(address)))
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

func txBaseFee(tx types.Tx) uint64 {
	if baseFee, found := vexoapp.TxUintTag(tx, ethcompat.TagBaseFee); found {
		return baseFee
	}
	return 0
}

func evmBankKey(address types.Address) []byte {
	return []byte(canonicalAddressKey(address))
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
	if !ethcompat.IsEthereumTx(tx) {
		return nil
	}
	return ethcompat.ValidateCanonicalTx(tx, ethcompat.ChainNumericID(ctx.ChainID))
}

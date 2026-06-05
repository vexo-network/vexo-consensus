package evm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/events"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "evm"

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

func NewModule() Module {
	return Module{registry: contract.NewRegistry()}
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
		if err := ctx.ConsumeGas(callGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		return module.deliverCall(ctx, tx, canonical.Args)
	case "deploy":
		if err := ctx.ConsumeGas(deployGasCost); err != nil {
			return types.Result{Code: 6, Log: err.Error()}
		}
		return module.deliverDeploy(ctx, tx, canonical.Args)
	default:
		return types.Result{Code: 2, Log: ErrInvalidEVMTx.Error()}
	}
}

func (Module) EndBlock(ctx vexoapp.Context) error { return nil }

func (Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return 0, ErrInvalidEVMTx
	}
	switch canonical.Action {
	case "call":
		return callGasCost, nil
	case "deploy":
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
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		return queryJSON(ctx, logsKey(types.Address(req.Path[1])))
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
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
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
	contractAddress := createAddress(types.Address(args[1]), code, args[3], ctx.Height)
	invocation := contract.Invocation{
		VM:       args[0],
		Caller:   types.Address(args[1]),
		Contract: contractAddress,
		Method:   "deploy",
		Input:    code,
		GasLimit: ctx.GasLimit(),
		Value:    value,
	}
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := ctx.Store.Set(ctx.GoContext(), ModuleName, codeKey(contractAddress), code); err != nil {
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
	result, err := module.registry.Execute(ctx.GoContext(), contract.Invocation{
		VM:       request.VM,
		Caller:   types.Address(request.From),
		Contract: types.Address(request.To),
		Method:   request.Method,
		Input:    input,
		GasLimit: request.GasLimit,
		Value:    request.Value,
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
		To:              string(invocation.Contract),
		ContractAddress: contractAddress,
		VM:              invocation.VM,
		GasUsed:         result.GasUsed,
		Output:          "0x" + hex.EncodeToString(result.Output),
		Logs:            make([]Log, 0, len(result.Logs)),
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
		if log.Address == "" {
			continue
		}
		logs := []Log{}
		raw, err := store.Get(ctx, ModuleName, logsKey(types.Address(log.Address)))
		if err != nil && !errors.Is(err, vexostore.ErrKeyNotFound) {
			return err
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &logs); err != nil {
				return err
			}
		}
		logs = append(logs, log)
		encodedLogs, err := json.Marshal(logs)
		if err != nil {
			return err
		}
		if err := store.Set(ctx, ModuleName, logsKey(types.Address(log.Address)), encodedLogs); err != nil {
			return err
		}
	}
	return nil
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

func createAddress(caller types.Address, code []byte, salt string, height types.Height) types.Address {
	seed := append([]byte(string(caller)+":"+salt+":"+strconv.FormatUint(uint64(height), 10)+":"), code...)
	hash := sha256.Sum256(seed)
	return types.Address("0x" + hex.EncodeToString(hash[len(hash)-20:]))
}

func txHash(tx types.Tx) string {
	hash := sha256.Sum256(tx)
	return "0x" + hex.EncodeToString(hash[:])
}

func receiptKey(hash string) []byte {
	return []byte("receipts/" + strings.TrimPrefix(hash, "0x"))
}

func codeKey(address types.Address) []byte {
	return []byte("code/" + string(address))
}

func logsKey(address types.Address) []byte {
	return []byte("logs/" + string(address))
}

func storageKey(address types.Address, slot string) []byte {
	return []byte("storage/" + string(address) + "/" + strings.TrimPrefix(slot, "0x"))
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

package evm

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/events"
	gethbackend "github.com/vexo-network/vexo-consensus/modules/evm/backend/geth"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func (module Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if len(req.Path) == 0 {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	if module.initErr != nil && req.Path[0] == "call" {
		return vexoapp.QueryResponse{Code: 1, Log: module.initErr.Error()}
	}
	if req.Path[0] == "call" {
		return module.queryCall(ctx, req.Data)
	}
	if req.Path[0] == "state-backend" || req.Path[0] == "state_backend" {
		return module.queryStateBackend()
	}
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStoreMissing.Error()}
	}
	switch req.Path[0] {
	case "eth_state_root":
		return queryEthereumStateRoot(ctx, module.roots, req.Data)
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
			if isHistoricalStateUnavailable(err) {
				return vexoapp.QueryResponse{Code: 3, Log: errHistoricalStateNotFound}
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
		txHash, err := ctx.Store.Get(ctx.GoContext(), ReceiptNamespace, blobSidecarHashIndexKey(req.Path[1]))
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
			if isHistoricalStateUnavailable(err) {
				return vexoapp.QueryResponse{Code: 3, Log: errHistoricalStateNotFound}
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
			if isHistoricalStateUnavailable(err) {
				return vexoapp.QueryResponse{Code: 3, Log: errHistoricalStateNotFound}
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
			return queryLogs(ctx, req.Data, globalLogHeightPrefix)
		}
		if len(req.Path) != 2 {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
		address := types.Address(req.Path[1])
		return queryLogs(ctx, req.Data, func(height uint64) []byte { return addressLogHeightPrefix(address, height) })
	default:
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
}

func (module Module) queryStateBackend() vexoapp.QueryResponse {
	gethDependency := gethbackend.GoEthereumDependencyInfo()
	info := StateBackendInfo{
		Name:                    "vexo-evm-state",
		Version:                 "1",
		ExecutionBackend:        "go-ethereum/core/vm",
		GethModule:              gethDependency.Module,
		GethVersion:             gethDependency.Version,
		GethChecksum:            gethDependency.Sum,
		GethReplace:             gethDependency.Replace,
		AccountNamespace:        ModuleName,
		AuthNamespace:           authNamespace,
		NativeBalanceNamespace:  bankNamespace,
		NativeBalanceKeyFormat:  "canonical 20-byte hex address under bank/balances/{address}",
		NativeBalanceBits:       256,
		NativeCoinSharedWithEVM: true,
		HistoricalSnapshots:     true,
		StateRootProofs:         true,
		StorageProofs:           true,
		StrictStateRootCapable:  true,
		GethLibraryBoundary:     "EVM opcode execution and Ethereum tx decoding stay inside the geth-backed VM adapter; Vexo owns consensus, storage, native coin accounting, and networking.",
	}
	if module.registry != nil {
		info.LinkedVMs = module.registry.Names()
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
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
	gasPrice, err := callRequestQuantity(request.GasPrice, request.GasPriceHex)
	if err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	gasFeeCap, err := callRequestQuantity(request.MaxFeePerGas, request.MaxFeePerGasHex)
	if err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	gasTipCap, err := callRequestQuantity(request.MaxPriorityFeePerGas, request.MaxPriorityFeePerGasHex)
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
		GasLimit:                  module.queryGasLimit(request.GasLimit),
		Value:                     uint64Amount(callValue),
		ValueBig:                  callValue,
		Code:                      code,
		State:                     state,
		ReadOnly:                  false,
		BlockNumber:               blockNumber,
		Timestamp:                 timestamp,
		BlockGasLimit:             blockGasLimit,
		GasPrice:                  uint64Amount(gasPrice),
		GasPriceBig:               gasPrice,
		GasFeeCap:                 uint64Amount(gasFeeCap),
		GasFeeCapBig:              gasFeeCap,
		GasTipCap:                 uint64Amount(gasTipCap),
		GasTipCapBig:              gasTipCap,
		BaseFee:                   baseFee,
		BlobBaseFee:               blobBaseFee,
		BlobHashes:                parseBlobHashes(request.BlobHashes),
		Coinbase:                  coinbaseAlias,
		AccessList:                append([]contract.AccessListEntry(nil), request.AccessList...),
		Nonce:                     request.Nonce,
		EthereumSimulation:        true,
		SetCodeAuthorizationsJSON: request.SetCodeAuthorizationsJSON,
		Trace:                     request.Trace,
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

// queryGasLimit bounds the gas one read-only execution may spend. A query runs
// on the node that serves it, outside any block's gas accounting, so an
// unbounded request would let a single caller occupy the node for as long as
// the interpreter runs. A request without a limit gets the cap itself.
func (module Module) queryGasLimit(requested uint64) uint64 {
	if gasCap := module.gasCap(); requested == 0 || requested > gasCap {
		return gasCap
	}
	return requested
}

func callRequestValue(request CallRequest) (*big.Int, error) {
	var parsed *big.Int
	if request.ValueWei != "" {
		value, ok := new(big.Int).SetString(request.ValueWei, 10)
		if !ok || value.Sign() < 0 || value.BitLen() > 256 {
			return nil, ErrInvalidEVMQuery
		}
		parsed = value
	}
	if request.ValueHex != "" {
		value, ok := new(big.Int).SetString(strings.TrimPrefix(request.ValueHex, "0x"), 16)
		if !ok || value.Sign() < 0 || value.BitLen() > 256 {
			return nil, ErrInvalidEVMQuery
		}
		if parsed != nil && parsed.Cmp(value) != 0 {
			return nil, ErrInvalidEVMQuery
		}
		parsed = value
	}
	if parsed != nil {
		return parsed, nil
	}
	return new(big.Int).SetUint64(request.Value), nil
}

func callRequestQuantity(compat uint64, hexValue string) (*big.Int, error) {
	var parsed *big.Int
	if hexValue != "" {
		if !strings.HasPrefix(hexValue, "0x") {
			return nil, ErrInvalidEVMQuery
		}
		value, ok := new(big.Int).SetString(strings.TrimPrefix(hexValue, "0x"), 16)
		if !ok || value.Sign() < 0 || value.BitLen() > 256 {
			return nil, ErrInvalidEVMQuery
		}
		parsed = value
	}
	if parsed != nil {
		if compat != 0 && (!parsed.IsUint64() || parsed.Uint64() != compat) {
			return nil, ErrInvalidEVMQuery
		}
		return parsed, nil
	}
	return new(big.Int).SetUint64(compat), nil
}

func (module Module) estimateInvocationGas(ctx vexoapp.Context, invocation contract.Invocation) (uint64, error) {
	if module.registry == nil {
		return 0, ErrVMRegistryEmpty
	}
	// Ante validation owns nonce checks; gas estimation must not reject a
	// transaction solely because its predecessor is still in the mempool.
	invocation.EthereumSimulation = true
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return 0, err
	}
	if result.GasUsed == 0 {
		return 0, ErrInvalidEVMGasEstimate
	}
	return result.GasUsed, nil
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

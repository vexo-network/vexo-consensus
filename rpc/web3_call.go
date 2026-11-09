package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	evmmodule "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/types"
)

func web3Code(ctx context.Context, provider StatusProvider, params []json.RawMessage) (string, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return "", &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	if len(params) == 0 || len(params) > 2 {
		return "", &JSONRPCError{Code: -32602, Message: "eth_getCode requires address and optional block tag"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, rpcErr := web3HistoricalAccountQueryData(ctx, provider, params, 1)
	if rpcErr != nil {
		return "", rpcErr
	}
	response, err := query.AppQuery(ctx, []string{"evm", "code", address}, data)
	if err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return "0x", nil
	}
	if response.Code != 0 {
		return "", &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Value, &payload); err != nil {
		return "", &JSONRPCError{Code: -32000, Message: "invalid EVM code response"}
	}
	return "0x" + strings.TrimPrefix(payload.Code, "0x"), nil
}

func web3StorageAt(ctx context.Context, provider StatusProvider, params []json.RawMessage) (string, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return "", &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	if len(params) < 2 || len(params) > 3 {
		return "", &JSONRPCError{Code: -32602, Message: "eth_getStorageAt requires address, slot, and optional block tag"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	slot, err := jsonRPCStringParam(params[1])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, rpcErr := web3HistoricalAccountQueryData(ctx, provider, params, 2)
	if rpcErr != nil {
		return "", rpcErr
	}
	response, err := query.AppQuery(ctx, []string{"evm", "storage", address, slot}, data)
	if err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return "0x0", nil
	}
	if response.Code != 0 {
		return "", &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var payload struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(response.Value, &payload); err != nil {
		return "", &JSONRPCError{Code: -32000, Message: "invalid EVM storage response"}
	}
	if payload.Value == "" {
		return "0x0", nil
	}
	return "0x" + strings.TrimPrefix(payload.Value, "0x"), nil
}

func web3HistoricalAccountQueryData(ctx context.Context, provider StatusProvider, params []json.RawMessage, tagIndex int) ([]byte, *JSONRPCError) {
	if len(params) <= tagIndex || len(params[tagIndex]) == 0 || string(params[tagIndex]) == "null" {
		return nil, nil
	}
	trimmed := bytes.TrimSpace(params[tagIndex])
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] != '{' {
		tag, err := jsonRPCStringParam(params[tagIndex])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		if tag == "latest" || tag == "pending" || tag == "" {
			return nil, nil
		}
	}
	height, rpcErr := web3BlockHeightParam(ctx, provider, params[tagIndex])
	if rpcErr != nil {
		return nil, rpcErr
	}
	encoded, err := json.Marshal(map[string]uint64{"height": uint64(height)})
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return encoded, nil
}

func web3EstimateGas(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (string, *JSONRPCError) {
	call, rpcErr := web3PreparedEVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return "", rpcErr
	}
	intrinsicGas, rpcErr := web3IntrinsicGasForCall(call, cfg)
	if rpcErr != nil {
		return "", rpcErr
	}
	high := call.GasLimit
	if high < intrinsicGas {
		high = intrinsicGas
	}
	if gasCap := cfg.rpcGasCap(); high > gasCap {
		return "", &JSONRPCError{Code: -32000, Message: fmt.Sprintf("gas required exceeds allowance (%d)", gasCap)}
	}
	probe := call
	probe.GasLimit = high
	callResponse, rpcErr := web3EVMCallRequest(ctx, provider, probe)
	if rpcErr != nil {
		return "", rpcErr
	}
	if callResponse.Failed {
		return "", web3CallFailureError(callResponse)
	}
	low := intrinsicGas
	if callResponse.GasUsed > low {
		low = callResponse.GasUsed
	}
	if low >= high {
		return hexQuantity(high), nil
	}
	left, right := low, high
	for left < right {
		// A cancelled request must stop the search: every probe after the
		// cancellation fails, and a search that reads failures as "too little
		// gas" would converge on, and report, the upper bound.
		if err := ctx.Err(); err != nil {
			return "", &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		mid := left + (right-left)/2
		probe.GasLimit = mid
		response, rpcErr := web3EVMCallRequest(ctx, provider, probe)
		if rpcErr != nil || response.Failed {
			left = mid + 1
			continue
		}
		right = mid
	}
	if err := ctx.Err(); err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return hexQuantity(left), nil
}

func web3IntrinsicGasForCall(call web3CallRequest, cfg Config) (uint64, *JSONRPCError) {
	input, err := hexBytes(call.Input)
	if err != nil {
		return 0, &JSONRPCError{Code: -32602, Message: "invalid data hex"}
	}
	gas, err := ethcompat.IntrinsicGasWithChainConfigJSON(input, call.AccessList, call.Method == "deploy", call.BlockOverride.Timestamp, cfg.EVMChainConfigJSON)
	if err != nil {
		return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return gas, nil
}

func saturatingAddUint64(left uint64, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}

func web3EVMCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (web3EVMCallResponse, *JSONRPCError) {
	call, rpcErr := web3PreparedEVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return web3EVMCallResponse{}, rpcErr
	}
	return web3EVMCallRequest(ctx, provider, call)
}

// web3TracedEVMCall runs a call that records an opcode trace. Tracing is the
// one thing that makes an execution expensive beyond its gas, so only the
// debug and trace methods ask for it.
func web3TracedEVMCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (web3EVMCallResponse, *JSONRPCError) {
	call, rpcErr := web3PreparedEVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return web3EVMCallResponse{}, rpcErr
	}
	call.Trace = true
	return web3EVMCallRequest(ctx, provider, call)
}

func web3PreparedEVMCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (web3CallRequest, *JSONRPCError) {
	call, rpcErr := evmCallParam(params)
	if rpcErr != nil {
		return web3CallRequest{}, rpcErr
	}
	if height, rpcErr := web3CallHeight(ctx, provider, params); rpcErr != nil {
		return web3CallRequest{}, rpcErr
	} else {
		call.Height = uint64(height)
	}
	baseFee := web3BaseFeeAtHeight(ctx, provider, types.Height(call.Height))
	call.BaseFee = baseFee
	call.BlobBaseFee = web3BlobBaseFeeAtHeight(ctx, provider, types.Height(call.Height))
	if call.GasPrice == 0 && call.GasPriceHex == "" && (call.MaxFeePerGas > 0 || call.MaxFeePerGasHex != "") {
		gasPrice := web3EffectiveCallGasPriceBig(baseFee, web3CallQuantityBig(call.MaxFeePerGasHex, call.MaxFeePerGas), web3CallQuantityBig(call.MaxPriorityFeePerGasHex, call.MaxPriorityFeePerGas))
		call.GasPriceHex = hexQuantityBig(gasPrice)
		if gasPrice.IsUint64() {
			call.GasPrice = gasPrice.Uint64()
		}
	}
	if overrides, rpcErr := web3StateOverridesParam(params); rpcErr != nil {
		return web3CallRequest{}, rpcErr
	} else {
		call.StateOverrides = overrides
	}
	if override, rpcErr := web3BlockOverrideParam(params); rpcErr != nil {
		return web3CallRequest{}, rpcErr
	} else {
		call.BlockOverride = override
		if override.BaseFee > 0 {
			call.BaseFee = override.BaseFee
		}
		if override.BlobBaseFee > 0 {
			call.BlobBaseFee = override.BlobBaseFee
		}
	}
	call.GasLimit = cfg.callGasLimit(call.GasLimit)
	return call, nil
}

// callGasLimit is the gas a read-only execution runs with: the caller's
// request, or the default block gas limit when it names none, held to the
// RPC gas cap. Every call, estimate and trace funnels through it, so no
// request can make the node run the interpreter longer than the cap allows.
func (cfg Config) callGasLimit(requested uint64) uint64 {
	if requested == 0 {
		requested = defaultWeb3BlockGasLimit
	}
	if gasCap := cfg.rpcGasCap(); requested > gasCap {
		return gasCap
	}
	return requested
}

func (cfg Config) rpcGasCap() uint64 {
	if cfg.RPCGasCap == 0 {
		return evmmodule.DefaultPolicy().RPCGasCap
	}
	return cfg.RPCGasCap
}

func web3EffectiveCallGasPrice(baseFee uint64, maxFeePerGas uint64, maxPriorityFeePerGas uint64) uint64 {
	if maxFeePerGas == 0 {
		return baseFee
	}
	capWithTip := saturatingAddUint64(baseFee, maxPriorityFeePerGas)
	if capWithTip > maxFeePerGas {
		return maxFeePerGas
	}
	return capWithTip
}

func web3EffectiveCallGasPriceBig(baseFee uint64, maxFeePerGas *big.Int, maxPriorityFeePerGas *big.Int) *big.Int {
	if maxFeePerGas == nil || maxFeePerGas.Sign() == 0 {
		return new(big.Int).SetUint64(baseFee)
	}
	capWithTip := new(big.Int).SetUint64(baseFee)
	if maxPriorityFeePerGas != nil {
		capWithTip.Add(capWithTip, maxPriorityFeePerGas)
	}
	if capWithTip.Cmp(maxFeePerGas) > 0 {
		return new(big.Int).Set(maxFeePerGas)
	}
	return capWithTip
}

func web3CallQuantityBig(hexValue string, fallback uint64) *big.Int {
	if hexValue != "" {
		parsed, ok := new(big.Int).SetString(strings.TrimPrefix(hexValue, "0x"), 16)
		if ok && parsed.Sign() >= 0 {
			return parsed
		}
	}
	return new(big.Int).SetUint64(fallback)
}

func web3EVMCallRequest(ctx context.Context, provider StatusProvider, call web3CallRequest) (web3EVMCallResponse, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	encoded, _ := json.Marshal(call)
	response, err := query.AppQuery(ctx, []string{"evm", "call"}, encoded)
	if err != nil {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code != 0 {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var callResponse web3EVMCallResponse
	if err := json.Unmarshal(response.Value, &callResponse); err != nil {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: "invalid EVM call response"}
	}
	return callResponse, nil
}

func web3CallFailureError(callResponse web3EVMCallResponse) *JSONRPCError {
	if callResponse.Error == "" {
		callResponse.Error = "execution reverted"
	}
	return &JSONRPCError{Code: -32000, Message: callResponse.Error}
}

func web3CallHeight(ctx context.Context, provider StatusProvider, params []json.RawMessage) (types.Height, *JSONRPCError) {
	for _, index := range []int{1, 2} {
		if len(params) <= index || len(params[index]) == 0 || string(params[index]) == "null" {
			continue
		}
		trimmed := bytes.TrimSpace(params[index])
		if len(trimmed) == 0 {
			continue
		}
		if index == 2 && trimmed[0] == '{' && !web3LooksLikeBlockSelector(trimmed) {
			continue
		}
		if trimmed[0] != '{' {
			var tag string
			if err := json.Unmarshal(params[index], &tag); err != nil || tag == "" {
				continue
			}
		}
		return web3BlockHeightParam(ctx, provider, params[index])
	}
	return provider.Status(ctx).LatestHeight, nil
}

func web3LooksLikeBlockSelector(raw json.RawMessage) bool {
	var selector map[string]json.RawMessage
	if err := json.Unmarshal(raw, &selector); err != nil {
		return false
	}
	_, hasNumber := selector["blockNumber"]
	_, hasHash := selector["blockHash"]
	_, hasCanonical := selector["requireCanonical"]
	return hasNumber || hasHash || hasCanonical
}

func web3StateOverridesParam(params []json.RawMessage) (map[string]evmmodule.CallStateOverride, *JSONRPCError) {
	if len(params) <= 2 || len(bytes.TrimSpace(params[2])) == 0 || string(bytes.TrimSpace(params[2])) == "null" {
		return nil, nil
	}
	raw := bytes.TrimSpace(params[2])
	if raw[0] != '{' || web3LooksLikeBlockSelector(raw) {
		return nil, nil
	}
	var payload map[string]web3StateOverrideAccount
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid state override"}
	}
	overrides := make(map[string]evmmodule.CallStateOverride, len(payload))
	for address, account := range payload {
		if _, err := parseHexAddress(address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid state override address"}
		}
		if rpcErr := validateWeb3StateOverrideAccount(account); rpcErr != nil {
			return nil, rpcErr
		}
		override := evmmodule.CallStateOverride{
			Balance:   account.Balance,
			Code:      account.Code,
			State:     normalizeOverrideStorage(account.State),
			StateDiff: normalizeOverrideStorage(account.StateDiff),
		}
		if account.Nonce != "" {
			nonce, err := parseHexQuantity(account.Nonce)
			if err != nil {
				return nil, &JSONRPCError{Code: -32602, Message: "invalid state override nonce"}
			}
			override.Nonce = &nonce
		}
		overrides[address] = override
	}
	return overrides, nil
}

func validateWeb3StateOverrideAccount(account web3StateOverrideAccount) *JSONRPCError {
	if account.Balance != "" {
		if _, err := parseOverrideBalance(account.Balance); err != nil {
			return &JSONRPCError{Code: -32602, Message: "invalid state override balance"}
		}
	}
	if account.Nonce != "" {
		if _, err := parseHexQuantity(account.Nonce); err != nil {
			return &JSONRPCError{Code: -32602, Message: "invalid state override nonce"}
		}
	}
	if account.Code != "" {
		if err := validateHexData(account.Code, "state override code"); err != nil {
			return &JSONRPCError{Code: -32602, Message: err.Error()}
		}
	}
	if len(account.State) > 0 && len(account.StateDiff) > 0 {
		return &JSONRPCError{Code: -32602, Message: "state override cannot include both state and stateDiff"}
	}
	if err := validateOverrideStorage(account.State, "state override state"); err != nil {
		return &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if err := validateOverrideStorage(account.StateDiff, "state override stateDiff"); err != nil {
		return &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	return nil
}

func parseOverrideBalance(value string) (*big.Int, error) {
	if strings.HasPrefix(value, "0x") {
		parsed, err := parseHexQuantityBig(value)
		if err != nil {
			return nil, err
		}
		if parsed.BitLen() > 256 {
			return nil, fmt.Errorf("balance exceeds uint256")
		}
		return parsed, nil
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() < 0 || parsed.BitLen() > 256 {
		return nil, fmt.Errorf("invalid decimal balance")
	}
	return parsed, nil
}

func validateHexData(value string, name string) error {
	if !strings.HasPrefix(value, "0x") {
		return fmt.Errorf("%s must use 0x hex data", name)
	}
	_, err := hexBytes(value)
	if err != nil {
		return fmt.Errorf("invalid %s", name)
	}
	return nil
}

func validateOverrideStorage(values map[string]string, name string) error {
	for slot, value := range values {
		slotBytes, err := parseFixedWidthHex(slot, 32)
		if err != nil {
			return fmt.Errorf("invalid %s slot", name)
		}
		if len(slotBytes) > 32 {
			return fmt.Errorf("invalid %s slot", name)
		}
		valueBytes, err := parseFixedWidthHex(value, 32)
		if err != nil {
			return fmt.Errorf("invalid %s value", name)
		}
		if len(valueBytes) > 32 {
			return fmt.Errorf("invalid %s value", name)
		}
	}
	return nil
}

func parseFixedWidthHex(value string, maxBytes int) ([]byte, error) {
	if !strings.HasPrefix(value, "0x") {
		return nil, fmt.Errorf("hex value must use 0x prefix")
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return []byte{}, nil
	}
	if len(trimmed) > maxBytes*2 {
		return nil, fmt.Errorf("hex value too wide")
	}
	return hexBytes(value)
}

func normalizeOverrideStorage(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(values))
	for slot, value := range values {
		normalized[slot] = value
	}
	return normalized
}

func web3BlockOverrideParam(params []json.RawMessage) (evmmodule.CallBlockOverride, *JSONRPCError) {
	if len(params) <= 3 || len(bytes.TrimSpace(params[3])) == 0 || string(bytes.TrimSpace(params[3])) == "null" {
		return evmmodule.CallBlockOverride{}, nil
	}
	var payload web3BlockOverride
	if err := json.Unmarshal(params[3], &payload); err != nil {
		return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override"}
	}
	var override evmmodule.CallBlockOverride
	var err error
	if payload.Number != "" {
		override.Number, err = parseHexQuantity(payload.Number)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override number"}
		}
	}
	timestamp := payload.Timestamp
	if timestamp == "" {
		timestamp = payload.Time
	}
	if timestamp != "" {
		override.Timestamp, err = parseHexQuantity(timestamp)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override timestamp"}
		}
	}
	if payload.GasLimit != "" {
		override.GasLimit, err = parseHexQuantity(payload.GasLimit)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override gasLimit"}
		}
	}
	if payload.BaseFee != "" {
		override.BaseFee, err = parseHexQuantity(payload.BaseFee)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override baseFeePerGas"}
		}
	}
	if payload.BlobBaseFee != "" {
		override.BlobBaseFee, err = parseHexQuantity(payload.BlobBaseFee)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override blobBaseFee"}
		}
	}
	return override, nil
}

func web3LatestBaseFee(ctx context.Context, provider StatusProvider) uint64 {
	query, ok := provider.(ChainQueryProvider)
	if !ok {
		return 1
	}
	state, err := query.LatestState(ctx)
	if err != nil {
		return 1
	}
	if state.NextBaseFee > 0 {
		return state.NextBaseFee
	}
	if state.BaseFee > 0 {
		return state.BaseFee
	}
	return 1
}

func web3CreateAccessList(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	callResponse, rpcErr := web3EVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"accessList": web3AccessList(callResponse.AccessList),
		"gasUsed":    hexQuantity(callResponse.GasUsed),
	}, nil
}

func web3DebugTraceCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceCall requires call object, optional block tag, and optional config"}
	}
	tracer, rpcErr := web3DebugCallTracer(params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	executionParams := web3DebugTraceExecutionParams(params)
	callResponse, rpcErr := web3TracedEVMCall(ctx, provider, cfg, executionParams)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if tracer == "callTracer" {
		call, _ := evmCallParam(params)
		trace := map[string]any{
			"type":       "CALL",
			"from":       call.From,
			"to":         call.To,
			"value":      web3CallValueHex(call),
			"gas":        hexQuantity(call.GasLimit),
			"gasUsed":    hexQuantity(callResponse.GasUsed),
			"input":      call.Input,
			"output":     callResponse.Output,
			"accessList": web3AccessList(callResponse.AccessList),
		}
		if callResponse.Failed {
			trace["error"] = callResponse.Error
		}
		if children := web3CallTraceChildren(callResponse.VMTrace); len(children) > 0 {
			trace["calls"] = children
		}
		return trace, nil
	}
	if tracer == "prestateTracer" {
		call, _ := evmCallParam(params)
		return web3CallPrestateTrace(call), nil
	}
	if tracer == "4byteTracer" {
		call, _ := evmCallParam(params)
		return web3SelectorTrace(call.Input), nil
	}
	return map[string]any{
		"gas":         callResponse.GasUsed,
		"failed":      callResponse.Failed,
		"returnValue": strings.TrimPrefix(callResponse.Output, "0x"),
		"structLogs":  web3StructLogs(callResponse.VMTrace),
	}, nil
}

func web3TraceCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_call requires call object, optional trace types, and optional block tag"}
	}
	call, rpcErr := evmCallParam(params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	callResponse, rpcErr := web3TracedEVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	trace := map[string]any{
		"action": map[string]any{
			"callType": "call",
			"from":     call.From,
			"to":       call.To,
			"gas":      hexQuantity(call.GasLimit),
			"input":    call.Input,
			"value":    web3CallValueHex(call),
		},
		"result": map[string]any{
			"gasUsed": hexQuantity(callResponse.GasUsed),
			"output":  callResponse.Output,
		},
		"subtraces":    len(web3CallTraceChildren(callResponse.VMTrace)),
		"traceAddress": []any{},
		"type":         "call",
	}
	if callResponse.Failed {
		trace["error"] = callResponse.Error
	}
	traces := []any{trace}
	traces = append(traces, web3ParityTraceChildren(callResponse.VMTrace, []uint64{})...)
	return map[string]any{
		"output":     callResponse.Output,
		"stateDiff":  web3StateDiff(callResponse.StateDiff),
		"trace":      traces,
		"vmTrace":    callResponse.VMTrace,
		"accessList": web3AccessList(callResponse.AccessList),
	}, nil
}

func web3StructLogs(vmTrace any) any {
	trace, ok := vmTrace.(map[string]any)
	if !ok {
		return []any{}
	}
	if logs, found := trace["structLogs"]; found {
		return logs
	}
	return []any{}
}

func web3DebugCallTracer(params []json.RawMessage) (string, *JSONRPCError) {
	if len(params) >= 3 {
		return web3DebugTracer(params, 2)
	}
	if len(params) == 2 && bytes.Contains(params[1], []byte(`"tracer"`)) {
		return web3DebugTracer(params, 1)
	}
	return "", nil
}

func web3DebugTraceExecutionParams(params []json.RawMessage) []json.RawMessage {
	if len(params) >= 3 {
		return params[:2]
	}
	if len(params) == 2 && bytes.Contains(params[1], []byte(`"tracer"`)) {
		return params[:1]
	}
	return params
}

func web3DebugTracer(params []json.RawMessage, index int) (string, *JSONRPCError) {
	if len(params) <= index || len(bytes.TrimSpace(params[index])) == 0 || string(bytes.TrimSpace(params[index])) == "null" {
		return "", nil
	}
	var config struct {
		Tracer string `json:"tracer"`
	}
	if err := json.Unmarshal(params[index], &config); err != nil {
		return "", &JSONRPCError{Code: -32602, Message: "debug trace config must be an object"}
	}
	switch config.Tracer {
	case "", "callTracer", "structLogger", "prestateTracer", "4byteTracer":
		return config.Tracer, nil
	default:
		return "structLogger", nil
	}
}

func web3StateDiff(stateDiff any) any {
	if stateDiff == nil {
		return map[string]any{}
	}
	return stateDiff
}

func web3AccessList(entries []web3AccessListEntry) []any {
	out := make([]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, map[string]any{
			"address":     entry.Address,
			"storageKeys": append([]string(nil), entry.StorageKeys...),
		})
	}
	return out
}

func lastParam(params []json.RawMessage) json.RawMessage {
	if len(params) == 0 {
		return nil
	}
	return params[len(params)-1]
}

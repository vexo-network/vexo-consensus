package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	evmmodule "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func web3DebugTraceTransaction(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceTransaction requires transaction hash and optional config"}
	}
	tracer, rpcErr := web3DebugTracer(params, 1)
	if rpcErr != nil {
		return nil, rpcErr
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !found {
		return nil, nil
	}
	receipt, ok := web3ReceiptFromResult(types.Result{Data: value})
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt"}
	}
	if tracer == "callTracer" {
		return web3ReceiptCallTrace(ctx, provider, receipt), nil
	}
	if tracer == "prestateTracer" {
		return web3ReceiptPrestateTrace(ctx, provider, receipt), nil
	}
	if tracer == "4byteTracer" {
		return web3Receipt4ByteTrace(ctx, provider, receipt), nil
	}
	return web3ReceiptDebugTraceResult(receipt), nil
}

func web3DebugTraceBlockByNumber(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceBlockByNumber requires block tag and optional config"}
	}
	record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3DebugTraceBlockRecord(record), nil
}

func web3DebugTraceBlockByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceBlockByHash requires block hash and optional config"}
	}
	record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3DebugTraceBlockRecord(record), nil
}

func web3DebugTraceBlockRecord(record store.BlockRecord) any {
	if record.Block.Header.Height == 0 {
		return nil
	}
	traces := make([]any, 0, len(record.TxResults))
	for index, result := range record.TxResults {
		receipt, ok := web3ReceiptFromResult(result)
		if !ok {
			continue
		}
		item := map[string]any{
			"txHash":  receipt.TxHash,
			"result":  web3ReceiptDebugTraceResult(receipt),
			"vmTrace": nil,
		}
		if index < len(record.Block.Txs) {
			item["transaction"] = web3TransactionFromBlockRecord(record, index, receipt.TxHash, record.Block.Txs[index])
		}
		traces = append(traces, item)
	}
	return traces
}

func web3TraceTransaction(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_transaction requires one transaction hash"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !found {
		return []any{}, nil
	}
	receipt, ok := web3ReceiptFromResult(types.Result{Data: value})
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt"}
	}
	traces := web3TraceListFromReceipt(ctx, provider, receipt)
	if len(traces) == 0 {
		return []any{}, nil
	}
	return traces, nil
}

func web3TraceGet(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_get requires transaction hash and trace address"}
	}
	traces, rpcErr := web3TraceTransaction(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	items, ok := traces.([]any)
	if !ok || len(items) == 0 {
		return nil, nil
	}
	var traceAddress []uint64
	if err := json.Unmarshal(params[1], &traceAddress); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid trace address"}
	}
	if len(traceAddress) == 0 {
		return items[0], nil
	}
	for _, item := range items {
		trace, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if web3TraceAddressEqual(trace["traceAddress"], traceAddress) {
			return trace, nil
		}
	}
	return nil, nil
}

func web3TraceBlock(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_block requires one block hash or tag"}
	}
	record, rpcErr := web3BlockRecordParam(ctx, provider, params[0])
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3TraceBlockRecord(ctx, provider, record), nil
}

func web3TraceFilter(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_filter requires one filter object"}
	}
	var filter struct {
		FromBlock   json.RawMessage `json:"fromBlock"`
		ToBlock     json.RawMessage `json:"toBlock"`
		FromAddress any             `json:"fromAddress"`
		ToAddress   any             `json:"toAddress"`
		After       uint64          `json:"after"`
		Count       uint64          `json:"count"`
	}
	if err := json.Unmarshal(params[0], &filter); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid trace filter"}
	}
	fromAddresses, rpcErr := web3LogAddresses(filter.FromAddress)
	if rpcErr != nil {
		return nil, rpcErr
	}
	toAddresses, rpcErr := web3LogAddresses(filter.ToAddress)
	if rpcErr != nil {
		return nil, rpcErr
	}
	from := provider.Status(ctx).LatestHeight
	to := from
	if len(filter.FromBlock) > 0 {
		height, rpcErr := web3BlockHeightParam(ctx, provider, filter.FromBlock)
		if rpcErr != nil {
			return nil, rpcErr
		}
		from = height
	}
	if len(filter.ToBlock) > 0 {
		height, rpcErr := web3BlockHeightParam(ctx, provider, filter.ToBlock)
		if rpcErr != nil {
			return nil, rpcErr
		}
		to = height
	}
	if to < from {
		return []any{}, nil
	}
	if uint64(to-from) > 127 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_filter block range exceeds 128 blocks"}
	}
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	traces := make([]any, 0)
	for height := from; height <= to; height++ {
		record, err := blockProvider.BlockByHeight(ctx, height)
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		items := web3TraceBlockRecord(ctx, provider, record)
		for _, item := range items {
			if !web3TraceMatchesFilter(item, fromAddresses, toAddresses) {
				continue
			}
			traces = append(traces, item)
		}
	}
	return web3TracePage(traces, filter.After, filter.Count), nil
}

func web3TraceMatchesFilter(raw any, fromAddresses []string, toAddresses []string) bool {
	trace, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	action, _ := trace["action"].(map[string]any)
	if len(fromAddresses) > 0 && !web3TraceActionAddressMatches(action["from"], fromAddresses) {
		return false
	}
	if len(toAddresses) > 0 && !web3TraceActionAddressMatches(action["to"], toAddresses) {
		return false
	}
	return true
}

func web3TraceActionAddressMatches(raw any, addresses []string) bool {
	address, ok := raw.(string)
	if !ok || address == "" {
		return false
	}
	for _, candidate := range addresses {
		if strings.EqualFold(address, candidate) {
			return true
		}
	}
	return false
}

func web3TracePage(traces []any, after uint64, count uint64) []any {
	if after >= uint64(len(traces)) {
		return []any{}
	}
	start := int(after)
	end := len(traces)
	if count > 0 && count < uint64(end-start) {
		end = start + int(count)
	}
	return traces[start:end]
}

func web3TraceReplayTransaction(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_replayTransaction requires transaction hash and optional trace types"}
	}
	replayTypes, rpcErr := web3ReplayTypes(params, 1)
	if rpcErr != nil {
		return nil, rpcErr
	}
	traces, rpcErr := web3TraceTransaction(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !found {
		return web3ReplayResponse("0x", traces, replayTypes, nil, nil), nil
	}
	receipt, ok := web3ReceiptFromResult(types.Result{Data: value})
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt"}
	}
	return web3ReplayResponse(receipt.Output, traces, replayTypes, receipt.StateDiff, nil), nil
}

func web3TraceReplayBlockTransactions(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_replayBlockTransactions requires block hash/tag and optional trace types"}
	}
	replayTypes, rpcErr := web3ReplayTypes(params, 1)
	if rpcErr != nil {
		return nil, rpcErr
	}
	record, rpcErr := web3BlockRecordParam(ctx, provider, params[0])
	if rpcErr != nil {
		return nil, rpcErr
	}
	if record.Block.Header.Height == 0 {
		return []any{}, nil
	}
	responses := make([]any, 0, len(record.TxResults))
	for _, result := range record.TxResults {
		receipt, ok := web3ReceiptFromResult(result)
		if !ok {
			continue
		}
		trace := web3TraceFromReceipt(ctx, provider, receipt)
		if trace == nil {
			continue
		}
		response := web3ReplayResponse(receipt.Output, []any{trace}, replayTypes, receipt.StateDiff, nil)
		response["transactionHash"] = receipt.TxHash
		responses = append(responses, response)
	}
	return responses, nil
}

func web3ReplayTypes(params []json.RawMessage, index int) (map[string]bool, *JSONRPCError) {
	types := map[string]bool{"trace": true, "stateDiff": true, "vmTrace": true}
	if len(params) <= index || string(params[index]) == "null" {
		return types, nil
	}
	var requested []string
	if err := json.Unmarshal(params[index], &requested); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid replay trace types"}
	}
	types = make(map[string]bool, len(requested))
	for _, item := range requested {
		if strings.TrimSpace(item) != "" {
			types[item] = true
		}
	}
	return types, nil
}

func web3ReplayResponse(output string, traces any, types map[string]bool, stateDiff any, vmTrace any) map[string]any {
	response := map[string]any{"output": output}
	if types["trace"] {
		response["trace"] = traces
	}
	if types["stateDiff"] {
		if stateDiff == nil {
			stateDiff = map[string]any{}
		}
		response["stateDiff"] = stateDiff
	}
	if types["vmTrace"] {
		response["vmTrace"] = vmTrace
	}
	for item := range types {
		switch item {
		case "trace", "stateDiff", "vmTrace":
		default:
			response[item] = nil
		}
	}
	return response
}

func web3ReceiptDebugTraceResult(receipt web3Receipt) map[string]any {
	return map[string]any{
		"gas":         receipt.GasUsed,
		"failed":      receipt.Status == 0,
		"returnValue": strings.TrimPrefix(receipt.Output, "0x"),
		"structLogs":  []any{},
	}
}

func web3ReceiptCallTrace(ctx context.Context, provider StatusProvider, receipt web3Receipt) map[string]any {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	details := web3TxDetails{}
	if found {
		details = web3TransactionDetails(tx)
	}
	to := receipt.To
	traceType := "CALL"
	if receipt.ContractAddress != "" && receipt.To == "" {
		to = receipt.ContractAddress
		traceType = "CREATE"
	}
	result := map[string]any{
		"type":    traceType,
		"from":    receipt.From,
		"to":      to,
		"value":   web3TxValueHex(details),
		"gas":     hexQuantity(details.Gas),
		"gasUsed": hexQuantity(receipt.GasUsed),
		"input":   details.Input,
		"output":  receipt.Output,
	}
	if !found {
		result["value"] = "0x0"
		result["gas"] = hexQuantity(receipt.GasUsed)
		result["input"] = "0x"
	}
	if receipt.Status == 0 {
		result["error"] = web3ReceiptError(receipt)
	}
	return result
}

func web3ReceiptPrestateTrace(ctx context.Context, provider StatusProvider, receipt web3Receipt) map[string]any {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	details := web3TxDetails{Input: "0x"}
	if found {
		details = web3TransactionDetails(tx)
	}
	out := map[string]any{}
	if receipt.From != "" {
		out[strings.ToLower(receipt.From)] = map[string]any{
			"balance": "0x0",
			"nonce":   hexQuantity(details.Nonce),
			"code":    "0x",
			"storage": map[string]any{},
		}
	}
	to := receipt.To
	if to == "" {
		to = receipt.ContractAddress
	}
	if to != "" {
		out[strings.ToLower(to)] = map[string]any{
			"balance": "0x0",
			"nonce":   "0x0",
			"code":    "0x",
			"storage": map[string]any{},
		}
	}
	return out
}

func web3Receipt4ByteTrace(ctx context.Context, provider StatusProvider, receipt web3Receipt) map[string]any {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return map[string]any{}
	}
	return web3SelectorTrace(web3TransactionDetails(tx).Input)
}

func web3CallPrestateTrace(call web3CallRequest) map[string]any {
	out := map[string]any{}
	if call.From != "" {
		out[strings.ToLower(call.From)] = web3CallPrestateAccount(call.From, call.StateOverrides)
	}
	if call.To != "" {
		out[strings.ToLower(call.To)] = web3CallPrestateAccount(call.To, call.StateOverrides)
	}
	return out
}

func web3CallPrestateAccount(address string, overrides map[string]evmmodule.CallStateOverride) map[string]any {
	account := map[string]any{
		"balance": "0x0",
		"nonce":   "0x0",
		"code":    "0x",
		"storage": map[string]any{},
	}
	override, ok := web3StateOverrideForAddress(address, overrides)
	if !ok {
		return account
	}
	if override.Balance != "" {
		account["balance"] = normalizeHexQuantityString(override.Balance)
	}
	if override.Nonce != nil {
		account["nonce"] = hexQuantity(*override.Nonce)
	}
	if override.Code != "" {
		account["code"] = normalizeHexDataString(override.Code)
	}
	storage := make(map[string]any)
	for slot, value := range override.State {
		storage[normalizeStorageHex(slot)] = normalizeStorageHex(value)
	}
	for slot, value := range override.StateDiff {
		storage[normalizeStorageHex(slot)] = normalizeStorageHex(value)
	}
	account["storage"] = storage
	return account
}

func web3StateOverrideForAddress(address string, overrides map[string]evmmodule.CallStateOverride) (evmmodule.CallStateOverride, bool) {
	if len(overrides) == 0 || address == "" {
		return evmmodule.CallStateOverride{}, false
	}
	candidates := []string{address, strings.ToLower(address), strings.ToUpper(address)}
	if parsed, err := parseHexAddress(address); err == nil {
		canonical := string(parsed)
		candidates = append(candidates, canonical, strings.ToLower(canonical), strings.ToUpper(canonical))
	}
	for _, candidate := range candidates {
		if override, ok := overrides[candidate]; ok {
			return override, true
		}
	}
	return evmmodule.CallStateOverride{}, false
}

func web3SelectorTrace(input string) map[string]any {
	decoded, err := hexBytes(input)
	if err != nil || len(decoded) < 4 {
		return map[string]any{}
	}
	key := "0x" + hex.EncodeToString(decoded[:4]) + "-" + strconv.Itoa(len(decoded)-4)
	return map[string]any{key: 1}
}

func web3TraceBlockRecord(ctx context.Context, provider StatusProvider, record store.BlockRecord) []any {
	if record.Block.Header.Height == 0 {
		return []any{}
	}
	traces := make([]any, 0, len(record.TxResults))
	for _, result := range record.TxResults {
		receipt, ok := web3ReceiptFromResult(result)
		if !ok {
			continue
		}
		traces = append(traces, web3TraceListFromReceipt(ctx, provider, receipt)...)
	}
	return traces
}

func web3TraceListFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) []any {
	trace := web3TraceFromReceipt(ctx, provider, receipt)
	if trace == nil {
		return nil
	}
	return []any{trace}
}

func web3TraceFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) any {
	blockHash, txIndex, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return nil
	}
	details := web3TransactionDetails(tx)
	to := details.To
	traceType := "call"
	result := map[string]any{"gasUsed": hexQuantity(receipt.GasUsed), "output": receipt.Output}
	if receipt.ContractAddress != "" && receipt.To == "" {
		traceType = "create"
		to = nil
		result["address"] = receipt.ContractAddress
	}
	action := map[string]any{
		"from":  receipt.From,
		"to":    to,
		"gas":   hexQuantity(details.Gas),
		"input": details.Input,
		"value": web3TxValueHex(details),
	}
	if traceType == "call" {
		action["callType"] = "call"
	}
	trace := map[string]any{
		"action":              action,
		"blockHash":           blockHash,
		"blockNumber":         hexQuantity(receipt.Height),
		"result":              result,
		"subtraces":           0,
		"traceAddress":        []any{},
		"transactionHash":     receipt.TxHash,
		"transactionPosition": hexQuantity(txIndex),
		"type":                traceType,
	}
	if receipt.Status == 0 {
		trace["error"] = web3ReceiptError(receipt)
	}
	return trace
}

func web3ReceiptError(receipt web3Receipt) string {
	if receipt.Error != "" {
		return receipt.Error
	}
	return "execution reverted"
}

func web3CallTraceChildren(vmTrace any) []any {
	trace, ok := vmTrace.(map[string]any)
	if !ok {
		return nil
	}
	calls, ok := trace["calls"].([]any)
	if ok {
		return calls
	}
	calls, ok = trace["children"].([]any)
	if ok {
		return calls
	}
	return nil
}

func web3ParityTraceChildren(vmTrace any, parent []uint64) []any {
	children := web3CallTraceChildren(vmTrace)
	if len(children) == 0 {
		return nil
	}
	out := make([]any, 0, len(children))
	for index, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		address := append(append([]uint64(nil), parent...), uint64(index))
		trace := web3ParityTraceFromCall(childMap, address)
		out = append(out, trace)
		out = append(out, web3ParityTraceChildren(childMap, address)...)
	}
	return out
}

func web3ParityTraceFromCall(call map[string]any, address []uint64) map[string]any {
	callType := strings.ToLower(stringValue(call["type"]))
	if callType == "" {
		callType = strings.ToLower(stringValue(call["callType"]))
	}
	if callType == "" {
		callType = "call"
	}
	action := map[string]any{
		"callType": callType,
		"from":     stringValue(call["from"]),
		"to":       stringValue(call["to"]),
		"gas":      stringValueOrDefault(call["gas"], "0x0"),
		"input":    stringValueOrDefault(call["input"], "0x"),
		"value":    stringValueOrDefault(call["value"], "0x0"),
	}
	result := map[string]any{
		"gasUsed": stringValueOrDefault(call["gasUsed"], "0x0"),
		"output":  stringValueOrDefault(call["output"], "0x"),
	}
	trace := map[string]any{
		"action":       action,
		"result":       result,
		"subtraces":    len(web3CallTraceChildren(call)),
		"traceAddress": uint64SliceToAny(address),
		"type":         callType,
	}
	if errText := stringValue(call["error"]); errText != "" {
		trace["error"] = errText
	}
	return trace
}

func stringValue(value any) string {
	item, _ := value.(string)
	return item
}

func stringValueOrDefault(value any, fallback string) string {
	if item := stringValue(value); item != "" {
		return item
	}
	return fallback
}

func uint64SliceToAny(values []uint64) []any {
	out := make([]any, len(values))
	for index, value := range values {
		out[index] = value
	}
	return out
}

func web3TraceAddressEqual(raw any, expected []uint64) bool {
	items, ok := raw.([]any)
	if !ok || len(items) != len(expected) {
		return false
	}
	for index, item := range items {
		switch value := item.(type) {
		case uint64:
			if value != expected[index] {
				return false
			}
		case float64:
			if value != float64(expected[index]) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func web3TraceWantsCallTracer(params []json.RawMessage) bool {
	if len(params) < 2 || string(params[1]) == "null" {
		return false
	}
	var config struct {
		Tracer string `json:"tracer"`
	}
	if err := json.Unmarshal(params[1], &config); err != nil {
		return false
	}
	return config.Tracer == "callTracer"
}

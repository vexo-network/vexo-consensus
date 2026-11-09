package rpc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	gethaccounts "github.com/ethereum/go-ethereum/accounts"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/node"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"golang.org/x/crypto/sha3"
)

func handleWeb3JSONRPC(writer http.ResponseWriter, request *http.Request, provider StatusProvider, cfg Config, filters *web3FilterStore) {
	if !allowPost(writer, request) {
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, cfg.MaxRequestBytes)
	defer request.Body.Close()
	var raw json.RawMessage
	if err := decodeStrictJSON(request.Body, &raw); err != nil {
		writeJSONRPC(writer, json.RawMessage("null"), nil, &JSONRPCError{Code: -32700, Message: "parse error"})
		return
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		writeJSONRPC(writer, json.RawMessage("null"), nil, &JSONRPCError{Code: -32700, Message: "parse error"})
		return
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			writeJSONRPC(writer, json.RawMessage("null"), nil, &JSONRPCError{Code: -32600, Message: "invalid JSON-RPC batch"})
			return
		}
		responses := make([]JSONRPCResponse, 0, len(batch))
		for _, item := range batch {
			response, notify := executeWeb3Payload(request.Context(), provider, cfg, filters, item)
			if notify {
				continue
			}
			responses = append(responses, response)
		}
		if len(responses) == 0 {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(writer, http.StatusOK, responses)
		return
	}
	response, notify := executeWeb3Payload(request.Context(), provider, cfg, filters, trimmed)
	if notify {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func executeWeb3Payload(ctx context.Context, provider StatusProvider, cfg Config, filters *web3FilterStore, raw json.RawMessage) (JSONRPCResponse, bool) {
	var payload JSONRPCRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &JSONRPCError{Code: -32600, Message: "invalid request"}}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &JSONRPCError{Code: -32600, Message: "invalid request"}}, false
	}
	if payload.JSONRPC != "" && payload.JSONRPC != "2.0" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Error: &JSONRPCError{Code: -32600, Message: "invalid JSON-RPC version"}}, false
	}
	if payload.Method == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Error: &JSONRPCError{Code: -32600, Message: "method is required"}}, false
	}
	notify := len(payload.ID) == 0
	result, rpcErr := executeWeb3Method(ctx, provider, cfg, filters, payload.Method, payload.Params)
	if notify {
		return JSONRPCResponse{}, true
	}
	if len(payload.ID) == 0 {
		payload.ID = json.RawMessage("null")
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Result: result, Error: rpcErr}, false
}

func writeFinalityProof(writer http.ResponseWriter, proof finality.Proof, err error, requireStrict bool) {
	if errors.Is(err, node.ErrFinalityNotFound) ||
		errors.Is(err, store.ErrBlockNotFound) ||
		errors.Is(err, store.ErrBlockIndexNotFound) {
		writeError(writer, http.StatusNotFound, "finality proof not found")
		return
	}
	if errors.Is(err, vexoruntime.ErrFinalityProofHeightMismatch) ||
		errors.Is(err, vexoruntime.ErrFinalityProofBlockMismatch) {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if requireStrict && !proof.HasCommitProof() {
		writeError(writer, http.StatusNotFound, "strict finality proof not found")
		return
	}
	writeJSON(writer, http.StatusOK, finalityProofResponse(proof))
}

func strictFinalityRequested(request *http.Request) bool {
	value := request.URL.Query().Get("strict")
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func executeWeb3Method(ctx context.Context, provider StatusProvider, cfg Config, filters *web3FilterStore, method string, params []json.RawMessage) (any, *JSONRPCError) {
	switch method {
	case "rpc_modules":
		return web3RPCModules(), nil
	case "vexo_web3Capabilities":
		return web3Capabilities(), nil
	case "web3_clientVersion":
		return "vexo-consensus/web3", nil
	case "web3_sha3":
		return web3Sha3(params)
	case "net_version":
		return strconv.FormatUint(web3ChainID(provider.Status(ctx)), 10), nil
	case "net_listening":
		return provider.Status(ctx).Running, nil
	case "net_peerCount":
		return hexQuantity(uint64(provider.Status(ctx).PeerCount)), nil
	case "eth_chainId":
		return hexQuantity(web3ChainID(provider.Status(ctx))), nil
	case "eth_protocolVersion":
		return "0x1", nil
	case "eth_syncing":
		return web3Syncing(provider.Status(ctx)), nil
	case "eth_mining":
		return web3Mining(provider, ctx), nil
	case "eth_hashrate":
		return hexQuantity(0), nil
	case "eth_accounts":
		return web3ConfiguredAccounts(cfg), nil
	case "eth_coinbase":
		accounts := web3ConfiguredAccounts(cfg)
		if len(accounts) == 0 {
			return "0x0000000000000000000000000000000000000000", nil
		}
		return accounts[0], nil
	case "eth_sign":
		return web3EthSign(cfg, params, false)
	case "personal_sign":
		return web3EthSign(cfg, params, true)
	case "eth_signTransaction":
		return web3SignTransaction(ctx, provider, cfg, params)
	case "eth_sendTransaction":
		return web3SendTransaction(ctx, provider, cfg, params)
	case "eth_blockNumber":
		return hexQuantity(uint64(provider.Status(ctx).LatestHeight)), nil
	case "eth_getBlockByNumber":
		record, rpcErr := web3BlockByNumber(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		fullTx := web3FullTransactionParam(params, 1)
		return web3BlockFromRecord(ctx, provider, cfg, record, fullTx)
	case "eth_getBlockTransactionCountByNumber":
		record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{firstParam(params, "null")})
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3BlockTransactionCount(record), nil
	case "eth_getBlockByHash":
		record, rpcErr := web3BlockByHash(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		fullTx := web3FullTransactionParam(params, 1)
		return web3BlockFromRecord(ctx, provider, cfg, record, fullTx)
	case "eth_getBlockTransactionCountByHash":
		record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{firstParam(params, "null")})
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3BlockTransactionCount(record), nil
	case "eth_getTransactionByBlockNumberAndIndex":
		return web3TransactionByBlockNumberAndIndex(ctx, provider, params)
	case "eth_getTransactionByBlockHashAndIndex":
		return web3TransactionByBlockHashAndIndex(ctx, provider, params)
	case "eth_getUncleCountByBlockNumber", "eth_getUncleCountByBlockHash":
		return hexQuantity(0), nil
	case "eth_getUncleByBlockNumberAndIndex", "eth_getUncleByBlockHashAndIndex":
		return nil, nil
	case "eth_gasPrice":
		return hexQuantity(web3LatestBaseFee(ctx, provider)), nil
	case "eth_blobBaseFee":
		return hexQuantity(web3LatestBlobBaseFee(ctx, provider)), nil
	case "eth_maxPriorityFeePerGas":
		return web3MaxPriorityFeePerGas(ctx, provider), nil
	case "eth_feeHistory":
		return web3FeeHistory(ctx, provider, params)
	case "eth_getBalance":
		if len(params) == 0 || len(params) > 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getBalance requires address and optional block tag"}
		}
		address, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		account, rpcErr := web3AccountState(ctx, provider, address, params, 1)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3AccountBalanceHex(account), nil
	case "eth_getTransactionCount":
		if len(params) == 0 || len(params) > 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionCount requires address and optional block tag"}
		}
		address, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		account, rpcErr := web3AccountState(ctx, provider, address, params, 1)
		if rpcErr != nil {
			return nil, rpcErr
		}
		sequence := account.Nonce
		if len(params) == 2 {
			tag := ""
			if trimmed := bytes.TrimSpace(params[1]); len(trimmed) > 0 && trimmed[0] != '{' {
				var err error
				tag, err = jsonRPCStringParam(params[1])
				if err != nil {
					return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
				}
			}
			if tag == "pending" {
				sequence, err = web3PendingSequence(ctx, provider, address, sequence)
				if err != nil {
					return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
				}
			}
		}
		return hexQuantity(sequence), nil
	case "eth_getCode":
		code, rpcErr := web3Code(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return code, nil
	case "eth_getStorageAt":
		value, rpcErr := web3StorageAt(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return value, nil
	case "eth_getProof":
		return web3GetProof(ctx, provider, params)
	case "eth_sendRawTransaction":
		submitter, ok := provider.(TxSubmitter)
		if !ok {
			return nil, &JSONRPCError{Code: -32000, Message: "transaction submission is unavailable"}
		}
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_sendRawTransaction requires one raw transaction parameter"}
		}
		rawTx, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		decoded, err := web3DecodeRawEthereumTx(ctx, provider, cfg, rawTx)
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		if len(decoded.BlobHashes) > 0 {
			return nil, &JSONRPCError{Code: -32602, Message: "blob transactions require eth_sendRawBlobTransaction or vexo_sendRawBlobTransaction with an explicit sidecar"}
		}
		if err := submitter.SubmitTx(ctx, decoded.Tx); err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return decoded.Hash, nil
	case "eth_sendRawBlobTransaction":
		return web3SendRawBlobTransaction(ctx, provider, cfg, params)
	case "vexo_sendRawBlobTransaction":
		return web3SendRawBlobTransaction(ctx, provider, cfg, params)
	case "vexo_getBlobSidecarByTxHash":
		return web3BlobSidecar(ctx, provider, params, "blob_sidecar")
	case "vexo_getBlobSidecarByBlobHash":
		return web3BlobSidecar(ctx, provider, params, "blob_sidecar_by_hash")
	case "eth_getTransactionReceipt":
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionReceipt requires one transaction hash"}
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
		return web3ReceiptObject(ctx, provider, value)
	case "eth_getBlockReceipts":
		return web3BlockReceipts(ctx, provider, params)
	case "eth_getTransactionByHash":
		return web3TransactionByHash(ctx, provider, params)
	case "eth_getRawTransactionByHash":
		return web3RawTransactionByHash(ctx, provider, params)
	case "eth_getRawTransactionByBlockNumberAndIndex":
		return web3RawTransactionByBlockNumberAndIndex(ctx, provider, params)
	case "eth_getRawTransactionByBlockHashAndIndex":
		return web3RawTransactionByBlockHashAndIndex(ctx, provider, params)
	case "txpool_status":
		return web3TxpoolStatus(ctx, provider)
	case "txpool_content":
		return web3TxpoolContent(ctx, provider)
	case "txpool_contentFrom":
		return web3TxpoolContentFrom(ctx, provider, params)
	case "txpool_inspect":
		return web3TxpoolInspect(ctx, provider)
	case "eth_pendingTransactions":
		return web3PendingTransactions(ctx, provider)
	case "debug_traceTransaction":
		return web3DebugTraceTransaction(ctx, provider, params)
	case "debug_traceBlockByNumber":
		return web3DebugTraceBlockByNumber(ctx, provider, params)
	case "debug_traceBlockByHash":
		return web3DebugTraceBlockByHash(ctx, provider, params)
	case "trace_transaction":
		return web3TraceTransaction(ctx, provider, params)
	case "trace_get":
		return web3TraceGet(ctx, provider, params)
	case "trace_block":
		return web3TraceBlock(ctx, provider, params)
	case "trace_filter":
		return web3TraceFilter(ctx, provider, params)
	case "trace_replayTransaction":
		return web3TraceReplayTransaction(ctx, provider, params)
	case "trace_replayBlockTransactions":
		return web3TraceReplayBlockTransactions(ctx, provider, params)
	case "eth_getLogs":
		filter, rpcErr := web3LogFilterParam(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3LogsForFilter(ctx, provider, cfg, filter)
	case "eth_newBlockFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		if len(params) != 0 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_newBlockFilter does not accept parameters"}
		}
		filterID := filters.addBlock(uint64(provider.Status(ctx).LatestHeight))
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return filterID, nil
	case "eth_newPendingTransactionFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		pendingProvider, ok := provider.(PendingTxProvider)
		if !ok {
			return nil, &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
		}
		if len(params) != 0 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_newPendingTransactionFilter does not accept parameters"}
		}
		hashes, err := pendingProvider.PendingTxHashes(ctx)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		filterID := filters.addPending(hashes)
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return filterID, nil
	case "eth_newFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		filter, rpcErr := web3LogFilterParam(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		logs, rpcErr := web3LogsForFilter(ctx, provider, cfg, filter)
		if rpcErr != nil {
			return nil, rpcErr
		}
		filterID := filters.addLog(filter, logs, uint64(provider.Status(ctx).LatestHeight))
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return filterID, nil
	case "eth_getFilterChanges", "eth_getFilterLogs":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: method + " requires filter id"}
		}
		filterID, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		filter, found := filters.get(filterID)
		if !found {
			return nil, &JSONRPCError{Code: -32000, Message: "filter not found"}
		}
		if filter.Type == "pending" {
			changes, updated, rpcErr := web3PendingFilterChanges(ctx, provider, filter, method == "eth_getFilterChanges")
			if rpcErr != nil {
				return nil, rpcErr
			}
			if method == "eth_getFilterChanges" {
				filters.replace(filterID, updated)
				if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
					return nil, rpcErr
				}
			}
			return changes, nil
		}
		changes, rpcErr := web3FilterChanges(ctx, provider, cfg, filter, method == "eth_getFilterChanges")
		if rpcErr != nil {
			return nil, rpcErr
		}
		if method == "eth_getFilterChanges" {
			if filter.Type == "log" {
				currentLogs, rpcErr := web3LogsForFilter(ctx, provider, cfg, filter)
				if rpcErr != nil {
					return nil, rpcErr
				}
				filter.SeenLogs = web3SeenLogSet(currentLogs)
				filter.LastHeight = uint64(provider.Status(ctx).LatestHeight)
				filters.replace(filterID, filter)
			} else {
				filters.mark(filterID, uint64(provider.Status(ctx).LatestHeight))
			}
			if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
				return nil, rpcErr
			}
		}
		return changes, nil
	case "eth_uninstallFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_uninstallFilter requires filter id"}
		}
		filterID, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		removed := filters.remove(filterID)
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return removed, nil
	case "eth_call":
		callResponse, rpcErr := web3EVMCall(ctx, provider, cfg, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return callResponse.Output, nil
	case "eth_estimateGas":
		return web3EstimateGas(ctx, provider, cfg, params)
	case "eth_createAccessList":
		return web3CreateAccessList(ctx, provider, cfg, params)
	case "debug_traceCall":
		return web3DebugTraceCall(ctx, provider, cfg, params)
	case "trace_call":
		return web3TraceCall(ctx, provider, cfg, params)
	case "eth_subscribe", "eth_unsubscribe":
		return nil, &JSONRPCError{Code: -32000, Message: method + " requires a WebSocket transport"}
	default:
		return nil, &JSONRPCError{Code: -32601, Message: "method not found"}
	}
}

func writeJSONRPC(writer http.ResponseWriter, id json.RawMessage, result any, rpcErr *JSONRPCError) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	writeJSON(writer, http.StatusOK, JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
}

func firstParam(params []json.RawMessage, fallback string) json.RawMessage {
	if len(params) == 0 {
		return json.RawMessage(fallback)
	}
	return params[0]
}

func web3RPCModules() map[string]string {
	return map[string]string{
		"debug":  "1.0",
		"eth":    "1.0",
		"net":    "1.0",
		"rpc":    "1.0",
		"trace":  "1.0",
		"txpool": "1.0",
		"vexo":   "1.0",
		"web3":   "1.0",
	}
}

func web3Capabilities() map[string]any {
	return map[string]any{
		"ethereum_p2p":                 false,
		"native_vexo_network":          true,
		"json_rpc_namespaces":          web3RPCModules(),
		"raw_transactions":             true,
		"blob_transactions":            true,
		"native_fee_token_accounting":  true,
		"dynamic_base_fee":             true,
		"dynamic_blob_base_fee":        true,
		"eth_call":                     true,
		"eth_estimateGas":              true,
		"eth_createAccessList":         true,
		"eth_getProof":                 true,
		"websocket_subscriptions":      true,
		"filters":                      true,
		"txpool":                       true,
		"debug_traceTransaction":       true,
		"debug_traceCall":              true,
		"trace_replayTransaction":      true,
		"trace_reexecution":            "receipt_or_call_context_backed",
		"trace_filter_max_block_range": 128,
		"unsupported_namespaces":       []string{"admin", "engine", "miner"},
		"compatibility_target":         "Ethereum JSON-RPC semantics on the Vexo native network; not Ethereum devp2p/engine API node compatibility",
		"recommended_conformance_suites": []string{
			"ethers",
			"web3.js",
			"MetaMask",
			"Hardhat",
			"Foundry",
		},
	}
}

func web3ConfiguredAccounts(cfg Config) []string {
	if !cfg.EnableEVMManagedAccounts {
		return []string{}
	}
	accounts := make([]string, 0, len(cfg.EVMAccountPrivateKeys))
	seen := make(map[string]struct{}, len(cfg.EVMAccountPrivateKeys))
	for _, rawKey := range cfg.EVMAccountPrivateKeys {
		key, err := web3PrivateKey(rawKey)
		if err != nil {
			continue
		}
		address := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
		lower := strings.ToLower(address)
		if _, found := seen[lower]; found {
			continue
		}
		seen[lower] = struct{}{}
		accounts = append(accounts, address)
	}
	return accounts
}

func web3AccountKey(cfg Config, address string) (*ecdsa.PrivateKey, *JSONRPCError) {
	if !cfg.EnableEVMManagedAccounts {
		return nil, &JSONRPCError{Code: -32000, Message: "managed accounts are disabled"}
	}
	parsed, err := parseHexAddress(address)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid account address"}
	}
	target := strings.ToLower(string(parsed))
	for _, rawKey := range cfg.EVMAccountPrivateKeys {
		key, err := web3PrivateKey(rawKey)
		if err != nil {
			continue
		}
		if strings.ToLower(gethcrypto.PubkeyToAddress(key.PublicKey).Hex()) == target {
			return key, nil
		}
	}
	return nil, &JSONRPCError{Code: -32000, Message: "account is not managed by this node"}
}

func web3PrivateKey(rawKey string) (*ecdsa.PrivateKey, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(rawKey, "0x"))
	if trimmed == "" {
		return nil, errors.New("empty private key")
	}
	return gethcrypto.HexToECDSA(trimmed)
}

func web3EthSign(cfg Config, params []json.RawMessage, personalOrder bool) (string, *JSONRPCError) {
	if len(params) != 2 {
		return "", &JSONRPCError{Code: -32602, Message: "signing requires address and data"}
	}
	addressIndex := 0
	dataIndex := 1
	if personalOrder {
		addressIndex = 1
		dataIndex = 0
	}
	address, err := jsonRPCStringParam(params[addressIndex])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	dataHex, err := jsonRPCStringParam(params[dataIndex])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, err := hexBytes(dataHex)
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: "invalid sign data hex"}
	}
	key, rpcErr := web3AccountKey(cfg, address)
	if rpcErr != nil {
		return "", rpcErr
	}
	signature, err := gethcrypto.Sign(gethaccounts.TextHash(data), key)
	if err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	signature[64] += 27
	return "0x" + hex.EncodeToString(signature), nil
}

func web3SignTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	signed, raw, rpcErr := web3SignEthereumTransaction(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"raw": "0x" + hex.EncodeToString(raw),
		"tx":  web3SignedTransactionObject(signed),
	}, nil
}

func web3SendTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	submitter, ok := provider.(TxSubmitter)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "transaction submission is unavailable"}
	}
	// A managed transaction without an explicit nonce derives it from committed
	// and pending state. Keep nonce selection, signing, and admission atomic so
	// concurrent JSON-RPC requests cannot sign the same sequence number.
	if cfg.managedTransactionMu != nil {
		cfg.managedTransactionMu.Lock()
		defer cfg.managedTransactionMu.Unlock()
	}
	signed, raw, rpcErr := web3SignEthereumTransaction(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	rawHex := "0x" + hex.EncodeToString(raw)
	decoded, err := web3DecodeRawEthereumTx(ctx, provider, cfg, rawHex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if len(decoded.BlobHashes) > 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "blob transactions require eth_sendRawBlobTransaction or vexo_sendRawBlobTransaction with an explicit sidecar"}
	}
	if err := submitter.SubmitTx(ctx, decoded.Tx); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return signed.Hash().Hex(), nil
}

func web3SignEthereumTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (*gethtypes.Transaction, []byte, *JSONRPCError) {
	if len(params) != 1 {
		return nil, nil, &JSONRPCError{Code: -32602, Message: "transaction signing requires one transaction object"}
	}
	var payload web3TransactionCall
	if err := json.Unmarshal(params[0], &payload); err != nil {
		return nil, nil, &JSONRPCError{Code: -32602, Message: "invalid transaction object"}
	}
	if payload.From == "" {
		return nil, nil, &JSONRPCError{Code: -32602, Message: "from address is required"}
	}
	key, rpcErr := web3AccountKey(cfg, payload.From)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}
	tx, rpcErr := web3UnsignedEthereumTransaction(ctx, provider, payload)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}
	chainID := new(big.Int).SetUint64(web3ChainID(provider.Status(ctx)))
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		return nil, nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return signed, raw, nil
}

func web3UnsignedEthereumTransaction(ctx context.Context, provider StatusProvider, payload web3TransactionCall) (*gethtypes.Transaction, *JSONRPCError) {
	dataHex := payload.Data
	if dataHex == "" {
		dataHex = payload.Input
	}
	if dataHex == "" {
		dataHex = "0x"
	}
	data, err := hexBytes(dataHex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid data hex"}
	}
	nonce, rpcErr := web3TransactionNonce(ctx, provider, payload)
	if rpcErr != nil {
		return nil, rpcErr
	}
	gas, rpcErr := web3TransactionGas(payload)
	if rpcErr != nil {
		return nil, rpcErr
	}
	value, rpcErr := web3TransactionBig(payload.Value, "invalid value quantity")
	if rpcErr != nil {
		return nil, rpcErr
	}
	accessList, rpcErr := web3GethAccessList(payload.AccessList)
	if rpcErr != nil {
		return nil, rpcErr
	}
	chainID := new(big.Int).SetUint64(web3ChainID(provider.Status(ctx)))
	to, rpcErr := web3OptionalToAddress(payload.To)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if len(bytes.TrimSpace(payload.AuthorizationList)) > 0 && string(bytes.TrimSpace(payload.AuthorizationList)) != "null" {
		authList, rpcErr := web3SetCodeAuthorizations(payload.AuthorizationList)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if to == nil {
			return nil, &JSONRPCError{Code: -32602, Message: "set-code transaction requires to address"}
		}
		gasFeeCap, rpcErr := web3RequiredUint256(payload.MaxFeePerGas, "maxFeePerGas is required for set-code transaction")
		if rpcErr != nil {
			return nil, rpcErr
		}
		gasTipCap, rpcErr := web3OptionalUint256(payload.MaxPriorityFeePerGas)
		if rpcErr != nil {
			return nil, rpcErr
		}
		txValue, rpcErr := web3BigToUint256(value, "value exceeds uint256")
		if rpcErr != nil {
			return nil, rpcErr
		}
		return gethtypes.NewTx(&gethtypes.SetCodeTx{
			ChainID:    uint256.MustFromBig(chainID),
			Nonce:      nonce,
			GasTipCap:  gasTipCap,
			GasFeeCap:  gasFeeCap,
			Gas:        gas,
			To:         *to,
			Value:      txValue,
			Data:       data,
			AccessList: accessList,
			AuthList:   authList,
		}), nil
	}
	if payload.MaxFeePerGas != "" || payload.MaxPriorityFeePerGas != "" {
		gasFeeCap, rpcErr := web3RequiredBig(payload.MaxFeePerGas, "maxFeePerGas is required for dynamic fee transaction")
		if rpcErr != nil {
			return nil, rpcErr
		}
		gasTipCap, rpcErr := web3TransactionBig(payload.MaxPriorityFeePerGas, "invalid maxPriorityFeePerGas quantity")
		if rpcErr != nil {
			return nil, rpcErr
		}
		return gethtypes.NewTx(&gethtypes.DynamicFeeTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasTipCap:  gasTipCap,
			GasFeeCap:  gasFeeCap,
			Gas:        gas,
			To:         to,
			Value:      value,
			Data:       data,
			AccessList: accessList,
		}), nil
	}
	gasPrice, rpcErr := web3TransactionGasPrice(ctx, provider, payload)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if len(accessList) > 0 {
		return gethtypes.NewTx(&gethtypes.AccessListTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasPrice:   gasPrice,
			Gas:        gas,
			To:         to,
			Value:      value,
			Data:       data,
			AccessList: accessList,
		}), nil
	}
	return gethtypes.NewTx(&gethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gas,
		To:       to,
		Value:    value,
		Data:     data,
	}), nil
}

func web3TransactionNonce(ctx context.Context, provider StatusProvider, payload web3TransactionCall) (uint64, *JSONRPCError) {
	if payload.Nonce != "" {
		nonce, err := parseHexQuantity(payload.Nonce)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid nonce quantity"}
		}
		return nonce, nil
	}
	account, rpcErr := web3AccountState(ctx, provider, payload.From, []json.RawMessage{json.RawMessage(strconv.Quote(payload.From)), json.RawMessage(`"pending"`)}, 1)
	if rpcErr != nil {
		return 0, rpcErr
	}
	nonce, err := web3PendingSequence(ctx, provider, payload.From, account.Nonce)
	if err != nil {
		return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return nonce, nil
}

func web3TransactionGas(payload web3TransactionCall) (uint64, *JSONRPCError) {
	if payload.Gas == "" {
		return defaultWeb3BlockGasLimit, nil
	}
	gas, err := parseHexQuantity(payload.Gas)
	if err != nil {
		return 0, &JSONRPCError{Code: -32602, Message: "invalid gas quantity"}
	}
	return gas, nil
}

func web3TransactionGasPrice(ctx context.Context, provider StatusProvider, payload web3TransactionCall) (*big.Int, *JSONRPCError) {
	if payload.GasPrice != "" {
		return web3RequiredBig(payload.GasPrice, "invalid gasPrice quantity")
	}
	if query, ok := provider.(ChainQueryProvider); ok {
		if state, err := query.LatestState(ctx); err == nil {
			price := state.NextBaseFee
			if price == 0 {
				price = state.BaseFee
			}
			if price == 0 {
				price = 1
			}
			return new(big.Int).SetUint64(price), nil
		}
	}
	return big.NewInt(1), nil
}

func web3TransactionBig(value string, message string) (*big.Int, *JSONRPCError) {
	if value == "" {
		return big.NewInt(0), nil
	}
	parsed, err := parseHexQuantityBig(value)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return parsed, nil
}

func web3RequiredBig(value string, message string) (*big.Int, *JSONRPCError) {
	if value == "" {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return web3TransactionBig(value, message)
}

func web3OptionalUint256(value string) (*uint256.Int, *JSONRPCError) {
	if value == "" {
		return uint256.NewInt(0), nil
	}
	parsed, rpcErr := web3TransactionBig(value, "invalid uint256 quantity")
	if rpcErr != nil {
		return nil, rpcErr
	}
	if parsed.BitLen() > 256 {
		return nil, &JSONRPCError{Code: -32602, Message: "quantity exceeds uint256"}
	}
	return uint256.MustFromBig(parsed), nil
}

func web3RequiredUint256(value string, message string) (*uint256.Int, *JSONRPCError) {
	if value == "" {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return web3OptionalUint256(value)
}

func web3BigToUint256(value *big.Int, message string) (*uint256.Int, *JSONRPCError) {
	if value == nil {
		return uint256.NewInt(0), nil
	}
	if value.Sign() < 0 || value.BitLen() > 256 {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return uint256.MustFromBig(value), nil
}

func web3OptionalToAddress(value string) (*gethcommon.Address, *JSONRPCError) {
	if value == "" {
		return nil, nil
	}
	parsed, err := parseHexAddress(value)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid to address"}
	}
	address := gethcommon.HexToAddress(string(parsed))
	return &address, nil
}

func web3GethAccessList(entries []web3AccessListEntry) (gethtypes.AccessList, *JSONRPCError) {
	if len(entries) == 0 {
		return nil, nil
	}
	out := make(gethtypes.AccessList, 0, len(entries))
	for _, entry := range entries {
		parsed, err := parseHexAddress(entry.Address)
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid access list address"}
		}
		item := gethtypes.AccessTuple{Address: gethcommon.HexToAddress(string(parsed))}
		for _, key := range entry.StorageKeys {
			if _, err := parseFixedWidthHex(key, 32); err != nil {
				return nil, &JSONRPCError{Code: -32602, Message: "invalid access list storage key"}
			}
			item.StorageKeys = append(item.StorageKeys, gethcommon.HexToHash(key))
		}
		out = append(out, item)
	}
	return out, nil
}

func web3SetCodeAuthorizations(raw json.RawMessage) ([]gethtypes.SetCodeAuthorization, *JSONRPCError) {
	var authList []gethtypes.SetCodeAuthorization
	if err := json.Unmarshal(raw, &authList); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid authorizationList"}
	}
	return authList, nil
}

func web3SignedTransactionObject(tx *gethtypes.Transaction) map[string]any {
	out := map[string]any{
		"hash":  tx.Hash().Hex(),
		"type":  hexQuantity(uint64(tx.Type())),
		"nonce": hexQuantity(tx.Nonce()),
		"gas":   hexQuantity(tx.Gas()),
		"input": "0x" + hex.EncodeToString(tx.Data()),
		"value": hexQuantityBig(tx.Value()),
	}
	if to := tx.To(); to != nil {
		out["to"] = to.Hex()
	} else {
		out["to"] = nil
	}
	if chainID := tx.ChainId(); chainID != nil {
		out["chainId"] = hexQuantityBig(chainID)
	}
	if gasPrice := tx.GasPrice(); gasPrice != nil {
		out["gasPrice"] = hexQuantityBig(gasPrice)
	}
	if tx.GasFeeCap() != nil {
		out["maxFeePerGas"] = hexQuantityBig(tx.GasFeeCap())
	}
	if tx.GasTipCap() != nil {
		out["maxPriorityFeePerGas"] = hexQuantityBig(tx.GasTipCap())
	}
	if accessList := tx.AccessList(); len(accessList) > 0 {
		items := make([]any, 0, len(accessList))
		for _, entry := range accessList {
			keys := make([]string, 0, len(entry.StorageKeys))
			for _, key := range entry.StorageKeys {
				keys = append(keys, key.Hex())
			}
			items = append(items, map[string]any{"address": entry.Address.Hex(), "storageKeys": keys})
		}
		out["accessList"] = items
	}
	if authList := tx.SetCodeAuthorizations(); len(authList) > 0 {
		out["authorizationList"] = authList
	}
	return out
}

type web3BlobSidecarParam struct {
	BlobHashes      []string `json:"blobHashes,omitempty"`
	BlobHashesSnake []string `json:"blob_hashes,omitempty"`
	Blobs           []string `json:"blobs"`
	Commitments     []string `json:"commitments"`
	Proofs          []string `json:"proofs"`
}

func web3DecodeRawEthereumTx(ctx context.Context, provider StatusProvider, cfg Config, rawTx string) (ethcompat.DecodedTransaction, error) {
	baseFee := uint64(0)
	if query, ok := provider.(ChainQueryProvider); ok {
		if state, err := query.LatestState(ctx); err == nil {
			baseFee = state.BaseFee
			if state.NextBaseFee > 0 {
				baseFee = state.NextBaseFee
			}
		}
	}
	return ethcompat.DecodeRawTransaction(rawTx, ethcompat.DecodeOptions{
		ChainID:                web3ChainID(provider.Status(ctx)),
		BaseFee:                baseFee,
		BlobBaseFee:            web3LatestBlobBaseFee(ctx, provider),
		VM:                     "evm",
		AllowUnprotectedLegacy: cfg.AllowUnprotectedLegacyTx,
	})
}

func web3SendRawBlobTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	submitter, ok := provider.(TxSubmitter)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "transaction submission is unavailable"}
	}
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "vexo_sendRawBlobTransaction requires raw transaction and blob sidecar parameters"}
	}
	rawTx, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	decoded, err := web3DecodeRawEthereumTx(ctx, provider, cfg, rawTx)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if len(decoded.BlobHashes) == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "raw transaction does not reference blobs"}
	}
	sidecarTag, rpcErr := web3EncodeBlobSidecarParam(params[1], decoded.BlobHashes)
	if rpcErr != nil {
		return nil, rpcErr
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if canonical.Tags == nil {
		canonical.Tags = make(map[string]string)
	}
	canonical.Tags[ethcompat.TagBlobSidecar] = sidecarTag
	tx, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if err := submitter.SubmitTx(ctx, tx); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return decoded.Hash, nil
}

func web3EncodeBlobSidecarParam(raw json.RawMessage, expectedHashes []string) (string, *JSONRPCError) {
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '"' {
		encoded, err := jsonRPCStringParam(raw)
		if err != nil {
			return "", &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		bundle, err := ethcompat.DecodeBlobSidecarBundle(encoded)
		if err != nil {
			return "", &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		if !web3SameBlobHashes(expectedHashes, bundle.BlobHashes) {
			return "", &JSONRPCError{Code: -32602, Message: ethcompat.ErrInvalidBlobSidecar.Error()}
		}
		return encoded, nil
	}
	var param web3BlobSidecarParam
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&param); err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	hashes := param.BlobHashes
	if len(hashes) == 0 {
		hashes = param.BlobHashesSnake
	}
	if len(hashes) == 0 {
		hashes = expectedHashes
	}
	if !web3SameBlobHashes(expectedHashes, hashes) {
		return "", &JSONRPCError{Code: -32602, Message: ethcompat.ErrInvalidBlobSidecar.Error()}
	}
	encoded, err := ethcompat.EncodeBlobSidecarBundle(ethcompat.BlobSidecarBundle{
		BlobHashes:  hashes,
		Blobs:       param.Blobs,
		Commitments: param.Commitments,
		Proofs:      param.Proofs,
	})
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	return encoded, nil
}

func web3SameBlobHashes(left []string, right []string) bool {
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

func web3BlobSidecar(ctx context.Context, provider StatusProvider, params []json.RawMessage, queryPath string) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: queryPath + " requires one hash parameter"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	response, err := query.AppQuery(ctx, []string{"evm", queryPath, hash}, nil)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return nil, nil
	}
	if response.Code != 0 {
		return nil, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	return rawJSONObject(response.Value)
}

func web3Sha3(params []json.RawMessage) (string, *JSONRPCError) {
	if len(params) != 1 {
		return "", &JSONRPCError{Code: -32602, Message: "web3_sha3 requires one data parameter"}
	}
	value, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, err := hexBytes(value)
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: "invalid hex data"}
	}
	hash := sha3.NewLegacyKeccak256()
	_, _ = hash.Write(data)
	return "0x" + hex.EncodeToString(hash.Sum(nil)), nil
}

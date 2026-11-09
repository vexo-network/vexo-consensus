package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/mempool"
	evmmodule "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type web3CallRequest struct {
	VM                        string                                 `json:"vm"`
	From                      string                                 `json:"from"`
	To                        string                                 `json:"to"`
	Method                    string                                 `json:"method"`
	Input                     string                                 `json:"input,omitempty"`
	GasLimit                  uint64                                 `json:"gas_limit,omitempty"`
	Value                     uint64                                 `json:"value,omitempty"`
	ValueHex                  string                                 `json:"value_hex,omitempty"`
	Height                    uint64                                 `json:"height,omitempty"`
	GasPrice                  uint64                                 `json:"gas_price,omitempty"`
	GasPriceHex               string                                 `json:"gas_price_hex,omitempty"`
	MaxFeePerGas              uint64                                 `json:"max_fee_per_gas,omitempty"`
	MaxFeePerGasHex           string                                 `json:"max_fee_per_gas_hex,omitempty"`
	MaxPriorityFeePerGas      uint64                                 `json:"max_priority_fee_per_gas,omitempty"`
	MaxPriorityFeePerGasHex   string                                 `json:"max_priority_fee_per_gas_hex,omitempty"`
	BaseFee                   uint64                                 `json:"base_fee,omitempty"`
	BlobBaseFee               uint64                                 `json:"blob_base_fee,omitempty"`
	BlobHashes                []string                               `json:"blob_hashes,omitempty"`
	Nonce                     uint64                                 `json:"nonce,omitempty"`
	AccessList                []contract.AccessListEntry             `json:"access_list,omitempty"`
	StateOverrides            map[string]evmmodule.CallStateOverride `json:"state_overrides,omitempty"`
	BlockOverride             evmmodule.CallBlockOverride            `json:"block_override,omitempty"`
	SetCodeAuthorizationsJSON string                                 `json:"set_code_authorizations_json,omitempty"`
	Trace                     bool                                   `json:"trace,omitempty"`
}

type web3TransactionCall struct {
	From                 string                `json:"from"`
	To                   string                `json:"to"`
	Data                 string                `json:"data"`
	Input                string                `json:"input"`
	Gas                  string                `json:"gas"`
	GasPrice             string                `json:"gasPrice"`
	MaxFeePerGas         string                `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string                `json:"maxPriorityFeePerGas"`
	Nonce                string                `json:"nonce"`
	Value                string                `json:"value"`
	VM                   string                `json:"vm"`
	Method               string                `json:"method"`
	AccessList           []web3AccessListEntry `json:"accessList,omitempty"`
	AuthorizationList    json.RawMessage       `json:"authorizationList,omitempty"`
}

type web3StateOverrideAccount struct {
	Balance   string            `json:"balance"`
	Nonce     string            `json:"nonce"`
	Code      string            `json:"code"`
	State     map[string]string `json:"state"`
	StateDiff map[string]string `json:"stateDiff"`
}

type web3BlockOverride struct {
	Number      string `json:"number"`
	Time        string `json:"time"`
	Timestamp   string `json:"timestamp"`
	GasLimit    string `json:"gasLimit"`
	BaseFee     string `json:"baseFeePerGas"`
	BlobBaseFee string `json:"blobBaseFee"`
}

type web3Receipt struct {
	TxHash          string `json:"tx_hash"`
	Height          uint64 `json:"height"`
	Status          uint32 `json:"status"`
	From            string `json:"from"`
	To              string `json:"to,omitempty"`
	ContractAddress string `json:"contract_address,omitempty"`
	GasUsed         uint64 `json:"gas_used"`
	Error           string `json:"error,omitempty"`
	Output          string `json:"output,omitempty"`
	Logs            []any  `json:"logs,omitempty"`
	StateDiff       any    `json:"state_diff,omitempty"`
}

type web3ReceiptIndex struct {
	TxHash  string `json:"tx_hash"`
	Height  uint64 `json:"height"`
	TxIndex uint64 `json:"tx_index"`
}

type web3EVMCallResponse struct {
	Output     string                `json:"output"`
	GasUsed    uint64                `json:"gas_used"`
	Failed     bool                  `json:"failed,omitempty"`
	Error      string                `json:"error,omitempty"`
	AccessList []web3AccessListEntry `json:"access_list,omitempty"`
	StateDiff  any                   `json:"state_diff,omitempty"`
	VMTrace    any                   `json:"vm_trace,omitempty"`
}

type web3AccessListEntry struct {
	Address     string   `json:"address"`
	StorageKeys []string `json:"storage_keys,omitempty"`
}

func (entry *web3AccessListEntry) UnmarshalJSON(data []byte) error {
	var payload struct {
		Address           string   `json:"address"`
		StorageKeys       []string `json:"storageKeys"`
		StorageKeysLegacy []string `json:"storage_keys"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	entry.Address = payload.Address
	entry.StorageKeys = append([]string(nil), payload.StorageKeys...)
	if len(entry.StorageKeys) == 0 {
		entry.StorageKeys = append([]string(nil), payload.StorageKeysLegacy...)
	}
	return nil
}

type web3AccountStateResponse struct {
	Address    string `json:"address"`
	Balance    uint64 `json:"balance"`
	BalanceHex string `json:"balance_hex"`
	Nonce      uint64 `json:"nonce"`
	Code       string `json:"code"`
}

func web3BlockByNumber(ctx context.Context, provider StatusProvider, params []json.RawMessage) (store.BlockRecord, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	if len(params) == 0 || len(params) > 2 {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "eth_getBlockByNumber requires block tag and optional full transaction flag"}
	}
	tag, err := jsonRPCStringParam(params[0])
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	var (
		record   store.BlockRecord
		queryErr error
	)
	switch tag {
	case "latest", "pending":
		record, queryErr = blockProvider.LatestBlock(ctx)
	case "safe", "finalized":
		height, rpcErr := web3FinalizedHeight(ctx, provider)
		if rpcErr != nil {
			return store.BlockRecord{}, rpcErr
		}
		record, queryErr = blockProvider.BlockByHeight(ctx, height)
	case "earliest":
		queryProvider, ok := provider.(ChainQueryProvider)
		if !ok {
			return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block index query is unavailable"}
		}
		index, err := queryProvider.BlockIndex(ctx)
		if err != nil {
			queryErr = err
			break
		}
		record, queryErr = blockProvider.BlockByHeight(ctx, index.EarliestHeight)
	default:
		height, err := parseHexQuantity(tag)
		if err != nil {
			return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "invalid block number tag"}
		}
		record, queryErr = blockProvider.BlockByHeight(ctx, types.Height(height))
	}
	if errors.Is(queryErr, store.ErrBlockNotFound) || errors.Is(queryErr, store.ErrBlockIndexNotFound) {
		return store.BlockRecord{}, nil
	}
	if queryErr != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: queryErr.Error()}
	}
	return record, nil
}

func web3BlockByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (store.BlockRecord, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	if len(params) == 0 || len(params) > 2 {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "eth_getBlockByHash requires block hash and optional full transaction flag"}
	}
	hashText, err := jsonRPCStringParam(params[0])
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	hash, err := parseHexHash(hashText)
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "invalid block hash"}
	}
	record, err := blockProvider.BlockByHash(ctx, hash)
	if errors.Is(err, store.ErrBlockNotFound) {
		return store.BlockRecord{}, nil
	}
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return record, nil
}

func web3FullTransactionParam(params []json.RawMessage, index int) bool {
	if len(params) <= index {
		return false
	}
	var full bool
	_ = json.Unmarshal(params[index], &full)
	return full
}

func web3BlockTransactionCount(record store.BlockRecord) any {
	if record.Block.Header.Height == 0 {
		return hexQuantity(0)
	}
	return hexQuantity(uint64(len(record.Block.Txs)))
}

func web3PendingSequence(ctx context.Context, provider StatusProvider, address string, sequence uint64) (uint64, error) {
	var txs []types.Tx
	if snapshot, ok := provider.(PendingTxSnapshotProvider); ok {
		var err error
		txs, err = snapshot.PendingTxSnapshot(ctx)
		if err != nil {
			return 0, err
		}
	} else {
		var rpcErr *JSONRPCError
		txs, rpcErr = web3PendingTxs(ctx, provider)
		if rpcErr != nil {
			if rpcErr.Message == "pending transaction query is unavailable" {
				return sequence, nil
			}
			return 0, errors.New(rpcErr.Message)
		}
	}
	candidates := make([]uint64, 0)
	for _, tx := range txs {
		details := web3TransactionDetails(tx)
		from, ok := details.From.(string)
		if !ok || !strings.EqualFold(from, address) {
			continue
		}
		candidates = append(candidates, details.Nonce)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	next := sequence
	for _, nonce := range candidates {
		if nonce < next {
			continue
		}
		if nonce > next {
			break
		}
		next++
	}
	return next, nil
}

func web3AccountState(ctx context.Context, provider StatusProvider, address string, params []json.RawMessage, tagIndex int) (web3AccountStateResponse, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		if tagIndex >= 0 && len(params) > tagIndex {
			tag, err := jsonRPCStringParam(params[tagIndex])
			if err == nil && tag != "" && tag != "latest" && tag != "pending" {
				return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: "historical EVM account query is unavailable"}
			}
		}
		accountProvider, ok := provider.(AccountQueryProvider)
		if !ok {
			return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: "account query is unavailable"}
		}
		sequence, err := accountProvider.AccountSequence(ctx, types.Address(address))
		if err != nil {
			return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return web3AccountStateResponse{Address: address, Nonce: sequence}, nil
	}
	data, rpcErr := web3HistoricalAccountQueryData(ctx, provider, params, tagIndex)
	if rpcErr != nil {
		return web3AccountStateResponse{}, rpcErr
	}
	response, err := query.AppQuery(ctx, []string{"evm", "account", address}, data)
	if err != nil {
		return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return web3AccountStateResponse{Address: address}, nil
	}
	if response.Code != 0 {
		return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var account web3AccountStateResponse
	if err := json.Unmarshal(response.Value, &account); err != nil {
		return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: "invalid EVM account response"}
	}
	return account, nil
}

func web3TransactionByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByHash requires one transaction hash"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if query, ok := provider.(AppQueryProvider); ok {
		response, err := query.AppQuery(ctx, []string{"evm", "receipt", hash}, nil)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if response.Code != 3 {
			if response.Code != 0 {
				return nil, &JSONRPCError{Code: -32000, Message: response.Log}
			}
			return web3TransactionFromReceipt(ctx, provider, response.Value)
		}
	}
	pending, rpcErr := web3PendingTransactionByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if pending != nil {
		return pending, nil
	}
	return web3CommittedTransactionByHash(ctx, provider, hash)
}

func web3TransactionByBlockNumberAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByBlockNumberAndIndex requires block tag and transaction index"}
	}
	record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3TransactionByBlockIndex(record, params[1])
}

func web3TransactionByBlockHashAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByBlockHashAndIndex requires block hash and transaction index"}
	}
	record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3TransactionByBlockIndex(record, params[1])
}

func web3TransactionByBlockIndex(record store.BlockRecord, rawIndex json.RawMessage) (any, *JSONRPCError) {
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	indexText, err := jsonRPCStringParam(rawIndex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	index, err := parseHexQuantity(indexText)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid transaction index"}
	}
	if index >= uint64(len(record.Block.Txs)) {
		return nil, nil
	}
	tx := record.Block.Txs[index]
	return web3TransactionFromBlockRecord(record, int(index), web3TxHash(tx), tx), nil
}

func web3PendingTransactionByHash(ctx context.Context, provider StatusProvider, hash string) (any, *JSONRPCError) {
	tx, found, rpcErr := web3PendingTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3PendingTransaction(tx), nil
}

func web3PendingTxByHash(ctx context.Context, provider StatusProvider, hash string) (types.Tx, bool, *JSONRPCError) {
	var txs []types.Tx
	if snapshot, ok := provider.(PendingTxSnapshotProvider); ok {
		var err error
		txs, err = snapshot.PendingTxSnapshot(ctx)
		if err != nil {
			return nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
	} else {
		var rpcErr *JSONRPCError
		txs, rpcErr = web3PendingTxs(ctx, provider)
		if rpcErr != nil {
			if rpcErr.Message == "pending transaction query is unavailable" {
				return nil, false, nil
			}
			return nil, false, rpcErr
		}
	}
	for _, tx := range txs {
		if web3TxMatchesHash(tx, hash) {
			return tx, true, nil
		}
	}
	return nil, false, nil
}

func web3TxMatchesHash(tx types.Tx, hash string) bool {
	return strings.EqualFold(web3TxHash(tx), hash) || strings.EqualFold(web3HashString(mempool.HashTx(tx)), hash)
}

func web3CommittedTransactionByHash(ctx context.Context, provider StatusProvider, hash string) (any, *JSONRPCError) {
	record, index, tx, found, rpcErr := web3CommittedTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3TransactionFromBlockRecord(record, index, web3TxHash(tx), tx), nil
}

func web3CommittedTxByHash(ctx context.Context, provider StatusProvider, hash string) (store.BlockRecord, int, types.Tx, bool, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, 0, nil, false, nil
	}
	if index, found, rpcErr := web3ReceiptIndexByHash(ctx, provider, hash); rpcErr != nil || found {
		if rpcErr != nil {
			return store.BlockRecord{}, 0, nil, false, rpcErr
		}
		record, err := blockProvider.BlockByHeight(ctx, types.Height(index.Height))
		if errors.Is(err, store.ErrBlockNotFound) {
			return store.BlockRecord{}, 0, nil, false, nil
		}
		if err != nil {
			return store.BlockRecord{}, 0, nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if index.TxIndex < uint64(len(record.Block.Txs)) {
			tx := record.Block.Txs[index.TxIndex]
			if strings.EqualFold(index.TxHash, hash) || web3TxMatchesHash(tx, hash) {
				return record, int(index.TxIndex), append(types.Tx(nil), tx...), true, nil
			}
		}
	}
	status := provider.Status(ctx)
	for height := status.LatestHeight; height > 0; height-- {
		record, err := blockProvider.BlockByHeight(ctx, height)
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return store.BlockRecord{}, 0, nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		for index, tx := range record.Block.Txs {
			if web3TxMatchesHash(tx, hash) {
				return record, index, tx, true, nil
			}
		}
	}
	return store.BlockRecord{}, 0, nil, false, nil
}

func web3RawTransactionByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getRawTransactionByHash requires one transaction hash"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if raw, rpcErr := web3PendingRawTransactionByHash(ctx, provider, hash); rpcErr != nil || raw != nil {
		return raw, rpcErr
	}
	_, _, tx, found, rpcErr := web3CommittedTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3RawTransaction(tx), nil
}

func web3PendingRawTransactionByHash(ctx context.Context, provider StatusProvider, hash string) (any, *JSONRPCError) {
	tx, found, rpcErr := web3PendingTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3RawTransaction(tx), nil
}

func web3RawTransactionByBlockNumberAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getRawTransactionByBlockNumberAndIndex requires block tag and transaction index"}
	}
	record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3RawTransactionByBlockIndex(record, params[1])
}

func web3RawTransactionByBlockHashAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getRawTransactionByBlockHashAndIndex requires block hash and transaction index"}
	}
	record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3RawTransactionByBlockIndex(record, params[1])
}

func web3RawTransactionByBlockIndex(record store.BlockRecord, rawIndex json.RawMessage) (any, *JSONRPCError) {
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	indexText, err := jsonRPCStringParam(rawIndex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	index, err := parseHexQuantity(indexText)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid transaction index"}
	}
	if index >= uint64(len(record.Block.Txs)) {
		return nil, nil
	}
	return web3RawTransaction(record.Block.Txs[index]), nil
}

func web3RawTransaction(tx types.Tx) any {
	if raw, found := vexoapp.TxTag(tx, ethcompat.TagRaw); found && raw != "" {
		return "0x" + strings.TrimPrefix(raw, "0x")
	}
	return "0x" + hex.EncodeToString(tx)
}

func web3BlockFromRecord(ctx context.Context, provider StatusProvider, cfg Config, record store.BlockRecord, fullTransactions bool) (any, *JSONRPCError) {
	if record.Block.Header.Height == 0 {
		stateRoot := "0x" + strings.Repeat("0", 64)
		if status := provider.Status(ctx); status.LatestAppHash != (types.Hash{}) {
			stateRoot = "0x" + hex.EncodeToString(status.LatestAppHash[:])
		}
		return map[string]any{
			"number":           "0x0",
			"hash":             "0x" + strings.Repeat("0", 64),
			"parentHash":       "0x" + strings.Repeat("0", 64),
			"nonce":            "0x0000000000000000",
			"sha3Uncles":       "0x" + strings.Repeat("0", 64),
			"mixHash":          "0x" + strings.Repeat("0", 64),
			"logsBloom":        "0x" + strings.Repeat("0", 512),
			"transactionsRoot": "0x" + strings.Repeat("0", 64),
			"stateRoot":        stateRoot,
			"receiptsRoot":     "0x" + strings.Repeat("0", 64),
			"miner":            "0x0000000000000000000000000000000000000000",
			"difficulty":       "0x0",
			"totalDifficulty":  "0x0",
			"extraData":        "0x",
			"size":             "0x0",
			"gasLimit":         "0x0",
			"gasUsed":          "0x0",
			"baseFeePerGas":    "0x0",
			"blobGasUsed":      "0x0",
			"excessBlobGas":    "0x0",
			"timestamp":        "0x0",
			"transactions":     []any{},
			"uncles":           []any{},
			"withdrawals":      []any{},
			"withdrawalsRoot":  "0x" + strings.Repeat("0", 64),
		}, nil
	}
	stateRoot, ok := web3StateRoot(ctx, provider, cfg, record)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "EVM state root is unavailable for block"}
	}
	transactions := make([]any, 0, len(record.Block.Txs))
	for index, tx := range record.Block.Txs {
		hashText := web3TxHash(tx)
		if !fullTransactions {
			transactions = append(transactions, hashText)
			continue
		}
		transactions = append(transactions, web3TransactionFromBlockRecord(record, index, hashText, tx))
	}
	return map[string]any{
		"number":           hexQuantity(uint64(record.Block.Header.Height)),
		"hash":             "0x" + hex.EncodeToString(record.Hash[:]),
		"parentHash":       "0x" + hex.EncodeToString(record.Block.Header.PreviousBlockHash[:]),
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"mixHash":          "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logsBloom":        web3LogsBloom(record.Block.Txs, record.TxResults),
		"transactionsRoot": web3TransactionsRoot(record.Block.Txs),
		"stateRoot":        stateRoot,
		"receiptsRoot":     web3ReceiptsRoot(record.Block.Txs, record.TxResults),
		"miner":            "0x0000000000000000000000000000000000000000",
		"difficulty":       "0x0",
		"totalDifficulty":  "0x0",
		"extraData":        "0x",
		"size":             hexQuantity(uint64(len(record.Block.Txs))),
		"gasLimit":         web3BlockGasLimit(record.TxResults),
		"gasUsed":          hexQuantity(web3BlockGasUsed(record.TxResults)),
		"baseFeePerGas":    hexQuantity(web3BlockBaseFee(ctx, provider, record)),
		"blobGasUsed":      hexQuantity(web3BlockBlobGasUsed(ctx, provider, record)),
		"excessBlobGas":    hexQuantity(web3BlockExcessBlobGas(ctx, provider, record)),
		"timestamp":        hexQuantity(uint64(record.Block.Header.TimeUnixNano / int64(time.Second))),
		"transactions":     transactions,
		"uncles":           []any{},
		"withdrawals":      []any{},
		"withdrawalsRoot":  "0x0000000000000000000000000000000000000000000000000000000000000000",
	}, nil
}

func web3GetProof(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	if len(params) < 2 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getProof requires address, storage keys, and optional block tag"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	var storageKeys []string
	if err := json.Unmarshal(params[1], &storageKeys); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "storage keys array is required"}
	}
	height := types.Height(0)
	if len(params) == 3 {
		var rpcErr *JSONRPCError
		height, rpcErr = web3BlockHeightParam(ctx, provider, params[2])
		if rpcErr != nil {
			return nil, rpcErr
		}
	}
	request := map[string]any{"address": address, "storage_keys": storageKeys}
	if height > 0 {
		request["height"] = uint64(height)
	}
	encoded, _ := json.Marshal(request)
	response, err := query.AppQuery(ctx, []string{"evm", "eth_proof"}, encoded)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code != 0 {
		return nil, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	payload, rpcErr := rawJSONObject(response.Value)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var result any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM proof response"}
	}
	return result, nil
}

func web3StateRoot(ctx context.Context, provider StatusProvider, _ Config, record store.BlockRecord) (string, bool) {
	if record.Block.Header.Height == provider.Status(ctx).LatestHeight {
		if query, ok := provider.(AppQueryProvider); ok {
			payload, _ := json.Marshal(map[string]uint64{"height": uint64(record.Block.Header.Height)})
			response, err := query.AppQuery(ctx, []string{"evm", "eth_state_root"}, payload)
			if err == nil && response.Code == 0 {
				var payload struct {
					StateRoot string `json:"state_root"`
				}
				if json.Unmarshal(response.Value, &payload) == nil && payload.StateRoot != "" {
					return payload.StateRoot, true
				}
			}
		}
	} else if query, ok := provider.(AppQueryProvider); ok {
		payload, _ := json.Marshal(map[string]uint64{"height": uint64(record.Block.Header.Height)})
		response, err := query.AppQuery(ctx, []string{"evm", "eth_state_root"}, payload)
		if err == nil && response.Code == 0 {
			var payload struct {
				StateRoot string `json:"state_root"`
			}
			if json.Unmarshal(response.Value, &payload) == nil && payload.StateRoot != "" {
				return payload.StateRoot, true
			}
		}
	}
	return "", false
}

func web3BlockBaseFee(ctx context.Context, provider StatusProvider, record store.BlockRecord) uint64 {
	if stateProvider, ok := provider.(StateByHeightProvider); ok && record.Block.Header.Height != 0 {
		state, err := stateProvider.StateByHeight(ctx, record.Block.Header.Height)
		if err == nil {
			if state.BaseFee > 0 {
				return state.BaseFee
			}
			return state.NextBaseFee
		}
	}
	query, ok := provider.(ChainQueryProvider)
	if !ok {
		return 0
	}
	state, err := query.LatestState(ctx)
	if err != nil {
		return 0
	}
	if state.Height != 0 && record.Block.Header.Height != 0 && state.Height != record.Block.Header.Height {
		return 0
	}
	if state.BaseFee > 0 {
		return state.BaseFee
	}
	return state.NextBaseFee
}

func web3LatestBlobBaseFee(ctx context.Context, provider StatusProvider) uint64 {
	if query, ok := provider.(ChainQueryProvider); ok {
		state, err := query.LatestState(ctx)
		if err == nil {
			if state.NextBlobBaseFee > 0 {
				return state.NextBlobBaseFee
			}
			return state.BlobBaseFee
		}
	}
	return 0
}

func web3BaseFeeAtHeight(ctx context.Context, provider StatusProvider, height types.Height) uint64 {
	if height == 0 {
		return web3LatestBaseFee(ctx, provider)
	}
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, height)
		if err == nil {
			if state.BaseFee > 0 {
				return state.BaseFee
			}
			return state.NextBaseFee
		}
	}
	if blockProvider, ok := provider.(BlockProvider); ok {
		record, err := blockProvider.BlockByHeight(ctx, height)
		if err == nil {
			return web3BlockBaseFee(ctx, provider, record)
		}
	}
	return 0
}

func web3NextBaseFeeAfterHeight(ctx context.Context, provider StatusProvider, height types.Height) uint64 {
	if height == 0 {
		return web3LatestBaseFee(ctx, provider)
	}
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, height)
		if err == nil && state.NextBaseFee > 0 {
			return state.NextBaseFee
		}
		nextState, err := query.StateByHeight(ctx, height+1)
		if err == nil && nextState.BaseFee > 0 {
			return nextState.BaseFee
		}
	}
	return web3BaseFeeAtHeight(ctx, provider, height)
}

func web3BlobBaseFeeAtHeight(ctx context.Context, provider StatusProvider, height types.Height) uint64 {
	if height == 0 {
		return web3LatestBlobBaseFee(ctx, provider)
	}
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, height)
		if err == nil {
			if state.BlobBaseFee > 0 {
				return state.BlobBaseFee
			}
			return state.NextBlobBaseFee
		}
	}
	return web3LatestBlobBaseFee(ctx, provider)
}

func web3BlockBlobGasUsed(ctx context.Context, provider StatusProvider, record store.BlockRecord) uint64 {
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, record.Block.Header.Height)
		if err == nil {
			return state.BlobGasUsed
		}
	}
	return web3TxsBlobGas(record.Block.Txs)
}

func web3BlockExcessBlobGas(ctx context.Context, provider StatusProvider, record store.BlockRecord) uint64 {
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, record.Block.Header.Height)
		if err == nil {
			return state.ExcessBlobGas
		}
	}
	return 0
}

func web3TxsBlobGas(txs []types.Tx) uint64 {
	var total uint64
	for _, tx := range txs {
		blobGas, found := vexoapp.TxUintTag(tx, ethcompat.TagBlobGas)
		if !found {
			continue
		}
		if total > ^uint64(0)-blobGas {
			return ^uint64(0)
		}
		total += blobGas
	}
	return total
}

func web3MaxPriorityFeePerGas(ctx context.Context, provider StatusProvider) string {
	if query, ok := provider.(ChainQueryProvider); ok {
		state, err := query.LatestState(ctx)
		if err == nil && (state.BaseFee > 0 || state.NextBaseFee > 0) {
			return hexQuantity(1)
		}
	}
	return hexQuantity(1)
}

func web3FeeHistory(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) < 2 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_feeHistory requires block count, newest block, and optional reward percentiles"}
	}
	blockCount, err := web3QuantityParam(params[0])
	if err != nil || blockCount == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid fee history block count"}
	}
	if blockCount > 1024 {
		blockCount = 1024
	}
	newest, rpcErr := web3BlockHeightParam(ctx, provider, params[1])
	if rpcErr != nil {
		return nil, rpcErr
	}
	oldest := uint64(0)
	if uint64(newest) >= blockCount {
		oldest = uint64(newest) - blockCount + 1
	}
	baseFees := make([]string, 0, blockCount+1)
	gasUsedRatios := make([]float64, 0, blockCount)
	for index := uint64(0); index < blockCount; index++ {
		height := types.Height(oldest + index)
		baseFees = append(baseFees, hexQuantity(web3BaseFeeAtHeight(ctx, provider, height)))
		gasUsedRatios = append(gasUsedRatios, web3BlockGasUsedRatio(ctx, provider, height))
	}
	baseFees = append(baseFees, hexQuantity(web3NextBaseFeeAfterHeight(ctx, provider, newest)))
	response := map[string]any{
		"oldestBlock":   hexQuantity(oldest),
		"baseFeePerGas": baseFees,
		"gasUsedRatio":  gasUsedRatios,
	}
	if len(params) == 3 && string(params[2]) != "null" {
		percentiles, err := web3RewardPercentiles(params[2])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		reward := make([][]string, 0, blockCount)
		for range blockCount {
			row := make([]string, len(percentiles))
			for index := range row {
				row[index] = web3MaxPriorityFeePerGas(ctx, provider)
			}
			reward = append(reward, row)
		}
		response["reward"] = reward
	}
	return response, nil
}

func web3BlockReceipts(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getBlockReceipts requires block hash or block tag"}
	}
	record, rpcErr := web3BlockRecordParam(ctx, provider, params[0])
	if rpcErr != nil {
		return nil, rpcErr
	}
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	receipts := make([]any, 0, len(record.TxResults))
	for _, result := range record.TxResults {
		if _, ok := web3ReceiptFromResult(result); !ok {
			continue
		}
		receipt, rpcErr := web3ReceiptObject(ctx, provider, result.Data)
		if rpcErr != nil {
			return nil, rpcErr
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func web3ReceiptValueByHash(ctx context.Context, provider StatusProvider, hash string) ([]byte, bool, *JSONRPCError) {
	if query, ok := provider.(AppQueryProvider); ok {
		response, err := query.AppQuery(ctx, []string{"evm", "receipt", hash}, nil)
		if err != nil {
			return nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if response.Code != 3 {
			if response.Code != 0 {
				return nil, false, &JSONRPCError{Code: -32000, Message: response.Log}
			}
			return response.Value, true, nil
		}
	}
	if _, pending, rpcErr := web3PendingTxByHash(ctx, provider, hash); rpcErr != nil {
		return nil, false, rpcErr
	} else if pending {
		return nil, false, nil
	}
	record, index, _, found, rpcErr := web3CommittedTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, false, rpcErr
	}
	if index >= len(record.TxResults) {
		return nil, false, nil
	}
	if _, ok := web3ReceiptFromResult(record.TxResults[index]); !ok {
		return nil, false, nil
	}
	return record.TxResults[index].Data, true, nil
}

func web3ReceiptIndexByHash(ctx context.Context, provider StatusProvider, hash string) (web3ReceiptIndex, bool, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return web3ReceiptIndex{}, false, nil
	}
	response, err := query.AppQuery(ctx, []string{"evm", "receipt_index", hash}, nil)
	if err != nil {
		return web3ReceiptIndex{}, false, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return web3ReceiptIndex{}, false, nil
	}
	if response.Code != 0 {
		return web3ReceiptIndex{}, false, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var index web3ReceiptIndex
	if err := json.Unmarshal(response.Value, &index); err != nil {
		return web3ReceiptIndex{}, false, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt index response"}
	}
	if index.TxHash == "" || index.Height == 0 {
		return web3ReceiptIndex{}, false, nil
	}
	return index, true, nil
}

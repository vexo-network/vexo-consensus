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

// web3LogsForAddress fetches the logs of one address, or of every address when
// address is empty, for the inclusive block range [fromBlock, toBlock] that
// match topics, at most limit of them. The module reads only the heights in the
// range and filters while it reads, so the cost of a query is bounded by the
// range and the limit, not by the length of the chain or the logs it holds.
func web3LogsForAddress(ctx context.Context, provider StatusProvider, address string, fromBlock uint64, toBlock uint64, topics [][]string, limit uint64) ([]any, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	path := []string{"evm", "logs"}
	if address != "" {
		path = append(path, address)
	}
	data, err := json.Marshal(evmmodule.LogsRequest{FromBlock: fromBlock, ToBlock: toBlock, Topics: topics, Limit: limit})
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	response, err := query.AppQuery(ctx, path, data)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return []any{}, nil
	}
	if response.Code != 0 {
		return nil, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	logs, rpcErr := rawJSONObject(response.Value)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3LogArray(logs), nil
}

// web3LogWindow resolves the blocks a log filter covers right now: an open
// ended filter follows the head and a bound past the head stops at it. The
// width limit applies to the resolved window, so a filter cannot avoid it by
// leaving its upper bound open.
func web3LogWindow(ctx context.Context, provider StatusProvider, cfg Config, filter web3Filter) (uint64, uint64, bool, *JSONRPCError) {
	latest := uint64(provider.Status(ctx).LatestHeight)
	toBlock := filter.ToBlock
	if toBlock > latest {
		toBlock = latest
	}
	// Genesis carries no transactions and so no logs.
	if toBlock == 0 || filter.FromBlock > toBlock {
		return 0, 0, false, nil
	}
	if toBlock-filter.FromBlock >= cfg.logMaxBlockRange() {
		return 0, 0, false, &JSONRPCError{Code: -32602, Message: "eth_getLogs block range exceeds configured limit"}
	}
	return filter.FromBlock, toBlock, true, nil
}

// logMaxResults is the most logs one log query may return: the configured limit, never above
// what the EVM module returns for one query.
func (cfg Config) logMaxResults() int {
	maxResults := cfg.Web3LogMaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxWeb3LogResults
	}
	if maxResults > evmmodule.MaxLogQueryResults {
		maxResults = evmmodule.MaxLogQueryResults
	}
	return maxResults
}

// logMaxBlockRange is the most blocks one log query may cover: the configured
// limit, never above what the EVM module itself reads for one query.
func (cfg Config) logMaxBlockRange() uint64 {
	maxRange := cfg.Web3LogMaxBlockRange
	if maxRange == 0 {
		maxRange = defaultMaxWeb3LogBlockRange
	}
	if maxRange > evmmodule.MaxLogBlockRange {
		maxRange = evmmodule.MaxLogBlockRange
	}
	return maxRange
}

func web3LogsForFilter(ctx context.Context, provider StatusProvider, cfg Config, filter web3Filter) ([]any, *JSONRPCError) {
	maxResults := cfg.logMaxResults()
	results := make([]any, 0)
	fromBlock, toBlock, nonEmpty, rpcErr := web3LogWindow(ctx, provider, cfg, filter)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !nonEmpty {
		return results, nil
	}
	locationCache := make(map[uint64]map[string]web3LogLocation)
	appendMatching := func(logs []any) *JSONRPCError {
		for _, log := range logs {
			if !web3LogMatchesFilter(log, filter) {
				continue
			}
			location, found := web3ResolveLogLocation(ctx, provider, log, locationCache)
			if found {
				results = append(results, web3NormalizeLogAt(log, location))
			} else {
				results = append(results, web3NormalizeLog(log))
			}
			if len(results) > maxResults {
				return &JSONRPCError{Code: -32005, Message: "eth_getLogs result limit exceeded"}
			}
		}
		return nil
	}
	addresses := filter.Addresses
	if len(addresses) == 0 {
		addresses = []string{""}
	}
	for _, address := range addresses {
		// One more log than may still be returned shows that the limit is exceeded.
		limit := uint64(maxResults-len(results)) + 1
		logs, rpcErr := web3LogsForAddress(ctx, provider, address, fromBlock, toBlock, filter.Topics, limit)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if rpcErr := appendMatching(logs); rpcErr != nil {
			return nil, rpcErr
		}
	}
	return results, nil
}

func web3ResolveLogLocation(
	ctx context.Context,
	provider StatusProvider,
	value any,
	cache map[uint64]map[string]web3LogLocation,
) (web3LogLocation, bool) {
	log, ok := value.(map[string]any)
	if !ok {
		return web3LogLocation{}, false
	}
	height := web3LogBlockNumber(log)
	txHash := web3LogString(log, "transaction_hash", "transactionHash", "tx_hash")
	if height == 0 || txHash == "" {
		return web3LogLocation{}, false
	}
	locations, cached := cache[height]
	if !cached {
		locations = web3LogLocationsAtHeight(ctx, provider, height)
		cache[height] = locations
	}
	location, found := locations[strings.ToLower(txHash)]
	if !found {
		return web3LogLocation{}, false
	}
	localIndex, _ := web3LogUintValue(log, "log_index", "logIndex")
	location.LogIndex += localIndex
	return location, true
}

func web3LogLocationsAtHeight(ctx context.Context, provider StatusProvider, height uint64) map[string]web3LogLocation {
	locations := make(map[string]web3LogLocation)
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return locations
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(height))
	if err != nil {
		return locations
	}
	blockHash := "0x" + hex.EncodeToString(record.Hash[:])
	var logIndexBase uint64
	for index, tx := range record.Block.Txs {
		txHash := web3TxHash(tx)
		locations[strings.ToLower(txHash)] = web3LogLocation{
			Mined:            true,
			BlockHash:        blockHash,
			BlockNumber:      height,
			TransactionHash:  txHash,
			TransactionIndex: uint64(index),
			LogIndex:         logIndexBase,
		}
		if index >= len(record.TxResults) {
			continue
		}
		receipt, ok := web3ReceiptFromResult(record.TxResults[index])
		if ok {
			logIndexBase += uint64(len(receipt.Logs))
		}
	}
	return locations
}

func web3FilterChanges(ctx context.Context, provider StatusProvider, cfg Config, filter web3Filter, onlyChanges bool) (any, *JSONRPCError) {
	switch filter.Type {
	case "block":
		return web3BlockFilterChanges(ctx, provider, filter, onlyChanges)
	default:
		logs, rpcErr := web3LogsForFilter(ctx, provider, cfg, filter)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if !onlyChanges {
			return logs, nil
		}
		changes := make([]any, 0, len(logs))
		for _, log := range logs {
			id := web3LogID(log)
			if id == "" || filter.SeenLogs[id] {
				continue
			}
			changes = append(changes, log)
		}
		return changes, nil
	}
}

func web3PendingFilterChanges(ctx context.Context, provider StatusProvider, filter web3Filter, onlyChanges bool) ([]string, web3Filter, *JSONRPCError) {
	pendingProvider, ok := provider.(PendingTxProvider)
	if !ok {
		return nil, filter, &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
	}
	hashes, err := pendingProvider.PendingTxHashes(ctx)
	if err != nil {
		return nil, filter, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	items := make([]string, 0, len(hashes))
	nextSeen := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		encoded := web3HashString(hash)
		nextSeen[encoded] = true
		if onlyChanges && filter.SeenPending[encoded] {
			continue
		}
		items = append(items, encoded)
	}
	filter.SeenPending = nextSeen
	return items, filter, nil
}

func web3BlockFilterChanges(ctx context.Context, provider StatusProvider, filter web3Filter, onlyChanges bool) ([]string, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	latest := uint64(provider.Status(ctx).LatestHeight)
	from := uint64(1)
	if onlyChanges {
		from = filter.LastHeight + 1
	}
	if from > latest {
		return []string{}, nil
	}
	hashes := make([]string, 0, latest-from+1)
	for height := from; height <= latest; height++ {
		record, err := blockProvider.BlockByHeight(ctx, types.Height(height))
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		hashes = append(hashes, "0x"+hex.EncodeToString(record.Hash[:]))
	}
	return hashes, nil
}

func web3LogMatchesFilter(value any, filter web3Filter) bool {
	log, ok := value.(map[string]any)
	if !ok {
		return false
	}
	blockNumber := web3LogBlockNumber(log)
	if blockNumber < filter.FromBlock || blockNumber > filter.ToBlock {
		return false
	}
	if len(filter.Topics) == 0 {
		return true
	}
	topics := web3LogTopicValues(log)
	for index, accepted := range filter.Topics {
		if len(accepted) == 0 {
			continue
		}
		if index >= len(topics) {
			return false
		}
		found := false
		actual := strings.ToLower(topics[index])
		for _, topic := range accepted {
			if actual == topic {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type web3LogLocation struct {
	Mined            bool
	BlockHash        string
	BlockNumber      uint64
	TransactionHash  string
	TransactionIndex uint64
	LogIndex         uint64
}

func web3NormalizeLog(value any) any {
	return web3NormalizeLogAt(value, web3LogLocation{})
}

func web3NormalizeLogAt(value any, location web3LogLocation) any {
	log, ok := value.(map[string]any)
	if !ok {
		return value
	}
	normalized := make(map[string]any, len(log)+6)
	for key, item := range log {
		normalized[key] = item
	}
	blockNumber := web3LogBlockNumber(log)
	if blockNumber == 0 {
		blockNumber = location.BlockNumber
	}
	if blockNumber > 0 {
		normalized["blockNumber"] = hexQuantity(blockNumber)
	}
	txHash := web3LogString(log, "transaction_hash", "transactionHash", "tx_hash")
	if txHash == "" {
		txHash = location.TransactionHash
	}
	if txHash != "" {
		normalized["transactionHash"] = txHash
	}
	logIndex, foundLogIndex := web3LogUintValue(log, "log_index", "logIndex")
	if !foundLogIndex || location.Mined {
		logIndex = location.LogIndex
	}
	normalized["logIndex"] = hexQuantity(logIndex)
	if location.Mined {
		normalized["blockHash"] = location.BlockHash
		normalized["transactionIndex"] = hexQuantity(location.TransactionIndex)
		normalized["removed"] = false
	}
	if _, found := normalized["topics"]; !found {
		normalized["topics"] = []any{}
	}
	if _, found := normalized["data"]; !found {
		normalized["data"] = "0x"
	}
	return normalized
}

func web3LogBlockNumber(log map[string]any) uint64 {
	return web3LogUint(log, "block_number", "blockNumber", "height")
}

func web3LogTopicValues(log map[string]any) []string {
	raw, found := log["topics"]
	if !found {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	topics := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		topics = append(topics, text)
	}
	return topics
}

func web3LogString(log map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := log[key].(string)
		if ok {
			return value
		}
	}
	return ""
}

func web3LogUint(log map[string]any, keys ...string) uint64 {
	value, _ := web3LogUintValue(log, keys...)
	return value
}

func web3LogUintValue(log map[string]any, keys ...string) (uint64, bool) {
	for _, key := range keys {
		value, found := log[key]
		if !found {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed >= 0 {
				return uint64(typed), true
			}
		case uint64:
			return typed, true
		case string:
			if strings.HasPrefix(typed, "0x") {
				parsed, err := parseHexQuantity(typed)
				if err == nil {
					return parsed, true
				}
				continue
			}
			parsed, err := strconv.ParseUint(typed, 10, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func web3SeenLogSet(logs []any) map[string]bool {
	seen := make(map[string]bool, len(logs))
	for _, log := range logs {
		id := web3LogID(log)
		if id != "" {
			seen[id] = true
		}
	}
	return seen
}

func web3LogID(value any) string {
	log, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	txHash := web3LogString(log, "transactionHash", "transaction_hash", "tx_hash")
	if txHash == "" {
		return ""
	}
	return txHash + ":" + strconv.FormatUint(web3LogUint(log, "logIndex", "log_index"), 10)
}

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

func web3TxpoolStatus(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		return web3PendingTransaction(tx)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"pending": hexQuantity(uint64(web3TxpoolBucketCount(pending))),
		"queued":  hexQuantity(uint64(web3TxpoolBucketCount(queued))),
	}, nil
}

func web3TxpoolContent(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		return web3PendingTransaction(tx)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{"pending": pending, "queued": queued}, nil
}

func web3TxpoolContentFrom(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "txpool_contentFrom requires one address"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		return web3PendingTransaction(tx)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"pending": web3TxpoolBucketForAddress(pending, address),
		"queued":  web3TxpoolBucketForAddress(queued, address),
	}, nil
}

func web3TxpoolInspect(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		details := web3TransactionDetails(tx)
		to := "<contract creation>"
		if raw, ok := details.To.(string); ok && raw != "" {
			to = raw
		}
		return fmt.Sprintf("%s: %s value + %d gas × %d wei", to, web3TxValueDecimal(details), details.Gas, details.GasPrice)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{"pending": pending, "queued": queued}, nil
}

func web3TxpoolBuckets(ctx context.Context, provider StatusProvider, txs []types.Tx, item func(types.Tx) any) (map[string]map[string]any, map[string]map[string]any, *JSONRPCError) {
	grouped := make(map[string][]types.Tx)
	for _, tx := range txs {
		from := web3TxpoolSender(tx)
		grouped[from] = append(grouped[from], tx)
	}
	pending := make(map[string]map[string]any)
	queued := make(map[string]map[string]any)
	for from, items := range grouped {
		sort.SliceStable(items, func(i, j int) bool {
			return web3TransactionDetails(items[i]).Nonce < web3TransactionDetails(items[j]).Nonce
		})
		account, rpcErr := web3AccountState(ctx, provider, from, []json.RawMessage{json.RawMessage(strconv.Quote(from)), json.RawMessage(`"latest"`)}, 1)
		if rpcErr != nil {
			return nil, nil, rpcErr
		}
		expected := account.Nonce
		for _, tx := range items {
			details := web3TransactionDetails(tx)
			target := queued
			if details.Nonce == expected {
				target = pending
				expected++
			}
			if target[from] == nil {
				target[from] = make(map[string]any)
			}
			target[from][hexQuantity(details.Nonce)] = item(tx)
		}
	}
	return pending, queued, nil
}

func web3TxpoolSender(tx types.Tx) string {
	details := web3TransactionDetails(tx)
	if raw, ok := details.From.(string); ok && raw != "" {
		return raw
	}
	return "0x0000000000000000000000000000000000000000"
}

func web3TxpoolBucketCount(bucket map[string]map[string]any) int {
	count := 0
	for _, byNonce := range bucket {
		count += len(byNonce)
	}
	return count
}

func web3TxpoolBucketForAddress(bucket map[string]map[string]any, address string) map[string]any {
	for from, byNonce := range bucket {
		if strings.EqualFold(from, address) {
			return byNonce
		}
	}
	return map[string]any{}
}

func web3PendingTransactions(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	transactions := make([]any, 0, len(txs))
	for _, tx := range txs {
		transactions = append(transactions, web3PendingTransaction(tx))
	}
	return transactions, nil
}

func web3PendingTxs(ctx context.Context, provider StatusProvider) ([]types.Tx, *JSONRPCError) {
	pendingProvider, ok := provider.(PendingTxsProvider)
	if !ok {
		hashProvider, ok := provider.(PendingTxProvider)
		if !ok {
			return nil, &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
		}
		hashes, err := hashProvider.PendingTxHashes(ctx)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		txs := make([]types.Tx, 0, len(hashes))
		for _, hash := range hashes {
			txs = append(txs, types.Tx("hash:"+web3HashString(hash)))
		}
		return txs, nil
	}
	txs, err := pendingProvider.PendingTxs(ctx)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return txs, nil
}

func web3PendingTransaction(tx types.Tx) any {
	details := web3TransactionDetails(tx)
	transaction := map[string]any{
		"hash":             web3TxHash(tx),
		"nonce":            hexQuantity(details.Nonce),
		"blockHash":        nil,
		"blockNumber":      nil,
		"transactionIndex": nil,
		"from":             details.From,
		"to":               details.To,
		"value":            web3TxValueHex(details),
		"gas":              hexQuantity(details.Gas),
		"gasPrice":         web3TxGasPriceHex(details),
		"input":            details.Input,
		"type":             hexQuantity(details.Type),
		"chainId":          hexQuantity(details.ChainID),
		"r":                details.R,
		"s":                details.S,
		"v":                details.V,
	}
	if details.MaxFeePerGas > 0 || details.MaxFeePerGasHex != "" {
		transaction["maxFeePerGas"] = web3TxMaxFeePerGasHex(details)
	}
	if details.MaxPriorityFeePerGas > 0 || details.MaxPriorityFeePerGasHex != "" {
		transaction["maxPriorityFeePerGas"] = web3TxMaxPriorityFeePerGasHex(details)
	}
	return transaction
}

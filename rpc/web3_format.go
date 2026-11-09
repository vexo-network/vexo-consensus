package rpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"strconv"
	"strings"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	evmmodule "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func web3TransactionFromReceipt(ctx context.Context, provider StatusProvider, value []byte) (any, *JSONRPCError) {
	var receipt web3Receipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt response"}
	}
	if receipt.TxHash == "" {
		return nil, &JSONRPCError{Code: -32000, Message: "missing EVM receipt hash"}
	}
	blockHash, txIndex, tx, foundTx := web3ReceiptBlockLocation(ctx, provider, receipt)
	details := web3TxDetails{Nonce: 0, Gas: receipt.GasUsed, GasPrice: 0, Value: 0, Input: receipt.Output}
	if foundTx {
		details = web3TransactionDetails(tx)
	}
	var to any
	if receipt.To != "" {
		to = receipt.To
	}
	if foundTx {
		to = details.To
	}
	if receipt.ContractAddress != "" {
		to = nil
	}
	return map[string]any{
		"hash":             receipt.TxHash,
		"nonce":            hexQuantity(details.Nonce),
		"blockHash":        blockHash,
		"blockNumber":      hexQuantity(receipt.Height),
		"transactionIndex": hexQuantity(txIndex),
		"from":             receipt.From,
		"to":               to,
		"value":            web3TxValueHex(details),
		"gas":              hexQuantity(details.Gas),
		"gasPrice":         web3TxGasPriceHex(details),
		"input":            details.Input,
		"type":             hexQuantity(details.Type),
		"chainId":          hexQuantity(details.ChainID),
		"r":                details.R,
		"s":                details.S,
		"v":                details.V,
	}, nil
}

func web3ReceiptObject(ctx context.Context, provider StatusProvider, value []byte) (any, *JSONRPCError) {
	var receipt web3Receipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt response"}
	}
	if receipt.TxHash == "" {
		return nil, &JSONRPCError{Code: -32000, Message: "missing EVM receipt hash"}
	}
	blockHash, txIndex, _, foundLocation := web3ReceiptBlockLocation(ctx, provider, receipt)
	cumulativeGasUsed := web3CumulativeGasUsed(ctx, provider, receipt, txIndex)
	to := any(receipt.To)
	if receipt.To == "" {
		to = nil
	}
	contractAddress := any(nil)
	if receipt.ContractAddress != "" {
		contractAddress = receipt.ContractAddress
	}
	logs := make([]any, 0, len(receipt.Logs))
	logIndexBase := web3ReceiptLogIndexBase(ctx, provider, receipt, txIndex)
	for index, log := range receipt.Logs {
		location := web3LogLocation{
			BlockNumber:     receipt.Height,
			TransactionHash: receipt.TxHash,
			LogIndex:        logIndexBase + uint64(index),
		}
		if foundLocation {
			location.Mined = true
			location.BlockHash, _ = blockHash.(string)
			location.TransactionIndex = txIndex
		}
		logs = append(logs, web3NormalizeLogAt(log, location))
	}
	return map[string]any{
		"transactionHash":   receipt.TxHash,
		"transactionIndex":  hexQuantity(txIndex),
		"blockHash":         blockHash,
		"blockNumber":       hexQuantity(receipt.Height),
		"from":              receipt.From,
		"to":                to,
		"cumulativeGasUsed": hexQuantity(cumulativeGasUsed),
		"gasUsed":           hexQuantity(receipt.GasUsed),
		"contractAddress":   contractAddress,
		"logs":              logs,
		"logsBloom":         web3ReceiptBloom(ctx, provider, receipt),
		"status":            hexQuantity(uint64(receipt.Status)),
		"effectiveGasPrice": web3EffectiveGasPriceHexFromReceipt(ctx, provider, receipt),
		"type":              hexQuantity(web3TransactionTypeFromReceipt(ctx, provider, receipt)),
	}, nil
}

func web3TransactionFromBlockRecord(record store.BlockRecord, index int, hashText string, tx types.Tx) any {
	details := web3TransactionDetails(tx)
	transaction := map[string]any{
		"hash":             hashText,
		"nonce":            hexQuantity(details.Nonce),
		"blockHash":        "0x" + hex.EncodeToString(record.Hash[:]),
		"blockNumber":      hexQuantity(uint64(record.Block.Header.Height)),
		"transactionIndex": hexQuantity(uint64(index)),
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
	if details.BlobGasFeeCap > 0 || details.BlobGasFeeCapHex != "" {
		transaction["maxFeePerBlobGas"] = web3TxBlobGasFeeCapHex(details)
	}
	if len(details.BlobHashes) > 0 {
		transaction["blobVersionedHashes"] = append([]string(nil), details.BlobHashes...)
	}
	if index >= len(record.TxResults) {
		return transaction
	}
	receipt, ok := web3ReceiptFromResult(record.TxResults[index])
	if !ok {
		return transaction
	}
	to := details.To
	if receipt.To != "" {
		to = receipt.To
	}
	if receipt.ContractAddress != "" {
		to = nil
	}
	transaction["from"] = receipt.From
	transaction["to"] = to
	return transaction
}

func web3EffectiveGasPrice(tx types.Tx) uint64 {
	price, found := web3EffectiveGasPriceBig(tx)
	if !found || !price.IsUint64() {
		return 0
	}
	return price.Uint64()
}

func web3EffectiveGasPriceBig(tx types.Tx) (*big.Int, bool) {
	if gasPrice, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagGasPrice); found {
		return gasPrice, true
	}
	meta := vexoapp.ParseTxMeta(tx)
	if meta.FeeBig == nil || meta.FeeBig.Sign() == 0 || meta.Gas == 0 {
		return nil, false
	}
	return new(big.Int).Div(new(big.Int).Set(meta.FeeBig), new(big.Int).SetUint64(meta.Gas)), true
}

func web3EffectiveGasPriceHex(tx types.Tx) string {
	price, found := web3EffectiveGasPriceBig(tx)
	if !found {
		return hexQuantity(0)
	}
	return hexQuantityBig(price)
}

type web3TxDetails struct {
	From                    any
	To                      any
	Input                   string
	Nonce                   uint64
	Gas                     uint64
	GasPrice                uint64
	GasPriceHex             string
	MaxFeePerGas            uint64
	MaxFeePerGasHex         string
	MaxPriorityFeePerGas    uint64
	MaxPriorityFeePerGasHex string
	BlobGasFeeCap           uint64
	BlobGasFeeCapHex        string
	BlobHashes              []string
	Value                   uint64
	ValueHex                string
	Type                    uint64
	ChainID                 uint64
	R                       string `json:"r"`
	S                       string `json:"s"`
	V                       string `json:"v"`
}

func web3TransactionDetails(tx types.Tx) web3TxDetails {
	meta := vexoapp.ParseTxMeta(tx)
	details := web3TxDetails{
		From:        nil,
		To:          nil,
		Input:       "0x" + hex.EncodeToString(tx),
		Nonce:       meta.Nonce,
		Gas:         meta.Gas,
		GasPrice:    web3EffectiveGasPrice(tx),
		GasPriceHex: web3EffectiveGasPriceHex(tx),
		Type:        0,
	}
	if meta.Signer != "" {
		details.From = string(meta.Signer)
	}
	details.R, details.S, details.V = web3TransactionSignature(tx)
	if chainID, found := vexoapp.TxUintTag(tx, ethcompat.TagChainID); found {
		details.ChainID = chainID
	}
	if txType, found := vexoapp.TxUintTag(tx, ethcompat.TagType); found {
		details.Type = txType
	}
	if maxFee, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagMaxFeePerGas); found {
		setWeb3TxQuantity(&details.MaxFeePerGas, &details.MaxFeePerGasHex, maxFee)
	}
	if maxPriority, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagMaxPriorityFeePerGas); found {
		setWeb3TxQuantity(&details.MaxPriorityFeePerGas, &details.MaxPriorityFeePerGasHex, maxPriority)
	}
	if blobGasFeeCap, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagBlobGasFeeCap); found {
		setWeb3TxQuantity(&details.BlobGasFeeCap, &details.BlobGasFeeCapHex, blobGasFeeCap)
	}
	details.BlobHashes = web3BlobVersionedHashes(tx)
	if value, found := vexoapp.TxTag(tx, ethcompat.TagValue); found {
		setWeb3TxValue(&details, value)
	}
	if input, found := vexoapp.TxTag(tx, ethcompat.TagInput); found && input != "" {
		details.Input = "0x" + strings.TrimPrefix(input, "0x")
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		return details
	}
	switch canonical.Action {
	case "call":
		if len(canonical.Args) >= 7 {
			details.To = canonical.Args[2]
			if details.Input == "" || details.Input == "0x" {
				details.Input = "0x" + strings.TrimPrefix(canonical.Args[4], "0x")
			}
			setWeb3TxValue(&details, canonical.Args[6])
		}
	case "deploy", "eth_deploy":
		if len(canonical.Args) >= 5 {
			details.To = nil
			if details.Input == "" || details.Input == "0x" {
				details.Input = "0x" + strings.TrimPrefix(canonical.Args[2], "0x")
			}
			setWeb3TxValue(&details, canonical.Args[4])
		}
	}
	return details
}

func setWeb3TxValue(details *web3TxDetails, decimal string) {
	value, ok := new(big.Int).SetString(decimal, 10)
	if !ok || value.Sign() < 0 {
		return
	}
	details.ValueHex = hexQuantityBig(value)
	if value.IsUint64() {
		details.Value = value.Uint64()
	}
}

func web3TransactionSignature(tx types.Tx) (string, string, string) {
	rawHex, found := vexoapp.TxTag(tx, ethcompat.TagRaw)
	if !found || rawHex == "" {
		return "0x0", "0x0", "0x0"
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(rawHex, "0x"))
	if err != nil || len(raw) == 0 {
		return "0x0", "0x0", "0x0"
	}
	var ethTx gethtypes.Transaction
	if err := ethTx.UnmarshalBinary(raw); err != nil {
		return "0x0", "0x0", "0x0"
	}
	v, r, s := ethTx.RawSignatureValues()
	return hexQuantityBig(r), hexQuantityBig(s), hexQuantityBig(v)
}

func setWeb3TxQuantity(compat *uint64, hexValue *string, value *big.Int) {
	if value == nil || value.Sign() < 0 {
		return
	}
	*hexValue = hexQuantityBig(value)
	if value.IsUint64() {
		*compat = value.Uint64()
	}
}

func web3TxValueHex(details web3TxDetails) string {
	if details.ValueHex != "" {
		return details.ValueHex
	}
	return hexQuantity(details.Value)
}

func web3TxGasPriceHex(details web3TxDetails) string {
	if details.GasPriceHex != "" {
		return details.GasPriceHex
	}
	return hexQuantity(details.GasPrice)
}

func web3TxMaxFeePerGasHex(details web3TxDetails) string {
	if details.MaxFeePerGasHex != "" {
		return details.MaxFeePerGasHex
	}
	return hexQuantity(details.MaxFeePerGas)
}

func web3TxMaxPriorityFeePerGasHex(details web3TxDetails) string {
	if details.MaxPriorityFeePerGasHex != "" {
		return details.MaxPriorityFeePerGasHex
	}
	return hexQuantity(details.MaxPriorityFeePerGas)
}

func web3TxBlobGasFeeCapHex(details web3TxDetails) string {
	if details.BlobGasFeeCapHex != "" {
		return details.BlobGasFeeCapHex
	}
	return hexQuantity(details.BlobGasFeeCap)
}

func web3TxValueDecimal(details web3TxDetails) string {
	if details.ValueHex == "" {
		return strconv.FormatUint(details.Value, 10)
	}
	value, ok := new(big.Int).SetString(strings.TrimPrefix(details.ValueHex, "0x"), 16)
	if !ok {
		return "0"
	}
	return value.String()
}

func web3BlobVersionedHashes(tx types.Tx) []string {
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
	normalized := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		if !strings.HasPrefix(hash, "0x") && !strings.HasPrefix(hash, "0X") {
			hash = "0x" + hash
		}
		normalized = append(normalized, strings.ToLower(hash))
	}
	return normalized
}

func web3AccountBalanceHex(account web3AccountStateResponse) string {
	if account.BalanceHex != "" {
		return account.BalanceHex
	}
	return hexQuantity(account.Balance)
}

func web3CallValueHex(call web3CallRequest) string {
	if call.ValueHex != "" {
		return call.ValueHex
	}
	return hexQuantity(call.Value)
}

// web3TxHash is the hash the EVM module stores a transaction's receipt under,
// so a hash the RPC reports always finds the receipt it names.
func web3TxHash(tx types.Tx) string {
	return evmmodule.TxHash(tx)
}

func web3ReceiptFromResult(result types.Result) (web3Receipt, bool) {
	if len(result.Data) == 0 {
		return web3Receipt{}, false
	}
	var receipt web3Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil || receipt.TxHash == "" {
		return web3Receipt{}, false
	}
	return receipt, true
}

func web3EffectiveGasPriceFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) uint64 {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return 0
	}
	return web3EffectiveGasPrice(tx)
}

func web3EffectiveGasPriceHexFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) string {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return hexQuantity(0)
	}
	return web3EffectiveGasPriceHex(tx)
}

func web3TransactionTypeFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) uint64 {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return 0
	}
	if txType, ok := vexoapp.TxUintTag(tx, ethcompat.TagType); ok {
		return txType
	}
	return 0
}

func web3CumulativeGasUsed(ctx context.Context, provider StatusProvider, receipt web3Receipt, txIndex uint64) uint64 {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 {
		return receipt.GasUsed
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return receipt.GasUsed
	}
	if txIndex >= uint64(len(record.TxResults)) {
		return receipt.GasUsed
	}
	var cumulative uint64
	for index := uint64(0); index <= txIndex; index++ {
		gasUsed := record.TxResults[index].GasUsed
		if parsed, ok := web3ReceiptFromResult(record.TxResults[index]); ok {
			gasUsed = parsed.GasUsed
		}
		if ^uint64(0)-cumulative < gasUsed {
			return ^uint64(0)
		}
		cumulative += gasUsed
	}
	if cumulative == 0 {
		return receipt.GasUsed
	}
	return cumulative
}

func web3ReceiptLogIndexBase(ctx context.Context, provider StatusProvider, receipt web3Receipt, txIndex uint64) uint64 {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 || txIndex == 0 {
		return 0
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return 0
	}
	limit := txIndex
	if limit > uint64(len(record.TxResults)) {
		limit = uint64(len(record.TxResults))
	}
	var base uint64
	for index := uint64(0); index < limit; index++ {
		previous, ok := web3ReceiptFromResult(record.TxResults[index])
		if !ok {
			continue
		}
		base += uint64(len(previous.Logs))
	}
	return base
}

func web3ReceiptBlockLocation(ctx context.Context, provider StatusProvider, receipt web3Receipt) (any, uint64, types.Tx, bool) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 {
		return nil, 0, nil, false
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return nil, 0, nil, false
	}
	if index, found, _ := web3ReceiptIndexByHash(ctx, provider, receipt.TxHash); found && index.Height == receipt.Height && index.TxIndex < uint64(len(record.Block.Txs)) {
		tx := record.Block.Txs[index.TxIndex]
		if web3TxMatchesHash(tx, receipt.TxHash) || strings.EqualFold(index.TxHash, receipt.TxHash) {
			return "0x" + hex.EncodeToString(record.Hash[:]), index.TxIndex, append(types.Tx(nil), tx...), true
		}
	}
	for index, tx := range record.Block.Txs {
		if web3TxHash(tx) == receipt.TxHash {
			return "0x" + hex.EncodeToString(record.Hash[:]), uint64(index), append(types.Tx(nil), tx...), true
		}
	}
	if record.Hash != (types.Hash{}) {
		return "0x" + hex.EncodeToString(record.Hash[:]), 0, nil, false
	}
	return nil, 0, nil, false
}

func web3ReceiptsRoot(txs []types.Tx, results []types.Result) string {
	if root, ok := ethcompat.ReceiptRoot(txs, results); ok {
		return root
	}
	hasher := sha256.New()
	for _, result := range results {
		code := make([]byte, 4)
		code[0] = byte(result.Code >> 24)
		code[1] = byte(result.Code >> 16)
		code[2] = byte(result.Code >> 8)
		code[3] = byte(result.Code)
		_, _ = hasher.Write(code)
		_, _ = hasher.Write(result.Data)
		gas := strconv.FormatUint(result.GasUsed, 10)
		_, _ = hasher.Write([]byte(gas))
		_, _ = hasher.Write([]byte{0})
	}
	sum := hasher.Sum(nil)
	return "0x" + hex.EncodeToString(sum)
}

func web3LogsBloom(txs []types.Tx, results []types.Result) string {
	if bloom, ok := ethcompat.LogsBloom(txs, results); ok {
		return bloom
	}
	return "0x" + strings.Repeat("00", 256)
}

func web3ReceiptBloom(ctx context.Context, provider StatusProvider, receipt web3Receipt) string {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 {
		return "0x" + strings.Repeat("00", 256)
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return "0x" + strings.Repeat("00", 256)
	}
	for index, tx := range record.Block.Txs {
		if index >= len(record.TxResults) {
			break
		}
		if web3TxHash(tx) != receipt.TxHash {
			continue
		}
		if bloom, ok := ethcompat.ReceiptBloom(tx, record.TxResults[index]); ok {
			return bloom
		}
	}
	return "0x" + strings.Repeat("00", 256)
}

func web3BlockGasUsed(results []types.Result) uint64 {
	var total uint64
	for _, result := range results {
		if total > ^uint64(0)-result.GasUsed {
			return ^uint64(0)
		}
		total += result.GasUsed
	}
	return total
}

func web3BlockGasLimit(results []types.Result) string {
	return hexQuantity(web3BlockGasLimitValue(results))
}

func web3BlockGasLimitValue(results []types.Result) uint64 {
	used := web3BlockGasUsed(results)
	if used > defaultWeb3BlockGasLimit {
		return used
	}
	return defaultWeb3BlockGasLimit
}

func web3BlockGasUsedRatio(ctx context.Context, provider StatusProvider, height types.Height) float64 {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || height == 0 {
		return 0
	}
	record, err := blockProvider.BlockByHeight(ctx, height)
	if err != nil {
		return 0
	}
	if web3BlockGasUsed(record.TxResults) == 0 {
		return 0
	}
	gasLimit := web3BlockGasLimitValue(record.TxResults)
	if gasLimit == 0 {
		return 0
	}
	return float64(web3BlockGasUsed(record.TxResults)) / float64(gasLimit)
}

func web3BlockRecordParam(ctx context.Context, provider StatusProvider, raw json.RawMessage) (store.BlockRecord, *JSONRPCError) {
	text, err := jsonRPCStringParam(raw)
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	if strings.HasPrefix(text, "0x") && len(text) == 66 {
		hash, err := parseHexHash(text)
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
	height, rpcErr := web3BlockHeightParam(ctx, provider, raw)
	if rpcErr != nil {
		return store.BlockRecord{}, rpcErr
	}
	record, err := blockProvider.BlockByHeight(ctx, height)
	if errors.Is(err, store.ErrBlockNotFound) {
		return store.BlockRecord{}, nil
	}
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return record, nil
}

func web3BlockHeightParam(ctx context.Context, provider StatusProvider, raw json.RawMessage) (types.Height, *JSONRPCError) {
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{' {
		var selector struct {
			BlockNumber      string `json:"blockNumber"`
			BlockHash        string `json:"blockHash"`
			RequireCanonical bool   `json:"requireCanonical"`
		}
		if err := json.Unmarshal(raw, &selector); err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid block selector"}
		}
		if selector.BlockNumber != "" {
			encoded, _ := json.Marshal(selector.BlockNumber)
			return web3BlockHeightParam(ctx, provider, encoded)
		}
		if selector.BlockHash == "" {
			return 0, &JSONRPCError{Code: -32602, Message: "block selector requires blockHash or blockNumber"}
		}
		blockProvider, ok := provider.(BlockProvider)
		if !ok {
			return 0, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
		}
		hash, err := parseHexHash(selector.BlockHash)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid block hash"}
		}
		record, err := blockProvider.BlockByHash(ctx, hash)
		if errors.Is(err, store.ErrBlockNotFound) {
			return 0, &JSONRPCError{Code: -32000, Message: "block not found"}
		}
		if err != nil {
			return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return record.Block.Header.Height, nil
	}
	text, err := jsonRPCStringParam(raw)
	if err != nil {
		return 0, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	switch text {
	case "latest", "pending":
		return provider.Status(ctx).LatestHeight, nil
	case "safe", "finalized":
		return web3FinalizedHeight(ctx, provider)
	case "earliest":
		query, ok := provider.(ChainQueryProvider)
		if !ok {
			return 0, &JSONRPCError{Code: -32000, Message: "block index query is unavailable"}
		}
		index, err := query.BlockIndex(ctx)
		if err != nil {
			return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return index.EarliestHeight, nil
	default:
		height, err := parseHexQuantity(text)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid block tag"}
		}
		return types.Height(height), nil
	}
}

func web3FinalizedHeight(ctx context.Context, provider StatusProvider) (types.Height, *JSONRPCError) {
	status := provider.Status(ctx)
	if status.LatestFinalizedHeight > 0 {
		return status.LatestFinalizedHeight, nil
	}
	if finalityProvider, ok := provider.(FinalityProvider); ok {
		proof, err := finalityProvider.LatestFinalityProof(ctx)
		if err == nil && proof.Header.Height > 0 {
			return proof.Header.Height, nil
		}
		if err != nil && !errors.Is(err, node.ErrFinalityNotFound) && !errors.Is(err, store.ErrBlockNotFound) && !errors.Is(err, store.ErrBlockIndexNotFound) {
			return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
	}
	return 0, &JSONRPCError{Code: -32000, Message: "finalized block is unavailable"}
}

func web3QuantityParam(raw json.RawMessage) (uint64, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseHexQuantity(text)
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, nil
	}
	return 0, fmt.Errorf("quantity parameter is required")
}

func web3RewardPercentiles(raw json.RawMessage) ([]float64, error) {
	var percentiles []float64
	if err := json.Unmarshal(raw, &percentiles); err != nil {
		return nil, fmt.Errorf("invalid reward percentiles")
	}
	for _, percentile := range percentiles {
		if percentile < 0 || percentile > 100 {
			return nil, fmt.Errorf("reward percentile out of range")
		}
	}
	return percentiles, nil
}

func evmCallParam(params []json.RawMessage) (web3CallRequest, *JSONRPCError) {
	if len(params) == 0 {
		return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "call object is required"}
	}
	var payload web3TransactionCall
	if err := json.Unmarshal(params[0], &payload); err != nil {
		return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid call object"}
	}
	if payload.VM == "" {
		payload.VM = "evm"
	}
	if payload.Method == "" {
		payload.Method = "call"
	}
	if payload.From == "" {
		payload.From = "0x0000000000000000000000000000000000000000"
	}
	if payload.To == "" {
		payload.Method = "deploy"
		payload.To = "0x0000000000000000000000000000000000000000"
	}
	gasLimit := uint64(0)
	if payload.Gas != "" {
		value, err := parseHexQuantity(payload.Gas)
		if err != nil {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid gas quantity"}
		}
		gasLimit = value
	}
	callValue := uint64(0)
	callValueHex := ""
	if payload.Value != "" {
		value, err := parseHexQuantityBig(payload.Value)
		if err != nil {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid value quantity"}
		}
		callValueHex = hexQuantityBig(value)
		if value.IsUint64() {
			callValue = value.Uint64()
		}
	}
	gasPrice := uint64(0)
	gasPriceHex := ""
	maxFeePerGas := uint64(0)
	maxFeePerGasHex := ""
	maxPriorityFeePerGas := uint64(0)
	maxPriorityFeePerGasHex := ""
	nonce := uint64(0)
	switch {
	case payload.GasPrice != "":
		value, err := parseHexQuantityBig(payload.GasPrice)
		if err != nil || value.BitLen() > 256 {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid gasPrice quantity"}
		}
		gasPriceHex = hexQuantityBig(value)
		if value.IsUint64() {
			gasPrice = value.Uint64()
		}
	case payload.MaxFeePerGas != "":
		value, err := parseHexQuantityBig(payload.MaxFeePerGas)
		if err != nil || value.BitLen() > 256 {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid maxFeePerGas quantity"}
		}
		maxFeePerGasHex = hexQuantityBig(value)
		if value.IsUint64() {
			maxFeePerGas = value.Uint64()
		}
		if payload.MaxPriorityFeePerGas != "" {
			priority, err := parseHexQuantityBig(payload.MaxPriorityFeePerGas)
			if err != nil || priority.BitLen() > 256 {
				return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid maxPriorityFeePerGas quantity"}
			}
			maxPriorityFeePerGasHex = hexQuantityBig(priority)
			if priority.IsUint64() {
				maxPriorityFeePerGas = priority.Uint64()
			}
		}
	}
	if payload.Nonce != "" {
		value, err := parseHexQuantity(payload.Nonce)
		if err != nil {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid nonce quantity"}
		}
		nonce = value
	}
	if payload.Data == "" {
		payload.Data = "0x"
	}
	if _, err := hexBytes(payload.Data); err != nil {
		return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid data hex"}
	}
	setCodeAuthorizationsJSON := ""
	if raw := bytes.TrimSpace(payload.AuthorizationList); len(raw) > 0 && string(raw) != "null" {
		if !json.Valid(raw) || raw[0] != '[' {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid authorizationList"}
		}
		setCodeAuthorizationsJSON = string(raw)
	}
	return web3CallRequest{
		VM:                        payload.VM,
		From:                      payload.From,
		To:                        payload.To,
		Method:                    payload.Method,
		Input:                     payload.Data,
		GasLimit:                  gasLimit,
		Value:                     callValue,
		ValueHex:                  callValueHex,
		GasPrice:                  gasPrice,
		GasPriceHex:               gasPriceHex,
		MaxFeePerGas:              maxFeePerGas,
		MaxFeePerGasHex:           maxFeePerGasHex,
		MaxPriorityFeePerGas:      maxPriorityFeePerGas,
		MaxPriorityFeePerGasHex:   maxPriorityFeePerGasHex,
		Nonce:                     nonce,
		AccessList:                web3ContractAccessList(payload.AccessList),
		SetCodeAuthorizationsJSON: setCodeAuthorizationsJSON,
	}, nil
}

func web3ContractAccessList(entries []web3AccessListEntry) []contract.AccessListEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]contract.AccessListEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, contract.AccessListEntry{
			Address:     types.Address(entry.Address),
			StorageKeys: append([]string(nil), entry.StorageKeys...),
		})
	}
	return out
}

func web3LogFilterParam(ctx context.Context, provider StatusProvider, params []json.RawMessage) (web3Filter, *JSONRPCError) {
	if len(params) != 1 {
		return web3Filter{}, &JSONRPCError{Code: -32602, Message: "filter object is required"}
	}
	var payload struct {
		Address   any             `json:"address"`
		FromBlock string          `json:"fromBlock"`
		ToBlock   string          `json:"toBlock"`
		Topics    json.RawMessage `json:"topics"`
	}
	if err := json.Unmarshal(params[0], &payload); err != nil {
		return web3Filter{}, &JSONRPCError{Code: -32602, Message: "invalid filter object"}
	}
	addresses, rpcErr := web3LogAddresses(payload.Address)
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	latest := uint64(provider.Status(ctx).LatestHeight)
	// As in Ethereum, an omitted fromBlock is the latest block, not genesis.
	fromBlock, rpcErr := web3LogBlockBound(ctx, provider, payload.FromBlock, latest, latest)
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	toBlock, rpcErr := web3LogBlockBound(ctx, provider, payload.ToBlock, latest, ^uint64(0))
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	topics, rpcErr := web3LogTopics(payload.Topics)
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	return web3Filter{Type: "log", Addresses: addresses, FromBlock: fromBlock, ToBlock: toBlock, Topics: topics}, nil
}

func web3LogAddresses(value any) ([]string, *JSONRPCError) {
	switch address := value.(type) {
	case nil:
		return nil, nil
	case string:
		if address == "" {
			return nil, &JSONRPCError{Code: -32602, Message: "address is required"}
		}
		return []string{address}, nil
	case []any:
		if len(address) == 0 {
			return nil, nil
		}
		// Every address is a scan of the whole block range.
		if len(address) > maxWeb3LogAddresses {
			return nil, &JSONRPCError{Code: -32602, Message: fmt.Sprintf("a log filter accepts at most %d addresses", maxWeb3LogAddresses)}
		}
		addresses := make([]string, 0, len(address))
		seen := make(map[string]struct{}, len(address))
		for _, item := range address {
			text, ok := item.(string)
			if !ok || text == "" {
				return nil, &JSONRPCError{Code: -32602, Message: "address is required"}
			}
			key := strings.ToLower(text)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			addresses = append(addresses, text)
		}
		return addresses, nil
	default:
		return nil, &JSONRPCError{Code: -32602, Message: "address is required"}
	}
}

func web3LogBlockBound(ctx context.Context, provider StatusProvider, tag string, latest uint64, empty uint64) (uint64, *JSONRPCError) {
	if tag == "" {
		return empty, nil
	}
	switch tag {
	case "latest", "pending":
		return latest, nil
	case "safe", "finalized":
		height, rpcErr := web3FinalizedHeight(ctx, provider)
		if rpcErr != nil {
			return 0, rpcErr
		}
		return uint64(height), nil
	case "earliest":
		return 0, nil
	default:
		height, err := parseHexQuantity(tag)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid log block tag"}
		}
		return height, nil
	}
}

func web3LogTopics(raw json.RawMessage) ([][]string, *JSONRPCError) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid log topics"}
	}
	if len(values) > evmmodule.MaxLogTopics {
		return nil, &JSONRPCError{Code: -32602, Message: fmt.Sprintf("a log filter has at most %d topic positions", evmmodule.MaxLogTopics)}
	}
	topics := make([][]string, 0, len(values))
	for _, value := range values {
		switch item := value.(type) {
		case nil:
			topics = append(topics, nil)
		case string:
			topics = append(topics, []string{strings.ToLower(item)})
		case []any:
			if len(item) > evmmodule.MaxLogTopicOptions {
				return nil, &JSONRPCError{Code: -32602, Message: fmt.Sprintf("a log topic position accepts at most %d options", evmmodule.MaxLogTopicOptions)}
			}
			options := make([]string, 0, len(item))
			for _, option := range item {
				text, ok := option.(string)
				if !ok || text == "" {
					return nil, &JSONRPCError{Code: -32602, Message: "invalid log topic option"}
				}
				options = append(options, strings.ToLower(text))
			}
			topics = append(topics, options)
		default:
			return nil, &JSONRPCError{Code: -32602, Message: "invalid log topic"}
		}
	}
	return topics, nil
}

func jsonRPCStringParam(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("string parameter is required")
	}
	if value == "" {
		return "", fmt.Errorf("string parameter is empty")
	}
	return value, nil
}

func rawJSONObject(value []byte) (json.RawMessage, *JSONRPCError) {
	if !json.Valid(value) {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid application JSON"}
	}
	return append(json.RawMessage(nil), value...), nil
}

func hexBytes(value string) ([]byte, error) {
	value = strings.TrimPrefix(value, "0x")
	if len(value)%2 == 1 {
		value = "0" + value
	}
	return hex.DecodeString(value)
}

func normalizeHexDataString(value string) string {
	decoded, err := hexBytes(value)
	if err != nil {
		return value
	}
	return "0x" + hex.EncodeToString(decoded)
}

func normalizeHexQuantityString(value string) string {
	if strings.HasPrefix(value, "0x") {
		parsed, err := parseHexQuantityBig(value)
		if err != nil {
			return value
		}
		return "0x" + parsed.Text(16)
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return value
	}
	return "0x" + parsed.Text(16)
}

func normalizeStorageHex(value string) string {
	decoded, err := parseFixedWidthHex(value, 32)
	if err != nil {
		return value
	}
	if len(decoded) > 32 {
		return value
	}
	out := make([]byte, 32)
	copy(out[32-len(decoded):], decoded)
	return "0x" + hex.EncodeToString(out)
}

func parseHexQuantity(value string) (uint64, error) {
	if !strings.HasPrefix(value, "0x") {
		return 0, fmt.Errorf("quantity must use 0x prefix")
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return 0, fmt.Errorf("empty quantity")
	}
	return strconv.ParseUint(trimmed, 16, 64)
}

func parseHexQuantityBig(value string) (*big.Int, error) {
	if !strings.HasPrefix(value, "0x") {
		return nil, fmt.Errorf("quantity must use 0x prefix")
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return nil, fmt.Errorf("empty quantity")
	}
	parsed, ok := new(big.Int).SetString(trimmed, 16)
	if !ok || parsed.Sign() < 0 {
		return nil, fmt.Errorf("invalid quantity")
	}
	return parsed, nil
}

func parseHexAddress(value string) (types.Address, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "0x") {
		return "", fmt.Errorf("address must use 0x prefix")
	}
	clean := strings.TrimPrefix(trimmed, "0x")
	if len(clean) != 40 {
		return "", fmt.Errorf("address must be 20 bytes")
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) != 20 {
		return "", fmt.Errorf("invalid address")
	}
	return types.Address("0x" + strings.ToLower(clean)), nil
}

func parseHexHash(value string) (types.Hash, error) {
	var hash types.Hash
	decoded, err := hexBytes(value)
	if err != nil {
		return hash, err
	}
	if len(decoded) != len(hash) {
		return hash, fmt.Errorf("hash must be 32 bytes")
	}
	copy(hash[:], decoded)
	return hash, nil
}

func web3TransactionsRoot(txs []types.Tx) string {
	if root, ok := ethcompat.TransactionRoot(txs); ok {
		return root
	}
	hasher := sha256.New()
	for _, tx := range txs {
		_, _ = hasher.Write([]byte(web3TxHash(tx)))
	}
	return "0x" + hex.EncodeToString(hasher.Sum(nil))
}

func hexQuantity(value uint64) string {
	return "0x" + strconv.FormatUint(value, 16)
}

func hexQuantityBig(value *big.Int) string {
	if value == nil || value.Sign() == 0 {
		return "0x0"
	}
	return "0x" + value.Text(16)
}

func chainNumericID(chainID string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(chainID))
	value := hash.Sum64()
	if value == 0 {
		return 1
	}
	return value
}

func web3ChainID(status node.Status) uint64 {
	if status.EVMChainID != 0 {
		return status.EVMChainID
	}
	return chainNumericID(status.ChainID)
}

func web3Syncing(status node.Status) any {
	if !status.Running || status.LatestHeight == 0 {
		return false
	}
	finalized := status.LatestFinalizedHeight
	if finalized == 0 || finalized >= status.LatestHeight {
		return false
	}
	return map[string]any{
		"startingBlock": hexQuantity(uint64(finalized)),
		"currentBlock":  hexQuantity(uint64(finalized)),
		"highestBlock":  hexQuantity(uint64(status.LatestHeight)),
	}
}

func web3Mining(provider StatusProvider, ctx context.Context) bool {
	status := provider.Status(ctx)
	if !status.Running {
		return false
	}
	if controller, ok := provider.(ConsensusLoopController); ok {
		return controller.ConsensusLoopRunning()
	}
	return true
}

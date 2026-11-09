package evm

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func receiptFromResult(tx types.Tx, height types.Height, invocation contract.Invocation, contractAddress string, result contract.Result) Receipt {
	receipt := Receipt{
		TxHash:          TxHash(tx),
		Height:          uint64(height),
		Status:          1,
		From:            string(invocation.Caller),
		ContractAddress: contractAddress,
		VM:              invocation.VM,
		GasUsed:         result.GasUsed,
		Output:          "0x" + hex.EncodeToString(result.Output),
		Logs:            make([]Log, 0, len(result.Logs)),
		StateDiff:       stateDiffFromResult(result, types.Address(receiptTargetAddress(invocation, contractAddress))),
	}
	if result.Failed {
		receipt.Status = 0
		receipt.Error = result.Error
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
			TransactionHash: TxHash(tx),
			LogIndex:        uint64(index),
			Meta:            cloneMap(log.Meta),
		})
	}
	return receipt
}

func receiptTargetAddress(invocation contract.Invocation, contractAddress string) string {
	if contractAddress != "" {
		return string(contractAddress)
	}
	return string(invocation.Contract)
}

func stateDiffFromResult(result contract.Result, defaultAddress types.Address) any {
	diff := make(map[string]map[string]any)
	account := func(address types.Address) map[string]any {
		if address == "" {
			address = defaultAddress
		}
		if address == "" {
			return nil
		}
		key := canonicalAddressKey(address)
		item := diff[key]
		if item == nil {
			item = make(map[string]any)
			diff[key] = item
		}
		return item
	}
	for _, write := range result.BalanceWrites {
		entry := account(write.Address)
		if entry != nil {
			entry["balance"] = map[string]any{"to": balanceWriteHex(write)}
		}
	}
	for _, write := range result.NonceWrites {
		entry := account(write.Address)
		if entry != nil {
			entry["nonce"] = map[string]any{"to": hexQuantityLocal(write.Nonce)}
		}
	}
	for _, write := range result.CodeWrites {
		entry := account(write.Address)
		if entry == nil {
			continue
		}
		if write.Delete {
			entry["code"] = map[string]any{"delete": true}
			continue
		}
		entry["code"] = map[string]any{"to": "0x" + hex.EncodeToString(write.Code)}
	}
	for _, write := range result.StorageWrites {
		if write.Slot == "" {
			continue
		}
		entry := account(write.Address)
		if entry == nil {
			continue
		}
		storage, _ := entry["storage"].(map[string]any)
		if storage == nil {
			storage = make(map[string]any)
			entry["storage"] = storage
		}
		if write.Delete {
			storage[normalizeSlot(write.Slot)] = map[string]any{"delete": true}
			continue
		}
		storage[normalizeSlot(write.Slot)] = map[string]any{"to": "0x" + hex.EncodeToString(write.Value)}
	}
	for _, deletion := range result.AccountDeletions {
		entry := account(deletion.Address)
		if entry != nil {
			entry["delete"] = true
		}
	}
	if len(diff) == 0 {
		return nil
	}
	return diff
}

func hexQuantityLocal(value uint64) string {
	if value == 0 {
		return "0x0"
	}
	return "0x" + strconv.FormatUint(value, 16)
}

func balanceWriteHex(write contract.BalanceWrite) string {
	if write.BalanceBig != nil {
		return hexQuantityBigLocal(write.BalanceBig)
	}
	return hexQuantityLocal(write.Balance)
}

func hexQuantityBigLocal(value *big.Int) string {
	if value == nil || value.Sign() == 0 {
		return "0x0"
	}
	return "0x" + value.Text(16)
}

// receiptWrites returns the writes that store a receipt and its logs. A
// receipt is written once: a second transaction resolving to the same hash is
// rejected instead of replacing the first receipt.
func receiptWrites(ctx context.Context, store vexoapp.StateStore, receipt Receipt) ([]vexostore.KVWrite, error) {
	key := receiptKey(receipt.TxHash)
	if _, err := store.Get(ctx, ReceiptNamespace, key); err == nil {
		return nil, nil
	} else if !errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, err
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	writes := []vexostore.KVWrite{{Namespace: ReceiptNamespace, Key: key, Value: encoded}}
	for _, log := range receipt.Logs {
		if log.Address == "" {
			continue
		}
		encodedLog, err := json.Marshal(log)
		if err != nil {
			return nil, err
		}
		writes = append(writes,
			vexostore.KVWrite{Namespace: ReceiptNamespace, Key: globalLogKey(log), Value: encodedLog},
			vexostore.KVWrite{Namespace: ReceiptNamespace, Key: addressLogKey(log), Value: encodedLog},
		)
	}
	return writes, nil
}

func persistReceipt(ctx context.Context, store vexoapp.StateStore, receipt Receipt) error {
	writes, err := receiptWrites(ctx, store, receipt)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, writes)
}

func blobSidecarBundleFromTx(tx types.Tx) (ethcompat.BlobSidecarBundle, error) {
	return Module{policy: DefaultPolicy()}.blobSidecarBundleFromTx(tx)
}

func (module Module) blobSidecarBundleFromTx(tx types.Tx) (ethcompat.BlobSidecarBundle, error) {
	expectedHashes := txBlobHashStrings(tx)
	encoded, found := vexoapp.TxTag(tx, ethcompat.TagBlobSidecar)
	if !found || encoded == "" {
		if len(expectedHashes) > 0 {
			return ethcompat.BlobSidecarBundle{}, ethcompat.ErrInvalidBlobSidecar
		}
		return ethcompat.BlobSidecarBundle{}, nil
	}
	bundle, err := ethcompat.DecodeBlobSidecarBundle(encoded)
	if err != nil {
		return ethcompat.BlobSidecarBundle{}, err
	}
	if !sameBlobHashes(expectedHashes, bundle.BlobHashes) {
		return ethcompat.BlobSidecarBundle{}, ethcompat.ErrInvalidBlobSidecar
	}
	if err := module.validateBlobSidecarLimits(encoded, bundle); err != nil {
		return ethcompat.BlobSidecarBundle{}, err
	}
	return bundle, nil
}

func persistBlobSidecar(ctx context.Context, store vexoapp.StateStore, tx types.Tx) error {
	return Module{policy: DefaultPolicy()}.persistBlobSidecar(ctx, store, tx)
}

func (module Module) persistBlobSidecar(ctx context.Context, store vexoapp.StateStore, tx types.Tx) error {
	writes, err := module.blobSidecarWrites(tx)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, writes)
}

// blobSidecarWrites returns the writes that store the blob sidecar a
// transaction carries, or none when it carries no blobs.
func (module Module) blobSidecarWrites(tx types.Tx) ([]vexostore.KVWrite, error) {
	bundle, err := module.blobSidecarBundleFromTx(tx)
	if err != nil {
		return nil, err
	}
	if len(bundle.BlobHashes) == 0 {
		return nil, nil
	}
	record := BlobSidecarRecord{TxHash: TxHash(tx), Sidecar: bundle}
	encoded, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	writes := []vexostore.KVWrite{{
		Namespace: ReceiptNamespace,
		Key:       blobSidecarKey(record.TxHash),
		Value:     encoded,
	}}
	for _, blobHash := range bundle.BlobHashes {
		writes = append(writes, vexostore.KVWrite{
			Namespace: ReceiptNamespace,
			Key:       blobSidecarHashIndexKey(blobHash),
			Value:     []byte(record.TxHash),
		})
	}
	return writes, nil
}

func (module Module) validateBlobSidecarLimits(encoded string, bundle ethcompat.BlobSidecarBundle) error {
	policy := module.policy
	if policy.MaxBlobSidecarBlobs == 0 {
		policy.MaxBlobSidecarBlobs = DefaultPolicy().MaxBlobSidecarBlobs
	}
	if policy.MaxBlobSidecarBytes == 0 {
		policy.MaxBlobSidecarBytes = DefaultPolicy().MaxBlobSidecarBytes
	}
	if uint64(len(bundle.BlobHashes)) > policy.MaxBlobSidecarBlobs {
		return ethcompat.ErrInvalidBlobSidecar
	}
	if uint64(len(encoded)) > policy.MaxBlobSidecarBytes {
		return ethcompat.ErrInvalidBlobSidecar
	}
	return nil
}

func persistReceiptIndexes(ctx context.Context, store vexoapp.StateStore, results []types.Result) error {
	writes := make([]vexostore.KVWrite, 0)
	for index, result := range results {
		receipt, ok := receiptFromTxResult(result)
		if !ok || receipt.TxHash == "" || receipt.Height == 0 {
			continue
		}
		encoded, err := json.Marshal(ReceiptIndex{TxHash: receipt.TxHash, Height: receipt.Height, TxIndex: uint64(index)})
		if err != nil {
			return err
		}
		writes = append(writes, vexostore.KVWrite{
			Namespace: ReceiptNamespace,
			Key:       receiptIndexKey(receipt.TxHash),
			Value:     encoded,
		}, vexostore.KVWrite{
			Namespace: ReceiptNamespace,
			Key:       receiptHeightKey(receipt.Height, receipt.TxHash),
			Value:     []byte(receipt.TxHash),
		})
	}
	if len(writes) == 0 {
		return nil
	}
	if batchStore, ok := store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	return ErrAtomicStoreRequired
}

func receiptFromTxResult(result types.Result) (Receipt, bool) {
	if len(result.Data) == 0 {
		return Receipt{}, false
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil || receipt.TxHash == "" {
		return Receipt{}, false
	}
	return receipt, true
}

func queryJSON(ctx vexoapp.Context, key []byte) vexoapp.QueryResponse {
	value, err := ctx.Store.Get(ctx.GoContext(), ReceiptNamespace, key)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return vexoapp.QueryResponse{Code: 3, Log: "EVM state not found"}
	}
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: append([]byte(nil), value...)}
}

// queryLogs returns the logs of the blocks a LogsRequest covers that match its
// topics, reading one height prefix at a time. A request is held to the
// committed chain, to MaxLogBlockRange blocks and to its result limit, so no
// query reads more of the log index than that window or holds more logs than
// it may return, whatever the request asks for.
func queryLogs(ctx vexoapp.Context, data []byte, heightPrefix func(height uint64) []byte) vexoapp.QueryResponse {
	var request LogsRequest
	if len(data) > 0 {
		if err := json.Unmarshal(data, &request); err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
		}
	}
	topics, ok := normalizeLogTopics(request.Topics)
	if !ok {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	limit := request.Limit
	if limit == 0 || limit > MaxLogQueryResults+1 {
		limit = MaxLogQueryResults + 1
	}
	prefixStore, ok := ctx.Store.(vexostore.PrefixKVStore)
	if !ok {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStoreMissing.Error()}
	}
	latest := uint64(ctx.Height)
	toBlock := request.ToBlock
	if toBlock == 0 || toBlock > latest {
		toBlock = latest
	}
	if request.FromBlock > toBlock {
		return vexoapp.QueryResponse{Value: []byte("[]")}
	}
	if toBlock-request.FromBlock >= MaxLogBlockRange {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	logs := make([]Log, 0)
scan:
	for height := request.FromBlock; ; height++ {
		pairs, err := prefixStore.ExportPrefix(ctx.GoContext(), ReceiptNamespace, heightPrefix(height))
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		for _, pair := range pairs {
			var log Log
			if err := json.Unmarshal(pair.Value, &log); err != nil {
				return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
			}
			if !logMatchesTopics(log, topics) {
				continue
			}
			logs = append(logs, log)
			if uint64(len(logs)) >= limit {
				break scan
			}
		}
		if height == toBlock {
			break
		}
	}
	encoded, err := json.Marshal(logs)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

// normalizeLogTopics lowercases a topic filter and rejects one that is larger than a log can be
// matched against.
func normalizeLogTopics(topics [][]string) ([][]string, bool) {
	if len(topics) > MaxLogTopics {
		return nil, false
	}
	normalized := make([][]string, len(topics))
	for index, options := range topics {
		if len(options) > MaxLogTopicOptions {
			return nil, false
		}
		for _, option := range options {
			if option == "" {
				return nil, false
			}
			normalized[index] = append(normalized[index], strings.ToLower(option))
		}
	}
	return normalized, true
}

func logMatchesTopics(log Log, topics [][]string) bool {
	for index, accepted := range topics {
		if len(accepted) == 0 {
			continue
		}
		if index >= len(log.Topics) {
			return false
		}
		actual := strings.ToLower(log.Topics[index])
		found := false
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

// evmHistoryPruneWrites deletes the receipts, logs and blob sidecars of the blocks below
// retainFrom. Receipts are listed by height (receiptHeightKey), and the height pruning has reached
// is kept, so a prune visits only the receipts of the heights it prunes.
func evmHistoryPruneWrites(ctx context.Context, store vexoapp.StateStore, retainFrom uint64) ([]vexostore.KVWrite, error) {
	prefixStore, ok := store.(vexostore.PrefixKVStore)
	if !ok {
		return nil, nil
	}
	from := uint64(1)
	encoded, err := store.Get(ctx, ReceiptNamespace, receiptsPrunedBelowKey)
	switch {
	case errors.Is(err, vexostore.ErrKeyNotFound):
	case err != nil:
		return nil, err
	case len(encoded) != 8:
		return nil, fmt.Errorf("receipt prune height has %d bytes", len(encoded))
	default:
		from = binary.BigEndian.Uint64(encoded)
	}
	if from >= retainFrom {
		return nil, nil
	}
	writes := make([]vexostore.KVWrite, 0)
	for height := from; height < retainFrom; height++ {
		pairs, err := prefixStore.ExportPrefix(ctx, ReceiptNamespace, receiptHeightPrefix(height))
		if err != nil {
			return nil, err
		}
		for _, pair := range pairs {
			txHash := string(pair.Value)
			writes = append(writes, deleteWrite(ReceiptNamespace, append([]byte(nil), pair.Key...)))
			receiptWrites, err := receiptPruneWrites(ctx, store, txHash)
			if err != nil {
				return nil, err
			}
			writes = append(writes, receiptWrites...)
		}
	}
	writes = append(writes, vexostore.KVWrite{Namespace: ReceiptNamespace, Key: receiptsPrunedBelowKey, Value: binary.BigEndian.AppendUint64(nil, retainFrom)})
	return writes, nil
}

// receiptPruneWrites deletes the receipt of txHash with its index, logs and blob sidecar.
func receiptPruneWrites(ctx context.Context, store vexoapp.StateStore, txHash string) ([]vexostore.KVWrite, error) {
	receipt, err := loadReceiptForPrune(ctx, store, txHash)
	if err != nil {
		return nil, err
	}
	writes := []vexostore.KVWrite{
		deleteWrite(ReceiptNamespace, receiptIndexKey(txHash)),
		deleteWrite(ReceiptNamespace, receiptKey(txHash)),
	}
	for _, log := range receipt.Logs {
		writes = append(writes,
			deleteWrite(ReceiptNamespace, globalLogKey(log)),
			deleteWrite(ReceiptNamespace, addressLogKey(log)),
		)
	}
	blobRecord, found, err := loadBlobSidecarForPrune(ctx, store, txHash)
	if err != nil {
		return nil, err
	}
	if found {
		writes = append(writes, deleteWrite(ReceiptNamespace, blobSidecarKey(txHash)))
		for _, blobHash := range blobRecord.Sidecar.BlobHashes {
			writes = append(writes, deleteWrite(ReceiptNamespace, blobSidecarHashIndexKey(blobHash)))
		}
	}
	return writes, nil
}

func loadReceiptForPrune(ctx context.Context, store vexoapp.StateStore, txHash string) (Receipt, error) {
	value, err := store.Get(ctx, ReceiptNamespace, receiptKey(txHash))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return Receipt{TxHash: txHash}, nil
	}
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func loadBlobSidecarForPrune(ctx context.Context, store vexoapp.StateStore, txHash string) (BlobSidecarRecord, bool, error) {
	value, err := store.Get(ctx, ReceiptNamespace, blobSidecarKey(txHash))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return BlobSidecarRecord{}, false, nil
	}
	if err != nil {
		return BlobSidecarRecord{}, false, err
	}
	var record BlobSidecarRecord
	if err := json.Unmarshal(value, &record); err != nil {
		return BlobSidecarRecord{}, false, err
	}
	return record, true, nil
}

func applyKVWrites(ctx context.Context, store vexoapp.StateStore, writes []vexostore.KVWrite) error {
	if len(writes) == 0 {
		return nil
	}
	if batchStore, ok := store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	return ErrAtomicStoreRequired
}

func deleteWrite(namespace string, key []byte) vexostore.KVWrite {
	return vexostore.KVWrite{Namespace: namespace, Key: append([]byte(nil), key...), Delete: true}
}

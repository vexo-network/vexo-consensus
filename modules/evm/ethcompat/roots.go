package ethcompat

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

type receiptJSON struct {
	TxHash          string    `json:"tx_hash"`
	Status          uint64    `json:"status"`
	ContractAddress string    `json:"contract_address,omitempty"`
	GasUsed         uint64    `json:"gas_used"`
	Logs            []logJSON `json:"logs,omitempty"`
}

type logJSON struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics,omitempty"`
	Data            string   `json:"data,omitempty"`
	BlockNumber     uint64   `json:"block_number,omitempty"`
	TransactionHash string   `json:"transaction_hash,omitempty"`
	LogIndex        uint64   `json:"log_index,omitempty"`
}

func TransactionRoot(txs []types.Tx) (string, bool) {
	transactions := make(gethtypes.Transactions, 0, len(txs))
	for _, tx := range txs {
		ethTx, ok := rawTransactionFromCanonical(tx)
		if !ok {
			return "", false
		}
		transactions = append(transactions, ethTx)
	}
	root := gethtypes.DeriveSha(transactions, trie.NewStackTrie(nil))
	return root.Hex(), true
}

func ReceiptRoot(txs []types.Tx, results []types.Result) (string, bool) {
	if len(results) == 0 {
		root := gethtypes.DeriveSha(gethtypes.Receipts{}, trie.NewStackTrie(nil))
		return root.Hex(), true
	}
	if len(txs) < len(results) {
		return "", false
	}
	receipts := make(gethtypes.Receipts, 0, len(results))
	var cumulativeGas uint64
	for index, result := range results {
		receipt, ok := receiptFromResult(txs[index], result, cumulativeGas)
		if !ok {
			return "", false
		}
		cumulativeGas = receipt.CumulativeGasUsed
		receipts = append(receipts, receipt)
	}
	root := gethtypes.DeriveSha(receipts, trie.NewStackTrie(nil))
	return root.Hex(), true
}

func rawTransactionFromCanonical(tx types.Tx) (*gethtypes.Transaction, bool) {
	raw, found := vexoapp.TxTag(tx, TagRaw)
	if !found || raw == "" {
		if len(tx) == 0 {
			return nil, false
		}
		return nil, false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(normalizeHex(raw), "0x"))
	if err != nil || len(decoded) == 0 {
		return nil, false
	}
	var ethTx gethtypes.Transaction
	if err := ethTx.UnmarshalBinary(decoded); err != nil {
		return nil, false
	}
	return &ethTx, true
}

func receiptFromResult(tx types.Tx, result types.Result, previousCumulativeGas uint64) (*gethtypes.Receipt, bool) {
	if len(result.Data) == 0 {
		return nil, false
	}
	var payload receiptJSON
	if err := json.Unmarshal(result.Data, &payload); err != nil || payload.TxHash == "" {
		return nil, false
	}
	txHash, found := vexoapp.TxTag(tx, TagHash)
	if !found || !strings.EqualFold(txHash, payload.TxHash) {
		return nil, false
	}
	txType, _ := vexoapp.TxUintTag(tx, TagType)
	cumulativeGas := previousCumulativeGas + payload.GasUsed
	receipt := &gethtypes.Receipt{
		Type:              uint8(txType),
		Status:            payload.Status,
		CumulativeGasUsed: cumulativeGas,
		TxHash:            gethcommon.HexToHash(payload.TxHash),
		ContractAddress:   gethcommon.HexToAddress(payload.ContractAddress),
		GasUsed:           payload.GasUsed,
		Logs:              make([]*gethtypes.Log, 0, len(payload.Logs)),
	}
	for _, log := range payload.Logs {
		gethLog := &gethtypes.Log{
			Address:     gethcommon.HexToAddress(log.Address),
			Topics:      make([]gethcommon.Hash, 0, len(log.Topics)),
			Data:        hexData(log.Data),
			BlockNumber: log.BlockNumber,
			TxHash:      gethcommon.HexToHash(log.TransactionHash),
			Index:       uint(log.LogIndex),
		}
		for _, topic := range log.Topics {
			gethLog.Topics = append(gethLog.Topics, gethcommon.HexToHash(topic))
		}
		receipt.Logs = append(receipt.Logs, gethLog)
	}
	receipt.Bloom = gethtypes.CreateBloom(receipt)
	return receipt, true
}

func hexData(value string) []byte {
	clean := strings.TrimPrefix(value, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil {
		return nil
	}
	return decoded
}

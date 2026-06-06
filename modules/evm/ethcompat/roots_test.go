package ethcompat

import (
	"encoding/json"
	"math/big"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestTransactionRootMatchesGethDeriveSha(t *testing.T) {
	raw, _ := signedRawTestTx(t, 7, false)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	root, ok := TransactionRoot([]types.Tx{decoded.Tx})
	if !ok {
		t.Fatal("expected Ethereum transaction root")
	}
	ethTx, ok := rawTransactionFromCanonical(decoded.Tx)
	if !ok {
		t.Fatal("expected raw Ethereum transaction")
	}
	expected := gethtypes.DeriveSha(gethtypes.Transactions{ethTx}, trie.NewStackTrie(nil)).Hex()
	if root != expected {
		t.Fatalf("unexpected tx root: got %s want %s", root, expected)
	}
}

func TestReceiptRootMatchesGethDeriveSha(t *testing.T) {
	raw, _ := signedRawTestTx(t, 7, false)
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	receiptPayload := receiptJSON{
		TxHash:  decoded.Hash,
		Status:  1,
		GasUsed: 21_000,
		Logs: []logJSON{{
			Address:         "0x000000000000000000000000000000000000bEEF",
			Topics:          []string{"0x0000000000000000000000000000000000000000000000000000000000000001"},
			Data:            "0x1234",
			BlockNumber:     10,
			TransactionHash: decoded.Hash,
			LogIndex:        0,
		}},
	}
	encoded, err := json.Marshal(receiptPayload)
	if err != nil {
		t.Fatal(err)
	}
	root, ok := ReceiptRoot([]types.Tx{decoded.Tx}, []types.Result{{Data: encoded, GasUsed: 21_000}})
	if !ok {
		t.Fatal("expected Ethereum receipt root")
	}
	expectedReceipt := &gethtypes.Receipt{
		Type:              decoded.Type,
		Status:            1,
		CumulativeGasUsed: 21_000,
		TxHash:            gethcommon.HexToHash(decoded.Hash),
		GasUsed:           21_000,
		EffectiveGasPrice: new(big.Int).SetUint64(decoded.GasPrice),
		Logs: []*gethtypes.Log{{
			Address:     gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF"),
			Topics:      []gethcommon.Hash{gethcommon.HexToHash("0x1")},
			Data:        []byte{0x12, 0x34},
			BlockNumber: 10,
			TxHash:      gethcommon.HexToHash(decoded.Hash),
			Index:       0,
		}},
	}
	expectedReceipt.Bloom = gethtypes.CreateBloom(expectedReceipt)
	expected := gethtypes.DeriveSha(gethtypes.Receipts{expectedReceipt}, trie.NewStackTrie(nil)).Hex()
	if root != expected {
		t.Fatalf("unexpected receipt root: got %s want %s", root, expected)
	}
}

func TestEthereumRootsRejectMixedVexoTransactions(t *testing.T) {
	if _, ok := TransactionRoot([]types.Tx{types.Tx("bank:send")}); ok {
		t.Fatal("expected non-Ethereum tx root to be rejected")
	}
	if _, ok := ReceiptRoot([]types.Tx{types.Tx("bank:send")}, []types.Result{{Data: []byte("ok")}}); ok {
		t.Fatal("expected non-Ethereum receipt root to be rejected")
	}
}

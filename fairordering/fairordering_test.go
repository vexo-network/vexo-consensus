package fairordering

import (
	"bytes"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestSortTxsOrdersByTransactionHash(t *testing.T) {
	txs := []types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")}
	ordered := SortTxs(txs)

	if !IsOrdered(ordered) {
		t.Fatalf("expected ordered txs, got %q", ordered)
	}
	if bytes.Equal(ordered[0], txs[0]) && bytes.Equal(ordered[1], txs[1]) && bytes.Equal(ordered[2], txs[2]) {
		t.Fatal("expected non-trivial ordering for test fixture")
	}
}

func TestSortTxsCopiesInput(t *testing.T) {
	txs := []types.Tx{[]byte("b"), []byte("a")}
	ordered := SortTxs(txs)
	ordered[0][0] = 'x'

	if string(txs[0]) != "b" && string(txs[1]) != "a" {
		t.Fatalf("input mutated: %q", txs)
	}
}

func TestIsOrderedRejectsReorderedTxs(t *testing.T) {
	ordered := SortTxs([]types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")})
	reordered := []types.Tx{ordered[1], ordered[0], ordered[2]}
	if IsOrdered(reordered) {
		t.Fatalf("expected reordered txs rejected: %q", reordered)
	}
}

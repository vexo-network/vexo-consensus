package fairordering

import (
	"bytes"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func FuzzSortTxsWithSalt(f *testing.F) {
	f.Add([]byte("tx-a"), []byte("tx-b"), []byte("salt"))
	f.Add([]byte("same"), []byte("same"), []byte{})
	f.Add([]byte{}, []byte("non-empty"), []byte("height-1"))
	f.Fuzz(func(t *testing.T, first []byte, second []byte, salt []byte) {
		input := []types.Tx{
			append(types.Tx(nil), first...),
			append(types.Tx(nil), second...),
		}
		originalFirst := append([]byte(nil), input[0]...)
		originalSecond := append([]byte(nil), input[1]...)
		ordered := SortTxsWithSalt(input, salt)
		if !IsOrderedWithSalt(ordered, salt) {
			t.Fatalf("transactions are not ordered: %q %q", ordered[0], ordered[1])
		}
		if !bytes.Equal(input[0], originalFirst) || !bytes.Equal(input[1], originalSecond) {
			t.Fatal("SortTxsWithSalt mutated input transactions")
		}
	})
}

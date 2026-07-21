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

func TestSortTxsWithSaltIsDeterministicAndSaltSensitive(t *testing.T) {
	txs := []types.Tx{[]byte("alpha"), []byte("bravo"), []byte("charlie"), []byte("delta")}
	first := SortTxsWithSalt(txs, HeightSalt("vexo-test", 1))
	second := SortTxsWithSalt(txs, HeightSalt("vexo-test", 1))
	if !equalTxs(first, second) {
		t.Fatalf("expected deterministic salted order: %q != %q", first, second)
	}
	if !IsOrderedWithSalt(first, HeightSalt("vexo-test", 1)) {
		t.Fatalf("expected salted order to verify: %q", first)
	}

	saltSensitive := false
	for height := types.Height(2); height < 64; height++ {
		candidate := SortTxsWithSalt(txs, HeightSalt("vexo-test", height))
		if !equalTxs(first, candidate) {
			saltSensitive = true
			break
		}
	}
	if !saltSensitive {
		t.Fatal("expected at least one height salt to change transaction order")
	}
}

func TestSortTxsWithSaltPreservesSignerNonceOrder(t *testing.T) {
	txs := []types.Tx{
		[]byte("bank:send:alice:bob:1:signer=alice:nonce=3"),
		[]byte("bank:send:alice:bob:1:signer=alice:nonce=1"),
		[]byte("bank:send:alice:bob:1:signer=alice:nonce=2"),
		[]byte("bank:send:carol:dave:1:signer=carol:nonce=1"),
	}
	ordered := SortTxsWithSalt(txs, HeightSalt("vexo-test", 5))
	if !IsOrderedWithSalt(ordered, HeightSalt("vexo-test", 5)) {
		t.Fatalf("expected signer nonce order to validate, got %q", ordered)
	}
	seenAlice := make([]string, 0, 3)
	for _, tx := range ordered {
		if bytes.Contains(tx, []byte("signer=alice")) {
			seenAlice = append(seenAlice, string(tx))
		}
	}
	if len(seenAlice) != 3 ||
		seenAlice[0] != "bank:send:alice:bob:1:signer=alice:nonce=1" ||
		seenAlice[1] != "bank:send:alice:bob:1:signer=alice:nonce=2" ||
		seenAlice[2] != "bank:send:alice:bob:1:signer=alice:nonce=3" {
		t.Fatalf("expected alice nonce order to be ascending, got %q", seenAlice)
	}
}

func TestSortTxsWithSaltIsPermutationInvariantWithNonceDependencies(t *testing.T) {
	txs := []types.Tx{
		[]byte("bank:send:alice:bob:1:signer=alice:nonce=1"),
		[]byte("bank:send:alice:bob:1:signer=alice:nonce=2"),
		[]byte("bank:send:carol:dave:1:signer=carol:nonce=1"),
		[]byte("bank:send:carol:dave:1:signer=carol:nonce=2"),
		[]byte("bank:send:unsigned"),
	}
	salt := HeightSalt("vexo-test", 17)
	want := SortTxsWithSalt(txs, salt)
	permuted := cloneTxs(txs)
	var verifyPermutations func(int)
	verifyPermutations = func(index int) {
		if index == len(permuted) {
			if got := SortTxsWithSalt(permuted, salt); !equalTxs(got, want) {
				t.Fatalf("permutation changed canonical order: got %q want %q", got, want)
			}
			return
		}
		for swap := index; swap < len(permuted); swap++ {
			permuted[index], permuted[swap] = permuted[swap], permuted[index]
			verifyPermutations(index + 1)
			permuted[index], permuted[swap] = permuted[swap], permuted[index]
		}
	}
	verifyPermutations(0)
}

func equalTxs(left []types.Tx, right []types.Tx) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index], right[index]) {
			return false
		}
	}
	return true
}

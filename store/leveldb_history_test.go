package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func commitWritesForTest(t *testing.T, store *LevelDBStore, height types.Height, writes ...KVWrite) {
	t.Helper()
	if err := store.CommitBlockStateWithWrites(
		context.Background(),
		writes,
		BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height), byte(height >> 8)}},
		StateRecord{Height: height, AppHash: types.Hash{byte(height)}},
		nil,
	); err != nil {
		t.Fatal(err)
	}
}

func setWrite(namespace, key, value string) KVWrite {
	return KVWrite{Namespace: namespace, Key: []byte(key), Value: []byte(value)}
}

func valueAt(t *testing.T, store *LevelDBStore, height types.Height, namespace, key string) (string, bool) {
	t.Helper()
	value, err := store.GetAt(context.Background(), height, namespace, []byte(key))
	if errors.Is(err, ErrKeyNotFound) {
		return "", false
	}
	if err != nil {
		t.Fatalf("GetAt(%d, %s, %s): %v", height, namespace, key, err)
	}
	return string(value), true
}

func pairsAt(t *testing.T, store *LevelDBStore, height types.Height, namespace string, prefix string) string {
	t.Helper()
	var keyPrefix []byte
	if prefix != "" {
		keyPrefix = []byte(prefix)
	}
	pairs, err := store.ExportPrefixAt(context.Background(), height, namespace, keyPrefix)
	if err != nil {
		t.Fatalf("ExportPrefixAt(%d, %s, %q): %v", height, namespace, prefix, err)
	}
	text := ""
	for _, pair := range pairs {
		text += fmt.Sprintf("%s=%s;", pair.Key, pair.Value)
	}
	return text
}

// Genesis state is written directly, before the first committed state. A key
// the chain never changed afterwards still has its genesis value at every
// height, and a key changed later had its genesis value before the change.
func TestHistoricalReadsSeeGenesisStateThatBlocksDidNotChange(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx := context.Background()
	for _, write := range []KVWrite{setWrite("bank", "alice", "100"), setWrite("bank", "bob", "50")} {
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			t.Fatal(err)
		}
	}
	commitWritesForTest(t, store, 1, setWrite("bank", "carol", "7"))
	commitWritesForTest(t, store, 2, setWrite("bank", "alice", "90"))
	commitWritesForTest(t, store, 3, KVWrite{Namespace: "bank", Key: []byte("bob"), Delete: true})

	cases := []struct {
		height types.Height
		want   string
	}{
		{1, "alice=100;bob=50;carol=7;"},
		{2, "alice=90;bob=50;carol=7;"},
		{3, "alice=90;carol=7;"},
	}
	for _, tc := range cases {
		if got := pairsAt(t, store, tc.height, "bank", ""); got != tc.want {
			t.Fatalf("bank at height %d = %q, want %q", tc.height, got, tc.want)
		}
	}
	if value, found := valueAt(t, store, 1, "bank", "alice"); !found || value != "100" {
		t.Fatalf("alice at height 1 = %q/%v, want the genesis value", value, found)
	}
	if value, found := valueAt(t, store, 3, "bank", "carol"); !found || value != "7" {
		t.Fatalf("carol at height 3 = %q/%v", value, found)
	}
	if _, found := valueAt(t, store, 3, "bank", "bob"); found {
		t.Fatal("bob was deleted at height 3")
	}
	if value, found := valueAt(t, store, 2, "bank", "bob"); !found || value != "50" {
		t.Fatalf("bob at height 2 = %q/%v, want the genesis value", value, found)
	}
	if _, found := valueAt(t, store, 1, "bank", "dave"); found {
		t.Fatal("a key that never existed has no value")
	}
}

// A direct write after the chain started belongs to the next committed state:
// it must not rewrite the state of a height already committed.
func TestDirectWritesAfterACommittedStateDoNotRewriteThatState(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx := context.Background()
	commitWritesForTest(t, store, 1, setWrite("evm", "receipt/a", "r1"))
	if err := store.Set(ctx, "evm", []byte("receipt/a"), []byte("r2")); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "evm", []byte("receipt/b"), []byte("b2")); err != nil {
		t.Fatal(err)
	}
	if got := pairsAt(t, store, 1, "evm", ""); got != "receipt/a=r1;" {
		t.Fatalf("height 1 changed by a later direct write: %q", got)
	}
	commitWritesForTest(t, store, 2)
	if got := pairsAt(t, store, 2, "evm", ""); got != "receipt/a=r2;receipt/b=b2;" {
		t.Fatalf("height 2 = %q, want the direct writes", got)
	}
	if err := store.Delete(ctx, "evm", []byte("receipt/b")); err != nil {
		t.Fatal(err)
	}
	commitWritesForTest(t, store, 3)
	if _, found := valueAt(t, store, 3, "evm", "receipt/b"); found {
		t.Fatal("direct delete is not visible at the next height")
	}
	if value, found := valueAt(t, store, 2, "evm", "receipt/b"); !found || value != "b2" {
		t.Fatalf("direct delete rewrote height 2: %q/%v", value, found)
	}
}

func TestHistoricalReadsRefuseHeightsWithoutCommittedState(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx := context.Background()
	commitWritesForTest(t, store, 1, setWrite("bank", "alice", "1"))
	for _, height := range []types.Height{0, 2, ^types.Height(0)} {
		if _, err := store.GetAt(ctx, height, "bank", []byte("alice")); !errors.Is(err, ErrStateNotFound) {
			t.Fatalf("GetAt at uncommitted height %d: %v", height, err)
		}
		if _, err := store.ExportNamespaceAt(ctx, height, "bank"); !errors.Is(err, ErrStateNotFound) {
			t.Fatalf("ExportNamespaceAt at uncommitted height %d: %v", height, err)
		}
	}
}

func TestExportPrefixAtReturnsOnlyThePrefixSortedByKey(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx := context.Background()
	if err := store.Set(ctx, "evm", []byte("storage/0xb/1"), []byte("g")); err != nil {
		t.Fatal(err)
	}
	commitWritesForTest(t, store, 1,
		setWrite("evm", "storage/0xa/2", "a2"),
		setWrite("evm", "storage/0xa/1", "a1"),
		setWrite("evm", "storage/0xab/1", "ab"),
		setWrite("evm", "code/0xa", "c"),
	)
	commitWritesForTest(t, store, 2, setWrite("evm", "storage/0xa/1", "a1'"))
	if got := pairsAt(t, store, 1, "evm", "storage/0xa/"); got != "storage/0xa/1=a1;storage/0xa/2=a2;" {
		t.Fatalf("prefix at height 1 = %q", got)
	}
	if got := pairsAt(t, store, 2, "evm", "storage/"); got != "storage/0xa/1=a1';storage/0xa/2=a2;storage/0xab/1=ab;storage/0xb/1=g;" {
		t.Fatalf("prefix at height 2 = %q", got)
	}
}

// Pruning drops the versions no retained height reads. Values at retained
// heights stay exactly as they were, however many versions a key had.
func TestPruneBelowDropsUnreadableVersionsAndKeepsRetainedState(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx := context.Background()
	if err := store.Set(ctx, "bank", []byte("genesis"), []byte("g")); err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 10; height++ {
		writes := []KVWrite{setWrite("bank", "hot", fmt.Sprintf("v%d", height))}
		if height == 2 {
			writes = append(writes, setWrite("bank", "settled", "s"), setWrite("bank", "gone", "x"))
		}
		if height == 4 {
			writes = append(writes, KVWrite{Namespace: "bank", Key: []byte("gone"), Delete: true})
		}
		if height == 9 {
			writes = append(writes, setWrite("bank", "settled", "s2"))
		}
		commitWritesForTest(t, store, height, writes...)
	}
	before := map[types.Height]string{}
	for height := types.Height(6); height <= 10; height++ {
		before[height] = pairsAt(t, store, height, "bank", "")
	}
	result, err := store.PruneBelow(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	// hot: versions 0..5 go (6 replaces them), settled: version 0 goes (2 is
	// read by 6..8), gone: versions 0, 2 and the deletion at 4 go.
	if result.PrunedStateVersions != 6+1+3 {
		t.Fatalf("pruned %d versions, want 10", result.PrunedStateVersions)
	}
	for height, want := range before {
		if got := pairsAt(t, store, height, "bank", ""); got != want {
			t.Fatalf("pruning changed height %d: %q, want %q", height, got, want)
		}
	}
	if _, err := store.GetAt(ctx, 5, "bank", []byte("hot")); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("pruned height must be unavailable, got %v", err)
	}
	second, err := store.PruneBelow(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := pairsAt(t, store, 10, "bank", ""); got != before[10] {
		t.Fatalf("second prune changed height 10: %q, want %q", got, before[10])
	}
	// hot keeps only version 10; settled keeps none: its current value is its
	// value at every retained height.
	if second.PrunedStateVersions != 4+2 {
		t.Fatalf("second prune dropped %d versions, want 6", second.PrunedStateVersions)
	}
}

// An import installs the base state of a namespace; versions from before it
// describe a different chain and must not leak into reads after it.
func TestImportNamespaceDropsTheNamespaceHistory(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx := context.Background()
	commitWritesForTest(t, store, 1, setWrite("bank", "alice", "1"), setWrite("auth", "nonce", "1"))
	if err := store.ImportNamespace(ctx, "bank", []KVPair{{Namespace: "bank", Key: []byte("bob"), Value: []byte("2")}}); err != nil {
		t.Fatal(err)
	}
	if got := pairsAt(t, store, 1, "bank", ""); got != "bob=2;" {
		t.Fatalf("bank after import = %q", got)
	}
	if got := pairsAt(t, store, 1, "auth", ""); got != "nonce=1;" {
		t.Fatalf("import touched another namespace: %q", got)
	}
}

package store

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
)

// fullRoot recomputes the root of namespace from every pair the store holds.
func fullRoot(t *testing.T, storage *LevelDBStore, namespace string) types.Hash {
	t.Helper()
	pairs, err := storage.ExportNamespace(context.Background(), namespace)
	if err != nil {
		t.Fatal(err)
	}
	root, err := stateproof.Root(namespace, stateProofPairs(pairs))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// The tree the store keeps follows every way state changes, block commits, direct writes and
// namespace imports, and always has the root of the pairs it holds.
func TestStoreStateTreeFollowsEveryWrite(t *testing.T) {
	ctx := context.Background()
	storage, err := OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	random := rand.New(rand.NewSource(3))
	check := func(stage string) {
		t.Helper()
		for _, namespace := range []string{"bank", "staking"} {
			root, err := storage.Root(ctx, namespace)
			if err != nil {
				t.Fatal(err)
			}
			if want := fullRoot(t, storage, namespace); root != want {
				t.Fatalf("%s: namespace %s has tree root %x, its pairs %x", stage, namespace, root, want)
			}
		}
	}
	writes := func() []KVWrite {
		batch := make([]KVWrite, 0, 8)
		for index := 0; index < 1+random.Intn(8); index++ {
			namespace := []string{"bank", "staking"}[random.Intn(2)]
			key := []byte(fmt.Sprintf("key/%d", random.Intn(30)))
			if random.Intn(4) == 0 {
				batch = append(batch, KVWrite{Namespace: namespace, Key: key, Delete: true})
				continue
			}
			batch = append(batch, KVWrite{Namespace: namespace, Key: key, Value: []byte(fmt.Sprintf("%d", random.Int()))})
		}
		return batch
	}
	if err := storage.SetBatch(ctx, writes()); err != nil {
		t.Fatal(err)
	}
	check("genesis writes")
	for height := types.Height(1); height <= 30; height++ {
		batch := writes()
		predicted := make(map[string]types.Hash)
		for _, namespace := range []string{"bank", "staking"} {
			root, err := storage.RootWithWrites(ctx, namespace, batch)
			if err != nil {
				t.Fatal(err)
			}
			predicted[namespace] = root
		}
		block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height)}}
		if err := storage.CommitBlockStateWithWrites(ctx, batch, block, StateRecord{Height: height}, nil); err != nil {
			t.Fatal(err)
		}
		for namespace, root := range predicted {
			if actual, err := storage.Root(ctx, namespace); err != nil || actual != root {
				t.Fatalf("height %d: RootWithWrites predicted %x for %s, the commit has %x (%v)", height, root, namespace, actual, err)
			}
		}
		check(fmt.Sprintf("block %d", height))
		if height%7 == 0 {
			if err := storage.SetBatch(ctx, writes()); err != nil {
				t.Fatal(err)
			}
			check(fmt.Sprintf("direct writes after block %d", height))
		}
	}
	imported := []KVPair{{Namespace: "bank", Key: []byte("alice"), Value: []byte("1")}, {Namespace: "bank", Key: []byte("bob"), Value: []byte("2")}}
	if err := storage.ImportNamespace(ctx, "bank", imported); err != nil {
		t.Fatal(err)
	}
	check("import")
	if err := storage.ImportNamespace(ctx, "staking", nil); err != nil {
		t.Fatal(err)
	}
	if root, err := storage.Root(ctx, "staking"); err != nil || root != stateproof.EmptyRoot("staking") {
		t.Fatalf("an emptied namespace must have the empty root, got %x (%v)", root, err)
	}
}

// A proof of the current state reads the key's path from the stored tree and verifies against
// the namespace root; it is the proof the pairs of the namespace give.
func TestStoreProvesKeysFromItsStateTree(t *testing.T) {
	ctx := context.Background()
	storage, err := OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	for index := 0; index < 50; index++ {
		if err := storage.Set(ctx, "bank", []byte(fmt.Sprintf("account/%d", index)), []byte{byte(index)}); err != nil {
			t.Fatal(err)
		}
	}
	pairs, err := storage.ExportNamespace(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 60; index++ {
		key := []byte(fmt.Sprintf("account/%d", index))
		root, value, exists, proof, err := storage.StateProof(ctx, "bank", key)
		if err != nil {
			t.Fatal(err)
		}
		wantRoot, wantValue, wantExists, wantProof, err := stateproof.Prove("bank", stateProofPairs(pairs), key)
		if err != nil {
			t.Fatal(err)
		}
		if root != wantRoot || exists != wantExists || string(value) != string(wantValue) || len(proof.Siblings) != len(wantProof.Siblings) {
			t.Fatalf("key %s: stored proof differs from the proof of the pairs", key)
		}
		if exists && !stateproof.VerifyMembership("bank", key, value, proof, root) {
			t.Fatalf("key %s: membership must verify", key)
		}
		if !exists && !stateproof.VerifyNonMembership("bank", key, proof, root) {
			t.Fatalf("key %s: absence must verify", key)
		}
	}
}

// BenchmarkCommitIntoALargeNamespace commits blocks of ten writes into a namespace that holds
// many keys; with the stored tree the cost of a block does not grow with the namespace.
func BenchmarkCommitIntoALargeNamespace(b *testing.B) {
	for _, size := range []int{1_000, 50_000} {
		b.Run(fmt.Sprintf("keys=%d", size), func(b *testing.B) {
			ctx := context.Background()
			storage, err := OpenLevelDBWithOptions(b.TempDir(), LevelDBOptions{})
			if err != nil {
				b.Fatal(err)
			}
			defer storage.Close()
			batch := make([]KVWrite, 0, size)
			for index := 0; index < size; index++ {
				batch = append(batch, KVWrite{Namespace: "bank", Key: []byte(fmt.Sprintf("account/%d", index)), Value: []byte("1")})
			}
			if err := storage.SetBatch(ctx, batch); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				height := types.Height(iteration + 1)
				writes := make([]KVWrite, 0, 10)
				for index := 0; index < 10; index++ {
					writes = append(writes, KVWrite{Namespace: "bank", Key: []byte(fmt.Sprintf("account/%d", (iteration*10+index)%size)), Value: []byte(fmt.Sprintf("%d", iteration))})
				}
				root, err := storage.RootWithWrites(ctx, "bank", writes)
				if err != nil {
					b.Fatal(err)
				}
				block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{1}}
				roots := []StateRootRecord{{Height: height, Namespace: "bank", Root: root}}
				if err := storage.CommitBlockStateWithWrites(ctx, writes, block, StateRecord{Height: height}, roots); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A proof of a past height reads the versions of the tree nodes on the key's path: it verifies
// against the state root recorded for that height and proves the value the key held then. Pruning
// keeps every retained height provable and refuses the pruned ones.
func TestStoreProvesKeysAtPastHeightsFromTreeVersions(t *testing.T) {
	ctx := context.Background()
	storage, err := OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(ctx, "bank", []byte("genesis"), []byte("g")); err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(11))
	roots := map[types.Height]types.Hash{}
	states := map[types.Height]map[string][]byte{}
	state := map[string][]byte{"genesis": []byte("g")}
	for height := types.Height(1); height <= 20; height++ {
		writes := make([]KVWrite, 0, 6)
		for index := 0; index < 1+random.Intn(6); index++ {
			key := fmt.Sprintf("key/%d", random.Intn(25))
			if random.Intn(4) == 0 {
				writes = append(writes, KVWrite{Namespace: "bank", Key: []byte(key), Delete: true})
				delete(state, key)
				continue
			}
			value := []byte(fmt.Sprintf("%d/%d", height, index))
			writes = append(writes, KVWrite{Namespace: "bank", Key: []byte(key), Value: value})
			state[key] = value
		}
		root, err := storage.RootWithWrites(ctx, "bank", writes)
		if err != nil {
			t.Fatal(err)
		}
		block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height)}}
		if err := storage.CommitBlockStateWithWrites(ctx, writes, block, StateRecord{Height: height}, []StateRootRecord{{Height: height, Namespace: "bank", Root: root}}); err != nil {
			t.Fatal(err)
		}
		roots[height] = root
		snapshot := make(map[string][]byte, len(state))
		for key, value := range state {
			snapshot[key] = value
		}
		states[height] = snapshot
	}
	check := func(from types.Height) {
		t.Helper()
		for height := from; height <= 20; height++ {
			for index := -1; index < 25; index++ {
				key := fmt.Sprintf("key/%d", index)
				if index < 0 {
					key = "genesis"
				}
				root, value, exists, proof, err := storage.StateProofAt(ctx, height, "bank", []byte(key))
				if err != nil {
					t.Fatalf("height %d key %s: %v", height, key, err)
				}
				if root != roots[height] {
					t.Fatalf("height %d: tree root %x, recorded %x", height, root, roots[height])
				}
				want, present := states[height][key]
				if exists != present || string(value) != string(want) {
					t.Fatalf("height %d key %s: exists %v value %q, want %v %q", height, key, exists, value, present, want)
				}
				if exists && !stateproof.VerifyMembership("bank", []byte(key), value, proof, root) {
					t.Fatalf("height %d key %s: membership must verify", height, key)
				}
				if !exists && !stateproof.VerifyNonMembership("bank", []byte(key), proof, root) {
					t.Fatalf("height %d key %s: absence must verify", height, key)
				}
			}
		}
	}
	check(1)
	result, err := storage.PruneBelow(ctx, 12)
	if err != nil {
		t.Fatal(err)
	}
	if result.PrunedStateTreeVersions == 0 {
		t.Fatal("expected pruning to drop tree node versions")
	}
	check(12)
	if _, _, _, _, err := storage.StateProofAt(ctx, 11, "bank", []byte("genesis")); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("expected a pruned height to be refused, got %v", err)
	}
}

// ExportRangeAt pages through a namespace at a past height in key order and returns exactly the
// pairs ExportNamespaceAt returns, including keys holding zero bytes.
func TestExportRangeAtPagesThroughAPastState(t *testing.T) {
	ctx := context.Background()
	storage, err := OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	base := []KVWrite{
		{Namespace: "bank", Key: []byte("a"), Value: []byte("1")},
		{Namespace: "bank", Key: []byte{'a', 0x00}, Value: []byte("zero")},
		{Namespace: "bank", Key: []byte{'a', 0x00, 0x01}, Value: []byte("zero-one")},
		{Namespace: "bank", Key: []byte("b"), Value: []byte("2")},
	}
	if err := storage.SetBatch(ctx, base); err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 6; height++ {
		writes := []KVWrite{
			{Namespace: "bank", Key: []byte(fmt.Sprintf("k%02d", height)), Value: []byte{byte(height)}},
			{Namespace: "bank", Key: []byte{'a', 0x00}, Value: []byte{byte(height)}},
		}
		if height == 3 {
			writes = append(writes, KVWrite{Namespace: "bank", Key: []byte("b"), Delete: true})
		}
		block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height)}}
		if err := storage.CommitBlockStateWithWrites(ctx, writes, block, StateRecord{Height: height}, nil); err != nil {
			t.Fatal(err)
		}
	}
	for height := types.Height(1); height <= 6; height++ {
		want, err := storage.ExportNamespaceAt(ctx, height, "bank")
		if err != nil {
			t.Fatal(err)
		}
		for _, limit := range []int{1, 2, 3, 100} {
			var got []KVPair
			var after []byte
			for {
				page, more, err := storage.ExportRangeAt(ctx, height, "bank", after, limit)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, page...)
				if !more {
					break
				}
				after = page[len(page)-1].Key
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("height %d limit %d: pages %v, export %v", height, limit, got, want)
			}
		}
	}
	at2, err := storage.ExportNamespaceAt(ctx, 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(at2) != fmt.Sprint([]KVPair{
		{Namespace: "bank", Key: []byte("a"), Value: []byte("1")},
		{Namespace: "bank", Key: []byte{'a', 0x00}, Value: []byte{2}},
		{Namespace: "bank", Key: []byte{'a', 0x00, 0x01}, Value: []byte("zero-one")},
		{Namespace: "bank", Key: []byte("b"), Value: []byte("2")},
		{Namespace: "bank", Key: []byte("k01"), Value: []byte{1}},
		{Namespace: "bank", Key: []byte("k02"), Value: []byte{2}},
	}) {
		t.Fatalf("unexpected state at height 2: %v", at2)
	}
}

// A namespace imported page by page holds exactly the imported pairs, with the root of their tree,
// whatever it held before; pairs out of key order are refused.
func TestNamespaceImportWritesPagesIntoAFreshTree(t *testing.T) {
	ctx := context.Background()
	storage, err := OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(ctx, "bank", []byte("stale"), []byte("1")); err != nil {
		t.Fatal(err)
	}
	var want []stateproof.Pair
	var pages [][]KVPair
	for page := range 4 {
		var pairs []KVPair
		for index := range 25 {
			key := []byte(fmt.Sprintf("key/%03d", page*25+index))
			value := []byte(fmt.Sprintf("value-%d", page*25+index))
			pairs = append(pairs, KVPair{Namespace: "bank", Key: key, Value: value})
			want = append(want, stateproof.Pair{Key: key, Value: value})
		}
		pages = append(pages, pairs)
	}
	in, err := storage.BeginNamespaceImport(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		if err := in.Add(ctx, page); err != nil {
			t.Fatal(err)
		}
	}
	wantRoot, err := stateproof.Root("bank", want)
	if err != nil {
		t.Fatal(err)
	}
	root, err := in.Root(ctx)
	if err != nil || root != wantRoot || fullRoot(t, storage, "bank") != wantRoot {
		t.Fatalf("expected the root of the imported pairs %x, got %x (%v)", wantRoot, root, err)
	}
	if _, err := storage.Get(ctx, "bank", []byte("stale")); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected the import to replace the namespace, got %v", err)
	}
	if err := in.Add(ctx, pages[0][:1]); !errors.Is(err, ErrImportOrder) {
		t.Fatalf("expected a key before the imported ones to be refused, got %v", err)
	}
}

// Pruning in steps leaves exactly the versions one prune to the last height leaves, so the
// version index that lets each prune visit only the versions below its height loses nothing; and
// the index holds no more entries than there are versions.
func TestPruningInStepsMatchesOnePrune(t *testing.T) {
	ctx := context.Background()
	build := func() *LevelDBStore {
		storage, err := OpenLevelDB(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { storage.Close() })
		if err := storage.Set(ctx, "bank", []byte("genesis"), []byte("g")); err != nil {
			t.Fatal(err)
		}
		random := rand.New(rand.NewSource(5))
		for height := types.Height(1); height <= 40; height++ {
			var writes []KVWrite
			for index := 0; index < 1+random.Intn(5); index++ {
				key := []byte(fmt.Sprintf("key/%d", random.Intn(15)))
				if random.Intn(4) == 0 {
					writes = append(writes, KVWrite{Namespace: "bank", Key: key, Delete: true})
					continue
				}
				writes = append(writes, KVWrite{Namespace: "bank", Key: key, Value: []byte(fmt.Sprintf("%d/%d", height, index))})
			}
			root, err := storage.RootWithWrites(ctx, "bank", writes)
			if err != nil {
				t.Fatal(err)
			}
			block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height)}}
			if err := storage.CommitBlockStateWithWrites(ctx, writes, block, StateRecord{Height: height}, []StateRootRecord{{Height: height, Namespace: "bank", Root: root}}); err != nil {
				t.Fatal(err)
			}
		}
		return storage
	}
	count := func(storage *LevelDBStore, prefix []byte) int {
		iterator := storage.db.NewIterator(util.BytesPrefix(prefix), nil)
		defer iterator.Release()
		total := 0
		for iterator.Next() {
			total++
		}
		return total
	}
	stepped, once := build(), build()
	for _, retainFrom := range []types.Height{3, 8, 9, 17, 26, 30} {
		if _, err := stepped.PruneBelow(ctx, retainFrom); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := once.PruneBelow(ctx, 30); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range [][]byte{kvHistoryPrefix, stateTreeHistoryPrefix} {
		if got, want := count(stepped, prefix), count(once, prefix); got != want || want == 0 {
			t.Fatalf("pruning in steps left %d versions under %q, one prune %d", got, prefix, want)
		}
	}
	for _, pair := range [][2][]byte{{kvVersionIndexPrefix, kvHistoryPrefix}, {stateTreeVersionIndexPrefix, stateTreeHistoryPrefix}} {
		if entries, versions := count(stepped, pair[0]), count(stepped, pair[1]); entries > versions {
			t.Fatalf("the version index %q holds %d entries for %d versions", pair[0], entries, versions)
		}
	}
	for height := types.Height(30); height <= 40; height++ {
		for index := range 15 {
			key := []byte(fmt.Sprintf("key/%d", index))
			got, gotErr := stepped.GetAt(ctx, height, "bank", key)
			want, wantErr := once.GetAt(ctx, height, "bank", key)
			if string(got) != string(want) || !errors.Is(gotErr, wantErr) && gotErr != wantErr {
				t.Fatalf("height %d key %s: stepped %q %v, once %q %v", height, key, got, gotErr, want, wantErr)
			}
			root, _, _, _, err := stepped.StateProofAt(ctx, height, "bank", key)
			if err != nil {
				t.Fatal(err)
			}
			recorded, err := stepped.StateRoot(ctx, height, "bank")
			if err != nil || root != recorded.Root {
				t.Fatalf("height %d: proof root %x, recorded %x (%v)", height, root, recorded.Root, err)
			}
		}
	}
}

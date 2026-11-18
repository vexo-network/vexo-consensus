package store

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
)

// historyModel is the state a store must report: the current pairs of every namespace, and the
// pairs and roots of every committed height.
type historyModel struct {
	current map[string]map[string]string
	states  map[types.Height]map[string]map[string]string
	roots   map[types.Height]map[string]types.Hash
}

var historyModelNamespaces = []string{"bank", "evm"}

func (model *historyModel) apply(write KVWrite) {
	if write.Delete {
		delete(model.current[write.Namespace], string(write.Key))
		return
	}
	model.current[write.Namespace][string(write.Key)] = string(write.Value)
}

func (model *historyModel) root(t *testing.T, namespace string, pairs map[string]string) types.Hash {
	t.Helper()
	list := make([]stateproof.Pair, 0, len(pairs))
	for key, value := range pairs {
		list = append(list, stateproof.Pair{Key: []byte(key), Value: []byte(value)})
	}
	root, err := stateproof.Root(namespace, list)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func randomHistoryWrites(random *rand.Rand, count int) []KVWrite {
	writes := make([]KVWrite, 0, count)
	for range count {
		write := KVWrite{
			Namespace: historyModelNamespaces[random.Intn(len(historyModelNamespaces))],
			Key:       []byte(fmt.Sprintf("key/%d", random.Intn(12))),
		}
		if random.Intn(4) == 0 {
			write.Delete = true
		} else {
			write.Value = []byte(fmt.Sprintf("v%d", random.Intn(1000)))
		}
		writes = append(writes, write)
	}
	return writes
}

// The store's history matches a model of every committed state, through blocks that write a key
// more than once, direct writes before the first block and between blocks, prunes and reopens:
// every retained height reads and exports the pairs it held, and proves them against its root.
func TestHistoryMatchesAModelOfEveryCommittedState(t *testing.T) {
	for seed := int64(1); seed <= 6; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			ctx := context.Background()
			random := rand.New(rand.NewSource(seed))
			directory := filepath.Join(t.TempDir(), "store")
			storage, err := OpenPebble(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { storage.Close() }()
			model := &historyModel{current: map[string]map[string]string{}, states: map[types.Height]map[string]map[string]string{}, roots: map[types.Height]map[string]types.Hash{}}
			for _, namespace := range historyModelNamespaces {
				model.current[namespace] = map[string]string{}
			}
			direct := func() {
				writes := randomHistoryWrites(random, 1+random.Intn(4))
				var err error
				switch random.Intn(3) {
				case 0:
					err = storage.SetBatch(ctx, writes)
				case 1:
					write := writes[0]
					writes = writes[:1]
					if write.Delete {
						err = storage.Delete(ctx, write.Namespace, write.Key)
					} else {
						err = storage.Set(ctx, write.Namespace, write.Key, write.Value)
					}
				default:
					for _, write := range writes {
						if err = storage.SetBatch(ctx, []KVWrite{write}); err != nil {
							break
						}
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, write := range writes {
					model.apply(write)
				}
			}
			direct()
			retainFrom := types.Height(1)
			for height := types.Height(1); height <= 60; height++ {
				for random.Intn(3) == 0 {
					direct()
				}
				writes := randomHistoryWrites(random, 1+random.Intn(8))
				for _, write := range writes {
					model.apply(write)
				}
				model.states[height] = map[string]map[string]string{}
				model.roots[height] = map[string]types.Hash{}
				roots := make([]StateRootRecord, 0, len(historyModelNamespaces))
				for _, namespace := range historyModelNamespaces {
					model.states[height][namespace] = maps.Clone(model.current[namespace])
					want := model.root(t, namespace, model.current[namespace])
					model.roots[height][namespace] = want
					root, err := storage.RootWithWrites(ctx, namespace, writes)
					if err != nil {
						t.Fatal(err)
					}
					if root != want {
						t.Fatalf("height %d: root of %s over the block's writes is %x, want %x", height, namespace, root, want)
					}
					roots = append(roots, StateRootRecord{Height: height, Namespace: namespace, Root: root})
				}
				block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height), byte(height >> 8), 1}}
				if err := storage.CommitBlockStateWithWrites(ctx, writes, block, StateRecord{Height: height}, roots, nil); err != nil {
					t.Fatal(err)
				}
				if height > 10 && random.Intn(8) == 0 {
					retainFrom += types.Height(random.Intn(int(height - retainFrom + 1)))
					if _, err := storage.PruneBelow(ctx, retainFrom); err != nil {
						t.Fatal(err)
					}
				}
				if random.Intn(10) == 0 {
					if err := storage.Close(); err != nil {
						t.Fatal(err)
					}
					if storage, err = OpenPebble(directory); err != nil {
						t.Fatal(err)
					}
				}
				if height%5 == 0 {
					checkHistoryModel(t, storage, model, retainFrom, height)
				}
			}
		})
	}
}

func checkHistoryModel(t *testing.T, storage *PebbleStore, model *historyModel, retainFrom, latest types.Height) {
	t.Helper()
	ctx := context.Background()
	for height := retainFrom; height <= latest; height++ {
		for _, namespace := range historyModelNamespaces {
			want := model.states[height][namespace]
			pairs, err := storage.ExportNamespaceAt(ctx, height, namespace)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, pair := range pairs {
				got[string(pair.Key)] = string(pair.Value)
			}
			if !maps.Equal(got, want) {
				t.Fatalf("height %d: %s exports %v, want %v", height, namespace, got, want)
			}
			if !slices.IsSortedFunc(pairs, func(left, right KVPair) int { return slices.Compare(left.Key, right.Key) }) {
				t.Fatalf("height %d: %s exports pairs out of key order", height, namespace)
			}
			for index := range 12 {
				key := fmt.Sprintf("key/%d", index)
				value, err := storage.GetAt(ctx, height, namespace, []byte(key))
				wantValue, exists := want[key]
				switch {
				case errors.Is(err, ErrKeyNotFound):
					if exists {
						t.Fatalf("height %d: %s/%s reads absent, want %q", height, namespace, key, wantValue)
					}
				case err != nil:
					t.Fatal(err)
				case !exists || string(value) != wantValue:
					t.Fatalf("height %d: %s/%s reads %q, want %q (present %v)", height, namespace, key, value, wantValue, exists)
				}
				root, proved, found, proof, err := storage.StateProofAt(ctx, height, namespace, []byte(key))
				if err != nil {
					t.Fatal(err)
				}
				if root != model.roots[height][namespace] {
					t.Fatalf("height %d: %s proves against %x, want %x", height, namespace, root, model.roots[height][namespace])
				}
				if found != exists || (found && (string(proved) != wantValue || !stateproof.VerifyMembership(namespace, []byte(key), proved, proof, root))) ||
					(!found && !stateproof.VerifyNonMembership(namespace, []byte(key), proof, root)) {
					t.Fatalf("height %d: proof of %s/%s does not verify", height, namespace, key)
				}
			}
		}
	}
	if _, err := storage.GetAt(ctx, retainFrom-1, "bank", []byte("key/0")); retainFrom > 1 && !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("pruned height %d must be unavailable, got %v", retainFrom-1, err)
	}
	for _, namespace := range historyModelNamespaces {
		root, err := storage.Root(ctx, namespace)
		if err != nil {
			t.Fatal(err)
		}
		if want := model.root(t, namespace, model.current[namespace]); root != want {
			t.Fatalf("current root of %s is %x, want %x", namespace, root, want)
		}
	}
}

// A proof read while blocks commit verifies against the root it is read with: the tree node cache
// never mixes the nodes of two states.
func TestStateProofsStayConsistentWhileBlocksCommit(t *testing.T) {
	ctx := context.Background()
	storage := openTestStore(t)
	defer closeStore(t, storage)
	var wait sync.WaitGroup
	done := make(chan struct{})
	failures := make(chan string, 4)
	for reader := range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for proof := 0; ; proof++ {
				select {
				case <-done:
					return
				default:
				}
				key := []byte(fmt.Sprintf("key/%d", (reader+proof)%64))
				root, value, exists, path, err := storage.StateProof(ctx, "bank", key)
				if err != nil {
					failures <- err.Error()
					return
				}
				if (exists && !stateproof.VerifyMembership("bank", key, value, path, root)) || (!exists && !stateproof.VerifyNonMembership("bank", key, path, root)) {
					failures <- fmt.Sprintf("proof of %s does not verify against %x", key, root)
					return
				}
			}
		}()
	}
	random := rand.New(rand.NewSource(3))
	for height := types.Height(1); height <= 300; height++ {
		writes := make([]KVWrite, 0, 16)
		for range 16 {
			key := []byte(fmt.Sprintf("key/%d", random.Intn(64)))
			if random.Intn(3) == 0 {
				writes = append(writes, KVWrite{Namespace: "bank", Key: key, Delete: true})
			} else {
				writes = append(writes, KVWrite{Namespace: "bank", Key: key, Value: []byte(fmt.Sprintf("%d", height))})
			}
		}
		commitWritesForTest(t, storage, height, writes...)
	}
	close(done)
	wait.Wait()
	select {
	case failure := <-failures:
		t.Fatal(failure)
	default:
	}
}

// A reader that opened its snapshot before a block committed reads the nodes of its snapshot, not
// the cached nodes of the new state, and what it reads is not cached as current.
func TestTreeNodeCacheNeverServesOrKeepsTheNodesOfAnotherState(t *testing.T) {
	ctx := context.Background()
	storage := openTestStore(t)
	defer closeStore(t, storage)
	commitWritesForTest(t, storage, 1, setWrite("bank", "alice", "1"), setWrite("bank", "bob", "1"))
	before, err := storage.Root(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, view := storage.snapshot()
	defer snapshot.Close()
	if view.cache == nil {
		t.Fatal("a snapshot opened while no batch commits must use the cache")
	}
	commitWritesForTest(t, storage, 2, setWrite("bank", "alice", "2"))
	after, err := storage.Root(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatal("the block must change the root")
	}
	old, err := stateproof.NewTree("bank", treeNodes{reader: snapshot, namespace: "bank", cache: view}).Root()
	if err != nil {
		t.Fatal(err)
	}
	if old != before {
		t.Fatalf("the reader of the earlier snapshot reads root %x, want %x", old, before)
	}
	current, err := storage.Root(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if current != after {
		t.Fatalf("after the earlier snapshot was read the root is %x, want %x", current, after)
	}
}

// Roots computed over the same state for different writes, as for the proposals of two rounds at
// one height, are each the root of their own writes, and the block that commits is the one whose
// writes it carries, whichever was computed last.
func TestRootsOfDifferentWritesOverOneStateStayApart(t *testing.T) {
	ctx := context.Background()
	storage := openTestStore(t)
	defer closeStore(t, storage)
	commitWritesForTest(t, storage, 1, setWrite("bank", "alice", "1"), setWrite("bank", "bob", "1"))
	alternatives := [][]KVWrite{
		{setWrite("bank", "alice", "2")},
		{setWrite("bank", "alice", "3")},
		{{Namespace: "bank", Key: []byte("alice"), Value: []byte{}}},
		{{Namespace: "bank", Key: []byte("alice"), Delete: true}},
		{setWrite("bank", "alice", "2"), setWrite("bank", "alice", "3")},
		{setWrite("bank", "alice", "3"), setWrite("bank", "alice", "2")},
		// Without the kind of each write, these two would encode alike.
		{{Namespace: "bank", Key: []byte("alice"), Delete: true}, setWrite("bank", "bob", "x")},
		{setWrite("bank", "alice", "bob"), {Namespace: "bank", Key: []byte("x"), Delete: true}},
	}
	want := func(writes []KVWrite) types.Hash {
		state := map[string]string{"alice": "1", "bob": "1"}
		for _, write := range writes {
			if write.Delete {
				delete(state, string(write.Key))
			} else {
				state[string(write.Key)] = string(write.Value)
			}
		}
		return (&historyModel{}).root(t, "bank", state)
	}
	for round := range 2 {
		for index, writes := range alternatives {
			root, err := storage.RootWithWrites(ctx, "bank", writes)
			if err != nil {
				t.Fatal(err)
			}
			if root != want(writes) {
				t.Fatalf("round %d: root of alternative %d is %x, want %x", round, index, root, want(writes))
			}
		}
	}
	commitWritesForTest(t, storage, 2, alternatives[3]...)
	root, err := storage.Root(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if root != want(alternatives[3]) {
		t.Fatalf("committed root is %x, want the root of the committed writes %x", root, want(alternatives[3]))
	}
}

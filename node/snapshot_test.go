package node

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// exportCountingStore counts the namespace exports served through it. It is
// swapped in after the chain state exists, so it only needs the read side of
// the store.
type exportCountingStore struct {
	store.Store
	exports atomic.Int64
}

func (counting *exportCountingStore) ExportNamespace(ctx context.Context, namespace string) ([]store.KVPair, error) {
	counting.exports.Add(1)
	return counting.Store.(store.SnapshotKVStore).ExportNamespace(ctx, namespace)
}

func (counting *exportCountingStore) ExportNamespaceAt(ctx context.Context, height types.Height, namespace string) ([]store.KVPair, error) {
	counting.exports.Add(1)
	return counting.Store.ExportNamespaceAt(ctx, height, namespace)
}

func (counting *exportCountingStore) ImportNamespace(ctx context.Context, namespace string, pairs []store.KVPair) error {
	return counting.Store.(store.SnapshotKVStore).ImportNamespace(ctx, namespace, pairs)
}

// startNodeWithExportCounter starts a node whose store holds one block of state
// and one key-value pair, and counts the namespace exports from then on.
func startNodeWithExportCounter(t *testing.T) (*Node, *exportCountingStore) {
	t.Helper()
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { node.Stop(context.Background()) })
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Store.Set(context.Background(), "bank", []byte("balance/alice"), []byte("7")); err != nil {
		t.Fatal(err)
	}
	counting := &exportCountingStore{Store: runtime.Store}
	runtime.Store = counting
	return node, counting
}

func TestRecoveryReportDoesNotExportApplicationState(t *testing.T) {
	node, counting := startNodeWithExportCounter(t)

	for step := 0; step < 3; step++ {
		report, err := node.RecoveryReport(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		if !report.OK || !report.SnapshotAvailable || report.LatestStateHeight != 2 {
			t.Fatalf("unexpected recovery report: %+v", report)
		}
	}
	if exports := counting.exports.Load(); exports != 0 {
		t.Fatalf("a recovery report must not copy application state, got %d namespace exports", exports)
	}
}

func TestRecoveryReportStillReportsMissingSnapshotHead(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	report, err := node.RecoveryReport(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.SnapshotAvailable || report.OK {
		t.Fatalf("a node without committed state has no snapshot, got %+v", report)
	}
}

func TestStateSnapshotHeadReadsNoApplicationState(t *testing.T) {
	node, counting := startNodeWithExportCounter(t)

	head, err := node.StateSnapshotHead(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exports := counting.exports.Load(); exports != 0 {
		t.Fatalf("the snapshot head must not copy application state, got %d namespace exports", exports)
	}
	if head.State.Height != 2 || head.State.AppHash == (types.Hash{}) || head.State.LastBlockHash == (types.Hash{}) {
		t.Fatalf("unexpected snapshot head identity: %+v", head)
	}
	if len(head.StateRoots) != len(head.Namespaces) || head.StateRoots[0].Namespace != "bank" || head.StateRoots[0].Root == (types.Hash{}) {
		t.Fatalf("unexpected snapshot head roots: %+v", head.StateRoots)
	}
}

// A snapshot is the state a committed block header commits the app hash of, read at that height:
// block 3 names the validator set of registry height 2, so it commits the state after block 1, and
// what changed since is not in it.
func TestStateSnapshotExportsTheStateItsBlockHeaderCommits(t *testing.T) {
	ctx := context.Background()
	fixture := newCommitFixture(t)
	first := fixture.block(1, types.Hash{}, types.Tx("bank:send:a"))
	second := fixture.block(2, consensus.HashBlock(first), types.Tx("bank:send:b"))
	for _, block := range []types.Block{first, second} {
		if height, _ := fixture.deliver(fixture.commit(block)); height != block.Header.Height {
			t.Fatalf("expected block %d to be applied, node is at height %d", block.Header.Height, height)
		}
	}
	third := fixture.block(3, consensus.HashBlock(second), types.Tx("bank:send:c"))
	if height, _ := fixture.deliver(fixture.commit(third)); height != 3 {
		t.Fatalf("expected block 3 to be applied, node is at height %d", height)
	}
	runtime, err := fixture.node.Runtime()
	if err != nil {
		t.Fatal(err)
	}

	// Written after block 3, so it is state of height 4 on, not of the state block 3 commits.
	if err := runtime.Store.Set(ctx, "bank", []byte("balance/late"), []byte("9")); err != nil {
		t.Fatal(err)
	}

	snapshot, err := fixture.node.StateSnapshot(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if finality.HeaderHash(snapshot.Header) != consensus.HashBlock(third) {
		t.Fatalf("expected the latest snapshot to be anchored to block 3, got header %+v", snapshot.Header)
	}
	stateOne, err := runtime.Store.StateByHeight(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	headerAppHashOne, err := runtime.HeaderAppHashAt(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != stateOne || snapshot.Header.AppHash != headerAppHashOne {
		t.Fatalf("expected the state after block 1 %+v, got %+v", stateOne, snapshot.State)
	}
	want := []string{"bank", "auth", "feemarket", "validator_registry"}
	if len(snapshot.Namespaces) != len(want) || len(snapshot.StateRoots) != len(want) {
		t.Fatalf("expected namespaces %v, got %v roots %+v", want, snapshot.Namespaces, snapshot.StateRoots)
	}
	for index, namespace := range want {
		root, err := runtime.Store.StateRoot(ctx, 1, namespace)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Namespaces[index] != namespace || snapshot.StateRoots[index] != root {
			t.Fatalf("expected the roots of height 1 for %v, got %v roots %+v", want, snapshot.Namespaces, snapshot.StateRoots)
		}
		pairs, err := stateproof.Root(namespace, snapshotPairs(snapshot.KV, namespace))
		if err != nil {
			t.Fatal(err)
		}
		if pairs != root.Root {
			t.Fatalf("snapshot pairs of %s do not have the root of height 1", namespace)
		}
	}
	for _, pair := range snapshot.KV {
		if pair.Namespace == "bank" && string(pair.Key) == "balance/late" {
			t.Fatal("a write after the anchored state must not be in the snapshot")
		}
	}

	if _, err := fixture.node.StateSnapshot(ctx, 2); !errors.Is(err, ErrSnapshotNotAnchored) {
		t.Fatalf("block 2 commits the genesis state, which is no snapshot, got %v", err)
	}
	if _, err := fixture.node.StateSnapshot(ctx, 9); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected an uncommitted block to have no snapshot, got %v", err)
	}
}

func snapshotPairs(kv []store.KVPair, namespace string) []stateproof.Pair {
	var pairs []stateproof.Pair
	for _, pair := range kv {
		if pair.Namespace == namespace {
			pairs = append(pairs, stateproof.Pair{Key: pair.Key, Value: pair.Value})
		}
	}
	return pairs
}

func TestStateSnapshotHeadRequiresRunningNode(t *testing.T) {
	node := newTestNode(t)
	if _, err := node.StateSnapshotHead(context.Background()); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected a stopped node to have no snapshot head, got %v", err)
	}
}

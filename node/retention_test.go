package node

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// A node prunes to its retention as it commits, so its store holds a bounded number of heights.
func TestNodePrunesToItsRetentionAsItCommits(t *testing.T) {
	ctx := context.Background()
	fixture := newCommitFixture(t)
	fixture.node.cfg.RetainBlocks = 2
	runtime, err := fixture.node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	parent := types.Hash{}
	for height := types.Height(1); height <= 5; height++ {
		block := fixture.block(height, parent, types.Tx("bank:send:a"))
		if applied, _ := fixture.deliver(fixture.commit(block)); applied != height {
			t.Fatalf("expected block %d to be applied, node is at height %d", height, applied)
		}
		parent = consensus.HashBlock(block)
		index, err := runtime.BlockIndex(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Pruned once it holds one height past its retention of two.
		if index.TotalBlocks > 2 || index.LatestHeight != height || index.EarliestHeight != max(height, 2)-1 {
			t.Fatalf("after block %d: index %+v, want the last two heights", height, index)
		}
	}
	if _, err := runtime.Store.StateByHeight(ctx, 3); !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected the state of height 3 to be pruned, got %v", err)
	}
	if metrics, err := fixture.node.Metrics(ctx); err != nil || metrics.RetentionPruneFailures != 0 {
		t.Fatalf("expected no prune failures, got %d (%v)", metrics.RetentionPruneFailures, err)
	}
}

func TestConfigRefusesARetentionTooShortToServeASnapshot(t *testing.T) {
	cfg := UnsafeTestConfig("vexo-test", t.TempDir())
	for _, retain := range []uint64{0, MinRetainBlocks, DefaultRetainBlocks} {
		cfg.RetainBlocks = retain
		if err := cfg.Validate(); err != nil {
			t.Fatalf("retain_blocks %d: %v", retain, err)
		}
	}
	cfg.RetainBlocks = MinRetainBlocks - 1
	if err := cfg.Validate(); !errors.Is(err, ErrRetainBlocksTooFew) {
		t.Fatalf("expected ErrRetainBlocksTooFew, got %v", err)
	}
}

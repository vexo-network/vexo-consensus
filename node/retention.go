package node

import (
	"context"
	"errors"
	"fmt"

	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
)

// DefaultRetainBlocks is the retention of a new node: the blocks, states and history of the
// last 1,209,600 heights, the window evidence stays punishable in (slashing
// DefaultLifecyclePolicy().EvidenceMaxAge), so the node can serve every block evidence may concern.
const DefaultRetainBlocks uint64 = 1_209_600

// MinRetainBlocks is the least retention a pruning node may keep: the state at its earliest
// height has to stay anchored by a certified block at most MaxSnapshotAnchorLinks+1 heights
// above it, so the node always serves a snapshot.
const MinRetainBlocks uint64 = MaxSnapshotAnchorLinks + 2

// ErrRetainBlocksTooFew reports a retention below MinRetainBlocks.
var ErrRetainBlocksTooFew = errors.New("retain_blocks is below the minimum")

func validateRetainBlocks(retain uint64) error {
	if retain != 0 && retain < MinRetainBlocks {
		return fmt.Errorf("%w %d: %d", ErrRetainBlocksTooFew, MinRetainBlocks, retain)
	}
	return nil
}

// pruneToRetention prunes the store to the configured retention once it holds 1% more heights
// (at least one), so a prune drops a batch of heights rather than one at every commit. Pruning is local, so a failure leaves consensus to go on:
// it is counted in the metrics, and the next commit tries again.
func (node *Node) pruneToRetention(ctx context.Context, runtime *vexoruntime.Runtime) {
	retain := node.cfg.RetainBlocks
	if retain == 0 || runtime.Store == nil {
		return
	}
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		node.metrics.observeRetentionPruneFailure()
		return
	}
	if index.TotalBlocks < retain+max(retain/100, 1) {
		return
	}
	if _, err := runtime.PruneByRetention(ctx, store.RetentionPolicy{RetainRecent: retain}); err != nil {
		node.metrics.observeRetentionPruneFailure()
	}
}

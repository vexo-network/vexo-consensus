package node

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/types"
)

// Every validator of an even set leads views of a running chain. A schedule of the height and the
// view left every second validator without a view on the happy path, where both grow by one per
// block, so its transactions reached blocks only through the others.
func TestEveryValidatorOfAnEvenSetLeadsBlocks(t *testing.T) {
	nodes, links := newThroughputValidators(t, 4, 0, 0, false)
	for _, node := range nodes {
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	for _, link := range links {
		defer link.close()
	}
	ctx := context.Background()
	loop := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: 5 * time.Second, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeFinalized}
	for _, node := range nodes {
		if err := node.StartConsensusLoop(ctx, loop); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
	}
	const blocks = 12
	deadline := time.Now().Add(30 * time.Second)
	for nodes[0].Status(ctx).LatestHeight < blocks {
		if time.Now().After(deadline) {
			t.Fatalf("the chain did not reach height %d: %+v", blocks, nodes[0].Status(ctx))
		}
		time.Sleep(5 * time.Millisecond)
	}
	runtime, err := nodes[0].Runtime()
	if err != nil {
		t.Fatal(err)
	}
	machine, err := nodes[0].Consensus()
	if err != nil {
		t.Fatal(err)
	}
	set, err := machine.ProposalValidatorSet()
	if err != nil {
		t.Fatal(err)
	}
	led := map[types.ValidatorID]int{}
	for height := types.Height(1); height <= blocks; height++ {
		record, err := runtime.BlockByHeight(ctx, height)
		if err != nil {
			t.Fatal(err)
		}
		leader, ok := consensus.Leader(record.Block.Header, set)
		if !ok {
			t.Fatal("no leader")
		}
		led[leader]++
	}
	if len(led) != len(nodes) {
		t.Fatalf("expected all %d validators to lead blocks, got %v", len(nodes), led)
	}
}

// With one validator of four down, the others still certify their batches (three acknowledgements
// of four are a quorum) and commit every transaction submitted to them; the views of the crashed
// leader time out.
func TestLanesCommitEveryTransactionWithACrashedValidator(t *testing.T) {
	nodes, links := newThroughputValidators(t, 4, 0, 0, false)
	running := nodes[:3]
	for _, node := range running {
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	for _, link := range links {
		defer link.close()
	}
	ctx := context.Background()
	loop := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: 300 * time.Millisecond, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeFinalized}
	for _, node := range running {
		if err := node.StartConsensusLoop(ctx, loop); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
	}
	const total = 600
	for index := 0; index < total; index++ {
		if err := running[index%len(running)].SubmitTx(ctx, types.Tx(fmt.Sprintf("bank:crash-%04d", index))); err != nil {
			t.Fatal(err)
		}
	}
	runtime, err := running[0].Runtime()
	if err != nil {
		t.Fatal(err)
	}
	committed, counted := map[string]struct{}{}, types.Height(0)
	deadline := time.Now().Add(60 * time.Second)
	for len(committed) < total {
		if time.Now().After(deadline) {
			t.Fatalf("committed %d of %d transactions: %+v", len(committed), total, running[0].Status(ctx))
		}
		for height := running[0].Status(ctx).LatestHeight; counted < height; {
			counted++
			record, err := runtime.BlockByHeight(ctx, counted)
			if err != nil {
				t.Fatal(err)
			}
			for _, tx := range record.Block.Txs {
				if _, again := committed[string(tx)]; again {
					t.Fatalf("transaction %q committed twice", tx)
				}
				committed[string(tx)] = struct{}{}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A crashed validator fails the first view it leads and is benched: the views that directly follow a
// certified block pass over it, and only the views after a timeout, led by the whole set, can still
// fall on it. Scheduled by stake alone it would fail a quarter of the views.
func TestACrashedLeaderIsBenchedAfterItFailsAView(t *testing.T) {
	nodes, links := newThroughputValidators(t, 4, 0, 0, false)
	running := nodes[:3]
	for _, node := range running {
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	for _, link := range links {
		defer link.close()
	}
	ctx := context.Background()
	loop := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: 200 * time.Millisecond, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeFinalized}
	for _, node := range running {
		if err := node.StartConsensusLoop(ctx, loop); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
	}
	const blocks = 60
	deadline := time.Now().Add(60 * time.Second)
	for running[0].Status(ctx).LatestHeight < blocks {
		if time.Now().After(deadline) {
			t.Fatalf("the chain did not reach height %d: %+v", blocks, running[0].Status(ctx))
		}
		time.Sleep(5 * time.Millisecond)
	}
	runtime, err := running[0].Runtime()
	if err != nil {
		t.Fatal(err)
	}
	crashed := nodes[3].cfg.ValidatorID
	failed, benched := 0, 0
	for height := types.Height(2); height <= blocks; height++ {
		record, err := runtime.BlockByHeight(ctx, height)
		if err != nil {
			t.Fatal(err)
		}
		header := record.Block.Header
		failed += int(header.Round - header.JustifyRound - 1)
		for _, entry := range header.Benched {
			if entry.Validator == crashed {
				benched++
			}
		}
	}
	if failed > 6 {
		t.Fatalf("expected the crashed validator to cost a few views over %d blocks, %d views failed", blocks, failed)
	}
	if benched < blocks-10 {
		t.Fatalf("expected the crashed validator benched in nearly every block, it is in %d of %d", benched, blocks-1)
	}
}

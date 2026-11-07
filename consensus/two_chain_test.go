package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestTwoChainFinalizesParentWhenChildIsCertifiedInTheNextView(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	first := cluster.extend(machine, nil, 0, finality.TimeoutCert{})
	if decisions := machine.CommitDecisions(); len(decisions) != 0 {
		t.Fatalf("a lone certified block must not be final, got %+v", decisions)
	}
	second := cluster.extend(machine, &first, 1, finality.TimeoutCert{})

	decisions := machine.CommitDecisions()
	if len(decisions) != 1 || decisions[0].CommittedBlockHash != first.hash || decisions[0].CommittedHeight != 1 {
		t.Fatalf("expected the parent to be final once the child is certified, got %+v", decisions)
	}
	if decisions[0].CommitQC.BlockHash != first.hash || decisions[0].CommitQC.Round != 0 {
		t.Fatalf("expected the commit qc to be the parent's, got %+v", decisions[0].CommitQC)
	}
	if machine.Status(context.Background()).LastFinalized != first.hash {
		t.Fatal("expected the status to report the finalized parent")
	}
	if _, found := machine.blockTree.Get(second.hash); !found {
		t.Fatal("the child must stay in the tree until it is final itself")
	}
}

// Heights alone are not enough: a child certified two views after its parent does not prove
// that a quorum locked on the parent, so the parent stays pending until a later
// consecutive pair finalizes it together with all its ancestors.
func TestTwoChainRequiresConsecutiveViews(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	first := cluster.extend(machine, nil, 0, finality.TimeoutCert{})
	skipped := cluster.timeoutCert(2, 1, first.qc, "a", "b", "c")
	second := cluster.extend(machine, &first, 2, skipped)
	if decisions := machine.CommitDecisions(); len(decisions) != 0 {
		t.Fatalf("certificates in views 0 and 2 must not finalize the first block, got %+v", decisions)
	}

	third := cluster.extend(machine, &second, 3, finality.TimeoutCert{})
	decisions := machine.CommitDecisions()
	if len(decisions) != 2 {
		t.Fatalf("expected the second block and its ancestor to be final, got %+v", decisions)
	}
	if decisions[0].CommittedBlockHash != first.hash || decisions[1].CommittedBlockHash != second.hash {
		t.Fatalf("expected ancestors to finalize before their descendants, got %+v", decisions)
	}
	if machine.Status(context.Background()).LastFinalized != second.hash {
		t.Fatal("expected the second block to be the last finalized block")
	}
	if _, found := machine.blockTree.Get(third.hash); !found {
		t.Fatal("the certified head must remain in the tree")
	}
}

func TestTwoChainDoesNotFinalizeChildlessOrUncertifiedBlocks(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	blocks := cluster.chain(3, 0)

	// Three proposals chained by certificates finalize the first block only: the third
	// block has no certificate of its own yet.
	cluster.feed(machine, blocks)
	decisions := machine.CommitDecisions()
	if len(decisions) != 1 || decisions[0].CommittedBlockHash != blocks[0].hash {
		t.Fatalf("expected exactly the first block to be final, got %+v", decisions)
	}
}

func TestFinalizationFollowsTheCertifiedBranchOnly(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	first := cluster.extend(machine, nil, 0, finality.TimeoutCert{})
	// A rival at height 2 is stored but never certified; the certified branch finalizes.
	rival := cluster.block(2, first.hash, "rival")
	rivalTimeout := cluster.timeoutCert(2, 1, first.qc, "a", "b", "c")
	if err := machine.OnProposal(context.Background(), cluster.proposal("c", rival, 2, first.qc, rivalTimeout)); err != nil {
		t.Fatal(err)
	}
	second := cluster.extend(machine, &first, 3, cluster.timeoutCert(2, 2, first.qc, "a", "b", "c"))
	cluster.extend(machine, &second, 4, finality.TimeoutCert{})

	for _, decision := range machine.CommitDecisions() {
		if decision.CommittedBlockHash == HashBlock(rival) {
			t.Fatal("the uncertified rival must never finalize")
		}
	}
	if machine.Status(context.Background()).LastFinalized != second.hash {
		t.Fatal("expected the certified branch to finalize")
	}
}

func TestConflictingFinalizationIsRejected(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	first := cluster.extend(machine, nil, 0, finality.TimeoutCert{})
	cluster.extend(machine, &first, 1, finality.TimeoutCert{})
	if len(machine.CommitDecisions()) != 1 {
		t.Fatal("setup: expected the first block to be final")
	}

	// Genuine certificates for a rival at the same height: this can only appear when more
	// than a third of the validators equivocated, and it must never be accepted silently.
	rival := cluster.blockAt(1, types.Hash{}, 5, "rival")
	rivalHash := HashBlock(rival)
	rivalChild := cluster.blockAt(2, rivalHash, 6)
	rivalChildHash := HashBlock(rivalChild)
	candidate := CommitCandidate{
		BlockHeight:     1,
		BlockHash:       rivalHash,
		BlockQC:         cluster.qc(1, 5, rivalHash),
		ChildHeight:     2,
		ChildHash:       rivalChildHash,
		ChildParentHash: rivalHash,
		ChildQC:         cluster.qc(2, 6, rivalChildHash),
	}
	if _, err := machine.ApplyCommitRule(candidate); !errors.Is(err, ErrConflictingCommit) {
		t.Fatalf("expected a conflicting commit, got %v", err)
	}
	if err := machine.ObserveCommittedBlock(rival, candidate.BlockQC); !errors.Is(err, ErrConflictingCommit) {
		t.Fatalf("expected the committed rival to be refused, got %v", err)
	}
	if got := machine.CommitDecisions(); len(got) != 1 || got[0].CommittedBlockHash != first.hash {
		t.Fatalf("the finalized chain must not change, got %+v", got)
	}
}

func TestFinalizationIsIncrementalAndStateStaysBounded(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	const length = 700

	cluster.extendChain(machine, length)

	decisions := machine.CommitDecisions()
	if len(decisions) != length-1 {
		t.Fatalf("expected %d final blocks, got %d", length-1, len(decisions))
	}
	for index, decision := range decisions {
		if decision.CommittedHeight != types.Height(index+1) {
			t.Fatalf("decision %d is for height %d", index, decision.CommittedHeight)
		}
	}
	if machine.floorHeight != length-1 {
		t.Fatalf("expected the finalized root at height %d, got %d", length-1, machine.floorHeight)
	}
	if machine.blockTree.Len() > 3 {
		t.Fatalf("the block tree must hold only the unfinalized suffix, got %d blocks", machine.blockTree.Len())
	}
	if len(machine.votes) > 2 || len(machine.votedVotes) > 2 {
		t.Fatalf("vote tables must hold only recent heights, got %d and %d", len(machine.votes), len(machine.votedVotes))
	}
	if len(machine.viewSlots) > 3 || len(machine.pendingFinal) != 0 || machine.commitIndex.Len() > 3 {
		t.Fatalf("view slots, pending commits and the commit index must stay small: %d %d %d",
			len(machine.viewSlots), len(machine.pendingFinal), machine.commitIndex.Len())
	}

	machine.Prune(length - 1)
	if remaining := machine.CommitDecisions(); len(remaining) != 1 || remaining[0].CommittedHeight != length-1 {
		t.Fatalf("expected only the newest decision after pruning, got %d decisions", len(remaining))
	}
	machine.Prune(10)
	if machine.floorHeight != length-1 {
		t.Fatalf("pruning must never lower the finalized height, got %d", machine.floorHeight)
	}
}

func TestProposalsOutsideTheHeightWindowAreRejected(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	height := maxHeightWindow + 1
	parent := types.Hash{9}
	block := cluster.block(height, parent)
	err := machine.OnProposal(ctx, cluster.proposal("a", block, 1, cluster.qc(height-1, 0, parent), finality.TimeoutCert{}))
	if !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected a proposal above the window to be rejected, got %v", err)
	}
	if machine.blockTree.Len() != 0 {
		t.Fatal("a proposal outside the window was stored")
	}

	inside := cluster.block(maxHeightWindow, parent)
	// At the edge of the window the proposal passes the window check; the machine then needs its
	// parent to derive the validator set of the block.
	if err := machine.OnProposal(ctx, cluster.proposal("a", inside, 1, cluster.qc(maxHeightWindow-1, 0, parent), finality.TimeoutCert{})); !errors.Is(err, ErrUnknownParent) {
		t.Fatalf("expected a proposal at the edge of the window to pass the window check, got %v", err)
	}
}

func TestNothingIsInsertedBelowTheFinalizedHeight(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 4)
	if machine.floorHeight != 3 {
		t.Fatalf("setup: expected height 3 to be final, got %d", machine.floorHeight)
	}
	stored := machine.blockTree.Len()

	// A block for a finalized height, in a view long after, is stale whatever it contains.
	old := cluster.block(2, blocks[0].hash, "late")
	timeout := cluster.timeoutCert(2, 9, blocks[0].qc, "a", "b", "c")
	if err := machine.OnProposal(ctx, cluster.proposal("a", old, 10, blocks[0].qc, timeout)); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected a proposal below the finalized height to be stale, got %v", err)
	}
	// A different block at the finalized height itself contradicts finality.
	rival := cluster.block(3, blocks[1].hash, "rival")
	rivalTimeout := cluster.timeoutCert(3, 9, blocks[1].qc, "a", "b", "c")
	if err := machine.OnProposal(ctx, cluster.proposal("b", rival, 10, blocks[1].qc, rivalTimeout)); !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("expected a conflicting block at the finalized height to be unsafe, got %v", err)
	}
	if machine.blockTree.Len() != stored {
		t.Fatal("a block at or below the finalized height was stored")
	}
	if err := machine.OnVote(ctx, cluster.vote("a", 2, 11, blocks[1].hash)); !errors.Is(err, ErrStaleVote) {
		t.Fatalf("expected a vote for a finalized height to be stale, got %v", err)
	}
}

func TestRejectedVotesDoNotGrowTheVoteTables(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	cluster.extendChain(machine, 1)
	baseline := len(machine.votes)

	for index := 0; index < 300; index++ {
		hash := types.Hash{byte(index), byte(index >> 8), 1}
		// Unknown block in the current window.
		if err := machine.OnVote(ctx, cluster.vote("a", 2, machine.Status(ctx).Round, hash)); !errors.Is(err, ErrInvalidVote) {
			t.Fatalf("expected an unknown target to be rejected, got %v", err)
		}
		// Heights and views far beyond the window.
		if err := machine.OnVote(ctx, cluster.vote("b", 10_000+types.Height(index), 10_000, hash)); !errors.Is(err, ErrOutOfWindow) {
			t.Fatalf("expected a far vote to be rejected, got %v", err)
		}
	}
	if len(machine.votes) != baseline || len(machine.votedVotes) != baseline {
		t.Fatalf("rejected votes grew the vote tables: %d / %d, expected %d", len(machine.votes), len(machine.votedVotes), baseline)
	}

	// A conflicting vote is evidence and is not stored as a second vote.
	conflicting := cluster.vote("a", 1, 0, types.Hash{0xAB})
	if err := machine.OnVote(ctx, conflicting); !errors.Is(err, ErrConflictingVote) {
		t.Fatalf("expected equivocation to be detected, got %v", err)
	}
	if _, stored := machine.votes[1][0][types.Hash{0xAB}]; stored {
		t.Fatal("the conflicting vote was stored")
	}
	if len(machine.Evidence()) != 1 {
		t.Fatalf("expected equivocation evidence, got %d", len(machine.Evidence()))
	}
}

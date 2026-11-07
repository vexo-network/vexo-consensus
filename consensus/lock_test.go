package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

// TestLockComparesViewsNotHeights is the regression for the lock rule that ignored rounds.
// The machine locks on a certificate formed in view 4 at height 2. A proposal justified by a
// certificate formed in view 2 at height 3 has the higher height but the older view: a
// height-only comparison accepted it, which let a validator vote for a branch that a
// quorum had already moved past.
func TestLockComparesViewsNotHeights(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(first)
	firstQC := cluster.qc(1, 0, firstHash)
	if err := machine.OnProposal(ctx, cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	// Branch Y is certified in views 4 and 5 after a timeout of view 3.
	secondY := cluster.blockAt(2, firstHash, 4, "y")
	secondYHash := HashBlock(secondY)
	timeoutThree := cluster.timeoutCert(2, 3, firstQC, "a", "b", "c")
	if err := machine.OnProposal(ctx, cluster.proposal("b", secondY, 4, firstQC, timeoutThree)); err != nil {
		t.Fatal(err)
	}
	secondYQC := cluster.qc(2, 4, secondYHash)
	thirdY := cluster.block(3, secondYHash, "y")
	if err := machine.OnProposal(ctx, cluster.proposal("c", thirdY, 5, secondYQC, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	locked := machine.SafetyState().LockedQC
	if locked.BlockHash != secondYHash || locked.Round != 4 {
		t.Fatalf("expected the lock on the view 4 certificate, got %+v", locked)
	}

	// Branch X was certified in views 1 and 2, at heights 2 and 3.
	secondX := cluster.blockAt(2, firstHash, 1, "x")
	secondXHash := HashBlock(secondX)
	thirdX := cluster.blockAt(3, secondXHash, 2, "x")
	thirdXHash := HashBlock(thirdX)
	thirdXQC := cluster.qc(3, 2, thirdXHash)
	// The machine learns branch X late; the views are past, so it votes for none of it.
	if err := machine.OnProposal(ctx, cluster.proposal("b", secondX, 1, firstQC, finality.TimeoutCert{})); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected the late branch to be stale, got %v", err)
	}
	if err := machine.OnProposal(ctx, cluster.proposal("c", thirdX, 2, cluster.qc(2, 1, secondXHash), finality.TimeoutCert{})); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected the late branch to be stale, got %v", err)
	}
	timeoutFive := cluster.timeoutCert(4, 5, thirdXQC, "a", "b", "c")
	fourthX := cluster.block(4, thirdXHash)
	proposal := cluster.proposal("a", fourthX, 6, thirdXQC, timeoutFive)
	if thirdXQC.Height <= locked.Height || thirdXQC.Round >= locked.Round {
		t.Fatal("the scenario needs a justify qc with the higher height and the older view")
	}

	if machine.IsSafeProposal(proposal) {
		t.Fatal("a justify qc from an older view than the lock must not be safe")
	}
	if err := machine.OnProposal(ctx, proposal); !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("expected an unsafe proposal, got %v", err)
	}
	state := machine.SafetyState()
	if state.LastVotedRound != 5 || state.LockedQC.BlockHash != secondYHash {
		t.Fatalf("an unsafe proposal must not change the lock or the last voted view, got %+v", state)
	}
}

func TestLockMovesToTheNewerViewEvenAtALowerHeight(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(first)
	firstQC := cluster.qc(1, 0, firstHash)
	if err := machine.OnProposal(ctx, cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	// The second block is proposed after a timeout of view 1, so its certificate and the first
	// one are not in consecutive views and nothing is finalized.
	second := cluster.blockAt(2, firstHash, 2)
	secondHash := HashBlock(second)
	secondQC := cluster.qc(2, 2, secondHash)
	timeoutOne := cluster.timeoutCert(2, 1, firstQC, "a", "b", "c")
	if err := machine.OnProposal(ctx, cluster.proposal("c", second, 2, firstQC, timeoutOne)); err != nil {
		t.Fatal(err)
	}
	third := cluster.block(3, secondHash)
	if err := machine.OnProposal(ctx, cluster.proposal("a", third, 3, secondQC, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	if locked := machine.SafetyState().LockedQC; locked.Round != 2 || locked.Height != 2 {
		t.Fatalf("expected the lock on the view 2 certificate for height 2, got %+v", locked)
	}

	// A certificate for height 1 formed in view 6 is newer than the lock, so a proposal
	// justified by it is safe and moves the lock down to the lower height.
	rival := cluster.blockAt(1, types.Hash{}, 6, "rival")
	rivalHash := HashBlock(rival)
	rivalQC := cluster.qc(1, 6, rivalHash)
	timeoutFive := cluster.timeoutCert(1, 5, finality.QuorumCert{}, "a", "b", "c")
	if err := machine.OnProposal(ctx, cluster.proposal("a", rival, 6, finality.QuorumCert{}, timeoutFive)); !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("a genesis proposal does not justify beyond the lock, got %v", err)
	}
	child := cluster.block(2, rivalHash)
	if err := machine.OnProposal(ctx, cluster.proposal("b", child, 7, rivalQC, finality.TimeoutCert{})); err != nil {
		t.Fatalf("a justify qc from a newer view must be safe, got %v", err)
	}
	if locked := machine.SafetyState().LockedQC; locked.Round != 6 || locked.Height != 1 {
		t.Fatalf("expected the lock to follow the newer view, got %+v", locked)
	}
}

func TestMachineDoesNotVoteTwiceInAView(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	first := cluster.block(1, types.Hash{})
	proposal := cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	if err := machine.OnProposal(ctx, proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(ctx, proposal); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("a proposal in the view already voted in must not earn a second vote, got %v", err)
	}
	rival := cluster.blockAt(1, types.Hash{}, 6, "rival")
	if err := machine.OnProposal(ctx, cluster.proposal("a", rival, 0, finality.QuorumCert{}, finality.TimeoutCert{})); !errors.Is(err, ErrConflictingProposal) {
		t.Fatalf("a second block in one view must be refused, got %v", err)
	}
	if state := machine.SafetyState(); state.LastVotedHeight != 1 || state.LastVotedRound != 0 {
		t.Fatalf("unexpected last voted view %+v", state)
	}
}

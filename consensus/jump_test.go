package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

// A proposal must extend the certificate it carries by exactly one height. Before the fix
// a proposal could name any height and drag the machine, and the node loop behind it, there.
func TestProposalCannotJumpHeights(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(first)
	genuineFirst := cluster.qc(1, 0, firstHash)

	cases := []struct {
		name     string
		height   types.Height
		justify  finality.QuorumCert
		expected error
	}{
		{"1000 above genesis without a certificate", 1000, finality.QuorumCert{}, ErrInvalidProposal},
		{"1000 above a genuine certificate for height 1", 1000, genuineFirst, ErrInvalidProposal},
		{"skipping one height above a genuine certificate", 3, genuineFirst, ErrInvalidProposal},
		{"extending a genuine certificate by one height needs the parent hash", 2, genuineFirst, ErrInvalidProposal},
		{"2 without any certificate", 2, finality.QuorumCert{}, ErrInvalidProposal},
		{"a forged certificate for height 99", 100, cluster.garbageQC(99, 98, types.Hash{9}), ErrInvalidQuorumCert},
		{"a forged certificate for height 999", 1000, cluster.garbageQC(999, 998, types.Hash{9}), ErrOutOfWindow},
		{"a genuine certificate for a far height", 1000, cluster.qc(999, 998, types.Hash{9}), ErrOutOfWindow},
		{"a height that would overflow", types.Height(1<<63 + 5), cluster.qc(1<<63+4, 0, types.Hash{9}), ErrInvalidProposal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			machine := cluster.newMachine()
			ctx := context.Background()
			before := machine.Status(ctx)

			parent := types.Hash{9}
			if testCase.justify.Height == 1 {
				parent = firstHash
			}
			block := cluster.block(testCase.height, parent)
			// The parent hash of the named block stays a mismatch for the "needs the
			// parent hash" case: the proposal skips the real parent of height 2.
			if testCase.name == "extending a genuine certificate by one height needs the parent hash" {
				block.Header.PreviousBlockHash = types.Hash{0x77}
			}
			round := testCase.justify.Round + 1
			err := machine.OnProposal(ctx, cluster.proposal("a", block, round, testCase.justify, finality.TimeoutCert{}))
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
			if after := machine.Status(ctx); after.Height != before.Height || after.Round != before.Round {
				t.Fatalf("a rejected proposal moved the machine from %+v to %+v", before, after)
			}
			if machine.blockTree.Len() != 0 {
				t.Fatal("a rejected proposal was stored")
			}
		})
	}
}

func TestProposalHeightFollowsTheVerifiedCertificate(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 3)

	highQC := machine.HighQC(ctx)
	if highQC.Height != 3 {
		t.Fatalf("expected the high qc at height 3, got %+v", highQC)
	}
	status := machine.Status(ctx)
	if status.Height > highQC.Height+1 {
		t.Fatalf("the machine moved to height %d beyond the high qc height %d plus one", status.Height, highQC.Height)
	}

	// Height 5 cannot be reached from the certificate for height 3.
	jump := cluster.block(5, blocks[2].hash)
	err := machine.OnProposal(ctx, cluster.proposal("a", jump, 3, blocks[2].qc, finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected the height jump to be rejected, got %v", err)
	}
	if machine.Status(ctx).Height > highQC.Height+1 {
		t.Fatal("a rejected height jump moved the machine")
	}
}

// A view above the successor of the justify certificate's view needs proof that a quorum
// gave up on the view before it. Before the fix a proposal could jump to any round.
func TestProposalCannotJumpViews(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(first)
	firstQC := cluster.qc(1, 0, firstHash)
	second := cluster.block(2, firstHash)

	genuineTimeout := cluster.timeoutCert(2, 999, firstQC, "a", "b", "c")
	forgedTimeout := genuineTimeout
	forgedTimeout.Signature = types.AggregateSignature(`[{"height":2,"signers":["a","b","c"],"signature":"Z2FyYmFnZQ=="}]`)
	subQuorumTimeout := newEqualTestCluster(t, "a").timeoutCert(2, 999, firstQC, "a")
	otherViewTimeout := cluster.timeoutCert(2, 5, firstQC, "a", "b", "c")
	sameViewTimeout := cluster.timeoutCert(2, 1000, firstQC, "a", "b", "c")
	newerThanJustify := cluster.timeoutCert(2, 999, cluster.qc(2, 5, types.Hash{5}), "a", "b", "c")

	cases := []struct {
		name     string
		round    types.Round
		timeout  finality.TimeoutCert
		expected error
	}{
		{"round 1000 without a timeout certificate", 1000, finality.TimeoutCert{}, ErrInvalidTimeoutCert},
		{"a timeout certificate for another view", 1000, otherViewTimeout, ErrInvalidTimeoutCert},
		{"a timeout certificate for the proposal view itself", 1000, sameViewTimeout, ErrInvalidTimeoutCert},
		{"a forged timeout certificate", 1000, forgedTimeout, ErrInvalidTimeoutCert},
		{"a sub-quorum timeout certificate", 1000, subQuorumTimeout, ErrInvalidTimeoutCert},
		{"a timeout certificate on the direct successor view", 1, genuineTimeout, ErrInvalidTimeoutCert},
		{"a timeout certificate that proves a newer qc than the justify qc", 1000, newerThanJustify, ErrInvalidProposal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			machine := cluster.newMachine()
			ctx := context.Background()
			if err := machine.OnProposal(ctx, cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
				t.Fatal(err)
			}
			before := machine.Status(ctx)

			err := machine.OnProposal(ctx, cluster.proposal("b", second, testCase.round, firstQC, testCase.timeout))
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
			if after := machine.Status(ctx); after.Round != before.Round || after.Height != before.Height {
				t.Fatalf("a rejected view jump moved the machine from %+v to %+v", before, after)
			}
			if machine.blockTree.Len() != 1 {
				t.Fatal("a rejected view jump stored the block")
			}
		})
	}
}

func TestProposalJumpsViewsOnlyWithAVerifiedTimeoutCertificate(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(first)
	firstQC := cluster.qc(1, 0, firstHash)
	if err := machine.OnProposal(ctx, cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	timeout := cluster.timeoutCert(2, 999, firstQC, "a", "b", "c")
	second := cluster.block(2, firstHash)
	if err := machine.OnProposal(ctx, cluster.proposal("b", second, 1000, firstQC, timeout)); err != nil {
		t.Fatalf("the verified timeout certificate must license the jump, got %v", err)
	}
	if status := machine.Status(ctx); status.Round != 1000 || status.Height != 2 {
		t.Fatalf("expected the machine at height 2, view 1000, got %+v", status)
	}
}

func TestVoteAndTimeoutViewsStayWithinTheLookahead(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 1)

	farVote := cluster.vote("a", 1, maxViewLookahead+100, blocks[0].hash)
	if err := machine.OnVote(ctx, farVote); !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected a far-future vote to be out of the window, got %v", err)
	}
	farTimeout := cluster.timeoutVote("a", 2, maxViewLookahead+100, blocks[0].qc)
	if _, err := machine.OnTimeoutVote(ctx, farTimeout); !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected a far-future timeout vote to be out of the window, got %v", err)
	}
	if status := machine.Status(ctx); status.Round > 2 {
		t.Fatalf("far-future votes moved the view to %d", status.Round)
	}
	if len(machine.votes) > 1 || len(machine.timeouts.votes) != 0 {
		t.Fatal("far-future votes were stored")
	}
}

package consensus

import (
	"errors"
	"math"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func twoChainCandidate() CommitCandidate {
	return CommitCandidate{
		BlockHeight:     1,
		BlockHash:       types.Hash{1},
		BlockQC:         finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}},
		ChildHeight:     2,
		ChildHash:       types.Hash{2},
		ChildParentHash: types.Hash{1},
		ChildQC:         finality.QuorumCert{Height: 2, Round: 1, BlockHash: types.Hash{2}},
	}
}

func TestTwoChainCommitRuleFinalizesTheParentOfACertifiedChild(t *testing.T) {
	decision, err := TwoChainCommitRule{}.Decide(twoChainCandidate())
	if err != nil {
		t.Fatal(err)
	}
	if decision.CommittedBlockHash != (types.Hash{1}) {
		t.Fatalf("expected the parent committed, got %x", decision.CommittedBlockHash)
	}
	if decision.CommittedHeight != 1 {
		t.Fatalf("expected committed height 1, got %d", decision.CommittedHeight)
	}
	if decision.CommitQC.BlockHash != (types.Hash{1}) {
		t.Fatalf("expected the commit qc to be the parent's, got %+v", decision.CommitQC)
	}
}

func TestTwoChainCommitRuleRequiresConsecutiveViews(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CommitCandidate)
	}{
		{"missing hashes", func(candidate *CommitCandidate) { *candidate = CommitCandidate{} }},
		{"missing block hash", func(candidate *CommitCandidate) { candidate.BlockHash = types.Hash{} }},
		{"missing child hash", func(candidate *CommitCandidate) { candidate.ChildHash = types.Hash{} }},
		{"zero heights", func(candidate *CommitCandidate) { candidate.BlockHeight, candidate.ChildHeight = 0, 0 }},
		{"child is not a child of the block", func(candidate *CommitCandidate) { candidate.ChildParentHash = types.Hash{9} }},
		{"block qc certifies another block", func(candidate *CommitCandidate) { candidate.BlockQC.BlockHash = types.Hash{9} }},
		{"block qc is for another height", func(candidate *CommitCandidate) { candidate.BlockQC.Height = 5 }},
		{"child qc certifies another block", func(candidate *CommitCandidate) { candidate.ChildQC.BlockHash = types.Hash{9} }},
		{"child qc is for another height", func(candidate *CommitCandidate) { candidate.ChildQC.Height = 5 }},
		{"child height skips a height", func(candidate *CommitCandidate) {
			candidate.ChildHeight = 3
			candidate.ChildQC.Height = 3
		}},
		{"views skip a view", func(candidate *CommitCandidate) { candidate.ChildQC.Round = 2 }},
		{"both certificates in one view", func(candidate *CommitCandidate) { candidate.ChildQC.Round = 0 }},
		{"child certified before the block", func(candidate *CommitCandidate) { candidate.BlockQC.Round = 5 }},
		{"view counter would wrap", func(candidate *CommitCandidate) {
			candidate.BlockQC.Round = math.MaxUint64
			candidate.ChildQC.Round = 0
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := twoChainCandidate()
			testCase.mutate(&candidate)
			_, err := TwoChainCommitRule{}.Decide(candidate)
			if !errors.Is(err, ErrCommitRuleNotSatisfied) {
				t.Fatalf("expected commit rule not satisfied, got %v", err)
			}
		})
	}
}

func commitCandidate(parent, child testBlock) CommitCandidate {
	return CommitCandidate{
		BlockHeight:     parent.block.Header.Height,
		BlockHash:       parent.hash,
		BlockQC:         parent.qc,
		ChildHeight:     child.block.Header.Height,
		ChildHash:       child.hash,
		ChildParentHash: parent.hash,
		ChildQC:         child.qc,
	}
}

func TestStateMachineApplyCommitRuleFinalizesWithVerifiedCertificates(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	blocks := cluster.chain(2, 0)
	cluster.feed(machine, blocks)
	if len(machine.CommitDecisions()) != 0 {
		t.Fatal("expected nothing final before the child is certified")
	}

	decision, err := machine.ApplyCommitRule(commitCandidate(blocks[0], blocks[1]))
	if err != nil {
		t.Fatal(err)
	}
	if decision.CommittedBlockHash != blocks[0].hash || decision.CommittedHeight != 1 {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if machine.Status(nil).LastFinalized != blocks[0].hash {
		t.Fatal("expected last finalized to update")
	}
	decisions := machine.CommitDecisions()
	if len(decisions) != 1 || decisions[0].CommittedBlockHash != blocks[0].hash {
		t.Fatalf("expected exactly the parent to be final, got %+v", decisions)
	}
}

func TestStateMachineApplyCommitRuleRejectsUnverifiedCertificates(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	blocks := cluster.chain(2, 0)

	cases := []struct {
		name   string
		mutate func(*CommitCandidate)
	}{
		{"forged block qc", func(candidate *CommitCandidate) {
			candidate.BlockQC = cluster.garbageQC(1, 0, blocks[0].hash)
		}},
		{"forged child qc", func(candidate *CommitCandidate) {
			candidate.ChildQC = cluster.garbageQC(2, 1, blocks[1].hash)
		}},
		{"sub-quorum child qc", func(candidate *CommitCandidate) {
			candidate.ChildQC = cluster.qc(2, 1, blocks[1].hash, "a")
		}},
		{"child qc without signature", func(candidate *CommitCandidate) {
			candidate.ChildQC.Signature = nil
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			machine := cluster.newMachine()
			cluster.feed(machine, blocks)
			candidate := commitCandidate(blocks[0], blocks[1])
			testCase.mutate(&candidate)

			if _, err := machine.ApplyCommitRule(candidate); !errors.Is(err, ErrInvalidQuorumCert) {
				t.Fatalf("expected invalid quorum certificate, got %v", err)
			}
			if len(machine.CommitDecisions()) != 0 || machine.Status(nil).LastFinalized != (types.Hash{}) {
				t.Fatal("a forged certificate must not finalize anything")
			}
		})
	}
}

func TestStateMachineApplyCommitRuleRejectsCertificatesFromNonConsecutiveViews(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	blocks := cluster.chain(2, 0)
	cluster.feed(machine, blocks[:1])

	// Both certificates are genuine, but the child was proposed and certified two views
	// after its parent, so a quorum has not locked the parent yet.
	late := blocks[1]
	late.block = cluster.seal(late.block, 2, blocks[0].round)
	late.hash = HashBlock(late.block)
	late.round = 2
	late.qc = cluster.qc(2, 2, late.hash)
	if _, err := machine.ApplyCommitRule(commitCandidate(blocks[0], late)); !errors.Is(err, ErrCommitRuleNotSatisfied) {
		t.Fatalf("expected commit rule not satisfied, got %v", err)
	}
	if len(machine.CommitDecisions()) != 0 {
		t.Fatal("non-consecutive views must not finalize")
	}
}

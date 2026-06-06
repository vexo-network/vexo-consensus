package consensus

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestThreeChainCommitRuleCommitsGrandparent(t *testing.T) {
	decision, err := ThreeChainCommitRule{}.Decide(CommitCandidate{
		BlockHeight:       3,
		BlockHash:         types.Hash{3},
		ParentHeight:      2,
		ParentHash:        types.Hash{2},
		GrandparentHeight: 1,
		GrandparentHash:   types.Hash{1},
		BlockQC:           finality.QuorumCert{Height: 2, BlockHash: types.Hash{2}},
		ParentQC:          finality.QuorumCert{Height: 1, BlockHash: types.Hash{1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.CommittedBlockHash != (types.Hash{1}) {
		t.Fatalf("expected grandparent committed, got %x", decision.CommittedBlockHash)
	}
	if decision.CommittedHeight != 1 {
		t.Fatalf("expected committed height 1, got %d", decision.CommittedHeight)
	}
}

func TestThreeChainCommitRuleRejectsInvalidCandidates(t *testing.T) {
	cases := []struct {
		name      string
		candidate CommitCandidate
	}{
		{
			name: "missing hashes",
		},
		{
			name: "block qc does not justify parent",
			candidate: CommitCandidate{
				BlockHeight:       3,
				BlockHash:         types.Hash{3},
				ParentHeight:      2,
				ParentHash:        types.Hash{2},
				GrandparentHeight: 1,
				GrandparentHash:   types.Hash{1},
				BlockQC:           finality.QuorumCert{Height: 2, BlockHash: types.Hash{9}},
				ParentQC:          finality.QuorumCert{Height: 1, BlockHash: types.Hash{1}},
			},
		},
		{
			name: "parent qc does not justify grandparent",
			candidate: CommitCandidate{
				BlockHeight:       3,
				BlockHash:         types.Hash{3},
				ParentHeight:      2,
				ParentHash:        types.Hash{2},
				GrandparentHeight: 1,
				GrandparentHash:   types.Hash{1},
				BlockQC:           finality.QuorumCert{Height: 2, BlockHash: types.Hash{2}},
				ParentQC:          finality.QuorumCert{Height: 1, BlockHash: types.Hash{9}},
			},
		},
		{
			name: "non consecutive qcs",
			candidate: CommitCandidate{
				BlockHeight:       3,
				BlockHash:         types.Hash{3},
				ParentHeight:      2,
				ParentHash:        types.Hash{2},
				GrandparentHeight: 1,
				GrandparentHash:   types.Hash{1},
				BlockQC:           finality.QuorumCert{Height: 5, BlockHash: types.Hash{2}},
				ParentQC:          finality.QuorumCert{Height: 1, BlockHash: types.Hash{1}},
			},
		},
		{
			name: "parent qc height does not match grandparent",
			candidate: CommitCandidate{
				BlockHeight:       3,
				BlockHash:         types.Hash{3},
				ParentHeight:      2,
				ParentHash:        types.Hash{2},
				GrandparentHeight: 1,
				GrandparentHash:   types.Hash{1},
				BlockQC:           finality.QuorumCert{Height: 2, BlockHash: types.Hash{2}},
				ParentQC:          finality.QuorumCert{Height: 2, BlockHash: types.Hash{1}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := ThreeChainCommitRule{}.Decide(testCase.candidate)
			if !errors.Is(err, ErrCommitRuleNotSatisfied) {
				t.Fatalf("expected commit rule not satisfied, got %v", err)
			}
		})
	}
}

func TestStateMachineApplyCommitRule(t *testing.T) {
	set := newTestValidatorSet(nil)
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, err := machine.ApplyCommitRule(CommitCandidate{
		BlockHeight:       3,
		BlockHash:         types.Hash{3},
		ParentHeight:      2,
		ParentHash:        types.Hash{2},
		GrandparentHeight: 1,
		GrandparentHash:   types.Hash{1},
		BlockQC:           finality.QuorumCert{Height: 2, BlockHash: types.Hash{2}},
		ParentQC:          finality.QuorumCert{Height: 1, BlockHash: types.Hash{1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.CommittedBlockHash != (types.Hash{1}) {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if machine.Status(nil).LastFinalized != (types.Hash{1}) {
		t.Fatal("expected last finalized to update")
	}
	if len(machine.CommitDecisions()) != 1 {
		t.Fatal("expected one commit decision")
	}
}

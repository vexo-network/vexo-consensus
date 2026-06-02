package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineCreateProposalSortsTxsForFairOrdering(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	proposal, err := machine.CreateProposal(types.Block{
		Header: types.Header{Height: 1},
		Txs:    []types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")},
	}, 0, "a", machine.blockTree.HighQC())
	if err != nil {
		t.Fatal(err)
	}
	if !fairordering.IsOrderedWithSalt(proposal.Block.Txs, fairordering.HeightSalt("vexo-test", 1)) {
		t.Fatalf("expected ordered proposal txs, got %q", proposal.Block.Txs)
	}
	if proposal.Block.Header.ConsensusHash != dataavailability.Commitment(proposal.Block.Txs) {
		t.Fatal("expected DA commitment over ordered txs")
	}
}

func TestStateMachineRejectsReorderedProposalTxs(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	ordered := fairordering.SortTxsWithSalt(
		[]types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")},
		fairordering.HeightSalt("vexo-test", 1),
	)
	reordered := []types.Tx{ordered[1], ordered[0], ordered[2]}
	err = machine.OnProposal(context.Background(), Proposal{
		Block: dataavailability.AttachCommitment(types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()},
			Txs:    reordered,
		}),
		Proposer: "a",
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

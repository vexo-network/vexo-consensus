package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineCreateProposalAttachesDataCommitment(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	proposal, err := machine.CreateProposal(types.Block{
		Header: types.Header{Height: 1},
		Txs:    []types.Tx{[]byte("tx")},
	}, 0, "a", machine.blockTree.HighQC())
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Block.Header.ConsensusHash != dataavailability.Commitment(proposal.Block.Txs) {
		t.Fatal("expected proposal to attach data availability commitment")
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRejectsProposalWithMissingDataCommitment(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()},
			Txs:    []types.Tx{[]byte("tx")},
		},
		Proposer: "a",
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestStateMachineRejectsProposalWithWrongDataCommitment(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           1,
				ValidatorSetHash: set.Hash(),
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("other")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Proposer: "a",
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestInvalidProposalHashEvidenceVerifiesReasonSpecificProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{2},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}

	evidence, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), types.Hash{1}, types.Hash{2})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidence(evidence); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidProposalHashEvidenceRejectsActualHashNotInProposal(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{9},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}

	_, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), types.Hash{1}, types.Hash{2})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal proof, got %v", err)
	}
}

func TestInvalidProposalHashEvidenceRejectsMissingMismatch(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
		},
		Proposer: "validator-1",
	}

	_, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonAppHash), types.Hash{1}, types.Hash{1})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal proof, got %v", err)
	}
}

func TestInvalidProposalTxValidityEvidenceRequiresDeterministicMismatch(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("bad-tx")},
		},
		Proposer: "validator-1",
	}
	actual := txSetHash(proposal.Block.Txs)

	_, err := NewInvalidProposalTxValidityEvidence(proposal, types.Hash{1}, actual, "ante rejected tx")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewInvalidProposalTxValidityEvidence(proposal, actual, actual, "ante rejected tx")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid matching tx validity proof, got %v", err)
	}
	_, err = NewInvalidProposalTxValidityEvidence(proposal, types.Hash{1}, actual, "")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid missing verification message, got %v", err)
	}
	_, err = NewInvalidProposalTxValidityEvidence(proposal, types.Hash{1}, types.Hash{2}, "wrong actual tx set hash")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid unbound tx set hash, got %v", err)
	}
}

func TestInvalidProposalRejectsUnsupportedStateRootAndSignatureReasons(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
		},
		Proposer: "validator-1",
	}
	_, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonStateRoot), types.Hash{1}, types.Hash{2})
	if !errors.Is(err, ErrUnsupportedProposalReason) {
		t.Fatalf("expected unsupported state root reason, got %v", err)
	}
	_, err = NewInvalidProposalSignatureEvidence(proposal, "invalid signature")
	if !errors.Is(err, ErrUnsupportedProposalReason) {
		t.Fatalf("expected unsupported proposer signature reason, got %v", err)
	}
}

func TestInvalidProposalTimestampEvidenceBindsActualToProposal(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7, TimeUnixNano: 200},
		},
		Proposer: "validator-1",
	}

	_, err := NewInvalidProposalTimestampEvidence(proposal, 100, 300)
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected timestamp proof to bind actual proposal time, got %v", err)
	}
	if _, err := NewInvalidProposalTimestampEvidence(proposal, 100, 200); err != nil {
		t.Fatal(err)
	}
}

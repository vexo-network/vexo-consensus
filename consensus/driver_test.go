package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStepDriverDrivesThreeBlockFinality(t *testing.T) {
	machine, driver := newTestDriver(t)
	machine.StartRound(1, 0)

	first, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 2}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	third, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 3}}, "a")
	if err != nil {
		t.Fatal(err)
	}

	if second.Proposal.JustifyQC.BlockHash != first.BlockHash {
		t.Fatal("expected second proposal to justify first block")
	}
	if third.Proposal.JustifyQC.BlockHash != second.BlockHash {
		t.Fatal("expected third proposal to justify second block")
	}
	if machine.Status(context.Background()).LastFinalized != first.BlockHash {
		t.Fatal("expected first block to finalize through three-chain rule")
	}
	if len(machine.CommitDecisions()) != 1 {
		t.Fatal("expected one commit decision")
	}
}

func TestStepDriverTimeoutFeedsNextProposalHighQC(t *testing.T) {
	machine, driver := newTestDriver(t)
	machine.StartRound(1, 0)

	first, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(2, 0)

	timeoutCert, err := driver.TimeoutQuorum(context.Background(), first.QuorumQC)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != first.BlockHash {
		t.Fatalf("expected timeout cert to carry first block qc, got %+v", timeoutCert.HighQC)
	}
	if machine.Status(context.Background()).Round != 1 {
		t.Fatal("expected timeout quorum to advance round")
	}

	proposed, err := driver.Propose(context.Background(), types.Block{Header: types.Header{Height: 2}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if proposed.Proposal.Round != 1 {
		t.Fatal("expected proposal in advanced round")
	}
	if proposed.Proposal.JustifyQC.BlockHash != first.BlockHash {
		t.Fatal("expected proposal to use timeout high qc")
	}
}

func TestStepDriverRejectsForkWithMismatchedHighQC(t *testing.T) {
	machine, driver := newTestDriver(t)
	machine.StartRound(1, 0)

	if _, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a"); err != nil {
		t.Fatal(err)
	}

	_, err := driver.Propose(context.Background(), types.Block{Header: types.Header{
		Height:            2,
		PreviousBlockHash: types.Hash{9},
	}}, "a")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestStepDriverRejectsEmptyValidatorSet(t *testing.T) {
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: newTestValidatorSet(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewStepDriver(machine); !errors.Is(err, ErrNoValidators) {
		t.Fatalf("expected no validators, got %v", err)
	}
}

func TestStepDriverReturnsNoQuorumWithInsufficientPower(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 10},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 1}}, 0, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}

	err = machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: machine.hashBlock(proposal.Block), ValidatorID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, machine.hashBlock(proposal.Block)); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
}

func newTestDriver(t *testing.T) (*StateMachine, *StepDriver) {
	t.Helper()
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}
	driver, err := NewStepDriver(machine)
	if err != nil {
		t.Fatal(err)
	}
	return machine, driver
}

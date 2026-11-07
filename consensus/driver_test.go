package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStepDriverDrivesTwoChainFinality(t *testing.T) {
	machine, driver := newTestDriver(t)
	machine.StartRound(1, 0)

	first, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(machine.CommitDecisions()) != 0 {
		t.Fatal("a block with a single certificate must not be final")
	}
	second, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 2}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if second.Proposal.JustifyQC.BlockHash != first.BlockHash {
		t.Fatal("expected second proposal to justify first block")
	}
	if machine.Status(context.Background()).LastFinalized != first.BlockHash {
		t.Fatal("expected first block to finalize through the two-chain rule")
	}
	if len(machine.CommitDecisions()) != 1 {
		t.Fatal("expected one commit decision")
	}

	third, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 3}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if third.Proposal.JustifyQC.BlockHash != second.BlockHash {
		t.Fatal("expected third proposal to justify second block")
	}
	if machine.Status(context.Background()).LastFinalized != second.BlockHash {
		t.Fatal("expected second block to finalize")
	}
	if len(machine.CommitDecisions()) != 2 {
		t.Fatal("expected two commit decisions")
	}
}

func TestStepDriverTimeoutFeedsNextProposalHighQC(t *testing.T) {
	machine, driver := newTestDriver(t)
	machine.StartRound(1, 0)

	first, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := driver.TimeoutQuorum(context.Background(), first.QuorumQC)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != first.BlockHash {
		t.Fatalf("expected timeout cert to carry first block qc, got %+v", timeoutCert.HighQC)
	}
	if machine.Status(context.Background()).Round != 2 {
		t.Fatal("expected timeout quorum to advance the view past the certified one")
	}

	proposed, err := driver.Propose(context.Background(), types.Block{Header: types.Header{Height: 2}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if proposed.Proposal.Round != 2 {
		t.Fatal("expected proposal in advanced view")
	}
	if proposed.Proposal.JustifyQC.BlockHash != first.BlockHash {
		t.Fatal("expected proposal to use timeout high qc")
	}
	if proposed.Proposal.TimeoutCert.Round != 1 {
		t.Fatalf("expected the proposal to carry the timeout certificate of view 1, got %+v", proposed.Proposal.TimeoutCert)
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
	cluster := newTestCluster(t)
	machine := cluster.newMachine()
	if _, err := NewStepDriver(machine, cluster.signers); !errors.Is(err, ErrNoValidators) {
		t.Fatalf("expected no validators, got %v", err)
	}
}

func TestStepDriverRequiresASignerPerValidator(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(1, 0)
	driver, err := NewStepDriver(machine, SimulationSigners{"a": cluster.signers["a"]})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := driver.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}}, "b"); !errors.Is(err, ErrMissingSigner) {
		t.Fatalf("expected a proposer without a key to be refused, got %v", err)
	}
	proposed, err := driver.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.VoteQuorum(context.Background(), proposed.BlockHash); !errors.Is(err, ErrMissingSigner) {
		t.Fatalf("expected a voter without a key to be refused, got %v", err)
	}
}

func TestStepDriverReturnsNoQuorumWithInsufficientPower(t *testing.T) {
	cluster := newTestCluster(t,
		validator.Validator{ID: "a", VotingPower: 1},
		validator.Validator{ID: "b", VotingPower: 1},
		validator.Validator{ID: "c", VotingPower: 10},
	)
	machine := cluster.newMachine()
	machine.StartRound(1, 0)

	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 1}}, 0, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}

	blockHash := machine.hashBlock(proposal.Block)
	if err := machine.OnVote(context.Background(), cluster.vote("a", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
}

func newTestDriver(t *testing.T) (*StateMachine, *StepDriver) {
	t.Helper()
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	driver, err := NewStepDriver(machine, cluster.signers)
	if err != nil {
		t.Fatal(err)
	}
	return machine, driver
}

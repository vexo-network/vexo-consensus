package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineUpdatesValidatorSetFromRegistryAfterSlashing(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
		{ID: "b", Address: "b", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: initialSet,
	})
	if err != nil {
		t.Fatal(err)
	}
	initialHash := machine.Status(context.Background()).ValidatorSetHash

	evidence, err := NewConflictingVoteEvidence(
		testVote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.50", JailDuration: 10},
	})
	if _, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence); err != nil {
		t.Fatal(err)
	}
	if err := machine.UpdateValidatorSetFromRegistry(context.Background(), registry, 2); err != nil {
		t.Fatal(err)
	}

	updatedHash := machine.Status(context.Background()).ValidatorSetHash
	if updatedHash == initialHash {
		t.Fatal("expected validator set hash to change after slashing")
	}
	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 0, "b", machine.blockTree.HighQC())
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Block.Header.ValidatorSetHash != updatedHash {
		t.Fatal("expected proposal to use updated validator set hash")
	}
}

func TestStateMachineRejectsProposalWithOldValidatorSetHashAfterUpdate(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
		{ID: "b", Address: "b", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: initialSet,
	})
	if err != nil {
		t.Fatal(err)
	}
	oldHash := initialSet.Hash()

	if err := registry.UpdateVotingPower(context.Background(), "a", 50); err != nil {
		t.Fatal(err)
	}
	if err := machine.UpdateValidatorSetFromRegistry(context.Background(), registry, 2); err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block: types.Block{Header: types.Header{
			ChainID:          "vexo-test",
			Height:           2,
			ValidatorSetHash: oldHash,
		}},
		Proposer: "b",
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestStateMachineValidatorRotationAllowsNewValidatorAndRejectsRemovedValidator(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(validator.NewConfigurableAdmissionPolicy(validator.AdmissionConfig{
		Permissionless: true,
		MinStake:       1,
	}), []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
		{ID: "b", Address: "b", VotingPower: 1, Stake: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: initialSet,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := registry.ApplyJoin(context.Background(), validator.Candidate{Address: "c", Stake: 1}); err != nil {
		t.Fatal(err)
	}
	if err := registry.ApplyLeave(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if err := machine.UpdateValidatorSetFromRegistry(context.Background(), registry, 2); err != nil {
		t.Fatal(err)
	}

	if _, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 0, "c", machine.blockTree.HighQC()); err != nil {
		t.Fatalf("expected new validator proposer accepted, got %v", err)
	}
	if _, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 0, "a", machine.blockTree.HighQC()); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected removed validator rejected, got %v", err)
	}
}

func TestStateMachineUpdateValidatorSetResetsTimeoutCollector(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
		{ID: "b", Address: "b", VotingPower: 1, Stake: 1},
		{ID: "c", Address: "c", VotingPower: 1, Stake: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: initialSet,
	})
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)
	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 0, ValidatorID: "a"}); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := registry.ApplyLeave(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if err := machine.UpdateValidatorSetFromRegistry(context.Background(), registry, 2); err != nil {
		t.Fatal(err)
	}

	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 0, ValidatorID: "a"}); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected removed timeout voter rejected, got %v", err)
	}
}

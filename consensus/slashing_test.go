package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestSubmitEvidenceForSlashingReducesValidatorPower(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	evidence, err := NewConflictingVoteEvidence(
		testVote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected slash result: %+v", result)
	}
	if keeper.EvidenceCount() != 1 || keeper.PenaltyCount() != 1 {
		t.Fatalf("expected keeper records, got evidence=%d penalty=%d", keeper.EvidenceCount(), keeper.PenaltyCount())
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	validatorInfo, found := set.Get("a")
	if !found {
		t.Fatal("expected validator")
	}
	if validatorInfo.VotingPower != 75 {
		t.Fatalf("expected updated voting power 75, got %d", validatorInfo.VotingPower)
	}
}

func TestSubmitEvidenceForSlashingRejectsTamperedEvidence(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		testVote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Validator = "b"

	_, err = SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence)
	if !errors.Is(err, ErrVotePairMismatch) {
		t.Fatalf("expected pair mismatch, got %v", err)
	}
	if keeper.EvidenceCount() != 0 {
		t.Fatal("tampered evidence must not be submitted")
	}
}

func TestSubmitEvidenceForSlashingRejectsUnknownValidator(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		testVote("b", 1, 0, types.Hash{1}),
		testVote("b", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence)
	if !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected unknown evidence validator, got %v", err)
	}
}

func TestSubmitEvidenceForSlashingRejectsDuplicateEvidence(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		testVote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence); !errors.Is(err, slashing.ErrDuplicateEvidence) {
		t.Fatalf("expected duplicate evidence, got %v", err)
	}
}

func TestSubmitTimeoutEvidenceForSlashingReducesValidatorPower(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceConflictingTimeoutVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	evidence, err := NewConflictingTimeoutVoteEvidence(
		TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}},
		TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected slash result: %+v", result)
	}
}

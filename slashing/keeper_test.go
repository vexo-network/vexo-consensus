package slashing

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestInMemoryKeeperSubmitEvidence(t *testing.T) {
	keeper := NewInMemoryKeeper(nil)
	evidence := validEvidence(EvidenceDoubleSign)

	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if keeper.EvidenceCount() != 1 {
		t.Fatalf("expected evidence count 1, got %d", keeper.EvidenceCount())
	}
}

func TestInMemoryKeeperRejectsDuplicateEvidence(t *testing.T) {
	keeper := NewInMemoryKeeper(nil)
	evidence := validEvidence(EvidenceConflictingVote)

	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if err := keeper.SubmitEvidence(context.Background(), evidence); !errors.Is(err, ErrDuplicateEvidence) {
		t.Fatalf("expected duplicate evidence, got %v", err)
	}
}

func TestInMemoryKeeperRejectsInvalidEvidence(t *testing.T) {
	keeper := NewInMemoryKeeper(nil)
	cases := []struct {
		name     string
		evidence Evidence
		expected error
	}{
		{
			name:     "missing validator",
			evidence: Evidence{Type: EvidenceDoubleSign, Height: 1, Proof: []byte("proof")},
			expected: ErrMissingValidator,
		},
		{
			name:     "missing height",
			evidence: Evidence{Type: EvidenceDoubleSign, Validator: "alice", Proof: []byte("proof")},
			expected: ErrMissingHeight,
		},
		{
			name:     "empty proof",
			evidence: Evidence{Type: EvidenceDoubleSign, Validator: "alice", Height: 1},
			expected: ErrEmptyProof,
		},
		{
			name:     "unknown type",
			evidence: Evidence{Type: "unknown", Validator: "alice", Height: 1, Proof: []byte("proof")},
			expected: ErrUnknownEvidenceType,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := keeper.SubmitEvidence(context.Background(), testCase.evidence)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}

func TestInMemoryKeeperAppliesConfiguredPenalty(t *testing.T) {
	keeper := NewInMemoryKeeper(PenaltyPolicy{
		EvidenceDoubleSign: {SlashFraction: "0.25", JailDuration: 10},
	})

	penalty, err := keeper.ApplyPenalty(context.Background(), validEvidence(EvidenceDoubleSign))
	if err != nil {
		t.Fatal(err)
	}
	if penalty.SlashFraction != "0.25" || penalty.JailDuration != 10 {
		t.Fatalf("unexpected penalty: %+v", penalty)
	}
}

func TestInMemoryKeeperSubmitAndApplyRecordsReceipt(t *testing.T) {
	keeper := NewInMemoryKeeper(PenaltyPolicy{
		EvidenceConflictingVote: {SlashFraction: "0.10", JailDuration: 20},
	})
	evidence := validEvidence(EvidenceConflictingVote)

	receipt, err := keeper.SubmitAndApply(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if keeper.EvidenceCount() != 1 || keeper.PenaltyCount() != 1 {
		t.Fatalf("expected one evidence and penalty, got evidence=%d penalty=%d", keeper.EvidenceCount(), keeper.PenaltyCount())
	}
	if receipt.Penalty.SlashFraction != "0.10" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	stored, found := keeper.PenaltyReceipt(evidence)
	if !found {
		t.Fatal("expected stored penalty receipt")
	}
	if stored.Evidence.Validator != evidence.Validator || stored.Penalty.JailDuration != 20 {
		t.Fatalf("unexpected stored receipt: %+v", stored)
	}
}

func TestInMemoryKeeperSubmitAndApplyRejectsDuplicate(t *testing.T) {
	keeper := NewInMemoryKeeper(nil)
	evidence := validEvidence(EvidenceConflictingVote)
	if _, err := keeper.SubmitAndApply(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.SubmitAndApply(context.Background(), evidence); !errors.Is(err, ErrDuplicateEvidence) {
		t.Fatalf("expected duplicate evidence, got %v", err)
	}
}

func TestApplySlash(t *testing.T) {
	remaining, err := ApplySlash(100, Penalty{SlashFraction: "0.05"})
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 95 {
		t.Fatalf("expected 95 remaining power, got %d", remaining)
	}
	remaining, err = ApplySlash(1, Penalty{SlashFraction: "0.50"})
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected minimum nonzero power, got %d", remaining)
	}
	remaining, err = ApplySlash(100, Penalty{SlashFraction: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected full slash, got %d", remaining)
	}
}

func TestApplySlashRejectsInvalidFraction(t *testing.T) {
	cases := []Penalty{
		{SlashFraction: "invalid"},
		{SlashFraction: "-0.1"},
		{SlashFraction: "1.1"},
	}
	for _, penalty := range cases {
		if _, err := ApplySlash(100, penalty); !errors.Is(err, ErrInvalidSlashFraction) {
			t.Fatalf("expected invalid slash fraction for %+v, got %v", penalty, err)
		}
	}
}

func TestInMemoryKeeperRejectsEvidenceWithoutPenalty(t *testing.T) {
	keeper := NewInMemoryKeeper(PenaltyPolicy{
		EvidenceDoubleSign: {SlashFraction: "0.25", JailDuration: 10},
	})

	err := keeper.ValidateEvidence(context.Background(), validEvidence(EvidenceUnavailableData))
	if !errors.Is(err, ErrUnknownEvidenceType) {
		t.Fatalf("expected unknown evidence type for unconfigured policy, got %v", err)
	}
}

func TestInMemoryKeeperCopiesEvidenceProof(t *testing.T) {
	keeper := NewInMemoryKeeper(nil)
	evidence := validEvidence(EvidenceInvalidProposal)
	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	evidence.Proof[0] = 'X'

	penalty, err := keeper.ApplyPenalty(context.Background(), validEvidence(EvidenceInvalidProposal))
	if err != nil {
		t.Fatal(err)
	}
	if penalty.SlashFraction == "" {
		t.Fatal("expected default penalty")
	}
}

func TestInMemoryKeeperContextCancellation(t *testing.T) {
	keeper := NewInMemoryKeeper(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := keeper.SubmitEvidence(ctx, validEvidence(EvidenceDoubleSign)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected submit canceled, got %v", err)
	}
	if err := keeper.ValidateEvidence(ctx, validEvidence(EvidenceDoubleSign)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected validate canceled, got %v", err)
	}
	if _, err := keeper.ApplyPenalty(ctx, validEvidence(EvidenceDoubleSign)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected penalty canceled, got %v", err)
	}
}

func validEvidence(evidenceType EvidenceType) Evidence {
	return Evidence{
		Type:      evidenceType,
		Validator: types.ValidatorID("alice"),
		Height:    1,
		Round:     2,
		Proof:     []byte("proof"),
	}
}

package finality

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestAttackDetectorReportsConflictingFinalityProofs(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
		{ID: "d", VotingPower: 1, PublicKey: []byte("d-pub")},
	})
	first := validProof(set, []types.ValidatorID{"a", "b", "c"})
	second := conflictingProof(set, first, types.Hash{2}, []types.ValidatorID{"a", "b", "d"})

	detector := NewAttackDetector(set, acceptSignatureVerifier{})
	violation, err := detector.Observe(first)
	if err != nil {
		t.Fatal(err)
	}
	if violation != nil {
		t.Fatalf("unexpected violation for first proof: %+v", violation)
	}

	violation, err = detector.Observe(second)
	if err != nil {
		t.Fatal(err)
	}
	if violation == nil {
		t.Fatal("expected accountable safety violation")
	}
	if violation.Height != first.Header.Height || violation.FirstBlockHash == violation.SecondBlockHash {
		t.Fatalf("unexpected conflicting proof metadata: %+v", violation)
	}
	if violation.DoubleSignVotingPower != 2 || violation.TotalVotingPower != 4 || violation.FaultPowerThreshold != 2 {
		t.Fatalf("unexpected overlap power: %+v", violation)
	}
	if !violation.MeetsFaultThreshold() {
		t.Fatalf("expected overlap to meet fault threshold: %+v", violation)
	}
	expected := []types.ValidatorID{"a", "b"}
	if len(violation.DoubleSigners) != len(expected) {
		t.Fatalf("unexpected double signers: %+v", violation.DoubleSigners)
	}
	for index := range expected {
		if violation.DoubleSigners[index] != expected[index] {
			t.Fatalf("unexpected double signers: %+v", violation.DoubleSigners)
		}
	}
}

// Two certified blocks at one height are normal after a view change; only finality proofs conflict.
func TestAttackDetectorRejectsCertifiedBlocksWithoutChild(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
		{ID: "d", VotingPower: 1, PublicKey: []byte("d-pub")},
	})
	first := validProof(set, []types.ValidatorID{"a", "b", "c"})
	second := conflictingProof(set, first, types.Hash{2}, []types.ValidatorID{"a", "b", "d"})
	first.CommitChain = nil
	second.CommitChain = nil

	detector := NewAttackDetector(set, acceptSignatureVerifier{})
	if _, err := detector.Observe(first); !errors.Is(err, ErrCommitChainTooShort) {
		t.Fatalf("expected a certified block without its child to be rejected, got %v", err)
	}
	if _, err := detector.Observe(second); !errors.Is(err, ErrCommitChainTooShort) {
		t.Fatalf("expected a certified block without its child to be rejected, got %v", err)
	}
}

func TestVerifyConflictEvidenceRequiresFinalityProofsAndVerifier(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
		{ID: "d", VotingPower: 1, PublicKey: []byte("d-pub")},
	})
	first := validProof(set, []types.ValidatorID{"a", "b", "c"})
	second := conflictingProof(set, first, types.Hash{2}, []types.ValidatorID{"a", "b", "d"})
	evidence, err := NewConflictEvidence(set, first, second, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyConflictEvidence(StaticValidatorSets(set), nil, evidence); !errors.Is(err, ErrMissingSignatureVerifier) {
		t.Fatalf("expected evidence without a verifier to fail closed, got %v", err)
	}

	first.CommitChain = nil
	second.CommitChain = nil
	unfinal, err := NewConflictEvidence(set, first, second, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyConflictEvidence(StaticValidatorSets(set), acceptSignatureVerifier{}, unfinal); !errors.Is(err, ErrCommitChainTooShort) {
		t.Fatalf("expected evidence of certified-only blocks to be rejected, got %v", err)
	}
}

func TestAttackDetectorIgnoresSameFinalityBlock(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	detector := NewAttackDetector(set, acceptSignatureVerifier{})
	if violation, err := detector.Observe(proof); err != nil || violation != nil {
		t.Fatalf("first observe: violation=%+v err=%v", violation, err)
	}
	if violation, err := detector.Observe(proof); err != nil || violation != nil {
		t.Fatalf("second observe: violation=%+v err=%v", violation, err)
	}
}

func TestAttackDetectorRejectsInvalidProofBeforeCaching(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	invalid := validProof(set, []types.ValidatorID{"a", "b"})
	invalid.ValidatorSetHash = types.Hash{9}
	valid := validProof(set, []types.ValidatorID{"a", "b"})

	detector := NewAttackDetector(set, acceptSignatureVerifier{})
	if _, err := detector.Observe(invalid); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected validator set mismatch, got %v", err)
	}
	if violation, err := detector.Observe(valid); err != nil || violation != nil {
		t.Fatalf("valid proof should be first cached proof: violation=%+v err=%v", violation, err)
	}
}

func TestDetectAccountableSafetyViolationRejectsDifferentHeights(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	first := validProof(set, []types.ValidatorID{"a", "b"})
	second := validProof(set, []types.ValidatorID{"a", "c"})
	second.Header.Height = 2
	second.ValidatorSetHeight = 2
	second.QuorumCert.Height = 2

	_, err := DetectAccountableSafetyViolation(set, first, second)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected height mismatch, got %v", err)
	}
}

func TestRegistryAttackDetectorLoadsHeightSpecificSet(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
		{ID: "d", VotingPower: 1, PublicKey: []byte("d-pub")},
	})
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	first := validProof(set, []types.ValidatorID{"a", "b", "c"})
	second := conflictingProof(set, first, types.Hash{3}, []types.ValidatorID{"a", "b", "d"})

	detector := NewRegistryAttackDetector(registry, acceptSignatureVerifier{})
	if _, err := detector.Observe(first); err != nil {
		t.Fatal(err)
	}
	violation, err := detector.Observe(second)
	if err != nil {
		t.Fatal(err)
	}
	if violation == nil || violation.DoubleSignVotingPower != 2 {
		t.Fatalf("expected registry-backed violation, got %+v", violation)
	}
}

func TestNewConflictEvidenceRequiresAccountableSigner(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
		{ID: "d", VotingPower: 1, PublicKey: []byte("d-pub")},
	})
	first := validProof(set, []types.ValidatorID{"a", "b", "c"})
	second := conflictingProof(set, first, types.Hash{2}, []types.ValidatorID{"a", "b", "d"})

	evidence, err := NewConflictEvidence(set, first, second, "a")
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Type != "finality_conflict" || evidence.Validator != "a" || evidence.Height != 1 {
		t.Fatalf("unexpected evidence: %+v", evidence)
	}
	if _, err := VerifyConflictEvidence(StaticValidatorSets(set), acceptSignatureVerifier{}, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConflictEvidence(set, first, second, "c"); !errors.Is(err, ErrValidatorNotInFinalityConflict) {
		t.Fatalf("expected non-overlapping signer rejection, got %v", err)
	}
}

// conflictingProof is a complete proof for a different block at the same height as base, with the
// same certification view, signed by the given validators.
func conflictingProof(set validator.Set, base Proof, appHash types.Hash, signers []types.ValidatorID) Proof {
	return proofAt(set, base.Header.Height, appHash, base.QuorumCert.Round, signers)
}

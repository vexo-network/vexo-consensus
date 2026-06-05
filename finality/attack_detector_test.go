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
	if _, err := VerifyConflictEvidence(set, acceptSignatureVerifier{}, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConflictEvidence(set, first, second, "c"); !errors.Is(err, ErrValidatorNotInFinalityConflict) {
		t.Fatalf("expected non-overlapping signer rejection, got %v", err)
	}
}

func conflictingProof(set validator.Set, base Proof, blockHash types.Hash, signers []types.ValidatorID) Proof {
	proof := base
	proof.Header.AppHash = types.Hash{7}
	proof.Header.ConsensusHash = blockHash
	proof.BlockHash = blockHash
	proof.QuorumCert.BlockHash = blockHash
	proof.QuorumCert.Signers = EncodeSigners(signers)
	proof.QuorumCert.Signature = types.AggregateSignature("conflicting-signature")
	proof.QuorumCert.VotingPower = 0
	for _, signer := range signers {
		validatorInfo, found := set.Get(signer)
		if found {
			proof.QuorumCert.VotingPower += validatorInfo.VotingPower
		}
	}
	return proof
}

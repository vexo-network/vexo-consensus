package finality

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestVerifierAcceptsValidFinalityProof(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	verifier := NewVerifier(set, acceptSignatureVerifier{})
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierRejectsValidatorSetMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.ValidatorSetHash = types.Hash{9}

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected validator set mismatch, got %v", err)
	}
}

func TestVerifierRejectsBlockHashMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.QuorumCert.BlockHash = types.Hash{9}

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected block hash mismatch, got %v", err)
	}
}

func TestVerifierRejectsHeightMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.QuorumCert.Height = proof.Header.Height + 1

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected height mismatch, got %v", err)
	}
}

func TestVerifierRejectsMissingSignature(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.QuorumCert.Signature = nil

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected missing signature, got %v", err)
	}
}

func TestVerifierRejectsInsufficientQuorum(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a"})

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrInsufficientQuorum) {
		t.Fatalf("expected insufficient quorum, got %v", err)
	}
}

func TestVerifierRejectsUnknownSigner(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a", "unknown"})

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrUnknownSigner) {
		t.Fatalf("expected unknown signer, got %v", err)
	}
}

func TestVerifierRejectsDuplicateSigner(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a", "a", "b"})

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrDuplicateSigner) {
		t.Fatalf("expected duplicate signer, got %v", err)
	}
}

func TestVerifierRejectsVotingPowerMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.VotingPower = 3

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrVotingPowerMismatch) {
		t.Fatalf("expected voting power mismatch, got %v", err)
	}
}

func TestVerifierRejectsInvalidAggregateSignature(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	proof := validProof(set, []types.ValidatorID{"a"})

	err := NewVerifier(set, rejectSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestVerifierContextCancellation(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewVerifier(set, nil).VerifyFinalityProofWithContext(ctx, proof)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestParseSignersRejectsEmptySigner(t *testing.T) {
	_, err := ParseSigners(types.Bitmap("a,,b"))
	if !errors.Is(err, ErrEmptySigner) {
		t.Fatalf("expected empty signer, got %v", err)
	}
}

func TestHasQuorum(t *testing.T) {
	cases := []struct {
		power    types.VotingPower
		total    types.VotingPower
		expected bool
	}{
		{power: 0, total: 0, expected: false},
		{power: 1, total: 3, expected: false},
		{power: 2, total: 3, expected: true},
		{power: 6, total: 7, expected: true},
		{power: 4, total: 7, expected: false},
	}
	for _, testCase := range cases {
		if got := HasQuorum(testCase.power, testCase.total); got != testCase.expected {
			t.Fatalf("power %d total %d: expected %v, got %v", testCase.power, testCase.total, testCase.expected, got)
		}
	}
}

func validProof(set validator.Set, signers []types.ValidatorID) Proof {
	header := types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}
	proof := Proof{
		Header:             header,
		ValidatorSetHeight: header.Height,
		ValidatorSetHash:   set.Hash(),
	}
	proof.QuorumCert = QuorumCert{
		Height:      header.Height,
		Round:       0,
		Signers:     EncodeSigners(signers),
		Signature:   types.AggregateSignature("signature"),
		VotingPower: types.VotingPower(len(signers)),
	}
	proof.QuorumCert.BlockHash = proof.HeaderHash()
	return proof
}

func testValidatorSet(t *testing.T, validators []validator.Validator) validator.Set {
	t.Helper()
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

type acceptSignatureVerifier struct{}

func (acceptSignatureVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return true
}

type rejectSignatureVerifier struct{}

func (rejectSignatureVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return false
}

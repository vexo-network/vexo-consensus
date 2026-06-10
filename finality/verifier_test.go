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

func TestStrictVerifierRequiresCommitChain(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	err := NewStrictVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrCommitChainTooShort) {
		t.Fatalf("expected strict verifier to require commit chain, got %v", err)
	}
}

func TestStrictVerifierAcceptsThreeChainProof(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.CommitChain = validCommitChain(proof, []types.ValidatorID{"a", "b"})

	if err := NewStrictVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof); err != nil {
		t.Fatalf("expected strict verifier to accept 3-chain proof, got %v", err)
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

func validCommitChain(proof Proof, signers []types.ValidatorID) []CommitLink {
	firstHeader := types.Header{
		ChainID:           proof.Header.ChainID,
		Height:            proof.Header.Height + 1,
		PreviousBlockHash: proof.BlockHash,
		ValidatorSetHash:  proof.Header.ValidatorSetHash,
	}
	firstHash := HeaderHash(firstHeader)
	secondHeader := types.Header{
		ChainID:           proof.Header.ChainID,
		Height:            proof.Header.Height + 2,
		PreviousBlockHash: firstHash,
		ValidatorSetHash:  proof.Header.ValidatorSetHash,
	}
	secondHash := HeaderHash(secondHeader)
	return []CommitLink{
		{
			Header:    firstHeader,
			BlockHash: firstHash,
			QuorumCert: QuorumCert{
				Height:      proof.Header.Height,
				Round:       0,
				BlockHash:   proof.BlockHash,
				Signers:     EncodeSigners(signers),
				Signature:   types.AggregateSignature("signature"),
				VotingPower: types.VotingPower(len(signers)),
			},
		},
		{
			Header:    secondHeader,
			BlockHash: secondHash,
			QuorumCert: QuorumCert{
				Height:      firstHeader.Height,
				Round:       0,
				BlockHash:   firstHash,
				Signers:     EncodeSigners(signers),
				Signature:   types.AggregateSignature("signature"),
				VotingPower: types.VotingPower(len(signers)),
			},
		},
	}
}

func TestVerifierRejectsCommitChainHeaderHashMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.CommitChain = validCommitChain(proof, []types.ValidatorID{"a", "b"})
	proof.CommitChain[0].Header.AppHash = types.Hash{9}

	err := NewStrictVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected commit-chain header hash mismatch, got %v", err)
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

func TestVerifierAcceptsExplicitConsensusBlockHash(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.BlockHash = types.Hash{7}
	proof.QuorumCert.BlockHash = proof.BlockHash

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if err != nil {
		t.Fatalf("expected explicit block hash proof to verify, got %v", err)
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

func TestVerifierRejectsMissingValidatorSetHeight(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.ValidatorSetHeight = 0

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected height mismatch, got %v", err)
	}
}

func TestVerifierAcceptsHistoricalValidatorSetHeight(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.Header.Height = 10
	proof.ValidatorSetHeight = 7
	proof.QuorumCert.Height = proof.Header.Height
	proof.QuorumCert.BlockHash = proof.HeaderHash()
	proof.BlockHash = proof.QuorumCert.BlockHash

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if err != nil {
		t.Fatalf("expected historical validator set height to verify, got %v", err)
	}
}

func TestVerifierRejectsFutureValidatorSetHeight(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.ValidatorSetHeight = proof.Header.Height + 1

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected future validator set height rejection, got %v", err)
	}
}

func TestRegistryVerifierLoadsValidatorSetAtProofHeight(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	if err := NewRegistryVerifier(registry, acceptSignatureVerifier{}).VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryVerifierRejectsWrongHeightValidatorSet(t *testing.T) {
	correctSet := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	wrongSet := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 10, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(correctSet, []types.ValidatorID{"a", "b"})

	err := NewRegistryVerifier(staticRegistry{set: wrongSet}, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected validator set mismatch, got %v", err)
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

func TestVerifierRejectsVotingPowerOverflow(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: ^types.VotingPower(0)},
		{ID: "b", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, types.ErrVotingPowerOverflow) {
		t.Fatalf("expected voting power overflow, got %v", err)
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
		{power: ^types.VotingPower(0), total: ^types.VotingPower(0), expected: true},
		{power: ^types.VotingPower(0)/3*2 + 1, total: ^types.VotingPower(0), expected: true},
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
	proof.BlockHash = proof.QuorumCert.BlockHash
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

type staticRegistry struct {
	set validator.Set
}

func (registry staticRegistry) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	return registry.set, nil
}

func (registry staticRegistry) ApplyJoin(ctx context.Context, candidate validator.Candidate) (validator.Validator, error) {
	return validator.Validator{}, nil
}

func (registry staticRegistry) ApplyLeave(ctx context.Context, id types.ValidatorID) error {
	return nil
}

func (registry staticRegistry) UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error {
	return nil
}

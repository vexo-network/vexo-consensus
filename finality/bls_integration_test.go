package finality

import (
	"encoding/base64"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func blsValidator(id types.ValidatorID, key string, proof string) validator.Validator {
	return validator.Validator{
		ID:          id,
		VotingPower: 1,
		PublicKey:   []byte(key),
		Metadata: map[string]string{
			vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString([]byte(proof)),
		},
	}
}

func TestVerifierUsesBLSKeyRegistry(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		blsValidator("a", "a-bls", "a-pop"),
		blsValidator("b", "b-bls", "b-pop"),
		blsValidator("c", "c-bls", "c-pop"),
	})
	registry, err := vexocrypto.NewBLSKeyRegistry(finalityBLSAdapter{})
	if err != nil {
		t.Fatal(err)
	}

	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.Signature = []byte("aggregate")
	proof.CommitChain[0].QuorumCert.Signature = []byte("aggregate")
	verifier := NewVerifier(set, registry)

	// Keys whose proof of possession was never verified do not verify, whatever the signature.
	if err := verifier.VerifyFinalityProof(proof); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected unregistered bls keys to fail closed, got %v", err)
	}

	if err := registry.RegisterValidators(set.List()); err != nil {
		t.Fatal(err)
	}
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierRejectsBLSKeysWithoutProofOfPossession(t *testing.T) {
	registeredSet := testValidatorSet(t, []validator.Validator{
		blsValidator("a", "a-bls", "a-pop"),
		blsValidator("b", "b-bls", "b-pop"),
		blsValidator("c", "c-bls", "c-pop"),
	})
	registry, err := vexocrypto.NewBLSKeyRegistry(finalityBLSAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterValidators(registeredSet.List()); err != nil {
		t.Fatal(err)
	}

	// A set that swaps in a key nobody proved possession of cannot be registered, and its
	// proofs cannot be verified.
	rogueValidators := []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("mallory-bls")},
		blsValidator("b", "b-bls", "b-pop"),
		blsValidator("c", "c-bls", "c-pop"),
	}
	if err := registry.RegisterValidators(rogueValidators); !errors.Is(err, vexocrypto.ErrMissingBLSProof) {
		t.Fatalf("expected a key without proof of possession to be refused, got %v", err)
	}
	rogueSet := testValidatorSet(t, rogueValidators)
	rogueProof := validProof(rogueSet, []types.ValidatorID{"a", "b"})
	rogueProof.QuorumCert.Signature = []byte("aggregate")
	rogueProof.CommitChain[0].QuorumCert.Signature = []byte("aggregate")
	if err := NewVerifier(rogueSet, registry).VerifyFinalityProof(rogueProof); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected the unregistered bls key to fail finality verification, got %v", err)
	}
}

type finalityBLSAdapter struct{}

func (finalityBLSAdapter) PublicKey() types.PublicKey {
	return []byte("adapter-bls")
}

func (finalityBLSAdapter) Sign(message []byte) (types.Signature, error) {
	return append([]byte("bls:"), message...), nil
}

func (finalityBLSAdapter) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	return len(publicKey) > 0 && len(message) > 0 && len(signature) > 0
}

func (finalityBLSAdapter) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	return []byte("aggregate"), nil
}

func (finalityBLSAdapter) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return len(publicKeys) > 0 && len(message) > 0 && string(signature) == "aggregate"
}

func (finalityBLSAdapter) ValidatePublicKey(publicKey types.PublicKey) error {
	if len(publicKey) == 0 {
		return vexocrypto.ErrMissingBLSPublicKey
	}
	return nil
}

func (finalityBLSAdapter) VerifyProofOfPossession(publicKey types.PublicKey, proof types.Signature) bool {
	return len(publicKey) > 0 && len(proof) > 0
}

func (finalityBLSAdapter) Metadata() vexocrypto.BLSAdapterMetadata {
	return vexocrypto.BLSAdapterMetadata{
		Name:                  "finality-test-bls",
		Version:               "v1",
		Audited:               true,
		AuditReport:           "test-audit",
		DependencyAudit:       "test-dependency-audit",
		DomainSeparation:      true,
		PublicKeyValidation:   true,
		SubgroupChecks:        true,
		RogueKeyDefense:       true,
		DeterministicEncoding: true,
		MalformedInputFuzzed:  true,
		ProofOfPossession:     true,
	}
}

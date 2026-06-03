package finality

import (
	"encoding/base64"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestVerifierUsesBLSRegisteredKeyVerifier(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{
			ID:          "a",
			VotingPower: 1,
			PublicKey:   []byte("a-bls"),
			Metadata: map[string]string{
				vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString([]byte("a-pop")),
			},
		},
		{
			ID:          "b",
			VotingPower: 1,
			PublicKey:   []byte("b-bls"),
			Metadata: map[string]string{
				vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString([]byte("b-pop")),
			},
		},
		{
			ID:          "c",
			VotingPower: 1,
			PublicKey:   []byte("c-bls"),
			Metadata: map[string]string{
				vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString([]byte("c-pop")),
			},
		},
	})
	adapter := finalityBLSAdapter{}
	credentials, err := vexocrypto.ValidateBLSValidatorSet(adapter, set)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := vexocrypto.NewBLSAggregateVerifier(adapter, credentials)
	if err != nil {
		t.Fatal(err)
	}

	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.Signature = []byte("aggregate")
	if err := NewVerifier(set, verifier).VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}

	tamperedSet := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("mallory-bls")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-bls")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-bls")},
	})
	tamperedProof := validProof(tamperedSet, []types.ValidatorID{"a", "b"})
	tamperedProof.QuorumCert.Signature = []byte("aggregate")
	if err := NewVerifier(tamperedSet, verifier).VerifyFinalityProof(tamperedProof); err == nil {
		t.Fatal("expected unregistered bls key to fail finality verification")
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

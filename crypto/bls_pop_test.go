//go:build cgo

package crypto

import (
	"encoding/base64"
	"errors"
	"testing"

	blst "github.com/supranational/blst/bindings/go"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func newBLSTTestAdapter(t *testing.T, seed string) BLSTBLSAdapter {
	t.Helper()
	adapter, err := NewBLSTBLSAdapterFromSeed([]byte(seed))
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func blsTestValidator(id types.ValidatorID, publicKey types.PublicKey, proof types.Signature) validator.Validator {
	return validator.Validator{
		ID:          id,
		PublicKey:   publicKey,
		VotingPower: 1,
		Metadata:    map[string]string{BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString(proof)},
	}
}

func TestBLSTProofOfPossessionUsesDedicatedTag(t *testing.T) {
	adapter := newBLSTTestAdapter(t, "01234567890123456789012345678901")
	publicKey := adapter.PublicKey()
	proof, err := adapter.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	if !adapter.VerifyProofOfPossession(publicKey, proof) {
		t.Fatal("expected proof of possession to verify")
	}
	if adapter.Verify(publicKey, blsProofOfPossessionMessage(publicKey), proof) {
		t.Fatal("expected a proof of possession not to verify as a signature under the signing tag")
	}
	signature, err := adapter.Sign(blsProofOfPossessionMessage(publicKey))
	if err != nil {
		t.Fatal(err)
	}
	if adapter.VerifyProofOfPossession(publicKey, signature) {
		t.Fatal("expected a signing-tag signature over the proof message not to be accepted as a proof of possession")
	}
	other := newBLSTTestAdapter(t, "abcdefghijklmnopqrstuvwxyz123456")
	if adapter.VerifyProofOfPossession(other.PublicKey(), proof) {
		t.Fatal("expected a proof to be bound to its public key")
	}
}

func TestBLSProofOfPossessionIsInterchangeableBetweenAdapters(t *testing.T) {
	seed := []byte("01234567890123456789012345678901")
	blstAdapter := newBLSTTestAdapter(t, string(seed))
	blstProof, err := blstAdapter.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	circlAdapter, err := NewCIRCLBLSAdapterFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	circlProof, err := circlAdapter.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	if !circlAdapter.VerifyProofOfPossession(circlAdapter.PublicKey(), circlProof) {
		t.Fatal("expected CIRCL proof to verify under CIRCL")
	}
	if !blstAdapter.VerifyProofOfPossession(circlAdapter.PublicKey(), circlProof) {
		t.Fatal("expected CIRCL proof of possession to verify under BLST")
	}
	if !circlAdapter.VerifyProofOfPossession(blstAdapter.PublicKey(), blstProof) {
		t.Fatal("expected BLST proof of possession to verify under CIRCL")
	}
}

func TestValidateConsensusPublicKeyBLS(t *testing.T) {
	adapter := newBLSTTestAdapter(t, "01234567890123456789012345678901")
	other := newBLSTTestAdapter(t, "abcdefghijklmnopqrstuvwxyz123456")
	proof, err := adapter.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	otherProof, err := other.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	metadata := func(proof []byte) map[string]string {
		return map[string]string{BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString(proof)}
	}

	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, adapter.PublicKey(), metadata(proof)); err != nil {
		t.Fatalf("expected key with its own proof of possession to be admitted: %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, adapter.PublicKey(), nil); !errors.Is(err, ErrMissingBLSProof) {
		t.Fatalf("expected missing proof, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, adapter.PublicKey(), map[string]string{BLSProofOfPossessionMetadataKey: "%%%"}); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected undecodable proof to be invalid, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, adapter.PublicKey(), metadata(otherProof)); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected another key's proof to be invalid, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, adapter.PublicKey(), metadata(proof[:len(proof)-1])); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected truncated proof to be invalid, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, types.PublicKey("not-a-bls-key"), metadata(proof)); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected malformed key to be invalid, got %v", err)
	}
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, nil, nil); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected empty key to be invalid, got %v", err)
	}
	identity := make(types.PublicKey, blstBLSPublicKeyBytes)
	identity[0] = 0xc0
	if err := ValidateConsensusPublicKey(config.CryptoBackendBLS, identity, metadata(proof)); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected the identity point to be invalid, got %v", err)
	}
}

func TestBLSKeyRegistryVerifiesRealAggregatesOnlyForRegisteredKeys(t *testing.T) {
	left := newBLSTTestAdapter(t, "01234567890123456789012345678901")
	right := newBLSTTestAdapter(t, "abcdefghijklmnopqrstuvwxyz123456")
	message := []byte("vexo-finality-vote")
	leftSignature, err := left.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	rightSignature, err := right.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := left.Aggregate([]types.Signature{leftSignature, rightSignature})
	if err != nil {
		t.Fatal(err)
	}
	both := []types.PublicKey{left.PublicKey(), right.PublicKey()}

	suite, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier.VerifyAggregate(both, message, aggregate) {
		t.Fatal("expected keys without a registered proof of possession to fail")
	}

	leftProof, err := left.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.RegisterValidators([]validator.Validator{blsTestValidator("left", left.PublicKey(), leftProof)}); err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier.VerifyAggregate(both, message, aggregate) {
		t.Fatal("expected the aggregate with an unregistered key to fail")
	}

	rightProof, err := right.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.RegisterValidators([]validator.Validator{
		blsTestValidator("left", left.PublicKey(), leftProof),
		blsTestValidator("right", right.PublicKey(), rightProof),
	}); err != nil {
		t.Fatal(err)
	}
	if !suite.FinalityVerifier.VerifyAggregate(both, message, aggregate) {
		t.Fatal("expected the aggregate over registered keys to verify")
	}
	if suite.FinalityVerifier.VerifyAggregate(both, []byte("another message"), aggregate) {
		t.Fatal("expected the aggregate over another message to fail")
	}
}

func TestBLSKeyRegistryRejectsRogueKeyForgery(t *testing.T) {
	honest := newBLSTTestAdapter(t, "01234567890123456789012345678901")
	message := []byte("vexo-finality-vote")

	// The attacker holds secret x and publishes the rogue key x*G - honest, so that
	// honest + rogue = x*G and x alone signs for both.
	attackerSecret := blst.KeyGen([]byte("attacker-seed-attacker-seed-0123"), []byte(blstBLSKeyInfo))
	attackerPublic := new(blst.P1Affine).From(attackerSecret)
	honestPublic := new(blst.P1Affine).Uncompress(honest.PublicKey())
	var rogue blst.P1
	rogue.FromAffine(attackerPublic)
	rogue.SubAssign(honestPublic)
	rogueKey := types.PublicKey(rogue.ToAffine().Compress())
	forged := types.AggregateSignature(new(blst.P2Affine).Sign(attackerSecret, blsMessage(message), []byte(blstBLSMessageDST), true).Compress())
	forgedKeys := []types.PublicKey{honest.PublicKey(), rogueKey}

	if !NewBLSTBLSVerifierAdapter().VerifyAggregate(forgedKeys, message, forged) {
		t.Fatal("test premise: an unguarded aggregate verifier accepts the rogue-key forgery")
	}

	suite, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
	if err != nil {
		t.Fatal(err)
	}
	honestProof, err := honest.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.RegisterValidators([]validator.Validator{blsTestValidator("honest", honest.PublicKey(), honestProof)}); err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier.VerifyAggregate(forgedKeys, message, forged) {
		t.Fatal("expected the rogue-key forgery to fail against the registry")
	}

	// The attacker cannot prove possession of the rogue key's secret.
	attackerProof := new(blst.P2Affine).Sign(attackerSecret, blsProofOfPossessionMessage(rogueKey), []byte(blsProofOfPossessionDST), true).Compress()
	err = suite.RegisterValidators([]validator.Validator{
		blsTestValidator("honest", honest.PublicKey(), honestProof),
		blsTestValidator("rogue", rogueKey, attackerProof),
	})
	if !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected the rogue key's proof of possession to be rejected, got %v", err)
	}
	if suite.FinalityVerifier.VerifyAggregate(forgedKeys, message, forged) {
		t.Fatal("expected the forgery to keep failing after the rejected registration")
	}
}

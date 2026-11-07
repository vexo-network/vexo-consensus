package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestCIRCLBLSAdapterSignsVerifiesAndAggregates(t *testing.T) {
	left, err := NewCIRCLBLSAdapterFromSeed([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewCIRCLBLSAdapterFromSeed([]byte("abcdefghijklmnopqrstuvwxyz123456"))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vexo-finality-vote")
	leftSignature, err := left.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	rightSignature, err := right.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !left.Verify(left.PublicKey(), message, leftSignature) {
		t.Fatal("expected BLS signature to verify")
	}
	aggregated, err := left.Aggregate([]types.Signature{leftSignature, rightSignature})
	if err != nil {
		t.Fatal(err)
	}
	if !left.VerifyAggregate([]types.PublicKey{left.PublicKey(), right.PublicKey()}, message, aggregated) {
		t.Fatal("expected aggregate signature to verify")
	}
	if left.VerifyAggregate([]types.PublicKey{left.PublicKey(), left.PublicKey()}, message, aggregated) {
		t.Fatal("expected duplicate public keys to be rejected")
	}
}

func TestCIRCLBLSProofOfPossessionAndKeyDocument(t *testing.T) {
	adapter, err := NewCIRCLBLSAdapterFromSeed([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	proof, err := adapter.ProofOfPossession()
	if err != nil {
		t.Fatal(err)
	}
	if !adapter.VerifyProofOfPossession(adapter.PublicKey(), proof) {
		t.Fatal("expected proof of possession to verify")
	}
	document, err := NewCIRCLBLSKeyDocument(adapter)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := document.CIRCLBLSSigner()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.VerifyProofOfPossession(loaded.PublicKey(), proof) {
		t.Fatal("expected loaded BLS key to verify proof")
	}
}

func TestCIRCLBLSVerifierAdapterRejectsSigning(t *testing.T) {
	adapter := NewCIRCLBLSVerifierAdapter()
	if _, err := adapter.Sign([]byte("message")); !errors.Is(err, ErrMissingBLSPrivateKey) {
		t.Fatalf("expected missing private key, got %v", err)
	}
	if err := ValidateBLSAdapter(adapter); !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected unaudited built-in adapter to be unsafe, got %v", err)
	}
}

func TestCIRCLBLSProofOfPossessionUsesDedicatedTag(t *testing.T) {
	adapter, err := NewCIRCLBLSAdapterFromSeed([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewCIRCLBLSAdapterFromSeed([]byte("abcdefghijklmnopqrstuvwxyz123456"))
	if err != nil {
		t.Fatal(err)
	}
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
	if adapter.VerifyProofOfPossession(other.PublicKey(), proof) {
		t.Fatal("expected a proof to be bound to its public key")
	}
	if adapter.VerifyProofOfPossession(publicKey, append(append(types.Signature(nil), proof...), 0)) {
		t.Fatal("expected a proof with trailing bytes to be rejected")
	}
	if adapter.VerifyProofOfPossession(publicKey, proof[:len(proof)-1]) {
		t.Fatal("expected a truncated proof to be rejected")
	}
}

func TestCIRCLBLSRejectsNonCanonicalLengths(t *testing.T) {
	adapter, err := NewCIRCLBLSAdapterFromSeed([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vexo-finality-vote")
	signature, err := adapter.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := adapter.PublicKey()
	longKey := append(append(types.PublicKey(nil), publicKey...), 0)
	if err := adapter.ValidatePublicKey(longKey); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected public key with trailing bytes to be rejected, got %v", err)
	}
	longSignature := append(append(types.Signature(nil), signature...), 0)
	if adapter.Verify(publicKey, message, longSignature) {
		t.Fatal("expected signature with trailing bytes to be rejected")
	}
	aggregate, err := adapter.Aggregate([]types.Signature{signature})
	if err != nil {
		t.Fatal(err)
	}
	if !adapter.VerifyAggregate([]types.PublicKey{publicKey}, message, aggregate) {
		t.Fatal("expected single-signer aggregate to verify")
	}
	if adapter.VerifyAggregate([]types.PublicKey{publicKey}, message, append(append(types.AggregateSignature(nil), aggregate...), 0)) {
		t.Fatal("expected aggregate with trailing bytes to be rejected")
	}
}

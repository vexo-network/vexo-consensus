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
	if err := ValidateBLSAdapter(adapter); err != nil {
		t.Fatal(err)
	}
}

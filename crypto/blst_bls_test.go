//go:build cgo

package crypto

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestBLSTBLSAdapterSignsVerifiesAndAggregates(t *testing.T) {
	left, err := NewBLSTBLSAdapterFromSeed([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewBLSTBLSAdapterFromSeed([]byte("abcdefghijklmnopqrstuvwxyz123456"))
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
	if len(left.PublicKey()) != blstBLSPublicKeyBytes || len(leftSignature) != blstBLSSignatureBytes {
		t.Fatalf("unexpected BLST key/signature sizes: pk=%d sig=%d", len(left.PublicKey()), len(leftSignature))
	}
	if !left.Verify(left.PublicKey(), message, leftSignature) {
		t.Fatal("expected BLST BLS signature to verify")
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
	if left.VerifyAggregate([]types.PublicKey{left.PublicKey(), right.PublicKey()}, []byte("wrong"), aggregated) {
		t.Fatal("expected aggregate signature to reject wrong message")
	}
}

func TestBLSTBLSProofOfPossessionAndKeyDocument(t *testing.T) {
	adapter, err := NewBLSTBLSAdapterFromSeed([]byte("01234567890123456789012345678901"))
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
	document, err := NewBLSTBLSKeyDocument(adapter)
	if err != nil {
		t.Fatal(err)
	}
	if document.Metadata.BLSAdapter != BLSAdapterBLSTName {
		t.Fatalf("expected BLST metadata, got %+v", document.Metadata)
	}
	loaded, err := document.BLSTBLSSigner()
	if err != nil {
		t.Fatal(err)
	}
	decodedProof, err := base64.StdEncoding.DecodeString(document.Metadata.BLSProofOfPossession)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.VerifyProofOfPossession(loaded.PublicKey(), decodedProof) {
		t.Fatal("expected loaded BLS key to verify proof")
	}
}

func TestBLSTBLSProductionMetadataIsSafe(t *testing.T) {
	adapter := NewBLSTBLSVerifierAdapter()
	if _, err := adapter.Sign([]byte("message")); !errors.Is(err, ErrMissingBLSPrivateKey) {
		t.Fatalf("expected missing private key, got %v", err)
	}
	if err := ValidateBLSAdapter(adapter); err != nil {
		t.Fatalf("expected BLST adapter metadata to satisfy production checks: %v", err)
	}
}

func TestBLSTBLSRejectsMalformedInputs(t *testing.T) {
	adapter := NewBLSTBLSVerifierAdapter()
	if err := adapter.ValidatePublicKey(types.PublicKey("bad")); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected invalid public key, got %v", err)
	}
	if adapter.Verify(types.PublicKey("bad"), []byte("message"), types.Signature("bad")) {
		t.Fatal("expected malformed signature verification to fail")
	}
}

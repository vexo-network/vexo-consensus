package crypto

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestEd25519SignerSignsAndVerifies(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("hello")

	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(signer.PublicKey(), message, signature) {
		t.Fatal("expected ed25519 signature to verify")
	}
}

func TestEd25519SignerRejectsWrongMessageKeyAndSignature(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	other, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}

	signature, err := signer.Sign([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if signer.Verify(signer.PublicKey(), []byte("bye"), signature) {
		t.Fatal("wrong message verified")
	}
	if signer.Verify(other.PublicKey(), []byte("hello"), signature) {
		t.Fatal("wrong public key verified")
	}
	if signer.Verify(signer.PublicKey(), []byte("hello"), types.Signature("bad")) {
		t.Fatal("invalid signature verified")
	}
}

func TestNewEd25519SignerRejectsInvalidKeys(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEd25519Signer(nil, publicKey); !errors.Is(err, ErrInvalidEd25519PrivateKey) {
		t.Fatalf("expected invalid private key, got %v", err)
	}
	if _, err := NewEd25519Signer(privateKey, nil); !errors.Is(err, ErrInvalidEd25519PublicKey) {
		t.Fatalf("expected invalid public key, got %v", err)
	}

	otherPublicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEd25519Signer(privateKey, otherPublicKey); !errors.Is(err, ErrInvalidEd25519PublicKey) {
		t.Fatalf("expected mismatched public key, got %v", err)
	}
}

func TestEd25519PublicKeyIsCopy(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	publicKey := signer.PublicKey()
	publicKey[0] ^= 0xff
	if string(publicKey) == string(signer.PublicKey()) {
		t.Fatal("expected public key copy")
	}
}

func TestEd25519MultiVerifier(t *testing.T) {
	first, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")

	firstSignature, err := first.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	secondSignature, err := second.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := CombineEd25519Signatures([]types.Signature{firstSignature, secondSignature})
	if err != nil {
		t.Fatal(err)
	}

	verifier := Ed25519MultiVerifier{}
	if !verifier.VerifyAggregate([]types.PublicKey{first.PublicKey(), second.PublicKey()}, message, combined) {
		t.Fatal("expected combined ed25519 signatures to verify")
	}
	if verifier.VerifyAggregate([]types.PublicKey{first.PublicKey(), second.PublicKey()}, []byte("wrong"), combined) {
		t.Fatal("wrong message verified")
	}
	if verifier.VerifyAggregate([]types.PublicKey{first.PublicKey()}, message, combined) {
		t.Fatal("wrong signer count verified")
	}
}

func TestCombineEd25519SignaturesRejectsInvalidInputs(t *testing.T) {
	if _, err := CombineEd25519Signatures(nil); !errors.Is(err, ErrEmptySignature) {
		t.Fatalf("expected empty signature, got %v", err)
	}
	if _, err := CombineEd25519Signatures([]types.Signature{[]byte("bad")}); !errors.Is(err, ErrEmptySignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

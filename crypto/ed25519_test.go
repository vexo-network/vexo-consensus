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

func TestEd25519SignerVerifyRejectsMalformedKeysAndSignaturesWithoutPanicking(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	for name, malformed := range map[string]types.PublicKey{
		"nil":   nil,
		"short": signer.PublicKey()[:ed25519.PublicKeySize-1],
		"long":  append(signer.PublicKey(), 0),
	} {
		if signer.Verify(malformed, message, signature) {
			t.Fatalf("%s public key verified", name)
		}
	}
	for name, malformed := range map[string]types.Signature{
		"nil":   nil,
		"short": signature[:ed25519.SignatureSize-1],
		"long":  append(append(types.Signature(nil), signature...), 0),
	} {
		if signer.Verify(signer.PublicKey(), message, malformed) {
			t.Fatalf("%s signature verified", name)
		}
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

func TestEd25519MultiVerifierRejectsMalformedPublicKeysWithoutPanicking(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := CombineEd25519Signatures([]types.Signature{signature, signature})
	if err != nil {
		t.Fatal(err)
	}

	verifier := Ed25519MultiVerifier{}
	for name, malformed := range map[string]types.PublicKey{
		"nil":        nil,
		"short":      make(types.PublicKey, ed25519.PublicKeySize-1),
		"long":       make(types.PublicKey, ed25519.PublicKeySize+1),
		"all zeroes": make(types.PublicKey, ed25519.PublicKeySize),
	} {
		t.Run(name, func(t *testing.T) {
			if verifier.VerifyAggregate([]types.PublicKey{signer.PublicKey(), malformed}, message, combined) {
				t.Fatal("expected malformed public key to fail verification")
			}
			if verifier.VerifyAggregate([]types.PublicKey{malformed, signer.PublicKey()}, message, combined) {
				t.Fatal("expected malformed public key to fail verification")
			}
		})
	}
}

func TestEd25519MultiVerifierRejectsDuplicatePublicKeys(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := CombineEd25519Signatures([]types.Signature{signature, signature})
	if err != nil {
		t.Fatal(err)
	}
	if (Ed25519MultiVerifier{}).VerifyAggregate([]types.PublicKey{signer.PublicKey(), signer.PublicKey()}, message, combined) {
		t.Fatal("expected one signer counted twice to fail verification")
	}
}

package crypto

import (
	"errors"
	"testing"
)

func TestKeyRingRotatesActiveSigner(t *testing.T) {
	firstSigner, err := NewDeterministicSigner([]byte("first-validator-key"))
	if err != nil {
		t.Fatalf("new first signer: %v", err)
	}
	secondSigner, err := NewDeterministicSigner([]byte("second-validator-key"))
	if err != nil {
		t.Fatalf("new second signer: %v", err)
	}

	keyRing, err := NewKeyRing(
		KeyRecord{ID: "key-1", Signer: firstSigner, ActiveFrom: 1, ActiveUntil: 10},
		KeyRecord{ID: "key-2", Signer: secondSigner, ActiveFrom: 11},
	)
	if err != nil {
		t.Fatalf("new key ring: %v", err)
	}
	if keyRing.ActiveKeyID() != "key-1" {
		t.Fatalf("expected key-1 active, got %q", keyRing.ActiveKeyID())
	}

	if err := keyRing.Activate("key-2"); err != nil {
		t.Fatalf("activate key-2: %v", err)
	}
	activeSigner, err := keyRing.ActiveSigner()
	if err != nil {
		t.Fatalf("active signer: %v", err)
	}

	message := []byte("rotation-check")
	signature, err := activeSigner.Sign(message)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !activeSigner.Verify(secondSigner.PublicKey(), message, signature) {
		t.Fatal("expected signature from rotated active signer to verify")
	}
	if activeSigner.Verify(firstSigner.PublicKey(), message, signature) {
		t.Fatal("expected rotated signature to reject previous key")
	}
}

func TestKeyRingRejectsInvalidRecords(t *testing.T) {
	signer, err := NewDeterministicSigner([]byte("validator-key"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	keyRing, err := NewKeyRing(KeyRecord{ID: "key-1", Signer: signer})
	if err != nil {
		t.Fatalf("new key ring: %v", err)
	}
	if err := keyRing.Add(KeyRecord{ID: "key-1", Signer: signer}); !errors.Is(err, ErrKeyAlreadyExists) {
		t.Fatalf("expected duplicate key error, got %v", err)
	}
	if err := keyRing.Add(KeyRecord{Signer: signer}); !errors.Is(err, ErrEmptyKeyID) {
		t.Fatalf("expected empty key id error, got %v", err)
	}
	if err := keyRing.Add(KeyRecord{ID: "key-2"}); !errors.Is(err, ErrNilSigner) {
		t.Fatalf("expected nil signer error, got %v", err)
	}
	if err := keyRing.Activate("missing"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected missing key error, got %v", err)
	}
	if _, err := keyRing.Signer("missing"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected missing signer error, got %v", err)
	}
}

package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
)

func TestSignerRegistryGeneratesEd25519Signer(t *testing.T) {
	signer, err := NewSignerRegistry().Generate(config.CryptoBackendEd25519)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("hello")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(signer.PublicKey(), message, signature) {
		t.Fatal("expected generated signer to verify")
	}
}

func TestSignerRegistryBLSRequiresAdapter(t *testing.T) {
	_, err := NewSignerRegistry().Generate(config.CryptoBackendBLS)
	if !errors.Is(err, ErrBLSBackendUnavailable) {
		t.Fatalf("expected bls unavailable, got %v", err)
	}
}

func TestSignerRegistrySupportsCustomAdapter(t *testing.T) {
	expected, err := NewDeterministicSigner([]byte("custom"))
	if err != nil {
		t.Fatal(err)
	}
	registry := NewSignerRegistry().Register("custom", func() (Signer, error) {
		return expected, nil
	})
	signer, err := registry.Generate("custom")
	if err != nil {
		t.Fatal(err)
	}
	if string(signer.PublicKey()) != string(expected.PublicKey()) {
		t.Fatal("expected custom signer")
	}
}

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

func TestSignerRegistryGeneratesBuiltInBLSSigner(t *testing.T) {
	signer, err := NewSignerRegistry().Generate(config.CryptoBackendBLS)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("hello-bls")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(signer.PublicKey(), message, signature) {
		t.Fatal("expected generated BLS signer to verify")
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

func TestSignerRegistrySupportsSafeBLSAdapter(t *testing.T) {
	signer, err := NewSignerRegistry().
		RegisterBLSAdapter(func() (BLSAdapter, error) { return testBLSAdapter{safe: true}, nil }).
		Generate(config.CryptoBackendBLS)
	if err != nil {
		t.Fatal(err)
	}
	if string(signer.PublicKey()) != "bls-public" {
		t.Fatalf("unexpected bls public key: %q", signer.PublicKey())
	}
}

func TestSignerRegistryRejectsUnsafeBLSAdapter(t *testing.T) {
	_, err := NewSignerRegistry().
		RegisterBLSAdapter(func() (BLSAdapter, error) { return testBLSAdapter{}, nil }).
		Generate(config.CryptoBackendBLS)
	if !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected unsafe bls adapter, got %v", err)
	}
}

package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
)

func TestNewRuntimeSuiteDeterministic(t *testing.T) {
	suite, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendDeterministic})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier == nil {
		t.Fatal("expected finality verifier")
	}
}

func TestNewRuntimeSuiteEd25519(t *testing.T) {
	suite, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendEd25519})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier == nil {
		t.Fatal("expected finality verifier")
	}
}

func TestNewRuntimeSuiteRejectsUnsupportedBackend(t *testing.T) {
	_, err := NewRuntimeSuite(config.CryptoConfig{Backend: "unknown"})
	if !errors.Is(err, ErrUnsupportedCryptoBackend) {
		t.Fatalf("expected unsupported backend, got %v", err)
	}
}

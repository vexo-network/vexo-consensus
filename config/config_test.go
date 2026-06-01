package config

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultConfigIsValid(t *testing.T) {
	if err := Default("vexo-test").Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigValidateRejectsMissingChainID(t *testing.T) {
	if err := Default("").Validate(); !errors.Is(err, ErrMissingChainID) {
		t.Fatalf("expected missing chain id, got %v", err)
	}
}

func TestConfigValidateRejectsInvalidCommittee(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Committee.CommitteeSize = 0
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestConfigValidateRejectsMissingCryptoBackend(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestConfigValidateRejectsMissingCommitteeBackend(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Committee.Backend = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestConfigValidateRejectsNegativeP2PWindowResetInterval(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.P2P.WindowResetInterval = -time.Second
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

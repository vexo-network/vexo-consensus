package config

import (
	"errors"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
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

func TestConfigValidateRejectsUnsafeSettings(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "unknown crypto backend", mutate: func(cfg *Config) { cfg.Crypto.Backend = "unknown" }},
		{name: "negative max validators", mutate: func(cfg *Config) { cfg.Validator.MaxValidators = -1 }},
		{name: "unknown committee backend", mutate: func(cfg *Config) { cfg.Committee.Backend = "unknown" }},
		{name: "zero committee epoch", mutate: func(cfg *Config) { cfg.Committee.EpochLength = 0 }},
		{name: "zero committee size", mutate: func(cfg *Config) { cfg.Committee.CommitteeSize = 0 }},
		{name: "zero mempool tx bytes", mutate: func(cfg *Config) { cfg.Mempool.MaxTxBytes = 0 }},
		{name: "negative mempool tx bytes", mutate: func(cfg *Config) { cfg.Mempool.MaxTxBytes = -1 }},
		{name: "zero mempool tx count", mutate: func(cfg *Config) { cfg.Mempool.MaxTxs = 0 }},
		{name: "negative mempool tx count", mutate: func(cfg *Config) { cfg.Mempool.MaxTxs = -1 }},
		{name: "negative mempool seen ttl", mutate: func(cfg *Config) { cfg.Mempool.SeenTTL = -time.Second }},
		{name: "execution min gas greater than max", mutate: func(cfg *Config) {
			cfg.Execution.MinGas = 2
			cfg.Execution.MaxGas = 1
		}},
		{name: "unknown fee denom", mutate: func(cfg *Config) { cfg.Execution.FeeDenom = "unknown" }},
		{name: "missing gas denom", mutate: func(cfg *Config) { cfg.Execution.GasDenom = "" }},
		{name: "zero governance quorum", mutate: func(cfg *Config) { cfg.Governance.QuorumPower = 0 }},
		{name: "zero governance yes threshold", mutate: func(cfg *Config) { cfg.Governance.YesThresholdPower = 0 }},
		{name: "zero governance voting period", mutate: func(cfg *Config) { cfg.Governance.VotingPeriod = 0 }},
		{name: "zero governance timelock", mutate: func(cfg *Config) { cfg.Governance.Timelock = 0 }},
		{name: "initial score at ban threshold", mutate: func(cfg *Config) { cfg.P2P.InitialScore = cfg.P2P.BanThreshold }},
		{name: "initial score below ban threshold", mutate: func(cfg *Config) { cfg.P2P.InitialScore = cfg.P2P.BanThreshold - 1 }},
		{name: "max score below initial score", mutate: func(cfg *Config) { cfg.P2P.MaxScore = cfg.P2P.InitialScore - 1 }},
		{name: "negative valid reward", mutate: func(cfg *Config) { cfg.P2P.ValidMessageReward = -1 }},
		{name: "zero invalid cost", mutate: func(cfg *Config) { cfg.P2P.InvalidMessageCost = 0 }},
		{name: "negative invalid cost", mutate: func(cfg *Config) { cfg.P2P.InvalidMessageCost = -1 }},
		{name: "zero rate limit cost", mutate: func(cfg *Config) { cfg.P2P.RateLimitCost = 0 }},
		{name: "negative rate limit cost", mutate: func(cfg *Config) { cfg.P2P.RateLimitCost = -1 }},
		{name: "zero max messages window", mutate: func(cfg *Config) { cfg.P2P.MaxMessagesPerWindow = 0 }},
		{name: "zero total max messages window", mutate: func(cfg *Config) { cfg.P2P.MaxTotalMessagesPerWindow = 0 }},
		{name: "zero p2p window reset interval", mutate: func(cfg *Config) { cfg.P2P.WindowResetInterval = 0 }},
		{name: "negative p2p window reset interval", mutate: func(cfg *Config) { cfg.P2P.WindowResetInterval = -time.Second }},
		{name: "negative p2p ban duration", mutate: func(cfg *Config) { cfg.P2P.BanDuration = -time.Second }},
		{name: "negative p2p score recovery", mutate: func(cfg *Config) { cfg.P2P.ScoreRecovery = -1 }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := Default("vexo-test")
			testCase.mutate(&cfg)
			if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("expected invalid config, got %v", err)
			}
		})
	}
}

func TestConfigValidateAllowsOptionalSafetyKnobs(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Validator.MaxValidators = 0
	cfg.Committee.MinVotingPower = 0
	cfg.Governance.VetoPower = 0
	cfg.P2P.ValidMessageReward = 0
	cfg.P2P.BanDuration = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected optional zero settings to be valid, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsDeterministicCrypto(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Execution.RequireSigned = true
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected unsafe network config, got %v", err)
	}
}

func TestValidateNetworkSafetyAcceptsHardenedEd25519Config(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendEd25519
	cfg.Committee.Backend = committee.BackendVRF
	cfg.Execution.RequireSigned = true
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.WALPath = "mempool.wal"

	if err := cfg.ValidateNetworkSafety(); err != nil {
		t.Fatalf("expected hardened ed25519 config to pass, got %v", err)
	}
}

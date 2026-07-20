package config

import (
	"encoding/json"
	"errors"
	"strings"
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
		{name: "zero target gas", mutate: func(cfg *Config) { cfg.Execution.TargetGas = 0 }},
		{name: "zero max gas", mutate: func(cfg *Config) { cfg.Execution.MaxGas = 0 }},
		{name: "target gas above max gas", mutate: func(cfg *Config) {
			cfg.Execution.TargetGas = 11
			cfg.Execution.MaxGas = 10
		}},
		{name: "zero target blob gas", mutate: func(cfg *Config) { cfg.Execution.TargetBlobGas = 0 }},
		{name: "zero max blob gas", mutate: func(cfg *Config) { cfg.Execution.MaxBlobGas = 0 }},
		{name: "target blob gas above max blob gas", mutate: func(cfg *Config) {
			cfg.Execution.TargetBlobGas = 11
			cfg.Execution.MaxBlobGas = 10
		}},
		{name: "zero base fee change denominator", mutate: func(cfg *Config) { cfg.Execution.BaseFeeChangeDenominator = 0 }},
		{name: "zero blob fee change denominator", mutate: func(cfg *Config) { cfg.Execution.BlobFeeChangeDenominator = 0 }},
		{name: "zero min gas", mutate: func(cfg *Config) { cfg.Execution.MinGas = 0 }},
		{name: "signed execution without mint authority", mutate: func(cfg *Config) { cfg.Execution.RequireSigned = true }},
		{name: "zero staking unbonding delay", mutate: func(cfg *Config) { cfg.Staking.UnbondingDelay = 0 }},
		{name: "zero staking commission cap", mutate: func(cfg *Config) { cfg.Staking.MaxCommissionBPS = 0 }},
		{name: "staking commission cap above denominator", mutate: func(cfg *Config) { cfg.Staking.MaxCommissionBPS = 10001 }},
		{name: "execution min gas greater than max", mutate: func(cfg *Config) {
			cfg.Execution.MinGas = 2
			cfg.Execution.MaxGas = 1
		}},
		{name: "unknown fee denom", mutate: func(cfg *Config) { cfg.Execution.FeeDenom = "unknown" }},
		{name: "missing gas denom", mutate: func(cfg *Config) { cfg.Execution.GasDenom = "" }},
		{name: "unknown evm fork preset", mutate: func(cfg *Config) { cfg.Execution.EVMForkPreset = "surprise" }},
		{name: "custom evm fork preset without chain config", mutate: func(cfg *Config) { cfg.Execution.EVMForkPreset = "custom" }},
		{name: "invalid evm chain config json", mutate: func(cfg *Config) { cfg.Execution.EVMChainConfigJSON = "{invalid" }},
		{name: "zero max blob sidecar blobs", mutate: func(cfg *Config) { cfg.Execution.MaxBlobSidecarBlobs = 0 }},
		{name: "zero max blob sidecar bytes", mutate: func(cfg *Config) { cfg.Execution.MaxBlobSidecarBytes = 0 }},
		{name: "dynamic base fee without base fee", mutate: func(cfg *Config) {
			cfg.Execution.DynamicBaseFee = true
			cfg.Execution.BaseFee = 0
		}},
		{name: "dynamic base fee without target gas", mutate: func(cfg *Config) {
			cfg.Execution.DynamicBaseFee = true
			cfg.Execution.BaseFee = 1
			cfg.Execution.TargetGas = 0
		}},
		{name: "dynamic base fee without base fee change denominator", mutate: func(cfg *Config) {
			cfg.Execution.DynamicBaseFee = true
			cfg.Execution.BaseFee = 1
			cfg.Execution.BaseFeeChangeDenominator = 0
		}},
		{name: "dynamic base fee invalid bounds", mutate: func(cfg *Config) {
			cfg.Execution.DynamicBaseFee = true
			cfg.Execution.BaseFee = 1
			cfg.Execution.MinBaseFee = 10
			cfg.Execution.MaxBaseFee = 5
		}},
		{name: "zero governance quorum", mutate: func(cfg *Config) { cfg.Governance.QuorumPower = 0 }},
		{name: "zero governance yes threshold", mutate: func(cfg *Config) { cfg.Governance.YesThresholdPower = 0 }},
		{name: "zero governance voting period", mutate: func(cfg *Config) { cfg.Governance.VotingPeriod = 0 }},
		{name: "zero governance timelock", mutate: func(cfg *Config) { cfg.Governance.Timelock = 0 }},
		{name: "unknown governance deposit denom", mutate: func(cfg *Config) { cfg.Governance.DepositDenom = "unknown" }},
		{name: "invalid governance min deposit", mutate: func(cfg *Config) { cfg.Governance.MinDeposit = "bad" }},
		{name: "required governance deposit without minimum", mutate: func(cfg *Config) {
			cfg.Governance.RequireDeposit = true
			cfg.Governance.MinDeposit = ""
		}},
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
		{name: "total max messages below per peer window", mutate: func(cfg *Config) { cfg.P2P.MaxTotalMessagesPerWindow = cfg.P2P.MaxMessagesPerWindow - 1 }},
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
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Governance.RequireDeposit = true
	cfg.Governance.MinDeposit = "1avxo"
	cfg.Governance.DepositEscrow = "module:governance:deposit_escrow"
	cfg.Governance.RejectedDeposits = "module:governance:rejected_deposits"
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected unsafe network config, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsHardenedEd25519Config(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendEd25519
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AuditReport = "vrf-audit-2026"
	cfg.VRF.KeySource = "remote-signer"
	cfg.Execution.RequireSigned = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Governance.RequireDeposit = true
	cfg.Governance.MinDeposit = "1avxo"
	cfg.Governance.DepositEscrow = "module:governance:deposit_escrow"
	cfg.Governance.RejectedDeposits = "module:governance:rejected_deposits"
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.WALPath = "mempool.wal"
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected ed25519 network-safety rejection, got %v", err)
	}
}

func TestNetworkSafeTemplatePassesNetworkSafety(t *testing.T) {
	cfg := NetworkSafeTemplate("vexo-test", "/var/lib/vexo")
	if err := cfg.ValidateNetworkSafety(); err != nil {
		t.Fatalf("expected network-safe template to pass validation, got %v", err)
	}
	if cfg.Crypto.Backend != CryptoBackendBLS ||
		!cfg.Crypto.ProductionAdapter ||
		cfg.Crypto.AdapterName != NetworkSafeBLSAdapterBLST ||
		cfg.Crypto.AuditReport != NetworkSafeBLSAuditReport ||
		cfg.Crypto.DependencyAudit != NetworkSafeBLSDependencyAudit ||
		cfg.Crypto.AuditEvidenceSHA256 == "" ||
		cfg.Committee.Backend != committee.BackendVRF ||
		cfg.VRF.DependencyAudit != NetworkSafeVRFDependencyAudit ||
		cfg.VRF.AuditEvidenceSHA256 == "" ||
		cfg.Mempool.WALPath == "" ||
		!cfg.Execution.RequireSigned ||
		!cfg.Execution.RequireNonce ||
		!cfg.Governance.RequireDeposit ||
		cfg.Governance.MinDeposit == "" {
		t.Fatalf("unexpected network-safe template: %+v", cfg)
	}
}

func TestCryptoAndVRFConfigJSONUsesSnakeCase(t *testing.T) {
	cfg := NetworkSafeTemplate("vexo-test", "/var/lib/vexo")
	encoded, err := json.Marshal(cfg.VRF)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "audit_evidence_sha256") || !strings.Contains(string(encoded), "dependency_audit") {
		t.Fatalf("expected snake_case VRF fields, got %s", encoded)
	}
	var decoded VRFConfig
	if err := json.Unmarshal([]byte(`{"production_adapter":true,"adapter_name":"ecvrf-p256-sha256-tai-v1","audit_report":"audit","dependency_audit":"dep","audit_evidence_sha256":"`+strings.Repeat("a", 64)+`","key_source":"kms"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.ProductionAdapter || decoded.AdapterName == "" || decoded.DependencyAudit != "dep" || decoded.AuditEvidenceSHA256 == "" {
		t.Fatalf("expected snake_case decode, got %+v", decoded)
	}
}

func TestValidateNetworkSafetyAcceptsHardenedBLSTConfig(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendBLS
	cfg.Crypto.ProductionAdapter = true
	cfg.Crypto.AdapterName = "blst-bls12381-minpk-v1"
	cfg.Crypto.AuditReport = "ncc-group-blst-security-assessment"
	cfg.Crypto.DependencyAudit = "github.com/supranational/blst@v0.3.16"
	cfg.Crypto.AuditEvidenceSHA256 = NetworkSafeBLSAuditEvidence
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AdapterName = NetworkSafeVRFAdapterECVRFP256
	cfg.VRF.AuditReport = "vrf-audit-2026"
	cfg.VRF.DependencyAudit = NetworkSafeVRFDependencyAudit
	cfg.VRF.AuditEvidenceSHA256 = NetworkSafeVRFAuditEvidence
	cfg.VRF.KeySource = "remote-signer"
	cfg.Execution.RequireSigned = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Governance.RequireDeposit = true
	cfg.Governance.MinDeposit = "1avxo"
	cfg.Governance.DepositEscrow = "module:governance:deposit_escrow"
	cfg.Governance.RejectedDeposits = "module:governance:rejected_deposits"
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.WALPath = "mempool.wal"
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); err != nil {
		t.Fatalf("expected hardened BLST config to pass, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsVRFWithoutAdapterName(t *testing.T) {
	cfg := NetworkSafeTemplate("vexo-test", "/var/lib/vexo")
	cfg.VRF.AdapterName = ""

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing VRF adapter name rejection, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsVRFWithoutAuditEvidence(t *testing.T) {
	cfg := NetworkSafeTemplate("vexo-test", "/var/lib/vexo")
	cfg.VRF.AuditEvidenceSHA256 = ""

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing VRF audit evidence rejection, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsVRFWithoutDependencyAudit(t *testing.T) {
	cfg := NetworkSafeTemplate("vexo-test", "/var/lib/vexo")
	cfg.VRF.DependencyAudit = ""

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing VRF dependency audit rejection, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsBLSWithoutAuditEvidenceDigest(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendBLS
	cfg.Crypto.ProductionAdapter = true
	cfg.Crypto.AdapterName = "blst-bls12381-minpk-v1"
	cfg.Crypto.AuditReport = "ncc-group-blst-security-assessment"
	cfg.Crypto.DependencyAudit = "github.com/supranational/blst@v0.3.16"
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AuditReport = "vrf-audit-2026"
	cfg.VRF.KeySource = "remote-signer"
	cfg.Execution.RequireSigned = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.WALPath = "mempool.wal"
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing BLS audit evidence digest rejection, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsPlaceholderBLSAuditEvidenceDigest(t *testing.T) {
	cfg := NetworkSafeTemplate("vexo-test", "/var/lib/vexo")
	cfg.Crypto.AuditEvidenceSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected placeholder BLS audit evidence digest rejection, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsBuiltInBLSAdapterName(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendBLS
	cfg.Crypto.ProductionAdapter = true
	cfg.Crypto.AdapterName = "circl-bls12381-g1sigg2-basic-v1"
	cfg.Crypto.AuditReport = "external-audit-report-id"
	cfg.Crypto.DependencyAudit = "dependency-audit-id"
	cfg.Crypto.AuditEvidenceSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AuditReport = "vrf-audit-2026"
	cfg.VRF.KeySource = "remote-signer"
	cfg.Execution.RequireSigned = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.WALPath = "mempool.wal"
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected built-in bls adapter to be unsafe, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsUnprotectedLegacyEthereumTx(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendEd25519
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AuditReport = "vrf-audit-2026"
	cfg.VRF.KeySource = "remote-signer"
	cfg.Execution.RequireSigned = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.AllowUnprotectedLegacyTx = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.WALPath = "mempool.wal"
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected unprotected legacy tx support to be unsafe, got %v", err)
	}
}

func TestValidateNetworkSafetyRejectsDisabledMempoolReplacement(t *testing.T) {
	cfg := Default("vexo-test")
	cfg.Crypto.Backend = CryptoBackendEd25519
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AuditReport = "vrf-audit-2026"
	cfg.VRF.KeySource = "remote-signer"
	cfg.Execution.RequireSigned = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.EnableReplacement = false
	cfg.Mempool.WALPath = "mempool.wal"
	cfg.Mempool.SeenTTL = time.Hour

	if err := cfg.ValidateNetworkSafety(); !errors.Is(err, ErrUnsafeNetworkConfig) {
		t.Fatalf("expected disabled replacement to be unsafe, got %v", err)
	}
}

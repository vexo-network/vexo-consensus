package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrMissingChainID      = errors.New("chain id is required")
	ErrInvalidConfig       = errors.New("invalid config")
	ErrUnsafeNetworkConfig = errors.New("unsafe network config")
)

type Config struct {
	ChainID     string
	Application ApplicationConfig
	Execution   ExecutionConfig
	Crypto      CryptoConfig
	VRF         VRFConfig
	Validator   validator.AdmissionConfig
	Committee   committee.RotationPolicy
	Mempool     mempool.FIFOConfig
	Bank        BankConfig
	Staking     StakingConfig
	Governance  governance.TallyPolicy
	P2P         p2p.ScoreConfig
}

type ApplicationConfig struct {
	Modules []string
}

type ExecutionConfig struct {
	MinFee                   uint64
	BaseFee                  uint64
	BlobBaseFee              uint64
	EVMChainID               uint64
	DynamicBaseFee           bool
	DynamicBlobBaseFee       bool
	TargetGas                uint64
	TargetBlobGas            uint64
	MaxBlobGas               uint64
	BaseFeeChangeDenominator uint64
	BlobFeeChangeDenominator uint64
	MinBaseFee               uint64
	MaxBaseFee               uint64
	MinBlobBaseFee           uint64
	MaxBlobBaseFee           uint64
	MinGas                   uint64
	MaxGas                   uint64
	RequireNonce             bool
	RequireSigned            bool
	FeeCollector             string
	FeeDenom                 string
	DisplayDenom             string
	DisplayExponent          uint8
	GasDenom                 string
	EVMForkPreset            string
	EVMChainConfigJSON       string
	StrictEVMStateRoot       bool
	AllowUnprotectedLegacyTx bool
	MaxBlobSidecarBlobs      uint64
	MaxBlobSidecarBytes      uint64
}

type StakingConfig struct {
	UnbondingDelay   uint64
	MaxCommissionBPS uint64
}

type BankConfig struct {
	MintAuthority string
}

type CryptoBackend string

const (
	CryptoBackendDeterministic CryptoBackend = "deterministic"
	CryptoBackendEd25519       CryptoBackend = "ed25519"
	CryptoBackendBLS           CryptoBackend = "bls"
)

const (
	DefaultEVMChainID               uint64 = 83960
	NetworkSafeVRFAdapterECVRFP256         = "ecvrf-p256-sha256-tai-v1"
	NetworkSafeVRFAuditReport              = "built-in-ecvrf-p256-runtime-validation"
	NetworkSafeVRFDependencyAudit          = "github.com/vechain/go-ecvrf@v0.0.0-20251211112124-5d5a3ef70fc9"
	NetworkSafeVRFAuditEvidence            = "d391193d5a9e5da40a6e77171782083c6e2c2a055afccede6eb5f0e1896f8b1f"
	NetworkSafeVRFAuditEvidencePath        = "docs/security/ecvrf-audit-evidence.json"
	NetworkSafeVRFKeySource                = "local-encrypted-or-remote-kms"
	NetworkSafeBLSAdapterBLST              = "blst-bls12381-minpk-v1"
	NetworkSafeBLSAuditReport              = "ncc-group-blst-security-assessment"
	NetworkSafeBLSDependencyAudit          = "github.com/supranational/blst@v0.3.16"
	NetworkSafeBLSAuditEvidence            = "fe4310147a3d182952ba9a44ab94e6fe9fb2c160913248984973cd052b2dfb95"
	NetworkSafeBLSAuditEvidencePath        = "docs/security/blst-audit-evidence.json"
)

type CryptoConfig struct {
	Backend             CryptoBackend `json:"backend"`
	ProductionAdapter   bool          `json:"production_adapter"`
	AdapterName         string        `json:"adapter_name"`
	AuditReport         string        `json:"audit_report"`
	DependencyAudit     string        `json:"dependency_audit"`
	AuditEvidenceSHA256 string        `json:"audit_evidence_sha256"`
}

type VRFConfig struct {
	Keys                map[string][]byte `json:"keys,omitempty"`
	ProductionAdapter   bool              `json:"production_adapter"`
	AdapterName         string            `json:"adapter_name"`
	AuditReport         string            `json:"audit_report"`
	DependencyAudit     string            `json:"dependency_audit"`
	AuditEvidenceSHA256 string            `json:"audit_evidence_sha256"`
	KeySource           string            `json:"key_source"`
	TLSCertPath         string            `json:"tls_cert_path,omitempty"`
	TLSKeyPath          string            `json:"tls_key_path,omitempty"`
	TLSCAPath           string            `json:"tls_ca_path,omitempty"`
	TLSServerName       string            `json:"tls_server_name,omitempty"`
}

func Default(chainID string) Config {
	return Config{
		ChainID: chainID,
		Application: ApplicationConfig{
			Modules: []string{"bank", "staking", "governance", "params", "ibc"},
		},
		Execution: ExecutionConfig{
			EVMChainID:               DefaultEVMChainID,
			MaxGas:                   10_000_000,
			TargetGas:                5_000_000,
			BlobBaseFee:              1,
			TargetBlobGas:            393_216,
			MaxBlobGas:               786_432,
			BaseFeeChangeDenominator: 8,
			BlobFeeChangeDenominator: 6,
			MinBlobBaseFee:           1,
			FeeCollector:             "fee_collector",
			FeeDenom:                 economics.AtomicDenom,
			DisplayDenom:             economics.DisplayDenom,
			DisplayExponent:          18,
			GasDenom:                 "gas",
			EVMForkPreset:            "latest",
			StrictEVMStateRoot:       true,
			MaxBlobSidecarBlobs:      6,
			MaxBlobSidecarBytes:      2 * 1024 * 1024,
		},
		Crypto: CryptoConfig{
			Backend: CryptoBackendDeterministic,
		},
		Validator: validator.AdmissionConfig{
			Permissionless: true,
			MinStake:       1,
		},
		Committee: committee.RotationPolicy{
			EpochLength:    100,
			CommitteeSize:  128,
			MinVotingPower: 1,
			Backend:        committee.BackendDeterministic,
		},
		Mempool: mempool.FIFOConfig{
			MaxTxBytes:         1024 * 1024,
			MaxTxs:             100000,
			EnableReplacement:  true,
			ReplacementBumpBPS: 1000,
		},
		Bank: BankConfig{},
		Staking: StakingConfig{
			UnbondingDelay:   1209600,
			MaxCommissionBPS: 10000,
		},
		Governance: governance.TallyPolicy{
			QuorumPower:       1,
			YesThresholdPower: 1,
			VotingPeriod:      100,
			Timelock:          10,
			DepositDenom:      economics.AtomicDenom,
			DepositEscrow:     "module:governance:deposit_escrow",
			RejectedDeposits:  "module:governance:rejected_deposits",
		},
		P2P: p2p.ScoreConfig{
			InitialScore:              100,
			MaxScore:                  1000,
			ValidMessageReward:        1,
			InvalidMessageCost:        10,
			RateLimitCost:             5,
			BanThreshold:              0,
			MaxMessagesPerWindow:      1000,
			MaxTotalMessagesPerWindow: 100000,
			WindowResetInterval:       time.Second,
			ScoreRecovery:             1,
			BanDuration:               10 * time.Minute,
		},
	}
}

func NetworkSafeTemplate(chainID string, dataDir string) Config {
	cfg := Default(chainID)
	cfg.Crypto = CryptoConfig{
		Backend:             CryptoBackendBLS,
		ProductionAdapter:   true,
		AdapterName:         NetworkSafeBLSAdapterBLST,
		AuditReport:         NetworkSafeBLSAuditReport,
		DependencyAudit:     NetworkSafeBLSDependencyAudit,
		AuditEvidenceSHA256: NetworkSafeBLSAuditEvidence,
	}
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF = VRFConfig{
		ProductionAdapter:   true,
		AdapterName:         NetworkSafeVRFAdapterECVRFP256,
		AuditReport:         NetworkSafeVRFAuditReport,
		DependencyAudit:     NetworkSafeVRFDependencyAudit,
		AuditEvidenceSHA256: NetworkSafeVRFAuditEvidence,
		KeySource:           NetworkSafeVRFKeySource,
	}
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.BlobBaseFee = 1
	cfg.Execution.MinGas = 1
	cfg.Execution.RequireSigned = true
	cfg.Execution.RequireNonce = true
	cfg.Execution.AllowUnprotectedLegacyTx = false
	cfg.Execution.StrictEVMStateRoot = true
	cfg.Bank.MintAuthority = "governance"
	cfg.Governance.RequireDeposit = true
	cfg.Governance.MinDeposit = "1" + economics.AtomicDenom
	cfg.Governance.DepositDenom = economics.AtomicDenom
	cfg.Governance.DepositEscrow = "module:governance:deposit_escrow"
	cfg.Governance.RejectedDeposits = "module:governance:rejected_deposits"
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.EnableReplacement = true
	cfg.Mempool.SeenTTL = time.Hour
	if cfg.Mempool.WALPath == "" {
		if dataDir == "" {
			cfg.Mempool.WALPath = "mempool.wal"
		} else {
			cfg.Mempool.WALPath = filepath.Join(dataDir, "mempool.wal")
		}
	}
	return cfg
}

func (config Config) Validate() error {
	if config.ChainID == "" {
		return ErrMissingChainID
	}
	if !validCryptoBackend(config.Crypto.Backend) {
		return ErrInvalidConfig
	}
	if config.Execution.MaxGas > 0 && config.Execution.MinGas > config.Execution.MaxGas {
		return ErrInvalidConfig
	}
	if config.Execution.EVMChainID == 0 {
		return ErrInvalidConfig
	}
	if config.Execution.DynamicBaseFee &&
		(config.Execution.BaseFee == 0 ||
			config.Execution.TargetGas == 0 ||
			config.Execution.BaseFeeChangeDenominator == 0 ||
			(config.Execution.MaxBaseFee > 0 && config.Execution.MinBaseFee > config.Execution.MaxBaseFee)) {
		return ErrInvalidConfig
	}
	if config.Execution.DynamicBlobBaseFee &&
		(config.Execution.BlobBaseFee == 0 ||
			config.Execution.TargetBlobGas == 0 ||
			config.Execution.MaxBlobGas == 0 ||
			config.Execution.TargetBlobGas > config.Execution.MaxBlobGas ||
			config.Execution.BlobFeeChangeDenominator == 0 ||
			(config.Execution.MaxBlobBaseFee > 0 && config.Execution.MinBlobBaseFee > config.Execution.MaxBlobBaseFee)) {
		return ErrInvalidConfig
	}
	if _, ok := economics.DenomFactor(config.Execution.FeeDenom); !ok {
		return ErrInvalidConfig
	}
	if config.Execution.EVMForkPreset != "" &&
		config.Execution.EVMForkPreset != "latest" &&
		config.Execution.EVMForkPreset != "custom" {
		return ErrInvalidConfig
	}
	if config.Execution.EVMForkPreset == "custom" && config.Execution.EVMChainConfigJSON == "" {
		return ErrInvalidConfig
	}
	if config.Execution.EVMChainConfigJSON != "" && config.Execution.EVMForkPreset != "custom" {
		return ErrInvalidConfig
	}
	if config.Execution.EVMChainConfigJSON != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(config.Execution.EVMChainConfigJSON), &raw); err != nil || len(raw) == 0 {
			return ErrInvalidConfig
		}
	}
	if config.Execution.MaxBlobSidecarBlobs == 0 ||
		config.Execution.MaxBlobSidecarBytes == 0 {
		return ErrInvalidConfig
	}
	if config.Execution.DisplayDenom != economics.DisplayDenom ||
		config.Execution.DisplayExponent != 18 ||
		config.Execution.GasDenom == "" {
		return ErrInvalidConfig
	}
	if config.Validator.MaxValidators < 0 {
		return ErrInvalidConfig
	}
	if !validCommitteeBackend(config.Committee.Backend) ||
		config.Committee.EpochLength == 0 ||
		config.Committee.CommitteeSize == 0 {
		return ErrInvalidConfig
	}
	if config.Mempool.MaxTxBytes <= 0 ||
		config.Mempool.MaxTxs <= 0 ||
		config.Mempool.SeenTTL < 0 {
		return ErrInvalidConfig
	}
	if config.Bank.MintAuthority == "" && config.Execution.RequireSigned {
		return ErrInvalidConfig
	}
	if config.Staking.UnbondingDelay == 0 ||
		config.Staking.MaxCommissionBPS == 0 ||
		config.Staking.MaxCommissionBPS > 10000 {
		return ErrInvalidConfig
	}
	if config.Governance.QuorumPower == 0 ||
		config.Governance.YesThresholdPower == 0 ||
		config.Governance.VotingPeriod == 0 ||
		config.Governance.Timelock == 0 {
		return ErrInvalidConfig
	}
	if config.Governance.DepositDenom != "" && !isKnownDenom(config.Governance.DepositDenom) {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(config.Governance.MinDeposit) != "" {
		amount, err := economics.ParseAmountBig(config.Governance.MinDeposit)
		if err != nil || amount == nil || amount.Sign() <= 0 {
			return ErrInvalidConfig
		}
	}
	if config.Governance.RequireDeposit && strings.TrimSpace(config.Governance.MinDeposit) == "" {
		return ErrInvalidConfig
	}
	if config.Governance.RequireDeposit && (config.Governance.DepositEscrow == "" || config.Governance.RejectedDeposits == "") {
		return ErrInvalidConfig
	}
	if config.P2P.InitialScore <= config.P2P.BanThreshold ||
		(config.P2P.MaxScore > 0 && config.P2P.MaxScore < config.P2P.InitialScore) ||
		config.P2P.ValidMessageReward < 0 ||
		config.P2P.InvalidMessageCost <= 0 ||
		config.P2P.RateLimitCost <= 0 ||
		config.P2P.MaxMessagesPerWindow == 0 ||
		config.P2P.MaxTotalMessagesPerWindow == 0 ||
		config.P2P.MaxTotalMessagesPerWindow < config.P2P.MaxMessagesPerWindow ||
		config.P2P.WindowResetInterval <= 0 ||
		config.P2P.ScoreRecovery < 0 ||
		config.P2P.BanDuration < 0 {
		return ErrInvalidConfig
	}
	return nil
}

func (config Config) ValidateNetworkSafety() error {
	if err := config.Validate(); err != nil {
		return err
	}
	if config.Crypto.Backend != CryptoBackendBLS {
		return ErrUnsafeNetworkConfig
	}
	if !config.Crypto.ProductionAdapter {
		return ErrUnsafeNetworkConfig
	}
	if config.Crypto.AdapterName == "" || config.Crypto.AuditReport == "" || config.Crypto.DependencyAudit == "" {
		return ErrUnsafeNetworkConfig
	}
	if !validSHA256Hex(config.Crypto.AuditEvidenceSHA256) || unsafeAuditEvidenceDigest(config.Crypto.AuditEvidenceSHA256) {
		return ErrUnsafeNetworkConfig
	}
	if strings.HasPrefix(config.Crypto.AdapterName, "circl-bls12381-") {
		return ErrUnsafeNetworkConfig
	}
	if config.Committee.Backend != committee.BackendVRF {
		return ErrUnsafeNetworkConfig
	}
	if !config.VRF.ProductionAdapter || config.VRF.AdapterName == "" || config.VRF.AuditReport == "" || config.VRF.DependencyAudit == "" || config.VRF.KeySource == "" {
		return ErrUnsafeNetworkConfig
	}
	if !validSHA256Hex(config.VRF.AuditEvidenceSHA256) || unsafeAuditEvidenceDigest(config.VRF.AuditEvidenceSHA256) {
		return ErrUnsafeNetworkConfig
	}
	if !config.Execution.RequireSigned || !config.Execution.RequireNonce {
		return ErrUnsafeNetworkConfig
	}
	if config.Execution.AllowUnprotectedLegacyTx {
		return ErrUnsafeNetworkConfig
	}
	if config.Execution.MinFee == 0 || config.Execution.BaseFee == 0 || config.Execution.BlobBaseFee == 0 || config.Execution.MinGas == 0 {
		return ErrUnsafeNetworkConfig
	}
	if config.Mempool.MinFee == 0 || !config.Mempool.EnablePriority || !config.Mempool.EnableReplacement {
		return ErrUnsafeNetworkConfig
	}
	if config.Mempool.WALPath == "" {
		return ErrUnsafeNetworkConfig
	}
	if config.Mempool.SeenTTL <= 0 {
		return ErrUnsafeNetworkConfig
	}
	if !config.Governance.RequireDeposit || strings.TrimSpace(config.Governance.MinDeposit) == "" ||
		config.Governance.DepositEscrow == "" || config.Governance.RejectedDeposits == "" {
		return ErrUnsafeNetworkConfig
	}
	if config.P2P.MaxScore <= 0 ||
		config.P2P.MaxTotalMessagesPerWindow <= config.P2P.MaxMessagesPerWindow ||
		config.P2P.BanDuration <= 0 ||
		config.P2P.ScoreRecovery <= 0 {
		return ErrUnsafeNetworkConfig
	}
	return nil
}

func validSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func unsafeAuditEvidenceDigest(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef":
		return true
	default:
		return false
	}
}

func validCryptoBackend(backend CryptoBackend) bool {
	switch backend {
	case CryptoBackendDeterministic, CryptoBackendEd25519, CryptoBackendBLS:
		return true
	default:
		return false
	}
}

func validCommitteeBackend(backend committee.Backend) bool {
	switch backend {
	case committee.BackendDeterministic, committee.BackendVRF:
		return true
	default:
		return false
	}
}

func isKnownDenom(denom string) bool {
	_, found := economics.DenomFactor(denom)
	return found
}

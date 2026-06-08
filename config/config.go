package config

import (
	"encoding/json"
	"errors"
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

type CryptoConfig struct {
	Backend           CryptoBackend
	ProductionAdapter bool
	AdapterName       string
	AuditReport       string
	DependencyAudit   string
}

type VRFConfig struct {
	Keys              map[string][]byte
	ProductionAdapter bool
	AdapterName       string
	AuditReport       string
	KeySource         string
}

func Default(chainID string) Config {
	return Config{
		ChainID: chainID,
		Application: ApplicationConfig{
			Modules: []string{"bank", "staking", "governance", "params", "ibc"},
		},
		Execution: ExecutionConfig{
			EVMChainID:               1,
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
	if config.Crypto.Backend == CryptoBackendDeterministic {
		return ErrUnsafeNetworkConfig
	}
	if config.Crypto.Backend == CryptoBackendBLS && !config.Crypto.ProductionAdapter {
		return ErrUnsafeNetworkConfig
	}
	if config.Crypto.Backend == CryptoBackendBLS &&
		(config.Crypto.AdapterName == "" || config.Crypto.AuditReport == "" || config.Crypto.DependencyAudit == "") {
		return ErrUnsafeNetworkConfig
	}
	if config.Crypto.Backend == CryptoBackendBLS && strings.HasPrefix(config.Crypto.AdapterName, "circl-bls12381-") {
		return ErrUnsafeNetworkConfig
	}
	if config.Committee.Backend != committee.BackendVRF {
		return ErrUnsafeNetworkConfig
	}
	if !config.VRF.ProductionAdapter || config.VRF.AuditReport == "" || config.VRF.KeySource == "" {
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
	if config.P2P.MaxScore <= 0 ||
		config.P2P.MaxTotalMessagesPerWindow <= config.P2P.MaxMessagesPerWindow ||
		config.P2P.BanDuration <= 0 ||
		config.P2P.ScoreRecovery <= 0 {
		return ErrUnsafeNetworkConfig
	}
	return nil
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

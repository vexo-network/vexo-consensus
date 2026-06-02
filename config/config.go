package config

import (
	"errors"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrMissingChainID = errors.New("chain id is required")
	ErrInvalidConfig  = errors.New("invalid config")
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
	Governance  governance.TallyPolicy
	P2P         p2p.ScoreConfig
}

type ApplicationConfig struct {
	Modules []string
}

type ExecutionConfig struct {
	MinFee        uint64
	MinGas        uint64
	MaxGas        uint64
	RequireNonce  bool
	RequireSigned bool
	FeeCollector  string
}

type CryptoBackend string

const (
	CryptoBackendDeterministic CryptoBackend = "deterministic"
	CryptoBackendEd25519       CryptoBackend = "ed25519"
	CryptoBackendBLS           CryptoBackend = "bls"
)

type CryptoConfig struct {
	Backend CryptoBackend
}

type VRFConfig struct {
	Keys map[string][]byte
}

func Default(chainID string) Config {
	return Config{
		ChainID: chainID,
		Application: ApplicationConfig{
			Modules: []string{"bank"},
		},
		Execution: ExecutionConfig{
			MaxGas:       10_000_000,
			FeeCollector: "fee_collector",
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
			MaxTxBytes: 1024 * 1024,
			MaxTxs:     100000,
		},
		Governance: governance.TallyPolicy{
			QuorumPower:       1,
			YesThresholdPower: 1,
			VotingPeriod:      100,
			Timelock:          10,
		},
		P2P: p2p.ScoreConfig{
			InitialScore:              100,
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
	if config.Governance.QuorumPower == 0 ||
		config.Governance.YesThresholdPower == 0 ||
		config.Governance.VotingPeriod == 0 ||
		config.Governance.Timelock == 0 {
		return ErrInvalidConfig
	}
	if config.P2P.InitialScore <= config.P2P.BanThreshold ||
		config.P2P.ValidMessageReward < 0 ||
		config.P2P.InvalidMessageCost <= 0 ||
		config.P2P.RateLimitCost <= 0 ||
		config.P2P.MaxMessagesPerWindow == 0 ||
		config.P2P.MaxTotalMessagesPerWindow == 0 ||
		config.P2P.WindowResetInterval <= 0 ||
		config.P2P.ScoreRecovery < 0 ||
		config.P2P.BanDuration < 0 {
		return ErrInvalidConfig
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

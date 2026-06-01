package config

import (
	"errors"

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
	ChainID    string
	Validator  validator.AdmissionConfig
	Committee  committee.RotationPolicy
	Mempool    mempool.FIFOConfig
	Governance governance.TallyPolicy
	P2P        p2p.ScoreConfig
}

func Default(chainID string) Config {
	return Config{
		ChainID: chainID,
		Validator: validator.AdmissionConfig{
			Permissionless: true,
			MinStake:       1,
		},
		Committee: committee.RotationPolicy{
			EpochLength:    100,
			CommitteeSize:  128,
			MinVotingPower: 1,
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
			InitialScore:         100,
			ValidMessageReward:   1,
			InvalidMessageCost:   10,
			RateLimitCost:        5,
			BanThreshold:         0,
			MaxMessagesPerWindow: 1000,
		},
	}
}

func (config Config) Validate() error {
	if config.ChainID == "" {
		return ErrMissingChainID
	}
	if config.Committee.EpochLength == 0 || config.Committee.CommitteeSize == 0 {
		return ErrInvalidConfig
	}
	return nil
}

package node

import (
	"errors"
	"path/filepath"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrMissingDataDir    = errors.New("data dir is required")
	ErrMissingGenesis    = errors.New("genesis is required")
	ErrGenesisChainID    = errors.New("genesis chain id mismatch")
	ErrMissingValidators = errors.New("genesis validators are required")
)

type Config struct {
	Chain       config.Config
	DataDir     string
	ValidatorID types.ValidatorID
}

type Genesis struct {
	ChainID    string
	Validators []validator.Validator
	AppState   map[string][]byte
	Governance map[types.Address]types.VotingPower
}

func DefaultConfig(chainID string, dataDir string) Config {
	return Config{
		Chain:   config.Default(chainID),
		DataDir: dataDir,
	}
}

func (cfg Config) Validate() error {
	if err := cfg.Chain.Validate(); err != nil {
		return err
	}
	if cfg.DataDir == "" {
		return ErrMissingDataDir
	}
	return nil
}

func (cfg Config) StoreDir() string {
	return filepath.Join(cfg.DataDir, "store")
}

func (cfg Config) ConsensusWALPath() string {
	return filepath.Join(cfg.DataDir, "consensus.wal")
}

func (genesis Genesis) Validate(chainID string) error {
	if genesis.ChainID == "" {
		return ErrMissingGenesis
	}
	if genesis.ChainID != chainID {
		return ErrGenesisChainID
	}
	if len(genesis.Validators) == 0 {
		return ErrMissingValidators
	}
	return nil
}

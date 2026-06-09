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
	ErrMissingSigner     = errors.New("validator signer is required")
	ErrMissingBLSPoP     = errors.New("validator bls proof-of-possession metadata is required")
)

type Config struct {
	Chain                config.Config
	DataDir              string
	ValidatorID          types.ValidatorID
	RequireNetworkSafety bool
}

type Genesis struct {
	ChainID    string
	Validators []validator.Validator
	AppState   map[string][]byte
	Governance map[types.Address]types.VotingPower
}

// DefaultConfig returns the legacy test-friendly node config.
//
// Deprecated: use NetworkSafeConfig for real network nodes. Tests that
// intentionally need deterministic crypto should use UnsafeTestConfig.
func DefaultConfig(chainID string, dataDir string) Config {
	return UnsafeTestConfig(chainID, dataDir)
}

func UnsafeTestConfig(chainID string, dataDir string) Config {
	return Config{
		Chain:   config.Default(chainID),
		DataDir: dataDir,
	}
}

func NetworkSafeConfig(chainID string, dataDir string) Config {
	return Config{
		Chain:                config.NetworkSafeTemplate(chainID, dataDir),
		DataDir:              dataDir,
		RequireNetworkSafety: true,
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

func (cfg Config) PeerScorePath() string {
	return filepath.Join(cfg.DataDir, "peer_scores.json")
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

func (genesis Genesis) ValidateNetworkSafety(cfg config.Config) error {
	if err := genesis.Validate(cfg.ChainID); err != nil {
		return err
	}
	if cfg.Crypto.Backend == config.CryptoBackendBLS {
		for _, validatorInfo := range genesis.Validators {
			if validatorInfo.Metadata["bls_pop"] == "" {
				return ErrMissingBLSPoP
			}
		}
	}
	return nil
}

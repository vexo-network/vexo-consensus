package geth

import (
	"encoding/json"
	"errors"
	"math/big"

	gethparams "github.com/ethereum/go-ethereum/params"
)

const (
	DefaultForkPreset = "london"
	LatestForkPreset  = "latest"
)

var VexoLondonChainConfig = &gethparams.ChainConfig{
	ChainID:                 big.NewInt(1337),
	HomesteadBlock:          big.NewInt(0),
	EIP150Block:             big.NewInt(0),
	EIP155Block:             big.NewInt(0),
	EIP158Block:             big.NewInt(0),
	ByzantiumBlock:          big.NewInt(0),
	ConstantinopleBlock:     big.NewInt(0),
	PetersburgBlock:         big.NewInt(0),
	IstanbulBlock:           big.NewInt(0),
	BerlinBlock:             big.NewInt(0),
	LondonBlock:             big.NewInt(0),
	TerminalTotalDifficulty: big.NewInt(0),
	Ethash:                  new(gethparams.EthashConfig),
}

var VexoDefaultChainConfig = VexoLondonChainConfig

func NewWithChainConfigJSON(raw string, chainID uint64) (GethVM, error) {
	return NewWithChainConfigPresetJSON(DefaultForkPreset, raw, chainID)
}

func NewWithChainConfigPresetJSON(preset string, raw string, chainID uint64) (GethVM, error) {
	if raw == "" {
		chainConfig, err := ChainConfigForPreset(preset, chainID)
		if err != nil {
			return GethVM{}, err
		}
		return NewWithChainConfig(chainConfig), nil
	}
	var chainConfig gethparams.ChainConfig
	if err := json.Unmarshal([]byte(raw), &chainConfig); err != nil {
		return GethVM{}, err
	}
	if chainConfig.ChainID == nil && chainID > 0 {
		chainConfig.ChainID = new(big.Int).SetUint64(chainID)
	}
	if err := chainConfig.CheckConfigForkOrder(); err != nil {
		return GethVM{}, err
	}
	return NewWithChainConfig(&chainConfig), nil
}

func ChainConfigForPreset(preset string, chainID uint64) (*gethparams.ChainConfig, error) {
	var chainConfig *gethparams.ChainConfig
	switch preset {
	case "", DefaultForkPreset:
		chainConfig = VexoLondonChainConfig
	case LatestForkPreset:
		chainConfig = gethparams.AllDevChainProtocolChanges
	default:
		return nil, errors.New("unsupported EVM fork preset")
	}
	copyConfig := *chainConfig
	if chainID > 0 {
		copyConfig.ChainID = new(big.Int).SetUint64(chainID)
	}
	if err := copyConfig.CheckConfigForkOrder(); err != nil {
		return nil, err
	}
	return &copyConfig, nil
}

func normalizedChainConfig(chainConfig *gethparams.ChainConfig) *gethparams.ChainConfig {
	if chainConfig == nil {
		return VexoDefaultChainConfig
	}
	return chainConfig
}

func (vm GethVM) activeChainConfig() *gethparams.ChainConfig {
	return normalizedChainConfig(vm.chainConfig)
}

package geth

import (
	"encoding/json"
	"math/big"

	gethparams "github.com/ethereum/go-ethereum/params"
)

var VexoDefaultChainConfig = gethparams.AllDevChainProtocolChanges

func NewWithChainConfigJSON(raw string, chainID uint64) (GethVM, error) {
	if raw == "" {
		return New(), nil
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

func normalizedChainConfig(chainConfig *gethparams.ChainConfig) *gethparams.ChainConfig {
	if chainConfig == nil {
		return VexoDefaultChainConfig
	}
	return chainConfig
}

func (vm GethVM) activeChainConfig() *gethparams.ChainConfig {
	return normalizedChainConfig(vm.chainConfig)
}

package geth

import gethparams "github.com/ethereum/go-ethereum/params"

func normalizedChainConfig(chainConfig *gethparams.ChainConfig) *gethparams.ChainConfig {
	if chainConfig == nil {
		return gethparams.AllDevChainProtocolChanges
	}
	return chainConfig
}

func (vm GethVM) activeChainConfig() *gethparams.ChainConfig {
	return normalizedChainConfig(vm.chainConfig)
}

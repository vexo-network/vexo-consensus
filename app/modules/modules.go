package modules

import (
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/app/bank"
	"github.com/vexo-network/vexo-consensus/config"
)

func DefaultRegistry() vexoapp.Registry {
	registry := vexoapp.NewRegistry()
	_ = registry.Register(bank.ModuleName, func() vexoapp.Module { return bank.NewModule() })
	return registry
}

func Build(cfg config.ApplicationConfig) ([]vexoapp.Module, error) {
	return DefaultRegistry().Build(cfg.Modules)
}

func BuildCLICommands(cfg config.ApplicationConfig) ([]vexoapp.CLICommand, error) {
	return DefaultRegistry().BuildCLICommands(cfg.Modules)
}

func NewRuntime(chainID string, cfg config.ApplicationConfig) (*vexoapp.Runtime, error) {
	modules, err := Build(cfg)
	if err != nil {
		return nil, err
	}
	return vexoapp.NewRuntime(chainID, modules, vexoapp.PrefixRouter{})
}

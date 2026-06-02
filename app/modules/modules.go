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
	return NewRuntimeWithExecution(chainID, cfg, config.ExecutionConfig{})
}

func NewRuntimeWithExecution(chainID string, cfg config.ApplicationConfig, execution config.ExecutionConfig) (*vexoapp.Runtime, error) {
	modules, err := Build(cfg)
	if err != nil {
		return nil, err
	}
	runtime, err := vexoapp.NewRuntime(chainID, modules, vexoapp.PrefixRouter{})
	if err != nil {
		return nil, err
	}
	if execution.MinFee > 0 || execution.MinGas > 0 || execution.MaxGas > 0 || execution.RequireNonce {
		runtime.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{
			MinFee:       execution.MinFee,
			MinGas:       execution.MinGas,
			MaxGas:       execution.MaxGas,
			RequireNonce: execution.RequireNonce,
			FeeCollector: execution.FeeCollector,
		}))
	}
	return runtime, nil
}

package modules

import (
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	appevm "github.com/vexo-network/vexo-consensus/modules/evm"
	appgovernance "github.com/vexo-network/vexo-consensus/modules/governance"
	appibc "github.com/vexo-network/vexo-consensus/modules/ibc"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/params"
	"github.com/vexo-network/vexo-consensus/types"
)

func DefaultRegistry() vexoapp.Registry {
	registry := vexoapp.NewRegistry()
	_ = registry.Register(bank.ModuleName, func() vexoapp.Module { return bank.NewModule() })
	_ = registry.Register(staking.ModuleName, func() vexoapp.Module { return staking.NewModule() })
	_ = registry.Register(appgovernance.ModuleName, func() vexoapp.Module { return appgovernance.NewModule() })
	_ = registry.Register(params.Namespace, func() vexoapp.Module { return params.NewModule(nil) })
	_ = registry.Register(appibc.ModuleName, func() vexoapp.Module { return appibc.NewModule() })
	_ = registry.Register(appevm.ModuleName, func() vexoapp.Module { return appevm.NewModule() })
	return registry
}

func Build(cfg config.ApplicationConfig) ([]vexoapp.Module, error) {
	return DefaultRegistry().Build(cfg.Modules)
}

func BuildWithExecution(cfg config.ApplicationConfig, execution config.ExecutionConfig) ([]vexoapp.Module, error) {
	chain := config.Default("vexo-chain")
	chain.Application = cfg
	chain.Execution = execution
	return BuildWithChainConfig(chain)
}

func BuildWithChainConfig(chain config.Config) ([]vexoapp.Module, error) {
	registry := vexoapp.NewRegistry()
	_ = registry.Register(bank.ModuleName, func() vexoapp.Module {
		return bank.NewModuleWithMintAuthority(types.Address(chain.Bank.MintAuthority))
	})
	_ = registry.Register(staking.ModuleName, func() vexoapp.Module {
		return staking.NewModuleWithPolicy(staking.Policy{
			UnbondingDelay:   types.Height(chain.Staking.UnbondingDelay),
			FeeCollector:     types.Address(chain.Execution.FeeCollector),
			MaxCommissionBPS: chain.Staking.MaxCommissionBPS,
		})
	})
	_ = registry.Register(appgovernance.ModuleName, func() vexoapp.Module { return appgovernance.NewModule() })
	_ = registry.Register(params.Namespace, func() vexoapp.Module { return params.NewModule(nil) })
	_ = registry.Register(appibc.ModuleName, func() vexoapp.Module { return appibc.NewModule() })
	_ = registry.Register(appevm.ModuleName, func() vexoapp.Module { return appevm.NewModule() })
	return registry.Build(chain.Application.Modules)
}

func BuildCLICommands(cfg config.ApplicationConfig) ([]vexoapp.CLICommand, error) {
	return DefaultRegistry().BuildCLICommands(cfg.Modules)
}

func NewRuntime(chainID string, cfg config.ApplicationConfig) (*vexoapp.Runtime, error) {
	return NewRuntimeWithExecution(chainID, cfg, config.ExecutionConfig{})
}

func NewRuntimeWithExecution(chainID string, cfg config.ApplicationConfig, execution config.ExecutionConfig) (*vexoapp.Runtime, error) {
	modules, err := BuildWithExecution(cfg, execution)
	if err != nil {
		return nil, err
	}
	runtime, err := vexoapp.NewRuntime(chainID, modules, vexoapp.PrefixRouter{})
	if err != nil {
		return nil, err
	}
	if execution.MinFee > 0 || execution.BaseFee > 0 || execution.MinGas > 0 || execution.MaxGas > 0 || execution.RequireNonce || execution.RequireSigned {
		runtime.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{
			MinFee:        execution.MinFee,
			BaseFee:       execution.BaseFee,
			MinGas:        execution.MinGas,
			MaxGas:        execution.MaxGas,
			RequireNonce:  execution.RequireNonce,
			RequireSigned: execution.RequireSigned,
			FeeCollector:  execution.FeeCollector,
		}))
	}
	return runtime, nil
}

func NewRuntimeWithChainConfig(chainID string, chain config.Config) (*vexoapp.Runtime, error) {
	modules, err := BuildWithChainConfig(chain)
	if err != nil {
		return nil, err
	}
	runtime, err := vexoapp.NewRuntime(chainID, modules, vexoapp.PrefixRouter{})
	if err != nil {
		return nil, err
	}
	execution := chain.Execution
	if execution.MinFee > 0 || execution.BaseFee > 0 || execution.MinGas > 0 || execution.MaxGas > 0 || execution.RequireNonce || execution.RequireSigned {
		runtime.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{
			MinFee:        execution.MinFee,
			BaseFee:       execution.BaseFee,
			MinGas:        execution.MinGas,
			MaxGas:        execution.MaxGas,
			RequireNonce:  execution.RequireNonce,
			RequireSigned: execution.RequireSigned,
			FeeCollector:  execution.FeeCollector,
		}))
	}
	return runtime, nil
}

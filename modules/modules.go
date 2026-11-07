package modules

import (
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	appevm "github.com/vexo-network/vexo-consensus/modules/evm"
	appgovernance "github.com/vexo-network/vexo-consensus/modules/governance"
	appibc "github.com/vexo-network/vexo-consensus/modules/ibc"
	appslashing "github.com/vexo-network/vexo-consensus/modules/slashing"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/params"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var defaultModuleEntries = []struct {
	name    string
	factory vexoapp.ModuleFactory
}{
	{bank.ModuleName, func() vexoapp.Module { return bank.NewModule() }},
	{staking.ModuleName, func() vexoapp.Module { return staking.NewModule() }},
	{appgovernance.ModuleName, func() vexoapp.Module { return appgovernance.NewModule() }},
	{params.Namespace, func() vexoapp.Module { return params.NewModule(nil) }},
	{appibc.ModuleName, func() vexoapp.Module { return appibc.NewModule() }},
	{appslashing.ModuleName, func() vexoapp.Module { return appslashing.NewModule() }},
	{appevm.ModuleName, func() vexoapp.Module { return appevm.NewModule() }},
}

func DefaultRegistry() (vexoapp.Registry, error) {
	registry := vexoapp.NewRegistry()
	for _, entry := range defaultModuleEntries {
		if err := registry.Register(entry.name, entry.factory); err != nil {
			return vexoapp.Registry{}, err
		}
	}
	return registry, nil
}

func Build(cfg config.ApplicationConfig) ([]vexoapp.Module, error) {
	registry, err := DefaultRegistry()
	if err != nil {
		return nil, err
	}
	return registry.Build(cfg.Modules)
}

func BuildWithExecution(cfg config.ApplicationConfig, execution config.ExecutionConfig) ([]vexoapp.Module, error) {
	chain := config.Default("vexo-chain")
	chain.Application = cfg
	chain.Execution = execution
	return BuildWithChainConfig(chain)
}

func BuildWithChainConfig(chain config.Config) ([]vexoapp.Module, error) {
	var evmModule vexoapp.Module
	if moduleEnabled(chain.Application.Modules, appevm.ModuleName) {
		module, err := appevm.NewModuleWithPolicy(appevm.Policy{
			EVMChainID:               chain.Execution.EVMChainID,
			EVMForkPreset:            chain.Execution.EVMForkPreset,
			GethChainConfigJSON:      chain.Execution.EVMChainConfigJSON,
			AllowUnprotectedLegacyTx: chain.Execution.AllowUnprotectedLegacyTx,
			MaxBlobSidecarBlobs:      chain.Execution.MaxBlobSidecarBlobs,
			MaxBlobSidecarBytes:      chain.Execution.MaxBlobSidecarBytes,
			FeeCollector:             types.Address(chain.Execution.FeeCollector),
			RPCGasCap:                chain.Execution.RPCGasCap,
			MaxTxGas:                 chain.Execution.MaxGas,
		})
		if err != nil {
			return nil, err
		}
		evmModule = module
	}
	var slashingModule vexoapp.Module
	if moduleEnabled(chain.Application.Modules, appslashing.ModuleName) {
		suite, err := crypto.NewRuntimeSuite(chain.Crypto)
		if err != nil {
			return nil, err
		}
		module, err := appslashing.NewModuleWithPolicy(appslashing.Policy{}, suite)
		if err != nil {
			return nil, err
		}
		slashingModule = module
	}
	registry := vexoapp.NewRegistry()
	if err := registry.Register(bank.ModuleName, func() vexoapp.Module {
		return bank.NewModuleWithMintAuthority(types.Address(chain.Bank.MintAuthority))
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(staking.ModuleName, func() vexoapp.Module {
		return staking.NewModuleWithPolicy(staking.Policy{
			UnbondingDelay:   types.Height(chain.Staking.UnbondingDelay),
			FeeCollector:     types.Address(chain.Execution.FeeCollector),
			MaxCommissionBPS: chain.Staking.MaxCommissionBPS,
			CryptoBackend:    chain.Crypto.Backend,
			Admission:        validator.NewConfigurableAdmissionPolicy(chain.Validator),
		})
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(appgovernance.ModuleName, func() vexoapp.Module {
		return appgovernance.NewModuleWithPolicy(chain.Governance)
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(params.Namespace, func() vexoapp.Module { return params.NewModule(nil) }); err != nil {
		return nil, err
	}
	if err := registry.Register(appibc.ModuleName, func() vexoapp.Module { return appibc.NewModule() }); err != nil {
		return nil, err
	}
	if err := registry.Register(appslashing.ModuleName, func() vexoapp.Module {
		if slashingModule != nil {
			return slashingModule
		}
		return appslashing.NewModule()
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(appevm.ModuleName, func() vexoapp.Module {
		if evmModule != nil {
			return evmModule
		}
		return appevm.NewModule()
	}); err != nil {
		return nil, err
	}
	return registry.Build(chain.Application.Modules)
}

func moduleEnabled(modules []string, name string) bool {
	for _, module := range modules {
		if module == name {
			return true
		}
	}
	return false
}

func BuildCLICommands(cfg config.ApplicationConfig) ([]vexoapp.CLICommand, error) {
	registry, err := DefaultRegistry()
	if err != nil {
		return nil, err
	}
	return registry.BuildCLICommands(cfg.Modules)
}

func BuildAllCLICommands() ([]vexoapp.CLICommand, error) {
	registry, err := DefaultRegistry()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(defaultModuleEntries))
	for _, entry := range defaultModuleEntries {
		names = append(names, entry.name)
	}
	return registry.BuildCLICommands(names)
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
	runtime.WithEVMChainID(execution.EVMChainID)
	runtime.WithAnte(newAnteKeeper(execution, modules))
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
	runtime.WithEVMChainID(execution.EVMChainID)
	runtime.WithAnte(newAnteKeeper(execution, modules))
	return runtime, nil
}

// newAnteKeeper builds the transaction admission and fee rules from the
// execution configuration. The ante is always installed: the fee market state
// and the account sequences are consensus state whatever the thresholds are.
// The evm module authenticates the unsigned Ethereum transactions addressed to
// it; no other module can.
func newAnteKeeper(execution config.ExecutionConfig, modules []vexoapp.Module) *vexoapp.AnteKeeper {
	anteConfig := vexoapp.AnteConfig{
		MinFee:        execution.MinFee,
		BaseFee:       execution.BaseFee,
		MinGas:        execution.MinGas,
		MaxGas:        execution.MaxGas,
		RequireNonce:  execution.RequireNonce,
		RequireSigned: execution.RequireSigned,
		FeeCollector:  execution.FeeCollector,
		FeeMarket: vexoapp.FeeMarketConfig{
			BlobBaseFee:              execution.BlobBaseFee,
			DynamicBaseFee:           execution.DynamicBaseFee,
			DynamicBlobBaseFee:       execution.DynamicBlobBaseFee,
			TargetGas:                execution.TargetGas,
			TargetBlobGas:            execution.TargetBlobGas,
			MaxBlobGas:               execution.MaxBlobGas,
			BaseFeeChangeDenominator: execution.BaseFeeChangeDenominator,
			BlobFeeChangeDenominator: execution.BlobFeeChangeDenominator,
			MinBaseFee:               execution.MinBaseFee,
			MaxBaseFee:               execution.MaxBaseFee,
			MinBlobBaseFee:           execution.MinBlobBaseFee,
			MaxBlobBaseFee:           execution.MaxBlobBaseFee,
		},
	}
	for _, module := range modules {
		if authenticator, ok := module.(vexoapp.TxAuthenticator); ok && module.Name() == appevm.ModuleName {
			anteConfig.EthereumAuthenticator = authenticator
		}
		if authenticator, ok := module.(vexoapp.EvidenceAuthenticator); ok && module.Name() == appslashing.ModuleName {
			anteConfig.EvidenceAuthenticator = authenticator
		}
	}
	return vexoapp.NewAnteKeeper(anteConfig)
}

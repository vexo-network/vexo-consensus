package modules

import (
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	appevm "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/staking"
)

func TestBuildDefaultModules(t *testing.T) {
	modules, err := Build(config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 5 || modules[0].Name() != "bank" || modules[1].Name() != "staking" || modules[2].Name() != "governance" || modules[3].Name() != "params" || modules[4].Name() != "ibc" {
		t.Fatalf("expected default bank module, got %+v", modules)
	}
}

func TestBuildDefaultCLICommands(t *testing.T) {
	commands, err := BuildCLICommands(config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 5 ||
		commands[0].Name != "bank" ||
		commands[1].Name != "staking" ||
		commands[2].Name != "governance" ||
		commands[3].Name != "params" ||
		commands[4].Name != "ibc" ||
		len(commands[0].Children) == 0 ||
		len(commands[1].Children) == 0 ||
		len(commands[2].Children) == 0 ||
		len(commands[3].Children) == 0 ||
		len(commands[4].Children) == 0 {
		t.Fatalf("expected default bank cli command, got %+v", commands)
	}
}

func TestBuildAllowsNoApplicationModules(t *testing.T) {
	modules, err := Build(config.ApplicationConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 0 {
		t.Fatalf("expected no modules, got %+v", modules)
	}
}

func TestBuildRejectsUnknownModule(t *testing.T) {
	_, err := Build(config.ApplicationConfig{Modules: []string{"unknown"}})
	if !errors.Is(err, vexoapp.ErrModuleNotFound) {
		t.Fatalf("expected unknown module error, got %v", err)
	}
}

func TestBuildWithExecutionInjectsStakingFeeCollector(t *testing.T) {
	modules, err := BuildWithExecution(config.ApplicationConfig{Modules: []string{"staking"}}, config.ExecutionConfig{FeeCollector: "treasury"})
	if err != nil {
		t.Fatal(err)
	}
	stakingModule, ok := modules[0].(*staking.Module)
	if !ok {
		t.Fatalf("expected staking module, got %T", modules[0])
	}
	if stakingModule.FeeCollector() != "treasury" {
		t.Fatalf("expected treasury fee collector, got %s", stakingModule.FeeCollector())
	}
}

func TestBuildWithChainConfigInjectsStakingPolicy(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"staking"}
	cfg.Execution.FeeCollector = "treasury"
	cfg.Staking.UnbondingDelay = 42
	cfg.Staking.MaxCommissionBPS = 750
	modules, err := BuildWithChainConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	stakingModule, ok := modules[0].(*staking.Module)
	if !ok {
		t.Fatalf("expected staking module, got %T", modules[0])
	}
	policy := stakingModule.Policy()
	if policy.FeeCollector != "treasury" || policy.UnbondingDelay != 42 || policy.MaxCommissionBPS != 750 {
		t.Fatalf("unexpected staking policy: %+v", policy)
	}
}

func TestBuildWithChainConfigInjectsBankMintAuthority(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"bank"}
	cfg.Bank.MintAuthority = "governance"
	modules, err := BuildWithChainConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bankModule, ok := modules[0].(bank.Module)
	if !ok {
		t.Fatalf("expected bank module, got %T", modules[0])
	}
	if bankModule.MintAuthority() != "governance" {
		t.Fatalf("expected governance mint authority, got %s", bankModule.MintAuthority())
	}
}

func TestBuildWithChainConfigWiresEVMPolicy(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"evm"}
	cfg.Execution.EVMChainID = 77
	cfg.Execution.EVMChainConfigJSON = `{"chainId":77,"homesteadBlock":0,"eip150Block":0,"eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"berlinBlock":0,"londonBlock":0,"shanghaiTime":0}`
	cfg.Execution.MaxBlobSidecarBlobs = 2
	cfg.Execution.MaxBlobSidecarBytes = 1000
	modules, err := BuildWithChainConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected one module, got %d", len(modules))
	}
	if _, ok := modules[0].(appevm.Module); !ok {
		t.Fatalf("expected EVM module, got %T", modules[0])
	}
}

func TestBuildWithChainConfigRejectsInvalidEVMChainConfig(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"evm"}
	cfg.Execution.EVMChainConfigJSON = `{invalid`
	if _, err := BuildWithChainConfig(cfg); err == nil {
		t.Fatal("expected invalid geth chain config JSON to fail")
	}
}

func TestNewRuntimeUsesConfiguredModules(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	modules := runtime.Modules()
	if len(modules) != 5 || modules[0].Name() != "bank" || modules[1].Name() != "staking" || modules[2].Name() != "governance" || modules[3].Name() != "params" || modules[4].Name() != "ibc" {
		t.Fatalf("unexpected runtime modules: %+v", modules)
	}
}

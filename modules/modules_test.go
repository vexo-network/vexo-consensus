package modules

import (
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
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

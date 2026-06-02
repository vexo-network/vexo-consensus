package modules

import (
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
)

func TestBuildDefaultModules(t *testing.T) {
	modules, err := Build(config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 || modules[0].Name() != "bank" {
		t.Fatalf("expected default bank module, got %+v", modules)
	}
}

func TestBuildDefaultCLICommands(t *testing.T) {
	commands, err := BuildCLICommands(config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Name != "bank" || len(commands[0].Children) == 0 {
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

func TestNewRuntimeUsesConfiguredModules(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	modules := runtime.Modules()
	if len(modules) != 1 || modules[0].Name() != "bank" {
		t.Fatalf("unexpected runtime modules: %+v", modules)
	}
}

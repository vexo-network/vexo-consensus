package app

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRegistryBuildsEnabledModulesInOrder(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("bank", func() Module { return &recordingModule{name: "bank"} }); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("staking", func() Module { return &recordingModule{name: "staking"} }); err != nil {
		t.Fatal(err)
	}

	modules, err := registry.Build([]string{"staking", "bank"})
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 2 || modules[0].Name() != "staking" || modules[1].Name() != "bank" {
		t.Fatalf("unexpected modules: %+v", modules)
	}
}

func TestRegistryRejectsInvalidRegistrationsAndUnknownModules(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("", func() Module { return &recordingModule{name: "bank"} }); !errors.Is(err, ErrModuleNameRequired) {
		t.Fatalf("expected module name required, got %v", err)
	}
	if err := registry.Register("bank", nil); !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("expected nil factory rejection, got %v", err)
	}
	if err := registry.Register("bank", func() Module { return &recordingModule{name: "bank"} }); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("bank", func() Module { return &recordingModule{name: "bank"} }); !errors.Is(err, ErrModuleRegistered) {
		t.Fatalf("expected duplicate module rejection, got %v", err)
	}
	if _, err := registry.Build([]string{"unknown"}); !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("expected unknown module rejection, got %v", err)
	}
}

func TestRegistryBuildsCLICommands(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("bank", func() Module { return &cliModule{name: "bank"} }); err != nil {
		t.Fatal(err)
	}

	commands, err := registry.BuildCLICommands([]string{"bank"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Name != "bank" || commands[0].Usage == "" {
		t.Fatalf("unexpected cli commands: %+v", commands)
	}
	var buffer bytes.Buffer
	if err := commands[0].Run(&buffer, []string{"ping"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "bank:ping") {
		t.Fatalf("unexpected command output: %s", buffer.String())
	}
}

type cliModule struct {
	recordingModule
	name string
}

func (module *cliModule) Name() string {
	return module.name
}

func (module *cliModule) CLICommands() []CLICommand {
	return []CLICommand{
		{
			Name:        module.name,
			Usage:       module.name + " ping",
			Description: "test module command",
			Run: func(writer io.Writer, args []string) error {
				_, err := writer.Write([]byte(module.name + ":" + strings.Join(args, ",")))
				return err
			},
		},
	}
}

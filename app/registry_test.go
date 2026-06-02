package app

import (
	"errors"
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

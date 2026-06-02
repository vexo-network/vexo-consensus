package upgrade

import (
	"context"
	"errors"
	"testing"
)

func TestApplyGovernanceUpgradeAtHeight(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConfig(Migration{From: 1, To: 2})
	registry.RegisterStore(Migration{From: 1, To: 2})
	registry.RegisterAppState(Migration{From: 1, To: 2})

	result, err := Apply(context.Background(), State{
		Height:              10,
		BinaryVersion:       "v0.1.0",
		ConfigSchemaVersion: 1,
		StoreSchemaVersion:  1,
		AppStateVersion:     1,
	}, Plan{
		Name:               "v0.2.0",
		Height:             10,
		BinaryVersion:      "v0.2.0",
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     2,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   2,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.BinaryVersion != "v0.2.0" || result.StoreSchemaVersion != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplyWaitsForUpgradeHeight(t *testing.T) {
	result, err := Apply(context.Background(), State{Height: 9}, Plan{Name: "v2", Height: 10}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Fatalf("upgrade should wait for height: %+v", result)
	}
}

func TestApplyRequiresRollbackOnMigrationFailure(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConfig(Migration{From: 1, To: 2, Run: func(context.Context) error { return errors.New("boom") }})
	_, err := Apply(context.Background(), State{Height: 10, ConfigSchemaVersion: 1, StoreSchemaVersion: 1}, Plan{
		Name:             "v2",
		Height:           10,
		ConfigSchemaFrom: 1,
		ConfigSchemaTo:   2,
		StoreSchemaFrom:  1,
		StoreSchemaTo:    1,
	}, registry)
	if !errors.Is(err, ErrRollbackRequired) {
		t.Fatalf("expected rollback required, got %v", err)
	}
}

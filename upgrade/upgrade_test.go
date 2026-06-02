package upgrade

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
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

func TestApplyRejectsAppStateVersionMismatch(t *testing.T) {
	_, err := Apply(context.Background(), State{Height: 10, ConfigSchemaVersion: 1, StoreSchemaVersion: 1, AppStateVersion: 9}, Plan{
		Name:               "v2",
		Height:             10,
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     1,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      1,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   1,
	}, nil)
	if !errors.Is(err, ErrAppVersionMismatch) {
		t.Fatalf("expected app version mismatch, got %v", err)
	}
}

func TestExecutorPersistsAppliedRecord(t *testing.T) {
	recorder := JSONFileRecorder{Path: filepath.Join(t.TempDir(), "upgrades.json")}
	executor := NewExecutor(NewRegistry(), recorder)
	executor.Now = func() time.Time { return time.Unix(100, 0) }

	record, err := executor.Execute(context.Background(), State{
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
		ConfigSchemaTo:     1,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      1,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != ExecutionApplied || record.UpdatedAt != 100 {
		t.Fatalf("unexpected record: %+v", record)
	}

	reloaded, found, err := recorder.Load(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !found || reloaded.Status != ExecutionApplied || reloaded.Result.BinaryVersion != "v0.2.0" {
		t.Fatalf("unexpected persisted record: %+v found=%t", reloaded, found)
	}
}

func TestExecutorPersistsRollbackRequiredAndBlocksRetry(t *testing.T) {
	recorder := NewMemoryRecorder()
	registry := NewRegistry()
	registry.RegisterStore(Migration{From: 1, To: 2, Run: func(context.Context) error { return errors.New("store boom") }})
	executor := NewExecutor(registry, recorder)
	plan := Plan{
		Name:               "bad-upgrade",
		Height:             10,
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     1,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   1,
	}
	_, err := executor.Execute(context.Background(), State{
		Height:              10,
		ConfigSchemaVersion: 1,
		StoreSchemaVersion:  1,
		AppStateVersion:     1,
	}, plan)
	if !errors.Is(err, ErrRollbackRequired) {
		t.Fatalf("expected rollback required, got %v", err)
	}
	record := recorder.Records["bad-upgrade"]
	if record.Status != ExecutionRollbackRequired || record.Error == "" {
		t.Fatalf("expected rollback record, got %+v", record)
	}
	_, err = executor.Execute(context.Background(), State{
		Height:              10,
		ConfigSchemaVersion: 1,
		StoreSchemaVersion:  1,
		AppStateVersion:     1,
	}, plan)
	if !errors.Is(err, ErrRollbackRequired) {
		t.Fatalf("expected retry to stay blocked, got %v", err)
	}
}

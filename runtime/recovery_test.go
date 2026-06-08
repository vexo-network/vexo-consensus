package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/store"
)

func TestRuntimeRecoverWithoutStore(t *testing.T) {
	runtime, err := NewEphemeral(config.Default("vexo-test"), noopApp{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.Recover(context.Background())
	if !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected state not found, got %v", err)
	}
}

func TestRuntimeRecoverPropagatesStoreError(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.Recover(context.Background())
	if !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected state not found, got %v", err)
	}
}

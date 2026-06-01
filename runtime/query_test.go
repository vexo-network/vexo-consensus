package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestRuntimeQueriesReturnNotFoundWithoutStore(t *testing.T) {
	runtime, err := New(config.Default("vexo-test"), noopApp{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.BlockByHeight(context.Background(), 1); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found, got %v", err)
	}
	if _, err := runtime.BlockByHash(context.Background(), types.Hash{1}); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found, got %v", err)
	}
	if _, err := runtime.BlockIndex(context.Background()); !errors.Is(err, store.ErrBlockIndexNotFound) {
		t.Fatalf("expected block index not found, got %v", err)
	}
	if _, err := runtime.LatestState(context.Background()); !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected state not found, got %v", err)
	}
	if _, err := runtime.StateRoot(context.Background(), 1, "bank"); !errors.Is(err, store.ErrStateRootNotFound) {
		t.Fatalf("expected state root not found, got %v", err)
	}
	if _, err := runtime.PruneBelow(context.Background(), 1); !errors.Is(err, store.ErrBlockIndexNotFound) {
		t.Fatalf("expected block index not found, got %v", err)
	}
}

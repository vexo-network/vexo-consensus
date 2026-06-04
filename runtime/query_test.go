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
	if _, err := runtime.QueryEvents(context.Background(), "sender", "alice"); err == nil {
		t.Fatal("expected event query without store to fail")
	}
	if _, err := runtime.QueryProof(context.Background(), 0, "bank", []byte("alice")); !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected proof state not found, got %v", err)
	}
	if _, err := runtime.PruneBelow(context.Background(), 1); !errors.Is(err, store.ErrBlockIndexNotFound) {
		t.Fatalf("expected block index not found, got %v", err)
	}
}

func TestRuntimeQueryProofUsesLatestHeightOnly(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := storage.Set(ctx, "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(ctx, store.StateRecord{Height: 3, AppHash: types.Hash{3}}); err != nil {
		t.Fatal(err)
	}
	proof, err := runtime.QueryProof(ctx, 0, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if proof.ChainID != "vexo-test" || proof.Height != 3 || !proof.Exists {
		t.Fatalf("unexpected proof: %+v", proof)
	}
	if _, err := runtime.QueryProof(ctx, 2, "bank", []byte("alice")); !errors.Is(err, ErrHistoricalQueryProofUnsupported) {
		t.Fatalf("expected historical proof rejection, got %v", err)
	}
}

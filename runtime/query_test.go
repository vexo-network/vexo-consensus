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

func TestRuntimeQueryProofSupportsHistoricalHeight(t *testing.T) {
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
	commitQueryProofState(t, storage, 1, []byte("100"))
	commitQueryProofState(t, storage, 2, []byte("200"))
	proof, err := runtime.QueryProof(ctx, 0, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if proof.ChainID != "vexo-test" || proof.Height != 2 || !proof.Exists || string(proof.Value) != "200" {
		t.Fatalf("unexpected proof: %+v", proof)
	}
	historicalProof, err := runtime.QueryProof(ctx, 1, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if historicalProof.Height != 1 || !historicalProof.Exists || string(historicalProof.Value) != "100" {
		t.Fatalf("unexpected historical proof: %+v", historicalProof)
	}
}

func commitQueryProofState(t *testing.T, storage *store.LevelDBStore, height types.Height, value []byte) {
	t.Helper()
	ctx := context.Background()
	writes := []store.KVWrite{{Namespace: "bank", Key: []byte("alice"), Value: value}}
	root, err := storage.RootWithWrites(ctx, "bank", writes)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.CommitBlockStateWithWrites(
		ctx,
		writes,
		store.BlockRecord{Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: height}}, Hash: types.Hash{byte(height)}, AppHash: types.Hash{byte(height)}},
		store.StateRecord{Height: height, AppHash: types.Hash{byte(height)}, LastBlockHash: types.Hash{byte(height)}},
		[]store.StateRootRecord{{Height: height, Namespace: "bank", Root: root}},
	); err != nil {
		t.Fatal(err)
	}
}

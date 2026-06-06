package runtime

import (
	"context"
	"errors"
	"strconv"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/queryproof"
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
	if err := queryproof.Verify(proof, "vexo-test", 2, proof.StateRoot); err != nil {
		t.Fatal(err)
	}
	historicalProof, err := runtime.QueryProof(ctx, 1, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if historicalProof.Height != 1 || !historicalProof.Exists || string(historicalProof.Value) != "100" {
		t.Fatalf("unexpected historical proof: %+v", historicalProof)
	}
	if err := queryproof.Verify(historicalProof, "vexo-test", 1, historicalProof.StateRoot); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimePruneCallsAppModuleHooks(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	for height := types.Height(1); height <= 2; height++ {
		if err := storage.SaveBlock(context.Background(), store.BlockRecord{
			Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: height}},
			Hash:  types.Hash{byte(height)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	module := &pruneHookModule{}
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{module}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.PruneBelow(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.RetainFromHeight != 2 || module.retainFrom != 2 {
		t.Fatalf("expected module prune hook at retain height 2, result=%+v module=%d", result, module.retainFrom)
	}
	if value, err := storage.Get(context.Background(), "hook", []byte("retain")); err != nil || string(value) != "2" {
		t.Fatalf("expected hook store write, value=%q err=%v", value, err)
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

type pruneHookModule struct {
	retainFrom types.Height
}

func (module *pruneHookModule) Name() string { return "hook" }

func (module *pruneHookModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module *pruneHookModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module *pruneHookModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module *pruneHookModule) EndBlock(ctx vexoapp.Context) error { return nil }

func (module *pruneHookModule) Prune(ctx vexoapp.Context, retainFrom types.Height) error {
	module.retainFrom = retainFrom
	return ctx.Store.Set(ctx.GoContext(), "hook", []byte("retain"), []byte(strconv.FormatUint(uint64(retainFrom), 10)))
}

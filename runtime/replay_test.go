package runtime

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRuntimeReplayStoredBlocks(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	for height := types.Height(1); height <= 2; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}

	replayApplication, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	replayRuntime, err := NewWithStore(config.Default("vexo-test"), replayApplication, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	result, err := replayRuntime.Replay(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocks != 2 || result.ToHeight != 2 {
		t.Fatalf("unexpected replay result: %+v", result)
	}
	commit, err := replayApplication.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if commit.Height != 0 {
		t.Fatalf("expected live replay app to remain untouched, got height %d", commit.Height)
	}

	index, err := replayRuntime.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 1 || index.LatestHeight != 2 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected replay block index: %+v", index)
	}

	replayAllResult, err := replayRuntime.ReplayAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if replayAllResult.Blocks != 2 || replayAllResult.FromHeight != 1 || replayAllResult.ToHeight != 2 {
		t.Fatalf("unexpected replay all result: %+v", replayAllResult)
	}

	pruneResult, err := replayRuntime.PruneBelow(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if pruneResult.PrunedBlocks != 1 {
		t.Fatalf("expected one pruned block, got %+v", pruneResult)
	}
	if _, err := replayRuntime.BlockByHeight(context.Background(), 1); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected pruned block not found, got %v", err)
	}
}

func TestRuntimeReplayFromHistoricalSnapshotReexecutesApp(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 3; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := runtime.ReplayFromHistoricalSnapshot(context.Background(), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.FromHeight != 2 || result.ToHeight != 3 || result.Blocks != 2 {
		t.Fatalf("unexpected historical replay result: %+v", result)
	}
}

func TestRuntimeReplayStrictFallsBackToGenesisReplayWhenSnapshotUnavailable(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 3; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}

	replayApplication, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	replayRuntime, err := NewWithStore(config.Default("vexo-test"), replayApplication, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, noHistoricalStore{Store: storage})
	if err != nil {
		t.Fatal(err)
	}
	result, err := replayRuntime.ReplayStrict(context.Background(), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.FromHeight != 2 || result.ToHeight != 3 || result.Blocks != 2 {
		t.Fatalf("unexpected strict genesis fallback result: %+v", result)
	}
}

func TestRuntimeReplayStrictDoesNotFallbackToStoredRange(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	appHash := types.Hash{9}
	if err := storage.SaveBlock(context.Background(), store.BlockRecord{
		Block:   types.Block{Header: types.Header{ChainID: "vexo-test", Height: 2}},
		Hash:    types.Hash{2},
		AppHash: appHash,
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(context.Background(), store.StateRecord{Height: 2, AppHash: appHash}); err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.Replay(context.Background(), 2, 2); err == nil {
		t.Fatal("expected default replay to reject missing isolated historical snapshot")
	}
	stored, err := runtime.ReplayStoredRange(context.Background(), 2, 2)
	if err != nil {
		t.Fatalf("expected explicit stored range replay to verify metadata, got %v", err)
	}
	if stored.Blocks != 1 || stored.FromHeight != 2 || stored.ToHeight != 2 {
		t.Fatalf("unexpected stored range replay result: %+v", stored)
	}
	if _, err := runtime.ReplayStrict(context.Background(), 2, 2); err == nil {
		t.Fatal("expected strict replay to reject missing isolated historical snapshot")
	}
}

type noHistoricalStore struct {
	store.Store
}

func TestRuntimeReplayRejectsInvalidRange(t *testing.T) {
	runtime, err := NewEphemeral(config.Default("vexo-test"), noopApp{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.Replay(context.Background(), 2, 1)
	if !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("without store should fail before range validation, got %v", err)
	}

	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	runtime, err = NewWithStore(config.Default("vexo-test"), noopApp{}, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.Replay(context.Background(), 2, 1)
	if !errors.Is(err, ErrInvalidReplayRange) {
		t.Fatalf("expected invalid replay range, got %v", err)
	}
}

func TestRuntimeReplayAllWithoutIndex(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.ReplayAll(context.Background())
	if !errors.Is(err, store.ErrBlockIndexNotFound) {
		t.Fatalf("expected block index not found, got %v", err)
	}
}

func TestRuntimeReplayDetectsAppHashMismatch(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	block := types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}}
	if err := storage.SaveBlock(context.Background(), store.BlockRecord{
		Block:   block,
		Hash:    types.Hash{1},
		AppHash: types.Hash{9},
	}); err != nil {
		t.Fatal(err)
	}

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.Replay(context.Background(), 1, 1)
	if !errors.Is(err, ErrReplayAppHashMismatch) {
		t.Fatalf("expected app hash mismatch, got %v", err)
	}
}

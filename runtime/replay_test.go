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
	if commit.Height != 2 {
		t.Fatalf("expected replayed commit height 2, got %d", commit.Height)
	}
}

func TestRuntimeReplayRejectsInvalidRange(t *testing.T) {
	runtime, err := New(config.Default("vexo-test"), noopApp{}, nil, nil)
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

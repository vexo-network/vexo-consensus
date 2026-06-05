package runtime

import (
	"context"
	"errors"
	"os"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInvalidReplayRange         = errors.New("invalid replay range")
	ErrReplayAppHashMismatch      = errors.New("replayed app hash does not match stored app hash")
	ErrReplayRequiresIsolatedApp  = errors.New("replay requires isolated app factory")
	ErrReplayRequiresGenesisStart = errors.New("isolated replay requires genesis start")
)

type ReplayResult struct {
	FromHeight types.Height
	ToHeight   types.Height
	LastHash   types.Hash
	Blocks     uint64
}

type IsolatedReplayAppFactory interface {
	NewReplayApp(store app.StateStore) (app.Application, error)
}

func (runtime *Runtime) Replay(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from == 0 || to == 0 || from > to {
		return ReplayResult{}, ErrInvalidReplayRange
	}
	return runtime.ReplayIsolated(ctx, from, to)
}

func (runtime *Runtime) ReplayIsolated(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from == 0 || to == 0 || from > to {
		return ReplayResult{}, ErrInvalidReplayRange
	}
	if from != 1 {
		result, err := runtime.ReplayFromHistoricalSnapshot(ctx, from, to)
		if err == nil {
			return result, nil
		}
		if !canFallbackToStoredReplay(err) {
			return ReplayResult{}, err
		}
		return runtime.ReplayStoredRange(ctx, from, to)
	}
	return runtime.replayFromFreshStore(ctx, from, to)
}

func canFallbackToStoredReplay(err error) bool {
	return errors.Is(err, ErrReplayRequiresIsolatedApp) ||
		errors.Is(err, ErrReplayRequiresGenesisStart) ||
		errors.Is(err, store.ErrStateNotFound) ||
		errors.Is(err, store.ErrStateRootNotFound)
}

func (runtime *Runtime) replayFromFreshStore(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	factory, ok := runtime.App.(IsolatedReplayAppFactory)
	if !ok {
		return ReplayResult{}, ErrReplayRequiresIsolatedApp
	}
	tempDir, err := os.MkdirTemp("", "vexo-replay-*")
	if err != nil {
		return ReplayResult{}, err
	}
	defer os.RemoveAll(tempDir)
	replayStore, err := store.OpenLevelDB(tempDir)
	if err != nil {
		return ReplayResult{}, err
	}
	defer replayStore.Close()
	replayApp, err := factory.NewReplayApp(replayStore)
	if err != nil {
		return ReplayResult{}, err
	}
	return runtime.ReplayWithApp(ctx, from, to, replayApp)
}

func (runtime *Runtime) ReplayFromHistoricalSnapshot(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from <= 1 || to == 0 || from > to {
		return ReplayResult{}, ErrReplayRequiresGenesisStart
	}
	historicalStore, ok := runtime.Store.(store.HistoricalSnapshotKVStore)
	if !ok {
		return ReplayResult{}, ErrReplayRequiresGenesisStart
	}
	factory, ok := runtime.App.(IsolatedReplayAppFactory)
	if !ok {
		return ReplayResult{}, ErrReplayRequiresIsolatedApp
	}
	baseHeight := from - 1
	baseState, err := runtime.Store.StateByHeight(ctx, baseHeight)
	if err != nil {
		return ReplayResult{}, err
	}
	tempDir, err := os.MkdirTemp("", "vexo-replay-*")
	if err != nil {
		return ReplayResult{}, err
	}
	defer os.RemoveAll(tempDir)
	replayStore, err := store.OpenLevelDB(tempDir)
	if err != nil {
		return ReplayResult{}, err
	}
	defer replayStore.Close()
	for _, module := range runtime.AppModules() {
		pairs, err := historicalStore.ExportNamespaceAt(ctx, baseHeight, module.Name())
		if err != nil {
			return ReplayResult{}, err
		}
		if err := replayStore.ImportNamespace(ctx, module.Name(), pairs); err != nil {
			return ReplayResult{}, err
		}
	}
	replayApp, err := factory.NewReplayApp(replayStore)
	if err != nil {
		return ReplayResult{}, err
	}
	if appRuntime, ok := replayApp.(*app.Runtime); ok {
		appRuntime.Restore(baseHeight, baseState.AppHash)
	}
	return runtime.ReplayWithApp(ctx, from, to, replayApp)
}

func (runtime *Runtime) ReplayStoredRange(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from == 0 || to == 0 || from > to {
		return ReplayResult{}, ErrInvalidReplayRange
	}
	result := ReplayResult{FromHeight: from, ToHeight: to}
	for height := from; height <= to; height++ {
		record, err := runtime.Store.BlockByHeight(ctx, height)
		if err != nil {
			return ReplayResult{}, err
		}
		state, err := runtime.Store.StateByHeight(ctx, height)
		if err != nil {
			return ReplayResult{}, err
		}
		if state.AppHash != record.AppHash {
			return ReplayResult{}, ErrReplayAppHashMismatch
		}
		result.LastHash = record.AppHash
		result.Blocks++
	}
	return result, nil
}

func (runtime *Runtime) ReplayWithApp(ctx context.Context, from types.Height, to types.Height, replayApp app.Application) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from == 0 || to == 0 || from > to {
		return ReplayResult{}, ErrInvalidReplayRange
	}

	result := ReplayResult{FromHeight: from, ToHeight: to}
	for height := from; height <= to; height++ {
		record, err := runtime.Store.BlockByHeight(ctx, height)
		if err != nil {
			return ReplayResult{}, err
		}
		response, err := runtime.Executor.Execute(ctx, replayApp, record.Block)
		if err != nil {
			return ReplayResult{}, err
		}
		if response.AppHash != record.AppHash {
			return ReplayResult{}, ErrReplayAppHashMismatch
		}
		if appRuntime, ok := replayApp.(*app.Runtime); ok {
			appRuntime.Restore(height, response.AppHash)
		}
		result.LastHash = response.AppHash
		result.Blocks++
	}
	return result, nil
}

func (runtime *Runtime) ReplayAll(ctx context.Context) (ReplayResult, error) {
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		return ReplayResult{}, err
	}
	return runtime.Replay(ctx, index.EarliestHeight, index.LatestHeight)
}

package runtime

import (
	"context"
	"errors"
	"os"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
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
	return runtime.ReplayStrict(ctx, from, to)
}

func (runtime *Runtime) ReplayStrict(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
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
		if canFallbackToStoredReplay(err) {
			return runtime.ReplayStrictFromGenesis(ctx, from, to)
		}
		return ReplayResult{}, err
	}
	return runtime.replayFromFreshStore(ctx, from, to)
}

func (runtime *Runtime) ReplayIsolated(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from == 0 || to == 0 || from > to {
		return ReplayResult{}, ErrInvalidReplayRange
	}
	if from != 1 {
		return runtime.ReplayFromHistoricalSnapshot(ctx, from, to)
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
	if err := runtime.importValidatorRegistry(ctx, to, replayStore); err != nil {
		return ReplayResult{}, err
	}
	replayApp, err := factory.NewReplayApp(replayStore)
	if err != nil {
		return ReplayResult{}, err
	}
	// The chain began with InitChain: its genesis state is part of every app hash. A chain whose
	// genesis was never persisted had no InitChain to repeat.
	request, found, err := runtime.storedInitChain(ctx)
	if err != nil {
		return ReplayResult{}, err
	}
	if found {
		if _, err := replayApp.InitChain(request); err != nil {
			return ReplayResult{}, err
		}
	}
	return runtime.ReplayWithApp(ctx, from, to, replayApp)
}

// importValidatorRegistry copies the validator-set snapshots up to height into the store of a
// replay. Modules that judge past events, such as evidence, read the validator set of the height
// they judge from state; a lookup returns the latest snapshot at or below that height, so snapshots
// of later heights are never read by an earlier block.
func (runtime *Runtime) importValidatorRegistry(ctx context.Context, height types.Height, replayStore *store.LevelDBStore) error {
	pairs, err := runtime.Store.ExportNamespaceAt(ctx, height, validator.RegistryNamespace)
	if err != nil {
		return err
	}
	return replayStore.ImportNamespace(ctx, validator.RegistryNamespace, pairs)
}

func (runtime *Runtime) ReplayStrictFromGenesis(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from <= 1 || to == 0 || from > to {
		return ReplayResult{}, ErrInvalidReplayRange
	}
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		return ReplayResult{}, err
	}
	if index.EarliestHeight != 1 {
		return ReplayResult{}, ErrReplayRequiresGenesisStart
	}
	result, err := runtime.replayFromFreshStore(ctx, 1, to)
	if err != nil {
		return ReplayResult{}, err
	}
	result.FromHeight = from
	result.Blocks = uint64(to - from + 1)
	return result, nil
}

func (runtime *Runtime) ReplayFromHistoricalSnapshot(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
	if runtime.Store == nil {
		return ReplayResult{}, store.ErrBlockNotFound
	}
	if from <= 1 || to == 0 || from > to {
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
	for _, namespace := range runtime.replayNamespaces() {
		pairs, err := runtime.Store.ExportNamespaceAt(ctx, baseHeight, namespace)
		if err != nil {
			return ReplayResult{}, err
		}
		if err := replayStore.ImportNamespace(ctx, namespace, pairs); err != nil {
			return ReplayResult{}, err
		}
	}
	if err := runtime.importValidatorRegistry(ctx, to, replayStore); err != nil {
		return ReplayResult{}, err
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

func (runtime *Runtime) replayNamespaces() []string {
	seen := make(map[string]struct{})
	namespaces := make([]string, 0, len(runtime.AppModules()))
	add := func(namespace string) {
		if namespace == "" {
			return
		}
		if _, found := seen[namespace]; found {
			return
		}
		seen[namespace] = struct{}{}
		namespaces = append(namespaces, namespace)
	}
	for _, namespace := range app.CoreNamespaces() {
		add(namespace)
	}
	for _, module := range runtime.AppModules() {
		add(module.Name())
		if provider, ok := module.(app.ReplayNamespaceProvider); ok {
			for _, namespace := range provider.ReplayNamespaces() {
				add(namespace)
			}
		}
	}
	return namespaces
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

func (runtime *Runtime) ReplayAllStrict(ctx context.Context) (ReplayResult, error) {
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		return ReplayResult{}, err
	}
	return runtime.ReplayStrict(ctx, index.EarliestHeight, index.LatestHeight)
}

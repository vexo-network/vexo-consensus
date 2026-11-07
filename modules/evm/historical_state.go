package evm

import (
	"context"
	"errors"
	"sync"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// Historical Ethereum state is read from the store's per-key versions: the
// module keeps no copies of its accounts per height.

// ErrHistoricalStateUnavailable reports a store that keeps no state history.
var ErrHistoricalStateUnavailable = errors.New("historical EVM state is unavailable")

// ErrHistoricalStateReadOnly reports a write to the state of a past height.
var ErrHistoricalStateReadOnly = errors.New("historical EVM state is read-only")

const errHistoricalStateNotFound = "EVM state at the requested height is not available"

type historicalKVStore interface {
	vexostore.HistoricalKVStore
	vexostore.HistoricalPrefixKVStore
}

// historicalState is the read-only state of one committed height.
type historicalState struct {
	current vexoapp.StateStore
	history historicalKVStore
	height  types.Height
}

// stateAtHeight returns the state of a committed height, or the current state
// for height 0.
func stateAtHeight(stateStore vexoapp.StateStore, height uint64) (vexoapp.StateStore, error) {
	if stateStore == nil {
		return nil, ErrStoreMissing
	}
	if height == 0 {
		return stateStore, nil
	}
	history, ok := stateStore.(historicalKVStore)
	if !ok {
		return nil, ErrHistoricalStateUnavailable
	}
	return historicalState{current: stateStore, history: history, height: types.Height(height)}, nil
}

func isHistoricalStateUnavailable(err error) bool {
	return errors.Is(err, vexostore.ErrStateNotFound) || errors.Is(err, ErrHistoricalStateUnavailable)
}

func (state historicalState) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	return state.history.GetAt(ctx, state.height, namespace, key)
}

func (state historicalState) ExportPrefix(ctx context.Context, namespace string, prefix []byte) ([]vexostore.KVPair, error) {
	return state.history.ExportPrefixAt(ctx, state.height, namespace, prefix)
}

func (historicalState) Set(context.Context, string, []byte, []byte) error {
	return ErrHistoricalStateReadOnly
}

func (historicalState) Delete(context.Context, string, []byte) error {
	return ErrHistoricalStateReadOnly
}

// BlockByHeight serves BLOCKHASH: block records do not change with state.
func (state historicalState) BlockByHeight(ctx context.Context, height types.Height) (vexostore.BlockRecord, error) {
	blocks, ok := state.current.(interface {
		BlockByHeight(context.Context, types.Height) (vexostore.BlockRecord, error)
	})
	if !ok {
		return vexostore.BlockRecord{}, vexostore.ErrBlockNotFound
	}
	return blocks.BlockByHeight(ctx, height)
}

// stateRootCacheSize bounds the Ethereum state roots kept per module: each is a
// computation over every account, and block queries ask for recent heights.
const stateRootCacheSize = 256

// stateRootCache remembers the Ethereum state roots of committed heights. The
// state of a committed height never changes, so a root computed once stays
// right; the current state (height 0) is never cached.
type stateRootCache struct {
	mu    sync.Mutex
	roots map[uint64]string
}

func newStateRootCache() *stateRootCache {
	return &stateRootCache{roots: make(map[uint64]string)}
}

func (cache *stateRootCache) stateRoot(ctx context.Context, stateStore vexoapp.StateStore, height uint64) (string, error) {
	if cache != nil && height > 0 {
		cache.mu.Lock()
		root, found := cache.roots[height]
		cache.mu.Unlock()
		if found {
			return root, nil
		}
	}
	accounts, err := ethereumAccountsAtHeight(ctx, stateStore, height)
	if err != nil {
		return "", err
	}
	root, err := ethcompat.StateRoot(accounts)
	if err != nil {
		return "", err
	}
	if cache != nil && height > 0 {
		cache.mu.Lock()
		if len(cache.roots) >= stateRootCacheSize {
			lowest := height
			for cached := range cache.roots {
				if cached < lowest {
					lowest = cached
				}
			}
			delete(cache.roots, lowest)
		}
		if len(cache.roots) < stateRootCacheSize {
			cache.roots[height] = root
		}
		cache.mu.Unlock()
	}
	return root, nil
}

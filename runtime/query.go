package runtime

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrHistoricalQueryProofUnsupported = errors.New("historical query proof is unsupported without historical KV snapshots")
var ErrAppQueryUnavailable = errors.New("application query is unavailable")

func (runtime *Runtime) BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error) {
	if runtime.Store == nil {
		return store.BlockRecord{}, store.ErrBlockNotFound
	}
	return runtime.Store.BlockByHeight(ctx, height)
}

func (runtime *Runtime) BlockByHash(ctx context.Context, hash types.Hash) (store.BlockRecord, error) {
	if runtime.Store == nil {
		return store.BlockRecord{}, store.ErrBlockNotFound
	}
	return runtime.Store.BlockByHash(ctx, hash)
}

func (runtime *Runtime) BlockIndex(ctx context.Context) (store.BlockIndex, error) {
	if runtime.Store == nil {
		return store.BlockIndex{}, store.ErrBlockIndexNotFound
	}
	return runtime.Store.BlockIndex(ctx)
}

func (runtime *Runtime) LatestState(ctx context.Context) (store.StateRecord, error) {
	if runtime.Store == nil {
		return store.StateRecord{}, store.ErrStateNotFound
	}
	return runtime.Store.LatestState(ctx)
}

func (runtime *Runtime) StateByHeight(ctx context.Context, height types.Height) (store.StateRecord, error) {
	if runtime.Store == nil {
		return store.StateRecord{}, store.ErrStateNotFound
	}
	return runtime.Store.StateByHeight(ctx, height)
}

func (runtime *Runtime) StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error) {
	if runtime.Store == nil {
		return store.StateRootRecord{}, store.ErrStateRootNotFound
	}
	return runtime.Store.StateRoot(ctx, height, namespace)
}

func (runtime *Runtime) QueryEvents(ctx context.Context, key string, value string) ([]events.Record, error) {
	if runtime.Store == nil {
		return nil, events.ErrStoreMissing
	}
	return events.NewIndexer(runtime.Store).Query(ctx, key, value)
}

func (runtime *Runtime) QueryProof(ctx context.Context, height types.Height, namespace string, key []byte) (queryproof.Proof, error) {
	if runtime.Store == nil {
		return queryproof.Proof{}, store.ErrStateNotFound
	}
	state, err := runtime.Store.LatestState(ctx)
	if err != nil {
		return queryproof.Proof{}, err
	}
	if height == 0 {
		height = state.Height
	}
	if height != state.Height {
		return queryproof.Proof{}, ErrHistoricalQueryProofUnsupported
	}
	return queryproof.Build(ctx, runtime.Store, runtime.Config.ChainID, state.Height, namespace, key)
}

func (runtime *Runtime) IBCQuery(ctx context.Context, path []string) (app.QueryResponse, error) {
	if runtime.App == nil {
		return app.QueryResponse{}, ErrAppQueryUnavailable
	}
	select {
	case <-ctx.Done():
		return app.QueryResponse{}, ctx.Err()
	default:
	}
	queryPath := append([]string{"ibc"}, path...)
	response := runtime.App.Query(app.QueryRequest{Path: queryPath})
	return app.QueryResponse{
		Code:  response.Code,
		Value: append([]byte(nil), response.Value...),
		Log:   response.Log,
	}, nil
}

func (runtime *Runtime) PruneBelow(ctx context.Context, retainFrom types.Height) (store.PruneResult, error) {
	if runtime.Store == nil {
		return store.PruneResult{}, store.ErrBlockIndexNotFound
	}
	return runtime.Store.PruneBelow(ctx, retainFrom)
}

func (runtime *Runtime) PruneByRetention(ctx context.Context, policy store.RetentionPolicy) (store.PruneResult, error) {
	if runtime.Store == nil {
		return store.PruneResult{}, store.ErrBlockIndexNotFound
	}
	return runtime.Store.PruneByRetention(ctx, policy)
}

func (runtime *Runtime) RecoverIndexes(ctx context.Context) (store.RecoverResult, error) {
	if runtime.Store == nil {
		return store.RecoverResult{}, store.ErrBlockIndexNotFound
	}
	return runtime.Store.RecoverIndexes(ctx)
}

func (runtime *Runtime) Compact(ctx context.Context) error {
	compacted := false
	if runtime.Mempool != nil {
		if err := runtime.Mempool.CompactWAL(ctx); err != nil {
			return err
		}
		compacted = true
	}
	if runtime.Store == nil {
		if compacted {
			return nil
		}
		return store.ErrBlockIndexNotFound
	}
	return runtime.Store.Compact(ctx)
}

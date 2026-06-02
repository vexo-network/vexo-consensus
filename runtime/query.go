package runtime

import (
	"context"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

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
	if runtime.Store == nil {
		return store.ErrBlockIndexNotFound
	}
	return runtime.Store.Compact(ctx)
}

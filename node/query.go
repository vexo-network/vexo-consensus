package node

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func (node *Node) BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockRecord{}, err
	}
	return runtime.BlockByHeight(ctx, height)
}

func (node *Node) LatestBlock(ctx context.Context) (store.BlockRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockRecord{}, err
	}
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		return store.BlockRecord{}, err
	}
	return runtime.BlockByHeight(ctx, index.LatestHeight)
}

func (node *Node) BlockIndex(ctx context.Context) (store.BlockIndex, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockIndex{}, err
	}
	return runtime.BlockIndex(ctx)
}

func (node *Node) LatestState(ctx context.Context) (store.StateRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.StateRecord{}, err
	}
	return runtime.LatestState(ctx)
}

func (node *Node) StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.StateRootRecord{}, err
	}
	return runtime.StateRoot(ctx, height, namespace)
}

func (node *Node) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Validators.ValidatorSet(ctx, height)
}

func (node *Node) Committee(ctx context.Context, height types.Height, round types.Round, seed types.Hash) (committee.Committee, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return committee.Committee{}, err
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return committee.Committee{}, err
	}
	epoch := uint64(0)
	if runtime.Config.Committee.EpochLength > 0 && height > 0 {
		epoch = (uint64(height) - 1) / runtime.Config.Committee.EpochLength
	}
	return runtime.Committee.Select(ctx, epoch, round, seed, validatorSet)
}

func ignoreMissingMetricsError(err error) bool {
	return errors.Is(err, store.ErrBlockIndexNotFound) ||
		errors.Is(err, store.ErrStateNotFound) ||
		errors.Is(err, store.ErrBlockNotFound)
}

package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
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

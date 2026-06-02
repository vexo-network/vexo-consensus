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

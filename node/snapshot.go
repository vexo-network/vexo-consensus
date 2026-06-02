package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type StateSnapshot struct {
	Height           types.Height
	AppHash          types.Hash
	LastBlockHash    types.Hash
	ValidatorSetHash types.Hash
	StateRoots       []store.StateRootRecord
}

func (node *Node) StateSnapshot(ctx context.Context) (StateSnapshot, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return StateSnapshot{}, err
	}
	state, err := runtime.LatestState(ctx)
	if err != nil {
		return StateSnapshot{}, err
	}
	snapshot := StateSnapshot{
		Height:           state.Height,
		AppHash:          state.AppHash,
		LastBlockHash:    state.LastBlockHash,
		ValidatorSetHash: state.ValidatorSetHash,
		StateRoots:       make([]store.StateRootRecord, 0),
	}
	for _, module := range runtime.AppModules() {
		root, err := runtime.StateRoot(ctx, state.Height, module.Name())
		if err != nil {
			return StateSnapshot{}, err
		}
		snapshot.StateRoots = append(snapshot.StateRoots, root)
	}
	return snapshot, nil
}

package runtime

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInvalidReplayRange    = errors.New("invalid replay range")
	ErrReplayAppHashMismatch = errors.New("replayed app hash does not match stored app hash")
)

type ReplayResult struct {
	FromHeight types.Height
	ToHeight   types.Height
	LastHash   types.Hash
	Blocks     uint64
}

func (runtime *Runtime) Replay(ctx context.Context, from types.Height, to types.Height) (ReplayResult, error) {
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
		response, err := runtime.Executor.Execute(ctx, runtime.App, record.Block)
		if err != nil {
			return ReplayResult{}, err
		}
		if response.AppHash != record.AppHash {
			return ReplayResult{}, ErrReplayAppHashMismatch
		}
		if appRuntime, ok := runtime.App.(*app.Runtime); ok {
			appRuntime.Restore(height, response.AppHash)
		}
		result.LastHash = response.AppHash
		result.Blocks++
	}
	return result, nil
}

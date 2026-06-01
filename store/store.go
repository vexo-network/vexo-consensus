package store

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type BlockRecord struct {
	Block      types.Block
	Hash       types.Hash
	AppHash    types.Hash
	StateRoots []StateRootRecord
}

type StateRecord struct {
	Height           types.Height
	AppHash          types.Hash
	LastBlockHash    types.Hash
	ValidatorSetHash types.Hash
}

type StateRootRecord struct {
	Height    types.Height
	Namespace string
	Root      types.Hash
}

type BlockIndex struct {
	EarliestHeight types.Height
	LatestHeight   types.Height
	TotalBlocks    uint64
}

type PruneResult struct {
	RetainFromHeight types.Height
	PrunedBlocks     uint64
	PrunedStateRoots uint64
}

type Store interface {
	KVStore
	SaveBlock(ctx context.Context, record BlockRecord) error
	BlockByHeight(ctx context.Context, height types.Height) (BlockRecord, error)
	BlockByHash(ctx context.Context, hash types.Hash) (BlockRecord, error)
	BlockIndex(ctx context.Context) (BlockIndex, error)
	PruneBelow(ctx context.Context, retainFrom types.Height) (PruneResult, error)
	SaveState(ctx context.Context, state StateRecord) error
	LatestState(ctx context.Context) (StateRecord, error)
	SaveStateRoot(ctx context.Context, record StateRootRecord) error
	StateRoot(ctx context.Context, height types.Height, namespace string) (StateRootRecord, error)
	Close() error
}

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
	Root(ctx context.Context, namespace string) (types.Hash, error)
}

package mempool

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type Batch struct {
	ID       types.Hash
	Author   types.ValidatorID
	Txs      []types.Tx
	Parents  []types.Hash
	Metadata []byte
}

type Mempool interface {
	CheckTx(ctx context.Context, tx types.Tx) error
	AddTx(ctx context.Context, tx types.Tx) error
	// BuildBatch selects pending transactions for a block, skipping those whose hash is in
	// proposed: the transactions of the proposed blocks that are not committed yet.
	BuildBatch(ctx context.Context, maxBytes int64, proposed map[types.Hash]struct{}) (Batch, error)
	MarkCommitted(ctx context.Context, txs []types.Tx) error
	PendingTxs(ctx context.Context) ([]types.Tx, error)
}

type DAGMempool interface {
	Mempool
	AddBatch(ctx context.Context, batch Batch) error
	GetBatch(ctx context.Context, id types.Hash) (Batch, error)
	ReadyBatches(ctx context.Context) ([]Batch, error)
}

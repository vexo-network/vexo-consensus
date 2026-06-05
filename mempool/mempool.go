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
	BuildBatch(ctx context.Context, maxBytes int64) (Batch, error)
	MarkCommitted(ctx context.Context, txs []types.Tx) error
	PendingTxs(ctx context.Context) ([]types.Tx, error)
}

type DAGMempool interface {
	Mempool
	AddBatch(ctx context.Context, batch Batch) error
	GetBatch(ctx context.Context, id types.Hash) (Batch, error)
	ReadyBatches(ctx context.Context) ([]Batch, error)
}

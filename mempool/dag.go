package mempool

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrBatchNotFound      = errors.New("batch not found")
	ErrDuplicateBatch     = errors.New("duplicate batch")
	ErrUnknownParentBatch = errors.New("unknown parent batch")
)

type DAG struct {
	base     *FIFO
	batches  map[types.Hash]Batch
	children map[types.Hash]map[types.Hash]bool
	tips     map[types.Hash]bool
	wal      *WAL
}

func NewDAG(base *FIFO) *DAG {
	if base == nil {
		base = NewFIFO(FIFOConfig{})
	}
	return &DAG{
		base:     base,
		batches:  make(map[types.Hash]Batch),
		children: make(map[types.Hash]map[types.Hash]bool),
		tips:     make(map[types.Hash]bool),
	}
}

func (dag *DAG) CheckTx(ctx context.Context, tx types.Tx) error {
	return dag.base.CheckTx(ctx, tx)
}

func (dag *DAG) AddTx(ctx context.Context, tx types.Tx) error {
	if err := dag.base.CheckTx(ctx, tx); err != nil {
		return err
	}
	if dag.wal != nil {
		if err := dag.wal.AppendAddTx(ctx, tx); err != nil {
			return err
		}
	}
	return dag.base.addTxUnchecked(tx)
}

func (dag *DAG) BuildBatch(ctx context.Context, maxBytes int64) (Batch, error) {
	batch, err := dag.base.BuildBatch(ctx, maxBytes)
	if err != nil {
		return Batch{}, err
	}

	batch.Parents = dag.currentTips()
	batch.ID = HashBatch(batch)
	return batch, nil
}

func (dag *DAG) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	return dag.base.PendingTxs(ctx)
}

func (dag *DAG) MarkCommitted(ctx context.Context, txs []types.Tx) error {
	if dag.wal != nil {
		if err := dag.wal.AppendMarkCommitted(ctx, txs); err != nil {
			return err
		}
	}
	return dag.base.MarkCommitted(ctx, txs)
}

func (dag *DAG) CompactWAL(ctx context.Context) error {
	if dag == nil || dag.wal == nil {
		return nil
	}
	return dag.wal.Compact(ctx, dag)
}

func (dag *DAG) AddBatch(ctx context.Context, batch Batch) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if batch.ID == (types.Hash{}) {
		batch.ID = HashBatch(batch)
	}
	if _, found := dag.batches[batch.ID]; found {
		return ErrDuplicateBatch
	}
	for _, parent := range batch.Parents {
		if _, found := dag.batches[parent]; !found {
			return ErrUnknownParentBatch
		}
	}

	if dag.wal != nil {
		if err := dag.wal.AppendAddBatch(ctx, batch); err != nil {
			return err
		}
	}
	dag.addBatchUnchecked(batch)
	return nil
}

func (dag *DAG) AttachWAL(wal *WAL) {
	dag.wal = wal
}

func (dag *DAG) addBatchUnchecked(batch Batch) {
	copied := cloneBatch(batch)
	dag.batches[copied.ID] = copied
	dag.tips[copied.ID] = true

	for _, parent := range copied.Parents {
		delete(dag.tips, parent)
		if _, found := dag.children[parent]; !found {
			dag.children[parent] = make(map[types.Hash]bool)
		}
		dag.children[parent][copied.ID] = true
	}
}

func (dag *DAG) GetBatch(ctx context.Context, id types.Hash) (Batch, error) {
	select {
	case <-ctx.Done():
		return Batch{}, ctx.Err()
	default:
	}

	batch, found := dag.batches[id]
	if !found {
		return Batch{}, ErrBatchNotFound
	}
	return cloneBatch(batch), nil
}

func (dag *DAG) ReadyBatches(ctx context.Context) ([]Batch, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ready := make([]Batch, 0, len(dag.tips))
	for tip := range dag.tips {
		batch := dag.batches[tip]
		ready = append(ready, cloneBatch(batch))
	}
	return ready, nil
}

func (dag *DAG) Len() int {
	return dag.base.Len()
}

func (dag *DAG) BatchCount() int {
	return len(dag.batches)
}

func (dag *DAG) currentTips() []types.Hash {
	tips := make([]types.Hash, 0, len(dag.tips))
	for tip := range dag.tips {
		tips = append(tips, tip)
	}
	return tips
}

func cloneBatch(batch Batch) Batch {
	copied := Batch{
		ID:       batch.ID,
		Author:   batch.Author,
		Parents:  append([]types.Hash(nil), batch.Parents...),
		Metadata: append([]byte(nil), batch.Metadata...),
		Txs:      make([]types.Tx, 0, len(batch.Txs)),
	}
	for _, tx := range batch.Txs {
		copied.Txs = append(copied.Txs, append(types.Tx(nil), tx...))
	}
	return copied
}

package mempool

import (
	"context"
	"errors"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrBatchNotFound      = errors.New("batch not found")
	ErrDuplicateBatch     = errors.New("duplicate batch")
	ErrUnknownParentBatch = errors.New("unknown parent batch")
)

type DAG struct {
	mu       sync.Mutex
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
	dag.mu.Lock()
	defer dag.mu.Unlock()
	return dag.base.addTxWithHook(ctx, tx, func() error {
		if dag.wal != nil {
			return dag.wal.AppendAddTx(ctx, tx)
		}
		return nil
	})
}

func (dag *DAG) BuildBatch(ctx context.Context, maxBytes int64) (Batch, error) {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	batch, err := dag.base.BuildBatch(ctx, maxBytes)
	if err != nil {
		return Batch{}, err
	}

	batch.Parents = dag.currentTipsLocked()
	batch.ID = HashBatch(batch)
	return batch, nil
}

func (dag *DAG) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	return dag.base.PendingTxs(ctx)
}

func (dag *DAG) MarkCommitted(ctx context.Context, txs []types.Tx) error {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	return dag.base.markCommittedWithHook(ctx, txs, func() error {
		if dag.wal != nil {
			return dag.wal.AppendMarkCommitted(ctx, txs)
		}
		return nil
	})
}

func (dag *DAG) RetainTxs(ctx context.Context, keep func(types.Tx) bool) (int, error) {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	return dag.base.RetainTxs(ctx, keep)
}

func (dag *DAG) CompactWAL(ctx context.Context) error {
	if dag == nil || dag.wal == nil {
		return nil
	}
	dag.mu.Lock()
	defer dag.mu.Unlock()
	return dag.wal.Compact(ctx, dag)
}

func (dag *DAG) AddBatch(ctx context.Context, batch Batch) error {
	dag.mu.Lock()
	defer dag.mu.Unlock()
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
	dag.mu.Lock()
	defer dag.mu.Unlock()
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
	dag.mu.Lock()
	defer dag.mu.Unlock()
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
	dag.mu.Lock()
	defer dag.mu.Unlock()
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
	dag.mu.Lock()
	defer dag.mu.Unlock()
	return len(dag.batches)
}

func (dag *DAG) currentTips() []types.Hash {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	return dag.currentTipsLocked()
}

func (dag *DAG) currentTipsLocked() []types.Hash {
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

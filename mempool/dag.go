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

// DAG keeps the batches of a pool as a graph. With a WAL attached, its lock orders every change of
// the pool against the record of the change: both happen while the lock is held, and only the wait
// for the sync is left to the caller afterwards.
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

// AddTx admits tx and, when the DAG has a WAL, returns once the admission is written to the log;
// the log is synced periodically (see WAL).
//
// The record is queued and the pool updated under the DAG lock, so the log lists the changes in
// the order the pool made them; the wait for the write happens after the lock is released, so
// concurrent admissions share one write. If ctx ends during that wait, AddTx returns ctx.Err()
// although the transaction is in the pool: its record is still written.
func (dag *DAG) AddTx(ctx context.Context, tx types.Tx) error {
	logged, err := dag.QueueTx(ctx, tx)
	if err != nil {
		return err
	}
	return logged.Wait(ctx)
}

// Logged is a change of the pool whose WAL record is queued. Wait returns once the record is
// written to the log, which a crash of the process keeps, and WaitDurable once it is synced, which
// a crash of the machine keeps; both return at once for a DAG without a WAL.
type Logged struct {
	ticket walTicket
}

// Wait returns once the record is written, or ctx.Err() if ctx ends first; the record is still
// written then.
func (logged Logged) Wait(ctx context.Context) error {
	return logged.ticket.wait(ctx)
}

// WaitDurable returns once the record is written and synced, or ctx.Err() if ctx ends first.
func (logged Logged) WaitDurable(ctx context.Context) error {
	return logged.ticket.waitSynced(ctx)
}

// QueueTx admits tx like AddTx but returns as soon as its record is queued, so a caller that
// orders admissions under a lock of its own can release it before the write and let concurrent
// admissions share one. The transaction is in the pool when QueueTx returns.
func (dag *DAG) QueueTx(ctx context.Context, tx types.Tx) (Logged, error) {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	if err := dag.repairWAL(ctx); err != nil {
		return Logged{}, err
	}
	var ticket walTicket
	err := dag.base.addTxWithHook(ctx, tx, func() error {
		var err error
		ticket, err = dag.queue(WALEvent{Op: walOpAddTx, Tx: tx})
		return err
	})
	return Logged{ticket: ticket}, err
}

func (dag *DAG) BuildBatch(ctx context.Context, maxBytes int64, proposed map[types.Hash]struct{}) (Batch, error) {
	// The base pool replays application checks while it builds; that must not
	// run under the DAG lock or every AddTx would wait behind it.
	batch, err := dag.base.BuildBatch(ctx, maxBytes, proposed)
	if err != nil {
		return Batch{}, err
	}

	dag.mu.Lock()
	defer dag.mu.Unlock()
	batch.Parents = dag.currentTipsLocked()
	batch.ID = HashBatch(batch)
	return batch, nil
}

func (dag *DAG) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	return dag.base.PendingTxs(ctx)
}

func (dag *DAG) SnapshotTxs(ctx context.Context) ([]types.Tx, error) {
	return dag.base.SnapshotTxs(ctx)
}

// SenderTxs returns the pending transactions of sender (FIFO.SenderTxs).
func (dag *DAG) SenderTxs(ctx context.Context, sender string) ([]types.Tx, error) {
	return dag.base.SenderTxs(ctx, sender)
}

// MarkCommitted removes the committed transactions and, when the DAG has a WAL, returns once the
// removal is durable, synced and not only written: a removal a crash loses would replay committed
// transactions as pending. See AddTx for how the log is ordered.
func (dag *DAG) MarkCommitted(ctx context.Context, txs []types.Tx) error {
	ticket, err := dag.markCommittedQueued(ctx, txs)
	if err != nil {
		return err
	}
	return ticket.waitSynced(ctx)
}

// QueueMarkCommitted removes the committed transactions like MarkCommitted but returns as soon as
// the removal's record is queued; the returned Logged reports when it is written and durable. The
// transactions are out of the pool when QueueMarkCommitted returns.
func (dag *DAG) QueueMarkCommitted(ctx context.Context, txs []types.Tx) (Logged, error) {
	ticket, err := dag.markCommittedQueued(ctx, txs)
	return Logged{ticket: ticket}, err
}

func (dag *DAG) markCommittedQueued(ctx context.Context, txs []types.Tx) (walTicket, error) {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	if len(txs) > 0 {
		if err := dag.repairWAL(ctx); err != nil {
			return walTicket{}, err
		}
	}
	var ticket walTicket
	err := dag.base.markCommittedWithHook(ctx, txs, func() error {
		var err error
		ticket, err = dag.queue(WALEvent{Op: walOpMarkCommitted, Txs: txs})
		return err
	})
	return ticket, err
}

func (dag *DAG) RetainTxs(ctx context.Context, keep func(types.Tx) bool) (int, error) {
	return dag.base.RetainTxs(ctx, keep)
}

func (dag *DAG) CompactWAL(ctx context.Context) error {
	if dag == nil {
		return nil
	}
	dag.mu.Lock()
	defer dag.mu.Unlock()
	if dag.wal == nil {
		return nil
	}
	return dag.wal.Compact(ctx, dag)
}

// CompactWALWhenGrown compacts the WAL once it has grown enough since it was last compacted for a
// rewrite to pay (see walCompactMinBytes), so a commit does not rewrite the whole pool and the log
// stays proportional to the pool's content.
func (dag *DAG) CompactWALWhenGrown(ctx context.Context) error {
	if dag == nil {
		return nil
	}
	dag.mu.Lock()
	defer dag.mu.Unlock()
	if dag.wal == nil || !dag.wal.grown() {
		return nil
	}
	return dag.wal.Compact(ctx, dag)
}

// queue queues event in the WAL and returns the ticket to wait on, or the zero ticket when the
// DAG has no WAL. The caller holds the DAG lock.
func (dag *DAG) queue(event WALEvent) (walTicket, error) {
	if dag.wal == nil {
		return walTicket{}, nil
	}
	return dag.wal.queue(event)
}

// repairWAL rewrites a WAL that an I/O error stopped from the DAG, which still holds every change
// the log lost, so a transient fault ends with the fault instead of at the next restart. The
// caller holds the DAG lock.
func (dag *DAG) repairWAL(ctx context.Context) error {
	if dag.wal == nil || dag.wal.failure() == nil {
		return nil
	}
	return dag.wal.Compact(ctx, dag)
}

// AddBatch adds batch and, when the DAG has a WAL, returns once the addition is durable; see
// AddTx for how the log is ordered and awaited.
func (dag *DAG) AddBatch(ctx context.Context, batch Batch) error {
	ticket, err := dag.addBatchQueued(ctx, batch)
	if err != nil {
		return err
	}
	return ticket.wait(ctx)
}

func (dag *DAG) addBatchQueued(ctx context.Context, batch Batch) (walTicket, error) {
	dag.mu.Lock()
	defer dag.mu.Unlock()
	select {
	case <-ctx.Done():
		return walTicket{}, ctx.Err()
	default:
	}

	if batch.ID == (types.Hash{}) {
		batch.ID = HashBatch(batch)
	}
	if _, found := dag.batches[batch.ID]; found {
		return walTicket{}, ErrDuplicateBatch
	}
	for _, parent := range batch.Parents {
		if _, found := dag.batches[parent]; !found {
			return walTicket{}, ErrUnknownParentBatch
		}
	}

	if err := dag.repairWAL(ctx); err != nil {
		return walTicket{}, err
	}
	ticket, err := dag.queue(WALEvent{Op: walOpAddBatch, Batch: batch})
	if err != nil {
		return walTicket{}, err
	}
	dag.addBatchUnchecked(batch)
	return ticket, nil
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

// Has reports whether the pool holds the transaction of hash (FIFO.Has).
func (dag *DAG) Has(hash types.Hash) bool {
	return dag.base.Has(hash)
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

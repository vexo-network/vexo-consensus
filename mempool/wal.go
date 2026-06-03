package mempool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidWALPath = errors.New("invalid mempool wal path")

const (
	walOpAddTx         = "add_tx"
	walOpAddBatch      = "add_batch"
	walOpMarkCommitted = "mark_committed"
)

type WALEvent struct {
	Op    string     `json:"op"`
	Tx    types.Tx   `json:"tx,omitempty"`
	Txs   []types.Tx `json:"txs,omitempty"`
	Batch Batch      `json:"batch,omitempty"`
}

type WAL struct {
	mu   sync.Mutex
	file *os.File
}

func OpenWAL(path string) (*WAL, error) {
	if path == "" {
		return nil, ErrInvalidWALPath
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &WAL{file: file}, nil
}

func OpenDurableDAG(ctx context.Context, path string, base *FIFO) (*DAG, error) {
	wal, err := OpenWAL(path)
	if err != nil {
		return nil, err
	}
	dag := NewDAG(base)
	events, err := wal.Replay()
	if err != nil {
		_ = wal.Close()
		return nil, err
	}
	for _, event := range events {
		select {
		case <-ctx.Done():
			_ = wal.Close()
			return nil, ctx.Err()
		default:
		}
		switch event.Op {
		case walOpAddTx:
			if err := dag.base.AddTx(ctx, event.Tx); err != nil && !errors.Is(err, ErrDuplicateTx) {
				_ = wal.Close()
				return nil, err
			}
		case walOpAddBatch:
			if err := dag.AddBatch(ctx, event.Batch); err != nil && !errors.Is(err, ErrDuplicateBatch) {
				_ = wal.Close()
				return nil, err
			}
		case walOpMarkCommitted:
			if err := dag.MarkCommitted(ctx, event.Txs); err != nil {
				_ = wal.Close()
				return nil, err
			}
		}
	}
	dag.AttachWAL(wal)
	return dag, nil
}

func (wal *WAL) AppendAddTx(ctx context.Context, tx types.Tx) error {
	return wal.append(ctx, WALEvent{Op: walOpAddTx, Tx: append(types.Tx(nil), tx...)})
}

func (wal *WAL) AppendAddBatch(ctx context.Context, batch Batch) error {
	return wal.append(ctx, WALEvent{Op: walOpAddBatch, Batch: cloneBatch(batch)})
}

func (wal *WAL) AppendMarkCommitted(ctx context.Context, txs []types.Tx) error {
	copied := make([]types.Tx, 0, len(txs))
	for _, tx := range txs {
		copied = append(copied, append(types.Tx(nil), tx...))
	}
	return wal.append(ctx, WALEvent{Op: walOpMarkCommitted, Txs: copied})
}

func (wal *WAL) Replay() ([]WALEvent, error) {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	if _, err := wal.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(wal.file)
	events := make([]WALEvent, 0)
	for {
		var event WALEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		events = append(events, event)
	}
	_, err := wal.file.Seek(0, io.SeekEnd)
	return events, err
}

func (wal *WAL) Compact(ctx context.Context, dag *DAG) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	if err := wal.file.Truncate(0); err != nil {
		return err
	}
	if _, err := wal.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	encoder := json.NewEncoder(wal.file)
	for _, tx := range dag.base.orderedTxs() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := encoder.Encode(WALEvent{Op: walOpAddTx, Tx: tx}); err != nil {
			return err
		}
	}
	emitted := make(map[types.Hash]bool, len(dag.batches))
	var emitBatch func(types.Hash) error
	emitBatch = func(id types.Hash) error {
		if emitted[id] {
			return nil
		}
		batch, found := dag.batches[id]
		if !found {
			return nil
		}
		for _, parent := range batch.Parents {
			if err := emitBatch(parent); err != nil {
				return err
			}
		}
		if err := encoder.Encode(WALEvent{Op: walOpAddBatch, Batch: cloneBatch(batch)}); err != nil {
			return err
		}
		emitted[id] = true
		return nil
	}
	for id := range dag.batches {
		if err := emitBatch(id); err != nil {
			return err
		}
	}
	return wal.file.Sync()
}

func (wal *WAL) Close() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	return wal.file.Close()
}

func (wal *WAL) append(ctx context.Context, event WALEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := json.NewEncoder(wal.file).Encode(event); err != nil {
		return err
	}
	return wal.file.Sync()
}

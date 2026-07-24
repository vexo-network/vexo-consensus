package mempool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	path string
	file *os.File
}

func OpenWAL(path string) (*WAL, error) {
	if path == "" {
		return nil, ErrInvalidWALPath
	}
	file, err := openWALFile(path)
	if err != nil {
		return nil, err
	}
	return &WAL{path: path, file: file}, nil
}

func openWALFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
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
			if dag.base.config.ReplayCheckTx != nil {
				if err := dag.base.config.ReplayCheckTx(ctx, event.Tx); err != nil {
					continue
				}
			}
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
	reader := bufio.NewReader(wal.file)
	events := make([]WALEvent, 0)
	var offset int64
	for {
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) && len(line) == 0 {
			break
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			offset += int64(len(line))
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		var event WALEvent
		if decodeErr := json.Unmarshal(trimmed, &event); decodeErr != nil {
			if errors.Is(err, io.EOF) {
				if truncateErr := wal.file.Truncate(offset); truncateErr != nil {
					return nil, truncateErr
				}
				break
			}
			return nil, decodeErr
		}
		events = append(events, event)
		offset += int64(len(line))
		if errors.Is(err, io.EOF) {
			break
		}
	}
	_, err := wal.file.Seek(0, io.SeekEnd)
	return events, err
}

func (wal *WAL) Compact(ctx context.Context, dag *DAG) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.path == "" {
		return ErrInvalidWALPath
	}
	tempPath := wal.path + ".compact.tmp"
	if err := os.MkdirAll(filepath.Dir(wal.path), 0o700); err != nil {
		return err
	}
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	encoder := json.NewEncoder(tempFile)
	for _, tx := range dag.base.orderedTxs() {
		select {
		case <-ctx.Done():
			_ = tempFile.Close()
			return ctx.Err()
		default:
		}
		if err := encoder.Encode(WALEvent{Op: walOpAddTx, Tx: tx}); err != nil {
			_ = tempFile.Close()
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
				_ = tempFile.Close()
				return err
			}
		}
		if err := encoder.Encode(WALEvent{Op: walOpAddBatch, Batch: cloneBatch(batch)}); err != nil {
			_ = tempFile.Close()
			return err
		}
		emitted[id] = true
		return nil
	}
	for id := range dag.batches {
		if err := emitBatch(id); err != nil {
			_ = tempFile.Close()
			return err
		}
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := wal.file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, wal.path); err != nil {
		reopened, reopenErr := openWALFile(wal.path)
		if reopenErr == nil {
			wal.file = reopened
		}
		return errors.Join(err, reopenErr)
	}
	cleanupTemp = false
	syncDirectory(filepath.Dir(wal.path))
	reopened, err := openWALFile(wal.path)
	if err != nil {
		return err
	}
	wal.file = reopened
	return nil
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

func syncDirectory(path string) {
	dir, err := os.Open(path)
	if err != nil {
		return
	}
	_ = dir.Sync()
	_ = dir.Close()
}

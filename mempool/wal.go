package mempool

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidWALPath = errors.New("invalid mempool wal path")

// ErrWALClosed is returned for an operation on a WAL that was closed.
var ErrWALClosed = errors.New("mempool wal is closed")

// ErrWALCorrupt reports a log that is damaged before its end: a valid record follows a damaged
// one, so replaying the log would silently skip transactions.
var ErrWALCorrupt = errors.New("mempool wal is corrupt")

// ErrWALFailed wraps the I/O error that stopped the WAL. Nothing is appended after it, as the
// content of the file is unknown, until Compact rewrites the log.
var ErrWALFailed = errors.New("mempool wal failed")

// walOp is the kind of change a record logs.
type walOp byte

const (
	walOpAddTx         walOp = 1
	walOpAddBatch      walOp = 2
	walOpMarkCommitted walOp = 3
)

// walChecksumBytes is the size of the CRC-32C that ends every record.
const walChecksumBytes = 4

// walSnapshotBufferBytes is how much of a compacted log is buffered before it is written. A
// compaction follows every block that commits transactions, so the buffer stays small.
const walSnapshotBufferBytes = 64 << 10

// walSpareBufferBytes bounds the write buffer kept for the next batch, so one huge batch does not
// pin its memory.
const walSpareBufferBytes = 1 << 20

var walChecksumTable = crc32.MakeTable(crc32.Castagnoli)

// WALEvent is one decoded record of the log.
type WALEvent struct {
	Op    walOp
	Tx    types.Tx
	Txs   []types.Tx
	Batch Batch
}

// WAL is the write-ahead log of the mempool. A change of the pool is appended to it before the
// change is acknowledged, so a restart replays the pending transactions.
//
// The log is a sequence of records, each framed as
//
//	uvarint(n) | op | payload | crc32c(op | payload)
//
// where n is the length of op | payload, op is one byte and the checksum is the Castagnoli CRC in
// four big-endian bytes. The payload of add_tx is the transaction itself. The payload of
// mark_committed is uvarint(count) followed by count length-prefixed transactions. The payload of
// add_batch is the batch ID, the length-prefixed author, a count and that many length-prefixed
// transactions, a count and that many 32-byte parent IDs, and the length-prefixed metadata. Every
// length and count is a canonical uvarint.
//
// Appends are group committed. A caller queues its encoded record; the next write carries every
// record queued so far and one sync makes them all durable, after which every caller of the group
// is released. An append returns only once its record is durable, and records reach the file in
// the order they were queued, so concurrent callers share the cost of a sync instead of paying
// one each.
//
// After an I/O error the content of the file is unknown, so the WAL stops writing: every append
// returns ErrWALFailed until Compact rewrites the log from the pool, which still holds every
// change the log lost.
type WAL struct {
	path string
	// flush is the write token, a one-slot channel. Its holder owns file and writes the queued
	// records; Compact, Replay and Close take it as well, which serializes them with the writes.
	flush chan struct{}
	// file is the open log, written in append mode. Only the holder of the token touches it.
	file *os.File
	// syncFile makes the written records durable. It is a field so tests can count and fail it.
	syncFile func(*os.File) error

	mu sync.Mutex
	// pending is the batch the next write carries; nil while no record is queued.
	pending *walBatch
	// spare is the emptied buffer of the last written batch, kept for the next one.
	spare []byte
	// failed is the error that stopped the WAL; nil while it is healthy.
	failed error
	// closed is set by Close. Nothing is queued afterwards.
	closed bool
}

// walBatch is the records that one write and one sync make durable together.
type walBatch struct {
	// buf holds the encoded records in the order they were queued.
	buf []byte
	// done is closed once the batch is durable or has failed.
	done chan struct{}
	// err is the outcome of the batch. It is set before done closes.
	err error
	// detached reports that a caller gave up waiting, so a goroutine of its own writes the batch.
	detached bool
}

// walTicket is a record queued in a WAL. The zero ticket stands for a record that needs no
// write, as the DAG has no WAL, and waits for nothing.
type walTicket struct {
	wal   *WAL
	batch *walBatch
}

// OpenWAL opens the log at path, creating it if needed. A torn or damaged tail left by a crash
// is truncated, so the first append starts on a record boundary; a log damaged before its end is
// refused with ErrWALCorrupt.
func OpenWAL(path string) (*WAL, error) {
	if path == "" {
		return nil, ErrInvalidWALPath
	}
	file, err := openWALFile(path)
	if err != nil {
		return nil, err
	}
	wal := &WAL{
		path:     path,
		flush:    make(chan struct{}, 1),
		file:     file,
		syncFile: (*os.File).Sync,
	}
	if _, err := wal.scanFile(); err != nil {
		_ = file.Close()
		return nil, err
	}
	// The records synced later live in a file whose directory entry must be durable as well.
	syncDirectory(filepath.Dir(path))
	return wal, nil
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
			// A future-nonce transaction is not executable yet but is still the
			// sender's pending work: only a transaction that can never execute
			// is dropped on replay.
			if replay := dag.base.config.ReplayCheckTx; replay != nil {
				if err := replay(ctx, event.Tx); err != nil && !errors.Is(err, ErrTxNotReady) {
					if ctxErr := ctx.Err(); ctxErr != nil {
						_ = wal.Close()
						return nil, ctxErr
					}
					continue
				}
			}
			// A logged transaction the current pool policy refuses (limits or
			// minimum fee changed since it was logged, duplicate) is dropped;
			// it must not prevent the node from starting.
			if err := dag.base.AddTx(ctx, event.Tx); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					_ = wal.Close()
					return nil, ctxErr
				}
				continue
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

// AppendAddTx logs the admission of tx and returns once the record is durable. If ctx ends first
// the call returns ctx.Err(), but the record is still written: the caller only stopped waiting.
// Replay applies such a record like any other, which is harmless because the pool re-checks
// every logged transaction against its current policy.
func (wal *WAL) AppendAddTx(ctx context.Context, tx types.Tx) error {
	return wal.append(ctx, WALEvent{Op: walOpAddTx, Tx: tx})
}

// AppendAddBatch logs the addition of batch; see AppendAddTx for the durability contract.
func (wal *WAL) AppendAddBatch(ctx context.Context, batch Batch) error {
	return wal.append(ctx, WALEvent{Op: walOpAddBatch, Batch: batch})
}

// AppendMarkCommitted logs that txs were committed; see AppendAddTx for the durability contract.
func (wal *WAL) AppendMarkCommitted(ctx context.Context, txs []types.Tx) error {
	return wal.append(ctx, WALEvent{Op: walOpMarkCommitted, Txs: txs})
}

func (wal *WAL) append(ctx context.Context, event WALEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ticket, err := wal.queue(event)
	if err != nil {
		return err
	}
	return ticket.wait(ctx)
}

// queue encodes event into the batch the next write carries. It performs no I/O and never waits
// for the writer, so a caller may queue under the locks that fix the order of the pool's changes
// and wait for the ticket after releasing them.
func (wal *WAL) queue(event WALEvent) (walTicket, error) {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if wal.closed {
		return walTicket{}, ErrWALClosed
	}
	if wal.failed != nil {
		return walTicket{}, wal.failed
	}
	batch := wal.pending
	buf := wal.spare
	if batch != nil {
		buf = batch.buf
	}
	buf, err := appendWALEvent(buf, event)
	if err != nil {
		return walTicket{}, err
	}
	if batch == nil {
		batch = &walBatch{done: make(chan struct{})}
		wal.pending = batch
		wal.spare = nil
	}
	batch.buf = buf
	return walTicket{wal: wal, batch: batch}, nil
}

// wait blocks until the record is durable or its write failed. A caller that finds no write in
// progress performs the write for every record queued so far; the others sleep until the writer
// releases them. If ctx ends first the call returns ctx.Err() and a goroutine writes the record
// in the caller's place.
func (ticket walTicket) wait(ctx context.Context) error {
	if ticket.wal == nil {
		return nil
	}
	return ticket.wal.await(ctx, ticket.batch)
}

func (wal *WAL) await(ctx context.Context, batch *walBatch) error {
	select {
	case <-batch.done:
		return batch.err
	default:
	}
	select {
	case <-batch.done:
		return batch.err
	case wal.flush <- struct{}{}:
		wal.lead(batch)
		return batch.err
	case <-ctx.Done():
		// A batch that was written while the caller was giving up keeps its outcome.
		select {
		case <-batch.done:
			return batch.err
		default:
		}
		wal.detach(batch)
		return ctx.Err()
	}
}

// lead writes the queued records on behalf of every caller waiting for them. The caller holds
// the token, which lead releases. A batch that an earlier holder already wrote is left alone.
func (wal *WAL) lead(batch *walBatch) {
	defer func() { <-wal.flush }()
	select {
	case <-batch.done:
	default:
		wal.flushPending()
	}
}

// detach hands the write of a batch whose caller stopped waiting to a goroutine, so the record
// does not depend on another caller turning up to write it. One goroutine serves all the callers
// of a batch; it ends as soon as it holds the token, whether it still has a write to do or not.
func (wal *WAL) detach(batch *walBatch) {
	wal.mu.Lock()
	spawn := !batch.detached
	batch.detached = true
	wal.mu.Unlock()
	if spawn {
		go func() {
			wal.flush <- struct{}{}
			wal.lead(batch)
		}()
	}
}

// flushPending writes the queued records with one write and one sync, then releases their
// callers. The caller holds the token.
func (wal *WAL) flushPending() {
	wal.mu.Lock()
	batch := wal.pending
	wal.pending = nil
	failure := wal.failed
	wal.mu.Unlock()
	if batch == nil {
		return
	}
	if failure == nil {
		failure = wal.writeBatch(batch.buf)
	}
	batch.err = failure
	wal.mu.Lock()
	if cap(batch.buf) <= walSpareBufferBytes {
		wal.spare = batch.buf[:0]
	}
	batch.buf = nil
	wal.mu.Unlock()
	close(batch.done)
}

// writeBatch appends records to the file and syncs it. Whatever goes wrong stops the WAL: after a
// failed write or sync the kernel may have dropped part of the data, so appending behind it could
// leave a hole in the middle of the log.
func (wal *WAL) writeBatch(records []byte) error {
	if _, err := wal.file.Write(records); err != nil {
		return wal.fail(err)
	}
	if err := wal.syncFile(wal.file); err != nil {
		return wal.fail(err)
	}
	return nil
}

// fail stops the WAL after err and returns the error that every later append returns.
func (wal *WAL) fail(err error) error {
	err = fmt.Errorf("%w: %w", ErrWALFailed, err)
	wal.mu.Lock()
	wal.failed = err
	wal.mu.Unlock()
	return err
}

// failure returns the error that stopped the WAL, or nil while it is healthy.
func (wal *WAL) failure() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	return wal.failed
}

func (wal *WAL) isClosed() bool {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	return wal.closed
}

// Replay returns the durable records in the order they were written. A torn or damaged tail, as a
// crash in the middle of a write leaves, is truncated and ignored; a damaged record that valid
// records follow is corruption and fails with ErrWALCorrupt.
func (wal *WAL) Replay() ([]WALEvent, error) {
	wal.flush <- struct{}{}
	defer func() { <-wal.flush }()
	if wal.isClosed() {
		return nil, ErrWALClosed
	}
	return wal.scanFile()
}

// scanFile reads the log, cuts away the damaged tail a crash can leave and returns the records
// before it. The caller owns the file: it holds the token, or has not shared the WAL yet.
func (wal *WAL) scanFile() ([]WALEvent, error) {
	info, err := wal.file.Stat()
	if err != nil {
		return nil, err
	}
	data := make([]byte, info.Size())
	if _, err := io.ReadFull(io.NewSectionReader(wal.file, 0, info.Size()), data); err != nil {
		return nil, err
	}
	scan, err := scanWAL(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", wal.path, err)
	}
	if scan.validBytes < len(data) {
		if err := wal.file.Truncate(int64(scan.validBytes)); err != nil {
			return nil, fmt.Errorf("truncate torn wal tail: %w", err)
		}
		if err := wal.file.Sync(); err != nil {
			return nil, err
		}
	}
	return scan.events, nil
}

// Compact rewrites the log as the live content of dag: its pending transactions in proposal
// order, then its batches, parents first. The caller holds the DAG lock, as DAG.CompactWAL does,
// so no change is queued while the snapshot is taken. Records queued earlier are written to the
// old log first, so their callers are released before the log is replaced. A successful Compact
// also repairs a WAL that an I/O error stopped.
func (wal *WAL) Compact(ctx context.Context, dag *DAG) error {
	select {
	case wal.flush <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-wal.flush }()
	if wal.isClosed() {
		return ErrWALClosed
	}
	wal.flushPending()

	tempPath := wal.path + ".compact.tmp"
	if err := os.MkdirAll(filepath.Dir(wal.path), 0o700); err != nil {
		return err
	}
	if err := writeWALSnapshot(ctx, tempPath, dag); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return wal.swapFile(tempPath)
}

// writeWALSnapshot writes the live content of dag to a new log at path and syncs it.
func writeWALSnapshot(ctx context.Context, path string, dag *DAG) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	writer := bufio.NewWriterSize(file, walSnapshotBufferBytes)
	var record []byte
	for _, tx := range dag.base.orderedTxs() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if record, err = appendWALEvent(record[:0], WALEvent{Op: walOpAddTx, Tx: tx}); err != nil {
			return err
		}
		if _, err := writer.Write(record); err != nil {
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
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		if record, err = appendWALEvent(record[:0], WALEvent{Op: walOpAddBatch, Batch: batch}); err != nil {
			return err
		}
		if _, err := writer.Write(record); err != nil {
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
	if err := writer.Flush(); err != nil {
		return err
	}
	return file.Sync()
}

// swapFile replaces the log by the complete, durable file at tempPath. The old handle is closed
// first, as not every platform renames over an open file. Whatever happens, the WAL ends up
// appending to the file at its path, or failed.
func (wal *WAL) swapFile(tempPath string) error {
	// The snapshot holds everything the old log did, so its handle needs no final sync.
	_ = wal.file.Close()
	renameErr := os.Rename(tempPath, wal.path)
	if renameErr == nil {
		syncDirectory(filepath.Dir(wal.path))
	}
	file, openErr := openWALFile(wal.path)
	if openErr != nil {
		return wal.fail(errors.Join(renameErr, openErr))
	}
	wal.file = file
	if renameErr != nil {
		_ = os.Remove(tempPath)
		return renameErr
	}
	wal.mu.Lock()
	wal.failed = nil
	wal.mu.Unlock()
	return nil
}

// Close writes the records already queued, so their callers are released with their records
// durable, and closes the file. Appends after Close fail with ErrWALClosed.
func (wal *WAL) Close() error {
	wal.mu.Lock()
	if wal.closed {
		wal.mu.Unlock()
		return ErrWALClosed
	}
	wal.closed = true
	wal.mu.Unlock()

	wal.flush <- struct{}{}
	defer func() { <-wal.flush }()
	wal.flushPending()
	return wal.file.Close()
}

func syncDirectory(path string) {
	dir, err := os.Open(path)
	if err != nil {
		return
	}
	_ = dir.Sync()
	_ = dir.Close()
}

// appendWALEvent appends the framed record of event to dst.
func appendWALEvent(dst []byte, event WALEvent) ([]byte, error) {
	switch event.Op {
	case walOpAddTx:
		return appendTxRecord(dst, event.Tx), nil
	case walOpMarkCommitted:
		return appendCommittedRecord(dst, event.Txs), nil
	case walOpAddBatch:
		return appendBatchRecord(dst, event.Batch), nil
	}
	return dst, fmt.Errorf("mempool wal: unknown op %d", event.Op)
}

func appendTxRecord(dst []byte, tx types.Tx) []byte {
	dst, start := beginWALRecord(dst, walOpAddTx, len(tx))
	dst = append(dst, tx...)
	return endWALRecord(dst, start)
}

func appendCommittedRecord(dst []byte, txs []types.Tx) []byte {
	size := uvarintLen(uint64(len(txs)))
	for _, tx := range txs {
		size += uvarintLen(uint64(len(tx))) + len(tx)
	}
	dst, start := beginWALRecord(dst, walOpMarkCommitted, size)
	dst = binary.AppendUvarint(dst, uint64(len(txs)))
	for _, tx := range txs {
		dst = appendWALBytes(dst, tx)
	}
	return endWALRecord(dst, start)
}

func appendBatchRecord(dst []byte, batch Batch) []byte {
	size := len(batch.ID)
	size += uvarintLen(uint64(len(batch.Author))) + len(batch.Author)
	size += uvarintLen(uint64(len(batch.Txs)))
	for _, tx := range batch.Txs {
		size += uvarintLen(uint64(len(tx))) + len(tx)
	}
	size += uvarintLen(uint64(len(batch.Parents))) + len(batch.Parents)*len(types.Hash{})
	size += uvarintLen(uint64(len(batch.Metadata))) + len(batch.Metadata)

	dst, start := beginWALRecord(dst, walOpAddBatch, size)
	dst = append(dst, batch.ID[:]...)
	dst = binary.AppendUvarint(dst, uint64(len(batch.Author)))
	dst = append(dst, batch.Author...)
	dst = binary.AppendUvarint(dst, uint64(len(batch.Txs)))
	for _, tx := range batch.Txs {
		dst = appendWALBytes(dst, tx)
	}
	dst = binary.AppendUvarint(dst, uint64(len(batch.Parents)))
	for _, parent := range batch.Parents {
		dst = append(dst, parent[:]...)
	}
	dst = appendWALBytes(dst, batch.Metadata)
	return endWALRecord(dst, start)
}

// beginWALRecord starts a record whose payload is payloadSize bytes and returns the offset of its
// body, the part endWALRecord checksums. The record is allocated in one piece.
func beginWALRecord(dst []byte, op walOp, payloadSize int) ([]byte, int) {
	bodySize := 1 + payloadSize
	dst = slices.Grow(dst, uvarintLen(uint64(bodySize))+bodySize+walChecksumBytes)
	dst = binary.AppendUvarint(dst, uint64(bodySize))
	start := len(dst)
	return append(dst, byte(op)), start
}

func endWALRecord(dst []byte, bodyStart int) []byte {
	return binary.BigEndian.AppendUint32(dst, crc32.Checksum(dst[bodyStart:], walChecksumTable))
}

func appendWALBytes(dst []byte, field []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(field)))
	return append(dst, field...)
}

// uvarintLen is the size of the canonical uvarint encoding of value.
func uvarintLen(value uint64) int {
	return (bits.Len64(value|1) + 6) / 7
}

// walScan is what a pass over a log found: the events of its valid prefix and where it ends.
type walScan struct {
	events     []WALEvent
	validBytes int
}

// scanWAL decodes the records of data up to the first one that does not read: a torn record, a
// record whose checksum or content is wrong, or bytes that are not a record. What follows decides
// whether that is the end of the log or damage in the middle of it: a crash leaves nothing valid
// behind the damaged record, so the rest is a tail for the caller to cut away, while a valid
// record behind it means the log lost data it must not skip, and the scan fails.
func scanWAL(data []byte) (walScan, error) {
	var scan walScan
	offset := 0
	for offset < len(data) {
		event, size, ok := readWALEvent(data[offset:])
		if !ok {
			break
		}
		scan.events = append(scan.events, event)
		offset += size
	}
	scan.validBytes = offset
	for probe := offset + 1; probe < len(data); probe++ {
		if _, _, ok := readWALEvent(data[probe:]); ok {
			return walScan{}, fmt.Errorf("%w: valid record at offset %d follows damaged record at offset %d", ErrWALCorrupt, probe, offset)
		}
	}
	return scan, nil
}

// readWALEvent decodes the record at the start of data and returns the bytes it occupies. ok is
// false when data does not start with an intact record.
func readWALEvent(data []byte) (event WALEvent, size int, ok bool) {
	op, payload, size, ok := readWALRecord(data)
	if !ok {
		return WALEvent{}, 0, false
	}
	event, ok = decodeWALEvent(op, payload)
	return event, size, ok
}

// readWALRecord checks the framing of the record at the start of data and returns its op, its
// payload and the bytes it occupies. ok is false when data does not start with an intact record.
func readWALRecord(data []byte) (op walOp, payload []byte, size int, ok bool) {
	length, prefix := binary.Uvarint(data)
	if prefix <= 0 || prefix != uvarintLen(length) {
		return 0, nil, 0, false
	}
	rest := data[prefix:]
	if length == 0 || length > uint64(len(rest)) || uint64(len(rest))-length < walChecksumBytes {
		return 0, nil, 0, false
	}
	body := rest[:length]
	op = walOp(body[0])
	if !op.valid() || crc32.Checksum(body, walChecksumTable) != binary.BigEndian.Uint32(rest[length:]) {
		return 0, nil, 0, false
	}
	return op, body[1:], prefix + int(length) + walChecksumBytes, true
}

func (op walOp) valid() bool {
	return op == walOpAddTx || op == walOpAddBatch || op == walOpMarkCommitted
}

// decodeWALEvent decodes the payload of a record. The event owns its bytes. ok is false when the
// payload is not exactly what appendWALEvent writes for op.
func decodeWALEvent(op walOp, payload []byte) (WALEvent, bool) {
	switch op {
	case walOpAddTx:
		return WALEvent{Op: op, Tx: append(types.Tx(nil), payload...)}, true
	case walOpMarkCommitted:
		decoder := walDecoder{buf: payload}
		txs := decoder.txs()
		return WALEvent{Op: op, Txs: txs}, decoder.done()
	case walOpAddBatch:
		decoder := walDecoder{buf: payload}
		var batch Batch
		batch.ID = decoder.hash()
		batch.Author = types.ValidatorID(decoder.bytes())
		batch.Txs = decoder.txs()
		if count := decoder.count(len(batch.ID)); count > 0 {
			batch.Parents = make([]types.Hash, 0, count)
			for range count {
				batch.Parents = append(batch.Parents, decoder.hash())
			}
		}
		batch.Metadata = decoder.bytes()
		return WALEvent{Op: op, Batch: batch}, decoder.done()
	}
	return WALEvent{}, false
}

// walDecoder reads the fields of a record payload. The first malformed field poisons it: later
// reads return zero values and done reports false.
type walDecoder struct {
	buf []byte
	bad bool
}

func (decoder *walDecoder) done() bool {
	return !decoder.bad && len(decoder.buf) == 0
}

// uvarint reads a canonical uvarint. A longer encoding of the same value is refused, so one log
// content has one byte representation.
func (decoder *walDecoder) uvarint() uint64 {
	value, size := binary.Uvarint(decoder.buf)
	if size <= 0 || size != uvarintLen(value) {
		decoder.bad = true
		return 0
	}
	decoder.buf = decoder.buf[size:]
	return value
}

// count reads the number of elements that follow, each at least minSize bytes. A count the rest of
// the payload cannot hold is refused, so a damaged count never drives an allocation.
func (decoder *walDecoder) count(minSize int) int {
	count := decoder.uvarint()
	if decoder.bad || count > uint64(len(decoder.buf)/minSize) {
		decoder.bad = true
		return 0
	}
	return int(count)
}

// bytes reads a length-prefixed field and returns a copy of it, nil when it is empty.
func (decoder *walDecoder) bytes() []byte {
	length := decoder.uvarint()
	if decoder.bad || length > uint64(len(decoder.buf)) {
		decoder.bad = true
		return nil
	}
	field := append([]byte(nil), decoder.buf[:length]...)
	decoder.buf = decoder.buf[length:]
	return field
}

// txs reads a count of length-prefixed transactions, nil when there are none.
func (decoder *walDecoder) txs() []types.Tx {
	count := decoder.count(1)
	if count == 0 {
		return nil
	}
	txs := make([]types.Tx, 0, count)
	for range count {
		txs = append(txs, decoder.bytes())
	}
	return txs
}

func (decoder *walDecoder) hash() types.Hash {
	var hash types.Hash
	if len(decoder.buf) < len(hash) {
		decoder.bad = true
		return hash
	}
	copy(hash[:], decoder.buf)
	decoder.buf = decoder.buf[len(hash):]
	return hash
}

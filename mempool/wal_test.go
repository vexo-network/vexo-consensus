package mempool

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestDurableDAGReplaysPendingTransactions(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"

	dag, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, []byte("tx-1")); err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, []byte("tx-2")); err != nil {
		t.Fatal(err)
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	if reopened.Len() != 2 {
		t.Fatalf("expected 2 replayed txs, got %d", reopened.Len())
	}
	batch, err := reopened.BuildBatch(ctx, 1024, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Txs) != 2 {
		t.Fatalf("expected replayed batch txs, got %d", len(batch.Txs))
	}
}

func TestDurableDAGReplaysCommittedTransactionsAsRemoved(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"

	dag, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	tx := []byte("tx-1")
	if err := dag.AddTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := dag.MarkCommitted(ctx, []types.Tx{tx}); err != nil {
		t.Fatal(err)
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	if reopened.Len() != 0 {
		t.Fatalf("expected committed tx to stay removed, got %d", reopened.Len())
	}
}

func TestDurableDAGReplaysReplacementTransactions(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	cfg := FIFOConfig{Author: "alice", EnableReplacement: true}

	dag, err := OpenDurableDAG(ctx, path, NewFIFO(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, []byte("bank:send:fee=1:signer=alice:nonce=1")); err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, []byte("bank:send:fee=2:signer=alice:nonce=1")); err != nil {
		t.Fatal(err)
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(cfg))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	pending, err := reopened.PendingTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || string(pending[0]) != "bank:send:fee=2:signer=alice:nonce=1" {
		t.Fatalf("expected replaced tx after WAL replay, got %q", pending)
	}
}

func TestDurableDAGRepairsPartialWALTail(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	whole := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("tx-1")})
	torn := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("tx-2")})
	if err := os.WriteFile(path, slices.Concat(whole, torn[:len(torn)-3]), 0o600); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	if reopened.Len() != 1 {
		t.Fatalf("expected one replayed tx after partial tail repair, got %d", reopened.Len())
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, whole) {
		t.Fatalf("expected partial wal tail to be truncated, got %x", contents)
	}
}

func TestDurableDAGRejectsCorruptWALMiddle(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	first := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("tx-1")})
	second := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("tx-2")})
	third := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("tx-3")})
	second[len(second)/2] ^= 0xff
	if err := os.WriteFile(path, slices.Concat(first, second, third), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if !errors.Is(err, ErrWALCorrupt) {
		t.Fatalf("expected corrupt middle WAL record to fail instead of silently dropping data, got %v", err)
	}
}

func TestDurableDAGCompactRewritesAtomicallyAndKeepsWALUsable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "mempool.wal")

	dag, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, []byte("tx-1")); err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, []byte("tx-2")); err != nil {
		t.Fatal(err)
	}
	if err := dag.MarkCommitted(ctx, []types.Tx{[]byte("tx-1")}); err != nil {
		t.Fatal(err)
	}
	if err := dag.CompactWAL(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".compact.tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected compact temp file to be removed, got %v", err)
	}
	if err := dag.AddTx(ctx, []byte("tx-3")); err != nil {
		t.Fatal(err)
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	pending, err := reopened.PendingTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 || string(pending[0]) != "tx-2" || string(pending[1]) != "tx-3" {
		t.Fatalf("expected compacted WAL to replay only live txs, got %q", pending)
	}
}

// walWaitTimeout bounds how long a test waits for something a goroutine of the WAL must do.
const walWaitTimeout = 10 * time.Second

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(walWaitTimeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func openTestWAL(t *testing.T) (*WAL, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mempool.wal")
	wal, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wal.Close() })
	return wal, path
}

func openTestDAG(t *testing.T, config FIFOConfig) (*DAG, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mempool.wal")
	return reopenTestDAG(t, path, config), path
}

func reopenTestDAG(t *testing.T, path string, config FIFOConfig) *DAG {
	t.Helper()
	dag, err := OpenDurableDAG(context.Background(), path, NewFIFO(config))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dag.wal.Close() })
	return dag
}

// encodeWALRecords returns the bytes of a log that holds events.
func encodeWALRecords(t testing.TB, events ...WALEvent) []byte {
	t.Helper()
	var log []byte
	for _, event := range events {
		var err error
		if log, err = appendWALEvent(log, event); err != nil {
			t.Fatal(err)
		}
	}
	return log
}

// frameWALBody frames body as a record the way the format defines it, without the encoder under
// test: uvarint(len), the body, the Castagnoli checksum of the body in four big-endian bytes.
func frameWALBody(body []byte) []byte {
	record := binary.AppendUvarint(nil, uint64(len(body)))
	record = append(record, body...)
	return binary.BigEndian.AppendUint32(record, crc32.Checksum(body, crc32.MakeTable(crc32.Castagnoli)))
}

// readWALFile returns the records of the log at path, without repairing the log.
func readWALFile(t *testing.T, path string) []WALEvent {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := scanWAL(data)
	if err != nil {
		t.Fatal(err)
	}
	return scan.events
}

// replayedTxs counts how often each transaction was added by events.
func replayedTxs(events []WALEvent) map[string]int {
	counts := make(map[string]int)
	for _, event := range events {
		if event.Op == walOpAddTx {
			counts[string(event.Tx)]++
		}
	}
	return counts
}

func sortedTxs(txs []types.Tx) []string {
	sorted := make([]string, 0, len(txs))
	for _, tx := range txs {
		sorted = append(sorted, string(tx))
	}
	sort.Strings(sorted)
	return sorted
}

// queuedRecords is the number of records that wait for the next write.
func queuedRecords(wal *WAL) int {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if wal.pending == nil {
		return 0
	}
	scan, err := scanWAL(wal.pending.buf)
	if err != nil {
		return -1
	}
	return len(scan.events)
}

// syncGate holds back every sync of a WAL until it is opened, so a test decides which records are
// queued behind the write in progress.
type syncGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	syncs   atomic.Int64
}

func gateSyncs(t *testing.T, wal *WAL) *syncGate {
	t.Helper()
	gate := &syncGate{entered: make(chan struct{}, 1024), release: make(chan struct{})}
	wal.syncFile = func(file *os.File) error {
		gate.syncs.Add(1)
		gate.entered <- struct{}{}
		<-gate.release
		return file.Sync()
	}
	// Registered after the cleanup that closes the WAL, so it runs first: a failed test must not
	// leave Close waiting for a sync that is never released.
	t.Cleanup(gate.open)
	return gate
}

func (gate *syncGate) open() {
	gate.once.Do(func() { close(gate.release) })
}

func (gate *syncGate) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-gate.entered:
	case <-time.After(walWaitTimeout):
		t.Fatal("no sync started")
	}
}

// sampleWALEvents covers every op and the edges of the encoding: empty and absent fields, and
// bodies whose length takes one, two and three bytes of uvarint.
func sampleWALEvents() []WALEvent {
	return []WALEvent{
		{Op: walOpAddTx, Tx: types.Tx("tx-1")},
		{Op: walOpAddTx},
		{Op: walOpAddTx, Tx: bytes.Repeat([]byte{0xab}, 126)},
		{Op: walOpAddTx, Tx: bytes.Repeat([]byte{0xcd}, 127)},
		{Op: walOpAddTx, Tx: bytes.Repeat([]byte{0xef}, 70000)},
		{Op: walOpMarkCommitted, Txs: []types.Tx{types.Tx("tx-1"), types.Tx("tx-2")}},
		{Op: walOpMarkCommitted},
		{Op: walOpAddBatch, Batch: Batch{
			ID:       types.Hash{7},
			Author:   "alice",
			Txs:      []types.Tx{types.Tx("a"), types.Tx("bb")},
			Parents:  []types.Hash{{1}, {2}},
			Metadata: []byte("meta"),
		}},
		{Op: walOpAddBatch, Batch: Batch{ID: types.Hash{8}}},
	}
}

func appendSampleEvent(ctx context.Context, wal *WAL, event WALEvent) error {
	switch event.Op {
	case walOpAddTx:
		return wal.AppendAddTx(ctx, event.Tx)
	case walOpMarkCommitted:
		return wal.AppendMarkCommitted(ctx, event.Txs)
	case walOpAddBatch:
		return wal.AppendAddBatch(ctx, event.Batch)
	}
	return fmt.Errorf("unknown op %d", event.Op)
}

// The format is part of the on-disk contract: the golden records fix the checksum polynomial and
// the payload layout, independently of the encoder.
func TestWALRecordFraming(t *testing.T) {
	batch := Batch{
		ID:       types.Hash{0: 0xaa, 31: 0xbb},
		Author:   "al",
		Txs:      []types.Tx{types.Tx("t")},
		Parents:  []types.Hash{{0: 1}},
		Metadata: []byte("m"),
	}
	batchBody := []byte{byte(walOpAddBatch)}
	batchBody = append(batchBody, batch.ID[:]...)
	batchBody = append(batchBody, 0x02, 'a', 'l')
	batchBody = append(batchBody, 0x01, 0x01, 't')
	batchBody = append(batchBody, 0x01)
	batchBody = append(batchBody, batch.Parents[0][:]...)
	batchBody = append(batchBody, 0x01, 'm')

	for name, test := range map[string]struct {
		event WALEvent
		want  []byte
	}{
		"add_tx": {
			WALEvent{Op: walOpAddTx, Tx: types.Tx("ab")},
			[]byte{0x03, 0x01, 'a', 'b', 0xd6, 0xe6, 0x6f, 0xe0},
		},
		"empty add_tx": {
			WALEvent{Op: walOpAddTx},
			[]byte{0x01, 0x01, 0xa0, 0x16, 0xd0, 0x52},
		},
		"mark_committed": {
			WALEvent{Op: walOpMarkCommitted, Txs: []types.Tx{types.Tx("x"), types.Tx("yz")}},
			[]byte{0x07, 0x03, 0x02, 0x01, 'x', 0x02, 'y', 'z', 0xe7, 0xff, 0xe8, 0x9d},
		},
		"add_batch": {
			WALEvent{Op: walOpAddBatch, Batch: batch},
			frameWALBody(batchBody),
		},
	} {
		got := encodeWALRecords(t, test.event)
		if !bytes.Equal(got, test.want) {
			t.Fatalf("%s: encoded %x, want %x", name, got, test.want)
		}
		scan, err := scanWAL(test.want)
		if err != nil || scan.validBytes != len(test.want) || len(scan.events) != 1 {
			t.Fatalf("%s: scanned %d events of %d bytes, err %v", name, len(scan.events), scan.validBytes, err)
		}
		if !reflect.DeepEqual(scan.events[0], test.event) {
			t.Fatalf("%s: decoded %+v, want %+v", name, scan.events[0], test.event)
		}
	}
}

func TestWALRoundTripsEveryOp(t *testing.T) {
	ctx := context.Background()
	wal, path := openTestWAL(t)
	events := sampleWALEvents()
	for _, event := range events {
		if err := appendSampleEvent(ctx, wal, event); err != nil {
			t.Fatal(err)
		}
	}

	replayed, err := wal.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, events) {
		t.Fatalf("live replay differs:\n got %+v\nwant %+v", replayed, events)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	replayed, err = reopened.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, events) {
		t.Fatalf("replay after reopening differs:\n got %+v\nwant %+v", replayed, events)
	}
}

func TestOpenWALRejectsAnEmptyPath(t *testing.T) {
	if _, err := OpenWAL(""); !errors.Is(err, ErrInvalidWALPath) {
		t.Fatalf("expected ErrInvalidWALPath, got %v", err)
	}
}

// A crash leaves a prefix of what was being written, so every cut of the last record must cost
// exactly that record: its transactions were never acknowledged.
func TestWALTornTailIsTruncated(t *testing.T) {
	kept := []WALEvent{
		{Op: walOpAddTx, Tx: types.Tx("tx-1")},
		{Op: walOpAddBatch, Batch: Batch{ID: types.Hash{3}, Author: "alice", Txs: []types.Tx{types.Tx("a")}}},
	}
	prefix := encodeWALRecords(t, kept...)
	last := encodeWALRecords(t, WALEvent{Op: walOpMarkCommitted, Txs: []types.Tx{bytes.Repeat([]byte{1}, 100), bytes.Repeat([]byte{2}, 100)}})
	if len(last) < 200 {
		t.Fatalf("the last record must need a two-byte length, got %d bytes", len(last))
	}

	for cut := 1; cut <= len(last); cut++ {
		// Every cut of the length, the op and the start of the payload, every cut of the checksum, and
		// a sample of the cuts in between.
		if cut > 8 && cut < len(last)-8 && cut%13 != 0 {
			continue
		}
		path := filepath.Join(t.TempDir(), "mempool.wal")
		if err := os.WriteFile(path, slices.Concat(prefix, last[:len(last)-cut]), 0o600); err != nil {
			t.Fatal(err)
		}
		wal, err := OpenWAL(path)
		if err != nil {
			t.Fatalf("cut %d: %v", cut, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() != int64(len(prefix)) {
			t.Fatalf("cut %d: opening left %d bytes, want the %d of the intact records", cut, info.Size(), len(prefix))
		}
		replayed, err := wal.Replay()
		if err != nil {
			t.Fatalf("cut %d: %v", cut, err)
		}
		if !reflect.DeepEqual(replayed, kept) {
			t.Fatalf("cut %d: replayed %+v, want %+v", cut, replayed, kept)
		}
		if err := wal.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWALTailDamageIsTruncated(t *testing.T) {
	kept := []WALEvent{
		{Op: walOpAddTx, Tx: types.Tx("tx-1")},
		{Op: walOpMarkCommitted, Txs: []types.Tx{types.Tx("tx-0")}},
	}
	prefix := encodeWALRecords(t, kept...)
	last := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("tx-2")})
	flipped := func(index int) []byte {
		damaged := slices.Clone(last)
		damaged[index] ^= 0x40
		return damaged
	}
	for name, tail := range map[string][]byte{
		"zero padding":       make([]byte, 64),
		"garbage":            bytes.Repeat([]byte{0xff}, 12),
		"flipped length":     flipped(0),
		"flipped op":         flipped(1),
		"flipped payload":    flipped(len(last) - 6),
		"flipped checksum":   flipped(len(last) - 1),
		"unknown op":         frameWALBody([]byte{9, 'x'}),
		"missing op":         {0x00},
		"count beyond bytes": frameWALBody([]byte{byte(walOpMarkCommitted), 0x05}),
		"trailing bytes":     frameWALBody([]byte{byte(walOpMarkCommitted), 0x00, 0x00}),
		"padded length":      {0x81, 0x00, 0x01, 0xa0, 0x16, 0xd0, 0x52},
		"torn after garbage": slices.Concat([]byte{0xff, 0xff}, last[:len(last)-2]),
		"zeroed checksum":    slices.Concat(last[:len(last)-4], []byte{0, 0, 0, 0}),
	} {
		path := filepath.Join(t.TempDir(), "mempool.wal")
		if err := os.WriteFile(path, slices.Concat(prefix, tail), 0o600); err != nil {
			t.Fatal(err)
		}
		wal, err := OpenWAL(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		replayed, err := wal.Replay()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(replayed, kept) {
			t.Fatalf("%s: replayed %+v, want %+v", name, replayed, kept)
		}
		if err := wal.Close(); err != nil {
			t.Fatal(err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(contents, prefix) {
			t.Fatalf("%s: the damaged tail was not cut away, %d bytes remain of %d", name, len(contents), len(prefix))
		}
	}
}

func TestWALAppendsStartOnARecordBoundaryAfterTailRepair(t *testing.T) {
	ctx := context.Background()
	kept := WALEvent{Op: walOpAddTx, Tx: types.Tx("kept")}
	torn := encodeWALRecords(t, WALEvent{Op: walOpAddTx, Tx: types.Tx("torn")})
	path := filepath.Join(t.TempDir(), "mempool.wal")
	if err := os.WriteFile(path, slices.Concat(encodeWALRecords(t, kept), torn[:len(torn)-2]), 0o600); err != nil {
		t.Fatal(err)
	}

	wal, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := wal.AppendAddTx(ctx, types.Tx("after")); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	want := []WALEvent{kept, {Op: walOpAddTx, Tx: types.Tx("after")}}
	if got := readWALFile(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	reopened, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("a log appended after a repair must stay readable: %v", err)
	}
	_ = reopened.Close()
}

// Damage before the tail cannot come from a crash: a valid record behind it proves that the log
// once held more than what precedes it, so replaying the prefix would silently drop data.
func TestWALCorruptionBeforeTheTailIsRefused(t *testing.T) {
	events := []WALEvent{
		{Op: walOpAddTx, Tx: types.Tx("tx-1")},
		{Op: walOpAddBatch, Batch: Batch{ID: types.Hash{3}, Author: "alice", Txs: []types.Tx{types.Tx("a")}, Parents: []types.Hash{{1}}}},
		{Op: walOpMarkCommitted, Txs: []types.Tx{types.Tx("tx-1")}},
		{Op: walOpAddTx, Tx: types.Tx("tx-2")},
	}
	var offsets []int
	log := encodeWALRecords(t)
	for _, event := range events {
		offsets = append(offsets, len(log))
		log = slices.Concat(log, encodeWALRecords(t, event))
	}

	check := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "mempool.wal")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenWAL(path); !errors.Is(err, ErrWALCorrupt) {
			t.Fatalf("%s: expected ErrWALCorrupt, got %v", name, err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(contents, data) {
			t.Fatalf("%s: a log that fails to open must be left as it is", name)
		}
	}
	// Any damage to any byte of a record that valid records follow.
	for index := 0; index < offsets[len(offsets)-1]; index++ {
		damaged := slices.Clone(log)
		damaged[index] ^= 0x01
		check(fmt.Sprintf("flipped bit of byte %d", index), damaged)
	}
	// A hole, as a write that reached the disk out of order leaves.
	hole := slices.Concat(log[:offsets[1]], make([]byte, 9), log[offsets[2]:])
	check("zeroed record", hole)
}

func TestOpenWALDiscardsAJSONLog(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mempool.wal")
	legacy := "{\"op\":\"add_tx\",\"tx\":\"dHgtMQ==\"}\n{\"op\":\"add_tx\",\"tx\":\"dHgtMg==\"}\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	dag := reopenTestDAG(t, path, FIFOConfig{})
	if dag.Len() != 0 {
		t.Fatalf("a JSON log must not be replayed, got %d transactions", dag.Len())
	}
	if err := dag.AddTx(ctx, types.Tx("tx-3")); err != nil {
		t.Fatal(err)
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readWALFile(t, path); len(got) != 1 || string(got[0].Tx) != "tx-3" {
		t.Fatalf("expected a binary log holding only the new transaction, got %+v", got)
	}
}

func TestWALAppendersAreDurableWhenTheyReturn(t *testing.T) {
	const appenders, rounds = 64, 4
	ctx := context.Background()
	wal, path := openTestWAL(t)

	// Every sync records what the file holds when it completes. An appender that returns must find
	// its record in a completed sync.
	var mu sync.Mutex
	synced := make(map[string]bool)
	wal.syncFile = func(file *os.File) error {
		if err := file.Sync(); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scan, err := scanWAL(data)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		for _, event := range scan.events {
			synced[string(event.Tx)] = true
		}
		return nil
	}

	var wg sync.WaitGroup
	for appender := range appenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := range rounds {
				tx := fmt.Sprintf("tx-%d-%d", appender, round)
				if err := wal.AppendAddTx(ctx, types.Tx(tx)); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				durable := synced[tx]
				mu.Unlock()
				if !durable {
					t.Errorf("%s returned before a sync made it durable", tx)
				}
			}
		}()
	}
	wg.Wait()
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	counts := replayedTxs(readWALFile(t, path))
	if len(counts) != appenders*rounds {
		t.Fatalf("replayed %d distinct transactions, want %d", len(counts), appenders*rounds)
	}
	for tx, count := range counts {
		if count != 1 {
			t.Fatalf("%s was replayed %d times", tx, count)
		}
	}
}

func TestWALGroupCommitSharesOneSync(t *testing.T) {
	const followers = 63
	ctx := context.Background()
	wal, path := openTestWAL(t)
	gate := gateSyncs(t, wal)

	var wg sync.WaitGroup
	appendTx := func(tx string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := wal.AppendAddTx(ctx, types.Tx(tx)); err != nil {
				t.Error(err)
			}
		}()
	}
	// The first append writes alone and stalls in its sync; the others queue behind it.
	appendTx("leader")
	gate.waitEntered(t)
	for follower := range followers {
		appendTx(fmt.Sprintf("follower-%d", follower))
	}
	waitFor(t, "every follower to queue its record", func() bool { return queuedRecords(wal) == followers })
	gate.open()
	wg.Wait()

	if syncs := gate.syncs.Load(); syncs != 2 {
		t.Fatalf("expected one sync for the leader and one for the %d followers, got %d syncs", followers, syncs)
	}
	if counts := replayedTxs(readWALFile(t, path)); len(counts) != followers+1 {
		t.Fatalf("expected %d records, got %d", followers+1, len(counts))
	}
}

func TestWALCloseAfterConcurrentAppends(t *testing.T) {
	const appenders = 16
	ctx := context.Background()
	wal, path := openTestWAL(t)

	var acknowledged atomic.Int64
	acked := make([][]string, appenders)
	var wg sync.WaitGroup
	for appender := range appenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; ; round++ {
				tx := fmt.Sprintf("tx-%d-%d", appender, round)
				err := wal.AppendAddTx(ctx, types.Tx(tx))
				if err != nil {
					if !errors.Is(err, ErrWALClosed) {
						t.Errorf("%s: %v", tx, err)
					}
					return
				}
				acked[appender] = append(acked[appender], tx)
				acknowledged.Add(1)
			}
		}()
	}
	waitFor(t, "appends to be acknowledged", func() bool { return acknowledged.Load() >= 100 })
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	counts := replayedTxs(readWALFile(t, path))
	total := 0
	for _, txs := range acked {
		for _, tx := range txs {
			total++
			if counts[tx] != 1 {
				t.Fatalf("%s was acknowledged but replayed %d times", tx, counts[tx])
			}
		}
	}
	if len(counts) != total {
		t.Fatalf("%d records were written, %d appends were acknowledged: a refused append left a record", len(counts), total)
	}

	if err := wal.AppendAddTx(ctx, types.Tx("late")); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("append after Close: %v", err)
	}
	if err := wal.Close(); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := wal.Replay(); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("Replay after Close: %v", err)
	}
	if err := wal.Compact(ctx, NewDAG(nil)); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("Compact after Close: %v", err)
	}
}

// Close writes the records that were queued while a write was in progress, and their callers see
// them durable.
func TestWALCloseFlushesQueuedRecords(t *testing.T) {
	ctx := context.Background()
	wal, path := openTestWAL(t)
	gate := gateSyncs(t, wal)

	var wg sync.WaitGroup
	for _, tx := range []string{"leader", "queued-1", "queued-2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := wal.AppendAddTx(ctx, types.Tx(tx)); err != nil {
				t.Errorf("%s: %v", tx, err)
			}
		}()
		if tx == "leader" {
			gate.waitEntered(t)
		}
	}
	waitFor(t, "both records to queue", func() bool { return queuedRecords(wal) == 2 })

	closed := make(chan error, 1)
	go func() { closed <- wal.Close() }()
	gate.open()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if counts := replayedTxs(readWALFile(t, path)); len(counts) != 3 {
		t.Fatalf("expected the three acknowledged records on disk, got %v", counts)
	}
}

// A caller that stops waiting leaves its record to be written anyway, with nobody else appending.
func TestWALCancelledWaiterRecordIsStillWritten(t *testing.T) {
	ctx := context.Background()
	wal, path := openTestWAL(t)
	gate := gateSyncs(t, wal)

	leader := make(chan error, 1)
	go func() { leader <- wal.AppendAddTx(ctx, types.Tx("leader")) }()
	gate.waitEntered(t)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelled := make(chan error, 1)
	go func() { cancelled <- wal.AppendAddTx(cancelCtx, types.Tx("cancelled")) }()
	waitFor(t, "the record to queue", func() bool { return queuedRecords(wal) == 1 })
	cancel()
	if err := <-cancelled; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the context error, got %v", err)
	}

	gate.open()
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the cancelled record to be written", func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		scan, err := scanWAL(data)
		return err == nil && replayedTxs(scan.events)["cancelled"] == 1
	})
	if err := wal.AppendAddTx(ctx, types.Tx("after")); err != nil {
		t.Fatalf("the WAL must go on after a cancelled caller: %v", err)
	}
}

// Close writes the record of a caller that gave up before the goroutine standing in for it did,
// which would find the file closed.
func TestWALCloseWritesTheRecordsOfCancelledWaiters(t *testing.T) {
	ctx := context.Background()
	wal, path := openTestWAL(t)
	gate := gateSyncs(t, wal)

	leader := make(chan error, 1)
	go func() { leader <- wal.AppendAddTx(ctx, types.Tx("leader")) }()
	gate.waitEntered(t)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelled := make(chan error, 1)
	go func() { cancelled <- wal.AppendAddTx(cancelCtx, types.Tx("cancelled")) }()
	waitFor(t, "the record to queue", func() bool { return queuedRecords(wal) == 1 })
	cancel()
	if err := <-cancelled; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the context error, got %v", err)
	}

	closed := make(chan error, 1)
	go func() { closed <- wal.Close() }()
	waitFor(t, "Close to begin", wal.isClosed)
	gate.open()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
	counts := replayedTxs(readWALFile(t, path))
	if counts["leader"] != 1 || counts["cancelled"] != 1 || len(counts) != 2 {
		t.Fatalf("expected the leader and the cancelled record, got %v", counts)
	}
}

// Callers that give up at random moments must neither hang nor lose a record they were told is
// durable, nor leave one twice.
func TestWALCancellationsNeverLoseAcknowledgedRecords(t *testing.T) {
	const appenders, rounds = 32, 16
	ctx := context.Background()
	wal, path := openTestWAL(t)

	var mu sync.Mutex
	acked := make(map[string]bool)
	var wg sync.WaitGroup
	for appender := range appenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(appender), 1))
			for round := range rounds {
				tx := fmt.Sprintf("tx-%d-%d", appender, round)
				callCtx, cancel := context.WithTimeout(ctx, time.Duration(rng.IntN(20000))*time.Microsecond)
				err := wal.AppendAddTx(callCtx, types.Tx(tx))
				cancel()
				switch {
				case err == nil:
					mu.Lock()
					acked[tx] = true
					mu.Unlock()
				case errors.Is(err, context.DeadlineExceeded):
				default:
					t.Errorf("%s: %v", tx, err)
				}
			}
		}()
	}
	wg.Wait()
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	counts := replayedTxs(readWALFile(t, path))
	for tx := range acked {
		if counts[tx] != 1 {
			t.Fatalf("%s was acknowledged but replayed %d times", tx, counts[tx])
		}
	}
	for tx, count := range counts {
		if count != 1 {
			t.Fatalf("%s was replayed %d times", tx, count)
		}
	}
}

// Admissions queue their records under the DAG lock and wait for the sync outside it, so
// concurrent callers share one sync instead of taking one each in turn.
func TestDurableDAGAdmissionsShareASync(t *testing.T) {
	const followers = 8
	ctx := context.Background()
	dag, path := openTestDAG(t, FIFOConfig{})
	gate := gateSyncs(t, dag.wal)

	var wg sync.WaitGroup
	admit := func(signer string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := dag.AddTx(ctx, sendTx(signer, 1, 10)); err != nil {
				t.Errorf("%s: %v", signer, err)
			}
		}()
	}
	admit("leader")
	gate.waitEntered(t)
	for follower := range followers {
		admit(fmt.Sprintf("follower-%d", follower))
	}
	waitFor(t, "every follower to queue its record", func() bool { return queuedRecords(dag.wal) == followers })
	gate.open()
	wg.Wait()

	if syncs := gate.syncs.Load(); syncs != 2 {
		t.Fatalf("expected one sync for the leader and one for the followers, got %d", syncs)
	}
	pooled, err := dag.SnapshotTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pooled) != followers+1 {
		t.Fatalf("expected %d pooled transactions, got %d", followers+1, len(pooled))
	}
	if logged := readWALFile(t, path); len(logged) != followers+1 {
		t.Fatalf("expected %d records, got %d", followers+1, len(logged))
	}
}

// The log must list the changes in the order the pool made them, whatever the interleaving of the
// callers, or a replay would rebuild a pool in another order.
func TestDurableDAGLogsInPoolOrder(t *testing.T) {
	const workers, perWorker = 16, 12
	ctx := context.Background()
	dag, _ := openTestDAG(t, FIFOConfig{})

	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signer := fmt.Sprintf("signer-%d", worker)
			for nonce := range perWorker {
				if err := dag.AddTx(ctx, sendTx(signer, uint64(nonce), 10)); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	pooled, err := dag.SnapshotTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	events, err := dag.wal.Replay()
	if err != nil {
		t.Fatal(err)
	}
	var logged []types.Tx
	for _, event := range events {
		logged = append(logged, event.Tx)
	}
	if len(pooled) != workers*perWorker || !reflect.DeepEqual(pooled, logged) {
		t.Fatalf("the log lists %d transactions in another order than the pool's %d", len(logged), len(pooled))
	}
}

func TestDurableDAGCompactsWhileTransactionsArrive(t *testing.T) {
	const workers, perWorker = 8, 30
	ctx := context.Background()
	dag, path := openTestDAG(t, FIFOConfig{})

	stop := make(chan struct{})
	var compactions atomic.Int64
	compactorDone := make(chan struct{})
	go func() {
		defer close(compactorDone)
		for {
			if err := dag.CompactWAL(ctx); err != nil {
				t.Error(err)
				return
			}
			compactions.Add(1)
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()

	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signer := fmt.Sprintf("signer-%d", worker)
			for nonce := range perWorker {
				if err := dag.AddTx(ctx, sendTx(signer, uint64(nonce), 10)); err != nil {
					t.Error(err)
					return
				}
				if nonce%3 == 2 {
					if err := dag.MarkCommitted(ctx, []types.Tx{sendTx(signer, uint64(nonce-1), 10)}); err != nil {
						t.Error(err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(stop)
	<-compactorDone
	if compactions.Load() == 0 {
		t.Fatal("no compaction ran")
	}

	want, err := dag.SnapshotTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != workers*(perWorker-perWorker/3) {
		t.Fatalf("expected %d pending transactions, got %d", workers*(perWorker-perWorker/3), len(want))
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".compact.tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected compact temp file to be removed, got %v", err)
	}

	reopened := reopenTestDAG(t, path, FIFOConfig{})
	got, err := reopened.SnapshotTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sortedTxs(got), sortedTxs(want)) {
		t.Fatalf("the replayed pool differs from the pool that was logged: %d against %d transactions", len(got), len(want))
	}
}

// A queued record that is not written yet must not be lost to a compaction: the snapshot is taken
// after the pool applied it, and the callers of the record are released with the old log.
func TestDurableDAGCompactionKeepsQueuedAdmissions(t *testing.T) {
	ctx := context.Background()
	dag, path := openTestDAG(t, FIFOConfig{})
	gate := gateSyncs(t, dag.wal)

	var wg sync.WaitGroup
	for _, signer := range []string{"leader", "queued"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := dag.AddTx(ctx, sendTx(signer, 1, 10)); err != nil {
				t.Errorf("%s: %v", signer, err)
			}
		}()
		if signer == "leader" {
			gate.waitEntered(t)
		}
	}
	waitFor(t, "the second record to queue", func() bool { return queuedRecords(dag.wal) == 1 })

	compacted := make(chan error, 1)
	go func() { compacted <- dag.CompactWAL(ctx) }()
	gate.open()
	if err := <-compacted; err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}
	// The queued record went to the log that the compaction replaced, so the new log holds the
	// snapshot alone.
	if logged := readWALFile(t, path); len(logged) != 2 {
		t.Fatalf("expected the two records of the snapshot, got %d", len(logged))
	}
	reopened := reopenTestDAG(t, path, FIFOConfig{})
	if reopened.Len() != 2 {
		t.Fatalf("expected both admissions to survive the compaction, got %d", reopened.Len())
	}
}

// Compact is serialized with the writer: it cannot replace the file under a write in progress, and
// a compaction whose context ends while it waits leaves the WAL as it was.
func TestDurableDAGCompactionWaitsForTheWriteInProgress(t *testing.T) {
	ctx := context.Background()
	dag, path := openTestDAG(t, FIFOConfig{})
	gate := gateSyncs(t, dag.wal)

	leader := make(chan error, 1)
	go func() { leader <- dag.AddTx(ctx, sendTx("alice", 1, 10)) }()
	gate.waitEntered(t)

	compactCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	compacted := make(chan error, 1)
	go func() { compacted <- dag.CompactWAL(compactCtx) }()
	select {
	case err := <-compacted:
		t.Fatalf("Compact returned while a write was in progress: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-compacted; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the context error, got %v", err)
	}

	gate.open()
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(ctx, sendTx("alice", 2, 10)); err != nil {
		t.Fatalf("the WAL must go on after a cancelled compaction: %v", err)
	}
	if logged := readWALFile(t, path); len(logged) != 2 {
		t.Fatalf("expected the two records written, got %d", len(logged))
	}
}

// A caller that stops waiting for the sync gets its context error, but the transaction is in the
// pool and its record is still written.
func TestDurableDAGAddTxContextErrorKeepsTheTransaction(t *testing.T) {
	ctx := context.Background()
	dag, path := openTestDAG(t, FIFOConfig{})
	gate := gateSyncs(t, dag.wal)

	leader := make(chan error, 1)
	go func() { leader <- dag.AddTx(ctx, sendTx("leader", 1, 10)) }()
	gate.waitEntered(t)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelled := make(chan error, 1)
	go func() { cancelled <- dag.AddTx(cancelCtx, sendTx("cancelled", 1, 10)) }()
	waitFor(t, "the record to queue", func() bool { return queuedRecords(dag.wal) == 1 })
	cancel()
	if err := <-cancelled; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the context error, got %v", err)
	}
	if dag.Len() != 2 {
		t.Fatalf("expected the cancelled admission in the pool, got %d transactions", dag.Len())
	}

	gate.open()
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the cancelled record to be written", func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		scan, err := scanWAL(data)
		return err == nil && replayedTxs(scan.events)[string(sendTx("cancelled", 1, 10))] == 1
	})
}

// After a failed write the content of the file is unknown, so the WAL refuses appends. The first
// change the DAG makes after that must rewrite the log from the pool, which still holds the
// transaction whose record failed: commits are retried by the node before it compacts, so a WAL
// that waited for a compaction would fail them for good.
func TestDurableDAGRepairsAWALThatFailed(t *testing.T) {
	batch := Batch{Author: "a", Txs: []types.Tx{types.Tx("batch")}}
	batch.ID = HashBatch(batch)
	for name, change := range map[string]func(context.Context, *DAG) error{
		"AddTx": func(ctx context.Context, dag *DAG) error {
			return dag.AddTx(ctx, sendTx("alice", 3, 10))
		},
		"MarkCommitted": func(ctx context.Context, dag *DAG) error {
			return dag.MarkCommitted(ctx, []types.Tx{sendTx("alice", 1, 10)})
		},
		"AddBatch": func(ctx context.Context, dag *DAG) error {
			return dag.AddBatch(ctx, batch)
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			dag, path := openTestDAG(t, FIFOConfig{})
			if err := dag.AddTx(ctx, sendTx("alice", 1, 10)); err != nil {
				t.Fatal(err)
			}

			errSync := errors.New("injected sync failure")
			var syncs atomic.Int32
			dag.wal.syncFile = func(file *os.File) error {
				if syncs.Add(1) == 1 {
					return errSync
				}
				return file.Sync()
			}
			err := dag.AddTx(ctx, sendTx("alice", 2, 10))
			if !errors.Is(err, ErrWALFailed) || !errors.Is(err, errSync) {
				t.Fatalf("expected the sync failure, got %v", err)
			}
			if dag.Len() != 2 {
				t.Fatalf("the transaction whose record failed stays in the pool, got %d", dag.Len())
			}
			if err := dag.wal.AppendAddTx(ctx, types.Tx("direct")); !errors.Is(err, ErrWALFailed) {
				t.Fatalf("expected a failed WAL to refuse appends, got %v", err)
			}
			if syncs.Load() != 1 {
				t.Fatalf("a failed WAL must not write again, got %d syncs", syncs.Load())
			}

			if err := change(ctx, dag); err != nil {
				t.Fatalf("the DAG must repair the WAL: %v", err)
			}
			want, err := dag.SnapshotTxs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			wantBatches := dag.BatchCount()
			if err := dag.wal.Close(); err != nil {
				t.Fatal(err)
			}

			reopened := reopenTestDAG(t, path, FIFOConfig{})
			got, err := reopened.SnapshotTxs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(sortedTxs(got), sortedTxs(want)) || reopened.BatchCount() != wantBatches {
				t.Fatalf("expected %q and %d batches after the repair, got %q and %d", sortedTxs(want), wantBatches, sortedTxs(got), reopened.BatchCount())
			}
		})
	}
}

func TestDurableDAGReplaysBatchesParentsFirst(t *testing.T) {
	ctx := context.Background()
	dag, path := openTestDAG(t, FIFOConfig{})
	first := Batch{Author: "a", Txs: []types.Tx{types.Tx("first")}}
	first.ID = HashBatch(first)
	second := Batch{Author: "b", Txs: []types.Tx{types.Tx("second")}, Parents: []types.Hash{first.ID}, Metadata: []byte("meta")}
	second.ID = HashBatch(second)
	for _, batch := range []Batch{first, second} {
		if err := dag.AddBatch(ctx, batch); err != nil {
			t.Fatal(err)
		}
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	checkReplayed := func(stage string, dag *DAG) {
		t.Helper()
		if dag.BatchCount() != 2 {
			t.Fatalf("%s: expected 2 replayed batches, got %d", stage, dag.BatchCount())
		}
		got, err := dag.GetBatch(ctx, second.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, second) {
			t.Fatalf("%s: replayed %+v, want %+v", stage, got, second)
		}
		ready, err := dag.ReadyBatches(ctx)
		if err != nil || len(ready) != 1 || ready[0].ID != second.ID {
			t.Fatalf("%s: expected the second batch as the only tip, got %+v, %v", stage, ready, err)
		}
	}

	reopened := reopenTestDAG(t, path, FIFOConfig{})
	checkReplayed("after the appends", reopened)
	// Compaction must write a parent before its children, or the replay refuses the child.
	if err := reopened.CompactWAL(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reopened.wal.Close(); err != nil {
		t.Fatal(err)
	}
	checkReplayed("after the compaction", reopenTestDAG(t, path, FIFOConfig{}))
}

// FuzzWALScan checks that scanning never panics, that the valid prefix is canonical (encoding the
// scanned records reproduces it byte for byte), and that damage is reported only as ErrWALCorrupt.
func FuzzWALScan(f *testing.F) {
	valid := encodeWALRecords(f, sampleWALEvents()...)
	f.Add(valid)
	f.Add(valid[:len(valid)-1])
	f.Add(valid[:len(valid)/2])
	f.Add(slices.Concat(valid, make([]byte, 5)))
	f.Add(slices.Concat(valid[:3], valid))
	f.Add([]byte{})
	f.Add([]byte("{\"op\":\"add_tx\",\"tx\":\"dHgtMQ==\"}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		scan, err := scanWAL(data)
		if err != nil {
			if !errors.Is(err, ErrWALCorrupt) {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		if scan.validBytes < 0 || scan.validBytes > len(data) {
			t.Fatalf("valid prefix of %d bytes in %d", scan.validBytes, len(data))
		}
		var again []byte
		for _, event := range scan.events {
			if again, err = appendWALEvent(again, event); err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(again, data[:scan.validBytes]) {
			t.Fatalf("re-encoding the scanned records gives %x, want %x", again, data[:scan.validBytes])
		}
	})
}

// The log is compacted once it holds walCompactMinBytes and walCompactGrowth times what it held
// after the last compaction, not on every commit; whatever it holds, a restart replays the pool.
func TestCompactWALWhenGrownRewritesOnlyALogThatGrew(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mempool.wal")
	dag, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	// The test counts bytes, not durability; a sync per admission would only slow it down.
	dag.wal.syncFile = func(*os.File) error { return nil }
	size := func() int64 {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Size()
	}
	payload := bytes.Repeat([]byte("x"), 1000)
	var pending []types.Tx
	next := 0
	block := func(add, commit int) {
		for range add {
			tx := append([]byte(fmt.Sprintf("tx-%d-", next)), payload...)
			next++
			if err := dag.AddTx(ctx, tx); err != nil {
				t.Fatal(err)
			}
			pending = append(pending, tx)
		}
		if err := dag.MarkCommitted(ctx, pending[:commit]); err != nil {
			t.Fatal(err)
		}
		pending = pending[commit:]
		if err := dag.CompactWALWhenGrown(ctx); err != nil {
			t.Fatal(err)
		}
	}
	block(100, 90)
	if got := size(); got < 100*1000 {
		t.Fatalf("a log of %d bytes, under walCompactMinBytes, must not be compacted", got)
	}
	compactions := 0
	for range 200 {
		before := size()
		block(100, 100)
		if size() < before {
			compactions++
		}
		if got := size(); got > walCompactMinBytes+200*1100 {
			t.Fatalf("the log grew to %d bytes, past walCompactMinBytes and one block", got)
		}
	}
	if compactions == 0 || compactions > 10 {
		t.Fatalf("200 commits compacted the log %d times, want at least once and about every %d bytes", compactions, walCompactMinBytes)
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"}))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	replayed, err := reopened.PendingTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != len(pending) {
		t.Fatalf("restart replays %d pending transactions, want %d", len(replayed), len(pending))
	}
	for index := range pending {
		if !bytes.Equal(replayed[index], pending[index]) {
			t.Fatalf("restart replays transaction %d as %q", index, replayed[index][:8])
		}
	}
}

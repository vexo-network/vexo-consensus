package mempool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// benchmarkAppenders multiplies GOMAXPROCS to the number of concurrent
// callers a busy node has: RPC handlers and gossip receivers all admit
// transactions at once, and each waits for its own record to be durable.
const benchmarkAppenders = 8

// countSyncs makes wal count its syncs, so a benchmark can report how many
// records one sync made durable.
func countSyncs(wal *WAL) *atomic.Int64 {
	var syncs atomic.Int64
	wal.syncFile = func(file *os.File) error {
		syncs.Add(1)
		return file.Sync()
	}
	return &syncs
}

func reportRecordsPerSync(b *testing.B, syncs *atomic.Int64) {
	if count := syncs.Load(); count > 0 {
		b.ReportMetric(float64(b.N)/float64(count), "records/sync")
	}
}

// BenchmarkWALAppendParallel measures the durable append rate of concurrent
// callers. Every append returns only after its record is on stable storage, so
// the rate is bounded by the number of records one sync makes durable.
func BenchmarkWALAppendParallel(b *testing.B) {
	wal, err := OpenWAL(filepath.Join(b.TempDir(), "mempool.wal"))
	if err != nil {
		b.Fatal(err)
	}
	defer wal.Close()
	syncs := countSyncs(wal)
	ctx := context.Background()
	tx := bytes.Repeat([]byte{0xab}, 256)

	b.SetParallelism(benchmarkAppenders)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := wal.AppendAddTx(ctx, tx); err != nil {
				b.Error(err)
				return
			}
		}
	})
	b.StopTimer()
	reportRecordsPerSync(b, syncs)
}

// BenchmarkDurableDAGAddTxParallel measures transaction admission through a
// durable DAG, the path the node takes: admission, the WAL record and the
// pool insertion of one transaction, from concurrent callers.
func BenchmarkDurableDAGAddTxParallel(b *testing.B) {
	ctx := context.Background()
	dag, err := OpenDurableDAG(ctx, filepath.Join(b.TempDir(), "mempool.wal"), NewFIFO(FIFOConfig{}))
	if err != nil {
		b.Fatal(err)
	}
	defer dag.wal.Close()
	syncs := countSyncs(dag.wal)
	var workers atomic.Uint64

	b.SetParallelism(benchmarkAppenders)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		signer := fmt.Sprintf("signer-%d", workers.Add(1))
		for nonce := uint64(0); pb.Next(); nonce++ {
			if err := dag.AddTx(ctx, sendTx(signer, nonce, 10)); err != nil {
				b.Error(err)
				return
			}
		}
	})
	b.StopTimer()
	reportRecordsPerSync(b, syncs)
}

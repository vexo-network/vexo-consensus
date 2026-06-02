package mempool

import (
	"context"
	"errors"
	"testing"
)

func FuzzFIFOAddAndBuildBatch(f *testing.F) {
	f.Add([]byte("fee:10 bank:send"), []byte("fee:1 bank:mint"), int64(64), int64(128), uint64(1), false)
	f.Add([]byte("a"), []byte("a"), int64(1), int64(1), uint64(0), false)
	f.Add([]byte{}, []byte("b"), int64(4), int64(4), uint64(0), true)
	f.Fuzz(func(t *testing.T, first []byte, second []byte, maxTxBytes int64, maxBatchBytes int64, minFee uint64, allowDuplicate bool) {
		if maxTxBytes < 0 {
			maxTxBytes = -maxTxBytes
		}
		if maxBatchBytes < 0 {
			maxBatchBytes = -maxBatchBytes
		}
		pool := NewFIFO(FIFOConfig{
			MaxTxBytes:     maxTxBytes%1024 + 1,
			MaxTxs:         4,
			MinFee:         minFee % 1000,
			AllowDuplicate: allowDuplicate,
		})
		for _, tx := range [][]byte{first, second} {
			err := pool.AddTx(context.Background(), tx)
			if len(tx) == 0 && !errors.Is(err, ErrEmptyTx) {
				t.Fatalf("expected empty tx rejection, got %v", err)
			}
		}
		batch, err := pool.BuildBatch(context.Background(), maxBatchBytes%2048+1)
		if err != nil {
			t.Fatalf("unexpected batch build error: %v", err)
		}
		for _, tx := range batch.Txs {
			if len(tx) == 0 {
				t.Fatal("batch contains empty transaction")
			}
			if int64(len(tx)) > pool.config.MaxTxBytes {
				t.Fatalf("batch contains oversized transaction: %d > %d", len(tx), pool.config.MaxTxBytes)
			}
		}
	})
}

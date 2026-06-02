package mempool

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestFIFOAddAndBuildBatchInOrder(t *testing.T) {
	pool := NewFIFO(FIFOConfig{Author: "alice"})
	for _, tx := range []types.Tx{[]byte("a"), []byte("bb"), []byte("ccc")} {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}

	batch, err := pool.BuildBatch(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Author != "alice" {
		t.Fatalf("expected author alice, got %s", batch.Author)
	}
	if !reflect.DeepEqual(batch.Txs, []types.Tx{[]byte("a"), []byte("bb"), []byte("ccc")}) {
		t.Fatalf("unexpected batch txs: %q", batch.Txs)
	}
	if batch.ID == (types.Hash{}) {
		t.Fatal("expected non-zero batch id")
	}
}

func TestFIFOBuildBatchRespectsMaxBytes(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	for _, tx := range []types.Tx{[]byte("aa"), []byte("bbb"), []byte("c")} {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}

	batch, err := pool.BuildBatch(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batch.Txs, []types.Tx{[]byte("aa"), []byte("bbb")}) {
		t.Fatalf("unexpected batch txs: %q", batch.Txs)
	}
}

func TestFIFOBuildBatchSkipsOversizedFirstTx(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	if err := pool.AddTx(context.Background(), []byte("too-large")); err != nil {
		t.Fatal(err)
	}

	batch, err := pool.BuildBatch(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Txs) != 0 {
		t.Fatalf("expected empty batch, got %q", batch.Txs)
	}
}

func TestFIFOBuildBatchRejectsInvalidMaxBytes(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	_, err := pool.BuildBatch(context.Background(), 0)
	if !errors.Is(err, ErrInvalidMaxBytes) {
		t.Fatalf("expected invalid max bytes, got %v", err)
	}
}

func TestFIFORejectsEmptyDuplicateTooLargeAndFull(t *testing.T) {
	pool := NewFIFO(FIFOConfig{MaxTxBytes: 3, MaxTxs: 1})

	if err := pool.AddTx(context.Background(), nil); !errors.Is(err, ErrEmptyTx) {
		t.Fatalf("expected empty tx, got %v", err)
	}
	if err := pool.AddTx(context.Background(), []byte("toolarge")); !errors.Is(err, ErrTxTooLarge) {
		t.Fatalf("expected tx too large, got %v", err)
	}
	if err := pool.AddTx(context.Background(), []byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("abc")); !errors.Is(err, ErrMempoolFull) {
		t.Fatalf("full check should run before duplicate check, got %v", err)
	}
}

func TestFIFORejectsDuplicateWhenNotFull(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	if err := pool.AddTx(context.Background(), []byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("abc")); !errors.Is(err, ErrDuplicateTx) {
		t.Fatalf("expected duplicate tx, got %v", err)
	}
}

func TestFIFORejectsDuplicateFloodWithoutGrowing(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	if err := pool.AddTx(context.Background(), []byte("abc")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if err := pool.AddTx(context.Background(), []byte("abc")); !errors.Is(err, ErrDuplicateTx) {
			t.Fatalf("expected duplicate at attempt %d, got %v", i, err)
		}
	}
	if pool.Len() != 1 {
		t.Fatalf("expected duplicate flood not to grow pool, got len %d", pool.Len())
	}
}

func TestFIFORejectsOversizedFloodWithoutGrowing(t *testing.T) {
	pool := NewFIFO(FIFOConfig{MaxTxBytes: 4})
	for i := 0; i < 1000; i++ {
		if err := pool.AddTx(context.Background(), []byte("too-large")); !errors.Is(err, ErrTxTooLarge) {
			t.Fatalf("expected oversized tx at attempt %d, got %v", i, err)
		}
	}
	if pool.Len() != 0 {
		t.Fatalf("expected oversized flood not to grow pool, got len %d", pool.Len())
	}
}

func TestFIFOAllowsDuplicateWhenConfigured(t *testing.T) {
	pool := NewFIFO(FIFOConfig{AllowDuplicate: true})
	if err := pool.AddTx(context.Background(), []byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("abc")); err != nil {
		t.Fatalf("expected duplicate to be allowed, got %v", err)
	}
	if pool.Len() != 2 {
		t.Fatalf("expected len 2, got %d", pool.Len())
	}
}

func TestFIFOMarkCommittedRemovesTxsAndAllowsReadd(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	for _, tx := range []types.Tx{[]byte("a"), []byte("b"), []byte("c")} {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}

	if err := pool.MarkCommitted(context.Background(), []types.Tx{[]byte("b")}); err != nil {
		t.Fatal(err)
	}
	if pool.Len() != 2 {
		t.Fatalf("expected len 2, got %d", pool.Len())
	}
	batch, err := pool.BuildBatch(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batch.Txs, []types.Tx{[]byte("a"), []byte("c")}) {
		t.Fatalf("unexpected remaining txs: %q", batch.Txs)
	}
	if err := pool.AddTx(context.Background(), []byte("b")); err != nil {
		t.Fatalf("expected committed tx to be re-addable, got %v", err)
	}
}

func TestFIFOContextCancellation(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := pool.AddTx(ctx, []byte("a")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected add canceled, got %v", err)
	}
	if _, err := pool.BuildBatch(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected build canceled, got %v", err)
	}
	if err := pool.MarkCommitted(ctx, []types.Tx{[]byte("a")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected mark committed canceled, got %v", err)
	}
}

func TestHashBatchChangesWithContent(t *testing.T) {
	first := HashBatch(Batch{Author: "a", Txs: []types.Tx{[]byte("one")}})
	second := HashBatch(Batch{Author: "a", Txs: []types.Tx{[]byte("two")}})
	if first == second {
		t.Fatal("expected different batch hashes")
	}
}

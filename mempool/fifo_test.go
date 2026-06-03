package mempool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

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

func TestFIFORejectsRecentlySeenCommittedTx(t *testing.T) {
	pool := NewFIFO(FIFOConfig{SeenTTL: time.Minute})
	now := time.Unix(100, 0)
	pool.now = func() time.Time { return now }

	if err := pool.AddTx(context.Background(), []byte("b")); err != nil {
		t.Fatal(err)
	}
	if err := pool.MarkCommitted(context.Background(), []types.Tx{[]byte("b")}); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("b")); !errors.Is(err, ErrDuplicateTx) {
		t.Fatalf("expected recently seen tx to be rejected, got %v", err)
	}

	now = now.Add(time.Minute + time.Second)
	if err := pool.AddTx(context.Background(), []byte("b")); err != nil {
		t.Fatalf("expected seen tx to be accepted after ttl, got %v", err)
	}
}

func TestFIFORejectsInsufficientFee(t *testing.T) {
	pool := NewFIFO(FIFOConfig{MinFee: 10})
	if err := pool.AddTx(context.Background(), []byte("bank:send:fee=9")); !errors.Is(err, ErrInsufficientFee) {
		t.Fatalf("expected insufficient fee, got %v", err)
	}
	if err := pool.AddTx(context.Background(), []byte("bank:send:fee=10")); err != nil {
		t.Fatalf("expected minimum fee tx to pass, got %v", err)
	}
	if err := pool.AddTx(context.Background(), []byte("bank:send:fee=1gvxo")); err != nil {
		t.Fatalf("expected unit-denominated fee tx to pass, got %v", err)
	}
}

func TestFIFOBuildBatchPrioritizesPriorityThenFee(t *testing.T) {
	pool := NewFIFO(FIFOConfig{EnablePriority: true})
	for _, tx := range []types.Tx{
		[]byte("tx:low:fee=100:priority=1"),
		[]byte("tx:high-fee:fee=200:priority=5"),
		[]byte("tx:high-priority:fee=1:priority=10"),
		[]byte("tx:tie:fee=100:priority=5"),
	} {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}

	batch, err := pool.BuildBatch(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	expected := []types.Tx{
		[]byte("tx:high-priority:fee=1:priority=10"),
		[]byte("tx:high-fee:fee=200:priority=5"),
		[]byte("tx:tie:fee=100:priority=5"),
		[]byte("tx:low:fee=100:priority=1"),
	}
	if !reflect.DeepEqual(batch.Txs, expected) {
		t.Fatalf("unexpected priority order: %q", batch.Txs)
	}
}

func TestTxFeeAndPriorityParseTags(t *testing.T) {
	tx := types.Tx("bank:send:fee=42:priority=7")
	if TxFee(tx) != 42 {
		t.Fatalf("expected fee 42, got %d", TxFee(tx))
	}
	if TxFee(types.Tx("bank:send:fee=1gvxo")) != 1_000_000_000 {
		t.Fatalf("expected unit-denominated fee, got %d", TxFee(types.Tx("bank:send:fee=1gvxo")))
	}
	if TxPriority(tx) != 7 {
		t.Fatalf("expected priority 7, got %d", TxPriority(tx))
	}
}

func TestTxFeeAndPriorityUnwrapSignedTx(t *testing.T) {
	envelope, err := json.Marshal(map[string]string{
		"schema_version": "v1",
		"payload":        base64.StdEncoding.EncodeToString([]byte("bank:send:fee=42:priority=7")),
	})
	if err != nil {
		t.Fatal(err)
	}
	signedTx := types.Tx("signed:" + base64.StdEncoding.EncodeToString(envelope))
	if TxFee(signedTx) != 42 || TxPriority(signedTx) != 7 {
		t.Fatalf("expected signed tx fee/priority, got fee=%d priority=%d", TxFee(signedTx), TxPriority(signedTx))
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

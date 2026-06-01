package mempool

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestDAGBuildBatchUsesCurrentTipsAsParents(t *testing.T) {
	dag := NewDAG(NewFIFO(FIFOConfig{Author: "alice"}))
	if err := dag.AddTx(context.Background(), []byte("a")); err != nil {
		t.Fatal(err)
	}

	first, err := dag.BuildBatch(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Parents) != 0 {
		t.Fatalf("first batch should have no parents, got %d", len(first.Parents))
	}
	if err := dag.AddBatch(context.Background(), first); err != nil {
		t.Fatal(err)
	}

	second, err := dag.BuildBatch(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Parents) != 1 || second.Parents[0] != first.ID {
		t.Fatalf("expected first batch as parent, got %v", second.Parents)
	}
}

func TestDAGAddBatchUpdatesTips(t *testing.T) {
	dag := NewDAG(nil)
	first := Batch{Author: "a", Txs: []types.Tx{[]byte("first")}}
	first.ID = HashBatch(first)
	second := Batch{Author: "b", Parents: []types.Hash{first.ID}, Txs: []types.Tx{[]byte("second")}}
	second.ID = HashBatch(second)

	if err := dag.AddBatch(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	ready, err := dag.ReadyBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].ID != first.ID {
		t.Fatalf("expected first tip, got %v", ready)
	}

	if err := dag.AddBatch(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	ready, err = dag.ReadyBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].ID != second.ID {
		t.Fatalf("expected second tip, got %v", ready)
	}
}

func TestDAGRejectsDuplicateBatch(t *testing.T) {
	dag := NewDAG(nil)
	batch := Batch{Author: "a", Txs: []types.Tx{[]byte("tx")}}
	batch.ID = HashBatch(batch)

	if err := dag.AddBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if err := dag.AddBatch(context.Background(), batch); !errors.Is(err, ErrDuplicateBatch) {
		t.Fatalf("expected duplicate batch, got %v", err)
	}
}

func TestDAGRejectsUnknownParent(t *testing.T) {
	dag := NewDAG(nil)
	var unknown types.Hash
	unknown[0] = 9
	batch := Batch{Author: "a", Parents: []types.Hash{unknown}, Txs: []types.Tx{[]byte("tx")}}
	batch.ID = HashBatch(batch)

	if err := dag.AddBatch(context.Background(), batch); !errors.Is(err, ErrUnknownParentBatch) {
		t.Fatalf("expected unknown parent, got %v", err)
	}
}

func TestDAGGetBatchReturnsCopy(t *testing.T) {
	dag := NewDAG(nil)
	batch := Batch{Author: "a", Txs: []types.Tx{[]byte("tx")}, Metadata: []byte("meta")}
	batch.ID = HashBatch(batch)

	if err := dag.AddBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	got, err := dag.GetBatch(context.Background(), batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Txs[0][0] = 'X'
	got.Metadata[0] = 'X'

	again, err := dag.GetBatch(context.Background(), batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Txs[0]) != "tx" {
		t.Fatalf("batch tx mutated: %q", again.Txs[0])
	}
	if string(again.Metadata) != "meta" {
		t.Fatalf("batch metadata mutated: %q", again.Metadata)
	}
}

func TestDAGGetBatchNotFound(t *testing.T) {
	dag := NewDAG(nil)
	_, err := dag.GetBatch(context.Background(), types.Hash{1})
	if !errors.Is(err, ErrBatchNotFound) {
		t.Fatalf("expected batch not found, got %v", err)
	}
}

func TestDAGDelegatesTxLifecycleToBaseMempool(t *testing.T) {
	dag := NewDAG(NewFIFO(FIFOConfig{}))
	if err := dag.AddTx(context.Background(), []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := dag.AddTx(context.Background(), []byte("b")); err != nil {
		t.Fatal(err)
	}
	if dag.Len() != 2 {
		t.Fatalf("expected len 2, got %d", dag.Len())
	}
	if err := dag.MarkCommitted(context.Background(), []types.Tx{[]byte("a")}); err != nil {
		t.Fatal(err)
	}
	if dag.Len() != 1 {
		t.Fatalf("expected len 1, got %d", dag.Len())
	}
}

func TestDAGContextCancellation(t *testing.T) {
	dag := NewDAG(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := dag.AddBatch(ctx, Batch{Author: "a"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected add batch canceled, got %v", err)
	}
	if _, err := dag.GetBatch(ctx, types.Hash{1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected get batch canceled, got %v", err)
	}
	if _, err := dag.ReadyBatches(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ready batches canceled, got %v", err)
	}
}

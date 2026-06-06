package mempool

import (
	"context"
	"testing"

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
	batch, err := reopened.BuildBatch(ctx, 1024)
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

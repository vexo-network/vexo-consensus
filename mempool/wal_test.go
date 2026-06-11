package mempool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestDurableDAGRepairsPartialWALTail(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	if err := os.WriteFile(path, []byte("{\"op\":\"add_tx\",\"tx\":\"dHgtMQ==\"}\n{\"op\":\"add_tx\",\"tx\":"), 0o600); err != nil {
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
	if strings.Contains(string(contents), "\"tx\":") && !strings.HasSuffix(string(contents), "\n") {
		t.Fatalf("expected partial wal tail to be truncated, got %q", contents)
	}
}

func TestDurableDAGRejectsCorruptWALMiddle(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	if err := os.WriteFile(path, []byte("{\"op\":\"add_tx\",\"tx\":\"dHgtMQ==\"}\nnot-json\n{\"op\":\"add_tx\",\"tx\":\"dHgtMg==\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{Author: "alice"})); err == nil {
		t.Fatal("expected corrupt middle WAL record to fail instead of silently dropping data")
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

package mempool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

// Regression tests for the mempool audit findings: ambiguous tags, unbounded
// growth, admission blocked behind application checks, and a replay that
// dropped the sender's future-nonce transactions.

func sendTx(signer string, nonce uint64, fee uint64) types.Tx {
	return types.Tx(fmt.Sprintf("bank:send:a:b:1:fee=%d:gas=10:signer=%s:nonce=%d", fee, signer, nonce))
}

// The pool ranked a transaction by the first of two fee tags while the
// application charged the last one, so a sender could outbid the queue while
// paying nothing. A transaction with a repeated tag has no single meaning and
// is refused outright.
func TestFIFORejectsTransactionsWithRepeatedTags(t *testing.T) {
	pool := NewFIFO(FIFOConfig{EnableReplacement: true})
	for name, tx := range map[string]types.Tx{
		"duplicate fee":      types.Tx("bank:send:a:b:1:fee=1000:fee=0:signer=alice:nonce=1"),
		"duplicate signer":   types.Tx("bank:send:a:b:1:fee=1:signer=alice:signer=bob:nonce=1"),
		"duplicate nonce":    types.Tx("bank:send:a:b:1:fee=1:signer=alice:nonce=1:nonce=2"),
		"duplicate priority": types.Tx("bank:send:a:b:1:fee=1:priority=9:priority=0:signer=alice:nonce=1"),
		"empty signer":       types.Tx("bank:send:a:b:1:fee=1:signer=:nonce=1"),
		"invalid nonce":      types.Tx("bank:send:a:b:1:fee=1:signer=alice:nonce=x"),
		"signed duplicate":   signedTestMempoolPayload(t, "bank:send:a:b:1:fee=1:fee=2:signer=alice:nonce=1"),
		"undecodable signed": types.Tx("signed:@@@"),
	} {
		if err := pool.AddTx(context.Background(), tx); !errors.Is(err, ErrInvalidTx) {
			t.Fatalf("%s: expected ErrInvalidTx, got %v", name, err)
		}
		if err := pool.CheckTx(context.Background(), tx); !errors.Is(err, ErrInvalidTx) {
			t.Fatalf("%s: expected CheckTx to refuse the transaction, got %v", name, err)
		}
		if TxFee(tx) != 0 || TxPriority(tx) != 0 {
			t.Fatalf("%s: an ambiguous transaction must not carry a fee or priority", name)
		}
		if _, found := TxSigner(tx); found {
			t.Fatalf("%s: an ambiguous transaction must not name a signer", name)
		}
		if _, found := ReplacementKey(tx); found {
			t.Fatalf("%s: an ambiguous transaction must not occupy a sequence slot", name)
		}
	}
	if pool.Len() != 0 {
		t.Fatalf("expected no ambiguous transaction in the pool, got %d", pool.Len())
	}
}

func TestFIFOCapsPendingTransactionsPerSender(t *testing.T) {
	ctx := context.Background()
	pool := NewFIFO(FIFOConfig{MaxTxsPerSender: 2, EnableReplacement: true, ReplacementBumpBPS: 1000})
	for nonce := uint64(1); nonce <= 2; nonce++ {
		if err := pool.AddTx(ctx, sendTx("alice", nonce, 10)); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.AddTx(ctx, sendTx("alice", 3, 1000)); !errors.Is(err, ErrSenderLimit) {
		t.Fatalf("expected the sender cap to refuse a third transaction, got %v", err)
	}
	// The cap is per account, however the address is spelled.
	if err := pool.AddTx(ctx, sendTx("alice", 4, 10)); !errors.Is(err, ErrSenderLimit) {
		t.Fatalf("expected the sender cap to hold, got %v", err)
	}
	if err := pool.AddTx(ctx, sendTx("bob", 1, 10)); err != nil {
		t.Fatalf("another sender must not be affected: %v", err)
	}
	// Replacing a pending transaction does not add one.
	if err := pool.AddTx(ctx, sendTx("alice", 2, 20)); err != nil {
		t.Fatalf("expected a same-nonce replacement at the cap, got %v", err)
	}
	if pool.Len() != 3 {
		t.Fatalf("expected 3 pending transactions, got %d", pool.Len())
	}
	// A committed transaction frees its slot.
	if err := pool.MarkCommitted(ctx, []types.Tx{sendTx("alice", 1, 10)}); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(ctx, sendTx("alice", 3, 10)); err != nil {
		t.Fatalf("expected the committed slot to be reusable, got %v", err)
	}
}

func TestFIFOSenderSlotsAreReleasedByEveryRemovalPath(t *testing.T) {
	ctx := context.Background()
	dropped := map[string]bool{}
	pool := NewFIFO(FIFOConfig{
		MaxTxsPerSender: 1,
		ReplayCheckTx: func(_ context.Context, tx types.Tx) error {
			if dropped[string(tx)] {
				return errors.New("stale")
			}
			return nil
		},
	})

	// Dropped by the application replay check.
	stale := sendTx("alice", 1, 10)
	if err := pool.AddTx(ctx, stale); err != nil {
		t.Fatal(err)
	}
	dropped[string(stale)] = true
	if _, err := pool.PendingTxs(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(ctx, sendTx("alice", 2, 10)); err != nil {
		t.Fatalf("expected the replay-dropped slot to be released, got %v", err)
	}

	// Dropped by RetainTxs.
	if _, err := pool.RetainTxs(ctx, func(types.Tx) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(ctx, sendTx("alice", 3, 10)); err != nil {
		t.Fatalf("expected the retained-out slot to be released, got %v", err)
	}
	if pool.Len() != 1 {
		t.Fatalf("expected one pending transaction, got %d", pool.Len())
	}
}

func TestFIFOCapsTotalBytes(t *testing.T) {
	ctx := context.Background()
	size := int64(len(sendTx("a1", 1, 10)))
	pool := NewFIFO(FIFOConfig{MaxTotalBytes: 3 * size})
	for index, fee := range []uint64{10, 20, 30} {
		if err := pool.AddTx(ctx, sendTx(fmt.Sprintf("a%d", index), 1, fee)); err != nil {
			t.Fatal(err)
		}
	}
	// A same-fee transaction cannot push anyone out.
	if err := pool.AddTx(ctx, sendTx("a3", 1, 10)); !errors.Is(err, ErrMempoolFull) {
		t.Fatalf("expected a full pool to refuse an equally paying transaction, got %v", err)
	}
	if err := pool.AddTx(ctx, sendTx("a3", 1, 11)); err != nil {
		t.Fatalf("expected a higher paying transaction to take the lowest entry's place: %v", err)
	}
	if pool.Len() != 3 {
		t.Fatalf("expected the pool to stay at capacity, got %d", pool.Len())
	}
	if _, found := pool.index[HashTx(sendTx("a0", 1, 10))]; found {
		t.Fatal("expected the lowest fee transaction to be evicted")
	}

	// A transaction larger than the whole budget is never admitted, however
	// much it pays.
	oversized := NewFIFO(FIFOConfig{MaxTotalBytes: size - 1})
	if err := oversized.AddTx(ctx, sendTx("a1", 1, 1000)); !errors.Is(err, ErrMempoolFull) {
		t.Fatalf("expected a transaction above the byte budget to be refused, got %v", err)
	}
}

func TestFIFOEvictsTheLowestFeeWhenTheCountLimitIsReached(t *testing.T) {
	ctx := context.Background()
	pool := NewFIFO(FIFOConfig{MaxTxs: 3})
	for index, fee := range []uint64{10, 10, 10} {
		if err := pool.AddTx(ctx, sendTx(fmt.Sprintf("a%d", index), 1, fee)); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.AddTx(ctx, sendTx("rich", 1, 20)); err != nil {
		t.Fatalf("expected a higher paying transaction to be admitted to a full pool: %v", err)
	}
	// Among equally paying entries the newest goes first, so an older sender
	// cannot be displaced by a flood.
	if _, found := pool.index[HashTx(sendTx("a2", 1, 10))]; found {
		t.Fatal("expected the newest of the lowest paying entries to be evicted")
	}
	for _, survivor := range []string{"a0", "a1", "rich"} {
		fee := uint64(10)
		if survivor == "rich" {
			fee = 20
		}
		if _, found := pool.index[HashTx(sendTx(survivor, 1, fee))]; !found {
			t.Fatalf("expected %s to survive", survivor)
		}
	}
	if err := pool.AddTx(ctx, sendTx("poor", 1, 5)); !errors.Is(err, ErrMempoolFull) {
		t.Fatalf("expected a lower paying transaction to be refused, got %v", err)
	}
}

func TestFIFOEvictionKeepsTheAccountingConsistent(t *testing.T) {
	ctx := context.Background()
	pool := NewFIFO(FIFOConfig{MaxTxs: 2, MaxTxsPerSender: 2})
	if err := pool.AddTx(ctx, sendTx("alice", 1, 10)); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(ctx, sendTx("alice", 2, 10)); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(ctx, sendTx("bob", 1, 50)); err != nil {
		t.Fatal(err)
	}
	if pool.senders["alice"] != 1 || pool.senders["bob"] != 1 {
		t.Fatalf("unexpected sender accounting: %v", pool.senders)
	}
	var total int64
	for _, pending := range pool.txs {
		total += pending.size
	}
	if pool.totalBytes != total {
		t.Fatalf("total bytes %d does not match the pending transactions (%d)", pool.totalBytes, total)
	}
}

func TestFIFOReplacementStillPaysTheMinimumFee(t *testing.T) {
	ctx := context.Background()
	pool := NewFIFO(FIFOConfig{MinFee: 5, EnableReplacement: true, ReplacementBumpBPS: 1000})
	if err := pool.AddTx(ctx, types.Tx("bank:send:a:b:1:fee=10:priority=1:signer=alice:nonce=1")); err != nil {
		t.Fatal(err)
	}
	// The priority bump alone satisfies the price rule; the fee floor still holds.
	if err := pool.AddTx(ctx, types.Tx("bank:send:a:b:1:fee=1:priority=100:signer=alice:nonce=1")); !errors.Is(err, ErrInsufficientFee) {
		t.Fatalf("expected the replacement to pay the minimum fee, got %v", err)
	}
}

// The application check used to run while the pool lock was held, so one slow
// check stalled every submission.
func TestFIFOReplayCheckDoesNotBlockAdmission(t *testing.T) {
	ctx := context.Background()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	pool := NewFIFO(FIFOConfig{
		ReplayCheckTx: func(context.Context, types.Tx) error {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return nil
		},
	})
	if err := pool.AddTx(ctx, sendTx("alice", 1, 10)); err != nil {
		t.Fatal(err)
	}

	built := make(chan error, 1)
	go func() {
		_, err := pool.BuildBatch(ctx, 1<<20)
		built <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the replay check never ran")
	}

	added := make(chan error, 1)
	go func() { added <- pool.AddTx(ctx, sendTx("bob", 1, 10)) }()
	select {
	case err := <-added:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("admission is blocked behind a running application check")
	}
	if pool.Len() != 2 {
		t.Fatalf("expected the new transaction to be pending, got %d", pool.Len())
	}
	close(release)
	if err := <-built; err != nil {
		t.Fatal(err)
	}
}

func TestFIFORetainTxsDoesNotBlockAdmission(t *testing.T) {
	ctx := context.Background()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	pool := NewFIFO(FIFOConfig{})
	if err := pool.AddTx(ctx, sendTx("alice", 1, 10)); err != nil {
		t.Fatal(err)
	}
	retained := make(chan error, 1)
	go func() {
		_, err := pool.RetainTxs(ctx, func(types.Tx) bool {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return true
		})
		retained <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the retain callback never ran")
	}
	added := make(chan error, 1)
	go func() { added <- pool.AddTx(ctx, sendTx("bob", 1, 10)) }()
	select {
	case err := <-added:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("admission is blocked behind a retain callback")
	}
	close(release)
	if err := <-retained; err != nil {
		t.Fatal(err)
	}
}

// Replay dropped every logged transaction the application did not accept YET,
// so a restart lost the sender's whole queue of future-nonce transactions.
func TestDurableDAGKeepsFutureNonceTransactionsOnReplay(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	ready := sendTx("alice", 1, 10)
	future := sendTx("alice", 3, 10)
	stale := sendTx("alice", 0, 10)

	dag, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range []types.Tx{stale, ready, future} {
		if err := dag.AddTx(ctx, tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{
		ReplayCheckTx: func(_ context.Context, tx types.Tx) error {
			nonce, _ := TxNonce(tx)
			switch {
			case nonce == 0:
				return errors.New("nonce already used")
			case nonce > 1:
				return ErrTxNotReady
			default:
				return nil
			}
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.wal.Close()
	if reopened.Len() != 2 {
		t.Fatalf("expected the ready and the future-nonce transaction to survive replay, got %d", reopened.Len())
	}
	pending, err := reopened.base.SnapshotTxs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(pending[0]) != string(ready) || string(pending[1]) != string(future) {
		t.Fatalf("unexpected replayed transactions: %q", pending)
	}
}

// A logged transaction that the pool's current limits refuse must not stop the
// node from starting.
func TestDurableDAGStartsWhenPoolPolicyRefusesALoggedTransaction(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/mempool.wal"
	dag, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	for nonce := uint64(1); nonce <= 3; nonce++ {
		if err := dag.AddTx(ctx, sendTx("alice", nonce, 10)); err != nil {
			t.Fatal(err)
		}
	}
	if err := dag.wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenDurableDAG(ctx, path, NewFIFO(FIFOConfig{MaxTxsPerSender: 2}))
	if err != nil {
		t.Fatalf("a refused logged transaction must not abort startup: %v", err)
	}
	defer reopened.wal.Close()
	if reopened.Len() != 2 {
		t.Fatalf("expected the pool limit to apply to the replay, got %d", reopened.Len())
	}
}

func TestDurableDAGBuildBatchDoesNotBlockAdmission(t *testing.T) {
	ctx := context.Background()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	dag, err := OpenDurableDAG(ctx, t.TempDir()+"/mempool.wal", NewFIFO(FIFOConfig{
		ReplayCheckTx: func(context.Context, types.Tx) error {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return nil
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer dag.wal.Close()
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	if err := dag.AddTx(ctx, sendTx("alice", 1, 10)); err != nil {
		t.Fatal(err)
	}
	built := make(chan error, 1)
	go func() {
		_, err := dag.BuildBatch(ctx, 1<<20)
		built <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the replay check never ran")
	}
	added := make(chan error, 1)
	go func() { added <- dag.AddTx(ctx, sendTx("bob", 1, 10)) }()
	select {
	case err := <-added:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DAG admission is blocked behind a running application check")
	}
	unblock()
	if err := <-built; err != nil {
		t.Fatal(err)
	}
}

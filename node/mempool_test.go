package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/mempool"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/types"
)

// scriptedAdmissionApp is an application whose admission verdicts and committed
// state the test scripts.
type scriptedAdmissionApp struct {
	app.Application
	single func(ctx context.Context, tx types.Tx) app.CheckTxResponse
	batch  func(ctx context.Context, txs []types.Tx) app.CheckTxResponse

	mu     sync.Mutex
	height types.Height
}

func (scripted *scriptedAdmissionApp) CheckTxContext(ctx context.Context, tx types.Tx) app.CheckTxResponse {
	return scripted.single(ctx, tx)
}

func (scripted *scriptedAdmissionApp) CheckTxsContext(ctx context.Context, txs []types.Tx) app.CheckTxResponse {
	return scripted.batch(ctx, txs)
}

func (scripted *scriptedAdmissionApp) Commit() (app.CommitResponse, error) {
	scripted.mu.Lock()
	defer scripted.mu.Unlock()
	return app.CommitResponse{Height: scripted.height}, nil
}

// commitBlock stands for a block that commits while a check is running.
func (scripted *scriptedAdmissionApp) commitBlock() {
	scripted.mu.Lock()
	defer scripted.mu.Unlock()
	scripted.height++
}

func (scripted *scriptedAdmissionApp) committedHeight() types.Height {
	scripted.mu.Lock()
	defer scripted.mu.Unlock()
	return scripted.height
}

// newScriptedAdmissionNode returns a node whose runtime holds only the
// application the test scripts: admission asks it for verdicts and the node
// asks it for the committed height.
func newScriptedAdmissionNode() (*Node, *scriptedAdmissionApp) {
	scripted := &scriptedAdmissionApp{}
	return &Node{running: true, runtime: &vexoruntime.Runtime{App: scripted}}, scripted
}

var (
	acceptTx = app.CheckTxResponse{}
	rejectTx = app.CheckTxResponse{Result: types.Result{Code: 1, Log: "invalid transaction nonce"}}
)

func nonceTx(signer string, nonce uint64) types.Tx {
	return types.Tx(fmt.Sprintf("bank:send:%s:signer=%s:nonce=%d:fee=1", signer, signer, nonce))
}

func newAdmissionPool(t *testing.T, pending ...types.Tx) *mempool.DAG {
	t.Helper()
	pool := mempool.NewDAG(mempool.NewFIFO(mempool.FIFOConfig{EnableReplacement: true}))
	for _, tx := range pending {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func awaitSignal(t *testing.T, signal <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func awaitAdmission(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the admission to finish")
		return nil
	}
}

// A commit executes a block under admissionMu. An admission that held the same
// lock while it validated a transaction would stall every commit behind the
// slowest check, and an attacker controls how slow a check is.
func TestAdmissionHoldsNoLockWhileChecking(t *testing.T) {
	cases := []struct {
		name   string
		pool   func(t *testing.T) *mempool.DAG
		script func(scripted *scriptedAdmissionApp, entered chan<- struct{}, release <-chan struct{})
	}{
		{
			name: "single transaction check",
			pool: func(t *testing.T) *mempool.DAG { return newAdmissionPool(t) },
			script: func(scripted *scriptedAdmissionApp, entered chan<- struct{}, release <-chan struct{}) {
				scripted.single = func(context.Context, types.Tx) app.CheckTxResponse {
					close(entered)
					<-release
					return acceptTx
				}
			},
		},
		{
			name: "sequence check",
			pool: func(t *testing.T) *mempool.DAG { return newAdmissionPool(t, nonceTx("alice", 1)) },
			script: func(scripted *scriptedAdmissionApp, entered chan<- struct{}, release <-chan struct{}) {
				scripted.single = func(context.Context, types.Tx) app.CheckTxResponse { return rejectTx }
				scripted.batch = func(context.Context, []types.Tx) app.CheckTxResponse {
					close(entered)
					<-release
					return acceptTx
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			node, scripted := newScriptedAdmissionNode()
			pool := testCase.pool(t)
			entered := make(chan struct{})
			release := make(chan struct{})
			testCase.script(scripted, entered, release)

			result := make(chan error, 1)
			go func() {
				result <- node.admitMempoolTx(context.Background(), scripted, pool, nonceTx("alice", 2))
			}()
			awaitSignal(t, entered, "the admission check to start")

			if !node.admissionMu.TryLock() {
				close(release)
				t.Fatal("admission holds admissionMu while it validates the transaction, which stalls block commits")
			}
			node.admissionMu.Unlock()

			close(release)
			if err := awaitAdmission(t, result); err != nil {
				t.Fatalf("expected the transaction to be admitted, got %v", err)
			}
			if got := pool.Len(); got == 0 {
				t.Fatal("expected the admitted transaction in the pool")
			}
		})
	}
}

// The verdict of a check describes the state it read. If a block commits while
// the check runs, the verdict is stale and must not put a transaction into the
// pool that the commit may just have made invalid.
func TestAdmissionDoesNotAddOnAVerdictStaleAfterACommit(t *testing.T) {
	node, scripted := newScriptedAdmissionNode()
	pool := newAdmissionPool(t, nonceTx("alice", 1))
	var batches int
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse { return rejectTx }
	scripted.batch = func(_ context.Context, txs []types.Tx) app.CheckTxResponse {
		batches++
		if batches == 1 {
			// A block commits while the first check runs; its verdict is
			// computed against the state before that block.
			scripted.commitBlock()
			return acceptTx
		}
		return rejectTx
	}

	err := node.admitMempoolTx(context.Background(), scripted, pool, nonceTx("alice", 2))
	var rejected appCheckError
	if !errors.As(err, &rejected) {
		t.Fatalf("expected the check against the new state to reject the transaction, got %v", err)
	}
	if batches != 2 {
		t.Fatalf("expected the checks to run again after the commit, ran %d times", batches)
	}
	if got := pool.Len(); got != 1 {
		t.Fatalf("a stale verdict must not add the transaction, pool holds %d", got)
	}
}

func TestAdmissionAddsOnceAfterACommitDuringTheCheck(t *testing.T) {
	node, scripted := newScriptedAdmissionNode()
	pool := newAdmissionPool(t)
	var checks int
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse {
		checks++
		if checks == 1 {
			scripted.commitBlock()
		}
		return acceptTx
	}

	if err := node.admitMempoolTx(context.Background(), scripted, pool, nonceTx("alice", 1)); err != nil {
		t.Fatal(err)
	}
	if checks != 2 {
		t.Fatalf("expected one repeat after the commit, ran %d checks", checks)
	}
	if got := pool.Len(); got != 1 {
		t.Fatalf("expected exactly one pooled transaction, got %d", got)
	}
}

func TestAdmissionGivesUpWhenBlocksKeepCommitting(t *testing.T) {
	node, scripted := newScriptedAdmissionNode()
	pool := newAdmissionPool(t)
	var checks int
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse {
		checks++
		scripted.commitBlock()
		return acceptTx
	}

	err := node.admitMempoolTx(context.Background(), scripted, pool, nonceTx("alice", 1))
	if !errors.Is(err, ErrAdmissionStateChanged) {
		t.Fatalf("expected the admission to give up, got %v", err)
	}
	if checks != maxAdmissionAttempts {
		t.Fatalf("expected %d attempts, got %d", maxAdmissionAttempts, checks)
	}
	if got := pool.Len(); got != 0 {
		t.Fatalf("an admission that never saw a stable state must add nothing, pool holds %d", got)
	}
}

func TestAdmissionSequenceHoldsOnlyTheSignerTransactionsInNonceOrder(t *testing.T) {
	node, scripted := newScriptedAdmissionNode()
	// A pending transaction of the signer without a nonce spends the signer's
	// balance all the same, so it belongs to the sequence.
	unsequenced := types.Tx("bank:send:alice:signer=alice:fee=1")
	pool := newAdmissionPool(t,
		nonceTx("alice", 3),
		nonceTx("bob", 1),
		nonceTx("alice", 1),
		unsequenced,
		nonceTx("alice", 2),
	)
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse { return rejectTx }
	var checked []types.Tx
	scripted.batch = func(_ context.Context, txs []types.Tx) app.CheckTxResponse {
		checked = append([]types.Tx(nil), txs...)
		return acceptTx
	}

	// Replaces alice's pending nonce 2 with a different transaction.
	replacement := types.Tx("bank:send:alice:signer=alice:nonce=2:fee=9")
	if err := node.admitMempoolTx(context.Background(), scripted, pool, replacement); err != nil {
		t.Fatal(err)
	}
	want := []types.Tx{unsequenced, nonceTx("alice", 1), replacement, nonceTx("alice", 3)}
	if len(checked) != len(want) {
		t.Fatalf("expected the sequence %q, checked %q", want, checked)
	}
	for index := range want {
		if string(checked[index]) != string(want[index]) {
			t.Fatalf("expected the sequence %q, checked %q", want, checked)
		}
	}
}

func TestAdmissionCapsTheSequenceItValidates(t *testing.T) {
	node, scripted := newScriptedAdmissionNode()
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse { return rejectTx }
	var sequenceLength int
	scripted.batch = func(_ context.Context, txs []types.Tx) app.CheckTxResponse {
		sequenceLength = len(txs)
		return acceptTx
	}

	pending := make([]types.Tx, 0, maxAdmissionSequence)
	for nonce := uint64(1); nonce < maxAdmissionSequence; nonce++ {
		pending = append(pending, nonceTx("alice", nonce))
	}
	pool := newAdmissionPool(t, pending...)

	// The candidate completes a sequence of exactly the cap.
	if err := node.admitMempoolTx(context.Background(), scripted, pool, nonceTx("alice", maxAdmissionSequence)); err != nil {
		t.Fatalf("a sequence of %d transactions is within the cap, got %v", maxAdmissionSequence, err)
	}
	if sequenceLength != maxAdmissionSequence {
		t.Fatalf("expected a validated sequence of %d, got %d", maxAdmissionSequence, sequenceLength)
	}

	// One more would exceed it: the signer is refused without a sequence check.
	sequenceLength = 0
	err := node.admitMempoolTx(context.Background(), scripted, pool, nonceTx("alice", maxAdmissionSequence+1))
	if !errors.Is(err, ErrAdmissionSequenceTooLong) {
		t.Fatalf("expected the overlong sequence to be refused, got %v", err)
	}
	if sequenceLength != 0 {
		t.Fatalf("the overlong sequence must not reach the batch check, it validated %d transactions", sequenceLength)
	}
	if got := pool.Len(); got != maxAdmissionSequence {
		t.Fatalf("expected %d pooled transactions, got %d", maxAdmissionSequence, got)
	}
}

func TestAdmissionKeepsTheRejectionOfATransactionWithoutASequence(t *testing.T) {
	node, scripted := newScriptedAdmissionNode()
	pool := newAdmissionPool(t)
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse { return rejectTx }
	scripted.batch = func(context.Context, []types.Tx) app.CheckTxResponse {
		t.Error("a transaction without signer and nonce has no sequence to validate")
		return acceptTx
	}

	err := node.admitMempoolTx(context.Background(), scripted, pool, types.Tx("bank:send:alice:bob:1:fee=1"))
	var rejected appCheckError
	if !errors.As(err, &rejected) || rejected.Error() != rejectTx.Result.Log {
		t.Fatalf("expected the application's rejection, got %v", err)
	}
	if got := pool.Len(); got != 0 {
		t.Fatalf("expected an empty pool, got %d", got)
	}
}

func TestGossipedTxIsNotScoredWhenAdmissionKeepsRacingCommits(t *testing.T) {
	cfg := UnsafeTestConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	cfg.Chain.P2P.InitialScore = 10
	cfg.Chain.P2P.ValidMessageReward = 1
	cfg.Chain.P2P.InvalidMessageCost = 3
	cfg.Chain.P2P.BanThreshold = -100
	cfg.Chain.P2P.ScoreRecovery = 0
	node, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithSigner(deterministicSignerForID("alice"))
	startNode(t, node)
	defer node.Stop(context.Background())
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}

	scripted := &scriptedAdmissionApp{Application: runtime.App}
	racing := true
	scripted.single = func(context.Context, types.Tx) app.CheckTxResponse {
		if racing {
			scripted.commitBlock()
			return acceptTx
		}
		return rejectTx
	}
	scripted.batch = func(context.Context, []types.Tx) app.CheckTxResponse { return rejectTx }
	runtime.App = scripted

	node.acceptGossipTx(context.Background(), "bob", nonceTx("alice", 1))
	score, err := node.PeerScore(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("local commit churn is no verdict on the peer, got score %d", score)
	}

	racing = false
	node.acceptGossipTx(context.Background(), "bob", nonceTx("alice", 2))
	score, err = node.PeerScore(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	if score != 7 {
		t.Fatalf("a rejected transaction still costs the peer, got score %d", score)
	}
	if got := runtime.Mempool.Len(); got != 0 {
		t.Fatalf("expected no pooled transaction, got %d", got)
	}
}

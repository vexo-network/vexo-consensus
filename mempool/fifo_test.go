package mempool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
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

	batch, err := pool.BuildBatch(context.Background(), 10, nil)
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

func TestFIFOPendingTxsReturnsOrderedCopy(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	if err := pool.AddTx(context.Background(), []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("b")); err != nil {
		t.Fatal(err)
	}

	pending, err := pool.PendingTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pending, []types.Tx{[]byte("a"), []byte("b")}) {
		t.Fatalf("unexpected pending txs: %q", pending)
	}
	pending[0][0] = 'z'

	next, err := pool.PendingTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(next[0]) != "a" {
		t.Fatalf("pending tx snapshot mutated pool state: %q", next)
	}
}

func TestFIFOBuildBatchRespectsMaxBytes(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	for _, tx := range []types.Tx{[]byte("aa"), []byte("bbb"), []byte("c")} {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}

	batch, err := pool.BuildBatch(context.Background(), 5, nil)
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

	batch, err := pool.BuildBatch(context.Background(), 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Txs) != 0 {
		t.Fatalf("expected empty batch, got %q", batch.Txs)
	}
}

func TestFIFOBuildBatchRejectsInvalidMaxBytes(t *testing.T) {
	pool := NewFIFO(FIFOConfig{})
	_, err := pool.BuildBatch(context.Background(), 0, nil)
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
	batch, err := pool.BuildBatch(context.Background(), 10, nil)
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

// A transaction committed before a peer relayed it to the pool is refused when the copy arrives: it
// would otherwise enter a later block again.
func TestFIFORejectsACommittedTxItNeverHeld(t *testing.T) {
	pool := NewFIFO(FIFOConfig{SeenTTL: time.Minute})
	if err := pool.MarkCommitted(context.Background(), []types.Tx{[]byte("late")}); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("late")); !errors.Is(err, ErrDuplicateTx) {
		t.Fatalf("expected the committed tx to be refused, got %v", err)
	}
}

func TestFIFOPendingTxsPrunesReplayRejectedTxs(t *testing.T) {
	pool := NewFIFO(FIFOConfig{
		ReplayCheckTx: func(ctx context.Context, tx types.Tx) error {
			if string(tx) == "bank:send:signer=alice:nonce=0" {
				return ErrDuplicateTx
			}
			return nil
		},
	})
	if err := pool.AddTx(context.Background(), []byte("bank:send:signer=alice:nonce=0")); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("bank:send:signer=alice:nonce=1")); err != nil {
		t.Fatal(err)
	}
	pending, err := pool.PendingTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || string(pending[0]) != "bank:send:signer=alice:nonce=1" {
		t.Fatalf("expected replay rejected tx to be pruned, got %q", pending)
	}
	if pool.Len() != 1 {
		t.Fatalf("expected pool to retain only live txs, got len %d", pool.Len())
	}
}

func TestFIFOPendingTxsRetainsFutureNonce(t *testing.T) {
	pool := NewFIFO(FIFOConfig{
		ReplayCheckTx: func(ctx context.Context, tx types.Tx) error {
			switch string(tx) {
			case "future":
				return ErrTxNotReady
			case "invalid":
				return errors.New("invalid transaction")
			default:
				return nil
			}
		},
	})
	for _, tx := range []types.Tx{[]byte("ready"), []byte("future"), []byte("invalid")} {
		if err := pool.AddTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := pool.PendingTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pending, []types.Tx{[]byte("ready"), []byte("future")}) {
		t.Fatalf("unexpected pending transactions: %q", pending)
	}
}

func TestFIFOSnapshotTxsDoesNotReplayCheckTx(t *testing.T) {
	replayChecks := 0
	pool := NewFIFO(FIFOConfig{
		ReplayCheckTx: func(ctx context.Context, tx types.Tx) error {
			replayChecks++
			return nil
		},
	})
	if err := pool.AddTx(context.Background(), []byte("evm:pending")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := pool.SnapshotTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 || string(snapshot[0]) != "evm:pending" || replayChecks != 0 {
		t.Fatalf("snapshot unexpectedly replayed CheckTx: txs=%q checks=%d", snapshot, replayChecks)
	}
	if _, err := pool.PendingTxs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if replayChecks != 1 {
		t.Fatalf("expected stateful pending query to replay once, got %d", replayChecks)
	}
}

func TestFIFOPrunesExpiredSeenCache(t *testing.T) {
	pool := NewFIFO(FIFOConfig{SeenTTL: time.Minute})
	now := time.Unix(100, 0)
	pool.now = func() time.Time { return now }

	if err := pool.AddTx(context.Background(), []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := pool.MarkCommitted(context.Background(), []types.Tx{[]byte("a")}); err != nil {
		t.Fatal(err)
	}
	if len(pool.seen) != 1 {
		t.Fatalf("expected seen cache entry, got %d", len(pool.seen))
	}
	now = now.Add(time.Minute + time.Second)
	if err := pool.CheckTx(context.Background(), []byte("b")); err != nil {
		t.Fatal(err)
	}
	if len(pool.seen) != 0 || len(pool.seenOrder)-pool.seenHead != 0 {
		t.Fatalf("expected expired seen cache to be pruned, got %d", len(pool.seen))
	}
}

// Admission costs what expiry drops, not a scan of the seen cache: a cache of committed
// transactions made every admission linear in it.
func TestFIFOSeenCacheExpiryIsBoundedAndOldestFirst(t *testing.T) {
	pool := NewFIFO(FIFOConfig{SeenTTL: time.Minute})
	now := time.Unix(100, 0)
	pool.now = func() time.Time { return now }
	committed := make([]types.Tx, 0, MaxSeenTxs+10)
	for index := 0; index < MaxSeenTxs+10; index++ {
		committed = append(committed, types.Tx(fmt.Sprintf("tx-%d", index)))
	}
	if err := pool.MarkCommitted(context.Background(), committed); err != nil {
		t.Fatal(err)
	}
	if err := pool.CheckTx(context.Background(), []byte("probe")); err != nil {
		t.Fatal(err)
	}
	if len(pool.seen) != MaxSeenTxs {
		t.Fatalf("expected the seen cache bounded at %d, got %d", MaxSeenTxs, len(pool.seen))
	}
	if err := pool.AddTx(context.Background(), []byte("tx-0")); err != nil {
		t.Fatalf("expected the oldest record to go first, got %v", err)
	}
	if err := pool.AddTx(context.Background(), []byte(fmt.Sprintf("tx-%d", MaxSeenTxs+9))); !errors.Is(err, ErrDuplicateTx) {
		t.Fatalf("expected the newest record kept, got %v", err)
	}
	// A million live records: an admission that scanned them would take seconds for these.
	started := time.Now()
	for index := 0; index < 2000; index++ {
		if err := pool.CheckTx(context.Background(), types.Tx(fmt.Sprintf("fresh-%d", index))); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("2000 admissions over a full seen cache took %s", elapsed)
	}
	now = now.Add(time.Minute + time.Second)
	if err := pool.CheckTx(context.Background(), []byte("after-expiry")); err != nil {
		t.Fatal(err)
	}
	if len(pool.seen) != 0 {
		t.Fatalf("expected the expired cache dropped, got %d", len(pool.seen))
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

	batch, err := pool.BuildBatch(context.Background(), 1024, nil)
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

func TestFIFOReplacesSameSignerNonceWithBumpedPrice(t *testing.T) {
	pool := NewFIFO(FIFOConfig{MaxTxs: 1, EnableReplacement: true, ReplacementBumpBPS: 1000})
	if err := pool.AddTx(context.Background(), []byte("bank:send:fee=100:priority=1:signer=alice:nonce=7")); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), []byte("bank:send:fee=109:priority=1:signer=alice:nonce=7")); !errors.Is(err, ErrReplacementUnderpriced) {
		t.Fatalf("expected underpriced replacement, got %v", err)
	}
	if err := pool.AddTx(context.Background(), []byte("bank:send:fee=110:priority=1:signer=alice:nonce=7")); err != nil {
		t.Fatalf("expected bumped replacement to pass, got %v", err)
	}
	pending, err := pool.PendingTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || string(pending[0]) != "bank:send:fee=110:priority=1:signer=alice:nonce=7" {
		t.Fatalf("unexpected replacement pending txs: %q", pending)
	}
}

func TestFIFOMarkCommittedRemovesSameSignerNonceReplacement(t *testing.T) {
	pool := NewFIFO(FIFOConfig{EnableReplacement: true})
	local := types.Tx("bank:send:fee=110:signer=alice:nonce=7")
	committed := types.Tx("bank:send:fee=120:signer=alice:nonce=7")
	if err := pool.AddTx(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	if err := pool.MarkCommitted(context.Background(), []types.Tx{committed}); err != nil {
		t.Fatal(err)
	}
	if pool.Len() != 0 {
		t.Fatalf("same-nonce replacement survived commit: %d pending", pool.Len())
	}
}

func TestFIFOReplacementUsesSignedPayloadTags(t *testing.T) {
	pool := NewFIFO(FIFOConfig{EnableReplacement: true})
	first := signedTestMempoolPayload(t, "bank:send:fee=1:priority=1:signer=alice:nonce=9")
	second := signedTestMempoolPayload(t, "bank:send:fee=2:priority=1:signer=alice:nonce=9")
	if key, ok := ReplacementKey(first); !ok || key != "alice/9" {
		t.Fatalf("unexpected signed replacement key %q found=%v", key, ok)
	}
	if err := pool.AddTx(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := pool.AddTx(context.Background(), second); err != nil {
		t.Fatalf("expected signed payload replacement, got %v", err)
	}
	pending, err := pool.PendingTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || !reflect.DeepEqual(pending[0], second) {
		t.Fatalf("unexpected signed replacement pending txs: %q", pending)
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

func signedTestMempoolPayload(t *testing.T, payload string) types.Tx {
	t.Helper()
	document := map[string]string{
		"schema_version": "v1",
		"payload":        base64.StdEncoding.EncodeToString([]byte(payload)),
		"public_key":     base64.StdEncoding.EncodeToString([]byte("public-key")),
		"signature":      base64.StdEncoding.EncodeToString([]byte("signature")),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return types.Tx("signed:" + base64.StdEncoding.EncodeToString(encoded))
}

func TestTxFeeAndPriorityUnwrapSignedTx(t *testing.T) {
	signedTx := signedTestMempoolPayload(t, "bank:send:fee=42:priority=7")
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
	if _, err := pool.BuildBatch(ctx, 1, nil); !errors.Is(err, context.Canceled) {
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

// A full pool evicts what pays least, the newest first among equal fees, across random
// admissions, commits and evictions that keep the eviction heap in step with the pool.
func TestFIFOEvictsInFeeOrderUnderChurn(t *testing.T) {
	ctx := context.Background()
	random := rand.New(rand.NewSource(3))
	pool := NewFIFO(FIFOConfig{MaxTxs: 40, MaxTotalBytes: 40 * 48})
	reference := map[string]uint64{}
	for index := range 2000 {
		fee := uint64(random.Intn(50))
		tx := types.Tx(fmt.Sprintf("bank:send:fee=%d:id=%06d", fee, index))
		lowest := pool.lowestEntry()
		err := pool.AddTx(ctx, tx)
		if len(reference) >= 40 {
			if lowest == nil {
				t.Fatal("a full pool has a lowest entry")
			}
			for _, other := range reference {
				if other < lowest.fee {
					t.Fatalf("lowest entry pays %d, another pays %d", lowest.fee, other)
				}
			}
			if fee <= lowest.fee {
				if !errors.Is(err, ErrMempoolFull) {
					t.Fatalf("tx paying %d over a lowest of %d: expected ErrMempoolFull, got %v", fee, lowest.fee, err)
				}
				continue
			}
			delete(reference, string(lowest.tx))
		}
		if err != nil {
			t.Fatal(err)
		}
		reference[string(tx)] = fee
		if random.Intn(10) == 0 {
			pending, err := pool.SnapshotTxs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			committed := pending[random.Intn(len(pending))]
			if err := pool.MarkCommitted(ctx, []types.Tx{committed}); err != nil {
				t.Fatal(err)
			}
			delete(reference, string(committed))
		}
		if pool.Len() != len(reference) || len(pool.evictions) != len(reference) {
			t.Fatalf("pool holds %d (heap %d), want %d", pool.Len(), len(pool.evictions), len(reference))
		}
		var size int64
		for _, pending := range pool.txs {
			size += pending.size
			if pool.index[pending.hash] != pending || pool.evictions[pending.evictAt] != pending {
				t.Fatalf("entry %q is out of its lookups", pending.tx)
			}
		}
		if len(pool.index) != len(pool.txs) || pool.totalBytes != size {
			t.Fatalf("index holds %d of %d entries, total bytes %d of %d", len(pool.index), len(pool.txs), pool.totalBytes, size)
		}
	}
}

// The eviction heap yields the pending entries in exactly evictsBefore order.
func TestEvictionHeapYieldsEntriesInEvictionOrder(t *testing.T) {
	random := rand.New(rand.NewSource(9))
	pool := NewFIFO(FIFOConfig{})
	for index := range 300 {
		if err := pool.AddTx(context.Background(), types.Tx(fmt.Sprintf("bank:send:fee=%d:id=%d", random.Intn(20), index))); err != nil {
			t.Fatal(err)
		}
	}
	want := append([]*entry(nil), pool.txs...)
	sort.SliceStable(want, func(left, right int) bool { return evictsBefore(want[left], want[right]) })
	index := 0
	for got := range pool.evictions.ascending() {
		if got != want[index] {
			t.Fatalf("position %d: fee %d seq %d, want fee %d seq %d", index, got.fee, got.sequence, want[index].fee, want[index].sequence)
		}
		index++
	}
	if index != len(want) {
		t.Fatalf("yielded %d of %d entries", index, len(want))
	}
}

// A batch checks the transactions it takes, not the whole pool, and skips the transactions of
// proposed blocks that are not committed yet.
func TestBuildBatchChecksOnlyWhatItTakesAndSkipsProposedTxs(t *testing.T) {
	ctx := context.Background()
	checked := 0
	pool := NewFIFO(FIFOConfig{ReplayCheckTx: func(context.Context, types.Tx) error {
		checked++
		return nil
	}})
	var txs []types.Tx
	for index := range 1000 {
		tx := types.Tx(fmt.Sprintf("bank:send:id=%04d", index))
		txs = append(txs, tx)
		if err := pool.AddTx(ctx, tx); err != nil {
			t.Fatal(err)
		}
	}
	proposed := map[types.Hash]struct{}{HashTx(txs[0]): {}, HashTx(txs[1]): {}}
	batch, err := pool.BuildBatch(ctx, int64(10*len(txs[0])), proposed)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Txs) != 10 || string(batch.Txs[0]) != string(txs[2]) || string(batch.Txs[9]) != string(txs[11]) {
		t.Fatalf("expected txs 2..11, got %d txs starting %q", len(batch.Txs), batch.Txs[0])
	}
	if checked != 11 {
		t.Fatalf("expected the batch to check the 10 txs it took and the one that did not fit, got %d checks", checked)
	}
}

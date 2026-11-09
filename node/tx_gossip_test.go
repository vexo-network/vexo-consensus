package node

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestTxBatchRoundTripsAndRefusesMalformedMessages(t *testing.T) {
	txs := []types.Tx{types.Tx("a"), types.Tx("bank:second"), bytes.Repeat([]byte{7}, 300)}
	decoded, err := decodeTxBatch(encodeTxBatch(txs))
	if err != nil || len(decoded) != len(txs) {
		t.Fatalf("decoded %d txs (%v), want %d", len(decoded), err, len(txs))
	}
	for index := range txs {
		if !bytes.Equal(decoded[index], txs[index]) {
			t.Fatalf("tx %d: %q, want %q", index, decoded[index], txs[index])
		}
	}
	valid := encodeTxBatch(txs)
	for name, message := range map[string][]byte{
		"empty":           {},
		"no transactions": {txGossipBatchVersion},
		"another version": append([]byte{9}, valid[1:]...),
		"truncated":       valid[:len(valid)-1],
		"empty tx":        {txGossipBatchVersion, 0},
		"length past end": {txGossipBatchVersion, 5, 'a'},
		"bad length":      {txGossipBatchVersion, 0x80},
	} {
		if _, err := decodeTxBatch(message); !errors.Is(err, ErrInvalidTxBatch) {
			t.Fatalf("%s: expected ErrInvalidTxBatch, got %v", name, err)
		}
	}
	tooMany := make([]types.Tx, txGossipMaxBatchTxs+1)
	for index := range tooMany {
		tooMany[index] = types.Tx("x")
	}
	if _, err := decodeTxBatch(encodeTxBatch(tooMany)); !errors.Is(err, ErrInvalidTxBatch) {
		t.Fatalf("expected a batch over the limit to be refused, got %v", err)
	}
}

// The batcher publishes the transactions admitted together as one message, splits at the byte
// limit, and flushes a partial batch after the delay.
func TestTxGossipBatcherBatchesAndFlushes(t *testing.T) {
	published := make(chan []types.Tx, 8)
	batcher := &txGossipBatcher{publish: func(txs []types.Tx) { published <- txs }}
	for range 3 {
		batcher.add(types.Tx("small"))
	}
	select {
	case txs := <-published:
		if len(txs) != 3 {
			t.Fatalf("expected the three transactions in one message, got %d", len(txs))
		}
	case <-time.After(time.Second):
		t.Fatal("expected the partial batch to be flushed")
	}
	large := bytes.Repeat([]byte{1}, txGossipMaxBatchBytes/2+1)
	batcher.add(large)
	batcher.add(large)
	if txs := <-published; len(txs) != 1 {
		t.Fatalf("expected the batch to be split at the byte limit, got %d txs", len(txs))
	}
	if txs := <-published; len(txs) != 1 {
		t.Fatalf("expected the second large transaction flushed on its own, got %d txs", len(txs))
	}
}

// Under a load far past the per-peer message limit, validators relay every transaction to each
// other without banning one another.
func TestHeavyTxGossipDoesNotBanHonestValidators(t *testing.T) {
	ctx := context.Background()
	alice, bob, carol := newConsensusLoopNodes(t)
	for _, node := range []*Node{alice, bob, carol} {
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	const total = 5000
	for index := range total {
		if err := alice.SubmitTx(ctx, types.Tx(fmt.Sprintf("bank:heavy-%d", index))); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		runtime, err := bob.Runtime()
		if err != nil {
			t.Fatal(err)
		}
		if runtime.Mempool.Len() == total {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("bob holds %d of %d gossiped transactions", runtime.Mempool.Len(), total)
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, node := range []*Node{bob, carol} {
		if status := node.Status(ctx); status.BannedPeers != 0 {
			t.Fatalf("expected no banned peers, got %+v", status.Peers)
		}
	}
}

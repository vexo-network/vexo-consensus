package node

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

// Transaction gossip.
//
// Transactions are gossiped in batches: one message carries every transaction a node admitted in
// the last txGossipFlushDelay, up to txGossipMaxBatchBytes. A message per transaction would make
// the messages a peer sends grow with the load it relays, so the per-peer message limits of peer
// scoring would cap the network's throughput and, past them, ban the honest validators that relay
// it. A peer scores a batch as one message.

const (
	// txGossipFlushDelay is how long an admitted transaction may wait for others to share its message.
	txGossipFlushDelay = 2 * time.Millisecond
	// txGossipMaxBatchBytes bounds the transaction bytes of one gossip message; a larger
	// transaction travels alone.
	txGossipMaxBatchBytes = 1 << 20
	// txGossipMaxBatchTxs bounds the transactions one gossip message may carry.
	txGossipMaxBatchTxs = 8192
	// txGossipBatchVersion is the first byte of a gossip message.
	txGossipBatchVersion byte = 1
)

// ErrInvalidTxBatch reports a gossip message that is not a well-formed batch of transactions.
var ErrInvalidTxBatch = errors.New("invalid transaction gossip batch")

// encodeTxBatch encodes txs as a gossip message: the version byte, then each transaction as its
// uvarint length and its bytes.
func encodeTxBatch(txs []types.Tx) []byte {
	size := 1
	for _, tx := range txs {
		size += binary.MaxVarintLen64 + len(tx)
	}
	encoded := make([]byte, 1, size)
	encoded[0] = txGossipBatchVersion
	for _, tx := range txs {
		encoded = binary.AppendUvarint(encoded, uint64(len(tx)))
		encoded = append(encoded, tx...)
	}
	return encoded
}

// decodeTxBatch decodes a gossip message into its transactions, which share the message's memory.
// It refuses an empty batch, an empty transaction, a batch over the limits, and trailing bytes.
func decodeTxBatch(encoded []byte) ([]types.Tx, error) {
	if len(encoded) < 2 || encoded[0] != txGossipBatchVersion {
		return nil, ErrInvalidTxBatch
	}
	rest := encoded[1:]
	var txs []types.Tx
	for len(rest) > 0 {
		length, read := binary.Uvarint(rest)
		if read <= 0 || length == 0 || length > uint64(len(rest)-read) || len(txs) == txGossipMaxBatchTxs {
			return nil, ErrInvalidTxBatch
		}
		rest = rest[read:]
		txs = append(txs, types.Tx(rest[:length:length]))
		rest = rest[length:]
	}
	return txs, nil
}

// txGossipBatcher collects the transactions a node admitted and publishes them in batches.
type txGossipBatcher struct {
	mu      sync.Mutex
	pending []types.Tx
	bytes   int
	armed   bool
	publish func([]types.Tx)
}

// add queues tx for gossip. The batch is published once it reaches txGossipMaxBatchBytes or
// txGossipMaxBatchTxs, and otherwise txGossipFlushDelay after its first transaction.
func (batcher *txGossipBatcher) add(tx types.Tx) {
	batcher.mu.Lock()
	var full []types.Tx
	if batcher.bytes > 0 && batcher.bytes+len(tx) > txGossipMaxBatchBytes {
		full = batcher.takeLocked()
	}
	batcher.pending = append(batcher.pending, append(types.Tx(nil), tx...))
	batcher.bytes += len(tx)
	var ready []types.Tx
	if batcher.bytes >= txGossipMaxBatchBytes || len(batcher.pending) == txGossipMaxBatchTxs {
		ready = batcher.takeLocked()
	} else if !batcher.armed {
		batcher.armed = true
		time.AfterFunc(txGossipFlushDelay, batcher.flush)
	}
	batcher.mu.Unlock()
	if len(full) > 0 {
		batcher.publish(full)
	}
	if len(ready) > 0 {
		batcher.publish(ready)
	}
}

func (batcher *txGossipBatcher) flush() {
	batcher.mu.Lock()
	batcher.armed = false
	txs := batcher.takeLocked()
	batcher.mu.Unlock()
	if len(txs) > 0 {
		batcher.publish(txs)
	}
}

func (batcher *txGossipBatcher) takeLocked() []types.Tx {
	txs := batcher.pending
	batcher.pending = nil
	batcher.bytes = 0
	return txs
}

// gossipTx queues an admitted transaction for gossip to the peers.
func (node *Node) gossipTx(tx types.Tx) {
	node.txGossipOnce.Do(func() {
		node.txGossip = &txGossipBatcher{publish: node.publishTxBatch}
	})
	node.txGossip.add(tx)
}

func (node *Node) publishTxBatch(txs []types.Tx) {
	wire, ok := node.runningTransport()
	if !ok {
		return
	}
	if err := wire.Publish(context.Background(), p2p.TopicTx, encodeTxBatch(txs)); err != nil {
		node.logEvent("tx_gossip_failed", map[string]any{"error": err.Error(), "txs": len(txs)})
	}
}

// acceptGossipTxBatch admits the transactions of a gossip message from peer. The peer is scored
// once for the message: invalid when the message is malformed or carries a transaction the node
// rejects, and valid when its transactions were admitted or already pending. Transactions only
// local commit churn or the pool's own capacity kept out are no verdict.
func (node *Node) acceptGossipTxBatch(ctx context.Context, from p2p.PeerID, message []byte) {
	if !node.admitPeerMessage(ctx, from) {
		return
	}
	txs, err := decodeTxBatch(message)
	if err != nil {
		node.observePeerMessage(ctx, from, false)
		return
	}
	runtime, err := node.Runtime()
	if err != nil {
		return
	}
	judged, valid, admitted := false, true, false
	for _, tx := range txs {
		if isUnsignedEvidenceTx(tx) {
			// Evidence enters only through the evidence pool, which judges it on its own.
			node.acceptEvidenceMessage(ctx, from, tx)
			continue
		}
		err := node.admitMempoolTx(ctx, runtime.App, runtime.Mempool, tx)
		switch {
		case err == nil:
			judged, admitted = true, true
		case errors.Is(err, mempool.ErrDuplicateTx):
			judged = true
		case errors.Is(err, ErrAdmissionStateChanged), errors.Is(err, mempool.ErrMempoolFull),
			errors.Is(err, mempool.ErrSenderLimit), errors.Is(err, mempool.ErrReplacementUnderpriced):
			// Local churn or capacity: what the pool already holds decided it, not the peer. A
			// full pool must not ban the honest validators that keep relaying to it. The leader may
			// still propose the transaction: it is kept to rebuild compact proposals with.
			node.overflowTxs.add(tx)
		default:
			judged, valid = true, false
		}
	}
	if judged {
		node.observePeerMessage(ctx, from, valid)
	}
	if admitted {
		node.wakeConsensus(context.Background())
	}
}

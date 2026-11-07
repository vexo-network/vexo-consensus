package node

import (
	"bytes"
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/mempool"
	appslashing "github.com/vexo-network/vexo-consensus/modules/slashing"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

// SubmitEvidence verifies evidence against the committed chain state, pools it, queues its
// transaction in the mempool and gossips it. It changes no consensus state: an offence is punished
// only when a block carries the evidence transaction and every node executes it, so the transaction
// has to reach the mempool of a proposer. The returned hash identifies that transaction.
//
// Evidence of the height still being decided cannot be queued yet (see queueEvidenceTx). It is
// pooled and gossiped all the same, and the first commit that reaches its height queues it.
func (node *Node) SubmitEvidence(ctx context.Context, evidence slashing.Evidence) (types.Hash, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return types.Hash{}, err
	}
	admission, err := runtime.AdmitEvidence(ctx, evidence)
	if err != nil {
		return types.Hash{}, err
	}
	queued, queueErr := node.queueEvidenceTx(ctx, runtime, admission.Evidence, admission.Tx)
	// Peers judge the evidence themselves, so it is gossiped whether or not this node could queue it.
	if err := node.broadcastEvidence(ctx, admission.Tx); err != nil {
		return types.Hash{}, errors.Join(queueErr, err)
	}
	if queueErr != nil {
		return types.Hash{}, queueErr
	}
	if queued {
		node.wakeConsensus(context.Background())
	}
	return admission.Hash, nil
}

// queueEvidenceTx queues the transaction of verified evidence in the mempool through the admission
// every transaction passes, so a proposer includes it in a block. queued is false, without an error,
// when the evidence cannot be queued yet or the mempool holds it already.
//
// The mempool judges a transaction against the committed state, and in that state evidence of the
// next height to commit is from the future. Equivocation is detected in the view being voted on,
// which is that height or a later one, so evidence waits in the pool until the commit of its height,
// which queuePooledEvidence follows. The check uses the height the mempool judges at; a commit that
// races it only makes the evidence queueable earlier, never later, because the commit scans the pool
// after it raised the height.
func (node *Node) queueEvidenceTx(ctx context.Context, runtime *vexoruntime.Runtime, evidence slashing.Evidence, tx types.Tx) (queued bool, err error) {
	if committed, ok := node.currentCommitHeight(ctx); ok && evidence.Height > committed {
		return false, nil
	}
	err = node.admitMempoolTx(ctx, runtime.App, runtime.Mempool, tx)
	if errors.Is(err, mempool.ErrDuplicateTx) {
		return false, nil
	}
	return err == nil, err
}

// reconcileEvidence drops pooled evidence that can no longer be applied. A bad record never stops a
// node from starting.
func (node *Node) reconcileEvidence(ctx context.Context, runtime *vexoruntime.Runtime) error {
	_, err := runtime.ReconcileEvidence(ctx)
	return err
}

// queuePooledEvidence queues the transaction of every pooled evidence record that the mempool does
// not hold yet. The pool survives a restart and the mempool does not, and evidence of the height
// being decided waits in the pool for a commit, so without it such evidence would never reach a
// block. It runs when the node starts and after every commit. A record that cannot be queued stays
// pooled and never stops the node. The scan is bounded by the pool, which holds at most
// MaxPendingEvidence records, and is a single index read when the pool is empty.
func (node *Node) queuePooledEvidence(ctx context.Context) {
	runtime, err := node.Runtime()
	if err != nil || runtime.Store == nil {
		return
	}
	keys, err := runtime.Store.EvidenceIndex(ctx)
	if err != nil {
		if !errors.Is(err, store.ErrEvidenceNotFound) && ctx.Err() == nil {
			node.logEvent("evidence_requeue_failed", map[string]any{"error": err.Error()})
		}
		return
	}
	if len(keys) > vexoruntime.MaxPendingEvidence {
		keys = keys[:vexoruntime.MaxPendingEvidence]
	}
	held, err := pendingEvidenceTxs(ctx, runtime.Mempool)
	if err != nil {
		if ctx.Err() == nil {
			node.logEvent("evidence_requeue_failed", map[string]any{"error": err.Error()})
		}
		return
	}
	woke := false
	for _, key := range keys {
		if ctx.Err() != nil {
			break
		}
		record, err := runtime.Store.EvidenceByKey(ctx, key)
		if errors.Is(err, store.ErrEvidenceNotFound) {
			// A block settled the offence while the pool was scanned.
			continue
		}
		var tx types.Tx
		if err == nil {
			tx, err = appslashing.EncodeEvidenceTx(record.Evidence)
		}
		if err == nil {
			if _, found := held[string(tx)]; found {
				continue
			}
			var queued bool
			if queued, err = node.queueEvidenceTx(ctx, runtime, record.Evidence, tx); queued {
				woke = true
			}
		}
		if err != nil && ctx.Err() == nil {
			node.logEvent("evidence_requeue_failed", map[string]any{"key": key, "error": err.Error()})
		}
	}
	if woke {
		node.wakeConsensus(context.Background())
	}
}

// pendingEvidenceTxs returns the evidence transactions the mempool holds, so a scan of the pool
// skips them without checking them again.
func pendingEvidenceTxs(ctx context.Context, pool *mempool.DAG) (map[string]struct{}, error) {
	pending, err := pool.SnapshotTxs(ctx)
	if err != nil {
		return nil, err
	}
	held := make(map[string]struct{})
	for _, tx := range pending {
		if isUnsignedEvidenceTx(tx) {
			held[string(tx)] = struct{}{}
		}
	}
	return held, nil
}

// evidenceTxPrefix starts the payload of an unsigned evidence transaction.
var evidenceTxPrefix = []byte(appslashing.ModuleName + ":" + appslashing.ActionEvidence + ":")

// isUnsignedEvidenceTx reports whether tx is an unsigned evidence transaction, the form that pays no
// fee and that admission authenticates by its evidence alone.
func isUnsignedEvidenceTx(tx types.Tx) bool {
	return !txcodec.IsSigned(tx) && bytes.HasPrefix(tx, evidenceTxPrefix)
}

// maxDeferredEvidence bounds the evidence a node holds back until the pool can judge it. Every item
// is a pair of signatures by a validator of the set, so the list stays short unless a validator
// equivocates in every view; a full list refuses the newest evidence and frees up with every commit.
const maxDeferredEvidence = 64

// handleLocalEvidence submits the equivocation the consensus machine detected. The views being
// voted on run ahead of the executed chain, so the evidence is usually of a height the pool cannot
// judge yet; it is held back and submitted again after every commit. Whatever else keeps the
// evidence out of a block is reported, except the cases in which the evidence is merely old or
// already punished.
func (node *Node) handleLocalEvidence(ctx context.Context, evidence slashing.Evidence) {
	_, err := node.SubmitEvidence(ctx, evidence)
	switch {
	case err == nil:
	case errors.Is(err, appslashing.ErrEvidenceFromFuture):
		node.deferEvidence(evidence)
	case !evidenceErrorIsBenign(err):
		node.logEvent("evidence_submit_failed", map[string]any{"error": err.Error()})
	}
}

// deferEvidence holds evidence back for submitDeferredEvidence.
func (node *Node) deferEvidence(evidence slashing.Evidence) {
	key := store.EvidenceKey(evidence)
	if key == "" {
		return
	}
	node.evidenceMu.Lock()
	defer node.evidenceMu.Unlock()
	if len(node.deferredEvidence) >= maxDeferredEvidence {
		return
	}
	for _, held := range node.deferredEvidence {
		if store.EvidenceKey(held) == key {
			return
		}
	}
	node.deferredEvidence = append(node.deferredEvidence, evidence)
}

// submitDeferredEvidence submits the held back evidence again. What the pool cannot judge yet is
// held back once more.
func (node *Node) submitDeferredEvidence(ctx context.Context) {
	node.evidenceMu.Lock()
	held := node.deferredEvidence
	node.deferredEvidence = nil
	node.evidenceMu.Unlock()
	for _, evidence := range held {
		if ctx.Err() != nil {
			return
		}
		node.handleLocalEvidence(ctx, evidence)
	}
}

// queueEvidence takes up the evidence that waits for the chain to catch up with it: the equivocation
// this node detected ahead of the chain is submitted again, and the pooled evidence the mempool
// does not hold yet is queued. It runs after every commit.
func (node *Node) queueEvidence(ctx context.Context) {
	node.submitDeferredEvidence(ctx)
	node.queuePooledEvidence(ctx)
}

// broadcastEvidence gossips the evidence transaction payload, the one wire form of evidence.
func (node *Node) broadcastEvidence(ctx context.Context, tx types.Tx) error {
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	return wire.Publish(ctx, p2p.TopicEvidence, tx)
}

func (node *Node) startEvidenceGossip(ctx context.Context) error {
	if node.wire == nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	events, err := node.wire.Subscribe(runCtx, p2p.TopicEvidence)
	if err != nil {
		cancel()
		return err
	}
	done := make(chan struct{})
	node.evidenceCancel = cancel
	node.evidenceDone = done
	go node.consumeEvidenceGossip(runCtx, events, done)
	return nil
}

func (node *Node) consumeEvidenceGossip(ctx context.Context, events <-chan transport.Envelope, done chan struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			node.acceptEvidenceMessage(ctx, envelope.From, envelope.Data)
		}
	}
}

// acceptEvidenceMessage pools the evidence a peer gossiped and queues its transaction in the
// mempool. The peer is judged by what it sent: a message that is not verifiable evidence counts
// against it, while evidence that is merely old, already punished, ahead of this node or refused by
// a full pool does not. Whether this node can queue verified evidence says nothing about the peer.
func (node *Node) acceptEvidenceMessage(ctx context.Context, from p2p.PeerID, data []byte) {
	if !node.admitPeerMessage(ctx, from) {
		return
	}
	runtime, err := node.Runtime()
	if err != nil {
		return
	}
	admission, err := runtime.AdmitEvidenceTx(ctx, types.Tx(data))
	if err != nil {
		node.observePeerMessage(ctx, from, evidenceErrorIsBenign(err))
		return
	}
	node.observePeerMessage(ctx, from, true)
	// The canonical transaction is queued, not the received bytes, so one offence has one wire form.
	queued, err := node.queueEvidenceTx(ctx, runtime, admission.Evidence, admission.Tx)
	if err != nil {
		node.logEvent("evidence_queue_failed", map[string]any{"error": err.Error()})
		return
	}
	if queued {
		node.wakeConsensus(context.Background())
	}
}

func evidenceErrorIsBenign(err error) bool {
	return errors.Is(err, appslashing.ErrEvidenceAlreadyApplied) ||
		errors.Is(err, appslashing.ErrEvidenceExpired) ||
		errors.Is(err, appslashing.ErrEvidenceFromFuture) ||
		errors.Is(err, appslashing.ErrValidatorSetMissing) ||
		errors.Is(err, vexoruntime.ErrEvidencePoolFull) ||
		errors.Is(err, vexoruntime.ErrEvidenceUnsupported) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

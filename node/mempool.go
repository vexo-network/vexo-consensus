package node

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	appslashing "github.com/vexo-network/vexo-consensus/modules/slashing"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	// maxAdmissionSequence bounds the transactions one admission validates
	// together: the pending transactions of the signer plus the candidate. It
	// caps the work a single rejected transaction can cause.
	maxAdmissionSequence = 64

	// maxAdmissionAttempts bounds how often one admission repeats its checks
	// because a block committed while they ran.
	maxAdmissionAttempts = 3
)

var (
	// ErrAdmissionSequenceTooLong rejects a transaction whose signer already has
	// too many pending transactions to validate together with it.
	ErrAdmissionSequenceTooLong = errors.New("signer has too many pending transactions to admit another")

	// ErrAdmissionStateChanged reports that blocks kept committing while a
	// transaction was checked, so no verdict described the state it would run
	// against. The submitter may retry.
	ErrAdmissionStateChanged = errors.New("committed state changed during transaction admission")
)

func (node *Node) SubmitTx(ctx context.Context, tx types.Tx) error {
	if isUnsignedEvidenceTx(tx) {
		// Evidence enters the mempool only through the evidence pool, as the one transaction of
		// its offence.
		evidence, err := appslashing.DecodeEvidenceTx(tx)
		if err != nil {
			return err
		}
		_, err = node.SubmitEvidence(ctx, evidence)
		return err
	}
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	if err := node.admitMempoolTx(ctx, runtime.App, runtime.Mempool, tx); err != nil {
		return err
	}
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	if err := wire.Publish(ctx, p2p.TopicTx, append([]byte(nil), tx...)); err != nil {
		node.logEvent("tx_gossip_failed", map[string]any{"error": err.Error()})
	}
	node.wakeConsensus(context.Background())
	return nil
}

func (node *Node) PendingTxHashes(ctx context.Context) ([]types.Hash, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	txs, err := runtime.Mempool.PendingTxs(ctx)
	if err != nil {
		return nil, err
	}
	hashes := make([]types.Hash, 0, len(txs))
	for _, tx := range txs {
		hashes = append(hashes, mempool.HashTx(tx))
	}
	return hashes, nil
}

func (node *Node) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	txs, err := runtime.Mempool.PendingTxs(ctx)
	if err != nil {
		return nil, err
	}
	copied := make([]types.Tx, 0, len(txs))
	for _, tx := range txs {
		copied = append(copied, append(types.Tx(nil), tx...))
	}
	return copied, nil
}

func (node *Node) PendingTxSnapshot(ctx context.Context) ([]types.Tx, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Mempool.SnapshotTxs(ctx)
}

func (node *Node) mempoolHasPendingTx(ctx context.Context) bool {
	runtime, err := node.Runtime()
	if err != nil || runtime.Mempool == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	default:
	}
	return runtime.Mempool.Len() > 0
}

func (node *Node) hasPendingProposalAtHeight(height types.Height) bool {
	for _, proposal := range node.pendingProposals() {
		if proposal.Block.Header.Height == height {
			return true
		}
	}
	return false
}

func (node *Node) ProposeFromMempool(ctx context.Context, maxBytes int64) (consensus.Proposal, types.Hash, error) {
	return node.ProposeFromMempoolWithOptions(ctx, maxBytes, ProposalOptions{AllowEmpty: true})
}

type ProposalOptions struct {
	AllowEmpty bool
	ForceEmpty bool
}

func (node *Node) ProposeFromMempoolWithOptions(ctx context.Context, maxBytes int64, options ProposalOptions) (consensus.Proposal, types.Hash, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}

	batch := mempool.Batch{}
	if !options.ForceEmpty {
		var err error
		batch, err = runtime.Mempool.BuildBatch(ctx, maxBytes)
		if err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
	}
	if !options.AllowEmpty && len(batch.Txs) == 0 {
		return consensus.Proposal{}, types.Hash{}, ErrEmptyProposal
	}

	status := machine.Status(ctx)
	height := status.Height
	if height == 0 {
		height = 1
	}
	proposalResponse, err := prepareProposalWithContext(ctx, runtime.App, app.PrepareProposalRequest{
		Height: height,
		Txs:    batch.Txs,
	})
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	if len(batch.Txs) > 0 && len(proposalResponse.Txs) == 0 {
		return consensus.Proposal{}, types.Hash{}, ErrEmptyProposal
	}

	proposal, blockHash, err := node.ProposeBlock(ctx, types.Block{
		Header: types.Header{Height: height},
		Txs:    proposalResponse.Txs,
	})
	return proposal, blockHash, err
}

type appCheckError string

func (err appCheckError) Error() string {
	if err == "" {
		return "transaction rejected"
	}
	return string(err)
}

func checkTxWithContext(ctx context.Context, application app.Application, tx types.Tx) app.CheckTxResponse {
	if checker, ok := application.(app.ContextCheckTxApplication); ok {
		return checker.CheckTxContext(ctx, tx)
	}
	select {
	case <-ctx.Done():
		return app.CheckTxResponse{Result: types.Result{Code: 1, Log: ctx.Err().Error()}}
	default:
	}
	return application.CheckTx(tx)
}

func prepareProposalWithContext(ctx context.Context, application app.Application, req app.PrepareProposalRequest) (app.PrepareProposalResponse, error) {
	if preparer, ok := application.(app.ContextPrepareProposalApplication); ok {
		return preparer.PrepareProposalContext(ctx, req)
	}
	select {
	case <-ctx.Done():
		return app.PrepareProposalResponse{}, ctx.Err()
	default:
	}
	return application.PrepareProposal(req)
}

func (node *Node) startTxGossip(ctx context.Context) error {
	if node.wire == nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	events, err := node.wire.Subscribe(runCtx, p2p.TopicTx)
	if err != nil {
		cancel()
		return err
	}
	done := make(chan struct{})
	node.txCancel = cancel
	node.txDone = done
	go node.consumeTxGossip(runCtx, events, done)
	return nil
}

func (node *Node) consumeTxGossip(ctx context.Context, events <-chan transport.Envelope, done chan struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			node.acceptGossipTx(ctx, envelope.From, envelope.Data)
		}
	}
}

func (node *Node) acceptGossipTx(ctx context.Context, from p2p.PeerID, tx types.Tx) {
	if isUnsignedEvidenceTx(tx) {
		node.acceptEvidenceMessage(ctx, from, tx)
		return
	}
	if !node.admitPeerMessage(ctx, from) {
		return
	}
	runtime, err := node.Runtime()
	if err != nil {
		return
	}
	if err := node.admitMempoolTx(ctx, runtime.App, runtime.Mempool, tx); err != nil {
		if errors.Is(err, mempool.ErrDuplicateTx) {
			node.observePeerMessage(ctx, from, true)
			return
		}
		if errors.Is(err, ErrAdmissionStateChanged) {
			// Local churn, not a verdict on the peer or its transaction.
			return
		}
		node.observePeerMessage(ctx, from, false)
		return
	}
	node.observePeerMessage(ctx, from, true)
	node.wakeConsensus(context.Background())
}

// admitMempoolTx checks tx against the committed state and adds it to the pool.
//
// The checks can be expensive: a transaction the application rejects on its own
// is re-checked together with the pending transactions of its signer, which
// scans the pool and validates the whole sequence. They run without
// admissionMu, which commitBlock holds while it executes a block, so a slow
// admission never stalls a commit. Only the final step holds the lock: it
// re-checks that no block committed since the checks started, because a verdict
// about an older state must not put a transaction into the pool that the commit
// just removed or invalidated, and adds the transaction. If a block did commit,
// the checks run again against the new state.
func (node *Node) admitMempoolTx(ctx context.Context, application app.Application, pool *mempool.DAG, tx types.Tx) error {
	for attempt := 0; attempt < maxAdmissionAttempts; attempt++ {
		checkedAt, _ := node.currentCommitHeight(ctx)
		if err := checkAdmission(ctx, application, pool, tx); err != nil {
			return err
		}
		added, err := node.addAdmittedTx(ctx, pool, tx, checkedAt)
		if added || err != nil {
			return err
		}
	}
	return ErrAdmissionStateChanged
}

// addAdmittedTx adds tx to the pool under admissionMu, provided the committed
// height is still checkedAt, the one the checks read. added reports whether it
// was; the error is then the pool's verdict on tx.
func (node *Node) addAdmittedTx(ctx context.Context, pool *mempool.DAG, tx types.Tx, checkedAt types.Height) (added bool, err error) {
	node.admissionMu.Lock()
	defer node.admissionMu.Unlock()
	if committed, _ := node.currentCommitHeight(ctx); committed != checkedAt {
		return false, nil
	}
	return true, pool.AddTx(ctx, tx)
}

// checkAdmission decides whether tx may enter the pool. A transaction the
// application accepts on its own is admitted. One it rejects can still be
// executable behind the pending transactions of its signer (a nonce gap, fees
// that are only affordable in sequence), so it is validated together with
// them.
func checkAdmission(ctx context.Context, application app.Application, pool *mempool.DAG, tx types.Tx) error {
	response := checkTxWithContext(ctx, application, tx)
	if response.Result.Code == 0 {
		return nil
	}
	rejection := appCheckError(response.Result.Log)
	checker, ok := application.(app.ContextCheckTxsApplication)
	if !ok {
		return rejection
	}
	candidate, ok := slotOf(tx)
	if !ok || !candidate.hasNonce {
		return rejection
	}
	sequence, err := signerSequence(ctx, pool, candidate, tx)
	if err != nil {
		return err
	}
	if batch := checker.CheckTxsContext(ctx, sequence); batch.Result.Code != 0 {
		return appCheckError(batch.Result.Log)
	}
	return nil
}

// txSlot is the account sequence position a transaction occupies. A transaction
// without a nonce still belongs to its signer, whose balance it spends, but
// occupies no nonce.
type txSlot struct {
	signer   string
	nonce    uint64
	hasNonce bool
}

// slotOf parses the signer and nonce of tx; ok is false when tx does not parse
// or names no signer, so it belongs to no account sequence.
func slotOf(tx types.Tx) (txSlot, bool) {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil || meta.Signer == "" {
		return txSlot{}, false
	}
	return txSlot{signer: txcodec.NormalizeAddress(meta.Signer), nonce: meta.Nonce, hasNonce: meta.HasNonce}, true
}

// signerSequence returns the transactions that must be admissible together for
// tx to be executable: the pending transactions of its signer other than the
// one tx replaces, and tx itself. Transactions without a nonce come first in
// pool order, then the nonced ones in nonce order. It fails with
// ErrAdmissionSequenceTooLong when that sequence exceeds maxAdmissionSequence.
func signerSequence(ctx context.Context, pool *mempool.DAG, candidate txSlot, tx types.Tx) ([]types.Tx, error) {
	pending, err := pool.SnapshotTxs(ctx)
	if err != nil {
		return nil, err
	}
	type sequenced struct {
		tx   types.Tx
		slot txSlot
	}
	sequence := []sequenced{{tx: append(types.Tx(nil), tx...), slot: candidate}}
	for _, pendingTx := range pending {
		slot, ok := slotOf(pendingTx)
		if !ok || slot.signer != candidate.signer || (slot.hasNonce && slot.nonce == candidate.nonce) {
			continue
		}
		if len(sequence) >= maxAdmissionSequence {
			return nil, ErrAdmissionSequenceTooLong
		}
		sequence = append(sequence, sequenced{tx: pendingTx, slot: slot})
	}
	sort.SliceStable(sequence, func(left, right int) bool {
		first, second := sequence[left].slot, sequence[right].slot
		if first.hasNonce != second.hasNonce {
			return !first.hasNonce
		}
		return first.nonce < second.nonce
	})
	txs := make([]types.Tx, 0, len(sequence))
	for _, item := range sequence {
		txs = append(txs, item.tx)
	}
	return txs, nil
}

func (node *Node) runningTransport() (transport.Transport, bool) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if !node.running || node.wire == nil {
		return nil, false
	}
	return node.wire, true
}

func (node *Node) wakeConsensus(ctx context.Context) {
	node.mu.Lock()
	wake := node.loopWake
	node.mu.Unlock()
	if wake == nil {
		return
	}
	select {
	case <-ctx.Done():
	case wake <- struct{}{}:
	default:
	}
}

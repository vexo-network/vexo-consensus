package node

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

func (node *Node) SubmitTx(ctx context.Context, tx types.Tx) error {
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
		node.observePeerMessage(ctx, from, false)
		return
	}
	node.observePeerMessage(ctx, from, true)
	node.wakeConsensus(context.Background())
}

func (node *Node) admitMempoolTx(ctx context.Context, application app.Application, pool *mempool.DAG, tx types.Tx) error {
	node.admissionMu.Lock()
	defer node.admissionMu.Unlock()

	response := checkTxWithContext(ctx, application, tx)
	if response.Result.Code != 0 {
		checker, ok := application.(app.ContextCheckTxsApplication)
		if !ok {
			return appCheckError(response.Result.Log)
		}
		candidateSigner, signerFound := mempool.TxSigner(tx)
		_, nonceFound := mempool.TxNonce(tx)
		if !signerFound || !nonceFound {
			return appCheckError(response.Result.Log)
		}
		pending, err := pool.SnapshotTxs(ctx)
		if err != nil {
			return err
		}
		candidateKey, candidateHasKey := mempool.ReplacementKey(tx)
		sequence := make([]types.Tx, 0, len(pending)+1)
		for _, pendingTx := range pending {
			pendingSigner, found := mempool.TxSigner(pendingTx)
			if !found || pendingSigner != candidateSigner {
				continue
			}
			if candidateHasKey {
				if pendingKey, found := mempool.ReplacementKey(pendingTx); found && pendingKey == candidateKey {
					continue
				}
			}
			sequence = append(sequence, pendingTx)
		}
		sequence = append(sequence, append(types.Tx(nil), tx...))
		sort.SliceStable(sequence, func(left, right int) bool {
			leftNonce, leftFound := mempool.TxNonce(sequence[left])
			rightNonce, rightFound := mempool.TxNonce(sequence[right])
			if leftFound && rightFound && leftNonce != rightNonce {
				return leftNonce < rightNonce
			}
			return left < right
		})
		if batchResponse := checker.CheckTxsContext(ctx, sequence); batchResponse.Result.Code != 0 {
			return appCheckError(batchResponse.Result.Log)
		}
	}
	return pool.AddTx(ctx, tx)
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

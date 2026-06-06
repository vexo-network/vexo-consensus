package node

import (
	"context"
	"errors"

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
	if response := runtime.App.CheckTx(tx); response.Result.Code != 0 {
		return appCheckError(response.Result.Log)
	}
	if err := runtime.Mempool.AddTx(ctx, tx); err != nil {
		return err
	}
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	if err := wire.Publish(ctx, p2p.TopicTx, append([]byte(nil), tx...)); err != nil {
		return err
	}
	node.wakeConsensus(ctx)
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

func (node *Node) ProposeFromMempool(ctx context.Context, maxBytes int64) (consensus.Proposal, types.Hash, error) {
	return node.ProposeFromMempoolWithOptions(ctx, maxBytes, ProposalOptions{AllowEmpty: true})
}

type ProposalOptions struct {
	AllowEmpty bool
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

	batch, err := runtime.Mempool.BuildBatch(ctx, maxBytes)
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	if !options.AllowEmpty && len(batch.Txs) == 0 {
		return consensus.Proposal{}, types.Hash{}, ErrEmptyProposal
	}

	status := machine.Status(ctx)
	height := status.Height
	if height == 0 {
		height = 1
	}
	proposalResponse, err := runtime.App.PrepareProposal(app.PrepareProposalRequest{
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
	if response := runtime.App.CheckTx(tx); response.Result.Code != 0 {
		node.observePeerMessage(ctx, from, false)
		return
	}
	if err := runtime.Mempool.AddTx(ctx, tx); err != nil {
		if errors.Is(err, mempool.ErrDuplicateTx) {
			node.observePeerMessage(ctx, from, true)
			return
		}
		node.observePeerMessage(ctx, from, false)
		return
	}
	node.observePeerMessage(ctx, from, true)
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
	if !node.ConsensusLoopRunning() {
		return
	}
	node.mu.Lock()
	cfg := node.loopConfig
	node.mu.Unlock()
	_, _ = node.StepConsensusWithConfig(ctx, cfg)
}

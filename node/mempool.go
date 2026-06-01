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
	return wire.Publish(ctx, p2p.TopicTx, append([]byte(nil), tx...))
}

func (node *Node) ProposeFromMempool(ctx context.Context, maxBytes int64) (consensus.Proposal, types.Hash, error) {
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
	if len(batch.Txs) == 0 {
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
	if len(proposalResponse.Txs) == 0 {
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
	node.txCancel = cancel
	go node.consumeTxGossip(runCtx, events)
	return nil
}

func (node *Node) consumeTxGossip(ctx context.Context, events <-chan transport.Envelope) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			node.acceptGossipTx(ctx, envelope.Data)
		}
	}
}

func (node *Node) acceptGossipTx(ctx context.Context, tx types.Tx) {
	runtime, err := node.Runtime()
	if err != nil {
		return
	}
	if response := runtime.App.CheckTx(tx); response.Result.Code != 0 {
		return
	}
	if err := runtime.Mempool.AddTx(ctx, tx); err != nil && !errors.Is(err, mempool.ErrDuplicateTx) {
		return
	}
}

func (node *Node) runningTransport() (transport.Transport, bool) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if !node.running || node.wire == nil {
		return nil, false
	}
	return node.wire, true
}

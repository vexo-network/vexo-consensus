package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
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
	return runtime.Mempool.AddTx(ctx, tx)
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

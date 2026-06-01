package node

import (
	"context"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

type autoVoteReactor struct {
	machine            *consensus.StateMachine
	validatorID        types.ValidatorID
	broadcastVote      func(context.Context, consensus.Vote) error
	onProposalAccepted func(consensus.Proposal, types.Hash)
}

func (reactor *autoVoteReactor) OnProposal(ctx context.Context, proposal consensus.Proposal) error {
	if err := reactor.machine.OnProposal(ctx, proposal); err != nil {
		return err
	}
	blockHash := consensus.HashBlock(proposal.Block)
	if reactor.onProposalAccepted != nil {
		reactor.onProposalAccepted(proposal, blockHash)
	}
	vote := consensus.Vote{
		Height:      proposal.Block.Header.Height,
		Round:       proposal.Round,
		BlockHash:   blockHash,
		ValidatorID: reactor.validatorID,
	}
	if err := reactor.machine.OnVote(ctx, vote); err != nil {
		return err
	}
	if reactor.broadcastVote == nil {
		return nil
	}
	return reactor.broadcastVote(ctx, vote)
}

func (reactor *autoVoteReactor) OnVote(ctx context.Context, vote consensus.Vote) error {
	return reactor.machine.OnVote(ctx, vote)
}

func (reactor *autoVoteReactor) OnTimeoutVote(ctx context.Context, vote consensus.TimeoutVote) (finality.TimeoutCert, error) {
	return reactor.machine.OnTimeoutVote(ctx, vote)
}

func (node *Node) ProposeBlock(ctx context.Context, block types.Block) (consensus.Proposal, types.Hash, error) {
	if node.cfg.ValidatorID == "" {
		return consensus.Proposal{}, types.Hash{}, ErrMissingValidatorID
	}
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	reactor, err := node.ConsensusReactor()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, ErrConsensusOffline
	}

	status := machine.Status(ctx)
	height := block.Header.Height
	if height == 0 {
		height = status.Height
	}
	if height == 0 {
		height = 1
	}
	block.Header.Height = height
	round := status.Round
	if status.Height != height {
		round = 0
	}

	proposal, err := machine.CreateProposal(block, round, node.cfg.ValidatorID, finality.QuorumCert{})
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	if err := machine.OnProposal(ctx, proposal); err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	blockHash := consensus.HashBlock(proposal.Block)
	node.cacheProposal(proposal, blockHash)
	if err := reactor.BroadcastProposal(ctx, proposal); err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	return proposal, blockHash, nil
}

func (node *Node) VoteBlock(ctx context.Context, height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, bool, error) {
	if node.cfg.ValidatorID == "" {
		return finality.QuorumCert{}, false, ErrMissingValidatorID
	}
	machine, err := node.Consensus()
	if err != nil {
		return finality.QuorumCert{}, false, err
	}
	reactor, err := node.ConsensusReactor()
	if err != nil {
		return finality.QuorumCert{}, false, ErrConsensusOffline
	}

	vote := consensus.Vote{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		ValidatorID: node.cfg.ValidatorID,
	}
	if err := machine.OnVote(ctx, vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
	if err := reactor.BroadcastVote(ctx, vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
	qc, err := machine.BuildQuorumCert(height, round, blockHash)
	if err != nil {
		return finality.QuorumCert{}, false, nil
	}
	return qc, true, nil
}

func (node *Node) TimeoutRound(ctx context.Context) (finality.TimeoutCert, bool, error) {
	if node.cfg.ValidatorID == "" {
		return finality.TimeoutCert{}, false, ErrMissingValidatorID
	}
	machine, err := node.Consensus()
	if err != nil {
		return finality.TimeoutCert{}, false, err
	}
	reactor, err := node.ConsensusReactor()
	if err != nil {
		return finality.TimeoutCert{}, false, ErrConsensusOffline
	}

	status := machine.Status(ctx)
	if status.Height == 0 {
		machine.StartRound(1, status.Round)
		status = machine.Status(ctx)
	}
	vote := consensus.TimeoutVote{
		Height:      status.Height,
		Round:       status.Round,
		ValidatorID: node.cfg.ValidatorID,
	}
	timeoutCert, err := machine.OnTimeoutVote(ctx, vote)
	if err != nil && !errors.Is(err, consensus.ErrNoQuorum) {
		return finality.TimeoutCert{}, false, err
	}
	if err := reactor.BroadcastTimeoutVote(ctx, vote); err != nil {
		return finality.TimeoutCert{}, false, err
	}
	if err != nil {
		return finality.TimeoutCert{}, false, nil
	}
	return timeoutCert, true, nil
}

func (node *Node) CommitBlock(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) (app.FinalizeBlockResponse, error) {
	return node.commitBlock(ctx, block, quorumCert, true, true)
}

func (node *Node) commitBlock(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, requireLocalQC bool, broadcast bool) (app.FinalizeBlockResponse, error) {
	if quorumCert.Height != block.Header.Height {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: height mismatch", ErrInvalidCommitQC)
	}
	blockHash := consensus.HashBlock(block)
	if quorumCert.BlockHash != blockHash {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: block hash mismatch", ErrInvalidCommitQC)
	}

	runtime, err := node.Runtime()
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if requireLocalQC {
		if _, err := machine.BuildQuorumCert(quorumCert.Height, quorumCert.Round, quorumCert.BlockHash); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	} else if err := node.verifyCommitCertificate(ctx, block, quorumCert); err != nil {
		return app.FinalizeBlockResponse{}, err
	}

	response, err := runtime.ExecuteBlock(ctx, block)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if err := runtime.Mempool.MarkCommitted(ctx, block.Txs); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	nextHeight := block.Header.Height + 1
	if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, nextHeight); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	machine.StartRound(nextHeight, 0)
	node.removePending(blockHash)
	if broadcast {
		if err := node.broadcastCommit(ctx, block, quorumCert); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	return response, nil
}

type CommitReadyResult struct {
	Block      types.Block
	BlockHash  types.Hash
	QuorumCert finality.QuorumCert
	Response   app.FinalizeBlockResponse
}

func (node *Node) CommitReadyBlock(ctx context.Context) (CommitReadyResult, bool, error) {
	machine, err := node.Consensus()
	if err != nil {
		return CommitReadyResult{}, false, err
	}
	for blockHash, proposal := range node.pendingProposals() {
		qc, err := machine.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash)
		if err != nil {
			continue
		}
		response, err := node.CommitBlock(ctx, proposal.Block, qc)
		if err != nil {
			return CommitReadyResult{}, false, err
		}
		return CommitReadyResult{
			Block:      proposal.Block,
			BlockHash:  blockHash,
			QuorumCert: qc,
			Response:   response,
		}, true, nil
	}
	return CommitReadyResult{}, false, nil
}

func (node *Node) cacheProposal(proposal consensus.Proposal, blockHash types.Hash) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.pending == nil {
		node.pending = make(map[types.Hash]consensus.Proposal)
	}
	node.pending[blockHash] = proposal
}

func (node *Node) removePending(blockHash types.Hash) {
	node.mu.Lock()
	defer node.mu.Unlock()
	delete(node.pending, blockHash)
}

func (node *Node) pendingProposals() map[types.Hash]consensus.Proposal {
	node.mu.Lock()
	defer node.mu.Unlock()
	proposals := make(map[types.Hash]consensus.Proposal, len(node.pending))
	for blockHash, proposal := range node.pending {
		proposals[blockHash] = proposal
	}
	return proposals
}

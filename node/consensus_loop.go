package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

type autoVoteReactor struct {
	machine       *consensus.StateMachine
	validatorID   types.ValidatorID
	broadcastVote func(context.Context, consensus.Vote) error
}

func (reactor *autoVoteReactor) OnProposal(ctx context.Context, proposal consensus.Proposal) error {
	if err := reactor.machine.OnProposal(ctx, proposal); err != nil {
		return err
	}
	vote := consensus.Vote{
		Height:      proposal.Block.Header.Height,
		Round:       proposal.Round,
		BlockHash:   consensus.HashBlock(proposal.Block),
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

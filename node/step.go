package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/types"
)

type ConsensusStepResult struct {
	Committed bool
	Commit    CommitReadyResult
	Proposed  bool
	Proposal  consensus.Proposal
	BlockHash types.Hash
}

func (node *Node) StepConsensus(ctx context.Context, maxBytes int64) (ConsensusStepResult, error) {
	return node.StepConsensusWithConfig(ctx, ConsensusLoopConfig{
		MaxBlockBytes:     maxBytes,
		CreateEmptyBlocks: true,
	})
}

func (node *Node) StepConsensusWithConfig(ctx context.Context, cfg ConsensusLoopConfig) (ConsensusStepResult, error) {
	cfg = normalizeConsensusLoopConfig(cfg)
	commit, committed, err := node.commitCandidateForMode(ctx, cfg.ExecutionCommitMode)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	if committed {
		return ConsensusStepResult{
			Committed: true,
			Commit:    commit,
		}, nil
	}

	proposal, blockHash, proposed, err := node.TickConsensusWithConfig(ctx, cfg)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	return ConsensusStepResult{
		Proposed:  proposed,
		Proposal:  proposal,
		BlockHash: blockHash,
	}, nil
}

func (node *Node) commitCandidateForMode(ctx context.Context, mode ExecutionCommitMode) (CommitReadyResult, bool, error) {
	switch mode {
	case ExecutionCommitModeQC:
		return node.CommitReadyBlock(ctx)
	case "", ExecutionCommitModeFinalized:
		return node.CommitFinalizedBlock(ctx)
	default:
		return CommitReadyResult{}, false, ErrInvalidLoopConfig
	}
}

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
	node.stepMu.Lock()
	defer node.stepMu.Unlock()

	cfg = normalizeConsensusLoopConfig(cfg)
	commit, committed, err := node.commitCandidateForMode(ctx, cfg.ExecutionCommitMode, cfg.AllowUnsafeQCCommit)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	if committed {
		return ConsensusStepResult{
			Committed: true,
			Commit:    commit,
		}, nil
	}

	proposalOptions := ProposalOptions{AllowEmpty: cfg.CreateEmptyBlocks}
	if node.needsFinalityProgress(ctx, cfg.ExecutionCommitMode) {
		if err := node.advanceFinalityRound(ctx); err != nil {
			return ConsensusStepResult{}, err
		}
		proposalOptions.AllowEmpty = true
		proposalOptions.ForceEmpty = true
	}

	proposal, blockHash, proposed, err := node.tickConsensusWithProposalOptions(ctx, cfg, proposalOptions)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	return ConsensusStepResult{
		Proposed:  proposed,
		Proposal:  proposal,
		BlockHash: blockHash,
	}, nil
}

func (node *Node) commitCandidateForMode(ctx context.Context, mode ExecutionCommitMode, allowUnsafeQC bool) (CommitReadyResult, bool, error) {
	switch mode {
	case ExecutionCommitModeQC:
		if !allowUnsafeQC {
			return CommitReadyResult{}, false, ErrInvalidLoopConfig
		}
		return node.CommitReadyBlock(ctx)
	case "", ExecutionCommitModeFinalized:
		return node.CommitFinalizedBlock(ctx)
	default:
		return CommitReadyResult{}, false, ErrInvalidLoopConfig
	}
}

func (node *Node) needsFinalityProgress(ctx context.Context, mode ExecutionCommitMode) bool {
	if mode != "" && mode != ExecutionCommitModeFinalized {
		return false
	}
	for _, proposal := range node.pendingProposals() {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		if len(proposal.Block.Txs) > 0 {
			return true
		}
	}
	return false
}

func (node *Node) advanceFinalityRound(ctx context.Context) error {
	machine, err := node.Consensus()
	if err != nil {
		return err
	}
	highQC := machine.HighQC(ctx)
	if highQC.Height == 0 {
		return nil
	}
	status := machine.Status(ctx)
	if status.Height == 0 || highQC.Height >= status.Height {
		machine.StartRound(highQC.Height+1, 0)
	}
	return nil
}

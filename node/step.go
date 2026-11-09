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
		CreateEmptyBlocks: false,
	})
}

func (node *Node) StepConsensusWithConfig(ctx context.Context, cfg ConsensusLoopConfig) (ConsensusStepResult, error) {
	node.stepMu.Lock()
	defer node.stepMu.Unlock()

	cfg = normalizeConsensusLoopConfig(cfg)
	// A commit that failed after its block became durable is finished before anything else happens.
	if err := node.reconcileCommitted(ctx); err != nil {
		return ConsensusStepResult{}, err
	}
	commit, committed, err := node.commitCandidateForMode(ctx, cfg)
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
	needsFinality := node.needsFinalityProgress(ctx, cfg.ExecutionCommitMode)
	if needsFinality || node.mempoolHasPendingTx(ctx) {
		if err := node.advanceFinalityRound(ctx); err != nil {
			return ConsensusStepResult{}, err
		}
	}
	if needsFinality {
		// A proposed block with transactions is final only once a child is certified on top of
		// it. The child carries the next transactions, or none when there are none to carry.
		proposalOptions.AllowEmpty = true
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

func (node *Node) commitCandidateForMode(ctx context.Context, cfg ConsensusLoopConfig) (CommitReadyResult, bool, error) {
	switch cfg.ExecutionCommitMode {
	case ExecutionCommitModeQC:
		if !cfg.AllowUnsafeQCCommit {
			return CommitReadyResult{}, false, ErrInvalidLoopConfig
		}
		return node.UnsafeCommitReadyBlock(ctx)
	case "", ExecutionCommitModeFinalized:
		return node.commitFinalizedBlock(ctx, cfg.RecoveryFinalityGateEnabled)
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
		committed, _ := node.currentCommitHeight(ctx)
		machine.StartRound(boundedNextHeight(committed, highQC.Height), 0)
	}
	return nil
}

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
	if err := checkBlockBytes(cfg); err != nil {
		return ConsensusStepResult{}, err
	}
	if err := checkExecutionCommitMode(cfg); err != nil {
		return ConsensusStepResult{}, err
	}
	// A commit that failed after its block became durable is finished before anything else happens.
	if err := node.reconcileCommitted(ctx); err != nil {
		return ConsensusStepResult{}, err
	}
	// The leader of the view proposes before it executes a finalized block: a block names the state
	// of an older height (finality.ChildStateHeight), so the proposal never waits for the execution,
	// which then overlaps the view.
	proposal, blockHash, proposed, err := node.proposeFirst(ctx, cfg)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	commit, committed, err := node.commitCandidateForMode(ctx, cfg)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	if committed || proposed {
		return ConsensusStepResult{
			Committed: committed,
			Commit:    commit,
			Proposed:  proposed,
			Proposal:  proposal,
			BlockHash: blockHash,
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

	proposal, blockHash, proposed, err = node.tickConsensusWithProposalOptions(ctx, cfg, proposalOptions)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	return ConsensusStepResult{
		Proposed:  proposed,
		Proposal:  proposal,
		BlockHash: blockHash,
	}, nil
}

// checkExecutionCommitMode refuses an unknown execution boundary, and the QC boundary without its
// unsafe opt-in.
func checkExecutionCommitMode(cfg ConsensusLoopConfig) error {
	switch cfg.ExecutionCommitMode {
	case ExecutionCommitModeQC:
		if !cfg.AllowUnsafeQCCommit {
			return ErrInvalidLoopConfig
		}
		return nil
	case "", ExecutionCommitModeFinalized:
		return nil
	default:
		return ErrInvalidLoopConfig
	}
}

func (node *Node) commitCandidateForMode(ctx context.Context, cfg ConsensusLoopConfig) (CommitReadyResult, bool, error) {
	if err := checkExecutionCommitMode(cfg); err != nil {
		return CommitReadyResult{}, false, err
	}
	if cfg.ExecutionCommitMode == ExecutionCommitModeQC {
		return node.UnsafeCommitReadyBlock(ctx)
	}
	return node.commitFinalizedBlock(ctx, cfg.RecoveryFinalityGateEnabled)
}

func (node *Node) needsFinalityProgress(ctx context.Context, mode ExecutionCommitMode) bool {
	if mode != "" && mode != ExecutionCommitModeFinalized {
		return false
	}
	// A block at or below the highest decided height is final: it waits for its execution only.
	var decided types.Height
	if machine, err := node.Consensus(); err == nil {
		for _, decision := range machine.CommitDecisions() {
			decided = max(decided, decision.CommittedHeight)
		}
	}
	for _, proposal := range node.pendingProposals() {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		if len(proposal.Block.Txs) > 0 && proposal.Block.Header.Height > decided {
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

// proposeFirst proposes for the current view when the node leads it and has a block to propose:
// transactions, or a block the chain needs to finalize its tip.
func (node *Node) proposeFirst(ctx context.Context, cfg ConsensusLoopConfig) (consensus.Proposal, types.Hash, bool, error) {
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	status := machine.Status(ctx)
	height := max(status.Height, 1)
	if node.hasProposed(height, status.Round) {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	if leads, err := node.IsProposer(ctx, status.Round); err != nil || !leads {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	options := ProposalOptions{AllowEmpty: cfg.CreateEmptyBlocks || node.needsFinalityProgress(ctx, cfg.ExecutionCommitMode)}
	return node.tickConsensusWithProposalOptions(ctx, cfg, options)
}

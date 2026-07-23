package node

import (
	"context"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func SelectProposer(validators []validator.Validator, height types.Height, round types.Round) (types.ValidatorID, error) {
	proposer, ok := validator.SelectProposer(validators, height, round)
	if !ok {
		return "", ErrMissingValidators
	}
	return proposer, nil
}

func ProposerSchedule(validators []validator.Validator, height types.Height, rounds uint64) ([]types.ValidatorID, error) {
	schedule := make([]types.ValidatorID, 0, rounds)
	for round := uint64(0); round < rounds; round++ {
		proposer, err := SelectProposer(validators, height, types.Round(round))
		if err != nil {
			return nil, err
		}
		schedule = append(schedule, proposer)
	}
	return schedule, nil
}

func (node *Node) Proposer(ctx context.Context, height types.Height, round types.Round) (types.ValidatorID, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return "", err
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return "", err
	}
	return SelectProposer(validatorSet.List(), height, round)
}

func (node *Node) IsProposer(ctx context.Context, height types.Height, round types.Round) (bool, error) {
	proposer, err := node.Proposer(ctx, height, round)
	if err != nil {
		return false, err
	}
	return proposer == node.cfg.ValidatorID, nil
}

func (node *Node) TickConsensus(ctx context.Context, maxBytes int64) (consensus.Proposal, types.Hash, bool, error) {
	return node.TickConsensusWithConfig(ctx, ConsensusLoopConfig{
		MaxBlockBytes:     maxBytes,
		CreateEmptyBlocks: true,
	})
}

func (node *Node) TickConsensusWithConfig(ctx context.Context, cfg ConsensusLoopConfig) (consensus.Proposal, types.Hash, bool, error) {
	return node.tickConsensusWithProposalOptions(ctx, cfg, ProposalOptions{AllowEmpty: cfg.CreateEmptyBlocks})
}

func (node *Node) tickConsensusWithProposalOptions(ctx context.Context, cfg ConsensusLoopConfig, options ProposalOptions) (consensus.Proposal, types.Hash, bool, error) {
	cfg = normalizeConsensusLoopConfig(cfg)
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	status := machine.Status(ctx)
	height := status.Height
	if height == 0 {
		height = 1
	}
	isProposer, err := node.IsProposer(ctx, height, status.Round)
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	if !isProposer {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	if node.hasProposed(height, status.Round) {
		proposal, _, ok := node.cachedProposalForRound(height, status.Round)
		if !ok {
			return consensus.Proposal{}, types.Hash{}, false, nil
		}
		if !node.cachedProposalStillValid(ctx, proposal) {
			node.removePending(consensus.HashBlock(proposal.Block))
			node.removeProposed(height, status.Round)
		} else {
			reactor, err := node.ConsensusReactor()
			if err != nil {
				return consensus.Proposal{}, types.Hash{}, false, ErrConsensusOffline
			}
			node.broadcastAncestorProposals(reactor, proposal.Block.Header.Height)
			node.broadcastConsensusAsync("proposal_broadcast_failed", map[string]any{
				"height": proposal.Block.Header.Height,
				"round":  proposal.Round,
			}, func(ctx context.Context) error {
				return reactor.BroadcastProposal(ctx, proposal)
			})
			return consensus.Proposal{}, types.Hash{}, false, nil
		}
	}
	alreadyVoted, err := node.hasRecordedLocalVote(height, status.Round)
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	if alreadyVoted {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	proposal, blockHash, err := node.ProposeFromMempoolWithOptions(ctx, cfg.MaxBlockBytes, options)
	if errors.Is(err, ErrEmptyProposal) {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	if errors.Is(err, ErrLocalVoteRecorded) {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	node.markProposed(proposal.Block.Header.Height, proposal.Round)
	return proposal, blockHash, true, nil
}

func (node *Node) cachedProposalStillValid(ctx context.Context, proposal consensus.Proposal) bool {
	if node.hasCommitted(ctx, proposal.Block.Header.Height) {
		return false
	}
	runtime, err := node.Runtime()
	if err != nil {
		return false
	}
	machine, err := node.Consensus()
	if err != nil {
		return false
	}
	status := machine.Status(ctx)
	blockHash := consensus.HashBlock(proposal.Block)
	// A proposal that is certified, or is an ancestor of the certified tip,
	// must remain available until the finalized execution path consumes it.
	// Re-evaluating it against a newer locked QC would incorrectly classify
	// the certified proposal itself as unsafe because its justify QC is its
	// parent QC.
	if machine.IsAncestorOfHighQC(blockHash) {
		return true
	}
	if proposal.Block.Header.Height < status.Height {
		return false
	}
	if proposal.Block.Header.Height == status.Height && proposal.Round < status.Round {
		return false
	}
	if !machine.IsSafeProposal(proposal) {
		return false
	}
	appRuntime, ok := runtime.App.(*app.Runtime)
	if !ok || appRuntime == nil {
		return true
	}
	return appRuntime.ProcessProposalContext(ctx, app.ProcessProposalRequest{Block: proposal.Block}).Accepted
}

func (node *Node) handleRejectedProposal(proposal consensus.Proposal, err error) {
	if err == nil {
		return
	}
	node.removePending(consensus.HashBlock(proposal.Block))
	node.removeProposed(proposal.Block.Header.Height, proposal.Round)
	if !strings.Contains(err.Error(), "invalid transaction nonce") {
		return
	}
	node.removeNonceInvalidProposalTxs(proposal.Block, proposal.Round)
}

func (node *Node) pruneStalePendingProposals(ctx context.Context) {
	for _, proposal := range node.pendingProposals() {
		if node.cachedProposalStillValid(ctx, proposal) {
			continue
		}
		node.removePending(consensus.HashBlock(proposal.Block))
		node.removeProposed(proposal.Block.Header.Height, proposal.Round)
	}
}

func (node *Node) removeNonceInvalidProposalTxs(block types.Block, round types.Round) {
	runtime, runtimeErr := node.Runtime()
	if runtimeErr != nil || runtime == nil || runtime.Mempool == nil {
		return
	}
	rejected := make(map[types.Hash]struct{}, len(block.Txs))
	for _, tx := range block.Txs {
		rejected[mempool.HashTx(tx)] = struct{}{}
	}
	_, _ = runtime.Mempool.RetainTxs(context.Background(), func(tx types.Tx) bool {
		_, found := rejected[mempool.HashTx(tx)]
		return !found
	})
	node.removePending(consensus.HashBlock(block))
	node.removeProposed(block.Header.Height, round)
}

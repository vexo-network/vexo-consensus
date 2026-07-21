package node

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func SelectProposer(validators []validator.Validator, height types.Height, round types.Round) (types.ValidatorID, error) {
	if height == 0 {
		height = 1
	}
	activeValidators := make([]validator.Validator, 0, len(validators))
	for _, validatorInfo := range validators {
		if validatorInfo.VotingPower > 0 {
			activeValidators = append(activeValidators, validatorInfo)
		}
	}
	if len(activeValidators) == 0 {
		return "", ErrMissingValidators
	}
	sort.Slice(activeValidators, func(left, right int) bool {
		return activeValidators[left].ID < activeValidators[right].ID
	})
	index := (uint64(height) - 1 + uint64(round)) % uint64(len(activeValidators))
	return activeValidators[index].ID, nil
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
		if node.shouldRecoverProposerRound(ctx, height, status.Round, options) {
			recoveryRound, ok, err := node.nextLocalProposerRound(ctx, height, status.Round)
			if err != nil {
				return consensus.Proposal{}, types.Hash{}, false, err
			}
			if ok {
				machine.StartRound(height, recoveryRound)
				return node.tickConsensusWithProposalOptions(ctx, cfg, options)
			}
		}
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
	proposal, blockHash, err := node.ProposeFromMempoolWithOptions(ctx, cfg.MaxBlockBytes, options)
	if errors.Is(err, ErrEmptyProposal) {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	node.markProposed(proposal.Block.Header.Height, proposal.Round)
	return proposal, blockHash, true, nil
}

func (node *Node) shouldRecoverProposerRound(ctx context.Context, height types.Height, round types.Round, options ProposalOptions) bool {
	if round == 0 || options.ForceEmpty {
		return false
	}
	if node.hasPendingProposalAtHeight(height) {
		return false
	}
	if !node.quorumHealthAllowsProposerRecovery(ctx, height) {
		return false
	}
	return node.mempoolHasPendingTx(ctx)
}

func (node *Node) quorumHealthAllowsProposerRecovery(ctx context.Context, height types.Height) bool {
	runtime, err := node.Runtime()
	if err != nil {
		return true
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return true
	}
	validatorCount := len(validatorSet.List())
	if validatorCount <= 1 {
		return true
	}
	status := node.Status(ctx)
	if status.ActivePeerCount <= 0 {
		return true
	}
	if status.ActivePeerCount < validatorCount-1 {
		return false
	}
	if status.ConfiguredPeerCount > 0 {
		ratio := float64(status.ActivePeerCount) / float64(status.ConfiguredPeerCount)
		if ratio < 0.75 {
			return false
		}
	}
	return true
}

func (node *Node) nextLocalProposerRound(ctx context.Context, height types.Height, round types.Round) (types.Round, bool, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return 0, false, err
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return 0, false, err
	}
	validators := validatorSet.List()
	if len(validators) == 0 {
		return 0, false, ErrMissingValidators
	}
	if uint64(round) < uint64(len(validators)) {
		return 0, false, nil
	}
	for offset := 1; offset <= len(validators); offset++ {
		candidateRound := round + types.Round(offset)
		proposer, err := SelectProposer(validators, height, candidateRound)
		if err != nil {
			return 0, false, err
		}
		if proposer == node.cfg.ValidatorID {
			return candidateRound, true, nil
		}
	}
	return 0, false, nil
}

func (node *Node) cachedProposalStillValid(ctx context.Context, proposal consensus.Proposal) bool {
	runtime, err := node.Runtime()
	if err != nil {
		return false
	}
	appRuntime, ok := runtime.App.(*app.Runtime)
	if !ok || appRuntime == nil {
		return false
	}
	return appRuntime.ProcessProposalContext(ctx, app.ProcessProposalRequest{Block: proposal.Block}).Accepted
}

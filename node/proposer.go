package node

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func SelectProposer(validators []validator.Validator, height types.Height, round types.Round) (types.ValidatorID, error) {
	if height == 0 {
		height = 1
	}
	if len(validators) == 0 {
		return "", ErrMissingValidators
	}
	index := (uint64(height) - 1 + uint64(round)) % uint64(len(validators))
	return validators[index].ID, nil
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
	proposal, blockHash, err := node.ProposeFromMempool(ctx, maxBytes)
	if errors.Is(err, ErrEmptyProposal) {
		return consensus.Proposal{}, types.Hash{}, false, nil
	}
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, false, err
	}
	return proposal, blockHash, true, nil
}

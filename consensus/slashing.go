package consensus

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrEvidenceValidatorNotFound = errors.New("evidence validator not found")

type SlashResult struct {
	Receipt        slashing.PenaltyReceipt
	PreviousPower  types.VotingPower
	RemainingPower types.VotingPower
}

func SubmitEvidenceForSlashing(ctx context.Context, keeper *slashing.InMemoryKeeper, registry validator.Registry, evidence slashing.Evidence) (SlashResult, error) {
	if err := verifyConsensusEvidence(evidence); err != nil {
		return SlashResult{}, err
	}
	receipt, err := keeper.SubmitAndApply(ctx, evidence)
	if err != nil {
		return SlashResult{}, err
	}

	set, err := registry.ValidatorSet(ctx, evidence.Height)
	if err != nil {
		return SlashResult{}, err
	}
	validatorInfo, found := set.Get(evidence.Validator)
	if !found {
		return SlashResult{}, ErrEvidenceValidatorNotFound
	}
	remainingPower, err := slashing.ApplySlash(validatorInfo.VotingPower, receipt.Penalty)
	if err != nil {
		return SlashResult{}, err
	}
	if err := registry.UpdateVotingPower(ctx, evidence.Validator, remainingPower); err != nil {
		return SlashResult{}, err
	}

	return SlashResult{
		Receipt:        receipt,
		PreviousPower:  validatorInfo.VotingPower,
		RemainingPower: remainingPower,
	}, nil
}

func verifyConsensusEvidence(evidence slashing.Evidence) error {
	switch evidence.Type {
	case slashing.EvidenceConflictingVote:
		return VerifyConflictingVoteEvidence(evidence)
	case slashing.EvidenceConflictingTimeoutVote:
		return VerifyConflictingTimeoutVoteEvidence(evidence)
	default:
		return nil
	}
}

package consensus

import (
	"errors"
	"sort"
	"strings"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrConflictingTimeoutVote = errors.New("conflicting timeout vote")
)

type TimeoutVote struct {
	Height      types.Height
	Round       types.Round
	ValidatorID types.ValidatorID
	HighQC      finality.QuorumCert
	Signature   types.Signature
}

type TimeoutCollector struct {
	validatorSet validator.Set
	aggregator   aggregateSigner
	votes        map[types.Height]map[types.Round]map[types.ValidatorID]TimeoutVote
}

func NewTimeoutCollector(validatorSet validator.Set) *TimeoutCollector {
	return NewTimeoutCollectorWithAggregator(validatorSet, nil)
}

func NewTimeoutCollectorWithAggregator(validatorSet validator.Set, aggregator aggregateSigner) *TimeoutCollector {
	return &TimeoutCollector{
		validatorSet: validatorSet,
		aggregator:   aggregator,
		votes:        make(map[types.Height]map[types.Round]map[types.ValidatorID]TimeoutVote),
	}
}

func (collector *TimeoutCollector) AddVote(vote TimeoutVote) error {
	if _, found := collector.validatorSet.Get(vote.ValidatorID); !found {
		return ErrUnknownValidator
	}
	collector.ensureMaps(vote.Height, vote.Round)
	if previous, found := collector.votes[vote.Height][vote.Round][vote.ValidatorID]; found {
		if !sameQC(previous.HighQC, vote.HighQC) {
			return ErrConflictingTimeoutVote
		}
		return nil
	}
	collector.votes[vote.Height][vote.Round][vote.ValidatorID] = vote
	return nil
}

func (collector *TimeoutCollector) BuildTimeoutCert(height types.Height, round types.Round) (finality.TimeoutCert, error) {
	roundVotes := collector.votesForRound(height, round)
	if len(roundVotes) == 0 {
		return finality.TimeoutCert{}, ErrNoQuorum
	}

	type signedTimeoutVote struct {
		validatorID types.ValidatorID
		signature   types.Signature
	}
	var votingPower types.VotingPower
	var highQC finality.QuorumCert
	votes := make([]signedTimeoutVote, 0, len(roundVotes))
	for validatorID, vote := range roundVotes {
		validatorInfo, found := collector.validatorSet.Get(validatorID)
		if !found {
			continue
		}
		votingPower += validatorInfo.VotingPower
		votes = append(votes, signedTimeoutVote{validatorID: validatorID, signature: vote.Signature})
		if isBetterQC(vote.HighQC, highQC) {
			highQC = vote.HighQC
		}
	}
	if !hasQuorum(votingPower, collector.validatorSet.TotalVotingPower()) {
		return finality.TimeoutCert{}, ErrNoQuorum
	}

	sort.Slice(votes, func(firstIndex int, secondIndex int) bool {
		return votes[firstIndex].validatorID < votes[secondIndex].validatorID
	})
	signers := make([]string, 0, len(votes))
	signatures := make([]types.Signature, 0, len(votes))
	for _, vote := range votes {
		signers = append(signers, string(vote.validatorID))
		signatures = append(signatures, vote.signature)
	}
	aggregateSignature := types.AggregateSignature("placeholder-timeout-signature")
	if collector.aggregator != nil && allSignaturesPresent(signatures) {
		signature, err := collector.aggregator.Aggregate(signatures)
		if err != nil {
			return finality.TimeoutCert{}, err
		}
		aggregateSignature = signature
	}
	return finality.TimeoutCert{
		Height:    height,
		Round:     round,
		HighQC:    highQC,
		Signers:   types.Bitmap(strings.Join(signers, ",")),
		Signature: aggregateSignature,
	}, nil
}

func (collector *TimeoutCollector) votesForRound(height types.Height, round types.Round) map[types.ValidatorID]TimeoutVote {
	if _, found := collector.votes[height]; !found {
		return nil
	}
	return collector.votes[height][round]
}

func (collector *TimeoutCollector) ensureMaps(height types.Height, round types.Round) {
	if _, found := collector.votes[height]; !found {
		collector.votes[height] = make(map[types.Round]map[types.ValidatorID]TimeoutVote)
	}
	if _, found := collector.votes[height][round]; !found {
		collector.votes[height][round] = make(map[types.ValidatorID]TimeoutVote)
	}
}

func sameQC(first finality.QuorumCert, second finality.QuorumCert) bool {
	return first.Height == second.Height &&
		first.Round == second.Round &&
		first.BlockHash == second.BlockHash
}

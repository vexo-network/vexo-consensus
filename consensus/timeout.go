package consensus

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrConflictingTimeoutVote = errors.New("conflicting timeout vote")
	ErrInvalidTimeoutCert     = errors.New("invalid timeout certificate")
)

// TimeoutVote is a validator's signed statement that it gave up on view Round while
// waiting for a block at Height, together with the highest QC it knows. The signature
// covers the whole vote so that two votes in one view with different HighQC values are
// provable equivocation.
type TimeoutVote struct {
	Height      types.Height
	Round       types.Round
	ValidatorID types.ValidatorID
	HighQC      finality.QuorumCert
	Signature   types.Signature
}

// timeoutSignatureGroup aggregates the timeout votes that signed identical bytes. Votes
// with different Height or HighQC values sign different messages, so a TimeoutCert keeps
// one aggregate signature per distinct signed message, encoded in TimeoutCert.Signature.
type timeoutSignatureGroup struct {
	Height    types.Height             `json:"height"`
	HighQC    finality.QuorumCert      `json:"high_qc"`
	Signers   []types.ValidatorID      `json:"signers"`
	Signature types.AggregateSignature `json:"signature"`
}

// TimeoutCollector gathers timeout votes per global view. Callers verify the vote
// signature and its HighQC before adding it.
type TimeoutCollector struct {
	validatorSets func(types.Height) validator.Set
	aggregator    aggregateSigner
	votes         map[types.Round]map[types.ValidatorID]TimeoutVote
}

func NewTimeoutCollector(validatorSet validator.Set, aggregator aggregateSigner) *TimeoutCollector {
	return newTimeoutCollector(func(types.Height) validator.Set { return validatorSet }, aggregator)
}

func newTimeoutCollector(validatorSets func(types.Height) validator.Set, aggregator aggregateSigner) *TimeoutCollector {
	return &TimeoutCollector{
		validatorSets: validatorSets,
		aggregator:    aggregator,
		votes:         make(map[types.Round]map[types.ValidatorID]TimeoutVote),
	}
}

func (collector *TimeoutCollector) AddVote(vote TimeoutVote) error {
	if _, found := collector.validatorSets(vote.Height).Get(vote.ValidatorID); !found {
		return ErrUnknownValidator
	}
	if previous, found := collector.votes[vote.Round][vote.ValidatorID]; found {
		if !sameQC(previous.HighQC, vote.HighQC) {
			return ErrConflictingTimeoutVote
		}
		return nil
	}
	if collector.votes[vote.Round] == nil {
		collector.votes[vote.Round] = make(map[types.ValidatorID]TimeoutVote)
	}
	collector.votes[vote.Round][vote.ValidatorID] = vote
	return nil
}

func (collector *TimeoutCollector) ConflictingVote(vote TimeoutVote) (TimeoutVote, bool) {
	previous, found := collector.votes[vote.Round][vote.ValidatorID]
	if !found || sameQC(previous.HighQC, vote.HighQC) {
		return TimeoutVote{}, false
	}
	return previous, true
}

// Prune forgets the votes of every view below round.
func (collector *TimeoutCollector) Prune(round types.Round) {
	for viewRound := range collector.votes {
		if viewRound < round {
			delete(collector.votes, viewRound)
		}
	}
}

// BuildTimeoutCert aggregates the votes of one view into a TimeoutCert once they reach the
// two-thirds quorum. The certificate height is the highest vote height, and the validator
// set of that height decides membership and quorum.
func (collector *TimeoutCollector) BuildTimeoutCert(round types.Round) (finality.TimeoutCert, error) {
	roundVotes := collector.votes[round]
	if len(roundVotes) == 0 {
		return finality.TimeoutCert{}, ErrNoQuorum
	}
	validatorIDs := make([]types.ValidatorID, 0, len(roundVotes))
	var certHeight types.Height
	for validatorID, vote := range roundVotes {
		validatorIDs = append(validatorIDs, validatorID)
		if vote.Height > certHeight {
			certHeight = vote.Height
		}
	}
	sort.Slice(validatorIDs, func(first, second int) bool { return validatorIDs[first] < validatorIDs[second] })

	validatorSet := collector.validatorSets(certHeight)
	type voteGroup struct {
		height     types.Height
		highQC     finality.QuorumCert
		signers    []types.ValidatorID
		signatures []types.Signature
	}
	groups := make(map[string]*voteGroup)
	signers := make([]types.ValidatorID, 0, len(validatorIDs))
	var votingPower types.VotingPower
	var highQC finality.QuorumCert
	for _, validatorID := range validatorIDs {
		vote := roundVotes[validatorID]
		validatorInfo, found := validatorSet.Get(validatorID)
		if !found {
			continue
		}
		var err error
		votingPower, err = types.AddVotingPower(votingPower, validatorInfo.VotingPower)
		if err != nil {
			return finality.TimeoutCert{}, err
		}
		signers = append(signers, validatorID)
		key := string(TimeoutVoteSignBytes(vote))
		group, found := groups[key]
		if !found {
			group = &voteGroup{height: vote.Height, highQC: vote.HighQC}
			groups[key] = group
		}
		group.signers = append(group.signers, validatorID)
		group.signatures = append(group.signatures, vote.Signature)
		if isBetterQC(vote.HighQC, highQC) {
			highQC = vote.HighQC
		}
	}
	if !hasQuorum(votingPower, validatorSet.TotalVotingPower()) {
		return finality.TimeoutCert{}, ErrNoQuorum
	}
	if collector.aggregator == nil {
		return finality.TimeoutCert{}, ErrAggregateRequired
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	encodedGroups := make([]timeoutSignatureGroup, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		if !allSignaturesPresent(group.signatures) {
			return finality.TimeoutCert{}, ErrMissingSignature
		}
		aggregateSignature, err := collector.aggregator.Aggregate(group.signatures)
		if err != nil {
			return finality.TimeoutCert{}, err
		}
		encodedGroups = append(encodedGroups, timeoutSignatureGroup{
			Height:    group.height,
			HighQC:    group.highQC,
			Signers:   group.signers,
			Signature: aggregateSignature,
		})
	}
	encoded, err := json.Marshal(encodedGroups)
	if err != nil {
		return finality.TimeoutCert{}, err
	}
	return finality.TimeoutCert{
		Height:    certHeight,
		Round:     round,
		HighQC:    highQC,
		Signers:   finality.EncodeSigners(signers),
		Signature: types.AggregateSignature(encoded),
	}, nil
}

func decodeTimeoutSignatureGroups(signature types.AggregateSignature) ([]timeoutSignatureGroup, error) {
	if len(signature) == 0 {
		return nil, errors.New("missing timeout signature groups")
	}
	var groups []timeoutSignatureGroup
	if err := json.Unmarshal(signature, &groups); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, errors.New("empty timeout signature groups")
	}
	return groups, nil
}

func sameQC(first finality.QuorumCert, second finality.QuorumCert) bool {
	return first.Height == second.Height &&
		first.Round == second.Round &&
		first.BlockHash == second.BlockHash
}

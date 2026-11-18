package consensus

import (
	"encoding/binary"
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
	Height    types.Height
	HighQC    finality.QuorumCert
	Signers   []types.ValidatorID
	Signature types.AggregateSignature
}

// maxTimeoutSigners bounds the signers a timeout certificate lists across its signature groups,
// and so its groups: a group has at least one signer.
const maxTimeoutSigners = 1 << 16

// minTimeoutGroupBytes is the length of the encoding of the smallest group, which bounds the groups
// an input can hold.
var minTimeoutGroupBytes = len(appendTimeoutSignatureGroup(nil, timeoutSignatureGroup{}))

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
	return finality.TimeoutCert{
		Height:    certHeight,
		Round:     round,
		HighQC:    highQC,
		Signers:   finality.EncodeSigners(signers),
		Signature: encodeTimeoutSignatureGroups(encodedGroups),
	}, nil
}

// The signature groups of a timeout certificate are encoded as the count of groups followed by each
// group: its height, its high QC, the count of its signers followed by each signer, and its
// aggregate signature, in the canonical encoding of package types. A proposal signature commits to
// the hash of these bytes, so a set of groups has exactly one encoding and the decoder refuses any
// other.

func appendTimeoutSignatureGroup(dst []byte, group timeoutSignatureGroup) []byte {
	dst = binary.AppendUvarint(dst, uint64(group.Height))
	dst = finality.AppendQuorumCert(dst, group.HighQC)
	dst = binary.AppendUvarint(dst, uint64(len(group.Signers)))
	for _, signer := range group.Signers {
		dst = types.AppendBytes(dst, signer)
	}
	return types.AppendBytes(dst, group.Signature)
}

func encodeTimeoutSignatureGroups(groups []timeoutSignatureGroup) types.AggregateSignature {
	encoded := binary.AppendUvarint(nil, uint64(len(groups)))
	for _, group := range groups {
		encoded = appendTimeoutSignatureGroup(encoded, group)
	}
	return types.AggregateSignature(encoded)
}

func decodeTimeoutSignatureGroups(signature types.AggregateSignature) ([]timeoutSignatureGroup, error) {
	if len(signature) == 0 {
		return nil, errors.New("missing timeout signature groups")
	}
	reader := types.NewReader(signature)
	var groups []timeoutSignatureGroup
	listed := 0
	if count := reader.Count(maxTimeoutSigners, minTimeoutGroupBytes); count > 0 {
		groups = make([]timeoutSignatureGroup, count)
		for index := range groups {
			group := &groups[index]
			group.Height = types.Height(reader.Uvarint())
			group.HighQC = finality.ReadQuorumCert(reader)
			if signers := reader.Count(maxTimeoutSigners-listed, 1); signers > 0 {
				listed += signers
				group.Signers = make([]types.ValidatorID, signers)
				for position := range group.Signers {
					group.Signers[position] = types.ValidatorID(reader.String(types.MaxIDBytes))
				}
			}
			group.Signature = types.AggregateSignature(reader.Bytes(finality.MaxAggregateSignatureBytes))
		}
	}
	if err := reader.Done(); err != nil {
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

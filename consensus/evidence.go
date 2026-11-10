package consensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrVotesDoNotConflict        = errors.New("votes do not conflict")
	ErrTimeoutVotesDoNotConflict = errors.New("timeout votes do not conflict")
	ErrVotePairMismatch          = errors.New("votes are not from the same validator height and round")
	ErrUnsupportedProposalReason = errors.New("unsupported invalid proposal reason")
	// ErrMissingExpectedHash reports hash-mismatch evidence verified without the hash the
	// proposal's height requires.
	ErrMissingExpectedHash = errors.New("the hash the proposal's height requires is missing")
	// ErrParentNotNamed rejects parent-linkage evidence whose parent header is not the one the
	// proposal's header names by hash: it proves nothing about the proposer.
	ErrParentNotNamed = errors.New("the parent header is not the one the proposal names")
)

// InvalidProposalReason names the fault of an invalid proposal. Every reason is decidable from the
// proposal and data every node holds alike: the proposal's transactions (da_mismatch,
// missing_data), the replicated validator sets of the heights its header names
// (validator_set_hash, next_validator_set_hash), the app hash certified blocks committed for the
// state it names (app_hash, from replicated state), or its parent header (parent_linkage).
//
// parent_linkage rests on the hash link: the proposer signs its header, the header names the hash
// of its parent, so the parent header the proof carries is the one the proposer built on, and
// every rule that is a function of the header and its parent header (height, time, the validator
// set and state heights) is decidable from the proof alone. What needs more than that,
// such as a re-execution of an invalid transaction, is not evidence: an invalid transaction has
// no effect on the state, and an app hash that does not follow from the executed state is app_hash
// evidence.
type InvalidProposalReason string

const (
	InvalidProposalReasonDAMismatch           InvalidProposalReason = "da_mismatch"
	InvalidProposalReasonMissingData          InvalidProposalReason = "missing_data"
	InvalidProposalReasonValidatorSetHash     InvalidProposalReason = "validator_set_hash"
	InvalidProposalReasonNextValidatorSetHash InvalidProposalReason = "next_validator_set_hash"
	InvalidProposalReasonAppHash              InvalidProposalReason = "app_hash"
	InvalidProposalReasonParentLinkage        InvalidProposalReason = "parent_linkage"
)

type ConflictingVoteProof struct {
	First  Vote `json:"first"`
	Second Vote `json:"second"`
}

type ConflictingTimeoutVoteProof struct {
	First  TimeoutVote `json:"first"`
	Second TimeoutVote `json:"second"`
}

// InvalidProposalProof proves a proposal carries ActualHash where its height requires
// ExpectedHash: the data-availability commitment of its transactions, its validator-set hash, its
// next validator-set hash, or its app hash, as Reason says. For parent_linkage it carries the
// parent header instead, which the proposal's header names by hash; the first block has none.
type InvalidProposalProof struct {
	Proposal     Proposal              `json:"proposal"`
	Reason       InvalidProposalReason `json:"reason"`
	ExpectedHash types.Hash            `json:"expected_hash,omitempty"`
	ActualHash   types.Hash            `json:"actual_hash,omitempty"`
	Parent       *types.Header         `json:"parent,omitempty"`
}

type UnavailableDataProof struct {
	Proposal           Proposal   `json:"proposal"`
	Reason             string     `json:"reason"`
	ExpectedCommitment types.Hash `json:"expected_commitment"`
	ActualCommitment   types.Hash `json:"actual_commitment"`
	TxCount            uint64     `json:"tx_count"`
	TotalBytes         uint64     `json:"total_bytes"`
}

func NewConflictingVoteEvidence(first Vote, second Vote) (slashing.Evidence, error) {
	if first.ValidatorID != second.ValidatorID || first.Height != second.Height || first.Round != second.Round {
		return slashing.Evidence{}, ErrVotePairMismatch
	}
	if first.BlockHash == second.BlockHash {
		return slashing.Evidence{}, ErrVotesDoNotConflict
	}

	proof, err := json.Marshal(ConflictingVoteProof{
		First:  first,
		Second: second,
	})
	if err != nil {
		return slashing.Evidence{}, err
	}

	return slashing.Evidence{
		Type:      slashing.EvidenceConflictingVote,
		Validator: first.ValidatorID,
		Height:    first.Height,
		Round:     first.Round,
		Proof:     proof,
	}, nil
}

func NewConflictingTimeoutVoteEvidence(first TimeoutVote, second TimeoutVote) (slashing.Evidence, error) {
	if first.ValidatorID != second.ValidatorID || first.Height != second.Height || first.Round != second.Round {
		return slashing.Evidence{}, ErrVotePairMismatch
	}
	if sameQC(first.HighQC, second.HighQC) {
		return slashing.Evidence{}, ErrTimeoutVotesDoNotConflict
	}

	proof, err := json.Marshal(ConflictingTimeoutVoteProof{
		First:  first,
		Second: second,
	})
	if err != nil {
		return slashing.Evidence{}, err
	}

	return slashing.Evidence{
		Type:      slashing.EvidenceConflictingTimeoutVote,
		Validator: first.ValidatorID,
		Height:    first.Height,
		Round:     first.Round,
		Proof:     proof,
	}, nil
}

// NewDataAvailabilityInvalidProposalEvidence proves proposal's header carries a data commitment
// other than the one of its transactions (da_mismatch) or none (missing_data).
func NewDataAvailabilityInvalidProposalEvidence(proposal Proposal, reason InvalidProposalReason) (slashing.Evidence, error) {
	if reason != InvalidProposalReasonDAMismatch && reason != InvalidProposalReasonMissingData {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	return newInvalidProposalEvidence(InvalidProposalProof{
		Proposal:     proposal,
		Reason:       reason,
		ExpectedHash: dataavailability.Commitment(proposal.Block.Txs),
		ActualHash:   proposal.Block.Header.ConsensusHash,
	}, types.Hash{})
}

// NewParentLinkageEvidence proves proposal's header breaks a rule its parent header fixes for it:
// its height, time, validator set height or state height. parent is the header the
// proposal's header names by hash, nil for the first block.
func NewParentLinkageEvidence(proposal Proposal, parent *types.Header) (slashing.Evidence, error) {
	var carried *types.Header
	if parent != nil {
		copied := *parent
		carried = &copied
	}
	return newInvalidProposalEvidence(InvalidProposalProof{
		Proposal: proposal,
		Reason:   InvalidProposalReasonParentLinkage,
		Parent:   carried,
	}, types.Hash{})
}

// ParentLinkageFault reports why header breaks a rule its parent fixes for it, nil when it keeps
// them all. parent is the header header names by hash, nil for the first block. Every rule is a
// function of the two headers, so every node decides it alike.
func ParentLinkageFault(header types.Header, parent *types.Header) error {
	if header.Height <= 1 {
		if parent != nil {
			return fmt.Errorf("%w: the first block has no parent", ErrParentNotNamed)
		}
		if header.PreviousBlockHash != (types.Hash{}) || header.TimeUnixNano <= 0 ||
			header.ValidatorSetHeight != finality.FirstValidatorSetHeight || header.StateHeight != 0 || len(header.Benched) != 0 {
			return fmt.Errorf("%w: the first block must name no parent, a positive time, the genesis set and state, and bench no one", ErrInvalidProposal)
		}
		return nil
	}
	if parent == nil || finality.HeaderHash(*parent) != header.PreviousBlockHash {
		return fmt.Errorf("%w: block %d", ErrParentNotNamed, header.Height)
	}
	switch {
	case parent.Height+1 != header.Height:
		return fmt.Errorf("%w: height %d on a parent of height %d", ErrInvalidProposal, header.Height, parent.Height)
	case header.TimeUnixNano <= parent.TimeUnixNano:
		return fmt.Errorf("%w: block time %d does not follow its parent's %d", ErrInvalidProposal, header.TimeUnixNano, parent.TimeUnixNano)
	case header.ValidatorSetHeight != finality.ChildValidatorSetHeight(*parent) || header.StateHeight != finality.ChildStateHeight(*parent):
		return fmt.Errorf("%w: validator set height %d and state height %d, the parent fixes %d and %d", ErrInvalidProposal, header.ValidatorSetHeight, header.StateHeight, finality.ChildValidatorSetHeight(*parent), finality.ChildStateHeight(*parent))
	}
	return nil
}

// NewInvalidProposalHashEvidence proves proposal's header carries a validator-set hash
// (validator_set_hash), a next validator-set hash (next_validator_set_hash) or an app hash
// (app_hash) other than expected, the one its height requires.
func NewInvalidProposalHashEvidence(proposal Proposal, reason InvalidProposalReason, expected types.Hash) (slashing.Evidence, error) {
	actual, ok := headerHashFor(proposal.Block.Header, reason)
	if !ok {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	return newInvalidProposalEvidence(InvalidProposalProof{
		Proposal:     proposal,
		Reason:       reason,
		ExpectedHash: expected,
		ActualHash:   actual,
	}, expected)
}

func newInvalidProposalEvidence(proof InvalidProposalProof, expected types.Hash) (slashing.Evidence, error) {
	if proof.Proposal.Proposer == "" || proof.Proposal.Block.Header.Height == 0 {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if err := verifyInvalidProposal(proof, expected); err != nil {
		return slashing.Evidence{}, err
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceInvalidProposal,
		Validator: proof.Proposal.Proposer,
		Height:    proof.Proposal.Block.Header.Height,
		Round:     proof.Proposal.Round,
		Proof:     encoded,
	}, nil
}

// headerHashFor returns the header hash a hash-mismatch reason is about.
func headerHashFor(header types.Header, reason InvalidProposalReason) (types.Hash, bool) {
	switch reason {
	case InvalidProposalReasonValidatorSetHash:
		return header.ValidatorSetHash, true
	case InvalidProposalReasonNextValidatorSetHash:
		return header.NextValidatorSetHash, true
	case InvalidProposalReasonAppHash:
		return header.AppHash, true
	default:
		return types.Hash{}, false
	}
}

// verifyInvalidProposal checks that proof proves its reason. expected is the hash the proposal's
// height requires for a hash-mismatch reason, and unused for a data-availability one.
func verifyInvalidProposal(proof InvalidProposalProof, expected types.Hash) error {
	if proof.Reason != InvalidProposalReasonParentLinkage && proof.Parent != nil {
		return ErrInvalidProposal
	}
	switch proof.Reason {
	case InvalidProposalReasonParentLinkage:
		if proof.ExpectedHash != (types.Hash{}) || proof.ActualHash != (types.Hash{}) {
			return ErrInvalidProposal
		}
		err := ParentLinkageFault(proof.Proposal.Block.Header, proof.Parent)
		if err == nil {
			return ErrInvalidProposal
		}
		if errors.Is(err, ErrParentNotNamed) {
			return err
		}
		return nil
	case InvalidProposalReasonDAMismatch, InvalidProposalReasonMissingData:
		if proof.ExpectedHash != dataavailability.Commitment(proof.Proposal.Block.Txs) ||
			proof.ActualHash != proof.Proposal.Block.Header.ConsensusHash {
			return ErrInvalidProposal
		}
		want := dataavailability.ErrCommitmentMismatch
		if proof.Reason == InvalidProposalReasonMissingData {
			want = dataavailability.ErrMissingData
		}
		err := dataavailability.Verify(proof.Proposal.Block.Header, proof.Proposal.Block.Txs)
		if !errors.Is(err, want) {
			if err == nil {
				return ErrInvalidProposal
			}
			return err
		}
		return nil
	case InvalidProposalReasonValidatorSetHash, InvalidProposalReasonNextValidatorSetHash, InvalidProposalReasonAppHash:
		if expected == (types.Hash{}) {
			return ErrMissingExpectedHash
		}
		actual, _ := headerHashFor(proof.Proposal.Block.Header, proof.Reason)
		if proof.ExpectedHash != expected || proof.ActualHash != actual || expected == actual {
			return ErrInvalidProposal
		}
		return nil
	default:
		return ErrUnsupportedProposalReason
	}
}

func NewUnavailableDataEvidence(proposal Proposal, reason string) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 || reason == "" {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if !errors.Is(dataavailability.Verify(proposal.Block.Header, proposal.Block.Txs), dataavailability.ErrMissingData) {
		return slashing.Evidence{}, dataavailability.ErrMissingData
	}
	daProof := dataavailability.BuildProof(proposal.Block.Txs)
	proof, err := json.Marshal(UnavailableDataProof{
		Proposal:           proposal,
		Reason:             reason,
		ExpectedCommitment: daProof.Commitment,
		ActualCommitment:   proposal.Block.Header.ConsensusHash,
		TxCount:            daProof.TxCount,
		TotalBytes:         daProof.TotalBytes,
	})
	if err != nil {
		return slashing.Evidence{}, err
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceUnavailableData,
		Validator: proposal.Proposer,
		Height:    proposal.Block.Header.Height,
		Round:     proposal.Round,
		Proof:     proof,
	}, nil
}

// The Decode*Proof functions are strict: a proof has exactly one spelling, the one json.Marshal gives
// the value it decodes to, and anything else is rejected (see slashing.DecodeProof). A proof is part
// of an evidence transaction, so a second spelling of one proof would be a second transaction for
// the same offence.

func DecodeConflictingVoteProof(proof []byte) (ConflictingVoteProof, error) {
	return slashing.DecodeProof[ConflictingVoteProof](proof)
}

func DecodeConflictingTimeoutVoteProof(proof []byte) (ConflictingTimeoutVoteProof, error) {
	return slashing.DecodeProof[ConflictingTimeoutVoteProof](proof)
}

func DecodeInvalidProposalProof(proof []byte) (InvalidProposalProof, error) {
	return slashing.DecodeProof[InvalidProposalProof](proof)
}

func DecodeUnavailableDataProof(proof []byte) (UnavailableDataProof, error) {
	return slashing.DecodeProof[UnavailableDataProof](proof)
}

func VerifyConflictingVoteEvidence(evidence slashing.Evidence) error {
	if evidence.Type != slashing.EvidenceConflictingVote {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeConflictingVoteProof(evidence.Proof)
	if err != nil {
		return err
	}
	if decoded.First.ValidatorID != evidence.Validator ||
		decoded.First.Height != evidence.Height ||
		decoded.First.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	if decoded.Second.ValidatorID != evidence.Validator ||
		decoded.Second.Height != evidence.Height ||
		decoded.Second.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	if bytes.Equal(decoded.First.BlockHash[:], decoded.Second.BlockHash[:]) {
		return ErrVotesDoNotConflict
	}
	return nil
}

func VerifyConflictingTimeoutVoteEvidence(evidence slashing.Evidence) error {
	if evidence.Type != slashing.EvidenceConflictingTimeoutVote {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeConflictingTimeoutVoteProof(evidence.Proof)
	if err != nil {
		return err
	}
	if decoded.First.ValidatorID != evidence.Validator ||
		decoded.First.Height != evidence.Height ||
		decoded.First.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	if decoded.Second.ValidatorID != evidence.Validator ||
		decoded.Second.Height != evidence.Height ||
		decoded.Second.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	if sameQC(decoded.First.HighQC, decoded.Second.HighQC) {
		return ErrTimeoutVotesDoNotConflict
	}
	return nil
}

// VerifyInvalidProposalEvidence checks that evidence proves an invalid proposal of
// evidence.Validator at evidence.Height and evidence.Round. expected is the hash the proposal's
// height requires for validator_set_hash and app_hash evidence (ErrMissingExpectedHash without
// it); data-availability evidence is decided from the proposal alone and ignores it. It does not
// check the proposal's signature (see VerifyEvidence).
func VerifyInvalidProposalEvidence(evidence slashing.Evidence, expected types.Hash) error {
	if evidence.Type != slashing.EvidenceInvalidProposal {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		return err
	}
	if decoded.Proposal.Proposer != evidence.Validator ||
		decoded.Proposal.Block.Header.Height != evidence.Height ||
		decoded.Proposal.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	return verifyInvalidProposal(decoded, expected)
}

func VerifyUnavailableDataEvidence(evidence slashing.Evidence) error {
	if evidence.Type != slashing.EvidenceUnavailableData {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeUnavailableDataProof(evidence.Proof)
	if err != nil {
		return err
	}
	if decoded.Reason == "" ||
		decoded.Proposal.Proposer != evidence.Validator ||
		decoded.Proposal.Block.Header.Height != evidence.Height ||
		decoded.Proposal.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	daProof := dataavailability.BuildProof(decoded.Proposal.Block.Txs)
	if decoded.ExpectedCommitment == (types.Hash{}) ||
		decoded.ExpectedCommitment != daProof.Commitment ||
		decoded.ActualCommitment != decoded.Proposal.Block.Header.ConsensusHash ||
		decoded.ActualCommitment != (types.Hash{}) ||
		decoded.TxCount != daProof.TxCount ||
		decoded.TotalBytes != daProof.TotalBytes {
		return ErrInvalidProposal
	}
	if !errors.Is(dataavailability.Verify(decoded.Proposal.Block.Header, decoded.Proposal.Block.Txs), dataavailability.ErrMissingData) {
		return dataavailability.ErrMissingData
	}
	return nil
}

func VoteConflictFromPrevious(previousBlock types.Hash, vote Vote) (slashing.Evidence, error) {
	previous := Vote{
		Height:      vote.Height,
		Round:       vote.Round,
		BlockHash:   previousBlock,
		ValidatorID: vote.ValidatorID,
	}
	return NewConflictingVoteEvidence(previous, vote)
}

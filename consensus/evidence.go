package consensus

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/dataavailability"
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
)

// InvalidProposalReason names the fault of an invalid proposal. Every reason is decidable from the
// proposal and data every node holds alike: the proposal's transactions (da_mismatch,
// missing_data), the validator set of its height (validator_set_hash), or the app hash certified
// blocks committed for the state it names (app_hash, from replicated state). Faults that need
// other chain data or a re-execution, such as a bad timestamp or an invalid transaction, are not
// evidence: nodes could disagree about them, in particular one that pruned the data.
type InvalidProposalReason string

const (
	InvalidProposalReasonDAMismatch       InvalidProposalReason = "da_mismatch"
	InvalidProposalReasonMissingData      InvalidProposalReason = "missing_data"
	InvalidProposalReasonValidatorSetHash InvalidProposalReason = "validator_set_hash"
	InvalidProposalReasonAppHash          InvalidProposalReason = "app_hash"
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
// ExpectedHash: the data-availability commitment of its transactions, its validator-set hash, or
// its app hash, as Reason says.
type InvalidProposalProof struct {
	Proposal     Proposal              `json:"proposal"`
	Reason       InvalidProposalReason `json:"reason"`
	ExpectedHash types.Hash            `json:"expected_hash,omitempty"`
	ActualHash   types.Hash            `json:"actual_hash,omitempty"`
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

// NewInvalidProposalHashEvidence proves proposal's header carries a validator-set hash
// (validator_set_hash) or an app hash (app_hash) other than expected, the one its height requires.
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
	case InvalidProposalReasonAppHash:
		return header.AppHash, true
	default:
		return types.Hash{}, false
	}
}

// verifyInvalidProposal checks that proof proves its reason. expected is the hash the proposal's
// height requires for a hash-mismatch reason, and unused for a data-availability one.
func verifyInvalidProposal(proof InvalidProposalProof, expected types.Hash) error {
	switch proof.Reason {
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
	case InvalidProposalReasonValidatorSetHash, InvalidProposalReasonAppHash:
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

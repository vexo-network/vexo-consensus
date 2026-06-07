package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
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
	ErrInvalidProposalContext    = errors.New("invalid proposal verification context is required")
)

type InvalidProposalReason string

const (
	InvalidProposalReasonDAMismatch       InvalidProposalReason = "da_mismatch"
	InvalidProposalReasonMissingData      InvalidProposalReason = "missing_data"
	InvalidProposalReasonValidatorSetHash InvalidProposalReason = "validator_set_hash"
	InvalidProposalReasonAppHash          InvalidProposalReason = "app_hash"
	InvalidProposalReasonTimestamp        InvalidProposalReason = "timestamp"
	InvalidProposalReasonTxValidity       InvalidProposalReason = "tx_validity"
)

func SupportedInvalidProposalReasons() []InvalidProposalReason {
	return []InvalidProposalReason{
		InvalidProposalReasonDAMismatch,
		InvalidProposalReasonMissingData,
		InvalidProposalReasonValidatorSetHash,
		InvalidProposalReasonAppHash,
		InvalidProposalReasonTxValidity,
		InvalidProposalReasonTimestamp,
	}
}

type ConflictingVoteProof struct {
	First  Vote `json:"first"`
	Second Vote `json:"second"`
}

type ConflictingTimeoutVoteProof struct {
	First  TimeoutVote `json:"first"`
	Second TimeoutVote `json:"second"`
}

type InvalidProposalProof struct {
	Proposal             Proposal              `json:"proposal"`
	Reason               InvalidProposalReason `json:"reason"`
	ExpectedHash         types.Hash            `json:"expected_hash,omitempty"`
	ActualHash           types.Hash            `json:"actual_hash,omitempty"`
	ContextProofHash     types.Hash            `json:"context_proof_hash,omitempty"`
	ExpectedTimeUnixNano int64                 `json:"expected_time_unix_nano,omitempty"`
	ActualTimeUnixNano   int64                 `json:"actual_time_unix_nano,omitempty"`
	VerificationMessage  string                `json:"verification_message,omitempty"`
}

type InvalidProposalVerificationContext struct {
	ExpectedValidatorSetHash types.Hash
	ExpectedAppHash          types.Hash
	ExpectedTxResultsHash    types.Hash
	ContextProofHash         types.Hash
	ExpectedTimeUnixNano     int64
}

func (context InvalidProposalVerificationContext) ProofHash() types.Hash {
	if context.ExpectedValidatorSetHash == (types.Hash{}) &&
		context.ExpectedAppHash == (types.Hash{}) &&
		context.ExpectedTxResultsHash == (types.Hash{}) &&
		context.ExpectedTimeUnixNano == 0 {
		return types.Hash{}
	}
	hasher := sha256.New()
	hasher.Write([]byte("vexo.invalid_proposal.context.v1"))
	hasher.Write(context.ExpectedValidatorSetHash[:])
	hasher.Write(context.ExpectedAppHash[:])
	hasher.Write(context.ExpectedTxResultsHash[:])
	writeResultUint64(hasher, uint64(context.ExpectedTimeUnixNano))
	var out types.Hash
	copy(out[:], hasher.Sum(nil))
	return out
}

type UnavailableDataProof struct {
	Proposal Proposal `json:"proposal"`
	Reason   string   `json:"reason"`
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

func NewInvalidProposalEvidence(proposal Proposal, reason string) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 || reason == "" {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	proposalReason := InvalidProposalReason(reason)
	if !validInvalidProposalReason(proposalReason) {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	if proposalReason != InvalidProposalReasonDAMismatch && proposalReason != InvalidProposalReasonMissingData {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	if err := verifyInvalidProposalByReason(InvalidProposalProof{Proposal: proposal, Reason: proposalReason}); err != nil {
		return slashing.Evidence{}, err
	}
	proof, err := json.Marshal(InvalidProposalProof{Proposal: proposal, Reason: proposalReason})
	if err != nil {
		return slashing.Evidence{}, err
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceInvalidProposal,
		Validator: proposal.Proposer,
		Height:    proposal.Block.Header.Height,
		Round:     proposal.Round,
		Proof:     proof,
	}, nil
}

func NewInvalidProposalHashEvidence(proposal Proposal, reason string, expected types.Hash, actual types.Hash) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 || reason == "" {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	proposalReason := InvalidProposalReason(reason)
	switch proposalReason {
	case InvalidProposalReasonValidatorSetHash, InvalidProposalReasonAppHash:
	default:
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	proof := InvalidProposalProof{
		Proposal:     proposal,
		Reason:       proposalReason,
		ExpectedHash: expected,
		ActualHash:   actual,
	}
	if err := verifyInvalidProposalEnvelope(proof, proposal.Proposer, proposal.Block.Header.Height, proposal.Round); err != nil {
		return slashing.Evidence{}, err
	}
	if err := verifyInvalidProposalByReason(proof); err != nil {
		return slashing.Evidence{}, err
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceInvalidProposal,
		Validator: proposal.Proposer,
		Height:    proposal.Block.Header.Height,
		Round:     proposal.Round,
		Proof:     encoded,
	}, nil
}

func NewInvalidProposalTxValidityEvidence(proposal Proposal, expected types.Hash, actual types.Hash, message string) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	proof := InvalidProposalProof{
		Proposal:            proposal,
		Reason:              InvalidProposalReasonTxValidity,
		ExpectedHash:        expected,
		ActualHash:          actual,
		VerificationMessage: message,
	}
	return newInvalidProposalEvidenceFromProof(proof)
}

func NewInvalidProposalTimestampEvidence(proposal Proposal, expected int64, actual int64) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	proof := InvalidProposalProof{
		Proposal:             proposal,
		Reason:               InvalidProposalReasonTimestamp,
		ExpectedTimeUnixNano: expected,
		ActualTimeUnixNano:   actual,
	}
	if err := verifyInvalidProposalByReason(proof); err != nil {
		return slashing.Evidence{}, err
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceInvalidProposal,
		Validator: proposal.Proposer,
		Height:    proposal.Block.Header.Height,
		Round:     proposal.Round,
		Proof:     encoded,
	}, nil
}

func newInvalidProposalEvidenceFromProof(proof InvalidProposalProof) (slashing.Evidence, error) {
	if proof.Proposal.Proposer == "" || proof.Proposal.Block.Header.Height == 0 {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if err := verifyInvalidProposalEnvelope(proof, proof.Proposal.Proposer, proof.Proposal.Block.Header.Height, proof.Proposal.Round); err != nil {
		return slashing.Evidence{}, err
	}
	if err := verifyInvalidProposalByReason(proof); err != nil {
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

func verifyInvalidProposalEnvelope(decoded InvalidProposalProof, validatorID types.ValidatorID, height types.Height, round types.Round) error {
	if decoded.Reason == "" ||
		!validInvalidProposalReason(decoded.Reason) ||
		decoded.Proposal.Proposer != validatorID ||
		decoded.Proposal.Block.Header.Height != height ||
		decoded.Proposal.Round != round {
		return ErrVotePairMismatch
	}
	return nil
}

func verifyInvalidProposalByReason(decoded InvalidProposalProof) error {
	switch decoded.Reason {
	case InvalidProposalReasonDAMismatch:
		err := dataavailability.Verify(decoded.Proposal.Block.Header, decoded.Proposal.Block.Txs)
		if !errors.Is(err, dataavailability.ErrCommitmentMismatch) {
			if err == nil {
				return ErrInvalidProposal
			}
			return err
		}
		return nil
	case InvalidProposalReasonMissingData:
		err := dataavailability.Verify(decoded.Proposal.Block.Header, decoded.Proposal.Block.Txs)
		if !errors.Is(err, dataavailability.ErrMissingData) {
			if err == nil {
				return ErrInvalidProposal
			}
			return err
		}
		return nil
	case InvalidProposalReasonValidatorSetHash:
		if err := verifyHashMismatch(decoded, decoded.Proposal.Block.Header.ValidatorSetHash); err != nil {
			return err
		}
		return nil
	case InvalidProposalReasonAppHash:
		if err := verifyHashMismatch(decoded, decoded.Proposal.Block.Header.AppHash); err != nil {
			return err
		}
		return nil
	case InvalidProposalReasonTxValidity:
		if decoded.VerificationMessage == "" {
			return ErrInvalidProposal
		}
		if err := verifyHashMismatch(decoded, txSetHash(decoded.Proposal.Block.Txs)); err != nil {
			return err
		}
		return nil
	case InvalidProposalReasonTimestamp:
		if decoded.ExpectedTimeUnixNano == 0 ||
			decoded.ActualTimeUnixNano == 0 ||
			decoded.ExpectedTimeUnixNano == decoded.ActualTimeUnixNano ||
			(decoded.Proposal.Block.Header.TimeUnixNano != 0 && decoded.ActualTimeUnixNano != decoded.Proposal.Block.Header.TimeUnixNano) {
			return ErrInvalidProposal
		}
		return nil
	default:
		return ErrUnsupportedProposalReason
	}
}

func verifyHashMismatch(decoded InvalidProposalProof, proposalActual types.Hash) error {
	if decoded.ExpectedHash == (types.Hash{}) ||
		decoded.ActualHash == (types.Hash{}) ||
		decoded.ExpectedHash == decoded.ActualHash {
		return ErrInvalidProposal
	}
	if proposalActual != (types.Hash{}) && decoded.ActualHash != proposalActual {
		return ErrInvalidProposal
	}
	return nil
}

func txSetHash(txs []types.Tx) types.Hash {
	hasher := sha256.New()
	for _, tx := range txs {
		hash := sha256.Sum256(tx)
		_, _ = hasher.Write(hash[:])
	}
	var out types.Hash
	copy(out[:], hasher.Sum(nil))
	return out
}

func TxSetHashForEvidence(txs []types.Tx) types.Hash {
	return txSetHash(txs)
}

func NewUnavailableDataEvidence(proposal Proposal, reason string) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 || reason == "" {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if !errors.Is(dataavailability.Verify(proposal.Block.Header, proposal.Block.Txs), dataavailability.ErrMissingData) {
		return slashing.Evidence{}, dataavailability.ErrMissingData
	}
	proof, err := json.Marshal(UnavailableDataProof{Proposal: proposal, Reason: reason})
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

func DecodeConflictingVoteProof(proof []byte) (ConflictingVoteProof, error) {
	var decoded ConflictingVoteProof
	if err := json.Unmarshal(proof, &decoded); err != nil {
		return ConflictingVoteProof{}, err
	}
	return decoded, nil
}

func DecodeConflictingTimeoutVoteProof(proof []byte) (ConflictingTimeoutVoteProof, error) {
	var decoded ConflictingTimeoutVoteProof
	if err := json.Unmarshal(proof, &decoded); err != nil {
		return ConflictingTimeoutVoteProof{}, err
	}
	return decoded, nil
}

func DecodeInvalidProposalProof(proof []byte) (InvalidProposalProof, error) {
	var decoded InvalidProposalProof
	if err := json.Unmarshal(proof, &decoded); err != nil {
		return InvalidProposalProof{}, err
	}
	return decoded, nil
}

func DecodeUnavailableDataProof(proof []byte) (UnavailableDataProof, error) {
	var decoded UnavailableDataProof
	if err := json.Unmarshal(proof, &decoded); err != nil {
		return UnavailableDataProof{}, err
	}
	return decoded, nil
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

func VerifyInvalidProposalEvidence(evidence slashing.Evidence) error {
	if evidence.Type != slashing.EvidenceInvalidProposal {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		return err
	}
	if err := verifyInvalidProposalEnvelope(decoded, evidence.Validator, evidence.Height, evidence.Round); err != nil {
		return err
	}
	switch decoded.Reason {
	case InvalidProposalReasonDAMismatch, InvalidProposalReasonMissingData:
	default:
		return ErrInvalidProposalContext
	}
	if decoded.ContextProofHash != (types.Hash{}) {
		return ErrInvalidProposalContext
	}
	return verifyInvalidProposalByReason(decoded)
}

func VerifyInvalidProposalEvidenceWithContext(evidence slashing.Evidence, context InvalidProposalVerificationContext) error {
	if evidence.Type != slashing.EvidenceInvalidProposal {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		return err
	}
	if err := verifyInvalidProposalEnvelope(decoded, evidence.Validator, evidence.Height, evidence.Round); err != nil {
		return err
	}
	switch decoded.Reason {
	case InvalidProposalReasonValidatorSetHash:
		if context.ExpectedValidatorSetHash == (types.Hash{}) {
			return ErrInvalidProposalContext
		}
		if decoded.ExpectedHash != context.ExpectedValidatorSetHash {
			return ErrInvalidProposal
		}
	case InvalidProposalReasonAppHash:
		if context.ExpectedAppHash == (types.Hash{}) {
			return ErrInvalidProposalContext
		}
		if decoded.ExpectedHash != context.ExpectedAppHash {
			return ErrInvalidProposal
		}
	case InvalidProposalReasonTimestamp:
		if context.ExpectedTimeUnixNano == 0 {
			return ErrInvalidProposalContext
		}
		if decoded.ExpectedTimeUnixNano != context.ExpectedTimeUnixNano {
			return ErrInvalidProposal
		}
	case InvalidProposalReasonTxValidity:
		if context.ExpectedTxResultsHash == (types.Hash{}) {
			return ErrInvalidProposalContext
		}
		if decoded.ExpectedHash != context.ExpectedTxResultsHash {
			return ErrInvalidProposal
		}
	}
	if decoded.ContextProofHash != (types.Hash{}) {
		computedContextProofHash := context.ProofHash()
		if context.ContextProofHash == (types.Hash{}) || computedContextProofHash == (types.Hash{}) {
			return ErrInvalidProposalContext
		}
		if context.ContextProofHash != computedContextProofHash {
			return ErrInvalidProposalContext
		}
		if decoded.ContextProofHash != computedContextProofHash {
			return ErrInvalidProposal
		}
	} else if context.ContextProofHash != (types.Hash{}) && context.ContextProofHash != context.ProofHash() {
		return ErrInvalidProposalContext
	}
	return verifyInvalidProposalByReason(decoded)
}

func HashTxResults(results []types.Result) types.Hash {
	hasher := sha256.New()
	writeResultUint64(hasher, uint64(len(results)))
	for _, result := range results {
		writeResultUint64(hasher, uint64(result.Code))
		writeResultBytes(hasher, []byte(result.Log))
		writeResultBytes(hasher, result.Data)
		writeResultUint64(hasher, result.GasUsed)
		writeResultUint64(hasher, result.FeePaid)
	}
	var out types.Hash
	copy(out[:], hasher.Sum(nil))
	return out
}

func writeResultBytes(hasher interface{ Write([]byte) (int, error) }, data []byte) {
	writeResultUint64(hasher, uint64(len(data)))
	_, _ = hasher.Write(data)
}

func writeResultUint64(hasher interface{ Write([]byte) (int, error) }, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	_, _ = hasher.Write(buffer[:])
}

func validInvalidProposalReason(reason InvalidProposalReason) bool {
	for _, supported := range SupportedInvalidProposalReasons() {
		if reason == supported {
			return true
		}
	}
	return false
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

package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/queryproof"
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

var builtinInvalidProposalReasons = []InvalidProposalReason{
	InvalidProposalReasonDAMismatch,
	InvalidProposalReasonMissingData,
	InvalidProposalReasonValidatorSetHash,
	InvalidProposalReasonAppHash,
	InvalidProposalReasonTxValidity,
	InvalidProposalReasonTimestamp,
}

type InvalidProposalVerifier func(InvalidProposalProof, InvalidProposalVerificationContext) error

type InvalidProposalVerifierOptions struct {
	RequireContext bool
}

type invalidProposalVerifierEntry struct {
	verifier       InvalidProposalVerifier
	requireContext bool
}

var invalidProposalVerifierRegistry = struct {
	sync.RWMutex
	entries map[InvalidProposalReason]invalidProposalVerifierEntry
}{
	entries: make(map[InvalidProposalReason]invalidProposalVerifierEntry),
}

func SupportedInvalidProposalReasons() []InvalidProposalReason {
	reasons := append([]InvalidProposalReason(nil), builtinInvalidProposalReasons...)
	custom := registeredInvalidProposalReasons()
	sort.Slice(custom, func(i int, j int) bool { return custom[i] < custom[j] })
	return append(reasons, custom...)
}

func RegisterInvalidProposalVerifier(reason InvalidProposalReason, verifier InvalidProposalVerifier) error {
	return RegisterInvalidProposalVerifierWithOptions(reason, verifier, InvalidProposalVerifierOptions{})
}

func RegisterInvalidProposalVerifierWithOptions(reason InvalidProposalReason, verifier InvalidProposalVerifier, options InvalidProposalVerifierOptions) error {
	if reason == "" || verifier == nil || isBuiltinInvalidProposalReason(reason) {
		return ErrUnsupportedProposalReason
	}
	invalidProposalVerifierRegistry.Lock()
	defer invalidProposalVerifierRegistry.Unlock()
	if _, exists := invalidProposalVerifierRegistry.entries[reason]; exists {
		return ErrUnsupportedProposalReason
	}
	invalidProposalVerifierRegistry.entries[reason] = invalidProposalVerifierEntry{
		verifier:       verifier,
		requireContext: options.RequireContext,
	}
	return nil
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
	StateProof           *queryproof.Proof     `json:"state_proof,omitempty"`
	ExecutionProof       *TxExecutionProof     `json:"execution_proof,omitempty"`
	ExpectedTimeUnixNano int64                 `json:"expected_time_unix_nano,omitempty"`
	ActualTimeUnixNano   int64                 `json:"actual_time_unix_nano,omitempty"`
	VerificationMessage  string                `json:"verification_message,omitempty"`
}

type TxExecutionProof struct {
	TxIndex         uint64         `json:"tx_index"`
	Tx              types.Tx       `json:"tx"`
	ExpectedResults []types.Result `json:"expected_results"`
	ActualResults   []types.Result `json:"actual_results"`
}

type InvalidProposalVerificationContext struct {
	ExpectedValidatorSetHash types.Hash
	ExpectedAppHash          types.Hash
	ExpectedTxResultsHash    types.Hash
	ExpectedTxResults        []types.Result
	ActualTxResults          []types.Result
	TxIndex                  uint64
	ContextProofHash         types.Hash
	ChainID                  string
	ExpectedStateRoot        types.Hash
	ExpectedProofNamespace   string
	ExpectedProofKey         []byte
	ExpectedProofValue       []byte
	ExpectedProofExists      *bool
	RequireStateProof        bool
	ExpectedTimeUnixNano     int64
}

func (context InvalidProposalVerificationContext) ProofHash() types.Hash {
	if context.ExpectedValidatorSetHash == (types.Hash{}) &&
		context.ExpectedAppHash == (types.Hash{}) &&
		context.ExpectedTxResultsHash == (types.Hash{}) &&
		len(context.ExpectedTxResults) == 0 &&
		len(context.ActualTxResults) == 0 &&
		context.TxIndex == 0 &&
		context.ExpectedStateRoot == (types.Hash{}) &&
		context.ChainID == "" &&
		context.ExpectedProofNamespace == "" &&
		len(context.ExpectedProofKey) == 0 &&
		len(context.ExpectedProofValue) == 0 &&
		context.ExpectedProofExists == nil &&
		!context.RequireStateProof &&
		context.ExpectedTimeUnixNano == 0 {
		return types.Hash{}
	}
	hasher := sha256.New()
	hasher.Write([]byte("vexo.invalid_proposal.context.v1"))
	hasher.Write(context.ExpectedValidatorSetHash[:])
	hasher.Write(context.ExpectedAppHash[:])
	hasher.Write(context.ExpectedTxResultsHash[:])
	hasher.Write(context.ExpectedStateRoot[:])
	writeResultBytes(hasher, []byte(context.ChainID))
	writeResultBytes(hasher, []byte(context.ExpectedProofNamespace))
	writeResultBytes(hasher, context.ExpectedProofKey)
	writeResultBytes(hasher, context.ExpectedProofValue)
	if context.ExpectedProofExists != nil {
		writeResultUint64(hasher, 1)
		if *context.ExpectedProofExists {
			writeResultUint64(hasher, 1)
		} else {
			writeResultUint64(hasher, 0)
		}
	} else {
		writeResultUint64(hasher, 0)
	}
	if context.RequireStateProof {
		writeResultUint64(hasher, 1)
	} else {
		writeResultUint64(hasher, 0)
	}
	writeResultUint64(hasher, uint64(context.ExpectedTimeUnixNano))
	if len(context.ExpectedTxResults) > 0 || len(context.ActualTxResults) > 0 {
		expectedResultsHash := HashTxResults(context.ExpectedTxResults)
		actualResultsHash := HashTxResults(context.ActualTxResults)
		hasher.Write(expectedResultsHash[:])
		hasher.Write(actualResultsHash[:])
		writeResultUint64(hasher, context.TxIndex)
	}
	var out types.Hash
	copy(out[:], hasher.Sum(nil))
	return out
}

func BindInvalidProposalEvidenceContext(evidence slashing.Evidence, context InvalidProposalVerificationContext) (slashing.Evidence, error) {
	if evidence.Type != slashing.EvidenceInvalidProposal {
		return slashing.Evidence{}, slashing.ErrUnknownEvidenceType
	}
	proof, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	if err := verifyInvalidProposalEnvelope(proof, evidence.Validator, evidence.Height, evidence.Round); err != nil {
		return slashing.Evidence{}, err
	}
	contextProofHash := context.ProofHash()
	if contextProofHash == (types.Hash{}) {
		return slashing.Evidence{}, ErrInvalidProposalContext
	}
	proof.ContextProofHash = contextProofHash
	encoded, err := json.Marshal(proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	evidence.Proof = encoded
	return evidence, nil
}

func BindInvalidProposalEvidenceStateProof(evidence slashing.Evidence, context InvalidProposalVerificationContext, stateProof queryproof.Proof) (slashing.Evidence, error) {
	if evidence.Type != slashing.EvidenceInvalidProposal {
		return slashing.Evidence{}, slashing.ErrUnknownEvidenceType
	}
	proof, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	if err := verifyInvalidProposalEnvelope(proof, evidence.Validator, evidence.Height, evidence.Round); err != nil {
		return slashing.Evidence{}, err
	}
	if context.ExpectedStateRoot == (types.Hash{}) {
		return slashing.Evidence{}, ErrInvalidProposalContext
	}
	context.RequireStateProof = true
	if contextProofHash := context.ProofHash(); contextProofHash != (types.Hash{}) {
		proof.ContextProofHash = contextProofHash
	}
	proof.StateProof = cloneQueryProof(stateProof)
	encoded, err := json.Marshal(proof)
	if err != nil {
		return slashing.Evidence{}, err
	}
	evidence.Proof = encoded
	return evidence, nil
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

func NewDataAvailabilityInvalidProposalEvidence(proposal Proposal, reason InvalidProposalReason) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 || reason == "" {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if !validInvalidProposalReason(reason) {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	if reason != InvalidProposalReasonDAMismatch && reason != InvalidProposalReasonMissingData {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	expectedCommitment := dataavailability.Commitment(proposal.Block.Txs)
	invalidProposalProof := InvalidProposalProof{
		Proposal:     proposal,
		Reason:       reason,
		ExpectedHash: expectedCommitment,
		ActualHash:   proposal.Block.Header.ConsensusHash,
	}
	if err := verifyInvalidProposalByReason(invalidProposalProof); err != nil {
		return slashing.Evidence{}, err
	}
	proof, err := json.Marshal(invalidProposalProof)
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

// NewInvalidProposalEvidence builds legacy data-availability invalid proposal evidence.
//
// Deprecated: use NewDataAvailabilityInvalidProposalEvidence for data-availability
// reasons, or a reason-specific constructor such as NewInvalidProposalHashEvidence,
// NewInvalidProposalTimestampEvidence, NewInvalidProposalTxValidityEvidence, or
// NewInvalidProposalEvidenceWithContext for context-bound proposal validation.
func NewInvalidProposalEvidence(proposal Proposal, reason string) (slashing.Evidence, error) {
	return NewDataAvailabilityInvalidProposalEvidence(proposal, InvalidProposalReason(reason))
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

func NewInvalidProposalTxExecutionEvidence(proposal Proposal, expectedResults []types.Result, actualResults []types.Result, txIndex uint64, message string) (slashing.Evidence, error) {
	if proposal.Proposer == "" || proposal.Block.Header.Height == 0 {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if message == "" || len(expectedResults) == 0 || len(actualResults) == 0 || len(expectedResults) != len(proposal.Block.Txs) || len(actualResults) != len(proposal.Block.Txs) || txIndex >= uint64(len(proposal.Block.Txs)) {
		return slashing.Evidence{}, ErrInvalidProposal
	}
	expectedHash := HashTxResults(expectedResults)
	actualHash := HashTxResults(actualResults)
	proof := InvalidProposalProof{
		Proposal:            proposal,
		Reason:              InvalidProposalReasonTxValidity,
		ExpectedHash:        expectedHash,
		ActualHash:          actualHash,
		VerificationMessage: message,
		ExecutionProof: &TxExecutionProof{
			TxIndex:         txIndex,
			Tx:              append(types.Tx(nil), proposal.Block.Txs[txIndex]...),
			ExpectedResults: cloneResults(expectedResults),
			ActualResults:   cloneResults(actualResults),
		},
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

func NewInvalidProposalEvidenceWithContext(proposal Proposal, context InvalidProposalVerificationContext, reason InvalidProposalReason, actual types.Hash, message string) (slashing.Evidence, error) {
	var evidence slashing.Evidence
	var err error
	switch reason {
	case InvalidProposalReasonValidatorSetHash:
		evidence, err = NewInvalidProposalHashEvidence(proposal, string(reason), context.ExpectedValidatorSetHash, actual)
	case InvalidProposalReasonAppHash:
		evidence, err = NewInvalidProposalHashEvidence(proposal, string(reason), context.ExpectedAppHash, actual)
	case InvalidProposalReasonTxValidity:
		expectedResultsHash := context.ExpectedTxResultsHash
		if expectedResultsHash == (types.Hash{}) && len(context.ExpectedTxResults) > 0 {
			expectedResultsHash = HashTxResults(context.ExpectedTxResults)
		}
		if expectedResultsHash == (types.Hash{}) ||
			len(context.ExpectedTxResults) == 0 ||
			len(context.ActualTxResults) == 0 {
			return slashing.Evidence{}, ErrInvalidProposalContext
		}
		actualResultsHash := HashTxResults(context.ActualTxResults)
		if actual != (types.Hash{}) && actual != actualResultsHash {
			return slashing.Evidence{}, ErrInvalidProposal
		}
		evidence, err = NewInvalidProposalTxExecutionEvidence(proposal, context.ExpectedTxResults, context.ActualTxResults, context.TxIndex, message)
	case InvalidProposalReasonTimestamp:
		evidence, err = NewInvalidProposalTimestampEvidence(proposal, context.ExpectedTimeUnixNano, proposal.Block.Header.TimeUnixNano)
	case InvalidProposalReasonDAMismatch, InvalidProposalReasonMissingData:
		return NewDataAvailabilityInvalidProposalEvidence(proposal, reason)
	default:
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	if err != nil {
		return slashing.Evidence{}, err
	}
	if invalidProposalReasonRequiresContext(reason) {
		return BindInvalidProposalEvidenceContext(evidence, context)
	}
	return evidence, nil
}

func NewInvalidProposalEvidenceWithStateProof(proposal Proposal, context InvalidProposalVerificationContext, reason InvalidProposalReason, actual types.Hash, stateProof queryproof.Proof, message string) (slashing.Evidence, error) {
	evidence, err := NewInvalidProposalEvidenceWithContext(proposal, context, reason, actual, message)
	if err != nil {
		return slashing.Evidence{}, err
	}
	return BindInvalidProposalEvidenceStateProof(evidence, context, stateProof)
}

func NewCustomInvalidProposalEvidence(proposal Proposal, reason InvalidProposalReason, expected types.Hash, actual types.Hash, message string) (slashing.Evidence, error) {
	return NewCustomInvalidProposalEvidenceWithContext(proposal, InvalidProposalVerificationContext{}, reason, expected, actual, message)
}

func NewCustomInvalidProposalEvidenceWithContext(proposal Proposal, context InvalidProposalVerificationContext, reason InvalidProposalReason, expected types.Hash, actual types.Hash, message string) (slashing.Evidence, error) {
	if !isCustomInvalidProposalReason(reason) {
		return slashing.Evidence{}, ErrUnsupportedProposalReason
	}
	proof := InvalidProposalProof{
		Proposal:            proposal,
		Reason:              reason,
		ExpectedHash:        expected,
		ActualHash:          actual,
		VerificationMessage: message,
	}
	evidence, err := newInvalidProposalEvidenceFromProofWithContext(proof, context)
	if err != nil {
		return slashing.Evidence{}, err
	}
	if invalidProposalReasonRequiresContext(reason) || context.ProofHash() != (types.Hash{}) {
		return BindInvalidProposalEvidenceContext(evidence, context)
	}
	return evidence, nil
}

func newInvalidProposalEvidenceFromProof(proof InvalidProposalProof) (slashing.Evidence, error) {
	return newInvalidProposalEvidenceFromProofWithContext(proof, InvalidProposalVerificationContext{})
}

func newInvalidProposalEvidenceFromProofWithContext(proof InvalidProposalProof, context InvalidProposalVerificationContext) (slashing.Evidence, error) {
	if proof.Proposal.Proposer == "" || proof.Proposal.Block.Header.Height == 0 {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	if err := verifyInvalidProposalEnvelope(proof, proof.Proposal.Proposer, proof.Proposal.Block.Header.Height, proof.Proposal.Round); err != nil {
		return slashing.Evidence{}, err
	}
	if err := verifyInvalidProposalByReasonWithContext(proof, context); err != nil {
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
	return verifyInvalidProposalByReasonWithContext(decoded, InvalidProposalVerificationContext{})
}

func verifyInvalidProposalByReasonWithContext(decoded InvalidProposalProof, context InvalidProposalVerificationContext) error {
	switch decoded.Reason {
	case InvalidProposalReasonDAMismatch:
		if err := verifyDACommitmentProof(decoded); err != nil {
			return err
		}
		err := dataavailability.Verify(decoded.Proposal.Block.Header, decoded.Proposal.Block.Txs)
		if !errors.Is(err, dataavailability.ErrCommitmentMismatch) {
			if err == nil {
				return ErrInvalidProposal
			}
			return err
		}
		return nil
	case InvalidProposalReasonMissingData:
		if err := verifyDACommitmentProof(decoded); err != nil {
			return err
		}
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
		if err := verifyTxValidityMismatch(decoded); err != nil {
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
		if verifier, ok := invalidProposalVerifier(decoded.Reason); ok {
			return verifier(decoded, context)
		}
		return ErrUnsupportedProposalReason
	}
}

func verifyDACommitmentProof(decoded InvalidProposalProof) error {
	expectedCommitment := dataavailability.Commitment(decoded.Proposal.Block.Txs)
	actualCommitment := decoded.Proposal.Block.Header.ConsensusHash
	if decoded.ExpectedHash == (types.Hash{}) ||
		decoded.ExpectedHash != expectedCommitment ||
		decoded.ActualHash != actualCommitment {
		return ErrInvalidProposal
	}
	if decoded.Reason == InvalidProposalReasonDAMismatch && decoded.ExpectedHash == decoded.ActualHash {
		return ErrInvalidProposal
	}
	if decoded.Reason == InvalidProposalReasonMissingData && decoded.ActualHash != (types.Hash{}) {
		return ErrInvalidProposal
	}
	return nil
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

func verifyTxValidityMismatch(decoded InvalidProposalProof) error {
	if decoded.ExpectedHash == (types.Hash{}) ||
		decoded.ActualHash == (types.Hash{}) ||
		decoded.ExpectedHash == decoded.ActualHash {
		return ErrInvalidProposal
	}
	if decoded.ExecutionProof == nil {
		if decoded.ActualHash != txSetHash(decoded.Proposal.Block.Txs) {
			return ErrInvalidProposal
		}
		return nil
	}
	proof := decoded.ExecutionProof
	if proof.TxIndex >= uint64(len(decoded.Proposal.Block.Txs)) ||
		!bytes.Equal(proof.Tx, decoded.Proposal.Block.Txs[proof.TxIndex]) ||
		len(proof.ExpectedResults) != len(decoded.Proposal.Block.Txs) ||
		len(proof.ActualResults) != len(decoded.Proposal.Block.Txs) {
		return ErrInvalidProposal
	}
	if HashTxResults(proof.ExpectedResults) != decoded.ExpectedHash ||
		HashTxResults(proof.ActualResults) != decoded.ActualHash {
		return ErrInvalidProposal
	}
	if sameResult(proof.ExpectedResults[proof.TxIndex], proof.ActualResults[proof.TxIndex]) {
		return ErrInvalidProposal
	}
	return nil
}

func cloneResults(results []types.Result) []types.Result {
	if len(results) == 0 {
		return nil
	}
	cloned := make([]types.Result, len(results))
	for index, result := range results {
		cloned[index] = cloneResult(result)
	}
	return cloned
}

func cloneResult(result types.Result) types.Result {
	result.Data = append([]byte(nil), result.Data...)
	return result
}

func sameResult(left types.Result, right types.Result) bool {
	return left.Code == right.Code &&
		left.Log == right.Log &&
		bytes.Equal(left.Data, right.Data) &&
		left.GasUsed == right.GasUsed &&
		left.FeePaid == right.FeePaid
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

func cloneQueryProof(proof queryproof.Proof) *queryproof.Proof {
	encoded, err := queryproof.Encode(proof)
	if err != nil {
		cloned := proof
		return &cloned
	}
	cloned, err := queryproof.Decode(encoded)
	if err != nil {
		cloned := proof
		return &cloned
	}
	return &cloned
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
		if !isCustomInvalidProposalReason(decoded.Reason) {
			return ErrInvalidProposalContext
		}
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
	if err := verifyInvalidProposalBoundContext(decoded, context); err != nil {
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
		if decoded.ExecutionProof == nil {
			return ErrInvalidProposal
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
	if err := verifyInvalidProposalStateProof(decoded, context, evidence.Height); err != nil {
		return err
	}
	return verifyInvalidProposalByReasonWithContext(decoded, context)
}

func VerifyInvalidProposalEvidenceWithBoundContext(evidence slashing.Evidence, context InvalidProposalVerificationContext) error {
	if evidence.Type != slashing.EvidenceInvalidProposal {
		return slashing.ErrUnknownEvidenceType
	}
	return VerifyInvalidProposalEvidenceWithContext(evidence, context)
}

func invalidProposalReasonRequiresContext(reason InvalidProposalReason) bool {
	switch reason {
	case InvalidProposalReasonValidatorSetHash,
		InvalidProposalReasonAppHash,
		InvalidProposalReasonTimestamp,
		InvalidProposalReasonTxValidity:
		return true
	default:
		entry, ok := invalidProposalVerifierEntryFor(reason)
		return ok && entry.requireContext
	}
}

func verifyInvalidProposalBoundContext(decoded InvalidProposalProof, context InvalidProposalVerificationContext) error {
	if !invalidProposalReasonRequiresContext(decoded.Reason) {
		return nil
	}
	if decoded.ContextProofHash == (types.Hash{}) || context.ContextProofHash == (types.Hash{}) {
		return ErrInvalidProposalContext
	}
	computed := context.ProofHash()
	if computed == (types.Hash{}) || context.ContextProofHash != computed {
		return ErrInvalidProposalContext
	}
	if decoded.ContextProofHash != computed {
		return ErrInvalidProposal
	}
	return nil
}

func verifyInvalidProposalStateProof(decoded InvalidProposalProof, context InvalidProposalVerificationContext, height types.Height) error {
	if decoded.StateProof == nil {
		if context.RequireStateProof {
			return ErrInvalidProposalContext
		}
		return nil
	}
	if context.ExpectedStateRoot == (types.Hash{}) {
		return ErrInvalidProposalContext
	}
	if err := queryproof.Verify(*decoded.StateProof, context.ChainID, height, context.ExpectedStateRoot); err != nil {
		return err
	}
	if context.ExpectedProofNamespace != "" && decoded.StateProof.Namespace != context.ExpectedProofNamespace {
		return ErrInvalidProposal
	}
	if len(context.ExpectedProofKey) > 0 && !bytes.Equal(decoded.StateProof.Key, context.ExpectedProofKey) {
		return ErrInvalidProposal
	}
	if context.ExpectedProofExists != nil && decoded.StateProof.Exists != *context.ExpectedProofExists {
		return ErrInvalidProposal
	}
	if len(context.ExpectedProofValue) > 0 && !bytes.Equal(decoded.StateProof.Value, context.ExpectedProofValue) {
		return ErrInvalidProposal
	}
	return nil
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
	return isBuiltinInvalidProposalReason(reason) || isCustomInvalidProposalReason(reason)
}

func isBuiltinInvalidProposalReason(reason InvalidProposalReason) bool {
	for _, supported := range builtinInvalidProposalReasons {
		if reason == supported {
			return true
		}
	}
	return false
}

func isCustomInvalidProposalReason(reason InvalidProposalReason) bool {
	_, ok := invalidProposalVerifierEntryFor(reason)
	return ok
}

func invalidProposalVerifier(reason InvalidProposalReason) (InvalidProposalVerifier, bool) {
	entry, ok := invalidProposalVerifierEntryFor(reason)
	if !ok {
		return nil, false
	}
	return entry.verifier, true
}

func invalidProposalVerifierEntryFor(reason InvalidProposalReason) (invalidProposalVerifierEntry, bool) {
	invalidProposalVerifierRegistry.RLock()
	defer invalidProposalVerifierRegistry.RUnlock()
	entry, ok := invalidProposalVerifierRegistry.entries[reason]
	return entry, ok
}

func registeredInvalidProposalReasons() []InvalidProposalReason {
	invalidProposalVerifierRegistry.RLock()
	defer invalidProposalVerifierRegistry.RUnlock()
	reasons := make([]InvalidProposalReason, 0, len(invalidProposalVerifierRegistry.entries))
	for reason := range invalidProposalVerifierRegistry.entries {
		reasons = append(reasons, reason)
	}
	return reasons
}

func unregisterInvalidProposalVerifierForTest(reason InvalidProposalReason) {
	invalidProposalVerifierRegistry.Lock()
	defer invalidProposalVerifierRegistry.Unlock()
	delete(invalidProposalVerifierRegistry.entries, reason)
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

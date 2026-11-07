package consensus

import (
	"errors"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrEvidenceValidatorNotFound       = errors.New("evidence validator not found")
	ErrEvidenceSignatureVerifier       = errors.New("evidence signature verifier is required")
	ErrEvidenceFinalityVerifier        = errors.New("evidence finality verifier is required")
	ErrUnsupportedEvidenceProof        = errors.New("unsupported evidence proof")
	ErrInvalidEvidenceVoteSignature    = errors.New("invalid evidence vote signature")
	ErrInvalidEvidenceTimeoutSignature = errors.New("invalid evidence timeout vote signature")
	// ErrEvidenceNotSelfContained rejects evidence whose truth depends on data outside the proof and
	// the validator set of its height, such as the application hash or the block timestamp of a
	// height. Such evidence cannot be judged identically by every node, in particular by one that
	// pruned the data, so block execution must not act on it.
	ErrEvidenceNotSelfContained = errors.New("evidence is not decidable from the proof and the validator set")
)

type EvidenceSignatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

type EvidenceAggregateVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

type EvidenceVerifierBundle struct {
	Signatures EvidenceSignatureVerifier
	Finality   EvidenceAggregateVerifier
	// FinalitySets serves the validator sets the headers of a finality conflict name. Without it
	// a conflict is judged with the set of the evidence height alone.
	FinalitySets finality.ValidatorSetSource
}

func NewEvidenceVerifier(signatures EvidenceSignatureVerifier, finalityVerifier EvidenceAggregateVerifier) EvidenceVerifierBundle {
	return EvidenceVerifierBundle{Signatures: signatures, Finality: finalityVerifier}
}

func (verifier EvidenceVerifierBundle) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	return verifier.Signatures != nil && verifier.Signatures.Verify(publicKey, message, signature)
}

func (verifier EvidenceVerifierBundle) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return verifier.Finality != nil && verifier.Finality.VerifyAggregate(publicKeys, message, signature)
}

// VerifyEvidence checks that evidence proves a fault of evidence.Validator, judging only from the
// proof and validatorSet, the validator set in effect at evidence.Height. It has no side effects.
// Evidence that is already normalized or still carries the legacy double_sign label is accepted.
//
// The accepted faults are decidable from those two inputs alone: conflicting votes, conflicting
// timeout votes, a finality conflict, an unavailable-data proposal and an invalid proposal whose
// invalidity is a data-availability or validator-set-hash mismatch. An invalid proposal that has
// to be compared with other chain data is ErrEvidenceNotSelfContained. Deciding it from node-local
// data would let a node that pruned that data disagree with the others about a block.
//
// For the BLS backend, validatorSet's keys must have been registered with the crypto suite behind
// verifier, which verifies their proofs of possession.
func VerifyEvidence(evidence slashing.Evidence, validatorSet validator.Set, verifier EvidenceVerifierBundle) error {
	evidence = evidence.Normalize()
	validatorInfo, found := validatorSet.Get(evidence.Validator)
	if !found {
		return ErrEvidenceValidatorNotFound
	}
	switch evidence.Type {
	case slashing.EvidenceConflictingVote:
		if err := VerifyConflictingVoteEvidence(evidence); err != nil {
			return err
		}
		decoded, err := DecodeConflictingVoteProof(evidence.Proof)
		if err != nil {
			return err
		}
		if verifier.Signatures == nil {
			return ErrEvidenceSignatureVerifier
		}
		if !verifyDomainSignature(verifier, validatorInfo.PublicKey, vexocrypto.DomainConsensusVote, VoteSignBytes(decoded.First), decoded.First.Signature) ||
			!verifyDomainSignature(verifier, validatorInfo.PublicKey, vexocrypto.DomainConsensusVote, VoteSignBytes(decoded.Second), decoded.Second.Signature) {
			return ErrInvalidEvidenceVoteSignature
		}
		return nil
	case slashing.EvidenceConflictingTimeoutVote:
		if err := VerifyConflictingTimeoutVoteEvidence(evidence); err != nil {
			return err
		}
		decoded, err := DecodeConflictingTimeoutVoteProof(evidence.Proof)
		if err != nil {
			return err
		}
		if verifier.Signatures == nil {
			return ErrEvidenceSignatureVerifier
		}
		if !verifyDomainSignature(verifier, validatorInfo.PublicKey, vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(decoded.First), decoded.First.Signature) ||
			!verifyDomainSignature(verifier, validatorInfo.PublicKey, vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(decoded.Second), decoded.Second.Signature) {
			return ErrInvalidEvidenceTimeoutSignature
		}
		return nil
	case slashing.EvidenceInvalidProposal:
		decoded, err := DecodeInvalidProposalProof(evidence.Proof)
		if err != nil {
			return err
		}
		switch decoded.Reason {
		case InvalidProposalReasonDAMismatch, InvalidProposalReasonMissingData:
			if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{}); err != nil {
				return err
			}
		case InvalidProposalReasonValidatorSetHash:
			context := InvalidProposalVerificationContext{ExpectedValidatorSetHash: validatorSet.Hash()}
			context.ContextProofHash = context.ProofHash()
			if err := VerifyInvalidProposalEvidenceWithBoundContext(evidence, context); err != nil {
				return err
			}
		default:
			return ErrEvidenceNotSelfContained
		}
		return verifyProposalEvidenceSignature(decoded.Proposal, validatorInfo, verifier)
	case slashing.EvidenceUnavailableData:
		if err := VerifyUnavailableDataEvidence(evidence); err != nil {
			return err
		}
		decoded, err := DecodeUnavailableDataProof(evidence.Proof)
		if err != nil {
			return err
		}
		return verifyProposalEvidenceSignature(decoded.Proposal, validatorInfo, verifier)
	case slashing.EvidenceFinalityConflict:
		if verifier.Finality == nil {
			return ErrEvidenceFinalityVerifier
		}
		sets := verifier.FinalitySets
		if sets == nil {
			sets = finality.StaticValidatorSets(validatorSet)
		}
		_, err := finality.VerifyConflictEvidence(sets, verifier, evidence)
		return err
	default:
		return ErrUnsupportedEvidenceProof
	}
}

func verifyProposalEvidenceSignature(proposal Proposal, validatorInfo validator.Validator, verifier EvidenceSignatureVerifier) error {
	if verifier == nil {
		return ErrEvidenceSignatureVerifier
	}
	if !verifyDomainSignature(verifier, validatorInfo.PublicKey, vexocrypto.DomainConsensusProposal, ProposalSignBytes(proposal), proposal.Signature) {
		return ErrInvalidProposal
	}
	return nil
}

func verifyDomainSignature(verifier EvidenceSignatureVerifier, publicKey types.PublicKey, domain vexocrypto.Domain, message []byte, signature types.Signature) bool {
	if len(signature) == 0 {
		return false
	}
	domainMessage, err := vexocrypto.DomainMessage(domain, message)
	if err != nil {
		return false
	}
	return verifier.Verify(publicKey, domainMessage, signature)
}

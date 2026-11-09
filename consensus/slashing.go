package consensus

import (
	"context"
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
	// ErrEvidenceNotSelfContained rejects app-hash evidence judged without the certified app hashes
	// of replicated state (EvidenceVerifierBundle.CertifiedAppHashes): judged from node-local data,
	// nodes could disagree about it, so block execution must not act on it.
	ErrEvidenceNotSelfContained = errors.New("evidence is not decidable from the proof and replicated state")
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
	// CertifiedAppHashes serves the app hash certified blocks committed for a state height
	// (finality.AppHashHeight), from replicated state. Without it app-hash evidence is not judged.
	CertifiedAppHashes CertifiedAppHashSource
}

// CertifiedAppHashSource serves the header app hash certified blocks committed for the state of a
// height; found is false when no retained certified block named that height.
type CertifiedAppHashSource interface {
	CertifiedAppHash(height types.Height) (hash types.Hash, found bool, err error)
}

// ErrAppHashNotCertified reports app-hash evidence about a state height no retained certified
// block committed an app hash for: there is nothing to compare the proposal with.
var ErrAppHashNotCertified = errors.New("no certified app hash for the state height the proposal names")

// validatorSets returns the source of the replicated validator sets of past registry heights:
// FinalitySets, or validatorSet for every height without it.
func (verifier EvidenceVerifierBundle) validatorSets(validatorSet validator.Set) finality.ValidatorSetSource {
	if verifier.FinalitySets != nil {
		return verifier.FinalitySets
	}
	return finality.StaticValidatorSets(validatorSet)
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
// The accepted faults are decidable from the proof and replicated state alone: conflicting votes,
// conflicting timeout votes, a finality conflict, an unavailable-data proposal, and an invalid
// proposal whose invalidity is a data-availability mismatch, a broken link to the parent header it
// names (parent_linkage), or a validator-set or next-validator-set hash other than the replicated
// set of the registry height the header names (from verifier.FinalitySets, validatorSet for every
// height without it). An app-hash mismatch is judged against verifier.CertifiedAppHashes, which
// serves replicated state, so every node decides it alike; without it the evidence is
// ErrEvidenceNotSelfContained.
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
		var expected types.Hash
		header := decoded.Proposal.Block.Header
		switch decoded.Reason {
		case InvalidProposalReasonValidatorSetHash, InvalidProposalReasonNextValidatorSetHash:
			// The set of a registry height is replicated state: a header naming another hash for
			// the set of the height it names is wrong whatever height it should have named, which
			// parent_linkage evidence judges.
			registryHeight := header.ValidatorSetHeight
			if decoded.Reason == InvalidProposalReasonNextValidatorSetHash {
				registryHeight = header.StateHeight + 1
			}
			if registryHeight == 0 || registryHeight > evidence.Height {
				return ErrInvalidProposal
			}
			set, err := verifier.validatorSets(validatorSet).ValidatorSet(context.Background(), registryHeight)
			if err != nil {
				return err
			}
			expected = set.Hash()
		case InvalidProposalReasonAppHash:
			if verifier.CertifiedAppHashes == nil {
				return ErrEvidenceNotSelfContained
			}
			stateHeight, ok := finality.AppHashHeight(decoded.Proposal.Block.Header)
			if !ok {
				return ErrInvalidProposal
			}
			certified, found, err := verifier.CertifiedAppHashes.CertifiedAppHash(stateHeight)
			if err != nil {
				return err
			}
			if !found {
				return ErrAppHashNotCertified
			}
			expected = certified
		}
		if err := VerifyInvalidProposalEvidence(evidence, expected); err != nil {
			return err
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
		sets := verifier.validatorSets(validatorSet)
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

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
)

type SlashResult struct {
	Receipt        slashing.PenaltyReceipt
	PreviousPower  types.VotingPower
	RemainingPower types.VotingPower
}

type SlashingKeeper interface {
	SubmitEvidence(ctx context.Context, evidence slashing.Evidence) error
	ApplyPenaltyWithStake(ctx context.Context, evidence slashing.Evidence, currentPower types.VotingPower) (slashing.PenaltyReceipt, error)
}

type contextPenaltyReceiptReader interface {
	PenaltyReceipt(ctx context.Context, evidence slashing.Evidence) (slashing.PenaltyReceipt, bool, error)
}

type memoryPenaltyReceiptReader interface {
	PenaltyReceipt(evidence slashing.Evidence) (slashing.PenaltyReceipt, bool)
}

type EvidenceSignatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

type EvidenceAggregateVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

type EvidenceVerifierBundle struct {
	Signatures EvidenceSignatureVerifier
	Finality   EvidenceAggregateVerifier
}

type EvidenceVerificationContext struct {
	InvalidProposal InvalidProposalVerificationContext
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

func SubmitEvidenceForSlashing(ctx context.Context, keeper SlashingKeeper, registry validator.VersionedRegistry, verifier EvidenceSignatureVerifier, applyHeight types.Height, evidence slashing.Evidence) (SlashResult, error) {
	return SubmitEvidenceForSlashingWithContext(ctx, keeper, registry, verifier, applyHeight, evidence, EvidenceVerificationContext{})
}

func SubmitEvidenceForSlashingWithContext(ctx context.Context, keeper SlashingKeeper, registry validator.VersionedRegistry, verifier EvidenceSignatureVerifier, applyHeight types.Height, evidence slashing.Evidence, verificationContext EvidenceVerificationContext) (SlashResult, error) {
	if applyHeight == 0 {
		applyHeight = evidence.Height
	}
	set, err := registry.ValidatorSet(ctx, evidence.Height)
	if err != nil {
		return SlashResult{}, err
	}
	validatorInfo, found := set.Get(evidence.Validator)
	if !found {
		return SlashResult{}, ErrEvidenceValidatorNotFound
	}
	if err := verifyConsensusEvidence(evidence, set, validatorInfo, verifier, verificationContext); err != nil {
		return SlashResult{}, err
	}
	receipt, receiptFound, err := penaltyReceipt(ctx, keeper, evidence)
	if err != nil {
		return SlashResult{}, err
	}
	applySet, err := registry.ValidatorSet(ctx, applyHeight)
	if err != nil && !receiptFound {
		return SlashResult{}, err
	}
	applyValidatorInfo, applyFound := validator.Validator{}, false
	if err == nil {
		applyValidatorInfo, applyFound = applySet.Get(evidence.Validator)
	}
	if err := keeper.SubmitEvidence(ctx, evidence); err != nil && !errors.Is(err, slashing.ErrDuplicateEvidence) {
		return SlashResult{}, err
	}
	if !receiptFound {
		basePower := validatorInfo.VotingPower
		if applyFound {
			basePower = applyValidatorInfo.VotingPower
		}
		receipt, err = keeper.ApplyPenaltyWithStake(ctx, evidence, basePower)
	}
	if err != nil {
		return SlashResult{}, err
	}
	if applyFound {
		if receipt.RemainingPower == 0 {
			if err := registry.ApplyLeaveAt(ctx, applyHeight, evidence.Validator); err != nil {
				return SlashResult{}, err
			}
		} else if applyValidatorInfo.VotingPower != receipt.RemainingPower {
			if err := registry.UpdateVotingPowerAt(ctx, applyHeight, evidence.Validator, receipt.RemainingPower); err != nil {
				return SlashResult{}, err
			}
		}
	}

	return SlashResult{
		Receipt:        receipt,
		PreviousPower:  receipt.PreviousPower,
		RemainingPower: receipt.RemainingPower,
	}, nil
}

func penaltyReceipt(ctx context.Context, keeper SlashingKeeper, evidence slashing.Evidence) (slashing.PenaltyReceipt, bool, error) {
	if reader, ok := keeper.(contextPenaltyReceiptReader); ok {
		return reader.PenaltyReceipt(ctx, evidence)
	}
	if reader, ok := keeper.(memoryPenaltyReceiptReader); ok {
		receipt, found := reader.PenaltyReceipt(evidence)
		return receipt, found, nil
	}
	return slashing.PenaltyReceipt{}, false, nil
}

func verifyConsensusEvidence(evidence slashing.Evidence, validatorSet validator.Set, validatorInfo validator.Validator, verifier EvidenceSignatureVerifier, verificationContext EvidenceVerificationContext) error {
	switch evidence.Type {
	case slashing.EvidenceDoubleSign, slashing.EvidenceConflictingVote:
		if evidence.Type == slashing.EvidenceDoubleSign {
			evidence.Type = slashing.EvidenceConflictingVote
		}
		if err := VerifyConflictingVoteEvidence(evidence); err != nil {
			return err
		}
		decoded, err := DecodeConflictingVoteProof(evidence.Proof)
		if err != nil {
			return err
		}
		if verifier == nil {
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
		if verifier == nil {
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
			if err := VerifyInvalidProposalEvidence(evidence); err != nil {
				return err
			}
		case InvalidProposalReasonValidatorSetHash:
			if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedValidatorSetHash: validatorSet.Hash()}); err != nil {
				return err
			}
		case InvalidProposalReasonAppHash:
			if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedAppHash: verificationContext.InvalidProposal.ExpectedAppHash}); err != nil {
				return err
			}
		case InvalidProposalReasonTimestamp:
			if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTimeUnixNano: verificationContext.InvalidProposal.ExpectedTimeUnixNano}); err != nil {
				return err
			}
		default:
			return ErrInvalidProposalContext
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
		aggregateVerifier, ok := verifier.(EvidenceAggregateVerifier)
		if !ok || aggregateVerifier == nil {
			return ErrEvidenceFinalityVerifier
		}
		return verifyFinalityConflictEvidence(evidence, validatorSet, aggregateVerifier)
	default:
		return ErrUnsupportedEvidenceProof
	}
}

func verifyFinalityConflictEvidence(evidence slashing.Evidence, validatorSet validator.Set, verifier EvidenceAggregateVerifier) error {
	if verifier == nil {
		return ErrEvidenceFinalityVerifier
	}
	_, err := finality.VerifyConflictEvidence(validatorSet, verifier, evidence)
	return err
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

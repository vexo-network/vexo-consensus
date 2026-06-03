package consensus

import (
	"context"
	"errors"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrEvidenceValidatorNotFound       = errors.New("evidence validator not found")
	ErrEvidenceSignatureVerifier       = errors.New("evidence signature verifier is required")
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

func SubmitEvidenceForSlashing(ctx context.Context, keeper SlashingKeeper, registry validator.VersionedRegistry, verifier EvidenceSignatureVerifier, applyHeight types.Height, evidence slashing.Evidence) (SlashResult, error) {
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
	if err := verifyConsensusEvidence(evidence, validatorInfo, verifier); err != nil {
		return SlashResult{}, err
	}
	applySet, err := registry.ValidatorSet(ctx, applyHeight)
	if err != nil {
		return SlashResult{}, err
	}
	applyValidatorInfo, found := applySet.Get(evidence.Validator)
	if !found {
		return SlashResult{}, ErrEvidenceValidatorNotFound
	}
	if err := keeper.SubmitEvidence(ctx, evidence); err != nil && !errors.Is(err, slashing.ErrDuplicateEvidence) {
		return SlashResult{}, err
	}
	receipt, found, err := penaltyReceipt(ctx, keeper, evidence)
	if err != nil {
		return SlashResult{}, err
	}
	if !found {
		receipt, err = keeper.ApplyPenaltyWithStake(ctx, evidence, applyValidatorInfo.VotingPower)
	}
	if err != nil {
		return SlashResult{}, err
	}
	if err := registry.UpdateVotingPowerAt(ctx, applyHeight, evidence.Validator, receipt.RemainingPower); err != nil {
		return SlashResult{}, err
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

func verifyConsensusEvidence(evidence slashing.Evidence, validatorInfo validator.Validator, verifier EvidenceSignatureVerifier) error {
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
	default:
		return ErrUnsupportedEvidenceProof
	}
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

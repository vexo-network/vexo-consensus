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

type EvidenceSignatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

func SubmitEvidenceForSlashing(ctx context.Context, keeper SlashingKeeper, registry validator.Registry, verifier EvidenceSignatureVerifier, evidence slashing.Evidence) (SlashResult, error) {
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
	if err := keeper.SubmitEvidence(ctx, evidence); err != nil {
		return SlashResult{}, err
	}
	receipt, err := keeper.ApplyPenaltyWithStake(ctx, evidence, validatorInfo.VotingPower)
	if err != nil {
		return SlashResult{}, err
	}
	if err := registry.UpdateVotingPower(ctx, evidence.Validator, receipt.RemainingPower); err != nil {
		return SlashResult{}, err
	}

	return SlashResult{
		Receipt:        receipt,
		PreviousPower:  validatorInfo.VotingPower,
		RemainingPower: receipt.RemainingPower,
	}, nil
}

func verifyConsensusEvidence(evidence slashing.Evidence, validatorInfo validator.Validator, verifier EvidenceSignatureVerifier) error {
	switch evidence.Type {
	case slashing.EvidenceConflictingVote:
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

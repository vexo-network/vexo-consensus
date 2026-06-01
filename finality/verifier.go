package finality

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrValidatorSetMismatch = errors.New("validator set hash mismatch")
	ErrBlockHashMismatch    = errors.New("quorum certificate block hash mismatch")
	ErrInsufficientQuorum   = errors.New("insufficient quorum voting power")
	ErrMissingQCSignature   = errors.New("quorum certificate signature is missing")
	ErrUnknownSigner        = errors.New("quorum certificate contains unknown signer")
)

type SignatureVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

type Verifier struct {
	validatorSet validator.Set
	signatures   SignatureVerifier
}

func NewVerifier(validatorSet validator.Set, signatures SignatureVerifier) Verifier {
	return Verifier{
		validatorSet: validatorSet,
		signatures:   signatures,
	}
}

func (verifier Verifier) VerifyFinalityProof(proof Proof) error {
	return verifier.VerifyFinalityProofWithContext(context.Background(), proof)
}

func (verifier Verifier) VerifyFinalityProofWithContext(ctx context.Context, proof Proof) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if proof.ValidatorSetHash != verifier.validatorSet.Hash() || proof.Header.ValidatorSetHash != verifier.validatorSet.Hash() {
		return ErrValidatorSetMismatch
	}
	if proof.QuorumCert.BlockHash != proof.HeaderHash() {
		return ErrBlockHashMismatch
	}
	if len(proof.QuorumCert.Signature) == 0 {
		return ErrMissingQCSignature
	}

	signers, err := ParseSigners(proof.QuorumCert.Signers)
	if err != nil {
		return err
	}

	var votingPower types.VotingPower
	publicKeys := make([]types.PublicKey, 0, len(signers))
	for _, signer := range signers {
		validatorInfo, found := verifier.validatorSet.Get(signer)
		if !found {
			return ErrUnknownSigner
		}
		votingPower += validatorInfo.VotingPower
		publicKeys = append(publicKeys, validatorInfo.PublicKey)
	}
	if !HasQuorum(votingPower, verifier.validatorSet.TotalVotingPower()) {
		return ErrInsufficientQuorum
	}
	if verifier.signatures != nil && !verifier.signatures.VerifyAggregate(publicKeys, proof.SignBytes(), proof.QuorumCert.Signature) {
		return ErrMissingQCSignature
	}
	return nil
}

func HasQuorum(power types.VotingPower, total types.VotingPower) bool {
	if total == 0 {
		return false
	}
	return power*3 >= total*2
}

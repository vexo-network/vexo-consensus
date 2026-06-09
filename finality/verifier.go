package finality

import (
	"context"
	"errors"
	"math/bits"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrValidatorSetMismatch = errors.New("validator set hash mismatch")
	ErrBlockHashMismatch    = errors.New("quorum certificate block hash mismatch")
	ErrHeightMismatch       = errors.New("finality proof height mismatch")
	ErrInsufficientQuorum   = errors.New("insufficient quorum voting power")
	ErrVotingPowerMismatch  = errors.New("quorum certificate voting power mismatch")
	ErrMissingQCSignature   = errors.New("quorum certificate signature is missing")
	ErrUnknownSigner        = errors.New("quorum certificate contains unknown signer")
	ErrDuplicateSigner      = errors.New("quorum certificate contains duplicate signer")
)

type SignatureVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

type Verifier struct {
	validatorSet validator.Set
	signatures   SignatureVerifier
}

type RegistryVerifier struct {
	registry   validator.Registry
	signatures SignatureVerifier
}

func NewVerifier(validatorSet validator.Set, signatures SignatureVerifier) Verifier {
	if signatures != nil {
		if wrapped, err := vexocrypto.NewDomainAggregateVerifier(signatures, vexocrypto.DomainConsensusVote); err == nil {
			signatures = wrapped
		}
	}
	return Verifier{
		validatorSet: validatorSet,
		signatures:   signatures,
	}
}

func NewRegistryVerifier(registry validator.Registry, signatures SignatureVerifier) RegistryVerifier {
	return RegistryVerifier{registry: registry, signatures: signatures}
}

func (verifier Verifier) VerifyFinalityProof(proof Proof) error {
	return verifier.VerifyFinalityProofWithContext(context.Background(), proof)
}

func (verifier RegistryVerifier) VerifyFinalityProof(proof Proof) error {
	return verifier.VerifyFinalityProofWithContext(context.Background(), proof)
}

func (verifier RegistryVerifier) VerifyFinalityProofWithContext(ctx context.Context, proof Proof) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if verifier.registry == nil {
		return ErrValidatorSetMismatch
	}
	validatorSetHeight := proof.ValidatorSetHeight
	if validatorSetHeight == 0 {
		validatorSetHeight = proof.Header.Height
	}
	validatorSet, err := verifier.registry.ValidatorSet(ctx, validatorSetHeight)
	if err != nil {
		return err
	}
	return NewVerifier(validatorSet, verifier.signatures).VerifyFinalityProofWithContext(ctx, proof)
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
	if proof.ValidatorSetHeight == 0 || proof.ValidatorSetHeight > proof.Header.Height {
		return ErrHeightMismatch
	}
	if proof.QuorumCert.Height != proof.Header.Height {
		return ErrHeightMismatch
	}
	blockHash := proof.BlockHash
	if blockHash == (types.Hash{}) {
		blockHash = proof.HeaderHash()
	}
	if proof.QuorumCert.BlockHash != blockHash {
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
	seen := make(map[types.ValidatorID]struct{}, len(signers))
	for _, signer := range signers {
		if _, found := seen[signer]; found {
			return ErrDuplicateSigner
		}
		seen[signer] = struct{}{}
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
	if proof.QuorumCert.VotingPower != 0 && proof.QuorumCert.VotingPower != votingPower {
		return ErrVotingPowerMismatch
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
	threshold := quorumThreshold(total)
	return power >= threshold
}

func quorumThreshold(total types.VotingPower) types.VotingPower {
	base := total / 3 * 2
	remainder := total % 3
	extra := types.VotingPower(0)
	if remainder > 0 {
		extra = types.VotingPower((uint64(remainder)*2 + 2) / 3)
	}
	threshold, carry := bits.Add64(uint64(base), uint64(extra), 0)
	if carry != 0 {
		return ^types.VotingPower(0)
	}
	return types.VotingPower(threshold)
}

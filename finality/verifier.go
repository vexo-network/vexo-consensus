package finality

import (
	"context"
	"errors"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrValidatorSetMismatch     = errors.New("validator set hash mismatch")
	ErrBlockHashMismatch        = errors.New("block hash mismatch")
	ErrHeightMismatch           = errors.New("finality proof height mismatch")
	ErrInsufficientQuorum       = errors.New("insufficient quorum voting power")
	ErrVotingPowerMismatch      = errors.New("quorum certificate voting power mismatch")
	ErrMissingQCSignature       = errors.New("quorum certificate signature is missing")
	ErrUnknownSigner            = errors.New("quorum certificate contains unknown signer")
	ErrDuplicateSigner          = errors.New("quorum certificate contains duplicate signer")
	ErrCommitChainTooShort      = errors.New("commit chain proof is too short")
	ErrCommitChainTooLong       = errors.New("commit chain proof is too long")
	ErrCommitChainBroken        = errors.New("commit chain proof is broken")
	ErrViewsNotConsecutive      = errors.New("commit chain views are not consecutive")
	ErrMissingSignatureVerifier = errors.New("finality signature verifier is required")
)

type SignatureVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

// Verifier verifies quorum certificates and finality proofs against one validator set. It fails
// closed: without a signature verifier every verification returns ErrMissingSignatureVerifier.
//
// A finality proof is accepted exactly when the 2-chain commit rule of consensus.TwoChainCommitRule
// holds for it: QC(B) certifies the proven block B, the single child link C extends B by one
// height, and QC(C) certifies C in the view directly after the view of QC(B). Every header names the
// validator set that certifies it; both must be the verifier's set.
type Verifier struct {
	validatorSet validator.Set
	signatures   SignatureVerifier
}

// RegistryVerifier verifies finality proofs against the validator sets the certified headers name.
// A header is certified by its own quorum, so the prover does not choose the set, and the set of the
// child must be the one the proven block fixes for it (ChildValidatorSetHeight).
type RegistryVerifier struct {
	registry   ValidatorSetSource
	signatures SignatureVerifier
}

func NewVerifier(validatorSet validator.Set, signatures SignatureVerifier) Verifier {
	return Verifier{validatorSet: validatorSet, signatures: voteDomainVerifier(signatures)}
}

func NewRegistryVerifier(registry ValidatorSetSource, signatures SignatureVerifier) RegistryVerifier {
	return RegistryVerifier{registry: registry, signatures: signatures}
}

// voteDomainVerifier verifies in the consensus vote domain. A nil verifier stays nil, which every
// verification rejects.
func voteDomainVerifier(signatures SignatureVerifier) SignatureVerifier {
	if signatures == nil {
		return nil
	}
	wrapped, err := vexocrypto.NewDomainAggregateVerifier(signatures, vexocrypto.DomainConsensusVote)
	if err != nil {
		return nil
	}
	return wrapped
}

func (verifier Verifier) VerifyFinalityProof(proof Proof) error {
	return verifier.VerifyFinalityProofWithContext(context.Background(), proof)
}

func (verifier RegistryVerifier) VerifyFinalityProof(proof Proof) error {
	return verifier.VerifyFinalityProofWithContext(context.Background(), proof)
}

// VerifyFinalityProofWithContext loads the validator sets the proven header and its child name.
func (verifier RegistryVerifier) VerifyFinalityProofWithContext(ctx context.Context, proof Proof) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if verifier.registry == nil {
		return ErrValidatorSetMismatch
	}
	child, err := checkProofStructure(proof)
	if err != nil {
		return err
	}
	blockSet, err := verifier.registry.ValidatorSet(ctx, proof.Header.ValidatorSetHeight)
	if err != nil {
		return err
	}
	childSet, err := verifier.registry.ValidatorSet(ctx, child.Header.ValidatorSetHeight)
	if err != nil {
		return err
	}
	if err := NewVerifier(blockSet, verifier.signatures).verifyCertified(ctx, proof.Header, proof.QuorumCert); err != nil {
		return err
	}
	return NewVerifier(childSet, verifier.signatures).verifyCertified(ctx, child.Header, child.QuorumCert)
}

// VerifyBlockCertWithContext verifies that cert is a quorum certificate of the validator set the
// header names, for the block with that header.
func (verifier RegistryVerifier) VerifyBlockCertWithContext(ctx context.Context, header types.Header, cert QuorumCert) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if verifier.registry == nil {
		return ErrValidatorSetMismatch
	}
	if err := checkHeaderValidatorSetHeight(header); err != nil {
		return err
	}
	validatorSet, err := verifier.registry.ValidatorSet(ctx, header.ValidatorSetHeight)
	if err != nil {
		return err
	}
	return NewVerifier(validatorSet, verifier.signatures).verifyCertified(ctx, header, cert)
}

// VerifyBlockCertWithContext verifies that cert is a quorum certificate of the validator set for
// the block with the given header: a quorum voted for the block in the view it was proposed in. It
// does not prove that the block is final; VerifyFinalityProofWithContext does.
func (verifier Verifier) VerifyBlockCertWithContext(ctx context.Context, header types.Header, cert QuorumCert) error {
	if err := checkHeaderValidatorSetHeight(header); err != nil {
		return err
	}
	return verifier.verifyCertified(ctx, header, cert)
}

// verifyCertified verifies that cert certifies header in the view the header names, with the
// verifier's set, which must be the set the header names.
func (verifier Verifier) verifyCertified(ctx context.Context, header types.Header, cert QuorumCert) error {
	if err := verifier.ready(ctx); err != nil {
		return err
	}
	if header.ValidatorSetHash != verifier.validatorSet.Hash() {
		return ErrValidatorSetMismatch
	}
	if cert.Round != header.Round {
		return ErrViewsNotConsecutive
	}
	return verifier.verifyQuorumCert(ctx, cert, header.Height, HeaderHash(header))
}

// VerifyFinalityProofWithContext verifies that the block of proof.Header is final under the
// 2-chain commit rule. The structure of the proof is checked before any signature.
func (verifier Verifier) VerifyFinalityProofWithContext(ctx context.Context, proof Proof) error {
	if err := verifier.ready(ctx); err != nil {
		return err
	}
	child, err := checkProofStructure(proof)
	if err != nil {
		return err
	}
	if err := verifier.verifyCertified(ctx, proof.Header, proof.QuorumCert); err != nil {
		return err
	}
	return verifier.verifyCertified(ctx, child.Header, child.QuorumCert)
}

// checkProofStructure checks everything about a finality proof but the signatures: the proof's
// validator set fields repeat what the header names, both headers name a set they may name, the
// child extends the proven block and names the set the proven block fixes for it, and the
// certificates are in consecutive views that match the headers.
func checkProofStructure(proof Proof) (CommitLink, error) {
	if proof.ValidatorSetHeight != proof.Header.ValidatorSetHeight {
		return CommitLink{}, ErrHeightMismatch
	}
	if proof.ValidatorSetHash != proof.Header.ValidatorSetHash {
		return CommitLink{}, ErrValidatorSetMismatch
	}
	if err := checkHeaderValidatorSetHeight(proof.Header); err != nil {
		return CommitLink{}, err
	}
	blockHash := HeaderHash(proof.Header)
	if proof.BlockHash != blockHash {
		return CommitLink{}, ErrBlockHashMismatch
	}
	if err := checkCertAnchor(proof.QuorumCert, proof.Header.Height, blockHash); err != nil {
		return CommitLink{}, err
	}
	if proof.QuorumCert.Round != proof.Header.Round {
		return CommitLink{}, ErrViewsNotConsecutive
	}
	child, err := commitChild(proof, blockHash)
	if err != nil {
		return CommitLink{}, err
	}
	if child.Header.ValidatorSetHeight != ChildValidatorSetHeight(proof.Header) {
		return CommitLink{}, ErrValidatorSetMismatch
	}
	if child.Header.JustifyRound != proof.Header.Round || child.QuorumCert.Round != child.Header.Round {
		return CommitLink{}, ErrViewsNotConsecutive
	}
	return child, nil
}

// checkHeaderValidatorSetHeight rejects a header naming a set no block of its height can have: the
// first block has the genesis set, and a later block at most the set after its grandparent.
func checkHeaderValidatorSetHeight(header types.Header) error {
	if header.ValidatorSetHeight == 0 {
		return ErrHeightMismatch
	}
	if header.Height <= 1 {
		if header.ValidatorSetHeight != FirstValidatorSetHeight {
			return ErrHeightMismatch
		}
		return nil
	}
	if header.ValidatorSetHeight > header.Height-1 {
		return ErrHeightMismatch
	}
	return nil
}

func (verifier Verifier) ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if verifier.signatures == nil {
		return ErrMissingSignatureVerifier
	}
	if verifier.validatorSet == nil {
		return ErrValidatorSetMismatch
	}
	return nil
}

// commitChild checks the structure of the 2-chain commit rule, apart from the signatures: the proof
// holds exactly one child link, the link's block extends the proven block by exactly one height,
// and its certificate is for the child in the view directly after the view of QC(B).
func commitChild(proof Proof, blockHash types.Hash) (CommitLink, error) {
	if len(proof.CommitChain) == 0 {
		return CommitLink{}, ErrCommitChainTooShort
	}
	if len(proof.CommitChain) > 1 {
		return CommitLink{}, ErrCommitChainTooLong
	}
	child := proof.CommitChain[0]
	if child.BlockHash != HeaderHash(child.Header) {
		return CommitLink{}, ErrBlockHashMismatch
	}
	if child.Header.ChainID != proof.Header.ChainID {
		return CommitLink{}, ErrCommitChainBroken
	}
	if proof.Header.Height == ^types.Height(0) || child.Header.Height != proof.Header.Height+1 {
		return CommitLink{}, ErrCommitChainBroken
	}
	if child.Header.PreviousBlockHash != blockHash {
		return CommitLink{}, ErrCommitChainBroken
	}
	if err := checkCertAnchor(child.QuorumCert, child.Header.Height, child.BlockHash); err != nil {
		return CommitLink{}, err
	}
	if proof.QuorumCert.Round == ^types.Round(0) || child.QuorumCert.Round != proof.QuorumCert.Round+1 {
		return CommitLink{}, ErrViewsNotConsecutive
	}
	return child, nil
}

func checkCertAnchor(cert QuorumCert, height types.Height, blockHash types.Hash) error {
	if cert.Height != height {
		return ErrHeightMismatch
	}
	if cert.BlockHash != blockHash {
		return ErrBlockHashMismatch
	}
	return nil
}

func (verifier Verifier) verifyQuorumCert(ctx context.Context, cert QuorumCert, height types.Height, blockHash types.Hash) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkCertAnchor(cert, height, blockHash); err != nil {
		return err
	}
	if len(cert.Signature) == 0 {
		return ErrMissingQCSignature
	}
	signers, err := ParseSigners(cert.Signers)
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
		votingPower, err = types.AddVotingPower(votingPower, validatorInfo.VotingPower)
		if err != nil {
			return err
		}
		publicKeys = append(publicKeys, validatorInfo.PublicKey)
	}
	if !HasQuorum(votingPower, verifier.validatorSet.TotalVotingPower()) {
		return ErrInsufficientQuorum
	}
	if cert.VotingPower != 0 && cert.VotingPower != votingPower {
		return ErrVotingPowerMismatch
	}
	if !verifier.signatures.VerifyAggregate(publicKeys, cert.SignBytes(), cert.Signature) {
		return ErrMissingQCSignature
	}
	return nil
}

func HasQuorum(power types.VotingPower, total types.VotingPower) bool {
	return types.HasTwoThirdsQuorum(power, total)
}

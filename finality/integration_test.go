package finality

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func deterministicSigners(t *testing.T) (vexocrypto.DeterministicSigner, vexocrypto.DeterministicSigner, vexocrypto.DeterministicSigner) {
	t.Helper()
	first, err := vexocrypto.NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := vexocrypto.NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	third, err := vexocrypto.NewDeterministicSigner([]byte("third"))
	if err != nil {
		t.Fatal(err)
	}
	return first, second, third
}

func TestVerifierAcceptsDeterministicAggregateSignature(t *testing.T) {
	firstSigner, secondSigner, thirdSigner := deterministicSigners(t)
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	signProof(t, &proof, vexocrypto.DeterministicAggregateSigner{}.Aggregate, firstSigner, secondSigner)

	verifier := NewVerifier(set, vexocrypto.DeterministicAggregateSigner{})
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierRejectsDeterministicAggregateWithWrongSignerSet(t *testing.T) {
	firstSigner, secondSigner, thirdSigner := deterministicSigners(t)
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	verifier := NewVerifier(set, vexocrypto.DeterministicAggregateSigner{})

	// The block certificate names a and b but is signed by a and c.
	wrongBlockSigners := validProof(set, []types.ValidatorID{"a", "b"})
	signProof(t, &wrongBlockSigners, vexocrypto.DeterministicAggregateSigner{}.Aggregate, firstSigner, thirdSigner)
	if err := verifier.VerifyFinalityProof(wrongBlockSigners); err == nil {
		t.Fatal("expected wrong signer aggregate to be rejected")
	}

	// Only the child certificate is signed by the wrong validators.
	wrongChildSigners := validProof(set, []types.ValidatorID{"a", "b"})
	signProof(t, &wrongChildSigners, vexocrypto.DeterministicAggregateSigner{}.Aggregate, firstSigner, secondSigner)
	childCert := &wrongChildSigners.CommitChain[0].QuorumCert
	childCert.Signature = signCert(t, *childCert, vexocrypto.DeterministicAggregateSigner{}.Aggregate, firstSigner, thirdSigner)
	if err := verifier.VerifyFinalityProof(wrongChildSigners); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected the child certificate to be verified too, got %v", err)
	}
}

// The votes sign the block hash and the view: a signature cannot be moved to another block or view.
func TestVerifierRejectsDeterministicAggregateAfterBlockHashMutation(t *testing.T) {
	firstSigner, secondSigner, thirdSigner := deterministicSigners(t)
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	signProof(t, &proof, vexocrypto.DeterministicAggregateSigner{}.Aggregate, firstSigner, secondSigner)
	verifier := NewVerifier(set, vexocrypto.DeterministicAggregateSigner{})
	if err := verifier.VerifyBlockCertWithContext(context.Background(), proof.Header, proof.QuorumCert); err != nil {
		t.Fatal(err)
	}

	// A different, internally consistent block carrying the old signature.
	forged := proof.Header
	forged.AppHash = types.Hash{9}
	forgedCert := proof.QuorumCert
	forgedCert.BlockHash = HeaderHash(forged)
	if err := verifier.VerifyBlockCertWithContext(context.Background(), forged, forgedCert); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected the signature to be bound to the original block, got %v", err)
	}

	// The same block certified in another view than the one it was proposed in.
	replayed := proof.QuorumCert
	replayed.Round++
	if err := verifier.VerifyBlockCertWithContext(context.Background(), proof.Header, replayed); !errors.Is(err, ErrViewsNotConsecutive) {
		t.Fatalf("expected the certificate to be bound to the view of the block, got %v", err)
	}
	// A header moved to that view as well no longer matches the signature.
	moved := proof.Header
	moved.Round = replayed.Round
	movedCert := replayed
	movedCert.BlockHash = HeaderHash(moved)
	if err := verifier.VerifyBlockCertWithContext(context.Background(), moved, movedCert); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected the signature to be bound to the view, got %v", err)
	}
}

// signProof signs both certificates of the proof, QC(B) and QC(C), with the given validators.
func signProof(t *testing.T, proof *Proof, aggregate func([]types.Signature) (types.AggregateSignature, error), signers ...vexocrypto.Signer) {
	t.Helper()
	proof.QuorumCert.Signature = signCert(t, proof.QuorumCert, aggregate, signers...)
	for index := range proof.CommitChain {
		link := &proof.CommitChain[index]
		link.QuorumCert.Signature = signCert(t, link.QuorumCert, aggregate, signers...)
	}
}

func signCert(t *testing.T, cert QuorumCert, aggregate func([]types.Signature) (types.AggregateSignature, error), signers ...vexocrypto.Signer) types.AggregateSignature {
	t.Helper()
	signatures := make([]types.Signature, 0, len(signers))
	for _, signer := range signers {
		signature, err := vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusVote, cert.SignBytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures = append(signatures, signature)
	}
	aggregateSignature, err := aggregate(signatures)
	if err != nil {
		t.Fatal(err)
	}
	return aggregateSignature
}

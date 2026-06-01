package finality

import (
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestVerifierAcceptsDeterministicAggregateSignature(t *testing.T) {
	firstSigner, err := vexocrypto.NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	secondSigner, err := vexocrypto.NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	thirdSigner, err := vexocrypto.NewDeterministicSigner([]byte("third"))
	if err != nil {
		t.Fatal(err)
	}

	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.Signature = signProof(t, proof, firstSigner, secondSigner)

	verifier := NewVerifier(set, vexocrypto.DeterministicAggregateSigner{})
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierRejectsDeterministicAggregateWithWrongSignerSet(t *testing.T) {
	firstSigner, err := vexocrypto.NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	secondSigner, err := vexocrypto.NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	thirdSigner, err := vexocrypto.NewDeterministicSigner([]byte("third"))
	if err != nil {
		t.Fatal(err)
	}

	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.Signature = signProof(t, proof, firstSigner, thirdSigner)

	verifier := NewVerifier(set, vexocrypto.DeterministicAggregateSigner{})
	if err := verifier.VerifyFinalityProof(proof); err == nil {
		t.Fatal("expected wrong signer aggregate to be rejected")
	}
}

func TestVerifierRejectsDeterministicAggregateAfterHeaderMutation(t *testing.T) {
	firstSigner, err := vexocrypto.NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	secondSigner, err := vexocrypto.NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	thirdSigner, err := vexocrypto.NewDeterministicSigner([]byte("third"))
	if err != nil {
		t.Fatal(err)
	}

	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.Signature = signProof(t, proof, firstSigner, secondSigner)
	proof.Header.AppHash = types.Hash{9}

	verifier := NewVerifier(set, vexocrypto.DeterministicAggregateSigner{})
	if err := verifier.VerifyFinalityProof(proof); err == nil {
		t.Fatal("expected mutated header to be rejected")
	}
}

func signProof(t *testing.T, proof Proof, signers ...vexocrypto.DeterministicSigner) types.AggregateSignature {
	t.Helper()

	signatures := make([]types.Signature, 0, len(signers))
	for _, signer := range signers {
		signature, err := signer.Sign(proof.SignBytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures = append(signatures, signature)
	}

	aggregateSignature, err := (vexocrypto.DeterministicAggregateSigner{}).Aggregate(signatures)
	if err != nil {
		t.Fatal(err)
	}
	return aggregateSignature
}

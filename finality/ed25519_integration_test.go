package finality

import (
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestVerifierAcceptsEd25519MultiSignature(t *testing.T) {
	firstSigner, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	secondSigner, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	thirdSigner, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}

	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", VotingPower: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", VotingPower: 1, PublicKey: thirdSigner.PublicKey()},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	firstSignature, err := vexocrypto.SignWithDomain(firstSigner, vexocrypto.DomainConsensusVote, proof.SignBytes())
	if err != nil {
		t.Fatal(err)
	}
	secondSignature, err := vexocrypto.SignWithDomain(secondSigner, vexocrypto.DomainConsensusVote, proof.SignBytes())
	if err != nil {
		t.Fatal(err)
	}
	proof.QuorumCert.Signature, err = vexocrypto.CombineEd25519Signatures([]types.Signature{firstSignature, secondSignature})
	if err != nil {
		t.Fatal(err)
	}

	if err := NewVerifier(set, vexocrypto.Ed25519MultiVerifier{}).VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

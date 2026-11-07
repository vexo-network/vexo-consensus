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
	signProof(t, &proof, vexocrypto.CombineEd25519Signatures, firstSigner, secondSigner)

	verifier := NewVerifier(set, vexocrypto.Ed25519MultiVerifier{})
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}

	// Signatures of the block certificate are not valid for the child certificate.
	proof.CommitChain[0].QuorumCert.Signature = proof.QuorumCert.Signature
	if err := verifier.VerifyFinalityProof(proof); err == nil {
		t.Fatal("expected the child certificate to need its own signatures")
	}
}

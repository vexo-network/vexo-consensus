//go:build cgo

package finality

import (
	"encoding/base64"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type blstValidator struct {
	id      types.ValidatorID
	adapter vexocrypto.BLSTBLSAdapter
}

func newBLSTValidators(t *testing.T, ids ...types.ValidatorID) ([]blstValidator, []validator.Validator) {
	t.Helper()
	signers := make([]blstValidator, 0, len(ids))
	validators := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		adapter, err := vexocrypto.NewBLSTBLSAdapterFromSeed([]byte("finality-seed-" + string(id) + "-0123456789abcdef"))
		if err != nil {
			t.Fatal(err)
		}
		proof, err := adapter.ProofOfPossession()
		if err != nil {
			t.Fatal(err)
		}
		signers = append(signers, blstValidator{id: id, adapter: adapter})
		validators = append(validators, validator.Validator{
			ID:          id,
			VotingPower: 1,
			PublicKey:   adapter.PublicKey(),
			Metadata:    map[string]string{vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString(proof)},
		})
	}
	return signers, validators
}

// TestVerifierVerifiesRealBLSTwoChainProof drives the whole stack with real BLS keys: key registry,
// vote-domain wrapper, and both certificates of a 2-chain proof.
func TestVerifierVerifiesRealBLSTwoChainProof(t *testing.T) {
	signers, validators := newBLSTValidators(t, "a", "b", "c")
	set := testValidatorSet(t, validators)
	registry, err := vexocrypto.NewBLSKeyRegistry(vexocrypto.NewBLSTBLSVerifierAdapter())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterValidators(set.List()); err != nil {
		t.Fatal(err)
	}

	proof := validProof(set, []types.ValidatorID{"a", "b"})
	aggregator := vexocrypto.NewBLSTBLSVerifierAdapter()
	signProof(t, &proof, aggregator.Aggregate, signers[0].adapter, signers[1].adapter)
	verifier := NewVerifier(set, registry)
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}

	// A quorum signature from the wrong signers fails on the child certificate alone.
	wrongChild := proof
	wrongChild.CommitChain = append([]CommitLink(nil), proof.CommitChain...)
	wrongChild.CommitChain[0].QuorumCert.Signature = signCert(t, wrongChild.CommitChain[0].QuorumCert, aggregator.Aggregate, signers[0].adapter, signers[2].adapter)
	if err := verifier.VerifyFinalityProof(wrongChild); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected the child certificate signed by other validators to be rejected, got %v", err)
	}

	// A swapped header is rejected even though the BLS aggregates are genuine.
	swapped := proof
	swapped.Header.AppHash = types.Hash{9}
	if err := verifier.VerifyFinalityProof(swapped); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected the swapped header to be rejected, got %v", err)
	}
}

// A validator that never proved possession of its BLS key cannot take part in a certificate: the
// rogue-key attack is closed at registration.
func TestRegistryRefusesRogueBLSKey(t *testing.T) {
	_, validators := newBLSTValidators(t, "a", "b", "c")
	registry, err := vexocrypto.NewBLSKeyRegistry(vexocrypto.NewBLSTBLSVerifierAdapter())
	if err != nil {
		t.Fatal(err)
	}

	// Mallory registers c's public key under her own name, with a's proof of possession.
	rogue := append([]validator.Validator(nil), validators...)
	rogue[2] = validators[2]
	rogue[2].Metadata = validators[0].Metadata
	if err := registry.RegisterValidators(rogue); !errors.Is(err, vexocrypto.ErrInvalidBLSProof) {
		t.Fatalf("expected a proof of possession for another key to be refused, got %v", err)
	}
}

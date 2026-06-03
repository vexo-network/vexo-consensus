package crypto

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestBLSValidatorCredentialsRequirePoPAndUniqueKeys(t *testing.T) {
	adapter := testBLSAdapter{safe: true}
	credentials := []BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
		{ValidatorID: "bob", PublicKey: []byte("bob-bls"), ProofOfPossession: []byte("bob-pop")},
	}
	registered, err := ValidateBLSValidatorCredentials(adapter, credentials)
	if err != nil {
		t.Fatal(err)
	}
	if len(registered) != 2 {
		t.Fatalf("expected two registered keys, got %+v", registered)
	}

	duplicate := append(credentials, BLSValidatorCredential{ValidatorID: "mallory", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("mallory-pop")})
	if _, err := ValidateBLSValidatorCredentials(adapter, duplicate); !errors.Is(err, ErrDuplicateBLSPublicKey) {
		t.Fatalf("expected duplicate public key rejection, got %v", err)
	}
	missingProof := []BLSValidatorCredential{{ValidatorID: "alice", PublicKey: []byte("alice-bls")}}
	if _, err := ValidateBLSValidatorCredentials(adapter, missingProof); !errors.Is(err, ErrMissingBLSProof) {
		t.Fatalf("expected missing proof rejection, got %v", err)
	}
	invalidProof := []BLSValidatorCredential{{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("bad")}}
	if _, err := ValidateBLSValidatorCredentials(testBLSAdapter{safe: true, rejectProof: true}, invalidProof); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected invalid proof rejection, got %v", err)
	}
	invalidKey := []BLSValidatorCredential{{ValidatorID: "alice", PublicKey: []byte("bad-key"), ProofOfPossession: []byte("alice-pop")}}
	if _, err := ValidateBLSValidatorCredentials(testBLSAdapter{safe: true, rejectPublicKey: true}, invalidKey); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected invalid public key rejection, got %v", err)
	}
}

func TestBLSAggregateVerifierRejectsUnregisteredPublicKeys(t *testing.T) {
	adapter := testBLSAdapter{safe: true}
	verifier, err := NewBLSAggregateVerifier(adapter, []BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !verifier.VerifyAggregate([]types.PublicKey{[]byte("alice-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected registered key aggregate to verify")
	}
	if verifier.VerifyAggregate([]types.PublicKey{[]byte("mallory-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected unregistered key aggregate to fail")
	}
}

func TestBLSValidatorSetMetadataProofs(t *testing.T) {
	validatorSet := testBLSValidatorSet{validators: []validator.Validator{
		{
			ID:          "alice",
			PublicKey:   []byte("alice-bls"),
			VotingPower: 1,
			Metadata: map[string]string{
				BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString([]byte("alice-pop")),
			},
		},
	}}
	credentials, err := ValidateBLSValidatorSet(testBLSAdapter{safe: true}, validatorSet)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 1 || string(credentials[0].ProofOfPossession) != "alice-pop" {
		t.Fatalf("unexpected credentials: %+v", credentials)
	}
}

type testBLSValidatorSet struct {
	validators []validator.Validator
}

func (set testBLSValidatorSet) Hash() types.Hash {
	return types.Hash{1}
}

func (set testBLSValidatorSet) TotalVotingPower() types.VotingPower {
	var total types.VotingPower
	for _, validatorInfo := range set.validators {
		total += validatorInfo.VotingPower
	}
	return total
}

func (set testBLSValidatorSet) Get(id types.ValidatorID) (validator.Validator, bool) {
	for _, validatorInfo := range set.validators {
		if validatorInfo.ID == id {
			return validatorInfo, true
		}
	}
	return validator.Validator{}, false
}

func (set testBLSValidatorSet) List() []validator.Validator {
	return append([]validator.Validator(nil), set.validators...)
}

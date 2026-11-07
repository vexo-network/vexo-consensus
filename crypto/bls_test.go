package crypto

import (
	"encoding/base64"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func newTestBLSKeyRegistry(t *testing.T, adapter BLSAdapter) *BLSKeyRegistry {
	t.Helper()
	registry, err := NewBLSKeyRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestBLSKeyRegistryRegisterRequiresPoPAndUniqueKeys(t *testing.T) {
	credentials := []BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
		{ValidatorID: "bob", PublicKey: []byte("bob-bls"), ProofOfPossession: []byte("bob-pop")},
	}
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true}).Register(credentials); err != nil {
		t.Fatal(err)
	}

	duplicate := append(append([]BLSValidatorCredential(nil), credentials...), BLSValidatorCredential{ValidatorID: "mallory", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("mallory-pop")})
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true}).Register(duplicate); !errors.Is(err, ErrDuplicateBLSPublicKey) {
		t.Fatalf("expected duplicate public key rejection, got %v", err)
	}
	missingID := []BLSValidatorCredential{{PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")}}
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true}).Register(missingID); !errors.Is(err, ErrMissingBLSValidatorID) {
		t.Fatalf("expected missing validator id rejection, got %v", err)
	}
	missingKey := []BLSValidatorCredential{{ValidatorID: "alice", ProofOfPossession: []byte("alice-pop")}}
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true}).Register(missingKey); !errors.Is(err, ErrMissingBLSPublicKey) {
		t.Fatalf("expected missing public key rejection, got %v", err)
	}
	missingProof := []BLSValidatorCredential{{ValidatorID: "alice", PublicKey: []byte("alice-bls")}}
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true}).Register(missingProof); !errors.Is(err, ErrMissingBLSProof) {
		t.Fatalf("expected missing proof rejection, got %v", err)
	}
	invalidProof := []BLSValidatorCredential{{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("bad")}}
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true, rejectProof: true}).Register(invalidProof); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected invalid proof rejection, got %v", err)
	}
	invalidKey := []BLSValidatorCredential{{ValidatorID: "alice", PublicKey: []byte("bad-key"), ProofOfPossession: []byte("alice-pop")}}
	if err := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true, rejectPublicKey: true}).Register(invalidKey); !errors.Is(err, ErrInvalidBLSPublicKey) {
		t.Fatalf("expected invalid public key rejection, got %v", err)
	}
	if _, err := NewBLSKeyRegistry(testBLSAdapter{}); !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected unsafe adapter rejection, got %v", err)
	}
}

func TestBLSKeyRegistryVerifiesOnlyRegisteredKeys(t *testing.T) {
	registry := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true})
	alice := []types.PublicKey{[]byte("alice-bls")}
	if registry.VerifyAggregate(alice, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected a key that was never registered to fail closed")
	}
	if err := registry.Register([]BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
	}); err != nil {
		t.Fatal(err)
	}
	if !registry.VerifyAggregate(alice, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected registered key aggregate to verify")
	}
	if registry.VerifyAggregate([]types.PublicKey{[]byte("mallory-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected unregistered key aggregate to fail")
	}
	if registry.VerifyAggregate([]types.PublicKey{[]byte("alice-bls"), []byte("mallory-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected one unregistered key to fail the whole aggregate")
	}
	if registry.VerifyAggregate(nil, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected empty key set to fail")
	}
}

func TestBLSKeyRegistryRegistrationIsAllOrNothing(t *testing.T) {
	registry := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true})
	err := registry.Register([]BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
		{ValidatorID: "bob", PublicKey: []byte("bob-bls")},
	})
	if !errors.Is(err, ErrMissingBLSProof) {
		t.Fatalf("expected missing proof rejection, got %v", err)
	}
	if registry.VerifyAggregate([]types.PublicKey{[]byte("alice-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected a rejected snapshot to leave none of its keys registered")
	}
}

func TestBLSKeyRegistryKeepsKeysOfEarlierSnapshots(t *testing.T) {
	registry := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true})
	if err := registry.Register([]BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register([]BLSValidatorCredential{
		{ValidatorID: "bob", PublicKey: []byte("bob-bls"), ProofOfPossession: []byte("bob-pop")},
	}); err != nil {
		t.Fatal(err)
	}
	both := []types.PublicKey{[]byte("alice-bls"), []byte("bob-bls")}
	if !registry.VerifyAggregate(both, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected keys of every registered snapshot to verify")
	}
	if !registry.VerifyAggregate(both[:1], []byte("message"), []byte("aggregate")) {
		t.Fatal("expected keys of the earlier snapshot to stay verifiable")
	}
}

type countingProofAdapter struct {
	testBLSAdapter
	verifications *atomic.Int64
}

func (adapter countingProofAdapter) VerifyProofOfPossession(publicKey types.PublicKey, proof types.Signature) bool {
	adapter.verifications.Add(1)
	return string(proof) == "valid-"+string(publicKey)
}

func TestBLSKeyRegistryCachesVerifiedKeys(t *testing.T) {
	verifications := new(atomic.Int64)
	registry := newTestBLSKeyRegistry(t, countingProofAdapter{testBLSAdapter: testBLSAdapter{safe: true}, verifications: verifications})
	credentials := []BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("valid-alice-bls")},
	}
	for range 3 {
		if err := registry.Register(credentials); err != nil {
			t.Fatal(err)
		}
	}
	if verifications.Load() != 1 {
		t.Fatalf("expected one proof verification for a repeatedly registered key, got %d", verifications.Load())
	}
	wrongProof := []BLSValidatorCredential{
		{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("another-proof")},
	}
	if err := registry.Register(wrongProof); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected a different invalid proof for a cached key to be verified and rejected, got %v", err)
	}
}

func TestBLSKeyRegistryIsSafeForConcurrentUse(t *testing.T) {
	registry := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true})
	var group sync.WaitGroup
	for index := range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			key := types.PublicKey([]byte{'k', byte(index)})
			if err := registry.Register([]BLSValidatorCredential{
				{ValidatorID: "validator", PublicKey: key, ProofOfPossession: []byte("pop")},
			}); err != nil {
				t.Error(err)
				return
			}
			if !registry.VerifyAggregate([]types.PublicKey{key}, []byte("message"), []byte("aggregate")) {
				t.Error("expected registered key to verify")
			}
		}()
	}
	group.Wait()
}

func TestBLSKeyRegistryRegistersValidatorMetadataProofs(t *testing.T) {
	registry := newTestBLSKeyRegistry(t, testBLSAdapter{safe: true})
	validators := []validator.Validator{
		{
			ID:          "alice",
			PublicKey:   []byte("alice-bls"),
			VotingPower: 1,
			Metadata: map[string]string{
				BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString([]byte("alice-pop")),
			},
		},
	}
	if err := registry.RegisterValidators(validators); err != nil {
		t.Fatal(err)
	}
	if !registry.VerifyAggregate([]types.PublicKey{[]byte("alice-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected validator key registered from metadata to verify")
	}

	withoutProof := []validator.Validator{{ID: "bob", PublicKey: []byte("bob-bls"), VotingPower: 1}}
	if err := registry.RegisterValidators(withoutProof); !errors.Is(err, ErrMissingBLSProof) {
		t.Fatalf("expected a validator without bls_pop to be rejected, got %v", err)
	}
	badEncoding := []validator.Validator{{ID: "bob", PublicKey: []byte("bob-bls"), VotingPower: 1, Metadata: map[string]string{BLSProofOfPossessionMetadataKey: "%%%"}}}
	if err := registry.RegisterValidators(badEncoding); !errors.Is(err, ErrInvalidBLSProof) {
		t.Fatalf("expected an undecodable bls_pop to be rejected, got %v", err)
	}
	if registry.VerifyAggregate([]types.PublicKey{[]byte("bob-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected rejected validator key to stay unregistered")
	}
}

package validator

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestStoreRegistryPersistsHeightVersionedSets(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := NewStoreRegistry(context.Background(), storage, nil, 1, []Validator{
		{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100, PublicKey: []byte("alice-pub")},
	})
	if err != nil {
		t.Fatal(err)
	}
	registry.SetEffectiveHeight(2)
	if _, err := registry.ApplyJoin(context.Background(), Candidate{Address: "bob", Stake: 50, PublicKey: []byte("bob-pub")}); err != nil {
		t.Fatal(err)
	}
	registry.SetEffectiveHeight(3)
	if err := registry.UpdateVotingPower(context.Background(), "alice", 80); err != nil {
		t.Fatal(err)
	}

	set1, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := set1.Get("bob"); found {
		t.Fatal("bob must not exist at height 1")
	}
	set2, err := registry.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := set2.Get("bob"); !found {
		t.Fatal("bob must exist at height 2")
	}
	set3, err := registry.ValidatorSet(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := set3.Get("alice")
	if !found || alice.VotingPower != 80 {
		t.Fatalf("expected alice power 80 at height 3, got %+v found=%t", alice, found)
	}

	reopened, err := NewStoreRegistry(context.Background(), storage, nil, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	reopenedSet, err := reopened.ValidatorSet(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if reopenedSet.Hash() != set3.Hash() {
		t.Fatal("expected persisted validator set hash")
	}
}

func TestStoreRegistryRejectsCorruptExistingState(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(context.Background(), validatorRegistryNamespace, []byte("heights"), []byte("{")); err != nil {
		t.Fatal(err)
	}

	_, err = NewStoreRegistry(context.Background(), storage, nil, 1, []Validator{{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1}})
	if err == nil || errors.Is(err, ErrValidatorSetNotFound) {
		t.Fatalf("expected corrupt validator registry to fail startup, got %v", err)
	}
}

func TestStoreRegistryRejectsZeroPower(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = NewStoreRegistry(context.Background(), storage, nil, 1, []Validator{{ID: "alice", Address: "alice", VotingPower: 0}})
	if err != ErrZeroVotingPower {
		t.Fatalf("expected zero voting power, got %v", err)
	}
}

func TestStoreRegistrySnapshotIsImmutable(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := NewStoreRegistry(context.Background(), storage, nil, 1, []Validator{{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, Metadata: map[string]string{"region": "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	list := set.List()
	list[0].Metadata["region"] = "mutated"
	again, found := set.Get(types.ValidatorID("alice"))
	if !found || again.Metadata["region"] != "a" {
		t.Fatalf("snapshot mutated: %+v found=%t", again, found)
	}
}

func TestStoreRegistryStagesCommitsAndLeaves(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry, err := NewStoreRegistry(context.Background(), storage, NewConfigurableAdmissionPolicy(AdmissionConfig{
		Permissionless:   true,
		MinStake:         1,
		RequirePublicKey: true,
	}), 1, []Validator{{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10, PublicKey: []byte("alice-pub")}})
	if err != nil {
		t.Fatal(err)
	}
	set1, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if total := set1.TotalVotingPower(); total != 10 {
		t.Fatalf("expected total power 10, got %d", total)
	}
	staged, writes, err := registry.StageValidatorUpdatesAt(context.Background(), 2, []types.ValidatorUpdate{
		{ID: "alice", Address: "alice", VotingPower: 12, Stake: 12, PublicKey: []byte("alice-new"), Metadata: map[string]string{"role": "existing"}},
		{ID: "bob", Address: "bob", VotingPower: 5, Stake: 5, PublicKey: []byte("bob-pub"), Metadata: map[string]string{"role": "new"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 2 || string(writes[0].Key) != string(validatorSetKey(2)) || string(writes[1].Key) != "heights" {
		t.Fatalf("expected the staged set and the registered heights to be the only writes, got %+v", writes)
	}
	if total := staged.TotalVotingPower(); total != 17 {
		t.Fatalf("expected staged total power 17, got %d", total)
	}
	if current, err := registry.ValidatorSet(context.Background(), 2); err != nil {
		t.Fatal(err)
	} else if _, found := current.Get("bob"); found {
		t.Fatal("staging must not mutate registry before writes are committed")
	}
	if err := storage.SetBatch(context.Background(), writes); err != nil {
		t.Fatal(err)
	}
	registry.CommitStagedValidatorUpdates(2)
	set2, err := registry.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := set2.Get("alice")
	if !found || alice.VotingPower != 12 || string(alice.PublicKey) != "alice-new" || alice.Metadata["role"] != "existing" {
		t.Fatalf("unexpected staged alice update: %+v found=%t", alice, found)
	}
	bob, found := set2.Get("bob")
	if !found || bob.VotingPower != 5 || bob.Metadata["role"] != "new" {
		t.Fatalf("unexpected staged bob join: %+v found=%t", bob, found)
	}
	if err := registry.ApplyLeave(context.Background(), "bob"); err != nil {
		t.Fatal(err)
	}
	registry.SetEffectiveHeight(4)
	if err := registry.ApplyLeave(context.Background(), "missing"); err != ErrValidatorNotFound {
		t.Fatalf("expected missing leave rejection, got %v", err)
	}
	if _, _, err := registry.StageValidatorUpdatesAt(context.Background(), 4, []types.ValidatorUpdate{{ID: "charlie", VotingPower: 0}}); err != nil {
		t.Fatalf("a removal of an unknown validator must be skipped, got %v", err)
	}
	reopened, err := NewStoreRegistry(context.Background(), storage, nil, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	reopenedSet, err := reopened.ValidatorSet(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := reopenedSet.Get("bob"); found {
		t.Fatal("expected bob leave to persist")
	}
}

func TestStoreRegistryServesAStagedSetCommittedBeforeTheInMemoryCommit(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry, err := NewStoreRegistry(context.Background(), storage, nil, 1, []Validator{{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10}})
	if err != nil {
		t.Fatal(err)
	}
	_, writes, err := registry.StageValidatorUpdatesAt(context.Background(), 2, []types.ValidatorUpdate{{ID: "bob", Address: "bob", VotingPower: 5, Stake: 5, PublicKey: []byte("bob-pub")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SetBatch(context.Background(), writes); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStoreRegistry(context.Background(), storage, nil, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, err := reopened.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if bob, found := set.Get("bob"); !found || bob.VotingPower != 5 {
		t.Fatalf("expected the staged set to survive a crash before the in-memory commit, got %+v found=%t", bob, found)
	}
}

func stagedSet(t *testing.T, registry *StoreRegistry, storage *store.LevelDBStore, height types.Height, updates []types.ValidatorUpdate) Set {
	t.Helper()
	set, writes, err := registry.StageValidatorUpdatesAt(context.Background(), height, updates)
	if err != nil {
		t.Fatalf("staging must not fail on update content, got %v", err)
	}
	if err := storage.SetBatch(context.Background(), writes); err != nil {
		t.Fatal(err)
	}
	registry.CommitStagedValidatorUpdates(height)
	return set
}

func newStagingRegistry(t *testing.T, policy AdmissionPolicy, validators []Validator) (*StoreRegistry, *store.LevelDBStore) {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.Close() })
	registry, err := NewStoreRegistry(context.Background(), storage, policy, 1, validators)
	if err != nil {
		t.Fatal(err)
	}
	return registry, storage
}

// A certified block must never be aborted by a validator update the registry cannot apply.
func TestStoreRegistryStagingSkipsInvalidUpdatesInsteadOfFailing(t *testing.T) {
	policy := NewConfigurableAdmissionPolicy(AdmissionConfig{Permissionless: true, MinStake: 10, MaxValidators: 3})
	registry, storage := newStagingRegistry(t, policy, []Validator{
		{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10, PublicKey: []byte("alice-pub")},
	})
	set := stagedSet(t, registry, storage, 2, []types.ValidatorUpdate{
		{ID: "ghost", VotingPower: 0},
		{ID: "bob", Address: "bob", VotingPower: 20, Stake: 20},
		{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1, PublicKey: []byte("carol-pub")},
		{Address: "", VotingPower: 5},
	})
	if got := len(set.List()); got != 1 {
		t.Fatalf("unknown removal, keyless join, under-staked join and anonymous update must be skipped, got %+v", set.List())
	}
	if _, found := set.Get("alice"); !found {
		t.Fatal("alice must be untouched")
	}
}

func TestStoreRegistryStagingEnforcesAdmissionForJoins(t *testing.T) {
	policy := NewConfigurableAdmissionPolicy(AdmissionConfig{Permissionless: false, MinStake: 1, MaxValidators: 2, Whitelist: map[string]bool{"bob": true, "carol": true, "dave": true}})
	registry, storage := newStagingRegistry(t, policy, []Validator{
		{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10, PublicKey: []byte("alice-pub")},
	})
	set := stagedSet(t, registry, storage, 2, []types.ValidatorUpdate{
		{ID: "mallory", Address: "mallory", VotingPower: 1000, Stake: 1000, PublicKey: []byte("mallory-pub")},
		{ID: "bob", Address: "bob", VotingPower: 5, Stake: 5, PublicKey: []byte("bob-pub")},
		{ID: "carol", Address: "carol", VotingPower: 5, Stake: 5, PublicKey: []byte("carol-pub")},
	})
	if _, found := set.Get("mallory"); found {
		t.Fatal("a non-whitelisted candidate must not join through a validator update")
	}
	if _, found := set.Get("bob"); !found {
		t.Fatal("the whitelisted candidate that fits must join")
	}
	if _, found := set.Get("carol"); found {
		t.Fatal("the set is full, carol must be skipped")
	}
}

func TestStoreRegistryStagingAppliesRemovalsBeforeJoins(t *testing.T) {
	policy := NewConfigurableAdmissionPolicy(AdmissionConfig{Permissionless: true, MinStake: 1, MaxValidators: 1})
	registry, storage := newStagingRegistry(t, policy, []Validator{
		{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10, PublicKey: []byte("alice-pub")},
	})
	set := stagedSet(t, registry, storage, 2, []types.ValidatorUpdate{
		{ID: "bob", Address: "bob", VotingPower: 7, Stake: 7, PublicKey: []byte("bob-pub")},
		{ID: "alice", VotingPower: 0},
	})
	if _, found := set.Get("alice"); found {
		t.Fatal("alice must leave")
	}
	if bob, found := set.Get("bob"); !found || bob.VotingPower != 7 {
		t.Fatalf("bob must take the slot freed in the same block, got %+v found=%t", bob, found)
	}
}

func TestStoreRegistryStagingNeverEmptiesTheSet(t *testing.T) {
	registry, storage := newStagingRegistry(t, nil, []Validator{
		{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10, PublicKey: []byte("alice-pub")},
		{ID: "bob", Address: "bob", VotingPower: 10, Stake: 10, PublicKey: []byte("bob-pub")},
	})
	set := stagedSet(t, registry, storage, 2, []types.ValidatorUpdate{
		{ID: "alice", VotingPower: 0},
		{ID: "bob", VotingPower: 0},
	})
	if len(set.List()) != 2 {
		t.Fatalf("removals that would empty the set must be ignored, got %+v", set.List())
	}
}

// Plain power updates must not clobber the key or the metadata of a validator.
func TestStoreRegistryStagingKeepsKeyAndMergesMetadata(t *testing.T) {
	registry, storage := newStagingRegistry(t, nil, []Validator{
		{ID: "alice", Address: "alice-op", VotingPower: 10, Stake: 10, PublicKey: []byte("alice-pub"), Metadata: map[string]string{"bls_pop": "pop", "account_address": "acct"}},
	})
	set := stagedSet(t, registry, storage, 2, []types.ValidatorUpdate{{ID: "alice", VotingPower: 3, Stake: 3}})
	alice, _ := set.Get("alice")
	if string(alice.PublicKey) != "alice-pub" || alice.Metadata["bls_pop"] != "pop" || alice.Metadata["account_address"] != "acct" || alice.Address != "alice-op" || alice.VotingPower != 3 {
		t.Fatalf("keyless power update must only change power, got %+v", alice)
	}
	set = stagedSet(t, registry, storage, 3, []types.ValidatorUpdate{{ID: "alice", VotingPower: 3, PublicKey: []byte("alice-rotated"), Metadata: map[string]string{"bls_pop": "pop2"}}})
	alice, _ = set.Get("alice")
	if string(alice.PublicKey) != "alice-rotated" || alice.Metadata["bls_pop"] != "pop2" || alice.Metadata["account_address"] != "acct" {
		t.Fatalf("rotation must replace the key and merge metadata, got %+v", alice)
	}
}

func TestValidatorUpdateContentErrorsAreClassified(t *testing.T) {
	for _, err := range []error{
		ErrMissingCandidateID, ErrValidatorNotFound, ErrValidatorExists, ErrZeroVotingPower,
		ErrInsufficientStake, ErrValidatorSetFull, ErrCandidateDenied, ErrMissingCandidateKey,
		fmt.Errorf("%w: address is not whitelisted", ErrCandidateDenied),
	} {
		if !errors.Is(err, ErrInvalidUpdate) {
			t.Fatalf("%v must be classified as invalid update content", err)
		}
	}
	if errors.Is(ErrValidatorSetNotFound, ErrInvalidUpdate) || errors.Is(context.Canceled, ErrInvalidUpdate) {
		t.Fatal("storage and context errors are not update content errors")
	}
	if ErrValidatorNotFound.Error() != "validator not found" {
		t.Fatalf("sentinel message changed: %q", ErrValidatorNotFound)
	}
}

func TestLoadValidatorSetIsReadOnly(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if _, err := LoadValidatorSet(context.Background(), storage, 1); !errors.Is(err, ErrValidatorSetNotFound) {
		t.Fatalf("expected missing set, got %v", err)
	}
	if _, err := storage.Get(context.Background(), validatorRegistryNamespace, []byte("heights")); err == nil {
		t.Fatal("loading a missing set must not create registry state")
	}
	if _, err := NewStoreRegistry(context.Background(), storage, nil, 1, []Validator{{ID: "alice", Address: "alice", VotingPower: 4, Stake: 4}}); err != nil {
		t.Fatal(err)
	}
	set, err := LoadValidatorSet(context.Background(), storage, 1)
	if err != nil {
		t.Fatal(err)
	}
	if alice, found := set.Get("alice"); !found || alice.VotingPower != 4 {
		t.Fatalf("unexpected genesis set: %+v", set.List())
	}
}

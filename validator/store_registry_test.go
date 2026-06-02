package validator

import (
	"context"
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

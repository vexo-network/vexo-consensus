package validator

import (
	"context"
	"errors"
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

func TestStoreRegistryStagesCommitsLeavesAndRotationEvents(t *testing.T) {
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
	if len(writes) != 2 {
		t.Fatalf("expected staged snapshot writes, got %+v", writes)
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
	if err := registry.CommitStagedValidatorUpdates(context.Background(), 2, []types.ValidatorUpdate{
		{ID: "alice", VotingPower: 12},
		{ID: "bob", VotingPower: 5},
	}); err != nil {
		t.Fatal(err)
	}
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
	if _, _, err := registry.StageValidatorUpdatesAt(context.Background(), 4, []types.ValidatorUpdate{{ID: "charlie", VotingPower: 0}}); err != ErrValidatorNotFound {
		t.Fatalf("expected zero-power missing update rejection, got %v", err)
	}
	events := registry.RotationEvents()
	if len(events) < 3 {
		t.Fatalf("expected join/power/leave events, got %+v", events)
	}
	seenJoin, seenPower, seenLeave := false, false, false
	for _, event := range events {
		if event.Type == RotationEventJoin && event.ValidatorID == "bob" && event.ValidatorSetHash != (types.Hash{}) {
			seenJoin = true
		}
		if event.Type == RotationEventPowerChange && event.ValidatorID == "alice" && event.VotingPower == 12 {
			seenPower = true
		}
		if event.Type == RotationEventLeave && event.ValidatorID == "bob" {
			seenLeave = true
		}
	}
	if !seenJoin || !seenPower || !seenLeave {
		t.Fatalf("missing expected rotation events: %+v", events)
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

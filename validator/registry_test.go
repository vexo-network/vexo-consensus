package validator

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestInMemoryRegistryPermissionlessJoin(t *testing.T) {
	registry, err := NewInMemoryRegistry(NewConfigurableAdmissionPolicy(AdmissionConfig{
		Permissionless: true,
		MinStake:       100,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}

	validatorInfo, err := registry.ApplyJoin(context.Background(), Candidate{
		Address: "alice",
		Stake:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if validatorInfo.ID != "alice" {
		t.Fatalf("expected validator id alice, got %s", validatorInfo.ID)
	}
	if validatorInfo.VotingPower != 100 {
		t.Fatalf("expected voting power from stake")
	}
}

func TestInMemoryRegistryRejectsLowStake(t *testing.T) {
	registry, err := NewInMemoryRegistry(NewConfigurableAdmissionPolicy(AdmissionConfig{
		Permissionless: true,
		MinStake:       100,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.ApplyJoin(context.Background(), Candidate{
		Address: "alice",
		Stake:   99,
	})
	if !errors.Is(err, ErrInsufficientStake) {
		t.Fatalf("expected insufficient stake, got %v", err)
	}
}

func TestInMemoryRegistryRejectsWhenFull(t *testing.T) {
	registry, err := NewInMemoryRegistry(NewConfigurableAdmissionPolicy(AdmissionConfig{
		Permissionless: true,
		MinStake:       1,
		MaxValidators:  1,
	}), []Validator{{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.ApplyJoin(context.Background(), Candidate{
		Address: "bob",
		Stake:   1,
	})
	if !errors.Is(err, ErrValidatorSetFull) {
		t.Fatalf("expected validator set full, got %v", err)
	}
}

func TestInMemoryRegistryWhitelistMode(t *testing.T) {
	registry, err := NewInMemoryRegistry(NewConfigurableAdmissionPolicy(AdmissionConfig{
		Permissionless: false,
		MinStake:       1,
		Whitelist:      map[string]bool{"alice": true},
	}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := registry.ApplyJoin(context.Background(), Candidate{Address: "alice", Stake: 1}); err != nil {
		t.Fatalf("expected whitelisted candidate to join, got %v", err)
	}
	_, err = registry.ApplyJoin(context.Background(), Candidate{Address: "bob", Stake: 1})
	if !errors.Is(err, ErrCandidateDenied) {
		t.Fatalf("expected candidate denied, got %v", err)
	}
}

func TestInMemoryRegistryRejectsDuplicateJoin(t *testing.T) {
	registry, err := NewInMemoryRegistry(NewConfigurableAdmissionPolicy(AdmissionConfig{
		Permissionless: true,
		MinStake:       1,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := registry.ApplyJoin(context.Background(), Candidate{Address: "alice", Stake: 1}); err != nil {
		t.Fatal(err)
	}
	_, err = registry.ApplyJoin(context.Background(), Candidate{Address: "alice", Stake: 1})
	if !errors.Is(err, ErrValidatorExists) {
		t.Fatalf("expected validator exists, got %v", err)
	}
}

func TestInMemoryRegistryLeaveAndUpdatePower(t *testing.T) {
	registry, err := NewInMemoryRegistry(nil, []Validator{{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1}})
	if err != nil {
		t.Fatal(err)
	}

	if err := registry.UpdateVotingPower(context.Background(), "alice", 10); err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	validatorInfo, found := set.Get("alice")
	if !found {
		t.Fatal("expected alice")
	}
	if validatorInfo.VotingPower != 10 {
		t.Fatalf("expected power 10, got %d", validatorInfo.VotingPower)
	}

	if err := registry.ApplyLeave(context.Background(), "alice"); err != nil {
		t.Fatal(err)
	}
	set, err = registry.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := set.Get("alice"); found {
		t.Fatal("expected alice to leave")
	}
}

func TestInMemoryRegistrySnapshotIsImmutable(t *testing.T) {
	registry, err := NewInMemoryRegistry(nil, []Validator{{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1}})
	if err != nil {
		t.Fatal(err)
	}

	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	list := set.List()
	list[0].VotingPower = 99

	again, found := set.Get(types.ValidatorID("alice"))
	if !found {
		t.Fatal("expected alice")
	}
	if again.VotingPower != 1 {
		t.Fatalf("snapshot mutated through list, got %d", again.VotingPower)
	}
}

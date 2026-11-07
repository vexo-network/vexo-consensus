package staking

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestTotalBondedPowerSumsEveryValidatorIncludingJailed(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "60")
	registerValidator(t, module, storage, 1, "bob", "validator-2", "40")
	if total, err := TotalBondedPower(context.Background(), storage); err != nil || total != 100 {
		t.Fatalf("expected total bonded power 100, got %d err=%v", total, err)
	}
	slash(t, storage, "offence-1", "validator-1", 1, 5, 0, 100)
	if total, err := TotalBondedPower(context.Background(), storage); err != nil || total != 100 {
		t.Fatalf("a jailed validator's stake is still bonded, got %d err=%v", total, err)
	}
	slash(t, storage, "offence-2", "validator-2", 1, 5, 5000, 0)
	if total, err := TotalBondedPower(context.Background(), storage); err != nil || total != 80 {
		t.Fatalf("expected total bonded power 80 after the slash, got %d err=%v", total, err)
	}
	if _, err := TotalBondedPower(context.Background(), nil); err == nil {
		t.Fatal("expected a missing store to fail")
	}
}

// Stake added at or after the snapshot height carries no voting power for that snapshot, so delegating
// right before a vote and undelegating right after cannot swing a proposal.
func TestVotingPowerAtIgnoresStakeAddedSinceTheSnapshot(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 5, "alice", "validator-1", "10")
	fund(t, storage, "flash", 1000)
	mustDeliver(t, module, storage, 9, stakingTx("flash", "delegate", "flash", "validator-1", "1000"))

	for _, tc := range []struct {
		name     string
		delegate types.Address
		snapshot types.Height
		want     types.VotingPower
	}{
		{"stake bonded before the snapshot", "alice", 6, 10},
		{"stake bonded in the snapshot block", "alice", 5, 0},
		{"flash stake added after the snapshot", "flash", 9, 0},
		{"flash stake added before a later snapshot", "flash", 10, 1000},
		{"account without stake", "nobody", 100, 0},
	} {
		power, err := VotingPowerAt(context.Background(), storage, tc.delegate, tc.snapshot)
		if err != nil || power != tc.want {
			t.Fatalf("%s: expected %d, got %d err=%v", tc.name, tc.want, power, err)
		}
	}
	if _, err := VotingPowerAt(context.Background(), storage, "a/b", 1); err == nil {
		t.Fatal("expected an invalid identity to fail")
	}
}

func TestVotingPowerAtDoesNotReviveStakeAfterUndelegation(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "10")
	mustDeliver(t, module, storage, 4, stakingTx("alice", "undelegate", "alice", "validator-1", "6"))
	if power, err := VotingPowerAt(context.Background(), storage, "alice", 3); err != nil || power != 4 {
		t.Fatalf("expected only the remaining stake to vote, got %d err=%v", power, err)
	}
}

package staking

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
	vexovalidator "github.com/vexo-network/vexo-consensus/validator"
)

func slash(t *testing.T, storage vexoapp.StateStore, offence string, validatorID types.ValidatorID, infraction types.Height, current types.Height, bps uint64, jailUntil types.Height) (types.ValidatorUpdate, bool) {
	t.Helper()
	update, applied, err := SlashValidator(context.Background(), storage, offence, validatorID, infraction, current, bps, jailUntil)
	if err != nil {
		t.Fatal(err)
	}
	return update, applied
}

// delegateStakes registers validator-1 for operator with the first stake and adds the rest as delegators.
func delegateStakes(t *testing.T, module *Module, storage vexoapp.StateStore, stakes map[string]string) {
	t.Helper()
	first := true
	for _, delegator := range []string{"alice", "bob", "carol"} {
		stake, found := stakes[delegator]
		if !found {
			continue
		}
		if first {
			registerValidator(t, module, storage, 1, delegator, "validator-1", stake)
			first = false
			continue
		}
		amount, err := parseAmount(stake)
		if err != nil {
			t.Fatal(err)
		}
		fund(t, storage, types.Address(delegator), amount)
		mustDeliver(t, module, storage, 1, stakingTx(delegator, "delegate", delegator, "validator-1", stake))
	}
}

func TestSlashValidatorSlashesEveryDelegationAndKeepsPowerBacked(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	delegateStakes(t, module, storage, map[string]string{"alice": "40", "bob": "60"})
	update, applied := slash(t, storage, "conflicting-vote/validator-1/5", "validator-1", 5, 6, 2500, 0)
	if !applied {
		t.Fatal("expected the slash to apply")
	}
	if alice, bob, power := mustStake(t, storage, "alice", "validator-1"), mustStake(t, storage, "bob", "validator-1"), mustPower(t, storage, "validator-1"); alice != 30 || bob != 45 || power != 75 {
		t.Fatalf("unexpected slash ledger alice=%d bob=%d power=%d", alice, bob, power)
	}
	if update.ID != "validator-1" || update.VotingPower != 75 || update.Stake != 75 || string(update.PublicKey) != "key-validator-1" {
		t.Fatalf("unexpected slash update %+v", update)
	}
}

// Regression: the slash rounded each delegation and the validator power independently, so power could
// stay above the sum of the stakes that back it.
func TestSlashValidatorPowerEqualsSumOfRemainingStakes(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "33")
	for _, delegator := range []struct {
		name  string
		stake string
	}{{"bob", "33"}, {"carol", "34"}} {
		amount, _ := strconv.ParseUint(delegator.stake, 10, 64)
		fund(t, storage, types.Address(delegator.name), amount)
		mustDeliver(t, module, storage, 1, stakingTx(delegator.name, "delegate", delegator.name, "validator-1", delegator.stake))
	}
	update, _ := slash(t, storage, "offence-rounding", "validator-1", 1, 2, 1000, 0)
	alice, bob, carol := mustStake(t, storage, "alice", "validator-1"), mustStake(t, storage, "bob", "validator-1"), mustStake(t, storage, "carol", "validator-1")
	if alice != 29 || bob != 29 || carol != 30 {
		t.Fatalf("expected remaining stakes 29/29/30 (rounded down), got %d/%d/%d", alice, bob, carol)
	}
	if power := mustPower(t, storage, "validator-1"); power != alice+bob+carol || update.VotingPower != types.VotingPower(power) {
		t.Fatalf("power %d (update %d) is not backed by stake %d", power, update.VotingPower, alice+bob+carol)
	}
}

func TestSlashValidatorIsIdempotentPerOffenceKey(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	delegateStakes(t, module, storage, map[string]string{"alice": "100"})
	first, applied := slash(t, storage, "offence-1", "validator-1", 1, 2, 5000, 0)
	if !applied {
		t.Fatal("expected first slash to apply")
	}
	second, applied := slash(t, storage, "offence-1", "validator-1", 1, 3, 5000, 90)
	if applied {
		t.Fatal("expected the replayed offence to be ignored")
	}
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 50 {
		t.Fatalf("expected idempotent stake 50, got %d", stake)
	}
	if first.VotingPower != second.VotingPower || second.VotingPower != 50 {
		t.Fatalf("replay must report the current standing, first=%+v second=%+v", first, second)
	}
	if _, jailed, err := JailedUntil(context.Background(), storage, "validator-1"); err != nil || jailed {
		t.Fatalf("a replayed offence must not jail, jailed=%t err=%v", jailed, err)
	}
	if _, _, err := SlashValidator(context.Background(), storage, "offence-1", "other", 1, 3, 5000, 0); err == nil {
		t.Fatal("expected an offence key reused for another validator to fail")
	}
	if _, applied := slash(t, storage, "offence-2", "validator-1", 1, 4, 5000, 0); !applied {
		t.Fatal("expected a distinct offence to apply")
	}
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 25 {
		t.Fatalf("expected the second offence to slash again, got %d", stake)
	}
}

func TestSlashValidatorFullSlashTombstonesAndRemovesDelegations(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	delegateStakes(t, module, storage, map[string]string{"alice": "100"})
	update, _ := slash(t, storage, "offence-full", "validator-1", 1, 2, FractionDenominatorBPS, 0)
	if stake, power := mustStake(t, storage, "alice", "validator-1"), mustPower(t, storage, "validator-1"); stake != 0 || power != 0 {
		t.Fatalf("expected full slash to zero ledger, stake=%d power=%d", stake, power)
	}
	if update.VotingPower != 0 {
		t.Fatalf("expected zero voting power update, got %+v", update)
	}
	if tombstoned, err := Tombstoned(context.Background(), storage, "validator-1"); err != nil || !tombstoned {
		t.Fatalf("expected full slash to tombstone, tombstoned=%t err=%v", tombstoned, err)
	}
	fund(t, storage, "alice", 10)
	if result := deliver(module, storage, 11, stakingTx("alice", "delegate", "alice", "validator-1", "1")); result.Code == 0 {
		t.Fatal("expected tombstoned validator delegation to fail")
	}
	if result := deliver(module, storage, 11, stakingTx("alice", "unjail", "validator-1")); result.Code == 0 {
		t.Fatal("expected tombstoned validator unjail to fail")
	}
}

func TestSlashValidatorRejectsInvalidRequests(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	delegateStakes(t, module, storage, map[string]string{"alice": "10"})
	for name, call := range map[string]func() error{
		"nil store": func() error {
			_, _, err := SlashValidator(context.Background(), nil, "k", "validator-1", 1, 2, 1, 0)
			return err
		},
		"empty offence": func() error {
			_, _, err := SlashValidator(context.Background(), storage, "", "validator-1", 1, 2, 1, 0)
			return err
		},
		"fraction too big": func() error {
			_, _, err := SlashValidator(context.Background(), storage, "k", "validator-1", 1, 2, 10001, 0)
			return err
		},
		"unknown": func() error {
			_, _, err := SlashValidator(context.Background(), storage, "k", "nobody", 1, 2, 1, 0)
			return err
		},
		"slash id": func() error {
			_, _, err := SlashValidator(context.Background(), storage, "k", "a/b", 1, 2, 1, 0)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 10 {
		t.Fatalf("a refused slash changed stake to %d", stake)
	}
}

func TestSlashValidatorBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	module := NewModule()
	delegateStakes(t, module, base, map[string]string{"alice": "100"})
	storage := failingBatchStore{LevelDBStore: base, err: errors.New("batch failed")}
	if _, _, err := SlashValidator(context.Background(), storage, "offence-1", "validator-1", 1, 2, 5000, 0); err == nil {
		t.Fatal("expected slashing batch failure")
	}
	if stake, power := mustStake(t, storage, "alice", "validator-1"), mustPower(t, storage, "validator-1"); stake != 100 || power != 100 {
		t.Fatalf("expected unchanged slash state, stake=%d power=%d", stake, power)
	}
	if _, applied := slash(t, base, "offence-1", "validator-1", 1, 2, 5000, 0); !applied {
		t.Fatal("a failed slash must not consume its offence key")
	}
}

func TestSlashValidatorRequiresSnapshotStore(t *testing.T) {
	storage := newBatchOnlyStore()
	if _, _, err := SlashValidator(context.Background(), storage, "k", "validator-1", 1, 2, 1, 0); err == nil {
		t.Fatal("expected a store without prefix export to fail")
	}
}

func TestSlashValidatorSlashesUnbondingEntriesFromTheInfractionOn(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(100)
	delegateStakes(t, module, storage, map[string]string{"alice": "100"})
	mustDeliver(t, module, storage, 3, stakingTx("alice", "undelegate", "alice", "validator-1", "10"))
	mustDeliver(t, module, storage, 8, stakingTx("alice", "undelegate", "alice", "validator-1", "20"))
	slash(t, storage, "offence-1", "validator-1", 5, 9, 5000, 0)
	// 70 bonded stake halves to 35; only the entry created at height 8 (after the infraction at 5) is slashed.
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 35 {
		t.Fatalf("expected bonded stake 35, got %d", stake)
	}
	if amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1"); err != nil || amount != 10+10 {
		t.Fatalf("expected unbonding 10 (untouched) + 10 (halved), got %d err=%v", amount, err)
	}
	if power := mustPower(t, storage, "validator-1"); power != 35 {
		t.Fatalf("expected power 35, got %d", power)
	}
}

// Regression: a jail was never enforced; the validator kept power, proposer slots and rewards.
func TestSlashValidatorJailRemovesValidatorFromSetProposerAndRewards(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "60")
	registerValidator(t, module, storage, 1, "bob", "validator-2", "40")
	update, applied := slash(t, storage, "offence-1", "validator-1", 1, 5, 0, 100)
	if !applied || update.VotingPower != 0 || update.Stake != 60 {
		t.Fatalf("expected a power-0 update while jailed, got applied=%t update=%+v", applied, update)
	}
	if end, jailed, err := JailedUntil(context.Background(), storage, "validator-1"); err != nil || !jailed || end != 100 {
		t.Fatalf("expected jail until 100, got end=%d jailed=%t err=%v", end, jailed, err)
	}
	fund(t, storage, defaultFeeCollector, 100)
	if err := module.EndBlock(vexoapp.Context{Height: 6, Store: storage}); err != nil {
		t.Fatal(err)
	}
	if jailed, free := mustRewards(t, storage, "alice", "validator-1"), mustRewards(t, storage, "bob", "validator-2"); jailed != 0 || free != 100 {
		t.Fatalf("a jailed validator must earn nothing, jailed=%d free=%d", jailed, free)
	}

	// New stake for the jailed validator is bonded but does not bring it back into the set.
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 7})
	fund(t, storage, "carol", 5)
	mustDeliver(t, module, storage, 7, stakingTx("carol", "delegate", "carol", "validator-1", "5"))
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].VotingPower != 0 || updates[0].Stake != 65 {
		t.Fatalf("expected the jailed validator to stay at power 0, got %+v", updates)
	}

	// The jail never ends earlier, and a later jail extends it.
	slash(t, storage, "offence-2", "validator-1", 1, 8, 0, 50)
	if end, _, _ := JailedUntil(context.Background(), storage, "validator-1"); end != 100 {
		t.Fatalf("a shorter jail must not shorten an existing one, got %d", end)
	}
	slash(t, storage, "offence-3", "validator-1", 1, 9, 0, 200)
	if end, _, _ := JailedUntil(context.Background(), storage, "validator-1"); end != 200 {
		t.Fatalf("a longer jail must extend, got %d", end)
	}
	// A jail that already ended at the time of the offence does not jail.
	slash(t, storage, "offence-4", "validator-2", 1, 10, 0, 10)
	if _, jailed, err := JailedUntil(context.Background(), storage, "validator-2"); err != nil || jailed {
		t.Fatalf("an expired jail must not jail, jailed=%t err=%v", jailed, err)
	}
}

// Regression: undelegations were unbounded entries at flat gas, so a delegator could grow the state a
// withdrawal must read without paying for it.
func TestStakingUnbondingEntriesAreCappedAndPaidPerEntry(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithPolicy(Policy{UnbondingDelay: 1000, MaxUnbondingEntries: 3})
	registerValidator(t, module, storage, 1, "alice", "validator-1", "100")
	for height := types.Height(2); height <= 4; height++ {
		mustDeliver(t, module, storage, height, stakingTx("alice", "undelegate", "alice", "validator-1", "1"))
	}
	if result := deliver(module, storage, 5, stakingTx("alice", "undelegate", "alice", "validator-1", "1")); result.Code == 0 || !strings.Contains(result.Log, ErrTooManyUnbondingEntries.Error()) {
		t.Fatalf("expected the entry cap to refuse a fourth entry, got %+v", result)
	}
	// Undelegating again in the block of an existing entry merges into it instead of adding one.
	mustDeliver(t, module, storage, 4, stakingTx("alice", "undelegate", "alice", "validator-1", "2"))
	if amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1"); err != nil || amount != 5 {
		t.Fatalf("expected merged unbonding total 5, got %d err=%v", amount, err)
	}
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 95 {
		t.Fatalf("a refused undelegation changed stake to %d", stake)
	}

	// Gas grows with the entries the transaction reads.
	undelegate := stakingTx("alice", "undelegate", "alice", "validator-1", "1")
	estimate, err := module.EstimateGas(vexoapp.Context{Store: storage}, undelegate)
	if err != nil || estimate != undelegateGasCost+unbondingEntryGasCost*4 {
		t.Fatalf("expected gas for 3 entries plus the new one, got %d err=%v", estimate, err)
	}
	empty, err := module.EstimateGas(vexoapp.Context{Store: newStakingStore(t)}, undelegate)
	if err != nil || empty >= estimate {
		t.Fatalf("expected a pair without entries to be cheaper, got %d vs %d err=%v", empty, estimate, err)
	}
	limited := vexoapp.Context{Height: 4, Store: storage, Gas: vexoapp.NewGasMeter(undelegateGasCost + unbondingEntryGasCost*3)}
	if result := module.DeliverTx(limited, undelegate); result.Code != codeOutOfGas {
		t.Fatalf("expected the per-entry gas to run out, got %+v", result)
	}
	withdraw := stakingTx("alice", "withdraw-unbonded", "alice", "validator-1")
	if gas, err := module.EstimateGas(vexoapp.Context{Store: storage}, withdraw); err != nil || gas != withdrawUnbondedGasCost+unbondingEntryGasCost*3 {
		t.Fatalf("expected withdraw gas for 3 entries, got %d err=%v", gas, err)
	}
}

// The update a jail produces takes the validator out of the validator set and the proposer rotation,
// and the update an unjail produces brings it back with its full identity.
func TestJailedValidatorLeavesValidatorSetAndProposerRotation(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "60")
	registerValidator(t, module, storage, 1, "bob", "validator-2", "40")
	registry, err := vexovalidator.NewStoreRegistry(context.Background(), storage, nil, 1, []vexovalidator.Validator{
		{ID: "validator-1", Address: "alice", PublicKey: types.PublicKey("key-validator-1"), VotingPower: 60, Stake: 60},
		{ID: "validator-2", Address: "bob", PublicKey: types.PublicKey("key-validator-2"), VotingPower: 40, Stake: 40},
	})
	if err != nil {
		t.Fatal(err)
	}
	jailUpdate, _ := slash(t, storage, "offence-1", "validator-1", 1, 5, 0, 20)
	set, _, err := registry.StageValidatorUpdatesAt(context.Background(), 6, []types.ValidatorUpdate{jailUpdate})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := set.Get("validator-1"); found {
		t.Fatal("a jailed validator must leave the validator set")
	}
	for view := types.Round(0); view < 6; view++ {
		if proposer, ok := vexovalidator.SelectProposer(set.List(), view); !ok || proposer != "validator-2" {
			t.Fatalf("view %d: expected the only free validator to propose, got %q ok=%t", view, proposer, ok)
		}
	}
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 20})
	mustDeliver(t, module, storage, 20, stakingTx("alice", "unjail", "validator-1"))
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	set, _, err = registry.StageValidatorUpdatesAt(context.Background(), 21, updates)
	if err != nil {
		t.Fatal(err)
	}
	rejoined, found := set.Get("validator-1")
	if !found || rejoined.VotingPower != 60 || string(rejoined.PublicKey) != "key-validator-1" {
		t.Fatalf("expected the unjailed validator to rejoin with its key and power, got %+v found=%t", rejoined, found)
	}
}

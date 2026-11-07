package staking

import (
	"context"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

// Regression: staking delegate/undelegate/claim/withdraw/unjail never checked the transaction signer,
// so anybody could move or lock another account's funds by naming it in the arguments.

func TestStakingActionsRequireTheDelegatorToSign(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(5)
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	fund(t, storage, "victim", 100)
	mustDeliver(t, module, storage, 1, stakingTx("victim", "delegate", "victim", "validator-1", "40"))
	mustDeliver(t, module, storage, 2, stakingTx("victim", "undelegate", "victim", "validator-1", "10"))
	if err := setUint64(context.Background(), storage, ModuleName, rewardKey("victim", "validator-1"), 7); err != nil {
		t.Fatal(err)
	}

	for _, attempt := range []struct {
		name string
		tx   types.Tx
	}{
		{"delegate without signer", types.Tx("staking:delegate:victim:validator-1:50")},
		{"delegate as attacker", stakingTx("attacker", "delegate", "victim", "validator-1", "50")},
		{"undelegate without signer", types.Tx("staking:undelegate:victim:validator-1:30")},
		{"undelegate as attacker", stakingTx("attacker", "undelegate", "victim", "validator-1", "30")},
		{"claim as attacker", stakingTx("attacker", "claim-rewards", "victim", "validator-1")},
		{"claim without signer", types.Tx("staking:claim-rewards:victim:validator-1")},
		{"withdraw as attacker", stakingTx("attacker", "withdraw-unbonded", "victim", "validator-1")},
		{"withdraw without signer", types.Tx("staking:withdraw-unbonded:victim:validator-1")},
		{"signer is a prefix of the delegator", stakingTx("vic", "delegate", "victim", "validator-1", "50")},
	} {
		result := deliver(module, storage, 20, attempt.tx)
		if result.Code != codeUnauthorized {
			t.Fatalf("%s: expected unauthorized result, got %+v", attempt.name, result)
		}
	}
	if stake := mustStake(t, storage, "victim", "validator-1"); stake != 30 {
		t.Fatalf("victim stake changed to %d", stake)
	}
	if balance := mustBalance(t, storage, "victim"); balance != 60 {
		t.Fatalf("victim balance changed to %d", balance)
	}
	if amount, err := UnbondingAmount(context.Background(), storage, "victim", "validator-1"); err != nil || amount != 10 {
		t.Fatalf("victim unbonding changed to %d err=%v", amount, err)
	}
	if rewards := mustRewards(t, storage, "victim", "validator-1"); rewards != 7 {
		t.Fatalf("victim rewards changed to %d", rewards)
	}
	mustDeliver(t, module, storage, 20, stakingTx("victim", "claim-rewards", "victim", "validator-1"))
	if balance := mustBalance(t, storage, "victim"); balance != 67 {
		t.Fatalf("expected the signed claim to pay 7, balance %d", balance)
	}
}

func TestStakingTreatsEquivalentHexSignerSpellingsAsTheSameAccount(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	account := types.Address("0x00000000000000000000000000000000000000aa")
	fund(t, storage, account, 10)
	mustDeliver(t, module, storage, 1, stakingTx("0xAA", "delegate", string(account), "validator-1", "4"))
	if stake := mustStake(t, storage, account, "validator-1"); stake != 4 {
		t.Fatalf("expected stake 4, got %d", stake)
	}
	if stake := mustStake(t, storage, "0xaa", "validator-1"); stake != 4 {
		t.Fatalf("expected one ledger for both spellings, got %d", stake)
	}
}

func TestStakingFailedTransactionsNeverPanicOrChangeState(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	for _, tx := range []types.Tx{
		types.Tx("staking:"),
		types.Tx("staking:delegate"),
		types.Tx("staking:delegate:alice:validator-1:1:extra:parts:signer=alice"),
		stakingTx("alice", "delegate", "alice", "validator-1", "0"),
		stakingTx("alice", "delegate", "alice", "validator-1", "-1"),
		stakingTx("alice", "delegate", "alice", "missing-validator", "1"),
		stakingTx("alice", "undelegate", "alice", "validator-1", "1"),
		stakingTx("alice", "withdraw-unbonded", "alice", "validator-1"),
		stakingTx("alice", "claim-rewards", "alice", "validator-1"),
		stakingTx("alice", "unjail", "validator-1"),
		stakingTx("alice", "set-commission", "validator-1", "1"),
		stakingTx("alice", "set-commission", "validator-1", "x"),
		stakingTx("alice", "create-validator", "alice", "validator-2", "1", "!!!"),
	} {
		if result := deliver(module, storage, 3, tx); result.Code == 0 {
			t.Fatalf("expected %q to fail", tx)
		}
	}
	if power := mustPower(t, storage, "validator-1"); power != 10 {
		t.Fatalf("expected validator power untouched, got %d", power)
	}
}

func TestStakingUnjailRequiresOperatorAndExpiredJail(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	if result := deliver(module, storage, 2, stakingTx("operator", "unjail", "validator-1")); result.Code == 0 || !strings.Contains(result.Log, ErrValidatorNotJailed.Error()) {
		t.Fatalf("expected not-jailed failure, got %+v", result)
	}
	if _, applied, err := SlashValidator(context.Background(), storage, "offence-1", "validator-1", 1, 5, 0, 50); err != nil || !applied {
		t.Fatalf("unexpected jail result applied=%t err=%v", applied, err)
	}
	if result := deliver(module, storage, 20, stakingTx("attacker", "unjail", "validator-1")); result.Code != codeUnauthorized {
		t.Fatalf("expected unauthorized unjail by non-operator, got %+v", result)
	}
	if result := deliver(module, storage, 20, types.Tx("staking:unjail:validator-1")); result.Code != codeUnauthorized {
		t.Fatalf("expected unauthorized unsigned unjail, got %+v", result)
	}
	if result := deliver(module, storage, 20, stakingTx("operator", "unjail", "validator-1")); result.Code == 0 || !strings.Contains(result.Log, ErrJailNotExpired.Error()) {
		t.Fatalf("expected jail-not-expired failure, got %+v", result)
	}
	if end, jailed, err := JailedUntil(context.Background(), storage, "validator-1"); err != nil || !jailed || end != 50 {
		t.Fatalf("expected jail until 50, got end=%d jailed=%t err=%v", end, jailed, err)
	}
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 50})
	mustDeliver(t, module, storage, 50, stakingTx("operator", "unjail", "validator-1"))
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].VotingPower != 10 {
		t.Fatalf("expected unjail to restore voting power 10, got %+v", updates)
	}
	if _, jailed, err := JailedUntil(context.Background(), storage, "validator-1"); err != nil || jailed {
		t.Fatalf("expected validator to be free, jailed=%t err=%v", jailed, err)
	}
}

func TestStakingSetCommissionRequiresOperator(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "10")
	if result := deliver(module, storage, 2, stakingTx("attacker", "set-commission", "validator-1", "9000")); result.Code != codeUnauthorized {
		t.Fatalf("expected unauthorized commission change, got %+v", result)
	}
	if commission, err := Commission(context.Background(), storage, "validator-1"); err != nil || commission != 0 {
		t.Fatalf("expected commission untouched, got %d err=%v", commission, err)
	}
}

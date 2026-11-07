package staking

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestStakingModuleDistributesFeesAndClaimsRewards(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "40")
	registerValidator(t, module, storage, 1, "bob", "validator-2", "60")
	fund(t, storage, defaultFeeCollector, 30)
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	aliceRewards, bobRewards := mustRewards(t, storage, "alice", "validator-1"), mustRewards(t, storage, "bob", "validator-2")
	if collector := mustBalance(t, storage, defaultFeeCollector); aliceRewards != 12 || bobRewards != 18 || collector != 0 {
		t.Fatalf("unexpected rewards alice=%d bob=%d collector=%d", aliceRewards, bobRewards, collector)
	}
	mustDeliver(t, module, storage, 2, stakingTx("alice", "claim-rewards", "alice", "validator-1"))
	if balance, rewards := mustBalance(t, storage, "alice"), mustRewards(t, storage, "alice", "validator-1"); balance != 12 || rewards != 0 {
		t.Fatalf("unexpected claim accounting balance=%d rewards=%d", balance, rewards)
	}
}

func TestStakingFeeDistributionRequiresSnapshotWhenFeesExist(t *testing.T) {
	storage := newBatchOnlyStore()
	module := NewModule()
	fund(t, storage, defaultFeeCollector, 10)
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); !errors.Is(err, ErrStakingSnapshot) {
		t.Fatalf("expected snapshot requirement when fees exist, got %v", err)
	}
}

func TestStakingModuleUsesConfiguredFeeCollector(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithFeeCollector("treasury")
	registerValidator(t, module, storage, 1, "alice", "validator-1", "100")
	fund(t, storage, "treasury", 10)
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	if rewards, treasury := mustRewards(t, storage, "alice", "validator-1"), mustBalance(t, storage, "treasury"); rewards != 10 || treasury != 0 {
		t.Fatalf("expected configured collector rewards=10 treasury=0, got rewards=%d treasury=%d", rewards, treasury)
	}
}

func TestStakingModuleDistributesValidatorCommission(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "1")
	fund(t, storage, "alice", 99)
	mustDeliver(t, module, storage, 1, stakingTx("alice", "delegate", "alice", "validator-1", "99"))
	mustDeliver(t, module, storage, 1, stakingTx("operator", "set-commission", "validator-1", "1000"))
	fund(t, storage, defaultFeeCollector, 100)
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	// 10 commission to the operator, 90 shared by stake: alice 99%, operator 1%.
	if operator, alice := mustRewards(t, storage, "operator", "validator-1"), mustRewards(t, storage, "alice", "validator-1"); operator != 10+1 || alice != 89 {
		t.Fatalf("unexpected commission accounting operator=%d alice=%d", operator, alice)
	}
}

func TestStakingModuleEnforcesPolicyCommissionCap(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithPolicy(Policy{MaxCommissionBPS: 500})
	registerValidator(t, module, storage, 1, "operator", "validator-1", "100")
	if result := deliver(module, storage, 1, stakingTx("operator", "set-commission", "validator-1", "600")); result.Code == 0 {
		t.Fatalf("expected commission cap rejection, got %+v", result)
	}
	mustDeliver(t, module, storage, 1, stakingTx("operator", "set-commission", "validator-1", "500"))
	fund(t, storage, defaultFeeCollector, 100)
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	// The operator is the only delegator, so commission and stake share both land with it.
	if rewards := mustRewards(t, storage, "operator", "validator-1"); rewards != 100 {
		t.Fatalf("expected all fees with the operator, got %d", rewards)
	}
	if commission, err := Commission(context.Background(), storage, "validator-1"); err != nil || commission != 500 {
		t.Fatalf("expected stored commission 500, got %d err=%v", commission, err)
	}
}

func TestStakingModuleAssignsRewardRoundingRemainderToOperator(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "operator", "validator-1", "1")
	fund(t, storage, "bob", 1)
	mustDeliver(t, module, storage, 1, stakingTx("bob", "delegate", "bob", "validator-1", "1"))
	fund(t, storage, "carol", 1)
	mustDeliver(t, module, storage, 1, stakingTx("carol", "delegate", "carol", "validator-1", "1"))
	fund(t, storage, defaultFeeCollector, 1)
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	if rewards, collector := mustRewards(t, storage, "operator", "validator-1"), mustBalance(t, storage, defaultFeeCollector); rewards != 1 || collector != 0 {
		t.Fatalf("expected operator dust reward and empty collector, reward=%d collector=%d", rewards, collector)
	}
}

func TestStakingModulePolicyGasQueriesAndErrorEdges(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithPolicy(Policy{UnbondingDelay: 0, FeeCollector: "", MaxCommissionBPS: 20_000})
	policy := module.Policy()
	if policy.UnbondingDelay != defaultUnbondingDelay || policy.FeeCollector != defaultFeeCollector || policy.MaxCommissionBPS != commissionDenominatorBPS || policy.MaxUnbondingEntries != defaultMaxUnbondingEntries {
		t.Fatalf("expected normalized policy, got %+v", policy)
	}
	if clone := module.CloneModule(); clone.Name() != ModuleName {
		t.Fatalf("unexpected clone: %+v", clone)
	}
	createTx := stakingTx("alice", "create-validator", "alice", "validator-1", "40", keyArg("validator-key"))
	if gas, err := module.EstimateGas(vexoapp.Context{}, createTx); err != nil || gas != createValidatorGasCost {
		t.Fatalf("unexpected create-validator gas=%d err=%v", gas, err)
	}
	fund(t, storage, "alice", 100)
	if result := module.DeliverTx(vexoapp.Context{Height: 5, Store: storage, Gas: vexoapp.NewGasMeter(1)}, createTx); result.Code != codeOutOfGas {
		t.Fatalf("expected gas failure, got %+v", result)
	}
	mustDeliver(t, module, storage, 5, createTx)
	delegateTx := stakingTx("alice", "delegate", "alice", "validator-1", "5")
	if gas, err := module.EstimateGas(vexoapp.Context{}, delegateTx); err != nil || gas != delegateGasCost {
		t.Fatalf("unexpected delegate gas=%d err=%v", gas, err)
	}
	for name, path := range map[string][]string{
		"stake":      {"stake", "alice", "validator-1"},
		"validator":  {"validator", "validator-1"},
		"unbonding":  {"unbonding", "alice", "validator-1"},
		"rewards":    {"rewards", "alice", "validator-1"},
		"commission": {"commission", "validator-1"},
		"tombstone":  {"tombstone", "validator-1"},
	} {
		if response := module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: path}); response.Code != 0 {
			t.Fatalf("expected %s query to succeed, got %+v", name, response)
		}
	}
	if result := deliver(module, storage, 5, stakingTx("alice", "delegate", "alice", "validator-1", "not-a-number")); result.Code != codeBadArguments {
		t.Fatalf("expected parse failure, got %+v", result)
	}
	if _, err := module.EstimateGas(vexoapp.Context{}, types.Tx("staking:bad")); err != ErrInvalidStakingTx {
		t.Fatalf("expected invalid gas estimate failure, got %v", err)
	}
}

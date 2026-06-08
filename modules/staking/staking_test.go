package staking

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestStakingModuleDelegatesAndEmitsValidatorUpdate(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	result := module.DeliverTx(vexoapp.Context{Height: 7, Store: storage}, types.Tx("staking:delegate:alice:validator-1:40:"+publicKey))
	if result.Code != 0 {
		t.Fatalf("unexpected delegate result: %+v", result)
	}
	stake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if stake != 40 {
		t.Fatalf("expected stake 40, got %d", stake)
	}
	balance, err := bankBalance(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if balance != 60 {
		t.Fatalf("expected bank balance 60, got %d", balance)
	}
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].ID != "validator-1" || updates[0].VotingPower != 40 || string(updates[0].PublicKey) != "validator-key" {
		t.Fatalf("unexpected validator updates: %+v", updates)
	}
}

func TestStakingModuleUndelegatesAndRecordsUnbonding(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	if result := module.DeliverTx(vexoapp.Context{Height: 7, Store: storage}, types.Tx("staking:delegate:alice:validator-1:40:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected delegate result: %+v", result)
	}
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 8})
	result := module.DeliverTx(vexoapp.Context{Height: 8, Store: storage}, types.Tx("staking:undelegate:alice:validator-1:15"))
	if result.Code != 0 {
		t.Fatalf("unexpected undelegate result: %+v", result)
	}
	stake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if stake != 25 {
		t.Fatalf("expected stake 25, got %d", stake)
	}
	releaseHeight, err := UnbondingReleaseHeight(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if releaseHeight != 18 {
		t.Fatalf("expected release height 18, got %d", releaseHeight)
	}
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].VotingPower != 25 {
		t.Fatalf("unexpected validator updates: %+v", updates)
	}
}

func TestStakingDelegateBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	storage := failingBatchStore{Store: base, err: errors.New("batch failed")}
	module := NewModuleWithUnbondingDelay(10)
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	result := module.DeliverTx(vexoapp.Context{Height: 7, Store: storage}, types.Tx("staking:delegate:alice:validator-1:40:"+publicKey))
	if result.Code == 0 {
		t.Fatalf("expected delegate batch failure, got %+v", result)
	}
	stake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	balance, err := bankBalance(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	power, err := ValidatorPower(context.Background(), storage, "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if stake != 0 || balance != 100 || power != 0 {
		t.Fatalf("expected unchanged state after batch failure, stake=%d balance=%d power=%d", stake, balance, power)
	}
}

func TestStakingAppliesSlashingPenaltyToDelegationLedger(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	if err := setStake(context.Background(), storage, "alice", "validator-1", 40); err != nil {
		t.Fatal(err)
	}
	if err := setStake(context.Background(), storage, "bob", "validator-1", 60); err != nil {
		t.Fatal(err)
	}
	if err := setValidatorPower(context.Background(), storage, "validator-1", 100); err != nil {
		t.Fatal(err)
	}
	receipt := testSlashReceipt("validator-1", 100, 75)
	if err := module.ApplySlashingPenalty(context.Background(), storage, receipt); err != nil {
		t.Fatal(err)
	}
	aliceStake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	bobStake, err := Stake(context.Background(), storage, "bob", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	power, err := ValidatorPower(context.Background(), storage, "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if aliceStake != 30 || bobStake != 45 || power != 75 {
		t.Fatalf("unexpected slash ledger alice=%d bob=%d power=%d", aliceStake, bobStake, power)
	}
}

func TestStakingSlashingPenaltyIsIdempotent(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	if err := setStake(context.Background(), storage, "alice", "validator-1", 100); err != nil {
		t.Fatal(err)
	}
	receipt := testSlashReceipt("validator-1", 100, 50)
	if err := module.ApplySlashingPenalty(context.Background(), storage, receipt); err != nil {
		t.Fatal(err)
	}
	if err := module.ApplySlashingPenalty(context.Background(), storage, receipt); err != nil {
		t.Fatal(err)
	}
	stake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if stake != 50 {
		t.Fatalf("expected idempotent stake 50, got %d", stake)
	}
}

func TestStakingFullSlashRemovesDelegations(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	if err := setStake(context.Background(), storage, "alice", "validator-1", 100); err != nil {
		t.Fatal(err)
	}
	if err := module.ApplySlashingPenalty(context.Background(), storage, testSlashReceipt("validator-1", 100, 0)); err != nil {
		t.Fatal(err)
	}
	stake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	power, err := ValidatorPower(context.Background(), storage, "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if stake != 0 || power != 0 {
		t.Fatalf("expected full slash to zero ledger, stake=%d power=%d", stake, power)
	}
	tombstoned, err := Tombstoned(context.Background(), storage, "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if !tombstoned {
		t.Fatal("expected full slash to tombstone validator")
	}
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-public-key"))
	if result := module.DeliverTx(vexoapp.Context{Height: 11, Store: storage}, types.Tx("staking:delegate:alice:validator-1:1:"+publicKey)); result.Code == 0 {
		t.Fatal("expected tombstoned validator delegation to fail")
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 11, Store: storage}, types.Tx("staking:unjail:validator-1")); result.Code == 0 {
		t.Fatal("expected tombstoned validator unjail to fail")
	}
}

func TestStakingSlashingBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	storage := failingBatchStore{Store: base, err: errors.New("batch failed")}
	module := NewModule()
	if err := setStake(context.Background(), storage, "alice", "validator-1", 100); err != nil {
		t.Fatal(err)
	}
	if err := setValidatorPower(context.Background(), storage, "validator-1", 100); err != nil {
		t.Fatal(err)
	}
	if err := module.ApplySlashingPenalty(context.Background(), storage, testSlashReceipt("validator-1", 100, 50)); err == nil {
		t.Fatal("expected slashing batch failure")
	}
	stake, err := Stake(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	power, err := ValidatorPower(context.Background(), storage, "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if stake != 100 || power != 100 {
		t.Fatalf("expected unchanged slash state, stake=%d power=%d", stake, power)
	}
}

func TestStakingModuleQueries(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	if err := setStake(context.Background(), storage, "alice", "validator-1", 50); err != nil {
		t.Fatal(err)
	}
	if err := setValidatorPower(context.Background(), storage, "validator-1", 75); err != nil {
		t.Fatal(err)
	}
	response := module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: []string{"stake", "alice", "validator-1"}})
	if response.Code != 0 || string(response.Value) != "50" {
		t.Fatalf("unexpected stake query: %+v", response)
	}
	response = module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: []string{"validator", "validator-1"}})
	if response.Code != 0 || string(response.Value) != "75" {
		t.Fatalf("unexpected validator query: %+v", response)
	}
	if err := setUint64(context.Background(), storage, ModuleName, rewardKey("alice", "validator-1"), 11); err != nil {
		t.Fatal(err)
	}
	if err := setUint64(context.Background(), storage, ModuleName, commissionKey("validator-1"), 500); err != nil {
		t.Fatal(err)
	}
	response = module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: []string{"rewards", "alice", "validator-1"}})
	if response.Code != 0 || string(response.Value) != "11" {
		t.Fatalf("unexpected rewards query: %+v", response)
	}
	response = module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: []string{"commission", "validator-1"}})
	if response.Code != 0 || string(response.Value) != "500" {
		t.Fatalf("unexpected commission query: %+v", response)
	}
}

func testSlashReceipt(validatorID types.ValidatorID, previousPower types.VotingPower, remainingPower types.VotingPower) slashing.PenaltyReceipt {
	return slashing.PenaltyReceipt{
		Evidence: slashing.Evidence{
			Type:      slashing.EvidenceConflictingVote,
			Validator: validatorID,
			Height:    1,
			Proof:     []byte("proof-" + string(validatorID)),
		},
		PreviousPower:  previousPower,
		RemainingPower: remainingPower,
	}
}

func TestStakingModuleDistributesFeesAndClaimsRewards(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, "bob", 100); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, defaultFeeCollector, 30); err != nil {
		t.Fatal(err)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:alice:validator-1:40:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected alice delegate result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:bob:validator-2:60:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected bob delegate result: %+v", result)
	}
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	aliceRewards, err := Rewards(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	bobRewards, err := Rewards(context.Background(), storage, "bob", "validator-2")
	if err != nil {
		t.Fatal(err)
	}
	collectorBalance, err := bankBalance(context.Background(), storage, defaultFeeCollector)
	if err != nil {
		t.Fatal(err)
	}
	if aliceRewards != 12 || bobRewards != 18 || collectorBalance != 0 {
		t.Fatalf("unexpected rewards alice=%d bob=%d collector=%d", aliceRewards, bobRewards, collectorBalance)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 2, Store: storage}, types.Tx("staking:claim-rewards:alice:validator-1")); result.Code != 0 {
		t.Fatalf("unexpected claim result: %+v", result)
	}
	aliceBalance, err := bankBalance(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	aliceRewards, err = Rewards(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if aliceBalance != 72 || aliceRewards != 0 {
		t.Fatalf("unexpected claim accounting balance=%d rewards=%d", aliceBalance, aliceRewards)
	}
}

func TestStakingModuleUsesConfiguredFeeCollector(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithFeeCollector("treasury")
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, "treasury", 10); err != nil {
		t.Fatal(err)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:alice:validator-1:100:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected delegate result: %+v", result)
	}
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	rewards, err := Rewards(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	treasuryBalance, err := bankBalance(context.Background(), storage, "treasury")
	if err != nil {
		t.Fatal(err)
	}
	if rewards != 10 || treasuryBalance != 0 {
		t.Fatalf("expected configured collector rewards=10 treasury=0, got rewards=%d treasury=%d", rewards, treasuryBalance)
	}
}

func TestStakingModuleDistributesValidatorCommission(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, defaultFeeCollector, 100); err != nil {
		t.Fatal(err)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:alice:validator-1:100:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected delegate result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:set-commission:validator-1:1000:signer=validator-1")); result.Code != 0 {
		t.Fatalf("unexpected commission result: %+v", result)
	}
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	validatorRewards, err := Rewards(context.Background(), storage, "validator-1", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	aliceRewards, err := Rewards(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if validatorRewards != 10 || aliceRewards != 90 {
		t.Fatalf("unexpected commission accounting validator=%d alice=%d", validatorRewards, aliceRewards)
	}
}

func TestStakingModuleEnforcesPolicyCommissionCap(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithPolicy(Policy{MaxCommissionBPS: 500})
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, defaultFeeCollector, 100); err != nil {
		t.Fatal(err)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:alice:validator-1:100:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected delegate result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:set-commission:validator-1:600:signer=validator-1")); result.Code == 0 {
		t.Fatalf("expected commission cap rejection, got %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:set-commission:validator-1:500:signer=validator-1")); result.Code != 0 {
		t.Fatalf("unexpected capped commission result: %+v", result)
	}
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	validatorRewards, err := Rewards(context.Background(), storage, "validator-1", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	aliceRewards, err := Rewards(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if validatorRewards != 5 || aliceRewards != 95 {
		t.Fatalf("expected 5%% commission with 500bps cap, validator=%d alice=%d", validatorRewards, aliceRewards)
	}
}

func TestStakingModuleAssignsRewardRoundingRemainderToValidator(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	publicKey := base64.StdEncoding.EncodeToString([]byte("validator-key"))
	if err := setBankBalance(context.Background(), storage, "alice", 10); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, "bob", 10); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, defaultFeeCollector, 1); err != nil {
		t.Fatal(err)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:alice:validator-1:1:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected alice delegate result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:delegate:bob:validator-1:1:"+publicKey)); result.Code != 0 {
		t.Fatalf("unexpected bob delegate result: %+v", result)
	}
	if err := module.EndBlock(vexoapp.Context{Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	validatorRewards, err := Rewards(context.Background(), storage, "validator-1", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	collectorBalance, err := bankBalance(context.Background(), storage, defaultFeeCollector)
	if err != nil {
		t.Fatal(err)
	}
	if validatorRewards != 1 || collectorBalance != 0 {
		t.Fatalf("expected validator dust reward and empty collector, reward=%d collector=%d", validatorRewards, collectorBalance)
	}
}

func TestStakingClaimRewardsBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	storage := failingBatchStore{Store: base, err: errors.New("batch failed")}
	module := NewModule()
	if err := setBankBalance(context.Background(), storage, "alice", 10); err != nil {
		t.Fatal(err)
	}
	if err := setUint64(context.Background(), storage, ModuleName, rewardKey("alice", "validator-1"), 5); err != nil {
		t.Fatal(err)
	}
	result := module.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, types.Tx("staking:claim-rewards:alice:validator-1"))
	if result.Code == 0 {
		t.Fatalf("expected claim batch failure, got %+v", result)
	}
	balance, err := bankBalance(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	rewards, err := Rewards(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 || rewards != 5 {
		t.Fatalf("expected unchanged state after claim failure, balance=%d rewards=%d", balance, rewards)
	}
}

func TestStakingCLICommands(t *testing.T) {
	command := stakingCLICommand()
	if command.Name != ModuleName || len(command.Children) != 2 {
		t.Fatalf("unexpected staking command: %+v", command)
	}
	var output bytes.Buffer
	if err := runClaimRewardsCLI(&output, []string{"alice", "validator-1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "staking:claim-rewards:alice:validator-1") {
		t.Fatalf("unexpected claim cli output: %s", output.String())
	}
	output.Reset()
	if err := runSetCommissionCLI(&output, []string{"validator-1", "500", "--signer", "validator-1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "staking:set-commission:validator-1:500:signer=validator-1") {
		t.Fatalf("unexpected commission cli output: %s", output.String())
	}
	output.Reset()
	if err := runRewardsQueryCLI(&output, []string{"alice", "validator-1"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: staking/rewards/alice/validator-1" {
		t.Fatalf("unexpected rewards query cli output: %s", output.String())
	}
	output.Reset()
	if err := runTombstoneQueryCLI(&output, []string{"validator-1"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: staking/tombstone/validator-1" {
		t.Fatalf("unexpected tombstone query cli output: %s", output.String())
	}
}

func newStakingStore(t *testing.T) store.Store {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	return storage
}

type failingBatchStore struct {
	store.Store
	err error
}

func (storage failingBatchStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	return storage.err
}

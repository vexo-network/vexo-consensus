package staking

import (
	"context"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// stakingTx builds a staking payload as the ante handler delivers it: positional arguments followed by
// the authenticated signer tag.
func stakingTx(signer string, parts ...string) types.Tx {
	return types.Tx("staking:" + strings.Join(parts, ":") + ":signer=" + signer)
}

func keyArg(label string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(label))
}

func deliver(module *Module, storage vexoapp.StateStore, height types.Height, tx types.Tx) types.Result {
	return module.DeliverTx(vexoapp.Context{Height: height, Store: storage}, tx)
}

func mustDeliver(t *testing.T, module *Module, storage vexoapp.StateStore, height types.Height, tx types.Tx) {
	t.Helper()
	if result := deliver(module, storage, height, tx); result.Code != 0 {
		t.Fatalf("unexpected result for %q: %+v", tx, result)
	}
}

func fund(t *testing.T, storage vexoapp.StateStore, address types.Address, amount uint64) {
	t.Helper()
	if err := setBankBalance(context.Background(), storage, address, amount); err != nil {
		t.Fatal(err)
	}
}

// registerValidator funds operator and registers validatorID with the operator's self-delegation.
func registerValidator(t *testing.T, module *Module, storage vexoapp.StateStore, height types.Height, operator string, validatorID string, selfStake string) {
	t.Helper()
	amount, err := parseAmount(selfStake)
	if err != nil {
		t.Fatal(err)
	}
	fund(t, storage, types.Address(operator), amount)
	mustDeliver(t, module, storage, height, stakingTx(operator, "create-validator", operator, validatorID, selfStake, keyArg("key-"+validatorID)))
}

func mustStake(t *testing.T, storage vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) uint64 {
	t.Helper()
	stake, err := Stake(context.Background(), storage, delegator, validatorID)
	if err != nil {
		t.Fatal(err)
	}
	return stake
}

func mustPower(t *testing.T, storage vexoapp.StateStore, validatorID types.ValidatorID) uint64 {
	t.Helper()
	power, err := ValidatorPower(context.Background(), storage, validatorID)
	if err != nil {
		t.Fatal(err)
	}
	return power
}

func mustBalance(t *testing.T, storage vexoapp.StateStore, address types.Address) uint64 {
	t.Helper()
	balance, err := bankBalance(context.Background(), storage, address)
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

func mustRewards(t *testing.T, storage vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) uint64 {
	t.Helper()
	rewards, err := Rewards(context.Background(), storage, delegator, validatorID)
	if err != nil {
		t.Fatal(err)
	}
	return rewards
}

func TestStakingModuleCreatesValidatorAndEmitsValidatorUpdate(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	fund(t, storage, "alice", 100)
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 7})
	mustDeliver(t, module, storage, 7, stakingTx("alice", "create-validator", "alice", "validator-1", "40", keyArg("validator-key")))
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 40 {
		t.Fatalf("expected stake 40, got %d", stake)
	}
	if balance := mustBalance(t, storage, "alice"); balance != 60 {
		t.Fatalf("expected bank balance 60, got %d", balance)
	}
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].ID != "validator-1" || updates[0].VotingPower != 40 || updates[0].Stake != 40 || string(updates[0].PublicKey) != "validator-key" {
		t.Fatalf("unexpected validator updates: %+v", updates)
	}

	fund(t, storage, "bob", 30)
	mustDeliver(t, module, storage, 7, stakingTx("bob", "delegate", "bob", "validator-1", "10"))
	updates = module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].VotingPower != 50 || string(updates[0].PublicKey) != "validator-key" {
		t.Fatalf("expected one update with the registered key and power 50, got %+v", updates)
	}
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 8})
	if updates := module.ValidatorUpdates(vexoapp.Context{Store: storage}); len(updates) != 0 {
		t.Fatalf("expected no updates in a block without staking changes, got %+v", updates)
	}
}

func TestStakingModuleReadsEVMStyleBankBalance(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	if err := storage.Set(context.Background(), bankNamespace, bankBalanceKey("alice"), []byte{100}); err != nil {
		t.Fatal(err)
	}
	mustDeliver(t, module, storage, 7, stakingTx("alice", "create-validator", "alice", "validator-1", "40", keyArg("validator-key")))
	if balance := mustBalance(t, storage, "alice"); balance != 60 {
		t.Fatalf("expected remaining balance 60, got %d", balance)
	}
}

func TestStakingModuleDelegatesFromUint256BankBalance(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	largeBalance := new(big.Int).Lsh(big.NewInt(1), 80)
	if err := setBankBalanceBig(context.Background(), storage, "alice", largeBalance); err != nil {
		t.Fatal(err)
	}
	mustDeliver(t, module, storage, 7, stakingTx("alice", "create-validator", "alice", "validator-1", "10", keyArg("validator-key")))
	balance, err := bankBalanceBig(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if expected := new(big.Int).Sub(largeBalance, big.NewInt(10)); balance.Cmp(expected) != 0 {
		t.Fatalf("expected remaining balance %s, got %s", expected, balance)
	}
}

func TestStakingModuleRejectsUint256RewardOverflow(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	fund(t, storage, "alice", 1)
	if err := storage.Set(context.Background(), ModuleName, rewardKey("alice", "validator-1"), maxUint256.Bytes()); err != nil {
		t.Fatal(err)
	}
	result := deliver(module, storage, 7, stakingTx("alice", "claim-rewards", "alice", "validator-1"))
	if result.Code == 0 || !strings.Contains(result.Log, ErrStakeOverflow.Error()) {
		t.Fatalf("expected reward overflow rejection, got %+v", result)
	}
	if balance := mustBalance(t, storage, "alice"); balance != 1 {
		t.Fatalf("expected unchanged balance 1, got %d", balance)
	}
}

func TestStakingModuleUndelegatesAndRecordsUnbonding(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	registerValidator(t, module, storage, 7, "alice", "validator-1", "40")
	module.BeginBlock(vexoapp.Context{}, types.Header{Height: 8})
	mustDeliver(t, module, storage, 8, stakingTx("alice", "undelegate", "alice", "validator-1", "15"))
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 25 {
		t.Fatalf("expected stake 25, got %d", stake)
	}
	releaseHeight, err := UnbondingReleaseHeight(context.Background(), storage, "alice", "validator-1")
	if err != nil || releaseHeight != 18 {
		t.Fatalf("expected release height 18, got %d err=%v", releaseHeight, err)
	}
	if amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1"); err != nil || amount != 15 {
		t.Fatalf("expected unbonding amount 15, got %d err=%v", amount, err)
	}
	updates := module.ValidatorUpdates(vexoapp.Context{Store: storage})
	if len(updates) != 1 || updates[0].VotingPower != 25 {
		t.Fatalf("unexpected validator updates: %+v", updates)
	}
}

func TestStakingUndelegateWritesOnlyTheNewEntry(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	registerValidator(t, module, storage, 1, "alice", "validator-1", "80")
	for height := types.Height(2); height <= 5; height++ {
		mustDeliver(t, module, storage, height, stakingTx("alice", "undelegate", "alice", "validator-1", "1"))
	}
	recording := &recordingBatchStore{LevelDBStore: storage}
	if result := module.DeliverTx(vexoapp.Context{Height: 6, Store: recording}, stakingTx("alice", "undelegate", "alice", "validator-1", "1")); result.Code != 0 {
		t.Fatalf("unexpected undelegate result: %+v", result)
	}
	entryWrites := 0
	for _, write := range recording.writes {
		if strings.HasPrefix(string(write.Key), unbondingKeyPrefix) {
			entryWrites++
		}
	}
	if entryWrites != 1 {
		t.Fatalf("expected exactly one unbonding entry write for the new entry, got %d in %+v", entryWrites, recording.writes)
	}
}

func TestStakingWithdrawsMaturedUnbonding(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	registerValidator(t, module, storage, 1, "alice", "validator-1", "40")
	fund(t, storage, "alice", 60)
	mustDeliver(t, module, storage, 2, stakingTx("alice", "undelegate", "alice", "validator-1", "15"))
	if result := deliver(module, storage, 11, stakingTx("alice", "withdraw-unbonded", "alice", "validator-1")); result.Code == 0 || !strings.Contains(result.Log, ErrUnbondingNotMature.Error()) {
		t.Fatalf("expected immature withdrawal failure, got %+v", result)
	}
	mustDeliver(t, module, storage, 12, stakingTx("alice", "withdraw-unbonded", "alice", "validator-1"))
	if balance := mustBalance(t, storage, "alice"); balance != 75 {
		t.Fatalf("expected withdrawn bank balance 75, got %d", balance)
	}
	amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1")
	releaseHeight, releaseErr := UnbondingReleaseHeight(context.Background(), storage, "alice", "validator-1")
	if err != nil || releaseErr != nil || amount != 0 || releaseHeight != 0 {
		t.Fatalf("expected cleared unbonding state, amount=%d release=%d err=%v/%v", amount, releaseHeight, err, releaseErr)
	}
}

func TestStakingWithdrawsOnlyMaturedUnbondingEntries(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	registerValidator(t, module, storage, 1, "alice", "validator-1", "80")
	fund(t, storage, "alice", 20)
	mustDeliver(t, module, storage, 2, stakingTx("alice", "undelegate", "alice", "validator-1", "20"))
	mustDeliver(t, module, storage, 7, stakingTx("alice", "undelegate", "alice", "validator-1", "30"))
	if amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1"); err != nil || amount != 50 {
		t.Fatalf("expected total unbonding 50, got %d err=%v", amount, err)
	}
	if releaseHeight, err := UnbondingReleaseHeight(context.Background(), storage, "alice", "validator-1"); err != nil || releaseHeight != 12 {
		t.Fatalf("expected earliest release height 12, got %d err=%v", releaseHeight, err)
	}
	mustDeliver(t, module, storage, 12, stakingTx("alice", "withdraw-unbonded", "alice", "validator-1"))
	if balance := mustBalance(t, storage, "alice"); balance != 40 {
		t.Fatalf("expected balance 40 after first withdrawal, got %d", balance)
	}
	if amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1"); err != nil || amount != 30 {
		t.Fatalf("expected pending unbonding 30, got %d err=%v", amount, err)
	}
	if result := deliver(module, storage, 16, stakingTx("alice", "withdraw-unbonded", "alice", "validator-1")); result.Code == 0 || !strings.Contains(result.Log, ErrUnbondingNotMature.Error()) {
		t.Fatalf("expected remaining entry immature failure, got %+v", result)
	}
	mustDeliver(t, module, storage, 17, stakingTx("alice", "withdraw-unbonded", "alice", "validator-1"))
	if balance := mustBalance(t, storage, "alice"); balance != 70 {
		t.Fatalf("expected balance 70 after all withdrawals, got %d", balance)
	}
}

func TestStakingWithdrawUnbondedBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	storage := failingBatchStore{LevelDBStore: base, err: errors.New("batch failed")}
	module := NewModuleWithUnbondingDelay(1)
	fund(t, storage, "alice", 10)
	entry := unbondingEntry{CreatedHeight: 1, ReleaseHeight: 2, Amount: 5}
	if err := storage.Set(context.Background(), ModuleName, unbondingEntryKey("alice", "validator-1", 1), encodeUnbondingEntry(entry)); err != nil {
		t.Fatal(err)
	}
	if result := deliver(module, storage, 2, stakingTx("alice", "withdraw-unbonded", "alice", "validator-1")); result.Code == 0 {
		t.Fatalf("expected withdraw batch failure, got %+v", result)
	}
	amount, err := UnbondingAmount(context.Background(), storage, "alice", "validator-1")
	if err != nil {
		t.Fatal(err)
	}
	if balance := mustBalance(t, storage, "alice"); balance != 10 || amount != 5 {
		t.Fatalf("expected unchanged withdrawal state, balance=%d amount=%d", balance, amount)
	}
}

func TestStakingDelegateBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	module := NewModuleWithUnbondingDelay(10)
	registerValidator(t, module, base, 1, "operator", "validator-1", "10")
	fund(t, base, "alice", 100)
	storage := failingBatchStore{LevelDBStore: base, err: errors.New("batch failed")}
	if result := deliver(module, storage, 7, stakingTx("alice", "delegate", "alice", "validator-1", "40")); result.Code == 0 {
		t.Fatalf("expected delegate batch failure, got %+v", result)
	}
	if stake := mustStake(t, storage, "alice", "validator-1"); stake != 0 {
		t.Fatalf("expected no stake after batch failure, got %d", stake)
	}
	if balance := mustBalance(t, storage, "alice"); balance != 100 {
		t.Fatalf("expected balance 100 after batch failure, got %d", balance)
	}
	if power := mustPower(t, storage, "validator-1"); power != 10 {
		t.Fatalf("expected power 10 after batch failure, got %d", power)
	}
}

func TestStakingClaimRewardsBatchFailureDoesNotMutateState(t *testing.T) {
	base := newStakingStore(t)
	storage := failingBatchStore{LevelDBStore: base, err: errors.New("batch failed")}
	module := NewModule()
	fund(t, storage, "alice", 10)
	if err := setUint64(context.Background(), storage, ModuleName, rewardKey("alice", "validator-1"), 5); err != nil {
		t.Fatal(err)
	}
	if result := deliver(module, storage, 1, stakingTx("alice", "claim-rewards", "alice", "validator-1")); result.Code == 0 {
		t.Fatalf("expected claim batch failure, got %+v", result)
	}
	if balance, rewards := mustBalance(t, storage, "alice"), mustRewards(t, storage, "alice", "validator-1"); balance != 10 || rewards != 5 {
		t.Fatalf("expected unchanged state after claim failure, balance=%d rewards=%d", balance, rewards)
	}
}

func TestStakingModuleQueries(t *testing.T) {
	storage := newStakingStore(t)
	module := NewModule()
	registerValidator(t, module, storage, 1, "alice", "validator-1", "50")
	fund(t, storage, "bob", 25)
	mustDeliver(t, module, storage, 1, stakingTx("bob", "delegate", "bob", "validator-1", "25"))
	if err := setUint64(context.Background(), storage, ModuleName, rewardKey("alice", "validator-1"), 11); err != nil {
		t.Fatal(err)
	}
	mustDeliver(t, module, storage, 1, stakingTx("alice", "set-commission", "validator-1", "500"))
	for path, expected := range map[string]string{
		"stake/alice/validator-1":             "50",
		"validator/validator-1":               "75",
		"rewards/alice/validator-1":           "11",
		"commission/validator-1":              "500",
		"tombstone/validator-1":               "false",
		"jail/validator-1":                    "0",
		"operator/validator-1":                "alice",
		"unbonding-balance/alice/validator-1": "0",
	} {
		response := module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: strings.Split(path, "/")})
		if response.Code != 0 || string(response.Value) != expected {
			t.Fatalf("unexpected %s query: %+v, expected %s", path, response, expected)
		}
	}
	for _, path := range [][]string{{"bad"}, {"stake", "a/b", "validator-1"}, {"operator", "missing"}} {
		if response := module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: path}); response.Code == 0 {
			t.Fatalf("expected query %v to fail, got %+v", path, response)
		}
	}
}

func newStakingStore(t *testing.T) *store.LevelDBStore {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	return storage
}

type failingBatchStore struct {
	*store.LevelDBStore
	err error
}

func (storage failingBatchStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	return storage.err
}

// recordingBatchStore records every batch a module commits.
type recordingBatchStore struct {
	*store.LevelDBStore
	writes []kvbatch.KVWrite
}

func (storage *recordingBatchStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	storage.writes = append(storage.writes, writes...)
	return storage.LevelDBStore.SetBatch(ctx, writes)
}

type batchOnlyStore struct {
	values map[string][]byte
}

func newBatchOnlyStore() *batchOnlyStore {
	return &batchOnlyStore{values: make(map[string][]byte)}
}

func (storage *batchOnlyStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	storage.values[namespace+"/"+string(key)] = append([]byte(nil), value...)
	return nil
}

func (storage *batchOnlyStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	value, found := storage.values[namespace+"/"+string(key)]
	if !found {
		return nil, store.ErrKeyNotFound
	}
	return append([]byte(nil), value...), nil
}

func (storage *batchOnlyStore) Delete(ctx context.Context, namespace string, key []byte) error {
	delete(storage.values, namespace+"/"+string(key))
	return nil
}

func (storage *batchOnlyStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	for _, write := range writes {
		if write.Delete {
			if err := storage.Delete(ctx, write.Namespace, write.Key); err != nil {
				return err
			}
			continue
		}
		if err := storage.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

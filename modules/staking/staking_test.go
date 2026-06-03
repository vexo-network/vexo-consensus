package staking

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/kvbatch"
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
}

func TestStakingCLICommands(t *testing.T) {
	command := stakingCLICommand()
	if command.Name != ModuleName || len(command.Children) != 2 {
		t.Fatalf("unexpected staking command: %+v", command)
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

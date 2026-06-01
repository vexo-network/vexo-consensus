package runtime

import (
	"context"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRuntimeExecuteBlockUsesConfiguredApplication(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one tx result, got %d", len(response.Results))
	}
	commit, err := application.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if commit.Height != 1 {
		t.Fatalf("expected app committed height 1, got %d", commit.Height)
	}
}

func TestRuntimeExecuteBlockPersistsBlockAndState(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2, ValidatorSetHash: types.Hash{9}},
		Txs:    []types.Tx{[]byte("bank:send")},
	}
	response, err := runtime.ExecuteBlock(context.Background(), block)
	if err != nil {
		t.Fatal(err)
	}

	record, err := storage.BlockByHeight(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if record.AppHash != response.AppHash {
		t.Fatalf("expected stored app hash %x, got %x", response.AppHash, record.AppHash)
	}
	if len(record.StateRoots) != 1 || record.StateRoots[0].Namespace != "bank" {
		t.Fatalf("expected stored block state root summary, got %+v", record.StateRoots)
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 2 || state.ValidatorSetHash != (types.Hash{9}) {
		t.Fatalf("unexpected stored state: %+v", state)
	}
	rootRecord, err := storage.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if rootRecord.Root == (types.Hash{}) {
		t.Fatal("expected non-zero bank state root")
	}

	queriedBlock, err := runtime.BlockByHeight(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if queriedBlock.Hash != record.Hash {
		t.Fatalf("expected runtime block query hash %x, got %x", record.Hash, queriedBlock.Hash)
	}
	queriedState, err := runtime.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if queriedState.Height != 2 {
		t.Fatalf("expected latest state height 2, got %d", queriedState.Height)
	}
	queriedRoot, err := runtime.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if queriedRoot.Root != rootRecord.Root {
		t.Fatalf("expected queried root %x, got %x", rootRecord.Root, queriedRoot.Root)
	}
}

func TestRuntimeExecuteBlockAppliesValidatorUpdates(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "alice", Address: "alice", VotingPower: 2},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs:    []types.Tx{[]byte("staking:update")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ValidatorUpdates) != 2 {
		t.Fatalf("expected two validator updates, got %d", len(response.ValidatorUpdates))
	}

	updatedSet, err := runtime.Validators.ValidatorSet(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := updatedSet.Get("alice")
	if !found || alice.VotingPower != 2 {
		t.Fatalf("expected alice power 2, got %+v found=%v", alice, found)
	}
	bob, found := updatedSet.Get("bob")
	if !found || bob.VotingPower != 1 {
		t.Fatalf("expected bob joined with power 1, got %+v found=%v", bob, found)
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.ValidatorSetHash != updatedSet.Hash() {
		t.Fatal("expected persisted state to use updated validator set hash")
	}
}

func TestRuntimeExecuteBlockRemovesValidatorFromUpdates(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "bob", Address: "bob", VotingPower: 0},
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
		{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 6},
		Txs:    []types.Tx{[]byte("staking:update")},
	}); err != nil {
		t.Fatal(err)
	}
	updatedSet, err := runtime.Validators.ValidatorSet(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := updatedSet.Get("bob"); found {
		t.Fatal("expected bob removed from validator set")
	}
}

func TestRuntimeInjectsStoreIntoAppRuntime(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 3},
		Txs:    []types.Tx{[]byte("bank:set")},
	}); err != nil {
		t.Fatal(err)
	}

	value, err := storage.Get(context.Background(), "bank", []byte("runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "ok" {
		t.Fatalf("expected runtime store value ok, got %s", value)
	}
}

func TestRuntimeRecoversLatestStateIntoAppRuntime(t *testing.T) {
	path := t.TempDir()
	storage, err := store.OpenLevelDB(path)
	if err != nil {
		t.Fatal(err)
	}

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 4},
		Txs:    []types.Tx{[]byte("bank:set")},
	}); err != nil {
		t.Fatal(err)
	}
	expectedCommit, err := application.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.OpenLevelDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	recoveredApplication, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	recoveredRuntime, err := NewWithStore(config.Default("vexo-test"), recoveredApplication, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, reopened)
	if err != nil {
		t.Fatal(err)
	}

	state, err := recoveredRuntime.Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 4 {
		t.Fatalf("expected recovered height 4, got %d", state.Height)
	}
	recoveredCommit, err := recoveredApplication.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if recoveredCommit.Height != expectedCommit.Height || recoveredCommit.AppHash != expectedCommit.AppHash {
		t.Fatalf("unexpected recovered commit: %+v expected %+v", recoveredCommit, expectedCommit)
	}
}

type runtimeModule struct {
	name string
}

func (module *runtimeModule) Name() string {
	return module.name
}

func (module *runtimeModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module *runtimeModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module *runtimeModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module *runtimeModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}

type storeWritingModule struct {
	name string
}

func (module *storeWritingModule) Name() string {
	return module.name
}

func (module *storeWritingModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module *storeWritingModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module *storeWritingModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: "missing store"}
	}
	if err := ctx.Store.Set(context.Background(), "bank", []byte("runtime"), []byte("ok")); err != nil {
		return types.Result{Code: 2, Log: err.Error()}
	}
	return types.Result{}
}

func (module *storeWritingModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}

type validatorUpdateModule struct {
	runtimeModule
	updates []types.ValidatorUpdate
}

func (module *validatorUpdateModule) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	return module.updates
}

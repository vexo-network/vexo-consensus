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
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 2 || state.ValidatorSetHash != (types.Hash{9}) {
		t.Fatalf("unexpected stored state: %+v", state)
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

package runtime

import (
	"context"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
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

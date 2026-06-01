package main

import (
	"context"
	"fmt"
	"io"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func writeDemo(writer io.Writer) error {
	application, err := vexoapp.NewRuntime("vexo-local", []vexoapp.Module{demoModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		return err
	}
	runtime, err := vexoruntime.New(config.Default("vexo-local"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, map[types.Address]types.VotingPower{"alice": 1})
	if err != nil {
		return err
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-local", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	}
	response, err := runtime.ExecuteBlock(context.Background(), block)
	if err != nil {
		return err
	}
	commit, err := application.Commit()
	if err != nil {
		return err
	}

	fmt.Fprintf(writer, "vexo-consensus demo\n")
	fmt.Fprintf(writer, "executed_height: %d\n", commit.Height)
	fmt.Fprintf(writer, "tx_results: %d\n", len(response.Results))
	fmt.Fprintf(writer, "app_hash: %x\n", commit.AppHash)
	return nil
}

type demoModule struct {
	name string
}

func (module demoModule) Name() string {
	return module.name
}

func (module demoModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module demoModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module demoModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module demoModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}

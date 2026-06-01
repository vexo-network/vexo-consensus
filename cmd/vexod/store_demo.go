package main

import (
	"context"
	"fmt"
	"io"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func writeStoreDemo(writer io.Writer, path string) error {
	storage, err := store.OpenLevelDB(path)
	if err != nil {
		return err
	}
	defer storage.Close()

	application, err := vexoapp.NewRuntime("vexo-local", []vexoapp.Module{demoModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		return err
	}
	runtime, err := vexoruntime.NewWithStore(config.Default("vexo-local"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, map[types.Address]types.VotingPower{"alice": 1}, storage)
	if err != nil {
		return err
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-local", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	}
	if _, err := runtime.ExecuteBlock(context.Background(), block); err != nil {
		return err
	}

	storedBlock, err := runtime.BlockByHeight(context.Background(), 1)
	if err != nil {
		return err
	}
	latestState, err := runtime.LatestState(context.Background())
	if err != nil {
		return err
	}

	fmt.Fprintf(writer, "vexo-consensus store demo\n")
	fmt.Fprintf(writer, "stored_height: %d\n", storedBlock.Block.Header.Height)
	fmt.Fprintf(writer, "stored_block_hash: %x\n", storedBlock.Hash)
	fmt.Fprintf(writer, "latest_state_height: %d\n", latestState.Height)
	fmt.Fprintf(writer, "state_roots: %d\n", len(storedBlock.StateRoots))
	return nil
}

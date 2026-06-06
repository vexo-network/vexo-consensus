package main

import (
	"context"
	"fmt"
	"io"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/fairordering"
	appmodules "github.com/vexo-network/vexo-consensus/modules"
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

	cfg := config.Default("vexo-chain")
	application, err := appmodules.NewRuntime("vexo-chain", cfg.Application)
	if err != nil {
		return err
	}
	runtime, err := vexoruntime.NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, map[types.Address]types.VotingPower{"alice": 1}, storage)
	if err != nil {
		return err
	}
	if _, err := application.InitChain(vexoapp.InitChainRequest{Genesis: vexoapp.GenesisState{"bank:alice": []byte("100")}}); err != nil {
		return err
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-chain", Height: 1},
		Txs: fairordering.SortTxsWithSalt(
			[]types.Tx{
				[]byte("bank:send:alice:bob:25"),
				[]byte("bank:mint:carol:7"),
			},
			fairordering.HeightSalt("vexo-chain", 1),
		),
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
	fmt.Fprintf(writer, "bob_balance: %s\n", application.QueryContext(context.Background(), vexoapp.QueryRequest{Path: []string{"bank", "balance", "bob"}}).Value)
	return nil
}

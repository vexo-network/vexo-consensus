package main

import (
	"context"
	"fmt"
	"io"
	"os"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/fairordering"
	appmodules "github.com/vexo-network/vexo-consensus/modules"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func writeDemo(writer io.Writer) error {
	cfg := config.Default("vexo-chain")
	application, err := appmodules.NewRuntime("vexo-chain", cfg.Application)
	if err != nil {
		return err
	}
	dataDir, err := os.MkdirTemp("", "vexo-consensus-demo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	storage, err := store.OpenLevelDB(dataDir)
	if err != nil {
		return err
	}
	defer storage.Close()
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
	fmt.Fprintf(writer, "alice_balance: %s\n", application.Query(vexoapp.QueryRequest{Path: []string{"bank", "balance", "alice"}}).Value)
	fmt.Fprintf(writer, "bob_balance: %s\n", application.Query(vexoapp.QueryRequest{Path: []string{"bank", "balance", "bob"}}).Value)
	fmt.Fprintf(writer, "app_hash: %x\n", commit.AppHash)
	return nil
}

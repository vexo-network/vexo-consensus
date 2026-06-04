package params

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestKeeperSetGetExportAndAuthority(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()

	param, err := keeper.Set(ctx, Change{Module: "staking", Key: "max_validators", Value: []byte("100"), Authority: "gov"})
	if err != nil {
		t.Fatal(err)
	}
	if param.Version != 1 {
		t.Fatalf("expected version 1, got %d", param.Version)
	}
	if _, err := keeper.Set(ctx, Change{Module: "staking", Key: "max_validators", Value: []byte("10"), Authority: "attacker"}); err != ErrUnauthorized {
		t.Fatalf("expected unauthorized change, got %v", err)
	}
	loaded, found, err := keeper.Get(ctx, "staking", "max_validators")
	if err != nil || !found || string(loaded.Value) != "100" {
		t.Fatalf("unexpected loaded param found=%t param=%+v err=%v", found, loaded, err)
	}
	exported, err := keeper.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) != 1 || exported[0].Module != "staking" {
		t.Fatalf("unexpected export: %+v", exported)
	}
}

func TestCLICommandsBuildParamsPayloads(t *testing.T) {
	module := NewModule(nil)
	commands := module.CLICommands()
	if len(commands) != 1 || commands[0].Name != "params" {
		t.Fatalf("unexpected commands: %+v", commands)
	}
	var txOutput bytes.Buffer
	if err := commands[0].Execute(&txOutput, []string{"tx", "set", "gov", "mempool", "min_fee", "1avxo"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(txOutput.String(), "params:set:gov:mempool:min_fee:") {
		t.Fatalf("unexpected tx output: %s", txOutput.String())
	}
	var queryOutput bytes.Buffer
	if err := commands[0].Execute(&queryOutput, []string{"query", "param", "mempool", "min_fee"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(queryOutput.String()) != "params/param/mempool/min_fee" {
		t.Fatalf("unexpected query output: %s", queryOutput.String())
	}
}

func TestModuleTxAndQuery(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule(nil)
	ctx := vexoapp.Context{Ctx: context.Background(), Store: storage, Height: 1}
	if err := module.InitGenesis(ctx, nil); err != nil {
		t.Fatal(err)
	}
	tx := types.Tx("params:set:gov:mempool:min_fee:" + base64.StdEncoding.EncodeToString([]byte("1avxo")))
	result := module.DeliverTx(ctx, tx)
	if result.Code != 0 {
		t.Fatalf("unexpected tx result: %+v", result)
	}
	response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"param", "mempool", "min_fee"}})
	if response.Code != 0 {
		t.Fatalf("unexpected query response: %+v", response)
	}
	var param Param
	if err := json.Unmarshal(response.Value, &param); err != nil {
		t.Fatal(err)
	}
	if string(param.Value) != "1avxo" {
		t.Fatalf("unexpected param: %+v", param)
	}
}

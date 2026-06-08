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

func TestKeeperApplyBatchesVersionsAndRejectsUnauthorized(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewKeeper(storage)
	ctx := context.Background()

	if err := keeper.Apply(ctx, []Change{
		{Module: "mempool", Key: "min_fee", Value: []byte("1avxo"), Authority: "gov"},
		{Module: "consensus", Key: "timeout_commit", Value: []byte("1s"), Authority: "gov"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Apply(ctx, []Change{{Module: "mempool", Key: "min_fee", Value: []byte("2avxo"), Authority: "attacker"}}); err != ErrUnauthorized {
		t.Fatalf("expected unauthorized batch rejection, got %v", err)
	}
	param, found, err := keeper.Get(ctx, "mempool", "min_fee")
	if err != nil || !found {
		t.Fatalf("expected mempool param found=%t err=%v", found, err)
	}
	if string(param.Value) != "1avxo" || param.Version != 1 {
		t.Fatalf("expected unchanged param after rejection, got %+v", param)
	}
	if err := keeper.Apply(ctx, []Change{{Module: "mempool", Key: "min_fee", Value: []byte("2avxo"), Authority: "gov"}}); err != nil {
		t.Fatal(err)
	}
	param, found, err = keeper.Get(ctx, "mempool", "min_fee")
	if err != nil || !found || string(param.Value) != "2avxo" || param.Version != 2 {
		t.Fatalf("unexpected updated param found=%t param=%+v err=%v", found, param, err)
	}
	exported, err := keeper.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) != 2 || exported[0].Module != "consensus" || exported[1].Module != "mempool" {
		t.Fatalf("expected sorted export, got %+v", exported)
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

func TestModuleGenesisPendingEventsAndQueries(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule([]Change{{Module: "execution", Key: "base_fee", Value: []byte("7avxo"), Authority: "gov"}})
	ctx := vexoapp.Context{Ctx: context.Background(), Store: storage, Height: 1}
	if err := module.InitGenesis(ctx, vexoapp.GenesisState{"params:mempool:min_fee": []byte("1avxo")}); err != nil {
		t.Fatal(err)
	}
	all := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"params"}})
	if all.Code != 0 || !bytes.Contains(all.Value, []byte(`"module":"execution"`)) || !bytes.Contains(all.Value, []byte(`"module":"mempool"`)) {
		t.Fatalf("unexpected params query: %+v", all)
	}
	if response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"param", "missing", "key"}}); response.Code == 0 {
		t.Fatalf("expected missing param query to fail")
	}
	if response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"bad"}}); response.Code == 0 {
		t.Fatalf("expected invalid query to fail")
	}
	tx := types.Tx("params:set:gov:execution:base_fee:" + base64.StdEncoding.EncodeToString([]byte("8avxo")))
	result := module.DeliverTx(ctx, tx)
	if result.Code != 0 {
		t.Fatalf("unexpected tx result: %+v", result)
	}
	if len(module.PendingChanges()) != 1 {
		t.Fatalf("expected pending change, got %+v", module.PendingChanges())
	}
	emitted := module.Events(ctx, tx, result)
	if len(emitted) != 1 || emitted[0].Type != "param_set" {
		t.Fatalf("unexpected events: %+v", emitted)
	}
	if err := module.BeginBlock(ctx, types.Header{Height: 2}); err != nil {
		t.Fatal(err)
	}
	if len(module.PendingChanges()) != 0 {
		t.Fatalf("expected pending reset, got %+v", module.PendingChanges())
	}
	if err := module.EndBlock(ctx); err != nil {
		t.Fatal(err)
	}
	if clone := module.CloneModule(); clone.Name() != Namespace {
		t.Fatalf("unexpected clone: %+v", clone)
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

package evm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type testVM struct{}

func (testVM) Name() string { return "evm" }

func (testVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	return contract.Result{
		Output:  append([]byte("out:"), invocation.Input...),
		GasUsed: 7,
		StorageWrites: []contract.StorageWrite{{
			Slot:  "0x0",
			Value: []byte("stored:" + invocation.Method),
		}},
		Logs: []contract.Log{{
			Address: invocation.Contract,
			Topics:  []string{"0x01"},
			Data:    []byte("log"),
			Meta:    map[string]string{"method": invocation.Method},
		}},
	}, nil
}

func TestModuleExecutesAndPersistsReceiptsCodeAndLogs(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry := contract.NewRegistry()
	if err := registry.Register(testVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 3, Store: storage}
	deployTx := types.Tx("evm:deploy:evm:0xaaaa:6001:salt")
	result := module.DeliverTx(ctx, deployTx)
	if result.Code != 0 {
		t.Fatalf("deploy failed: %+v", result)
	}
	var deployReceipt Receipt
	if err := json.Unmarshal(result.Data, &deployReceipt); err != nil {
		t.Fatal(err)
	}
	if deployReceipt.ContractAddress == "" || deployReceipt.GasUsed != 7 || len(deployReceipt.Logs) != 1 {
		t.Fatalf("unexpected deploy receipt: %+v", deployReceipt)
	}
	codeQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"code", deployReceipt.ContractAddress}})
	if codeQuery.Code != 0 || !strings.Contains(string(codeQuery.Value), `"code":"6001"`) {
		t.Fatalf("unexpected code query: %+v", codeQuery)
	}
	callTx := types.Tx("evm:call:evm:0xaaaa:" + deployReceipt.ContractAddress + ":transfer:aabb:100000")
	result = module.DeliverTx(ctx, callTx)
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	var callReceipt Receipt
	if err := json.Unmarshal(result.Data, &callReceipt); err != nil {
		t.Fatal(err)
	}
	receiptQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"receipt", callReceipt.TxHash}})
	if receiptQuery.Code != 0 || !strings.Contains(string(receiptQuery.Value), callReceipt.TxHash) {
		t.Fatalf("unexpected receipt query: %+v", receiptQuery)
	}
	logsQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs", deployReceipt.ContractAddress}})
	if logsQuery.Code != 0 || !strings.Contains(string(logsQuery.Value), `"topics":["0x01"]`) {
		t.Fatalf("unexpected logs query: %+v", logsQuery)
	}
	allLogsQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs"}})
	if allLogsQuery.Code != 0 || !strings.Contains(string(allLogsQuery.Value), deployReceipt.ContractAddress) || !strings.Contains(string(allLogsQuery.Value), callReceipt.TxHash) {
		t.Fatalf("unexpected global logs query: %+v", allLogsQuery)
	}
	missingLogsQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs", "0xmissing"}})
	if missingLogsQuery.Code != 0 || strings.TrimSpace(string(missingLogsQuery.Value)) != "[]" {
		t.Fatalf("unexpected missing logs query: %+v", missingLogsQuery)
	}
	storageQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"storage", deployReceipt.ContractAddress, "0x0"}})
	if storageQuery.Code != 0 || !strings.Contains(string(storageQuery.Value), `"value":"0x73746f7265643a7472616e73666572"`) {
		t.Fatalf("unexpected storage query: %+v", storageQuery)
	}
}

func TestModuleQueryCallExecutesReadOnly(t *testing.T) {
	registry := contract.NewRegistry()
	if err := registry.Register(testVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	request := CallRequest{
		VM:     "evm",
		From:   "0xaaaa",
		To:     "0xbbbb",
		Method: "balanceOf",
		Input:  "0x1234",
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := module.Query(vexoapp.Context{Ctx: context.Background()}, vexoapp.QueryRequest{Path: []string{"call"}, Data: encoded})
	if response.Code != 0 || !strings.Contains(string(response.Value), `"output":"0x6f75743a1234"`) {
		t.Fatalf("unexpected call query: %+v", response)
	}
}

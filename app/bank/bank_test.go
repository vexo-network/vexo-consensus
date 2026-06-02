package bank

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestBankModuleInitializesGenesisAndQueriesBalance(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	if err := module.InitGenesis(vexoapp.Context{Store: storage}, vexoapp.GenesisState{
		"bank:alice": []byte("100"),
		"other:key":  []byte("ignored"),
	}); err != nil {
		t.Fatal(err)
	}

	response := module.Query(vexoapp.Context{Store: storage}, vexoapp.QueryRequest{Path: []string{"balance", "alice"}})
	if response.Code != 0 || string(response.Value) != "100" {
		t.Fatalf("unexpected balance query: %+v", response)
	}
}

func TestBankModuleMintsAndSends(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:mint:alice:100")); result.Code != 0 {
		t.Fatalf("unexpected mint result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:send:alice:bob:35")); result.Code != 0 {
		t.Fatalf("unexpected send result: %+v", result)
	}

	assertBalance(t, storage, "alice", 65)
	assertBalance(t, storage, "bob", 35)
}

func TestBankModuleRejectsInvalidTransactions(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	for _, tx := range []types.Tx{
		[]byte("bank:mint:alice:0"),
		[]byte("bank:mint::10"),
		[]byte("bank:send:alice:bob:1"),
		[]byte("bank:send:alice::1"),
		[]byte("bank:unknown"),
		[]byte("staking:delegate"),
	} {
		result := module.DeliverTx(vexoapp.Context{Store: storage}, tx)
		if result.Code == 0 {
			t.Fatalf("expected tx %q to fail", tx)
		}
	}
}

func TestBankModuleRejectsBalanceOverflow(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	maxUint := "18446744073709551615"
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, types.Tx("bank:mint:alice:"+maxUint)); result.Code != 0 {
		t.Fatalf("unexpected max mint result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:mint:alice:1")); result.Code == 0 || result.Log != ErrBalanceOverflow.Error() {
		t.Fatalf("expected mint overflow, got %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:mint:bob:1")); result.Code != 0 {
		t.Fatalf("unexpected bob mint result: %+v", result)
	}
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:send:bob:alice:1")); result.Code == 0 || result.Log != ErrBalanceOverflow.Error() {
		t.Fatalf("expected send overflow, got %+v", result)
	}
	assertBalance(t, storage, "alice", ^uint64(0))
	assertBalance(t, storage, "bob", 1)
}

func TestBankModuleInvalidTxFloodDoesNotMutateState(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	if result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:mint:alice:10")); result.Code != 0 {
		t.Fatalf("unexpected mint result: %+v", result)
	}
	for i := 0; i < 1000; i++ {
		result := module.DeliverTx(vexoapp.Context{Store: storage}, []byte("bank:send:alice:bob:999999"))
		if result.Code == 0 {
			t.Fatalf("expected invalid flood tx %d to fail", i)
		}
	}
	assertBalance(t, storage, "alice", 10)
	assertBalance(t, storage, "bob", 0)
}

func TestBankModuleRejectsInvalidGenesis(t *testing.T) {
	storage := newBankStore(t)
	err := NewModule().InitGenesis(vexoapp.Context{Store: storage}, vexoapp.GenesisState{"bank:alice": []byte("bad")})
	if !errors.Is(err, ErrInvalidGenesisBalance) {
		t.Fatalf("expected invalid genesis balance, got %v", err)
	}
}

func TestBankModuleQueryRejectsInvalidRequests(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	cases := []vexoapp.QueryRequest{
		{Path: nil},
		{Path: []string{"balance"}},
		{Path: []string{"supply", "alice"}},
	}
	for _, req := range cases {
		response := module.Query(vexoapp.Context{Store: storage}, req)
		if response.Code == 0 {
			t.Fatalf("expected invalid query to fail: %+v", req)
		}
	}
}

func TestBankModuleWorksThroughAppRuntime(t *testing.T) {
	storage := newBankStore(t)
	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{Genesis: vexoapp.GenesisState{"bank:alice": []byte("100")}}); err != nil {
		t.Fatal(err)
	}
	response, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 1},
			Txs: fairordering.SortTxsWithSalt(
				[]types.Tx{
					[]byte("bank:send:alice:bob:40"),
					[]byte("bank:mint:carol:7"),
				},
				fairordering.HeightSalt("vexo-test", 1),
			),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 2 || response.AppHash == (types.Hash{}) {
		t.Fatalf("unexpected finalize response: %+v", response)
	}

	query := runtime.Query(vexoapp.QueryRequest{Path: []string{"bank", "balance", "bob"}})
	if query.Code != 0 || string(query.Value) != "40" {
		t.Fatalf("unexpected runtime query: %+v", query)
	}
}

func TestBankModuleCLICommands(t *testing.T) {
	commands := NewModule().CLICommands()
	if len(commands) != 1 || commands[0].Name != ModuleName || len(commands[0].Children) != 2 {
		t.Fatalf("unexpected cli commands: %+v", commands)
	}

	cases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "mint transaction",
			args:     []string{"tx", "mint", "alice", "100"},
			expected: "tx: bank:mint:alice:100",
		},
		{
			name:     "send transaction",
			args:     []string{"tx", "send", "alice", "bob", "25"},
			expected: "tx: bank:send:alice:bob:25",
		},
		{
			name:     "balance query",
			args:     []string{"query", "balance", "alice"},
			expected: "query_path: bank/balance/alice",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := commands[0].Execute(&output, tc.args); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(output.String()) != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, output.String())
			}
		})
	}
}

func TestBankModuleCLIHelp(t *testing.T) {
	command := NewModule().CLICommands()[0]
	for _, args := range [][]string{
		nil,
		{"--help"},
		{"tx", "--help"},
		{"tx", "mint", "--help"},
		{"query", "--help"},
		{"query", "balance", "--help"},
	} {
		var output bytes.Buffer
		if err := command.Execute(&output, args); err != nil {
			t.Fatalf("expected help args %v to succeed: %v", args, err)
		}
		if !strings.Contains(output.String(), "Usage:") || !strings.Contains(output.String(), "bank") {
			t.Fatalf("unexpected help output for %v: %s", args, output.String())
		}
	}
}

func TestBankModuleCLIRejectsInvalidCommands(t *testing.T) {
	command := NewModule().CLICommands()[0]
	for _, args := range [][]string{
		{"unknown"},
		{"tx", "mint", "alice", "0"},
		{"tx", "send", "alice", "bob", "bad"},
		{"query", "balance", "alice", "extra"},
	} {
		if err := command.Execute(&bytes.Buffer{}, args); err == nil {
			t.Fatalf("expected cli args %v to fail", args)
		}
	}
}

func newBankStore(t *testing.T) *store.LevelDBStore {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return storage
}

func assertBalance(t *testing.T, storage vexoapp.StateStore, address types.Address, expected uint64) {
	t.Helper()
	balance, err := Balance(context.Background(), storage, address)
	if err != nil {
		t.Fatal(err)
	}
	if balance != expected {
		t.Fatalf("expected balance %d for %s, got %d", expected, address, balance)
	}
}

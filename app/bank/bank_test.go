package bank

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
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
			Txs: []types.Tx{
				[]byte("bank:send:alice:bob:40"),
				[]byte("bank:mint:carol:7"),
			},
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

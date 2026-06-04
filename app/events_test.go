package app

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type eventModule struct{}

func (eventModule) Name() string { return "eventmod" }

func (eventModule) InitGenesis(ctx Context, genesis GenesisState) error { return nil }

func (eventModule) BeginBlock(ctx Context, header types.Header) error { return nil }

func (eventModule) DeliverTx(ctx Context, tx types.Tx) types.Result { return types.Result{} }

func (eventModule) EndBlock(ctx Context) error { return nil }

func (eventModule) Events(ctx Context, tx types.Tx, result types.Result) []events.Event {
	return []events.Event{{Type: "executed", Attributes: []events.Attribute{{Key: "module", Value: "eventmod", Index: true}}}}
}

func TestRuntimeIndexesModuleEvents(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	runtime, err := NewRuntime("vexo-test", []Module{eventModule{}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	if _, err := runtime.InitChain(InitChainRequest{ChainID: "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}, Txs: []types.Tx{[]byte("eventmod:run")}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.TxEvents) != 1 || len(response.TxEvents[0]) != 1 {
		t.Fatalf("expected tx events, got %+v", response.TxEvents)
	}
	records, err := events.NewIndexer(storage).Query(context.Background(), "module", "eventmod")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Height != 1 || records[0].Event.Type != "executed" {
		t.Fatalf("unexpected indexed records: %+v", records)
	}
}

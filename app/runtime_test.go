package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestRuntimeExecutesBlockThroughModules(t *testing.T) {
	bank := &recordingModule{name: "bank"}
	staking := &recordingModule{name: "staking"}
	runtime, err := NewRuntime("vexo-test", []Module{bank, staking}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.InitChain(InitChainRequest{Genesis: GenesisState{"bank": []byte("genesis")}}); err != nil {
		t.Fatal(err)
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: []types.Tx{
			[]byte("bank:send"),
			[]byte("staking:delegate"),
		},
	}
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: block})
	if err != nil {
		t.Fatal(err)
	}

	if len(response.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(response.Results))
	}
	if !reflect.DeepEqual(bank.delivered, []string{"bank:send"}) {
		t.Fatalf("unexpected bank txs: %v", bank.delivered)
	}
	if !reflect.DeepEqual(staking.delivered, []string{"staking:delegate"}) {
		t.Fatalf("unexpected staking txs: %v", staking.delivered)
	}

	commit, err := runtime.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if commit.Height != 1 {
		t.Fatalf("expected committed height 1, got %d", commit.Height)
	}
	if commit.AppHash == (types.Hash{}) {
		t.Fatal("expected non-zero app hash")
	}
}

func TestRuntimePrepareProposalFiltersInvalidTxs(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.PrepareProposal(PrepareProposalRequest{
		Height: 1,
		Txs: []types.Tx{
			[]byte("bank:send"),
			[]byte("malformed"),
			[]byte("unknown:tx"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.Txs, []types.Tx{[]byte("bank:send")}) {
		t.Fatalf("unexpected prepared txs: %q", response.Txs)
	}
}

func TestRuntimePrepareProposalSortsAcceptedTxs(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.PrepareProposal(PrepareProposalRequest{
		Height: 1,
		Txs: []types.Tx{
			[]byte("bank:charlie"),
			[]byte("bank:alpha"),
			[]byte("bank:bravo"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fairordering.IsOrdered(response.Txs) {
		t.Fatalf("expected ordered txs, got %q", response.Txs)
	}
}

func TestRuntimeProcessProposalRejectsInvalidTx(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	response := runtime.ProcessProposal(ProcessProposalRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    []types.Tx{[]byte("unknown:tx")},
		},
	})
	if response.Accepted {
		t.Fatal("expected proposal rejection")
	}
}

func TestRuntimeFinalizeRejectsBadProposal(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.FinalizeBlock(FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    []types.Tx{[]byte("bad")},
		},
	})
	if !errors.Is(err, ErrProposalRejected) {
		t.Fatalf("expected proposal rejected, got %v", err)
	}
}

func TestRuntimePropagatesModuleErrors(t *testing.T) {
	beginErr := errors.New("begin failed")
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank", beginErr: beginErr}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.FinalizeBlock(FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    []types.Tx{[]byte("bank:send")},
		},
	})
	if !errors.Is(err, beginErr) {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestRuntimeRejectsFailedDeliverTx(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank", deliverCode: 7, deliverLog: "deliver failed"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.FinalizeBlock(FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    []types.Tx{[]byte("bank:send")},
		},
	})
	if err == nil || err.Error() != "deliver failed" {
		t.Fatalf("expected deliver failed, got %v", err)
	}
}

func TestRuntimePassesStateStoreToModules(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewRuntime("vexo-test", []Module{&statefulModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)

	_, err = runtime.FinalizeBlock(FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    []types.Tx{[]byte("bank:alice=100")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	value, err := storage.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "100" {
		t.Fatalf("expected persisted value 100, got %s", value)
	}
}

func TestRuntimeAppHashReflectsStateRoot(t *testing.T) {
	firstStore, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer firstStore.Close()
	secondStore, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer secondStore.Close()

	firstRuntime, err := NewRuntime("vexo-test", []Module{&statefulModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	firstRuntime.WithStore(firstStore)
	secondRuntime, err := NewRuntime("vexo-test", []Module{&statefulModule{name: "bank", value: "200"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	secondRuntime.WithStore(secondStore)

	firstResponse, err := firstRuntime.FinalizeBlock(FinalizeBlockRequest{
		Block: types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("bank:alice=100")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondResponse, err := secondRuntime.FinalizeBlock(FinalizeBlockRequest{
		Block: types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("bank:alice=100")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstResponse.AppHash == secondResponse.AppHash {
		t.Fatal("expected app hash to differ when stored state differs")
	}
}

func TestRuntimeRoutesQueriesToModules(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&queryModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	response := runtime.Query(QueryRequest{Path: []string{"bank", "balance", "alice"}, Data: []byte("payload")})
	if response.Code != 0 || string(response.Value) != "balance/alice:payload" {
		t.Fatalf("unexpected query response: %+v", response)
	}
}

func TestRuntimeReportsQueryErrors(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []QueryRequest{
		{},
		{Path: []string{"bank"}},
		{Path: []string{"staking"}},
	} {
		response := runtime.Query(req)
		if response.Code == 0 {
			t.Fatalf("expected query error for %+v", req)
		}
	}
}

func TestRuntimeRequiresChainID(t *testing.T) {
	_, err := NewRuntime("", nil, nil)
	if !errors.Is(err, ErrEmptyChainID) {
		t.Fatalf("expected empty chain id, got %v", err)
	}
}

func TestFirstModuleRouter(t *testing.T) {
	module := &recordingModule{name: "bank"}
	routed, err := FirstModuleRouter{}.RouteTx(Context{}, []byte("anything"), []Module{module})
	if err != nil {
		t.Fatal(err)
	}
	if routed.Name() != "bank" {
		t.Fatalf("expected bank, got %s", routed.Name())
	}

	_, err = FirstModuleRouter{}.RouteTx(Context{}, []byte("anything"), nil)
	if !errors.Is(err, ErrNoModules) {
		t.Fatalf("expected no modules, got %v", err)
	}
}

func TestPrefixRouterErrors(t *testing.T) {
	router := PrefixRouter{}

	_, err := router.RouteTx(Context{}, []byte("malformed"), []Module{&recordingModule{name: "bank"}})
	if !errors.Is(err, ErrMalformedRoutedTx) {
		t.Fatalf("expected malformed routed tx, got %v", err)
	}

	_, err = router.RouteTx(Context{}, []byte("staking:delegate"), []Module{&recordingModule{name: "bank"}})
	if !errors.Is(err, ErrNoRouteForTx) {
		t.Fatalf("expected no route, got %v", err)
	}
}

type recordingModule struct {
	name        string
	delivered   []string
	beginErr    error
	endErr      error
	deliverCode uint32
	deliverLog  string
}

func (module *recordingModule) Name() string {
	return module.name
}

func (module *recordingModule) InitGenesis(ctx Context, genesis GenesisState) error {
	return nil
}

func (module *recordingModule) BeginBlock(ctx Context, header types.Header) error {
	return module.beginErr
}

func (module *recordingModule) DeliverTx(ctx Context, tx types.Tx) types.Result {
	if module.deliverCode != 0 {
		return types.Result{Code: module.deliverCode, Log: module.deliverLog}
	}
	module.delivered = append(module.delivered, string(tx))
	return types.Result{}
}

func (module *recordingModule) EndBlock(ctx Context) error {
	return module.endErr
}

type statefulModule struct {
	name  string
	value string
}

func (module *statefulModule) Name() string {
	return module.name
}

func (module *statefulModule) InitGenesis(ctx Context, genesis GenesisState) error {
	return nil
}

func (module *statefulModule) BeginBlock(ctx Context, header types.Header) error {
	return nil
}

func (module *statefulModule) DeliverTx(ctx Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: "missing store"}
	}
	payload := string(tx)
	if payload != "bank:alice=100" {
		return types.Result{Code: 2, Log: "unexpected tx"}
	}
	value := module.value
	if value == "" {
		value = "100"
	}
	if err := ctx.Store.Set(context.Background(), "bank", []byte("alice"), []byte(value)); err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	return types.Result{}
}

func (module *statefulModule) EndBlock(ctx Context) error {
	return nil
}

type queryModule struct {
	name string
}

func (module *queryModule) Name() string {
	return module.name
}

func (module *queryModule) InitGenesis(ctx Context, genesis GenesisState) error {
	return nil
}

func (module *queryModule) BeginBlock(ctx Context, header types.Header) error {
	return nil
}

func (module *queryModule) DeliverTx(ctx Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module *queryModule) EndBlock(ctx Context) error {
	return nil
}

func (module *queryModule) Query(ctx Context, req QueryRequest) QueryResponse {
	return QueryResponse{Value: []byte(strings.Join(req.Path, "/") + ":" + string(req.Data))}
}

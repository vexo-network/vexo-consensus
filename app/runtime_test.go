package app

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
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
		Txs: fairordering.SortTxsWithSalt(
			[]types.Tx{
				[]byte("bank:send"),
				[]byte("staking:delegate"),
			},
			fairordering.HeightSalt("vexo-test", 1),
		),
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
	if !fairordering.IsOrderedWithSalt(response.Txs, fairordering.HeightSalt("vexo-test", 1)) {
		t.Fatalf("expected ordered txs, got %q", response.Txs)
	}
}

func TestRuntimeProcessProposalRejectsMismatchedOrdering(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	ordered := fairordering.SortTxsWithSalt(
		[]types.Tx{[]byte("bank:charlie"), []byte("bank:alpha"), []byte("bank:bravo")},
		fairordering.HeightSalt("vexo-test", 1),
	)
	reordered := []types.Tx{ordered[1], ordered[0], ordered[2]}

	response := runtime.ProcessProposal(ProcessProposalRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    reordered,
		},
	})
	if response.Accepted {
		t.Fatal("expected proposal rejection for ordering mismatch")
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

func TestRuntimeRunsModuleTxValidatorInAdmissionPaths(t *testing.T) {
	validateErr := errors.New("module validation failed")
	module := &recordingModule{name: "bank", validateErr: validateErr}
	runtime, err := NewRuntime("vexo-test", []Module{module}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}

	if response := runtime.CheckTx([]byte("bank:send")); response.Result.Code == 0 || response.Result.Log != validateErr.Error() {
		t.Fatalf("expected CheckTx validation failure, got %+v", response)
	}
	proposal := runtime.ProcessProposal(ProcessProposalRequest{
		Block: types.Block{
			Header: types.Header{Height: 1},
			Txs:    []types.Tx{[]byte("bank:send")},
		},
	})
	if proposal.Accepted || proposal.Reason != validateErr.Error() {
		t.Fatalf("expected ProcessProposal validation failure, got %+v", proposal)
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

func TestRuntimeAnteRejectsReplayCommitsNonceAndCollectsFee(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(context.Background(), "bank", []byte("alice"), encodeTestBalance(100)); err != nil {
		t.Fatal(err)
	}

	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage).WithAnte(NewAnteKeeper(AnteConfig{MinFee: 1, RequireNonce: true, FeeCollector: "treasury"}))

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: fairordering.SortTxsWithSalt(
			[]types.Tx{[]byte("bank:send:fee=7:gas=55:signer=alice:nonce=1")},
			fairordering.HeightSalt("vexo-test", 1),
		),
	}
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: block})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || string(response.Results[0].Data) != "gas_used=55 fee_paid=7" {
		t.Fatalf("unexpected execution result metadata: %+v", response.Results)
	}
	alice, err := storage.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	treasury, err := storage.Get(context.Background(), "bank", []byte("treasury"))
	if err != nil {
		t.Fatal(err)
	}
	if decodeTestBalance(alice) != 93 || decodeTestBalance(treasury) != 7 {
		t.Fatalf("unexpected fee balances: alice=%d treasury=%d", decodeTestBalance(alice), decodeTestBalance(treasury))
	}
	if runtime.CheckTx([]byte("bank:send:fee=1:signer=alice:nonce=1")).Result.Code == 0 {
		t.Fatal("expected committed nonce replay to be rejected")
	}
	if runtime.CheckTx([]byte("bank:send:fee=1:signer=alice:nonce=2")).Result.Code != 0 {
		t.Fatal("expected next nonce to pass")
	}
}

func TestRuntimeDeliversSignedTxPayload(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	module := &recordingModule{name: "bank"}
	runtime, err := NewRuntime("vexo-test", []Module{module}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithAnte(NewAnteKeeper(AnteConfig{RequireSigned: true}))
	payload := types.Tx("bank:send")
	signedTx, err := SignTx("vexo-test", payload, signer)
	if err != nil {
		t.Fatal(err)
	}
	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: fairordering.SortTxsWithSalt(
			[]types.Tx{signedTx},
			fairordering.HeightSalt("vexo-test", 1),
		),
	}
	if _, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: block}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(module.delivered, []string{"bank:send"}) {
		t.Fatalf("expected payload delivery, got %v", module.delivered)
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

func TestRuntimeStagedAppHashUsesBlockHeight(t *testing.T) {
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

	response, _, err := runtime.FinalizeBlockStaged(FinalizeBlockRequest{
		Block: types.Block{Header: types.Header{Height: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.AppHash == runtime.computeAppHashAtHeight(context.Background(), 0, storage) {
		t.Fatal("expected staged app hash to include block height, not previous runtime height")
	}
	if response.AppHash != runtime.computeAppHashAtHeight(context.Background(), 5, storage) {
		t.Fatal("expected staged app hash to match the finalized block height")
	}
}

func TestRuntimeContextMethodsHonorCancellation(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	block := types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}, Txs: []types.Tx{[]byte("bank:send")}}

	if response := runtime.CheckTxContext(ctx, []byte("bank:send")); response.Result.Code == 0 || !strings.Contains(response.Result.Log, context.Canceled.Error()) {
		t.Fatalf("expected canceled CheckTxContext, got %+v", response)
	}
	if _, err := runtime.PrepareProposalContext(ctx, PrepareProposalRequest{Height: 1, Txs: []types.Tx{[]byte("bank:send")}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled PrepareProposalContext, got %v", err)
	}
	if response := runtime.ProcessProposalContext(ctx, ProcessProposalRequest{Block: block}); response.Accepted || !strings.Contains(response.Reason, context.Canceled.Error()) {
		t.Fatalf("expected canceled ProcessProposalContext, got %+v", response)
	}
	if _, err := runtime.FinalizeBlockContext(ctx, FinalizeBlockRequest{Block: block}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled FinalizeBlockContext, got %v", err)
	}
	if response := runtime.QueryContext(ctx, QueryRequest{Path: []string{"bank"}}); response.Code == 0 || !strings.Contains(response.Log, context.Canceled.Error()) {
		t.Fatalf("expected canceled QueryContext, got %+v", response)
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
	validateErr error
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

func (module *recordingModule) ValidateTx(ctx Context, tx types.Tx) error {
	return module.validateErr
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
	if err := ctx.Store.Set(ctx.GoContext(), "bank", []byte("alice"), []byte(value)); err != nil {
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

func encodeTestBalance(balance uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], balance)
	return encoded[:]
}

func decodeTestBalance(value []byte) uint64 {
	return binary.BigEndian.Uint64(value)
}

package app

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// Regression tests for the application runtime: a transaction never fails the
// block, proposals are validated cumulatively, and the fee market is state.

func newFeeRuntime(t *testing.T, storage StateStore, module Module, config AnteConfig) *Runtime {
	t.Helper()
	runtime, err := NewRuntime("vexo-test", []Module{module}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	if config.FeeCollector == "" {
		config.FeeCollector = "treasury"
	}
	config.RequireNonce = true
	return runtime.WithStore(storage).WithAnte(NewAnteKeeper(config))
}

func fundAccount(t *testing.T, storage StateStore, account types.Address, balance uint64) {
	t.Helper()
	if err := setBankBalance(context.Background(), storage, account, balance); err != nil {
		t.Fatal(err)
	}
}

func accountBalance(t *testing.T, storage StateStore, account types.Address) uint64 {
	t.Helper()
	balance, err := bankBalance(context.Background(), storage, account)
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

func blockAt(height types.Height, txs ...types.Tx) types.Block {
	return types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: height},
		Txs:    fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt("vexo-test", height)),
	}
}

// One failing transaction used to abort FinalizeBlock, which let any account
// halt the chain for the price of one bad transaction. The failure is the
// transaction's own: it pays its fee, uses its nonce and changes nothing else.
func TestFinalizeBlockChargesAFailedTransactionAndKeepsGoing(t *testing.T) {
	storage := openAnteStore(t)
	fundAccount(t, storage, "alice", 100)
	module := &scriptedModule{name: "bank", writeBeforeFailure: true}
	runtime := newFeeRuntime(t, storage, module, AnteConfig{MinFee: 1})

	failing := types.Tx("bank:fail:fee=7:gas=20:signer=alice:nonce=1")
	succeeding := types.Tx("bank:ok:fee=5:gas=20:signer=alice:nonce=2")
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: blockAt(1, failing, succeeding)})
	if err != nil {
		t.Fatalf("a failing transaction must not abort the block: %v", err)
	}
	if len(response.Results) != 2 || response.Results[0].Code != 7 || response.Results[1].Code != 0 {
		t.Fatalf("expected the module failure and then a success, got %+v", response.Results)
	}
	if response.Results[0].FeePaid != 7 {
		t.Fatalf("expected the failed transaction to report its fee, got %+v", response.Results[0])
	}
	if got := accountBalance(t, storage, "alice"); got != 88 {
		t.Fatalf("expected both fees to be charged, alice has %d", got)
	}
	if got := accountBalance(t, storage, "treasury"); got != 12 {
		t.Fatalf("expected the collector to receive both fees, treasury has %d", got)
	}
	if sequence, err := NewAccountKeeper().Sequence(context.Background(), storage, "alice"); err != nil || sequence != 2 {
		t.Fatalf("expected the failed transaction to use its nonce, sequence=%d err=%v", sequence, err)
	}
	if _, err := storage.Get(context.Background(), "bank", []byte("fail")); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected the failed transaction's write to be discarded, got %v", err)
	}
	if _, err := storage.Get(context.Background(), "bank", []byte("ok")); err != nil {
		t.Fatalf("expected the successful transaction's write to be kept: %v", err)
	}
}

// A transaction whose payer cannot pay is a failed result that changes nothing;
// it must not abort the block either.
func TestFinalizeBlockSkipsATransactionTheFeePayerCannotAfford(t *testing.T) {
	storage := openAnteStore(t)
	fundAccount(t, storage, "alice", 10)
	module := &scriptedModule{name: "bank"}
	runtime := newFeeRuntime(t, storage, module, AnteConfig{MinFee: 1})

	first := types.Tx("bank:ok:fee=8:gas=20:signer=alice:nonce=1")
	second := types.Tx("bank:ok:fee=8:gas=20:signer=alice:nonce=2")
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: blockAt(1, first, second)})
	if err != nil {
		t.Fatalf("an unaffordable transaction must not abort the block: %v", err)
	}
	if len(response.Results) != 2 || response.Results[0].Code != 0 || response.Results[1].Code == 0 {
		t.Fatalf("expected the second transaction to fail, got %+v", response.Results)
	}
	if got := accountBalance(t, storage, "alice"); got != 2 {
		t.Fatalf("expected only the first fee to be charged, alice has %d", got)
	}
	if sequence, _ := NewAccountKeeper().Sequence(context.Background(), storage, "alice"); sequence != 1 {
		t.Fatalf("an unaffordable transaction must not use a nonce, sequence=%d", sequence)
	}
	if !reflect.DeepEqual(module.order, []string{string(first)}) {
		t.Fatalf("the unaffordable transaction must not execute, got %v", module.order)
	}
}

// Nothing in a block's content may fail FinalizeBlock: bad envelopes, bad
// nonces, bad signers and unroutable transactions are all failed results.
func TestFinalizeBlockNeverFailsBecauseOfTransactionContent(t *testing.T) {
	storage := openAnteStore(t)
	fundAccount(t, storage, "alice", 100)
	module := &scriptedModule{name: "bank"}
	runtime := newFeeRuntime(t, storage, module, AnteConfig{MinFee: 1})

	txs := []types.Tx{
		[]byte("signed:@@@"),
		[]byte("bank:ok:fee=1:gas=20:signer=alice:signer=bob:nonce=1"),
		[]byte("bank:ok:fee=1:gas=20:signer=alice:nonce=99"),
		[]byte("bank:ok:fee=0:gas=20:signer=alice:nonce=1"),
		[]byte("nobody:ok:fee=1:gas=20:signer=alice:nonce=1"),
		[]byte("bank:ok:fee=1:gas=20:signer=alice:nonce=1:eth_raw=00:eth_hash=00"),
		[]byte(""),
		[]byte("bank:ok:fee=1:gas=20:signer=alice:nonce=1"),
	}
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: types.Block{Header: types.Header{Height: 1}, Txs: txs}})
	if err != nil {
		t.Fatalf("transaction content must never fail the block: %v", err)
	}
	if len(response.Results) != len(txs) || len(response.TxEvents) != len(txs) {
		t.Fatalf("expected one result and one event list per transaction, got %d and %d", len(response.Results), len(response.TxEvents))
	}
	succeeded := 0
	for _, result := range response.Results {
		if result.Code == 0 {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected exactly the one valid transaction to succeed, got %+v", response.Results)
	}
	if got := accountBalance(t, storage, "alice"); got != 99 {
		t.Fatalf("only the valid transaction may charge a fee, alice has %d", got)
	}
}

// The proposal paths agree with the block: the fees and nonces of a sender are
// validated together, not one at a time.
func TestProposalPathsValidateAccountsCumulatively(t *testing.T) {
	storage := openAnteStore(t)
	fundAccount(t, storage, "alice", 10)
	module := &scriptedModule{name: "bank"}
	runtime := newFeeRuntime(t, storage, module, AnteConfig{MinFee: 1})

	first := types.Tx("bank:ok:fee=8:gas=20:signer=alice:nonce=1")
	second := types.Tx("bank:ok:fee=8:gas=20:signer=alice:nonce=2")

	if response := runtime.CheckTx(second); response.Result.Code == 0 {
		t.Fatal("a nonce gap must fail CheckTx")
	}
	if response := runtime.ProcessProposal(ProcessProposalRequest{Block: blockAt(1, first, second)}); response.Accepted {
		t.Fatal("a block whose fees the payer cannot afford together must be rejected")
	}
	if response := runtime.CheckTxsContext(context.Background(), blockAt(1, first, second).Txs); response.Result.Code == 0 {
		t.Fatal("a batch whose fees the payer cannot afford together must be rejected")
	}
	prepared, err := runtime.PrepareProposal(PrepareProposalRequest{Height: 1, Txs: []types.Tx{second, first}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prepared.Txs, []types.Tx{first}) {
		t.Fatalf("expected a proposal of the affordable prefix only, got %q", prepared.Txs)
	}
	if response := runtime.ProcessProposal(ProcessProposalRequest{Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}, Txs: prepared.Txs}}); !response.Accepted {
		t.Fatalf("the proposal the runtime prepared must pass its own validation: %s", response.Reason)
	}

	gap := types.Tx("bank:ok:fee=1:gas=20:signer=alice:nonce=3")
	if response := runtime.ProcessProposal(ProcessProposalRequest{Block: blockAt(1, first, gap)}); response.Accepted {
		t.Fatal("a block with a nonce gap must be rejected")
	}
}

// The blob gas of a block's Ethereum transactions is bounded; a proposal that
// exceeds the limit is invalid and the proposer leaves the excess out.
func TestProposalPathsEnforceTheBlockBlobGasLimit(t *testing.T) {
	storage := openAnteStore(t)
	runtime, err := NewRuntime("vexo-test", []Module{&recordingModule{name: "evm"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage).WithAnte(NewAnteKeeper(AnteConfig{
		RequireNonce:          true,
		EthereumAuthenticator: &ethereumAuthenticatorStub{},
		FeeMarket:             FeeMarketConfig{MaxBlobGas: 100},
	}))
	first := types.Tx("evm:call:fee=1:gas=21000:signer=alice:nonce=0:eth_raw=00:eth_hash=01:eth_blob_gas=60")
	second := types.Tx("evm:call:fee=1:gas=21000:signer=bob:nonce=0:eth_raw=00:eth_hash=02:eth_blob_gas=60")

	if response := runtime.ProcessProposal(ProcessProposalRequest{Block: blockAt(1, first)}); !response.Accepted {
		t.Fatalf("a block within the blob gas limit must pass: %s", response.Reason)
	}
	response := runtime.ProcessProposal(ProcessProposalRequest{Block: blockAt(1, first, second)})
	if response.Accepted || response.Reason != ErrBlobGasLimitExceeded.Error() {
		t.Fatalf("a block above the blob gas limit must be rejected, got %+v", response)
	}
	prepared, err := runtime.PrepareProposal(PrepareProposalRequest{Height: 1, Txs: []types.Tx{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Txs) != 1 {
		t.Fatalf("expected the proposal to stop at the blob gas limit, got %d transactions", len(prepared.Txs))
	}
}

// The base fee is application state: it is stored by the block that moved it,
// a restarted runtime prices against it, and replaying the same blocks yields
// the same fees and the same app hash.
func TestFeeMarketIsApplicationState(t *testing.T) {
	config := AnteConfig{
		BaseFee: 100,
		FeeMarket: FeeMarketConfig{
			DynamicBaseFee:           true,
			TargetGas:                1000,
			BaseFeeChangeDenominator: 8,
			MinBaseFee:               1,
			MaxBaseFee:               1_000_000,
		},
	}
	run := func(t *testing.T) (*Runtime, *store.LevelDBStore, types.Hash) {
		storage := openAnteStore(t)
		runtime := newFeeRuntime(t, storage, &scriptedModule{name: "bank"}, config)
		var appHash types.Hash
		for height := types.Height(1); height <= 3; height++ {
			response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: blockAt(height)})
			if err != nil {
				t.Fatal(err)
			}
			appHash = response.AppHash
			if height == 1 && (response.FeeMarket.BaseFee != 100 || response.FeeMarket.NextBaseFee != 88) {
				t.Fatalf("expected the first block to move the base fee from 100 to 88, got %+v", response.FeeMarket)
			}
		}
		return runtime, storage, appHash
	}
	_, firstStore, firstHash := run(t)
	_, secondStore, secondHash := run(t)
	if firstHash != secondHash {
		t.Fatal("replaying the same blocks must give the same app hash")
	}
	first, err := firstStore.Get(context.Background(), feeMarketNamespace, feeMarketStateKey)
	if err != nil {
		t.Fatal(err)
	}
	second, err := secondStore.Get(context.Background(), feeMarketNamespace, feeMarketStateKey)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("replayed fee market state differs: %x vs %x (%v)", first, second, err)
	}

	// Empty blocks use no gas, so the base fee fell below its initial value.
	restarted := newFeeRuntime(t, firstStore, &scriptedModule{name: "bank"}, config)
	ctx, err := restarted.newContext(context.Background(), 4, types.Header{}, firstStore)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BaseFee == 0 || ctx.BaseFee >= 100 {
		t.Fatalf("expected the stored base fee to have fallen, got %d", ctx.BaseFee)
	}
	fundAccount(t, firstStore, "alice", 1_000_000)
	price := ctx.BaseFee * 20
	affordable := types.Tx("bank:ok:fee=" + strconv.FormatUint(price, 10) + ":gas=20:signer=alice:nonce=1")
	underpriced := types.Tx("bank:ok:fee=" + strconv.FormatUint(price-1, 10) + ":gas=20:signer=alice:nonce=1")
	if response := restarted.CheckTx(affordable); response.Result.Code != 0 {
		t.Fatalf("expected a transaction paying the stored base fee to pass: %+v", response.Result)
	}
	if response := restarted.CheckTx(underpriced); response.Result.Code == 0 {
		t.Fatal("expected a transaction below the stored base fee to fail")
	}
	// A runtime that was built with a different initial base fee still prices
	// against the stored one.
	other := config
	other.BaseFee = 5000
	reconfigured := newFeeRuntime(t, firstStore, &scriptedModule{name: "bank"}, other)
	if response := reconfigured.CheckTx(affordable); response.Result.Code != 0 {
		t.Fatalf("the configured base fee is only the initial value: %+v", response.Result)
	}
}

// rootFailingStore reports a failure instead of a state root.
type rootFailingStore struct {
	StateStore
	err error
}

func (store rootFailingStore) Root(context.Context, string) (types.Hash, error) {
	return types.Hash{}, store.err
}

// A namespace whose root cannot be computed used to be left out of the app
// hash silently, so two nodes with different state could agree on the hash.
func TestAppHashFailsWhenARootCannotBeComputed(t *testing.T) {
	rootErr := errors.New("root unavailable")
	storage := rootFailingStore{StateStore: openAnteStore(t), err: rootErr}
	runtime, err := NewRuntime("vexo-test", []Module{&scriptedModule{name: "bank"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	if _, err := runtime.InitChain(InitChainRequest{}); !errors.Is(err, rootErr) {
		t.Fatalf("expected InitChain to report the root error, got %v", err)
	}
	if _, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: blockAt(1)}); !errors.Is(err, rootErr) {
		t.Fatalf("expected FinalizeBlock to report the root error, got %v", err)
	}
}

// Without an ante handler a transaction with repeated tags is still ambiguous
// and still refused everywhere.
func TestRuntimeRejectsAmbiguousTagsWithoutAnteHandler(t *testing.T) {
	module := &recordingModule{name: "bank"}
	runtime, err := NewRuntime("vexo-test", []Module{module}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	ambiguous := types.Tx("bank:send:fee=1:fee=1000:signer=alice")
	if response := runtime.CheckTx(ambiguous); response.Result.Code == 0 {
		t.Fatal("expected CheckTx to refuse repeated tags")
	}
	if response := runtime.ProcessProposal(ProcessProposalRequest{Block: blockAt(1, ambiguous)}); response.Accepted {
		t.Fatal("expected ProcessProposal to refuse repeated tags")
	}
	response, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: blockAt(1, ambiguous)})
	if err != nil {
		t.Fatal(err)
	}
	if response.Results[0].Code == 0 || len(module.delivered) != 0 {
		t.Fatalf("expected the ambiguous transaction not to execute, results=%+v delivered=%v", response.Results, module.delivered)
	}
}

type genesisRecorder struct {
	name       string
	seen       []types.ValidatorUpdate
	mutateSeen bool
}

func (module *genesisRecorder) Name() string { return module.name }

func (module *genesisRecorder) InitGenesis(ctx Context, genesis GenesisState) error {
	module.seen = ctx.GenesisValidators
	if module.mutateSeen {
		for index := range ctx.GenesisValidators {
			ctx.GenesisValidators[index].PublicKey[0] ^= 0xff
			ctx.GenesisValidators[index].Metadata["role"] = "tampered"
		}
	}
	return nil
}

func (module *genesisRecorder) BeginBlock(ctx Context, header types.Header) error { return nil }

func (module *genesisRecorder) DeliverTx(ctx Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module *genesisRecorder) EndBlock(ctx Context) error { return nil }

// Modules read the genesis validators from the context; each one gets a copy,
// so one module cannot change what the next one reads and the request stays
// intact.
func TestInitChainGivesEveryModuleItsOwnGenesisValidators(t *testing.T) {
	validators := []types.ValidatorUpdate{{
		ID:          "validator-1",
		PublicKey:   []byte{1, 2, 3},
		VotingPower: 10,
		Metadata:    map[string]string{"role": "genesis"},
	}}
	tamperer := &genesisRecorder{name: "first", mutateSeen: true}
	reader := &genesisRecorder{name: "second"}
	runtime, err := NewRuntime("vexo-test", []Module{tamperer, reader}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.InitChain(InitChainRequest{Validators: validators}); err != nil {
		t.Fatal(err)
	}
	if len(reader.seen) != 1 || reader.seen[0].PublicKey[0] != 1 || reader.seen[0].Metadata["role"] != "genesis" {
		t.Fatalf("a module saw another module's changes: %+v", reader.seen)
	}
	if validators[0].PublicKey[0] != 1 || validators[0].Metadata["role"] != "genesis" {
		t.Fatalf("the request was modified: %+v", validators)
	}

	probe := &genesisRecorder{name: "probe"}
	runtime, err = NewRuntime("vexo-test", []Module{probe}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.FinalizeBlock(FinalizeBlockRequest{Block: blockAt(1)}); err != nil {
		t.Fatal(err)
	}
	ctx, err := runtime.newContext(context.Background(), 1, types.Header{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.GenesisValidators != nil {
		t.Fatal("genesis validators exist only during InitChain")
	}
}

func TestStagedStoreRejectsInvalidNamespaceAndKey(t *testing.T) {
	staged := NewStagedStore(openAnteStore(t))
	ctx := context.Background()
	if err := staged.Set(ctx, "", []byte("key"), []byte("v")); !errors.Is(err, store.ErrInvalidNamespace) {
		t.Fatalf("expected an invalid namespace, got %v", err)
	}
	if err := staged.Set(ctx, "bank", nil, []byte("v")); !errors.Is(err, store.ErrInvalidKey) {
		t.Fatalf("expected an invalid key, got %v", err)
	}
	if err := staged.Delete(ctx, "bank", nil); !errors.Is(err, store.ErrInvalidKey) {
		t.Fatalf("expected an invalid key, got %v", err)
	}
	err := staged.SetBatch(ctx, []kvbatch.KVWrite{
		{Namespace: "bank", Key: []byte("a"), Value: []byte("1")},
		{Namespace: "bank", Key: nil, Value: []byte("2")},
	})
	if !errors.Is(err, store.ErrInvalidKey) {
		t.Fatalf("expected an invalid key, got %v", err)
	}
	if len(staged.Writes()) != 0 {
		t.Fatalf("an invalid batch must stage nothing, got %+v", staged.Writes())
	}
	if staged.Fault() != nil {
		t.Fatalf("a rejected write is not a store fault: %v", staged.Fault())
	}
}

func TestStagedStoreLatchesInfrastructureFaults(t *testing.T) {
	staged := NewStagedStore(openAnteStore(t))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := staged.Set(canceled, "bank", []byte("a"), []byte("1")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected a canceled context, got %v", err)
	}
	if !errors.Is(staged.Fault(), context.Canceled) {
		t.Fatalf("expected the fault to be latched, got %v", staged.Fault())
	}
	// A missing key is an outcome of the data, not a fault of the node.
	clean := NewStagedStore(openAnteStore(t))
	if _, err := clean.Get(context.Background(), "bank", []byte("missing")); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected a missing key, got %v", err)
	}
	if clean.Fault() != nil {
		t.Fatalf("a missing key is not a fault: %v", clean.Fault())
	}
}

func TestStagedStoreExportsMergeStagedWritesInKeyOrder(t *testing.T) {
	base := openAnteStore(t)
	ctx := context.Background()
	for key, value := range map[string]string{"a": "1", "c": "3", "d": "4"} {
		if err := base.Set(ctx, "bank", []byte(key), []byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	staged := NewStagedStore(base)
	if err := staged.Delete(ctx, "bank", []byte("c")); err != nil {
		t.Fatal(err)
	}
	if err := staged.Set(ctx, "bank", []byte("b"), []byte("2")); err != nil {
		t.Fatal(err)
	}
	if err := staged.Set(ctx, "bank", []byte("d"), []byte("40")); err != nil {
		t.Fatal(err)
	}
	if err := staged.Set(ctx, "other", []byte("z"), []byte("9")); err != nil {
		t.Fatal(err)
	}

	pairs, err := staged.ExportNamespace(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	var keys, values []string
	for _, pair := range pairs {
		keys = append(keys, string(pair.Key))
		values = append(values, string(pair.Value))
	}
	if !reflect.DeepEqual(keys, []string{"a", "b", "d"}) || !reflect.DeepEqual(values, []string{"1", "2", "40"}) {
		t.Fatalf("unexpected merged export: keys=%v values=%v", keys, values)
	}
	prefixed, err := staged.ExportPrefix(ctx, "bank", []byte("b"))
	if err != nil {
		t.Fatal(err)
	}
	if len(prefixed) != 1 || string(prefixed[0].Key) != "b" {
		t.Fatalf("unexpected prefix export: %+v", prefixed)
	}
}

// A layer over a layer is how a transaction is isolated inside a block: what
// the transaction writes stays out of the block until it is applied.
func TestStagedStoreLayersIsolateAFailedTransaction(t *testing.T) {
	ctx := context.Background()
	block := NewStagedStore(openAnteStore(t))
	if err := block.Set(ctx, "bank", []byte("a"), []byte("1")); err != nil {
		t.Fatal(err)
	}
	txLayer := NewStagedStore(block)
	if err := txLayer.Set(ctx, "bank", []byte("a"), []byte("2")); err != nil {
		t.Fatal(err)
	}
	if value, _ := txLayer.Get(ctx, "bank", []byte("a")); string(value) != "2" {
		t.Fatalf("the layer must see its own write, got %q", value)
	}
	if value, _ := block.Get(ctx, "bank", []byte("a")); string(value) != "1" {
		t.Fatalf("the block must not see an unapplied write, got %q", value)
	}
	if err := txLayer.ApplyTo(ctx, block); err != nil {
		t.Fatal(err)
	}
	if value, _ := block.Get(ctx, "bank", []byte("a")); string(value) != "2" {
		t.Fatalf("the block must see the applied write, got %q", value)
	}
}

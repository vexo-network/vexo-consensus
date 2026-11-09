package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var errRuntimeCommitFailed = errors.New("runtime commit failed")

func TestRuntimeExecuteBlockUsesConfiguredApplication(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEphemeralForTesting(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one tx result, got %d", len(response.Results))
	}
	commit, err := application.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if commit.Height != 1 {
		t.Fatalf("expected app committed height 1, got %d", commit.Height)
	}
}

// The base fee is application state: the runtime keeps no copy of it. A block is priced against
// what the previous block stored, so a runtime that recovers from the store prices the next block
// exactly like the one that never stopped.
func TestRuntimeTakesTheFeeMarketFromTheApplicationAndRecoversIt(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	cfg := config.Default("vexo-test")
	newApplication := func() *vexoapp.Runtime {
		application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule()}, vexoapp.PrefixRouter{})
		if err != nil {
			t.Fatal(err)
		}
		application.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{
			BaseFee:      100,
			RequireNonce: true,
			FeeMarket: vexoapp.FeeMarketConfig{
				DynamicBaseFee:           true,
				TargetGas:                10,
				BaseFeeChangeDenominator: 8,
				MinBaseFee:               1,
			},
		}))
		application.WithStore(storage)
		return application
	}
	application := newApplication()
	if _, err := application.InitChain(vexoapp.InitChainRequest{Genesis: vexoapp.GenesisState{"bank:alice": []byte("10000")}}); err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	first, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: fairordering.SortTxsWithSalt([]types.Tx{
			[]byte("bank:send:alice:bob:1:fee=1000:gas=10:signer=alice:nonce=1"),
			[]byte("bank:send:alice:bob:1:fee=1000:gas=10:signer=alice:nonce=2"),
		}, fairordering.HeightSalt("vexo-test", 1)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.FeeMarket.BaseFee != 100 || first.FeeMarket.NextBaseFee != 112 {
		t.Fatalf("expected the block to move the base fee from 100 to 112, got %+v", first.FeeMarket)
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.BaseFee != 100 || state.NextBaseFee != 112 {
		t.Fatalf("the state record must keep the fee market the application reported: %+v", state)
	}

	recovered, err := NewWithStore(cfg, newApplication(), []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := recovered.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs: []types.Tx{
			[]byte("bank:send:alice:bob:1:fee=1000:gas=10:signer=alice:nonce=3"),
			[]byte("bank:send:alice:bob:1:fee=1120:gas=10:signer=alice:nonce=3"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.FeeMarket.BaseFee != 112 {
		t.Fatalf("the recovered runtime must price the block at 112, got %+v", second.FeeMarket)
	}
	if second.Results[0].Code == 0 || second.Results[1].Code != 0 {
		t.Fatalf("a fee of 1000 is below 10 gas at 112 and a fee of 1120 is exactly enough: %+v", second.Results)
	}
}

// reportingApplication reports a fixed fee market for every block, to show what the runtime does
// with the report.
type reportingApplication struct {
	*vexoapp.Runtime
	report vexoapp.FeeMarketReport
}

func (application reportingApplication) FinalizeBlockStagedContext(ctx context.Context, req vexoapp.FinalizeBlockRequest) (vexoapp.FinalizeBlockResponse, []store.KVWrite, error) {
	response, writes, err := application.Runtime.FinalizeBlockStagedContext(ctx, req)
	response.FeeMarket = application.report
	return response, writes, err
}

func TestRuntimeStateRecordKeepsTheReportedFeeMarket(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	inner, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default("vexo-test")
	cfg.Execution.TargetBlobGas = 10
	application := reportingApplication{Runtime: inner, report: vexoapp.FeeMarketReport{BaseFee: 7, NextBaseFee: 8, BlobBaseFee: 9, NextBlobBaseFee: 10, BlobGasUsed: 25}}
	runtime, err := NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	response, err := runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if response.FeeMarket != application.report {
		t.Fatalf("the runtime must pass the report on, got %+v", response.FeeMarket)
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.BaseFee != 7 || state.NextBaseFee != 8 || state.BlobBaseFee != 9 || state.NextBlobBaseFee != 10 || state.BlobGasUsed != 25 || state.ExcessBlobGas != 15 {
		t.Fatalf("the state record must mirror the reported fee market: %+v", state)
	}
}

// A certified block is executed whatever its content: limits such as the blob gas cap are enforced
// when a block is proposed, and a runtime that refused a block a quorum already certified would
// stop the node.
func TestRuntimeExecutesACertifiedBlockAboveMaxBlobGas(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Execution.TargetBlobGas = 5
	cfg.Execution.MaxBlobGas = 10
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEphemeralForTesting(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: []types.Tx{
			[]byte("bank:send:alice:bob:1:" + ethcompat.TagBlobGas + "=11"),
		},
	})
	if err != nil {
		t.Fatalf("a certified block must not fail in the runtime: %v", err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(response.Results))
	}
}

func TestRuntimeExecuteBlockPersistsBlockAndState(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2, ValidatorSetHash: types.Hash{9}},
		Txs:    []types.Tx{[]byte("bank:send")},
	}
	response, err := runtime.ExecuteBlock(context.Background(), block)
	if err != nil {
		t.Fatal(err)
	}

	record, err := storage.BlockByHeight(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if record.AppHash != response.AppHash {
		t.Fatalf("expected stored app hash %x, got %x", response.AppHash, record.AppHash)
	}
	if namespaces := runtime.StateNamespaces(); len(record.StateRoots) != len(namespaces) || record.StateRoots[0].Namespace != "bank" || record.StateRoots[len(namespaces)-1].Namespace != "validator_registry" {
		t.Fatalf("expected a stored root for every namespace of %v, got %+v", namespaces, record.StateRoots)
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 2 || state.ValidatorSetHash != (types.Hash{9}) {
		t.Fatalf("unexpected stored state: %+v", state)
	}
	rootRecord, err := storage.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if rootRecord.Root == (types.Hash{}) {
		t.Fatal("expected non-zero bank state root")
	}

	queriedBlock, err := runtime.BlockByHeight(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if queriedBlock.Hash != record.Hash {
		t.Fatalf("expected runtime block query hash %x, got %x", record.Hash, queriedBlock.Hash)
	}
	queriedState, err := runtime.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if queriedState.Height != 2 {
		t.Fatalf("expected latest state height 2, got %d", queriedState.Height)
	}
	queriedRoot, err := runtime.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if queriedRoot.Root != rootRecord.Root {
		t.Fatalf("expected queried root %x, got %x", rootRecord.Root, queriedRoot.Root)
	}
}

func TestRuntimeExecuteBlockAppliesValidatorUpdates(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "alice", Address: "alice", VotingPower: 2},
			joinUpdate("bob", 1),
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs:    []types.Tx{[]byte("staking:update")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ValidatorUpdates) != 2 {
		t.Fatalf("expected two validator updates, got %d", len(response.ValidatorUpdates))
	}
	previousSet, err := runtime.Validators.ValidatorSet(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := previousSet.Get("bob"); found {
		t.Fatal("bob must not exist until next validator-set height")
	}

	updatedSet, err := runtime.Validators.ValidatorSet(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := updatedSet.Get("alice")
	if !found || alice.VotingPower != 2 {
		t.Fatalf("expected alice power 2, got %+v found=%v", alice, found)
	}
	bob, found := updatedSet.Get("bob")
	if !found || bob.VotingPower != 1 {
		t.Fatalf("expected bob joined with power 1, got %+v found=%v", bob, found)
	}
	if !bytes.Equal(bob.PublicKey, joinKey("bob")) {
		t.Fatalf("expected bob to join with the key of the update, got %x", bob.PublicKey)
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.ValidatorSetHash != updatedSet.Hash() {
		t.Fatal("expected persisted state to use updated validator set hash")
	}
}

// The validator set changes through the staking module: the create-validator transaction of an
// operator registers the consensus key and its self-delegation backs the power, and the validator joins
// the set at the next height with exactly that key and power. A delegation moves the power of a genesis
// validator and keeps its key.
func TestRuntimeAppliesAStakingCreateValidatorToTheValidatorSet(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default("vexo-test")
	aliceKey := joinKey("alice")
	bobKey := joinKey("bob")
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{
		bank.NewModule(),
		staking.NewModuleWithPolicy(staking.Policy{
			CryptoBackend: cfg.Crypto.Backend,
			Admission:     validator.NewConfigurableAdmissionPolicy(cfg.Validator),
		}),
	}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	runtime, err := NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: aliceKey},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.InitChain(ctx, vexoapp.InitChainRequest{
		Genesis:    vexoapp.GenesisState{"bank:alice": []byte("10"), "bank:bob": []byte("10")},
		Validators: []types.ValidatorUpdate{{ID: "alice", Address: "alice", PublicKey: aliceKey, VotingPower: 1, Stake: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	initialSet, err := runtime.Validators.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.ExecuteBlock(ctx, types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs: []types.Tx{
			types.Tx("staking:create-validator:bob:bob:3:" + base64.RawStdEncoding.EncodeToString(bobKey) + ":signer=bob"),
			types.Tx("staking:delegate:alice:alice:2:signer=alice"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, result := range response.Results {
		if result.Code != 0 {
			t.Fatalf("staking transaction %d failed: %+v", index, result)
		}
	}
	if len(response.ValidatorUpdates) != 2 {
		t.Fatalf("expected an update for alice and one for bob, got %+v", response.ValidatorUpdates)
	}
	beforeSet, err := runtime.Validators.ValidatorSet(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := beforeSet.Get("bob"); found {
		t.Fatal("bob must not join before the next validator-set height")
	}
	updatedSet, err := runtime.Validators.ValidatorSet(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := updatedSet.Get("alice")
	if !found || alice.VotingPower != 3 || !bytes.Equal(alice.PublicKey, aliceKey) {
		t.Fatalf("expected alice with power 3 and her genesis key, got %+v found=%v", alice, found)
	}
	bob, found := updatedSet.Get("bob")
	if !found || bob.VotingPower != 3 || !bytes.Equal(bob.PublicKey, bobKey) {
		t.Fatalf("expected bob to join with power 3 and the key of his create-validator transaction, got %+v found=%v", bob, found)
	}
	state, err := storage.LatestState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.ValidatorSetHash != updatedSet.Hash() {
		t.Fatal("expected the persisted state to commit to the updated validator set")
	}
}

// A block is executed after a quorum certified it, so a validator update the registry cannot apply never
// fails the block: it is skipped, and the updates around it still apply. An unknown validator joins only
// with a consensus key, and only when the admission policy accepts it.
func TestRuntimeSkipsValidatorJoinsTheRegistryRefuses(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default("vexo-test")
	cfg.Validator.MinStake = 5
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "keyless", Address: "keyless", VotingPower: 5, Stake: 5},
			joinUpdate("underfunded", 1),
			joinUpdate("dave", 5),
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	runtime, err := NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := runtime.Validators.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.ExecuteBlock(ctx, types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs:    []types.Tx{[]byte("staking:update")},
	}); err != nil {
		t.Fatalf("an update the registry refuses must not fail the certified block: %v", err)
	}
	updatedSet, err := runtime.Validators.ValidatorSet(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	for _, refused := range []types.ValidatorID{"keyless", "underfunded"} {
		if _, found := updatedSet.Get(refused); found {
			t.Fatalf("%s must not join the validator set", refused)
		}
	}
	if dave, found := updatedSet.Get("dave"); !found || dave.VotingPower != 5 {
		t.Fatalf("expected the accepted join next to the refused ones, got %+v found=%v", dave, found)
	}
	if _, found := updatedSet.Get("alice"); !found || len(updatedSet.List()) != 2 {
		t.Fatalf("expected only alice and dave, got %+v", updatedSet.List())
	}
	if _, err := storage.StateByHeight(ctx, 5); err != nil {
		t.Fatalf("expected the block to be committed: %v", err)
	}
}

func TestRuntimeStagedValidatorUpdatesRollbackWhenCommitFails(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "alice", Address: "alice", VotingPower: 2},
			joinUpdate("bob", 1),
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	attempted := new([]store.KVWrite)
	failingStore := failingAppCommitStore{LevelDBStore: storage, attempted: attempted}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, failingStore)
	if err != nil {
		t.Fatal(err)
	}
	initialSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs:    []types.Tx{[]byte("staking:update")},
	})
	if !errors.Is(err, errRuntimeCommitFailed) {
		t.Fatalf("expected commit failure, got %v", err)
	}
	// The join of bob was part of the batch that failed: the rollback below is not for lack of a join.
	joinStaged := false
	for _, write := range *attempted {
		if write.Namespace == validator.RegistryNamespace && bytes.Contains(write.Value, []byte(`"bob"`)) {
			joinStaged = true
		}
	}
	if !joinStaged {
		t.Fatalf("expected the validator set with bob in the failed commit, got %d writes", len(*attempted))
	}
	reopened, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, err := reopened.ValidatorSet(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	if bob, found := set.Get("bob"); found {
		t.Fatalf("validator update must not persist when block commit fails, got bob=%+v", bob)
	}
	alice, found := set.Get("alice")
	if !found || alice.VotingPower != 1 {
		t.Fatalf("expected alice power to remain 1, got %+v found=%v", alice, found)
	}
	if _, err := storage.StateByHeight(context.Background(), 5); !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected block state rollback too, got %v", err)
	}
}

func TestRuntimeCountsAPostCommitReconcileFailureWithoutFailingTheBlock(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "alice", Address: "alice", VotingPower: 2},
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	storeRegistry, ok := runtime.Validators.(*validator.StoreRegistry)
	if !ok {
		t.Fatalf("expected store registry, got %T", runtime.Validators)
	}
	failures := 1
	runtime.Validators = failingKeyRegistry{StoreRegistry: storeRegistry, failHeight: 6, failures: &failures}
	initialSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs:    []types.Tx{[]byte("staking:update")},
	}); err != nil {
		t.Fatalf("post-commit reconcile failure should recover without failing committed block: %v", err)
	}
	if failures := runtime.PostCommitReconciliationFailures(); failures != 1 {
		t.Fatalf("expected one reconcile failure, got %d", failures)
	}
	if _, err := storage.StateByHeight(context.Background(), 5); err != nil {
		t.Fatalf("block/state commit should remain durable before reconcile failure report: %v", err)
	}
	recoveredSet, err := runtime.Validators.ValidatorSet(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := recoveredSet.Get("alice")
	if !found || alice.VotingPower != 2 {
		t.Fatalf("expected recovered validator power 2, got %+v found=%v", alice, found)
	}
}

func TestRuntimeExecuteBlockRemovesValidatorFromUpdates(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "bob", Address: "bob", VotingPower: 0},
		},
	}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEphemeralForTesting(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
		{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 6},
		Txs:    []types.Tx{[]byte("staking:update")},
	}); err != nil {
		t.Fatal(err)
	}
	updatedSet, err := runtime.Validators.ValidatorSet(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := updatedSet.Get("bob"); found {
		t.Fatal("expected bob removed from validator set")
	}
}

func TestRuntimeInjectsStoreIntoAppRuntime(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 3},
		Txs:    []types.Tx{[]byte("bank:set")},
	}); err != nil {
		t.Fatal(err)
	}

	value, err := storage.Get(context.Background(), "bank", []byte("runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "ok" {
		t.Fatalf("expected runtime store value ok, got %s", value)
	}
}

func TestRuntimeRecoversLatestStateIntoAppRuntime(t *testing.T) {
	path := t.TempDir()
	storage, err := store.OpenLevelDB(path)
	if err != nil {
		t.Fatal(err)
	}

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 4},
		Txs:    []types.Tx{[]byte("bank:set")},
	}); err != nil {
		t.Fatal(err)
	}
	expectedCommit, err := application.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.OpenLevelDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	recoveredApplication, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&storeWritingModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	recoveredRuntime, err := NewWithStore(config.Default("vexo-test"), recoveredApplication, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, reopened)
	if err != nil {
		t.Fatal(err)
	}

	state, err := recoveredRuntime.Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 4 {
		t.Fatalf("expected recovered height 4, got %d", state.Height)
	}
	recoveredCommit, err := recoveredApplication.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if recoveredCommit.Height != expectedCommit.Height || recoveredCommit.AppHash != expectedCommit.AppHash {
		t.Fatalf("unexpected recovered commit: %+v expected %+v", recoveredCommit, expectedCommit)
	}
}

type runtimeModule struct {
	name string
}

func (module *runtimeModule) Name() string {
	return module.name
}

func (module *runtimeModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module *runtimeModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module *runtimeModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module *runtimeModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}

type storeWritingModule struct {
	name string
}

func (module *storeWritingModule) Name() string {
	return module.name
}

func (module *storeWritingModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module *storeWritingModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module *storeWritingModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: "missing store"}
	}
	if err := ctx.Store.Set(context.Background(), "bank", []byte("runtime"), []byte("ok")); err != nil {
		return types.Result{Code: 2, Log: err.Error()}
	}
	return types.Result{}
}

func (module *storeWritingModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}

type validatorUpdateModule struct {
	runtimeModule
	updates []types.ValidatorUpdate
}

// joinKey is the consensus key joinUpdate gives a validator.
func joinKey(id types.ValidatorID) types.PublicKey {
	return types.PublicKey("consensus-key-" + string(id))
}

// joinUpdate is the update the staking module reports for a validator that is not in the set yet: its
// full identity, the consensus key included. The registry admits a validator only with a key.
func joinUpdate(id types.ValidatorID, power types.VotingPower) types.ValidatorUpdate {
	return types.ValidatorUpdate{
		ID:          id,
		Address:     types.Address(id),
		PublicKey:   joinKey(id),
		VotingPower: power,
		Stake:       uint64(power),
	}
}

func (module *validatorUpdateModule) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	return module.updates
}

type failingAppCommitStore struct {
	*store.LevelDBStore
	// attempted receives the writes of every commit the store refused.
	attempted *[]store.KVWrite
}

func (failing failingAppCommitStore) CommitBlockStateWithWrites(ctx context.Context, writes []store.KVWrite, block store.BlockRecord, state store.StateRecord, roots []store.StateRootRecord) error {
	if failing.attempted != nil {
		*failing.attempted = append(*failing.attempted, writes...)
	}
	return errRuntimeCommitFailed
}

// failingKeyRegistry fails the first lookups of the validator set of height failHeight, which the
// runtime makes after the commit to admit the keys of a new set.
type failingKeyRegistry struct {
	*validator.StoreRegistry
	failHeight types.Height
	failures   *int
}

func (registry failingKeyRegistry) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	if height == registry.failHeight && *registry.failures > 0 {
		*registry.failures--
		return nil, errRuntimeCommitFailed
	}
	return registry.StoreRegistry.ValidatorSet(ctx, height)
}

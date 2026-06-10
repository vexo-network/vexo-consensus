package runtime

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
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
	runtime, err := NewEphemeral(config.Default("vexo-test"), application, []validator.Validator{
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

func TestRuntimeUpdatesAndRecoversDynamicBaseFee(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	cfg := config.Default("vexo-test")
	cfg.Execution.BaseFee = 100
	cfg.Execution.DynamicBaseFee = true
	cfg.Execution.TargetGas = 10
	cfg.Execution.BaseFeeChangeDenominator = 8
	cfg.Execution.MinBaseFee = 1
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	application.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{
		BaseFee:      cfg.Execution.BaseFee,
		RequireNonce: true,
	}))
	application.WithStore(storage)
	if _, err := application.InitChain(vexoapp.InitChainRequest{Genesis: vexoapp.GenesisState{"bank:alice": []byte("10000")}}); err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: fairordering.SortTxsWithSalt([]types.Tx{
			[]byte("bank:send:alice:bob:1:fee=1000:gas=10:signer=alice:nonce=1"),
			[]byte("bank:send:alice:bob:1:fee=1000:gas=10:signer=alice:nonce=2"),
		}, fairordering.HeightSalt("vexo-test", 1)),
	}
	if _, err := runtime.ExecuteBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}
	if runtime.CurrentBaseFee() != 112 {
		t.Fatalf("expected next base fee 112, got %d", runtime.CurrentBaseFee())
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.BaseFee != 100 || state.NextBaseFee != 112 {
		t.Fatalf("unexpected persisted fee market state: %+v", state)
	}

	recoveredApplication, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	recoveredApplication.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{
		BaseFee:      cfg.Execution.BaseFee,
		RequireNonce: true,
	}))
	recovered, err := NewWithStore(cfg, recoveredApplication, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentBaseFee() != 112 {
		t.Fatalf("expected recovered base fee 112, got %d", recovered.CurrentBaseFee())
	}
}

func TestEphemeralRuntimeUpdatesDynamicBaseFee(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Execution.BaseFee = 100
	cfg.Execution.DynamicBaseFee = true
	cfg.Execution.TargetGas = 10
	cfg.Execution.BaseFeeChangeDenominator = 8
	cfg.Execution.MinBaseFee = 1
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1

	runtime, err := NewEphemeral(cfg, gasApp{gasUsed: 10}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("a"), []byte("b")},
	}
	if _, err := runtime.ExecuteBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}
	if runtime.CurrentBaseFee() != 112 {
		t.Fatalf("expected next base fee 112, got %d", runtime.CurrentBaseFee())
	}
}

type gasApp struct {
	gasUsed uint64
}

func (gasApp) InitChain(req vexoapp.InitChainRequest) (vexoapp.InitChainResponse, error) {
	return vexoapp.InitChainResponse{}, nil
}

func (gasApp) CheckTx(tx types.Tx) vexoapp.CheckTxResponse {
	return vexoapp.CheckTxResponse{}
}

func (gasApp) PrepareProposal(req vexoapp.PrepareProposalRequest) (vexoapp.PrepareProposalResponse, error) {
	return vexoapp.PrepareProposalResponse{Txs: req.Txs}, nil
}

func (gasApp) ProcessProposal(req vexoapp.ProcessProposalRequest) vexoapp.ProcessProposalResponse {
	return vexoapp.ProcessProposalResponse{Accepted: true}
}

func (application gasApp) FinalizeBlock(req vexoapp.FinalizeBlockRequest) (vexoapp.FinalizeBlockResponse, error) {
	results := make([]types.Result, 0, len(req.Block.Txs))
	for range req.Block.Txs {
		results = append(results, types.Result{GasUsed: application.gasUsed})
	}
	return vexoapp.FinalizeBlockResponse{Results: results}, nil
}

func (gasApp) Commit() (vexoapp.CommitResponse, error) {
	return vexoapp.CommitResponse{}, nil
}

func (gasApp) Query(req vexoapp.QueryRequest) vexoapp.QueryResponse {
	return vexoapp.QueryResponse{}
}

func TestRuntimeUpdatesAndRecoversDynamicBlobBaseFee(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	cfg := config.Default("vexo-test")
	cfg.Execution.BlobBaseFee = 100
	cfg.Execution.DynamicBlobBaseFee = true
	cfg.Execution.TargetBlobGas = 10
	cfg.Execution.MaxBlobGas = 100
	cfg.Execution.BlobFeeChangeDenominator = 8
	cfg.Execution.MinBlobBaseFee = 1

	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	application.WithStore(storage)
	if _, err := application.InitChain(vexoapp.InitChainRequest{Genesis: vexoapp.GenesisState{"bank:alice": []byte("10000")}}); err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}

	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: []types.Tx{
			[]byte("bank:send:alice:bob:1:fee=1:gas=1:" + ethcompat.TagBlobGas + "=20"),
		},
	}
	if _, err := runtime.ExecuteBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}
	if runtime.CurrentBlobBaseFee() != 112 {
		t.Fatalf("expected next blob base fee 112, got %d", runtime.CurrentBlobBaseFee())
	}
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.BlobBaseFee != 100 || state.NextBlobBaseFee != 112 || state.BlobGasUsed != 20 || state.ExcessBlobGas != 10 {
		t.Fatalf("unexpected persisted blob fee state: %+v", state)
	}

	recoveredApplication, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := NewWithStore(cfg, recoveredApplication, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if recovered.CurrentBlobBaseFee() != 112 {
		t.Fatalf("expected recovered blob base fee 112, got %d", recovered.CurrentBlobBaseFee())
	}
}

func TestRuntimeRejectsBlockAboveMaxBlobGas(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Execution.MaxBlobGas = 10
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEphemeral(cfg, application, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	block := types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs: []types.Tx{
			[]byte("bank:send:alice:bob:1:" + ethcompat.TagBlobGas + "=11"),
		},
	}
	if _, err := runtime.ExecuteBlock(context.Background(), block); !errors.Is(err, ErrBlobGasLimitExceeded) {
		t.Fatalf("expected blob gas limit rejection, got %v", err)
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
	if len(record.StateRoots) != 1 || record.StateRoots[0].Namespace != "bank" {
		t.Fatalf("expected stored block state root summary, got %+v", record.StateRoots)
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
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
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
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.ValidatorSetHash != updatedSet.Hash() {
		t.Fatal("expected persisted state to use updated validator set hash")
	}
	events := runtime.Validators.RotationEvents()
	if len(events) != 2 || events[0].Height != 6 || events[1].Height != 6 {
		t.Fatalf("expected validator update events at height 6, got %+v", events)
	}
}

func TestRuntimeStagedValidatorUpdatesRollbackWhenCommitFails(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&validatorUpdateModule{
		runtimeModule: runtimeModule{name: "staking"},
		updates: []types.ValidatorUpdate{
			{ID: "alice", Address: "alice", VotingPower: 2},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
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
	failingStore := failingAppCommitStore{LevelDBStore: storage}
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

func TestRuntimeReportsStagedValidatorReconcileFailure(t *testing.T) {
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
	runtime.Validators = failingStagedValidatorRegistry{StoreRegistry: storeRegistry}
	initialSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHash: initialSet.Hash()},
		Txs:    []types.Tx{[]byte("staking:update")},
	})
	if !errors.Is(err, ErrValidatorRegistryCommitFailed) {
		t.Fatalf("expected validator reconcile error, got %v", err)
	}
	if failures := runtime.PostCommitReconciliationFailures(); failures != 1 {
		t.Fatalf("expected one reconcile failure, got %d", failures)
	}
	if _, err := storage.StateByHeight(context.Background(), 5); err != nil {
		t.Fatalf("block/state commit should remain durable before reconcile failure report: %v", err)
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
	runtime, err := NewEphemeral(config.Default("vexo-test"), application, []validator.Validator{
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

func (module *validatorUpdateModule) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	return module.updates
}

type failingAppCommitStore struct {
	*store.LevelDBStore
}

func (failingAppCommitStore) CommitBlockStateWithWrites(ctx context.Context, writes []store.KVWrite, block store.BlockRecord, state store.StateRecord, roots []store.StateRootRecord) error {
	return errRuntimeCommitFailed
}

type failingStagedValidatorRegistry struct {
	*validator.StoreRegistry
}

func (registry failingStagedValidatorRegistry) CommitStagedValidatorUpdates(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) error {
	return errRuntimeCommitFailed
}

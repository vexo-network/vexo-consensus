package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRuntimeNewWiresModules(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, map[types.Address]types.VotingPower{"alice": 1})
	if err != nil {
		t.Fatal(err)
	}

	if runtime.Config.ChainID != "vexo-test" {
		t.Fatalf("unexpected chain id: %s", runtime.Config.ChainID)
	}
	if runtime.Validators == nil || runtime.Mempool == nil || runtime.Slashing == nil || runtime.Governance == nil || runtime.P2PScore == nil {
		t.Fatal("expected modules to be wired")
	}
}

func TestRuntimeRejectsInvalidConfig(t *testing.T) {
	_, err := New(config.Default(""), noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrMissingChainID) {
		t.Fatalf("expected missing chain id, got %v", err)
	}
}

func TestRuntimeRejectsUnsupportedCryptoBackend(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Crypto.Backend = "unknown"
	_, err := New(cfg, noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestRuntimeRejectsUnsupportedCommitteeBackend(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Committee.Backend = "unknown"
	_, err := New(cfg, noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestRuntimeBuildsVRFCommitteeSelector(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Committee.Backend = committee.BackendVRF
	cfg.Committee.CommitteeSize = 1
	cfg.VRF.Keys = map[string][]byte{"alice-pub": []byte("alice-secret")}

	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("alice-pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	set, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	committeeResult, err := runtime.Committee.Select(context.Background(), 0, 0, types.Hash{1}, set)
	if err != nil {
		t.Fatal(err)
	}
	if len(committeeResult.Members) != 1 || len(committeeResult.Members[0].Proof) == 0 {
		t.Fatalf("expected VRF selected member with proof, got %+v", committeeResult.Members)
	}
}

func TestRuntimeDoesNotLoadVRFAdapterForDeterministicCommittee(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Committee.Backend = committee.BackendDeterministic
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AdapterName = "missing-vrf"

	if _, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("alice-pub")},
	}, nil); err != nil {
		t.Fatalf("deterministic committee must not require VRF adapter: %v", err)
	}
}

func TestRuntimeBuildsConsensusStateMachine(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	stateMachine, err := runtime.NewConsensusStateMachine(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	stateMachine.StartRound(1, 0)
	status := stateMachine.Status(context.Background())
	if status.ChainID != "vexo-test" || status.Height != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRuntimeBuildsFinalityVerifier(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.NewFinalityVerifier(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeAppliesUpgradeHookAndPersistsSchemaState(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	registry := upgrade.NewRegistry()
	registry.RegisterConfig(upgrade.Migration{From: 1, To: 2})
	registry.RegisterStore(upgrade.Migration{From: 1, To: 2})
	registry.RegisterAppState(upgrade.Migration{From: 1, To: 2})
	runtime.WithUpgrade(upgrade.Plan{
		Name:               "v2",
		Height:             2,
		BinaryVersion:      "v2.0.0",
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     2,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   2,
	}, upgrade.State{
		Height:              1,
		BinaryVersion:       "v1.0.0",
		ConfigSchemaVersion: 1,
		StoreSchemaVersion:  1,
		AppStateVersion:     1,
	}, upgrade.NewExecutor(registry, upgrade.NewMemoryRecorder()))

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: 2}}); err != nil {
		t.Fatal(err)
	}
	schemaState, err := storage.SchemaState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if schemaState.BinaryVersion != "v2.0.0" || schemaState.StoreSchemaVersion != 2 || schemaState.AppStateVersion != 2 {
		t.Fatalf("unexpected schema state: %+v", schemaState)
	}
}

func TestRuntimeWithStoreUsesDurableSlashingKeeper(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(); err != nil {
			t.Fatal(err)
		}
	})
	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	keeper, ok := runtime.Slashing.(*slashing.StoreKeeper)
	if !ok {
		t.Fatalf("expected durable slashing keeper, got %T", runtime.Slashing)
	}

	evidence := slashing.Evidence{Type: slashing.EvidenceDoubleSign, Validator: "alice", Height: 1, Round: 0, Proof: []byte("proof")}
	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.ApplyPenaltyWithStake(context.Background(), evidence, 100); err != nil {
		t.Fatal(err)
	}

	reopened, err := slashing.NewStoreKeeper(storage, nil)
	if err != nil {
		t.Fatal(err)
	}
	receipt, found, err := reopened.PenaltyReceipt(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || receipt.RemainingPower >= receipt.PreviousPower {
		t.Fatalf("expected persisted slashing receipt, got found=%t receipt=%+v", found, receipt)
	}
}

func TestRuntimeWithStoreUsesDurableValidatorRegistry(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(); err != nil {
			t.Fatal(err)
		}
	})
	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runtime.Validators.(*validator.StoreRegistry); !ok {
		t.Fatalf("expected store-backed validator registry, got %T", runtime.Validators)
	}
	if err := runtime.ApplyValidatorUpdatesAt(context.Background(), 2, []types.ValidatorUpdate{{ID: "alice", Address: "alice", VotingPower: 75, Stake: 75}}); err != nil {
		t.Fatal(err)
	}
	reopened, err := validator.NewStoreRegistry(context.Background(), storage, nil, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, err := reopened.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := set.Get("alice")
	if !found || alice.VotingPower != 75 {
		t.Fatalf("expected persisted alice power 75, got %+v found=%t", alice, found)
	}
}

func TestRuntimeBuildsEd25519FinalityVerifier(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Crypto.Backend = config.CryptoBackendEd25519
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.NewFinalityVerifier(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeContextCancellation(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := runtime.NewConsensusStateMachine(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected consensus context canceled, got %v", err)
	}
	if _, err := runtime.NewFinalityVerifier(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected finality context canceled, got %v", err)
	}
}

type noopApp struct{}

func (noopApp) InitChain(req app.InitChainRequest) (app.InitChainResponse, error) {
	return app.InitChainResponse{}, nil
}

func (noopApp) CheckTx(tx types.Tx) app.CheckTxResponse {
	return app.CheckTxResponse{}
}

func (noopApp) PrepareProposal(req app.PrepareProposalRequest) (app.PrepareProposalResponse, error) {
	return app.PrepareProposalResponse{Txs: req.Txs}, nil
}

func (noopApp) ProcessProposal(req app.ProcessProposalRequest) app.ProcessProposalResponse {
	return app.ProcessProposalResponse{Accepted: true}
}

func (noopApp) FinalizeBlock(req app.FinalizeBlockRequest) (app.FinalizeBlockResponse, error) {
	return app.FinalizeBlockResponse{}, nil
}

func (noopApp) Commit() (app.CommitResponse, error) {
	return app.CommitResponse{}, nil
}

func (noopApp) Query(req app.QueryRequest) app.QueryResponse {
	return app.QueryResponse{}
}

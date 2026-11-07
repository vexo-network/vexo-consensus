package runtime

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRuntimeNewWiresModules(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, map[types.Address]types.VotingPower{"alice": 1})
	if err != nil {
		t.Fatal(err)
	}

	if runtime.Config.ChainID != "vexo-test" {
		t.Fatalf("unexpected chain id: %s", runtime.Config.ChainID)
	}
	if runtime.Validators == nil || runtime.Mempool == nil || runtime.Governance == nil || runtime.P2PScore == nil {
		t.Fatal("expected modules to be wired")
	}
}

func TestRuntimeNewRequiresDurableStore(t *testing.T) {
	_, err := New(config.Default("vexo-test"), noopApp{}, nil, nil)
	if !errors.Is(err, ErrDurableStoreRequired) {
		t.Fatalf("expected durable store requirement, got %v", err)
	}
}

func TestRuntimeNewEphemeralRejectsNetworkLikeChainID(t *testing.T) {
	_, err := NewEphemeral(config.Default("vexo-chain"), noopApp{}, nil, nil)
	if !errors.Is(err, ErrEphemeralRuntimeNotAllowed) {
		t.Fatalf("expected ephemeral runtime opt-in error, got %v", err)
	}
}

func TestRuntimeRejectsInvalidConfig(t *testing.T) {
	_, err := NewEphemeral(config.Default(""), noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrMissingChainID) {
		t.Fatalf("expected missing chain id, got %v", err)
	}
}

func TestRuntimeRejectsUnsupportedCryptoBackend(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Crypto.Backend = "unknown"
	_, err := NewEphemeral(cfg, noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestRuntimeRejectsUnsupportedCommitteeBackend(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Committee.Backend = "unknown"
	_, err := NewEphemeral(cfg, noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestRuntimeBuildsVRFCommitteeSelector(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Committee.Backend = committee.BackendVRF
	cfg.Committee.CommitteeSize = 1
	privateKey := []byte("12345678901234567890123456789012")
	publicKey, err := vexocrypto.ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg.VRF.AdapterName = vexocrypto.VRFAdapterECVRFP256Name
	cfg.VRF.Keys = map[string][]byte{base64.StdEncoding.EncodeToString(publicKey): privateKey}

	runtime, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: publicKey},
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

	if _, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("alice-pub")},
	}, nil); err != nil {
		t.Fatalf("deterministic committee must not require VRF adapter: %v", err)
	}
}

func TestRuntimeBuildsConsensusStateMachine(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
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

// The finality verifier loads the set a header names; a registry height whose set the runtime has
// not executed yet is pending, not answered with the latest set.
func TestRuntimeFinalityVerifierRefusesValidatorSetsItHasNotExecuted(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	verifier := runtime.NewFinalityVerifier()
	header := types.Header{ChainID: "vexo-test", Height: 5, ValidatorSetHeight: 4}
	err = verifier.VerifyBlockCertWithContext(context.Background(), header, finality.QuorumCert{Height: 5, BlockHash: finality.HeaderHash(header)})
	if !errors.Is(err, consensus.ErrValidatorSetPending) {
		t.Fatalf("expected a pending validator set, got %v", err)
	}
}

func TestRuntimeAppliesUpgradeHookAndPersistsSchemaState(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewWithStore(config.Default("vexo-test"), newEmptyRuntimeApp(t), []validator.Validator{
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

func TestRuntimeLoadsUpgradePlanFromStoreByHeight(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	plan := upgrade.Plan{
		Name:               "v2",
		Height:             2,
		BinaryVersion:      "v2.0.0",
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     2,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   2,
	}
	if err := storage.SaveUpgradePlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), newEmptyRuntimeApp(t), []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	registry := upgrade.NewRegistry()
	registry.RegisterConfig(upgrade.Migration{From: 1, To: 2})
	registry.RegisterStore(upgrade.Migration{From: 1, To: 2})
	registry.RegisterAppState(upgrade.Migration{From: 1, To: 2})
	runtime.UpgradeExecutor = upgrade.NewExecutor(registry, upgrade.NewMemoryRecorder())
	runtime.UpgradeState = upgrade.State{
		Height:              1,
		BinaryVersion:       "v1.0.0",
		ConfigSchemaVersion: 1,
		StoreSchemaVersion:  1,
		AppStateVersion:     1,
	}

	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: 2}}); err != nil {
		t.Fatal(err)
	}
	if runtime.UpgradePlan != nil {
		t.Fatalf("an applied upgrade plan must be cleared, got %+v", runtime.UpgradePlan)
	}
	schemaState, err := storage.SchemaState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if schemaState.BinaryVersion != "v2.0.0" || schemaState.StoreSchemaVersion != 2 || schemaState.AppStateVersion != 2 {
		t.Fatalf("unexpected schema state: %+v", schemaState)
	}
}

// Once a plan is applied the runtime must look for the next one: a plan that stays loaded after it
// ran would shadow every later plan, and the chain would never upgrade a second time.
func TestRuntimePicksUpTheNextStoredUpgradePlanAfterOneIsApplied(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	for _, plan := range []upgrade.Plan{
		{Name: "v2", Height: 2, BinaryVersion: "v2.0.0", ConfigSchemaFrom: 1, ConfigSchemaTo: 2, StoreSchemaFrom: 1, StoreSchemaTo: 2, AppStateSchemaFrom: 1, AppStateSchemaTo: 2},
		{Name: "v3", Height: 4, BinaryVersion: "v3.0.0", ConfigSchemaFrom: 2, ConfigSchemaTo: 3, StoreSchemaFrom: 2, StoreSchemaTo: 3, AppStateSchemaFrom: 2, AppStateSchemaTo: 3},
	} {
		if err := storage.SaveUpgradePlan(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), newEmptyRuntimeApp(t), []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	registry := upgrade.NewRegistry()
	for from := uint64(1); from <= 2; from++ {
		registry.RegisterConfig(upgrade.Migration{From: from, To: from + 1})
		registry.RegisterStore(upgrade.Migration{From: from, To: from + 1})
		registry.RegisterAppState(upgrade.Migration{From: from, To: from + 1})
	}
	runtime.UpgradeExecutor = upgrade.NewExecutor(registry, upgrade.NewMemoryRecorder())
	runtime.UpgradeState = upgrade.State{Height: 1, BinaryVersion: "v1.0.0", ConfigSchemaVersion: 1, StoreSchemaVersion: 1, AppStateVersion: 1}

	for _, step := range []struct {
		height types.Height
		binary string
	}{{2, "v2.0.0"}, {3, "v2.0.0"}, {4, "v3.0.0"}, {5, "v3.0.0"}} {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: step.height}}); err != nil {
			t.Fatalf("height %d: %v", step.height, err)
		}
		schemaState, err := storage.SchemaState(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if schemaState.BinaryVersion != step.binary {
			t.Fatalf("height %d: expected binary %s, got %+v", step.height, step.binary, schemaState)
		}
	}
	schemaState, err := storage.SchemaState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if schemaState.StoreSchemaVersion != 3 || schemaState.AppStateVersion != 3 || schemaState.ConfigSchemaVersion != 3 {
		t.Fatalf("the second plan must migrate every schema to version 3, got %+v", schemaState)
	}
	if runtime.UpgradePlan != nil {
		t.Fatalf("no plan is left to apply, got %+v", runtime.UpgradePlan)
	}
}

func TestRuntimeHaltsStoredUpgradePlanWithoutExecutorRecorder(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	if err := storage.SaveUpgradePlan(context.Background(), upgrade.Plan{
		Name:                "v2",
		Height:              2,
		BinaryVersion:       "v2.0.0",
		ConfigSchemaFrom:    1,
		ConfigSchemaTo:      1,
		StoreSchemaFrom:     1,
		StoreSchemaTo:       1,
		AppStateSchemaFrom:  1,
		AppStateSchemaTo:    1,
		AllowNoopMigrations: true,
	}); err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	runtime.UpgradeState = upgrade.State{
		Height:              1,
		BinaryVersion:       "v1.0.0",
		ConfigSchemaVersion: 1,
		StoreSchemaVersion:  1,
		AppStateVersion:     1,
	}

	_, err = runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: 2}})
	if !errors.Is(err, ErrUpgradeExecutorMissing) {
		t.Fatalf("expected missing upgrade executor error, got %v", err)
	}
	if !runtime.UpgradeHalted {
		t.Fatal("expected runtime to halt when stored upgrade has no durable executor")
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
	runtime, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The set of the first block loads and its keys are admitted; only the header's set hash,
	// which names no real set, is wrong.
	header := types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHeight: 1, ValidatorSetHash: types.Hash{0xEE}}
	err = runtime.NewFinalityVerifier().VerifyBlockCertWithContext(context.Background(), header, finality.QuorumCert{Height: 1, BlockHash: finality.HeaderHash(header)})
	if !errors.Is(err, finality.ErrValidatorSetMismatch) {
		t.Fatalf("expected a validator set mismatch, got %v", err)
	}
}

func TestRuntimeContextCancellation(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := NewEphemeral(cfg, noopApp{}, []validator.Validator{
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
	header := types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHeight: 1}
	if err := runtime.NewFinalityVerifier().VerifyBlockCertWithContext(ctx, header, finality.QuorumCert{Height: 1, BlockHash: finality.HeaderHash(header)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected finality context canceled, got %v", err)
	}
}

func TestNetworkSafeRuntimeRequiresAtomicApplicationAtStartup(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = NewWithStore(config.NetworkSafeTemplate("vexo-test", t.TempDir()), noopApp{}, nil, nil, storage)
	if !errors.Is(err, ErrAtomicAppCommitUnavailable) {
		t.Fatalf("expected atomic app startup guard, got %v", err)
	}
}

func TestNewNetworkSafeWithStoreRejectsUnsafeConfig(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = NewNetworkSafeWithStore(config.Default("vexo-test"), newEmptyRuntimeApp(t), nil, nil, storage)
	if !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected unsafe network config error, got %v", err)
	}
}

func TestNewNetworkSafeWithStoreRejectsMissingStore(t *testing.T) {
	_, err := NewNetworkSafeWithStore(config.NetworkSafeTemplate("vexo-test", t.TempDir()), newEmptyRuntimeApp(t), nil, nil, nil)
	if !errors.Is(err, ErrDurableStoreRequired) {
		t.Fatalf("expected durable store requirement, got %v", err)
	}
}

func TestNewNetworkSafeWithStoreAcceptsAtomicDurableInputs(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := NewNetworkSafeWithStore(config.NetworkSafeTemplate("vexo-test", t.TempDir()), newEmptyRuntimeApp(t), nil, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Store == nil {
		t.Fatal("expected durable store to be wired")
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

func newEmptyRuntimeApp(t *testing.T) *app.Runtime {
	t.Helper()
	application, err := app.NewRuntime("vexo-test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func encodeRuntimeTestUint64(value uint64) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, value)
	return encoded
}

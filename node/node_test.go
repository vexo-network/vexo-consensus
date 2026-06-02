package node

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestNodeStartStopLifecycle(t *testing.T) {
	node := newTestNode(t)

	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := node.Status(context.Background())
	if !status.Running || status.ChainID != "vexo-test" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := node.Runtime(); err != nil {
		t.Fatal(err)
	}
	if err := node.Start(context.Background()); !errors.Is(err, ErrNodeAlreadyRunning) {
		t.Fatalf("expected already running, got %v", err)
	}
	if err := node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := node.Runtime(); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running, got %v", err)
	}
	if err := node.Stop(context.Background()); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running on second stop, got %v", err)
	}
}

func TestNodeExecutesBlockThroughRuntime(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}
	status := node.Status(context.Background())
	if status.LatestHeight != 1 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("unexpected status after block: %+v", status)
	}
}

func TestNodePersistsRuntimeStore(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	restarted := newTestNodeWithDataDir(t, node.cfg.DataDir)
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop(context.Background())
	restartedRuntime, err := restarted.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	record, err := restartedRuntime.BlockByHeight(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if record.Block.Header.Height != 2 {
		t.Fatalf("expected stored block height 2, got %+v", record)
	}
}

func TestNodeQueriesBlocks(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:first")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:second")},
	}); err != nil {
		t.Fatal(err)
	}

	first, err := node.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Block.Header.Height != 1 || len(first.Block.Txs) != 1 {
		t.Fatalf("unexpected first block: %+v", first)
	}
	latest, err := node.LatestBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Block.Header.Height != 2 || len(latest.Block.Txs) != 1 {
		t.Fatalf("unexpected latest block: %+v", latest)
	}
	index, err := node.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 1 || index.LatestHeight != 2 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected block index: %+v", index)
	}
	state, err := node.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 2 || state.AppHash == (types.Hash{}) {
		t.Fatalf("unexpected latest state: %+v", state)
	}
	root, err := node.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if root.Height != 2 || root.Namespace != "bank" || root.Root == (types.Hash{}) {
		t.Fatalf("unexpected state root: %+v", root)
	}
}

func TestNodeValidation(t *testing.T) {
	application := newTestApplication(t)
	_, err := New(DefaultConfig("vexo-test", ""), validGenesis(), application)
	if !errors.Is(err, ErrMissingDataDir) {
		t.Fatalf("expected missing data dir, got %v", err)
	}
	_, err = New(DefaultConfig("vexo-test", t.TempDir()), Genesis{ChainID: "other"}, application)
	if !errors.Is(err, ErrGenesisChainID) {
		t.Fatalf("expected genesis chain id mismatch, got %v", err)
	}
	_, err = New(DefaultConfig("vexo-test", t.TempDir()), Genesis{ChainID: "vexo-test"}, application)
	if !errors.Is(err, ErrMissingValidators) {
		t.Fatalf("expected missing validators, got %v", err)
	}
	_, err = New(DefaultConfig("vexo-test", t.TempDir()), validGenesis(), nil)
	if !errors.Is(err, ErrMissingApplication) {
		t.Fatalf("expected missing application, got %v", err)
	}
}

func newTestNode(t *testing.T) *Node {
	t.Helper()
	return newTestNodeWithDataDir(t, t.TempDir())
}

func newTestNodeWithDataDir(t *testing.T, dataDir string) *Node {
	t.Helper()
	node, err := New(DefaultConfig("vexo-test", dataDir), validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func validGenesis() Genesis {
	return Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
}

func newTestApplication(t *testing.T) vexoapp.Application {
	t.Helper()
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{testModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	return application
}

type testModule struct {
	name string
}

func (module testModule) Name() string {
	return module.name
}

func (module testModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module testModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module testModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module testModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}

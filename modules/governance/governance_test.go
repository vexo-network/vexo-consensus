package governance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/fairordering"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestGovernanceModuleLifecycleThroughRuntime(t *testing.T) {
	keeper := vexogov.NewInMemoryKeeper(vexogov.TallyPolicy{
		QuorumPower:       2,
		YesThresholdPower: 2,
		VotingPeriod:      1,
		Timelock:          1,
	}, nil)
	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{NewModuleWithKeeper(keeper)}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{}); err != nil {
		t.Fatal(err)
	}

	finalizeGovernanceBlock(t, runtime, 1, []types.Tx{
		[]byte("governance:submit:alice:max-gas:execution:max_gas:20000000"),
	})
	finalizeGovernanceBlock(t, runtime, 2, []types.Tx{
		[]byte("governance:vote:1:alice:yes:1"),
		[]byte("governance:vote:1:bob:yes:1"),
	})
	tally := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "tally", "1"}})
	if tally.Code != 0 || !strings.Contains(string(tally.Value), `"Passed":true`) {
		t.Fatalf("unexpected tally query: %+v", tally)
	}

	finalizeGovernanceBlock(t, runtime, 3, []types.Tx{[]byte("governance:execute:1")})

	proposal := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "proposal", "1"}})
	if proposal.Code != 0 || !strings.Contains(string(proposal.Value), `"Executed":true`) {
		t.Fatalf("unexpected proposal query: %+v", proposal)
	}
	applied := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "applied"}})
	if applied.Code != 0 || !strings.Contains(string(applied.Value), `"Module":"execution"`) {
		t.Fatalf("unexpected applied query: %+v", applied)
	}
}

func TestGovernanceModulePersistsStateWithRuntimeStore(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule(), staking.NewModule(), NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{Genesis: governanceTestGenesis("alice", 10)}); err != nil {
		t.Fatal(err)
	}
	finalizeGovernanceBlock(t, runtime, 1, []types.Tx{governanceTestDelegateTx("alice", "validator-1", 1)})
	finalizeGovernanceBlock(t, runtime, 2, []types.Tx{[]byte("governance:submit:alice:title:execution:max_gas:20000000")})
	finalizeGovernanceBlock(t, runtime, 3, []types.Tx{[]byte("governance:vote:1:alice:yes")})

	recovered, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule(), staking.NewModule(), NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	recovered.WithStore(storage)
	if _, err := recovered.InitChain(vexoapp.InitChainRequest{}); err != nil {
		t.Fatal(err)
	}
	proposal := recovered.Query(vexoapp.QueryRequest{Path: []string{"governance", "proposal", "1"}})
	if proposal.Code != 0 || !strings.Contains(string(proposal.Value), `"ID":1`) {
		t.Fatalf("expected persisted proposal, got %+v", proposal)
	}
	tally := recovered.Query(vexoapp.QueryRequest{Path: []string{"governance", "tally", "1"}})
	if tally.Code != 0 || !strings.Contains(string(tally.Value), `"Passed":true`) {
		t.Fatalf("expected persisted tally, got %+v", tally)
	}
}

func TestGovernanceModuleRebindsStoreAfterRecoverWithoutInitChain(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{}); err != nil {
		t.Fatal(err)
	}
	finalizeGovernanceBlock(t, runtime, 1, []types.Tx{[]byte("governance:submit:alice:title:execution:max_gas:20000000")})

	recovered, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	recovered.WithStore(storage)
	recovered.Restore(1, types.Hash{1})
	proposal := recovered.Query(vexoapp.QueryRequest{Path: []string{"governance", "proposal", "1"}})
	if proposal.Code != 0 || !strings.Contains(string(proposal.Value), `"ID":1`) {
		t.Fatalf("expected recover rebind proposal query, got %+v", proposal)
	}
}

func TestGovernanceModuleRejectsInvalidTransactions(t *testing.T) {
	module := NewModule()
	for _, tx := range []types.Tx{
		[]byte("governance:submit:alice:title:module:key"),
		[]byte("governance:vote:0:alice:yes:1"),
		[]byte("governance:vote:1:alice:maybe:1"),
		[]byte("governance:vote:1:alice:yes:0"),
		[]byte("governance:vote:1:alice:yes"),
		[]byte("governance:execute:0"),
		[]byte("bank:mint:alice:1"),
	} {
		result := module.DeliverTx(vexoapp.Context{Height: 1}, tx)
		if result.Code == 0 {
			t.Fatalf("expected tx %q to fail", tx)
		}
	}
}

func TestGovernanceModuleDerivesVotingPowerFromStakingState(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule(), staking.NewModule(), NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{Genesis: governanceTestGenesis("alice", 10)}); err != nil {
		t.Fatal(err)
	}
	finalizeGovernanceBlock(t, runtime, 1, []types.Tx{governanceTestDelegateTx("alice", "validator-1", 3)})
	finalizeGovernanceBlock(t, runtime, 2, []types.Tx{[]byte("governance:submit:alice:title:execution:max_gas:20000000")})
	finalizeGovernanceBlock(t, runtime, 3, []types.Tx{[]byte("governance:vote:1:alice:yes")})
	tally := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "tally", "1"}})
	if tally.Code != 0 || !strings.Contains(string(tally.Value), `"Yes":3`) {
		t.Fatalf("expected staking-derived voting power, got %+v", tally)
	}
	legacyPower := NewModule()
	result := legacyPower.DeliverTx(vexoapp.Context{Height: 1, Store: storage}, []byte("governance:vote:1:alice:yes:999"))
	if result.Code == 0 {
		t.Fatalf("expected store-backed vote with explicit power to fail")
	}
}

func TestGovernanceModuleQueryRejectsInvalidRequests(t *testing.T) {
	module := NewModule()
	for _, req := range []vexoapp.QueryRequest{
		{Path: nil},
		{Path: []string{"proposal", "0"}},
		{Path: []string{"proposal", "99"}},
		{Path: []string{"tally", "99"}},
		{Path: []string{"unknown"}},
	} {
		response := module.Query(vexoapp.Context{}, req)
		if response.Code == 0 {
			t.Fatalf("expected query to fail: %+v", req)
		}
	}
}

func TestGovernanceModulePropagatesStoreWriteErrors(t *testing.T) {
	expected := errors.New("store write failed")
	module := NewModuleWithKeeper(failingContextKeeper{
		InMemoryKeeper: vexogov.NewInMemoryKeeper(vexogov.TallyPolicy{}, nil),
		err:            expected,
	})
	if err := module.BeginBlock(vexoapp.Context{}, types.Header{Height: 1}); !errors.Is(err, expected) {
		t.Fatalf("expected BeginBlock write error, got %v", err)
	}
	result := module.DeliverTx(vexoapp.Context{Height: 1}, []byte("governance:submit:alice:title:execution:max_gas:20000000"))
	if result.Code == 0 || !strings.Contains(result.Log, expected.Error()) {
		t.Fatalf("expected DeliverTx write error, got %+v", result)
	}
}

func TestGovernanceModulePropagatesStoreQueryErrors(t *testing.T) {
	expected := errors.New("store read failed")
	module := NewModuleWithKeeper(failingContextKeeper{
		InMemoryKeeper: vexogov.NewInMemoryKeeper(vexogov.TallyPolicy{}, nil),
		err:            expected,
	})
	for _, req := range []vexoapp.QueryRequest{
		{Path: []string{"proposal", "1"}},
		{Path: []string{"tally", "1"}},
		{Path: []string{"applied"}},
	} {
		response := module.Query(vexoapp.Context{}, req)
		if response.Code == 0 || !strings.Contains(response.Log, expected.Error()) {
			t.Fatalf("expected query error for %+v, got %+v", req, response)
		}
	}
}

func TestGovernanceModuleUsesCallerContext(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	if err := module.InitGenesis(vexoapp.Context{Store: storage}, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := module.DeliverTx(vexoapp.Context{Ctx: ctx, Height: 1, Store: storage}, []byte("governance:submit:alice:title:execution:max_gas:20000000"))
	if result.Code == 0 || !strings.Contains(result.Log, context.Canceled.Error()) {
		t.Fatalf("expected canceled context to abort governance tx, got %+v", result)
	}
}

func TestGovernanceCLICommands(t *testing.T) {
	command := governanceCLICommand()
	if command.Name != ModuleName || len(command.Children) != 2 {
		t.Fatalf("unexpected governance command: %+v", command)
	}

	cases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "submit transaction",
			args:     []string{"tx", "submit", "alice", "max-gas", "execution", "max_gas", "20000000"},
			expected: "tx: governance:submit:alice:max-gas:execution:max_gas:20000000",
		},
		{
			name:     "vote transaction",
			args:     []string{"tx", "vote", "1", "alice", "yes"},
			expected: "tx: governance:vote:1:alice:yes",
		},
		{
			name:     "vote transaction with execution tags",
			args:     []string{"tx", "vote", "1", "alice", "yes", "--fee", "1", "--gas", "1000"},
			expected: "tx: governance:vote:1:alice:yes:fee=1:gas=1000",
		},
		{
			name:     "execute transaction",
			args:     []string{"tx", "execute", "1"},
			expected: "tx: governance:execute:1",
		},
		{
			name:     "proposal query",
			args:     []string{"query", "proposal", "1"},
			expected: "query_path: governance/proposal/1",
		},
		{
			name:     "tally query",
			args:     []string{"query", "tally", "1"},
			expected: "query_path: governance/tally/1",
		},
		{
			name:     "applied query",
			args:     []string{"query", "applied"},
			expected: "query_path: governance/applied",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := command.Execute(&output, tc.args); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(output.String()) != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, output.String())
			}
		})
	}
}

func governanceTestGenesis(address types.Address, balance uint64) vexoapp.GenesisState {
	return vexoapp.GenesisState{"bank:" + string(address): []byte(strconv.FormatUint(balance, 10))}
}

func governanceTestDelegateTx(delegator types.Address, validatorID types.ValidatorID, amount uint64) types.Tx {
	return []byte("staking:delegate:" + string(delegator) + ":" + string(validatorID) + ":" + strconv.FormatUint(amount, 10) + ":cHVibGljLWtleQ==")
}

func TestGovernanceQueryJSONIsValid(t *testing.T) {
	module := NewModule()
	result := module.DeliverTx(vexoapp.Context{Height: 1}, []byte("governance:submit:alice:title:module:key:value"))
	if result.Code != 0 {
		t.Fatalf("unexpected submit result: %+v", result)
	}
	response := module.Query(vexoapp.Context{}, vexoapp.QueryRequest{Path: []string{"proposal", "1"}})
	if response.Code != 0 {
		t.Fatalf("unexpected proposal query: %+v", response)
	}
	var decoded map[string]any
	if err := json.Unmarshal(response.Value, &decoded); err != nil {
		t.Fatal(err)
	}
}

func finalizeGovernanceBlock(t *testing.T, runtime *vexoapp.Runtime, height types.Height, txs []types.Tx) {
	t.Helper()
	_, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt("vexo-test", height)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

type failingContextKeeper struct {
	*vexogov.InMemoryKeeper
	err error
}

func (keeper failingContextKeeper) SetTimeContext(ctx context.Context, now uint64) error {
	return keeper.err
}

func (keeper failingContextKeeper) SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error {
	return keeper.err
}

func (keeper failingContextKeeper) ProposalContext(ctx context.Context, proposalID uint64) (vexogov.ProposalState, bool, error) {
	return vexogov.ProposalState{}, false, keeper.err
}

func (keeper failingContextKeeper) AppliedChangesContext(ctx context.Context) ([]vexogov.ParameterChange, error) {
	return nil, keeper.err
}

func (keeper failingContextKeeper) TallyContext(ctx context.Context, proposalID uint64) (vexogov.TallyResult, bool, error) {
	return vexogov.TallyResult{}, false, keeper.err
}

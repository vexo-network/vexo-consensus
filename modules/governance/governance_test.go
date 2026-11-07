package governance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestGovernanceModuleLifecycle(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 60)
	chain.delegate(1, "bob", "validator-1", 40)

	id := chain.submit(3, "alice")
	if id != 1 {
		t.Fatalf("expected the first proposal to have id 1, got %d", id)
	}
	for _, voter := range []string{"alice", "bob"} {
		if result := chain.vote(4, id, voter, "yes"); result.Code != 0 {
			t.Fatalf("unexpected vote result for %s: %+v", voter, result)
		}
	}
	tally := chain.tally(id)
	if tally.Yes != 100 || tally.Total != 100 || !tally.Passed {
		t.Fatalf("unexpected tally: %+v", tally)
	}

	if result := chain.execute(8, id); result.Code == 0 {
		t.Fatal("expected execution to wait for the end of the voting period")
	}
	if result := chain.execute(9, id); result.Code == 0 {
		t.Fatal("expected execution to wait for the timelock")
	}
	if result := chain.execute(10, id); result.Code != 0 {
		t.Fatalf("unexpected execute result: %+v", result)
	}
	if state := chain.proposal(id); !state.Executed || state.SnapshotHeight != 3 || state.SnapshotPower != 100 {
		t.Fatalf("unexpected proposal state: %+v", state)
	}
	applied := chain.query("applied")
	if applied.Code != 0 || !strings.Contains(string(applied.Value), `"Module":"execution"`) {
		t.Fatalf("unexpected applied query: %+v", applied)
	}
	if result := chain.execute(11, id); result.Code == 0 {
		t.Fatal("expected a second execution to fail")
	}
}

func TestGovernanceModuleLifecycleThroughRuntime(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{bank.NewModule(), staking.NewModule(), NewModuleWithPolicy(testPolicy())}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)
	genesis := vexoapp.GenesisState{"bank:alice": []byte("100"), "bank:bob": []byte("50")}
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{Genesis: genesis}); err != nil {
		t.Fatal(err)
	}
	mustSucceed := func(results []types.Result) {
		t.Helper()
		for _, result := range results {
			if result.Code != 0 {
				t.Fatalf("unexpected result %+v", result)
			}
		}
	}

	mustSucceed(finalizeGovernanceBlock(t, runtime, 1,
		stakingPayload("alice", "create-validator", "alice", "validator-1", "60", keyArg("key-validator-1")),
		stakingPayload("bob", "delegate", "bob", "validator-1", "40"),
	))
	mustSucceed(finalizeGovernanceBlock(t, runtime, 2, govTx("alice", "submit", "alice", "max-gas", "execution", "max_gas", "20000000")))
	mustSucceed(finalizeGovernanceBlock(t, runtime, 3, govTx("alice", "vote", "1", "alice", "yes")))
	tally := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "tally", "1"}})
	if tally.Code != 0 || !strings.Contains(string(tally.Value), `"Passed":true`) || !strings.Contains(string(tally.Value), `"Yes":60`) {
		t.Fatalf("unexpected tally query: %+v", tally)
	}
	for height := types.Height(4); height < 9; height++ {
		finalizeGovernanceBlock(t, runtime, height)
	}
	mustSucceed(finalizeGovernanceBlock(t, runtime, 9, types.Tx("governance:execute:1")))

	proposal := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "proposal", "1"}})
	if proposal.Code != 0 || !strings.Contains(string(proposal.Value), `"Executed":true`) {
		t.Fatalf("unexpected proposal query: %+v", proposal)
	}
	applied := runtime.Query(vexoapp.QueryRequest{Path: []string{"governance", "applied"}})
	if applied.Code != 0 || !strings.Contains(string(applied.Value), `"Module":"execution"`) {
		t.Fatalf("unexpected applied query: %+v", applied)
	}
}

func TestGovernanceModuleStateLivesInTheContextStore(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	chain.submit(2, "alice")

	recovered := NewModuleWithPolicy(testPolicy())
	response := recovered.Query(vexoapp.Context{Store: chain.store}, vexoapp.QueryRequest{Path: []string{"proposal", "1"}})
	if response.Code != 0 || !strings.Contains(string(response.Value), `"ID":1`) {
		t.Fatalf("expected a module instance to read the proposal from the store it is given, got %+v", response)
	}

	empty, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	if response := recovered.Query(vexoapp.Context{Store: empty}, vexoapp.QueryRequest{Path: []string{"proposal", "1"}}); response.Code == 0 {
		t.Fatalf("expected no proposal in another store, got %+v", response)
	}
	if response := recovered.Query(vexoapp.Context{Store: empty}, vexoapp.QueryRequest{Path: []string{"applied"}}); response.Code != 0 || string(response.Value) != "[]" && string(response.Value) != "null" {
		t.Fatalf("expected an empty applied list from an empty store, got %+v", response)
	}
	if _, err := empty.Get(context.Background(), "governance", []byte("next_id")); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected a query to leave the store untouched, got %v", err)
	}
}

func TestGovernanceModuleRejectsInvalidTransactions(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	for _, tx := range []types.Tx{
		govTx("alice", "submit", "alice", "title", "module", "key"),
		govTx("alice", "submit", "alice", "", "module", "key", "value"),
		govTx("alice", "vote", "0", "alice", "yes"),
		govTx("alice", "vote", "1", "alice", "yes", "1"),
		govTx("alice", "vote", "1", "alice"),
		govTx("alice", "vote", "1", "alice", "yes", "1", "1"),
		types.Tx("governance:execute:0"),
		types.Tx("governance:execute"),
		types.Tx("governance"),
		types.Tx("bank:mint:alice:1"),
	} {
		if result := chain.deliver(2, tx); result.Code == 0 {
			t.Fatalf("expected tx %q to fail", tx)
		}
	}
	chain.submit(2, "alice")
	if result := chain.vote(3, 1, "alice", "maybe"); result.Code == 0 {
		t.Fatal("expected an unknown vote option to fail")
	}
	if result := chain.vote(3, 99, "alice", "yes"); result.Code == 0 {
		t.Fatal("expected a vote on an unknown proposal to fail")
	}
}

func TestGovernanceModuleSubmitsMultiChangeJSONProposal(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.fund("alice", 100)
	payload := `{"submitter":"alice","title":"multi-change","description":"raise limits","metadata_uri":"ipfs://proposal","type":"parameter_change","deposit":"100avxo","changes":[{"module":"execution","key":"max_gas","value":"20000000"},{"module":"mempool","key":"max_txs","value":"50000"}]}`
	result := chain.deliver(1, governanceSubmitJSONTx("alice", payload))
	if result.Code != 0 {
		t.Fatalf("unexpected submit-json result: %+v", result)
	}
	response := chain.query("proposal", "1")
	if response.Code != 0 ||
		!strings.Contains(string(response.Value), `"max_txs"`) ||
		!strings.Contains(string(response.Value), `"max_gas"`) ||
		!strings.Contains(string(response.Value), `raise limits`) ||
		!strings.Contains(string(response.Value), `metadata_uri=ipfs://proposal`) ||
		!strings.Contains(string(response.Value), `deposit=100avxo`) {
		t.Fatalf("expected multi-change proposal query, got %+v", response)
	}
}

func TestGovernanceModuleDerivesVotingPowerFromStakingState(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 3)
	id := chain.submit(2, "alice")
	if result := chain.vote(3, id, "alice", "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	if tally := chain.tally(id); tally.Yes != 3 {
		t.Fatalf("expected staking-derived voting power, got %+v", tally)
	}
	if result := chain.deliver(3, govTx("alice", "vote", "1", "alice", "yes", "999")); result.Code == 0 {
		t.Fatal("expected a vote with a self-declared power to fail")
	}
}

func TestGovernanceModuleEscrowsAndRefundsProposalDeposit(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 1)
	chain.fund("alice", 9)
	payload := `{"submitter":"alice","title":"deposit","type":"parameter_change","deposit":"3avxo","changes":[{"module":"execution","key":"max_gas","value":"20000000"}]}`
	chain.mustDeliver(2, governanceSubmitJSONTx("alice", payload))
	if balance, escrow := chain.balance("alice"), chain.balance(governanceEscrowAddress); balance != "6" || escrow != "3" {
		t.Fatalf("expected 6 left and 3 escrowed, got %s and %s", balance, escrow)
	}

	if result := chain.vote(3, 1, "alice", "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	chain.mustDeliver(9, types.Tx("governance:execute:1"))
	if balance, escrow := chain.balance("alice"), chain.balance(governanceEscrowAddress); balance != "9" || escrow != "0" {
		t.Fatalf("expected the deposit back (9 and 0), got %s and %s", balance, escrow)
	}
}

func TestGovernanceModuleRequiresConfiguredProposalDeposit(t *testing.T) {
	policy := testPolicy()
	policy.RequireDeposit = true
	policy.MinDeposit = "3avxo"
	policy.DepositDenom = "avxo"
	chain := newTestChain(t, policy)
	chain.validator(1, "alice", "validator-1", 1)
	chain.fund("alice", 9)

	if result := chain.deliver(2, govTx("alice", "submit", "alice", "no-deposit", "execution", "max_gas", "20000000")); result.Code == 0 {
		t.Fatal("expected proposal without required deposit to fail")
	}
	if result := chain.deliver(2, govTx("alice", "submit", "alice", "too-low", "execution", "max_gas", "20000000", "2avxo")); result.Code == 0 {
		t.Fatal("expected proposal below minimum deposit to fail")
	}
	chain.mustDeliver(2, govTx("alice", "submit", "alice", "with-deposit", "execution", "max_gas", "20000000", "3avxo"))
	if balance, escrow := chain.balance("alice"), chain.balance(governanceEscrowAddress); balance != "6" || escrow != "3" {
		t.Fatalf("expected 6 left and 3 escrowed, got %s and %s", balance, escrow)
	}
}

func TestGovernanceModuleSlashesRejectedProposalDeposit(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 1)
	chain.fund("alice", 9)
	payload := `{"submitter":"alice","title":"reject","type":"parameter_change","deposit":"3avxo","changes":[{"module":"execution","key":"max_gas","value":"20000000"}]}`
	chain.mustDeliver(2, governanceSubmitJSONTx("alice", payload))
	if result := chain.vote(3, 1, "alice", "no"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}

	result := chain.execute(9, 1)
	if result.Code != 0 || string(result.Data) != "proposal_rejected" {
		t.Fatalf("expected a committed rejection, got %+v", result)
	}
	if alice, escrow, rejected := chain.balance("alice"), chain.balance(governanceEscrowAddress), chain.balance(governanceRejectedDepositAddress); alice != "6" || escrow != "0" || rejected != "3" {
		t.Fatalf("expected the deposit forfeited (6, 0, 3), got %s, %s, %s", alice, escrow, rejected)
	}
	if result := chain.execute(10, 1); result.Code == 0 {
		t.Fatal("expected a second execution of a rejected proposal to fail")
	}
	if rejected := chain.balance(governanceRejectedDepositAddress); rejected != "3" {
		t.Fatalf("expected the forfeited deposit to stay 3, got %s", rejected)
	}
	if state := chain.proposal(1); !state.Rejected || state.Executed {
		t.Fatalf("expected a rejected proposal, got %+v", state)
	}
}

func TestGovernanceModuleQueryRejectsInvalidRequests(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	for _, path := range [][]string{
		nil,
		{"proposal", "0"},
		{"proposal", "99"},
		{"tally", "99"},
		{"unknown"},
	} {
		if response := chain.query(path...); response.Code == 0 {
			t.Fatalf("expected query to fail: %v", path)
		}
	}
	if response := NewModule().Query(vexoapp.Context{}, vexoapp.QueryRequest{Path: []string{"applied"}}); response.Code == 0 {
		t.Fatal("expected a query without a store to fail")
	}
}

type failingStore struct {
	*store.LevelDBStore
	err error
}

func (storage failingStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	return nil, storage.err
}

func (storage failingStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	return storage.err
}

func TestGovernanceModulePropagatesStoreErrors(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	expected := errors.New("store failed")
	failing := failingStore{LevelDBStore: chain.store, err: expected}
	module := NewModuleWithPolicy(testPolicy())

	result := module.DeliverTx(vexoapp.Context{Height: 1, Store: failing}, govTx("alice", "submit", "alice", "title", "execution", "max_gas", "20000000"))
	if result.Code == 0 || !strings.Contains(result.Log, expected.Error()) {
		t.Fatalf("expected DeliverTx store error, got %+v", result)
	}
	for _, path := range [][]string{{"proposal", "1"}, {"tally", "1"}, {"applied"}} {
		response := module.Query(vexoapp.Context{Store: failing}, vexoapp.QueryRequest{Path: path})
		if response.Code == 0 || !strings.Contains(response.Log, expected.Error()) {
			t.Fatalf("expected query error for %v, got %+v", path, response)
		}
	}
}

func TestGovernanceModuleUsesCallerContext(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := chain.governance.DeliverTx(vexoapp.Context{Ctx: ctx, Height: 2, Store: chain.store}, govTx("alice", "submit", "alice", "title", "execution", "max_gas", "20000000"))
	if result.Code == 0 || !strings.Contains(result.Log, context.Canceled.Error()) {
		t.Fatalf("expected canceled context to abort governance tx, got %+v", result)
	}
}

func TestGovernanceModuleRequiresAStore(t *testing.T) {
	result := NewModule().DeliverTx(vexoapp.Context{Height: 1}, govTx("alice", "submit", "alice", "title", "execution", "max_gas", "20000000"))
	if result.Code == 0 || !strings.Contains(result.Log, ErrGovernanceRequired.Error()) {
		t.Fatalf("expected a transaction without a store to fail, got %+v", result)
	}
}

func TestGovernanceModuleEstimatesGas(t *testing.T) {
	module := NewModule()
	for tx, expected := range map[string]uint64{
		"governance:submit:alice:t:m:k:v":                  submitGasCost,
		"governance:submit:alice:t:m:k:v:5avxo:signer=bob": submitGasCost,
		"governance:submit-json:e30":                       submitGasCost,
		"governance:vote:1:alice:yes:signer=alice":         voteGasCost,
		"governance:execute:1":                             executeGasCost,
		"governance:execute:1:fee=1:gas=100:signer=alice":  executeGasCost,
	} {
		gas, err := module.EstimateGas(vexoapp.Context{}, types.Tx(tx))
		if err != nil || gas != expected {
			t.Fatalf("%s: expected gas %d, got %d (%v)", tx, expected, gas, err)
		}
	}
	for _, tx := range []string{"governance:vote:1:alice:yes:5", "governance:unknown:1", "governance"} {
		if _, err := module.EstimateGas(vexoapp.Context{}, types.Tx(tx)); err == nil {
			t.Fatalf("expected gas estimation of %q to fail", tx)
		}
	}
}

func TestGovernanceQueryJSONIsValid(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	chain.submit(2, "alice")
	response := chain.query("proposal", "1")
	if response.Code != 0 {
		t.Fatalf("unexpected proposal query: %+v", response)
	}
	var decoded map[string]any
	if err := json.Unmarshal(response.Value, &decoded); err != nil {
		t.Fatal(err)
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
			name:     "submit json transaction",
			args:     []string{"tx", "submit-json", `{"submitter":"alice","title":"multi","changes":[{"module":"execution","key":"max_gas","value":"20000000"}]}`},
			expected: "tx: governance:submit-json:eyJzdWJtaXR0ZXIiOiJhbGljZSIsInRpdGxlIjoibXVsdGkiLCJjaGFuZ2VzIjpbeyJtb2R1bGUiOiJleGVjdXRpb24iLCJrZXkiOiJtYXhfZ2FzIiwidmFsdWUiOiIyMDAwMDAwMCJ9XX0",
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

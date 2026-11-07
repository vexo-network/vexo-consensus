package governance

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// testPolicy has a voting period of 5 blocks and a timelock of 2: a proposal submitted at height h takes
// votes up to and including h+5 and can be executed from h+7.
func testPolicy() vexogov.TallyPolicy {
	return vexogov.TallyPolicy{QuorumBPS: 3340, YesThresholdBPS: 5000, VetoBPS: 3340, VotingPeriod: 5, Timelock: 2}
}

// govTx builds a governance payload as the ante handler delivers it: positional arguments followed by
// the authenticated signer tag.
func govTx(signer string, parts ...string) types.Tx {
	return types.Tx("governance:" + strings.Join(parts, ":") + ":signer=" + signer)
}

func stakingPayload(signer string, parts ...string) types.Tx {
	return types.Tx("staking:" + strings.Join(parts, ":") + ":signer=" + signer)
}

func keyArg(label string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(label))
}

// testChain is a governance module and a staking module over one store, driven at explicit heights.
type testChain struct {
	t          *testing.T
	store      *store.LevelDBStore
	staking    *staking.Module
	governance *Module
}

func newTestChain(t *testing.T, policy vexogov.TallyPolicy) *testChain {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	return &testChain{t: t, store: storage, staking: staking.NewModule(), governance: NewModuleWithPolicy(policy)}
}

func (chain *testChain) fund(address types.Address, amount uint64) {
	chain.t.Helper()
	if err := bank.MintBig(context.Background(), chain.store, address, new(big.Int).SetUint64(amount)); err != nil {
		chain.t.Fatal(err)
	}
}

func (chain *testChain) balance(address types.Address) string {
	chain.t.Helper()
	balance, err := bank.BalanceBig(context.Background(), chain.store, address)
	if err != nil {
		chain.t.Fatal(err)
	}
	return balance.String()
}

func (chain *testChain) stakingTx(height types.Height, tx types.Tx) {
	chain.t.Helper()
	result := chain.staking.DeliverTx(vexoapp.Context{Height: height, Store: chain.store}, tx)
	if result.Code != 0 {
		chain.t.Fatalf("staking tx %q failed: %+v", tx, result)
	}
}

// validator registers validatorID for operator with a self-delegation of stake.
func (chain *testChain) validator(height types.Height, operator string, validatorID string, stake uint64) {
	chain.t.Helper()
	chain.fund(types.Address(operator), stake)
	chain.stakingTx(height, stakingPayload(operator, "create-validator", operator, validatorID, strconv.FormatUint(stake, 10), keyArg("key-"+validatorID)))
}

// delegate funds delegator and bonds amount to validatorID at height.
func (chain *testChain) delegate(height types.Height, delegator string, validatorID string, amount uint64) {
	chain.t.Helper()
	chain.fund(types.Address(delegator), amount)
	chain.stakingTx(height, stakingPayload(delegator, "delegate", delegator, validatorID, strconv.FormatUint(amount, 10)))
}

func (chain *testChain) deliver(height types.Height, tx types.Tx) types.Result {
	return chain.governance.DeliverTx(vexoapp.Context{Height: height, Store: chain.store}, tx)
}

func (chain *testChain) mustDeliver(height types.Height, tx types.Tx) types.Result {
	chain.t.Helper()
	result := chain.deliver(height, tx)
	if result.Code != 0 {
		chain.t.Fatalf("governance tx %q failed: %+v", tx, result)
	}
	return result
}

// submit opens a parameter-change proposal as submitter and returns its id.
func (chain *testChain) submit(height types.Height, submitter string, deposit ...string) uint64 {
	chain.t.Helper()
	parts := []string{"submit", submitter, "max-gas", "execution", "max_gas", "20000000"}
	parts = append(parts, deposit...)
	result := chain.mustDeliver(height, govTx(submitter, parts...))
	id, err := strconv.ParseUint(string(result.Data), 10, 64)
	if err != nil {
		chain.t.Fatalf("unexpected submit result %+v", result)
	}
	return id
}

func (chain *testChain) vote(height types.Height, id uint64, voter string, option string) types.Result {
	return chain.deliver(height, govTx(voter, "vote", strconv.FormatUint(id, 10), voter, option))
}

func (chain *testChain) execute(height types.Height, id uint64) types.Result {
	return chain.deliver(height, types.Tx("governance:execute:"+strconv.FormatUint(id, 10)))
}

func (chain *testChain) query(path ...string) vexoapp.QueryResponse {
	return chain.governance.Query(vexoapp.Context{Store: chain.store}, vexoapp.QueryRequest{Path: path})
}

func (chain *testChain) tally(id uint64) vexogov.TallyResult {
	chain.t.Helper()
	response := chain.query("tally", strconv.FormatUint(id, 10))
	if response.Code != 0 {
		chain.t.Fatalf("tally query failed: %+v", response)
	}
	var tally vexogov.TallyResult
	if err := json.Unmarshal(response.Value, &tally); err != nil {
		chain.t.Fatal(err)
	}
	return tally
}

func (chain *testChain) proposal(id uint64) vexogov.ProposalState {
	chain.t.Helper()
	response := chain.query("proposal", strconv.FormatUint(id, 10))
	if response.Code != 0 {
		chain.t.Fatalf("proposal query failed: %+v", response)
	}
	var view proposalView
	if err := json.Unmarshal(response.Value, &view); err != nil {
		chain.t.Fatal(err)
	}
	return view.State
}

func governanceSubmitJSONTx(signer string, payload string) types.Tx {
	return types.Tx("governance:submit-json:" + base64.RawStdEncoding.EncodeToString([]byte(payload)) + ":signer=" + signer)
}

func finalizeGovernanceBlock(t *testing.T, runtime *vexoapp.Runtime, height types.Height, txs ...types.Tx) []types.Result {
	t.Helper()
	response, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    txs,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return response.Results
}

package governance

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

// Anyone could submit a proposal in the name of another account and have that account's deposit moved
// into escrow, because the submitter named in the arguments was never compared to the signer.
func TestSubmitRequiresTheSubmitterToSign(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	chain.fund("victim", 100)

	forged := map[string]types.Tx{
		"signed by another account": govTx("mallory", "submit", "victim", "drain", "execution", "max_gas", "1", "50avxo"),
		"no signer":                 types.Tx("governance:submit:victim:drain:execution:max_gas:1:50avxo"),
		"json signed by another": governanceSubmitJSONTx("mallory",
			`{"submitter":"victim","title":"drain","deposit":"50avxo","changes":[{"module":"execution","key":"max_gas","value":"1"}]}`),
		"json without signer": types.Tx("governance:submit-json:" + keyArg(`{"submitter":"victim","title":"drain","deposit":"50avxo","changes":[{"module":"execution","key":"max_gas","value":"1"}]}`)),
	}
	for name, tx := range forged {
		if result := chain.deliver(2, tx); result.Code != codeUnauthorized {
			t.Fatalf("%s: expected an unauthorized result, got %+v", name, result)
		}
	}
	if balance, escrow := chain.balance("victim"), chain.balance(governanceEscrowAddress); balance != "100" || escrow != "0" {
		t.Fatalf("expected the victim's balance untouched, got %s with %s escrowed", balance, escrow)
	}
	if response := chain.query("proposal", "1"); response.Code == 0 {
		t.Fatalf("expected no proposal to be created, got %+v", response)
	}

	chain.mustDeliver(2, govTx("victim", "submit", "victim", "own", "execution", "max_gas", "1", "50avxo"))
	if balance, escrow := chain.balance("victim"), chain.balance(governanceEscrowAddress); balance != "50" || escrow != "50" {
		t.Fatalf("expected the victim's own deposit escrowed, got %s with %s escrowed", balance, escrow)
	}
}

// A vote named any voter, so anyone could vote with the power of every delegator.
func TestVoteRequiresTheVoterToSign(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 60)
	chain.delegate(1, "bob", "validator-1", 40)
	id := chain.submit(3, "alice")

	for name, tx := range map[string]types.Tx{
		"signed by another account": govTx("mallory", "vote", "1", "bob", "veto"),
		"no signer":                 types.Tx("governance:vote:1:bob:veto"),
	} {
		if result := chain.deliver(4, tx); result.Code != codeUnauthorized {
			t.Fatalf("%s: expected an unauthorized result, got %+v", name, result)
		}
	}
	if tally := chain.tally(id); tally.Total != 0 {
		t.Fatalf("expected no vote to be recorded, got %+v", tally)
	}
	if result := chain.vote(4, id, "bob", "yes"); result.Code != 0 {
		t.Fatalf("expected the voter to vote for themselves, got %+v", result)
	}
	if tally := chain.tally(id); tally.Yes != 40 || tally.Veto != 0 {
		t.Fatalf("unexpected tally %+v", tally)
	}
}

// Votes were keyed by the spelling of the voter, so one account could vote once per spelling of its
// address.
func TestVoteSpellingsOfOneAddressAreOneVoter(t *testing.T) {
	const upper = "0x00000000000000000000000000000000000000AB"
	const lower = "0x00000000000000000000000000000000000000ab"
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 60)
	chain.delegate(1, upper, "validator-1", 40)
	id := chain.submit(3, "alice")

	if result := chain.vote(4, id, upper, "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	if result := chain.vote(4, id, lower, "yes"); result.Code == 0 || !strings.Contains(result.Log, vexogov.ErrDuplicateVote.Error()) {
		t.Fatalf("expected the second spelling to be a duplicate vote, got %+v", result)
	}
	if tally := chain.tally(id); tally.Yes != 40 {
		t.Fatalf("expected the voter counted once, got %+v", tally)
	}
}

// Votes were accepted at any height, so a result could be changed after the voting period ended.
func TestLateVoteIsRefused(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 60)
	chain.delegate(1, "bob", "validator-1", 40)
	id := chain.submit(3, "alice")

	if result := chain.vote(8, id, "alice", "yes"); result.Code != 0 {
		t.Fatalf("expected a vote in the last block of the voting period, got %+v", result)
	}
	if result := chain.vote(9, id, "bob", "veto"); result.Code == 0 || !strings.Contains(result.Log, vexogov.ErrVotingPeriodClosed.Error()) {
		t.Fatalf("expected a late vote to be refused, got %+v", result)
	}
	if tally := chain.tally(id); tally.Total != 60 || tally.Veto != 0 {
		t.Fatalf("expected the late vote not to count, got %+v", tally)
	}
}

// Votes were accepted on proposals that were already executed or rejected, so a published result could
// be rewritten afterwards.
func TestVoteOnDecidedProposalIsRefused(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 60)
	chain.delegate(1, "bob", "validator-1", 40)

	passed := chain.submit(3, "alice")
	if result := chain.vote(4, passed, "alice", "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	chain.mustDeliver(10, types.Tx("governance:execute:1"))
	if result := chain.vote(10, passed, "bob", "veto"); result.Code == 0 || !strings.Contains(result.Log, vexogov.ErrProposalExecuted.Error()) {
		t.Fatalf("expected a vote on an executed proposal to be refused, got %+v", result)
	}
	if tally := chain.tally(passed); tally.Veto != 0 || !tally.Passed {
		t.Fatalf("expected the executed result to stay as it was, got %+v", tally)
	}

	rejected := chain.submit(11, "alice")
	if result := chain.execute(18, rejected); result.Code != 0 || string(result.Data) != "proposal_rejected" {
		t.Fatalf("expected a rejection, got %+v", result)
	}
	if result := chain.vote(18, rejected, "alice", "yes"); result.Code == 0 || !strings.Contains(result.Log, vexogov.ErrProposalRejected.Error()) {
		t.Fatalf("expected a vote on a rejected proposal to be refused, got %+v", result)
	}
}

// Stake delegated after a proposal was submitted carried full voting power, so a flash delegation
// could decide any open proposal.
func TestStakeDelegatedAfterSubmissionCannotVote(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 100)
	chain.delegate(3, "early-attacker", "validator-1", 1_000_000)
	id := chain.submit(3, "alice")
	chain.delegate(4, "attacker", "validator-1", 1_000_000)

	for _, attacker := range []string{"early-attacker", "attacker"} {
		if result := chain.vote(5, id, attacker, "veto"); result.Code == 0 || !strings.Contains(result.Log, ErrNoGovernanceVotingPower.Error()) {
			t.Fatalf("%s: expected stake delegated at or after the snapshot not to vote, got %+v", attacker, result)
		}
	}
	if tally := chain.tally(id); tally.Total != 0 {
		t.Fatalf("expected no vote to be recorded, got %+v", tally)
	}
	if result := chain.vote(5, id, "alice", "yes"); result.Code != 0 {
		t.Fatalf("expected a long-standing delegator to vote, got %+v", result)
	}
	if tally := chain.tally(id); tally.Yes != 100 || tally.Veto != 0 {
		t.Fatalf("unexpected tally %+v", tally)
	}
}

// Thresholds were absolute numbers, so a single unit of stake could pass a proposal on a chain with
// millions of bonded units.
func TestThresholdsScaleWithBondedPower(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "whale", "validator-1", 1_000_000)
	chain.delegate(1, "dust", "validator-1", 1)

	dustOnly := chain.submit(3, "whale")
	if result := chain.vote(4, dustOnly, "dust", "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	if tally := chain.tally(dustOnly); tally.Passed {
		t.Fatalf("expected one unit of stake not to pass a proposal, got %+v", tally)
	}
	if result := chain.execute(10, dustOnly); result.Code != 0 || string(result.Data) != "proposal_rejected" {
		t.Fatalf("expected the proposal to be rejected, got %+v", result)
	}

	backed := chain.submit(11, "whale")
	if result := chain.vote(12, backed, "whale", "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	if result := chain.execute(18, backed); result.Code != 0 || len(result.Data) != 0 {
		t.Fatalf("expected the proposal to execute, got %+v", result)
	}
	if state := chain.proposal(backed); !state.Executed {
		t.Fatalf("expected the proposal to be executed, got %+v", state)
	}
}

// The thresholds are a share of the power bonded when the proposal was submitted: stake that appears
// later neither blocks nor passes the proposal.
func TestProposalSnapshotsBondedPower(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 100)
	id := chain.submit(3, "alice")
	chain.delegate(4, "late", "validator-1", 900)

	if result := chain.vote(5, id, "alice", "yes"); result.Code != 0 {
		t.Fatalf("unexpected vote result: %+v", result)
	}
	state := chain.proposal(id)
	if state.SnapshotPower != 100 || state.SnapshotHeight != 3 {
		t.Fatalf("expected the snapshot of the submission block, got %+v", state)
	}
	if tally := chain.tally(id); !tally.Passed {
		t.Fatalf("expected the whole snapshotted power to pass the proposal, got %+v", tally)
	}
}

// The governance keeper was bound to the base store when the module was created, so the writes of a
// transaction - including an upgrade plan - skipped the transaction's staged layer and survived a
// failed transaction or a discarded block.
func TestGovernanceWritesGoThroughTheContextStore(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 10)
	staged := vexoapp.NewStagedStore(chain.store)
	module := chain.governance
	ctx := context.Background()

	deliver := func(height types.Height, tx types.Tx) types.Result {
		return module.DeliverTx(vexoapp.Context{Height: height, Store: staged}, tx)
	}
	plan := upgrade.Plan{
		Name: "v2", Height: 50, BinaryVersion: "v2.0.0",
		ConfigSchemaFrom: 1, ConfigSchemaTo: 2, StoreSchemaFrom: 1, StoreSchemaTo: 2, AppStateSchemaFrom: 1, AppStateSchemaTo: 2,
	}
	encodedPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"submitter": "alice",
		"title":     "upgrade",
		"changes":   []map[string]string{{"module": "upgrade", "key": "plan", "value": string(encodedPlan)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		height types.Height
		tx     types.Tx
	}{
		{2, governanceSubmitJSONTx("alice", string(payload))},
		{3, govTx("alice", "vote", "1", "alice", "yes")},
		{9, types.Tx("governance:execute:1")},
	} {
		if result := deliver(step.height, step.tx); result.Code != 0 {
			t.Fatalf("unexpected result for %q: %+v", step.tx, result)
		}
	}

	if _, err := chain.store.Get(ctx, "governance", []byte("next_id")); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected the base store to stay untouched until the layer commits, got %v", err)
	}
	if _, found, err := vexogov.UpgradePlanAt(ctx, chain.store, plan.Height); err != nil || found {
		t.Fatalf("expected no upgrade plan in the base store, found=%t err=%v", found, err)
	}
	stored, found, err := vexogov.UpgradePlanAt(ctx, staged, plan.Height)
	if err != nil || !found || stored != plan {
		t.Fatalf("expected the plan in the transaction's layer, found=%t plan=%+v err=%v", found, stored, err)
	}

	if err := staged.ApplyTo(ctx, chain.store); err != nil {
		t.Fatal(err)
	}
	stored, found, err = vexogov.UpgradePlanAt(ctx, chain.store, plan.Height)
	if err != nil || !found || stored != plan {
		t.Fatalf("expected the plan in the base store once the layer commits, found=%t plan=%+v err=%v", found, stored, err)
	}
}

func TestSubmitRefusesOversizedProposals(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)

	longTitle := strings.Repeat("t", vexogov.MaxProposalTitleBytes+1)
	if result := chain.deliver(2, govTx("alice", "submit", "alice", longTitle, "execution", "max_gas", "1")); result.Code == 0 {
		t.Fatal("expected a title above the limit to be refused")
	}
	changes := make([]map[string]string, vexogov.MaxProposalChanges+1)
	for index := range changes {
		changes[index] = map[string]string{"module": "execution", "key": "max_gas", "value": strconv.Itoa(index)}
	}
	payload, err := json.Marshal(map[string]any{"submitter": "alice", "title": "many", "changes": changes})
	if err != nil {
		t.Fatal(err)
	}
	if result := chain.deliver(2, governanceSubmitJSONTx("alice", string(payload))); result.Code == 0 {
		t.Fatal("expected a proposal with too many changes to be refused")
	}
	if response := chain.query("proposal", "1"); response.Code == 0 {
		t.Fatalf("expected no proposal to be stored, got %+v", response)
	}
}

func TestSubmitRefusesUpgradePlansThatCannotRun(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	due := upgrade.Plan{
		Name: "v2", Height: 9, BinaryVersion: "v2.0.0",
		ConfigSchemaFrom: 1, ConfigSchemaTo: 2, StoreSchemaFrom: 1, StoreSchemaTo: 2, AppStateSchemaFrom: 1, AppStateSchemaTo: 2,
	}
	encodedDue, err := json.Marshal(due)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"malformed plan":            "not json",
		"plan due before execution": string(encodedDue),
	} {
		payload, err := json.Marshal(map[string]any{
			"submitter": "alice",
			"title":     name,
			"changes":   []map[string]string{{"module": "upgrade", "key": "plan", "value": value}},
		})
		if err != nil {
			t.Fatal(err)
		}
		// Submitted at height 2 the proposal can execute at height 9 at the earliest.
		if result := chain.deliver(2, governanceSubmitJSONTx("alice", string(payload))); result.Code == 0 {
			t.Fatalf("%s: expected the proposal to be refused", name)
		}
	}
}

func TestSubmitWithUnaffordableDepositCreatesNoProposal(t *testing.T) {
	chain := newTestChain(t, testPolicy())
	chain.validator(1, "alice", "validator-1", 5)
	chain.fund("poor", 1)
	if result := chain.deliver(2, govTx("poor", "submit", "poor", "title", "execution", "max_gas", "1", "5avxo")); result.Code == 0 {
		t.Fatalf("expected a deposit the submitter cannot pay to fail, got %+v", result)
	}
	if response := chain.query("proposal", "1"); response.Code == 0 {
		t.Fatalf("expected no proposal to be stored, got %+v", response)
	}
	if balance := chain.balance("poor"); balance != "1" {
		t.Fatalf("expected the balance untouched, got %s", balance)
	}
}

func TestPolicyDefaultsApplyToUnsetThresholds(t *testing.T) {
	policy := NewModule().policy
	if policy.QuorumBPS != vexogov.DefaultQuorumBPS || policy.YesThresholdBPS != vexogov.DefaultYesThresholdBPS || policy.VotingPeriod != 1 {
		t.Fatalf("expected a module without a policy to default its thresholds, got %+v", policy)
	}
	if policy.VetoBPS != 0 {
		t.Fatalf("expected the veto threshold to stay optional, got %+v", policy)
	}
	explicit := NewModuleWithPolicy(vexogov.TallyPolicy{QuorumBPS: 2000, YesThresholdBPS: 6000, VetoBPS: 1000, VotingPeriod: 9}).policy
	if explicit.QuorumBPS != 2000 || explicit.YesThresholdBPS != 6000 || explicit.VetoBPS != 1000 || explicit.VotingPeriod != 9 {
		t.Fatalf("expected explicit thresholds to be kept, got %+v", explicit)
	}
}

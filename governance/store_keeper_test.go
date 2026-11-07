package governance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

func openTestStore(t *testing.T) *store.LevelDBStore {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	return storage
}

func testStoreKeeper(t *testing.T, storage KVStore, now uint64) *StoreKeeper {
	t.Helper()
	keeper, err := NewStoreKeeper(storage, TallyPolicy{QuorumBPS: 5000, YesThresholdBPS: 5000, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(now)
	keeper.SetTotalPower(2)
	return keeper
}

func testUpgradePlan(name string, height types.Height) upgrade.Plan {
	return upgrade.Plan{
		Name:               name,
		Height:             height,
		BinaryVersion:      name + ".0.0",
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     2,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   2,
	}
}

func upgradeProposal(t *testing.T, plan upgrade.Plan) Proposal {
	t.Helper()
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	return Proposal{
		Submitter: "alice",
		Title:     "upgrade " + plan.Name,
		Changes:   []ParameterChange{{Module: "upgrade", Key: "plan", Value: encoded}},
	}
}

func TestStoreKeeperPersistsVotesAndTally(t *testing.T) {
	storage := openTestStore(t)
	keeper := testStoreKeeper(t, storage, 1)
	proposalID, err := keeper.SubmitProposal(context.Background(), Proposal{
		Submitter: "alice",
		Title:     "title",
		Changes:   []ParameterChange{{Module: "execution", Key: "max_gas", Value: []byte("1")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetVotingPower("alice", 1)
	if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if tally, found := keeper.Tally(proposalID); !found || tally.Yes != 1 {
		t.Fatalf("expected immediate yes tally, found=%t tally=%+v", found, tally)
	}

	reopened, err := NewStoreKeeper(storage, TallyPolicy{QuorumBPS: 5000, YesThresholdBPS: 5000, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tally, found := reopened.Tally(proposalID)
	if !found || tally.Yes != types.VotingPower(1) || !tally.Passed {
		t.Fatalf("expected persisted yes tally over the snapshot of 2, found=%t tally=%+v", found, tally)
	}
	state, _, err := reopened.ProposalContext(context.Background(), proposalID)
	if err != nil || state.SnapshotPower != 2 || state.SnapshotHeight != 1 {
		t.Fatalf("expected the snapshot to be persisted, state=%+v err=%v", state, err)
	}
}

func TestStoreKeeperClockAndPowersAreNotPersisted(t *testing.T) {
	storage := openTestStore(t)
	keeper := testStoreKeeper(t, storage, 1)
	keeper.SetVotingPower("alice", 5)
	if _, _, err := keeper.ProposalContext(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	for _, key := range [][]byte{nextIDKey, proposalKey(1)} {
		if _, err := storage.Get(context.Background(), governanceNamespace, key); !errors.Is(err, store.ErrKeyNotFound) {
			t.Fatalf("expected a keeper that only read and was configured to leave the store untouched, got %v", err)
		}
	}

	reopened := testStoreKeeper(t, storage, 100)
	proposalID, err := reopened.SubmitProposal(context.Background(), testProposal())
	if err != nil {
		t.Fatal(err)
	}
	state, _, _ := reopened.ProposalContext(context.Background(), proposalID)
	if state.SubmittedAt != 100 {
		t.Fatalf("expected the clock of the keeper that submitted, got %+v", state)
	}
	other, err := NewStoreKeeper(storage, TallyPolicy{QuorumBPS: 5000, YesThresholdBPS: 5000, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	other.SetTime(1)
	if err := other.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatalf("expected the clock set on this keeper to decide, got %v", err)
	}
	if tally, _ := other.Tally(proposalID); tally.Yes != 0 {
		t.Fatalf("expected a voter without configured power to carry none, got %+v", tally)
	}
}

func TestStoreKeeperWritesOnlyToTheStoreItWasGiven(t *testing.T) {
	base := openTestStore(t)
	staged := openTestStore(t)
	keeper := testStoreKeeper(t, staged, 1)
	if _, err := keeper.SubmitProposal(context.Background(), testProposal()); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Get(context.Background(), governanceNamespace, proposalKey(1)); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected the base store to stay untouched, got %v", err)
	}
	if _, err := staged.Get(context.Background(), governanceNamespace, proposalKey(1)); err != nil {
		t.Fatalf("expected the given store to hold the proposal, got %v", err)
	}
}

func TestStoreKeeperContextQueriesPropagateStoreErrors(t *testing.T) {
	expected := errors.New("read failed")
	keeper := &StoreKeeper{store: failingKVStore{err: expected}}
	if _, _, err := keeper.ProposalContext(context.Background(), 1); !errors.Is(err, expected) {
		t.Fatalf("expected proposal query error, got %v", err)
	}
	if _, err := keeper.AppliedChangesContext(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("expected applied changes query error, got %v", err)
	}
	if _, _, err := keeper.TallyContext(context.Background(), 1); !errors.Is(err, expected) {
		t.Fatalf("expected tally query error, got %v", err)
	}
}

func TestNewStoreKeeperContextHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := openTestStore(t)
	if _, err := NewStoreKeeperContext(ctx, storage, TallyPolicy{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context startup failure, got %v", err)
	}
	if _, err := NewStoreKeeper(nil, TallyPolicy{}, nil); !errors.Is(err, ErrGovernanceStoreRequired) {
		t.Fatalf("expected a missing store to be refused, got %v", err)
	}
}

type failingKVStore struct {
	err error
}

func (store failingKVStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	return store.err
}

func (store failingKVStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	return nil, store.err
}

func (store failingKVStore) Delete(ctx context.Context, namespace string, key []byte) error {
	return store.err
}

func TestStoreKeeperRejectsCorruptExistingState(t *testing.T) {
	storage := openTestStore(t)
	for _, key := range [][]byte{nextIDKey, proposalKey(1)} {
		if err := storage.Set(context.Background(), governanceNamespace, key, []byte("{")); err != nil {
			t.Fatal(err)
		}
	}
	keeper := testStoreKeeper(t, storage, 1)
	_, _, err := keeper.ProposalContext(context.Background(), 1)
	if !errors.Is(err, ErrCorruptGovernanceState) {
		t.Fatalf("expected corrupt governance state to fail, got %v", err)
	}
	if _, err := keeper.SubmitProposal(context.Background(), testProposal()); err == nil {
		t.Fatal("expected a corrupt state not to be overwritten by a new proposal")
	}
}

func TestStoreKeeperStoresExecutedUpgradePlanInGovernanceState(t *testing.T) {
	storage := openTestStore(t)
	keeper := testStoreKeeper(t, storage, 1)
	plan := testUpgradePlan("v2", 9)
	proposalID, err := keeper.SubmitProposal(context.Background(), upgradeProposal(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetVotingPower("alice", 2)
	if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(3)
	if err := keeper.Execute(context.Background(), proposalID); err != nil {
		t.Fatal(err)
	}
	storedPlan, found, err := UpgradePlanAt(context.Background(), storage, plan.Height)
	if err != nil {
		t.Fatal(err)
	}
	if !found || storedPlan != plan {
		t.Fatalf("unexpected stored upgrade plan found=%t plan=%+v", found, storedPlan)
	}
	if _, found, err := UpgradePlanAt(context.Background(), storage, plan.Height+1); err != nil || found {
		t.Fatalf("expected no plan at another height, found=%t err=%v", found, err)
	}
}

func TestStoreKeeperUpgradePlanFollowsTheStoreItWasExecutedIn(t *testing.T) {
	// The plan is written through the keeper's own store, never through a side channel of the
	// underlying database, so a store layer that is discarded takes the plan with it.
	layer := &batchKVStore{memoryKVStore: newMemoryKVStore()}
	keeper := testStoreKeeper(t, layer, 1)
	plan := testUpgradePlan("v2", 9)
	proposalID, err := keeper.SubmitProposal(context.Background(), upgradeProposal(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetVotingPower("alice", 2)
	if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(3)
	if err := keeper.Execute(context.Background(), proposalID); err != nil {
		t.Fatal(err)
	}
	if _, found, err := UpgradePlanAt(context.Background(), layer, plan.Height); err != nil || !found {
		t.Fatalf("expected the plan in the layer the proposal executed in, found=%t err=%v", found, err)
	}
	if _, found, err := UpgradePlanAt(context.Background(), openTestStore(t), plan.Height); err != nil || found {
		t.Fatalf("expected no plan in an unrelated store, found=%t err=%v", found, err)
	}
}

func TestStoreKeeperRejectsNonAtomicPersistence(t *testing.T) {
	storage := newMemoryKVStore()
	keeper := testStoreKeeper(t, storage, 1)
	if _, err := keeper.SubmitProposal(context.Background(), testProposal()); !errors.Is(err, ErrAtomicGovernanceStoreRequired) {
		t.Fatalf("expected a store without atomic batches to be refused, got %v", err)
	}
	if len(storage.values) != 0 {
		t.Fatalf("expected nothing to be written, got %d keys", len(storage.values))
	}
}

// Every proposal is its own record: voting on one proposal rewrites that proposal only, so the cost
// of an operation does not grow with the number of proposals and votes the chain already holds.
func TestStoreKeeperOperationsTouchOnlyTheProposalTheyActOn(t *testing.T) {
	storage := &countingKVStore{KVStore: openTestStore(t)}
	keeper := testStoreKeeper(t, storage, 1)
	keeper.SetVotingPower("alice", 2)
	var last uint64
	for range 20 {
		id, err := keeper.SubmitProposal(context.Background(), testProposal())
		if err != nil {
			t.Fatal(err)
		}
		if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
			t.Fatal(err)
		}
		last = id
	}
	storage.reset()
	keeper.SetVotingPower("bob", 1)
	if err := keeper.Vote(context.Background(), 1, "bob", VoteNo); err != nil {
		t.Fatal(err)
	}
	if want := []string{string(proposalKey(1))}; !equalStrings(storage.reads, want) || !equalStrings(storage.writes, want) {
		t.Fatalf("expected a vote to read and write its proposal only, read %v wrote %v", storage.reads, storage.writes)
	}
	if state, found, err := keeper.ProposalContext(context.Background(), last); err != nil || !found || len(state.Votes) != 1 {
		t.Fatalf("expected other proposals to keep their votes, found=%t state=%+v err=%v", found, state, err)
	}
}

type countingKVStore struct {
	KVStore
	reads  []string
	writes []string
}

func (store *countingKVStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	store.reads = append(store.reads, string(key))
	return store.KVStore.Get(ctx, namespace, key)
}

func (store *countingKVStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	store.writes = append(store.writes, string(key))
	return store.KVStore.Set(ctx, namespace, key, value)
}

func (store *countingKVStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	for _, write := range writes {
		store.writes = append(store.writes, string(write.Key))
	}
	return store.KVStore.(kvbatch.BatchKVStore).SetBatch(ctx, writes)
}

func (store *countingKVStore) reset() {
	store.reads = nil
	store.writes = nil
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestStoreKeeperRefusesUpgradePlansThatCannotRun(t *testing.T) {
	t.Run("height not after the earliest execution", func(t *testing.T) {
		keeper := testStoreKeeper(t, openTestStore(t), 1)
		// Voting ends at height 2 and the timelock is zero: the earliest execution is height 2.
		if _, err := keeper.SubmitProposal(context.Background(), upgradeProposal(t, testUpgradePlan("v2", 2))); !errors.Is(err, ErrUpgradePlanNotInFuture) {
			t.Fatalf("expected a plan that is due before it can be scheduled to be refused, got %v", err)
		}
	})

	t.Run("passed after the plan height", func(t *testing.T) {
		storage := openTestStore(t)
		keeper := testStoreKeeper(t, storage, 1)
		plan := testUpgradePlan("v2", 5)
		proposalID, err := keeper.SubmitProposal(context.Background(), upgradeProposal(t, plan))
		if err != nil {
			t.Fatal(err)
		}
		keeper.SetVotingPower("alice", 2)
		if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
			t.Fatal(err)
		}
		keeper.SetTime(50)
		if err := keeper.Execute(context.Background(), proposalID); !errors.Is(err, ErrProposalRejected) || !errors.Is(err, ErrUpgradePlanNotInFuture) {
			t.Fatalf("expected a plan whose height has passed to reject the proposal, got %v", err)
		}
		if state, _, _ := keeper.ProposalContext(context.Background(), proposalID); !state.Rejected || state.Executed {
			t.Fatalf("expected the rejection to be recorded, got %+v", state)
		}
		if _, found, _ := UpgradePlanAt(context.Background(), storage, plan.Height); found {
			t.Fatal("expected no plan to be scheduled")
		}
	})

	t.Run("second plan for a scheduled height", func(t *testing.T) {
		storage := openTestStore(t)
		keeper := testStoreKeeper(t, storage, 1)
		keeper.SetVotingPower("alice", 2)
		var ids []uint64
		for _, name := range []string{"first", "second"} {
			id, err := keeper.SubmitProposal(context.Background(), upgradeProposal(t, testUpgradePlan(name, 20)))
			if err != nil {
				t.Fatal(err)
			}
			if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		keeper.SetTime(3)
		if err := keeper.Execute(context.Background(), ids[0]); err != nil {
			t.Fatal(err)
		}
		if err := keeper.Execute(context.Background(), ids[1]); !errors.Is(err, ErrProposalRejected) || !errors.Is(err, ErrUpgradePlanConflict) {
			t.Fatalf("expected the second plan for the height to be rejected, got %v", err)
		}
		stored, found, err := UpgradePlanAt(context.Background(), storage, 20)
		if err != nil || !found || stored.Name != "first" {
			t.Fatalf("expected the first plan to stay scheduled, found=%t plan=%+v err=%v", found, stored, err)
		}
	})
}

// memoryKVStore is an in-memory KV store without an atomic multi-key write, like a store that cannot
// commit a proposal and its upgrade plan together.
type memoryKVStore struct {
	values map[string][]byte
}

func newMemoryKVStore() *memoryKVStore {
	return &memoryKVStore{values: make(map[string][]byte)}
}

func (memory *memoryKVStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	memory.values[namespace+"\x00"+string(key)] = append([]byte(nil), value...)
	return nil
}

func (memory *memoryKVStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	value, found := memory.values[namespace+"\x00"+string(key)]
	if !found {
		return nil, store.ErrKeyNotFound
	}
	return append([]byte(nil), value...), nil
}

func (memory *memoryKVStore) Delete(ctx context.Context, namespace string, key []byte) error {
	delete(memory.values, namespace+"\x00"+string(key))
	return nil
}

// batchKVStore is a memoryKVStore that also commits batches.
type batchKVStore struct {
	*memoryKVStore
}

func (memory *batchKVStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	for _, write := range writes {
		if write.Delete {
			if err := memory.Delete(ctx, write.Namespace, write.Key); err != nil {
				return err
			}
			continue
		}
		if err := memory.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

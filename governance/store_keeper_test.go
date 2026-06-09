package governance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

func TestStoreKeeperPersistsVotesAndTally(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper, err := NewStoreKeeper(storage, TallyPolicy{QuorumPower: 1, YesThresholdPower: 1, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(1)
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

	reopened, err := NewStoreKeeper(storage, TallyPolicy{QuorumPower: 1, YesThresholdPower: 1, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tally, found := reopened.Tally(proposalID)
	if !found || tally.Yes != types.VotingPower(1) || !tally.Passed {
		t.Fatalf("expected persisted yes tally, found=%t tally=%+v", found, tally)
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
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if _, err := NewStoreKeeperContext(ctx, storage, TallyPolicy{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context startup failure, got %v", err)
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
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(context.Background(), governanceNamespace, []byte("state"), []byte("{")); err != nil {
		t.Fatal(err)
	}

	_, err = NewStoreKeeper(storage, TallyPolicy{}, nil)
	if err == nil || errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected corrupt governance state to fail startup, got %v", err)
	}
}

func TestStoreKeeperPersistsExecutedUpgradePlan(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper, err := NewStoreKeeper(storage, TallyPolicy{QuorumPower: 1, YesThresholdPower: 1, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := upgrade.Plan{
		Name:               "v2",
		Height:             9,
		BinaryVersion:      "v2.0.0",
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     2,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   2,
	}
	encodedPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(1)
	keeper.SetVotingPower("alice", 1)
	proposalID, err := keeper.SubmitProposal(context.Background(), Proposal{
		Submitter: "alice",
		Title:     "upgrade v2",
		Changes:   []ParameterChange{{Module: "upgrade", Key: "plan", Value: encodedPlan}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(2)
	if err := keeper.Execute(context.Background(), proposalID); err != nil {
		t.Fatal(err)
	}
	storedPlan, found, err := storage.UpgradePlanByHeight(context.Background(), plan.Height)
	if err != nil {
		t.Fatal(err)
	}
	if !found || storedPlan != plan {
		t.Fatalf("unexpected stored upgrade plan found=%t plan=%+v", found, storedPlan)
	}
}

func TestStoreKeeperRejectsNonAtomicUpgradePlanPersistence(t *testing.T) {
	storage := newNonAtomicPlanStore()
	keeper, err := NewStoreKeeper(storage, TallyPolicy{QuorumPower: 1, YesThresholdPower: 1, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := upgrade.Plan{
		Name:               "v2",
		Height:             9,
		BinaryVersion:      "v2.0.0",
		ConfigSchemaFrom:   1,
		ConfigSchemaTo:     2,
		StoreSchemaFrom:    1,
		StoreSchemaTo:      2,
		AppStateSchemaFrom: 1,
		AppStateSchemaTo:   2,
	}
	encodedPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(1)
	keeper.SetVotingPower("alice", 1)
	proposalID, err := keeper.SubmitProposal(context.Background(), Proposal{
		Submitter: "alice",
		Title:     "upgrade v2",
		Changes:   []ParameterChange{{Module: "upgrade", Key: "plan", Value: encodedPlan}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(2)
	if err := keeper.Execute(context.Background(), proposalID); !errors.Is(err, ErrAtomicUpgradePlanStoreRequired) {
		t.Fatalf("expected atomic upgrade plan store rejection, got %v", err)
	}
	if _, found, err := storage.UpgradePlanByHeight(context.Background(), plan.Height); err != nil || found {
		t.Fatalf("expected no partial upgrade plan, found=%t err=%v", found, err)
	}
}

type nonAtomicPlanStore struct {
	values map[string][]byte
	plans  map[types.Height]upgrade.Plan
}

func newNonAtomicPlanStore() *nonAtomicPlanStore {
	return &nonAtomicPlanStore{
		values: make(map[string][]byte),
		plans:  make(map[types.Height]upgrade.Plan),
	}
}

func (memory *nonAtomicPlanStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	memory.values[namespace+"\x00"+string(key)] = append([]byte(nil), value...)
	return nil
}

func (memory *nonAtomicPlanStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	value, found := memory.values[namespace+"\x00"+string(key)]
	if !found {
		return nil, store.ErrKeyNotFound
	}
	return append([]byte(nil), value...), nil
}

func (memory *nonAtomicPlanStore) Delete(ctx context.Context, namespace string, key []byte) error {
	delete(memory.values, namespace+"\x00"+string(key))
	return nil
}

func (memory *nonAtomicPlanStore) SaveUpgradePlan(ctx context.Context, plan upgrade.Plan) error {
	memory.plans[plan.Height] = plan
	return nil
}

func (memory *nonAtomicPlanStore) UpgradePlanByHeight(ctx context.Context, height types.Height) (upgrade.Plan, bool, error) {
	plan, found := memory.plans[height]
	return plan, found, nil
}

func (memory *nonAtomicPlanStore) UpgradePlanByName(ctx context.Context, name string) (upgrade.Plan, bool, error) {
	for _, plan := range memory.plans {
		if plan.Name == name {
			return plan, true, nil
		}
	}
	return upgrade.Plan{}, false, nil
}

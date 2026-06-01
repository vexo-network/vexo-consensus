package committee

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestNewDeterministicSelectorRejectsInvalidPolicy(t *testing.T) {
	if _, err := NewDeterministicSelector(RotationPolicy{CommitteeSize: 1}); !errors.Is(err, ErrInvalidEpochLength) {
		t.Fatalf("expected invalid epoch length, got %v", err)
	}
	if _, err := NewDeterministicSelector(RotationPolicy{EpochLength: 1}); !errors.Is(err, ErrInvalidCommitteeSize) {
		t.Fatalf("expected invalid committee size, got %v", err)
	}
}

func TestDeterministicSelectorSelectsCommitteeSize(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 2})
	committee, err := selector.Select(context.Background(), 0, 0, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(committee.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(committee.Members))
	}
	for _, member := range committee.Members {
		if len(member.Proof) == 0 {
			t.Fatal("expected non-empty selection proof")
		}
	}
}

func TestDeterministicSelectorCapsCommitteeToValidatorCount(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 10})
	committee, err := selector.Select(context.Background(), 0, 0, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(committee.Members) != 2 {
		t.Fatalf("expected committee capped at 2, got %d", len(committee.Members))
	}
}

func TestDeterministicSelectorIsDeterministic(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 3})
	set := testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
		{ID: "d", VotingPower: 1},
	})

	first, err := selector.Select(context.Background(), 3, 7, testSeed(9), set)
	if err != nil {
		t.Fatal(err)
	}
	second, err := selector.Select(context.Background(), 3, 7, testSeed(9), set)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(memberIDs(first), memberIDs(second)) {
		t.Fatalf("expected deterministic selection, got %v and %v", memberIDs(first), memberIDs(second))
	}
}

func TestDeterministicSelectorChangesWithRoundOrSeed(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 2})
	set := testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
		{ID: "d", VotingPower: 1},
		{ID: "e", VotingPower: 1},
		{ID: "f", VotingPower: 1},
	})

	first, err := selector.Select(context.Background(), 0, 0, testSeed(1), set)
	if err != nil {
		t.Fatal(err)
	}
	second, err := selector.Select(context.Background(), 0, 1, testSeed(1), set)
	if err != nil {
		t.Fatal(err)
	}
	third, err := selector.Select(context.Background(), 0, 0, testSeed(2), set)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(memberIDs(first), memberIDs(second)) && reflect.DeepEqual(memberIDs(first), memberIDs(third)) {
		t.Fatalf("expected round or seed to affect committee")
	}
}

func TestDeterministicSelectorFiltersByMinVotingPower(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 3, MinVotingPower: 5})
	committee, err := selector.Select(context.Background(), 0, 0, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 5},
		{ID: "c", VotingPower: 10},
	}))
	if err != nil {
		t.Fatal(err)
	}
	ids := memberIDs(committee)
	if len(ids) != 2 {
		t.Fatalf("expected 2 eligible members, got %v", ids)
	}
	for _, id := range ids {
		if id == "a" {
			t.Fatal("low voting power validator selected")
		}
	}
}

func TestDeterministicSelectorRejectsEmptyEligibleSet(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 3, MinVotingPower: 5})
	_, err := selector.Select(context.Background(), 0, 0, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
	}))
	if !errors.Is(err, ErrEmptyValidatorSet) {
		t.Fatalf("expected empty validator set, got %v", err)
	}
}

func TestDeterministicSelectorEpochForHeight(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 1})
	cases := map[types.Height]uint64{
		0:  0,
		1:  0,
		10: 0,
		11: 1,
		20: 1,
		21: 2,
	}
	for height, expected := range cases {
		if got := selector.EpochForHeight(height); got != expected {
			t.Fatalf("height %d: expected epoch %d, got %d", height, expected, got)
		}
	}
}

func TestDeterministicSelectorHonorsCancelledContext(t *testing.T) {
	selector := mustSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := selector.Select(ctx, 0, 0, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
	}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func mustSelector(t *testing.T, policy RotationPolicy) DeterministicSelector {
	t.Helper()
	selector, err := NewDeterministicSelector(policy)
	if err != nil {
		t.Fatal(err)
	}
	return selector
}

func testSet(t *testing.T, validators []validator.Validator) validator.Set {
	t.Helper()
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func testSeed(value byte) types.Hash {
	var seed types.Hash
	seed[0] = value
	return seed
}

func memberIDs(committee Committee) []types.ValidatorID {
	ids := make([]types.ValidatorID, 0, len(committee.Members))
	for _, member := range committee.Members {
		ids = append(ids, member.Validator.ID)
	}
	return ids
}

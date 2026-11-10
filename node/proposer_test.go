package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestProposerScheduleRotatesByView(t *testing.T) {
	validators := []validator.Validator{
		{ID: "alice", VotingPower: 1},
		{ID: "bob", VotingPower: 1},
		{ID: "carol", VotingPower: 1},
		{ID: "dave", VotingPower: 1},
	}
	// Four slots, stride 3: every validator leads once in any four consecutive views.
	cases := []struct {
		name     string
		first    types.Round
		expected []types.ValidatorID
	}{
		{name: "from view zero", first: 0, expected: []types.ValidatorID{"alice", "dave", "carol", "bob", "alice"}},
		{name: "from view two", first: 2, expected: []types.ValidatorID{"carol", "bob", "alice", "dave", "carol"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			schedule, err := ProposerSchedule(validators, testCase.first, uint64(len(testCase.expected)))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(schedule, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, schedule)
			}
		})
	}
}

func TestSelectProposerRejectsEmptyValidatorSet(t *testing.T) {
	_, err := SelectProposer(nil, 0)
	if !errors.Is(err, ErrMissingValidators) {
		t.Fatalf("expected missing validators, got %v", err)
	}
}

func TestProposerScheduleSortsAndSkipsZeroPowerValidators(t *testing.T) {
	validators := []validator.Validator{
		{ID: "carol", VotingPower: 1},
		{ID: "alice", VotingPower: 1},
		{ID: "bob", VotingPower: 0},
	}

	schedule, err := ProposerSchedule(validators, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	expected := []types.ValidatorID{"alice", "carol", "alice", "carol"}
	if !reflect.DeepEqual(schedule, expected) {
		t.Fatalf("expected %v, got %v", expected, schedule)
	}
}

func TestSelectProposerRejectsOnlyZeroPowerValidators(t *testing.T) {
	_, err := SelectProposer([]validator.Validator{{ID: "alice", VotingPower: 0}}, 0)
	if !errors.Is(err, ErrMissingValidators) {
		t.Fatalf("expected missing validators, got %v", err)
	}
}

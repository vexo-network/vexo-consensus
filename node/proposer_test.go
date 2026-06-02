package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestProposerScheduleRotatesByHeightAndRound(t *testing.T) {
	validators := []validator.Validator{
		{ID: "alice", VotingPower: 1},
		{ID: "bob", VotingPower: 1},
		{ID: "carol", VotingPower: 1},
		{ID: "dave", VotingPower: 1},
	}

	cases := []struct {
		name     string
		height   types.Height
		rounds   uint64
		expected []types.ValidatorID
	}{
		{
			name:     "height one starts at first validator",
			height:   1,
			rounds:   5,
			expected: []types.ValidatorID{"alice", "bob", "carol", "dave", "alice"},
		},
		{
			name:     "height two starts at second validator",
			height:   2,
			rounds:   5,
			expected: []types.ValidatorID{"bob", "carol", "dave", "alice", "bob"},
		},
		{
			name:     "zero height normalizes to height one",
			height:   0,
			rounds:   3,
			expected: []types.ValidatorID{"alice", "bob", "carol"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			schedule, err := ProposerSchedule(validators, testCase.height, testCase.rounds)
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
	_, err := SelectProposer(nil, 1, 0)
	if !errors.Is(err, ErrMissingValidators) {
		t.Fatalf("expected missing validators, got %v", err)
	}
}

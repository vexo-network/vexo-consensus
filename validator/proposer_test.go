package validator

import (
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestSelectProposerSortsActiveValidatorsAndRotates(t *testing.T) {
	validators := []Validator{
		{ID: "carol", VotingPower: 1},
		{ID: "alice", VotingPower: 1},
		{ID: "disabled", VotingPower: 0},
		{ID: "bob", VotingPower: 1},
	}
	var schedule []types.ValidatorID
	for round := types.Round(0); round < 4; round++ {
		proposer, ok := SelectProposer(validators, 1, round)
		if !ok {
			t.Fatal("expected an active proposer")
		}
		schedule = append(schedule, proposer)
	}
	if expected := []types.ValidatorID{"alice", "bob", "carol", "alice"}; !reflect.DeepEqual(schedule, expected) {
		t.Fatalf("expected %v, got %v", expected, schedule)
	}
}

func TestSelectProposerRejectsMissingActiveValidators(t *testing.T) {
	if proposer, ok := SelectProposer([]Validator{{ID: "alice"}}, 1, 0); ok || proposer != "" {
		t.Fatalf("expected no proposer, got %q", proposer)
	}
}

package validator

import (
	"fmt"
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
	for round := types.Round(0); round < 6; round++ {
		proposer, ok := SelectProposer(validators, round)
		if !ok {
			t.Fatal("expected an active proposer")
		}
		schedule = append(schedule, proposer)
	}
	// Three slots, stride 2.
	if expected := []types.ValidatorID{"alice", "carol", "bob", "alice", "carol", "bob"}; !reflect.DeepEqual(schedule, expected) {
		t.Fatalf("expected %v, got %v", expected, schedule)
	}
}

// An even number of validators of equal power all lead: a schedule of the height and the view
// gave every second validator no view on the happy path, where both grow by one per block.
func TestSelectProposerGivesEveryValidatorOfAnEvenSetItsViews(t *testing.T) {
	for _, count := range []int{2, 4, 6, 10} {
		validators := make([]Validator, count)
		for index := range validators {
			validators[index] = Validator{ID: types.ValidatorID(fmt.Sprintf("validator-%02d", index)), VotingPower: 1}
		}
		led := map[types.ValidatorID]int{}
		for round := types.Round(0); round < types.Round(count); round++ {
			proposer, _ := SelectProposer(validators, round)
			led[proposer]++
		}
		if len(led) != count {
			t.Fatalf("%d validators: only %d lead in %d views: %v", count, len(led), count, led)
		}
	}
}

// Every window of W consecutive views, W the total power, gives each validator exactly its power
// in views, wherever the window starts.
func TestSelectProposerIsStakeProportionalInEveryWindow(t *testing.T) {
	validators := []Validator{
		{ID: "alice", VotingPower: 7},
		{ID: "bob", VotingPower: 2},
		{ID: "carol", VotingPower: 3},
		{ID: "dave", VotingPower: 1},
	}
	const total = 13
	for start := types.Round(0); start < 3*total; start++ {
		led := map[types.ValidatorID]types.VotingPower{}
		for round := start; round < start+total; round++ {
			proposer, ok := SelectProposer(validators, round)
			if !ok {
				t.Fatal("expected an active proposer")
			}
			led[proposer]++
		}
		for _, validatorInfo := range validators {
			if led[validatorInfo.ID] != validatorInfo.VotingPower {
				t.Fatalf("window from view %d: %s led %d views, holds %d of %d", start, validatorInfo.ID, led[validatorInfo.ID], validatorInfo.VotingPower, total)
			}
		}
	}
}

// The views of a validator are spread over the window: one holding a tenth of the power never
// leads two views in a row.
func TestSelectProposerSpreadsTheViewsOfAValidator(t *testing.T) {
	validators := []Validator{{ID: "alice", VotingPower: 900}, {ID: "bob", VotingPower: 100}}
	previous := types.ValidatorID("")
	for round := types.Round(0); round < 1000; round++ {
		proposer, _ := SelectProposer(validators, round)
		if proposer == "bob" && previous == "bob" {
			t.Fatalf("bob leads views %d and %d", round-1, round)
		}
		previous = proposer
	}
}

func TestSelectProposerHandlesPowerAtTheTopOfTheRange(t *testing.T) {
	validators := []Validator{{ID: "alice", VotingPower: 1 << 62}, {ID: "bob", VotingPower: 1 << 62}}
	led := map[types.ValidatorID]int{}
	for round := types.Round(1 << 40); round < 1<<40+64; round++ {
		proposer, ok := SelectProposer(validators, round)
		if !ok {
			t.Fatal("expected an active proposer")
		}
		led[proposer]++
	}
	if led["alice"] == 0 || led["bob"] == 0 {
		t.Fatalf("expected both to lead, got %v", led)
	}
	if _, ok := SelectProposer([]Validator{{ID: "alice", VotingPower: 1 << 63}, {ID: "bob", VotingPower: 1 << 63}}, 0); ok {
		t.Fatal("expected a total power that overflows to have no schedule")
	}
}

func TestSelectProposerRejectsMissingActiveValidators(t *testing.T) {
	if proposer, ok := SelectProposer([]Validator{{ID: "alice"}}, 0); ok || proposer != "" {
		t.Fatalf("expected no proposer, got %q", proposer)
	}
}

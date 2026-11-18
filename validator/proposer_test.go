package validator

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"reflect"
	"sort"
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

func TestNewScheduleRejectsSetsWithoutASchedule(t *testing.T) {
	if _, ok := NewSchedule(nil); ok {
		t.Fatal("expected no schedule without validators")
	}
	if _, ok := NewSchedule([]Validator{{ID: "alice"}, {ID: "bob"}}); ok {
		t.Fatal("expected no schedule without an active validator")
	}
	if _, ok := NewSchedule([]Validator{{ID: "alice", VotingPower: math.MaxUint64}, {ID: "bob", VotingPower: 1}}); ok {
		t.Fatal("expected no schedule for a total power that overflows 64 bits")
	}
	if _, ok := NewSchedule([]Validator{{ID: "alice", VotingPower: math.MaxUint64}, {ID: "bob"}}); !ok {
		t.Fatal("expected the largest total power that fits in 64 bits to have a schedule")
	}
}

// The proposer of a prepared schedule reports the voting power of the validator, whose slots the
// view falls on.
func TestScheduleReportsThePowerOfTheProposer(t *testing.T) {
	validators := []Validator{
		{ID: "carol", VotingPower: 3},
		{ID: "alice", VotingPower: 7},
		{ID: "idle"},
		{ID: "bob", VotingPower: 2},
	}
	schedule, ok := NewSchedule(validators)
	if !ok {
		t.Fatal("expected a schedule")
	}
	powers := map[types.ValidatorID]types.VotingPower{"alice": 7, "bob": 2, "carol": 3}
	for round := types.Round(0); round < 100; round++ {
		proposer, power := schedule.Proposer(round)
		if power != powers[proposer] {
			t.Fatalf("view %d: %s reported with power %d, holds %d", round, proposer, power, powers[proposer])
		}
	}
}

// referenceSelectProposer is SelectProposer as it was before the schedule was prepared once: every
// call filters, sorts and walks the validators.
func referenceSelectProposer(validators []Validator, round types.Round) (types.ValidatorID, bool) {
	active := make([]Validator, 0, len(validators))
	var total uint64
	for _, validatorInfo := range validators {
		if validatorInfo.VotingPower == 0 {
			continue
		}
		sum, carry := bits.Add64(total, uint64(validatorInfo.VotingPower), 0)
		if carry != 0 {
			return "", false
		}
		total = sum
		active = append(active, validatorInfo)
	}
	if len(active) == 0 {
		return "", false
	}
	sort.Slice(active, func(left, right int) bool {
		return active[left].ID < active[right].ID
	})
	high, low := bits.Mul64(uint64(round), scheduleStride(total))
	slot := bits.Rem64(high, low, total)
	for _, validatorInfo := range active {
		if slot < uint64(validatorInfo.VotingPower) {
			return validatorInfo.ID, true
		}
		slot -= uint64(validatorInfo.VotingPower)
	}
	return "", false
}

// randomValidators returns up to maxCount validators with distinct ids in random order, some without
// power and some with power at the top of the range, whose total power fits in 64 bits.
func randomValidators(random *rand.Rand, maxCount int) []Validator {
	count := 1 + random.Intn(maxCount)
	names := random.Perm(4 * maxCount)
	validators := make([]Validator, 0, count)
	var total uint64
	for index := 0; index < count; index++ {
		var power uint64
		switch random.Intn(8) {
		case 0:
		case 1:
			power = 1 << uint(30+random.Intn(30))
		default:
			power = 1 + uint64(random.Intn(20))
		}
		if total+power < total {
			power = 0
		}
		total += power
		validators = append(validators, Validator{ID: types.ValidatorID(fmt.Sprintf("validator-%03d", names[index])), VotingPower: types.VotingPower(power)})
	}
	return validators
}

// The prepared schedule leads every view as the schedule that sorts and walks the validators for
// each view did: same leader, whatever the order, power and view.
func TestScheduleMatchesTheReferenceSchedule(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for attempt := 0; attempt < 400; attempt++ {
		validators := randomValidators(random, 24)
		schedule, scheduled := NewSchedule(validators)
		_, referenced := referenceSelectProposer(validators, 0)
		if scheduled != referenced {
			t.Fatalf("attempt %d: schedule %v, reference %v for %v", attempt, scheduled, referenced, validators)
		}
		if !scheduled {
			continue
		}
		rounds := []types.Round{0, 1, 2, math.MaxUint64, math.MaxUint64 - 1, 1 << 62, 1 << 63}
		for len(rounds) < 120 {
			rounds = append(rounds, types.Round(random.Uint64()>>uint(random.Intn(64))))
		}
		for _, round := range rounds {
			want, _ := referenceSelectProposer(validators, round)
			if got, ok := SelectProposer(validators, round); !ok || got != want {
				t.Fatalf("attempt %d view %d: SelectProposer %q, reference %q", attempt, round, got, want)
			}
			if got, _ := schedule.Proposer(round); got != want {
				t.Fatalf("attempt %d view %d: schedule %q, reference %q", attempt, round, got, want)
			}
		}
	}
}

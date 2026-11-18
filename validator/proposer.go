package validator

import (
	"math"
	"math/bits"
	"slices"
	"sort"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

// SelectProposer returns the active validator that leads view round: the proposer of the Schedule
// of validators. It prepares the schedule for the one view it is asked about; a caller that asks
// about many views of the same validators prepares the Schedule once.
func SelectProposer(validators []Validator, round types.Round) (types.ValidatorID, bool) {
	schedule, ok := NewSchedule(validators)
	if !ok {
		return "", false
	}
	proposer, _ := schedule.Proposer(round)
	return proposer, true
}

// Schedule is the leader schedule of a set of validators, prepared once so that the leader of any
// number of views is found without copying or sorting the validators again.
//
// Views are global, so the leader is a function of the view alone. The schedule is stake
// proportional and stateless: the active validators, sorted by id, own consecutive slots of
// [0, W), each as many as its voting power, W the total. View v falls on slot v·S mod W, where the
// stride S is the first integer from W·(φ−1) on that is coprime to W. S is coprime to W, so any W
// consecutive views visit every slot once: every validator leads exactly its share of every window
// of W views. The golden-ratio stride spreads each validator's views evenly over the window
// instead of in one run (a Weyl sequence), so a crashed validator costs isolated views, not a
// stretch of them.
//
// A schedule of the height and the view would be wrong: on the happy path both grow by one per
// block, so with an even number of validators the parity of their sum never changes and half of
// the validators never lead.
type Schedule struct {
	ids []types.ValidatorID
	// ends[index] is one past the last slot of ids[index], so the slots of ids[index] are
	// [ends[index-1], ends[index]) and the last entry is the total voting power.
	ends   []uint64
	stride uint64
}

// NewSchedule prepares the schedule of the active validators, those with voting power. It reports
// false when no validator is active or their voting power does not add up in 64 bits: such a set
// has no schedule.
func NewSchedule(validators []Validator) (Schedule, bool) {
	type slot struct {
		id    types.ValidatorID
		power uint64
	}
	active := make([]slot, 0, len(validators))
	var total uint64
	for _, validatorInfo := range validators {
		if validatorInfo.VotingPower == 0 {
			continue
		}
		sum, carry := bits.Add64(total, uint64(validatorInfo.VotingPower), 0)
		if carry != 0 {
			return Schedule{}, false
		}
		total = sum
		active = append(active, slot{id: validatorInfo.ID, power: uint64(validatorInfo.VotingPower)})
	}
	if len(active) == 0 {
		return Schedule{}, false
	}
	slices.SortFunc(active, func(left, right slot) int {
		return strings.Compare(string(left.id), string(right.id))
	})
	schedule := Schedule{
		ids:    make([]types.ValidatorID, len(active)),
		ends:   make([]uint64, len(active)),
		stride: scheduleStride(total),
	}
	var end uint64
	for index, entry := range active {
		end += entry.power
		schedule.ids[index] = entry.id
		schedule.ends[index] = end
	}
	return schedule, true
}

// Proposer returns the validator that leads view round and its voting power. The zero Schedule has
// no validators and no proposer; NewSchedule reports it as false.
func (schedule Schedule) Proposer(round types.Round) (types.ValidatorID, types.VotingPower) {
	total := schedule.ends[len(schedule.ends)-1]
	high, low := bits.Mul64(uint64(round), schedule.stride)
	slot := bits.Rem64(high, low, total)
	index := sort.Search(len(schedule.ends), func(index int) bool { return slot < schedule.ends[index] })
	start := uint64(0)
	if index > 0 {
		start = schedule.ends[index-1]
	}
	return schedule.ids[index], types.VotingPower(schedule.ends[index] - start)
}

// scheduleStride returns the first integer from total·(φ−1) on that is coprime to total.
func scheduleStride(total uint64) uint64 {
	if total == 1 {
		return 1
	}
	stride := uint64(math.Ceil(float64(total) * (math.Sqrt(5) - 1) / 2))
	stride = max(1, min(stride, total-1))
	for gcd(stride, total) != 1 {
		stride++
		if stride == total {
			stride = 1
		}
	}
	return stride
}

func gcd(left, right uint64) uint64 {
	for right != 0 {
		left, right = right, left%right
	}
	return left
}

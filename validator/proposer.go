package validator

import (
	"math"
	"math/bits"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

// SelectProposer returns the active validator that leads view round.
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
func SelectProposer(validators []Validator, round types.Round) (types.ValidatorID, bool) {
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

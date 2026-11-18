package consensus

import (
	"math"
	"slices"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// BenchViews is how many views a validator that failed to lead a view stays benched: left out of
// the schedule of the views that directly follow a certified block.
const BenchViews types.Round = 1000

// Leader returns the validator that leads the view of header among validatorSet, the set the header
// names. A view that directly follows the certificate of the block's parent (Round ==
// JustifyRound+1, the happy path) is led by the stake-proportional schedule
// (validator.SelectProposer) of the validators the header does not bench. Any other view follows a
// timeout certificate, whose leader every validator must agree on whatever certified block it
// holds, and is led by the schedule of the whole set.
//
// The leader so follows from the header and the set alone. A benched list other than the one the
// parent fixes (ChildBench) makes the header invalid.
func Leader(header types.Header, validatorSet validator.Set) (types.ValidatorID, bool) {
	validators := validatorSet.List()
	if header.Round != header.JustifyRound+1 || len(header.Benched) == 0 {
		return validator.SelectProposer(validators, header.Round)
	}
	eligible := make([]validator.Validator, 0, len(validators))
	for _, validatorInfo := range validators {
		if !isBenched(header.Benched, validatorInfo.ID) {
			eligible = append(eligible, validatorInfo)
		}
	}
	if leader, ok := validator.SelectProposer(eligible, header.Round); ok {
		return leader, true
	}
	return validator.SelectProposer(validators, header.Round)
}

// ChildBench returns the benched list of a child of parent proposed in view round, certified by
// validatorSet. It is nil for the first block (parent nil). Otherwise it is the parent's list with
// the leader of every view between the parent's view and round added: that view produced no block
// this chain extends, so its leader failed it. A benched validator stays benched through BenchViews
// views after the view it failed, and the benched validators hold less than a third of the voting
// power: a failure that would reach a third benches no one, so the schedule always keeps more than
// two thirds of the power. Only the views from round-BenchViews on are considered: a view failed
// before is benched through a view before round.
//
// The list derives from the chain only, so every validator that holds the parent computes the same
// one, and the leader of a view never depends on what a validator observed locally.
//
// It takes a step for each view it considers, at most BenchViews of them, and prepares the schedule
// of the whole set once for all of them instead of copying the validators for each.
func ChildBench(parent *types.Header, round types.Round, validatorSet validator.Set) []types.Bench {
	if parent == nil || round <= parent.Round {
		return nil
	}
	total := uint64(validatorSet.TotalVotingPower())
	list := newBenchList(parent.Benched, validatorSet)
	leaders := viewLeaders{validatorSet: validatorSet}
	first := parent.Round + 1
	if round > BenchViews && round-BenchViews > first {
		first = round - BenchViews
	}
	for view := first; view < round; view++ {
		list.expire(view)
		failed, power, ok := leaders.failed(parent, list.entries, view)
		if !ok {
			continue
		}
		list.bench(failed, uint64(power), view+BenchViews, total)
	}
	list.expire(round)
	if len(list.entries) == 0 {
		return nil
	}
	return slices.Clip(list.entries)
}

// viewLeaders finds the leaders of the views ChildBench considers.
type viewLeaders struct {
	validatorSet validator.Set
	// schedule is the schedule of the whole set, prepared by the first view that needs it, and
	// scheduled whether the set has one.
	schedule  validator.Schedule
	prepared  bool
	scheduled bool
}

// failed returns the leader of view, a view without a block between parent's view and its child's,
// and its voting power: the view directly after the parent's was scheduled with the parent's bench
// (bench, as it stands in that view), and every later one followed a timeout certificate, whose
// leader comes from the schedule of the whole set (Leader).
func (leaders *viewLeaders) failed(parent *types.Header, bench []types.Bench, view types.Round) (types.ValidatorID, types.VotingPower, bool) {
	if view == parent.Round+1 && len(bench) > 0 {
		leader, ok := Leader(types.Header{Round: view, JustifyRound: view - 1, Benched: bench}, leaders.validatorSet)
		if !ok {
			return "", 0, false
		}
		member, _ := leaders.validatorSet.Get(leader)
		return leader, member.VotingPower, true
	}
	if !leaders.prepared {
		leaders.schedule, leaders.scheduled = validator.NewSchedule(leaders.validatorSet.List())
		leaders.prepared = true
	}
	if !leaders.scheduled {
		return "", 0, false
	}
	leader, power := leaders.schedule.Proposer(view)
	return leader, power, true
}

// benchList is the benched list ChildBench walks through the views: the entries ordered by
// validator, the voting power of each entry's validator, and the sum of those powers, so that
// neither the expiry of entries nor the power check of a new one visits the validator set. Sums
// are plain uint64 arithmetic.
type benchList struct {
	entries []types.Bench
	powers  []uint64
	sum     uint64
	// earliest is at most the first view through which an entry stays benched: no entry expires
	// before the view after it.
	earliest types.Round
}

// newBenchList returns a copy of bench without the entries of validators without power in
// validatorSet.
func newBenchList(bench []types.Bench, validatorSet validator.Set) benchList {
	list := benchList{
		entries:  make([]types.Bench, 0, len(bench)),
		powers:   make([]uint64, 0, len(bench)),
		earliest: math.MaxUint64,
	}
	for _, entry := range bench {
		member, found := validatorSet.Get(entry.Validator)
		if !found || member.VotingPower == 0 {
			continue
		}
		list.entries = append(list.entries, entry)
		list.powers = append(list.powers, uint64(member.VotingPower))
		list.sum += uint64(member.VotingPower)
		list.earliest = min(list.earliest, entry.Until)
	}
	return list
}

// expire drops the entries benched through a view before view.
func (list *benchList) expire(view types.Round) {
	if list.earliest >= view {
		return
	}
	kept := 0
	list.earliest = math.MaxUint64
	for index, entry := range list.entries {
		if entry.Until < view {
			list.sum -= list.powers[index]
			continue
		}
		list.entries[kept], list.powers[kept] = entry, list.powers[index]
		list.earliest = min(list.earliest, entry.Until)
		kept++
	}
	list.entries, list.powers = list.entries[:kept], list.powers[:kept]
}

// bench benches id, a validator of power, through until, or extends its bench, unless the benched
// power would reach a third of total.
func (list *benchList) bench(id types.ValidatorID, power uint64, until types.Round, total uint64) {
	index, found := slices.BinarySearchFunc(list.entries, id, func(entry types.Bench, target types.ValidatorID) int {
		return strings.Compare(string(entry.Validator), string(target))
	})
	if found {
		list.entries[index].Until = max(list.entries[index].Until, until)
		return
	}
	if list.sum+power >= total/3+min(total%3, 1) {
		// 3·power ≥ total, without overflow.
		return
	}
	list.entries = slices.Insert(list.entries, index, types.Bench{Validator: id, Until: until})
	list.powers = slices.Insert(list.powers, index, power)
	list.sum += power
	list.earliest = min(list.earliest, until)
}

func isBenched(bench []types.Bench, id types.ValidatorID) bool {
	for _, entry := range bench {
		if entry.Validator == id {
			return true
		}
	}
	return false
}

package consensus

import (
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
func ChildBench(parent *types.Header, round types.Round, validatorSet validator.Set) []types.Bench {
	if parent == nil || round <= parent.Round {
		return nil
	}
	total := uint64(validatorSet.TotalVotingPower())
	bench := slices.Clone(parent.Benched)
	first := parent.Round + 1
	if round > BenchViews && round-BenchViews > first {
		first = round - BenchViews
	}
	for view := first; view < round; view++ {
		bench = liveBench(bench, view, validatorSet)
		failed, ok := failedLeader(parent, bench, view, validatorSet)
		if !ok {
			continue
		}
		bench = benchLeader(bench, failed, view+BenchViews, validatorSet, total)
	}
	bench = liveBench(bench, round, validatorSet)
	if len(bench) == 0 {
		return nil
	}
	return bench
}

// failedLeader returns the leader of view, a view without a block between parent's view and its
// child's: the view directly after the parent's was scheduled with the parent's bench, and every
// later one followed a timeout certificate.
func failedLeader(parent *types.Header, bench []types.Bench, view types.Round, validatorSet validator.Set) (types.ValidatorID, bool) {
	return Leader(types.Header{Round: view, JustifyRound: view - 1, Benched: benchAt(parent, bench, view)}, validatorSet)
}

// benchAt is the bench the schedule of view used: the parent's for the view directly after it,
// none for the views after a timeout certificate (Leader ignores it there).
func benchAt(parent *types.Header, bench []types.Bench, view types.Round) []types.Bench {
	if view == parent.Round+1 {
		return bench
	}
	return nil
}

// liveBench drops the entries benched through a view before view, and those of validators without
// power in validatorSet.
func liveBench(bench []types.Bench, view types.Round, validatorSet validator.Set) []types.Bench {
	live := bench[:0:0]
	for _, entry := range bench {
		if entry.Until < view {
			continue
		}
		if member, found := validatorSet.Get(entry.Validator); !found || member.VotingPower == 0 {
			continue
		}
		live = append(live, entry)
	}
	return live
}

// benchLeader benches id through until, or extends its bench, unless the benched power would reach
// a third of total.
func benchLeader(bench []types.Bench, id types.ValidatorID, until types.Round, validatorSet validator.Set, total uint64) []types.Bench {
	index, found := slices.BinarySearchFunc(bench, id, func(entry types.Bench, target types.ValidatorID) int {
		return strings.Compare(string(entry.Validator), string(target))
	})
	if found {
		bench[index].Until = max(bench[index].Until, until)
		return bench
	}
	member, _ := validatorSet.Get(id)
	power := uint64(member.VotingPower)
	for _, entry := range bench {
		benched, _ := validatorSet.Get(entry.Validator)
		power += uint64(benched.VotingPower)
	}
	if power >= total/3+min(total%3, 1) {
		// 3·power ≥ total, without overflow.
		return bench
	}
	return slices.Insert(bench, index, types.Bench{Validator: id, Until: until})
}

func isBenched(bench []types.Bench, id types.ValidatorID) bool {
	for _, entry := range bench {
		if entry.Validator == id {
			return true
		}
	}
	return false
}

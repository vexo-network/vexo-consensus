package consensus

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func equalSet(ids ...types.ValidatorID) validator.Set {
	validators := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		validators = append(validators, validator.Validator{ID: id, VotingPower: 1})
	}
	return validator.NewSet(validators)
}

// directLeader returns the leader of view round directly after a parent of view round-1.
func directLeader(t *testing.T, parent types.Header, round types.Round, set validator.Set) types.ValidatorID {
	t.Helper()
	header := types.Header{Height: parent.Height + 1, Round: round, JustifyRound: parent.Round, Benched: ChildBench(&parent, round, set)}
	leader, ok := Leader(header, set)
	if !ok {
		t.Fatal("no leader")
	}
	return leader
}

// A view without a block benches its leader: the views that directly follow a certified block are
// led by the others until BenchViews views after the failure, and the leader is back after that.
func TestALeaderThatFailsItsViewIsBenched(t *testing.T) {
	set := equalSet("a", "b", "c", "d")
	parent := types.Header{Height: 5, Round: 9}
	failed, _ := validator.SelectProposer(set.List(), 10)
	// View 10 failed; the child is proposed in view 11 after a timeout certificate.
	child := types.Header{Height: 6, Round: 11, JustifyRound: 9, Benched: ChildBench(&parent, 11, set)}
	if want := []types.Bench{{Validator: failed, Until: 10 + BenchViews}}; !slices.Equal(child.Benched, want) {
		t.Fatalf("expected the leader of the failed view benched as %v, got %v", want, child.Benched)
	}
	led := map[types.ValidatorID]bool{}
	for round := child.Round + 1; round < child.Round+200; round++ {
		at := types.Header{Height: child.Height, Round: round - 1, Benched: ChildBench(&child, round-1, set)}
		led[directLeader(t, at, round, set)] = true
	}
	if led[failed] || len(led) != 3 {
		t.Fatalf("expected the three others to lead while %s is benched, got %v", failed, led)
	}
	late := types.Header{Height: 7, Round: 10 + BenchViews, JustifyRound: 10 + BenchViews - 1, Benched: child.Benched}
	if bench := ChildBench(&late, late.Round+1, set); len(bench) != 0 {
		t.Fatalf("expected the bench to expire after %d views, got %v", BenchViews, bench)
	}
}

// After a timeout certificate every validator schedules the view alike, whatever block it holds: the
// whole set leads it, benched or not.
func TestAViewAfterATimeoutIsLedByTheWholeSet(t *testing.T) {
	set := equalSet("a", "b", "c", "d")
	for round := types.Round(2); round < 50; round++ {
		want, _ := validator.SelectProposer(set.List(), round)
		for _, benched := range set.List() {
			header := types.Header{Height: 3, Round: round, JustifyRound: round - 2, Benched: []types.Bench{{Validator: benched.ID, Until: round + 5}}}
			if got, _ := Leader(header, set); got != want {
				t.Fatalf("view %d after a timeout: expected %s, got %s with %s benched", round, want, got, benched.ID)
			}
		}
	}
}

// The benched validators hold less than a third of the power: the failure that would reach a third
// benches no one.
func TestTheBenchHoldsLessThanAThirdOfThePower(t *testing.T) {
	set := equalSet("a", "b", "c", "d", "e", "f", "g")
	parent := types.Header{Height: 2, Round: 1}
	bench := ChildBench(&parent, 40, set)
	if len(bench) != 2 {
		t.Fatalf("expected 2 of 7 validators benched after many failed views, got %v", bench)
	}
	if bench := ChildBench(&types.Header{Height: 2, Round: 1}, 40, equalSet("a", "b", "c")); len(bench) != 0 {
		t.Fatalf("expected no validator of three benched, got %v", bench)
	}
}

// A proposal that benches another validator than its parent fixes, here to make its own proposer
// the leader of the view, is refused.
func TestStateMachineRefusesAProposalWithAForgedBench(t *testing.T) {
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	machine := cluster.newMachineWith(func(config *StateMachineConfig) { config.EnforceProposerSelection = true })
	first := extendLed(cluster, machine, nil, 0)
	second := extendLed(cluster, machine, &first, 1)

	leader, _ := Leader(types.Header{Round: 2, JustifyRound: 1}, cluster.set)
	var forger types.ValidatorID
	var forged []types.Bench
	for _, id := range cluster.ids {
		bench := []types.Bench{{Validator: leader, Until: 100}}
		if got, _ := Leader(types.Header{Round: 2, JustifyRound: 1, Benched: bench}, cluster.set); got == id && id != leader {
			forger, forged = id, bench
		}
	}
	if forger == "" {
		t.Fatal("no validator gains view 2 by benching its leader")
	}
	block := cluster.block(3, second.hash)
	block.Header.Benched = forged
	proposal := Proposal{Block: block, Round: 2, Proposer: forger, JustifyQC: second.qc}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), proposal); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected the forged bench refused, got %v", err)
	}
	if err := machine.CheckBench(block.Header); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected CheckBench to refuse the forged bench, got %v", err)
	}
	honest := cluster.proposal(leader, cluster.block(3, second.hash), 2, second.qc, finality.TimeoutCert{})
	if err := machine.OnProposal(context.Background(), honest); err != nil {
		t.Fatalf("expected the leader's proposal accepted, got %v", err)
	}
}

// The first block benches no one.
func TestTheFirstBlockBenchesNoOne(t *testing.T) {
	header := types.Header{Height: 1, TimeUnixNano: 1, ValidatorSetHeight: finality.FirstValidatorSetHeight, Benched: []types.Bench{{Validator: "a", Until: 5}}}
	if err := ParentLinkageFault(header, nil); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a first block with a bench refused, got %v", err)
	}
}

// extendLed proposes a child of parent in round through the machine by the view's leader and
// certifies it.
func extendLed(cluster *testCluster, machine *StateMachine, parent *testBlock, round types.Round) testBlock {
	cluster.t.Helper()
	height, parentHash, justify := types.Height(1), types.Hash{}, finality.QuorumCert{}
	if parent != nil {
		height, parentHash, justify = parent.block.Header.Height+1, parent.hash, parent.qc
	}
	block := cluster.seal(cluster.block(height, parentHash), round, justify.Round)
	leader, _ := Leader(block.Header, cluster.set)
	if err := machine.OnProposal(context.Background(), cluster.proposal(leader, block, round, justify, finality.TimeoutCert{})); err != nil {
		cluster.t.Fatalf("proposal for height %d view %d: %v", height, round, err)
	}
	hash := HashBlock(block)
	for _, voter := range cluster.ids {
		if err := machine.OnVote(context.Background(), cluster.vote(voter, height, round, hash)); err != nil {
			cluster.t.Fatal(err)
		}
		if qc, err := machine.BuildQuorumCert(height, round, hash); err == nil {
			return testBlock{block: block, hash: hash, round: round, qc: qc}
		}
	}
	cluster.t.Fatalf("no quorum for height %d view %d", height, round)
	return testBlock{}
}

// referenceChildBench is ChildBench as it was before the schedule of the whole set was prepared once
// per call: for every view it copies the validators to find the leader and rebuilds the live list.
// It is kept as the specification the fast ChildBench must match bit for bit, as every validator
// must compute the same list whatever its implementation.
func referenceChildBench(parent *types.Header, round types.Round, validatorSet validator.Set) []types.Bench {
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
		bench = referenceLiveBench(bench, view, validatorSet)
		failed, ok := referenceFailedLeader(parent, bench, view, validatorSet)
		if !ok {
			continue
		}
		bench = referenceBenchLeader(bench, failed, view+BenchViews, validatorSet, total)
	}
	bench = referenceLiveBench(bench, round, validatorSet)
	if len(bench) == 0 {
		return nil
	}
	return bench
}

func referenceFailedLeader(parent *types.Header, bench []types.Bench, view types.Round, validatorSet validator.Set) (types.ValidatorID, bool) {
	return Leader(types.Header{Round: view, JustifyRound: view - 1, Benched: referenceBenchAt(parent, bench, view)}, validatorSet)
}

func referenceBenchAt(parent *types.Header, bench []types.Bench, view types.Round) []types.Bench {
	if view == parent.Round+1 {
		return bench
	}
	return nil
}

func referenceLiveBench(bench []types.Bench, view types.Round, validatorSet validator.Set) []types.Bench {
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

func referenceBenchLeader(bench []types.Bench, id types.ValidatorID, until types.Round, validatorSet validator.Set, total uint64) []types.Bench {
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
		return bench
	}
	return slices.Insert(bench, index, types.Bench{Validator: id, Until: until})
}

// randomBenchSet returns a validator set of random size whose voting power is spread one of several
// ways: equal, small and unequal, spanning many magnitudes, with validators without power, or so
// large that the power does not add up in 64 bits (a set that has no schedule).
func randomBenchSet(random *rand.Rand) (validator.Set, string) {
	size := 1 + random.Intn(40)
	style := random.Intn(5)
	used := make(map[types.ValidatorID]struct{}, size)
	validators := make([]validator.Validator, 0, size)
	for len(validators) < size {
		id := types.ValidatorID(fmt.Sprintf("%c%d", 'a'+random.Intn(26), random.Intn(100)))
		if _, taken := used[id]; taken {
			continue
		}
		used[id] = struct{}{}
		var power uint64
		switch style {
		case 0:
			power = 1
		case 1:
			power = 1 + uint64(random.Intn(9))
		case 2:
			power = 1 + uint64(random.Int63n(1<<uint(1+random.Intn(40))))
		case 3:
			power = uint64(random.Intn(4)) * uint64(1+random.Intn(5))
		case 4:
			power = 1 << 63
			if len(validators) >= 2 {
				power = uint64(random.Intn(3))
			}
		}
		validators = append(validators, validator.Validator{ID: id, VotingPower: types.VotingPower(power)})
	}
	return validator.NewSet(validators), fmt.Sprintf("style %d, %d validators", style, size)
}

// randomBenchParent returns a parent header whose view is anywhere from the first view to the last
// a view can be and whose benched list holds random validators of the set, some without power or
// unknown, benched through a view before or after the views its child considers.
func randomBenchParent(random *rand.Rand, set validator.Set) types.Header {
	var round types.Round
	switch random.Intn(5) {
	case 0:
		round = types.Round(random.Intn(3))
	case 1:
		round = types.Round(random.Intn(2000))
	case 2:
		round = types.Round(random.Int63())
	case 3:
		round = 1 << 62
	case 4:
		round = math.MaxUint64 - types.Round(random.Intn(4000)) - 1
	}
	var bench []types.Bench
	if random.Intn(4) > 0 {
		ids := make([]types.ValidatorID, 0, len(set.List())+1)
		for _, member := range set.List() {
			ids = append(ids, member.ID)
		}
		ids = append(ids, "unknown")
		random.Shuffle(len(ids), func(first, second int) { ids[first], ids[second] = ids[second], ids[first] })
		for _, id := range ids[:random.Intn(len(ids)+1)] {
			until := round + types.Round(random.Intn(int(BenchViews)+6)) - 3
			if round < 3 && until > round+BenchViews+3 {
				until = 0
			}
			bench = append(bench, types.Bench{Validator: id, Until: until})
		}
		slices.SortFunc(bench, func(first, second types.Bench) int {
			return strings.Compare(string(first.Validator), string(second.Validator))
		})
		if random.Intn(8) == 0 {
			// A list a header cannot carry (unordered, a validator twice): both must treat it alike.
			random.Shuffle(len(bench), func(first, second int) { bench[first], bench[second] = bench[second], bench[first] })
			if len(bench) > 0 {
				bench = append(bench, bench[random.Intn(len(bench))])
			}
		}
	}
	return types.Header{Height: 7, Round: round, Benched: bench}
}

// The fast ChildBench returns exactly the list of the reference, for random validator sets, parent
// lists and gaps between the parent's view and its child's, short of and beyond BenchViews.
func TestChildBenchMatchesTheReference(t *testing.T) {
	const seed = 20240717
	t.Logf("seed %d", seed)
	random := rand.New(rand.NewSource(seed))
	cases := 300
	if testing.Short() {
		cases = 40
	}
	for index := 0; index < cases; index++ {
		set, description := randomBenchSet(random)
		parent := randomBenchParent(random, set)
		var gap types.Round
		switch random.Intn(10) {
		case 0, 1, 2, 3:
			gap = 1 + types.Round(random.Intn(20))
		case 4, 5, 6:
			gap = 21 + types.Round(random.Intn(280))
		case 7:
			gap = BenchViews - 1 + types.Round(random.Intn(4))
		default:
			gap = 301 + types.Round(random.Intn(2200))
		}
		round := parent.Round + gap
		if round <= parent.Round {
			round = math.MaxUint64
		}
		got := ChildBench(&parent, round, set)
		want := referenceChildBench(&parent, round, set)
		if !slices.Equal(got, want) || (got == nil) != (want == nil) {
			t.Fatalf("case %d (%s): parent view %d bench %v, child view %d: expected %v, got %v", index, description, parent.Round, parent.Benched, round, want, got)
		}
	}
	set := equalSet("a", "b", "c", "d")
	parent := types.Header{Round: 5, Benched: []types.Bench{{Validator: "a", Until: 50}}}
	for _, round := range []types.Round{0, 4, 5} {
		if bench := ChildBench(&parent, round, set); bench != nil {
			t.Fatalf("expected no list for view %d, which does not follow the parent's view %d, got %v", round, parent.Round, bench)
		}
	}
	if bench := ChildBench(nil, 9, set); bench != nil {
		t.Fatalf("expected no list for the first block, got %v", bench)
	}
}

// ChildBench leaves the parent's header and the set alone, and a list it returns is its own.
func TestChildBenchDoesNotAliasTheParentList(t *testing.T) {
	set := equalSet("a", "b", "c", "d", "e", "f", "g")
	parent := types.Header{Round: 10, Benched: []types.Bench{{Validator: "b", Until: 500}, {Validator: "d", Until: 12}}}
	before := slices.Clone(parent.Benched)
	bench := ChildBench(&parent, 40, set)
	if !slices.Equal(parent.Benched, before) {
		t.Fatalf("expected the parent's list untouched, got %v after %v", parent.Benched, before)
	}
	if len(bench) == 0 {
		t.Fatal("expected a list")
	}
	bench[0].Until = 1
	if again := ChildBench(&parent, 40, set); again[0].Until == 1 {
		t.Fatal("expected a fresh list on every call")
	}
}

// listCountingSet counts the walks of the validator list, the work ChildBench repeats for the views
// it considers.
type listCountingSet struct {
	validator.Set
	walks *int
}

func (set listCountingSet) List() []validator.Validator {
	*set.walks++
	return set.Set.List()
}

// The machine computes the benched list of a view once: every later check of a header of that view
// is answered from its memory, whether the header carries the list the parent fixes or not.
func TestTheMachineAnswersARepeatedBenchFromItsMemory(t *testing.T) {
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	walks := 0
	machine := cluster.newMachineWith(func(config *StateMachineConfig) {
		config.ValidatorSet = listCountingSet{Set: cluster.set, walks: &walks}
		config.EnforceProposerSelection = true
	})
	first := extendLed(cluster, machine, nil, 0)
	parent := first.block.Header
	headerAt := func(round types.Round) types.Header {
		return types.Header{
			Height:             2,
			PreviousBlockHash:  first.hash,
			Round:              round,
			JustifyRound:       parent.Round,
			ValidatorSetHeight: finality.FirstValidatorSetHeight,
			Benched:            ChildBench(&parent, round, cluster.set),
		}
	}

	header := headerAt(5)
	if len(header.Benched) == 0 {
		t.Fatal("expected the failed views to bench a leader")
	}
	walks = 0
	if err := machine.CheckBench(header); err != nil {
		t.Fatal(err)
	}
	if walks == 0 {
		t.Fatal("expected the first check to compute the list")
	}
	walks = 0
	unbenched := header
	unbenched.Benched = nil
	for range 20 {
		if err := machine.CheckBench(header); err != nil {
			t.Fatalf("expected the list the parent fixes accepted again, got %v", err)
		}
		if err := machine.CheckBench(unbenched); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("expected the header without the list refused again, got %v", err)
		}
	}
	if walks != 0 {
		t.Fatalf("expected the repeated checks answered from memory, got %d walks of the validators", walks)
	}

	// A view past the expiry of the bench has another list, which the memory of the first does not answer.
	late := headerAt(2 * BenchViews)
	if slices.Equal(late.Benched, header.Benched) {
		t.Fatalf("expected the lists of views 5 and %d to differ, got %v", late.Round, late.Benched)
	}
	if err := machine.CheckBench(late); err != nil {
		t.Fatalf("expected the list of view %d accepted, got %v", late.Round, err)
	}
	if err := machine.CheckBench(header); err != nil {
		t.Fatalf("expected the list of view 5 accepted after view %d, got %v", late.Round, err)
	}
	misplaced := late
	misplaced.Benched = header.Benched
	if err := machine.CheckBench(misplaced); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected the list of view 5 refused in view %d, got %v", late.Round, err)
	}
}

// Whatever the machine holds in memory, it answers as ChildBench does, for many parents, views and
// validator sets, more than the memory holds. A parent is asked about with several validator sets,
// as a header that names another set height asks about it (CheckBench), and a set with several
// parents.
func TestTheMachineMemoMatchesChildBench(t *testing.T) {
	const seed = 20240718
	t.Logf("seed %d", seed)
	random := rand.New(rand.NewSource(seed))
	machine := newThreeValidatorCluster(t).newMachine()
	// The parents are in nearby views, so that the lists of different parents are asked for the same
	// view.
	var sets []validator.Set
	var parents []types.Header
	for range 4 {
		set, _ := randomBenchSet(random)
		parent := types.Header{Height: 7, Round: 3 + types.Round(random.Intn(6))}
		for _, member := range set.List() {
			if random.Intn(3) == 0 {
				parent.Benched = append(parent.Benched, types.Bench{Validator: member.ID, Until: parent.Round - 3 + types.Round(random.Intn(int(BenchViews)+6))})
			}
		}
		slices.SortFunc(parent.Benched, func(first, second types.Bench) int {
			return strings.Compare(string(first.Validator), string(second.Validator))
		})
		sets = append(sets, set)
		parents = append(parents, parent)
	}
	queries := 6000
	if testing.Short() {
		queries = 1000
	}
	differing := make(map[string]struct{})
	for query := 0; query < queries; query++ {
		index := random.Intn(len(parents))
		parent, set := parents[index], sets[random.Intn(len(sets))]
		round := parent.Round + 1 + types.Round(random.Intn(40))
		hash := types.Hash{byte(index + 1)}
		machine.mu.Lock()
		got := machine.childBench(hash, &parent, round, set)
		remembered := machine.benches.size()
		machine.mu.Unlock()
		want := ChildBench(&parent, round, set)
		if !slices.Equal(got, want) || (got == nil) != (want == nil) {
			t.Fatalf("query %d: parent view %d bench %v, child view %d: expected %v, got %v", query, parent.Round, parent.Benched, round, want, got)
		}
		if remembered > maxChildBenches {
			t.Fatalf("query %d: the machine remembers %d lists, more than %d", query, remembered, maxChildBenches)
		}
		differing[fmt.Sprint(want)] = struct{}{}
	}
	if len(differing) < 8 {
		t.Fatalf("expected the queries to have many different answers, got %d", len(differing))
	}
	if machine.childBench(types.Hash{}, nil, 9, sets[0]) != nil {
		t.Fatal("expected no list for the first block")
	}
}

// The header of a proposal owns its benched list: a caller that changes the list it was handed does
// not change what the machine answers next, nor the check of the headers that follow.
func TestAProposalHeaderOwnsItsBenchedList(t *testing.T) {
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 1)

	// View 1 times out, and the proposal for view 2 benches its leader.
	for _, voter := range []types.ValidatorID{"a", "b", "c"} {
		_, _ = machine.OnTimeoutVote(ctx, cluster.timeoutVote(voter, 2, 1, blocks[0].qc))
	}
	first, err := machine.CreateProposal(types.Block{}, 2, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(first.Block.Header.Benched)
	if len(want) == 0 {
		t.Fatal("expected the failed view to bench its leader")
	}
	first.Block.Header.Benched[0].Until = 1

	second, err := machine.CreateProposal(types.Block{}, 2, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(second.Block.Header.Benched, want) {
		t.Fatalf("expected the list %v, got %v after a caller changed the list of an earlier proposal", want, second.Block.Header.Benched)
	}
	machine.mu.Lock()
	header, _, err := machine.nextProposalHeader(2)
	machine.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(header.Benched, want) {
		t.Fatalf("expected the list %v, got %v", want, header.Benched)
	}
	header.Benched[0].Until = 1
	if err := machine.CheckBench(second.Block.Header); err != nil {
		t.Fatalf("expected the list the parent fixes accepted after a caller changed a list, got %v", err)
	}
}

// benchedPower returns the voting power of the validators of bench in set.
func benchedPower(set validator.Set, bench []types.Bench) uint64 {
	var power uint64
	for _, entry := range bench {
		member, _ := set.Get(entry.Validator)
		power += uint64(member.VotingPower)
	}
	return power
}

// Byzantine validators holding less than a third of the voting power can grief the schedule by
// failing the views honest validators lead; the model grants them that, which a network that
// delivers messages in time does not. Each failed view benches its leader, and the model has them
// succeed in the views they lead themselves, so the chain advances on their blocks and the honest
// leaders fall into the bench.
//
// What the attack can achieve is bounded by the rule that the benched validators hold less than a
// third of the power. It can bench honest validators up to just under a third of the power: for
// BenchViews views after their last failure they lead none of the views that directly follow a
// block, and the Byzantine validators lead a larger share of those views than their power. But the
// validators that stay hold more than two thirds of the power, so that share stays below a half
// and the honest validators that stay lead the rest.
//
// What it cannot achieve is a view without a leader, a bench of a third of the power, the bench of
// a Byzantine validator (its views succeed), the leadership of a view that follows a timeout
// certificate (the whole set leads it), or a bench that outlives the attack by more than BenchViews
// views. Failing a view costs the network a timeout whether or not its leader is benched: the
// bench keeps a validator that fails its view from failing the next ones, and does not make the
// failures cheaper.
func TestByzantineValidatorsCannotBenchAThirdOfThePowerOrSilenceTheHonest(t *testing.T) {
	validators := func(power types.VotingPower, ids ...types.ValidatorID) []validator.Validator {
		members := make([]validator.Validator, 0, len(ids))
		for _, id := range ids {
			members = append(members, validator.Validator{ID: id, VotingPower: power})
		}
		return members
	}
	unequalHonest := []validator.Validator{{ID: "a", VotingPower: 10}, {ID: "b", VotingPower: 7}, {ID: "c", VotingPower: 5}, {ID: "d", VotingPower: 3}, {ID: "e", VotingPower: 2}}
	unequalByzantine := []validator.Validator{{ID: "x", VotingPower: 6}, {ID: "y", VotingPower: 3}}
	tests := []struct {
		name      string
		honest    []validator.Validator
		byzantine []validator.Validator
		// mostBenched is the most voting power the attack benches, zero where it depends on the
		// order of the failures.
		mostBenched uint64
	}{
		{name: "four equal validators, one Byzantine", honest: validators(1, "a", "b", "c"), byzantine: validators(1, "d"), mostBenched: 1},
		{name: "seven equal validators, two Byzantine", honest: validators(1, "a", "b", "c", "d", "e"), byzantine: validators(1, "f", "g"), mostBenched: 2},
		{name: "ten equal validators, three Byzantine", honest: validators(1, "a", "b", "c", "d", "e", "f", "g"), byzantine: validators(1, "h", "i", "j"), mostBenched: 3},
		{name: "unequal validators", honest: unequalHonest, byzantine: unequalByzantine},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set := validator.NewSet(append(slices.Clone(test.honest), test.byzantine...))
			total := uint64(set.TotalVotingPower())
			byzantine := make(map[types.ValidatorID]bool)
			var byzantinePower uint64
			for _, member := range test.byzantine {
				byzantine[member.ID] = true
				byzantinePower += uint64(member.VotingPower)
			}
			if 3*byzantinePower >= total {
				t.Fatalf("the Byzantine validators hold %d of %d, not less than a third", byzantinePower, total)
			}

			// The attack: the views of honest leaders fail, the views of Byzantine leaders succeed.
			tip := types.Header{Height: 1}
			var lastFailed types.Round
			var mostBenched uint64
			var worstBench []types.Bench
			for view := types.Round(1); view <= 3*BenchViews; view++ {
				header := types.Header{Height: tip.Height + 1, Round: view, JustifyRound: tip.Round, Benched: ChildBench(&tip, view, set)}
				power := benchedPower(set, header.Benched)
				if 3*power >= total {
					t.Fatalf("view %d: the bench %v holds %d of %d, a third of the power", view, header.Benched, power, total)
				}
				if power > mostBenched {
					mostBenched, worstBench = power, header.Benched
				}
				for _, entry := range header.Benched {
					if byzantine[entry.Validator] {
						t.Fatalf("view %d: %s is benched, but every view it leads succeeds", view, entry.Validator)
					}
					if entry.Until >= view+BenchViews {
						t.Fatalf("view %d: %s is benched through view %d, more than %d views after a failure", view, entry.Validator, entry.Until, BenchViews)
					}
				}
				leader, ok := Leader(header, set)
				if !ok {
					t.Fatalf("view %d has no leader", view)
				}
				if byzantine[leader] {
					tip = header
					continue
				}
				lastFailed = view
			}
			if mostBenched == 0 {
				t.Fatal("expected the attack to bench honest validators")
			}
			if test.mostBenched != 0 && mostBenched != test.mostBenched {
				t.Fatalf("expected the attack to bench %d of %d at most, got %d", test.mostBenched, total, mostBenched)
			}

			// At its worst the Byzantine validators lead the share of the views that directly follow a
			// block their power has among the unbenched, which is below a half.
			unbenched := total - mostBenched
			var byzantineLed, honestLed uint64
			for round := types.Round(1 << 20); round < 1<<20+types.Round(unbenched); round++ {
				leader, ok := Leader(types.Header{Round: round, JustifyRound: round - 1, Benched: worstBench}, set)
				if !ok || isBenched(worstBench, leader) {
					t.Fatalf("view %d: leader %q (found %t) is benched or missing", round, leader, ok)
				}
				if byzantine[leader] {
					byzantineLed++
				} else {
					honestLed++
				}
			}
			if byzantineLed != byzantinePower || 2*byzantineLed >= unbenched || honestLed == 0 {
				t.Fatalf("expected the Byzantine validators to lead %d of %d views, below a half, and the honest ones the rest, got %d and %d", byzantinePower, unbenched, byzantineLed, honestLed)
			}

			// The views succeed again: every entry expires BenchViews views after the last failure.
			for view := 3*BenchViews + 1; view <= lastFailed+BenchViews+2; view++ {
				header := types.Header{Height: tip.Height + 1, Round: view, JustifyRound: tip.Round, Benched: ChildBench(&tip, view, set)}
				for _, entry := range header.Benched {
					if entry.Until > lastFailed+BenchViews {
						t.Fatalf("view %d: %s is benched through view %d, after the last failure in view %d plus %d", view, entry.Validator, entry.Until, lastFailed, BenchViews)
					}
				}
				if view > lastFailed+BenchViews && len(header.Benched) != 0 {
					t.Fatalf("view %d: expected the bench to expire %d views after the last failure in view %d, got %v", view, BenchViews, lastFailed, header.Benched)
				}
				if _, ok := Leader(header, set); !ok {
					t.Fatalf("view %d has no leader", view)
				}
				tip = header
			}
		})
	}
}

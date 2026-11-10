package consensus

import (
	"context"
	"errors"
	"slices"
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

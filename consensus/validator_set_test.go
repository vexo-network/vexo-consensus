package consensus

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// registryCluster is a cluster whose validators also back an in-memory registry, so the machine
// reads its validator sets from the registry the way a node does.
type registryCluster struct {
	*testCluster
	registry *validator.InMemoryRegistry
}

func newRegistryCluster(t *testing.T, ids ...types.ValidatorID) *registryCluster {
	t.Helper()
	cluster := newEqualTestCluster(t, ids...)
	registry, err := validator.NewInMemoryRegistry(nil, cluster.validators(ids...))
	if err != nil {
		t.Fatal(err)
	}
	return &registryCluster{testCluster: cluster, registry: registry}
}

// newMachine builds a machine that has executed nothing yet: it holds the registry set of height 1.
func (cluster *registryCluster) newMachine() *StateMachine {
	cluster.t.Helper()
	genesis, err := cluster.registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		cluster.t.Fatal(err)
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:           testChainID,
		ValidatorSet:      genesis,
		Signatures:        vexocrypto.DeterministicSigner{},
		Aggregator:        vexocrypto.DeterministicAggregateSigner{},
		AggregateVerifier: vexocrypto.DeterministicAggregateSigner{},
	})
	if err != nil {
		cluster.t.Fatal(err)
	}
	cluster.executed(machine, 1)
	return machine
}

// executed records that the machine's node executed every block below registryHeight.
func (cluster *registryCluster) executed(machine *StateMachine, registryHeight types.Height) {
	cluster.t.Helper()
	if err := machine.UpdateValidatorSetFromRegistry(context.Background(), cluster.registry, registryHeight); err != nil {
		cluster.t.Fatal(err)
	}
}

// propose builds the next block on the machine's high QC with CreateProposal, delivers it signed
// by proposer, and returns the accepted proposal.
func (cluster *registryCluster) propose(machine *StateMachine, round types.Round, proposer types.ValidatorID) Proposal {
	cluster.t.Helper()
	proposal, err := machine.CreateProposal(types.Block{}, round, proposer, finality.QuorumCert{})
	if err != nil {
		cluster.t.Fatalf("create proposal in view %d: %v", round, err)
	}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		cluster.t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		cluster.t.Fatalf("proposal in view %d: %v", round, err)
	}
	cluster.headers[HashBlock(proposal.Block)] = proposal.Block.Header
	return proposal
}

// certify delivers the votes of voters for the proposal until they form a certificate.
func (cluster *registryCluster) certify(machine *StateMachine, proposal Proposal, voters ...types.ValidatorID) finality.QuorumCert {
	cluster.t.Helper()
	height := proposal.Block.Header.Height
	blockHash := HashBlock(proposal.Block)
	for _, voter := range voters {
		if err := machine.OnVote(context.Background(), cluster.vote(voter, height, proposal.Round, blockHash)); err != nil {
			cluster.t.Fatalf("vote of %s for height %d: %v", voter, height, err)
		}
		if qc, err := machine.BuildQuorumCert(height, proposal.Round, blockHash); err == nil {
			return qc
		}
	}
	cluster.t.Fatalf("no certificate for height %d", height)
	return finality.QuorumCert{}
}

// TestValidatorSetChangesOnlyWhereTheChainProvesTheCommit is the regression for the chain halt
// after a validator set update. The node executes block H once it is final and the registry holds
// the new set from registry height H+1, but in the pipelined 2-chain protocol block H+1 is already
// proposed and certified under the old set by then. Installing the new set for height H+1 made
// every node reject the next proposal with a validator set hash mismatch. The set of a block now
// derives from its parent header: it changes exactly at the child of a block proposed in the view
// directly after its justify certificate, whose ancestry proves the commit of the block the set
// depends on.
func TestValidatorSetChangesOnlyWhereTheChainProvesTheCommit(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c")
	machine := cluster.newMachine()
	oldSet, err := cluster.registry.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	first := cluster.propose(machine, 0, "a")
	cluster.certify(machine, first, "a", "b", "c")
	second := cluster.propose(machine, 1, "b")
	if second.Block.Header.ValidatorSetHeight != 1 {
		t.Fatalf("the second block must keep the genesis set, got registry height %d", second.Block.Header.ValidatorSetHeight)
	}
	secondQC := cluster.certify(machine, second, "a", "b", "c")
	// A follower sees only the proposals: it learns the certificate of the second block from the
	// third proposal.
	follower := cluster.newMachine()
	for _, proposal := range []Proposal{first, second} {
		if err := follower.OnProposal(ctx, proposal); err != nil {
			t.Fatal(err)
		}
	}
	if decisions := follower.CommitDecisions(); len(decisions) != 0 {
		t.Fatalf("nothing is final for the follower before the third block, got %+v", decisions)
	}

	// Executing the first block removes "a": the registry holds the new set from height 2.
	if err := cluster.registry.ApplyLeaveAt(ctx, 2, "a"); err != nil {
		t.Fatal(err)
	}
	newSet, err := cluster.registry.ValidatorSet(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	// The third block extends a parent proposed directly after its justify view, so it carries
	// the commit of the first block and is certified by the set after it. Until the first block is
	// executed the set is pending, but the certificate the proposal carries is observed, and that
	// is what finalizes the first block.
	third := cluster.block(3, HashBlock(second.Block))
	third.Header.ValidatorSetHash = newSet.Hash()
	third = cluster.seal(third, 2, secondQC.Round)
	if third.Header.ValidatorSetHeight != 2 {
		t.Fatalf("the third block must name the set after the first block, got registry height %d", third.Header.ValidatorSetHeight)
	}
	pending := cluster.proposal("b", third, 2, secondQC, finality.TimeoutCert{})
	if err := follower.OnProposal(ctx, pending); !errors.Is(err, ErrValidatorSetPending) {
		t.Fatalf("expected the set of the third block to be pending, got %v", err)
	}
	if decisions := follower.CommitDecisions(); len(decisions) != 1 || decisions[0].CommittedHeight != 1 {
		t.Fatalf("the carried certificate must finalize the first block, got %+v", decisions)
	}
	cluster.executed(follower, 2)
	if err := follower.OnProposal(ctx, pending); err != nil {
		t.Fatalf("the same proposal is accepted once the first block is executed, got %v", err)
	}

	if _, err := machine.CreateProposal(types.Block{}, 2, "b", finality.QuorumCert{}); !errors.Is(err, ErrValidatorSetPending) {
		t.Fatalf("a proposer must not build on a set it has not executed, got %v", err)
	}
	cluster.executed(machine, 2)
	if machine.Status(ctx).ValidatorSetHash != newSet.Hash() {
		t.Fatal("the machine reports the newest executed set")
	}
	if err := machine.OnProposal(ctx, pending); err != nil {
		t.Fatalf("the proposer's machine accepts the proposal once the first block is executed, got %v", err)
	}
	if pending.Block.Header.ValidatorSetHash == oldSet.Hash() {
		t.Fatal("setup: the sets must differ")
	}

	// The removed validator has no say in the third block; the remaining two certify it.
	thirdHash := HashBlock(pending.Block)
	if err := machine.OnVote(ctx, cluster.vote("a", 3, 2, thirdHash)); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected the removed validator's vote to be refused, got %v", err)
	}
	thirdQC := cluster.certify(machine, pending, "b", "c")
	if err := machine.VerifyQuorumCert(thirdQC); err != nil {
		t.Fatalf("the new set certifies the third block: %v", err)
	}
	// The second block stays certified by the set that certified it.
	if err := machine.VerifyQuorumCert(secondQC); err != nil {
		t.Fatalf("the second block keeps the old set: %v", err)
	}
	if err := machine.VerifyQuorumCert(cluster.qc(2, 1, HashBlock(second.Block), "a", "b")); err != nil {
		t.Fatalf("the removed validator still counts for the second block, got %v", err)
	}
	if err := machine.VerifyQuorumCert(cluster.qc(3, 2, thirdHash, "a", "b")); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("the removed validator must not count for the third block, got %v", err)
	}

	// A proposal that names the old set where the chain fixes the new one is invalid.
	stale := cluster.block(4, thirdHash)
	stale = cluster.seal(stale, 3, thirdQC.Round)
	stale.Header.ValidatorSetHeight = 1
	stale.Header.ValidatorSetHash = oldSet.Hash()
	staleProposal := Proposal{Block: stale, Round: 3, Proposer: "c", JustifyQC: thirdQC}
	if err := cluster.signers.signProposal(&staleProposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(ctx, staleProposal); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a block naming a superseded set to be invalid, got %v", err)
	}
	// The second block is final now and executed; the fourth block names the set after it.
	cluster.executed(machine, 3)
	fourth := cluster.propose(machine, 3, "c")
	if fourth.Block.Header.ValidatorSetHeight != 3 || fourth.Block.Header.ValidatorSetHash != newSet.Hash() {
		t.Fatalf("expected the fourth block on the set after the second block, got %+v", fourth.Block.Header)
	}
}

// A parent proposed after a timeout does not prove the commit of its own parent, so its child
// keeps the parent's set even when the node has executed further.
func TestValidatorSetDoesNotChangeBelowAnUnprovenCommit(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c")
	machine := cluster.newMachine()
	oldSet, err := cluster.registry.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	first := cluster.propose(machine, 0, "a")
	firstQC := cluster.certify(machine, first, "a", "b", "c")
	for _, voter := range []types.ValidatorID{"a", "b", "c"} {
		_, _ = machine.OnTimeoutVote(ctx, cluster.timeoutVote(voter, 2, 1, firstQC))
	}
	second := cluster.propose(machine, 2, "c")
	if second.TimeoutCert.Round != 1 {
		t.Fatalf("setup: the second block must be proposed after the timeout of view 1, got %+v", second.TimeoutCert)
	}
	cluster.certify(machine, second, "a", "b", "c")

	if err := cluster.registry.ApplyLeaveAt(ctx, 2, "a"); err != nil {
		t.Fatal(err)
	}
	cluster.executed(machine, 2)
	third := cluster.propose(machine, 3, "a")
	if third.Block.Header.ValidatorSetHeight != 1 || third.Block.Header.ValidatorSetHash != oldSet.Hash() {
		t.Fatalf("the third block must keep the genesis set, got %+v", third.Block.Header)
	}
}

func TestValidatorSetHistoryIsFilledWithoutGapsAndBounded(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c", "d")
	machine := cluster.newMachine()

	// One update per height: every registry height the machine records keeps its own set.
	last := types.Height(maxValidatorSetHistory) * 2
	for height := types.Height(2); height <= last; height++ {
		power := types.VotingPower(1 + height%2)
		if err := cluster.registry.UpdateVotingPowerAt(ctx, height, "d", power); err != nil {
			t.Fatal(err)
		}
	}
	// The node records heights in one call after a gap; none of them may be skipped.
	cluster.executed(machine, last)
	if len(machine.validatorSets) > maxValidatorSetHistory {
		t.Fatalf("the validator set history must stay bounded, got %d entries", len(machine.validatorSets))
	}
	for _, height := range []types.Height{last, last - 1, last - types.Height(maxValidatorSetHistory) + 1} {
		want, err := cluster.registry.ValidatorSet(ctx, height)
		if err != nil {
			t.Fatal(err)
		}
		got, err := machine.ValidatorSetAt(height)
		if err != nil || got.Hash() != want.Hash() {
			t.Fatalf("registry height %d: expected set %x, got %v %v", height, want.Hash(), got, err)
		}
	}
	if _, err := machine.ValidatorSetAt(2); !errors.Is(err, ErrUnknownValidatorSet) {
		t.Fatalf("expected a set older than the history to be unknown, got %v", err)
	}
	if _, err := machine.ValidatorSetAt(last + 1); !errors.Is(err, ErrValidatorSetPending) {
		t.Fatalf("expected a set after an unexecuted block to be pending, got %v", err)
	}
}

func TestStateMachineValidatorRemovalAppliesToTimeoutVotesOfItsHeights(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c")
	machine := cluster.newMachine()
	machine.StartRound(1, 0)

	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 1, 0, finality.QuorumCert{})); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := cluster.registry.ApplyLeaveAt(ctx, 2, "a"); err != nil {
		t.Fatal(err)
	}
	cluster.executed(machine, 2)

	// A block of height 2 can still name the old set; from height 3 on the newest executed set
	// is the one a block can name.
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 1, 0, finality.QuorumCert{})); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected the replayed vote at the old height to stay idempotent, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 3, 1, finality.QuorumCert{})); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected removed timeout voter rejected at the new heights, got %v", err)
	}
}

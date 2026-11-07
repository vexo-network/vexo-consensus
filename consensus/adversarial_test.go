package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestAdversarialRunnerOfflineMinorityStillReachesQuorum(t *testing.T) {
	machine, runner, _ := newAdversarialRunner(t, equalValidators("a", "b", "c"))
	machine.StartRound(1, 0)

	proposed, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	qc, err := runner.VoteWith(context.Background(), proposed.BlockHash, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if qc.VotingPower != 2 {
		t.Fatalf("expected quorum with two validators, got %d", qc.VotingPower)
	}
	if err := machine.VerifyQuorumCert(qc); err != nil {
		t.Fatalf("the quorum cert of the runner must verify: %v", err)
	}
}

func TestAdversarialRunnerOfflineMajorityPreventsQuorum(t *testing.T) {
	machine, runner, _ := newAdversarialRunner(t, equalValidators("a", "b", "c"))
	machine.StartRound(1, 0)

	proposed, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.VoteWith(context.Background(), proposed.BlockHash, "a")
	if !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if machine.Status(context.Background()).Phase == PhaseCommit {
		t.Fatal("single vote must not enter commit phase")
	}
}

func TestAdversarialRunnerWeightedOfflineQuorum(t *testing.T) {
	machine, runner, _ := newAdversarialRunner(t, []validator.Validator{
		{ID: "a", VotingPower: 4},
		{ID: "b", VotingPower: 2},
		{ID: "c", VotingPower: 1},
	})
	machine.StartRound(1, 0)

	proposed, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.VoteWith(context.Background(), proposed.BlockHash, "a"); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum with 4/7 power, got %v", err)
	}
	qc, err := runner.VoteWith(context.Background(), proposed.BlockHash, "b")
	if err != nil {
		t.Fatal(err)
	}
	if qc.VotingPower != 6 {
		t.Fatalf("expected quorum with 6/7 power, got %d", qc.VotingPower)
	}
}

func TestAdversarialRunnerConflictingVoteProducesEvidence(t *testing.T) {
	machine, runner, _ := newAdversarialRunner(t, equalValidators("a", "b", "c"))
	machine.StartRound(1, 0)

	first, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("first")}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	// A second block in the same view is refused outright; it exists only as the target of
	// the equivocating vote.
	if _, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("second")}}, "a"); !errors.Is(err, ErrConflictingProposal) {
		t.Fatalf("expected a second block in the same view to be refused, got %v", err)
	}
	secondProposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("second")}}, 0, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}

	err = runner.VoteConflict(context.Background(), "a", first.BlockHash, machine.hashBlock(secondProposal.Block))
	if !errors.Is(err, ErrConflictingVote) {
		t.Fatalf("expected conflicting vote, got %v", err)
	}
	evidence := runner.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
	}
}

func TestAdversarialRunnerTimeoutEquivocationRejected(t *testing.T) {
	machine, runner, cluster := newAdversarialRunner(t, equalValidators("a", "b", "c"))
	machine.StartRound(2, 1)

	err := runner.TimeoutEquivocation(
		context.Background(),
		"a",
		cluster.qc(1, 0, types.Hash{1}),
		finality.QuorumCert{},
	)
	if !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}
	if len(runner.Evidence()) != 1 {
		t.Fatalf("expected the equivocation to become evidence, got %d", len(runner.Evidence()))
	}
}

func TestAdversarialRunnerTimeoutQuorumWithOfflineMinority(t *testing.T) {
	machine, runner, cluster := newAdversarialRunner(t, equalValidators("a", "b", "c"))
	machine.StartRound(2, 1)

	highQC := cluster.qc(1, 0, types.Hash{1})
	timeoutCert, err := runner.TimeoutWith(context.Background(), highQC, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != highQC.BlockHash {
		t.Fatalf("expected timeout cert high qc, got %+v", timeoutCert.HighQC)
	}
	if machine.Status(context.Background()).Round != 2 {
		t.Fatal("expected timeout quorum to advance round")
	}
}

func TestAdversarialRunnerRefusesUnknownSigner(t *testing.T) {
	machine, runner, _ := newAdversarialRunner(t, equalValidators("a", "b", "c"))
	machine.StartRound(1, 0)

	proposed, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.VoteWith(context.Background(), proposed.BlockHash, "stranger"); !errors.Is(err, ErrMissingSigner) {
		t.Fatalf("expected a voter without a signing key to be refused, got %v", err)
	}
}

func equalValidators(ids ...types.ValidatorID) []validator.Validator {
	validators := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		validators = append(validators, validator.Validator{ID: id, VotingPower: 1})
	}
	return validators
}

func newAdversarialRunner(t *testing.T, validators []validator.Validator) (*StateMachine, *AdversarialRunner, *testCluster) {
	t.Helper()
	cluster := newTestCluster(t, validators...)
	machine := cluster.newMachine()
	runner, err := NewAdversarialRunner(machine, cluster.signers)
	if err != nil {
		t.Fatal(err)
	}
	return machine, runner, cluster
}

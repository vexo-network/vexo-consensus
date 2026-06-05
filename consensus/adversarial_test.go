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
	machine, runner := newAdversarialRunner(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
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
}

func TestAdversarialRunnerOfflineMajorityPreventsQuorum(t *testing.T) {
	machine, runner := newAdversarialRunner(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
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
	machine, runner := newAdversarialRunner(t, []validator.Validator{
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
	machine, runner := newAdversarialRunner(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine.StartRound(1, 0)

	first, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("first")}}, "a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Propose(context.Background(), types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("second")}}, "a")
	if err != nil {
		t.Fatal(err)
	}

	err = runner.VoteConflict(context.Background(), "a", first.BlockHash, second.BlockHash)
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
	machine, runner := newAdversarialRunner(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine.StartRound(1, 0)

	err := runner.TimeoutEquivocation(
		context.Background(),
		"a",
		finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}},
		finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}},
	)
	if !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}
}

func TestAdversarialRunnerTimeoutQuorumWithOfflineMinority(t *testing.T) {
	machine, runner := newAdversarialRunner(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine.StartRound(1, 0)

	highQC := finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}
	timeoutCert, err := runner.TimeoutWith(context.Background(), highQC, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != highQC.BlockHash {
		t.Fatalf("expected timeout cert high qc, got %+v", timeoutCert.HighQC)
	}
	if machine.Status(context.Background()).Round != 1 {
		t.Fatal("expected timeout quorum to advance round")
	}
}

func newAdversarialRunner(t *testing.T, validators []validator.Validator) (*StateMachine, *AdversarialRunner) {
	t.Helper()
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: newTestValidatorSet(validators),
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewAdversarialRunner(machine)
	if err != nil {
		t.Fatal(err)
	}
	return machine, runner
}

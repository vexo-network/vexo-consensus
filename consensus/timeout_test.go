package consensus

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestTimeoutCollectorBuildsTimeoutCert(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	if err := collector.AddVote(testCollectorTimeoutVote(1, 2, "a", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(1, 2); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(1, 2, "b", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := collector.BuildTimeoutCert(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.Height != 1 || timeoutCert.Round != 2 {
		t.Fatalf("unexpected timeout cert: %+v", timeoutCert)
	}
}

func TestTimeoutCollectorSelectsHighestQC(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	lowerQC := finality.QuorumCert{Height: 2, Round: 3, BlockHash: types.Hash{2}}
	higherQC := finality.QuorumCert{Height: 3, Round: 0, BlockHash: types.Hash{3}}
	if err := collector.AddVote(testCollectorTimeoutVote(4, 1, "a", lowerQC)); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(4, 1, "b", higherQC)); err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := collector.BuildTimeoutCert(4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != higherQC.BlockHash {
		t.Fatalf("expected highest qc in timeout cert, got %+v", timeoutCert.HighQC)
	}
}

func TestTimeoutCollectorUsesRoundTieBreakerForHighestQC(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	lowerRound := finality.QuorumCert{Height: 2, Round: 1, BlockHash: types.Hash{1}}
	higherRound := finality.QuorumCert{Height: 2, Round: 2, BlockHash: types.Hash{2}}
	if err := collector.AddVote(testCollectorTimeoutVote(3, 1, "a", lowerRound)); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(3, 1, "b", higherRound)); err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := collector.BuildTimeoutCert(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != higherRound.BlockHash {
		t.Fatalf("expected higher round qc, got %+v", timeoutCert.HighQC)
	}
}

func TestTimeoutCollectorWeightedQuorum(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 4},
		{ID: "b", VotingPower: 2},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	if err := collector.AddVote(testCollectorTimeoutVote(1, 0, "a", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(1, 0); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(1, 0, "b", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(1, 0); err != nil {
		t.Fatalf("expected weighted quorum, got %v", err)
	}
}

func TestTimeoutCollectorRejectsUnknownValidator(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	err := collector.AddVote(TimeoutVote{Height: 1, Round: 0, ValidatorID: "unknown"})
	if !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected unknown validator, got %v", err)
	}
}

func TestTimeoutCollectorRejectsConflictingTimeoutVote(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	first := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{BlockHash: types.Hash{1}}}
	second := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{BlockHash: types.Hash{2}}}
	if err := collector.AddVote(first); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(second); !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}
	previous, found := collector.ConflictingVote(second)
	if !found {
		t.Fatal("expected conflicting previous vote")
	}
	if previous.HighQC.BlockHash != first.HighQC.BlockHash {
		t.Fatalf("unexpected previous timeout vote: %+v", previous)
	}
}

func TestTimeoutCollectorAllowsRepeatedSameTimeoutVote(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	collector := NewTimeoutCollectorWithAggregator(set, testAggregateSigner{})

	vote := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 1, BlockHash: types.Hash{1}}}
	if err := collector.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(vote); err != nil {
		t.Fatalf("expected idempotent timeout vote, got %v", err)
	}
}

func TestPacemakerAdvancesRoundAndHeight(t *testing.T) {
	pacemaker := NewPacemaker(1, 2)
	if err := pacemaker.AdvanceRound(finality.TimeoutCert{Height: 1, Round: 2}); err != nil {
		t.Fatal(err)
	}
	if pacemaker.Round() != 3 {
		t.Fatalf("expected round 3, got %d", pacemaker.Round())
	}
	pacemaker.AdvanceHeight(2)
	if pacemaker.Height() != 2 || pacemaker.Round() != 0 {
		t.Fatalf("unexpected pacemaker state height=%d round=%d", pacemaker.Height(), pacemaker.Round())
	}
}

func TestPacemakerRejectsStaleTimeoutCert(t *testing.T) {
	pacemaker := NewPacemaker(2, 3)
	err := pacemaker.AdvanceRound(finality.TimeoutCert{Height: 1, Round: 3})
	if !errors.Is(err, ErrStaleTimeoutCert) {
		t.Fatalf("expected stale timeout cert by height, got %v", err)
	}
	err = pacemaker.AdvanceRound(finality.TimeoutCert{Height: 2, Round: 2})
	if !errors.Is(err, ErrStaleTimeoutCert) {
		t.Fatalf("expected stale timeout cert by round, got %v", err)
	}
}

func testCollectorTimeoutVote(height types.Height, round types.Round, validatorID types.ValidatorID, highQC finality.QuorumCert) TimeoutVote {
	vote := TimeoutVote{Height: height, Round: round, ValidatorID: validatorID, HighQC: highQC}
	vote.Signature = unsignedConsensusSignature("test-timeout-vote", validatorID, TimeoutVoteSignBytes(vote))
	return vote
}

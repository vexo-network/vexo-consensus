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
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	if err := collector.AddVote(testCollectorTimeoutVote(1, 2, "a", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(2); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(1, 2, "b", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := collector.BuildTimeoutCert(2)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.Height != 1 || timeoutCert.Round != 2 {
		t.Fatalf("unexpected timeout cert: %+v", timeoutCert)
	}
	if string(timeoutCert.Signers) != "a,b" {
		t.Fatalf("expected sorted signers, got %q", timeoutCert.Signers)
	}
}

func TestTimeoutCollectorKeepsViewsApart(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	// The same validators time out at different heights in different global views: the
	// view, not the height, groups timeout votes.
	if err := collector.AddVote(testCollectorTimeoutVote(1, 2, "a", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(2, 3, "b", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(2); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("one vote in view 2 is no quorum, got %v", err)
	}
	if _, err := collector.BuildTimeoutCert(3); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("one vote in view 3 is no quorum, got %v", err)
	}
}

func TestTimeoutCollectorSelectsHighestViewQC(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	// Views are global: the certificate formed in the later view is the newer one even
	// though it certifies a lower height on another fork.
	lowerView := finality.QuorumCert{Height: 3, Round: 0, BlockHash: types.Hash{3}}
	higherView := finality.QuorumCert{Height: 2, Round: 3, BlockHash: types.Hash{2}}
	if err := collector.AddVote(testCollectorTimeoutVote(4, 5, "a", lowerView)); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(4, 5, "b", higherView)); err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := collector.BuildTimeoutCert(5)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != higherView.BlockHash {
		t.Fatalf("expected the qc of the highest view in timeout cert, got %+v", timeoutCert.HighQC)
	}
}

func TestTimeoutCollectorUsesHeightTieBreakerForHighestQC(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	lowerHeight := finality.QuorumCert{Height: 2, Round: 1, BlockHash: types.Hash{1}}
	higherHeight := finality.QuorumCert{Height: 3, Round: 1, BlockHash: types.Hash{2}}
	if err := collector.AddVote(testCollectorTimeoutVote(4, 2, "a", lowerHeight)); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(4, 2, "b", higherHeight)); err != nil {
		t.Fatal(err)
	}

	timeoutCert, err := collector.BuildTimeoutCert(2)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != higherHeight.BlockHash {
		t.Fatalf("expected higher height qc, got %+v", timeoutCert.HighQC)
	}
}

func TestTimeoutCollectorGroupsVotesBySignedMessage(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	first := finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}
	second := finality.QuorumCert{Height: 1, Round: 1, BlockHash: types.Hash{2}}
	for _, vote := range []TimeoutVote{
		testCollectorTimeoutVote(2, 4, "a", first),
		testCollectorTimeoutVote(2, 4, "b", second),
		testCollectorTimeoutVote(2, 4, "c", second),
	} {
		if err := collector.AddVote(vote); err != nil {
			t.Fatal(err)
		}
	}
	timeoutCert, err := collector.BuildTimeoutCert(4)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := decodeTimeoutSignatureGroups(timeoutCert.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("votes that signed different messages need separate signatures, got %d groups", len(groups))
	}
	signed := 0
	for _, group := range groups {
		signed += len(group.Signers)
	}
	if signed != 3 {
		t.Fatalf("every signer belongs to exactly one group, got %d", signed)
	}
}

func TestTimeoutCollectorPruneForgetsOldViews(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	collector := NewTimeoutCollector(set, testAggregateSigner{})
	for round := types.Round(0); round < 5; round++ {
		if err := collector.AddVote(testCollectorTimeoutVote(1, round, "a", finality.QuorumCert{})); err != nil {
			t.Fatal(err)
		}
	}

	collector.Prune(3)
	if len(collector.votes) != 2 {
		t.Fatalf("expected views 3 and 4 to remain, got %d views", len(collector.votes))
	}
	if _, found := collector.ConflictingVote(testCollectorTimeoutVote(1, 1, "a", finality.QuorumCert{Height: 1, BlockHash: types.Hash{1}})); found {
		t.Fatal("a pruned view must not report conflicts")
	}
}

func TestTimeoutCollectorWeightedQuorum(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 4},
		{ID: "b", VotingPower: 2},
		{ID: "c", VotingPower: 1},
	})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	if err := collector.AddVote(testCollectorTimeoutVote(1, 0, "a", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(0); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(1, 0, "b", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.BuildTimeoutCert(0); err != nil {
		t.Fatalf("expected weighted quorum, got %v", err)
	}
}

func TestTimeoutCollectorRejectsVotingPowerOverflow(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: ^types.VotingPower(0)},
		{ID: "b", VotingPower: 1},
	})
	collector := NewTimeoutCollector(set, testAggregateSigner{})
	if err := collector.AddVote(testCollectorTimeoutVote(1, 0, "a", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	if err := collector.AddVote(testCollectorTimeoutVote(1, 0, "b", finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}

	if _, err := collector.BuildTimeoutCert(0); !errors.Is(err, types.ErrVotingPowerOverflow) {
		t.Fatalf("expected voting power overflow, got %v", err)
	}
}

func TestTimeoutCollectorRejectsUnknownValidator(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

	err := collector.AddVote(TimeoutVote{Height: 1, Round: 0, ValidatorID: "unknown"})
	if !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected unknown validator, got %v", err)
	}
}

func TestTimeoutCollectorRejectsConflictingTimeoutVote(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	collector := NewTimeoutCollector(set, testAggregateSigner{})

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
	collector := NewTimeoutCollector(set, testAggregateSigner{})

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
	if pacemaker.Height() != 2 || pacemaker.Round() != 3 {
		t.Fatalf("views are global and must survive a height change, got height=%d round=%d", pacemaker.Height(), pacemaker.Round())
	}
}

func TestPacemakerNeverMovesBackwards(t *testing.T) {
	pacemaker := NewPacemaker(5, 7)

	pacemaker.AdvanceHeight(3)
	if pacemaker.Height() != 5 {
		t.Fatalf("the height must not decrease, got %d", pacemaker.Height())
	}
	if pacemaker.Advance(4, 6) {
		t.Fatal("an older height and view must not move the pacemaker")
	}
	if !pacemaker.Advance(5, 8) || pacemaker.Height() != 5 || pacemaker.Round() != 8 {
		t.Fatalf("a newer view must move the pacemaker, got height=%d round=%d", pacemaker.Height(), pacemaker.Round())
	}
	if !pacemaker.Advance(9, 8) || pacemaker.Height() != 9 || pacemaker.Round() != 8 {
		t.Fatalf("a newer height must move the pacemaker, got height=%d round=%d", pacemaker.Height(), pacemaker.Round())
	}
}

func TestPacemakerRejectsStaleTimeoutCert(t *testing.T) {
	pacemaker := NewPacemaker(2, 3)
	err := pacemaker.AdvanceRound(finality.TimeoutCert{Height: 2, Round: 2})
	if !errors.Is(err, ErrStaleTimeoutCert) {
		t.Fatalf("expected stale timeout cert by view, got %v", err)
	}
	err = pacemaker.AdvanceRound(finality.TimeoutCert{Height: 2, Round: ^types.Round(0)})
	if !errors.Is(err, ErrStaleTimeoutCert) {
		t.Fatalf("a timeout cert for the last representable view must not overflow the view, got %v", err)
	}
	if pacemaker.Round() != 3 {
		t.Fatalf("stale timeout certs must not move the pacemaker, got round %d", pacemaker.Round())
	}
}

func testCollectorTimeoutVote(height types.Height, round types.Round, validatorID types.ValidatorID, highQC finality.QuorumCert) TimeoutVote {
	return TimeoutVote{
		Height:      height,
		Round:       round,
		ValidatorID: validatorID,
		HighQC:      highQC,
		Signature:   types.Signature("signature-of-" + string(validatorID)),
	}
}

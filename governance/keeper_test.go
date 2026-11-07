package governance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestInMemoryKeeperSubmitVoteAndExecute(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)

	if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), id, "bob", VoteYes); err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(15)

	if err := keeper.Execute(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(keeper.AppliedChanges()) != 1 {
		t.Fatalf("expected one applied change")
	}
}

func TestInMemoryKeeperRejectsInvalidProposal(t *testing.T) {
	keeper := newTestKeeper()
	cases := []struct {
		name     string
		proposal Proposal
		expected error
	}{
		{
			name:     "missing title",
			proposal: Proposal{Submitter: "alice", Changes: []ParameterChange{{Module: "consensus", Key: "x"}}},
			expected: ErrMissingProposalTitle,
		},
		{
			name:     "missing submitter",
			proposal: Proposal{Title: "change", Changes: []ParameterChange{{Module: "consensus", Key: "x"}}},
			expected: ErrMissingSubmitter,
		},
		{
			name:     "no changes",
			proposal: Proposal{Title: "change", Submitter: "alice"},
			expected: ErrNoProposalChanges,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := keeper.SubmitProposal(context.Background(), testCase.proposal)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}

func TestInMemoryKeeperRejectsDuplicateVote(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)

	if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), id, "alice", VoteNo); !errors.Is(err, ErrDuplicateVote) {
		t.Fatalf("expected duplicate vote, got %v", err)
	}
}

func TestInMemoryKeeperRejectsInvalidVote(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)

	if err := keeper.Vote(context.Background(), id, "", VoteYes); !errors.Is(err, ErrMissingVoter) {
		t.Fatalf("expected missing voter, got %v", err)
	}
	if err := keeper.Vote(context.Background(), id, "alice", VoteOption("maybe")); !errors.Is(err, ErrInvalidVoteOption) {
		t.Fatalf("expected invalid option, got %v", err)
	}
}

func TestInMemoryKeeperRejectsUnknownProposal(t *testing.T) {
	keeper := newTestKeeper()

	if err := keeper.Vote(context.Background(), 999, "alice", VoteYes); !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("expected proposal not found, got %v", err)
	}
	if err := keeper.Execute(context.Background(), 999); !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("expected proposal not found, got %v", err)
	}
}

func TestInMemoryKeeperEnforcesVotingPeriodAndTimelock(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)
	if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), id, "bob", VoteYes); err != nil {
		t.Fatal(err)
	}

	keeper.SetTime(5)
	if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrVotingPeriodOpen) {
		t.Fatalf("expected voting period open, got %v", err)
	}
	keeper.SetTime(11)
	if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrTimelockActive) {
		t.Fatalf("expected timelock active, got %v", err)
	}
}

func TestInMemoryKeeperRejectsFailedQuorumThresholdAndVeto(t *testing.T) {
	t.Run("quorum", func(t *testing.T) {
		keeper := newTestKeeper()
		id := submitTestProposal(t, keeper)
		if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
			t.Fatal(err)
		}
		keeper.SetTime(15)
		if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrProposalRejected) {
			t.Fatalf("expected rejected, got %v", err)
		}
	})

	t.Run("threshold", func(t *testing.T) {
		keeper := newTestKeeper()
		id := submitTestProposal(t, keeper)
		if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
			t.Fatal(err)
		}
		if err := keeper.Vote(context.Background(), id, "bob", VoteNo); err != nil {
			t.Fatal(err)
		}
		keeper.SetTime(15)
		if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrProposalRejected) {
			t.Fatalf("expected rejected, got %v", err)
		}
	})

	t.Run("veto", func(t *testing.T) {
		keeper := newTestKeeper()
		id := submitTestProposal(t, keeper)
		if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
			t.Fatal(err)
		}
		if err := keeper.Vote(context.Background(), id, "bob", VoteYes); err != nil {
			t.Fatal(err)
		}
		if err := keeper.Vote(context.Background(), id, "carol", VoteVeto); err != nil {
			t.Fatal(err)
		}
		keeper.SetTime(15)
		if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrProposalRejected) {
			t.Fatalf("expected rejected, got %v", err)
		}
	})
}

func TestInMemoryKeeperRejectsDoubleExecute(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)
	if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), id, "bob", VoteYes); err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(15)

	if err := keeper.Execute(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrProposalExecuted) {
		t.Fatalf("expected already executed, got %v", err)
	}
}

func TestInMemoryKeeperProposalReturnsCopy(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)
	state, found := keeper.Proposal(id)
	if !found {
		t.Fatal("expected proposal")
	}
	state.Proposal.Changes[0].Value = []byte("mutated")

	again, found := keeper.Proposal(id)
	if !found {
		t.Fatal("expected proposal")
	}
	if string(again.Proposal.Changes[0].Value) == "mutated" {
		t.Fatal("proposal state mutated through copy")
	}
}

func TestInMemoryKeeperTallyAndVotingPowerUpdate(t *testing.T) {
	keeper := newTestKeeper()
	keeper.SetVotingPower("dave", 3)
	id := submitTestProposal(t, keeper)

	if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if err := keeper.Vote(context.Background(), id, "dave", VoteAbstain); err != nil {
		t.Fatal(err)
	}

	tally, found := keeper.Tally(id)
	if !found {
		t.Fatal("expected tally")
	}
	if tally.Yes != 1 || tally.Abstain != 3 || tally.Total != 4 || tally.Passed {
		t.Fatalf("unexpected tally: %+v", tally)
	}

	if _, found := keeper.Tally(999); found {
		t.Fatal("unexpected unknown proposal tally")
	}
}

func TestInMemoryKeeperContextCancellation(t *testing.T) {
	keeper := newTestKeeper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := keeper.SubmitProposal(ctx, testProposal()); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected submit canceled, got %v", err)
	}
	if err := keeper.Vote(ctx, 1, "alice", VoteYes); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected vote canceled, got %v", err)
	}
	if err := keeper.Execute(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected execute canceled, got %v", err)
	}
}

func newTestKeeper() *InMemoryKeeper {
	return NewInMemoryKeeper(TallyPolicy{
		QuorumBPS:       6000,
		YesThresholdBPS: 6000,
		VetoBPS:         3000,
		VotingPeriod:    10,
		Timelock:        5,
	}, map[types.Address]types.VotingPower{
		"alice": 1,
		"bob":   1,
		"carol": 1,
	})
}

func submitTestProposal(t *testing.T, keeper *InMemoryKeeper) uint64 {
	t.Helper()
	id, err := keeper.SubmitProposal(context.Background(), testProposal())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testProposal() Proposal {
	return Proposal{
		Title:       "update committee size",
		Description: "increase committee size",
		Submitter:   "alice",
		Changes: []ParameterChange{
			{Module: "committee", Key: "size", Value: []byte("512")},
		},
	}
}

func TestInMemoryKeeperAcceptsVotesOnlyWhileVotingPeriodIsOpen(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)

	keeper.SetTime(10)
	if err := keeper.Vote(context.Background(), id, "alice", VoteYes); err != nil {
		t.Fatalf("expected a vote in the last block of the voting period, got %v", err)
	}
	keeper.SetTime(11)
	if err := keeper.Vote(context.Background(), id, "bob", VoteYes); !errors.Is(err, ErrVotingPeriodClosed) {
		t.Fatalf("expected a late vote to be refused, got %v", err)
	}
	keeper.SetTime(10)
	if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrVotingPeriodOpen) {
		t.Fatalf("expected execution to wait for the end of the voting period, got %v", err)
	}
}

func TestInMemoryKeeperRefusesVotesOnDecidedProposals(t *testing.T) {
	t.Run("executed", func(t *testing.T) {
		keeper := newTestKeeper()
		id := submitTestProposal(t, keeper)
		for _, voter := range []types.Address{"alice", "bob"} {
			if err := keeper.Vote(context.Background(), id, voter, VoteYes); err != nil {
				t.Fatal(err)
			}
		}
		keeper.SetTime(15)
		if err := keeper.Execute(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		keeper.SetTime(10)
		if err := keeper.Vote(context.Background(), id, "carol", VoteVeto); !errors.Is(err, ErrProposalExecuted) {
			t.Fatalf("expected a vote on an executed proposal to be refused, got %v", err)
		}
	})

	t.Run("rejected", func(t *testing.T) {
		keeper := newTestKeeper()
		id := submitTestProposal(t, keeper)
		keeper.SetTime(15)
		if err := keeper.Execute(context.Background(), id); !errors.Is(err, ErrProposalRejected) {
			t.Fatalf("expected rejection, got %v", err)
		}
		keeper.SetTime(10)
		if err := keeper.Vote(context.Background(), id, "alice", VoteYes); !errors.Is(err, ErrProposalRejected) {
			t.Fatalf("expected a vote on a rejected proposal to be refused, got %v", err)
		}
	})
}

func TestTallyPolicyUsesBasisPointsOfTheSnapshot(t *testing.T) {
	policy := TallyPolicy{QuorumBPS: 3340, YesThresholdBPS: 5000, VetoBPS: 3340}
	cases := []struct {
		name     string
		snapshot types.VotingPower
		tally    TallyResult
		passes   bool
	}{
		{name: "exactly the thresholds", snapshot: 10000, tally: TallyResult{Yes: 5000, Total: 5000}, passes: true},
		{name: "one below the yes threshold", snapshot: 10000, tally: TallyResult{Yes: 4999, Total: 5000}, passes: false},
		{name: "one below quorum", snapshot: 10000, tally: TallyResult{Yes: 3339, Total: 3339}, passes: false},
		{name: "quorum with the yes threshold unmet", snapshot: 10000, tally: TallyResult{Yes: 3340, Abstain: 2000, Total: 5340}, passes: false},
		{name: "yes threshold met but no outnumbers yes", snapshot: 10000, tally: TallyResult{Yes: 5000, No: 5000, Total: 10000}, passes: false},
		{name: "veto at its threshold", snapshot: 10000, tally: TallyResult{Yes: 6000, Veto: 3340, Total: 9340}, passes: false},
		{name: "veto one below its threshold", snapshot: 10000, tally: TallyResult{Yes: 6000, Veto: 3339, Total: 9339}, passes: true},
		{name: "thresholds round up", snapshot: 3, tally: TallyResult{Yes: 1, Total: 1}, passes: false},
		{name: "rounded thresholds met", snapshot: 3, tally: TallyResult{Yes: 2, Total: 2}, passes: true},
		{name: "empty snapshot needs a unit of power", snapshot: 0, tally: TallyResult{}, passes: false},
		{name: "huge snapshot does not overflow", snapshot: types.VotingPower(^uint64(0)), tally: TallyResult{Yes: types.VotingPower(^uint64(0)), Total: types.VotingPower(^uint64(0))}, passes: true},
		{name: "huge snapshot with a small tally", snapshot: types.VotingPower(^uint64(0)), tally: TallyResult{Yes: 1, Total: 1}, passes: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := policy.passes(testCase.snapshot, testCase.tally); got != testCase.passes {
				t.Fatalf("expected passes=%t for %+v over %d, got %t", testCase.passes, testCase.tally, testCase.snapshot, got)
			}
		})
	}
}

func TestInMemoryKeeperSnapshotsBondedPowerAtSubmission(t *testing.T) {
	keeper := NewInMemoryKeeper(TallyPolicy{QuorumBPS: 5000, YesThresholdBPS: 5000, VotingPeriod: 10, Timelock: 1}, nil)
	keeper.SetTotalPower(100)
	keeper.SetTime(7)
	id := submitTestProposal(t, keeper)

	// Power that appears after the proposal exists does not move the thresholds.
	keeper.SetTotalPower(1_000_000)
	keeper.SetVotingPower("whale", 40)
	keeper.SetVotingPower("alice", 60)
	for _, voter := range []types.Address{"whale", "alice"} {
		if err := keeper.Vote(context.Background(), id, voter, VoteYes); err != nil {
			t.Fatal(err)
		}
	}
	state, found := keeper.Proposal(id)
	if !found || state.SnapshotPower != 100 || state.SnapshotHeight != 7 {
		t.Fatalf("expected the submission snapshot (power 100 at height 7), got %+v", state)
	}
	if tally, _ := keeper.Tally(id); !tally.Passed {
		t.Fatalf("expected 100 of the 100 snapshotted power to pass, got %+v", tally)
	}
}

func TestInMemoryKeeperSnapshotDefaultsToTheSumOfKnownPowers(t *testing.T) {
	keeper := newTestKeeper()
	id := submitTestProposal(t, keeper)
	state, _ := keeper.Proposal(id)
	if state.SnapshotPower != 3 {
		t.Fatalf("expected the three known powers as snapshot, got %d", state.SnapshotPower)
	}
}

func TestProposalValidateEnforcesSizeLimitsAndUpgradePlans(t *testing.T) {
	valid := testProposal()
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*Proposal)
	}{
		{name: "title too long", mutate: func(p *Proposal) { p.Title = strings.Repeat("t", MaxProposalTitleBytes+1) }},
		{name: "description too long", mutate: func(p *Proposal) { p.Description = strings.Repeat("d", MaxProposalDescriptionBytes+1) }},
		{name: "too many changes", mutate: func(p *Proposal) {
			p.Changes = make([]ParameterChange, MaxProposalChanges+1)
			for index := range p.Changes {
				p.Changes[index] = ParameterChange{Module: "m", Key: "k", Value: []byte("v")}
			}
		}},
		{name: "change too large", mutate: func(p *Proposal) {
			p.Changes = []ParameterChange{{Module: "m", Key: "k", Value: make([]byte, MaxParameterChangeBytes)}}
		}},
		{name: "malformed upgrade plan", mutate: func(p *Proposal) {
			p.Changes = []ParameterChange{{Module: "upgrade", Key: "plan", Value: []byte("not json")}}
		}},
		{name: "invalid upgrade plan", mutate: func(p *Proposal) {
			p.Changes = []ParameterChange{{Module: "upgrade", Key: "plan", Value: []byte(`{"Name":"","Height":0}`)}}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			proposal := testProposal()
			testCase.mutate(&proposal)
			if err := proposal.Validate(); err == nil {
				t.Fatal("expected the proposal to be refused")
			}
			if _, err := newTestKeeper().SubmitProposal(context.Background(), proposal); err == nil {
				t.Fatal("expected the keeper to refuse the proposal")
			}
		})
	}
}

func TestTallyPolicyValidateRequiresBasisPointThresholds(t *testing.T) {
	valid := TallyPolicy{QuorumBPS: DefaultQuorumBPS, YesThresholdBPS: DefaultYesThresholdBPS, VetoBPS: DefaultVetoBPS}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*TallyPolicy){
		"zero quorum":          func(p *TallyPolicy) { p.QuorumBPS = 0 },
		"quorum above all":     func(p *TallyPolicy) { p.QuorumBPS = BasisPoints + 1 },
		"zero yes threshold":   func(p *TallyPolicy) { p.YesThresholdBPS = 0 },
		"yes threshold above":  func(p *TallyPolicy) { p.YesThresholdBPS = BasisPoints + 1 },
		"veto above all power": func(p *TallyPolicy) { p.VetoBPS = BasisPoints + 1 },
	} {
		policy := valid
		mutate(&policy)
		if err := policy.Validate(); !errors.Is(err, ErrInvalidTallyPolicy) {
			t.Fatalf("%s: expected an invalid policy, got %v", name, err)
		}
	}
	valid.VetoBPS = 0
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected the veto threshold to be optional, got %v", err)
	}
}

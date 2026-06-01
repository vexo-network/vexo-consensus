package governance

import (
	"context"
	"errors"
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
		QuorumPower:       2,
		YesThresholdPower: 2,
		VetoPower:         1,
		VotingPeriod:      10,
		Timelock:          5,
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

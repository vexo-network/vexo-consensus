package governance

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestStoreKeeperPersistsVotesAndTally(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper, err := NewStoreKeeper(storage, TallyPolicy{QuorumPower: 1, YesThresholdPower: 1, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetTime(1)
	proposalID, err := keeper.SubmitProposal(context.Background(), Proposal{
		Submitter: "alice",
		Title:     "title",
		Changes:   []ParameterChange{{Module: "execution", Key: "max_gas", Value: []byte("1")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper.SetVotingPower("alice", 1)
	if err := keeper.Vote(context.Background(), proposalID, "alice", VoteYes); err != nil {
		t.Fatal(err)
	}
	if tally, found := keeper.Tally(proposalID); !found || tally.Yes != 1 {
		t.Fatalf("expected immediate yes tally, found=%t tally=%+v", found, tally)
	}

	reopened, err := NewStoreKeeper(storage, TallyPolicy{QuorumPower: 1, YesThresholdPower: 1, VotingPeriod: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tally, found := reopened.Tally(proposalID)
	if !found || tally.Yes != types.VotingPower(1) || !tally.Passed {
		t.Fatalf("expected persisted yes tally, found=%t tally=%+v", found, tally)
	}
}

func TestStoreKeeperRejectsCorruptExistingState(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(context.Background(), governanceNamespace, []byte("state"), []byte("{")); err != nil {
		t.Fatal(err)
	}

	_, err = NewStoreKeeper(storage, TallyPolicy{}, nil)
	if err == nil || errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected corrupt governance state to fail startup, got %v", err)
	}
}

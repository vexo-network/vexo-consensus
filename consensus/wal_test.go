package consensus

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestWALRecordsAndReplaysVotes(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	vote := Vote{Height: 1, Round: 0, BlockHash: types.Hash{1}, ValidatorID: "alice"}
	if err := wal.RecordVote(vote); err != nil {
		t.Fatal(err)
	}
	if err := wal.RecordProposal(Proposal{Block: types.Block{Header: types.Header{Height: 1}}, Round: 0, Proposer: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recordedHash, found, err := reopened.RecordedVote("alice", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !found || recordedHash != vote.BlockHash {
		t.Fatalf("expected recorded vote %x, got found=%t hash=%x", vote.BlockHash, found, recordedHash)
	}
	if _, found, err := reopened.RecordedVote("alice", 2, 0); err != nil || found {
		t.Fatalf("expected missing vote lookup, got found=%t err=%v", found, err)
	}
	if err := reopened.RecordVote(vote); err != nil {
		t.Fatal(err)
	}
	events, err := reopened.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 wal events after replay and repeated vote, got %d", len(events))
	}
}

func TestWALRejectsConflictingVoteAfterRestart(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := wal.RecordVote(Vote{Height: 1, Round: 0, BlockHash: types.Hash{1}, ValidatorID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	err = reopened.RecordVote(Vote{Height: 1, Round: 0, BlockHash: types.Hash{2}, ValidatorID: "alice"})
	if !errors.Is(err, ErrDoubleSignDetected) {
		t.Fatalf("expected double-sign guard, got %v", err)
	}
}

func TestWALRejectsConflictingTimeoutVoteAfterRestart(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	first := TimeoutVote{Height: 1, Round: 0, ValidatorID: "alice", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}}
	second := TimeoutVote{Height: 1, Round: 0, ValidatorID: "alice", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}}
	if err := wal.RecordTimeoutVote(first); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.RecordTimeoutVote(second); !errors.Is(err, ErrDoubleSignDetected) {
		t.Fatalf("expected timeout double-sign guard, got %v", err)
	}
}

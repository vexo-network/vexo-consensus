package consensus

import (
	"bytes"
	"errors"
	"os"
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

func openTestWAL(t *testing.T, path string) *WAL {
	t.Helper()
	wal, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	return wal
}

func writeVotes(t *testing.T, path string, rounds ...types.Round) {
	t.Helper()
	wal := openTestWAL(t, path)
	for _, round := range rounds {
		if err := wal.RecordVote(Vote{Height: 1, Round: round, BlockHash: types.Hash{byte(round + 1)}, ValidatorID: "alice"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func appendBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
}

func TestWALTruncatesATornTailAndKeepsAppending(t *testing.T) {
	tails := map[string][]byte{
		"partial record without a newline":    []byte("deadbeef {\"type\":\"vote\",\"validator_id\":\"al"),
		"complete line with a bad checksum":   []byte("00000000 {\"type\":\"vote\",\"validator_id\":\"alice\",\"round\":9}\n"),
		"line without a checksum":             []byte("{\"type\":\"vote\",\"validator_id\":\"alice\",\"round\":9}\n"),
		"only the first bytes of a checksum":  []byte("dead"),
		"checksum with truncated json object": []byte("12345678 {\"type\":\n"),
	}
	for name, tail := range tails {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir() + "/consensus.wal"
			writeVotes(t, path, 0, 1)
			validSize := fileSize(t, path)
			appendBytes(t, path, tail)

			wal := openTestWAL(t, path)
			if got := fileSize(t, path); got != validSize {
				t.Fatalf("expected the torn tail to be truncated to %d bytes, file has %d", validSize, got)
			}
			for _, round := range []types.Round{0, 1} {
				if _, found, err := wal.RecordedVote("alice", 1, round); err != nil || !found {
					t.Fatalf("vote of round %d lost: found=%t err=%v", round, found, err)
				}
			}
			if _, found, _ := wal.RecordedVote("alice", 1, 9); found {
				t.Fatal("a torn record must not count as a recorded vote")
			}
			if err := wal.RecordVote(Vote{Height: 1, Round: 2, BlockHash: types.Hash{3}, ValidatorID: "alice"}); err != nil {
				t.Fatal(err)
			}
			if err := wal.Close(); err != nil {
				t.Fatal(err)
			}

			reopened := openTestWAL(t, path)
			defer reopened.Close()
			events, err := reopened.Replay()
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 3 {
				t.Fatalf("expected the two original votes and the new one, got %d events", len(events))
			}
		})
	}
}

func TestWALReplayIgnoresATornTailWithoutModifyingTheFile(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	writeVotes(t, path, 0)
	appendBytes(t, path, []byte("deadbeef {\"typ"))
	size := fileSize(t, path)

	wal := &WAL{path: path}
	events, err := wal.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected the single intact record, got %d", len(events))
	}
	if fileSize(t, path) != size {
		t.Fatal("replay must not modify the log")
	}
}

func TestWALReportsCorruptionFollowedByValidRecords(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	writeVotes(t, path, 0, 1, 2)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	// Flip one payload byte of the middle record: its checksum no longer matches while the
	// record after it stays valid, which a crash cannot produce.
	middle := append([]byte(nil), lines[1]...)
	middle[len(middle)-3] ^= 0x01
	damaged := bytes.Join([][]byte{lines[0], middle, lines[2]}, nil)
	if err := os.WriteFile(path, damaged, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenWAL(path); !errors.Is(err, ErrWALCorrupt) {
		t.Fatalf("expected corruption to be reported, got %v", err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, damaged) {
		t.Fatal("a corrupt log must be left untouched for inspection")
	}
}

func TestWALChecksumRejectsAFlippedBitInTheLastRecord(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	writeVotes(t, path, 0, 1)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	last := append([]byte(nil), lines[1]...)
	last[len(last)-3] ^= 0x01
	if err := os.WriteFile(path, append(append([]byte(nil), lines[0]...), last...), 0o600); err != nil {
		t.Fatal(err)
	}

	wal := openTestWAL(t, path)
	defer wal.Close()
	if _, found, _ := wal.RecordedVote("alice", 1, 0); !found {
		t.Fatal("the intact record must survive")
	}
	if _, found, _ := wal.RecordedVote("alice", 1, 1); found {
		t.Fatal("a record failing its checksum must not be trusted")
	}
}

func TestWALPersistsTheCompleteTimeoutVote(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	vote := TimeoutVote{
		Height:      4,
		Round:       9,
		ValidatorID: "alice",
		HighQC:      finality.QuorumCert{Height: 3, Round: 7, BlockHash: types.Hash{3}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("aggregate"), VotingPower: 2},
		Signature:   types.Signature("signature"),
	}
	if err := wal.RecordTimeoutVote(vote); err != nil {
		t.Fatal(err)
	}
	if err := wal.RecordTimeoutVote(vote); err != nil {
		t.Fatalf("repeating the same timeout vote must be idempotent, got %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openTestWAL(t, path)
	defer reopened.Close()
	recorded, found, err := reopened.RecordedTimeoutVote("alice", 9)
	if err != nil || !found {
		t.Fatalf("timeout vote lost: found=%t err=%v", found, err)
	}
	if recorded.Height != 4 || recorded.ValidatorID != "alice" || !bytes.Equal(recorded.Signature, vote.Signature) {
		t.Fatalf("unexpected recorded vote %+v", recorded)
	}
	if !sameQC(recorded.HighQC, vote.HighQC) || !bytes.Equal(recorded.HighQC.Signature, vote.HighQC.Signature) ||
		!bytes.Equal(recorded.HighQC.Signers, vote.HighQC.Signers) || recorded.HighQC.VotingPower != 2 {
		t.Fatalf("the recorded vote must keep the complete high qc, got %+v", recorded.HighQC)
	}
	if _, found, _ := reopened.RecordedTimeoutVote("alice", 8); found {
		t.Fatal("a timeout vote must only be found for its own view")
	}
}

func TestWALPersistsSafetyState(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	if _, found, err := wal.SafetyState(); err != nil || found {
		t.Fatalf("a new log has no safety state, got found=%t err=%v", found, err)
	}
	state := SafetyState{
		LockedQC:        finality.QuorumCert{Height: 2, Round: 3, BlockHash: types.Hash{2}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("locked"), VotingPower: 2},
		HighQC:          finality.QuorumCert{Height: 3, Round: 4, BlockHash: types.Hash{3}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("high"), VotingPower: 2},
		LastVotedHeight: 4,
		LastVotedRound:  5,
	}
	if err := wal.RecordSafetyState(state); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openTestWAL(t, path)
	defer reopened.Close()
	restored, found, err := reopened.SafetyState()
	if err != nil || !found {
		t.Fatalf("safety state lost: found=%t err=%v", found, err)
	}
	if restored.LastVotedHeight != 4 || restored.LastVotedRound != 5 ||
		!sameQC(restored.LockedQC, state.LockedQC) || !bytes.Equal(restored.LockedQC.Signature, state.LockedQC.Signature) ||
		!sameQC(restored.HighQC, state.HighQC) || !bytes.Equal(restored.HighQC.Signature, state.HighQC.Signature) {
		t.Fatalf("unexpected restored safety state %+v", restored)
	}
}

func TestWALSafetyStateNeverRegresses(t *testing.T) {
	wal := openTestWAL(t, t.TempDir()+"/consensus.wal")
	defer wal.Close()
	lock := finality.QuorumCert{Height: 2, Round: 3, BlockHash: types.Hash{2}}
	high := finality.QuorumCert{Height: 3, Round: 4, BlockHash: types.Hash{3}}
	if err := wal.RecordSafetyState(SafetyState{LockedQC: lock, HighQC: high, LastVotedHeight: 4, LastVotedRound: 5}); err != nil {
		t.Fatal(err)
	}

	cases := map[string]SafetyState{
		"older last voted view": {LockedQC: lock, HighQC: high, LastVotedHeight: 4, LastVotedRound: 4},
		"dropped lock":          {HighQC: high, LastVotedHeight: 4, LastVotedRound: 5},
		"older lock":            {LockedQC: finality.QuorumCert{Height: 1, Round: 1, BlockHash: types.Hash{1}}, HighQC: high, LastVotedHeight: 4, LastVotedRound: 5},
		"dropped high qc":       {LockedQC: lock, LastVotedHeight: 4, LastVotedRound: 5},
		"older high qc":         {LockedQC: lock, HighQC: finality.QuorumCert{Height: 2, Round: 2, BlockHash: types.Hash{9}}, LastVotedHeight: 4, LastVotedRound: 5},
	}
	for name, regressed := range cases {
		if err := wal.RecordSafetyState(regressed); !errors.Is(err, ErrSafetyRegression) {
			t.Fatalf("%s: expected a regression error, got %v", name, err)
		}
	}
	state, _, _ := wal.SafetyState()
	if state.LastVotedRound != 5 || state.LockedQC.Round != 3 || state.HighQC.Round != 4 {
		t.Fatalf("a refused state must not replace the recorded one, got %+v", state)
	}
	if err := wal.RecordSafetyState(SafetyState{LockedQC: lock, HighQC: high, LastVotedHeight: 4, LastVotedRound: 5}); err != nil {
		t.Fatalf("repeating the recorded state must be accepted, got %v", err)
	}
	if err := wal.RecordSafetyState(SafetyState{LockedQC: high, HighQC: high, LastVotedHeight: 5, LastVotedRound: 6}); err != nil {
		t.Fatalf("advancing the state must be accepted, got %v", err)
	}
}

func TestWALClosedRefusesRecords(t *testing.T) {
	wal := openTestWAL(t, t.TempDir()+"/consensus.wal")
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	if err := wal.RecordVote(Vote{Height: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"}); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("expected a closed log to refuse votes, got %v", err)
	}
	if err := wal.RecordSafetyState(SafetyState{}); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("expected a closed log to refuse the safety state, got %v", err)
	}
	if _, _, err := wal.SafetyState(); !errors.Is(err, ErrWALClosed) {
		t.Fatalf("expected a closed log to refuse reads, got %v", err)
	}
}

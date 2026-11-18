package consensus

import (
	"errors"
	"os"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func groupTestState(round types.Round) SafetyState {
	qc := finality.QuorumCert{Height: 1, Round: round, BlockHash: types.Hash{byte(round)}, Signers: types.Bitmap("alice"), Signature: types.AggregateSignature("aggregate"), VotingPower: 1}
	return SafetyState{LockedQC: qc, HighQC: qc, LastVotedHeight: 2, LastVotedRound: round + 1}
}

func replayTypes(t *testing.T, wal *WAL) []WALEventType {
	t.Helper()
	events, err := wal.Replay()
	if err != nil {
		t.Fatal(err)
	}
	kinds := make([]WALEventType, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Type)
	}
	return kinds
}

func sameTypes(got []WALEventType, want ...WALEventType) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

// A vote and the safety state it is cast under are one record: the log grows by one record, and a
// reopened log restores both guards.
func TestWALRecordsAVoteAndItsSafetyStateAsOneRecord(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	state := groupTestState(3)
	vote := Vote{Height: 2, Round: 4, BlockHash: types.Hash{7}, ValidatorID: "alice"}
	before := fileSize(t, path)
	if err := wal.RecordVoteWithSafetyState(state, vote); err != nil {
		t.Fatal(err)
	}
	record, err := appendWALRecord(nil, safetyStateEvent(state), voteEvent(vote))
	if err != nil {
		t.Fatal(err)
	}
	if grown := fileSize(t, path) - before; grown != int64(len(record)) {
		t.Fatalf("the log grew by %d bytes, want the one record of %d", grown, len(record))
	}
	if got := replayTypes(t, wal); !sameTypes(got, WALEventSafetyState, WALEventVote) {
		t.Fatalf("replay returns %v", got)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestWAL(t, path)
	defer reopened.Close()
	if hash, found, err := reopened.RecordedVote("alice", 2, 4); err != nil || !found || hash != vote.BlockHash {
		t.Fatalf("reopened vote: found=%t hash=%x err=%v", found, hash, err)
	}
	if restored, found, err := reopened.SafetyState(); err != nil || !found || restored.LastVotedRound != state.LastVotedRound || restored.LockedQC.Round != state.LockedQC.Round {
		t.Fatalf("reopened safety state: found=%t state=%+v err=%v", found, restored, err)
	}
	if err := reopened.RecordVoteWithSafetyState(state, Vote{Height: 2, Round: 4, BlockHash: types.Hash{8}, ValidatorID: "alice"}); !errors.Is(err, ErrDoubleSignDetected) {
		t.Fatalf("a second block in the view after a restart: %v", err)
	}
}

// What either half refuses records nothing: neither the vote nor the safety state.
func TestWALRecordsNothingOfAVoteOrSafetyStateItRefuses(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	defer wal.Close()
	if err := wal.RecordVoteWithSafetyState(groupTestState(5), Vote{Height: 2, Round: 6, BlockHash: types.Hash{1}, ValidatorID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := wal.RecordTimeoutVoteWithSafetyState(groupTestState(6), TimeoutVote{Height: 2, Round: 7, ValidatorID: "alice", HighQC: groupTestState(5).HighQC}); err != nil {
		t.Fatal(err)
	}
	size := fileSize(t, path)
	cases := []struct {
		name   string
		record func() error
		want   error
	}{
		{"double vote", func() error {
			return wal.RecordVoteWithSafetyState(groupTestState(7), Vote{Height: 2, Round: 6, BlockHash: types.Hash{2}, ValidatorID: "alice"})
		}, ErrDoubleSignDetected},
		{"vote under a regressed state", func() error {
			return wal.RecordVoteWithSafetyState(groupTestState(2), Vote{Height: 3, Round: 9, BlockHash: types.Hash{3}, ValidatorID: "alice"})
		}, ErrSafetyRegression},
		{"double timeout vote", func() error {
			return wal.RecordTimeoutVoteWithSafetyState(groupTestState(7), TimeoutVote{Height: 2, Round: 7, ValidatorID: "alice", HighQC: groupTestState(6).HighQC})
		}, ErrDoubleSignDetected},
		{"timeout vote under a regressed state", func() error {
			return wal.RecordTimeoutVoteWithSafetyState(groupTestState(2), TimeoutVote{Height: 3, Round: 9, ValidatorID: "alice", HighQC: groupTestState(5).HighQC})
		}, ErrSafetyRegression},
	}
	for _, testCase := range cases {
		if err := testCase.record(); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: got %v, want %v", testCase.name, err, testCase.want)
		}
	}
	if got := fileSize(t, path); got != size {
		t.Fatalf("refused records grew the log from %d to %d bytes", size, got)
	}
	if state, _, _ := wal.SafetyState(); state.LastVotedRound != groupTestState(6).LastVotedRound {
		t.Fatalf("a refused record moved the safety state to round %d", state.LastVotedRound)
	}
	if _, found, _ := wal.RecordedVote("alice", 3, 9); found {
		t.Fatal("a vote refused for its safety state was remembered")
	}
}

// A proposal record waits for the next record the log syncs and is written in it; until then a
// crash loses it, and Close writes one still waiting.
func TestWALWritesAProposalWithTheNextSyncedRecord(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	proposal := Proposal{Block: types.Block{Header: types.Header{Height: 2}}, Round: 4, Proposer: "alice"}
	before := fileSize(t, path)
	if err := wal.RecordProposal(proposal); err != nil {
		t.Fatal(err)
	}
	if got := fileSize(t, path); got != before {
		t.Fatalf("a proposal record cost a write of its own: %d to %d bytes", before, got)
	}
	crashed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := wal.RecordVoteWithSafetyState(groupTestState(3), Vote{Height: 2, Round: 4, BlockHash: HashBlock(proposal.Block), ValidatorID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if got := replayTypes(t, wal); !sameTypes(got, WALEventProposal, WALEventSafetyState, WALEventVote) {
		t.Fatalf("replay returns %v", got)
	}
	if err := wal.RecordProposal(Proposal{Block: types.Block{Header: types.Header{Height: 3}}, Round: 5, Proposer: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestWAL(t, path)
	if got := replayTypes(t, reopened); !sameTypes(got, WALEventProposal, WALEventSafetyState, WALEventVote, WALEventProposal) {
		t.Fatalf("after Close replay returns %v", got)
	}
	reopened.Close()

	crashedPath := t.TempDir() + "/consensus.wal"
	if err := os.WriteFile(crashedPath, crashed, 0o600); err != nil {
		t.Fatal(err)
	}
	afterCrash := openTestWAL(t, crashedPath)
	defer afterCrash.Close()
	if got := replayTypes(t, afterCrash); len(got) != 0 {
		t.Fatalf("a crash before the next synced record keeps %v", got)
	}
}

// A crash in the middle of a group record keeps none of its events.
func TestWALDropsATornGroupRecordWhole(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	if err := wal.RecordVoteWithSafetyState(groupTestState(1), Vote{Height: 2, Round: 2, BlockHash: types.Hash{1}, ValidatorID: "alice"}); err != nil {
		t.Fatal(err)
	}
	intact := fileSize(t, path)
	if err := wal.RecordVoteWithSafetyState(groupTestState(3), Vote{Height: 2, Round: 4, BlockHash: types.Hash{2}, ValidatorID: "alice"}); err != nil {
		t.Fatal(err)
	}
	full := fileSize(t, path)
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	for _, cut := range []int64{intact + 1, (intact + full) / 2, full - 1} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		tornPath := t.TempDir() + "/consensus.wal"
		if err := os.WriteFile(tornPath, data[:cut], 0o600); err != nil {
			t.Fatal(err)
		}
		torn := openTestWAL(t, tornPath)
		if _, found, _ := torn.RecordedVote("alice", 2, 4); found {
			t.Fatalf("cut at %d: the vote of a torn record was restored", cut)
		}
		if state, _, _ := torn.SafetyState(); state.LastVotedRound != groupTestState(1).LastVotedRound {
			t.Fatalf("cut at %d: the safety state of a torn record was restored", cut)
		}
		torn.Close()
		if got := fileSize(t, tornPath); got != intact {
			t.Fatalf("cut at %d: the torn record was not truncated, %d bytes left", cut, got)
		}
	}
}

// A group has one encoding: at least two events, none of them a group.
func TestWALRefusesAGroupWithoutTwoPlainEvents(t *testing.T) {
	single, err := appendWALPayload(nil, walVoteEvent(1))
	if err != nil {
		t.Fatal(err)
	}
	one := append([]byte{walKindGroup, 1}, types.AppendBytes(nil, single)...)
	if _, err := readWALPayload(one); err == nil {
		t.Fatal("a group of one event was accepted")
	}
	group, err := appendWALRecord(nil, walVoteEvent(1), walVoteEvent(2))
	if err != nil {
		t.Fatal(err)
	}
	inner := group[walLengthBytes : len(group)-walChecksumBytes]
	nested := append([]byte{walKindGroup, 2}, types.AppendBytes(types.AppendBytes(nil, inner), single)...)
	if _, err := readWALPayload(nested); err == nil {
		t.Fatal("a group inside a group was accepted")
	}
}

package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

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

// walTestTime is the time of the records the tests build by hand.
var walTestTime = time.Unix(0, 1_700_000_000_123_456_789).UTC()

func walVoteEvent(round types.Round) WALEvent {
	return WALEvent{Type: WALEventVote, Time: walTestTime, ValidatorID: "alice", Height: 1, Round: round, BlockHash: types.Hash{byte(round + 1)}}
}

// walRecord returns the bytes of the record of event.
func walRecord(t *testing.T, event WALEvent) []byte {
	t.Helper()
	record, err := appendWALRecord(nil, event)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

// walFrame frames a payload as a record: its length, the payload, and the CRC-32C of both.
func walFrame(payload []byte) []byte {
	record := binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
	record = append(record, payload...)
	return binary.BigEndian.AppendUint32(record, crc32.Checksum(record, crc32.MakeTable(crc32.Castagnoli)))
}

// walRecordOffsets returns where each record of a log starts, followed by where the log ends.
func walRecordOffsets(t *testing.T, data []byte) []int {
	t.Helper()
	offsets := []int{len(walHeader)}
	for offsets[len(offsets)-1] < len(data) {
		offset := offsets[len(offsets)-1]
		offsets = append(offsets, offset+walFrameBytes+int(binary.BigEndian.Uint32(data[offset:])))
	}
	if offsets[len(offsets)-1] != len(data) {
		t.Fatalf("the records of the log end at %d, not at its end %d", offsets[len(offsets)-1], len(data))
	}
	return offsets
}

func readWALFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// withFlippedBit returns a copy of data with one bit of the byte at index flipped.
func withFlippedBit(data []byte, index int) []byte {
	copied := slices.Clone(data)
	copied[index] ^= 0x01
	return copied
}

func TestWALTruncatesATornTailAndKeepsAppending(t *testing.T) {
	torn := walRecord(t, walVoteEvent(9))
	tails := map[string][]byte{
		"a record cut in its payload":           torn[:len(torn)-8],
		"a record without its checksum":         torn[:len(torn)-walChecksumBytes],
		"a record cut in its checksum":          torn[:len(torn)-1],
		"a complete record with a bad checksum": withFlippedBit(torn, len(torn)-1),
		"a complete record with a bad payload":  withFlippedBit(torn, walLengthBytes+3),
		"a record with a longer length":         append([]byte{0, 0, 0, byte(len(torn))}, torn[walLengthBytes:]...),
		"only the first bytes of a length":      torn[:2],
		"a length and a few bytes":              {0, 0, 0, 40, 1, 2, 3},
		"a length over the limit":               {0xff, 0xff, 0xff, 0xff, 1, 2, 3, 4, 5, 6, 7, 8},
		"zeros":                                 make([]byte, 20),
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
	torn := walRecord(t, walVoteEvent(9))
	appendBytes(t, path, torn[:len(torn)-3])
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
	data := readWALFile(t, path)
	offsets := walRecordOffsets(t, data)
	// A crash tears the last record only. Damage to a record that an intact record follows is not a
	// crash, wherever in the record it is: its payload, its checksum or its length.
	cases := map[string]int{
		"a payload byte":    offsets[1] + walLengthBytes + 2,
		"a checksum byte":   offsets[2] - 1,
		"the length":        offsets[1] + walLengthBytes - 1,
		"the length's high": offsets[1],
	}
	for name, index := range cases {
		t.Run(name, func(t *testing.T) {
			damaged := withFlippedBit(data, index)
			if err := os.WriteFile(path, damaged, 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := OpenWAL(path); !errors.Is(err, ErrWALCorrupt) {
				t.Fatalf("expected corruption to be reported, got %v", err)
			}
			if got := readWALFile(t, path); !bytes.Equal(got, damaged) {
				t.Fatal("a corrupt log must be left untouched for inspection")
			}
			if _, err := (&WAL{path: path}).Replay(); !errors.Is(err, ErrWALCorrupt) {
				t.Fatalf("expected replay to report corruption, got %v", err)
			}
		})
	}
}

func TestWALChecksumRejectsAFlippedBitInTheLastRecord(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	writeVotes(t, path, 0, 1)
	data := readWALFile(t, path)
	offsets := walRecordOffsets(t, data)
	for name, index := range map[string]int{
		"payload":  offsets[1] + walLengthBytes + 2,
		"checksum": offsets[2] - 1,
		"length":   offsets[1] + walLengthBytes - 1,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, withFlippedBit(data, index), 0o600); err != nil {
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
			if got := fileSize(t, path); got != int64(offsets[1]) {
				t.Fatalf("expected the damaged last record to be truncated to %d bytes, file has %d", offsets[1], got)
			}
		})
	}
}

// A damaged tail longer than any record cannot be the record a crash tore, so the records behind
// the damage that the log may hide are not given up silently.
func TestWALRefusesADamagedTailLongerThanARecord(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	writeVotes(t, path, 0)
	valid := readWALFile(t, path)

	appendBytes(t, path, bytes.Repeat([]byte{0xff}, walFrameBytes+maxWALPayload))
	wal := openTestWAL(t, path)
	wal.Close()
	if got := fileSize(t, path); got != int64(len(valid)) {
		t.Fatalf("expected a tail of the size of the largest record to be truncated, file has %d bytes", got)
	}

	appendBytes(t, path, bytes.Repeat([]byte{0xff}, walFrameBytes+maxWALPayload+1))
	damaged := readWALFile(t, path)
	if _, err := OpenWAL(path); !errors.Is(err, ErrWALCorrupt) {
		t.Fatalf("expected a tail longer than any record to be reported as corruption, got %v", err)
	}
	if got := readWALFile(t, path); !bytes.Equal(got, damaged) {
		t.Fatal("a corrupt log must be left untouched for inspection")
	}
}

// The header names the format. A log in another format is refused and kept as it is: reading it as a
// torn tail would truncate it, and with it the votes it records.
func TestWALRefusesAFileOfAnotherFormat(t *testing.T) {
	jsonLog := []byte("0f1e2d3c {\"type\":\"vote\",\"time\":\"2024-01-01T00:00:00Z\",\"validator_id\":\"alice\",\"round\":3}\n")
	otherVersion := slices.Clone(walHeader)
	otherVersion[len(otherVersion)-1]++
	cases := map[string][]byte{
		"json lines":                     jsonLog,
		"another version":                append(otherVersion, walRecord(t, walVoteEvent(0))...),
		"a cut header that then differs": append(slices.Clone(walHeader[:3]), 'x'),
		"text":                           []byte("this is not a write-ahead log"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir() + "/consensus.wal"
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenWAL(path); !errors.Is(err, ErrWALCorrupt) {
				t.Fatalf("expected a file of another format to be refused, got %v", err)
			}
			if _, err := (&WAL{path: path}).Replay(); !errors.Is(err, ErrWALCorrupt) {
				t.Fatalf("expected replay to refuse a file of another format, got %v", err)
			}
			if got := readWALFile(t, path); !bytes.Equal(got, content) {
				t.Fatal("a file of another format must be left untouched")
			}
		})
	}
}

// A crash while the header of a new log was written leaves a part of it, which holds no records.
func TestWALCompletesATornHeader(t *testing.T) {
	for length := range len(walHeader) {
		path := t.TempDir() + "/consensus.wal"
		if err := os.WriteFile(path, walHeader[:length], 0o600); err != nil {
			t.Fatal(err)
		}
		events, err := (&WAL{path: path}).Replay()
		if err != nil || len(events) != 0 {
			t.Fatalf("a header of %d bytes: expected no events, got %v %v", length, events, err)
		}
		wal := openTestWAL(t, path)
		if got := readWALFile(t, path); !bytes.Equal(got, walHeader) {
			t.Fatalf("a header of %d bytes: expected the header to be completed, got %q", length, got)
		}
		if err := wal.RecordVote(Vote{Height: 1, Round: 0, BlockHash: types.Hash{1}, ValidatorID: "alice"}); err != nil {
			t.Fatal(err)
		}
		if err := wal.Close(); err != nil {
			t.Fatal(err)
		}
		reopened := openTestWAL(t, path)
		if _, found, _ := reopened.RecordedVote("alice", 1, 0); !found {
			t.Fatalf("a header of %d bytes: the vote recorded after completing the header is lost", length)
		}
		reopened.Close()
	}
}

// A record that matches its checksum was written whole. If it does not decode, the log was written
// by something else, and dropping it as a torn tail would drop what it records.
func TestWALRefusesARecordThatMatchesItsChecksumButDoesNotDecode(t *testing.T) {
	valid := walRecord(t, walVoteEvent(0))
	payload := slices.Clone(valid[walLengthBytes : len(valid)-walChecksumBytes])
	unknownKind := slices.Clone(payload)
	unknownKind[0] = 99
	cases := map[string][]byte{
		"an unknown kind": unknownKind,
		"bytes left over": append(slices.Clone(payload), 0),
		"a cut payload":   payload[:len(payload)-1],
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir() + "/consensus.wal"
			log := append(slices.Clone(walHeader), valid...)
			log = append(log, walFrame(content)...)
			if err := os.WriteFile(path, log, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenWAL(path); !errors.Is(err, ErrWALCorrupt) {
				t.Fatalf("expected a record that does not decode to be reported as corruption, got %v", err)
			}
			if got := readWALFile(t, path); !bytes.Equal(got, log) {
				t.Fatal("a corrupt log must be left untouched for inspection")
			}
		})
	}
}

// The log reads its records through a buffer, and a record that straddles the end of the buffer must
// read as well as one that does not.
func TestWALOpensALogLargerThanItsReadBuffer(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	log := slices.Clone(walHeader)
	const votes = 4000
	for round := range types.Round(votes) {
		event := walVoteEvent(round)
		event.BlockHash = types.Hash{byte(round), byte(round >> 8)}
		log = append(log, walRecord(t, event)...)
		if round%1000 == 500 {
			state := SafetyState{HighQC: finality.QuorumCert{Height: 1, Round: round, Signature: bytes.Repeat([]byte{byte(round)}, 100_000)}, LastVotedHeight: 1, LastVotedRound: round}
			log = append(log, walRecord(t, WALEvent{Type: WALEventSafetyState, Time: walTestTime, Height: 1, Round: round, Safety: &state})...)
		}
	}
	if len(log) < 4*walReadBytes {
		t.Fatalf("the log of %d bytes must be larger than its read buffer of %d", len(log), walReadBytes)
	}
	if err := os.WriteFile(path, log, 0o600); err != nil {
		t.Fatal(err)
	}

	wal := openTestWAL(t, path)
	defer wal.Close()
	for _, round := range []types.Round{0, 1, 500, 1999, 2500, 3999} {
		hash, found, err := wal.RecordedVote("alice", 1, round)
		if err != nil || !found || hash != (types.Hash{byte(round), byte(round >> 8)}) {
			t.Fatalf("vote of round %d: found=%t hash=%x err=%v", round, found, hash, err)
		}
	}
	state, found, err := wal.SafetyState()
	if err != nil || !found || state.LastVotedRound != 3500 || len(state.HighQC.Signature) != 100_000 {
		t.Fatalf("unexpected safety state: found=%t err=%v round=%d", found, err, state.LastVotedRound)
	}
	events, err := wal.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != votes+4 {
		t.Fatalf("expected %d events, got %d", votes+4, len(events))
	}
}

// Every field of an event survives the log, whatever the kind of the event.
func TestWALEventsRoundTripEveryField(t *testing.T) {
	vote := filledWire[TimeoutVote](t)
	state := filledWire[SafetyState](t)
	events := []WALEvent{
		{Type: WALEventProposal, Time: walTestTime, ValidatorID: "alice", Height: 7, Round: 8, BlockHash: types.Hash{9, 10}},
		{Type: WALEventVote, Time: walTestTime.Add(time.Second), ValidatorID: "bob", Height: 11, Round: 12, BlockHash: types.Hash{13, 14}},
		{Type: WALEventTimeoutVote, Time: walTestTime.Add(2 * time.Second), ValidatorID: vote.ValidatorID, Height: vote.Height, Round: vote.Round, TimeoutVote: &vote},
		{Type: WALEventSafetyState, Time: walTestTime.Add(3 * time.Second), Height: state.LastVotedHeight, Round: state.LastVotedRound, Safety: &state},
	}
	eventType := reflect.TypeOf(WALEvent{})
	for index := range eventType.NumField() {
		covered := false
		for _, event := range events {
			covered = covered || !reflect.ValueOf(event).Field(index).IsZero()
		}
		if !covered {
			t.Fatalf("no event of the test sets %s, which a round trip cannot show", eventType.Field(index).Name)
		}
	}

	log := slices.Clone(walHeader)
	for _, event := range events {
		log = append(log, walRecord(t, event)...)
	}
	var replayed []WALEvent
	valid, err := scanWAL(bytes.NewReader(log), int64(len(log)), func(event WALEvent) { replayed = append(replayed, event) })
	if err != nil || valid != int64(len(log)) {
		t.Fatalf("expected the whole log of %d bytes to read, got %d bytes and %v", len(log), valid, err)
	}
	if !reflect.DeepEqual(replayed, events) {
		t.Fatalf("the events changed in the log\n got: %+v\nwant: %+v", replayed, events)
	}
}

// A record the log could not read back would make the next start fail after the validator had
// signed on the strength of it, so the log refuses it before it is written.
func TestWALRefusesWhatItCouldNotReadBack(t *testing.T) {
	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	defer wal.Close()
	size := fileSize(t, path)

	longID := types.ValidatorID(strings.Repeat("v", types.MaxIDBytes+1))
	if err := wal.RecordVote(Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: longID}); err == nil {
		t.Fatal("expected a vote of a validator id over the limit to be refused")
	}
	if _, found, _ := wal.RecordedVote(longID, 1, 1); found {
		t.Fatal("a refused vote must not be recorded")
	}
	oversized := TimeoutVote{Height: 1, Round: 1, ValidatorID: "alice", Signature: make(types.Signature, types.MaxSignatureBytes+1)}
	if err := wal.RecordTimeoutVote(oversized); err == nil {
		t.Fatal("expected a timeout vote with a signature over the limit to be refused")
	}
	if _, found, _ := wal.RecordedTimeoutVote("alice", 1); found {
		t.Fatal("a refused timeout vote must not be recorded")
	}
	huge := SafetyState{HighQC: finality.QuorumCert{Height: 1, Signature: make(types.AggregateSignature, finality.MaxAggregateSignatureBytes+1)}}
	if err := wal.RecordSafetyState(huge); err == nil {
		t.Fatal("expected a safety state with a signature over the limit to be refused")
	}
	if _, found, _ := wal.SafetyState(); found {
		t.Fatal("a refused safety state must not be recorded")
	}
	for name, event := range map[string]WALEvent{
		"an unknown type":           {Type: "unknown", Time: walTestTime},
		"a timeout vote without it": {Type: WALEventTimeoutVote, Time: walTestTime},
		"a state without it":        {Type: WALEventSafetyState, Time: walTestTime},
	} {
		if _, err := appendWALRecord(nil, event); err == nil {
			t.Fatalf("%s: expected the event to be refused", name)
		}
	}
	if got := fileSize(t, path); got != size {
		t.Fatalf("refused records must not reach the log: it grew from %d to %d bytes", size, got)
	}
	if err := wal.RecordVote(Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"}); err != nil {
		t.Fatalf("expected the log to keep working after refusing records, got %v", err)
	}
}

// FuzzScanWAL checks that no log makes the scan panic, and that what the scan accepts is a log of
// events with one encoding: the bytes of the accepted prefix.
func FuzzScanWAL(f *testing.F) {
	vote := filledWire[TimeoutVote](f)
	state := filledWire[SafetyState](f)
	full := slices.Clone(walHeader)
	for _, event := range []WALEvent{
		walVoteEvent(0),
		{Type: WALEventProposal, Time: walTestTime, ValidatorID: "alice", Height: 1, Round: 2, BlockHash: types.Hash{3}},
		{Type: WALEventTimeoutVote, Time: walTestTime, ValidatorID: vote.ValidatorID, Height: vote.Height, Round: vote.Round, TimeoutVote: &vote},
		{Type: WALEventSafetyState, Time: walTestTime, Height: state.LastVotedHeight, Round: state.LastVotedRound, Safety: &state},
	} {
		record, err := appendWALRecord(nil, event)
		if err != nil {
			f.Fatal(err)
		}
		full = append(full, record...)
	}
	f.Add(full)
	f.Add(full[:len(full)-3])
	f.Add(full[:len(walHeader)+5])
	f.Add(slices.Clone(walHeader))
	f.Add(walHeader[:3])
	f.Add([]byte{})
	f.Add([]byte("0f1e2d3c {\"type\":\"vote\"}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var events []WALEvent
		valid, err := scanWAL(bytes.NewReader(data), int64(len(data)), func(event WALEvent) { events = append(events, event) })
		if err != nil {
			return
		}
		if valid < 0 || valid > int64(len(data)) {
			t.Fatalf("the valid prefix of %d bytes lies outside the log of %d bytes", valid, len(data))
		}
		if valid == 0 {
			if len(events) != 0 {
				t.Fatalf("a log without records returned %d events", len(events))
			}
			return
		}
		prefix := data[:valid]
		var again []WALEvent
		validAgain, err := scanWAL(bytes.NewReader(prefix), valid, func(event WALEvent) { again = append(again, event) })
		if err != nil || validAgain != valid || !reflect.DeepEqual(again, events) {
			t.Fatalf("the valid prefix of a log must read as itself: %d bytes %v, %d events and %d events", validAgain, err, len(again), len(events))
		}
		rebuilt := slices.Clone(walHeader)
		for _, event := range events {
			rebuilt, err = appendWALRecord(rebuilt, event)
			if err != nil {
				t.Fatalf("an event read from a log does not encode: %v", err)
			}
		}
		if !bytes.Equal(rebuilt, prefix) {
			t.Fatalf("the events of a log encode to %x, not to the %x they were read from", rebuilt, prefix)
		}
	})
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

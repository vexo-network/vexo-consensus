package consensus

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrWALClosed          = errors.New("consensus wal is closed")
	ErrDoubleSignDetected = errors.New("double sign detected")
	ErrWALCorrupt         = errors.New("consensus wal is corrupt")
	ErrSafetyRegression   = errors.New("safety state regression")
)

type WALEventType string

const (
	WALEventProposal    WALEventType = "proposal"
	WALEventVote        WALEventType = "vote"
	WALEventTimeoutVote WALEventType = "timeout_vote"
	WALEventSafetyState WALEventType = "safety_state"
)

// WALEvent is one record of the log. Its Type tells which of the other fields it has: a proposal
// and a vote have a validator, a height, a round and a block hash; a timeout vote has the vote,
// whose validator, height and round it repeats; a safety state has the state, whose last voted
// height and round it repeats as its height and round.
type WALEvent struct {
	Type        WALEventType
	Time        time.Time
	ValidatorID types.ValidatorID
	Height      types.Height
	Round       types.Round
	BlockHash   types.Hash
	TimeoutVote *TimeoutVote
	Safety      *SafetyState
}

// WAL is the write-ahead log of the local validator's signing decisions. The file is a header
// that names the format, followed by records, and every record is fsynced before the append
// returns. A record is the length of its payload, the payload, and the CRC-32C of both. A record
// holds one event, or a group of events that must reach the log together: a vote and the safety
// state it is cast under are one record, so they cost one fsync and a crash keeps both or neither. A torn
// tail from a crash mid-write is detected by the length and the checksum and truncated when the log
// is opened; a damaged record that is followed by an intact record is reported as corruption, and
// so is a file that does not start with the header.
type WAL struct {
	mu        sync.Mutex
	path      string
	file      *os.File
	size      int64
	votes     map[types.ValidatorID]map[types.Round]walVote
	timeouts  map[types.ValidatorID]map[types.Round]TimeoutVote
	safety    SafetyState
	hasSafety bool
	closed    bool
	// staged holds the proposal records that wait for the next record the log syncs; see
	// RecordProposal.
	staged []WALEvent
}

// walVote is the single block a validator may vote for in one view.
type walVote struct {
	height    types.Height
	blockHash types.Hash
}

func OpenWAL(path string) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	wal := &WAL{
		path:     path,
		file:     file,
		votes:    make(map[types.ValidatorID]map[types.Round]walVote),
		timeouts: make(map[types.ValidatorID]map[types.Round]TimeoutVote),
	}
	if err := wal.recover(); err != nil {
		file.Close()
		return nil, err
	}
	syncDirectory(filepath.Dir(path))
	return wal, nil
}

// recover rebuilds the guards from the records of the log, drops a torn tail, and writes the
// header of a log that has none yet.
func (wal *WAL) recover() error {
	info, err := wal.file.Stat()
	if err != nil {
		return err
	}
	valid, err := scanWAL(wal.file, info.Size(), wal.applyRecord)
	if err != nil {
		return err
	}
	if valid < info.Size() {
		if err := wal.file.Truncate(valid); err != nil {
			return fmt.Errorf("truncate torn wal tail: %w", err)
		}
	}
	if valid == 0 {
		if _, err := wal.file.Write(walHeader); err != nil {
			return err
		}
		valid = int64(len(walHeader))
	}
	if err := wal.file.Sync(); err != nil {
		return err
	}
	wal.size = valid
	return nil
}

func (wal *WAL) Path() string {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	return wal.path
}

func (wal *WAL) Close() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if wal.closed {
		return ErrWALClosed
	}
	wal.closed = true
	if wal.file == nil {
		return nil
	}
	var stagedErr error
	if len(wal.staged) > 0 {
		stagedErr = wal.appendLocked()
	}
	return errors.Join(stagedErr, wal.file.Close())
}

// RecordProposal records the block the validator proposes in a view. Recovery never reads a
// proposal record, which documents what the validator proposed, so it costs no fsync of its own:
// it is written with the next record the log syncs, the vote the proposer casts for its proposal
// before the proposal leaves the node. A crash that loses the record loses that vote with it, so
// the proposal was never sent. Close writes a record that is still waiting.
func (wal *WAL) RecordProposal(proposal Proposal) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	event := WALEvent{
		Type:        WALEventProposal,
		Time:        time.Now().UTC(),
		ValidatorID: proposal.Proposer,
		Height:      proposal.Block.Header.Height,
		Round:       proposal.Round,
		BlockHash:   HashBlock(proposal.Block),
	}
	if _, err := appendWALRecord(nil, event); err != nil {
		return err
	}
	wal.staged = append(wal.staged, event)
	return nil
}

// RecordVote persists the validator's vote for a view. A validator may vote for one block
// per view, so a second block in the same view is a double sign.
func (wal *WAL) RecordVote(vote Vote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	if err := wal.checkVote(vote); err != nil {
		return err
	}
	if err := wal.appendLocked(voteEvent(vote)); err != nil {
		return err
	}
	wal.rememberVote(vote)
	return nil
}

// RecordVoteWithSafetyState persists the safety state the validator votes under and its vote for a
// view in one record, with one fsync: a restart finds both or neither. It refuses what
// RecordSafetyState and RecordVote refuse, and records nothing then.
func (wal *WAL) RecordVoteWithSafetyState(state SafetyState, vote Vote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	if err := wal.checkSafetyState(state); err != nil {
		return err
	}
	if err := wal.checkVote(vote); err != nil {
		return err
	}
	if err := wal.appendLocked(safetyStateEvent(state), voteEvent(vote)); err != nil {
		return err
	}
	wal.rememberSafetyState(state)
	wal.rememberVote(vote)
	return nil
}

func (wal *WAL) checkVote(vote Vote) error {
	if previous, ok := wal.votes[vote.ValidatorID][vote.Round]; ok && (previous.blockHash != vote.BlockHash || previous.height != vote.Height) {
		return fmt.Errorf("%w: validator=%s round=%d previous=%d/%x next=%d/%x", ErrDoubleSignDetected, vote.ValidatorID, vote.Round, previous.height, previous.blockHash, vote.Height, vote.BlockHash)
	}
	return nil
}

func (wal *WAL) rememberVote(vote Vote) {
	if wal.votes[vote.ValidatorID] == nil {
		wal.votes[vote.ValidatorID] = make(map[types.Round]walVote)
	}
	wal.votes[vote.ValidatorID][vote.Round] = walVote{height: vote.Height, blockHash: vote.BlockHash}
}

func voteEvent(vote Vote) WALEvent {
	return WALEvent{
		Type:        WALEventVote,
		Time:        time.Now().UTC(),
		ValidatorID: vote.ValidatorID,
		Height:      vote.Height,
		Round:       vote.Round,
		BlockHash:   vote.BlockHash,
	}
}

// RecordedVote returns the block previously selected by a validator for a height and
// round. Callers use it to restore the local anti-double-sign decision before signing
// again after a restart.
func (wal *WAL) RecordedVote(validatorID types.ValidatorID, height types.Height, round types.Round) (types.Hash, bool, error) {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return types.Hash{}, false, err
	}
	recorded, ok := wal.votes[validatorID][round]
	if !ok || recorded.height != height {
		return types.Hash{}, false, nil
	}
	return recorded.blockHash, true, nil
}

// RecordTimeoutVote persists the complete signed timeout vote so it can be replayed after a
// restart. A validator may time out once per view; a different HighQC is a double sign.
func (wal *WAL) RecordTimeoutVote(vote TimeoutVote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	if err := wal.checkTimeoutVote(vote); err != nil {
		return err
	}
	if err := wal.appendLocked(timeoutVoteEvent(vote)); err != nil {
		return err
	}
	wal.rememberTimeoutVote(vote)
	return nil
}

// RecordTimeoutVoteWithSafetyState persists the safety state the validator leaves a view under and
// its timeout vote for the view in one record, with one fsync: a restart finds both or neither. It
// refuses what RecordSafetyState and RecordTimeoutVote refuse, and records nothing then.
func (wal *WAL) RecordTimeoutVoteWithSafetyState(state SafetyState, vote TimeoutVote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	if err := wal.checkSafetyState(state); err != nil {
		return err
	}
	if err := wal.checkTimeoutVote(vote); err != nil {
		return err
	}
	if err := wal.appendLocked(safetyStateEvent(state), timeoutVoteEvent(vote)); err != nil {
		return err
	}
	wal.rememberSafetyState(state)
	wal.rememberTimeoutVote(vote)
	return nil
}

func (wal *WAL) checkTimeoutVote(vote TimeoutVote) error {
	if previous, ok := wal.timeouts[vote.ValidatorID][vote.Round]; ok && !sameQC(previous.HighQC, vote.HighQC) {
		return fmt.Errorf("%w: timeout validator=%s round=%d", ErrDoubleSignDetected, vote.ValidatorID, vote.Round)
	}
	return nil
}

func (wal *WAL) rememberTimeoutVote(vote TimeoutVote) {
	if wal.timeouts[vote.ValidatorID] == nil {
		wal.timeouts[vote.ValidatorID] = make(map[types.Round]TimeoutVote)
	}
	wal.timeouts[vote.ValidatorID][vote.Round] = vote
}

func timeoutVoteEvent(vote TimeoutVote) WALEvent {
	return WALEvent{
		Type:        WALEventTimeoutVote,
		Time:        time.Now().UTC(),
		ValidatorID: vote.ValidatorID,
		Height:      vote.Height,
		Round:       vote.Round,
		TimeoutVote: &vote,
	}
}

// RecordedTimeoutVote returns the timeout vote the validator signed in a view, so a
// restarted node re-sends it instead of signing a different one.
func (wal *WAL) RecordedTimeoutVote(validatorID types.ValidatorID, round types.Round) (TimeoutVote, bool, error) {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return TimeoutVote{}, false, err
	}
	vote, ok := wal.timeouts[validatorID][round]
	return vote, ok, nil
}

// RecordSafetyState persists the consensus safety state with fsync. The last voted view
// and the rounds of the locked and highest QC never move backwards.
func (wal *WAL) RecordSafetyState(state SafetyState) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	if err := wal.checkSafetyState(state); err != nil {
		return err
	}
	if err := wal.appendLocked(safetyStateEvent(state)); err != nil {
		return err
	}
	wal.rememberSafetyState(state)
	return nil
}

func (wal *WAL) checkSafetyState(state SafetyState) error {
	if !wal.hasSafety {
		return nil
	}
	return checkSafetyProgress(wal.safety, state)
}

func (wal *WAL) rememberSafetyState(state SafetyState) {
	wal.safety = state
	wal.hasSafety = true
}

func safetyStateEvent(state SafetyState) WALEvent {
	return WALEvent{
		Type:   WALEventSafetyState,
		Time:   time.Now().UTC(),
		Height: state.LastVotedHeight,
		Round:  state.LastVotedRound,
		Safety: &state,
	}
}

// SafetyState returns the most recently persisted safety state.
func (wal *WAL) SafetyState() (SafetyState, bool, error) {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return SafetyState{}, false, err
	}
	return wal.safety, wal.hasSafety, nil
}

func checkSafetyProgress(previous SafetyState, next SafetyState) error {
	if next.LastVotedRound < previous.LastVotedRound {
		return fmt.Errorf("%w: last voted round %d below recorded %d", ErrSafetyRegression, next.LastVotedRound, previous.LastVotedRound)
	}
	if regressesQC(previous.LockedQC, next.LockedQC) {
		return fmt.Errorf("%w: locked qc round %d below recorded %d", ErrSafetyRegression, next.LockedQC.Round, previous.LockedQC.Round)
	}
	if regressesQC(previous.HighQC, next.HighQC) {
		return fmt.Errorf("%w: high qc round %d below recorded %d", ErrSafetyRegression, next.HighQC.Round, previous.HighQC.Round)
	}
	return nil
}

func regressesQC(previous finality.QuorumCert, next finality.QuorumCert) bool {
	if previous.Height == 0 {
		return false
	}
	return next.Height == 0 || next.Round < previous.Round
}

// Replay returns every valid record in the log. A torn tail is ignored; a damaged record
// followed by an intact record is an error. It does not modify the log.
func (wal *WAL) Replay() ([]WALEvent, error) {
	file, err := os.Open(wal.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	var events []WALEvent
	if _, err := scanWAL(file, info.Size(), func(record []WALEvent) { events = append(events, record...) }); err != nil {
		return nil, err
	}
	return events, nil
}

// applyRecord rebuilds the guards from the events of one record, in order.
func (wal *WAL) applyRecord(record []WALEvent) {
	for _, event := range record {
		wal.apply(event)
	}
}

func (wal *WAL) apply(event WALEvent) {
	switch event.Type {
	case WALEventVote:
		if wal.votes[event.ValidatorID] == nil {
			wal.votes[event.ValidatorID] = make(map[types.Round]walVote)
		}
		wal.votes[event.ValidatorID][event.Round] = walVote{height: event.Height, blockHash: event.BlockHash}
	case WALEventTimeoutVote:
		if event.TimeoutVote == nil {
			return
		}
		if wal.timeouts[event.ValidatorID] == nil {
			wal.timeouts[event.ValidatorID] = make(map[types.Round]TimeoutVote)
		}
		wal.timeouts[event.ValidatorID][event.Round] = *event.TimeoutVote
	case WALEventSafetyState:
		if event.Safety == nil {
			return
		}
		wal.safety = *event.Safety
		wal.hasSafety = true
	}
}

// appendLocked writes the staged proposal records and events as one record and syncs it. The
// staged records are dropped once they are durable; a failed append keeps them for the next one.
func (wal *WAL) appendLocked(events ...WALEvent) error {
	if len(wal.staged) > 0 {
		events = append(slices.Clone(wal.staged), events...)
	}
	record, err := appendWALRecord(nil, events...)
	if err != nil {
		return err
	}
	if _, err := wal.file.Write(record); err != nil {
		// Drop whatever part of the record reached the file so the next append starts on
		// a record boundary.
		_ = wal.file.Truncate(wal.size)
		return err
	}
	if err := wal.file.Sync(); err != nil {
		// An unsynced record must not count as persisted: drop it so the log and the
		// in-memory view agree, and let the caller refuse to sign.
		_ = wal.file.Truncate(wal.size)
		return err
	}
	wal.size += int64(len(record))
	wal.staged = nil
	return nil
}

func (wal *WAL) ensureOpen() error {
	if wal.closed || wal.file == nil {
		return ErrWALClosed
	}
	return nil
}

// walHeader begins every log and names its format. A file of another format, such as a log of the
// JSON lines an earlier version wrote, is refused as corrupt instead of being read as a damaged tail
// and truncated, which would erase the votes it records.
var walHeader = []byte("vexo-wal\x01")

// A record is its length, its payload and its checksum. The length is the number of bytes of the
// payload, four bytes big-endian, and the checksum is the CRC-32C of the length and the payload,
// four bytes big-endian. The payload is the kind of the record, its time in nanoseconds since the
// Unix epoch as eight bytes big-endian, and the fields of the kind in the canonical encoding of
// package types, so that a record has exactly one encoding.
const (
	walLengthBytes   = 4
	walChecksumBytes = 4
	walFrameBytes    = walLengthBytes + walChecksumBytes
	// maxWALPayload bounds a payload: the largest record holds two certificates, each of the largest
	// signer list and signature.
	maxWALPayload = 8 << 20
	// walReadBytes is the size of the buffer that reads the log when it is opened.
	walReadBytes = 64 << 10
)

const (
	walKindProposal byte = iota + 1
	walKindVote
	walKindTimeoutVote
	walKindSafetyState
	// walKindGroup is a record of several events: a count of at least two, then the payload of each
	// event, length-prefixed. An event of a group is not itself a group.
	walKindGroup
)

var walChecksums = crc32.MakeTable(crc32.Castagnoli)

// appendWALRecord appends the record of events to dst: the record of the event, or a group record
// of several. It refuses events that the log could not read back, so that no record appended and
// fsynced can make the log fail to open.
func appendWALRecord(dst []byte, events ...WALEvent) ([]byte, error) {
	start := len(dst)
	dst = append(dst, 0, 0, 0, 0)
	var err error
	switch len(events) {
	case 0:
		return dst[:start], errors.New("wal record without an event")
	case 1:
		dst, err = appendWALPayload(dst, events[0])
	default:
		dst = append(dst, walKindGroup)
		dst = binary.AppendUvarint(dst, uint64(len(events)))
		var nested []byte
		for _, event := range events {
			if nested, err = appendWALPayload(nested[:0], event); err != nil {
				break
			}
			dst = types.AppendBytes(dst, nested)
		}
	}
	if err != nil {
		return dst[:start], err
	}
	payload := dst[start+walLengthBytes:]
	if len(payload) > maxWALPayload {
		return dst[:start], fmt.Errorf("wal record of %d bytes is over the limit of %d", len(payload), maxWALPayload)
	}
	if _, err := readWALPayload(payload); err != nil {
		return dst[:start], fmt.Errorf("wal record does not read back: %w", err)
	}
	binary.BigEndian.PutUint32(dst[start:], uint32(len(payload)))
	return binary.BigEndian.AppendUint32(dst, crc32.Checksum(dst[start:], walChecksums)), nil
}

// appendWALPayload appends the payload of event to dst.
func appendWALPayload(dst []byte, event WALEvent) ([]byte, error) {
	start := len(dst)
	switch event.Type {
	case WALEventProposal:
		dst = append(dst, walKindProposal)
	case WALEventVote:
		dst = append(dst, walKindVote)
	case WALEventTimeoutVote:
		dst = append(dst, walKindTimeoutVote)
	case WALEventSafetyState:
		dst = append(dst, walKindSafetyState)
	default:
		return dst[:start], fmt.Errorf("unknown wal event type %q", event.Type)
	}
	dst = binary.BigEndian.AppendUint64(dst, uint64(event.Time.UnixNano()))
	switch event.Type {
	case WALEventProposal, WALEventVote:
		dst = types.AppendBytes(dst, event.ValidatorID)
		dst = binary.AppendUvarint(dst, uint64(event.Height))
		dst = binary.AppendUvarint(dst, uint64(event.Round))
		dst = append(dst, event.BlockHash[:]...)
	case WALEventTimeoutVote:
		if event.TimeoutVote == nil {
			return dst[:start], errors.New("wal timeout vote event without a vote")
		}
		dst = appendTimeoutVote(dst, *event.TimeoutVote)
	case WALEventSafetyState:
		if event.Safety == nil {
			return dst[:start], errors.New("wal safety state event without a state")
		}
		dst = finality.AppendQuorumCert(dst, event.Safety.LockedQC)
		dst = finality.AppendQuorumCert(dst, event.Safety.HighQC)
		dst = binary.AppendUvarint(dst, uint64(event.Safety.LastVotedHeight))
		dst = binary.AppendUvarint(dst, uint64(event.Safety.LastVotedRound))
	}
	return dst, nil
}

// readWALPayload decodes the payload of a record into its events.
func readWALPayload(payload []byte) ([]WALEvent, error) {
	if len(payload) == 0 || payload[0] != walKindGroup {
		event, err := readWALEvent(payload)
		if err != nil {
			return nil, err
		}
		return []WALEvent{event}, nil
	}
	reader := types.NewReader(payload[1:])
	// An event takes at least its length, its kind and its time.
	count := reader.Count(maxWALPayload, 10)
	if reader.Err() == nil && count < 2 {
		reader.Fail("group of fewer than two events")
	}
	events := make([]WALEvent, 0, count)
	for range count {
		nested := reader.Bytes(maxWALPayload)
		if reader.Err() != nil {
			break
		}
		event, err := readWALEvent(nested)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := reader.Done(); err != nil {
		return nil, err
	}
	return events, nil
}

// readWALEvent decodes the payload of an event; a group is not an event.
func readWALEvent(payload []byte) (WALEvent, error) {
	reader := types.NewReader(payload)
	kind := reader.Byte()
	event := WALEvent{Time: time.Unix(0, reader.Int64()).UTC()}
	switch kind {
	case walKindProposal, walKindVote:
		event.Type = WALEventProposal
		if kind == walKindVote {
			event.Type = WALEventVote
		}
		event.ValidatorID = types.ValidatorID(reader.String(types.MaxIDBytes))
		event.Height = types.Height(reader.Uvarint())
		event.Round = types.Round(reader.Uvarint())
		event.BlockHash = reader.Hash()
	case walKindTimeoutVote:
		vote := readTimeoutVote(reader)
		event.Type = WALEventTimeoutVote
		event.ValidatorID, event.Height, event.Round = vote.ValidatorID, vote.Height, vote.Round
		event.TimeoutVote = &vote
	case walKindSafetyState:
		var state SafetyState
		state.LockedQC = finality.ReadQuorumCert(reader)
		state.HighQC = finality.ReadQuorumCert(reader)
		state.LastVotedHeight = types.Height(reader.Uvarint())
		state.LastVotedRound = types.Round(reader.Uvarint())
		event.Type = WALEventSafetyState
		event.Height, event.Round = state.LastVotedHeight, state.LastVotedRound
		event.Safety = &state
	default:
		reader.Fail("unknown record kind")
	}
	if err := reader.Done(); err != nil {
		return WALEvent{}, err
	}
	return event, nil
}

// parseWALRecord parses the record at the front of data into its events. It returns ok false when
// the bytes are not an intact record: they end early, their length is out of range, or their
// checksum does not match. A record that matches its checksum and does not decode is an error,
// because a crash does not produce one.
func parseWALRecord(data []byte) (events []WALEvent, length int, ok bool, err error) {
	if len(data) < walFrameBytes+1 {
		return nil, 0, false, nil
	}
	payloadLength := uint64(binary.BigEndian.Uint32(data))
	if payloadLength == 0 || payloadLength > maxWALPayload || walFrameBytes+payloadLength > uint64(len(data)) {
		return nil, 0, false, nil
	}
	length = walFrameBytes + int(payloadLength)
	checked, checksum := data[:length-walChecksumBytes], binary.BigEndian.Uint32(data[length-walChecksumBytes:length])
	if crc32.Checksum(checked, walChecksums) != checksum {
		return nil, 0, false, nil
	}
	events, err = readWALPayload(checked[walLengthBytes:])
	if err != nil {
		return nil, 0, false, fmt.Errorf("%w: a record that matches its checksum does not decode: %w", ErrWALCorrupt, err)
	}
	return events, length, true, nil
}

// readWALRecord reads the record at the front of reader, which has remaining bytes left, into
// buffer and parses it.
func readWALRecord(reader io.Reader, remaining int64, buffer *[]byte) (events []WALEvent, length int, ok bool, err error) {
	if remaining < walFrameBytes+1 {
		return nil, 0, false, nil
	}
	var frame [walLengthBytes]byte
	if _, err := io.ReadFull(reader, frame[:]); err != nil {
		return nil, 0, false, err
	}
	payloadLength := int64(binary.BigEndian.Uint32(frame[:]))
	if payloadLength == 0 || payloadLength > maxWALPayload || walFrameBytes+payloadLength > remaining {
		return nil, 0, false, nil
	}
	record := slices.Grow((*buffer)[:0], int(walFrameBytes+payloadLength))[:walFrameBytes+payloadLength]
	*buffer = record
	copy(record, frame[:])
	if _, err := io.ReadFull(reader, record[walLengthBytes:]); err != nil {
		return nil, 0, false, err
	}
	return parseWALRecord(record)
}

// scanWAL reads the log in src, which holds size bytes, and passes the events of every intact record
// to visit in order. It returns the length of the prefix of the log that ends the last intact
// record, which is shorter than size when the log ends in a torn record. A log of zero bytes or of a
// part of the header is a log that was never written, and has the prefix 0.
func scanWAL(src io.ReaderAt, size int64, visit func([]WALEvent)) (int64, error) {
	head := make([]byte, min(size, int64(len(walHeader))))
	if _, err := io.ReadFull(io.NewSectionReader(src, 0, int64(len(head))), head); err != nil {
		return 0, err
	}
	if !bytes.HasPrefix(walHeader, head) {
		return 0, fmt.Errorf("%w: the file does not start with the header of a consensus wal", ErrWALCorrupt)
	}
	if len(head) < len(walHeader) {
		return 0, nil
	}
	valid := int64(len(walHeader))
	reader := bufio.NewReaderSize(io.NewSectionReader(src, valid, size-valid), walReadBytes)
	var buffer []byte
	for valid < size {
		events, length, ok, err := readWALRecord(reader, size-valid, &buffer)
		if err != nil {
			return 0, err
		}
		if !ok {
			break
		}
		visit(events)
		valid += int64(length)
	}
	if valid < size {
		if err := checkWALTail(src, valid, size); err != nil {
			return 0, err
		}
	}
	return valid, nil
}

// checkWALTail judges the bytes of the log from the damaged record on, which are not an intact
// record. They are the torn tail of a crash only when a crash could have left them: a record is
// appended whole and fsynced before the next one, so the tail is shorter than one record and no intact
// record follows the damage. Otherwise the damage is in the middle of the log, and the records it
// hides cannot be trusted.
func checkWALTail(src io.ReaderAt, damaged, size int64) error {
	if size-damaged > walFrameBytes+maxWALPayload {
		return fmt.Errorf("%w: %d damaged bytes from offset %d", ErrWALCorrupt, size-damaged, damaged)
	}
	tail := make([]byte, size-damaged)
	if _, err := io.ReadFull(io.NewSectionReader(src, damaged, size-damaged), tail); err != nil {
		return err
	}
	for offset := 1; offset < len(tail); offset++ {
		if _, _, ok, err := parseWALRecord(tail[offset:]); ok || err != nil {
			return fmt.Errorf("%w: an intact record at offset %d follows the damaged record at offset %d", ErrWALCorrupt, damaged+int64(offset), damaged)
		}
	}
	return nil
}

func syncDirectory(path string) {
	directory, err := os.Open(path)
	if err != nil {
		return
	}
	_ = directory.Sync()
	_ = directory.Close()
}

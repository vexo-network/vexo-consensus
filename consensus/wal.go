package consensus

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
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

type WALEvent struct {
	Type        WALEventType      `json:"type"`
	Time        time.Time         `json:"time"`
	ValidatorID types.ValidatorID `json:"validator_id,omitempty"`
	Height      types.Height      `json:"height,omitempty"`
	Round       types.Round       `json:"round,omitempty"`
	BlockHash   string            `json:"block_hash,omitempty"`
	HighQC      string            `json:"high_qc,omitempty"`
	TimeoutVote *TimeoutVote      `json:"timeout_vote,omitempty"`
	Safety      *SafetyState      `json:"safety_state,omitempty"`
}

// WAL is the write-ahead log of the local validator's signing decisions. Every record is
// one line "<crc32 hex> <json>\n" and is fsynced before the append returns. A torn tail
// from a crash mid-write is detected by the checksum and truncated when the log is
// opened; a damaged record that is followed by valid records is reported as corruption.
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
}

// walVote is the single block a validator may vote for in one view.
type walVote struct {
	height    types.Height
	blockHash string
}

func OpenWAL(path string) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	scan, err := scanWAL(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if info.Size() > scan.validBytes {
		if err := file.Truncate(scan.validBytes); err != nil {
			file.Close()
			return nil, fmt.Errorf("truncate torn wal tail: %w", err)
		}
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, err
	}
	syncDirectory(filepath.Dir(path))

	wal := &WAL{
		path:     path,
		file:     file,
		size:     scan.validBytes,
		votes:    make(map[types.ValidatorID]map[types.Round]walVote),
		timeouts: make(map[types.ValidatorID]map[types.Round]TimeoutVote),
	}
	for _, event := range scan.events {
		wal.apply(event)
	}
	return wal, nil
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
	return wal.file.Close()
}

func (wal *WAL) RecordProposal(proposal Proposal) error {
	return wal.append(WALEvent{
		Type:        WALEventProposal,
		Time:        time.Now().UTC(),
		ValidatorID: proposal.Proposer,
		Height:      proposal.Block.Header.Height,
		Round:       proposal.Round,
		BlockHash:   hashString(HashBlock(proposal.Block)),
	})
}

// RecordVote persists the validator's vote for a view. A validator may vote for one block
// per view, so a second block in the same view is a double sign.
func (wal *WAL) RecordVote(vote Vote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	blockHash := hashString(vote.BlockHash)
	if previous, ok := wal.votes[vote.ValidatorID][vote.Round]; ok && (previous.blockHash != blockHash || previous.height != vote.Height) {
		return fmt.Errorf("%w: validator=%s round=%d previous=%d/%s next=%d/%s", ErrDoubleSignDetected, vote.ValidatorID, vote.Round, previous.height, previous.blockHash, vote.Height, blockHash)
	}
	if err := wal.appendLocked(WALEvent{
		Type:        WALEventVote,
		Time:        time.Now().UTC(),
		ValidatorID: vote.ValidatorID,
		Height:      vote.Height,
		Round:       vote.Round,
		BlockHash:   blockHash,
	}); err != nil {
		return err
	}
	if wal.votes[vote.ValidatorID] == nil {
		wal.votes[vote.ValidatorID] = make(map[types.Round]walVote)
	}
	wal.votes[vote.ValidatorID][vote.Round] = walVote{height: vote.Height, blockHash: blockHash}
	return nil
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
	decoded, err := hex.DecodeString(recorded.blockHash)
	if err != nil {
		return types.Hash{}, false, fmt.Errorf("decode recorded vote hash: %w", err)
	}
	var blockHash types.Hash
	if len(decoded) != len(blockHash) {
		return types.Hash{}, false, fmt.Errorf("decode recorded vote hash: expected %d bytes, got %d", len(blockHash), len(decoded))
	}
	copy(blockHash[:], decoded)
	return blockHash, true, nil
}

// RecordTimeoutVote persists the complete signed timeout vote so it can be replayed after a
// restart. A validator may time out once per view; a different HighQC is a double sign.
func (wal *WAL) RecordTimeoutVote(vote TimeoutVote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	if previous, ok := wal.timeouts[vote.ValidatorID][vote.Round]; ok && !sameQC(previous.HighQC, vote.HighQC) {
		return fmt.Errorf("%w: timeout validator=%s round=%d", ErrDoubleSignDetected, vote.ValidatorID, vote.Round)
	}
	if err := wal.appendLocked(WALEvent{
		Type:        WALEventTimeoutVote,
		Time:        time.Now().UTC(),
		ValidatorID: vote.ValidatorID,
		Height:      vote.Height,
		Round:       vote.Round,
		HighQC:      quorumCertFingerprint(vote.HighQC),
		TimeoutVote: &vote,
	}); err != nil {
		return err
	}
	if wal.timeouts[vote.ValidatorID] == nil {
		wal.timeouts[vote.ValidatorID] = make(map[types.Round]TimeoutVote)
	}
	wal.timeouts[vote.ValidatorID][vote.Round] = vote
	return nil
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
	if wal.hasSafety {
		if err := checkSafetyProgress(wal.safety, state); err != nil {
			return err
		}
	}
	if err := wal.appendLocked(WALEvent{
		Type:   WALEventSafetyState,
		Time:   time.Now().UTC(),
		Height: state.LastVotedHeight,
		Round:  state.LastVotedRound,
		Safety: &state,
	}); err != nil {
		return err
	}
	wal.safety = state
	wal.hasSafety = true
	return nil
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

// Replay returns every valid record in the log. A torn tail is ignored; a corrupt record
// followed by valid records is an error.
func (wal *WAL) Replay() ([]WALEvent, error) {
	file, err := os.Open(wal.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scan, err := scanWAL(file)
	if err != nil {
		return nil, err
	}
	return scan.events, nil
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

func (wal *WAL) append(event WALEvent) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	return wal.appendLocked(event)
}

func (wal *WAL) appendLocked(event WALEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	record := fmt.Appendf(nil, "%08x %s\n", crc32.ChecksumIEEE(payload), payload)
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
	return nil
}

func (wal *WAL) ensureOpen() error {
	if wal.closed || wal.file == nil {
		return ErrWALClosed
	}
	return nil
}

type walScanResult struct {
	events     []WALEvent
	validBytes int64
}

// scanWAL reads checksummed records until the first damaged one. Damage confined to the
// tail is reported through validBytes; a valid record after damage is corruption.
func scanWAL(reader io.Reader) (walScanResult, error) {
	var result walScanResult
	buffered := bufio.NewReader(reader)
	var offset int64
	damagedAt := int64(-1)
	for {
		line, err := buffered.ReadBytes('\n')
		if len(line) == 0 && errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return walScanResult{}, err
		}
		start := offset
		offset += int64(len(line))
		if event, ok := decodeWALRecord(line); ok {
			if damagedAt >= 0 {
				return walScanResult{}, fmt.Errorf("%w: valid record at offset %d follows damaged record at offset %d", ErrWALCorrupt, start, damagedAt)
			}
			result.events = append(result.events, event)
			result.validBytes = offset
		} else if damagedAt < 0 {
			damagedAt = start
		}
		if err != nil {
			return result, nil
		}
	}
}

func decodeWALRecord(line []byte) (WALEvent, bool) {
	const checksumHexLength = 8
	if len(line) < checksumHexLength+3 || line[len(line)-1] != '\n' || line[checksumHexLength] != ' ' {
		return WALEvent{}, false
	}
	checksum, err := hex.DecodeString(string(line[:checksumHexLength]))
	if err != nil {
		return WALEvent{}, false
	}
	payload := line[checksumHexLength+1 : len(line)-1]
	if binary.BigEndian.Uint32(checksum) != crc32.ChecksumIEEE(payload) {
		return WALEvent{}, false
	}
	var event WALEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return WALEvent{}, false
	}
	return event, true
}

func syncDirectory(path string) {
	directory, err := os.Open(path)
	if err != nil {
		return
	}
	_ = directory.Sync()
	_ = directory.Close()
}

func hashString(hash types.Hash) string {
	return hex.EncodeToString(hash[:])
}

func quorumCertFingerprint(qc finality.QuorumCert) string {
	data, _ := json.Marshal(struct {
		Height    types.Height `json:"height"`
		Round     types.Round  `json:"round"`
		BlockHash string       `json:"block_hash"`
	}{Height: qc.Height, Round: qc.Round, BlockHash: hashString(qc.BlockHash)})
	return hex.EncodeToString(data)
}

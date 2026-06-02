package consensus

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
)

type WALEventType string

const (
	WALEventProposal    WALEventType = "proposal"
	WALEventVote        WALEventType = "vote"
	WALEventTimeoutVote WALEventType = "timeout_vote"
)

type WALEvent struct {
	Type        WALEventType      `json:"type"`
	Time        time.Time         `json:"time"`
	ValidatorID types.ValidatorID `json:"validator_id,omitempty"`
	Height      types.Height      `json:"height,omitempty"`
	Round       types.Round       `json:"round,omitempty"`
	BlockHash   string            `json:"block_hash,omitempty"`
	HighQC      string            `json:"high_qc,omitempty"`
}

type WAL struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	votes    map[types.ValidatorID]map[voteKey]string
	timeouts map[types.ValidatorID]map[timeoutKey]string
	closed   bool
}

type voteKey struct {
	Height types.Height `json:"height"`
	Round  types.Round  `json:"round"`
}

type timeoutKey struct {
	Height types.Height `json:"height"`
	Round  types.Round  `json:"round"`
}

func OpenWAL(path string) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	wal := &WAL{
		path:     path,
		votes:    make(map[types.ValidatorID]map[voteKey]string),
		timeouts: make(map[types.ValidatorID]map[timeoutKey]string),
	}
	if err := wal.replay(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	wal.file = file
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

func (wal *WAL) RecordVote(vote Vote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	blockHash := hashString(vote.BlockHash)
	key := voteKey{Height: vote.Height, Round: vote.Round}
	if wal.votes[vote.ValidatorID] == nil {
		wal.votes[vote.ValidatorID] = make(map[voteKey]string)
	}
	if previous, ok := wal.votes[vote.ValidatorID][key]; ok && previous != blockHash {
		return fmt.Errorf("%w: validator=%s height=%d round=%d previous=%s next=%s", ErrDoubleSignDetected, vote.ValidatorID, vote.Height, vote.Round, previous, blockHash)
	}
	wal.votes[vote.ValidatorID][key] = blockHash
	return wal.appendLocked(WALEvent{
		Type:        WALEventVote,
		Time:        time.Now().UTC(),
		ValidatorID: vote.ValidatorID,
		Height:      vote.Height,
		Round:       vote.Round,
		BlockHash:   blockHash,
	})
}

func (wal *WAL) RecordTimeoutVote(vote TimeoutVote) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()
	if err := wal.ensureOpen(); err != nil {
		return err
	}
	highQC := quorumCertFingerprint(vote.HighQC)
	key := timeoutKey{Height: vote.Height, Round: vote.Round}
	if wal.timeouts[vote.ValidatorID] == nil {
		wal.timeouts[vote.ValidatorID] = make(map[timeoutKey]string)
	}
	if previous, ok := wal.timeouts[vote.ValidatorID][key]; ok && previous != highQC {
		return fmt.Errorf("%w: timeout validator=%s height=%d round=%d", ErrDoubleSignDetected, vote.ValidatorID, vote.Height, vote.Round)
	}
	wal.timeouts[vote.ValidatorID][key] = highQC
	return wal.appendLocked(WALEvent{
		Type:        WALEventTimeoutVote,
		Time:        time.Now().UTC(),
		ValidatorID: vote.ValidatorID,
		Height:      vote.Height,
		Round:       vote.Round,
		HighQC:      highQC,
	})
}

func (wal *WAL) Replay() ([]WALEvent, error) {
	file, err := os.Open(wal.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	events := make([]WALEvent, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event WALEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

func (wal *WAL) replay() error {
	events, err := wal.Replay()
	if err != nil {
		return err
	}
	for _, event := range events {
		wal.apply(event)
	}
	return nil
}

func (wal *WAL) apply(event WALEvent) {
	switch event.Type {
	case WALEventVote:
		key := voteKey{Height: event.Height, Round: event.Round}
		if wal.votes[event.ValidatorID] == nil {
			wal.votes[event.ValidatorID] = make(map[voteKey]string)
		}
		wal.votes[event.ValidatorID][key] = event.BlockHash
	case WALEventTimeoutVote:
		key := timeoutKey{Height: event.Height, Round: event.Round}
		if wal.timeouts[event.ValidatorID] == nil {
			wal.timeouts[event.ValidatorID] = make(map[timeoutKey]string)
		}
		wal.timeouts[event.ValidatorID][key] = event.HighQC
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
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := wal.file.Write(append(data, '\n')); err != nil {
		return err
	}
	return wal.file.Sync()
}

func (wal *WAL) ensureOpen() error {
	if wal.closed || wal.file == nil {
		return ErrWALClosed
	}
	return nil
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

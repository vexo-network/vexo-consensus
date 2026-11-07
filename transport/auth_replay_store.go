package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

const (
	// maxAuthReplayNodeIDBytes and maxAuthReplayNonceBytes bound every field of
	// a replay record so a single record can never exceed maxAuthReplayLineBytes.
	maxAuthReplayNodeIDBytes = p2p.MaxPeerIDBytes
	maxAuthReplayNonceBytes  = 128
	maxAuthReplayLineBytes   = 1024

	// maxAuthReplayTTL bounds how far in the future a record may expire.
	maxAuthReplayTTL = time.Hour

	// maxAuthReplayEntries and maxAuthReplayEntriesPerNode cap the live set so
	// the store, its file, and the compaction cost stay bounded even when a
	// peer floods handshakes with fresh nonces.
	maxAuthReplayEntries        = 65536
	maxAuthReplayEntriesPerNode = 1024

	// authReplayCompactEvery is the number of appended records after which the
	// file is rewritten from the live set, so expired records cannot
	// accumulate on disk.
	authReplayCompactEvery = 1024

	// authReplayPruneInterval is how often expired records are swept when no
	// quota is under pressure.
	authReplayPruneInterval = 30 * time.Second
)

var (
	ErrAuthReplayRecord = errors.New("p2p auth replay record is invalid")
	ErrAuthReplayFull   = errors.New("p2p auth replay store is full")
)

type authReplayKey struct {
	nodeID p2p.PeerID
	nonce  string
}

// replayWindow is the bounded set of live (node, nonce) pairs shared by the
// durable file store and the in-memory fallback.
type replayWindow struct {
	seen      map[authReplayKey]time.Time
	perNode   map[p2p.PeerID]int
	lastPrune time.Time
}

func newReplayWindow() replayWindow {
	return replayWindow{
		seen:    make(map[authReplayKey]time.Time),
		perNode: make(map[p2p.PeerID]int),
	}
}

func validateAuthReplayRecord(nodeID p2p.PeerID, nonce string, expires time.Time, now time.Time) error {
	if nodeID == "" || len(nodeID) > maxAuthReplayNodeIDBytes {
		return ErrAuthReplayRecord
	}
	if nonce == "" || len(nonce) > maxAuthReplayNonceBytes {
		return ErrAuthReplayRecord
	}
	if expires.IsZero() || !expires.After(now) || expires.Sub(now) > maxAuthReplayTTL {
		return ErrAuthReplayRecord
	}
	return nil
}

func (window *replayWindow) prune(now time.Time) {
	for key, expires := range window.seen {
		if !expires.After(now) {
			window.remove(key)
		}
	}
	window.lastPrune = now
}

func (window *replayWindow) remove(key authReplayKey) {
	delete(window.seen, key)
	if window.perNode[key.nodeID] <= 1 {
		delete(window.perNode, key.nodeID)
		return
	}
	window.perNode[key.nodeID]--
}

// admit reserves key until expires. It reports ErrAuthNonceReplay for a live
// duplicate and ErrAuthReplayFull when the identity's quota is exhausted; a
// full window makes room by dropping the record that expires first.
func (window *replayWindow) admit(key authReplayKey, expires time.Time, now time.Time) error {
	if previous, found := window.seen[key]; found {
		if previous.After(now) {
			return ErrAuthNonceReplay
		}
		window.remove(key)
	}
	if now.Sub(window.lastPrune) >= authReplayPruneInterval {
		window.prune(now)
	}
	if window.perNode[key.nodeID] >= maxAuthReplayEntriesPerNode {
		window.prune(now)
		if window.perNode[key.nodeID] >= maxAuthReplayEntriesPerNode {
			return ErrAuthReplayFull
		}
	}
	if len(window.seen) >= maxAuthReplayEntries {
		window.prune(now)
		if len(window.seen) >= maxAuthReplayEntries {
			window.evictEarliest()
		}
	}
	window.insert(key, expires)
	return nil
}

// evictEarliest drops the record that expires first. A full window must not refuse new handshakes:
// any holder of the shared secret can fill it with identities of its own, and a refusal would lock
// every honest peer out. Every proof is bound to a one-shot challenge this node chose, so the window
// is defense in depth and losing its oldest record weakens nothing a challenge does not cover.
func (window *replayWindow) evictEarliest() {
	var earliest authReplayKey
	var earliestExpires time.Time
	found := false
	for key, expires := range window.seen {
		if !found || expires.Before(earliestExpires) || expires.Equal(earliestExpires) && (key.nodeID < earliest.nodeID || key.nodeID == earliest.nodeID && key.nonce < earliest.nonce) {
			earliest, earliestExpires, found = key, expires, true
		}
	}
	if found {
		window.remove(earliest)
	}
}

func (window *replayWindow) insert(key authReplayKey, expires time.Time) {
	if _, found := window.seen[key]; !found {
		window.perNode[key.nodeID]++
	}
	window.seen[key] = expires
}

// retainLatest keeps at most limit records, preferring the ones that expire
// last because they protect the longest replay window.
func (window *replayWindow) retainLatest(limit int) {
	if len(window.seen) <= limit {
		return
	}
	keys := make([]authReplayKey, 0, len(window.seen))
	for key := range window.seen {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool {
		leftExpires, rightExpires := window.seen[keys[left]], window.seen[keys[right]]
		if !leftExpires.Equal(rightExpires) {
			return leftExpires.After(rightExpires)
		}
		if keys[left].nodeID != keys[right].nodeID {
			return keys[left].nodeID < keys[right].nodeID
		}
		return keys[left].nonce < keys[right].nonce
	})
	for _, key := range keys[limit:] {
		window.remove(key)
	}
}

// memoryAuthReplayStore is the bounded in-process replay window used when no
// durable store is configured.
type memoryAuthReplayStore struct {
	mu     sync.Mutex
	window replayWindow
}

func newMemoryAuthReplayStore() *memoryAuthReplayStore {
	return &memoryAuthReplayStore{window: newReplayWindow()}
}

func (store *memoryAuthReplayStore) MarkAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string, expires time.Time, now time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := validateAuthReplayRecord(nodeID, nonce, expires, now); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.window.admit(authReplayKey{nodeID: nodeID, nonce: nonce}, expires, now)
}

type FileAuthReplayStore struct {
	path     string
	mu       sync.Mutex
	window   replayWindow
	appended int
}

type authReplayRecord struct {
	NodeID    string `json:"node_id"`
	Nonce     string `json:"nonce"`
	ExpiresAt int64  `json:"expires_at_unix_nano"`
}

func NewFileAuthReplayStore(path string) (*FileAuthReplayStore, error) {
	if path == "" {
		return nil, ErrAuthReplayStore
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	store := &FileAuthReplayStore{
		path:   path,
		window: newReplayWindow(),
	}
	now := time.Now()
	if err := store.load(now); err != nil {
		return nil, err
	}
	if err := store.Compact(context.Background(), now); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *FileAuthReplayStore) MarkAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string, expires time.Time, now time.Time) error {
	if store == nil {
		return ErrAuthReplayStore
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := validateAuthReplayRecord(nodeID, nonce, expires, now); err != nil {
		return err
	}
	key := authReplayKey{nodeID: nodeID, nonce: nonce}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.window.admit(key, expires, now); err != nil {
		return err
	}
	if err := store.appendLocked(key, expires); err != nil {
		store.window.remove(key)
		return err
	}
	store.appended++
	if store.appended >= authReplayCompactEvery {
		store.window.prune(now)
		// The record above is already durable, so a failed compaction only
		// delays the cleanup to the next mark.
		_ = store.compactLocked()
	}
	return nil
}

func (store *FileAuthReplayStore) appendLocked(key authReplayKey, expires time.Time) error {
	file, err := os.OpenFile(store.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encodeErr := json.NewEncoder(file).Encode(authReplayRecord{
		NodeID:    string(key.nodeID),
		Nonce:     key.nonce,
		ExpiresAt: expires.UnixNano(),
	})
	syncErr := file.Sync()
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func (store *FileAuthReplayStore) Compact(ctx context.Context, now time.Time) error {
	if store == nil {
		return ErrAuthReplayStore
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.window.prune(now)
	return store.compactLocked()
}

// load restores the live replay window. A replay file is append-only and may
// contain torn, corrupt, or hostile lines (for example after a crash or from
// an older, unbounded writer); those are skipped instead of preventing startup
// or being allowed to exhaust memory.
func (store *FileAuthReplayStore) load(now time.Time) error {
	file, err := os.Open(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, maxAuthReplayLineBytes)
	for {
		line, tooLong, readErr := readBoundedLine(reader, maxAuthReplayLineBytes)
		if len(line) > 0 && !tooLong {
			store.loadRecord(line, now)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return readErr
		}
	}
	store.window.retainLatest(maxAuthReplayEntries)
	return nil
}

func (store *FileAuthReplayStore) loadRecord(line []byte, now time.Time) {
	var record authReplayRecord
	if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
		return
	}
	nodeID := p2p.PeerID(record.NodeID)
	expires := time.Unix(0, record.ExpiresAt)
	if err := validateAuthReplayRecord(nodeID, record.Nonce, expires, now); err != nil {
		return
	}
	key := authReplayKey{nodeID: nodeID, nonce: record.Nonce}
	if _, found := store.window.seen[key]; !found && store.window.perNode[nodeID] >= maxAuthReplayEntriesPerNode {
		return
	}
	store.window.insert(key, expires)
}

// readBoundedLine reads one newline-terminated line of at most limit bytes.
// Longer lines are consumed to their newline and reported as tooLong so the
// caller never buffers attacker-sized input.
func readBoundedLine(reader *bufio.Reader, limit int) (line []byte, tooLong bool, err error) {
	for {
		chunk, readErr := reader.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > limit {
				tooLong = true
				line = nil
			} else {
				line = append(line, chunk...)
			}
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		return line, tooLong, readErr
	}
}

func (store *FileAuthReplayStore) compactLocked() error {
	dir := filepath.Dir(store.path)
	temp, err := os.CreateTemp(dir, ".p2p-auth-replay-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	encoder := json.NewEncoder(temp)
	for key, expires := range store.window.seen {
		if err := encoder.Encode(authReplayRecord{
			NodeID:    string(key.nodeID),
			Nonce:     key.nonce,
			ExpiresAt: expires.UnixNano(),
		}); err != nil {
			_ = temp.Close()
			_ = os.Remove(tempPath)
			return err
		}
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, store.path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	store.appended = 0
	return nil
}

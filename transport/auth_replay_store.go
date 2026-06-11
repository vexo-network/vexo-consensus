package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

type FileAuthReplayStore struct {
	path string
	mu   sync.Mutex
	seen map[string]time.Time
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
		path: path,
		seen: make(map[string]time.Time),
	}
	if err := store.load(time.Now()); err != nil {
		return nil, err
	}
	if err := store.Compact(context.Background(), time.Now()); err != nil {
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
	if nodeID == "" || nonce == "" || expires.IsZero() || !expires.After(now) {
		return ErrHandshakeSignature
	}
	key := authReplayKey(nodeID, nonce)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(now)
	if _, found := store.seen[key]; found {
		return ErrAuthNonceReplay
	}
	file, err := os.OpenFile(store.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encodeErr := encoder.Encode(authReplayRecord{
		NodeID:    string(nodeID),
		Nonce:     nonce,
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
	if closeErr != nil {
		return closeErr
	}
	store.seen[key] = expires
	if len(store.seen)%1024 == 0 {
		return store.compactLocked()
	}
	return nil
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
	store.pruneLocked(now)
	return store.compactLocked()
}

func (store *FileAuthReplayStore) load(now time.Time) error {
	file, err := os.Open(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record authReplayRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return err
		}
		expires := time.Unix(0, record.ExpiresAt)
		if record.NodeID == "" || record.Nonce == "" || !expires.After(now) {
			continue
		}
		store.seen[authReplayKey(p2p.PeerID(record.NodeID), record.Nonce)] = expires
	}
	return scanner.Err()
}

func (store *FileAuthReplayStore) pruneLocked(now time.Time) {
	for key, expires := range store.seen {
		if !expires.After(now) {
			delete(store.seen, key)
		}
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
	for key, expires := range store.seen {
		nodeID, nonce, ok := splitAuthReplayKey(key)
		if !ok {
			continue
		}
		if err := encoder.Encode(authReplayRecord{
			NodeID:    string(nodeID),
			Nonce:     nonce,
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
	return os.Rename(tempPath, store.path)
}

func authReplayKey(nodeID p2p.PeerID, nonce string) string {
	return string(nodeID) + ":" + nonce
}

func splitAuthReplayKey(key string) (p2p.PeerID, string, bool) {
	nodeID, nonce, found := strings.Cut(key, ":")
	if !found || nodeID == "" || nonce == "" {
		return "", "", false
	}
	return p2p.PeerID(nodeID), nonce, true
}

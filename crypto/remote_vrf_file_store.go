package crypto

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileRemoteVRFReplayStore struct {
	path string
	mu   sync.Mutex
}

type remoteVRFReplayRecord struct {
	SchemaVersion string `json:"schema_version"`
	Domain        string `json:"domain"`
	Nonce         string `json:"nonce"`
	ExpiresAt     int64  `json:"expires_at_unix_nano"`
	RecordedAt    int64  `json:"recorded_at_unix_nano"`
}

func NewFileRemoteVRFReplayStore(path string) (*FileRemoteVRFReplayStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrRemoteVRFReplayStore
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	store := &FileRemoteVRFReplayStore{path: path}
	if _, err := store.load(time.Now().UTC()); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *FileRemoteVRFReplayStore) MarkNonce(domain string, nonce string, expires time.Time, now time.Time) error {
	if strings.TrimSpace(domain) == "" || strings.TrimSpace(nonce) == "" {
		return ErrRemoteVRFInvalidChallenge
	}
	if !expires.After(now) {
		return ErrRemoteVRFInvalidChallenge
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	unlock, err := store.acquireLock(now)
	if err != nil {
		return err
	}
	defer unlock()
	records, err := store.load(now)
	if err != nil {
		return err
	}
	key := remoteVRFReplayKey(domain, nonce)
	if _, found := records[key]; found {
		return ErrRemoteVRFDuplicateNonce
	}
	records[key] = remoteVRFReplayRecord{
		SchemaVersion: KeyDocumentVersionV1,
		Domain:        domain,
		Nonce:         nonce,
		ExpiresAt:     expires.UTC().UnixNano(),
		RecordedAt:    now.UTC().UnixNano(),
	}
	return store.save(records)
}

func (store *FileRemoteVRFReplayStore) load(now time.Time) (map[string]remoteVRFReplayRecord, error) {
	records := make(map[string]remoteVRFReplayRecord)
	file, err := os.Open(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return records, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record remoteVRFReplayRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("%w: corrupt replay record line %d: %v", ErrRemoteVRFReplayStore, lineNumber, err)
		}
		if record.SchemaVersion != KeyDocumentVersionV1 || record.Domain == "" || record.Nonce == "" || record.ExpiresAt <= 0 {
			return nil, fmt.Errorf("%w: invalid replay record line %d", ErrRemoteVRFReplayStore, lineNumber)
		}
		if time.Unix(0, record.ExpiresAt).After(now) {
			records[remoteVRFReplayKey(record.Domain, record.Nonce)] = record
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	return records, nil
}

func (store *FileRemoteVRFReplayStore) save(records map[string]remoteVRFReplayRecord) error {
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	tempPath := store.path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	encoder := json.NewEncoder(file)
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		record := records[key]
		if err := encoder.Encode(record); err != nil {
			_ = file.Close()
			return fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	if err := os.Rename(tempPath, store.path); err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	if dirFile, err := os.Open(filepath.Dir(store.path)); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

func remoteVRFReplayKey(domain string, nonce string) string {
	return domain + "/" + nonce
}

func (store *FileRemoteVRFReplayStore) acquireLock(now time.Time) (func(), error) {
	lockPath := store.path + ".lock"
	deadline := time.Now().Add(2 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = file.WriteString(now.UTC().Format(time.RFC3339Nano) + "\n")
			_ = file.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w: replay store lock timeout", ErrRemoteVRFReplayStore)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func NewRemoteVRFFileAuditSink(path string) (func(RemoteVRFAuditEvent), error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrRemoteVRFReplayStore
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRemoteVRFReplayStore, err)
	}
	var mu sync.Mutex
	return func(event RemoteVRFAuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		if event.At.IsZero() {
			event.At = time.Now().UTC()
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		_ = json.NewEncoder(file).Encode(event)
		_ = file.Sync()
		_ = file.Close()
	}, nil
}

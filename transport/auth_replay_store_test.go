package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestAuthReplayStoresRejectOversizedAndMalformedRecords(t *testing.T) {
	fileStore, err := NewFileAuthReplayStore(filepath.Join(t.TempDir(), "replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	stores := map[string]AuthReplayStore{"file": fileStore, "memory": newMemoryAuthReplayStore()}
	now := time.Now()
	soon := now.Add(time.Minute)
	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			cases := map[string]struct {
				nodeID  p2p.PeerID
				nonce   string
				expires time.Time
			}{
				"oversized node id": {p2p.PeerID(strings.Repeat("n", maxAuthReplayNodeIDBytes+1)), "nonce", soon},
				"oversized nonce":   {"alice", strings.Repeat("x", maxAuthReplayNonceBytes+1), soon},
				"empty node id":     {"", "nonce", soon},
				"empty nonce":       {"alice", "", soon},
				"already expired":   {"alice", "nonce", now.Add(-time.Second)},
				"expiry too far":    {"alice", "nonce", now.Add(maxAuthReplayTTL + time.Second)},
				"no expiry":         {"alice", "nonce", time.Time{}},
			}
			for label, record := range cases {
				if err := store.MarkAuthNonce(context.Background(), record.nodeID, record.nonce, record.expires, now); !errors.Is(err, ErrAuthReplayRecord) {
					t.Fatalf("%s: expected the record to be rejected, got %v", label, err)
				}
			}
			if err := store.MarkAuthNonce(context.Background(), "alice", strings.Repeat("x", maxAuthReplayNonceBytes), soon, now); err != nil {
				t.Fatalf("a record at the size limit must be accepted, got %v", err)
			}
		})
	}
}

func TestAuthReplayStoreDetectsReplayUntilExpiry(t *testing.T) {
	store := newMemoryAuthReplayStore()
	now := time.Now()
	if err := store.MarkAuthNonce(context.Background(), "alice", "n1", now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAuthNonce(context.Background(), "alice", "n1", now.Add(time.Minute), now.Add(time.Second)); !errors.Is(err, ErrAuthNonceReplay) {
		t.Fatalf("expected the replay to be detected, got %v", err)
	}
	if err := store.MarkAuthNonce(context.Background(), "bob", "n1", now.Add(time.Minute), now); err != nil {
		t.Fatalf("the same nonce from another node is not a replay, got %v", err)
	}
	later := now.Add(2 * time.Minute)
	if err := store.MarkAuthNonce(context.Background(), "alice", "n1", later.Add(time.Minute), later); err != nil {
		t.Fatalf("an expired record must not block a later nonce reuse, got %v", err)
	}
}

func TestAuthReplayStoreBoundsEntriesPerNodeAndOverall(t *testing.T) {
	store := newMemoryAuthReplayStore()
	now := time.Now()
	expires := now.Add(time.Minute)
	for index := 0; index < maxAuthReplayEntriesPerNode; index++ {
		if err := store.MarkAuthNonce(context.Background(), "flooder", fmt.Sprintf("n%d", index), expires, now); err != nil {
			t.Fatalf("entry %d: %v", index, err)
		}
	}
	if err := store.MarkAuthNonce(context.Background(), "flooder", "one-too-many", expires, now); !errors.Is(err, ErrAuthReplayFull) {
		t.Fatalf("expected a single node to be capped, got %v", err)
	}
	if err := store.MarkAuthNonce(context.Background(), "honest", "n", expires, now); err != nil {
		t.Fatalf("a flooding node must not lock out other nodes, got %v", err)
	}
	later := now.Add(2 * time.Minute)
	if err := store.MarkAuthNonce(context.Background(), "flooder", "after-expiry", later.Add(time.Minute), later); err != nil {
		t.Fatalf("expected expired entries to free the quota, got %v", err)
	}

	// Flooders fill the whole window with identities of their own. A full window must not lock an
	// honest peer out: it drops the record that expires first and stays within its bound.
	global := newMemoryAuthReplayStore()
	for node := 0; len(global.window.seen) < maxAuthReplayEntries; node++ {
		for index := 0; index < maxAuthReplayEntriesPerNode && len(global.window.seen) < maxAuthReplayEntries; index++ {
			if err := global.MarkAuthNonce(context.Background(), p2p.PeerID(fmt.Sprintf("node-%d", node)), fmt.Sprintf("n%d", index), expires, now); err != nil {
				t.Fatalf("node %d entry %d: %v", node, index, err)
			}
		}
	}
	if err := global.MarkAuthNonce(context.Background(), "honest", "n", expires.Add(time.Second), now); err != nil {
		t.Fatalf("a full window must not refuse an honest peer, got %v", err)
	}
	if len(global.window.seen) != maxAuthReplayEntries {
		t.Fatalf("store holds %d entries, want the %d limit", len(global.window.seen), maxAuthReplayEntries)
	}
	if err := global.MarkAuthNonce(context.Background(), "honest", "n", expires.Add(time.Second), now); !errors.Is(err, ErrAuthNonceReplay) {
		t.Fatalf("the honest record must be the one kept, got %v", err)
	}
}

func TestFileAuthReplayStoreLoadsPastCorruptAndHostileLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	now := time.Now()
	record := func(nodeID string, nonce string, expires time.Time) string {
		return fmt.Sprintf(`{"node_id":%q,"nonce":%q,"expires_at_unix_nano":%d}`, nodeID, nonce, expires.UnixNano())
	}
	var contents bytes.Buffer
	for _, line := range []string{
		"this is not json",
		record("alice", "valid", now.Add(time.Minute)),
		record(strings.Repeat("a", maxAuthReplayNodeIDBytes+1), "n", now.Add(time.Minute)),
		record("bob", strings.Repeat("x", maxAuthReplayNonceBytes+1), now.Add(time.Minute)),
		strings.Repeat("z", 4*1024*1024),
		record("carol", "expired", now.Add(-time.Minute)),
		record("dave", "too-far", now.Add(48*time.Hour)),
		record("erin", "after-oversized-line", now.Add(time.Minute)),
		`{"node_id":"fra`,
	} {
		contents.WriteString(line)
		contents.WriteByte('\n')
	}
	contents.WriteString(`{"node_id":"torn-last-line","nonce":"n","expires_at_unix_nano":`)
	if err := os.WriteFile(path, contents.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatalf("a corrupt replay file must not prevent startup, got %v", err)
	}
	for _, kept := range []struct {
		nodeID p2p.PeerID
		nonce  string
	}{{"alice", "valid"}, {"erin", "after-oversized-line"}} {
		if err := store.MarkAuthNonce(context.Background(), kept.nodeID, kept.nonce, now.Add(time.Minute), now); !errors.Is(err, ErrAuthNonceReplay) {
			t.Fatalf("expected %s/%s to survive the load as a known nonce, got %v", kept.nodeID, kept.nonce, err)
		}
	}
	if err := store.MarkAuthNonce(context.Background(), "carol", "expired", now.Add(time.Minute), now); err != nil {
		t.Fatalf("expected an expired record to be dropped on load, got %v", err)
	}

	compacted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(compacted) > 4096 {
		t.Fatalf("expected the hostile lines to be compacted away, file is %d bytes", len(compacted))
	}
	for _, junk := range []string{"not json", "torn-last-line", "too-far", "fra"} {
		if strings.Contains(string(compacted), junk) {
			t.Fatalf("expected %q to be compacted away: %s", junk, compacted)
		}
	}
}

func TestFileAuthReplayStoreSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	now := time.Now()
	first, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.MarkAuthNonce(context.Background(), "alice", "n1", now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	second, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.MarkAuthNonce(context.Background(), "alice", "n1", now.Add(time.Minute), now); !errors.Is(err, ErrAuthNonceReplay) {
		t.Fatalf("expected the replay to be detected after restart, got %v", err)
	}
}

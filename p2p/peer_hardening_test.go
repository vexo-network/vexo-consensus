package p2p

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestNodeIDFromPublicKeyIsDeterministicAndKeyBound(t *testing.T) {
	first, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	firstID := NodeIDFromPublicKey(types.PublicKey(first))
	if firstID != NodeIDFromPublicKey(types.PublicKey(first)) {
		t.Fatal("node id derivation must be deterministic")
	}
	if firstID == NodeIDFromPublicKey(types.PublicKey(second)) {
		t.Fatal("different keys must derive different node ids")
	}
	if !IsDerivedNodeID(firstID) || !ValidPeerID(firstID) {
		t.Fatalf("derived id %q must be a valid, recognisable node id", firstID)
	}
	for _, id := range []PeerID{"validator-1", "", PeerID(strings.ToUpper(string(firstID))), firstID + "0", firstID[:len(firstID)-1], PeerID(strings.Repeat("g", 2*nodeIDBytes))} {
		if IsDerivedNodeID(id) {
			t.Fatalf("%q must not be recognised as a derived node id", id)
		}
	}
}

func TestValidPeerIDBoundsAndCharacters(t *testing.T) {
	valid := []PeerID{"alice", "validator-1", "archive-0123456789ab", PeerID(strings.Repeat("a", MaxPeerIDBytes))}
	for _, id := range valid {
		if !ValidPeerID(id) {
			t.Fatalf("expected %q to be valid", id)
		}
	}
	invalid := []PeerID{"", "has space", "tab\there", "new\nline", "nul\x00", "del\x7f", "\xc3\xa9", PeerID(strings.Repeat("a", MaxPeerIDBytes+1))}
	for _, id := range invalid {
		if ValidPeerID(id) {
			t.Fatalf("expected %q to be invalid", id)
		}
	}
}

func TestAddrBookLearnedEntriesNeverClearBansOrRewriteAddresses(t *testing.T) {
	book, err := OpenAddrBookWithPolicy("", 1)
	if err != nil {
		t.Fatal(err)
	}
	book.Add("mallory", "10.0.0.66:26656", "handshake", false)
	book.MarkFailure("mallory", time.Hour)
	if !book.IsBanned("mallory") {
		t.Fatal("expected mallory to be banned after the failure")
	}

	// Re-learning a banned identity, even with a different address, is how an
	// attacker would try to launder a ban.
	book.Add("mallory", "10.0.0.67:26656", "handshake", false)
	if !book.IsBanned("mallory") {
		t.Fatal("a learned re-add must not clear a ban")
	}
	if got := book.peers["mallory"].Address; got != "10.0.0.66:26656" {
		t.Fatalf("a learned re-add must not rewrite the address, got %s", got)
	}
	if peers := book.PeerMap(""); len(peers) != 0 {
		t.Fatalf("a banned peer must not be offered for dialing: %+v", peers)
	}

	// Operator configuration is authoritative for the address but still does
	// not erase the failure history of the identity.
	book.Add("mallory", "10.0.0.68:26656", "cli-peer", true)
	if !book.IsBanned("mallory") || book.peers["mallory"].Address != "10.0.0.68:26656" || !book.peers["mallory"].Permanent {
		t.Fatalf("unexpected state after operator re-add: %+v", book.peers["mallory"])
	}
}

func TestAddrBookLearnedEntriesNeverDisplacePermanentPeers(t *testing.T) {
	book, err := OpenAddrBookWithPolicy("", 3)
	if err != nil {
		t.Fatal(err)
	}
	book.maxLearned = 4
	for index := 0; index < 3; index++ {
		book.Add(PeerID(fmt.Sprintf("seed-%d", index)), fmt.Sprintf("10.0.0.%d:26656", index+1), "cli-seed", true)
	}
	book.Add("seed-0", "10.9.9.9:26656", "handshake", false)
	for index := 0; index < 100; index++ {
		book.Add(PeerID(fmt.Sprintf("flood-%d", index)), fmt.Sprintf("10.1.0.%d:26656", index+1), "handshake", false)
	}

	permanent := book.PermanentPeerMap("")
	if len(permanent) != 3 || permanent["seed-0"] != "10.0.0.1:26656" {
		t.Fatalf("learned flood displaced or rewrote a permanent peer: %+v", permanent)
	}
	if learned := book.LearnedPeerMap(""); len(learned) != book.maxLearned {
		t.Fatalf("expected the learned bucket to hold %d entries, got %d", book.maxLearned, len(learned))
	}
}

func TestAddrBookFloodCannotEvictBannedEntries(t *testing.T) {
	book, err := OpenAddrBookWithPolicy("", 1)
	if err != nil {
		t.Fatal(err)
	}
	book.maxLearned = 3
	book.Add("bad", "10.0.0.66:26656", "handshake", false)
	book.MarkFailure("bad", time.Hour)
	for index := 0; index < 50; index++ {
		book.Add(PeerID(fmt.Sprintf("fresh-%d", index)), fmt.Sprintf("10.2.0.%d:26656", index+1), "handshake", false)
	}
	if !book.IsBanned("bad") {
		t.Fatal("flooding fresh identities must not evict a ban")
	}
}

func TestAddrBookLoadBoundsAndSanitizesPersistedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "addrbook.json")
	now := time.Now().UTC()
	document := AddrBookDocument{SchemaVersion: AddrBookVersionV1}
	document.Peers = append(document.Peers,
		AddrBookPeer{ID: "seed", Address: "10.0.0.1:26656", Permanent: true},
		AddrBookPeer{ID: "bad id", Address: "10.0.0.2:26656"},
		AddrBookPeer{ID: PeerID(strings.Repeat("a", MaxPeerIDBytes+1)), Address: "10.0.0.3:26656"},
		AddrBookPeer{ID: "no-address", Address: "not-an-address"},
		AddrBookPeer{ID: "banned", Address: "10.0.0.4:26656", Banned: true, BannedUntil: now.Add(time.Hour).Format(time.RFC3339Nano)},
	)
	for index := 0; index < 20; index++ {
		document.Peers = append(document.Peers, AddrBookPeer{
			ID:       PeerID(fmt.Sprintf("learned-%02d", index)),
			Address:  fmt.Sprintf("10.5.0.%d:26656", index+1),
			LastSeen: now.Add(time.Duration(index) * time.Second).Format(time.RFC3339Nano),
		})
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	book := &AddrBook{path: path, maxFailures: 3, maxLearned: 5, peers: make(map[PeerID]AddrBookPeer), now: time.Now}
	if err := book.Load(); err != nil {
		t.Fatal(err)
	}
	if len(book.peers) != 1+5 {
		t.Fatalf("expected the permanent peer and 5 learned entries, got %d: %+v", len(book.peers), book.Peers())
	}
	if _, found := book.peers["banned"]; !found || !book.IsBanned("banned") {
		t.Fatal("the cap must keep bans before healthy entries")
	}
	for _, rejected := range []PeerID{"bad id", "no-address"} {
		if _, found := book.peers[rejected]; found {
			t.Fatalf("expected %q to be dropped on load", rejected)
		}
	}
	if _, found := book.peers["learned-19"]; !found {
		t.Fatal("expected the most recently seen entries to be kept")
	}
	if _, found := book.peers["learned-00"]; found {
		t.Fatal("expected the stalest entries to be dropped")
	}
}

func TestAddrBookMergeIsDeterministicWhenBucketIsFull(t *testing.T) {
	peers := make(map[PeerID]string)
	for index := 0; index < 30; index++ {
		peers[PeerID(fmt.Sprintf("peer-%02d", index))] = fmt.Sprintf("10.0.0.%d:26656", index+1)
	}
	var first map[PeerID]string
	for run := 0; run < 10; run++ {
		book, err := OpenAddrBook("")
		if err != nil {
			t.Fatal(err)
		}
		book.maxLearned = 8
		book.Merge(peers, "handshake", false)
		kept := book.LearnedPeerMap("")
		if first == nil {
			first = kept
			continue
		}
		if len(kept) != len(first) {
			t.Fatalf("run %d kept %d entries, first run kept %d", run, len(kept), len(first))
		}
		for peerID := range first {
			if _, found := kept[peerID]; !found {
				t.Fatalf("run %d kept a different set than the first run: %v vs %v", run, kept, first)
			}
		}
	}
}

func TestAddrBookSaveDeferredCoalescesWritesAndFlushPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "addrbook.json")
	book, err := OpenAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 50; index++ {
		book.Add(PeerID(fmt.Sprintf("peer-%d", index)), fmt.Sprintf("10.0.0.%d:26656", index+1), "handshake", false)
		book.SaveDeferred()
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deferred saves must not write synchronously, stat error: %v", err)
	}
	if err := book.Flush(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := OpenAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Peers()) != 50 {
		t.Fatalf("expected the flush to persist every entry, got %d", len(reloaded.Peers()))
	}
	book.mu.Lock()
	timerPending := book.saveTimer != nil
	book.mu.Unlock()
	if timerPending {
		t.Fatal("flush must cancel the pending deferred save")
	}
}

func TestScoreKeeperBoundsTrackedPeers(t *testing.T) {
	ctx := context.Background()
	keeper := NewScoreKeeper(ScoreConfig{InitialScore: 10, BanThreshold: 0, InvalidMessageCost: 20, MaxTrackedPeers: 8})
	if err := keeper.ObserveMessage(ctx, "offender", false); err == nil {
		t.Fatal("expected the offender to be banned")
	}
	for index := 0; index < 500; index++ {
		_ = keeper.ObserveMessage(ctx, PeerID(fmt.Sprintf("fresh-%d", index)), true)
	}
	snapshot, err := keeper.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) > 8 {
		t.Fatalf("score table grew to %d, over the bound", len(snapshot))
	}
	if banned, err := keeper.IsBanned(ctx, "offender"); err != nil || !banned {
		t.Fatalf("flooding fresh identities must not evict a ban, banned=%v err=%v", banned, err)
	}
	if banned, err := keeper.IsBanned(ctx, "never-seen"); err != nil || banned {
		t.Fatalf("an unknown peer is not banned and must not be tracked by the lookup, banned=%v err=%v", banned, err)
	}
	after, err := keeper.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(snapshot) {
		t.Fatal("IsBanned must not create entries for unknown peers")
	}
}

func TestScoreKeeperRestoreDropsInvalidIdentitiesAndKeepsBansWithinTheBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	until := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	document := ScoreDocument{SchemaVersion: ScoreKeeperVersionV1}
	document.Peers = append(document.Peers, PeerScoreRecord{Peer: "bad id", Score: 5}, PeerScoreRecord{Peer: "banned", Score: -5, Banned: true, BannedUntil: until})
	for index := 0; index < 20; index++ {
		document.Peers = append(document.Peers, PeerScoreRecord{Peer: PeerID(fmt.Sprintf("healthy-%02d", index)), Score: 10})
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.TrimSpace(data), 0o600); err != nil {
		t.Fatal(err)
	}
	keeper := NewScoreKeeper(ScoreConfig{InitialScore: 10, BanThreshold: 0, MaxTrackedPeers: 4})
	if err := keeper.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	snapshot, err := keeper.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 4 {
		t.Fatalf("expected the restored table to respect the bound, got %d", len(snapshot))
	}
	if banned, _ := keeper.IsBanned(context.Background(), "banned"); !banned {
		t.Fatal("the bound must keep persisted bans first")
	}
	for _, entry := range snapshot {
		if entry.Peer == "bad id" {
			t.Fatal("an invalid identity must not be restored")
		}
	}
}

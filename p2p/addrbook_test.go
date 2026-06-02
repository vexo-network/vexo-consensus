package p2p

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAddrBookPersistsPeers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "addrbook.json")
	book, err := OpenAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	book.now = func() time.Time { return now }
	book.Add("bob", "127.0.0.1:26666", "seed", true)
	book.Add("alice", "127.0.0.1:26656", "handshake", false)
	if err := book.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := OpenAddrBook(path)
	if err != nil {
		t.Fatal(err)
	}
	peers := loaded.Peers()
	if len(peers) != 2 || peers[0].ID != "alice" || peers[1].ID != "bob" {
		t.Fatalf("unexpected peer ordering: %+v", peers)
	}
	if !loaded.peers["bob"].Permanent || loaded.peers["bob"].LastSeen == "" {
		t.Fatalf("unexpected bob peer metadata: %+v", loaded.peers["bob"])
	}
}

func TestAddrBookPeerMapExcludesSelf(t *testing.T) {
	book, err := OpenAddrBook("")
	if err != nil {
		t.Fatal(err)
	}
	book.Add("alice", "127.0.0.1:26656", "self", true)
	book.Add("bob", "127.0.0.1:26666", "seed", true)
	peers := book.PeerMap("alice")
	if len(peers) != 1 || peers["bob"] != "127.0.0.1:26666" {
		t.Fatalf("unexpected peer map: %+v", peers)
	}
}

func TestAddrBookMarksFailureAndFiltersBannedPeers(t *testing.T) {
	book, err := OpenAddrBookWithPolicy("", 2)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	book.now = func() time.Time { return now }
	book.Add("bob", "127.0.0.1:26666", "seed", false)
	book.MarkAttempt("bob")
	book.MarkFailure("bob", time.Minute)
	book.MarkFailure("bob", time.Minute)
	if !book.peers["bob"].Banned || book.peers["bob"].Failures != 2 {
		t.Fatalf("expected bob banned with failures, got %+v", book.peers["bob"])
	}
	if len(book.PeerMap("")) != 0 {
		t.Fatalf("expected banned peer filtered, got %+v", book.PeerMap(""))
	}
	now = now.Add(time.Minute + time.Second)
	if book.PeerMap("")["bob"] != "127.0.0.1:26666" {
		t.Fatalf("expected expired ban peer restored, got %+v", book.PeerMap(""))
	}
}

func TestAddrBookEvictsOnlyNonPermanentBannedPeers(t *testing.T) {
	book, err := OpenAddrBookWithPolicy("", 1)
	if err != nil {
		t.Fatal(err)
	}
	book.Add("seed", "127.0.0.1:26656", "seed", true)
	book.Add("bad", "127.0.0.1:26666", "handshake", false)
	book.MarkFailure("seed", 0)
	book.MarkFailure("bad", 0)
	if evicted := book.EvictBanned(); evicted != 1 {
		t.Fatalf("expected one evicted peer, got %d", evicted)
	}
	if _, found := book.peers["bad"]; found {
		t.Fatalf("expected bad peer evicted: %+v", book.peers)
	}
	if _, found := book.peers["seed"]; !found {
		t.Fatalf("expected permanent seed retained: %+v", book.peers)
	}
}

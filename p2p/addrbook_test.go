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

package p2p

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const AddrBookVersionV1 = "v1"

type AddrBook struct {
	path  string
	peers map[PeerID]AddrBookPeer
	now   func() time.Time
}

type AddrBookDocument struct {
	SchemaVersion string         `json:"schema_version"`
	Peers         []AddrBookPeer `json:"peers"`
}

type AddrBookPeer struct {
	ID        PeerID `json:"id"`
	Address   string `json:"address"`
	Source    string `json:"source,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
	Permanent bool   `json:"permanent,omitempty"`
}

func OpenAddrBook(path string) (*AddrBook, error) {
	book := &AddrBook{
		path:  path,
		peers: make(map[PeerID]AddrBookPeer),
		now:   time.Now,
	}
	if err := book.Load(); err != nil {
		return nil, err
	}
	return book, nil
}

func (book *AddrBook) Load() error {
	if book.path == "" {
		return nil
	}
	data, err := os.ReadFile(book.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var document AddrBookDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	if document.SchemaVersion != "" && document.SchemaVersion != AddrBookVersionV1 {
		return nil
	}
	for _, peer := range document.Peers {
		if peer.ID == "" || peer.Address == "" {
			continue
		}
		book.peers[peer.ID] = peer
	}
	return nil
}

func (book *AddrBook) Save() error {
	if book.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(book.path), 0o755); err != nil {
		return err
	}
	document := AddrBookDocument{
		SchemaVersion: AddrBookVersionV1,
		Peers:         book.Peers(),
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(book.path, append(data, '\n'), 0o600)
}

func (book *AddrBook) Add(peerID PeerID, address string, source string, permanent bool) {
	if peerID == "" || address == "" {
		return
	}
	peer := book.peers[peerID]
	peer.ID = peerID
	peer.Address = address
	peer.Source = source
	peer.LastSeen = book.now().UTC().Format(time.RFC3339Nano)
	peer.Permanent = peer.Permanent || permanent
	book.peers[peerID] = peer
}

func (book *AddrBook) Merge(peers map[PeerID]string, source string, permanent bool) {
	for peerID, address := range peers {
		book.Add(peerID, address, source, permanent)
	}
}

func (book *AddrBook) PeerMap(exclude PeerID) map[PeerID]string {
	peers := make(map[PeerID]string, len(book.peers))
	for peerID, peer := range book.peers {
		if peerID == exclude || peer.Address == "" {
			continue
		}
		peers[peerID] = peer.Address
	}
	return peers
}

func (book *AddrBook) Peers() []AddrBookPeer {
	peerIDs := make([]string, 0, len(book.peers))
	for peerID := range book.peers {
		peerIDs = append(peerIDs, string(peerID))
	}
	sort.Strings(peerIDs)
	peers := make([]AddrBookPeer, 0, len(peerIDs))
	for _, peerID := range peerIDs {
		peers = append(peers, book.peers[PeerID(peerID)])
	}
	return peers
}

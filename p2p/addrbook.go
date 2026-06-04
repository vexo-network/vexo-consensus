package p2p

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const AddrBookVersionV1 = "v1"

var ErrInvalidPeerAddress = errors.New("invalid peer address")

type AddrBook struct {
	mu          sync.Mutex
	path        string
	maxFailures int
	peers       map[PeerID]AddrBookPeer
	now         func() time.Time
}

type AddrBookDocument struct {
	SchemaVersion string         `json:"schema_version"`
	Peers         []AddrBookPeer `json:"peers"`
}

type AddrBookPeer struct {
	ID          PeerID `json:"id"`
	Address     string `json:"address"`
	Source      string `json:"source,omitempty"`
	LastSeen    string `json:"last_seen,omitempty"`
	LastAttempt string `json:"last_attempt,omitempty"`
	LastFailure string `json:"last_failure,omitempty"`
	Failures    uint64 `json:"failures,omitempty"`
	Banned      bool   `json:"banned,omitempty"`
	BannedUntil string `json:"banned_until,omitempty"`
	Permanent   bool   `json:"permanent,omitempty"`
}

func OpenAddrBook(path string) (*AddrBook, error) {
	return OpenAddrBookWithPolicy(path, 3)
}

func OpenAddrBookWithPolicy(path string, maxFailures int) (*AddrBook, error) {
	book := &AddrBook{
		path:        path,
		maxFailures: maxFailures,
		peers:       make(map[PeerID]AddrBookPeer),
		now:         time.Now,
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
	book.mu.Lock()
	defer book.mu.Unlock()
	for _, peer := range document.Peers {
		if peer.ID == "" || !ValidPeerAddress(peer.Address) {
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
		Peers:         book.peersSnapshot(),
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(book.path, append(data, '\n'), 0o600)
}

func (book *AddrBook) Add(peerID PeerID, address string, source string, permanent bool) {
	if peerID == "" || !ValidPeerAddress(address) {
		return
	}
	book.mu.Lock()
	defer book.mu.Unlock()
	peer := book.peers[peerID]
	peer.ID = peerID
	peer.Address = address
	peer.Source = source
	peer.LastSeen = book.now().UTC().Format(time.RFC3339Nano)
	peer.Failures = 0
	peer.LastFailure = ""
	peer.Banned = false
	peer.BannedUntil = ""
	peer.Permanent = peer.Permanent || permanent
	book.peers[peerID] = peer
}

func (book *AddrBook) Merge(peers map[PeerID]string, source string, permanent bool) {
	book.mu.Lock()
	defer book.mu.Unlock()
	for peerID, address := range peers {
		book.addLocked(peerID, address, source, permanent)
	}
}

func (book *AddrBook) PeerMap(exclude PeerID) map[PeerID]string {
	book.mu.Lock()
	defer book.mu.Unlock()
	peers := make(map[PeerID]string, len(book.peers))
	for peerID, peer := range book.peers {
		if peerID == exclude || peer.Address == "" || book.peerBannedLocked(peer) {
			continue
		}
		peers[peerID] = peer.Address
	}
	return peers
}

func (book *AddrBook) MarkAttempt(peerID PeerID) {
	book.mu.Lock()
	defer book.mu.Unlock()
	peer := book.peers[peerID]
	if peer.ID == "" {
		return
	}
	peer.LastAttempt = book.now().UTC().Format(time.RFC3339Nano)
	book.peers[peerID] = peer
}

func (book *AddrBook) MarkSuccess(peerID PeerID) {
	book.mu.Lock()
	defer book.mu.Unlock()
	peer := book.peers[peerID]
	if peer.ID == "" {
		return
	}
	peer.LastSeen = book.now().UTC().Format(time.RFC3339Nano)
	peer.Failures = 0
	peer.LastFailure = ""
	peer.Banned = false
	peer.BannedUntil = ""
	book.peers[peerID] = peer
}

func (book *AddrBook) MarkFailure(peerID PeerID, banDuration time.Duration) {
	book.mu.Lock()
	defer book.mu.Unlock()
	peer := book.peers[peerID]
	if peer.ID == "" {
		return
	}
	now := book.now().UTC()
	peer.LastFailure = now.Format(time.RFC3339Nano)
	peer.Failures++
	if book.maxFailures > 0 && int(peer.Failures) >= book.maxFailures {
		peer.Banned = true
		if banDuration > 0 {
			peer.BannedUntil = now.Add(banDuration).Format(time.RFC3339Nano)
		}
	}
	book.peers[peerID] = peer
}

func (book *AddrBook) Ban(peerID PeerID, banDuration time.Duration) {
	book.mu.Lock()
	defer book.mu.Unlock()
	peer := book.peers[peerID]
	if peer.ID == "" {
		return
	}
	peer.Banned = true
	if banDuration > 0 {
		peer.BannedUntil = book.now().UTC().Add(banDuration).Format(time.RFC3339Nano)
	}
	book.peers[peerID] = peer
}

func (book *AddrBook) IsBanned(peerID PeerID) bool {
	book.mu.Lock()
	defer book.mu.Unlock()
	peer := book.peers[peerID]
	if peer.ID == "" {
		return false
	}
	return book.peerBannedLocked(peer)
}

func (book *AddrBook) EvictBanned() int {
	book.mu.Lock()
	defer book.mu.Unlock()
	evicted := 0
	for peerID, peer := range book.peers {
		if !book.peerBannedLocked(peer) || peer.Permanent {
			continue
		}
		delete(book.peers, peerID)
		evicted++
	}
	return evicted
}

func (book *AddrBook) peerBannedLocked(peer AddrBookPeer) bool {
	if !peer.Banned {
		return false
	}
	if peer.BannedUntil == "" {
		return true
	}
	until, err := time.Parse(time.RFC3339Nano, peer.BannedUntil)
	if err != nil {
		return true
	}
	if book.now().Before(until) {
		return true
	}
	peer.Banned = false
	peer.BannedUntil = ""
	book.peers[peer.ID] = peer
	return false
}

func (book *AddrBook) Peers() []AddrBookPeer {
	book.mu.Lock()
	defer book.mu.Unlock()
	return book.peersSnapshotLocked()
}

func (book *AddrBook) addLocked(peerID PeerID, address string, source string, permanent bool) {
	if peerID == "" || !ValidPeerAddress(address) {
		return
	}
	peer := book.peers[peerID]
	peer.ID = peerID
	peer.Address = address
	peer.Source = source
	peer.LastSeen = book.now().UTC().Format(time.RFC3339Nano)
	peer.Failures = 0
	peer.LastFailure = ""
	peer.Banned = false
	peer.BannedUntil = ""
	peer.Permanent = peer.Permanent || permanent
	book.peers[peerID] = peer
}

func ValidPeerAddress(address string) bool {
	return ValidatePeerAddress(address) == nil
}

func ValidatePeerAddress(address string) error {
	host, portValue, err := net.SplitHostPort(address)
	if err != nil || host == "" || portValue == "" {
		return ErrInvalidPeerAddress
	}
	port, err := strconv.ParseUint(portValue, 10, 16)
	if err != nil || port == 0 {
		return ErrInvalidPeerAddress
	}
	if strings.ContainsAny(host, " \t\r\n/") {
		return ErrInvalidPeerAddress
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && ip.IsUnspecified() {
		return ErrInvalidPeerAddress
	}
	return nil
}

func (book *AddrBook) peersSnapshot() []AddrBookPeer {
	book.mu.Lock()
	defer book.mu.Unlock()
	return book.peersSnapshotLocked()
}

func (book *AddrBook) peersSnapshotLocked() []AddrBookPeer {
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

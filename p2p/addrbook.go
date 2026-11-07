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

const (
	AddrBookVersionV1 = "v1"

	// DefaultMaxLearnedPeers bounds the non-permanent entries an address book
	// retains. Permanent (operator-configured) entries are never counted
	// against, evicted by, or overwritten by learned entries.
	DefaultMaxLearnedPeers = 1024

	addrBookSaveDelay = time.Second
)

var ErrInvalidPeerAddress = errors.New("invalid peer address")
var ErrUnsupportedAddrBookSchema = errors.New("unsupported addrbook schema")

type AddrBook struct {
	mu          sync.Mutex
	path        string
	maxFailures int
	maxLearned  int
	peers       map[PeerID]AddrBookPeer
	now         func() time.Time
	saveTimer   *time.Timer
	closed      bool
	// saveMu serializes writes of the file, so a deferred save never interleaves with another
	// save and none runs after Close returns.
	saveMu sync.Mutex
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
		maxLearned:  DefaultMaxLearnedPeers,
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
		return ErrUnsupportedAddrBookSchema
	}
	book.mu.Lock()
	defer book.mu.Unlock()
	learned := make([]AddrBookPeer, 0, len(document.Peers))
	for _, peer := range document.Peers {
		if !ValidPeerID(peer.ID) || !ValidPeerAddress(peer.Address) {
			continue
		}
		if peer.Permanent {
			book.peers[peer.ID] = peer
			continue
		}
		learned = append(learned, peer)
	}
	// A file that holds more learned entries than the bucket allows (for
	// example one written before the bound existed) keeps bans first, so the
	// cap cannot launder a ban, and then the most recently seen entries.
	sort.Slice(learned, func(left int, right int) bool {
		if learned[left].Banned != learned[right].Banned {
			return learned[left].Banned
		}
		leftSeen, rightSeen := parseAddrBookTime(learned[left].LastSeen), parseAddrBookTime(learned[right].LastSeen)
		if !leftSeen.Equal(rightSeen) {
			return leftSeen.After(rightSeen)
		}
		return learned[left].ID < learned[right].ID
	})
	if len(learned) > book.maxLearned {
		learned = learned[:book.maxLearned]
	}
	for _, peer := range learned {
		book.peers[peer.ID] = peer
	}
	return nil
}

func (book *AddrBook) Save() error {
	if book.path == "" {
		return nil
	}
	book.saveMu.Lock()
	defer book.saveMu.Unlock()
	return book.saveLocked()
}

func (book *AddrBook) saveLocked() error {
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
	return writeFileAtomic(book.path, append(data, '\n'), 0o600)
}

// Add records a peer. Unauthenticated sources (permanent=false) can only insert
// new, bounded, non-permanent entries: they never rewrite the address of an
// existing entry and never displace an operator-configured peer. Operator
// configuration (permanent=true) is authoritative for the address of the peers
// it names. Neither source clears a ban or a failure history: a ban only ends
// when it expires or the entry is removed.
func (book *AddrBook) Add(peerID PeerID, address string, source string, permanent bool) {
	book.mu.Lock()
	defer book.mu.Unlock()
	book.addLocked(peerID, address, source, permanent)
}

// Merge applies Add to every entry in a deterministic order, so which learned
// entries fit into a full bucket does not depend on map iteration order.
func (book *AddrBook) Merge(peers map[PeerID]string, source string, permanent bool) {
	book.mu.Lock()
	defer book.mu.Unlock()
	peerIDs := make([]string, 0, len(peers))
	for peerID := range peers {
		peerIDs = append(peerIDs, string(peerID))
	}
	sort.Strings(peerIDs)
	for _, peerID := range peerIDs {
		book.addLocked(PeerID(peerID), peers[PeerID(peerID)], source, permanent)
	}
}

// PeerMap returns every non-banned entry except exclude.
func (book *AddrBook) PeerMap(exclude PeerID) map[PeerID]string {
	return book.selectPeers(exclude, func(AddrBookPeer) bool { return true })
}

// PermanentPeerMap returns the non-banned operator-configured entries.
func (book *AddrBook) PermanentPeerMap(exclude PeerID) map[PeerID]string {
	return book.selectPeers(exclude, func(peer AddrBookPeer) bool { return peer.Permanent })
}

// LearnedPeerMap returns the non-banned entries learned from the network.
// These are dial candidates that must be verified by a handshake; they are
// never promoted to operator-configured peers.
func (book *AddrBook) LearnedPeerMap(exclude PeerID) map[PeerID]string {
	return book.selectPeers(exclude, func(peer AddrBookPeer) bool { return !peer.Permanent })
}

func (book *AddrBook) selectPeers(exclude PeerID, include func(AddrBookPeer) bool) map[PeerID]string {
	book.mu.Lock()
	defer book.mu.Unlock()
	peers := make(map[PeerID]string, len(book.peers))
	for peerID, peer := range book.peers {
		if peerID == exclude || peer.Address == "" || !include(peer) || book.peerBannedLocked(peer) {
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
	if !peer.Permanent && book.maxFailures > 0 && int(peer.Failures) >= book.maxFailures {
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
	if !ValidPeerID(peerID) || !ValidPeerAddress(address) {
		return
	}
	now := book.now().UTC().Format(time.RFC3339Nano)
	existing, found := book.peers[peerID]
	if found {
		if !permanent {
			return
		}
		existing.Address = address
		existing.Source = source
		existing.LastSeen = now
		existing.Permanent = true
		book.peers[peerID] = existing
		return
	}
	if !permanent && !book.makeRoomLocked() {
		return
	}
	book.peers[peerID] = AddrBookPeer{
		ID:        peerID,
		Address:   address,
		Source:    source,
		LastSeen:  now,
		Permanent: permanent,
	}
}

// makeRoomLocked reports whether one more learned entry fits, evicting the
// stalest healthy learned entry when the learned bucket is full.
func (book *AddrBook) makeRoomLocked() bool {
	if book.maxLearned <= 0 {
		return false
	}
	if book.learnedCountLocked() < book.maxLearned {
		return true
	}
	victim, found := book.evictionVictimLocked()
	if !found {
		return false
	}
	delete(book.peers, victim)
	return true
}

func (book *AddrBook) learnedCountLocked() int {
	count := 0
	for _, peer := range book.peers {
		if !peer.Permanent {
			count++
		}
	}
	return count
}

// evictionVictimLocked picks the least recently seen learned entry that is not
// under an active ban. Banned entries are kept so that a ban cannot be erased
// by flooding the learned bucket with fresh identities.
func (book *AddrBook) evictionVictimLocked() (PeerID, bool) {
	var victim PeerID
	var victimSeen time.Time
	found := false
	for peerID, peer := range book.peers {
		if peer.Permanent || book.peerBannedLocked(peer) {
			continue
		}
		seen := parseAddrBookTime(peer.LastSeen)
		if !found || seen.Before(victimSeen) || (seen.Equal(victimSeen) && peerID < victim) {
			victim = peerID
			victimSeen = seen
			found = true
		}
	}
	return victim, found
}

func parseAddrBookTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// SaveDeferred schedules a write-behind Save and coalesces bursts of
// mutations into a single disk write.
func (book *AddrBook) SaveDeferred() {
	if book.path == "" {
		return
	}
	book.mu.Lock()
	defer book.mu.Unlock()
	if book.saveTimer != nil || book.closed {
		return
	}
	book.saveTimer = time.AfterFunc(addrBookSaveDelay, func() {
		book.saveMu.Lock()
		defer book.saveMu.Unlock()
		book.mu.Lock()
		book.saveTimer = nil
		closed := book.closed
		book.mu.Unlock()
		if closed {
			return
		}
		_ = book.saveLocked()
	})
}

// Close cancels any pending deferred save, writes the book, and schedules no deferred save after
// it: the owner of the file may be gone once Close returns. Save still writes on request.
func (book *AddrBook) Close() error {
	book.mu.Lock()
	book.closed = true
	book.mu.Unlock()
	return book.Flush()
}

// Flush cancels any pending deferred save and writes the book immediately.
func (book *AddrBook) Flush() error {
	book.mu.Lock()
	if book.saveTimer != nil {
		book.saveTimer.Stop()
		book.saveTimer = nil
	}
	book.mu.Unlock()
	return book.Save()
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

package transport

import (
	"errors"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

const (
	// DefaultMaxLearnedPeers bounds the pool of unverified dial candidates
	// learned from handshakes.
	DefaultMaxLearnedPeers = 128

	// maxLearnedPerHandshake and maxLearnedPerMinute bound how fast a single
	// peer, or the network as a whole, can make this node learn candidates.
	maxLearnedPerHandshake = 16
	maxLearnedPerMinute    = 64

	// learnedMaxFailures is the number of consecutive failed dials after which
	// a learned candidate is forgotten.
	learnedMaxFailures = 5

	// maxLearnedDialsPerTick bounds how many unverified candidates one
	// reconnect pass dials.
	maxLearnedDialsPerTick = 8
)

// Peers learned from handshakes are hints, not configuration. A peer list
// carried by a handshake is signed only by the peer that sent it, so it proves
// nothing about the listed peers. Hints therefore live in a separate bounded
// pool of dial candidates:
//
//   - they never evict, rewrite, or demote an operator-configured peer;
//   - a candidate is only ever dialed, and must pass the same authenticated
//     handshake as any peer before it receives traffic;
//   - learning is rate limited per handshake and per minute;
//   - when the pool is full only candidates without a live session are
//     replaced, oldest first;
//   - candidates that keep failing are forgotten.
type dialTarget struct {
	peerID  p2p.PeerID
	address string
	learned bool
}

func (transport *GRPCTransport) learnedCapacityLocked() int {
	capacity := transport.maxLearned
	if transport.maxPeers > 0 {
		capacity = min(capacity, transport.maxPeers-len(transport.peers))
	}
	return max(capacity, 0)
}

// learnPeerLocked adds an unverified candidate and reports whether it was
// admitted.
func (transport *GRPCTransport) learnPeerLocked(peerID p2p.PeerID, address string) bool {
	if peerID == transport.peerID || !p2p.ValidPeerID(peerID) || !p2p.ValidPeerAddress(address) {
		return false
	}
	// With key-bound identities a peer id that is not a key digest can never
	// complete a handshake, so it is not worth a pool slot.
	if transport.requireHandshakeSignature && !p2p.IsDerivedNodeID(peerID) {
		return false
	}
	if _, configured := transport.peers[peerID]; configured {
		return false
	}
	if _, known := transport.learned[peerID]; known {
		return false
	}
	capacity := transport.learnedCapacityLocked()
	if capacity == 0 || !transport.fitLearnedLocked(capacity-1) {
		return false
	}
	transport.learned[peerID] = address
	transport.learnedOrder = append(transport.learnedOrder, peerID)
	return true
}

// fitLearnedLocked evicts the oldest candidates without a live session until
// at most limit remain, and reports whether that was possible.
func (transport *GRPCTransport) fitLearnedLocked(limit int) bool {
	for len(transport.learned) > limit {
		victim := p2p.PeerID("")
		for _, peerID := range transport.learnedOrder {
			if transport.sessions[peerID] == nil {
				victim = peerID
				break
			}
		}
		if victim == "" {
			return false
		}
		transport.dropLearnedLocked(victim)
	}
	return true
}

func (transport *GRPCTransport) trimLearnedLocked() {
	transport.fitLearnedLocked(transport.learnedCapacityLocked())
}

func (transport *GRPCTransport) dropLearnedLocked(peerID p2p.PeerID) {
	if _, found := transport.learned[peerID]; !found {
		return
	}
	transport.closePeerSessionLocked(peerID)
	transport.closePeerConnectionLocked(peerID)
	delete(transport.learned, peerID)
	delete(transport.backoffUntil, peerID)
	delete(transport.failures, peerID)
	delete(transport.sessionLocks, peerID)
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_removed", PeerID: peerID})
	for index, existing := range transport.learnedOrder {
		if existing == peerID {
			transport.learnedOrder = append(transport.learnedOrder[:index], transport.learnedOrder[index+1:]...)
			return
		}
	}
}

// learnHandshakePeers records the dial hints carried by an authenticated
// handshake: the listen address its sender advertises for itself and the peers
// it lists. The peer-learned hook fires once per newly admitted candidate,
// outside the transport lock.
func (transport *GRPCTransport) learnHandshakePeers(handshake Handshake) {
	type discovery struct {
		peerID  p2p.PeerID
		address string
	}
	discovered := make([]discovery, 0, len(handshake.KnownPeers)+1)
	if handshake.NodeID != "" && handshake.ListenAddr != "" {
		discovered = append(discovered, discovery{peerID: handshake.NodeID, address: handshake.ListenAddr})
	}
	for _, peerID := range sortedPeerIDs(handshake.KnownPeers) {
		if peerID != handshake.NodeID {
			discovered = append(discovered, discovery{peerID: peerID, address: handshake.KnownPeers[peerID]})
		}
	}
	now := time.Now()
	admitted := make([]discovery, 0, len(discovered))
	transport.mu.Lock()
	if now.Sub(transport.learnWindowStart) >= time.Minute {
		transport.learnWindowStart = now
		transport.learnWindowCount = 0
	}
	for _, entry := range discovered {
		if len(admitted) >= maxLearnedPerHandshake || transport.learnWindowCount >= maxLearnedPerMinute {
			break
		}
		if transport.learnPeerLocked(entry.peerID, entry.address) {
			transport.learnWindowCount++
			admitted = append(admitted, entry)
		}
	}
	hook := transport.peerLearned
	transport.mu.Unlock()
	if hook == nil {
		return
	}
	for _, entry := range admitted {
		hook(entry.peerID, entry.address)
	}
}

// recordLearnedFailure accounts one failed dial of a learned candidate.
// Failures that retrying cannot fix, and repeated failures, forget it.
func (transport *GRPCTransport) recordLearnedFailure(peerID p2p.PeerID, err error) {
	if isPermanentHandshakeError(err) || errors.Is(err, ErrPeerRejected) {
		transport.mu.Lock()
		defer transport.mu.Unlock()
		transport.dropLearnedLocked(peerID)
		return
	}
	transport.markPeerBackoff(peerID)
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.failures[peerID] >= learnedMaxFailures {
		transport.dropLearnedLocked(peerID)
	}
}

// dialTargets lists the peers the reconnect loop should dial now: configured
// peers in configuration order, then a bounded number of learned candidates
// oldest first. Peers with a live session or an active backoff are skipped.
func (transport *GRPCTransport) dialTargets(now time.Time) []dialTarget {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	ready := func(peerID p2p.PeerID) bool {
		if transport.sessions[peerID] != nil {
			return false
		}
		until := transport.backoffUntil[peerID]
		return until.IsZero() || now.After(until)
	}
	targets := make([]dialTarget, 0, len(transport.peers))
	for _, peerID := range transport.peerOrder {
		if ready(peerID) {
			targets = append(targets, dialTarget{peerID: peerID, address: transport.peers[peerID]})
		}
	}
	learnedDials := 0
	for _, peerID := range transport.learnedOrder {
		if learnedDials >= maxLearnedDialsPerTick {
			break
		}
		if ready(peerID) {
			targets = append(targets, dialTarget{peerID: peerID, address: transport.learned[peerID], learned: true})
			learnedDials++
		}
	}
	return targets
}

// publishTargets lists the peers a broadcast goes to: every configured peer,
// and learned peers that have completed a handshake.
func (transport *GRPCTransport) publishTargets() map[p2p.PeerID]string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	targets := make(map[p2p.PeerID]string, len(transport.peers)+len(transport.sessions))
	for peerID, address := range transport.peers {
		targets[peerID] = address
	}
	for peerID, address := range transport.learned {
		if transport.sessions[peerID] != nil {
			targets[peerID] = address
		}
	}
	return targets
}

// advertisedPeers lists the peers this node tells others about: itself, its
// configured peers, and learned peers it holds a verified session with. The
// result is bounded and deterministic because the receiver rejects oversized
// or malformed lists outright.
func (transport *GRPCTransport) advertisedPeers() map[p2p.PeerID]string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	advertised := make(map[p2p.PeerID]string)
	add := func(peerID p2p.PeerID, address string) {
		if len(advertised) < maxHandshakePeers && p2p.ValidPeerID(peerID) && p2p.ValidPeerAddress(address) {
			advertised[peerID] = address
		}
	}
	add(transport.peerID, transport.listenAddr)
	for _, peerID := range sortedPeerIDs(transport.peers) {
		add(peerID, transport.peers[peerID])
	}
	for _, peerID := range transport.learnedOrder {
		if transport.sessions[peerID] != nil {
			add(peerID, transport.learned[peerID])
		}
	}
	return advertised
}

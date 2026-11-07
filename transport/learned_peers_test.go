package transport

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestLearnedPeersNeverEvictOrRewriteConfiguredPeers(t *testing.T) {
	configured := map[p2p.PeerID]string{"cfg-1": "127.0.0.1:1001", "cfg-2": "127.0.0.1:1002", "cfg-3": "127.0.0.1:1003"}
	transport := newUnstartedTransport(t, "alice", GRPCConfig{Peers: configured, MaxPeers: 8, MaxLearnedPeers: 16})

	for batch := 0; batch < 200; batch++ {
		hints := map[p2p.PeerID]string{
			// An attacker that lists a configured peer under its own address.
			"cfg-1": "10.0.0.66:9999",
		}
		for index := 0; index < 40; index++ {
			hints[p2p.PeerID(fmt.Sprintf("flood-%d-%d", batch, index))] = fmt.Sprintf("10.1.%d.%d:26656", batch, index)
		}
		// Reopen the learning window so the flood is limited only by the pool.
		transport.mu.Lock()
		transport.learnWindowStart = time.Time{}
		transport.mu.Unlock()
		transport.learnHandshakePeers(Handshake{NodeID: "mallory", ListenAddr: "10.0.0.1:26656", KnownPeers: hints})
	}

	known := transport.KnownPeers()
	for peerID, address := range configured {
		if known[peerID] != address {
			t.Fatalf("configured peer %s was displaced or rewritten: %+v", peerID, known)
		}
	}
	if got := transport.ConfiguredPeerIDs(); len(got) != len(configured) {
		t.Fatalf("learning changed the configured peer set: %v", got)
	}
	transport.mu.RLock()
	learned := len(transport.learned)
	transport.mu.RUnlock()
	if capacity := 8 - len(configured); learned > capacity {
		t.Fatalf("learned pool %d exceeds the room left by configured peers %d", learned, capacity)
	}
}

func TestLearnedPeersAreRateLimited(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{MaxLearnedPeers: 1000})
	hints := make(map[p2p.PeerID]string)
	for index := 0; index < 100; index++ {
		hints[p2p.PeerID(fmt.Sprintf("peer-%03d", index))] = fmt.Sprintf("10.0.0.%d:26656", index+1)
	}
	transport.learnHandshakePeers(Handshake{NodeID: "mallory", ListenAddr: "10.9.9.9:26656", KnownPeers: hints})
	transport.mu.RLock()
	afterOne := len(transport.learned)
	transport.mu.RUnlock()
	if afterOne != maxLearnedPerHandshake {
		t.Fatalf("expected %d candidates from one handshake, got %d", maxLearnedPerHandshake, afterOne)
	}

	for batch := 0; batch < 20; batch++ {
		fresh := make(map[p2p.PeerID]string)
		for index := 0; index < 40; index++ {
			fresh[p2p.PeerID(fmt.Sprintf("batch-%d-%d", batch, index))] = fmt.Sprintf("10.2.%d.%d:26656", batch, index+1)
		}
		transport.learnHandshakePeers(Handshake{NodeID: p2p.PeerID(fmt.Sprintf("sender-%d", batch)), KnownPeers: fresh})
	}
	transport.mu.RLock()
	afterMany := len(transport.learned)
	transport.mu.RUnlock()
	if afterMany != maxLearnedPerMinute {
		t.Fatalf("expected the per-minute limit of %d candidates, got %d", maxLearnedPerMinute, afterMany)
	}

	transport.mu.Lock()
	transport.learnWindowStart = time.Now().Add(-2 * time.Minute)
	transport.mu.Unlock()
	transport.learnHandshakePeers(Handshake{NodeID: "later", KnownPeers: map[p2p.PeerID]string{"after-window": "10.3.0.1:26656"}})
	if _, found := transport.KnownPeers()["after-window"]; !found {
		t.Fatal("expected learning to resume after the window elapsed")
	}
}

func TestLearnedPeerHookRunsOutsideTheTransportLock(t *testing.T) {
	var transport *GRPCTransport
	learned := make(map[p2p.PeerID]string)
	transport = newUnstartedTransport(t, "alice", GRPCConfig{PeerLearned: func(peerID p2p.PeerID, address string) {
		// Taking the read lock would deadlock if the hook ran under the write lock.
		_ = transport.KnownPeers()
		learned[peerID] = address
	}})
	transport.learnHandshakePeers(Handshake{NodeID: "bob", ListenAddr: "10.0.0.2:26656", KnownPeers: map[p2p.PeerID]string{"carol": "10.0.0.3:26656"}})
	if learned["bob"] != "10.0.0.2:26656" || learned["carol"] != "10.0.0.3:26656" {
		t.Fatalf("expected both candidates to be reported, got %+v", learned)
	}
}

func TestLearnedPeerEvictionSkipsPeersWithSessions(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{MaxLearnedPeers: 2})
	transport.learnHandshakePeers(Handshake{NodeID: "bob", ListenAddr: "10.0.0.2:26656"})
	transport.learnHandshakePeers(Handshake{NodeID: "carol", ListenAddr: "10.0.0.3:26656"})
	// bob and carol both hold verified sessions, so no unverified hint may
	// push them out.
	transport.mu.Lock()
	transport.sessions["bob"] = &grpcPeerSession{}
	transport.sessions["carol"] = &grpcPeerSession{}
	transport.mu.Unlock()
	transport.learnHandshakePeers(Handshake{NodeID: "mallory", ListenAddr: "10.0.0.66:26656"})
	known := transport.KnownPeers()
	if _, found := known["mallory"]; found || known["bob"] == "" || known["carol"] == "" {
		t.Fatalf("an unverified hint displaced connected peers: %+v", known)
	}

	transport.mu.Lock()
	delete(transport.sessions, "bob")
	transport.mu.Unlock()
	transport.learnHandshakePeers(Handshake{NodeID: "dave", ListenAddr: "10.0.0.4:26656"})
	known = transport.KnownPeers()
	if known["dave"] == "" || known["bob"] != "" || known["carol"] == "" {
		t.Fatalf("expected the sessionless candidate to be replaced first: %+v", known)
	}
	transport.mu.Lock()
	delete(transport.sessions, "carol")
	transport.mu.Unlock()
}

func TestLearnedPeersAreOnlyKeyBoundIdentitiesWhenSignaturesAreRequired(t *testing.T) {
	transport := newSignedTransport(t, GRPCConfig{})
	derived := p2p.NodeIDFromPublicKey(newTestHandshakeSigner(t).PublicKey())
	transport.learnHandshakePeers(Handshake{NodeID: derived, ListenAddr: "10.0.0.2:26656", KnownPeers: map[p2p.PeerID]string{"validator-9": "10.0.0.9:26656"}})
	known := transport.KnownPeers()
	if known[derived] == "" {
		t.Fatalf("expected the key-bound identity to be learned: %+v", known)
	}
	if _, found := known["validator-9"]; found {
		t.Fatalf("an identity that can never authenticate occupied a pool slot: %+v", known)
	}
}

func TestSetPeerPromotesLearnedPeer(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{})
	transport.learnHandshakePeers(Handshake{NodeID: "bob", ListenAddr: "10.0.0.2:26656"})
	transport.SetPeer("bob", "10.0.0.20:26656")
	transport.mu.RLock()
	_, stillLearned := transport.learned["bob"]
	transport.mu.RUnlock()
	if stillLearned || len(transport.ConfiguredPeerIDs()) != 1 || transport.KnownPeers()["bob"] != "10.0.0.20:26656" {
		t.Fatalf("expected bob to become a configured peer with the operator address: %+v", transport.KnownPeers())
	}
}

func TestSetPeerDropsLearnedCandidatesBeforeConfiguredPeers(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{MaxPeers: 3, MaxLearnedPeers: 8})
	transport.SetPeer("cfg-1", "127.0.0.1:1")
	transport.learnHandshakePeers(Handshake{NodeID: "x", ListenAddr: "10.0.0.1:1", KnownPeers: map[p2p.PeerID]string{"y": "10.0.0.2:1"}})
	transport.SetPeer("cfg-2", "127.0.0.1:2")
	known := transport.KnownPeers()
	if len(known) != 3 || known["cfg-1"] == "" || known["cfg-2"] == "" {
		t.Fatalf("expected configured peers to keep their place and learned candidates to shrink: %+v", known)
	}
}

func TestLearnedPeerIsForgottenAfterRepeatedDialFailures(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{
		DialTimeout:      20 * time.Millisecond,
		ReconnectBackoff: time.Millisecond,
		Peers:            map[p2p.PeerID]string{"cfg": "127.0.0.1:1"},
	})
	transport.learnHandshakePeers(Handshake{NodeID: "ghost", ListenAddr: "127.0.0.1:2"})
	transport.mu.Lock()
	transport.rootCtx = context.Background()
	transport.mu.Unlock()
	for attempt := 0; attempt < 4*learnedMaxFailures; attempt++ {
		transport.reconnectPeers(context.Background())
		time.Sleep(5 * time.Millisecond)
	}
	known := transport.KnownPeers()
	if _, found := known["ghost"]; found {
		t.Fatalf("expected an unreachable learned candidate to be forgotten: %+v", known)
	}
	if known["cfg"] == "" {
		t.Fatalf("an unreachable configured peer must be kept: %+v", known)
	}
}

func TestLearnedPeerIsForgottenWhenItsIdentityFails(t *testing.T) {
	// The candidate answers, but not as the peer it was learned as.
	imposter := newStartedGRPCPeer(t, "imposter", GRPCConfig{})
	defer stopGRPCPeer(t, imposter)
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{ReconnectInterval: 10 * time.Millisecond, DialTimeout: time.Second})
	defer stopGRPCPeer(t, alice)
	alice.learnHandshakePeers(Handshake{NodeID: "honest-peer", ListenAddr: imposter.Address()})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, found := alice.KnownPeers()["honest-peer"]; !found {
			if len(alice.ActivePeerIDs()) != 0 {
				t.Fatalf("a candidate with the wrong identity received a session: %v", alice.ActivePeerIDs())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected a candidate that answers with a different identity to be forgotten")
}

func TestReconnectBackoffIsExponentialByDefault(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{})
	if transport.reconnectBackoff != defaultReconnectBackoff || defaultReconnectBackoff <= 0 {
		t.Fatalf("expected a default reconnect backoff, got %s", transport.reconnectBackoff)
	}
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for index, expected := range want {
		if got := transport.backoffDelay(index + 1); got != expected {
			t.Fatalf("failure %d: expected backoff %s, got %s", index+1, expected, got)
		}
	}
	if got := transport.backoffDelay(1 << 30); got != maxReconnectBackoff {
		t.Fatalf("expected backoff to stay capped, got %s", got)
	}
	explicit := newUnstartedTransport(t, "bob", GRPCConfig{ReconnectBackoff: time.Minute})
	if got := explicit.backoffDelay(5); got != time.Minute {
		t.Fatalf("a configured base above the cap must be honoured, got %s", got)
	}
}

func TestFailedDialsBackOffInsteadOfRetryingEveryTick(t *testing.T) {
	var attempts atomic.Int32
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{
		ReconnectInterval: 5 * time.Millisecond,
		ReconnectBackoff:  50 * time.Millisecond,
		DialTimeout:       200 * time.Millisecond,
		Peers:             map[p2p.PeerID]string{"ghost": "127.0.0.1:1"},
		PeerAttempted:     func(p2p.PeerID) { attempts.Add(1) },
	})
	defer stopGRPCPeer(t, alice)
	time.Sleep(400 * time.Millisecond)
	// Without backoff the 5ms loop would dial about 80 times.
	if count := attempts.Load(); count == 0 || count > 8 {
		t.Fatalf("expected a handful of backed-off dial attempts, got %d", count)
	}
}

func TestReconnectLoopLeavesConnectedPeersAlone(t *testing.T) {
	var attempts atomic.Int32
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{})
	defer stopGRPCPeer(t, bob)
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{
		ReconnectInterval: 5 * time.Millisecond,
		Peers:             map[p2p.PeerID]string{"bob": bob.Address()},
		PeerAttempted:     func(p2p.PeerID) { attempts.Add(1) },
	})
	defer stopGRPCPeer(t, alice)
	waitForGRPCSessionCount(t, alice, 1)
	settled := attempts.Load()
	time.Sleep(150 * time.Millisecond)
	if count := attempts.Load(); count != settled {
		t.Fatalf("connected peer was re-dialed %d times, forcing hooks and address book writes", count-settled)
	}
}

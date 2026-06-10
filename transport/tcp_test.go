//go:build legacytcp

package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestTCPTransportPublishBroadcastsToConfiguredPeers(t *testing.T) {
	alice, bob, carol := newStartedTCPPeers(t)
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	carolTxs, err := carol.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); err != nil {
		t.Fatal(err)
	}

	assertEnvelope(t, bobTxs, "alice", "", p2p.TopicTx, "tx")
	assertEnvelope(t, carolTxs, "alice", "", p2p.TopicTx, "tx")
}

func TestTCPTransportSendDeliversOnlyTargetPeer(t *testing.T) {
	alice, bob, carol := newStartedTCPPeers(t)
	bobVotes, err := bob.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}
	carolVotes, err := carol.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}

	if err := alice.Send(context.Background(), "bob", p2p.TopicVote, []byte("vote")); err != nil {
		t.Fatal(err)
	}

	assertEnvelope(t, bobVotes, "alice", "bob", p2p.TopicVote, "vote")
	assertNoEnvelope(t, carolVotes)
}

func TestTCPTransportCanAddPeersAfterStart(t *testing.T) {
	alice := newStartedTCPPeer(t, "alice")
	defer stopTCPPeer(t, alice)
	bob := newStartedTCPPeer(t, "bob")
	defer stopTCPPeer(t, bob)
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	alice.SetPeer("bob", bob.Address())
	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("late-peer")); err != nil {
		t.Fatal(err)
	}

	assertEnvelope(t, bobTxs, "alice", "", p2p.TopicTx, "late-peer")
}

func TestTCPTransportCopiesPayload(t *testing.T) {
	alice, bob, _ := newStartedTCPPeers(t)
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("tx")
	if err := alice.Publish(context.Background(), p2p.TopicTx, payload); err != nil {
		t.Fatal(err)
	}
	payload[0] = 'X'

	assertEnvelope(t, bobTxs, "alice", "", p2p.TopicTx, "tx")
}

func TestTCPTransportRejectsStoppedPeer(t *testing.T) {
	alice := newStartedTCPPeer(t, "alice")
	if err := alice.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected transport closed, got %v", err)
	}
	if _, err := alice.Subscribe(context.Background(), p2p.TopicTx); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected transport closed, got %v", err)
	}
	if err := alice.Stop(context.Background()); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected transport closed on repeated stop, got %v", err)
	}
}

func TestTCPTransportValidationAndContext(t *testing.T) {
	if _, err := NewTCPTransport(TCPConfig{}); !errors.Is(err, ErrPeerIDRequired) {
		t.Fatalf("expected peer id required, got %v", err)
	}

	alice, err := NewTCPTransport(TCPConfig{PeerID: "alice", ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := alice.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled start, got %v", err)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected stopped transport error, got %v", err)
	}
}

func TestTCPTransportRejectsUnknownPeer(t *testing.T) {
	alice := newStartedTCPPeer(t, "alice")
	defer stopTCPPeer(t, alice)
	if err := alice.Send(context.Background(), "unknown", p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrPeerNotFound) {
		t.Fatalf("expected peer not found, got %v", err)
	}
}

func newStartedTCPPeers(t *testing.T) (*TCPTransport, *TCPTransport, *TCPTransport) {
	t.Helper()
	alice := newStartedTCPPeer(t, "alice")
	bob := newStartedTCPPeer(t, "bob")
	carol := newStartedTCPPeer(t, "carol")
	t.Cleanup(func() {
		stopTCPPeer(t, alice)
		stopTCPPeer(t, bob)
		stopTCPPeer(t, carol)
	})
	alice.SetPeer("bob", bob.Address())
	alice.SetPeer("carol", carol.Address())
	bob.SetPeer("alice", alice.Address())
	bob.SetPeer("carol", carol.Address())
	carol.SetPeer("alice", alice.Address())
	carol.SetPeer("bob", bob.Address())
	return alice, bob, carol
}

func newStartedTCPPeer(t *testing.T, peerID p2p.PeerID) *TCPTransport {
	t.Helper()
	peer, err := NewTCPTransport(TCPConfig{
		PeerID:      peerID,
		ListenAddr:  "127.0.0.1:0",
		DialTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return peer
}

func stopTCPPeer(t *testing.T, peer *TCPTransport) {
	t.Helper()
	if peer == nil {
		return
	}
	if err := peer.Stop(context.Background()); err != nil && !errors.Is(err, ErrTransportClosed) {
		t.Fatal(err)
	}
}

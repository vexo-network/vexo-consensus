package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestGRPCTransportPublishBroadcastsToConfiguredPeers(t *testing.T) {
	alice, bob, carol := newStartedGRPCPeers(t)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	defer stopGRPCPeer(t, carol)

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

func TestGRPCTransportSendDeliversOnlyTargetPeer(t *testing.T) {
	alice, bob, carol := newStartedGRPCPeers(t)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	defer stopGRPCPeer(t, carol)

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

func TestGRPCTransportReusesPeerStreamSession(t *testing.T) {
	alice, bob, _ := newStartedGRPCPeers(t)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)

	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	for _, payload := range []string{"one", "two", "three"} {
		if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte(payload)); err != nil {
			t.Fatal(err)
		}
		assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, payload)
	}

	if sessions := grpcSessionCount(alice); sessions != 1 {
		t.Fatalf("expected one cached grpc session, got %d", sessions)
	}
}

func TestGRPCTransportRejectsHandshakeMismatch(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{ChainID: "vexo-a", GenesisHash: GenesisHash([]byte("genesis-a"))})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{ChainID: "vexo-b", GenesisHash: GenesisHash([]byte("genesis-a"))})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx"))
	if !errors.Is(err, ErrChainIDMismatch) {
		t.Fatalf("expected chain id mismatch, got %v", err)
	}
}

func TestGRPCTransportRejectsGenesisHashMismatch(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{ChainID: "vexo-test", GenesisHash: GenesisHash([]byte("genesis-a"))})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{ChainID: "vexo-test", GenesisHash: GenesisHash([]byte("genesis-b"))})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx"))
	if !errors.Is(err, ErrGenesisHashMismatch) {
		t.Fatalf("expected genesis hash mismatch, got %v", err)
	}
}

func TestGRPCTransportValidationAndContext(t *testing.T) {
	if _, err := NewGRPCTransport(GRPCConfig{}); !errors.Is(err, ErrPeerIDRequired) {
		t.Fatalf("expected peer id required, got %v", err)
	}
	alice, err := NewGRPCTransport(GRPCConfig{PeerID: "alice", ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := alice.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled start, got %v", err)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected closed transport, got %v", err)
	}
}

func TestGRPCTransportRejectsUnknownPeer(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	if err := alice.Send(context.Background(), "unknown", p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrPeerNotFound) {
		t.Fatalf("expected peer not found, got %v", err)
	}
}

func newStartedGRPCPeers(t *testing.T) (*GRPCTransport, *GRPCTransport, *GRPCTransport) {
	t.Helper()
	base := GRPCConfig{NetworkID: "localnet", ChainID: "vexo-test", GenesisHash: GenesisHash([]byte("genesis"))}
	alice := newStartedGRPCPeer(t, "alice", base)
	bob := newStartedGRPCPeer(t, "bob", base)
	carol := newStartedGRPCPeer(t, "carol", base)
	alice.SetPeer("bob", bob.Address())
	alice.SetPeer("carol", carol.Address())
	bob.SetPeer("alice", alice.Address())
	bob.SetPeer("carol", carol.Address())
	carol.SetPeer("alice", alice.Address())
	carol.SetPeer("bob", bob.Address())
	return alice, bob, carol
}

func newStartedGRPCPeer(t *testing.T, peerID p2p.PeerID, config GRPCConfig) *GRPCTransport {
	t.Helper()
	config.PeerID = peerID
	if config.ListenAddr == "" {
		config.ListenAddr = "127.0.0.1:0"
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	peer, err := NewGRPCTransport(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return peer
}

func stopGRPCPeer(t *testing.T, peer *GRPCTransport) {
	t.Helper()
	if peer == nil {
		return
	}
	if err := peer.Stop(context.Background()); err != nil && !errors.Is(err, ErrTransportClosed) {
		t.Fatal(err)
	}
}

func grpcSessionCount(peer *GRPCTransport) int {
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	return len(peer.sessions)
}

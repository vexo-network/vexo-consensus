package transport

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// dialRawGossip opens a Gossip stream the way an attacker without a real
// transport would, so the test controls every handshake frame.
func dialRawGossip(t *testing.T, address string) grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage] {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	connection, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcBinaryCodec{})),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	stream, err := newP2PTransportClient(connection).Gossip(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return stream
}

type eventRecorder struct {
	mu     sync.Mutex
	events []PeerEvent
}

func (recorder *eventRecorder) record(event PeerEvent) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
}

func (recorder *eventRecorder) count(eventType string) int {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	count := 0
	for _, event := range recorder.events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func TestGRPCTransportRejectsSpoofedNodeIDOverTheWire(t *testing.T) {
	recorder := &eventRecorder{}
	bob := newSignedTransport(t, GRPCConfig{ListenAddr: "127.0.0.1:0", PeerEvent: recorder.record})
	if err := bob.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer stopGRPCPeer(t, bob)

	victim := newTestHandshakeSigner(t)
	attacker := newTestHandshakeSigner(t)
	victimID := p2p.NodeIDFromPublicKey(victim.PublicKey())
	stream := dialRawGossip(t, bob.Address())
	challenge, err := randomChallenge()
	if err != nil {
		t.Fatal(err)
	}
	hello := Handshake{
		ProtocolVersion: bob.protocolVersion,
		NetworkID:       bob.networkID,
		ChainID:         bob.chainID,
		GenesisHash:     bob.genesisHash,
		NodeID:          victimID,
		NodePublicKey:   attacker.PublicKey(),
		Challenge:       challenge,
	}
	if err := stream.Send(&grpcStreamMessage{Handshake: &hello}); err != nil {
		t.Fatal(err)
	}
	_, err = stream.Recv()
	if !errors.Is(normalizeGRPCError(err), ErrNodeIDMismatch) {
		t.Fatalf("expected the spoofed node id to be rejected as a key mismatch, got %v", err)
	}
	if recorder.count("peer_connected") != 0 || recorder.count("peer_handshake_failed") != 1 {
		t.Fatalf("expected one failed handshake and no connection: %+v", recorder.events)
	}
}

func TestGRPCTransportRefusesPeerThatAnswersAsSomeoneElse(t *testing.T) {
	alice := newStartedSignedPeer(t, GRPCConfig{})
	bob := newStartedSignedPeer(t, GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)

	// The operator configured the id of a validator, but the address leads to
	// bob. Bob proves he is bob, which is not the peer that was configured.
	validatorID := p2p.NodeIDFromPublicKey(newTestHandshakeSigner(t).PublicKey())
	alice.SetPeer(validatorID, bob.Address())
	err := alice.Send(context.Background(), validatorID, p2p.TopicTx, []byte("secret"))
	if !errors.Is(err, ErrHandshakeFailed) {
		t.Fatalf("expected a peer answering with another identity to be refused, got %v", err)
	}
	if len(alice.ActivePeerIDs()) != 0 {
		t.Fatalf("no session may exist with the wrong peer: %v", alice.ActivePeerIDs())
	}
}

func TestGRPCTransportInboundHandshakeTimesOut(t *testing.T) {
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{HandshakeTimeout: 100 * time.Millisecond})
	defer stopGRPCPeer(t, bob)
	stream := dialRawGossip(t, bob.Address())

	started := time.Now()
	_, err := stream.Recv()
	if err == nil || !strings.Contains(err.Error(), ErrHandshakeFailed.Error()) {
		t.Fatalf("expected a stalled handshake to be abandoned by the server, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the server held a silent peer for %s", elapsed)
	}
}

func newStartedSignedPeer(t *testing.T, config GRPCConfig) *GRPCTransport {
	t.Helper()
	if config.ListenAddr == "" {
		config.ListenAddr = "127.0.0.1:0"
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	peer := newSignedTransport(t, config)
	if err := peer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return peer
}

func TestGRPCTransportSignedPeersExchangeMessagesOverMutualTLS(t *testing.T) {
	caCert, caKey, pool := newTestCertificateAuthority(t)
	replayDir := t.TempDir()
	newPeer := func(name string) *GRPCTransport {
		store, err := NewFileAuthReplayStore(filepath.Join(replayDir, name+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		return newStartedSignedPeer(t, GRPCConfig{
			TLSConfig:              newTestPeerTLSConfig(t, caCert, caKey, pool, name),
			RequireTLS:             true,
			AuthToken:              "shared-token",
			AuthReplayStore:        store,
			RequireAuthReplayStore: true,
			DialTimeout:            30 * time.Second,
		})
	}
	alice, bob := newPeer("alice"), newPeer("bob")
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	alice.SetPeer(bob.peerID, bob.Address())
	if err := alice.Send(context.Background(), bob.peerID, p2p.TopicTx, []byte("authenticated")); err != nil {
		t.Fatal(err)
	}
	assertEnvelope(t, bobTxs, alice.peerID, bob.peerID, p2p.TopicTx, "authenticated")
}

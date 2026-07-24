package transport

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestGRPCTransportEmitsPeerConfigurationEvents(t *testing.T) {
	events := make(chan PeerEvent, 4)
	peer, err := NewGRPCTransport(GRPCConfig{
		PeerID: "alice",
		PeerEvent: func(event PeerEvent) {
			events <- event
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	peer.SetPeer("bob", "127.0.0.1:26656")
	peer.DisconnectPeer("bob")
	peer.RemovePeer("bob")

	first := <-events
	if first.Type != "peer_configured" || first.PeerID != "bob" || first.Address != "127.0.0.1:26656" {
		t.Fatalf("unexpected configured event: %+v", first)
	}
	second := <-events
	if second.Type != "peer_disconnected" || second.PeerID != "bob" || second.Reason != "disconnect_requested" {
		t.Fatalf("unexpected disconnect event: %+v", second)
	}
	third := <-events
	if third.Type != "peer_removed" || third.PeerID != "bob" {
		t.Fatalf("unexpected removed event: %+v", third)
	}
}

func TestGRPCTransportReportsConfiguredAndActivePeers(t *testing.T) {
	alice, bob, carol := newStartedGRPCPeers(t)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	defer stopGRPCPeer(t, carol)

	configured := alice.ConfiguredPeerIDs()
	if len(configured) != 2 || configured[0] != "bob" || configured[1] != "carol" {
		t.Fatalf("unexpected configured peers: %v", configured)
	}
	if active := alice.ActivePeerIDs(); len(active) != 0 {
		t.Fatalf("expected no active session before publish, got %v", active)
	}
	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); err != nil {
		t.Fatal(err)
	}
	waitForGRPCSessionCount(t, alice, 2)
	active := alice.ActivePeerIDs()
	if len(active) != 2 || active[0] != "bob" || active[1] != "carol" {
		t.Fatalf("unexpected active peers: %v", active)
	}
}

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

func TestGRPCBinaryCodecRoundTrip(t *testing.T) {
	codec := grpcBinaryCodec{}
	original := &grpcStreamMessage{
		Handshake: &Handshake{
			ProtocolVersion: GRPCProtocolVersion,
			NetworkID:       "vexo-network",
			ChainID:         "vexo-test",
			GenesisHash:     GenesisHash([]byte("genesis")),
			NodeID:          "alice",
			ListenAddr:      "127.0.0.1:26656",
			AuthToken:       "shared-secret",
			SignatureNonce:  "nonce-1",
			NodePublicKey:   types.PublicKey("public-key"),
			Signature:       types.Signature("signature"),
			KnownPeers:      map[p2p.PeerID]string{"bob": "127.0.0.1:26657"},
		},
		Envelope: &grpcEnvelope{
			Topic: p2p.TopicVote,
			From:  "alice",
			To:    "bob",
			Data:  []byte("vote"),
		},
	}
	data, err := codec.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded grpcStreamMessage
	if err := codec.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Handshake == nil || decoded.Envelope == nil {
		t.Fatalf("expected decoded handshake and envelope: %+v", decoded)
	}
	if decoded.Handshake.NodeID != original.Handshake.NodeID || decoded.Handshake.ChainID != original.Handshake.ChainID {
		t.Fatalf("unexpected decoded handshake: %+v", decoded.Handshake)
	}
	if decoded.Handshake.AuthToken != original.Handshake.AuthToken {
		t.Fatalf("unexpected decoded auth token: %+v", decoded.Handshake)
	}
	if decoded.Handshake.SignatureNonce != original.Handshake.SignatureNonce || string(decoded.Handshake.NodePublicKey) != string(original.Handshake.NodePublicKey) || string(decoded.Handshake.Signature) != string(original.Handshake.Signature) {
		t.Fatalf("unexpected decoded handshake signature fields: %+v", decoded.Handshake)
	}
	if decoded.Handshake.KnownPeers["bob"] != "127.0.0.1:26657" {
		t.Fatalf("unexpected decoded known peers: %+v", decoded.Handshake.KnownPeers)
	}
	if decoded.Envelope.Topic != original.Envelope.Topic || decoded.Envelope.From != original.Envelope.From || decoded.Envelope.To != original.Envelope.To || string(decoded.Envelope.Data) != string(original.Envelope.Data) {
		t.Fatalf("unexpected decoded envelope: %+v", decoded.Envelope)
	}
}

func TestGRPCBinaryCodecDecodesLegacyV1Handshake(t *testing.T) {
	var buffer bytes.Buffer
	buffer.WriteByte(grpcCodecVersionV1)
	buffer.WriteByte(1)
	writeBinaryString(&buffer, GRPCProtocolVersion)
	writeBinaryString(&buffer, "vexo-network")
	writeBinaryString(&buffer, "vexo-test")
	writeBinaryString(&buffer, GenesisHash([]byte("genesis")))
	writeBinaryString(&buffer, "alice")
	writeBinaryString(&buffer, "127.0.0.1:26656")
	writeBinaryString(&buffer, "shared-secret")
	writeBinaryPeerMap(&buffer, map[p2p.PeerID]string{"bob": "127.0.0.1:26666"})
	buffer.WriteByte(0)

	decoded, err := decodeGRPCStreamMessage(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Handshake == nil {
		t.Fatalf("expected legacy handshake")
	}
	if decoded.Handshake.NodeID != "alice" || decoded.Handshake.KnownPeers["bob"] != "127.0.0.1:26666" {
		t.Fatalf("unexpected decoded legacy handshake: %+v", decoded.Handshake)
	}
	if decoded.Handshake.SignatureNonce != "" || len(decoded.Handshake.NodePublicKey) != 0 || len(decoded.Handshake.Signature) != 0 {
		t.Fatalf("legacy v1 handshake must not synthesize signature fields: %+v", decoded.Handshake)
	}
}

func TestGRPCTransportPeerLearnedHook(t *testing.T) {
	learned := make(map[p2p.PeerID]string)
	transport, err := NewGRPCTransport(GRPCConfig{
		PeerID:      "alice",
		ListenAddr:  "127.0.0.1:0",
		NetworkID:   "vexo-network",
		ChainID:     "vexo-test",
		GenesisHash: GenesisHash([]byte("genesis")),
		PeerLearned: func(peerID p2p.PeerID, address string) {
			learned[peerID] = address
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	transport.learnHandshakePeers(Handshake{
		NodeID:     "bob",
		ListenAddr: "127.0.0.1:26666",
		KnownPeers: map[p2p.PeerID]string{
			"carol": "127.0.0.1:26676",
			"alice": "127.0.0.1:26656",
		},
	})

	if learned["bob"] != "127.0.0.1:26666" || learned["carol"] != "127.0.0.1:26676" {
		t.Fatalf("expected learned peers, got %+v", learned)
	}
	if _, found := learned["alice"]; found {
		t.Fatalf("did not expect self peer to be learned: %+v", learned)
	}
}

func TestGRPCTransportIgnoresInvalidDiscoveredPeerAddresses(t *testing.T) {
	learned := make(map[p2p.PeerID]string)
	transport, err := NewGRPCTransport(GRPCConfig{
		PeerID:      "alice",
		ListenAddr:  "127.0.0.1:0",
		NetworkID:   "vexo-network",
		ChainID:     "vexo-test",
		GenesisHash: GenesisHash([]byte("genesis")),
		PeerLearned: func(peerID p2p.PeerID, address string) {
			learned[peerID] = address
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	transport.learnHandshakePeers(Handshake{
		NodeID:     "bob",
		ListenAddr: "0.0.0.0:26666",
		KnownPeers: map[p2p.PeerID]string{
			"carol": "validator-3.example.com:26676",
			"dave":  "bad/address:26686",
		},
	})

	if _, found := learned["bob"]; found {
		t.Fatalf("did not expect unspecified bind address to be learned: %+v", learned)
	}
	if _, found := learned["dave"]; found {
		t.Fatalf("did not expect malformed address to be learned: %+v", learned)
	}
	if learned["carol"] != "validator-3.example.com:26676" {
		t.Fatalf("expected valid discovered hostname, got %+v", learned)
	}
}

func TestGRPCTransportPeerDialHooks(t *testing.T) {
	attempts := make(map[p2p.PeerID]int)
	results := make(map[p2p.PeerID]bool)
	transport, err := NewGRPCTransport(GRPCConfig{
		PeerID:            "alice",
		ListenAddr:        "127.0.0.1:0",
		Peers:             map[p2p.PeerID]string{"bob": "127.0.0.1:1"},
		NetworkID:         "vexo-network",
		ChainID:           "vexo-test",
		GenesisHash:       GenesisHash([]byte("genesis")),
		DialTimeout:       time.Millisecond,
		ReconnectInterval: time.Hour,
		PeerAttempted: func(peerID p2p.PeerID) {
			attempts[peerID]++
		},
		PeerDialResult: func(peerID p2p.PeerID, success bool) {
			results[peerID] = success
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	transport.reconnectConfiguredPeers(context.Background())
	if attempts["bob"] != 1 {
		t.Fatalf("expected bob attempt, got %+v", attempts)
	}
	if results["bob"] {
		t.Fatalf("expected failed dial result, got %+v", results)
	}
}

func TestGRPCTransportPeerGateRejectsOutboundSend(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{
		PeerGate: func(ctx context.Context, peerID p2p.PeerID) error {
			if peerID == "bob" {
				return p2p.ErrPeerBanned
			}
			return nil
		},
	})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("blocked")); !errors.Is(err, ErrPeerRejected) {
		t.Fatalf("expected peer rejected, got %v", err)
	}
	if sessions := grpcSessionCount(alice); sessions != 0 {
		t.Fatalf("expected rejected send to avoid session creation, got %d sessions", sessions)
	}
}

func TestGRPCTransportPeerGateRejectsInboundHandshake(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{
		PeerGate: func(ctx context.Context, peerID p2p.PeerID) error {
			if peerID == "alice" {
				return p2p.ErrPeerBanned
			}
			return nil
		},
	})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("blocked")); !errors.Is(err, ErrPeerRejected) {
		t.Fatalf("expected peer rejected, got %v", err)
	}
}

func TestGRPCTransportChainsPeerGates(t *testing.T) {
	peer, err := NewGRPCTransport(GRPCConfig{
		PeerID: "alice",
		PeerGate: func(ctx context.Context, peerID p2p.PeerID) error {
			if peerID == "bob" {
				return p2p.ErrPeerBanned
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	peer.AddPeerGate(func(ctx context.Context, peerID p2p.PeerID) error {
		if peerID == "carol" {
			return p2p.ErrPeerBanned
		}
		return nil
	})
	if err := peer.checkPeerGate(context.Background(), "bob"); !errors.Is(err, ErrPeerRejected) {
		t.Fatalf("expected bob rejected, got %v", err)
	}
	if err := peer.checkPeerGate(context.Background(), "carol"); !errors.Is(err, ErrPeerRejected) {
		t.Fatalf("expected carol rejected, got %v", err)
	}
	if err := peer.checkPeerGate(context.Background(), "dave"); err != nil {
		t.Fatalf("expected dave accepted, got %v", err)
	}
}

func TestGRPCTransportRemovePeerClosesSessionAndConnection(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("before-remove")); err != nil {
		t.Fatal(err)
	}
	assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, "before-remove")
	alice.RemovePeer("bob")
	if _, found := alice.KnownPeers()["bob"]; found {
		t.Fatalf("expected bob removed from peers: %+v", alice.KnownPeers())
	}
	if sessions := grpcSessionCount(alice); sessions != 0 {
		t.Fatalf("expected sessions closed, got %d", sessions)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("after-remove")); !errors.Is(err, ErrPeerNotFound) {
		t.Fatalf("expected peer not found after remove, got %v", err)
	}
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

	var firstSession *grpcPeerSession
	for index, payload := range []string{"one", "two", "three"} {
		if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte(payload)); err != nil {
			t.Fatal(err)
		}
		assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, payload)
		if index == 0 {
			firstSession = grpcSessionFor(alice, "bob")
			if firstSession == nil {
				t.Fatal("expected bob grpc session to be cached")
			}
			continue
		}
		if session := grpcSessionFor(alice, "bob"); session != firstSession {
			t.Fatalf("expected bob grpc stream session to be reused")
		}
	}
}

func TestGRPCTransportStartContextCancelDoesNotStopPeerSessions(t *testing.T) {
	alice, err := NewGRPCTransport(GRPCConfig{PeerID: "alice", ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{})
	defer stopGRPCPeer(t, bob)
	startCtx, cancelStart := context.WithCancel(context.Background())
	if err := alice.Start(startCtx); err != nil {
		t.Fatal(err)
	}
	defer stopGRPCPeer(t, alice)
	cancelStart()
	alice.SetPeer("bob", bob.Address())
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("after-start-cancel")); err != nil {
		t.Fatalf("transport lifetime must not be tied to start context: %v", err)
	}
	assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, "after-start-cancel")
}

func TestGRPCTransportRejectsOversizedMessage(t *testing.T) {
	config := GRPCConfig{
		NetworkID:       "vexo-network",
		ChainID:         "vexo-test",
		GenesisHash:     GenesisHash([]byte("genesis")),
		MaxMessageBytes: 4,
	}
	alice := newStartedGRPCPeer(t, "alice", config)
	bob := newStartedGRPCPeer(t, "bob", config)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("12345"))
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected message too large, got %v", err)
	}
	if sessions := grpcSessionCount(alice); sessions != 0 {
		t.Fatalf("expected oversized message to avoid session creation, got %d sessions", sessions)
	}
}

func TestGRPCTransportSupportsMutualTLS(t *testing.T) {
	caCert, caKey, certPool := newTestCertificateAuthority(t)
	config := GRPCConfig{
		NetworkID:   "vexo-network",
		ChainID:     "vexo-test",
		GenesisHash: GenesisHash([]byte("genesis")),
		DialTimeout: 30 * time.Second,
	}
	aliceConfig := config
	aliceConfig.TLSConfig = newTestPeerTLSConfig(t, caCert, caKey, certPool, "alice")
	bobConfig := config
	bobConfig.TLSConfig = newTestPeerTLSConfig(t, caCert, caKey, certPool, "bob")
	alice := newStartedGRPCPeer(t, "alice", aliceConfig)
	bob := newStartedGRPCPeer(t, "bob", bobConfig)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)

	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	alice.SetPeer("bob", bob.Address())
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("secure")); err != nil {
		t.Fatal(err)
	}
	assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, "secure")
}

func TestGRPCTransportRejectsUntrustedClientCertificate(t *testing.T) {
	trustedCACert, trustedCAKey, trustedPool := newTestCertificateAuthority(t)
	untrustedCACert, untrustedCAKey, _ := newTestCertificateAuthority(t)
	config := GRPCConfig{
		NetworkID:   "vexo-network",
		ChainID:     "vexo-test",
		GenesisHash: GenesisHash([]byte("genesis")),
		DialTimeout: 30 * time.Second,
	}
	aliceConfig := config
	aliceConfig.TLSConfig = newTestPeerTLSConfig(t, untrustedCACert, untrustedCAKey, trustedPool, "alice")
	bobConfig := config
	bobConfig.TLSConfig = newTestPeerTLSConfig(t, trustedCACert, trustedCAKey, trustedPool, "bob")
	alice := newStartedGRPCPeer(t, "alice", aliceConfig)
	bob := newStartedGRPCPeer(t, "bob", bobConfig)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("blocked")); err == nil {
		t.Fatal("expected untrusted client certificate to fail")
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

func TestGRPCTransportRejectsAuthTokenMismatch(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{AuthToken: "alice-token"})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{AuthToken: "bob-token"})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	alice.SetPeer("bob", bob.Address())

	err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx"))
	if !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected auth token mismatch, got %v", err)
	}
}

func TestGRPCTransportRejectsReplayedAuthProof(t *testing.T) {
	transport, err := NewGRPCTransport(GRPCConfig{
		PeerID:      "alice",
		AuthToken:   "shared-secret",
		NetworkID:   "vexo-network",
		ChainID:     "vexo-test",
		GenesisHash: GenesisHash([]byte("genesis")),
	})
	if err != nil {
		t.Fatal(err)
	}
	handshake := Handshake{
		ProtocolVersion: GRPCProtocolVersion,
		NetworkID:       "vexo-network",
		ChainID:         "vexo-test",
		GenesisHash:     GenesisHash([]byte("genesis")),
		NodeID:          "bob",
		AuthToken:       transport.authProof("bob"),
	}
	if err := transport.validateHandshake(context.Background(), handshake); err != nil {
		t.Fatalf("expected first proof to pass, got %v", err)
	}
	if err := transport.validateHandshake(context.Background(), handshake); !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected replayed proof rejection, got %v", err)
	}
}

func TestGRPCTransportRejectsReplayedAuthProofAfterRestart(t *testing.T) {
	path := t.TempDir() + "/auth-replay.jsonl"
	firstStore, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewGRPCTransport(GRPCConfig{
		PeerID:          "alice",
		AuthToken:       "shared-secret",
		AuthReplayStore: firstStore,
		NetworkID:       "vexo-network",
		ChainID:         "vexo-test",
		GenesisHash:     GenesisHash([]byte("genesis")),
	})
	if err != nil {
		t.Fatal(err)
	}
	handshake := Handshake{
		ProtocolVersion: GRPCProtocolVersion,
		NetworkID:       "vexo-network",
		ChainID:         "vexo-test",
		GenesisHash:     GenesisHash([]byte("genesis")),
		NodeID:          "bob",
		AuthToken:       first.authProof("bob"),
	}
	if err := first.validateHandshake(context.Background(), handshake); err != nil {
		t.Fatalf("expected first proof to pass, got %v", err)
	}
	secondStore, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewGRPCTransport(GRPCConfig{
		PeerID:          "alice",
		AuthToken:       "shared-secret",
		AuthReplayStore: secondStore,
		NetworkID:       "vexo-network",
		ChainID:         "vexo-test",
		GenesisHash:     GenesisHash([]byte("genesis")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := second.validateHandshake(context.Background(), handshake); !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected replay rejection after restart, got %v", err)
	}
}

func TestGRPCTransportCanRequireDurableAuthReplayStore(t *testing.T) {
	_, err := NewGRPCTransport(GRPCConfig{
		PeerID:                 "alice",
		AuthToken:              "shared-secret",
		RequireAuthReplayStore: true,
	})
	if !errors.Is(err, ErrAuthReplayStore) {
		t.Fatalf("expected auth replay store requirement, got %v", err)
	}
}

func TestFileAuthReplayStoreCompactsExpiredRecords(t *testing.T) {
	path := t.TempDir() + "/auth-replay.jsonl"
	store, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := store.MarkAuthNonce(context.Background(), "alice", "old", now.Add(time.Second), now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAuthNonce(context.Background(), "alice", "fresh", now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := store.Compact(context.Background(), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	compacted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(compacted), "old") || !strings.Contains(string(compacted), "fresh") {
		t.Fatalf("expected expired nonce to be compacted away, got %s", compacted)
	}
}

func TestGRPCBinaryCodecRejectsOversizedLength(t *testing.T) {
	var input bytes.Buffer
	input.WriteByte(grpcCodecVersion)
	input.WriteByte(0)
	input.WriteByte(1)
	var length [4]byte
	length[0] = 0xff
	length[1] = 0xff
	length[2] = 0xff
	length[3] = 0xff
	input.Write(length[:])
	if _, err := decodeGRPCStreamMessage(input.Bytes()); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected oversized codec error, got %v", err)
	}
}

func TestGRPCTransportValidatesSignedHandshake(t *testing.T) {
	alicePublicKey, alicePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bobPublicKey, bobPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	aliceSigner := testHandshakeSigner{publicKey: alicePublicKey, privateKey: alicePrivateKey}
	bobSigner := testHandshakeSigner{publicKey: bobPublicKey, privateKey: bobPrivateKey}
	verifier := testHandshakeVerifier{}
	alice, err := NewGRPCTransport(GRPCConfig{
		PeerID:                    "alice",
		NetworkID:                 "vexo-network",
		ChainID:                   "vexo-test",
		GenesisHash:               GenesisHash([]byte("genesis")),
		HandshakeSigner:           aliceSigner,
		HandshakeVerifier:         verifier,
		RequireHandshakeSignature: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := NewGRPCTransport(GRPCConfig{
		PeerID:                    "bob",
		NetworkID:                 "vexo-network",
		ChainID:                   "vexo-test",
		GenesisHash:               GenesisHash([]byte("genesis")),
		HandshakeSigner:           bobSigner,
		HandshakeVerifier:         verifier,
		RequireHandshakeSignature: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handshake := alice.LocalHandshake()
	handshake.NodeID = "alice"
	if err := bob.validateHandshake(context.Background(), handshake); err != nil {
		t.Fatalf("expected signed handshake to verify, got %v", err)
	}
	replayed := alice.LocalHandshake()
	replayed.SignatureNonce = handshake.SignatureNonce
	replayed.Signature = handshake.Signature
	if err := bob.validateHandshake(context.Background(), replayed); !errors.Is(err, ErrHandshakeSignature) {
		t.Fatalf("expected replayed signed handshake rejection, got %v", err)
	}
	handshake = alice.LocalHandshake()
	handshake.ListenAddr = "127.0.0.1:9999"
	if err := bob.validateHandshake(context.Background(), handshake); !errors.Is(err, ErrHandshakeSignature) {
		t.Fatalf("expected tampered signed handshake rejection, got %v", err)
	}
}

func TestGRPCTransportRequiresSignerForRequiredHandshakeSignature(t *testing.T) {
	if _, err := NewGRPCTransport(GRPCConfig{
		PeerID:                    "alice",
		HandshakeVerifier:         testHandshakeVerifier{},
		RequireHandshakeSignature: true,
	}); !errors.Is(err, ErrHandshakeSignature) {
		t.Fatalf("expected missing signer to fail strict handshake config, got %v", err)
	}
}

type testHandshakeSigner struct {
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

func (signer testHandshakeSigner) PublicKey() types.PublicKey {
	return types.PublicKey(append([]byte(nil), signer.publicKey...))
}

func (signer testHandshakeSigner) Sign(message []byte) (types.Signature, error) {
	return types.Signature(ed25519.Sign(signer.privateKey, message)), nil
}

type testHandshakeVerifier struct{}

func (testHandshakeVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}

func TestGRPCTransportReconnectLoopEstablishesPeerSession(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{
		ReconnectInterval: 10 * time.Duration(1_000_000),
		DialTimeout:       100 * time.Duration(1_000_000),
	})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)

	alice.SetPeer("bob", bob.Address())
	waitForGRPCSessionCount(t, alice, 1)
}

func TestGRPCTransportKeepsSessionAfterHandshakeDeadline(t *testing.T) {
	dialTimeout := 50 * time.Millisecond
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{
		ReconnectInterval: 10 * time.Millisecond,
		DialTimeout:       dialTimeout,
	})
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)

	alice.SetPeer("bob", bob.Address())
	waitForGRPCSessionCount(t, alice, 1)
	time.Sleep(3 * dialTimeout)
	if active := alice.ActivePeerIDs(); len(active) != 1 || active[0] != "bob" {
		t.Fatalf("successful session was canceled after handshake deadline: %v", active)
	}
}

func TestGRPCTransportEstablishesPreconfiguredAuthenticatedFullMesh(t *testing.T) {
	const peerCount = 4
	addresses := make([]string, peerCount)
	reservations := make([]net.Listener, 0, peerCount)
	for index := range addresses {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		reservations = append(reservations, listener)
		addresses[index] = listener.Addr().String()
	}
	for _, listener := range reservations {
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}

	peers := make([]*GRPCTransport, 0, peerCount)
	for index := range addresses {
		peerID := p2p.PeerID(fmt.Sprintf("validator-%d", index+1))
		configuredPeers := make(map[p2p.PeerID]string, peerCount-1)
		for remoteIndex, address := range addresses {
			if remoteIndex == index {
				continue
			}
			configuredPeers[p2p.PeerID(fmt.Sprintf("validator-%d", remoteIndex+1))] = address
		}
		peerHome := t.TempDir()
		addressBook, err := p2p.OpenAddrBookWithPolicy(filepath.Join(peerHome, "addrbook.json"), 3)
		if err != nil {
			t.Fatal(err)
		}
		addressBook.Merge(configuredPeers, "cli-peer", true)
		if err := addressBook.Save(); err != nil {
			t.Fatal(err)
		}
		replayStore, err := NewFileAuthReplayStore(filepath.Join(peerHome, "auth-replay.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		scoreKeeper := p2p.NewScoreKeeper(p2p.ScoreConfig{InitialScore: 100, MaxScore: 1000, BanThreshold: 0})
		peer, err := NewGRPCTransport(GRPCConfig{
			PeerID:            peerID,
			ListenAddr:        addresses[index],
			Peers:             configuredPeers,
			DialTimeout:       500 * time.Millisecond,
			ReconnectInterval: 10 * time.Millisecond,
			NetworkID:         "vexo-network",
			ChainID:           "vexo-test",
			GenesisHash:       GenesisHash([]byte("genesis")),
			AuthToken:         "shared-token",
			AuthReplayStore:   replayStore,
			PeerLearned: func(remoteID p2p.PeerID, address string) {
				addressBook.Add(remoteID, address, "handshake", false)
				_ = addressBook.Save()
			},
			PeerAttempted: func(remoteID p2p.PeerID) {
				addressBook.MarkAttempt(remoteID)
				_ = addressBook.Save()
			},
			PeerDialResult: func(remoteID p2p.PeerID, success bool) {
				if success {
					addressBook.MarkSuccess(remoteID)
				} else {
					addressBook.MarkFailure(remoteID, time.Minute)
				}
				_ = addressBook.Save()
			},
			PeerGate: func(ctx context.Context, remoteID p2p.PeerID) error {
				if addressBook.IsBanned(remoteID) {
					return p2p.ErrPeerBanned
				}
				banned, err := scoreKeeper.IsBanned(ctx, remoteID)
				if err != nil {
					return err
				}
				if banned {
					return p2p.ErrPeerBanned
				}
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		peers = append(peers, peer)
	}
	for _, peer := range peers {
		if err := peer.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		peer := peer
		t.Cleanup(func() { stopGRPCPeer(t, peer) })
	}
	for _, peer := range peers {
		waitForGRPCSessionCount(t, peer, peerCount-1)
	}
}

func TestGRPCTransportDiscoversPeersFromHandshake(t *testing.T) {
	base := GRPCConfig{
		NetworkID:         "vexo-network",
		ChainID:           "vexo-test",
		GenesisHash:       GenesisHash([]byte("genesis")),
		ReconnectInterval: 10 * time.Millisecond,
		DialTimeout:       250 * time.Millisecond,
	}
	alice := newStartedGRPCPeer(t, "alice", base)
	bob := newStartedGRPCPeer(t, "bob", base)
	carol := newStartedGRPCPeer(t, "carol", base)
	defer stopGRPCPeer(t, alice)
	defer stopGRPCPeer(t, bob)
	defer stopGRPCPeer(t, carol)

	bob.SetPeer("carol", carol.Address())
	alice.SetPeer("bob", bob.Address())
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("bootstrap")); err != nil {
		t.Fatal(err)
	}
	assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, "bootstrap")
	waitForKnownPeer(t, alice, "carol")

	carolTxs, err := carol.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Send(context.Background(), "carol", p2p.TopicTx, []byte("discovered")); err != nil {
		t.Fatal(err)
	}
	assertEnvelope(t, carolTxs, "alice", "carol", p2p.TopicTx, "discovered")
}

func TestGRPCTransportReconnectsAfterPeerRestart(t *testing.T) {
	base := GRPCConfig{
		NetworkID:         "vexo-network",
		ChainID:           "vexo-test",
		GenesisHash:       GenesisHash([]byte("genesis")),
		ReconnectInterval: 10 * time.Millisecond,
		ReconnectBackoff:  20 * time.Millisecond,
		DialTimeout:       100 * time.Millisecond,
	}
	alice := newStartedGRPCPeer(t, "alice", base)
	bob := newStartedGRPCPeer(t, "bob", base)
	defer stopGRPCPeer(t, alice)
	alice.SetPeer("bob", bob.Address())
	waitForGRPCSessionCount(t, alice, 1)
	bobAddress := bob.Address()

	stopGRPCPeer(t, bob)
	waitForGRPCSessionCount(t, alice, 0)
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("during-restart")); err == nil {
		t.Fatal("expected send to stopped peer to fail")
	}

	restartedConfig := base
	restartedConfig.ListenAddr = bobAddress
	bob = newStartedGRPCPeer(t, "bob", restartedConfig)
	defer stopGRPCPeer(t, bob)
	waitForGRPCSessionCount(t, alice, 1)

	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("after-restart")); err != nil {
		t.Fatal(err)
	}
	assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, "after-restart")
}

func TestGRPCTransportValidationAndContext(t *testing.T) {
	if _, err := NewGRPCTransport(GRPCConfig{}); !errors.Is(err, ErrPeerIDRequired) {
		t.Fatalf("expected peer id required, got %v", err)
	}
	if _, err := NewGRPCTransport(GRPCConfig{PeerID: "alice", RequireTLS: true}); !errors.Is(err, ErrTLSRequired) {
		t.Fatalf("expected tls required, got %v", err)
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

func TestNewNetworkSafeGRPCTransportFailsClosed(t *testing.T) {
	if _, err := NewNetworkSafeGRPCTransport(GRPCConfig{PeerID: "alice"}); !errors.Is(err, ErrTLSRequired) {
		t.Fatalf("expected tls requirement, got %v", err)
	}
	caCert, caKey, certPool := newTestCertificateAuthority(t)
	tlsConfig := newTestPeerTLSConfig(t, caCert, caKey, certPool, "alice")
	if _, err := NewNetworkSafeGRPCTransport(GRPCConfig{PeerID: "alice", TLSConfig: tlsConfig, RequireTLS: true}); !errors.Is(err, ErrAuthTokenRequired) {
		t.Fatalf("expected auth token requirement, got %v", err)
	}
	replayStore, err := NewFileAuthReplayStore(filepath.Join(t.TempDir(), "auth-replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := testHandshakeSigner{publicKey: publicKey, privateKey: privateKey}
	transport, err := NewNetworkSafeGRPCTransport(GRPCConfig{
		PeerID:                    "alice",
		TLSConfig:                 tlsConfig,
		RequireTLS:                true,
		AuthToken:                 "shared-token",
		AuthReplayStore:           replayStore,
		RequireAuthReplayStore:    true,
		HandshakeSigner:           signer,
		HandshakeVerifier:         testHandshakeVerifier{},
		RequireHandshakeSignature: true,
	})
	if err != nil {
		t.Fatalf("expected complete network-safe config to pass, got %v", err)
	}
	if transport.transportCredentials() == nil {
		t.Fatal("expected tls transport credentials")
	}
}

func TestGRPCTransportRejectsUnknownPeer(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	if err := alice.Send(context.Background(), "unknown", p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrPeerNotFound) {
		t.Fatalf("expected peer not found, got %v", err)
	}
}

func TestGRPCTransportAppliesReconnectBackoff(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{
		DialTimeout:      25 * time.Millisecond,
		ReconnectBackoff: time.Minute,
	})
	defer stopGRPCPeer(t, alice)
	alice.SetPeer("bob", "127.0.0.1:1")

	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx")); err == nil {
		t.Fatal("expected first send to unavailable peer to fail")
	}
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrPeerBackoffActive) {
		t.Fatalf("expected reconnect backoff, got %v", err)
	}
}

func TestGRPCTransportEvictsOldestPeerWhenLimitExceeded(t *testing.T) {
	peer, err := NewGRPCTransport(GRPCConfig{PeerID: "alice", MaxPeers: 2})
	if err != nil {
		t.Fatal(err)
	}
	peer.SetPeer("bob", "127.0.0.1:10001")
	peer.SetPeer("carol", "127.0.0.1:10002")
	peer.SetPeer("dave", "127.0.0.1:10003")

	if _, ok := peer.peerAddress("bob"); ok {
		t.Fatal("expected oldest peer bob to be evicted")
	}
	if _, ok := peer.peerAddress("carol"); !ok {
		t.Fatal("expected carol to remain")
	}
	if _, ok := peer.peerAddress("dave"); !ok {
		t.Fatal("expected dave to be added")
	}
}

func TestGRPCTransportCountsSubscriberDrops(t *testing.T) {
	peer := newStartedGRPCPeer(t, "alice", GRPCConfig{SubscriberBuffer: 1})
	defer stopGRPCPeer(t, peer)
	if _, err := peer.Subscribe(context.Background(), p2p.TopicTx); err != nil {
		t.Fatal(err)
	}

	peer.deliver(Envelope{Topic: p2p.TopicTx, From: "bob", Data: []byte("one")})
	peer.deliver(Envelope{Topic: p2p.TopicTx, From: "bob", Data: []byte("two")})
	peer.deliver(Envelope{Topic: p2p.TopicTx, From: "bob", Data: []byte("three")})

	if drops := peer.DroppedMessages(); drops == 0 {
		t.Fatal("expected subscriber drops to be counted")
	}
}

func TestGRPCTransportDoesNotDropConsensusTopics(t *testing.T) {
	peer := newStartedGRPCPeer(t, "alice", GRPCConfig{SubscriberBuffer: 1})
	defer stopGRPCPeer(t, peer)
	votes, err := peer.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for index := 0; index < 3; index++ {
			<-votes
		}
	}()

	peer.deliver(Envelope{Topic: p2p.TopicVote, From: "bob", Data: []byte("one")})
	peer.deliver(Envelope{Topic: p2p.TopicVote, From: "bob", Data: []byte("two")})
	peer.deliver(Envelope{Topic: p2p.TopicVote, From: "bob", Data: []byte("three")})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reliable consensus deliveries")
	}
	if drops := peer.DroppedMessages(); drops != 0 {
		t.Fatalf("expected no consensus drops, got %d", drops)
	}
}

func newStartedGRPCPeers(t *testing.T) (*GRPCTransport, *GRPCTransport, *GRPCTransport) {
	t.Helper()
	base := GRPCConfig{NetworkID: "vexo-network", ChainID: "vexo-test", GenesisHash: GenesisHash([]byte("genesis"))}
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

func grpcSessionFor(peer *GRPCTransport, peerID p2p.PeerID) *grpcPeerSession {
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	return peer.sessions[peerID]
}

func waitForGRPCSessionCount(t *testing.T, peer *GRPCTransport, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Duration(2_000_000_000))
	for time.Now().Before(deadline) {
		if grpcSessionCount(peer) == expected {
			return
		}
		time.Sleep(10 * time.Duration(1_000_000))
	}
	if actual := grpcSessionCount(peer); actual != expected {
		t.Fatalf("expected %d grpc sessions, got %d", expected, actual)
	}
}

func waitForKnownPeer(t *testing.T, peer *GRPCTransport, expected p2p.PeerID) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if address := peer.KnownPeers()[expected]; address != "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected peer %s to be discovered, got %+v", expected, peer.KnownPeers())
}

func newTestCertificateAuthority(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "vexo-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return cert, key, pool
}

func newTestPeerTLSConfig(t *testing.T, caCert *x509.Certificate, caKey *rsa.PrivateKey, certPool *x509.CertPool, commonName string) *tls.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		RootCAs:      certPool,
		ClientCAs:    certPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
}

package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
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

func TestGRPCBinaryCodecRoundTrip(t *testing.T) {
	codec := grpcBinaryCodec{}
	original := &grpcStreamMessage{
		Handshake: &Handshake{
			ProtocolVersion: GRPCProtocolVersion,
			NetworkID:       "localnet",
			ChainID:         "vexo-test",
			GenesisHash:     GenesisHash([]byte("genesis")),
			NodeID:          "alice",
			ListenAddr:      "127.0.0.1:26656",
			AuthToken:       "shared-secret",
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
	if decoded.Handshake.KnownPeers["bob"] != "127.0.0.1:26657" {
		t.Fatalf("unexpected decoded known peers: %+v", decoded.Handshake.KnownPeers)
	}
	if decoded.Envelope.Topic != original.Envelope.Topic || decoded.Envelope.From != original.Envelope.From || decoded.Envelope.To != original.Envelope.To || string(decoded.Envelope.Data) != string(original.Envelope.Data) {
		t.Fatalf("unexpected decoded envelope: %+v", decoded.Envelope)
	}
}

func TestGRPCTransportPeerLearnedHook(t *testing.T) {
	learned := make(map[p2p.PeerID]string)
	transport, err := NewGRPCTransport(GRPCConfig{
		PeerID:      "alice",
		ListenAddr:  "127.0.0.1:0",
		NetworkID:   "localnet",
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

func TestGRPCTransportPeerDialHooks(t *testing.T) {
	attempts := make(map[p2p.PeerID]int)
	results := make(map[p2p.PeerID]bool)
	transport, err := NewGRPCTransport(GRPCConfig{
		PeerID:            "alice",
		ListenAddr:        "127.0.0.1:0",
		Peers:             map[p2p.PeerID]string{"bob": "127.0.0.1:1"},
		NetworkID:         "localnet",
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

	firstSessionCount := 0
	for index, payload := range []string{"one", "two", "three"} {
		if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte(payload)); err != nil {
			t.Fatal(err)
		}
		assertEnvelope(t, bobTxs, "alice", "bob", p2p.TopicTx, payload)
		if index == 0 {
			firstSessionCount = grpcSessionCount(alice)
		}
	}

	if sessions := grpcSessionCount(alice); sessions != firstSessionCount {
		t.Fatalf("expected cached grpc sessions to be reused, got first=%d final=%d", firstSessionCount, sessions)
	}
}

func TestGRPCTransportRejectsOversizedMessage(t *testing.T) {
	config := GRPCConfig{
		NetworkID:       "localnet",
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
		NetworkID:   "localnet",
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
		NetworkID:   "localnet",
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

func TestGRPCTransportDiscoversPeersFromHandshake(t *testing.T) {
	base := GRPCConfig{
		NetworkID:         "localnet",
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
		NetworkID:         "localnet",
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
	if err := alice.Send(context.Background(), "bob", p2p.TopicTx, []byte("during-restart")); err == nil {
		t.Fatal("expected send to stopped peer to fail")
	}
	waitForGRPCSessionCount(t, alice, 0)

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

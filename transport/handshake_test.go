package transport

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// pipeStream is an in-memory handshakeStream. Frames cross it through the real
// wire codec, so decoding bounds apply, and may be rewritten in transit to
// model an active attacker on the path.
type pipeStream struct {
	ctx     context.Context
	in      chan *grpcStreamMessage
	out     chan *grpcStreamMessage
	rewrite func(*grpcStreamMessage) *grpcStreamMessage

	mu   sync.Mutex
	sent []*grpcStreamMessage
}

func newPipePair(ctx context.Context) (client *pipeStream, server *pipeStream) {
	toServer := make(chan *grpcStreamMessage, 8)
	toClient := make(chan *grpcStreamMessage, 8)
	return &pipeStream{ctx: ctx, in: toClient, out: toServer}, &pipeStream{ctx: ctx, in: toServer, out: toClient}
}

func (stream *pipeStream) Context() context.Context { return stream.ctx }

func (stream *pipeStream) Send(message *grpcStreamMessage) error {
	data, err := encodeGRPCStreamMessage(message)
	if err != nil {
		return err
	}
	decoded, err := decodeGRPCStreamMessage(data)
	if err != nil {
		return err
	}
	if stream.rewrite != nil {
		if decoded = stream.rewrite(decoded); decoded == nil {
			return nil
		}
	}
	stream.mu.Lock()
	stream.sent = append(stream.sent, decoded)
	stream.mu.Unlock()
	select {
	case stream.out <- decoded:
		return nil
	case <-stream.ctx.Done():
		return stream.ctx.Err()
	}
}

func (stream *pipeStream) Recv() (*grpcStreamMessage, error) {
	select {
	case message := <-stream.in:
		return message, nil
	case <-stream.ctx.Done():
		return nil, stream.ctx.Err()
	}
}

func (stream *pipeStream) sentFrames() []*grpcStreamMessage {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return append([]*grpcStreamMessage(nil), stream.sent...)
}

type handshakeOutcome struct {
	remote        Handshake
	serverHello   *Handshake
	serverLearned Handshake
	clientErr     error
	serverErr     error
}

// failure returns the error that ended the exchange: whichever side failed
// first, not the cancellation it caused on the other side.
func (outcome handshakeOutcome) failure() error {
	for _, err := range []error{outcome.serverErr, outcome.clientErr} {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func runHandshake(t *testing.T, client *GRPCTransport, server *GRPCTransport, expected p2p.PeerID, clientStream *pipeStream, serverStream *pipeStream, cancel context.CancelFunc) handshakeOutcome {
	t.Helper()
	var outcome handshakeOutcome
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		outcome.serverHello, outcome.serverLearned, outcome.serverErr = server.serveHandshake(serverStream, serverStream.Recv)
		if outcome.serverErr != nil {
			cancel()
		}
	}()
	go func() {
		defer wait.Done()
		outcome.remote, outcome.clientErr = client.dialHandshake(clientStream, expected)
		if outcome.clientErr != nil {
			cancel()
		}
	}()
	wait.Wait()
	return outcome
}

func handshakeContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx, cancel
}

func testBaseConfig() GRPCConfig {
	return GRPCConfig{NetworkID: "vexo-network", ChainID: "vexo-test", GenesisHash: GenesisHash([]byte("genesis"))}
}

func newTestHandshakeSigner(t *testing.T) testHandshakeSigner {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return testHandshakeSigner{publicKey: publicKey, privateKey: privateKey}
}

// newSignedTransport builds a key-bound transport: its peer id is derived from
// a fresh signing key and it requires the same of its peers.
func newSignedTransport(t *testing.T, config GRPCConfig) *GRPCTransport {
	t.Helper()
	signer := newTestHandshakeSigner(t)
	config.PeerID = p2p.NodeIDFromPublicKey(signer.PublicKey())
	config.HandshakeSigner = signer
	config.HandshakeVerifier = testHandshakeVerifier{}
	config.RequireHandshakeSignature = true
	if config.NetworkID == "" {
		base := testBaseConfig()
		config.NetworkID, config.ChainID, config.GenesisHash = base.NetworkID, base.ChainID, base.GenesisHash
	}
	transport, err := NewGRPCTransport(config)
	if err != nil {
		t.Fatal(err)
	}
	return transport
}

func newUnstartedTransport(t *testing.T, peerID p2p.PeerID, config GRPCConfig) *GRPCTransport {
	t.Helper()
	config.PeerID = peerID
	base := testBaseConfig()
	if config.NetworkID == "" {
		config.NetworkID, config.ChainID, config.GenesisHash = base.NetworkID, base.ChainID, base.GenesisHash
	}
	transport, err := NewGRPCTransport(config)
	if err != nil {
		t.Fatal(err)
	}
	return transport
}

func TestHandshakeAuthenticatesBothPeersWithTokenAndSignatures(t *testing.T) {
	store, err := NewFileAuthReplayStore(filepath.Join(t.TempDir(), "replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	config := GRPCConfig{AuthToken: "shared-token", AuthReplayStore: store}
	client := newSignedTransport(t, config)
	server := newSignedTransport(t, config)
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)

	outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel)
	if err := outcome.failure(); err != nil {
		t.Fatalf("expected authenticated handshake, got %v", err)
	}
	if outcome.remote.NodeID != server.peerID || outcome.serverLearned.NodeID != client.peerID {
		t.Fatalf("handshake authenticated the wrong identities: remote=%s learned=%s", outcome.remote.NodeID, outcome.serverLearned.NodeID)
	}
}

func TestHandshakeRejectsClientClaimingVictimNodeIDWithAttackerKey(t *testing.T) {
	server := newSignedTransport(t, GRPCConfig{})
	victim := newTestHandshakeSigner(t)
	attacker := newTestHandshakeSigner(t)
	victimID := p2p.NodeIDFromPublicKey(victim.PublicKey())
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)

	var serverErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, serverErr = server.serveHandshake(serverStream, serverStream.Recv)
		cancel()
	}()
	challenge, err := randomChallenge()
	if err != nil {
		t.Fatal(err)
	}
	hello := Handshake{
		ProtocolVersion: server.protocolVersion,
		NetworkID:       server.networkID,
		ChainID:         server.chainID,
		GenesisHash:     server.genesisHash,
		NodeID:          victimID,
		NodePublicKey:   attacker.PublicKey(),
		Challenge:       challenge,
	}
	if err := clientStream.Send(&grpcStreamMessage{Handshake: &hello}); err != nil {
		t.Fatal(err)
	}
	<-done
	if !errors.Is(serverErr, ErrNodeIDMismatch) {
		t.Fatalf("expected spoofed node id to be rejected as a key mismatch, got %v", serverErr)
	}
	if frames := serverStream.sentFrames(); len(frames) != 0 {
		t.Fatalf("server must not answer or sign for a spoofed identity, sent %d frames", len(frames))
	}
}

func TestHandshakeRejectsClientPresentingVictimKeyWithoutPossession(t *testing.T) {
	server := newSignedTransport(t, GRPCConfig{})
	victim := newTestHandshakeSigner(t)
	attacker := newTestHandshakeSigner(t)
	victimID := p2p.NodeIDFromPublicKey(victim.PublicKey())
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)

	var serverErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, serverErr = server.serveHandshake(serverStream, serverStream.Recv)
		cancel()
	}()
	challenge, err := randomChallenge()
	if err != nil {
		t.Fatal(err)
	}
	hello := Handshake{
		ProtocolVersion: server.protocolVersion,
		NetworkID:       server.networkID,
		ChainID:         server.chainID,
		GenesisHash:     server.genesisHash,
		NodeID:          victimID,
		NodePublicKey:   victim.PublicKey(),
		Challenge:       challenge,
	}
	if err := clientStream.Send(&grpcStreamMessage{Handshake: &hello}); err != nil {
		t.Fatal(err)
	}
	reply, err := clientStream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	proof := Handshake{NodeID: victimID}
	transcript := handshakeTranscript(handshakeRoleClient, nil, &hello, reply.Handshake, proof.KnownPeers)
	signature, err := attacker.Sign(transcript)
	if err != nil {
		t.Fatal(err)
	}
	proof.Signature = signature
	if err := clientStream.Send(&grpcStreamMessage{Handshake: &proof}); err != nil {
		t.Fatal(err)
	}
	<-done
	if !errors.Is(serverErr, ErrHandshakeSignature) {
		t.Fatalf("expected a signature made without the victim key to fail, got %v", serverErr)
	}
}

func TestHandshakeRejectsServerImpersonatingConfiguredPeer(t *testing.T) {
	client := newSignedTransport(t, GRPCConfig{})
	victim := newTestHandshakeSigner(t)
	victimID := p2p.NodeIDFromPublicKey(victim.PublicKey())

	t.Run("attacker answers with its own identity", func(t *testing.T) {
		attacker := newSignedTransport(t, GRPCConfig{})
		ctx, cancel := handshakeContext(t)
		clientStream, serverStream := newPipePair(ctx)
		outcome := runHandshake(t, client, attacker, victimID, clientStream, serverStream, cancel)
		if !errors.Is(outcome.clientErr, ErrHandshakeFailed) {
			t.Fatalf("expected client to refuse a different node id than configured, got %v", outcome.clientErr)
		}
	})
	t.Run("attacker claims the victim id with its own key", func(t *testing.T) {
		attacker := newSignedTransport(t, GRPCConfig{})
		ctx, cancel := handshakeContext(t)
		clientStream, serverStream := newPipePair(ctx)
		serverStream.rewrite = func(message *grpcStreamMessage) *grpcStreamMessage {
			if message.Handshake != nil {
				message.Handshake.NodeID = victimID
			}
			return message
		}
		outcome := runHandshake(t, client, attacker, victimID, clientStream, serverStream, cancel)
		if !errors.Is(outcome.clientErr, ErrNodeIDMismatch) {
			t.Fatalf("expected client to reject a node id not derived from the presented key, got %v", outcome.clientErr)
		}
	})
	t.Run("attacker presents the victim key without possessing it", func(t *testing.T) {
		attacker := newSignedTransport(t, GRPCConfig{})
		ctx, cancel := handshakeContext(t)
		clientStream, serverStream := newPipePair(ctx)
		serverStream.rewrite = func(message *grpcStreamMessage) *grpcStreamMessage {
			if message.Handshake != nil {
				message.Handshake.NodeID = victimID
				message.Handshake.NodePublicKey = victim.PublicKey()
			}
			return message
		}
		outcome := runHandshake(t, client, attacker, victimID, clientStream, serverStream, cancel)
		if !errors.Is(outcome.clientErr, ErrHandshakeSignature) {
			t.Fatalf("expected client to reject a signature not made by the victim key, got %v", outcome.clientErr)
		}
	})
}

func TestHandshakeRejectsReplayedClientHandshake(t *testing.T) {
	client := newSignedTransport(t, GRPCConfig{})
	server := newSignedTransport(t, GRPCConfig{})
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)
	if outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel); outcome.failure() != nil {
		t.Fatalf("expected recorded handshake to succeed, got %v", outcome.failure())
	}
	recorded := clientStream.sentFrames()
	if len(recorded) != 2 {
		t.Fatalf("expected hello and proof frames, got %d", len(recorded))
	}

	// An attacker replays the recorded hello and proof against a new session
	// of the same server. The server picks a new challenge, so the recorded
	// proof no longer matches the transcript.
	replayCtx, replayCancel := handshakeContext(t)
	attackerStream, replayServerStream := newPipePair(replayCtx)
	var serverErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, serverErr = server.serveHandshake(replayServerStream, replayServerStream.Recv)
		replayCancel()
	}()
	if err := attackerStream.Send(recorded[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := attackerStream.Recv(); err != nil {
		t.Fatal(err)
	}
	if err := attackerStream.Send(recorded[1]); err != nil {
		t.Fatal(err)
	}
	<-done
	if !errors.Is(serverErr, ErrHandshakeSignature) {
		t.Fatalf("expected replayed proof to fail on a fresh challenge, got %v", serverErr)
	}
}

func TestHandshakeRejectsFramesTamperedInTransit(t *testing.T) {
	cases := map[string]struct {
		rewriteClient func(*grpcStreamMessage) *grpcStreamMessage
		rewriteServer func(*grpcStreamMessage) *grpcStreamMessage
	}{
		"advertised peers of the server": {
			rewriteServer: func(message *grpcStreamMessage) *grpcStreamMessage {
				if message.Handshake != nil {
					message.Handshake.KnownPeers = map[p2p.PeerID]string{"injected": "127.0.0.1:9"}
				}
				return message
			},
		},
		"advertised peers of the client": {
			rewriteClient: func(message *grpcStreamMessage) *grpcStreamMessage {
				if message.Handshake != nil && len(message.Handshake.Challenge) == 0 {
					message.Handshake.KnownPeers = map[p2p.PeerID]string{"injected": "127.0.0.1:9"}
				}
				return message
			},
		},
		"listen address of the client": {
			rewriteClient: func(message *grpcStreamMessage) *grpcStreamMessage {
				if message.Handshake != nil && len(message.Handshake.Challenge) > 0 {
					message.Handshake.ListenAddr = "127.0.0.1:9"
				}
				return message
			},
		},
		"challenge of the server": {
			rewriteServer: func(message *grpcStreamMessage) *grpcStreamMessage {
				if message.Handshake != nil {
					message.Handshake.Challenge = bytes.Repeat([]byte{1}, handshakeChallengeBytes)
				}
				return message
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			client := newSignedTransport(t, GRPCConfig{})
			server := newSignedTransport(t, GRPCConfig{})
			ctx, cancel := handshakeContext(t)
			clientStream, serverStream := newPipePair(ctx)
			clientStream.rewrite = testCase.rewriteClient
			serverStream.rewrite = testCase.rewriteServer
			outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel)
			if err := outcome.failure(); !errors.Is(err, ErrHandshakeSignature) {
				t.Fatalf("expected tampering to fail signature verification, got %v", err)
			}
		})
	}
}

func TestHandshakeRequiresSignatureWhenConfigured(t *testing.T) {
	server := newSignedTransport(t, GRPCConfig{})
	unsigned := newUnstartedTransport(t, "alice", GRPCConfig{})
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)
	outcome := runHandshake(t, unsigned, server, server.peerID, clientStream, serverStream, cancel)
	if !errors.Is(outcome.serverErr, ErrHandshakeSignature) {
		t.Fatalf("expected unsigned peer to be rejected by a signature-requiring server, got %v", outcome.serverErr)
	}
}

func TestHandshakeAcceptsUnsignedPeersWhenSignaturesAreNotRequired(t *testing.T) {
	alice := newUnstartedTransport(t, "alice", GRPCConfig{})
	bob := newUnstartedTransport(t, "bob", GRPCConfig{})
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)
	if outcome := runHandshake(t, alice, bob, "bob", clientStream, serverStream, cancel); outcome.failure() != nil {
		t.Fatalf("expected private harness handshake to succeed, got %v", outcome.failure())
	}
}

func TestHandshakeRejectsSelfAndInvalidIdentities(t *testing.T) {
	server := newUnstartedTransport(t, "bob", GRPCConfig{})
	for name, nodeID := range map[string]p2p.PeerID{
		"self":              "bob",
		"empty":             "",
		"whitespace":        "bad id",
		"control character": "bad\x01id",
		"oversized":         p2p.PeerID(bytes.Repeat([]byte("a"), p2p.MaxPeerIDBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			challenge, err := randomChallenge()
			if err != nil {
				t.Fatal(err)
			}
			hello := Handshake{ProtocolVersion: server.protocolVersion, NetworkID: server.networkID, ChainID: server.chainID, GenesisHash: server.genesisHash, NodeID: nodeID, Challenge: challenge}
			if err := server.checkHandshakeIdentity(&hello); !errors.Is(err, ErrHandshakeFailed) {
				t.Fatalf("expected %q to be rejected, got %v", nodeID, err)
			}
		})
	}
	hello := Handshake{NodeID: "alice", Challenge: []byte("short")}
	if err := server.checkHandshakeIdentity(&hello); !errors.Is(err, ErrHandshakeFailed) {
		t.Fatalf("expected a short challenge to be rejected, got %v", err)
	}
}

func TestHandshakeRejectsBannedPeerBeforeSigning(t *testing.T) {
	var signed atomic.Int32
	signer := newTestHandshakeSigner(t)
	server := newUnstartedTransport(t, p2p.NodeIDFromPublicKey(signer.PublicKey()), GRPCConfig{
		HandshakeSigner:           countingSigner{testHandshakeSigner: signer, signed: &signed},
		HandshakeVerifier:         testHandshakeVerifier{},
		RequireHandshakeSignature: true,
		PeerGate: func(_ context.Context, peerID p2p.PeerID) error {
			return p2p.ErrPeerBanned
		},
	})
	client := newSignedTransport(t, GRPCConfig{})
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)
	outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel)
	if !errors.Is(outcome.serverErr, ErrPeerRejected) {
		t.Fatalf("expected banned peer to be rejected, got %v", outcome.serverErr)
	}
	if signed.Load() != 0 {
		t.Fatal("server signed for a peer its gate had already rejected")
	}
}

type countingSigner struct {
	testHandshakeSigner
	signed *atomic.Int32
}

func (signer countingSigner) Sign(message []byte) (types.Signature, error) {
	signer.signed.Add(1)
	return signer.testHandshakeSigner.Sign(message)
}

func TestHandshakeTokenProofIsBoundToTranscriptAndSingleUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	store, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	verify := func(transport *GRPCTransport, nodeID p2p.PeerID, transcript []byte, proof string) error {
		return transport.verifyProofs(context.Background(), &Handshake{NodeID: nodeID}, &Handshake{AuthToken: proof}, transcript)
	}
	prover := newUnstartedTransport(t, "alice", GRPCConfig{AuthToken: "shared-token"})
	verifier := newUnstartedTransport(t, "bob", GRPCConfig{AuthToken: "shared-token", AuthReplayStore: store})
	transcript := []byte("transcript")

	proof, err := prover.authProof(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(verifier, "alice", []byte("another transcript"), proof); !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected proof for another transcript to fail, got %v", err)
	}
	if err := verify(verifier, "alice", transcript, proof); err != nil {
		t.Fatalf("expected first use to pass, got %v", err)
	}
	if err := verify(verifier, "alice", transcript, proof); !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected replayed proof rejection, got %v", err)
	}

	restartedStore, err := NewFileAuthReplayStore(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := newUnstartedTransport(t, "bob", GRPCConfig{AuthToken: "shared-token", AuthReplayStore: restartedStore})
	if err := verify(restarted, "alice", transcript, proof); !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected replay rejection after restart, got %v", err)
	}

	wrongToken := newUnstartedTransport(t, "carol", GRPCConfig{AuthToken: "other-token"})
	otherProof, err := wrongToken.authProof(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(verifier, "carol", transcript, otherProof); !errors.Is(err, ErrAuthTokenMismatch) {
		t.Fatalf("expected proof made with another token to fail, got %v", err)
	}
	for _, malformed := range []string{"", "v3", "v3:short:00", "v2:" + string(bytes.Repeat([]byte("0"), 2*handshakeProofNonceBytes)) + ":00"} {
		if err := verify(verifier, "carol", transcript, malformed); !errors.Is(err, ErrAuthTokenMismatch) {
			t.Fatalf("expected malformed proof %q to fail, got %v", malformed, err)
		}
	}
}

// recordingReplayStore counts the nonces a transport consumes.
type recordingReplayStore struct {
	inner AuthReplayStore

	mu      sync.Mutex
	records []string
}

func (store *recordingReplayStore) MarkAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string, expires time.Time, now time.Time) error {
	if err := store.inner.MarkAuthNonce(ctx, nodeID, nonce, expires, now); err != nil {
		return err
	}
	store.mu.Lock()
	store.records = append(store.records, string(nodeID)+"/"+nonce)
	store.mu.Unlock()
	return nil
}

func (store *recordingReplayStore) recorded() []string {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]string(nil), store.records...)
}

func TestHandshakeRecordsAuthNonceOnlyAfterSignatureVerifies(t *testing.T) {
	victim := newTestHandshakeSigner(t)
	attacker := newTestHandshakeSigner(t)
	victimID := p2p.NodeIDFromPublicKey(victim.PublicKey())
	store := &recordingReplayStore{inner: newMemoryAuthReplayStore()}
	server := newSignedTransport(t, GRPCConfig{AuthToken: "shared-token", AuthReplayStore: store})
	// Every network member knows the shared token, so the attacker can build a
	// proof whose MAC is valid; only the key signature is out of its reach.
	member := newUnstartedTransport(t, "member", GRPCConfig{AuthToken: "shared-token"})

	t.Run("signature made without the claimed key", func(t *testing.T) {
		ctx, cancel := handshakeContext(t)
		clientStream, serverStream := newPipePair(ctx)
		var serverErr error
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _, serverErr = server.serveHandshake(serverStream, serverStream.Recv)
			cancel()
		}()
		challenge, err := randomChallenge()
		if err != nil {
			t.Fatal(err)
		}
		hello := Handshake{
			ProtocolVersion: server.protocolVersion,
			NetworkID:       server.networkID,
			ChainID:         server.chainID,
			GenesisHash:     server.genesisHash,
			NodeID:          victimID,
			NodePublicKey:   victim.PublicKey(),
			Challenge:       challenge,
		}
		if err := clientStream.Send(&grpcStreamMessage{Handshake: &hello}); err != nil {
			t.Fatal(err)
		}
		reply, err := clientStream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		proof := Handshake{NodeID: victimID}
		transcript := handshakeTranscript(handshakeRoleClient, nil, &hello, reply.Handshake, proof.KnownPeers)
		if proof.Signature, err = attacker.Sign(transcript); err != nil {
			t.Fatal(err)
		}
		if proof.AuthToken, err = member.authProof(transcript); err != nil {
			t.Fatal(err)
		}
		if err := clientStream.Send(&grpcStreamMessage{Handshake: &proof}); err != nil {
			t.Fatal(err)
		}
		<-done
		if !errors.Is(serverErr, ErrHandshakeSignature) {
			t.Fatalf("expected a signature made without the victim key to fail, got %v", serverErr)
		}
		if records := store.recorded(); len(records) != 0 {
			t.Fatalf("a proof whose signature failed must not be recorded, got %v", records)
		}
	})

	t.Run("forged proofs cannot exhaust the quota of the claimed identity", func(t *testing.T) {
		identity := &Handshake{NodeID: victimID, NodePublicKey: victim.PublicKey()}
		transcript := []byte("transcript")
		forged := Handshake{}
		var err error
		if forged.Signature, err = attacker.Sign(transcript); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt <= maxAuthReplayEntriesPerNode; attempt++ {
			if forged.AuthToken, err = member.authProof(transcript); err != nil {
				t.Fatal(err)
			}
			if err := server.verifyProofs(context.Background(), identity, &forged, transcript); !errors.Is(err, ErrHandshakeSignature) {
				t.Fatalf("attempt %d: expected the forged signature to fail, got %v", attempt, err)
			}
		}
		genuine := Handshake{}
		if genuine.Signature, err = victim.Sign(transcript); err != nil {
			t.Fatal(err)
		}
		if genuine.AuthToken, err = member.authProof(transcript); err != nil {
			t.Fatal(err)
		}
		if err := server.verifyProofs(context.Background(), identity, &genuine, transcript); err != nil {
			t.Fatalf("the victim must still be able to authenticate after the forgeries, got %v", err)
		}
		if err := server.verifyProofs(context.Background(), identity, &genuine, transcript); !errors.Is(err, ErrAuthTokenMismatch) {
			t.Fatalf("a verified proof must still be single use, got %v", err)
		}
	})
}

func TestHandshakeRecordsOneNoncePerVerifiedProof(t *testing.T) {
	clientStore := &recordingReplayStore{inner: newMemoryAuthReplayStore()}
	serverStore := &recordingReplayStore{inner: newMemoryAuthReplayStore()}
	client := newSignedTransport(t, GRPCConfig{AuthToken: "shared-token", AuthReplayStore: clientStore})
	server := newSignedTransport(t, GRPCConfig{AuthToken: "shared-token", AuthReplayStore: serverStore})
	ctx, cancel := handshakeContext(t)
	clientStream, serverStream := newPipePair(ctx)
	if outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel); outcome.failure() != nil {
		t.Fatalf("expected authenticated handshake, got %v", outcome.failure())
	}
	if records := serverStore.recorded(); len(records) != 1 || !strings.HasPrefix(records[0], string(client.peerID)+"/") {
		t.Fatalf("expected the server to record the client proof once, got %v", records)
	}
	if records := clientStore.recorded(); len(records) != 1 || !strings.HasPrefix(records[0], string(server.peerID)+"/") {
		t.Fatalf("expected the client to record the server proof once, got %v", records)
	}
}

func TestVerifySignatureRecoversFromVerifierPanic(t *testing.T) {
	if verifySignatureSafely(panickingVerifier{}, types.PublicKey("k"), []byte("m"), types.Signature("s")) {
		t.Fatal("a verifier that panics must not validate a signature")
	}
}

type panickingVerifier struct{}

func (panickingVerifier) Verify(types.PublicKey, []byte, types.Signature) bool {
	panic("malformed key")
}

func TestHandshakeRejectsNonTLSOrMissingBinding(t *testing.T) {
	tlsTransport := newUnstartedTransport(t, "alice", GRPCConfig{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13}})
	if _, err := tlsTransport.tlsBinding(context.Background()); !errors.Is(err, ErrTLSRequired) {
		t.Fatalf("expected missing tls session to fail closed, got %v", err)
	}
	plain := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: plainAuthInfo{}})
	if _, err := tlsTransport.tlsBinding(plain); !errors.Is(err, ErrTLSRequired) {
		t.Fatalf("expected non-tls connection to fail closed, got %v", err)
	}
	insecureTransport := newUnstartedTransport(t, "bob", GRPCConfig{})
	if binding, err := insecureTransport.tlsBinding(context.Background()); err != nil || binding != nil {
		t.Fatalf("expected no binding without tls, got %x %v", binding, err)
	}
}

type plainAuthInfo struct{}

func (plainAuthInfo) AuthType() string { return "plain" }

func TestHandshakeIsBoundToTheTLSSession(t *testing.T) {
	caCert, caKey, pool := newTestCertificateAuthority(t)
	serverConfig := newTestPeerTLSConfig(t, caCert, caKey, pool, "server")
	clientConfig := newTestPeerTLSConfig(t, caCert, caKey, pool, "client")
	clientConfig.ServerName = "127.0.0.1"
	sessionA := newTLSSessionPair(t, clientConfig, serverConfig)
	sessionB := newTLSSessionPair(t, clientConfig, serverConfig)

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
	withSession := func(ctx context.Context, state tls.ConnectionState) context.Context {
		return peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: state}})
	}

	t.Run("matching sessions", func(t *testing.T) {
		client := newSignedTransport(t, GRPCConfig{TLSConfig: tlsConfig})
		server := newSignedTransport(t, GRPCConfig{TLSConfig: tlsConfig})
		ctx, cancel := handshakeContext(t)
		clientStream, serverStream := newPipePair(ctx)
		clientStream.ctx = withSession(ctx, sessionA.client)
		serverStream.ctx = withSession(ctx, sessionA.server)
		if outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel); outcome.failure() != nil {
			t.Fatalf("expected handshake bound to one tls session to succeed, got %v", outcome.failure())
		}
	})
	t.Run("proofs relayed to another tls session", func(t *testing.T) {
		client := newSignedTransport(t, GRPCConfig{TLSConfig: tlsConfig})
		server := newSignedTransport(t, GRPCConfig{TLSConfig: tlsConfig})
		ctx, cancel := handshakeContext(t)
		clientStream, serverStream := newPipePair(ctx)
		// The client runs inside session A while the server answers inside
		// session B, as when an attacker relays a handshake between two
		// separately terminated tls connections.
		clientStream.ctx = withSession(ctx, sessionA.client)
		serverStream.ctx = withSession(ctx, sessionB.server)
		outcome := runHandshake(t, client, server, server.peerID, clientStream, serverStream, cancel)
		if err := outcome.failure(); !errors.Is(err, ErrHandshakeSignature) {
			t.Fatalf("expected relayed handshake to fail, got %v", err)
		}
	})
}

type tlsSessionPair struct {
	client tls.ConnectionState
	server tls.ConnectionState
}

func newTLSSessionPair(t *testing.T, clientConfig *tls.Config, serverConfig *tls.Config) tlsSessionPair {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	client := tls.Client(clientConn, clientConfig)
	server := tls.Server(serverConn, serverConfig)
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	errs := make(chan error, 2)
	go func() { errs <- client.Handshake() }()
	go func() { errs <- server.Handshake() }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	return tlsSessionPair{client: client.ConnectionState(), server: server.ConnectionState()}
}

func TestHandshakeFramesAreBoundedByTheCodec(t *testing.T) {
	encode := func(handshake Handshake) []byte {
		data, err := encodeGRPCStreamMessage(&grpcStreamMessage{Handshake: &handshake})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	base := Handshake{ProtocolVersion: GRPCProtocolVersion, NodeID: "alice", Challenge: bytes.Repeat([]byte{1}, handshakeChallengeBytes)}
	if _, err := decodeGRPCStreamMessage(encode(base)); err != nil {
		t.Fatalf("expected a well formed handshake to decode, got %v", err)
	}

	oversized := map[string]func(*Handshake){
		"node id":        func(h *Handshake) { h.NodeID = p2p.PeerID(bytes.Repeat([]byte("a"), p2p.MaxPeerIDBytes+1)) },
		"listen address": func(h *Handshake) { h.ListenAddr = string(bytes.Repeat([]byte("a"), maxHandshakeAddressBytes+1)) },
		"public key":     func(h *Handshake) { h.NodePublicKey = bytes.Repeat([]byte{1}, maxHandshakeKeyBytes+1) },
		"signature":      func(h *Handshake) { h.Signature = bytes.Repeat([]byte{1}, maxHandshakeSignatureBytes+1) },
		"challenge":      func(h *Handshake) { h.Challenge = bytes.Repeat([]byte{1}, maxHandshakeChallengeBytes+1) },
		"auth token":     func(h *Handshake) { h.AuthToken = string(bytes.Repeat([]byte("a"), maxHandshakeProofBytes+1)) },
		"protocol":       func(h *Handshake) { h.ProtocolVersion = string(bytes.Repeat([]byte("a"), maxHandshakeTextBytes+1)) },
	}
	for name, mutate := range oversized {
		t.Run(name, func(t *testing.T) {
			handshake := base
			mutate(&handshake)
			if _, err := decodeGRPCStreamMessage(encode(handshake)); !errors.Is(err, ErrMessageTooLarge) {
				t.Fatalf("expected oversized %s to be rejected, got %v", name, err)
			}
		})
	}

	t.Run("too many known peers", func(t *testing.T) {
		handshake := base
		handshake.KnownPeers = make(map[p2p.PeerID]string)
		for index := 0; index <= maxHandshakePeers; index++ {
			handshake.KnownPeers[p2p.PeerID(string(rune('a'+index%26))+string(rune('a'+index/26)))] = "127.0.0.1:26656"
		}
		if _, err := decodeGRPCStreamMessage(encode(handshake)); !errors.Is(err, ErrMessageTooLarge) {
			t.Fatalf("expected an oversized peer list to be rejected, got %v", err)
		}
	})
	for name, peers := range map[string]map[p2p.PeerID]string{
		"unspecified address": {"bob": "0.0.0.0:26656"},
		"bad address":         {"bob": "not-an-address"},
		"invalid peer id":     {"bad id": "127.0.0.1:26656"},
	} {
		t.Run("malformed known peer: "+name, func(t *testing.T) {
			handshake := base
			handshake.KnownPeers = peers
			if _, err := decodeGRPCStreamMessage(encode(handshake)); !errors.Is(err, ErrHandshakeFailed) {
				t.Fatalf("expected malformed peer entry to reject the frame, got %v", err)
			}
		})
	}
	t.Run("legacy codec versions", func(t *testing.T) {
		for _, version := range []byte{1, 2} {
			data := encode(base)
			data[0] = version
			if _, err := decodeGRPCStreamMessage(data); err == nil {
				t.Fatalf("expected codec version %d to be rejected", version)
			}
		}
	})
	t.Run("trailing bytes", func(t *testing.T) {
		if _, err := decodeGRPCStreamMessage(append(encode(base), 0)); err == nil {
			t.Fatal("expected trailing bytes to be rejected")
		}
	})
}

func TestHandshakeSlotsBoundUnauthenticatedPeers(t *testing.T) {
	transport := newUnstartedTransport(t, "alice", GRPCConfig{})
	var releases []func()
	for range maxInboundHandshakes {
		release, err := transport.acquireInboundHandshake()
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	if _, err := transport.acquireInboundHandshake(); !errors.Is(err, ErrHandshakeBusy) {
		t.Fatalf("expected handshake capacity to be exhausted, got %v", err)
	}
	releases[0]()
	release, err := transport.acquireInboundHandshake()
	if err != nil {
		t.Fatalf("expected a released slot to be reusable, got %v", err)
	}
	release()
}

func TestDeadlineRecvAbandonsStalledPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, server := newPipePair(ctx)
	deadline := make(chan time.Time, 1)
	recv := deadlineRecv(server, deadline)
	deadline <- time.Now()
	if _, err := recv(); !errors.Is(err, ErrHandshakeFailed) {
		t.Fatalf("expected a stalled handshake to time out, got %v", err)
	}
}

func TestGRPCTransportConstructorBindsPeerIDToHandshakeKey(t *testing.T) {
	signer := newTestHandshakeSigner(t)
	_, err := NewGRPCTransport(GRPCConfig{
		PeerID:            "validator-1",
		HandshakeSigner:   signer,
		HandshakeVerifier: testHandshakeVerifier{},
	})
	if !errors.Is(err, ErrNodeIDMismatch) {
		t.Fatalf("expected a peer id not derived from the signer key to be rejected, got %v", err)
	}
	if _, err := NewGRPCTransport(GRPCConfig{PeerID: "bad id"}); !errors.Is(err, ErrInvalidPeerID) {
		t.Fatalf("expected an invalid peer id to be rejected, got %v", err)
	}
}

package transport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

const (
	handshakeChallengeBytes   = 32
	handshakeProofNonceBytes  = 16
	handshakeAuthTTL          = 2 * time.Minute
	handshakeAuthVersion      = "v3"
	handshakeTranscriptDomain = "vexo-p2p-handshake-v3"
	handshakeAuthDomain       = "vexo-p2p-auth-v3"
	tlsExporterLabel          = "EXPERIMENTAL vexo-p2p-handshake-v3"
	tlsBindingBytes           = 32
	defaultHandshakeTimeout   = 10 * time.Second

	// maxInboundHandshakes bounds the number of unauthenticated peers that
	// may hold a handshake open at the same time.
	maxInboundHandshakes = 32
)

type handshakeRole byte

const (
	handshakeRoleClient handshakeRole = 'c'
	handshakeRoleServer handshakeRole = 's'
)

// Handshake is one identity frame of the Gossip stream preamble.
//
// The preamble is a four frame challenge-response exchange. The dialing
// client opens with its identity and a fresh random Challenge. The server
// answers with its identity, its own fresh Challenge, and a proof over the
// transcript of both frames. The client then sends a proof over the transcript
// of everything it has seen, and the server closes the exchange with an empty
// accept frame. Every proof covers both challenges, so a recorded proof is
// useless on any other connection, and both identities, so a proof cannot be
// relayed to a different peer.
//
// A peer that presents NodePublicKey must use the NodeID derived from it
// (p2p.NodeIDFromPublicKey) and prove possession of the matching private key
// with Signature. The identity is therefore self-certifying: a NodeID is never
// accepted on the strength of a key chosen by the peer that claims it.
type Handshake struct {
	ProtocolVersion string                `json:"protocol_version"`
	NetworkID       string                `json:"network_id"`
	ChainID         string                `json:"chain_id"`
	GenesisHash     string                `json:"genesis_hash"`
	NodeID          p2p.PeerID            `json:"node_id"`
	ListenAddr      string                `json:"listen_addr,omitempty"`
	NodePublicKey   types.PublicKey       `json:"node_public_key,omitempty"`
	Challenge       []byte                `json:"challenge,omitempty"`
	AuthToken       string                `json:"auth_token,omitempty"`
	Signature       types.Signature       `json:"signature,omitempty"`
	KnownPeers      map[p2p.PeerID]string `json:"known_peers,omitempty"`
}

type HandshakeSigner interface {
	PublicKey() types.PublicKey
	Sign(message []byte) (types.Signature, error)
}

type HandshakeSignatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

// handshakeStream is the part of a gRPC stream both ends of the preamble use.
type handshakeStream interface {
	Context() context.Context
	Send(*grpcStreamMessage) error
	Recv() (*grpcStreamMessage, error)
}

type recvFunc func() (*grpcStreamMessage, error)

// identityHandshake returns the identity fields every frame of this node shares.
func (transport *GRPCTransport) identityHandshake() Handshake {
	transport.mu.RLock()
	listenAddr := transport.listenAddr
	transport.mu.RUnlock()
	handshake := Handshake{
		ProtocolVersion: transport.protocolVersion,
		NetworkID:       transport.networkID,
		ChainID:         transport.chainID,
		GenesisHash:     transport.genesisHash,
		NodeID:          transport.peerID,
		ListenAddr:      listenAddr,
	}
	if transport.handshakeSigner != nil {
		handshake.NodePublicKey = append(types.PublicKey(nil), transport.handshakeSigner.PublicKey()...)
	}
	return handshake
}

func randomChallenge() ([]byte, error) {
	challenge := make([]byte, handshakeChallengeBytes)
	if _, err := rand.Read(challenge); err != nil {
		return nil, fmt.Errorf("%w: challenge: %v", ErrHandshakeFailed, err)
	}
	return challenge, nil
}

// writeHandshakeFields appends the signed fields of a frame. AuthToken and
// Signature are the proofs over the transcript and are not part of it.
func writeHandshakeFields(buffer *bytes.Buffer, handshake *Handshake) {
	writeBinaryString(buffer, handshake.ProtocolVersion)
	writeBinaryString(buffer, handshake.NetworkID)
	writeBinaryString(buffer, handshake.ChainID)
	writeBinaryString(buffer, handshake.GenesisHash)
	writeBinaryString(buffer, string(handshake.NodeID))
	writeBinaryString(buffer, handshake.ListenAddr)
	writeBinaryBytes(buffer, handshake.NodePublicKey)
	writeBinaryBytes(buffer, handshake.Challenge)
	writeBinaryPeerMap(buffer, handshake.KnownPeers)
}

// handshakeTranscript is the byte string both proofs of a role are computed
// over. Each side builds it from the frames exactly as it sent and received
// them, so a single changed byte, a different challenge, a different identity,
// or a different TLS session makes the proofs fail.
func handshakeTranscript(role handshakeRole, binding []byte, hello *Handshake, reply *Handshake, clientPeers map[p2p.PeerID]string) []byte {
	var buffer bytes.Buffer
	writeBinaryString(&buffer, handshakeTranscriptDomain)
	buffer.WriteByte(byte(role))
	writeBinaryBytes(&buffer, binding)
	writeHandshakeFields(&buffer, hello)
	writeHandshakeFields(&buffer, reply)
	if role == handshakeRoleClient {
		writeBinaryPeerMap(&buffer, clientPeers)
	}
	return buffer.Bytes()
}

// tlsBinding returns the TLS exporter value of the connection the stream runs
// on, which ties the application level proofs to that exact TLS session. It
// fails closed when TLS is configured but the session cannot be inspected.
func (transport *GRPCTransport) tlsBinding(ctx context.Context) ([]byte, error) {
	if transport.tlsConfig == nil {
		return nil, nil
	}
	info, ok := peer.FromContext(ctx)
	if !ok || info.AuthInfo == nil {
		return nil, fmt.Errorf("%w: tls session is unavailable for channel binding", ErrTLSRequired)
	}
	tlsInfo, ok := info.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("%w: connection is not protected by tls", ErrTLSRequired)
	}
	binding, err := tlsInfo.State.ExportKeyingMaterial(tlsExporterLabel, nil, tlsBindingBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: tls channel binding: %v", ErrTLSRequired, err)
	}
	return binding, nil
}

func (transport *GRPCTransport) checkHandshakeParameters(handshake *Handshake) error {
	if handshake.ProtocolVersion != transport.protocolVersion {
		return fmt.Errorf("%w: local=%s remote=%s", ErrProtocolMismatch, transport.protocolVersion, handshake.ProtocolVersion)
	}
	if transport.networkID != "" && handshake.NetworkID != transport.networkID {
		return fmt.Errorf("%w: local=%s remote=%s", ErrNetworkMismatch, transport.networkID, handshake.NetworkID)
	}
	if transport.chainID != "" && handshake.ChainID != transport.chainID {
		return fmt.Errorf("%w: local=%s remote=%s", ErrChainIDMismatch, transport.chainID, handshake.ChainID)
	}
	if transport.genesisHash != "" && handshake.GenesisHash != transport.genesisHash {
		return fmt.Errorf("%w: local=%s remote=%s", ErrGenesisHashMismatch, transport.genesisHash, handshake.GenesisHash)
	}
	return nil
}

// checkHandshakeIdentity validates the claimed identity of an identity frame
// and its binding to the presented public key. It does not prove possession of
// the key; verifySignature does.
func (transport *GRPCTransport) checkHandshakeIdentity(handshake *Handshake) error {
	if !p2p.ValidPeerID(handshake.NodeID) {
		return fmt.Errorf("%w: invalid node id", ErrHandshakeFailed)
	}
	if handshake.NodeID == transport.peerID {
		return fmt.Errorf("%w: connection to self", ErrHandshakeFailed)
	}
	if len(handshake.Challenge) != handshakeChallengeBytes {
		return fmt.Errorf("%w: invalid challenge", ErrHandshakeFailed)
	}
	if len(handshake.NodePublicKey) == 0 {
		if transport.requireHandshakeSignature {
			return fmt.Errorf("%w: node public key is required", ErrHandshakeSignature)
		}
		return nil
	}
	if derived := p2p.NodeIDFromPublicKey(handshake.NodePublicKey); handshake.NodeID != derived {
		return fmt.Errorf("%w: node id %q is not derived from the presented public key", ErrNodeIDMismatch, handshake.NodeID)
	}
	return nil
}

func (transport *GRPCTransport) attachProofs(handshake *Handshake, transcript []byte) error {
	if transport.handshakeSigner != nil {
		signature, err := transport.handshakeSigner.Sign(transcript)
		if err != nil {
			return fmt.Errorf("%w: sign: %v", ErrHandshakeSignature, err)
		}
		handshake.Signature = append(types.Signature(nil), signature...)
	}
	proof, err := transport.authProof(transcript)
	if err != nil {
		return err
	}
	handshake.AuthToken = proof
	return nil
}

// verifyProofs checks the shared-secret proof and the key signature carried
// by proof over transcript, on behalf of the peer described by identity.
//
// The single-use nonce of the shared-secret proof is recorded last. Every
// member of the network knows the shared secret, so a valid MAC says nothing
// about who sent the proof; only the key signature binds it to identity.
// Recording before that check would let any holder of the secret fill the
// replay quota of an identity whose key it does not own.
func (transport *GRPCTransport) verifyProofs(ctx context.Context, identity *Handshake, proof *Handshake, transcript []byte) error {
	nonce, err := transport.checkAuthProof(transcript, proof.AuthToken)
	if err != nil {
		return err
	}
	if err := transport.verifySignature(identity, proof.Signature, transcript); err != nil {
		return err
	}
	return transport.recordAuthNonce(ctx, identity.NodeID, nonce)
}

func (transport *GRPCTransport) verifySignature(identity *Handshake, signature types.Signature, transcript []byte) error {
	hasKey := len(identity.NodePublicKey) > 0
	if !transport.requireHandshakeSignature && !hasKey && len(signature) == 0 {
		return nil
	}
	if !hasKey || len(signature) == 0 || transport.handshakeVerifier == nil {
		return ErrHandshakeSignature
	}
	if !verifySignatureSafely(transport.handshakeVerifier, identity.NodePublicKey, transcript, signature) {
		return ErrHandshakeSignature
	}
	return nil
}

// verifySignatureSafely treats a verifier that panics on malformed
// peer-supplied input as a failed verification instead of a process crash.
func verifySignatureSafely(verifier HandshakeSignatureVerifier, publicKey types.PublicKey, message []byte, signature types.Signature) (valid bool) {
	defer func() {
		if recover() != nil {
			valid = false
		}
	}()
	return verifier.Verify(publicKey, message, signature)
}

func (transport *GRPCTransport) authProof(transcript []byte) (string, error) {
	if transport.authToken == "" {
		return "", nil
	}
	var nonce [handshakeProofNonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("%w: auth nonce: %v", ErrHandshakeFailed, err)
	}
	encoded := hex.EncodeToString(nonce[:])
	return handshakeAuthVersion + ":" + encoded + ":" + transport.authMAC(encoded, transcript), nil
}

// checkAuthProof verifies the shared-secret MAC over transcript and returns the
// proof's nonce. It records nothing: the caller consumes the nonce with
// recordAuthNonce once the rest of the peer's proof has been verified. No
// nonce is returned when neither side uses a shared secret.
func (transport *GRPCTransport) checkAuthProof(transcript []byte, proof string) (string, error) {
	if transport.authToken == "" && proof == "" {
		return "", nil
	}
	if transport.authToken == "" || proof == "" {
		return "", ErrAuthTokenMismatch
	}
	parts := strings.Split(proof, ":")
	if len(parts) != 3 || parts[0] != handshakeAuthVersion || len(parts[1]) != 2*handshakeProofNonceBytes {
		return "", ErrAuthTokenMismatch
	}
	if !hmac.Equal([]byte(parts[2]), []byte(transport.authMAC(parts[1], transcript))) {
		return "", ErrAuthTokenMismatch
	}
	return parts[1], nil
}

// recordAuthNonce consumes the single-use nonce of a verified proof. The proof
// is already bound to a one-shot challenge chosen by this node; the replay
// window is defense in depth for the shared secret.
func (transport *GRPCTransport) recordAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string) error {
	if nonce == "" {
		return nil
	}
	now := time.Now()
	if err := transport.authReplayStore.MarkAuthNonce(ctx, nodeID, nonce, now.Add(handshakeAuthTTL), now); err != nil {
		return fmt.Errorf("%w: %v", ErrAuthTokenMismatch, err)
	}
	return nil
}

func (transport *GRPCTransport) authMAC(nonce string, transcript []byte) string {
	mac := hmac.New(sha256.New, []byte(transport.authToken))
	mac.Write([]byte(handshakeAuthDomain))
	mac.Write([]byte{0})
	mac.Write([]byte(nonce))
	mac.Write([]byte{0})
	mac.Write(transcript)
	return hex.EncodeToString(mac.Sum(nil))
}

// serveHandshake runs the server side of the preamble and returns the
// authenticated identity of the dialing peer. The returned hello is non-nil
// whenever the client got far enough to name itself, so failures can be
// attributed in logs.
func (transport *GRPCTransport) serveHandshake(stream handshakeStream, recv recvFunc) (hello *Handshake, learned Handshake, err error) {
	ctx := stream.Context()
	first, err := recv()
	if err != nil {
		return nil, Handshake{}, err
	}
	if first.Handshake == nil {
		return nil, Handshake{}, fmt.Errorf("%w: missing handshake", ErrHandshakeFailed)
	}
	hello = first.Handshake
	if err := transport.checkHandshakeParameters(hello); err != nil {
		return hello, Handshake{}, err
	}
	if err := transport.checkHandshakeIdentity(hello); err != nil {
		return hello, Handshake{}, err
	}
	// Rejecting a banned identity before signing keeps the cost of a rejected
	// peer to one frame. The gate only reads state, and the identity is
	// authenticated by the proofs below before anything else depends on it.
	if err := transport.checkPeerGate(ctx, hello.NodeID); err != nil {
		return hello, Handshake{}, err
	}
	binding, err := transport.tlsBinding(ctx)
	if err != nil {
		return hello, Handshake{}, err
	}
	challenge, err := randomChallenge()
	if err != nil {
		return hello, Handshake{}, err
	}
	reply := transport.identityHandshake()
	reply.Challenge = challenge
	reply.KnownPeers = transport.advertisedPeers()
	if err := transport.attachProofs(&reply, handshakeTranscript(handshakeRoleServer, binding, hello, &reply, nil)); err != nil {
		return hello, Handshake{}, err
	}
	if err := stream.Send(&grpcStreamMessage{Handshake: &reply}); err != nil {
		return hello, Handshake{}, err
	}
	second, err := recv()
	if err != nil {
		return hello, Handshake{}, err
	}
	proof := second.Handshake
	if proof == nil {
		return hello, Handshake{}, fmt.Errorf("%w: missing handshake proof", ErrHandshakeFailed)
	}
	if proof.NodeID != hello.NodeID {
		return hello, Handshake{}, fmt.Errorf("%w: handshake proof names a different node", ErrHandshakeFailed)
	}
	if err := transport.verifyProofs(ctx, hello, proof, handshakeTranscript(handshakeRoleClient, binding, hello, &reply, proof.KnownPeers)); err != nil {
		return hello, Handshake{}, err
	}
	if err := stream.Send(&grpcStreamMessage{}); err != nil {
		return hello, Handshake{}, err
	}
	return hello, Handshake{NodeID: hello.NodeID, ListenAddr: hello.ListenAddr, KnownPeers: proof.KnownPeers}, nil
}

// dialHandshake runs the client side of the preamble against the peer that is
// configured as expected, and returns what it learned about the server.
func (transport *GRPCTransport) dialHandshake(stream handshakeStream, expected p2p.PeerID) (Handshake, error) {
	challenge, err := randomChallenge()
	if err != nil {
		return Handshake{}, err
	}
	hello := transport.identityHandshake()
	hello.Challenge = challenge
	if err := stream.Send(&grpcStreamMessage{Handshake: &hello}); err != nil {
		return Handshake{}, fmt.Errorf("send hello: %w", err)
	}
	received, err := stream.Recv()
	if err != nil {
		return Handshake{}, fmt.Errorf("receive handshake challenge: %w", normalizeGRPCError(err))
	}
	reply := received.Handshake
	if reply == nil {
		return Handshake{}, fmt.Errorf("%w: missing remote handshake", ErrHandshakeFailed)
	}
	ctx := stream.Context()
	if reply.NodeID != expected {
		return Handshake{}, fmt.Errorf("%w: expected peer %s got %s", ErrHandshakeFailed, expected, reply.NodeID)
	}
	if err := transport.checkHandshakeParameters(reply); err != nil {
		return Handshake{}, err
	}
	if err := transport.checkHandshakeIdentity(reply); err != nil {
		return Handshake{}, err
	}
	binding, err := transport.tlsBinding(ctx)
	if err != nil {
		return Handshake{}, err
	}
	if err := transport.verifyProofs(ctx, reply, reply, handshakeTranscript(handshakeRoleServer, binding, &hello, reply, nil)); err != nil {
		return Handshake{}, err
	}
	if err := transport.checkPeerGate(ctx, reply.NodeID); err != nil {
		return Handshake{}, fmt.Errorf("remote peer gate: %w", err)
	}
	proof := Handshake{NodeID: transport.peerID, KnownPeers: transport.advertisedPeers()}
	if err := transport.attachProofs(&proof, handshakeTranscript(handshakeRoleClient, binding, &hello, reply, proof.KnownPeers)); err != nil {
		return Handshake{}, err
	}
	if err := stream.Send(&grpcStreamMessage{Handshake: &proof}); err != nil {
		return Handshake{}, fmt.Errorf("send handshake proof: %w", err)
	}
	accept, err := stream.Recv()
	if err != nil {
		return Handshake{}, fmt.Errorf("receive handshake accept: %w", normalizeGRPCError(err))
	}
	if accept.Handshake != nil || accept.Envelope != nil {
		return Handshake{}, fmt.Errorf("%w: unexpected frame instead of accept", ErrHandshakeFailed)
	}
	return Handshake{NodeID: reply.NodeID, ListenAddr: reply.ListenAddr, KnownPeers: reply.KnownPeers}, nil
}

// deadlineRecv wraps stream.Recv so the server can abandon a peer that stalls
// inside the preamble. The abandoned receive ends when the handler returns and
// gRPC cancels the stream.
func deadlineRecv(stream handshakeStream, deadline <-chan time.Time) recvFunc {
	type result struct {
		message *grpcStreamMessage
		err     error
	}
	return func() (*grpcStreamMessage, error) {
		received := make(chan result, 1)
		go func() {
			message, err := stream.Recv()
			received <- result{message: message, err: err}
		}()
		select {
		case outcome := <-received:
			return outcome.message, outcome.err
		case <-deadline:
			return nil, fmt.Errorf("%w: handshake timed out", ErrHandshakeFailed)
		case <-stream.Context().Done():
			return nil, stream.Context().Err()
		}
	}
}

func normalizeGRPCError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	for _, kind := range []error{
		ErrProtocolMismatch,
		ErrNetworkMismatch,
		ErrChainIDMismatch,
		ErrGenesisHashMismatch,
		ErrAuthTokenMismatch,
		ErrNodeIDMismatch,
		ErrHandshakeSignature,
		ErrPeerRejected,
		ErrTLSRequired,
		ErrHandshakeBusy,
		ErrHandshakeFailed,
	} {
		if strings.Contains(message, kind.Error()) {
			return fmt.Errorf("%w: %v", kind, err)
		}
	}
	return err
}

// acquireInboundHandshake reserves one of the bounded unauthenticated
// handshake slots; the returned function releases it.
func (transport *GRPCTransport) acquireInboundHandshake() (func(), error) {
	select {
	case transport.handshakeSlots <- struct{}{}:
		return func() { <-transport.handshakeSlots }, nil
	default:
		return nil, ErrHandshakeBusy
	}
}

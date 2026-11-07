package transport

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	GRPCProtocolVersion      = "vexo-p2p/2"
	defaultGRPCDialTimeout   = 3 * time.Second
	defaultReconnectInterval = 500 * time.Millisecond
	defaultReconnectBackoff  = 500 * time.Millisecond
	maxReconnectBackoff      = 30 * time.Second
	defaultMaxMessageBytes   = 4 * 1024 * 1024
	defaultSubscriberBuffer  = 32
)

var (
	ErrHandshakeFailed     = errors.New("p2p handshake failed")
	ErrProtocolMismatch    = errors.New("protocol version mismatch")
	ErrNetworkMismatch     = errors.New("network id mismatch")
	ErrChainIDMismatch     = errors.New("chain id mismatch")
	ErrGenesisHashMismatch = errors.New("genesis hash mismatch")
	ErrAuthTokenMismatch   = errors.New("p2p auth token mismatch")
	ErrHandshakeSignature  = errors.New("p2p handshake signature invalid")
	ErrNodeIDMismatch      = errors.New("p2p node id does not match its public key")
	ErrHandshakeBusy       = errors.New("p2p handshake capacity exhausted")
	ErrMessageTooLarge     = errors.New("p2p message too large")
	ErrPeerBackoffActive   = errors.New("peer reconnect backoff active")
	ErrPeerDialInProgress  = errors.New("peer dial already in progress")
	ErrTLSRequired         = errors.New("p2p tls is required")
	ErrAuthTokenRequired   = errors.New("p2p auth token is required")
	ErrAuthNonceReplay     = errors.New("p2p auth nonce replay")
	ErrAuthReplayStore     = errors.New("p2p auth replay store is required")
	ErrInvalidPeerID       = errors.New("peer id is invalid")
)

type AuthReplayStore interface {
	MarkAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string, expires time.Time, now time.Time) error
}

type GRPCConfig struct {
	PeerID      p2p.PeerID
	ListenAddr  string
	DialTimeout time.Duration
	// Peers are the operator-configured peers. They are trusted dial targets:
	// handshake discovery never evicts, rewrites, or demotes them.
	Peers map[p2p.PeerID]string
	// LearnedPeers seeds the bounded pool of unverified dial candidates, for
	// example from a persisted address book. A candidate becomes a peer only
	// by completing an authenticated handshake.
	LearnedPeers map[p2p.PeerID]string
	// MaxLearnedPeers bounds the learned candidate pool. Zero selects
	// DefaultMaxLearnedPeers and a negative value disables learning.
	MaxLearnedPeers  int
	ProtocolVersion  string
	NetworkID        string
	ChainID          string
	GenesisHash      string
	MaxMessageBytes  uint64
	TLSConfig        *tls.Config
	RequireTLS       bool
	MaxPeers         int
	ReconnectBackoff time.Duration
	// ReconnectInterval is how often the reconnect loop looks for peers
	// without a session. A zero ReconnectBackoff selects a default; failures
	// back off exponentially from it.
	ReconnectInterval time.Duration
	// HandshakeTimeout bounds how long an inbound peer may take to complete
	// the handshake.
	HandshakeTimeout       time.Duration
	SubscriberBuffer       int
	AuthToken              string
	AuthReplayStore        AuthReplayStore
	RequireAuthReplayStore bool
	PeerLearned            func(p2p.PeerID, string)
	PeerAttempted          func(p2p.PeerID)
	PeerDialResult         func(p2p.PeerID, bool)
	PeerEvent              func(PeerEvent)
	PeerGate               func(context.Context, p2p.PeerID) error
	// Stopped runs once Stop has shut every connection down, so no peer hook can fire after it:
	// state the hooks write behind, such as the address book, is flushed there.
	Stopped                   func() error
	HandshakeSigner           HandshakeSigner
	HandshakeVerifier         HandshakeSignatureVerifier
	RequireHandshakeSignature bool
}

type PeerEvent struct {
	Type         string
	PeerID       p2p.PeerID
	Address      string
	Direction    string
	Reason       string
	BackoffUntil time.Time
}

type GRPCTransport struct {
	peerID                    p2p.PeerID
	listenAddr                string
	dialTimeout               time.Duration
	protocolVersion           string
	networkID                 string
	chainID                   string
	genesisHash               string
	maxMessageBytes           uint64
	tlsConfig                 *tls.Config
	maxPeers                  int
	maxLearned                int
	reconnectBackoff          time.Duration
	reconnectInterval         time.Duration
	handshakeTimeout          time.Duration
	subscriberBuffer          int
	authToken                 string
	authReplayStore           AuthReplayStore
	peerLearned               func(p2p.PeerID, string)
	peerAttempted             func(p2p.PeerID)
	peerDialResult            func(p2p.PeerID, bool)
	stopped                   func() error
	peerEvent                 func(PeerEvent)
	peerGates                 []func(context.Context, p2p.PeerID) error
	handshakeSigner           HandshakeSigner
	handshakeVerifier         HandshakeSignatureVerifier
	requireHandshakeSignature bool
	handshakeSlots            chan struct{}

	mu               sync.RWMutex
	listener         net.Listener
	server           *grpc.Server
	started          bool
	rootCtx          context.Context
	rootCancel       context.CancelFunc
	reconnectCancel  context.CancelFunc
	reconnectDone    chan struct{}
	peers            map[p2p.PeerID]string
	peerOrder        []p2p.PeerID
	learned          map[p2p.PeerID]string
	learnedOrder     []p2p.PeerID
	learnWindowStart time.Time
	learnWindowCount int
	connections      map[p2p.PeerID]*grpc.ClientConn
	sessions         map[p2p.PeerID]*grpcPeerSession
	sessionLocks     map[p2p.PeerID]*sync.Mutex
	backoffUntil     map[p2p.PeerID]time.Time
	failures         map[p2p.PeerID]int
	subscribers      map[p2p.Topic][]*subscription
	droppedMessages  uint64
}

type grpcPeerSession struct {
	stream grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage]
	cancel context.CancelFunc
	mu     sync.Mutex
}

func NewGRPCTransport(config GRPCConfig) (*GRPCTransport, error) {
	if config.PeerID == "" {
		return nil, ErrPeerIDRequired
	}
	if !p2p.ValidPeerID(config.PeerID) {
		return nil, ErrInvalidPeerID
	}
	if config.DialTimeout <= 0 {
		config.DialTimeout = defaultGRPCDialTimeout
	}
	if config.ProtocolVersion == "" {
		config.ProtocolVersion = GRPCProtocolVersion
	}
	if config.MaxMessageBytes == 0 {
		config.MaxMessageBytes = defaultMaxMessageBytes
	}
	if config.SubscriberBuffer <= 0 {
		config.SubscriberBuffer = defaultSubscriberBuffer
	}
	if config.ReconnectInterval <= 0 {
		config.ReconnectInterval = defaultReconnectInterval
	}
	if config.ReconnectBackoff <= 0 {
		config.ReconnectBackoff = defaultReconnectBackoff
	}
	if config.HandshakeTimeout <= 0 {
		config.HandshakeTimeout = defaultHandshakeTimeout
	}
	maxLearned := config.MaxLearnedPeers
	if maxLearned == 0 {
		maxLearned = DefaultMaxLearnedPeers
	}
	if maxLearned < 0 {
		maxLearned = 0
	}
	tlsConfig := cloneGRPCTLSConfig(config.TLSConfig)
	if config.RequireTLS && tlsConfig == nil {
		return nil, ErrTLSRequired
	}
	if config.RequireAuthReplayStore && config.AuthReplayStore == nil {
		return nil, ErrAuthReplayStore
	}
	if config.RequireHandshakeSignature && (config.HandshakeSigner == nil || config.HandshakeVerifier == nil) {
		return nil, ErrHandshakeSignature
	}
	if config.HandshakeSigner != nil {
		if derived := p2p.NodeIDFromPublicKey(config.HandshakeSigner.PublicKey()); config.PeerID != derived {
			return nil, fmt.Errorf("%w: peer id %q must equal %q", ErrNodeIDMismatch, config.PeerID, derived)
		}
	}
	authReplayStore := config.AuthReplayStore
	if authReplayStore == nil {
		authReplayStore = newMemoryAuthReplayStore()
	}
	transport := &GRPCTransport{
		peerID:                    config.PeerID,
		listenAddr:                config.ListenAddr,
		dialTimeout:               config.DialTimeout,
		protocolVersion:           config.ProtocolVersion,
		networkID:                 config.NetworkID,
		chainID:                   config.ChainID,
		genesisHash:               config.GenesisHash,
		maxMessageBytes:           config.MaxMessageBytes,
		tlsConfig:                 tlsConfig,
		maxPeers:                  config.MaxPeers,
		maxLearned:                maxLearned,
		reconnectBackoff:          config.ReconnectBackoff,
		reconnectInterval:         config.ReconnectInterval,
		handshakeTimeout:          config.HandshakeTimeout,
		subscriberBuffer:          config.SubscriberBuffer,
		authToken:                 config.AuthToken,
		authReplayStore:           authReplayStore,
		peerLearned:               config.PeerLearned,
		peerAttempted:             config.PeerAttempted,
		peerDialResult:            config.PeerDialResult,
		stopped:                   config.Stopped,
		peerEvent:                 config.PeerEvent,
		peerGates:                 compactPeerGates(config.PeerGate),
		handshakeSigner:           config.HandshakeSigner,
		handshakeVerifier:         config.HandshakeVerifier,
		requireHandshakeSignature: config.RequireHandshakeSignature,
		handshakeSlots:            make(chan struct{}, maxInboundHandshakes),
		peers:                     make(map[p2p.PeerID]string, len(config.Peers)),
		peerOrder:                 make([]p2p.PeerID, 0, len(config.Peers)),
		learned:                   make(map[p2p.PeerID]string),
		connections:               make(map[p2p.PeerID]*grpc.ClientConn),
		sessions:                  make(map[p2p.PeerID]*grpcPeerSession),
		sessionLocks:              make(map[p2p.PeerID]*sync.Mutex),
		backoffUntil:              make(map[p2p.PeerID]time.Time),
		failures:                  make(map[p2p.PeerID]int),
		subscribers:               make(map[p2p.Topic][]*subscription),
	}
	for _, peerID := range sortedPeerIDs(config.Peers) {
		address := config.Peers[peerID]
		if peerID == "" || address == "" || peerID == config.PeerID {
			continue
		}
		if config.MaxPeers > 0 && len(transport.peers) >= config.MaxPeers {
			continue
		}
		transport.peers[peerID] = address
		transport.peerOrder = append(transport.peerOrder, peerID)
	}
	for _, peerID := range sortedPeerIDs(config.LearnedPeers) {
		transport.learnPeerLocked(peerID, config.LearnedPeers[peerID])
	}
	return transport, nil
}

// NewNetworkSafeGRPCTransport constructs a transport for public/value-bearing
// nodes and fails closed when authentication, replay protection, TLS, or
// signed peer identity are missing. NewGRPCTransport remains available for
// tests, private harnesses, and SDK embedders that intentionally choose a
// weaker boundary.
func NewNetworkSafeGRPCTransport(config GRPCConfig) (*GRPCTransport, error) {
	if !config.RequireTLS || config.TLSConfig == nil {
		return nil, ErrTLSRequired
	}
	if config.AuthToken == "" {
		return nil, ErrAuthTokenRequired
	}
	if !config.RequireAuthReplayStore || config.AuthReplayStore == nil {
		return nil, ErrAuthReplayStore
	}
	if !config.RequireHandshakeSignature || config.HandshakeSigner == nil || config.HandshakeVerifier == nil {
		return nil, ErrHandshakeSignature
	}
	return NewGRPCTransport(config)
}

func GenesisHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (transport *GRPCTransport) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.started {
		return nil
	}
	listener, err := net.Listen("tcp", transport.listenAddr)
	if err != nil {
		return err
	}
	serverOptions := []grpc.ServerOption{
		grpc.ForceServerCodec(grpcBinaryCodec{}),
		grpc.MaxRecvMsgSize(int(transport.maxMessageBytes)),
		grpc.MaxSendMsgSize(int(transport.maxMessageBytes)),
		grpc.ConnectionTimeout(transport.handshakeTimeout),
		// Stop must not return while a handler can still deliver into the
		// subscriptions it is tearing down.
		grpc.WaitForHandlers(true),
	}
	if transport.tlsConfig != nil {
		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(transport.tlsConfig.Clone())))
	}
	server := grpc.NewServer(serverOptions...)
	registerP2PTransportServer(server, transport)
	transport.listener = listener
	transport.server = server
	transport.listenAddr = listener.Addr().String()
	transport.started = true
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	reconnectCtx, cancelReconnect := context.WithCancel(rootCtx)
	reconnectDone := make(chan struct{})
	transport.rootCtx = rootCtx
	transport.rootCancel = cancelRoot
	transport.reconnectCancel = cancelReconnect
	transport.reconnectDone = reconnectDone
	go func() {
		_ = server.Serve(listener)
	}()
	go transport.runReconnectLoop(reconnectCtx, reconnectDone)
	return nil
}

func (transport *GRPCTransport) Stop(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	transport.mu.Lock()
	if !transport.started {
		transport.mu.Unlock()
		return ErrTransportClosed
	}
	server := transport.server
	listener := transport.listener
	connections := transport.connections
	sessions := transport.sessions
	rootCancel := transport.rootCancel
	reconnectCancel := transport.reconnectCancel
	reconnectDone := transport.reconnectDone
	subscriptions := transport.subscribers
	transport.server = nil
	transport.listener = nil
	transport.started = false
	transport.rootCtx = nil
	transport.rootCancel = nil
	transport.reconnectCancel = nil
	transport.reconnectDone = nil
	transport.connections = make(map[p2p.PeerID]*grpc.ClientConn)
	transport.sessions = make(map[p2p.PeerID]*grpcPeerSession)
	transport.sessionLocks = make(map[p2p.PeerID]*sync.Mutex)
	transport.backoffUntil = make(map[p2p.PeerID]time.Time)
	transport.failures = make(map[p2p.PeerID]int)
	transport.subscribers = make(map[p2p.Topic][]*subscription)
	transport.mu.Unlock()
	if reconnectCancel != nil {
		reconnectCancel()
	}
	if rootCancel != nil {
		rootCancel()
	}
	// Subscriptions are torn down before the server so that no handler is left
	// blocked on a reliable delivery. close releases blocked senders, waits for
	// in-flight deliveries to leave, and only then closes the channel.
	for _, topicSubscriptions := range subscriptions {
		for _, subscriber := range topicSubscriptions {
			subscriber.close()
		}
	}
	if reconnectDone != nil {
		select {
		case <-reconnectDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if server != nil {
		stopped := make(chan struct{})
		go func() {
			server.Stop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
	}
	for _, connection := range connections {
		_ = connection.Close()
	}
	for _, session := range sessions {
		_ = session.stream.CloseSend()
		session.cancel()
	}
	if transport.stopped != nil {
		return transport.stopped()
	}
	return nil
}

func (transport *GRPCTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	if err := transport.validatePayloadSize(data); err != nil {
		return err
	}
	peers := transport.publishTargets()
	errs := make(chan error, len(peers))
	var wait sync.WaitGroup
	for peerID, address := range peers {
		wait.Add(1)
		go func(peerID p2p.PeerID, address string) {
			defer wait.Done()
			if err := transport.sendEnvelope(ctx, peerID, address, Envelope{Topic: topic, From: transport.peerID, Data: append([]byte(nil), data...)}); err != nil {
				if errors.Is(err, ErrPeerRejected) {
					return
				}
				errs <- err
			}
		}(peerID, address)
	}
	wait.Wait()
	close(errs)
	publishErrs := make([]error, 0, len(errs))
	for err := range errs {
		publishErrs = append(publishErrs, err)
	}
	return errors.Join(publishErrs...)
}

func (transport *GRPCTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	if err := transport.validatePayloadSize(data); err != nil {
		return err
	}
	address, ok := transport.peerAddress(to)
	if !ok {
		return ErrPeerNotFound
	}
	return transport.sendEnvelope(ctx, to, address, Envelope{Topic: topic, From: transport.peerID, To: to, Data: append([]byte(nil), data...)})
}

// Subscribe returns a channel of envelopes for topic. The channel is closed
// when ctx ends or the transport stops, never while a delivery is in flight.
func (transport *GRPCTransport) Subscribe(ctx context.Context, topic p2p.Topic) (<-chan Envelope, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	subscriber := newSubscription(topic, transport.subscriberBuffer)
	transport.mu.Lock()
	if !transport.started {
		transport.mu.Unlock()
		return nil, ErrTransportClosed
	}
	transport.subscribers[topic] = append(transport.subscribers[topic], subscriber)
	transport.mu.Unlock()
	go func() {
		select {
		case <-ctx.Done():
			transport.removeSubscription(subscriber)
			subscriber.close()
		case <-subscriber.done:
		}
	}()
	return subscriber.ch, nil
}

func (transport *GRPCTransport) removeSubscription(subscriber *subscription) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	remaining := transport.subscribers[subscriber.topic][:0:0]
	for _, existing := range transport.subscribers[subscriber.topic] {
		if existing != subscriber {
			remaining = append(remaining, existing)
		}
	}
	if len(remaining) == 0 {
		delete(transport.subscribers, subscriber.topic)
		return
	}
	transport.subscribers[subscriber.topic] = remaining
}

func (transport *GRPCTransport) PeerID() p2p.PeerID { return transport.peerID }

func (transport *GRPCTransport) Address() string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	return transport.listenAddr
}

// SetPeer configures an operator-trusted peer. Configuring a peer that was
// only learned promotes it. When the peer table is full the oldest configured
// peer is replaced, because this call is operator intent; learned candidates
// are dropped first to make room.
func (transport *GRPCTransport) SetPeer(peerID p2p.PeerID, address string) {
	if peerID == "" || address == "" || peerID == transport.peerID {
		return
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.dropLearnedLocked(peerID)
	if _, exists := transport.peers[peerID]; !exists {
		if transport.maxPeers > 0 && len(transport.peers) >= transport.maxPeers {
			transport.evictPeerLocked(transport.peerOrder[0])
		}
		transport.peerOrder = append(transport.peerOrder, peerID)
	}
	transport.closePeerSessionLocked(peerID)
	transport.closePeerConnectionLocked(peerID)
	delete(transport.backoffUntil, peerID)
	delete(transport.failures, peerID)
	transport.peers[peerID] = address
	transport.trimLearnedLocked()
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_configured", PeerID: peerID, Address: address})
}

func (transport *GRPCTransport) SetPeerLearnedHook(hook func(p2p.PeerID, string)) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.peerLearned = hook
}

func (transport *GRPCTransport) SetPeerDialHooks(attempted func(p2p.PeerID), result func(p2p.PeerID, bool)) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.peerAttempted = attempted
	transport.peerDialResult = result
}

func (transport *GRPCTransport) SetPeerEventHook(hook func(PeerEvent)) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.peerEvent = hook
}

func (transport *GRPCTransport) SetPeerGate(gate func(context.Context, p2p.PeerID) error) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.peerGates = compactPeerGates(gate)
}

func (transport *GRPCTransport) AddPeerGate(gate func(context.Context, p2p.PeerID) error) {
	if gate == nil {
		return
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.peerGates = append(transport.peerGates, gate)
}

func (transport *GRPCTransport) DisconnectPeer(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.closePeerSessionLocked(peerID)
	transport.closePeerConnectionLocked(peerID)
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_disconnected", PeerID: peerID, Reason: "disconnect_requested"})
}

// RemovePeer forgets a peer, configured or learned, and closes its session.
func (transport *GRPCTransport) RemovePeer(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if _, learned := transport.learned[peerID]; learned {
		transport.dropLearnedLocked(peerID)
		return
	}
	transport.evictPeerLocked(peerID)
}

// KnownPeers returns every address the transport can dial: configured peers
// and learned candidates.
func (transport *GRPCTransport) KnownPeers() map[p2p.PeerID]string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	peers := make(map[p2p.PeerID]string, len(transport.peers)+len(transport.learned))
	for peerID, address := range transport.learned {
		peers[peerID] = address
	}
	for peerID, address := range transport.peers {
		peers[peerID] = address
	}
	return peers
}

// ConfiguredPeerIDs returns the operator-configured peers.
func (transport *GRPCTransport) ConfiguredPeerIDs() []p2p.PeerID {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	peers := make([]p2p.PeerID, 0, len(transport.peers))
	for peerID := range transport.peers {
		peers = append(peers, peerID)
	}
	sort.Slice(peers, func(left int, right int) bool { return peers[left] < peers[right] })
	return peers
}

func (transport *GRPCTransport) ActivePeerIDs() []p2p.PeerID {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	peers := make([]p2p.PeerID, 0, len(transport.sessions))
	for peerID, session := range transport.sessions {
		if session != nil {
			peers = append(peers, peerID)
		}
	}
	sort.Slice(peers, func(left int, right int) bool { return peers[left] < peers[right] })
	return peers
}

func (transport *GRPCTransport) DroppedMessages() uint64 {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	return transport.droppedMessages
}

func (transport *GRPCTransport) AuthTokenConfigured() bool {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	return transport.authToken != ""
}

func (transport *GRPCTransport) Gossip(stream grpc.BidiStreamingServer[grpcStreamMessage, grpcStreamMessage]) error {
	release, err := transport.acquireInboundHandshake()
	if err != nil {
		transport.emitPeerEvent(PeerEvent{Type: "peer_handshake_failed", Reason: err.Error()})
		return err
	}
	deadline := time.NewTimer(transport.handshakeTimeout)
	hello, remote, err := transport.serveHandshake(stream, deadlineRecv(stream, deadline.C))
	deadline.Stop()
	release()
	if err != nil {
		event := PeerEvent{Type: "peer_handshake_failed", Reason: err.Error()}
		if hello != nil {
			event.PeerID, event.Address = hello.NodeID, hello.ListenAddr
		}
		transport.emitPeerEvent(event)
		return err
	}
	remotePeerID := remote.NodeID
	transport.learnHandshakePeers(remote)
	transport.emitPeerEvent(PeerEvent{Type: "peer_connected", PeerID: remotePeerID, Address: remote.ListenAddr, Direction: "inbound"})
	ctx := stream.Context()
	for {
		message, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if message.Envelope == nil {
			continue
		}
		envelope := Envelope{
			Topic: message.Envelope.Topic,
			From:  message.Envelope.From,
			To:    message.Envelope.To,
			Data:  append([]byte(nil), message.Envelope.Data...),
		}
		if envelope.From != remotePeerID {
			return fmt.Errorf("%w: envelope from %s does not match handshake peer %s", ErrHandshakeFailed, envelope.From, remotePeerID)
		}
		if err := transport.checkPeerGate(ctx, envelope.From); err != nil {
			return err
		}
		if err := transport.validatePayloadSize(envelope.Data); err != nil {
			return err
		}
		if envelope.To != "" && envelope.To != transport.peerID {
			continue
		}
		transport.deliver(ctx, envelope)
	}
}

func (transport *GRPCTransport) sendEnvelope(ctx context.Context, peerID p2p.PeerID, address string, envelope Envelope) error {
	ctx, cancel := context.WithTimeout(ctx, transport.dialTimeout)
	defer cancel()
	if err := transport.checkPeerGate(ctx, peerID); err != nil {
		return err
	}
	if err := transport.checkPeerBackoff(peerID); err != nil {
		return err
	}
	err := transport.sendEnvelopeOnce(ctx, peerID, address, envelope)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPeerDialInProgress) || errors.Is(err, ErrMessageTooLarge) {
		return err
	}
	if isPermanentHandshakeError(err) {
		transport.markPeerBackoff(peerID)
		return err
	}
	transport.closePeerSession(peerID)
	if retryErr := transport.sendEnvelopeOnce(ctx, peerID, address, envelope); retryErr != nil {
		transport.markPeerBackoff(peerID)
		return retryErr
	}
	return nil
}

// isPermanentHandshakeError reports failures that retrying the same peer with
// the same configuration cannot fix.
func isPermanentHandshakeError(err error) bool {
	return errors.Is(err, ErrHandshakeFailed) ||
		errors.Is(err, ErrProtocolMismatch) ||
		errors.Is(err, ErrNetworkMismatch) ||
		errors.Is(err, ErrChainIDMismatch) ||
		errors.Is(err, ErrGenesisHashMismatch) ||
		errors.Is(err, ErrAuthTokenMismatch) ||
		errors.Is(err, ErrHandshakeSignature) ||
		errors.Is(err, ErrNodeIDMismatch)
}

func (transport *GRPCTransport) sendEnvelopeOnce(ctx context.Context, peerID p2p.PeerID, address string, envelope Envelope) error {
	session, err := transport.peerSession(ctx, peerID, address)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := session.stream.Send(&grpcStreamMessage{Envelope: &grpcEnvelope{Topic: envelope.Topic, From: envelope.From, To: envelope.To, Data: append([]byte(nil), envelope.Data...)}}); err != nil {
		transport.closePeerSession(peerID)
		return err
	}
	return nil
}

func (transport *GRPCTransport) peerConnection(ctx context.Context, peerID p2p.PeerID, address string) (*grpc.ClientConn, error) {
	transport.mu.RLock()
	connection := transport.connections[peerID]
	transport.mu.RUnlock()
	if connection != nil {
		return connection, nil
	}
	connection, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(transport.transportCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcBinaryCodec{})),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(transport.maxMessageBytes))),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(int(transport.maxMessageBytes))),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if existing := transport.connections[peerID]; existing != nil {
		_ = connection.Close()
		return existing, nil
	}
	transport.connections[peerID] = connection
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_connection_opened", PeerID: peerID, Address: address, Direction: "outbound"})
	return connection, nil
}

func (transport *GRPCTransport) transportCredentials() credentials.TransportCredentials {
	if transport.tlsConfig == nil {
		return insecure.NewCredentials()
	}
	return credentials.NewTLS(transport.tlsConfig.Clone())
}

// cloneGRPCTLSConfig copies config and pins the minimum version to TLS 1.3,
// the version whose exporter secrets bind the handshake to the TLS session.
func cloneGRPCTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return nil
	}
	cloned := config.Clone()
	if cloned.MinVersion < tls.VersionTLS13 {
		cloned.MinVersion = tls.VersionTLS13
	}
	return cloned
}

func (transport *GRPCTransport) peerSession(ctx context.Context, peerID p2p.PeerID, address string) (*grpcPeerSession, error) {
	lock, err := transport.lockPeerSession(ctx, peerID)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := transport.checkPeerGate(ctx, peerID); err != nil {
		return nil, err
	}
	transport.mu.RLock()
	session := transport.sessions[peerID]
	rootCtx := transport.rootCtx
	transport.mu.RUnlock()
	if session != nil {
		return session, nil
	}
	if rootCtx == nil {
		return nil, ErrTransportClosed
	}
	connection, err := transport.peerConnection(ctx, peerID, address)
	if err != nil {
		return nil, fmt.Errorf("dial peer connection: %w", err)
	}
	streamCtx, cancel := context.WithCancel(rootCtx)
	handshakeTimer := time.AfterFunc(transport.dialTimeout, cancel)
	failHandshake := func() {
		handshakeTimer.Stop()
		cancel()
		transport.closePeerConnection(peerID)
	}
	client := newP2PTransportClient(connection)
	stream, err := client.Gossip(streamCtx)
	if err != nil {
		failHandshake()
		return nil, fmt.Errorf("open gossip stream: %w", err)
	}
	remote, err := transport.dialHandshake(stream, peerID)
	if err != nil {
		failHandshake()
		return nil, err
	}
	if !handshakeTimer.Stop() || streamCtx.Err() != nil {
		failHandshake()
		return nil, fmt.Errorf("handshake deadline exceeded: %w", context.DeadlineExceeded)
	}
	transport.learnHandshakePeers(remote)
	session = &grpcPeerSession{stream: stream, cancel: cancel}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if existing := transport.sessions[peerID]; existing != nil {
		_ = stream.CloseSend()
		cancel()
		return existing, nil
	}
	transport.sessions[peerID] = session
	delete(transport.backoffUntil, peerID)
	delete(transport.failures, peerID)
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_connected", PeerID: peerID, Address: address, Direction: "outbound"})
	go transport.monitorPeerSession(peerID, session)
	return session, nil
}

func (transport *GRPCTransport) lockPeerSession(ctx context.Context, peerID p2p.PeerID) (*sync.Mutex, error) {
	lock := transport.peerSessionLock(peerID)
	for {
		if lock.TryLock() {
			return lock, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %v", ErrPeerDialInProgress, ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (transport *GRPCTransport) peerSessionLock(peerID p2p.PeerID) *sync.Mutex {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	lock := transport.sessionLocks[peerID]
	if lock == nil {
		lock = &sync.Mutex{}
		transport.sessionLocks[peerID] = lock
	}
	return lock
}

func (transport *GRPCTransport) monitorPeerSession(peerID p2p.PeerID, session *grpcPeerSession) {
	for {
		if _, err := session.stream.Recv(); err != nil {
			transport.closePeerSessionIfCurrent(peerID, session, err.Error())
			return
		}
	}
}

func (transport *GRPCTransport) closePeerSession(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.closePeerSessionLocked(peerID)
}

func (transport *GRPCTransport) closePeerConnection(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.closePeerConnectionLocked(peerID)
}

func (transport *GRPCTransport) closePeerSessionIfCurrent(peerID p2p.PeerID, session *grpcPeerSession, reason string) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.sessions[peerID] != session {
		return
	}
	transport.closePeerSessionWithReasonLocked(peerID, reason)
}

func (transport *GRPCTransport) closePeerSessionLocked(peerID p2p.PeerID) {
	transport.closePeerSessionWithReasonLocked(peerID, "")
}

func (transport *GRPCTransport) closePeerSessionWithReasonLocked(peerID p2p.PeerID, reason string) {
	session := transport.sessions[peerID]
	if session == nil {
		return
	}
	_ = session.stream.CloseSend()
	session.cancel()
	delete(transport.sessions, peerID)
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_disconnected", PeerID: peerID, Reason: reason})
}

func (transport *GRPCTransport) closePeerConnectionLocked(peerID p2p.PeerID) {
	if connection := transport.connections[peerID]; connection != nil {
		_ = connection.Close()
		delete(transport.connections, peerID)
	}
}

func (transport *GRPCTransport) checkPeerBackoff(peerID p2p.PeerID) error {
	transport.mu.RLock()
	until := transport.backoffUntil[peerID]
	transport.mu.RUnlock()
	if until.IsZero() || time.Now().After(until) {
		return nil
	}
	return fmt.Errorf("%w: peer=%s until=%s", ErrPeerBackoffActive, peerID, until.UTC().Format(time.RFC3339Nano))
}

func (transport *GRPCTransport) checkPeerGate(ctx context.Context, peerID p2p.PeerID) error {
	if peerID == "" {
		return nil
	}
	transport.mu.RLock()
	gates := append([]func(context.Context, p2p.PeerID) error(nil), transport.peerGates...)
	transport.mu.RUnlock()
	for _, gate := range gates {
		if err := gate(ctx, peerID); err != nil {
			return fmt.Errorf("%w: peer=%s: %v", ErrPeerRejected, peerID, err)
		}
	}
	return nil
}

func compactPeerGates(gates ...func(context.Context, p2p.PeerID) error) []func(context.Context, p2p.PeerID) error {
	compacted := make([]func(context.Context, p2p.PeerID) error, 0, len(gates))
	for _, gate := range gates {
		if gate != nil {
			compacted = append(compacted, gate)
		}
	}
	return compacted
}

// markPeerBackoff records a failure and delays the next dial exponentially:
// the delay starts at the configured base, doubles with every consecutive
// failure, and stops growing at maxReconnectBackoff (or the base when that is
// larger). A successful session resets it.
func (transport *GRPCTransport) markPeerBackoff(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.failures[peerID]++
	until := time.Now().Add(transport.backoffDelay(transport.failures[peerID]))
	transport.backoffUntil[peerID] = until
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_backoff", PeerID: peerID, BackoffUntil: until})
}

func (transport *GRPCTransport) backoffDelay(failures int) time.Duration {
	limit := max(transport.reconnectBackoff, maxReconnectBackoff)
	delay := transport.reconnectBackoff
	for step := 1; step < failures && delay < limit; step++ {
		delay *= 2
	}
	return min(delay, limit)
}

func (transport *GRPCTransport) evictPeerLocked(peerID p2p.PeerID) {
	transport.closePeerSessionLocked(peerID)
	transport.closePeerConnectionLocked(peerID)
	delete(transport.peers, peerID)
	delete(transport.backoffUntil, peerID)
	delete(transport.failures, peerID)
	delete(transport.sessionLocks, peerID)
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_removed", PeerID: peerID})
	for index, existing := range transport.peerOrder {
		if existing == peerID {
			transport.peerOrder = append(transport.peerOrder[:index], transport.peerOrder[index+1:]...)
			return
		}
	}
}

func (transport *GRPCTransport) emitPeerEvent(event PeerEvent) {
	transport.mu.RLock()
	hook := transport.peerEvent
	transport.mu.RUnlock()
	if hook != nil {
		hook(event)
	}
}

func (transport *GRPCTransport) emitPeerEventLocked(event PeerEvent) {
	if transport.peerEvent != nil {
		transport.peerEvent(event)
	}
}

func (transport *GRPCTransport) runReconnectLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	transport.reconnectPeers(ctx)
	ticker := time.NewTicker(transport.reconnectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			transport.reconnectPeers(ctx)
		}
	}
}

// reconnectPeers dials every peer that has no session and is not backing off.
// Peers that are already connected are left alone, so a healthy mesh costs no
// dials and no address book writes.
func (transport *GRPCTransport) reconnectPeers(ctx context.Context) {
	var wait sync.WaitGroup
	for _, target := range transport.dialTargets(time.Now()) {
		wait.Add(1)
		go func(target dialTarget) {
			defer wait.Done()
			transport.reconnectPeer(ctx, target)
		}(target)
	}
	wait.Wait()
}

func (transport *GRPCTransport) reconnectPeer(ctx context.Context, target dialTarget) {
	transport.notifyPeerAttempted(target.peerID)
	dialCtx, cancel := context.WithTimeout(ctx, transport.dialTimeout)
	_, err := transport.peerSession(dialCtx, target.peerID, target.address)
	cancel()
	if errors.Is(err, ErrPeerDialInProgress) || ctx.Err() != nil {
		return
	}
	if err == nil {
		transport.notifyPeerDialResult(target.peerID, true)
		return
	}
	transport.notifyPeerDialResult(target.peerID, false)
	transport.emitPeerEvent(PeerEvent{Type: "peer_dial_failed", PeerID: target.peerID, Address: target.address, Reason: err.Error()})
	if target.learned {
		transport.recordLearnedFailure(target.peerID, err)
		return
	}
	if !isPermanentHandshakeError(err) {
		transport.markPeerBackoff(target.peerID)
	}
}

func (transport *GRPCTransport) notifyPeerAttempted(peerID p2p.PeerID) {
	transport.mu.RLock()
	hook := transport.peerAttempted
	transport.mu.RUnlock()
	if hook != nil {
		hook(peerID)
	}
}

func (transport *GRPCTransport) notifyPeerDialResult(peerID p2p.PeerID, success bool) {
	transport.mu.RLock()
	hook := transport.peerDialResult
	transport.mu.RUnlock()
	if hook != nil {
		hook(peerID, success)
	}
}

func (transport *GRPCTransport) validatePayloadSize(data []byte) error {
	if transport.maxMessageBytes == 0 {
		return nil
	}
	if uint64(len(data)) > transport.maxMessageBytes {
		return fmt.Errorf("%w: size=%d limit=%d", ErrMessageTooLarge, len(data), transport.maxMessageBytes)
	}
	return nil
}

// deliver hands envelope to every subscriber of its topic. Consensus topics
// wait for buffer space until ctx ends or the subscription is torn down; every
// other topic is dropped and counted when a subscriber is behind.
func (transport *GRPCTransport) deliver(ctx context.Context, envelope Envelope) {
	transport.mu.RLock()
	subscribers := append([]*subscription(nil), transport.subscribers[envelope.Topic]...)
	transport.mu.RUnlock()
	reliable := isReliableTopic(envelope.Topic)
	for _, subscriber := range subscribers {
		if subscriber.deliver(ctx, cloneEnvelope(envelope), reliable) == deliveryDropped {
			transport.recordDroppedMessage()
		}
	}
}

func isReliableTopic(topic p2p.Topic) bool {
	return topic == p2p.TopicProposal || topic == p2p.TopicVote || topic == p2p.TopicTimeout
}

func (transport *GRPCTransport) recordDroppedMessage() {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.droppedMessages++
}

func (transport *GRPCTransport) ensureStarted() error {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if !transport.started || transport.server == nil {
		return ErrTransportClosed
	}
	return nil
}

// peerAddress resolves a configured or learned peer.
func (transport *GRPCTransport) peerAddress(peerID p2p.PeerID) (string, bool) {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if address, ok := transport.peers[peerID]; ok {
		return address, true
	}
	address, ok := transport.learned[peerID]
	return address, ok
}

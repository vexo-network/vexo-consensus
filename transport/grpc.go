package transport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	GRPCProtocolVersion      = "vexo-p2p/1"
	grpcCodecName            = "vexo-binary"
	grpcCodecVersion         = byte(2)
	grpcCodecVersionV1       = byte(1)
	defaultGRPCDialTimeout   = 3 * time.Second
	defaultReconnectInterval = time.Duration(500_000_000)
	defaultMaxMessageBytes   = 4 * 1024 * 1024
	defaultSubscriberBuffer  = 32
	handshakeAuthTTL         = 2 * time.Minute
	handshakeAuthVersion     = "v2"
)

var (
	ErrHandshakeFailed     = errors.New("p2p handshake failed")
	ErrProtocolMismatch    = errors.New("protocol version mismatch")
	ErrNetworkMismatch     = errors.New("network id mismatch")
	ErrChainIDMismatch     = errors.New("chain id mismatch")
	ErrGenesisHashMismatch = errors.New("genesis hash mismatch")
	ErrAuthTokenMismatch   = errors.New("p2p auth token mismatch")
	ErrHandshakeSignature  = errors.New("p2p handshake signature invalid")
	ErrMessageTooLarge     = errors.New("p2p message too large")
	ErrPeerBackoffActive   = errors.New("peer reconnect backoff active")
	ErrPeerDialInProgress  = errors.New("peer dial already in progress")
	ErrTLSRequired         = errors.New("p2p tls is required")
	ErrAuthTokenRequired   = errors.New("p2p auth token is required")
	ErrAuthNonceReplay     = errors.New("p2p auth nonce replay")
	ErrAuthReplayStore     = errors.New("p2p auth replay store is required")
)

type Handshake struct {
	ProtocolVersion string                `json:"protocol_version"`
	NetworkID       string                `json:"network_id"`
	ChainID         string                `json:"chain_id"`
	GenesisHash     string                `json:"genesis_hash"`
	NodeID          p2p.PeerID            `json:"node_id"`
	ListenAddr      string                `json:"listen_addr,omitempty"`
	AuthToken       string                `json:"auth_token,omitempty"`
	SignatureNonce  string                `json:"signature_nonce,omitempty"`
	NodePublicKey   types.PublicKey       `json:"node_public_key,omitempty"`
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

type AuthReplayStore interface {
	MarkAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string, expires time.Time, now time.Time) error
}

type GRPCConfig struct {
	PeerID                    p2p.PeerID
	ListenAddr                string
	Peers                     map[p2p.PeerID]string
	DialTimeout               time.Duration
	ProtocolVersion           string
	NetworkID                 string
	ChainID                   string
	GenesisHash               string
	MaxMessageBytes           uint64
	TLSConfig                 *tls.Config
	RequireTLS                bool
	MaxPeers                  int
	ReconnectBackoff          time.Duration
	ReconnectInterval         time.Duration
	SubscriberBuffer          int
	AuthToken                 string
	AuthReplayStore           AuthReplayStore
	RequireAuthReplayStore    bool
	PeerLearned               func(p2p.PeerID, string)
	PeerAttempted             func(p2p.PeerID)
	PeerDialResult            func(p2p.PeerID, bool)
	PeerEvent                 func(PeerEvent)
	PeerGate                  func(context.Context, p2p.PeerID) error
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
	reconnectBackoff          time.Duration
	reconnectInterval         time.Duration
	subscriberBuffer          int
	authToken                 string
	authReplayStore           AuthReplayStore
	peerLearned               func(p2p.PeerID, string)
	peerAttempted             func(p2p.PeerID)
	peerDialResult            func(p2p.PeerID, bool)
	peerEvent                 func(PeerEvent)
	peerGates                 []func(context.Context, p2p.PeerID) error
	handshakeSigner           HandshakeSigner
	handshakeVerifier         HandshakeSignatureVerifier
	requireHandshakeSignature bool

	mu              sync.RWMutex
	listener        net.Listener
	server          *grpc.Server
	started         bool
	rootCtx         context.Context
	rootCancel      context.CancelFunc
	reconnectCancel context.CancelFunc
	reconnectDone   chan struct{}
	peers           map[p2p.PeerID]string
	peerOrder       []p2p.PeerID
	connections     map[p2p.PeerID]*grpc.ClientConn
	sessions        map[p2p.PeerID]*grpcPeerSession
	sessionLocks    map[p2p.PeerID]*sync.Mutex
	backoffUntil    map[p2p.PeerID]time.Time
	authNonces      map[string]time.Time
	subscribers     map[p2p.Topic][]chan Envelope
	droppedMessages uint64
}

type grpcPeerSession struct {
	stream grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage]
	cancel context.CancelFunc
	mu     sync.Mutex
}

type grpcEnvelope struct {
	Topic p2p.Topic  `json:"topic"`
	From  p2p.PeerID `json:"from"`
	To    p2p.PeerID `json:"to,omitempty"`
	Data  []byte     `json:"data"`
}

type grpcStreamMessage struct {
	Handshake *Handshake    `json:"handshake,omitempty"`
	Envelope  *grpcEnvelope `json:"envelope,omitempty"`
}

type grpcBinaryCodec struct{}

func init() {
	encoding.RegisterCodec(grpcBinaryCodec{})
}

func (grpcBinaryCodec) Name() string { return grpcCodecName }

func (grpcBinaryCodec) Marshal(value any) ([]byte, error) {
	switch message := value.(type) {
	case *grpcStreamMessage:
		return encodeGRPCStreamMessage(message)
	case grpcStreamMessage:
		return encodeGRPCStreamMessage(&message)
	default:
		return nil, fmt.Errorf("unsupported grpc codec value %T", value)
	}
}

func (grpcBinaryCodec) Unmarshal(data []byte, value any) error {
	message, ok := value.(*grpcStreamMessage)
	if !ok {
		return fmt.Errorf("unsupported grpc codec target %T", value)
	}
	decoded, err := decodeGRPCStreamMessage(data)
	if err != nil {
		return err
	}
	*message = *decoded
	return nil
}

func encodeGRPCStreamMessage(message *grpcStreamMessage) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte(grpcCodecVersion)
	if message != nil && message.Handshake != nil {
		buffer.WriteByte(1)
		writeBinaryString(&buffer, message.Handshake.ProtocolVersion)
		writeBinaryString(&buffer, message.Handshake.NetworkID)
		writeBinaryString(&buffer, message.Handshake.ChainID)
		writeBinaryString(&buffer, message.Handshake.GenesisHash)
		writeBinaryString(&buffer, string(message.Handshake.NodeID))
		writeBinaryString(&buffer, message.Handshake.ListenAddr)
		writeBinaryString(&buffer, message.Handshake.AuthToken)
		writeBinaryString(&buffer, message.Handshake.SignatureNonce)
		writeBinaryBytes(&buffer, message.Handshake.NodePublicKey)
		writeBinaryBytes(&buffer, message.Handshake.Signature)
		writeBinaryPeerMap(&buffer, message.Handshake.KnownPeers)
	} else {
		buffer.WriteByte(0)
	}
	if message != nil && message.Envelope != nil {
		buffer.WriteByte(1)
		writeBinaryString(&buffer, string(message.Envelope.Topic))
		writeBinaryString(&buffer, string(message.Envelope.From))
		writeBinaryString(&buffer, string(message.Envelope.To))
		writeBinaryBytes(&buffer, message.Envelope.Data)
	} else {
		buffer.WriteByte(0)
	}
	return buffer.Bytes(), nil
}

func decodeGRPCStreamMessage(data []byte) (*grpcStreamMessage, error) {
	reader := bytes.NewReader(data)
	version, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != grpcCodecVersion && version != grpcCodecVersionV1 {
		return nil, fmt.Errorf("unsupported grpc codec version %d", version)
	}
	message := &grpcStreamMessage{}
	hasHandshake, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch hasHandshake {
	case 0:
	case 1:
		protocolVersion, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		networkID, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		chainID, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		genesisHash, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		nodeID, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		listenAddr, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		authToken, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		var signatureNonce string
		var nodePublicKey []byte
		var signature []byte
		if version >= grpcCodecVersion {
			signatureNonce, err = readBinaryString(reader)
			if err != nil {
				return nil, err
			}
			nodePublicKey, err = readBinaryBytes(reader)
			if err != nil {
				return nil, err
			}
			signature, err = readBinaryBytes(reader)
			if err != nil {
				return nil, err
			}
		}
		knownPeers, err := readBinaryPeerMap(reader)
		if err != nil {
			return nil, err
		}
		message.Handshake = &Handshake{
			ProtocolVersion: protocolVersion,
			NetworkID:       networkID,
			ChainID:         chainID,
			GenesisHash:     genesisHash,
			NodeID:          p2p.PeerID(nodeID),
			ListenAddr:      listenAddr,
			AuthToken:       authToken,
			SignatureNonce:  signatureNonce,
			NodePublicKey:   types.PublicKey(nodePublicKey),
			Signature:       types.Signature(signature),
			KnownPeers:      knownPeers,
		}
	default:
		return nil, fmt.Errorf("invalid handshake flag %d", hasHandshake)
	}
	hasEnvelope, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch hasEnvelope {
	case 0:
	case 1:
		topic, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		from, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		to, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		payload, err := readBinaryBytes(reader)
		if err != nil {
			return nil, err
		}
		message.Envelope = &grpcEnvelope{
			Topic: p2p.Topic(topic),
			From:  p2p.PeerID(from),
			To:    p2p.PeerID(to),
			Data:  payload,
		}
	default:
		return nil, fmt.Errorf("invalid envelope flag %d", hasEnvelope)
	}
	if reader.Len() != 0 {
		return nil, errors.New("trailing grpc codec bytes")
	}
	return message, nil
}

func writeBinaryString(writer io.Writer, value string) {
	writeBinaryBytes(writer, []byte(value))
}

func writeBinaryBytes(writer io.Writer, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func writeBinaryPeerMap(writer io.Writer, peers map[p2p.PeerID]string) {
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(peers)))
	_, _ = writer.Write(count[:])
	peerIDs := make([]string, 0, len(peers))
	for peerID := range peers {
		peerIDs = append(peerIDs, string(peerID))
	}
	sort.Strings(peerIDs)
	for _, peerID := range peerIDs {
		writeBinaryString(writer, peerID)
		writeBinaryString(writer, peers[p2p.PeerID(peerID)])
	}
}

func readBinaryString(reader io.Reader) (string, error) {
	value, err := readBinaryBytes(reader)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func readBinaryPeerMap(reader io.Reader) (map[p2p.PeerID]string, error) {
	var countBytes [4]byte
	if _, err := io.ReadFull(reader, countBytes[:]); err != nil {
		return nil, err
	}
	count := binary.BigEndian.Uint32(countBytes[:])
	if count > 4096 {
		return nil, ErrMessageTooLarge
	}
	peers := make(map[p2p.PeerID]string, int(count))
	for index := uint32(0); index < count; index++ {
		peerID, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		address, err := readBinaryString(reader)
		if err != nil {
			return nil, err
		}
		if peerID != "" && address != "" {
			peers[p2p.PeerID(peerID)] = address
		}
	}
	return peers, nil
}

func readBinaryBytes(reader io.Reader) ([]byte, error) {
	var lengthBytes [4]byte
	if _, err := io.ReadFull(reader, lengthBytes[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBytes[:])
	if length > defaultMaxMessageBytes {
		return nil, ErrMessageTooLarge
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(reader, value); err != nil {
		return nil, err
	}
	return value, nil
}

func NewGRPCTransport(config GRPCConfig) (*GRPCTransport, error) {
	if config.PeerID == "" {
		return nil, ErrPeerIDRequired
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
	peers := make(map[p2p.PeerID]string, len(config.Peers))
	peerOrder := make([]p2p.PeerID, 0, len(config.Peers))
	for peerID, address := range config.Peers {
		if peerID == "" || address == "" {
			continue
		}
		if config.MaxPeers > 0 && len(peers) >= config.MaxPeers {
			continue
		}
		peers[peerID] = address
		peerOrder = append(peerOrder, peerID)
	}
	return &GRPCTransport{
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
		reconnectBackoff:          config.ReconnectBackoff,
		reconnectInterval:         config.ReconnectInterval,
		subscriberBuffer:          config.SubscriberBuffer,
		authToken:                 config.AuthToken,
		authReplayStore:           config.AuthReplayStore,
		peerLearned:               config.PeerLearned,
		peerAttempted:             config.PeerAttempted,
		peerDialResult:            config.PeerDialResult,
		peerEvent:                 config.PeerEvent,
		peerGates:                 compactPeerGates(config.PeerGate),
		handshakeSigner:           config.HandshakeSigner,
		handshakeVerifier:         config.HandshakeVerifier,
		requireHandshakeSignature: config.RequireHandshakeSignature,
		peers:                     peers,
		peerOrder:                 peerOrder,
		connections:               make(map[p2p.PeerID]*grpc.ClientConn),
		sessions:                  make(map[p2p.PeerID]*grpcPeerSession),
		sessionLocks:              make(map[p2p.PeerID]*sync.Mutex),
		backoffUntil:              make(map[p2p.PeerID]time.Time),
		authNonces:                make(map[string]time.Time),
		subscribers:               make(map[p2p.Topic][]chan Envelope),
	}, nil
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
	for _, subscribers := range transport.subscribers {
		for _, subscriber := range subscribers {
			close(subscriber)
		}
	}
	transport.subscribers = make(map[p2p.Topic][]chan Envelope)
	transport.mu.Unlock()
	if reconnectCancel != nil {
		reconnectCancel()
	}
	if rootCancel != nil {
		rootCancel()
	}
	if reconnectDone != nil {
		select {
		case <-reconnectDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if server != nil {
		server.Stop()
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
	return nil
}

func (transport *GRPCTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	if err := transport.validatePayloadSize(data); err != nil {
		return err
	}
	peers := transport.peerAddresses()
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

func (transport *GRPCTransport) Subscribe(ctx context.Context, topic p2p.Topic) (<-chan Envelope, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if err := transport.ensureStarted(); err != nil {
		return nil, err
	}
	channel := make(chan Envelope, transport.subscriberBuffer)
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if !transport.started {
		close(channel)
		return nil, ErrTransportClosed
	}
	transport.subscribers[topic] = append(transport.subscribers[topic], channel)
	return channel, nil
}

func (transport *GRPCTransport) PeerID() p2p.PeerID { return transport.peerID }

func (transport *GRPCTransport) Address() string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	return transport.listenAddr
}

func (transport *GRPCTransport) SetPeer(peerID p2p.PeerID, address string) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if peerID == "" || address == "" {
		return
	}
	if _, exists := transport.peers[peerID]; !exists {
		if transport.maxPeers > 0 && len(transport.peers) >= transport.maxPeers {
			transport.evictPeerLocked(transport.peerOrder[0])
		}
		transport.peerOrder = append(transport.peerOrder, peerID)
	}
	transport.closePeerSessionLocked(peerID)
	transport.closePeerConnectionLocked(peerID)
	delete(transport.backoffUntil, peerID)
	transport.peers[peerID] = address
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

func (transport *GRPCTransport) RemovePeer(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.evictPeerLocked(peerID)
}

func (transport *GRPCTransport) KnownPeers() map[p2p.PeerID]string {
	return transport.peerAddresses()
}

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

func (transport *GRPCTransport) LocalHandshake() Handshake {
	transport.mu.RLock()
	listenAddr := transport.listenAddr
	signer := transport.handshakeSigner
	knownPeers := make(map[p2p.PeerID]string, len(transport.peers)+1)
	for peerID, address := range transport.peers {
		knownPeers[peerID] = address
	}
	if listenAddr != "" {
		knownPeers[transport.peerID] = listenAddr
	}
	transport.mu.RUnlock()
	handshake := Handshake{
		ProtocolVersion: transport.protocolVersion,
		NetworkID:       transport.networkID,
		ChainID:         transport.chainID,
		GenesisHash:     transport.genesisHash,
		NodeID:          transport.peerID,
		ListenAddr:      listenAddr,
		AuthToken:       transport.authProof(transport.peerID),
		KnownPeers:      knownPeers,
	}
	if signer != nil {
		signatureNonce, err := randomAuthNonce()
		if err == nil {
			handshake.SignatureNonce = signatureNonce
			handshake.NodePublicKey = append(types.PublicKey(nil), signer.PublicKey()...)
			if signature, err := signer.Sign(handshakeSignaturePayload(handshake)); err == nil {
				handshake.Signature = append(types.Signature(nil), signature...)
			}
		}
	}
	return handshake
}

func (transport *GRPCTransport) Gossip(stream grpc.BidiStreamingServer[grpcStreamMessage, grpcStreamMessage]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.Handshake == nil {
		transport.emitPeerEvent(PeerEvent{Type: "peer_handshake_failed", Reason: "missing handshake"})
		return fmt.Errorf("%w: missing handshake", ErrHandshakeFailed)
	}
	if err := transport.validateHandshake(stream.Context(), *first.Handshake); err != nil {
		transport.emitPeerEvent(PeerEvent{Type: "peer_handshake_failed", PeerID: first.Handshake.NodeID, Address: first.Handshake.ListenAddr, Reason: err.Error()})
		return err
	}
	remotePeerID := first.Handshake.NodeID
	if err := transport.checkPeerGate(stream.Context(), remotePeerID); err != nil {
		transport.emitPeerEvent(PeerEvent{Type: "peer_handshake_failed", PeerID: remotePeerID, Address: first.Handshake.ListenAddr, Reason: err.Error()})
		return err
	}
	transport.learnHandshakePeers(*first.Handshake)
	if err := stream.Send(&grpcStreamMessage{Handshake: ptrHandshake(transport.LocalHandshake())}); err != nil {
		return err
	}
	transport.emitPeerEvent(PeerEvent{Type: "peer_connected", PeerID: remotePeerID, Address: first.Handshake.ListenAddr, Direction: "inbound"})
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
		if err := transport.checkPeerGate(stream.Context(), envelope.From); err != nil {
			return err
		}
		if err := transport.validatePayloadSize(envelope.Data); err != nil {
			return err
		}
		if envelope.From == transport.peerID {
			continue
		}
		if envelope.To != "" && envelope.To != transport.peerID {
			continue
		}
		transport.deliver(envelope)
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
	if err := transport.sendEnvelopeOnce(ctx, peerID, address, envelope); err != nil {
		if errors.Is(err, ErrPeerDialInProgress) {
			return err
		}
		if errors.Is(err, ErrMessageTooLarge) || errors.Is(err, ErrHandshakeFailed) || errors.Is(err, ErrProtocolMismatch) || errors.Is(err, ErrNetworkMismatch) || errors.Is(err, ErrChainIDMismatch) || errors.Is(err, ErrGenesisHashMismatch) {
			return err
		}
		transport.closePeerSession(peerID)
		if retryErr := transport.sendEnvelopeOnce(ctx, peerID, address, envelope); retryErr != nil {
			transport.markPeerBackoff(peerID)
			return retryErr
		}
	}
	return nil
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

func cloneGRPCTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return nil
	}
	cloned := config.Clone()
	if cloned.MinVersion == 0 {
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
	transport.mu.RUnlock()
	if session != nil {
		return session, nil
	}
	connection, err := transport.peerConnection(ctx, peerID, address)
	if err != nil {
		return nil, fmt.Errorf("dial peer connection: %w", err)
	}
	transport.mu.RLock()
	rootCtx := transport.rootCtx
	transport.mu.RUnlock()
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	handshakeCtx, cancelHandshake := context.WithTimeout(rootCtx, transport.dialTimeout)
	defer cancelHandshake()
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
	if err := stream.Send(&grpcStreamMessage{Handshake: ptrHandshake(transport.LocalHandshake())}); err != nil {
		failHandshake()
		return nil, fmt.Errorf("send local handshake: %w", err)
	}
	remote, err := stream.Recv()
	if err != nil {
		failHandshake()
		return nil, fmt.Errorf("receive remote handshake: %w", normalizeGRPCError(err))
	}
	if remote.Handshake == nil {
		failHandshake()
		return nil, fmt.Errorf("%w: missing remote handshake", ErrHandshakeFailed)
	}
	if remote.Handshake.NodeID != peerID {
		failHandshake()
		return nil, fmt.Errorf("%w: expected peer %s got %s", ErrHandshakeFailed, peerID, remote.Handshake.NodeID)
	}
	if err := transport.validateHandshake(handshakeCtx, *remote.Handshake); err != nil {
		failHandshake()
		return nil, fmt.Errorf("validate remote handshake: %w", err)
	}
	if err := transport.checkPeerGate(handshakeCtx, remote.Handshake.NodeID); err != nil {
		failHandshake()
		return nil, fmt.Errorf("remote peer gate: %w", err)
	}
	transport.learnHandshakePeers(*remote.Handshake)
	if !handshakeTimer.Stop() || streamCtx.Err() != nil {
		failHandshake()
		return nil, fmt.Errorf("handshake deadline exceeded: %w", context.DeadlineExceeded)
	}
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

func (transport *GRPCTransport) markPeerBackoff(peerID p2p.PeerID) {
	if transport.reconnectBackoff <= 0 {
		return
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	until := time.Now().Add(transport.reconnectBackoff)
	transport.backoffUntil[peerID] = until
	transport.emitPeerEventLocked(PeerEvent{Type: "peer_backoff", PeerID: peerID, BackoffUntil: until})
}

func (transport *GRPCTransport) evictPeerLocked(peerID p2p.PeerID) {
	transport.closePeerSessionLocked(peerID)
	transport.closePeerConnectionLocked(peerID)
	delete(transport.peers, peerID)
	delete(transport.backoffUntil, peerID)
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

func (transport *GRPCTransport) learnHandshakePeers(handshake Handshake) {
	discovered := make(map[p2p.PeerID]string, len(handshake.KnownPeers)+1)
	if handshake.NodeID != "" && handshake.ListenAddr != "" {
		discovered[handshake.NodeID] = handshake.ListenAddr
	}
	for peerID, address := range handshake.KnownPeers {
		if peerID != "" && address != "" {
			discovered[peerID] = address
		}
	}
	learned := make(map[p2p.PeerID]string)
	transport.mu.Lock()
	for peerID, address := range discovered {
		if peerID == "" || peerID == transport.peerID || !p2p.ValidPeerAddress(address) {
			continue
		}
		if _, exists := transport.peers[peerID]; exists {
			continue
		}
		if transport.maxPeers > 0 && len(transport.peers) >= transport.maxPeers {
			transport.evictPeerLocked(transport.peerOrder[0])
		}
		transport.peers[peerID] = address
		transport.peerOrder = append(transport.peerOrder, peerID)
		learned[peerID] = address
	}
	hook := transport.peerLearned
	transport.mu.Unlock()
	if hook == nil {
		return
	}
	for peerID, address := range learned {
		hook(peerID, address)
	}
}

func normalizeGRPCError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	switch {
	case strings.Contains(message, ErrProtocolMismatch.Error()):
		return fmt.Errorf("%w: %v", ErrProtocolMismatch, err)
	case strings.Contains(message, ErrNetworkMismatch.Error()):
		return fmt.Errorf("%w: %v", ErrNetworkMismatch, err)
	case strings.Contains(message, ErrChainIDMismatch.Error()):
		return fmt.Errorf("%w: %v", ErrChainIDMismatch, err)
	case strings.Contains(message, ErrGenesisHashMismatch.Error()):
		return fmt.Errorf("%w: %v", ErrGenesisHashMismatch, err)
	case strings.Contains(message, ErrAuthTokenMismatch.Error()):
		return fmt.Errorf("%w: %v", ErrAuthTokenMismatch, err)
	case strings.Contains(message, ErrPeerRejected.Error()):
		return fmt.Errorf("%w: %v", ErrPeerRejected, err)
	case strings.Contains(message, ErrHandshakeFailed.Error()):
		return fmt.Errorf("%w: %v", ErrHandshakeFailed, err)
	default:
		return err
	}
}

func (transport *GRPCTransport) validateHandshake(ctx context.Context, handshake Handshake) error {
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
	if transport.authToken != "" || handshake.AuthToken != "" {
		if transport.authToken == "" || handshake.AuthToken == "" || !transport.verifyAuthProof(ctx, handshake.NodeID, handshake.AuthToken) {
			return ErrAuthTokenMismatch
		}
	}
	if handshake.NodeID == "" {
		return fmt.Errorf("%w: missing node id", ErrHandshakeFailed)
	}
	if err := transport.validateHandshakeSignature(ctx, handshake); err != nil {
		return err
	}
	return nil
}

func (transport *GRPCTransport) validateHandshakeSignature(ctx context.Context, handshake Handshake) error {
	hasSignatureMaterial := len(handshake.NodePublicKey) > 0 || len(handshake.Signature) > 0 || handshake.SignatureNonce != ""
	if !transport.requireHandshakeSignature && !hasSignatureMaterial {
		return nil
	}
	if len(handshake.NodePublicKey) == 0 || len(handshake.Signature) == 0 || handshake.SignatureNonce == "" {
		return ErrHandshakeSignature
	}
	if transport.handshakeVerifier == nil {
		return ErrHandshakeSignature
	}
	if !transport.handshakeVerifier.Verify(handshake.NodePublicKey, handshakeSignaturePayload(handshake), handshake.Signature) {
		return ErrHandshakeSignature
	}
	if !transport.markAuthNonce(ctx, handshake.NodeID, "sig:"+handshake.SignatureNonce, time.Now()) {
		return ErrHandshakeSignature
	}
	return nil
}

func handshakeSignaturePayload(handshake Handshake) []byte {
	var builder strings.Builder
	builder.WriteString("vexo-p2p-handshake-v1\n")
	builder.WriteString(handshake.ProtocolVersion)
	builder.WriteByte('\n')
	builder.WriteString(handshake.NetworkID)
	builder.WriteByte('\n')
	builder.WriteString(handshake.ChainID)
	builder.WriteByte('\n')
	builder.WriteString(handshake.GenesisHash)
	builder.WriteByte('\n')
	builder.WriteString(string(handshake.NodeID))
	builder.WriteByte('\n')
	builder.WriteString(handshake.ListenAddr)
	builder.WriteByte('\n')
	builder.WriteString(handshake.SignatureNonce)
	builder.WriteByte('\n')
	if len(handshake.KnownPeers) > 0 {
		peers := make([]string, 0, len(handshake.KnownPeers))
		for peerID, address := range handshake.KnownPeers {
			peers = append(peers, string(peerID)+"="+address)
		}
		sort.Strings(peers)
		builder.WriteString(strings.Join(peers, "\n"))
	}
	return []byte(builder.String())
}

func (transport *GRPCTransport) authProof(nodeID p2p.PeerID) string {
	if transport.authToken == "" {
		return ""
	}
	timestamp := time.Now().UnixNano()
	nonce, err := randomAuthNonce()
	if err != nil {
		return ""
	}
	signature := transport.authMAC(nodeID, timestamp, nonce)
	return handshakeAuthVersion + ":" + strconv.FormatInt(timestamp, 10) + ":" + nonce + ":" + signature
}

func (transport *GRPCTransport) verifyAuthProof(ctx context.Context, nodeID p2p.PeerID, proof string) bool {
	parts := strings.Split(proof, ":")
	if len(parts) != 4 || parts[0] != handshakeAuthVersion {
		return false
	}
	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || timestamp <= 0 || parts[2] == "" || parts[3] == "" {
		return false
	}
	now := time.Now()
	issuedAt := time.Unix(0, timestamp)
	if issuedAt.After(now.Add(handshakeAuthTTL)) || now.Sub(issuedAt) > handshakeAuthTTL {
		return false
	}
	expected := transport.authMAC(nodeID, timestamp, parts[2])
	if !hmac.Equal([]byte(parts[3]), []byte(expected)) {
		return false
	}
	return transport.markAuthNonce(ctx, nodeID, parts[2], now)
}

func (transport *GRPCTransport) markAuthNonce(ctx context.Context, nodeID p2p.PeerID, nonce string, now time.Time) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	key := string(nodeID) + ":" + nonce
	if transport.authReplayStore != nil {
		return transport.authReplayStore.MarkAuthNonce(ctx, nodeID, nonce, now.Add(handshakeAuthTTL), now) == nil
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	for nonceKey, seenAt := range transport.authNonces {
		if now.Sub(seenAt) > handshakeAuthTTL {
			delete(transport.authNonces, nonceKey)
		}
	}
	if _, found := transport.authNonces[key]; found {
		return false
	}
	transport.authNonces[key] = now
	return true
}

func (transport *GRPCTransport) authMAC(nodeID p2p.PeerID, timestamp int64, nonce string) string {
	mac := hmac.New(sha256.New, []byte(transport.authToken))
	mac.Write([]byte(transport.protocolVersion))
	mac.Write([]byte{0})
	mac.Write([]byte(transport.networkID))
	mac.Write([]byte{0})
	mac.Write([]byte(transport.chainID))
	mac.Write([]byte{0})
	mac.Write([]byte(transport.genesisHash))
	mac.Write([]byte{0})
	mac.Write([]byte(nodeID))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte{0})
	mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomAuthNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce[:]), nil
}

func (transport *GRPCTransport) runReconnectLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	transport.reconnectConfiguredPeers(ctx)
	ticker := time.NewTicker(transport.reconnectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			transport.reconnectConfiguredPeers(ctx)
		}
	}
}

func (transport *GRPCTransport) reconnectConfiguredPeers(ctx context.Context) {
	var wait sync.WaitGroup
	for peerID, address := range transport.peerAddresses() {
		if err := transport.checkPeerBackoff(peerID); err != nil {
			continue
		}
		wait.Add(1)
		go func(peerID p2p.PeerID, address string) {
			defer wait.Done()
			transport.reconnectConfiguredPeer(ctx, peerID, address)
		}(peerID, address)
	}
	wait.Wait()
}

func (transport *GRPCTransport) reconnectConfiguredPeer(ctx context.Context, peerID p2p.PeerID, address string) {
	transport.notifyPeerAttempted(peerID)
	dialCtx, cancel := context.WithTimeout(ctx, transport.dialTimeout)
	_, err := transport.peerSession(dialCtx, peerID, address)
	cancel()
	if errors.Is(err, ErrPeerDialInProgress) {
		return
	}
	if err != nil {
		transport.notifyPeerDialResult(peerID, false)
		transport.emitPeerEvent(PeerEvent{Type: "peer_dial_failed", PeerID: peerID, Address: address, Reason: err.Error()})
	}
	if err == nil {
		transport.notifyPeerDialResult(peerID, true)
	}
	if err != nil && !errors.Is(err, ErrHandshakeFailed) && !errors.Is(err, ErrProtocolMismatch) && !errors.Is(err, ErrNetworkMismatch) && !errors.Is(err, ErrChainIDMismatch) && !errors.Is(err, ErrGenesisHashMismatch) && !errors.Is(err, ErrAuthTokenMismatch) {
		transport.markPeerBackoff(peerID)
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

func (transport *GRPCTransport) deliver(envelope Envelope) {
	transport.mu.RLock()
	subscribers := append([]chan Envelope(nil), transport.subscribers[envelope.Topic]...)
	transport.mu.RUnlock()
	for _, subscriber := range subscribers {
		message := cloneEnvelope(envelope)
		if isReliableTopic(envelope.Topic) {
			subscriber <- message
			continue
		}
		select {
		case subscriber <- message:
		default:
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

func (transport *GRPCTransport) peerAddress(peerID p2p.PeerID) (string, bool) {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	address, ok := transport.peers[peerID]
	return address, ok
}

func (transport *GRPCTransport) peerAddresses() map[p2p.PeerID]string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	peers := make(map[p2p.PeerID]string, len(transport.peers))
	for peerID, address := range transport.peers {
		peers[peerID] = address
	}
	return peers
}

func ptrHandshake(handshake Handshake) *Handshake { return &handshake }

type p2pTransportServer interface {
	Gossip(grpc.BidiStreamingServer[grpcStreamMessage, grpcStreamMessage]) error
}

type p2pTransportClient interface {
	Gossip(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage], error)
}

type p2pTransportClientImpl struct{ connection grpc.ClientConnInterface }

func newP2PTransportClient(connection grpc.ClientConnInterface) p2pTransportClient {
	return &p2pTransportClientImpl{connection: connection}
}

func (client *p2pTransportClientImpl) Gossip(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[grpcStreamMessage, grpcStreamMessage], error) {
	stream, err := client.connection.NewStream(ctx, &p2pTransportServiceDesc.Streams[0], "/vexo.transport.P2P/Gossip", opts...)
	if err != nil {
		return nil, err
	}
	return &grpc.GenericClientStream[grpcStreamMessage, grpcStreamMessage]{ClientStream: stream}, nil
}

func registerP2PTransportServer(server *grpc.Server, service p2pTransportServer) {
	server.RegisterService(&p2pTransportServiceDesc, service)
}

var p2pTransportServiceDesc = grpc.ServiceDesc{
	ServiceName: "vexo.transport.P2P",
	HandlerType: (*p2pTransportServer)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Gossip",
			Handler:       p2pTransportGossipHandler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}

func p2pTransportGossipHandler(service any, stream grpc.ServerStream) error {
	return service.(p2pTransportServer).Gossip(&p2pTransportGossipServer{ServerStream: stream})
}

type p2pTransportGossipServer struct{ grpc.ServerStream }

func (server *p2pTransportGossipServer) Send(message *grpcStreamMessage) error {
	return server.ServerStream.SendMsg(message)
}

func (server *p2pTransportGossipServer) Recv() (*grpcStreamMessage, error) {
	message := new(grpcStreamMessage)
	if err := server.ServerStream.RecvMsg(message); err != nil {
		return nil, err
	}
	return message, nil
}

package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	GRPCProtocolVersion    = "vexo-p2p/1"
	grpcCodecName          = "vexo-json"
	defaultGRPCDialTimeout = 3 * time.Second
)

var (
	ErrHandshakeFailed     = errors.New("p2p handshake failed")
	ErrProtocolMismatch    = errors.New("protocol version mismatch")
	ErrNetworkMismatch     = errors.New("network id mismatch")
	ErrChainIDMismatch     = errors.New("chain id mismatch")
	ErrGenesisHashMismatch = errors.New("genesis hash mismatch")
)

type Handshake struct {
	ProtocolVersion string     `json:"protocol_version"`
	NetworkID       string     `json:"network_id"`
	ChainID         string     `json:"chain_id"`
	GenesisHash     string     `json:"genesis_hash"`
	NodeID          p2p.PeerID `json:"node_id"`
	ListenAddr      string     `json:"listen_addr,omitempty"`
}

type GRPCConfig struct {
	PeerID          p2p.PeerID
	ListenAddr      string
	Peers           map[p2p.PeerID]string
	DialTimeout     time.Duration
	ProtocolVersion string
	NetworkID       string
	ChainID         string
	GenesisHash     string
}

type GRPCTransport struct {
	peerID          p2p.PeerID
	listenAddr      string
	dialTimeout     time.Duration
	protocolVersion string
	networkID       string
	chainID         string
	genesisHash     string

	mu          sync.RWMutex
	listener    net.Listener
	server      *grpc.Server
	started     bool
	peers       map[p2p.PeerID]string
	connections map[p2p.PeerID]*grpc.ClientConn
	sessions    map[p2p.PeerID]*grpcPeerSession
	subscribers map[p2p.Topic][]chan Envelope
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

type grpcJSONCodec struct{}

func init() {
	encoding.RegisterCodec(grpcJSONCodec{})
}

func (grpcJSONCodec) Name() string { return grpcCodecName }

func (grpcJSONCodec) Marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func (grpcJSONCodec) Unmarshal(data []byte, value any) error {
	return json.Unmarshal(data, value)
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
	peers := make(map[p2p.PeerID]string, len(config.Peers))
	for peerID, address := range config.Peers {
		if peerID == "" || address == "" {
			continue
		}
		peers[peerID] = address
	}
	return &GRPCTransport{
		peerID:          config.PeerID,
		listenAddr:      config.ListenAddr,
		dialTimeout:     config.DialTimeout,
		protocolVersion: config.ProtocolVersion,
		networkID:       config.NetworkID,
		chainID:         config.ChainID,
		genesisHash:     config.GenesisHash,
		peers:           peers,
		connections:     make(map[p2p.PeerID]*grpc.ClientConn),
		sessions:        make(map[p2p.PeerID]*grpcPeerSession),
		subscribers:     make(map[p2p.Topic][]chan Envelope),
	}, nil
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
	server := grpc.NewServer(grpc.ForceServerCodec(grpcJSONCodec{}))
	registerP2PTransportServer(server, transport)
	transport.listener = listener
	transport.server = server
	transport.listenAddr = listener.Addr().String()
	transport.started = true
	go func() {
		_ = server.Serve(listener)
	}()
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
	transport.server = nil
	transport.listener = nil
	transport.started = false
	transport.connections = make(map[p2p.PeerID]*grpc.ClientConn)
	transport.sessions = make(map[p2p.PeerID]*grpcPeerSession)
	for _, subscribers := range transport.subscribers {
		for _, subscriber := range subscribers {
			close(subscriber)
		}
	}
	transport.subscribers = make(map[p2p.Topic][]chan Envelope)
	transport.mu.Unlock()
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
	peers := transport.peerAddresses()
	for peerID, address := range peers {
		if err := transport.sendEnvelope(ctx, peerID, address, Envelope{Topic: topic, From: transport.peerID, Data: append([]byte(nil), data...)}); err != nil {
			return err
		}
	}
	return nil
}

func (transport *GRPCTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
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
	channel := make(chan Envelope, 32)
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
	transport.closePeerSessionLocked(peerID)
	if connection := transport.connections[peerID]; connection != nil {
		_ = connection.Close()
		delete(transport.connections, peerID)
	}
	transport.peers[peerID] = address
}

func (transport *GRPCTransport) LocalHandshake() Handshake {
	transport.mu.RLock()
	listenAddr := transport.listenAddr
	transport.mu.RUnlock()
	return Handshake{
		ProtocolVersion: transport.protocolVersion,
		NetworkID:       transport.networkID,
		ChainID:         transport.chainID,
		GenesisHash:     transport.genesisHash,
		NodeID:          transport.peerID,
		ListenAddr:      listenAddr,
	}
}

func (transport *GRPCTransport) Gossip(stream grpc.BidiStreamingServer[grpcStreamMessage, grpcStreamMessage]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.Handshake == nil {
		return fmt.Errorf("%w: missing handshake", ErrHandshakeFailed)
	}
	if err := transport.validateHandshake(*first.Handshake); err != nil {
		return err
	}
	if err := stream.Send(&grpcStreamMessage{Handshake: ptrHandshake(transport.LocalHandshake())}); err != nil {
		return err
	}
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
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(grpcJSONCodec{})),
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
	return connection, nil
}

func (transport *GRPCTransport) peerSession(ctx context.Context, peerID p2p.PeerID, address string) (*grpcPeerSession, error) {
	transport.mu.RLock()
	session := transport.sessions[peerID]
	transport.mu.RUnlock()
	if session != nil {
		return session, nil
	}
	connection, err := transport.peerConnection(ctx, peerID, address)
	if err != nil {
		return nil, err
	}
	streamCtx, cancel := context.WithCancel(context.Background())
	client := newP2PTransportClient(connection)
	stream, err := client.Gossip(streamCtx)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := stream.Send(&grpcStreamMessage{Handshake: ptrHandshake(transport.LocalHandshake())}); err != nil {
		cancel()
		return nil, err
	}
	remote, err := stream.Recv()
	if err != nil {
		cancel()
		return nil, normalizeGRPCError(err)
	}
	if remote.Handshake == nil {
		cancel()
		return nil, fmt.Errorf("%w: missing remote handshake", ErrHandshakeFailed)
	}
	if remote.Handshake.NodeID != peerID {
		cancel()
		return nil, fmt.Errorf("%w: expected peer %s got %s", ErrHandshakeFailed, peerID, remote.Handshake.NodeID)
	}
	if err := transport.validateHandshake(*remote.Handshake); err != nil {
		cancel()
		return nil, err
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
	return session, nil
}

func (transport *GRPCTransport) closePeerSession(peerID p2p.PeerID) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.closePeerSessionLocked(peerID)
}

func (transport *GRPCTransport) closePeerSessionLocked(peerID p2p.PeerID) {
	session := transport.sessions[peerID]
	if session == nil {
		return
	}
	_ = session.stream.CloseSend()
	session.cancel()
	delete(transport.sessions, peerID)
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
	case strings.Contains(message, ErrHandshakeFailed.Error()):
		return fmt.Errorf("%w: %v", ErrHandshakeFailed, err)
	default:
		return err
	}
}

func (transport *GRPCTransport) validateHandshake(handshake Handshake) error {
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
	if handshake.NodeID == "" {
		return fmt.Errorf("%w: missing node id", ErrHandshakeFailed)
	}
	return nil
}

func (transport *GRPCTransport) deliver(envelope Envelope) {
	transport.mu.RLock()
	subscribers := append([]chan Envelope(nil), transport.subscribers[envelope.Topic]...)
	transport.mu.RUnlock()
	for _, subscriber := range subscribers {
		message := cloneEnvelope(envelope)
		select {
		case subscriber <- message:
		default:
		}
	}
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

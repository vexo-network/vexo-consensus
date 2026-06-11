//go:build legacytcp

package transport

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

const defaultTCPDialTimeout = 3 * time.Second

type TCPConfig struct {
	PeerID      p2p.PeerID
	ListenAddr  string
	Peers       map[p2p.PeerID]string
	DialTimeout time.Duration
}

type TCPTransport struct {
	peerID      p2p.PeerID
	listenAddr  string
	dialTimeout time.Duration

	mu          sync.RWMutex
	listener    net.Listener
	started     bool
	peers       map[p2p.PeerID]string
	subscribers map[p2p.Topic][]chan Envelope
}

type tcpFrame struct {
	Topic p2p.Topic  `json:"topic"`
	From  p2p.PeerID `json:"from"`
	To    p2p.PeerID `json:"to,omitempty"`
	Data  []byte     `json:"data"`
}

func NewTCPTransport(config TCPConfig) (*TCPTransport, error) {
	if config.PeerID == "" {
		return nil, ErrPeerIDRequired
	}
	if config.DialTimeout <= 0 {
		config.DialTimeout = defaultTCPDialTimeout
	}
	peers := make(map[p2p.PeerID]string, len(config.Peers))
	for peerID, address := range config.Peers {
		if peerID == "" || address == "" {
			continue
		}
		peers[peerID] = address
	}
	return &TCPTransport{
		peerID:      config.PeerID,
		listenAddr:  config.ListenAddr,
		dialTimeout: config.DialTimeout,
		peers:       peers,
		subscribers: make(map[p2p.Topic][]chan Envelope),
	}, nil
}

func (transport *TCPTransport) Start(ctx context.Context) error {
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
	transport.listener = listener
	transport.listenAddr = listener.Addr().String()
	transport.started = true
	go transport.acceptLoop(listener)
	return nil
}

func (transport *TCPTransport) Stop(ctx context.Context) error {
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
	listener := transport.listener
	transport.listener = nil
	transport.started = false
	for _, subscribers := range transport.subscribers {
		for _, subscriber := range subscribers {
			close(subscriber)
		}
	}
	transport.subscribers = make(map[p2p.Topic][]chan Envelope)
	transport.mu.Unlock()
	if listener != nil {
		return listener.Close()
	}
	return nil
}

func (transport *TCPTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	peers := transport.peerAddresses()
	frame := tcpFrame{
		Topic: topic,
		From:  transport.peerID,
		Data:  append([]byte(nil), data...),
	}
	for _, address := range peers {
		if err := transport.writeFrame(ctx, address, frame); err != nil {
			return err
		}
	}
	return nil
}

func (transport *TCPTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	address, ok := transport.peerAddress(to)
	if !ok {
		return ErrPeerNotFound
	}
	frame := tcpFrame{
		Topic: topic,
		From:  transport.peerID,
		To:    to,
		Data:  append([]byte(nil), data...),
	}
	return transport.writeFrame(ctx, address, frame)
}

func (transport *TCPTransport) Subscribe(ctx context.Context, topic p2p.Topic) (<-chan Envelope, error) {
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

func (transport *TCPTransport) PeerID() p2p.PeerID {
	return transport.peerID
}

func (transport *TCPTransport) Address() string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	return transport.listenAddr
}

func (transport *TCPTransport) SetPeer(peerID p2p.PeerID, address string) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if peerID == "" || address == "" {
		return
	}
	transport.peers[peerID] = address
}

func (transport *TCPTransport) KnownPeers() map[p2p.PeerID]string {
	return transport.peerAddresses()
}

func (transport *TCPTransport) ConfiguredPeerIDs() []p2p.PeerID {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	peers := make([]p2p.PeerID, 0, len(transport.peers))
	for peerID := range transport.peers {
		peers = append(peers, peerID)
	}
	sort.Slice(peers, func(left int, right int) bool { return peers[left] < peers[right] })
	return peers
}

func (transport *TCPTransport) ActivePeerIDs() []p2p.PeerID {
	return nil
}

func (transport *TCPTransport) acceptLoop(listener net.Listener) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		go transport.handleConnection(connection)
	}
}

func (transport *TCPTransport) handleConnection(connection net.Conn) {
	defer connection.Close()
	var frame tcpFrame
	if err := json.NewDecoder(connection).Decode(&frame); err != nil {
		return
	}
	envelope := Envelope{
		Topic: frame.Topic,
		From:  frame.From,
		To:    frame.To,
		Data:  append([]byte(nil), frame.Data...),
	}
	if envelope.From == transport.peerID {
		return
	}
	if envelope.To != "" && envelope.To != transport.peerID {
		return
	}
	transport.deliver(envelope)
}

func (transport *TCPTransport) deliver(envelope Envelope) {
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

func (transport *TCPTransport) writeFrame(ctx context.Context, address string, frame tcpFrame) error {
	dialer := net.Dialer{Timeout: transport.dialTimeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	return json.NewEncoder(connection).Encode(frame)
}

func (transport *TCPTransport) ensureStarted() error {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if !transport.started || transport.listener == nil {
		return ErrTransportClosed
	}
	return nil
}

func (transport *TCPTransport) peerAddress(peerID p2p.PeerID) (string, bool) {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	address, ok := transport.peers[peerID]
	return address, ok
}

func (transport *TCPTransport) peerAddresses() map[p2p.PeerID]string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	peers := make(map[p2p.PeerID]string, len(transport.peers))
	for peerID, address := range transport.peers {
		peers[peerID] = address
	}
	return peers
}

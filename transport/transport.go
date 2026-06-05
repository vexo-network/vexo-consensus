package transport

import (
	"context"
	"errors"
	"sync"

	"github.com/vexo-network/vexo-consensus/p2p"
)

var (
	ErrTransportClosed = errors.New("transport is closed")
	ErrPeerIDRequired  = errors.New("peer id is required")
	ErrPeerRejected    = errors.New("peer rejected by transport gate")
)

type Envelope struct {
	Topic p2p.Topic
	From  p2p.PeerID
	To    p2p.PeerID
	Data  []byte
}

type Transport interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Publish(ctx context.Context, topic p2p.Topic, data []byte) error
	Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error
	Subscribe(ctx context.Context, topic p2p.Topic) (<-chan Envelope, error)
	PeerID() p2p.PeerID
}

type PeerGateTransport interface {
	SetPeerGate(func(context.Context, p2p.PeerID) error)
}

type PeerGateChainTransport interface {
	AddPeerGate(func(context.Context, p2p.PeerID) error)
}

type PeerDisconnectTransport interface {
	DisconnectPeer(p2p.PeerID)
}

type PeerRemoveTransport interface {
	RemovePeer(p2p.PeerID)
}

type PeerExchangeTransport interface {
	SetPeer(p2p.PeerID, string)
	KnownPeers() map[p2p.PeerID]string
}

type InMemoryBus struct {
	mu          sync.RWMutex
	subscribers map[p2p.Topic]map[p2p.PeerID][]chan Envelope
	closed      bool
}

func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		subscribers: make(map[p2p.Topic]map[p2p.PeerID][]chan Envelope),
	}
}

func (bus *InMemoryBus) Close() {
	bus.close()
}

func (bus *InMemoryBus) NewPeer(peerID p2p.PeerID) (*InMemoryTransport, error) {
	if peerID == "" {
		return nil, ErrPeerIDRequired
	}
	return &InMemoryTransport{
		peerID: peerID,
		bus:    bus,
	}, nil
}

func (bus *InMemoryBus) close() {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	if bus.closed {
		return
	}
	bus.closed = true
	for _, byPeer := range bus.subscribers {
		for _, channels := range byPeer {
			for _, channel := range channels {
				close(channel)
			}
		}
	}
	bus.subscribers = make(map[p2p.Topic]map[p2p.PeerID][]chan Envelope)
}

func (bus *InMemoryBus) publish(ctx context.Context, envelope Envelope) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	bus.mu.RLock()
	defer bus.mu.RUnlock()
	if bus.closed {
		return ErrTransportClosed
	}

	targets := make([]chan Envelope, 0)
	if byPeer, found := bus.subscribers[envelope.Topic]; found {
		if envelope.To != "" {
			targets = append(targets, byPeer[envelope.To]...)
		} else {
			for peerID, channels := range byPeer {
				if peerID == envelope.From {
					continue
				}
				targets = append(targets, channels...)
			}
		}
	}

	for _, target := range targets {
		message := cloneEnvelope(envelope)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case target <- message:
		default:
		}
	}
	return nil
}

func (bus *InMemoryBus) subscribe(ctx context.Context, peerID p2p.PeerID, topic p2p.Topic) (<-chan Envelope, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	bus.mu.Lock()
	defer bus.mu.Unlock()
	if bus.closed {
		return nil, ErrTransportClosed
	}
	if _, found := bus.subscribers[topic]; !found {
		bus.subscribers[topic] = make(map[p2p.PeerID][]chan Envelope)
	}
	channel := make(chan Envelope, 32)
	bus.subscribers[topic][peerID] = append(bus.subscribers[topic][peerID], channel)
	return channel, nil
}

type InMemoryTransport struct {
	peerID  p2p.PeerID
	bus     *InMemoryBus
	mu      sync.RWMutex
	started bool
}

func (transport *InMemoryTransport) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.bus == nil {
		return ErrTransportClosed
	}
	transport.started = true
	return nil
}

func (transport *InMemoryTransport) Stop(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.started = false
	return nil
}

func (transport *InMemoryTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	return transport.bus.publish(ctx, Envelope{
		Topic: topic,
		From:  transport.peerID,
		Data:  append([]byte(nil), data...),
	})
}

func (transport *InMemoryTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	if err := transport.ensureStarted(); err != nil {
		return err
	}
	return transport.bus.publish(ctx, Envelope{
		Topic: topic,
		From:  transport.peerID,
		To:    to,
		Data:  append([]byte(nil), data...),
	})
}

func (transport *InMemoryTransport) Subscribe(ctx context.Context, topic p2p.Topic) (<-chan Envelope, error) {
	if err := transport.ensureStarted(); err != nil {
		return nil, err
	}
	return transport.bus.subscribe(ctx, transport.peerID, topic)
}

func (transport *InMemoryTransport) PeerID() p2p.PeerID {
	return transport.peerID
}

func (transport *InMemoryTransport) ensureStarted() error {
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if !transport.started || transport.bus == nil {
		return ErrTransportClosed
	}
	return nil
}

func cloneEnvelope(envelope Envelope) Envelope {
	envelope.Data = append([]byte(nil), envelope.Data...)
	return envelope
}

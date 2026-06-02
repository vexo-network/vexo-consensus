package consensus

import (
	"context"
	"errors"
	"sync"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
)

var ErrReactorAlreadyRunning = errors.New("consensus reactor already running")

type TransportReactor struct {
	transport transport.Transport
	reactor   Reactor
	admit     func(context.Context, p2p.PeerID) bool
	observe   func(context.Context, p2p.PeerID, bool) bool

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func NewTransportReactor(transport transport.Transport, reactor Reactor) *TransportReactor {
	return &TransportReactor{
		transport: transport,
		reactor:   reactor,
	}
}

func (reactor *TransportReactor) SetPeerScoring(admit func(context.Context, p2p.PeerID) bool, observe func(context.Context, p2p.PeerID, bool) bool) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	reactor.admit = admit
	reactor.observe = observe
}

func (reactor *TransportReactor) Start(ctx context.Context) error {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()

	if reactor.running {
		return ErrReactorAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	if err := reactor.transport.Start(runCtx); err != nil {
		cancel()
		return err
	}

	for _, topic := range []p2p.Topic{p2p.TopicProposal, p2p.TopicVote, p2p.TopicTimeout} {
		events, err := reactor.transport.Subscribe(runCtx, topic)
		if err != nil {
			cancel()
			return err
		}
		go reactor.consume(runCtx, events)
	}

	reactor.cancel = cancel
	reactor.running = true
	return nil
}

func (reactor *TransportReactor) Stop(ctx context.Context) error {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()

	if reactor.cancel != nil {
		reactor.cancel()
	}
	reactor.running = false
	return reactor.transport.Stop(ctx)
}

func (reactor *TransportReactor) BroadcastProposal(ctx context.Context, proposal Proposal) error {
	data, err := EncodeProposal(proposal)
	if err != nil {
		return err
	}
	return reactor.transport.Publish(ctx, p2p.TopicProposal, data)
}

func (reactor *TransportReactor) BroadcastVote(ctx context.Context, vote Vote) error {
	data, err := EncodeVote(vote)
	if err != nil {
		return err
	}
	return reactor.transport.Publish(ctx, p2p.TopicVote, data)
}

func (reactor *TransportReactor) BroadcastTimeoutVote(ctx context.Context, vote TimeoutVote) error {
	data, err := EncodeTimeoutVote(vote)
	if err != nil {
		return err
	}
	return reactor.transport.Publish(ctx, p2p.TopicTimeout, data)
}

func (reactor *TransportReactor) consume(ctx context.Context, events <-chan transport.Envelope) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			reactor.handleEnvelope(ctx, envelope)
		}
	}
}

func (reactor *TransportReactor) handleEnvelope(ctx context.Context, envelope transport.Envelope) {
	if !reactor.admitPeer(ctx, envelope.From) {
		return
	}
	message, err := DecodeWireMessage(envelope.Data)
	if err != nil {
		reactor.observePeer(ctx, envelope.From, false)
		return
	}
	valid := true
	switch message.Type {
	case MessageProposal:
		if err := reactor.reactor.OnProposal(ctx, *message.Proposal); err != nil {
			valid = isMaliciousConsensusError(err) == false
		}
	case MessageVote:
		if err := reactor.reactor.OnVote(ctx, *message.Vote); err != nil {
			valid = isMaliciousConsensusError(err) == false
		}
	case MessageTimeoutVote:
		if _, err := reactor.reactor.OnTimeoutVote(ctx, *message.TimeoutVote); err != nil {
			valid = isMaliciousConsensusError(err) == false
		}
	default:
		valid = false
	}
	reactor.observePeer(ctx, envelope.From, valid)
}

func (reactor *TransportReactor) admitPeer(ctx context.Context, peer p2p.PeerID) bool {
	reactor.mu.Lock()
	admit := reactor.admit
	reactor.mu.Unlock()
	return admit == nil || admit(ctx, peer)
}

func (reactor *TransportReactor) observePeer(ctx context.Context, peer p2p.PeerID, valid bool) bool {
	reactor.mu.Lock()
	observe := reactor.observe
	reactor.mu.Unlock()
	return observe == nil || observe(ctx, peer, valid)
}

func isMaliciousConsensusError(err error) bool {
	return errors.Is(err, ErrUnknownValidator) ||
		errors.Is(err, ErrConflictingVote) ||
		errors.Is(err, ErrInvalidProposal) ||
		errors.Is(err, ErrInvalidVote) ||
		errors.Is(err, ErrUnsafeProposal) ||
		errors.Is(err, ErrUnsafeVote) ||
		errors.Is(err, ErrUnknownConsensusMessage) ||
		errors.Is(err, ErrConflictingTimeoutVote)
}

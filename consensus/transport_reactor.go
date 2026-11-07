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

	// scoringMu guards the peer scoring hooks only. Consumers take it on every
	// message, so it must stay independent of mu: Start and Stop wait for the
	// consumers while holding mu.
	scoringMu sync.RWMutex
	admit     func(context.Context, p2p.PeerID) bool
	observe   func(context.Context, p2p.PeerID, bool) bool

	// mu serializes Start and Stop.
	mu        sync.Mutex
	cancel    context.CancelFunc
	consumers *sync.WaitGroup
	running   bool
}

func NewTransportReactor(transport transport.Transport, reactor Reactor) *TransportReactor {
	return &TransportReactor{
		transport: transport,
		reactor:   reactor,
	}
}

func (reactor *TransportReactor) SetPeerScoring(admit func(context.Context, p2p.PeerID) bool, observe func(context.Context, p2p.PeerID, bool) bool) {
	reactor.scoringMu.Lock()
	defer reactor.scoringMu.Unlock()
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

	consumers := new(sync.WaitGroup)
	for _, topic := range []p2p.Topic{p2p.TopicProposal, p2p.TopicVote, p2p.TopicTimeout} {
		events, err := reactor.transport.Subscribe(runCtx, topic)
		if err != nil {
			cancel()
			err = errors.Join(err, reactor.transport.Stop(ctx), waitForConsumers(ctx, consumers))
			return err
		}
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			reactor.consume(runCtx, events)
		}()
	}

	reactor.cancel = cancel
	reactor.consumers = consumers
	reactor.running = true
	return nil
}

// Stop cancels the consumers, stops the transport, and returns only after every
// consumer has returned or ctx ends, so no message handler runs after a
// successful Stop.
func (reactor *TransportReactor) Stop(ctx context.Context) error {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()

	if reactor.cancel != nil {
		reactor.cancel()
	}
	consumers := reactor.consumers
	reactor.cancel = nil
	reactor.consumers = nil
	reactor.running = false
	return errors.Join(reactor.transport.Stop(ctx), waitForConsumers(ctx, consumers))
}

// waitForConsumers blocks until consumers finish or ctx ends. A nil group has
// nothing to wait for.
func waitForConsumers(ctx context.Context, consumers *sync.WaitGroup) error {
	if consumers == nil {
		return nil
	}
	drained := make(chan struct{})
	go func() {
		consumers.Wait()
		close(drained)
	}()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	var handleErr error
	switch message.Type {
	case MessageProposal:
		handleErr = reactor.reactor.OnProposal(ctx, *message.Proposal)
	case MessageVote:
		handleErr = reactor.reactor.OnVote(ctx, *message.Vote)
	case MessageTimeoutVote:
		_, handleErr = reactor.reactor.OnTimeoutVote(ctx, *message.TimeoutVote)
	default:
		handleErr = ErrUnknownConsensusMessage
	}
	reactor.observePeer(ctx, envelope.From, !isPenalizedConsensusError(handleErr))
}

func (reactor *TransportReactor) admitPeer(ctx context.Context, peer p2p.PeerID) bool {
	reactor.scoringMu.RLock()
	admit := reactor.admit
	reactor.scoringMu.RUnlock()
	return admit == nil || admit(ctx, peer)
}

func (reactor *TransportReactor) observePeer(ctx context.Context, peer p2p.PeerID, valid bool) bool {
	reactor.scoringMu.RLock()
	observe := reactor.observe
	reactor.scoringMu.RUnlock()
	return observe == nil || observe(ctx, peer, valid)
}

// penalizedConsensusErrors are the outcomes only a faulty or hostile sender can
// provoke: a malformed message, a bad signature or certificate, or a message
// from a validator that may not send it. Everything else the state machine
// reports is either an ordinary race (a stale, out-of-window or not yet
// resolvable message), safety evidence about a validator (conflicting votes,
// unsafe targets), or a local failure, and does not penalize the relaying peer.
var penalizedConsensusErrors = [...]error{
	ErrUnknownConsensusMessage,
	ErrInvalidProposal,
	ErrInvalidVote,
	ErrUnknownValidator,
	ErrUnexpectedProposer,
	ErrInvalidQuorumCert,
	ErrInvalidTimeoutCert,
	ErrConflictingProposal,
}

func isPenalizedConsensusError(err error) bool {
	if err == nil {
		return false
	}
	// The state machine reports a vote for a block this node has not received
	// yet as an ErrInvalidVote wrapping ErrUnknownVoteBlock. That is a race
	// between honest peers, so it is checked before the penalized sentinels.
	if errors.Is(err, ErrUnknownVoteBlock) {
		return false
	}
	for _, penalized := range penalizedConsensusErrors {
		if errors.Is(err, penalized) {
			return true
		}
	}
	return false
}

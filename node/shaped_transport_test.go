package node

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
)

// shapedTransport models a validator's wide-area link on top of the in-memory bus: an upload of
// rate bytes per second shared by everything the validator sends, and a one-way latency. A
// broadcast costs its size once per validator it reaches, as on a network of point-to-point links.
//
// The link has two classes, as the channels of a production transport do: consensus messages
// (proposals, votes, timeouts, commits, evidence) go before bulk data (transaction gossip and lane
// batches), so neither treatment loses views to a proposal queued behind megabytes of
// transactions. Within a class messages leave in the order they were sent, and every message
// arrives latency after the link finished sending it.
type shapedTransport struct {
	*transport.InMemoryTransport
	rate    float64
	latency time.Duration
	fanout  int

	urgent   chan shapedMessage
	bulk     chan shapedMessage
	inFlight chan shapedMessage
	once     sync.Once
	done     chan struct{}
	sent     map[p2p.Topic]*atomic.Int64
}

type shapedMessage struct {
	at     time.Time
	to     p2p.PeerID
	topic  p2p.Topic
	data   []byte
	copies int
}

func newShapedTransport(inner *transport.InMemoryTransport, rate float64, latency time.Duration, fanout int) *shapedTransport {
	shaped := &shapedTransport{
		InMemoryTransport: inner,
		rate:              rate,
		latency:           latency,
		fanout:            fanout,
		urgent:            make(chan shapedMessage, 1<<14),
		bulk:              make(chan shapedMessage, 1<<16),
		inFlight:          make(chan shapedMessage, 1<<16),
		done:              make(chan struct{}),
		sent:              make(map[p2p.Topic]*atomic.Int64),
	}
	for _, topic := range []p2p.Topic{p2p.TopicTx, p2p.TopicBatch, p2p.TopicProposal, p2p.TopicVote, p2p.TopicTimeout, p2p.TopicCommit, p2p.TopicEvidence} {
		shaped.sent[topic] = new(atomic.Int64)
	}
	go shaped.transmit()
	go shaped.deliver()
	return shaped
}

func (shaped *shapedTransport) close() {
	shaped.once.Do(func() { close(shaped.done) })
}

// Sent returns the bytes the validator uploaded on topic.
func (shaped *shapedTransport) Sent(topic p2p.Topic) int64 {
	return shaped.sent[topic].Load()
}

func (shaped *shapedTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	return shaped.enqueue(ctx, "", topic, data, shaped.fanout)
}

func (shaped *shapedTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	return shaped.enqueue(ctx, to, topic, data, 1)
}

func (shaped *shapedTransport) enqueue(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte, copies int) error {
	if counter, found := shaped.sent[topic]; found {
		counter.Add(int64(len(data)) * int64(copies))
	}
	queue := shaped.urgent
	if topic == p2p.TopicTx || topic == p2p.TopicBatch {
		queue = shaped.bulk
	}
	select {
	case queue <- shapedMessage{to: to, topic: topic, data: append([]byte(nil), data...), copies: copies}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-shaped.done:
		return transport.ErrTransportClosed
	}
}

// transmit sends the queued messages over the link, consensus messages first, keeping a virtual
// clock of when the link is free so short messages need no sleep of their own.
func (shaped *shapedTransport) transmit() {
	linkFree := time.Now()
	for {
		var message shapedMessage
		select {
		case message = <-shaped.urgent:
		default:
			select {
			case message = <-shaped.urgent:
			case message = <-shaped.bulk:
			case <-shaped.done:
				return
			}
		}
		now := time.Now()
		if linkFree.Before(now) {
			linkFree = now
		}
		if shaped.rate > 0 {
			linkFree = linkFree.Add(time.Duration(float64(len(message.data)*message.copies) / shaped.rate * float64(time.Second)))
		}
		if wait := time.Until(linkFree); wait > time.Millisecond {
			select {
			case <-time.After(wait):
			case <-shaped.done:
				return
			}
		}
		message.at = linkFree.Add(shaped.latency)
		select {
		case shaped.inFlight <- message:
		case <-shaped.done:
			return
		}
	}
}

// deliver hands each message to the bus once it crossed the link.
func (shaped *shapedTransport) deliver() {
	for {
		select {
		case <-shaped.done:
			return
		case message := <-shaped.inFlight:
			if wait := time.Until(message.at); wait > 0 {
				select {
				case <-time.After(wait):
				case <-shaped.done:
					return
				}
			}
			ctx := context.Background()
			if message.to == "" {
				_ = shaped.InMemoryTransport.Publish(ctx, message.topic, message.data)
			} else {
				_ = shaped.InMemoryTransport.Send(ctx, message.to, message.topic, message.data)
			}
		}
	}
}

package transport

import (
	"context"
	"sync"

	"github.com/vexo-network/vexo-consensus/p2p"
)

type deliveryResult int

const (
	// deliveryQueued: the message is in the subscriber's buffer.
	deliveryQueued deliveryResult = iota
	// deliveryDropped: a non-reliable message found the buffer full.
	deliveryDropped
	// deliveryAborted: the subscription or the sending stream ended first.
	deliveryAborted
)

// subscription owns the delivery channel of one Subscribe call.
//
// Senders hold mu.RLock for the whole send and the channel is closed only
// after done is closed (which releases blocked reliable senders) and mu.Lock
// has drained every in-flight sender. A send can therefore never race with
// close, and a reliable send can never outlive the subscription, the stream
// that carries it, or the transport that owns it.
type subscription struct {
	topic p2p.Topic
	ch    chan Envelope

	done      chan struct{}
	closeOnce sync.Once

	mu     sync.RWMutex
	closed bool
}

func newSubscription(topic p2p.Topic, buffer int) *subscription {
	return &subscription{
		topic: topic,
		ch:    make(chan Envelope, buffer),
		done:  make(chan struct{}),
	}
}

// deliver queues message. A reliable message waits for buffer space until the
// subscription is torn down or ctx ends; any other message is dropped when the
// buffer is full.
func (subscriber *subscription) deliver(ctx context.Context, message Envelope, reliable bool) deliveryResult {
	subscriber.mu.RLock()
	defer subscriber.mu.RUnlock()
	if subscriber.closed {
		return deliveryAborted
	}
	if reliable {
		select {
		case subscriber.ch <- message:
			return deliveryQueued
		case <-subscriber.done:
			return deliveryAborted
		case <-ctx.Done():
			return deliveryAborted
		}
	}
	select {
	case subscriber.ch <- message:
		return deliveryQueued
	default:
		return deliveryDropped
	}
}

// close tears the subscription down: it releases blocked senders, waits for
// in-flight senders to leave, and then closes the channel exactly once.
func (subscriber *subscription) close() {
	subscriber.closeOnce.Do(func() {
		close(subscriber.done)
		subscriber.mu.Lock()
		subscriber.closed = true
		close(subscriber.ch)
		subscriber.mu.Unlock()
	})
}

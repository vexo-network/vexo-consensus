package transport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestGRPCTransportStopDuringGossipDoesNotPanic(t *testing.T) {
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{SubscriberBuffer: 1})
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	alice.SetPeer("bob", bob.Address())
	// The subscriber never reads, so bob's stream handlers block on a reliable
	// delivery with a full buffer: the state in which closing the channel used
	// to race with a send.
	votes, err := bob.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 32; index++ {
		if err := alice.Send(context.Background(), "bob", p2p.TopicVote, []byte{byte(index)}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(votes) < cap(votes) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	stopped := make(chan error, 1)
	go func() { stopped <- bob.Stop(context.Background()) }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not return while a handler was blocked on a reliable delivery")
	}
	for range votes {
	}
}

func TestGRPCTransportStopWhileDeliveriesAreInFlight(t *testing.T) {
	bob := newStartedGRPCPeer(t, "bob", GRPCConfig{SubscriberBuffer: 4})
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	alice.SetPeer("bob", bob.Address())
	votes, err := bob.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}
	txs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	var senders sync.WaitGroup
	stop := make(chan struct{})
	for _, topic := range []p2p.Topic{p2p.TopicVote, p2p.TopicTx} {
		senders.Add(1)
		go func(topic p2p.Topic) {
			defer senders.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = alice.Send(context.Background(), "bob", topic, []byte("m"))
			}
		}(topic)
	}
	// Drain slowly so deliveries are both flowing and blocked at Stop time.
	go func() {
		for range votes {
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		for range txs {
		}
	}()
	time.Sleep(100 * time.Millisecond)
	if err := bob.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(stop)
	senders.Wait()
}

func TestGRPCTransportSubscriptionEndsWithItsContext(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	defer stopGRPCPeer(t, alice)
	ctx, cancel := context.WithCancel(context.Background())
	events, err := alice.Subscribe(ctx, p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case _, open := <-events:
		if open {
			t.Fatal("expected the channel to close, got a message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscription channel stayed open after its context ended")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		alice.mu.RLock()
		remaining := len(alice.subscribers[p2p.TopicTx])
		alice.mu.RUnlock()
		if remaining == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	alice.mu.RLock()
	remaining := len(alice.subscribers[p2p.TopicTx])
	alice.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("expected the ended subscription to be unregistered, %d remain", remaining)
	}
	// Delivering after the subscriber left must neither panic nor block.
	alice.deliver(context.Background(), Envelope{Topic: p2p.TopicTx, From: "bob", Data: []byte("late")})
}

func TestGRPCTransportSubscribeAfterStopFails(t *testing.T) {
	alice := newStartedGRPCPeer(t, "alice", GRPCConfig{})
	stopGRPCPeer(t, alice)
	if _, err := alice.Subscribe(context.Background(), p2p.TopicTx); err == nil {
		t.Fatal("expected subscribing to a stopped transport to fail")
	}
}

func TestSubscriptionCloseReleasesBlockedAndRacingSenders(t *testing.T) {
	subscriber := newSubscription(p2p.TopicVote, 1)
	var senders sync.WaitGroup
	for range 16 {
		senders.Add(1)
		go func() {
			defer senders.Done()
			for range 100 {
				subscriber.deliver(context.Background(), Envelope{Topic: p2p.TopicVote}, true)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	subscriber.close()
	subscriber.close()
	finished := make(chan struct{})
	go func() {
		senders.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("senders stayed blocked after the subscription closed")
	}
	if got := subscriber.deliver(context.Background(), Envelope{}, false); got != deliveryAborted {
		t.Fatalf("expected delivery to a closed subscription to abort, got %v", got)
	}
}

func TestSubscriptionReliableDeliveryEndsWithTheSendingStream(t *testing.T) {
	subscriber := newSubscription(p2p.TopicVote, 1)
	subscriber.deliver(context.Background(), Envelope{}, true)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan deliveryResult, 1)
	go func() { result <- subscriber.deliver(ctx, Envelope{}, true) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case got := <-result:
		if got != deliveryAborted {
			t.Fatalf("expected a canceled stream to abort the delivery, got %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a reliable delivery outlived the stream that carried it")
	}
	if got := subscriber.deliver(context.Background(), Envelope{}, false); got != deliveryDropped {
		t.Fatalf("expected a non-reliable delivery to a full buffer to drop, got %v", got)
	}
}

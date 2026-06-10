package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestInMemoryTransportPublishBroadcastsToSubscribers(t *testing.T) {
	alice, bob, carol := newStartedPeers(t)
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	carolTxs, err := carol.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); err != nil {
		t.Fatal(err)
	}

	assertEnvelope(t, bobTxs, "alice", "", p2p.TopicTx, "tx")
	assertEnvelope(t, carolTxs, "alice", "", p2p.TopicTx, "tx")
}

func TestInMemoryTransportSendDeliversOnlyTargetPeer(t *testing.T) {
	alice, bob, carol := newStartedPeers(t)
	bobVotes, err := bob.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}
	carolVotes, err := carol.Subscribe(context.Background(), p2p.TopicVote)
	if err != nil {
		t.Fatal(err)
	}

	if err := alice.Send(context.Background(), "bob", p2p.TopicVote, []byte("vote")); err != nil {
		t.Fatal(err)
	}

	assertEnvelope(t, bobVotes, "alice", "bob", p2p.TopicVote, "vote")
	assertNoEnvelope(t, carolVotes)
}

func TestInMemoryTransportDoesNotEchoPublishToSender(t *testing.T) {
	alice, _, _ := newStartedPeers(t)
	aliceTxs, err := alice.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); err != nil {
		t.Fatal(err)
	}
	assertNoEnvelope(t, aliceTxs)
}

func TestInMemoryTransportCopiesPayload(t *testing.T) {
	alice, bob, _ := newStartedPeers(t)
	bobTxs, err := bob.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("tx")
	if err := alice.Publish(context.Background(), p2p.TopicTx, payload); err != nil {
		t.Fatal(err)
	}
	payload[0] = 'X'

	assertEnvelope(t, bobTxs, "alice", "", p2p.TopicTx, "tx")
}

func TestInMemoryTransportRejectsStoppedPeer(t *testing.T) {
	alice, _, _ := newStartedPeers(t)
	if err := alice.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected transport closed, got %v", err)
	}
	if _, err := alice.Subscribe(context.Background(), p2p.TopicTx); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected transport closed, got %v", err)
	}
}

func TestInMemoryBusCloseClosesSubscribers(t *testing.T) {
	bus := NewInMemoryBus()
	alice, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ch, err := alice.Subscribe(context.Background(), p2p.TopicTx)
	if err != nil {
		t.Fatal(err)
	}

	bus.Close()
	if _, ok := <-ch; ok {
		t.Fatal("expected subscriber channel closed")
	}
	if err := alice.Publish(context.Background(), p2p.TopicTx, []byte("tx")); !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected transport closed, got %v", err)
	}
}

func TestInMemoryTransportValidationAndContext(t *testing.T) {
	bus := NewInMemoryBus()
	if _, err := bus.NewPeer(""); !errors.Is(err, ErrPeerIDRequired) {
		t.Fatalf("expected peer id required, got %v", err)
	}

	alice, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := alice.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled start, got %v", err)
	}
}

func newStartedPeers(t *testing.T) (*InMemoryTransport, *InMemoryTransport, *InMemoryTransport) {
	t.Helper()
	bus := NewInMemoryBus()
	alice := newStartedPeer(t, bus, "alice")
	bob := newStartedPeer(t, bus, "bob")
	carol := newStartedPeer(t, bus, "carol")
	return alice, bob, carol
}

func newStartedPeer(t *testing.T, bus *InMemoryBus, peerID p2p.PeerID) *InMemoryTransport {
	t.Helper()
	peer, err := bus.NewPeer(peerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return peer
}

func assertEnvelope(t *testing.T, ch <-chan Envelope, from p2p.PeerID, to p2p.PeerID, topic p2p.Topic, data string) {
	t.Helper()
	select {
	case envelope := <-ch:
		if envelope.From != from || envelope.To != to || envelope.Topic != topic || string(envelope.Data) != data {
			t.Fatalf("unexpected envelope: %+v", envelope)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for envelope")
	}
}

func assertNoEnvelope(t *testing.T, ch <-chan Envelope) {
	t.Helper()
	select {
	case envelope := <-ch:
		t.Fatalf("unexpected envelope: %+v", envelope)
	case <-time.After(20 * time.Millisecond):
	}
}

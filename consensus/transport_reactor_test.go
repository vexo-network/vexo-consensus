package consensus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestTransportReactorRoutesConsensusMessages(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}

	alice := newRecordingReactor()
	bob := newRecordingReactor()
	aliceReactor := NewTransportReactor(aliceTransport, alice)
	bobReactor := NewTransportReactor(bobTransport, bob)

	if err := aliceReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceReactor.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())

	if err := aliceReactor.BroadcastProposal(context.Background(), Proposal{Round: 1, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := aliceReactor.BroadcastVote(context.Background(), Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := aliceReactor.BroadcastTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 1, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}

	bob.waitFor(t, 1, 1, 1)
	proposals, votes, timeouts := alice.counts()
	if proposals != 0 || votes != 0 || timeouts != 0 {
		t.Fatalf("sender should not receive own broadcast: proposals=%d votes=%d timeouts=%d", proposals, votes, timeouts)
	}
}

func TestTransportReactorRejectsDoubleStart(t *testing.T) {
	bus := transport.NewInMemoryBus()
	peer, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	reactor := NewTransportReactor(peer, newRecordingReactor())
	if err := reactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer reactor.Stop(context.Background())
	if err := reactor.Start(context.Background()); !errors.Is(err, ErrReactorAlreadyRunning) {
		t.Fatalf("expected already running, got %v", err)
	}
}

func TestTransportReactorIgnoresMalformedMessages(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	bob := newRecordingReactor()
	aliceReactor := NewTransportReactor(aliceTransport, newRecordingReactor())
	bobReactor := NewTransportReactor(bobTransport, bob)

	if err := aliceReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceReactor.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())

	if err := aliceTransport.Publish(context.Background(), p2p.TopicProposal, []byte(`{"type":"proposal"}`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	proposals, votes, timeouts := bob.counts()
	if proposals != 0 || votes != 0 || timeouts != 0 {
		t.Fatalf("malformed message should be ignored: proposals=%d votes=%d timeouts=%d", proposals, votes, timeouts)
	}
}

type recordingReactor struct {
	mu        sync.Mutex
	proposals int
	votes     int
	timeouts  int
}

func newRecordingReactor() *recordingReactor {
	return &recordingReactor{}
}

func (reactor *recordingReactor) OnProposal(ctx context.Context, proposal Proposal) error {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	reactor.proposals++
	return nil
}

func (reactor *recordingReactor) OnVote(ctx context.Context, vote Vote) error {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	reactor.votes++
	return nil
}

func (reactor *recordingReactor) OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	reactor.timeouts++
	return finality.TimeoutCert{}, nil
}

func (reactor *recordingReactor) waitFor(t *testing.T, proposals int, votes int, timeouts int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		currentProposals, currentVotes, currentTimeouts := reactor.counts()
		if currentProposals == proposals && currentVotes == votes && currentTimeouts == timeouts {
			return
		}
		time.Sleep(time.Millisecond)
	}
	currentProposals, currentVotes, currentTimeouts := reactor.counts()
	if currentProposals == proposals && currentVotes == votes && currentTimeouts == timeouts {
		return
	}
	t.Fatalf("timed out waiting for counts proposals=%d votes=%d timeouts=%d, got proposals=%d votes=%d timeouts=%d", proposals, votes, timeouts, currentProposals, currentVotes, currentTimeouts)
}

func (reactor *recordingReactor) counts() (int, int, int) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	return reactor.proposals, reactor.votes, reactor.timeouts
}

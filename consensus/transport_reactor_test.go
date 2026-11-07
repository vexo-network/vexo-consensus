package consensus

import (
	"context"
	"errors"
	"fmt"
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

func TestTransportReactorScoresMalformedMessages(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	bobReactor := NewTransportReactor(bobTransport, newRecordingReactor())
	observed := make(chan bool, 1)
	bobReactor.SetPeerScoring(nil, func(ctx context.Context, peer p2p.PeerID, valid bool) bool {
		if peer == "alice" {
			observed <- valid
		}
		return true
	})
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())

	if err := aliceTransport.Publish(context.Background(), p2p.TopicProposal, []byte(`{"type":"proposal"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case valid := <-observed:
		if valid {
			t.Fatal("expected malformed consensus message to be scored invalid")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for peer score observation")
	}
}

func TestTransportReactorAdmissionDropsBannedPeer(t *testing.T) {
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
	bobReactor := NewTransportReactor(bobTransport, bob)
	bobReactor.SetPeerScoring(func(ctx context.Context, peer p2p.PeerID) bool {
		return peer != "alice"
	}, nil)
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())

	if err := aliceTransport.Publish(context.Background(), p2p.TopicVote, mustEncodeVote(t, Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	proposals, votes, timeouts := bob.counts()
	if proposals != 0 || votes != 0 || timeouts != 0 {
		t.Fatalf("admission-rejected peer should be dropped: proposals=%d votes=%d timeouts=%d", proposals, votes, timeouts)
	}
}

func TestTransportReactorTreatsNoQuorumAsValidProgress(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	bobReactor := NewTransportReactor(bobTransport, &errorReactor{voteErr: ErrNoQuorum})
	observed := make(chan bool, 1)
	bobReactor.SetPeerScoring(nil, func(ctx context.Context, peer p2p.PeerID, valid bool) bool {
		if peer == "alice" {
			observed <- valid
		}
		return true
	})
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())

	if err := aliceTransport.Publish(context.Background(), p2p.TopicVote, mustEncodeVote(t, Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})); err != nil {
		t.Fatal(err)
	}
	select {
	case valid := <-observed:
		if !valid {
			t.Fatal("expected no-quorum consensus progress to be scored valid")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for peer score observation")
	}
}

func TestTransportReactorTreatsUnknownVoteBlockAsRaceNotMisbehavior(t *testing.T) {
	// The state machine reports a vote for a block this node has not received
	// yet as an invalid vote wrapping ErrUnknownVoteBlock.
	racy := fmt.Errorf("%w: %w", ErrInvalidVote, ErrUnknownVoteBlock)
	if !scoreRelayedMessage(t, &errorReactor{voteErr: racy}, p2p.TopicVote, mustEncodeVote(t, Vote{Height: 2, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})) {
		t.Fatal("expected a vote for a block that has not arrived yet to avoid peer penalty")
	}
}

func TestTransportReactorPenalizesPeersRelayingInvalidMessages(t *testing.T) {
	penalized := []error{
		ErrInvalidProposal,
		fmt.Errorf("%w: invalid proposal signature", ErrInvalidProposal),
		ErrInvalidVote,
		fmt.Errorf("%w: invalid vote signature", ErrInvalidVote),
		ErrUnknownValidator,
		ErrUnexpectedProposer,
		fmt.Errorf("%w: height=1 round=1", ErrUnexpectedProposer),
		ErrInvalidQuorumCert,
		// A quorum certificate that lacks quorum wraps ErrNoQuorum; the
		// valid-progress meaning of ErrNoQuorum must not hide the invalid
		// certificate.
		fmt.Errorf("%w: %w", ErrInvalidQuorumCert, ErrNoQuorum),
		ErrInvalidTimeoutCert,
		fmt.Errorf("%w: high qc: %w", ErrInvalidTimeoutCert, ErrInvalidQuorumCert),
		fmt.Errorf("%w: view=3", ErrConflictingProposal),
	}
	for _, kind := range relayedMessageKinds(t) {
		for _, cause := range penalized {
			t.Run(fmt.Sprintf("%s/%v", kind.name, cause), func(t *testing.T) {
				handler := &errorReactor{proposalErr: cause, voteErr: cause, timeoutErr: cause}
				if scoreRelayedMessage(t, handler, kind.topic, kind.payload) {
					t.Fatalf("expected %v relayed as a %s to penalize the peer", cause, kind.name)
				}
			})
		}
	}
}

func TestTransportReactorDoesNotPenalizeRacesEvidenceOrLocalFailures(t *testing.T) {
	neutral := []error{
		ErrStaleVote,
		ErrStaleProposal,
		ErrStaleTimeoutCert,
		fmt.Errorf("%w: vote height=9 round=9", ErrOutOfWindow),
		fmt.Errorf("%w: %w", ErrInvalidVote, ErrUnknownVoteBlock),
		ErrUnknownVoteBlock,
		ErrBlockNotFound,
		ErrConflictingVote,
		ErrConflictingTimeoutVote,
		ErrUnsafeProposal,
		ErrUnsafeVote,
		ErrNoQuorum,
		context.Canceled,
		errors.New("wal: disk full"),
	}
	for _, kind := range relayedMessageKinds(t) {
		for _, cause := range neutral {
			t.Run(fmt.Sprintf("%s/%v", kind.name, cause), func(t *testing.T) {
				handler := &errorReactor{proposalErr: cause, voteErr: cause, timeoutErr: cause}
				if !scoreRelayedMessage(t, handler, kind.topic, kind.payload) {
					t.Fatalf("expected %v relayed as a %s to avoid peer penalty", cause, kind.name)
				}
			})
		}
	}
}

func TestTransportReactorStopWaitsForRunningHandlers(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	handler := newBlockingReactor()
	bobReactor := NewTransportReactor(bobTransport, handler)
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := aliceTransport.Publish(context.Background(), p2p.TopicVote, mustEncodeVote(t, Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handler.entered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the handler to start")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bobReactor.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handler.exited:
	default:
		t.Fatal("Stop returned while a message handler was still running")
	}
}

func TestTransportReactorStopIsBoundedByContext(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	handler := newBlockingReactor()
	handler.ignoreCancel = true
	bobReactor := NewTransportReactor(bobTransport, handler)
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := aliceTransport.Publish(context.Background(), p2p.TopicVote, mustEncodeVote(t, Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handler.entered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the handler to start")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := bobReactor.Stop(stopCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected Stop to give up at its deadline while a handler is stuck, got %v", err)
	}
	close(handler.release)
	select {
	case <-handler.exited:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the released handler to finish")
	}
}

func TestTransportReactorRunsNoHandlerAfterStop(t *testing.T) {
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
	bobReactor := NewTransportReactor(bobTransport, bob)
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	gossipCtx, stopGossip := context.WithCancel(context.Background())
	gossipDone := make(chan struct{})
	go func() {
		defer close(gossipDone)
		payload := mustEncodeVote(t, Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})
		for gossipCtx.Err() == nil {
			_ = aliceTransport.Publish(gossipCtx, p2p.TopicVote, payload)
		}
	}()
	bob.waitForVotes(t)
	if err := bobReactor.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, stoppedVotes, _ := bob.counts()
	time.Sleep(20 * time.Millisecond)
	stopGossip()
	<-gossipDone
	if _, votes, _ := bob.counts(); votes != stoppedVotes {
		t.Fatalf("a handler ran after Stop returned: %d votes at Stop, %d afterwards", stoppedVotes, votes)
	}
}

func TestTransportReactorStartFailureReleasesTransport(t *testing.T) {
	subscribeErr := errors.New("subscribe refused")
	fake := &failingSubscribeTransport{failOn: 2, err: subscribeErr}
	reactor := NewTransportReactor(fake, newRecordingReactor())

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := reactor.Start(startCtx); !errors.Is(err, subscribeErr) {
		t.Fatalf("expected the subscribe error, got %v", err)
	}
	// Release the fake's lock before the second Start, which subscribes again.
	fake.mu.Lock()
	stops := fake.stops
	subscribeContexts := append([]context.Context(nil), fake.subscribeContexts...)
	fake.mu.Unlock()
	if stops != 1 {
		t.Fatalf("expected the half-started transport to be stopped once, got %d stops", stops)
	}
	for index, subscribed := range subscribeContexts {
		if subscribed.Err() == nil {
			t.Fatalf("subscription %d outlived the failed Start", index)
		}
	}
	// The fake refuses only its second Subscribe, so a reactor left startable starts now.
	if err := reactor.Start(startCtx); err != nil {
		t.Fatalf("a failed Start must leave the reactor startable, got %v", err)
	}
	if err := reactor.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type relayedMessageKind struct {
	name    string
	topic   p2p.Topic
	payload []byte
}

func relayedMessageKinds(t *testing.T) []relayedMessageKind {
	t.Helper()
	proposal, err := EncodeProposal(Proposal{Round: 1, Proposer: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	timeoutVote, err := EncodeTimeoutVote(TimeoutVote{Height: 1, Round: 1, ValidatorID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	return []relayedMessageKind{
		{name: "proposal", topic: p2p.TopicProposal, payload: proposal},
		{name: "vote", topic: p2p.TopicVote, payload: mustEncodeVote(t, Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})},
		{name: "timeout vote", topic: p2p.TopicTimeout, payload: timeoutVote},
	}
}

// scoreRelayedMessage has peer "alice" publish payload to a reactor wrapped
// around handler and returns how the reactor scored alice.
func scoreRelayedMessage(t *testing.T, handler Reactor, topic p2p.Topic, payload []byte) bool {
	t.Helper()
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	bobReactor := NewTransportReactor(bobTransport, handler)
	observed := make(chan bool, 1)
	bobReactor.SetPeerScoring(nil, func(ctx context.Context, peer p2p.PeerID, valid bool) bool {
		if peer == "alice" {
			observed <- valid
		}
		return true
	})
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())
	if err := aliceTransport.Publish(context.Background(), topic, payload); err != nil {
		t.Fatal(err)
	}
	select {
	case valid := <-observed:
		return valid
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for peer score observation")
		return false
	}
}

func TestTransportReactorTreatsConflictingVoteAsEvidenceNotPeerBan(t *testing.T) {
	bus := transport.NewInMemoryBus()
	aliceTransport, err := bus.NewPeer("alice")
	if err != nil {
		t.Fatal(err)
	}
	bobTransport, err := bus.NewPeer("bob")
	if err != nil {
		t.Fatal(err)
	}
	bobReactor := NewTransportReactor(bobTransport, &errorReactor{voteErr: ErrConflictingVote})
	observed := make(chan bool, 1)
	bobReactor.SetPeerScoring(nil, func(ctx context.Context, peer p2p.PeerID, valid bool) bool {
		if peer == "alice" {
			observed <- valid
		}
		return true
	})
	if err := aliceTransport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer aliceTransport.Stop(context.Background())
	if err := bobReactor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer bobReactor.Stop(context.Background())

	if err := aliceTransport.Publish(context.Background(), p2p.TopicVote, mustEncodeVote(t, Vote{Height: 2, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "alice"})); err != nil {
		t.Fatal(err)
	}
	select {
	case valid := <-observed:
		if !valid {
			t.Fatal("expected conflicting vote evidence to avoid peer penalty")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for peer score observation")
	}
}

type recordingReactor struct {
	mu        sync.Mutex
	proposals int
	votes     int
	timeouts  int
}

type errorReactor struct {
	proposalErr error
	voteErr     error
	timeoutErr  error
}

func (reactor *errorReactor) OnProposal(ctx context.Context, proposal Proposal) error {
	return reactor.proposalErr
}

func (reactor *errorReactor) OnVote(ctx context.Context, vote Vote) error {
	return reactor.voteErr
}

func (reactor *errorReactor) OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error) {
	return finality.TimeoutCert{}, reactor.timeoutErr
}

// blockingReactor holds the first vote until the run context ends (or, with
// ignoreCancel, until release is closed) and then takes a moment to finish, so
// a Stop that does not wait for handlers is observable.
type blockingReactor struct {
	entered      chan struct{}
	exited       chan struct{}
	release      chan struct{}
	ignoreCancel bool
	once         sync.Once
}

func newBlockingReactor() *blockingReactor {
	return &blockingReactor{
		entered: make(chan struct{}),
		exited:  make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (reactor *blockingReactor) OnProposal(ctx context.Context, proposal Proposal) error {
	return nil
}

func (reactor *blockingReactor) OnVote(ctx context.Context, vote Vote) error {
	reactor.once.Do(func() {
		close(reactor.entered)
		defer close(reactor.exited)
		if reactor.ignoreCancel {
			<-reactor.release
		} else {
			<-ctx.Done()
		}
		time.Sleep(20 * time.Millisecond)
	})
	return nil
}

func (reactor *blockingReactor) OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error) {
	return finality.TimeoutCert{}, nil
}

// failingSubscribeTransport fails its failOn-th Subscribe call.
type failingSubscribeTransport struct {
	failOn int
	err    error

	mu                sync.Mutex
	subscribes        int
	stops             int
	subscribeContexts []context.Context
}

func (fake *failingSubscribeTransport) Start(ctx context.Context) error { return nil }

func (fake *failingSubscribeTransport) Stop(ctx context.Context) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.stops++
	return nil
}

func (fake *failingSubscribeTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	return nil
}

func (fake *failingSubscribeTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	return nil
}

func (fake *failingSubscribeTransport) Subscribe(ctx context.Context, topic p2p.Topic) (<-chan transport.Envelope, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.subscribes++
	fake.subscribeContexts = append(fake.subscribeContexts, ctx)
	if fake.subscribes == fake.failOn {
		return nil, fake.err
	}
	return make(chan transport.Envelope), nil
}

func (fake *failingSubscribeTransport) PeerID() p2p.PeerID { return "fake" }

func mustEncodeVote(t *testing.T, vote Vote) []byte {
	t.Helper()
	data, err := EncodeVote(vote)
	if err != nil {
		t.Fatal(err)
	}
	return data
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

func (reactor *recordingReactor) waitForVotes(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, votes, _ := reactor.counts(); votes > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for a vote to reach the reactor")
}

func (reactor *recordingReactor) counts() (int, int, int) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	return reactor.proposals, reactor.votes, reactor.timeouts
}

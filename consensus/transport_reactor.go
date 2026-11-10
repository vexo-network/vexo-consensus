package consensus

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/lanes"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
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

	// proposalsMu guards the lane proposal state: the lanes that rebuild lane proposals, the full
	// proposals this node can serve, and the lane proposals waiting for batches.
	proposalsMu sync.Mutex
	lanes       *lanes.Lanes
	authentic   LaneAuthenticator
	held        []heldProposal
	pending     []pendingLane
	requested   map[types.Hash]time.Time
	stats       LaneProposalStats
	runCtx      context.Context
	// deferred are the proposals this node could not judge yet, and retry signals that what they
	// wait for may have arrived.
	deferred []deferredProposal
	retry    chan struct{}

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
		retry:     make(chan struct{}, 1),
	}
}

func (reactor *TransportReactor) SetPeerScoring(admit func(context.Context, p2p.PeerID) bool, observe func(context.Context, p2p.PeerID, bool) bool) {
	reactor.scoringMu.Lock()
	defer reactor.scoringMu.Unlock()
	reactor.admit = admit
	reactor.observe = observe
}

// heldProposals bounds the full proposals a node keeps to serve requests, and the lane proposals
// it waits on batches for: the recent views only.
const heldProposals = 32

// maxProposalServes bounds how often a node serves one whole proposal to one peer, so a request
// cannot make the node upload a block repeatedly.
const maxProposalServes = 2

// proposalRequestInterval is how long a node waits for the next batch of a lane proposal before it
// asks the sender for the whole proposal: the batches of a proposal it rebuilds arrive one after
// the other, a 512 KiB batch over a congested link taking longer than this, and asking the leader
// for the whole block while they still arrive puts on its uplink what lanes take off it.
const proposalRequestInterval = 300 * time.Millisecond

// maxLaneWait bounds how long a node waits for the batches of a lane proposal while they still
// arrive: past it the proposal is asked whole, so peers that serve batches slowly cannot hold a
// vote back for longer.
const maxLaneWait = 2 * time.Second

// maxDeferredProposals bounds the proposals a node keeps until it can judge them: those of the
// next views only. Past it the oldest is dropped, and its proposer's retransmission delivers it.
const maxDeferredProposals = 16

// deferredProposal is a proposal, whole or by lanes, that the node could not judge yet: the block
// its validator set depends on is not executed, or its parent has not arrived.
type deferredProposal struct {
	blockHash types.Hash
	from      p2p.PeerID
	message   WireMessage
}

// proposalPending reports whether err refuses a proposal only until the node executes the block
// its validator set depends on or receives its parent.
func proposalPending(err error) bool {
	return errors.Is(err, ErrValidatorSetPending) || errors.Is(err, ErrUnknownParent)
}

// heldProposal is a full proposal this node broadcast or verified, with how often it was served to
// each peer.
type heldProposal struct {
	blockHash types.Hash
	proposal  Proposal
	served    map[p2p.PeerID]int
}

// pendingLane is a lane proposal waiting for batches this node asked for.
type pendingLane struct {
	blockHash types.Hash
	compact   LaneProposal
	from      p2p.PeerID
	// whole is set once the node asked for the whole proposal instead of the batches.
	whole bool
	// since is when the node started to wait for the batches, progressed when the last of them
	// arrived.
	since      time.Time
	progressed time.Time
}

// LaneProposalStats counts the lane proposals a node received: those it rebuilt from the batches
// it stored, those it rebuilt after fetching batches, and those it fetched whole.
type LaneProposalStats struct {
	Rebuilt   uint64
	Completed uint64
	Fetched   uint64
	// ServedBytes counts the bytes of the whole proposals this node sent to peers that asked.
	ServedBytes uint64
}

// LaneProposalStats returns the counts of the lane proposals received so far.
func (reactor *TransportReactor) LaneProposalStats() LaneProposalStats {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	return reactor.stats
}

// EnableLanes makes the reactor broadcast proposals by the lane batches their blocks are made of
// (LaneProposal) and rebuild the lane proposals it receives from the batches the lanes store.
// authentic judges a lane proposal before the reactor fetches batches for it.
func (reactor *TransportReactor) EnableLanes(service *lanes.Lanes, authentic LaneAuthenticator) {
	reactor.proposalsMu.Lock()
	reactor.lanes = service
	reactor.authentic = authentic
	reactor.proposalsMu.Unlock()
	service.OnArrival(reactor.onBatchArrival)
}

func (reactor *TransportReactor) laneService() *lanes.Lanes {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	return reactor.lanes
}

// hold keeps proposal to serve the peers that cannot rebuild it.
func (reactor *TransportReactor) hold(proposal Proposal) {
	if len(proposal.Block.Txs) == 0 {
		return
	}
	blockHash := HashBlock(proposal.Block)
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for _, held := range reactor.held {
		if held.blockHash == blockHash {
			return
		}
	}
	if len(reactor.held) == heldProposals {
		copy(reactor.held, reactor.held[1:])
		reactor.held = reactor.held[:heldProposals-1]
	}
	reactor.held = append(reactor.held, heldProposal{blockHash: blockHash, proposal: proposal, served: make(map[p2p.PeerID]int)})
}

// serve returns the held proposal of blockHash for peer, false when the node holds none or served
// it to peer as often as it may.
func (reactor *TransportReactor) serve(peer p2p.PeerID, blockHash types.Hash) (Proposal, bool) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for _, held := range reactor.held {
		if held.blockHash != blockHash {
			continue
		}
		if held.served[peer] >= maxProposalServes {
			return Proposal{}, false
		}
		held.served[peer]++
		return held.proposal, true
	}
	return Proposal{}, false
}

// await records a lane proposal waiting for batches.
func (reactor *TransportReactor) await(waiting pendingLane) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for index, pending := range reactor.pending {
		if pending.blockHash == waiting.blockHash {
			// A retransmission does not restart the wait maxLaneWait bounds.
			if !pending.since.IsZero() && (waiting.since.IsZero() || pending.since.Before(waiting.since)) {
				waiting.since = pending.since
			}
			reactor.pending[index] = waiting
			return
		}
	}
	if len(reactor.pending) == heldProposals {
		copy(reactor.pending, reactor.pending[1:])
		reactor.pending = reactor.pending[:heldProposals-1]
	}
	reactor.pending = append(reactor.pending, waiting)
}

// takePending removes and returns the lane proposal of blockHash waiting for batches.
func (reactor *TransportReactor) takePending(blockHash types.Hash) (pendingLane, bool) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for index, pending := range reactor.pending {
		if pending.blockHash == blockHash {
			reactor.pending = append(reactor.pending[:index], reactor.pending[index+1:]...)
			return pending, true
		}
	}
	return pendingLane{}, false
}

// pendingNaming returns the lane proposals waiting for the batch of id.
func (reactor *TransportReactor) pendingNaming(id types.Hash) []pendingLane {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	var naming []pendingLane
	for _, pending := range reactor.pending {
		for _, ref := range pending.compact.Batches {
			if ref.ID == id {
				naming = append(naming, pending)
				break
			}
		}
	}
	return naming
}

// shouldRequest reports whether to ask for the whole proposal of blockHash now: not while a request
// for it is in flight.
func (reactor *TransportReactor) shouldRequest(blockHash types.Hash) bool {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	if reactor.requested == nil {
		reactor.requested = make(map[types.Hash]time.Time)
	}
	now := time.Now()
	if last, found := reactor.requested[blockHash]; found && now.Sub(last) < proposalRequestInterval {
		return false
	}
	if len(reactor.requested) >= heldProposals {
		for hash, last := range reactor.requested {
			if now.Sub(last) >= proposalRequestInterval {
				delete(reactor.requested, hash)
			}
		}
	}
	reactor.requested[blockHash] = now
	return true
}

func (reactor *TransportReactor) count(update func(*LaneProposalStats)) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	update(&reactor.stats)
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
	consumers.Add(1)
	go func() {
		defer consumers.Done()
		reactor.judgeDeferred(runCtx)
	}()
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
	reactor.proposalsMu.Lock()
	reactor.runCtx = runCtx
	reactor.proposalsMu.Unlock()
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

// BroadcastProposal publishes proposal. With lanes, a block whose transactions lane batches carry
// travels as a LaneProposal naming them, and the proposal is held for the peers that cannot
// rebuild it.
func (reactor *TransportReactor) BroadcastProposal(ctx context.Context, proposal Proposal) error {
	var data []byte
	var err error
	service := reactor.laneService()
	var batches []lanes.Batch
	if service != nil && len(proposal.Block.Txs) > 0 {
		batches = service.Covering(proposal.Block.Txs)
	}
	if len(batches) > 0 {
		reactor.hold(proposal)
		compact := NewLaneProposal(proposal, batches)
		service.Reference(HashBlock(proposal.Block), proposal.Block.Header.Height, compact.IDs())
		data, err = EncodeLaneProposal(compact)
	} else {
		data, err = EncodeProposal(proposal)
	}
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
	case MessageProposal, MessageLaneProposal:
		handleErr = reactor.judgeProposal(ctx, envelope.From, message)
	case MessageProposalRequest:
		reactor.onProposalRequest(ctx, envelope.From, *message.ProposalRequest)
	case MessageVote:
		handleErr = reactor.reactor.OnVote(ctx, *message.Vote)
	case MessageTimeoutVote:
		_, handleErr = reactor.reactor.OnTimeoutVote(ctx, *message.TimeoutVote)
	default:
		handleErr = ErrUnknownConsensusMessage
	}
	reactor.observePeer(ctx, envelope.From, !isPenalizedConsensusError(handleErr))
}

// judgeProposal judges a proposal off the wire, whole or by lanes. One the node cannot judge yet is
// kept and judged again once the node executed a block or accepted another proposal (Retry), so
// a validator that is behind does not wait for the proposer to retransmit, and the next leader,
// whose view needs the proposal's certificate, does not lose its view.
func (reactor *TransportReactor) judgeProposal(ctx context.Context, from p2p.PeerID, message WireMessage) error {
	var err error
	var blockHash types.Hash
	if message.Type == MessageProposal {
		blockHash = HashBlock(message.Proposal.Block)
		err = reactor.onWholeProposal(ctx, *message.Proposal)
	} else {
		blockHash = message.LaneProposal.BlockHash()
		err = reactor.onLaneProposal(ctx, from, *message.LaneProposal)
	}
	if proposalPending(err) {
		reactor.deferProposal(deferredProposal{blockHash: blockHash, from: from, message: message})
	}
	return err
}

// deferProposal keeps a proposal the node cannot judge yet.
func (reactor *TransportReactor) deferProposal(waiting deferredProposal) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for index, deferred := range reactor.deferred {
		if deferred.blockHash == waiting.blockHash {
			reactor.deferred[index] = waiting
			return
		}
	}
	if len(reactor.deferred) == maxDeferredProposals {
		reactor.deferred[0] = deferredProposal{}
		reactor.deferred = reactor.deferred[1:]
	}
	reactor.deferred = append(reactor.deferred, waiting)
}

// Retry makes the reactor judge again the proposals it could not judge yet. The node calls it once
// it executed a block, which may be the one their validator set depends on.
func (reactor *TransportReactor) Retry() {
	select {
	case reactor.retry <- struct{}{}:
	default:
	}
}

// judgeDeferred judges the deferred proposals each time Retry is called.
func (reactor *TransportReactor) judgeDeferred(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-reactor.retry:
		}
		reactor.proposalsMu.Lock()
		deferred := reactor.deferred
		reactor.deferred = nil
		reactor.proposalsMu.Unlock()
		for _, waiting := range deferred {
			if ctx.Err() != nil {
				return
			}
			err := reactor.judgeProposal(ctx, waiting.from, waiting.message)
			reactor.observePeer(ctx, waiting.from, !isPenalizedConsensusError(err))
		}
	}
}

// onWholeProposal judges a full proposal off the wire, counting a whole proposal this node fetched
// for a lane proposal.
func (reactor *TransportReactor) onWholeProposal(ctx context.Context, proposal Proposal) error {
	if len(proposal.Block.Txs) > 0 {
		if _, waited := reactor.takePending(HashBlock(proposal.Block)); waited {
			reactor.count(func(stats *LaneProposalStats) { stats.Fetched++ })
		}
	}
	return reactor.onProposal(ctx, proposal, nil)
}

// onProposal judges a full proposal and, once it is accepted, holds it for the peers that cannot
// rebuild its lane form.
// ids are the lane batches the block is made of, nil when it arrived whole.
func (reactor *TransportReactor) onProposal(ctx context.Context, proposal Proposal, ids []types.Hash) error {
	err := reactor.reactor.OnProposal(ctx, proposal)
	if err == nil {
		// The proposal may be the parent a deferred one waits for.
		reactor.Retry()
	}
	if service := reactor.laneService(); err == nil && service != nil && len(proposal.Block.Txs) > 0 {
		reactor.hold(proposal)
		if ids == nil {
			for _, batch := range service.Covering(proposal.Block.Txs) {
				ids = append(ids, batch.ID())
			}
		}
		// The batches are not proposed again while the block is pending.
		service.Reference(HashBlock(proposal.Block), proposal.Block.Header.Height, ids)
	}
	return err
}

// onLaneProposal rebuilds a lane proposal from the stored batches and judges it. Missing batches
// are asked from their authors and the sender; when they do not arrive in time, or the batches
// make another block than the header commits to, the whole proposal is asked from the sender. The
// sender is not at fault for either.
func (reactor *TransportReactor) onLaneProposal(ctx context.Context, from p2p.PeerID, compact LaneProposal) error {
	service := reactor.laneService()
	if service == nil {
		return reactor.requestWhole(ctx, compact.BlockHash(), from)
	}
	proposal, missing, complete, err := compact.Rebuild(service.Get)
	if err != nil {
		return err
	}
	blockHash := compact.BlockHash()
	if len(missing) > 0 {
		// Fetching batches costs this node and the validators it asks, so only for a proposal of
		// the scheduled leader. One it cannot judge yet is asked whole from the sender, which pays
		// for it alone.
		if err := reactor.authenticate(ctx, compact); err != nil {
			if errors.Is(err, ErrInvalidProposal) || proposalPending(err) {
				// A proposal whose validator set is pending is judged again once the node executed
				// the block the set depends on, before anything is fetched for it.
				return err
			}
			return reactor.fetchWhole(ctx, pendingLane{blockHash: blockHash, compact: compact, from: from})
		}
		now := time.Now()
		reactor.await(pendingLane{blockHash: blockHash, compact: compact, from: from, since: now, progressed: now})
		peers := make([]p2p.PeerID, 0, len(compact.Batches)+1)
		for _, ref := range compact.Batches {
			peers = append(peers, p2p.PeerID(ref.Author))
		}
		service.Request(ctx, missing, append(peers, from))
		reactor.watchBatches(blockHash, proposalRequestInterval)
		return nil
	}
	if !complete {
		return reactor.fetchWhole(ctx, pendingLane{blockHash: blockHash, compact: compact, from: from})
	}
	reactor.count(func(stats *LaneProposalStats) { stats.Rebuilt++ })
	return reactor.onProposal(ctx, proposal, compact.IDs())
}

// watchBatches asks for the whole lane proposal of blockHash after wait, unless a batch it names
// arrived since: then it watches again, until maxLaneWait passed since the node started to wait.
func (reactor *TransportReactor) watchBatches(blockHash types.Hash, wait time.Duration) {
	time.AfterFunc(wait, func() {
		reactor.proposalsMu.Lock()
		var waiting pendingLane
		found := false
		for _, pending := range reactor.pending {
			if pending.blockHash == blockHash {
				waiting, found = pending, true
				break
			}
		}
		reactor.proposalsMu.Unlock()
		if !found || waiting.whole {
			return
		}
		now := time.Now()
		next := waiting.progressed.Add(proposalRequestInterval)
		if now.Before(next) && now.Sub(waiting.since) < maxLaneWait {
			reactor.watchBatches(blockHash, min(next.Sub(now), maxLaneWait-now.Sub(waiting.since)))
			return
		}
		if waiting, still := reactor.takePending(blockHash); still && !waiting.whole {
			_ = reactor.fetchWhole(reactor.backgroundContext(), waiting)
		}
	})
}

// progress records that a batch the lane proposal of blockHash names arrived.
func (reactor *TransportReactor) progress(blockHash types.Hash, now time.Time) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for index := range reactor.pending {
		if reactor.pending[index].blockHash == blockHash {
			reactor.pending[index].progressed = now
			return
		}
	}
}

// onBatchArrival completes the lane proposals waiting for the batch of id.
func (reactor *TransportReactor) onBatchArrival(id types.Hash) {
	service := reactor.laneService()
	if service == nil {
		return
	}
	for _, waiting := range reactor.pendingNaming(id) {
		if waiting.whole {
			continue
		}
		proposal, missing, complete, err := waiting.compact.Rebuild(service.Get)
		if err != nil {
			continue
		}
		if len(missing) > 0 {
			reactor.progress(waiting.blockHash, time.Now())
			continue
		}
		if _, still := reactor.takePending(waiting.blockHash); !still {
			continue
		}
		ctx := reactor.backgroundContext()
		if !complete {
			_ = reactor.fetchWhole(ctx, waiting)
			continue
		}
		reactor.count(func(stats *LaneProposalStats) { stats.Completed++ })
		err = reactor.onProposal(ctx, proposal, waiting.compact.IDs())
		reactor.observePeer(ctx, waiting.from, !isPenalizedConsensusError(err))
	}
}

func (reactor *TransportReactor) authenticate(ctx context.Context, compact LaneProposal) error {
	reactor.proposalsMu.Lock()
	authentic := reactor.authentic
	reactor.proposalsMu.Unlock()
	if authentic == nil {
		return errors.New("no lane proposal authenticator")
	}
	return authentic(ctx, compact)
}

func (reactor *TransportReactor) backgroundContext() context.Context {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	if reactor.runCtx == nil {
		return context.Background()
	}
	return reactor.runCtx
}

// fetchWhole asks the sender of a lane proposal for the whole proposal, and waits for it in place
// of the batches.
func (reactor *TransportReactor) fetchWhole(ctx context.Context, waiting pendingLane) error {
	waiting.whole = true
	reactor.await(waiting)
	return reactor.requestWhole(ctx, waiting.blockHash, waiting.from)
}

// requestWhole asks peer for the whole proposal of blockHash.
func (reactor *TransportReactor) requestWhole(ctx context.Context, blockHash types.Hash, peer p2p.PeerID) error {
	if !reactor.shouldRequest(blockHash) {
		return nil
	}
	data, err := EncodeProposalRequest(ProposalRequest{BlockHash: blockHash})
	if err != nil {
		return err
	}
	// A failed send is retried on the proposer's retransmission.
	_ = reactor.transport.Send(ctx, peer, p2p.TopicProposal, data)
	return nil
}

// onProposalRequest sends peer the whole proposal it asked for.
func (reactor *TransportReactor) onProposalRequest(ctx context.Context, peer p2p.PeerID, request ProposalRequest) {
	proposal, ok := reactor.serve(peer, request.BlockHash)
	if !ok {
		return
	}
	data, err := EncodeProposal(proposal)
	if err != nil {
		return
	}
	if reactor.transport.Send(ctx, peer, p2p.TopicProposal, data) == nil {
		reactor.count(func(stats *LaneProposalStats) { stats.ServedBytes += uint64(len(data)) })
	}
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

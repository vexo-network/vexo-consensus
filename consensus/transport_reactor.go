package consensus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/mempool"
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

	// proposalsMu guards the compact proposal state: the transaction source that rebuilds
	// compact proposals, the full proposals this node can serve, and the requests in flight.
	proposalsMu sync.Mutex
	txSource    TxSource
	held        []heldProposal
	pending     []pendingCompact
	requested   map[types.Hash]time.Time
	compact     CompactProposalStats

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

// heldProposals bounds the full proposals a node keeps to serve requests, and the compact
// proposals it waits on transactions for: the recent views only.
const heldProposals = 32

// maxProposalServes bounds how often a node serves one proposal to one peer: the missing
// transactions, then the whole proposal when those did not complete it. A request cannot make the
// node upload a block repeatedly.
const maxProposalServes = 2

// proposalRequestInterval is how long a node waits for what it asked a peer for before it asks
// again.
const proposalRequestInterval = 200 * time.Millisecond

// heldProposal is a full proposal this node broadcast or verified, with how often it was served to
// each peer.
type heldProposal struct {
	blockHash types.Hash
	proposal  Proposal
	served    map[p2p.PeerID]int
}

// pendingCompact is a compact proposal waiting for the transactions this node asked a peer for.
type pendingCompact struct {
	blockHash types.Hash
	compact   CompactProposal
	txs       []types.Tx
	from      p2p.PeerID
	whole     bool
}

// CompactProposalStats counts the compact proposals a node received: those it rebuilt from its
// pool alone, those it completed with the missing transactions fetched from a peer, and those it
// fetched whole.
type CompactProposalStats struct {
	Rebuilt   uint64
	Completed uint64
	Fetched   uint64
	// FetchedTxs counts the transactions fetched for compact proposals, whole proposals included.
	FetchedTxs uint64
	// ServedBytes counts the bytes this node sent to peers that asked for transactions.
	ServedBytes uint64
}

// CompactProposalStats returns the counts of the compact proposals received so far.
func (reactor *TransportReactor) CompactProposalStats() CompactProposalStats {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	return reactor.compact
}

// EnableCompactProposals makes the reactor broadcast proposals compactly (CompactProposal) and
// rebuild the compact proposals it receives from the transactions source holds.
func (reactor *TransportReactor) EnableCompactProposals(source TxSource) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	reactor.txSource = source
}

func (reactor *TransportReactor) compactSource() (TxSource, bool) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	return reactor.txSource, reactor.txSource != nil
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

// await records a compact proposal waiting for transactions, and reports whether to ask for them
// now: not while a request for the same block is in flight.
func (reactor *TransportReactor) await(waiting pendingCompact) bool {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	if reactor.requested == nil {
		reactor.requested = make(map[types.Hash]time.Time)
	}
	now := time.Now()
	if last, found := reactor.requested[waiting.blockHash]; found && now.Sub(last) < proposalRequestInterval {
		return false
	}
	if len(reactor.requested) >= heldProposals {
		for hash, last := range reactor.requested {
			if now.Sub(last) >= proposalRequestInterval {
				delete(reactor.requested, hash)
			}
		}
	}
	reactor.requested[waiting.blockHash] = now
	for index, pending := range reactor.pending {
		if pending.blockHash == waiting.blockHash {
			reactor.pending[index] = waiting
			return true
		}
	}
	if len(reactor.pending) == heldProposals {
		copy(reactor.pending, reactor.pending[1:])
		reactor.pending = reactor.pending[:heldProposals-1]
	}
	reactor.pending = append(reactor.pending, waiting)
	return true
}

// takePending removes and returns the compact proposal of blockHash waiting on transactions from
// peer.
func (reactor *TransportReactor) takePending(blockHash types.Hash, peer p2p.PeerID) (pendingCompact, bool) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for index, pending := range reactor.pending {
		if pending.blockHash == blockHash && pending.from == peer {
			reactor.pending = append(reactor.pending[:index], reactor.pending[index+1:]...)
			delete(reactor.requested, blockHash)
			return pending, true
		}
	}
	return pendingCompact{}, false
}

// settled drops what the node waited on for blockHash once it holds the full proposal.
func (reactor *TransportReactor) settled(blockHash types.Hash) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	for index, pending := range reactor.pending {
		if pending.blockHash == blockHash {
			reactor.pending = append(reactor.pending[:index], reactor.pending[index+1:]...)
			break
		}
	}
	delete(reactor.requested, blockHash)
}

func (reactor *TransportReactor) count(update func(*CompactProposalStats)) {
	reactor.proposalsMu.Lock()
	defer reactor.proposalsMu.Unlock()
	update(&reactor.compact)
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
	var data []byte
	var err error
	if _, compact := reactor.compactSource(); compact && len(proposal.Block.Txs) > 0 {
		reactor.hold(proposal)
		data, err = EncodeCompactProposal(NewCompactProposal(proposal))
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
	case MessageProposal:
		handleErr = reactor.onWholeProposal(ctx, envelope.From, *message.Proposal)
	case MessageCompactProposal:
		handleErr = reactor.onCompactProposal(ctx, envelope.From, *message.CompactProposal)
	case MessageProposalRequest:
		reactor.onProposalRequest(ctx, envelope.From, *message.ProposalRequest)
	case MessageProposalTxs:
		handleErr = reactor.onProposalTxs(ctx, envelope.From, *message.ProposalTxs)
	case MessageVote:
		handleErr = reactor.reactor.OnVote(ctx, *message.Vote)
	case MessageTimeoutVote:
		_, handleErr = reactor.reactor.OnTimeoutVote(ctx, *message.TimeoutVote)
	default:
		handleErr = ErrUnknownConsensusMessage
	}
	reactor.observePeer(ctx, envelope.From, !isPenalizedConsensusError(handleErr))
}

// onWholeProposal judges a full proposal off the wire, counting a whole proposal this node fetched
// for a compact one.
func (reactor *TransportReactor) onWholeProposal(ctx context.Context, from p2p.PeerID, proposal Proposal) error {
	if len(proposal.Block.Txs) > 0 {
		if waiting, ok := reactor.takePending(HashBlock(proposal.Block), from); ok && waiting.whole {
			reactor.count(func(stats *CompactProposalStats) {
				stats.Fetched++
				stats.FetchedTxs += uint64(len(proposal.Block.Txs))
			})
		}
	}
	return reactor.onProposal(ctx, proposal)
}

// onProposal judges a full proposal and, once it is accepted, holds it for the peers that cannot
// rebuild its compact form.
func (reactor *TransportReactor) onProposal(ctx context.Context, proposal Proposal) error {
	if _, compact := reactor.compactSource(); compact && len(proposal.Block.Txs) > 0 {
		reactor.settled(HashBlock(proposal.Block))
	}
	err := reactor.reactor.OnProposal(ctx, proposal)
	if err == nil {
		if _, compact := reactor.compactSource(); compact {
			reactor.hold(proposal)
		}
	}
	return err
}

// onCompactProposal rebuilds a compact proposal from the pool and judges it. When transactions
// are missing it asks the sending peer for them, or for the whole proposal when most are missing;
// the sender is not at fault.
func (reactor *TransportReactor) onCompactProposal(ctx context.Context, from p2p.PeerID, compact CompactProposal) error {
	source, _ := reactor.compactSource()
	txs, missing, err := compact.Rebuild(source)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		if proposal, ok := compact.Complete(txs); ok {
			if len(txs) > 0 {
				reactor.count(func(stats *CompactProposalStats) { stats.Rebuilt++ })
			}
			return reactor.onProposal(ctx, proposal)
		}
		return reactor.requestTxs(ctx, pendingCompact{blockHash: compact.BlockHash(), compact: compact, from: from, whole: true})
	}
	return reactor.requestTxs(ctx, pendingCompact{blockHash: compact.BlockHash(), compact: compact, txs: txs, from: from, whole: 2*len(missing) > len(txs)})
}

// requestTxs asks waiting.from for what the compact proposal misses: the missing transactions, or
// the whole proposal.
func (reactor *TransportReactor) requestTxs(ctx context.Context, waiting pendingCompact) error {
	if !reactor.await(waiting) {
		return nil
	}
	request := ProposalRequest{BlockHash: waiting.blockHash}
	if !waiting.whole {
		for index, tx := range waiting.txs {
			if tx == nil {
				request.Missing = append(request.Missing, uint32(index))
			}
		}
	}
	data, err := EncodeProposalRequest(request)
	if err != nil {
		return err
	}
	// A failed send is retried on the proposer's retransmission.
	_ = reactor.transport.Send(ctx, waiting.from, p2p.TopicProposal, data)
	return nil
}

// onProposalRequest sends peer what it asked for: the transactions at the missing positions, or the
// whole proposal.
func (reactor *TransportReactor) onProposalRequest(ctx context.Context, peer p2p.PeerID, request ProposalRequest) {
	proposal, ok := reactor.serve(peer, request.BlockHash)
	if !ok {
		return
	}
	var data []byte
	var err error
	if len(request.Missing) == 0 {
		data, err = EncodeProposal(proposal)
	} else {
		answer := ProposalTxs{BlockHash: request.BlockHash, Indexes: make([]uint32, 0, len(request.Missing)), Txs: make([]types.Tx, 0, len(request.Missing))}
		for _, index := range request.Missing {
			if int(index) >= len(proposal.Block.Txs) {
				return
			}
			answer.Indexes = append(answer.Indexes, index)
			answer.Txs = append(answer.Txs, proposal.Block.Txs[index])
		}
		data, err = EncodeProposalTxs(answer)
	}
	if err != nil {
		return
	}
	if reactor.transport.Send(ctx, peer, p2p.TopicProposal, data) == nil {
		reactor.count(func(stats *CompactProposalStats) { stats.ServedBytes += uint64(len(data)) })
	}
}

// onProposalTxs completes the compact proposal waiting on the transactions peer sent. A transaction
// whose short id is not the one the proposal names was not asked for: the peer is at fault. When
// the transactions complete a block other than the header commits to, the pool held a colliding
// transaction, and the whole proposal is fetched.
func (reactor *TransportReactor) onProposalTxs(ctx context.Context, peer p2p.PeerID, answer ProposalTxs) error {
	waiting, ok := reactor.takePending(answer.BlockHash, peer)
	if !ok || waiting.whole {
		return nil
	}
	for position, index := range answer.Indexes {
		tx := answer.Txs[position]
		if int(index) >= len(waiting.txs) || len(tx) == 0 ||
			mempool.ShortTxID(mempool.HashTx(tx)) != waiting.compact.ShortID(int(index)) {
			return fmt.Errorf("%w: a transaction the compact proposal does not name", ErrInvalidProposal)
		}
		if waiting.txs[index] == nil {
			waiting.txs[index] = tx
		}
	}
	for _, tx := range waiting.txs {
		if tx == nil {
			// The answer was to an earlier request; ask for the rest.
			return reactor.requestTxs(ctx, waiting)
		}
	}
	proposal, complete := waiting.compact.Complete(waiting.txs)
	if !complete {
		waiting.whole = true
		waiting.txs = nil
		return reactor.requestTxs(ctx, waiting)
	}
	reactor.count(func(stats *CompactProposalStats) {
		stats.Completed++
		stats.FetchedTxs += uint64(len(answer.Txs))
	})
	return reactor.onProposal(ctx, proposal)
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

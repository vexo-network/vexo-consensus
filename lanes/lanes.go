package lanes

import (
	"context"
	"encoding/binary"
	"errors"
	"sort"
	"sync"
	"time"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	// sealBytes and SealDelay decide when the transactions waiting for a batch are sealed: once
	// they reach sealBytes, or SealDelay after the first of them. A lightly loaded lane, whose
	// previous batch was below paceBytes, also waits for the certificate of that batch, at most
	// certWait after it was sealed: a batch so gathers what arrives in one certification round
	// trip instead of a few transactions, and every batch costs a signature, an acknowledgement
	// and a certificate verification at each validator. A loaded lane does not wait, so its
	// batches pipeline and the queues that lengthen the round trip under load do not pace it.
	sealBytes = 512 << 10
	SealDelay = 5 * time.Millisecond
	paceBytes = 16 << 10
	certWait  = 50 * time.Millisecond
	// maxPoolBytes bounds the batch bytes a node stores, and maxLaneBytes those of one author that
	// are not consumed yet: a validator flooding its lane fills its own share only.
	maxPoolBytes = 256 << 20
	maxLaneBytes = 64 << 20
	// batchTTL is how long a batch no committed block consumed is kept; consumedRetention how
	// long a consumed one is still served to validators that catch up.
	batchTTL          = 30 * time.Second
	consumedRetention = 5 * time.Second
	// tickInterval paces retransmission, requests and expiry.
	tickInterval = 50 * time.Millisecond
	// resendInterval is how long the author waits for acknowledgements before it sends its batch
	// again to the validators that did not acknowledge it; it doubles the wait after each of at
	// most maxResends retransmissions, so a congested link is not flooded with copies of what it
	// is still carrying. certResendInterval is how long it waits before it announces the
	// certificate of a batch no proposal named yet again, at most maxAnnouncements times in all. A
	// lost certificate only keeps the leaders that missed it from proposing the batch.
	resendInterval     = time.Second
	maxResends         = 4
	certResendInterval = 2 * time.Second
	maxAnnouncements   = 3
	// requestInterval is how long a node waits for a batch it asked for before it asks another
	// validator.
	requestInterval = 200 * time.Millisecond
	// FallbackDelay is how long a node holds a transaction another validator's lane owns before it
	// batches the transaction itself, when no batch it stores carries it: the owner may be faulty.
	FallbackDelay = 3 * time.Second
	// maxServes bounds how often a node serves one batch to one peer.
	maxServes = 2
	// maxHeld bounds the transactions a node holds for other lanes.
	maxHeld = 1 << 17
	// storedQueue bounds the stored batches waiting for the Stored hook; past it the hook misses
	// batches, which only the pool's view of what is pending loses.
	storedQueue = 1024
)

// ErrNotRunning reports a lane operation on a stopped service.
var ErrNotRunning = errors.New("lanes are not running")

// Config wires a lane service into a validator.
type Config struct {
	ChainID string
	// Self is the validator this node runs; its lane carries the transactions it batches.
	Self types.ValidatorID
	// Sign signs message, a lane statement of signType, with the validator key in domain.
	Sign func(signType vexocrypto.SignType, domain vexocrypto.Domain, message []byte) (types.Signature, error)
	// Signatures verifies the signatures of batches and acknowledgements; Aggregator and
	// Aggregates make and verify certificates. They are the consensus signature suite.
	Signatures SignatureVerifier
	Aggregator interface {
		Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
	}
	Aggregates AggregateVerifier
	// ValidatorSet returns the validator set that certifies the next block: the authors and
	// certifiers of the batches it may carry.
	ValidatorSet func(context.Context) (validator.Set, error)
	// Pending reports whether the node still holds the transaction of hash in its pool, uncommitted.
	Pending func(types.Hash) bool
	// Ready is called when a batch becomes proposable.
	Ready func()
	// Stored is called, in the order the batches arrive and off the message loop, with every batch
	// of another author the node starts to store: the node's pool takes its transactions up, so it
	// knows what is pending network-wide.
	Stored func(Batch)
	// Admit and Observe score the peers that send lane messages.
	Admit   func(context.Context, p2p.PeerID) bool
	Observe func(context.Context, p2p.PeerID, bool) bool
}

// Stats counts what a lane service did.
type Stats struct {
	// Sealed counts the batches this node authored, Received those of other authors it stored.
	Sealed   uint64
	Received uint64
	// Certified counts the batches this node learnt certified, its own included.
	Certified uint64
	// Requested counts the batches this node asked a peer for, Served those it sent on request.
	Requested uint64
	Served    uint64
	// SentBytes counts the lane bytes this node uploaded: its own batches to every validator,
	// acknowledgements, certificates and served batches.
	SentBytes uint64
}

// Lanes is the lane service of a validator: it seals, disseminates and certifies the batches of its
// own lane, stores and acknowledges the batches of the other lanes, and serves them.
type Lanes struct {
	cfg Config

	mu      sync.Mutex
	wire    transport.Transport
	cancel  context.CancelFunc
	done    chan struct{}
	stored  chan Batch
	entries map[types.Hash]*entry
	lanes   map[types.ValidatorID]*lane
	// index names, for a transaction, the first stored batch that carries it.
	index map[types.Hash]*entry
	bytes int
	// The author state: the next sequence number, the transactions waiting for a batch, and the
	// batch that carries each transaction of this node's lane.
	seq        uint64
	queue      []types.Tx
	queued     map[types.Hash]struct{}
	queueBytes int
	armed      bool
	// uncertified is this node's latest batch while it is small and awaits its certificate, since
	// when, and holding reports transactions waiting for that certificate.
	uncertified      types.Hash
	uncertifiedSince time.Time
	holding          bool
	batched          map[types.Hash]types.Hash
	// held are the transactions this node keeps for other lanes, oldest first.
	held []heldTx
	// refs are the batches each proposed block names.
	refs    map[types.Hash]blockRef
	arrived func(types.Hash)
	stats   Stats
}

type lane struct {
	// entries are the stored batches of the author, in sequence order.
	entries []*entry
	// bytes are the bytes of the stored batches no committed block consumed.
	bytes int
}

type entry struct {
	id     types.Hash
	author types.ValidatorID
	batch  *Batch
	// hashes are the hashes of the batch's transactions.
	hashes []types.Hash
	cert   *Cert
	// ack is this node's acknowledgement, kept to answer a retransmission.
	ack types.Signature
	// acks are the acknowledgements of an own batch, by voter.
	acks map[types.ValidatorID]types.Signature
	// referenced is the latest proposed block that names the batch, at refHeight.
	referenced types.Hash
	refHeight  types.Height
	consumed   bool
	since      time.Time
	settled    time.Time
	lastSent   time.Time
	resends    int
	announced  int
	served     map[p2p.PeerID]int
	// wanted is set for a certified batch this node does not hold; askers are the peers it asks
	// in turn, asked how many it asked.
	wanted   bool
	askers   []p2p.PeerID
	asked    int
	lastAsk  time.Time
	counted  bool
	ownBatch bool
}

type heldTx struct {
	tx       types.Tx
	hash     types.Hash
	deadline time.Time
}

type blockRef struct {
	height types.Height
	ids    []types.Hash
}

// New returns a lane service for the validator cfg.Self.
func New(cfg Config) *Lanes {
	return &Lanes{
		cfg:     cfg,
		entries: make(map[types.Hash]*entry),
		lanes:   make(map[types.ValidatorID]*lane),
		index:   make(map[types.Hash]*entry),
		queued:  make(map[types.Hash]struct{}),
		batched: make(map[types.Hash]types.Hash),
		refs:    make(map[types.Hash]blockRef),
		// A restarted author continues past every sequence number it used before.
		seq: uint64(time.Now().UnixNano()),
	}
}

// OnArrival registers a hook called with the id of every batch the node starts to hold.
func (lanes *Lanes) OnArrival(hook func(types.Hash)) {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	lanes.arrived = hook
}

// Start subscribes to the lane messages of wire, which must be started.
func (lanes *Lanes) Start(ctx context.Context, wire transport.Transport) error {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	if lanes.wire != nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	events, err := wire.Subscribe(runCtx, p2p.TopicBatch)
	if err != nil {
		cancel()
		return err
	}
	lanes.wire = wire
	lanes.cancel = cancel
	lanes.done = make(chan struct{})
	lanes.stored = make(chan Batch, storedQueue)
	go lanes.run(runCtx, events, lanes.stored, lanes.done)
	return nil
}

// Stop ends the service and waits for its loop.
func (lanes *Lanes) Stop() {
	lanes.mu.Lock()
	cancel, done := lanes.cancel, lanes.done
	lanes.cancel, lanes.done, lanes.wire, lanes.stored = nil, nil, nil, nil
	lanes.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// Running reports whether the service is started.
func (lanes *Lanes) Running() bool {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	return lanes.wire != nil
}

func (lanes *Lanes) run(ctx context.Context, events <-chan transport.Envelope, stored chan Batch, done chan struct{}) {
	hooked := make(chan struct{})
	defer func() {
		<-hooked
		close(done)
	}()
	go func() {
		defer close(hooked)
		for {
			select {
			case <-ctx.Done():
				return
			case batch := <-stored:
				if hook := lanes.cfg.Stored; hook != nil {
					hook(batch)
				}
			}
		}
	}()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			lanes.handle(ctx, envelope)
		case <-ticker.C:
			lanes.tick(ctx)
		}
	}
}

// Stats returns what the service did so far.
func (lanes *Lanes) Stats() Stats {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	return lanes.stats
}

// Owner returns the validator whose lane carries the transactions of key, a sender or, for a
// transaction without one, its hash: members, the validators of the set sorted by id, take the
// keys in turn by their hash. One lane carries all the transactions of a sender, in the order its
// author admits them, so lanes never split a nonce sequence.
func Owner(members []types.ValidatorID, key []byte) types.ValidatorID {
	if len(members) == 0 {
		return ""
	}
	hash := mempool.HashTx(key)
	return members[binary.BigEndian.Uint64(hash[:8])%uint64(len(members))]
}

// Members returns the validators of set sorted by id.
func Members(set validator.Set) []types.ValidatorID {
	list := set.List()
	members := make([]types.ValidatorID, 0, len(list))
	for _, member := range list {
		members = append(members, member.ID)
	}
	sort.Slice(members, func(left, right int) bool { return members[left] < members[right] })
	return members
}

// Submit queues tx, a transaction of this node's lane, for its next batch.
func (lanes *Lanes) Submit(tx types.Tx) {
	if len(tx) == 0 || len(tx) > MaxBatchBytes {
		return
	}
	hash := mempool.HashTx(tx)
	lanes.mu.Lock()
	if lanes.wire == nil {
		lanes.mu.Unlock()
		return
	}
	if _, found := lanes.batched[hash]; found {
		lanes.mu.Unlock()
		return
	}
	if _, found := lanes.queued[hash]; found {
		lanes.mu.Unlock()
		return
	}
	var sealed [][]types.Tx
	if lanes.queueBytes+len(tx) > MaxBatchBytes {
		sealed = append(sealed, lanes.takeQueueLocked())
	}
	lanes.queue = append(lanes.queue, append(types.Tx(nil), tx...))
	lanes.queued[hash] = struct{}{}
	lanes.queueBytes += len(tx)
	if lanes.queueBytes >= sealBytes || len(lanes.queue) == MaxBatchTxs {
		sealed = append(sealed, lanes.takeQueueLocked())
	} else if !lanes.armed {
		lanes.armed = true
		time.AfterFunc(SealDelay, lanes.sealQueue)
	}
	lanes.mu.Unlock()
	for _, txs := range sealed {
		lanes.seal(context.Background(), txs)
	}
}

// Hold keeps tx, a transaction another validator's lane carries, and batches it on this node's
// lane when the pool still holds it after FallbackDelay.
func (lanes *Lanes) Hold(tx types.Tx) {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	if lanes.wire == nil || len(lanes.held) >= maxHeld {
		return
	}
	lanes.held = append(lanes.held, heldTx{tx: append(types.Tx(nil), tx...), hash: mempool.HashTx(tx), deadline: time.Now().Add(FallbackDelay)})
}

// Flush seals the transactions waiting for a batch now, so a proposal can carry them.
func (lanes *Lanes) Flush(ctx context.Context) {
	lanes.mu.Lock()
	txs := lanes.takeQueueLocked()
	lanes.mu.Unlock()
	lanes.seal(ctx, txs)
}

func (lanes *Lanes) sealQueue() {
	lanes.mu.Lock()
	lanes.armed = false
	if wait := lanes.certWaitLocked(time.Now()); wait > 0 {
		// Sealed once the certificate forms (sealAfterCert), or when certWait passed.
		lanes.holding = true
		time.AfterFunc(wait, lanes.sealQueue)
		lanes.mu.Unlock()
		return
	}
	lanes.holding = false
	txs := lanes.takeQueueLocked()
	lanes.mu.Unlock()
	lanes.seal(context.Background(), txs)
}

// certWaitLocked returns how much longer the waiting transactions wait for the certificate of the
// lane's latest batch, zero when they do not.
func (lanes *Lanes) certWaitLocked(now time.Time) time.Duration {
	if lanes.uncertified == (types.Hash{}) {
		return 0
	}
	return max(0, certWait-now.Sub(lanes.uncertifiedSince))
}

// sealAfterCert seals the transactions that waited for the certificate of id, the lane's latest
// batch.
func (lanes *Lanes) sealAfterCert(ctx context.Context, id types.Hash) {
	lanes.mu.Lock()
	if lanes.uncertified != id {
		lanes.mu.Unlock()
		return
	}
	lanes.uncertified = types.Hash{}
	if !lanes.holding {
		// No transaction waits for the certificate: the seal delay of those queued has not passed.
		lanes.mu.Unlock()
		return
	}
	lanes.holding = false
	txs := lanes.takeQueueLocked()
	lanes.mu.Unlock()
	lanes.seal(ctx, txs)
}

func (lanes *Lanes) takeQueueLocked() []types.Tx {
	txs := lanes.queue
	lanes.queue = nil
	lanes.queueBytes = 0
	return txs
}

// seal makes txs a batch of this node's lane, signs it and disseminates it. The author's own
// acknowledgement counts towards the certificate.
func (lanes *Lanes) seal(ctx context.Context, txs []types.Tx) {
	if len(txs) == 0 {
		return
	}
	lanes.mu.Lock()
	lanes.seq++
	batch := Batch{ChainID: lanes.cfg.ChainID, Author: lanes.cfg.Self, Seq: lanes.seq, Txs: txs}
	lanes.mu.Unlock()
	id := batch.ID()
	signature, signErr := lanes.cfg.Sign(vexocrypto.SignTypeLaneBatch, vexocrypto.DomainLaneBatch, BatchSignBytes(id))
	ack, ackErr := lanes.cfg.Sign(vexocrypto.SignTypeLaneBatchAck, vexocrypto.DomainLaneBatchAck, AckSignBytes(id))
	if signErr != nil || ackErr != nil {
		// The transactions stay pending; they are batched again after the fallback delay.
		lanes.mu.Lock()
		now := time.Now()
		for _, tx := range txs {
			hash := mempool.HashTx(tx)
			delete(lanes.queued, hash)
			if len(lanes.held) < maxHeld {
				lanes.held = append(lanes.held, heldTx{tx: tx, hash: hash, deadline: now.Add(FallbackDelay)})
			}
		}
		lanes.mu.Unlock()
		return
	}
	batch.Signature = signature
	lanes.mu.Lock()
	stored := lanes.storeLocked(batch, id)
	stored.ack = ack
	stored.ownBatch = true
	stored.acks = map[types.ValidatorID]types.Signature{lanes.cfg.Self: ack}
	stored.lastSent = time.Now()
	lanes.uncertified, lanes.uncertifiedSince = types.Hash{}, time.Time{}
	if batch.Size() < paceBytes {
		lanes.uncertified, lanes.uncertifiedSince = id, stored.lastSent
	}
	for _, hash := range stored.hashes {
		delete(lanes.queued, hash)
		lanes.batched[hash] = id
	}
	lanes.stats.Sealed++
	wire := lanes.wire
	lanes.mu.Unlock()
	if wire != nil {
		if data, err := encode(Message{Type: MessageBatch, Batch: &batch}); err == nil && wire.Publish(ctx, p2p.TopicBatch, data) == nil {
			lanes.countSent(len(data), lanes.peerCount(ctx))
		}
	}
	lanes.certifyIfQuorum(ctx, id)
	lanes.ready()
}

// storeLocked stores batch under id and indexes its transactions.
func (lanes *Lanes) storeLocked(batch Batch, id types.Hash) *entry {
	stored, found := lanes.entries[id]
	if !found {
		stored = &entry{id: id, author: batch.Author, since: time.Now()}
		lanes.entries[id] = stored
	}
	if stored.batch != nil {
		return stored
	}
	stored.batch = &batch
	stored.wanted = false
	stored.hashes = make([]types.Hash, len(batch.Txs))
	for index, tx := range batch.Txs {
		stored.hashes[index] = mempool.HashTx(tx)
	}
	size := batch.Size()
	lanes.bytes += size
	authorLane := lanes.lanes[batch.Author]
	if authorLane == nil {
		authorLane = &lane{}
		lanes.lanes[batch.Author] = authorLane
	}
	authorLane.bytes += size
	position := sort.Search(len(authorLane.entries), func(index int) bool {
		return authorLane.entries[index].batch.Seq > batch.Seq
	})
	authorLane.entries = append(authorLane.entries, nil)
	copy(authorLane.entries[position+1:], authorLane.entries[position:])
	authorLane.entries[position] = stored
	for _, hash := range stored.hashes {
		if _, indexed := lanes.index[hash]; !indexed {
			lanes.index[hash] = stored
		}
	}
	return stored
}

// removeLocked forgets a stored batch.
func (lanes *Lanes) removeLocked(stored *entry) {
	delete(lanes.entries, stored.id)
	if stored.batch == nil {
		return
	}
	size := stored.batch.Size()
	lanes.bytes -= size
	if authorLane := lanes.lanes[stored.author]; authorLane != nil {
		if !stored.consumed {
			authorLane.bytes -= size
		}
		for index, candidate := range authorLane.entries {
			if candidate == stored {
				authorLane.entries = append(authorLane.entries[:index], authorLane.entries[index+1:]...)
				break
			}
		}
		if len(authorLane.entries) == 0 {
			delete(lanes.lanes, stored.author)
		}
	}
	for _, hash := range stored.hashes {
		if lanes.index[hash] == stored {
			delete(lanes.index, hash)
		}
	}
}

// Get returns the stored batch of id.
func (lanes *Lanes) Get(id types.Hash) (Batch, bool) {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	stored, found := lanes.entries[id]
	if !found || stored.batch == nil {
		return Batch{}, false
	}
	return *stored.batch, true
}

// Carries reports whether a batch the node stores carries the transaction of hash.
func (lanes *Lanes) Carries(hash types.Hash) bool {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	_, found := lanes.index[hash]
	return found
}

// Covering returns the stored batches whose transactions make up txs, in the order txs first names
// them, for a proposal that names batches instead of carrying their transactions.
func (lanes *Lanes) Covering(txs []types.Tx) []Batch {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	seen := make(map[types.Hash]struct{})
	var batches []Batch
	for _, tx := range txs {
		stored, found := lanes.index[mempool.HashTx(tx)]
		if !found {
			continue
		}
		if _, named := seen[stored.id]; named {
			continue
		}
		seen[stored.id] = struct{}{}
		batches = append(batches, *stored.batch)
	}
	return batches
}

// Candidates returns the batches the next block may carry, up to maxBytes of transactions: in each
// lane the batches no proposed block names yet, in sequence order, up to the first one not
// certified (this node's own batches need no certificate: it serves them itself). The lanes take
// turns batch by batch, so every validator's lane gets an equal share of a full block.
func (lanes *Lanes) Candidates(maxBytes int) []Batch {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	authors := make([]types.ValidatorID, 0, len(lanes.lanes))
	for author := range lanes.lanes {
		authors = append(authors, author)
	}
	sort.Slice(authors, func(left, right int) bool { return authors[left] < authors[right] })
	cursors := make([]int, len(authors))
	finished := make([]bool, len(authors))
	var batches []Batch
	total := 0
	for progress := true; progress; {
		progress = false
		for turn, author := range authors {
			if finished[turn] {
				continue
			}
			entries := lanes.lanes[author].entries
			for cursors[turn] < len(entries) {
				stored := entries[cursors[turn]]
				if stored.consumed || stored.referenced != (types.Hash{}) {
					cursors[turn]++
					continue
				}
				break
			}
			if cursors[turn] == len(entries) {
				finished[turn] = true
				continue
			}
			stored := entries[cursors[turn]]
			if !lanes.proposableLocked(stored) || total+stored.batch.Size() > maxBytes {
				finished[turn] = true
				continue
			}
			batches = append(batches, *stored.batch)
			total += stored.batch.Size()
			cursors[turn]++
			progress = true
		}
	}
	return batches
}

func (lanes *Lanes) proposableLocked(stored *entry) bool {
	return stored.batch != nil && (stored.cert != nil || stored.author == lanes.cfg.Self)
}

// HasProposable reports whether a batch waits for a block.
func (lanes *Lanes) HasProposable() bool {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	for _, authorLane := range lanes.lanes {
		for _, stored := range authorLane.entries {
			if stored.consumed || stored.referenced != (types.Hash{}) {
				continue
			}
			if lanes.proposableLocked(stored) {
				return true
			}
			break
		}
	}
	return len(lanes.queue) > 0
}

// Reference records that the proposed block of blockHash, at height, names the batches of ids:
// they are not proposed again unless a block of that height commits without them.
func (lanes *Lanes) Reference(blockHash types.Hash, height types.Height, ids []types.Hash) {
	if len(ids) == 0 {
		return
	}
	lanes.mu.Lock()
	defer lanes.mu.Unlock()
	lanes.refs[blockHash] = blockRef{height: height, ids: append([]types.Hash(nil), ids...)}
	for _, id := range ids {
		if stored, found := lanes.entries[id]; found && !stored.consumed {
			stored.referenced = blockHash
			stored.refHeight = height
		}
	}
}

// Committed records the commit of the block of blockHash at height with txs. The batches it names,
// and those that carry one of its transactions, are consumed; the batches only an abandoned block
// of that height or below named are proposable again. The transactions of this node's consumed
// batches that the pool still holds, which the block left out, are batched again.
func (lanes *Lanes) Committed(blockHash types.Hash, height types.Height, txs []types.Tx) {
	now := time.Now()
	var released []types.Tx
	lanes.mu.Lock()
	consume := func(stored *entry) {
		if stored.consumed {
			return
		}
		stored.consumed = true
		stored.settled = now
		stored.referenced = types.Hash{}
		if stored.batch == nil {
			return
		}
		if authorLane := lanes.lanes[stored.author]; authorLane != nil {
			authorLane.bytes -= stored.batch.Size()
		}
		if stored.ownBatch {
			for index, hash := range stored.hashes {
				if lanes.batched[hash] == stored.id {
					delete(lanes.batched, hash)
					released = append(released, stored.batch.Txs[index])
				}
			}
		}
	}
	if ref, found := lanes.refs[blockHash]; found {
		for _, id := range ref.ids {
			if stored, found := lanes.entries[id]; found {
				consume(stored)
			}
		}
	}
	for _, tx := range txs {
		if stored, found := lanes.index[mempool.HashTx(tx)]; found {
			consume(stored)
		}
	}
	for hash, ref := range lanes.refs {
		if ref.height > height {
			continue
		}
		for _, id := range ref.ids {
			if stored, found := lanes.entries[id]; found && stored.referenced == hash {
				stored.referenced = types.Hash{}
			}
		}
		delete(lanes.refs, hash)
	}
	pending := lanes.cfg.Pending
	lanes.mu.Unlock()
	for _, tx := range released {
		if pending != nil && pending(mempool.HashTx(tx)) {
			lanes.Submit(tx)
		}
	}
	lanes.ready()
}

// Request asks peers, in turn, for the batches of ids this node does not hold.
func (lanes *Lanes) Request(ctx context.Context, ids []types.Hash, peers []p2p.PeerID) {
	lanes.mu.Lock()
	var ask []types.Hash
	now := time.Now()
	for _, id := range ids {
		stored, found := lanes.entries[id]
		if !found {
			stored = &entry{id: id, since: now}
			lanes.entries[id] = stored
		}
		if stored.batch != nil {
			continue
		}
		stored.wanted = true
		for _, peer := range peers {
			if peer != "" && peer != p2p.PeerID(lanes.cfg.Self) && !containsPeer(stored.askers, peer) {
				stored.askers = append(stored.askers, peer)
			}
		}
		if now.Sub(stored.lastAsk) >= requestInterval {
			ask = append(ask, id)
		}
	}
	lanes.mu.Unlock()
	lanes.ask(ctx, ask)
}

// ask sends each batch of ids a request to the next peer it may ask.
func (lanes *Lanes) ask(ctx context.Context, ids []types.Hash) {
	byPeer := make(map[p2p.PeerID][]types.Hash)
	lanes.mu.Lock()
	wire := lanes.wire
	now := time.Now()
	for _, id := range ids {
		stored, found := lanes.entries[id]
		if !found || stored.batch != nil || len(stored.askers) == 0 {
			continue
		}
		peer := stored.askers[stored.asked%len(stored.askers)]
		stored.asked++
		stored.lastAsk = now
		if len(byPeer[peer]) < MaxRequestIDs {
			byPeer[peer] = append(byPeer[peer], id)
		}
		lanes.stats.Requested++
	}
	lanes.mu.Unlock()
	if wire == nil {
		return
	}
	for peer, wanted := range byPeer {
		if data, err := encode(Message{Type: MessageRequest, Request: &Request{IDs: wanted}}); err == nil && wire.Send(ctx, peer, p2p.TopicBatch, data) == nil {
			lanes.countSent(len(data), 1)
		}
	}
}

func containsPeer(peers []p2p.PeerID, peer p2p.PeerID) bool {
	for _, candidate := range peers {
		if candidate == peer {
			return true
		}
	}
	return false
}

func (lanes *Lanes) handle(ctx context.Context, envelope transport.Envelope) {
	if admit := lanes.cfg.Admit; admit != nil && !admit(ctx, envelope.From) {
		return
	}
	message, err := Decode(envelope.Data)
	if err != nil {
		lanes.observe(ctx, envelope.From, false)
		return
	}
	set, err := lanes.cfg.ValidatorSet(ctx)
	if err != nil {
		// The set is not known yet; the sender is not at fault.
		return
	}
	switch message.Type {
	case MessageBatch:
		err = lanes.onBatch(ctx, *message.Batch, set)
	case MessageAck:
		err = lanes.onAck(ctx, *message.Ack, set)
	case MessageCert:
		err = lanes.onCert(ctx, *message.Cert, set)
	case MessageRequest:
		lanes.onRequest(ctx, envelope.From, *message.Request)
	}
	lanes.observe(ctx, envelope.From, err == nil)
}

func (lanes *Lanes) observe(ctx context.Context, peer p2p.PeerID, valid bool) {
	if observe := lanes.cfg.Observe; observe != nil {
		observe(ctx, peer, valid)
	}
}

// onBatch stores a batch of a validator's lane and acknowledges it to its author. A batch the
// node already holds is acknowledged again: the author retransmits only what was not acknowledged.
func (lanes *Lanes) onBatch(ctx context.Context, batch Batch, set validator.Set) error {
	if err := VerifyBatch(lanes.cfg.ChainID, batch, set, lanes.cfg.Signatures); err != nil {
		return err
	}
	id := batch.ID()
	lanes.mu.Lock()
	stored, found := lanes.entries[id]
	if !found || stored.batch == nil {
		authorLane := lanes.lanes[batch.Author]
		laneBytes := 0
		if authorLane != nil {
			laneBytes = authorLane.bytes
		}
		if laneBytes+batch.Size() > maxLaneBytes || lanes.bytes+batch.Size() > maxPoolBytes {
			// Over its share the author's batches wait; it sends them again.
			lanes.mu.Unlock()
			return nil
		}
		stored = lanes.storeLocked(batch, id)
		if batch.Author != lanes.cfg.Self {
			lanes.stats.Received++
			select {
			case lanes.stored <- batch:
			default:
			}
		}
	}
	ack := stored.ack
	certified := stored.cert != nil
	arrived := lanes.arrived
	lanes.mu.Unlock()
	if arrived != nil {
		arrived(id)
	}
	if certified {
		lanes.ready()
	}
	if batch.Author == lanes.cfg.Self {
		return nil
	}
	if _, member := set.Get(lanes.cfg.Self); !member {
		return nil
	}
	if ack == nil {
		signature, err := lanes.cfg.Sign(vexocrypto.SignTypeLaneBatchAck, vexocrypto.DomainLaneBatchAck, AckSignBytes(id))
		if err != nil {
			return nil
		}
		ack = signature
		lanes.mu.Lock()
		stored.ack = signature
		lanes.mu.Unlock()
	}
	lanes.send(ctx, p2p.PeerID(batch.Author), Message{Type: MessageAck, Ack: &Ack{BatchID: id, Voter: lanes.cfg.Self, Signature: ack}})
	return nil
}

// onAck records an acknowledgement of one of this node's batches and certifies the batch once a
// quorum acknowledged it.
func (lanes *Lanes) onAck(ctx context.Context, ack Ack, set validator.Set) error {
	if err := VerifyAck(ack, set, lanes.cfg.Signatures); err != nil {
		return err
	}
	lanes.mu.Lock()
	stored, found := lanes.entries[ack.BatchID]
	if !found || !stored.ownBatch || stored.cert != nil {
		lanes.mu.Unlock()
		return nil
	}
	stored.acks[ack.Voter] = ack.Signature
	lanes.mu.Unlock()
	lanes.certifyIfQuorum(ctx, ack.BatchID)
	return nil
}

// certifyIfQuorum certifies an own batch whose acknowledgements hold a quorum of the current set,
// and announces the certificate.
func (lanes *Lanes) certifyIfQuorum(ctx context.Context, id types.Hash) {
	set, err := lanes.cfg.ValidatorSet(ctx)
	if err != nil {
		return
	}
	lanes.mu.Lock()
	stored, found := lanes.entries[id]
	if !found || !stored.ownBatch || stored.cert != nil {
		lanes.mu.Unlock()
		return
	}
	var power types.VotingPower
	var signers []types.ValidatorID
	var signatures []types.Signature
	for _, member := range set.List() {
		signature, acked := stored.acks[member.ID]
		if !acked {
			continue
		}
		if power, err = types.AddVotingPower(power, member.VotingPower); err != nil {
			lanes.mu.Unlock()
			return
		}
		signers = append(signers, member.ID)
		signatures = append(signatures, signature)
	}
	lanes.mu.Unlock()
	if !types.HasTwoThirdsQuorum(power, set.TotalVotingPower()) || lanes.cfg.Aggregator == nil {
		return
	}
	aggregate, err := lanes.cfg.Aggregator.Aggregate(signatures)
	if err != nil {
		return
	}
	cert := Cert{BatchID: id, Author: lanes.cfg.Self, Signers: finality.EncodeSigners(signers), Signature: aggregate}
	lanes.mu.Lock()
	if stored.cert != nil {
		lanes.mu.Unlock()
		return
	}
	stored.cert = &cert
	stored.lastSent = time.Now()
	stored.announced = 1
	lanes.stats.Certified++
	lanes.mu.Unlock()
	lanes.broadcast(ctx, Message{Type: MessageCert, Cert: &cert})
	lanes.sealAfterCert(ctx, id)
	lanes.ready()
}

// onCert records that a batch is certified, and asks for the batch when the node does not hold it:
// its author and the validators that acknowledged it store it.
func (lanes *Lanes) onCert(ctx context.Context, cert Cert, set validator.Set) error {
	if err := VerifyCert(cert, set, lanes.cfg.Aggregates); err != nil {
		return err
	}
	lanes.mu.Lock()
	stored, found := lanes.entries[cert.BatchID]
	if !found {
		stored = &entry{id: cert.BatchID, author: cert.Author, since: time.Now()}
		lanes.entries[cert.BatchID] = stored
	}
	if stored.cert != nil {
		lanes.mu.Unlock()
		return nil
	}
	stored.cert = &cert
	lanes.stats.Certified++
	held := stored.batch != nil
	lanes.mu.Unlock()
	if held {
		lanes.ready()
		return nil
	}
	peers := []p2p.PeerID{p2p.PeerID(cert.Author)}
	if signers, err := finality.ParseSigners(cert.Signers); err == nil {
		for _, signer := range signers {
			peers = append(peers, p2p.PeerID(signer))
		}
	}
	lanes.Request(ctx, []types.Hash{cert.BatchID}, peers)
	return nil
}

// onRequest serves peer the batches it asked for that this node holds, each a bounded number of
// times.
func (lanes *Lanes) onRequest(ctx context.Context, peer p2p.PeerID, request Request) {
	var batches []Batch
	lanes.mu.Lock()
	for _, id := range request.IDs {
		stored, found := lanes.entries[id]
		if !found || stored.batch == nil {
			continue
		}
		if stored.served == nil {
			stored.served = make(map[p2p.PeerID]int)
		}
		if stored.served[peer] >= maxServes {
			continue
		}
		stored.served[peer]++
		batches = append(batches, *stored.batch)
		lanes.stats.Served++
	}
	lanes.mu.Unlock()
	for index := range batches {
		lanes.send(ctx, peer, Message{Type: MessageBatch, Batch: &batches[index]})
	}
}

func (lanes *Lanes) send(ctx context.Context, peer p2p.PeerID, message Message) {
	lanes.mu.Lock()
	wire := lanes.wire
	lanes.mu.Unlock()
	if wire == nil {
		return
	}
	data, err := encode(message)
	if err != nil {
		return
	}
	if wire.Send(ctx, peer, p2p.TopicBatch, data) == nil {
		lanes.countSent(len(data), 1)
	}
}

func (lanes *Lanes) broadcast(ctx context.Context, message Message) {
	lanes.mu.Lock()
	wire := lanes.wire
	lanes.mu.Unlock()
	if wire == nil {
		return
	}
	data, err := encode(message)
	if err != nil {
		return
	}
	if wire.Publish(ctx, p2p.TopicBatch, data) == nil {
		lanes.countSent(len(data), lanes.peerCount(ctx))
	}
}

// peerCount returns how many validators a broadcast reaches.
func (lanes *Lanes) peerCount(ctx context.Context) int {
	set, err := lanes.cfg.ValidatorSet(ctx)
	if err != nil {
		return 0
	}
	count := len(set.List())
	if _, member := set.Get(lanes.cfg.Self); member {
		count--
	}
	return count
}

func (lanes *Lanes) countSent(size int, peers int) {
	if peers <= 0 {
		return
	}
	lanes.mu.Lock()
	lanes.stats.SentBytes += uint64(size) * uint64(peers)
	lanes.mu.Unlock()
}

func (lanes *Lanes) ready() {
	if ready := lanes.cfg.Ready; ready != nil {
		ready()
	}
}

// tick retransmits what was not acknowledged, announces again the certificates no proposal named,
// asks again for wanted batches, batches the held transactions whose owner did not, and expires
// old batches.
func (lanes *Lanes) tick(ctx context.Context) {
	set, err := lanes.cfg.ValidatorSet(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	type resend struct {
		batch Batch
		peers []p2p.PeerID
	}
	var resends []resend
	var certs []Cert
	var wanted []types.Hash
	var fallback []types.Tx
	var released []types.Tx
	lanes.mu.Lock()
	for _, stored := range lanes.entries {
		switch {
		case stored.consumed:
			if now.Sub(stored.settled) > consumedRetention {
				lanes.removeLocked(stored)
			}
			continue
		case now.Sub(stored.since) > batchTTL:
			if stored.ownBatch && stored.batch != nil {
				for index, hash := range stored.hashes {
					if lanes.batched[hash] == stored.id {
						delete(lanes.batched, hash)
						released = append(released, stored.batch.Txs[index])
					}
				}
			}
			if !stored.ownBatch && stored.batch != nil {
				// No block took the batch up: the validators that hold its transactions batch them.
				for index, tx := range stored.batch.Txs {
					if len(lanes.held) < maxHeld {
						lanes.held = append(lanes.held, heldTx{tx: tx, hash: stored.hashes[index], deadline: now})
					}
				}
			}
			lanes.removeLocked(stored)
			continue
		}
		if stored.ownBatch && stored.cert == nil && stored.resends < maxResends && now.Sub(stored.lastSent) >= resendInterval<<stored.resends {
			var peers []p2p.PeerID
			for _, member := range set.List() {
				if _, acked := stored.acks[member.ID]; !acked {
					peers = append(peers, p2p.PeerID(member.ID))
				}
			}
			stored.lastSent = now
			stored.resends++
			resends = append(resends, resend{batch: *stored.batch, peers: peers})
		}
		if stored.ownBatch && stored.cert != nil && stored.referenced == (types.Hash{}) && stored.announced < maxAnnouncements && now.Sub(stored.lastSent) >= certResendInterval {
			stored.lastSent = now
			stored.announced++
			certs = append(certs, *stored.cert)
		}
		if stored.wanted && stored.batch == nil && now.Sub(stored.lastAsk) >= requestInterval {
			wanted = append(wanted, stored.id)
		}
	}
	kept := lanes.held[:0]
	for _, held := range lanes.held {
		if now.Before(held.deadline) {
			kept = append(kept, held)
			continue
		}
		if _, carried := lanes.index[held.hash]; !carried {
			fallback = append(fallback, held.tx)
		}
	}
	lanes.held = kept
	pending := lanes.cfg.Pending
	lanes.mu.Unlock()

	for _, retry := range resends {
		data, err := encode(Message{Type: MessageBatch, Batch: &retry.batch})
		if err != nil {
			continue
		}
		for _, peer := range retry.peers {
			lanes.mu.Lock()
			wire := lanes.wire
			lanes.mu.Unlock()
			if wire != nil && wire.Send(ctx, peer, p2p.TopicBatch, data) == nil {
				lanes.countSent(len(data), 1)
			}
		}
	}
	for index := range certs {
		lanes.broadcast(ctx, Message{Type: MessageCert, Cert: &certs[index]})
	}
	lanes.ask(ctx, wanted)
	for _, tx := range append(fallback, released...) {
		if pending == nil || pending(mempool.HashTx(tx)) {
			lanes.Submit(tx)
		}
	}
}

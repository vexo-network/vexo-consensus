package mempool

import (
	"bytes"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrEmptyTx                = errors.New("transaction is empty")
	ErrDuplicateTx            = errors.New("duplicate transaction")
	ErrTxTooLarge             = errors.New("transaction exceeds maximum size")
	ErrMempoolFull            = errors.New("mempool is full")
	ErrInvalidMaxBytes        = errors.New("max bytes must be greater than zero")
	ErrInsufficientFee        = errors.New("transaction fee is below minimum")
	ErrReplacementUnderpriced = errors.New("replacement transaction is underpriced")
	ErrTxNotReady             = errors.New("transaction is not executable yet")
	ErrInvalidTx              = errors.New("transaction is malformed")
	ErrSenderLimit            = errors.New("sender has too many pending transactions")
)

// FIFOConfig configures a FIFO pool. Zero limits mean "unlimited".
type FIFOConfig struct {
	Author     types.ValidatorID
	MaxTxBytes int64
	MaxTxs     int
	// MaxTotalBytes bounds the combined size of every pending transaction.
	MaxTotalBytes int64
	// MaxTxsPerSender bounds the pending transactions of one signer so a single
	// account cannot occupy the whole pool.
	MaxTxsPerSender    int
	AllowDuplicate     bool
	SeenTTL            time.Duration
	MinFee             uint64
	EnablePriority     bool
	WALPath            string
	EnableReplacement  bool
	ReplacementBumpBPS uint64
	// ReplayCheckTx revalidates a pending transaction against current state. It
	// runs outside the pool lock and may be slow. ErrTxNotReady keeps the
	// transaction pending; any other error drops it.
	ReplayCheckTx func(context.Context, types.Tx) error `json:"-"`
}

// entry is one pending transaction with the metadata parsed once at admission.
// Ranking, replacement and the per-sender accounting read this cache; they
// never re-parse the payload.
type entry struct {
	tx       types.Tx
	hash     types.Hash
	size     int64
	fee      uint64
	priority uint64
	sender   string
	key      string
	sequence uint64
	// evictAt is the entry's position in the pool's eviction heap.
	evictAt int
}

// admission is the outcome of a successful admission check: the entry the new
// transaction replaces, or the entries evicted to make room for it.
type admission struct {
	replaced *entry
	evicted  []*entry
}

type FIFO struct {
	mu           sync.Mutex
	config       FIFOConfig
	txs          []*entry
	index        map[types.Hash]*entry
	replacements map[string]*entry
	senders      map[string]int
	// evictions orders the pending entries by evictsBefore, so the entry a full pool evicts
	// first is found without scanning the pool.
	evictions  evictionHeap
	totalBytes int64
	sequence   uint64
	seen       map[types.Hash]time.Time
	now        func() time.Time
}

func NewFIFO(config FIFOConfig) *FIFO {
	return &FIFO{
		config:       config,
		index:        make(map[types.Hash]*entry),
		replacements: make(map[string]*entry),
		senders:      make(map[string]int),
		seen:         make(map[types.Hash]time.Time),
		now:          time.Now,
	}
}

func (pool *FIFO) CheckTx(ctx context.Context, tx types.Tx) error {
	candidate, err := pool.prepare(ctx, tx)
	if err != nil {
		return err
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	_, err = pool.admissionLocked(candidate)
	return err
}

// prepare runs the checks that need no pool state and parses the transaction
// metadata once, outside the pool lock.
func (pool *FIFO) prepare(ctx context.Context, tx types.Tx) (*entry, error) {
	if err := pool.checkContext(ctx); err != nil {
		return nil, err
	}
	if len(tx) == 0 {
		return nil, ErrEmptyTx
	}
	if pool.config.MaxTxBytes > 0 && int64(len(tx)) > pool.config.MaxTxBytes {
		return nil, ErrTxTooLarge
	}
	return newEntry(tx)
}

func newEntry(tx types.Tx) (*entry, error) {
	meta, err := txcodec.ParseMeta(tx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTx, err)
	}
	copied := append(types.Tx(nil), tx...)
	candidate := &entry{
		tx:       copied,
		hash:     HashTx(copied),
		size:     int64(len(copied)),
		fee:      meta.FeeSaturated(),
		priority: meta.Priority,
	}
	if meta.Signer != "" {
		candidate.sender = txcodec.NormalizeAddress(meta.Signer)
	}
	if key, ok := replacementKey(meta); ok {
		candidate.key = key
	}
	return candidate, nil
}

// admissionLocked decides whether candidate may enter the pool without
// changing the pool.
func (pool *FIFO) admissionLocked(candidate *entry) (admission, error) {
	if old := pool.replacementTarget(candidate); old != nil {
		if err := pool.checkReplacement(old, candidate); err != nil {
			return admission{}, err
		}
		if pool.config.MinFee > 0 && candidate.fee < pool.config.MinFee {
			return admission{}, ErrInsufficientFee
		}
		if pool.config.MaxTotalBytes > 0 && pool.totalBytes-old.size+candidate.size > pool.config.MaxTotalBytes {
			return admission{}, ErrMempoolFull
		}
		return admission{replaced: old}, nil
	}
	if pool.config.MaxTxsPerSender > 0 && candidate.sender != "" && pool.senders[candidate.sender] >= pool.config.MaxTxsPerSender {
		return admission{}, ErrSenderLimit
	}
	evicted, err := pool.evictionPlan(candidate)
	if err != nil {
		return admission{}, err
	}
	pool.pruneSeenExpired()
	if !pool.config.AllowDuplicate {
		if _, found := pool.index[candidate.hash]; found {
			return admission{}, ErrDuplicateTx
		}
		if pool.seenRecently(candidate.hash) {
			return admission{}, ErrDuplicateTx
		}
	}
	if pool.config.MinFee > 0 && candidate.fee < pool.config.MinFee {
		return admission{}, ErrInsufficientFee
	}
	return admission{evicted: evicted}, nil
}

// evictionPlan returns the lowest-fee entries that must leave the pool to make
// room for candidate. A candidate only displaces entries that pay strictly less
// than it does; among equal fees the newest entry is evicted first, so an
// attacker cannot push out older equally-paying transactions.
func (pool *FIFO) evictionPlan(candidate *entry) ([]*entry, error) {
	needCount := 0
	if pool.config.MaxTxs > 0 {
		needCount = len(pool.txs) + 1 - pool.config.MaxTxs
	}
	var needBytes int64
	if pool.config.MaxTotalBytes > 0 {
		if candidate.size > pool.config.MaxTotalBytes {
			return nil, ErrMempoolFull
		}
		needBytes = pool.totalBytes + candidate.size - pool.config.MaxTotalBytes
	}
	if needCount <= 0 && needBytes <= 0 {
		return nil, nil
	}
	if needBytes <= 0 && needCount == 1 {
		victim := pool.lowestEntry()
		if victim == nil || victim.fee >= candidate.fee {
			return nil, ErrMempoolFull
		}
		return []*entry{victim}, nil
	}
	var victims []*entry
	var freed int64
	for victim := range pool.evictions.ascending() {
		if len(victims) >= needCount && freed >= needBytes {
			break
		}
		if victim.fee >= candidate.fee {
			return nil, ErrMempoolFull
		}
		victims = append(victims, victim)
		freed += victim.size
	}
	if len(victims) < needCount || freed < needBytes {
		return nil, ErrMempoolFull
	}
	return victims, nil
}

func (pool *FIFO) lowestEntry() *entry {
	if len(pool.evictions) == 0 {
		return nil
	}
	return pool.evictions[0]
}

// evictionHeap is a binary min-heap of entries under evictsBefore.
type evictionHeap []*entry

func (h evictionHeap) Len() int           { return len(h) }
func (h evictionHeap) Less(i, j int) bool { return evictsBefore(h[i], h[j]) }
func (h evictionHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].evictAt = i
	h[j].evictAt = j
}

func (h *evictionHeap) Push(value any) {
	pending := value.(*entry)
	pending.evictAt = len(*h)
	*h = append(*h, pending)
}

func (h *evictionHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	last.evictAt = -1
	return last
}

// ascending yields the entries in eviction order without changing the heap, in O(k log k) for
// the first k: a frontier of heap positions, smallest first, whose children join it as each is
// yielded.
func (h evictionHeap) ascending() func(func(*entry) bool) {
	return func(yield func(*entry) bool) {
		if len(h) == 0 {
			return
		}
		frontier := &positionHeap{entries: h, positions: []int{0}}
		for frontier.Len() > 0 {
			position := heap.Pop(frontier).(int)
			if !yield(h[position]) {
				return
			}
			for _, child := range [2]int{2*position + 1, 2*position + 2} {
				if child < len(h) {
					heap.Push(frontier, child)
				}
			}
		}
	}
}

type positionHeap struct {
	entries   evictionHeap
	positions []int
}

func (h *positionHeap) Len() int { return len(h.positions) }
func (h *positionHeap) Less(i, j int) bool {
	return evictsBefore(h.entries[h.positions[i]], h.entries[h.positions[j]])
}
func (h *positionHeap) Swap(i, j int) {
	h.positions[i], h.positions[j] = h.positions[j], h.positions[i]
}
func (h *positionHeap) Push(value any) { h.positions = append(h.positions, value.(int)) }
func (h *positionHeap) Pop() any {
	last := h.positions[len(h.positions)-1]
	h.positions = h.positions[:len(h.positions)-1]
	return last
}

func evictsBefore(left *entry, right *entry) bool {
	if left.fee != right.fee {
		return left.fee < right.fee
	}
	return left.sequence > right.sequence
}

func (pool *FIFO) AddTx(ctx context.Context, tx types.Tx) error {
	return pool.addTxWithHook(ctx, tx, nil)
}

func (pool *FIFO) addTxWithHook(ctx context.Context, tx types.Tx, beforeCommit func() error) error {
	candidate, err := pool.prepare(ctx, tx)
	if err != nil {
		return err
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	plan, err := pool.admissionLocked(candidate)
	if err != nil {
		return err
	}
	if beforeCommit != nil {
		if err := beforeCommit(); err != nil {
			return err
		}
	}
	for _, victim := range plan.evicted {
		pool.removeLocked(victim)
		pool.markSeen(victim.hash)
	}
	if plan.replaced != nil {
		pool.replaceLocked(plan.replaced, candidate)
		return nil
	}
	pool.insertLocked(candidate)
	return nil
}

func (pool *FIFO) markCommittedWithHook(ctx context.Context, committed []types.Tx, beforeCommit func() error) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if err := pool.checkContext(ctx); err != nil {
		return err
	}
	if len(committed) == 0 {
		return nil
	}
	if beforeCommit != nil {
		if err := beforeCommit(); err != nil {
			return err
		}
	}
	return pool.markCommittedLocked(committed)
}

func (pool *FIFO) insertLocked(candidate *entry) {
	candidate.sequence = pool.sequence
	pool.sequence++
	pool.txs = append(pool.txs, candidate)
	pool.index[candidate.hash] = candidate
	if candidate.key != "" {
		pool.replacements[candidate.key] = candidate
	}
	if candidate.sender != "" {
		pool.senders[candidate.sender]++
	}
	pool.totalBytes += candidate.size
	heap.Push(&pool.evictions, candidate)
	pool.markSeen(candidate.hash)
}

// replaceLocked swaps old for candidate in place so the replacement keeps the
// queue position of the transaction it replaces.
func (pool *FIFO) replaceLocked(old *entry, candidate *entry) {
	position := pool.positionOf(old)
	if position < 0 {
		pool.insertLocked(candidate)
		return
	}
	candidate.sequence = old.sequence
	pool.txs[position] = candidate
	pool.unindexLocked(old)
	pool.index[candidate.hash] = candidate
	if candidate.key != "" {
		pool.replacements[candidate.key] = candidate
	}
	if candidate.sender != "" {
		pool.senders[candidate.sender]++
	}
	pool.totalBytes += candidate.size
	heap.Push(&pool.evictions, candidate)
	pool.markSeen(old.hash)
	pool.markSeen(candidate.hash)
}

func (pool *FIFO) removeLocked(victim *entry) {
	position := pool.positionOf(victim)
	if position < 0 {
		return
	}
	copy(pool.txs[position:], pool.txs[position+1:])
	pool.txs[len(pool.txs)-1] = nil
	pool.txs = pool.txs[:len(pool.txs)-1]
	pool.unindexLocked(victim)
}

// unindexLocked removes the derived lookups and accounting of entry.
func (pool *FIFO) unindexLocked(removed *entry) {
	if pool.index[removed.hash] == removed {
		delete(pool.index, removed.hash)
	}
	if removed.key != "" && pool.replacements[removed.key] == removed {
		delete(pool.replacements, removed.key)
	}
	if removed.sender != "" {
		if pool.senders[removed.sender] <= 1 {
			delete(pool.senders, removed.sender)
		} else {
			pool.senders[removed.sender]--
		}
	}
	pool.totalBytes -= removed.size
	if removed.evictAt >= 0 && removed.evictAt < len(pool.evictions) && pool.evictions[removed.evictAt] == removed {
		heap.Remove(&pool.evictions, removed.evictAt)
	}
}

func (pool *FIFO) positionOf(target *entry) int {
	for position, candidate := range pool.txs {
		if candidate == target {
			return position
		}
	}
	return -1
}

func (pool *FIFO) BuildBatch(ctx context.Context, maxBytes int64, proposed map[types.Hash]struct{}) (Batch, error) {
	if err := pool.checkContext(ctx); err != nil {
		return Batch{}, err
	}
	if maxBytes <= 0 {
		return Batch{}, ErrInvalidMaxBytes
	}
	var totalBytes int64
	batch := Batch{Author: pool.config.Author}
	err := pool.eachLiveEntry(ctx, proposed, func(pending *entry) bool {
		if totalBytes+pending.size > maxBytes {
			return false
		}
		totalBytes += pending.size
		batch.Txs = append(batch.Txs, append(types.Tx(nil), pending.tx...))
		return true
	})
	if err != nil {
		return Batch{}, err
	}
	batch.ID = HashBatch(batch)
	return batch, nil
}

func (pool *FIFO) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	if err := pool.checkContext(ctx); err != nil {
		return nil, err
	}
	var live []types.Tx
	err := pool.eachLiveEntry(ctx, nil, func(pending *entry) bool {
		live = append(live, append(types.Tx(nil), pending.tx...))
		return true
	})
	if err != nil {
		return nil, err
	}
	return live, nil
}

// SnapshotTxs returns the currently admitted entries without replaying CheckTx.
// Proposal construction still uses PendingTxs and performs the stateful replay
// check; read-only RPC polling should not execute transactions again.
func (pool *FIFO) SnapshotTxs(ctx context.Context) ([]types.Tx, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if err := pool.checkContext(ctx); err != nil {
		return nil, err
	}
	return copyTxs(pool.txs), nil
}

func (pool *FIFO) MarkCommitted(ctx context.Context, committed []types.Tx) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if err := pool.checkContext(ctx); err != nil {
		return err
	}
	return pool.markCommittedLocked(committed)
}

// markCommittedLocked drops every pending transaction whose bytes or account
// sequence slot (signer and nonce) was committed.
func (pool *FIFO) markCommittedLocked(committed []types.Tx) error {
	if len(committed) == 0 {
		return nil
	}
	pool.pruneSeenExpired()

	hashes := make(map[types.Hash]struct{}, len(committed))
	keys := make(map[string]struct{}, len(committed))
	for _, tx := range committed {
		hashes[HashTx(tx)] = struct{}{}
		if key, ok := ReplacementKey(tx); ok {
			keys[key] = struct{}{}
		}
	}
	pool.filterLocked(func(pending *entry) bool {
		if _, found := hashes[pending.hash]; found {
			return false
		}
		if pending.key != "" {
			if _, found := keys[pending.key]; found {
				return false
			}
		}
		return true
	})
	return nil
}

// filterLocked keeps the entries for which keep returns true and removes the rest, marking them
// recently seen. Only the removed entries leave the derived lookups, so a commit costs what it
// removes plus one pass over the queue.
func (pool *FIFO) filterLocked(keep func(*entry) bool) int {
	remaining := pool.txs[:0]
	removed := 0
	for _, pending := range pool.txs {
		if keep(pending) {
			remaining = append(remaining, pending)
			continue
		}
		removed++
		pool.unindexLocked(pending)
		pool.markSeen(pending.hash)
	}
	clear(pool.txs[len(remaining):])
	pool.txs = remaining
	return removed
}

// RetainTxs removes every pending transaction keep rejects. keep runs outside
// the pool lock, so it may execute application checks.
func (pool *FIFO) RetainTxs(ctx context.Context, keep func(types.Tx) bool) (int, error) {
	if err := pool.checkContext(ctx); err != nil {
		return 0, err
	}
	if keep == nil {
		return 0, nil
	}
	pool.mu.Lock()
	snapshot := append([]*entry(nil), pool.txs...)
	pool.mu.Unlock()

	rejected := make(map[*entry]struct{})
	for _, pending := range snapshot {
		if err := pool.checkContext(ctx); err != nil {
			return 0, err
		}
		if !keep(append(types.Tx(nil), pending.tx...)) {
			rejected[pending] = struct{}{}
		}
	}
	if len(rejected) == 0 {
		return 0, nil
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.filterLocked(func(pending *entry) bool {
		_, drop := rejected[pending]
		return !drop
	}), nil
}

func (pool *FIFO) orderedTxs() []types.Tx {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return copyTxs(pool.rankLocked(append([]*entry(nil), pool.txs...)))
}

// rankLocked applies the configured proposal order to entries, in place.
func (pool *FIFO) rankLocked(entries []*entry) []*entry {
	ordered := entries
	if !pool.config.EnablePriority {
		return ordered
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].priority != ordered[right].priority {
			return ordered[left].priority > ordered[right].priority
		}
		if ordered[left].fee != ordered[right].fee {
			return ordered[left].fee > ordered[right].fee
		}
		return bytes.Compare(ordered[left].hash[:], ordered[right].hash[:]) < 0
	})
	return ordered
}

// eachLiveEntry walks the pending transactions not in proposed in proposal order and yields the
// ones the application check still accepts, until yield returns false. A transaction is checked
// only when the walk reaches it, so a block costs the checks of what it carries, not of the whole
// pool. The checks run on a snapshot outside the pool lock, as a slow application check must
// never block admission; the transactions they reject are dropped.
func (pool *FIFO) eachLiveEntry(ctx context.Context, proposed map[types.Hash]struct{}, yield func(*entry) bool) error {
	pool.mu.Lock()
	snapshot := make([]*entry, 0, len(pool.txs))
	for _, pending := range pool.txs {
		if _, found := proposed[pending.hash]; !found {
			snapshot = append(snapshot, pending)
		}
	}
	ranked := pool.rankLocked(snapshot)
	replay := pool.config.ReplayCheckTx
	pool.mu.Unlock()

	var dead []*entry
	defer func() {
		if len(dead) > 0 {
			pool.dropEntries(dead)
		}
	}()
	for _, pending := range ranked {
		if err := pool.checkContext(ctx); err != nil {
			return err
		}
		if replay != nil {
			if err := replay(ctx, append(types.Tx(nil), pending.tx...)); err != nil && !errors.Is(err, ErrTxNotReady) {
				if ctxErr := pool.checkContext(ctx); ctxErr != nil {
					return ctxErr
				}
				dead = append(dead, pending)
				continue
			}
		}
		if !yield(pending) {
			return nil
		}
	}
	return nil
}

func (pool *FIFO) dropEntries(dead []*entry) {
	doomed := make(map[*entry]struct{}, len(dead))
	for _, pending := range dead {
		doomed[pending] = struct{}{}
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.filterLocked(func(pending *entry) bool {
		_, drop := doomed[pending]
		return !drop
	})
}

func (pool *FIFO) seenRecently(hash types.Hash) bool {
	if pool.config.SeenTTL <= 0 {
		return false
	}
	seenAt, found := pool.seen[hash]
	if !found {
		return false
	}
	if pool.now().Sub(seenAt) > pool.config.SeenTTL {
		delete(pool.seen, hash)
		return false
	}
	return true
}

func (pool *FIFO) pruneSeenExpired() {
	if pool.config.SeenTTL <= 0 || len(pool.seen) == 0 {
		return
	}
	now := pool.now()
	for hash, seenAt := range pool.seen {
		if now.Sub(seenAt) > pool.config.SeenTTL {
			delete(pool.seen, hash)
		}
	}
}

func (pool *FIFO) markSeen(hash types.Hash) {
	if pool.config.SeenTTL <= 0 {
		return
	}
	pool.seen[hash] = pool.now()
}

func (pool *FIFO) Len() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return len(pool.txs)
}

// rebuildIndex recomputes every derived lookup from the queue. It reads the
// cached hashes and sizes only; it never hashes or parses a payload.
func (pool *FIFO) rebuildIndex() {
	pool.index = make(map[types.Hash]*entry, len(pool.txs))
	pool.replacements = make(map[string]*entry, len(pool.txs))
	pool.senders = make(map[string]int)
	pool.evictions = make(evictionHeap, 0, len(pool.txs))
	pool.totalBytes = 0
	for _, pending := range pool.txs {
		pending.evictAt = len(pool.evictions)
		pool.evictions = append(pool.evictions, pending)
		pool.index[pending.hash] = pending
		if pending.key != "" {
			pool.replacements[pending.key] = pending
		}
		if pending.sender != "" {
			pool.senders[pending.sender]++
		}
		pool.totalBytes += pending.size
	}
	heap.Init(&pool.evictions)
}

func (pool *FIFO) replacementTarget(candidate *entry) *entry {
	if !pool.config.EnableReplacement || candidate.key == "" {
		return nil
	}
	return pool.replacements[candidate.key]
}

func (pool *FIFO) checkReplacement(old *entry, candidate *entry) error {
	if old.hash == candidate.hash {
		return ErrDuplicateTx
	}
	if !replacementPriceBumped(old.fee, old.priority, candidate.fee, candidate.priority, pool.config.ReplacementBumpBPS) {
		return ErrReplacementUnderpriced
	}
	return nil
}

func (pool *FIFO) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func copyTxs(entries []*entry) []types.Tx {
	txs := make([]types.Tx, 0, len(entries))
	for _, pending := range entries {
		txs = append(txs, append(types.Tx(nil), pending.tx...))
	}
	return txs
}

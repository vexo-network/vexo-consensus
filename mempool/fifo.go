package mempool

import (
	"bytes"
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
	totalBytes   int64
	sequence     uint64
	seen         map[types.Hash]time.Time
	now          func() time.Time
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
	ranked := append([]*entry(nil), pool.txs...)
	sort.SliceStable(ranked, func(left int, right int) bool {
		return evictsBefore(ranked[left], ranked[right])
	})
	var victims []*entry
	var freed int64
	for _, victim := range ranked {
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
	var lowest *entry
	for _, candidate := range pool.txs {
		if lowest == nil || evictsBefore(candidate, lowest) {
			lowest = candidate
		}
	}
	return lowest
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
}

func (pool *FIFO) positionOf(target *entry) int {
	for position, candidate := range pool.txs {
		if candidate == target {
			return position
		}
	}
	return -1
}

func (pool *FIFO) BuildBatch(ctx context.Context, maxBytes int64) (Batch, error) {
	if err := pool.checkContext(ctx); err != nil {
		return Batch{}, err
	}
	if maxBytes <= 0 {
		return Batch{}, ErrInvalidMaxBytes
	}
	live, err := pool.liveEntries(ctx)
	if err != nil {
		return Batch{}, err
	}

	var totalBytes int64
	batch := Batch{
		Author: pool.config.Author,
		Txs:    make([]types.Tx, 0, len(live)),
	}
	for _, pending := range live {
		txSize := pending.size
		if len(batch.Txs) > 0 && totalBytes+txSize > maxBytes {
			break
		}
		if len(batch.Txs) == 0 && txSize > maxBytes {
			break
		}
		totalBytes += txSize
		batch.Txs = append(batch.Txs, append(types.Tx(nil), pending.tx...))
	}

	batch.ID = HashBatch(batch)
	return batch, nil
}

func (pool *FIFO) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	if err := pool.checkContext(ctx); err != nil {
		return nil, err
	}
	live, err := pool.liveEntries(ctx)
	if err != nil {
		return nil, err
	}
	return copyTxs(live), nil
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

// filterLocked keeps the entries for which keep returns true, marks the rest as
// recently seen and rebuilds the derived lookups.
func (pool *FIFO) filterLocked(keep func(*entry) bool) int {
	remaining := make([]*entry, 0, len(pool.txs))
	removed := 0
	for _, pending := range pool.txs {
		if keep(pending) {
			remaining = append(remaining, pending)
			continue
		}
		removed++
		pool.markSeen(pending.hash)
	}
	if removed == 0 {
		return 0
	}
	pool.txs = remaining
	pool.rebuildIndex()
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
	return copyTxs(pool.rankLocked(pool.txs))
}

// rankLocked applies the configured proposal order to a copy of entries.
func (pool *FIFO) rankLocked(entries []*entry) []*entry {
	ordered := append([]*entry(nil), entries...)
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

// liveEntries replays the application check on every pending transaction and
// returns the survivors in proposal order. The replay runs on a snapshot
// outside the pool lock: a slow application check must never block admission.
func (pool *FIFO) liveEntries(ctx context.Context) ([]*entry, error) {
	pool.mu.Lock()
	snapshot := append([]*entry(nil), pool.txs...)
	replay := pool.config.ReplayCheckTx
	pool.mu.Unlock()

	live := snapshot
	if replay != nil {
		live = make([]*entry, 0, len(snapshot))
		var dead []*entry
		for _, pending := range snapshot {
			if err := pool.checkContext(ctx); err != nil {
				return nil, err
			}
			if err := replay(ctx, append(types.Tx(nil), pending.tx...)); err != nil && !errors.Is(err, ErrTxNotReady) {
				if ctxErr := pool.checkContext(ctx); ctxErr != nil {
					return nil, ctxErr
				}
				dead = append(dead, pending)
				continue
			}
			live = append(live, pending)
		}
		if len(dead) > 0 {
			pool.dropEntries(dead)
		}
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.rankLocked(live), nil
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
	pool.totalBytes = 0
	for _, pending := range pool.txs {
		pool.index[pending.hash] = pending
		if pending.key != "" {
			pool.replacements[pending.key] = pending
		}
		if pending.sender != "" {
			pool.senders[pending.sender]++
		}
		pool.totalBytes += pending.size
	}
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

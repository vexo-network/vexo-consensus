package mempool

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"time"

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
)

type FIFOConfig struct {
	Author             types.ValidatorID
	MaxTxBytes         int64
	MaxTxs             int
	AllowDuplicate     bool
	SeenTTL            time.Duration
	MinFee             uint64
	EnablePriority     bool
	WALPath            string
	EnableReplacement  bool
	ReplacementBumpBPS uint64
}

type FIFO struct {
	config       FIFOConfig
	txs          []types.Tx
	index        map[types.Hash]int
	replacements map[string]int
	seen         map[types.Hash]time.Time
	now          func() time.Time
}

func NewFIFO(config FIFOConfig) *FIFO {
	return &FIFO{
		config:       config,
		index:        make(map[types.Hash]int),
		replacements: make(map[string]int),
		seen:         make(map[types.Hash]time.Time),
		now:          time.Now,
	}
}

func (pool *FIFO) CheckTx(ctx context.Context, tx types.Tx) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if len(tx) == 0 {
		return ErrEmptyTx
	}
	if pool.config.MaxTxBytes > 0 && int64(len(tx)) > pool.config.MaxTxBytes {
		return ErrTxTooLarge
	}
	if pool.replacementIndex(tx) >= 0 {
		return pool.checkReplacement(tx)
	}
	if pool.config.MaxTxs > 0 && len(pool.txs) >= pool.config.MaxTxs {
		return ErrMempoolFull
	}
	hash := HashTx(tx)
	pool.pruneSeenExpired()
	if !pool.config.AllowDuplicate {
		if _, found := pool.index[hash]; found {
			return ErrDuplicateTx
		}
		if pool.seenRecently(hash) {
			return ErrDuplicateTx
		}
	}
	if pool.config.MinFee > 0 && TxFee(tx) < pool.config.MinFee {
		return ErrInsufficientFee
	}
	return nil
}

func (pool *FIFO) AddTx(ctx context.Context, tx types.Tx) error {
	if err := pool.CheckTx(ctx, tx); err != nil {
		return err
	}
	if replaced := pool.replaceTxUnchecked(tx); replaced {
		return nil
	}

	return pool.addTxUnchecked(tx)
}

func (pool *FIFO) addTxUnchecked(tx types.Tx) error {
	copied := append(types.Tx(nil), tx...)
	hash := HashTx(copied)
	pool.index[hash] = len(pool.txs)
	if key, ok := ReplacementKey(copied); ok {
		pool.replacements[key] = len(pool.txs)
	}
	pool.markSeen(hash)
	pool.txs = append(pool.txs, copied)
	return nil
}

func (pool *FIFO) replaceTxUnchecked(tx types.Tx) bool {
	index := pool.replacementIndex(tx)
	if index < 0 {
		return false
	}
	old := pool.txs[index]
	delete(pool.index, HashTx(old))
	if oldKey, ok := ReplacementKey(old); ok {
		delete(pool.replacements, oldKey)
	}
	copied := append(types.Tx(nil), tx...)
	pool.txs[index] = copied
	pool.index[HashTx(copied)] = index
	if key, ok := ReplacementKey(copied); ok {
		pool.replacements[key] = index
	}
	pool.markSeen(HashTx(old))
	pool.markSeen(HashTx(copied))
	return true
}

func (pool *FIFO) BuildBatch(ctx context.Context, maxBytes int64) (Batch, error) {
	select {
	case <-ctx.Done():
		return Batch{}, ctx.Err()
	default:
	}
	if maxBytes <= 0 {
		return Batch{}, ErrInvalidMaxBytes
	}

	var totalBytes int64
	batch := Batch{
		Author: pool.config.Author,
		Txs:    make([]types.Tx, 0, len(pool.txs)),
	}

	txs := pool.orderedTxs()
	for _, tx := range txs {
		txSize := int64(len(tx))
		if len(batch.Txs) > 0 && totalBytes+txSize > maxBytes {
			break
		}
		if len(batch.Txs) == 0 && txSize > maxBytes {
			break
		}
		totalBytes += txSize
		batch.Txs = append(batch.Txs, append(types.Tx(nil), tx...))
	}

	batch.ID = HashBatch(batch)
	return batch, nil
}

func (pool *FIFO) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return pool.orderedTxs(), nil
}

func (pool *FIFO) MarkCommitted(ctx context.Context, committed []types.Tx) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(committed) == 0 {
		return nil
	}
	pool.pruneSeenExpired()

	remaining := make([]types.Tx, 0, len(pool.txs))
	for _, tx := range pool.txs {
		if containsTx(committed, tx) {
			hash := HashTx(tx)
			delete(pool.index, hash)
			if key, ok := ReplacementKey(tx); ok {
				delete(pool.replacements, key)
			}
			pool.markSeen(hash)
			continue
		}
		remaining = append(remaining, tx)
	}

	pool.txs = remaining
	pool.rebuildIndex()
	return nil
}

func (pool *FIFO) orderedTxs() []types.Tx {
	ordered := make([]types.Tx, 0, len(pool.txs))
	for _, tx := range pool.txs {
		ordered = append(ordered, append(types.Tx(nil), tx...))
	}
	if !pool.config.EnablePriority {
		return ordered
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		leftPriority := TxPriority(ordered[left])
		rightPriority := TxPriority(ordered[right])
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		leftFee := TxFee(ordered[left])
		rightFee := TxFee(ordered[right])
		if leftFee != rightFee {
			return leftFee > rightFee
		}
		leftHash := HashTx(ordered[left])
		rightHash := HashTx(ordered[right])
		return bytes.Compare(leftHash[:], rightHash[:]) < 0
	})
	return ordered
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
	return len(pool.txs)
}

func (pool *FIFO) rebuildIndex() {
	pool.index = make(map[types.Hash]int, len(pool.txs))
	pool.replacements = make(map[string]int, len(pool.txs))
	for txIndex, tx := range pool.txs {
		pool.index[HashTx(tx)] = txIndex
		if key, ok := ReplacementKey(tx); ok {
			pool.replacements[key] = txIndex
		}
	}
}

func (pool *FIFO) replacementIndex(tx types.Tx) int {
	if !pool.config.EnableReplacement {
		return -1
	}
	key, ok := ReplacementKey(tx)
	if !ok {
		return -1
	}
	index, found := pool.replacements[key]
	if !found {
		return -1
	}
	return index
}

func (pool *FIFO) checkReplacement(tx types.Tx) error {
	index := pool.replacementIndex(tx)
	if index < 0 {
		return nil
	}
	old := pool.txs[index]
	if HashTx(old) == HashTx(tx) {
		return ErrDuplicateTx
	}
	if !replacementPriceBumped(old, tx, pool.config.ReplacementBumpBPS) {
		return ErrReplacementUnderpriced
	}
	return nil
}

func containsTx(txs []types.Tx, target types.Tx) bool {
	for _, tx := range txs {
		if bytes.Equal(tx, target) {
			return true
		}
	}
	return false
}

package node

import (
	"sync"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/types"
)

// Compact proposal reconstruction.
//
// A compact proposal names its transactions by short id, and a validator rebuilds the block from
// the transactions it holds. A validator whose pool is full turns gossiped transactions away, and
// the leader then proposes transactions only its own pool held: the validator keeps the
// transactions it turned away for lack of room in a bounded overflow, which it rebuilds compact
// proposals from as well. An overflow transaction is not admitted: the block's transaction root
// and its execution decide what it does.

const (
	// overflowMaxTxs and overflowMaxBytes bound the overflow; the oldest transactions leave first.
	overflowMaxTxs         = 1 << 17
	overflowMaxBytes int64 = 64 << 20
)

// overflowTxs holds recent gossiped transactions the pool had no room for, by short id.
type overflowTxs struct {
	mu    sync.Mutex
	byID  map[uint64]overflowTx
	order []uint64
	head  int
	bytes int64
}

// overflowTx is the transaction of one short id; colliding transactions make the id ambiguous.
type overflowTx struct {
	tx        types.Tx
	refs      int
	ambiguous bool
}

func newOverflowTxs() *overflowTxs {
	return &overflowTxs{byID: make(map[uint64]overflowTx)}
}

// add keeps tx, evicting the oldest transactions past the bounds.
func (overflow *overflowTxs) add(tx types.Tx) {
	if overflow == nil || int64(len(tx)) > overflowMaxBytes {
		return
	}
	id := mempool.ShortTxID(mempool.HashTx(tx))
	overflow.mu.Lock()
	defer overflow.mu.Unlock()
	held, found := overflow.byID[id]
	switch {
	case !found:
		held = overflowTx{tx: append(types.Tx(nil), tx...)}
		overflow.bytes += int64(len(tx))
	case string(held.tx) == string(tx):
		return
	default:
		held.ambiguous = true
	}
	held.refs++
	overflow.byID[id] = held
	overflow.order = append(overflow.order, id)
	for len(overflow.order)-overflow.head > overflowMaxTxs || overflow.bytes > overflowMaxBytes {
		overflow.evictOldestLocked()
	}
	if overflow.head > len(overflow.order)/2 {
		overflow.order = append([]uint64(nil), overflow.order[overflow.head:]...)
		overflow.head = 0
	}
}

func (overflow *overflowTxs) evictOldestLocked() {
	id := overflow.order[overflow.head]
	overflow.head++
	held := overflow.byID[id]
	held.refs--
	if held.refs > 0 {
		overflow.byID[id] = held
		return
	}
	overflow.bytes -= int64(len(held.tx))
	delete(overflow.byID, id)
}

// TxByShortID returns the overflow transaction of id, false when it holds none or colliding ones.
func (overflow *overflowTxs) TxByShortID(id uint64) (types.Tx, bool) {
	if overflow == nil {
		return nil, false
	}
	overflow.mu.Lock()
	defer overflow.mu.Unlock()
	held, found := overflow.byID[id]
	if !found || held.ambiguous {
		return nil, false
	}
	return held.tx, true
}

// proposalTxSource finds the transactions of compact proposals in the pool, then in the overflow.
type proposalTxSource struct {
	pool     consensus.TxSource
	overflow *overflowTxs
}

func (source proposalTxSource) TxByShortID(id uint64) (types.Tx, bool) {
	if tx, found := source.pool.TxByShortID(id); found {
		return tx, true
	}
	return source.overflow.TxByShortID(id)
}

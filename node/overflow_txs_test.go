package node

import (
	"fmt"
	"testing"

	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/types"
)

type emptyTxSource struct{}

func (emptyTxSource) TxByShortID(uint64) (types.Tx, bool) { return nil, false }

// A gossiped transaction a full pool turned away still rebuilds the compact proposals that carry
// it; the overflow stays bounded, and an id two transactions share names neither.
func TestOverflowTxsRebuildWhatAFullPoolTurnedAway(t *testing.T) {
	overflow := newOverflowTxs()
	source := proposalTxSource{pool: emptyTxSource{}, overflow: overflow}
	tx := types.Tx("bank:turned-away")
	id := mempool.ShortTxID(mempool.HashTx(tx))
	if _, found := source.TxByShortID(id); found {
		t.Fatal("setup: expected nothing before the transaction arrives")
	}
	overflow.add(tx)
	overflow.add(tx)
	if got, found := source.TxByShortID(id); !found || string(got) != string(tx) {
		t.Fatalf("expected the turned-away transaction, got %q %v", got, found)
	}

	// No two transactions are known to share a short id: mark the id as a collision would.
	held := overflow.byID[id]
	held.ambiguous = true
	overflow.byID[id] = held
	if _, found := source.TxByShortID(id); found {
		t.Fatal("expected an ambiguous id to name no transaction")
	}

	for index := 0; index < overflowMaxTxs+10; index++ {
		overflow.add(types.Tx(fmt.Sprintf("bank:fill-%d", index)))
	}
	if held := len(overflow.order) - overflow.head; held != overflowMaxTxs || len(overflow.byID) != overflowMaxTxs {
		t.Fatalf("expected the overflow bounded at %d, got %d ids and %d entries", overflowMaxTxs, len(overflow.byID), held)
	}
	if _, found := source.TxByShortID(mempool.ShortTxID(mempool.HashTx(types.Tx("bank:fill-0")))); found {
		t.Fatal("expected the oldest transactions to leave first")
	}
	if _, found := source.TxByShortID(mempool.ShortTxID(mempool.HashTx(types.Tx(fmt.Sprintf("bank:fill-%d", overflowMaxTxs+9))))); !found {
		t.Fatal("expected the newest transaction kept")
	}
}

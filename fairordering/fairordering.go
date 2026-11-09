package fairordering

import (
	"bytes"
	"cmp"
	"container/heap"
	"crypto/sha256"
	"encoding/binary"
	"slices"

	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

func HashTx(tx types.Tx) types.Hash {
	return HashTxWithSalt(nil, tx)
}

func HashTxWithSalt(salt []byte, tx types.Tx) types.Hash {
	hasher := sha256.New()
	if len(salt) > 0 {
		hasher.Write(salt)
	}
	hasher.Write(tx)

	var hash types.Hash
	hasher.Sum(hash[:0])
	return hash
}

func HeightSalt(chainID string, height types.Height) []byte {
	var heightBuffer [8]byte
	binary.BigEndian.PutUint64(heightBuffer[:], uint64(height))

	hasher := sha256.New()
	hasher.Write([]byte(chainID))
	hasher.Write([]byte{0})
	hasher.Write(heightBuffer[:])
	return hasher.Sum(nil)
}

func SortTxs(txs []types.Tx) []types.Tx {
	return SortTxsWithSalt(txs, nil)
}

// SortTxsWithSalt returns copies of txs in canonical order: the transactions of one signer in
// nonce order, and the heads of all signer chains and unsequenced transactions interleaved by
// their salted hash.
func SortTxsWithSalt(txs []types.Tx, salt []byte) []types.Tx {
	ordered := make([]types.Tx, 0, len(txs))
	for _, index := range canonicalOrder(txs, salt) {
		ordered = append(ordered, append(types.Tx(nil), txs[index]...))
	}
	return ordered
}

func IsOrdered(txs []types.Tx) bool {
	return IsOrderedWithSalt(txs, nil)
}

func IsOrderedWithSalt(txs []types.Tx, salt []byte) bool {
	for position, index := range canonicalOrder(txs, salt) {
		if index != position && !bytes.Equal(txs[position], txs[index]) {
			return false
		}
	}
	return true
}

// item is one transaction with the keys the order compares, computed once.
type item struct {
	index int
	hash  types.Hash
	nonce uint64
}

type transactionChain struct {
	txs   []types.Tx
	items []item
	head  int
}

func (chain *transactionChain) current() *item { return &chain.items[chain.head] }

// canonicalOrder returns the positions of txs in canonical order. Each transaction is parsed and
// hashed exactly once.
func canonicalOrder(txs []types.Tx, salt []byte) []int {
	bySigner := make(map[string][]item)
	var unsequenced []item
	for index, tx := range txs {
		entry := item{index: index, hash: HashTxWithSalt(salt, tx)}
		meta, err := txcodec.ParseMeta(tx)
		if err != nil || meta.Signer == "" || !meta.HasNonce {
			unsequenced = append(unsequenced, entry)
			continue
		}
		entry.nonce = meta.Nonce
		signer := txcodec.NormalizeAddress(meta.Signer)
		bySigner[signer] = append(bySigner[signer], entry)
	}
	chains := make(transactionQueue, 0, len(bySigner)+1)
	for _, signerItems := range bySigner {
		slices.SortFunc(signerItems, func(left, right item) int {
			if left.nonce != right.nonce {
				return cmp.Compare(left.nonce, right.nonce)
			}
			return compareHeads(txs, &left, &right)
		})
		chains = append(chains, &transactionChain{items: signerItems, txs: txs})
	}
	if len(unsequenced) > 0 {
		// Unsequenced transactions are independent, so merging each as its own chain takes
		// them in key order: one chain of them sorted by key merges identically, and cheaper.
		slices.SortFunc(unsequenced, func(left, right item) int {
			return compareHeads(txs, &left, &right)
		})
		chains = append(chains, &transactionChain{items: unsequenced, txs: txs})
	}
	heap.Init(&chains)

	order := make([]int, 0, len(txs))
	for len(chains) > 0 {
		chain := chains[0]
		order = append(order, chain.current().index)
		chain.head++
		if chain.head < len(chain.items) {
			heap.Fix(&chains, 0)
			continue
		}
		heap.Pop(&chains)
	}
	return order
}

// transactionQueue is a min-heap of chains by the salted hash of their head, then its bytes.
type transactionQueue []*transactionChain

func (queue transactionQueue) Len() int { return len(queue) }

func (queue transactionQueue) Less(left, right int) bool {
	leftChain := queue[left]
	return compareHeads(leftChain.txs, leftChain.current(), queue[right].current()) < 0
}

// compareHeads orders two transactions of txs by their salted hash, then their bytes.
func compareHeads(txs []types.Tx, left *item, right *item) int {
	if order := bytes.Compare(left.hash[:], right.hash[:]); order != 0 {
		return order
	}
	return bytes.Compare(txs[left.index], txs[right.index])
}

func (queue transactionQueue) Swap(left, right int) {
	queue[left], queue[right] = queue[right], queue[left]
}

func (queue *transactionQueue) Push(value any) {
	*queue = append(*queue, value.(*transactionChain))
}

func (queue *transactionQueue) Pop() any {
	previous := *queue
	last := len(previous) - 1
	value := previous[last]
	previous[last] = nil
	*queue = previous[:last]
	return value
}

func cloneTxs(txs []types.Tx) []types.Tx {
	cloned := make([]types.Tx, 0, len(txs))
	for _, tx := range txs {
		cloned = append(cloned, append(types.Tx(nil), tx...))
	}
	return cloned
}

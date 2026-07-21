package fairordering

import (
	"bytes"
	"container/heap"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/vexo-network/vexo-consensus/mempool"
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
	copy(hash[:], hasher.Sum(nil))
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

func SortTxsWithSalt(txs []types.Tx, salt []byte) []types.Tx {
	chains := transactionChains(txs, salt)
	queue := make(transactionQueue, 0, len(chains))
	for _, chain := range chains {
		queue = append(queue, newTransactionCandidate(chain, salt))
	}
	heap.Init(&queue)

	ordered := make([]types.Tx, 0, len(txs))
	for queue.Len() > 0 {
		candidate := heap.Pop(&queue).(transactionCandidate)
		ordered = append(ordered, append(types.Tx(nil), candidate.tx...))
		candidate.chain.index++
		if candidate.chain.index < len(candidate.chain.txs) {
			heap.Push(&queue, newTransactionCandidate(candidate.chain, salt))
		}
	}
	return ordered
}

func IsOrdered(txs []types.Tx) bool {
	return IsOrderedWithSalt(txs, nil)
}

func IsOrderedWithSalt(txs []types.Tx, salt []byte) bool {
	ordered := SortTxsWithSalt(txs, salt)
	for index := range txs {
		if !bytes.Equal(txs[index], ordered[index]) {
			return false
		}
	}
	return true
}

type transactionChain struct {
	txs   []types.Tx
	index int
}

type transactionCandidate struct {
	chain *transactionChain
	tx    types.Tx
	hash  types.Hash
}

type transactionQueue []transactionCandidate

func (queue transactionQueue) Len() int { return len(queue) }

func (queue transactionQueue) Less(left, right int) bool {
	if queue[left].hash != queue[right].hash {
		return bytes.Compare(queue[left].hash[:], queue[right].hash[:]) < 0
	}
	return bytes.Compare(queue[left].tx, queue[right].tx) < 0
}

func (queue transactionQueue) Swap(left, right int) {
	queue[left], queue[right] = queue[right], queue[left]
}

func (queue *transactionQueue) Push(value any) {
	*queue = append(*queue, value.(transactionCandidate))
}

func (queue *transactionQueue) Pop() any {
	previous := *queue
	last := len(previous) - 1
	value := previous[last]
	previous[last] = transactionCandidate{}
	*queue = previous[:last]
	return value
}

func transactionChains(txs []types.Tx, salt []byte) []*transactionChain {
	bySigner := make(map[string][]types.Tx)
	chains := make([]*transactionChain, 0, len(txs))
	for _, original := range txs {
		tx := append(types.Tx(nil), original...)
		signer, signerFound := mempool.TxSigner(tx)
		_, nonceFound := mempool.TxNonce(tx)
		if !signerFound || !nonceFound {
			chains = append(chains, &transactionChain{txs: []types.Tx{tx}})
			continue
		}
		bySigner[signer] = append(bySigner[signer], tx)
	}

	signers := make([]string, 0, len(bySigner))
	for signer := range bySigner {
		signers = append(signers, string(signer))
	}
	sort.Strings(signers)
	for _, signerText := range signers {
		signerTxs := bySigner[signerText]
		sort.Slice(signerTxs, func(left, right int) bool {
			leftNonce, _ := mempool.TxNonce(signerTxs[left])
			rightNonce, _ := mempool.TxNonce(signerTxs[right])
			if leftNonce != rightNonce {
				return leftNonce < rightNonce
			}
			leftHash := HashTxWithSalt(salt, signerTxs[left])
			rightHash := HashTxWithSalt(salt, signerTxs[right])
			if leftHash != rightHash {
				return bytes.Compare(leftHash[:], rightHash[:]) < 0
			}
			return bytes.Compare(signerTxs[left], signerTxs[right]) < 0
		})
		chains = append(chains, &transactionChain{txs: signerTxs})
	}
	return chains
}

func newTransactionCandidate(chain *transactionChain, salt []byte) transactionCandidate {
	tx := chain.txs[chain.index]
	return transactionCandidate{chain: chain, tx: tx, hash: HashTxWithSalt(salt, tx)}
}

func cloneTxs(txs []types.Tx) []types.Tx {
	cloned := make([]types.Tx, 0, len(txs))
	for _, tx := range txs {
		cloned = append(cloned, append(types.Tx(nil), tx...))
	}
	return cloned
}

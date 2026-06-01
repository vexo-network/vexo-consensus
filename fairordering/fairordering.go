package fairordering

import (
	"bytes"
	"crypto/sha256"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

func HashTx(tx types.Tx) types.Hash {
	hasher := sha256.New()
	hasher.Write(tx)

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

func SortTxs(txs []types.Tx) []types.Tx {
	ordered := cloneTxs(txs)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftHash := HashTx(ordered[left])
		rightHash := HashTx(ordered[right])
		if leftHash != rightHash {
			return bytes.Compare(leftHash[:], rightHash[:]) < 0
		}
		return bytes.Compare(ordered[left], ordered[right]) < 0
	})
	return ordered
}

func IsOrdered(txs []types.Tx) bool {
	for index := 1; index < len(txs); index++ {
		previousHash := HashTx(txs[index-1])
		currentHash := HashTx(txs[index])
		if previousHash == currentHash {
			if bytes.Compare(txs[index-1], txs[index]) > 0 {
				return false
			}
			continue
		}
		if bytes.Compare(previousHash[:], currentHash[:]) > 0 {
			return false
		}
	}
	return true
}

func cloneTxs(txs []types.Tx) []types.Tx {
	cloned := make([]types.Tx, 0, len(txs))
	for _, tx := range txs {
		cloned = append(cloned, append(types.Tx(nil), tx...))
	}
	return cloned
}

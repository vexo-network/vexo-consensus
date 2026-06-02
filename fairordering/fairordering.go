package fairordering

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"

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
	ordered := cloneTxs(txs)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftHash := HashTxWithSalt(salt, ordered[left])
		rightHash := HashTxWithSalt(salt, ordered[right])
		if leftHash != rightHash {
			return bytes.Compare(leftHash[:], rightHash[:]) < 0
		}
		return bytes.Compare(ordered[left], ordered[right]) < 0
	})
	return ordered
}

func IsOrdered(txs []types.Tx) bool {
	return IsOrderedWithSalt(txs, nil)
}

func IsOrderedWithSalt(txs []types.Tx, salt []byte) bool {
	for index := 1; index < len(txs); index++ {
		previousHash := HashTxWithSalt(salt, txs[index-1])
		currentHash := HashTxWithSalt(salt, txs[index])
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

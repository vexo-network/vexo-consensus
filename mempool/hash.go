package mempool

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/types"
)

func HashTx(tx types.Tx) types.Hash {
	hasher := sha256.New()
	hasher.Write(tx)

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

// ShortTxID is the short id of the transaction with hash: the first eight bytes of the hash. A
// compact proposal names its transactions by short id, and a receiver finds them in its pool
// (FIFO.TxByShortID). Short ids may collide, by chance or by a sender grinding them; a receiver
// then misses the transaction and fetches the whole proposal, and the block hash still decides
// which transactions the block holds.
func ShortTxID(hash types.Hash) uint64 {
	return binary.BigEndian.Uint64(hash[:8])
}

func HashBatch(batch Batch) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(batch.Author))
	for _, parent := range batch.Parents {
		hasher.Write(parent[:])
	}
	for _, tx := range batch.Txs {
		writeUint64(hasher, uint64(len(tx)))
		hasher.Write(tx)
	}
	hasher.Write(batch.Metadata)

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

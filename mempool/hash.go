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

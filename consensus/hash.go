package consensus

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/types"
)

func HashBlock(block types.Block) types.Hash {
	header := block.Header
	if header.TxRoot == (types.Hash{}) {
		header.TxRoot = TxRoot(block.Txs)
	}
	return HashHeader(header)
}

func HashHeader(header types.Header) types.Hash {
	hasher := sha256.New()

	hasher.Write([]byte(header.ChainID))
	writeUint64(hasher, uint64(header.Height))
	writeUint64(hasher, uint64(header.TimeUnixNano))
	hasher.Write(header.PreviousBlockHash[:])
	hasher.Write(header.AppHash[:])
	hasher.Write(header.ValidatorSetHash[:])
	hasher.Write(header.ConsensusHash[:])
	hasher.Write(header.TxRoot[:])

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

func TxRoot(txs []types.Tx) types.Hash {
	if len(txs) == 0 {
		return types.Hash{}
	}
	hasher := sha256.New()

	for _, tx := range txs {
		writeUint64(hasher, uint64(len(tx)))
		hasher.Write(tx)
	}

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

package consensus

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/types"
)

func HashBlock(block types.Block) types.Hash {
	hasher := sha256.New()

	writeUint64(hasher, uint64(block.Header.Height))
	writeUint64(hasher, uint64(block.Header.TimeUnixNano))
	hasher.Write(block.Header.PreviousBlockHash[:])
	hasher.Write(block.Header.AppHash[:])
	hasher.Write(block.Header.ValidatorSetHash[:])
	hasher.Write(block.Header.ConsensusHash[:])

	for _, tx := range block.Txs {
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

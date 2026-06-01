package dataavailability

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrCommitmentMismatch = errors.New("data availability commitment mismatch")
	ErrMissingData        = errors.New("data availability data is missing")
)

type Proof struct {
	Commitment types.Hash
	TxCount    uint64
	TotalBytes uint64
}

func BuildProof(txs []types.Tx) Proof {
	var totalBytes uint64
	for _, tx := range txs {
		totalBytes += uint64(len(tx))
	}
	return Proof{
		Commitment: Commitment(txs),
		TxCount:    uint64(len(txs)),
		TotalBytes: totalBytes,
	}
}

func Commitment(txs []types.Tx) types.Hash {
	hasher := sha256.New()
	writeUint64(hasher, uint64(len(txs)))
	for _, tx := range txs {
		writeUint64(hasher, uint64(len(tx)))
		hasher.Write(tx)
	}

	var commitment types.Hash
	copy(commitment[:], hasher.Sum(nil))
	return commitment
}

func Verify(header types.Header, txs []types.Tx) error {
	if header.ConsensusHash == (types.Hash{}) {
		if len(txs) == 0 {
			return nil
		}
		return ErrMissingData
	}
	if Commitment(txs) != header.ConsensusHash {
		return ErrCommitmentMismatch
	}
	return nil
}

func AttachCommitment(block types.Block) types.Block {
	block.Header.ConsensusHash = Commitment(block.Txs)
	return block
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

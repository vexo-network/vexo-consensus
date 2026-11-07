package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidBlockBody = errors.New("invalid block body")

// HashBlock hashes the header with the transaction root derived from the block
// body, so the hash always commits to the transactions regardless of the
// TxRoot value a sender claims in the header.
func HashBlock(block types.Block) types.Hash {
	header := block.Header
	header.TxRoot = TxRoot(block.Txs)
	return HashHeader(header)
}

// VerifyBlockBody checks that the block body is consistent with its header:
// the header transaction root and the data availability commitment match the
// transactions, and the transactions are in the deterministic fair order.
func VerifyBlockBody(block types.Block) error {
	header := block.Header
	if header.TxRoot != TxRoot(block.Txs) {
		return fmt.Errorf("%w: transaction root mismatch", ErrInvalidBlockBody)
	}
	if err := dataavailability.Verify(header, block.Txs); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBlockBody, err)
	}
	if !fairordering.IsOrderedWithSalt(block.Txs, fairordering.HeightSalt(header.ChainID, header.Height)) {
		return fmt.Errorf("%w: transaction ordering mismatch", ErrInvalidBlockBody)
	}
	return nil
}

// HashHeader is the block hash a quorum certificate certifies. It is finality.HeaderHash, so a
// light client derives the same hash from a header.
func HashHeader(header types.Header) types.Hash {
	return finality.HeaderHash(header)
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

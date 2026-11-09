package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

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
//
// A block is checked at its proposal, at its commit and when it is observed committed. The
// transaction root binds the body to the header and is checked every time; the data availability
// and ordering checks are functions of the header and the transactions it binds, so a header
// hash that passed them once is not checked again.
func VerifyBlockBody(block types.Block) error {
	header := block.Header
	if header.TxRoot != TxRoot(block.Txs) {
		return fmt.Errorf("%w: transaction root mismatch", ErrInvalidBlockBody)
	}
	blockHash := HashHeader(header)
	if verifiedBodies.contains(blockHash) {
		return nil
	}
	if err := dataavailability.Verify(header, block.Txs); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBlockBody, err)
	}
	if !fairordering.IsOrderedWithSalt(block.Txs, fairordering.HeightSalt(header.ChainID, header.Height)) {
		return fmt.Errorf("%w: transaction ordering mismatch", ErrInvalidBlockBody)
	}
	verifiedBodies.add(blockHash)
	return nil
}

// verifiedBodies holds the hashes of the most recent blocks whose bodies passed VerifyBlockBody.
var verifiedBodies = newHashSet(1024)

// hashSet is a bounded set of hashes that forgets the oldest first.
type hashSet struct {
	mu      sync.Mutex
	members map[types.Hash]struct{}
	order   []types.Hash
	next    int
}

func newHashSet(capacity int) *hashSet {
	return &hashSet{members: make(map[types.Hash]struct{}, capacity), order: make([]types.Hash, capacity)}
}

func (set *hashSet) contains(hash types.Hash) bool {
	set.mu.Lock()
	defer set.mu.Unlock()
	_, found := set.members[hash]
	return found
}

func (set *hashSet) add(hash types.Hash) {
	set.mu.Lock()
	defer set.mu.Unlock()
	if _, found := set.members[hash]; found {
		return
	}
	if len(set.members) == len(set.order) {
		delete(set.members, set.order[set.next])
	}
	set.members[hash] = struct{}{}
	set.order[set.next] = hash
	set.next = (set.next + 1) % len(set.order)
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

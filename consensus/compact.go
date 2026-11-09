package consensus

import (
	"encoding/binary"
	"fmt"

	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/types"
)

// shortTxIDSize is the size of one short transaction id on the wire (mempool.ShortTxID).
const shortTxIDSize = 8

// CompactProposal is a proposal on the wire without its transactions: the leader names them by
// short id (mempool.ShortTxID), in block order, and every validator rebuilds the block from the
// transactions it already holds. Transactions reach every validator through transaction gossip,
// which every validator shares, so the leader uploads a few bytes per transaction instead of the
// transaction itself to each validator: its upload no longer bounds the throughput.
//
// The proposal is unchanged: its signature, block hash and transaction root are the ones of the
// full block, so a rebuilt block is accepted only when it is exactly the block the proposer
// signed. A validator that misses a transaction, or holds a colliding one, asks the peer for the
// full proposal (ProposalRequest) and judges that.
type CompactProposal struct {
	Proposal Proposal `json:"proposal"`
	TxIDs    []byte   `json:"tx_ids"`
}

// ProposalRequest asks a peer for the transactions at Missing of the block it announced compactly,
// or for the whole proposal when Missing is empty.
type ProposalRequest struct {
	BlockHash types.Hash `json:"block_hash"`
	Missing   []uint32   `json:"missing,omitempty"`
}

// ProposalTxs answers a ProposalRequest with the transactions at the requested positions.
type ProposalTxs struct {
	BlockHash types.Hash `json:"block_hash"`
	Indexes   []uint32   `json:"indexes"`
	Txs       []types.Tx `json:"txs"`
}

// TxSource finds pending transactions by short id. It reports false for an id it holds no
// transaction for, or more than one.
type TxSource interface {
	TxByShortID(id uint64) (types.Tx, bool)
}

// NewCompactProposal returns proposal with its transactions replaced by their short ids.
func NewCompactProposal(proposal Proposal) CompactProposal {
	ids := make([]byte, 0, len(proposal.Block.Txs)*shortTxIDSize)
	for _, tx := range proposal.Block.Txs {
		ids = binary.BigEndian.AppendUint64(ids, mempool.ShortTxID(mempool.HashTx(tx)))
	}
	proposal.Block.Txs = nil
	return CompactProposal{Proposal: proposal, TxIDs: ids}
}

// TxCount returns the number of transactions the compact proposal names, and an error when its
// ids cannot name a block: a compact proposal carries no transaction itself.
func (compact CompactProposal) TxCount() (int, error) {
	if len(compact.Proposal.Block.Txs) != 0 || len(compact.TxIDs)%shortTxIDSize != 0 {
		return 0, fmt.Errorf("%w: malformed compact proposal", ErrInvalidProposal)
	}
	return len(compact.TxIDs) / shortTxIDSize, nil
}

// ShortID returns the short id of the transaction at index.
func (compact CompactProposal) ShortID(index int) uint64 {
	return binary.BigEndian.Uint64(compact.TxIDs[index*shortTxIDSize:])
}

// Rebuild finds the transactions of the compact proposal in source. It returns them in block
// order, nil where source holds none, and the positions it missed.
func (compact CompactProposal) Rebuild(source TxSource) ([]types.Tx, []uint32, error) {
	count, err := compact.TxCount()
	if err != nil {
		return nil, nil, err
	}
	txs := make([]types.Tx, count)
	var missing []uint32
	for index := range txs {
		if source != nil {
			if tx, found := source.TxByShortID(compact.ShortID(index)); found {
				txs[index] = tx
				continue
			}
		}
		missing = append(missing, uint32(index))
	}
	return txs, missing, nil
}

// Complete returns the proposal with txs, false when they are not the transactions its header
// commits to: a transaction the pool held under a colliding short id.
func (compact CompactProposal) Complete(txs []types.Tx) (Proposal, bool) {
	if TxRoot(txs) != compact.Proposal.Block.Header.TxRoot {
		return Proposal{}, false
	}
	proposal := compact.Proposal
	proposal.Block.Txs = txs
	return proposal, true
}

// BlockHash returns the hash of the block the compact proposal names: the hash of its header,
// whose transaction root commits to the transactions it leaves out.
func (compact CompactProposal) BlockHash() types.Hash {
	return HashHeader(compact.Proposal.Block.Header)
}

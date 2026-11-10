package consensus

import (
	"context"
	"fmt"

	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/lanes"
	"github.com/vexo-network/vexo-consensus/types"
)

// LaneProposal is a proposal on the wire that names the lane batches its block is made of instead
// of carrying their transactions (see package lanes). Validators store the batches before a
// leader may name them, so the leader uploads a few bytes per batch whatever the size of the
// block, and every validator rebuilds the block from what it stores.
//
// The proposal is unchanged: its signature, block hash and transaction root are the ones of the
// full block. The block's transactions are the transactions of the named batches in order, less
// the Excluded positions of that sequence (transactions the leader's application check refused),
// with the Extra transactions no batch carries, in the deterministic fair order of the height. A
// rebuilt block is judged only when it is exactly the block the header commits to; otherwise, or
// when a batch cannot be fetched, the validator asks for the whole proposal.
type LaneProposal struct {
	Proposal Proposal  `json:"proposal"`
	Batches  []LaneRef `json:"batches"`
	// Excluded are increasing positions in the transactions of Batches, concatenated.
	Excluded []uint32   `json:"excluded,omitempty"`
	Extra    []types.Tx `json:"extra,omitempty"`
}

// LaneRef names a batch, with its author, which serves it.
type LaneRef struct {
	ID     types.Hash        `json:"id"`
	Author types.ValidatorID `json:"author"`
}

// ProposalRequest asks a peer for the whole proposal of a block it announced by lane batches.
type ProposalRequest struct {
	BlockHash types.Hash `json:"block_hash"`
}

// NewLaneProposal returns proposal naming batches: the transactions of the block that batches
// carry travel as batch ids, the others as Extra.
func NewLaneProposal(proposal Proposal, batches []lanes.Batch) LaneProposal {
	remaining := make(map[string]int, len(proposal.Block.Txs))
	for _, tx := range proposal.Block.Txs {
		remaining[string(tx)]++
	}
	compact := LaneProposal{Proposal: proposal, Batches: make([]LaneRef, 0, len(batches))}
	compact.Proposal.Block.Txs = nil
	position := uint32(0)
	for _, batch := range batches {
		compact.Batches = append(compact.Batches, LaneRef{ID: batch.ID(), Author: batch.Author})
		for _, tx := range batch.Txs {
			if remaining[string(tx)] > 0 {
				remaining[string(tx)]--
			} else {
				compact.Excluded = append(compact.Excluded, position)
			}
			position++
		}
	}
	for _, tx := range proposal.Block.Txs {
		if remaining[string(tx)] > 0 {
			remaining[string(tx)]--
			compact.Extra = append(compact.Extra, tx)
		}
	}
	return compact
}

// BlockHash returns the hash of the block the lane proposal names: the hash of its header, whose
// transaction root commits to the transactions the proposal leaves out.
func (compact LaneProposal) BlockHash() types.Hash {
	return HashHeader(compact.Proposal.Block.Header)
}

// SignBytes returns what the proposer signed: the proposal of the block the header commits to.
func (compact LaneProposal) SignBytes() []byte {
	return proposalSignBytes(compact.Proposal, compact.BlockHash())
}

// MaxLaneRefs bounds the batches a lane proposal names.
const MaxLaneRefs = 4096

// LaneAuthenticator judges the signature of a lane proposal before a validator fetches batches for
// it: nil when the scheduled leader signed it, an error wrapping ErrInvalidProposal when the
// signature is forged, and any other error when the validator cannot judge it yet.
type LaneAuthenticator func(context.Context, LaneProposal) error

// IDs returns the ids of the batches the lane proposal names.
func (compact LaneProposal) IDs() []types.Hash {
	ids := make([]types.Hash, 0, len(compact.Batches))
	for _, ref := range compact.Batches {
		ids = append(ids, ref.ID)
	}
	return ids
}

// Rebuild rebuilds the proposal from the batches get finds. It returns the ids get misses, and
// complete is false when the batches are found but make another block than the header commits
// to. A lane proposal that cannot name a block is invalid.
func (compact LaneProposal) Rebuild(get func(types.Hash) (lanes.Batch, bool)) (proposal Proposal, missing []types.Hash, complete bool, err error) {
	if len(compact.Proposal.Block.Txs) != 0 || len(compact.Batches) == 0 || len(compact.Batches) > MaxLaneRefs {
		return Proposal{}, nil, false, fmt.Errorf("%w: a lane proposal names its transactions by batch", ErrInvalidProposal)
	}
	batches := make([]lanes.Batch, 0, len(compact.Batches))
	for _, ref := range compact.Batches {
		batch, found := get(ref.ID)
		if !found {
			missing = append(missing, ref.ID)
			continue
		}
		batches = append(batches, batch)
	}
	if len(missing) > 0 {
		return Proposal{}, missing, false, nil
	}
	txs := make([]types.Tx, 0)
	position, next, size := uint32(0), 0, BlockTxBytes(types.Block{Txs: compact.Extra})
	for _, batch := range batches {
		for _, tx := range batch.Txs {
			if next < len(compact.Excluded) && compact.Excluded[next] == position {
				next++
			} else {
				txs = append(txs, tx)
				size += len(tx)
			}
			position++
		}
		if size > MaxBlockBytes {
			return Proposal{}, nil, false, fmt.Errorf("%w: the named batches carry more than %d transaction bytes", ErrInvalidProposal, MaxBlockBytes)
		}
	}
	if next != len(compact.Excluded) {
		return Proposal{}, nil, false, fmt.Errorf("%w: excluded positions are not increasing positions of the batches", ErrInvalidProposal)
	}
	txs = append(txs, compact.Extra...)
	proposal = compact.Proposal
	header := proposal.Block.Header
	proposal.Block.Txs = fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt(header.ChainID, header.Height))
	if TxRoot(proposal.Block.Txs) != header.TxRoot {
		return Proposal{}, nil, false, nil
	}
	return proposal, nil, true, nil
}

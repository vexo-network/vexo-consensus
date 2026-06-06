package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrBlockNotFound    = errors.New("block not found")
	ErrInvalidBlockCert = errors.New("invalid block certificate")
)

type BlockNode struct {
	Block      types.Block
	Hash       types.Hash
	ParentHash types.Hash
	JustifyQC  finality.QuorumCert
	QuorumCert finality.QuorumCert
}

type BlockTree struct {
	blocks map[types.Hash]BlockNode
	highQC finality.QuorumCert
}

func NewBlockTree() *BlockTree {
	return &BlockTree{
		blocks: make(map[types.Hash]BlockNode),
	}
}

func (tree *BlockTree) Insert(block types.Block, blockHash types.Hash, justifyQC finality.QuorumCert) BlockNode {
	node := BlockNode{
		Block:      block,
		Hash:       blockHash,
		ParentHash: block.Header.PreviousBlockHash,
		JustifyQC:  justifyQC,
	}
	if previous, found := tree.blocks[blockHash]; found {
		node.QuorumCert = previous.QuorumCert
	}
	tree.blocks[blockHash] = node
	tree.ObserveQuorumCert(justifyQC)
	return node
}

func (tree *BlockTree) Get(blockHash types.Hash) (BlockNode, bool) {
	node, found := tree.blocks[blockHash]
	return node, found
}

func (tree *BlockTree) SetQuorumCert(qc finality.QuorumCert) error {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return ErrInvalidBlockCert
	}
	node, found := tree.Get(qc.BlockHash)
	if !found {
		return ErrBlockNotFound
	}
	if node.Block.Header.Height != qc.Height {
		return ErrInvalidBlockCert
	}
	node.QuorumCert = qc
	tree.blocks[qc.BlockHash] = node
	tree.ObserveQuorumCert(qc)
	return nil
}

func (tree *BlockTree) ObserveQuorumCert(qc finality.QuorumCert) {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return
	}
	if isBetterQC(qc, tree.highQC) {
		tree.highQC = qc
	}
}

func (tree *BlockTree) HighQC() finality.QuorumCert {
	return tree.highQC
}

func (tree *BlockTree) Extends(descendant types.Hash, ancestor types.Hash) bool {
	if descendant == (types.Hash{}) || ancestor == (types.Hash{}) {
		return false
	}
	for current := descendant; current != (types.Hash{}); {
		if current == ancestor {
			return true
		}
		node, found := tree.Get(current)
		if !found {
			return false
		}
		current = node.ParentHash
	}
	return false
}

func (tree *BlockTree) CommitCandidate(blockHash types.Hash) (CommitCandidate, bool) {
	node, found := tree.Get(blockHash)
	if !found || node.JustifyQC.Height == 0 {
		return CommitCandidate{}, false
	}

	parent, found := tree.Get(node.JustifyQC.BlockHash)
	if !found || parent.JustifyQC.Height == 0 {
		return CommitCandidate{}, false
	}

	return CommitCandidate{
		BlockHeight:       node.Block.Header.Height,
		BlockHash:         node.Hash,
		ParentHeight:      parent.Block.Header.Height,
		ParentHash:        parent.Hash,
		GrandparentHeight: parent.JustifyQC.Height,
		GrandparentHash:   parent.JustifyQC.BlockHash,
		BlockQC:           node.JustifyQC,
		ParentQC:          parent.JustifyQC,
	}, true
}

func isBetterQC(candidate finality.QuorumCert, current finality.QuorumCert) bool {
	if candidate.Height != current.Height {
		return candidate.Height > current.Height
	}
	return candidate.Round > current.Round
}

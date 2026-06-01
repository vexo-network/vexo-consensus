package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrBlockNotFound = errors.New("block not found")

type BlockNode struct {
	Block      types.Block
	Hash       types.Hash
	ParentHash types.Hash
	JustifyQC  finality.QuorumCert
}

type BlockTree struct {
	blocks map[types.Hash]BlockNode
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
	tree.blocks[blockHash] = node
	return node
}

func (tree *BlockTree) Get(blockHash types.Hash) (BlockNode, bool) {
	node, found := tree.blocks[blockHash]
	return node, found
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
		BlockHash:       node.Hash,
		ParentHash:      parent.Hash,
		GrandparentHash: parent.JustifyQC.BlockHash,
		BlockQC:         node.JustifyQC,
		ParentQC:        parent.JustifyQC,
	}, true
}

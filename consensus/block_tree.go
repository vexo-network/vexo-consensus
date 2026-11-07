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

// BlockNode is a block together with the QC it carries for its parent (JustifyQC) and the
// QC formed for the block itself (QuorumCert, zero until one is observed).
type BlockNode struct {
	Block      types.Block
	Hash       types.Hash
	ParentHash types.Hash
	JustifyQC  finality.QuorumCert
	QuorumCert finality.QuorumCert
}

// BlockTree stores blocks and the quorum certificates observed for them. It does not
// verify anything: the state machine hands it only blocks and certificates that already
// passed verification.
type BlockTree struct {
	blocks map[types.Hash]BlockNode
	// certs holds verified QCs whose block is not in the tree yet; they attach on Insert.
	certs  map[types.Hash]finality.QuorumCert
	highQC finality.QuorumCert
}

func NewBlockTree() *BlockTree {
	return &BlockTree{
		blocks: make(map[types.Hash]BlockNode),
		certs:  make(map[types.Hash]finality.QuorumCert),
	}
}

func (tree *BlockTree) Insert(block types.Block, blockHash types.Hash, justifyQC finality.QuorumCert) BlockNode {
	node, found := tree.blocks[blockHash]
	if found {
		if node.JustifyQC.Height == 0 && justifyQC.Height > 0 {
			node.JustifyQC = justifyQC
			tree.blocks[blockHash] = node
		}
	} else {
		node = BlockNode{
			Block:      block,
			Hash:       blockHash,
			ParentHash: block.Header.PreviousBlockHash,
			JustifyQC:  justifyQC,
		}
		if cert, certified := tree.certs[blockHash]; certified && cert.Height == block.Header.Height {
			node.QuorumCert = cert
		}
		delete(tree.certs, blockHash)
		tree.blocks[blockHash] = node
	}
	tree.ObserveQuorumCert(justifyQC)
	return node
}

func (tree *BlockTree) Get(blockHash types.Hash) (BlockNode, bool) {
	node, found := tree.blocks[blockHash]
	return node, found
}

// Len reports the number of blocks held by the tree.
func (tree *BlockTree) Len() int {
	return len(tree.blocks)
}

// SetQuorumCert records qc as the certificate of its block. The first certificate
// recorded for a block is kept.
func (tree *BlockTree) SetQuorumCert(qc finality.QuorumCert) error {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return ErrInvalidBlockCert
	}
	node, found := tree.Get(qc.BlockHash)
	if !found {
		tree.ObserveQuorumCert(qc)
		return ErrBlockNotFound
	}
	if node.Block.Header.Height != qc.Height {
		return ErrInvalidBlockCert
	}
	tree.ObserveQuorumCert(qc)
	return nil
}

// ObserveQuorumCert tracks qc as a candidate highest QC and attaches it to its block, or
// holds it until the block arrives.
func (tree *BlockTree) ObserveQuorumCert(qc finality.QuorumCert) {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return
	}
	if isBetterQC(qc, tree.highQC) {
		tree.highQC = qc
	}
	node, found := tree.blocks[qc.BlockHash]
	if !found {
		if _, held := tree.certs[qc.BlockHash]; !held {
			tree.certs[qc.BlockHash] = qc
		}
		return
	}
	if node.QuorumCert.Height == 0 && node.Block.Header.Height == qc.Height {
		node.QuorumCert = qc
		tree.blocks[qc.BlockHash] = node
	}
}

func (tree *BlockTree) HighQC() finality.QuorumCert {
	return tree.highQC
}

func (tree *BlockTree) Extends(descendant types.Hash, ancestor types.Hash) bool {
	if descendant == (types.Hash{}) || ancestor == (types.Hash{}) {
		return false
	}
	current := descendant
	for steps := 0; steps <= len(tree.blocks) && current != (types.Hash{}); steps++ {
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

// CommitCandidate builds the commit pair (B, C) for child block C: B is the parent that C
// certifies through its justify QC, and QC(C) must already be known. Whether the pair
// finalizes B is decided by TwoChainCommitRule.
func (tree *BlockTree) CommitCandidate(childHash types.Hash) (CommitCandidate, bool) {
	child, found := tree.Get(childHash)
	if !found || child.QuorumCert.Height == 0 || child.JustifyQC.Height == 0 {
		return CommitCandidate{}, false
	}
	return CommitCandidate{
		BlockHeight:     child.JustifyQC.Height,
		BlockHash:       child.JustifyQC.BlockHash,
		BlockQC:         child.JustifyQC,
		ChildHeight:     child.Block.Header.Height,
		ChildHash:       child.Hash,
		ChildParentHash: child.ParentHash,
		ChildQC:         child.QuorumCert,
	}, true
}

// Prune drops every block and held certificate below height.
func (tree *BlockTree) Prune(height types.Height) {
	for blockHash, node := range tree.blocks {
		if node.Block.Header.Height < height {
			delete(tree.blocks, blockHash)
		}
	}
	for blockHash, qc := range tree.certs {
		if qc.Height < height {
			delete(tree.certs, blockHash)
		}
	}
}

// isBetterQC orders certificates by view first: views are global, so the QC formed in the
// later view is the more recent one even when it certifies a lower height on another fork.
func isBetterQC(candidate finality.QuorumCert, current finality.QuorumCert) bool {
	if candidate.Height == 0 {
		return false
	}
	if current.Height == 0 {
		return true
	}
	if candidate.Round != current.Round {
		return candidate.Round > current.Round
	}
	return candidate.Height > current.Height
}

package consensus

import (
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestBlockTreeBuildsCommitCandidate(t *testing.T) {
	tree := NewBlockTree()
	grandparent := types.Hash{1}
	parent := types.Hash{2}
	block := types.Hash{3}

	tree.Insert(types.Block{Header: types.Header{Height: 1}}, grandparent, finality.QuorumCert{})
	tree.Insert(
		types.Block{Header: types.Header{Height: 2, PreviousBlockHash: grandparent}},
		parent,
		finality.QuorumCert{Height: 1, BlockHash: grandparent},
	)
	tree.Insert(
		types.Block{Header: types.Header{Height: 3, PreviousBlockHash: parent}},
		block,
		finality.QuorumCert{Height: 2, BlockHash: parent},
	)

	candidate, found := tree.CommitCandidate(block)
	if !found {
		t.Fatal("expected commit candidate")
	}
	if candidate.BlockHash != block || candidate.ParentHash != parent || candidate.GrandparentHash != grandparent {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func TestBlockTreeWaitsForParentQC(t *testing.T) {
	tree := NewBlockTree()
	parent := types.Hash{2}
	block := types.Hash{3}

	tree.Insert(types.Block{Header: types.Header{Height: 2}}, parent, finality.QuorumCert{})
	tree.Insert(
		types.Block{Header: types.Header{Height: 3, PreviousBlockHash: parent}},
		block,
		finality.QuorumCert{Height: 2, BlockHash: parent},
	)

	if _, found := tree.CommitCandidate(block); found {
		t.Fatal("expected no candidate without parent justify qc")
	}
}

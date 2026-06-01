package consensus

import (
	"errors"
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

func TestBlockTreeStoresQuorumCert(t *testing.T) {
	tree := NewBlockTree()
	blockHash := types.Hash{1}
	tree.Insert(types.Block{Header: types.Header{Height: 1}}, blockHash, finality.QuorumCert{})

	qc := finality.QuorumCert{Height: 1, BlockHash: blockHash, VotingPower: 2}
	if err := tree.SetQuorumCert(qc); err != nil {
		t.Fatal(err)
	}

	node, found := tree.Get(blockHash)
	if !found {
		t.Fatal("expected block node")
	}
	if node.QuorumCert.BlockHash != blockHash || node.QuorumCert.VotingPower != 2 {
		t.Fatalf("unexpected quorum cert: %+v", node.QuorumCert)
	}
	if tree.HighQC().BlockHash != blockHash {
		t.Fatal("expected stored quorum cert to become high qc")
	}
}

func TestBlockTreeRejectsInvalidQuorumCert(t *testing.T) {
	tree := NewBlockTree()
	blockHash := types.Hash{1}
	tree.Insert(types.Block{Header: types.Header{Height: 2}}, blockHash, finality.QuorumCert{})

	cases := []struct {
		name     string
		qc       finality.QuorumCert
		expected error
	}{
		{
			name:     "missing height",
			qc:       finality.QuorumCert{BlockHash: blockHash},
			expected: ErrInvalidBlockCert,
		},
		{
			name:     "missing block hash",
			qc:       finality.QuorumCert{Height: 2},
			expected: ErrInvalidBlockCert,
		},
		{
			name:     "unknown block",
			qc:       finality.QuorumCert{Height: 2, BlockHash: types.Hash{9}},
			expected: ErrBlockNotFound,
		},
		{
			name:     "height mismatch",
			qc:       finality.QuorumCert{Height: 1, BlockHash: blockHash},
			expected: ErrInvalidBlockCert,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := tree.SetQuorumCert(testCase.qc)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}

func TestBlockTreeTracksHighestQuorumCert(t *testing.T) {
	tree := NewBlockTree()
	first := finality.QuorumCert{Height: 1, Round: 2, BlockHash: types.Hash{1}}
	second := finality.QuorumCert{Height: 2, Round: 0, BlockHash: types.Hash{2}}
	lowerRound := finality.QuorumCert{Height: 2, Round: 0, BlockHash: types.Hash{3}}
	higherRound := finality.QuorumCert{Height: 2, Round: 1, BlockHash: types.Hash{4}}

	tree.ObserveQuorumCert(first)
	tree.ObserveQuorumCert(second)
	tree.ObserveQuorumCert(lowerRound)
	if tree.HighQC().BlockHash != second.BlockHash {
		t.Fatalf("expected second qc to remain high qc, got %+v", tree.HighQC())
	}
	tree.ObserveQuorumCert(higherRound)
	if tree.HighQC().BlockHash != higherRound.BlockHash {
		t.Fatalf("expected higher round qc, got %+v", tree.HighQC())
	}
}

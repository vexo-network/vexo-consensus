package consensus

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestBlockTreeBuildsCommitCandidate(t *testing.T) {
	tree := NewBlockTree()
	parent := types.Hash{1}
	child := types.Hash{2}

	parentQC := finality.QuorumCert{Height: 1, Round: 0, BlockHash: parent}
	childQC := finality.QuorumCert{Height: 2, Round: 1, BlockHash: child}
	tree.Insert(types.Block{Header: types.Header{Height: 1}}, parent, finality.QuorumCert{})
	tree.Insert(
		types.Block{Header: types.Header{Height: 2, PreviousBlockHash: parent}},
		child,
		parentQC,
	)
	if err := tree.SetQuorumCert(childQC); err != nil {
		t.Fatal(err)
	}

	candidate, found := tree.CommitCandidate(child)
	if !found {
		t.Fatal("expected commit candidate")
	}
	if candidate.BlockHash != parent || candidate.BlockHeight != 1 || !sameQC(candidate.BlockQC, parentQC) {
		t.Fatalf("unexpected block side of the candidate: %+v", candidate)
	}
	if candidate.ChildHash != child || candidate.ChildHeight != 2 || candidate.ChildParentHash != parent || !sameQC(candidate.ChildQC, childQC) {
		t.Fatalf("unexpected child side of the candidate: %+v", candidate)
	}
}

func TestBlockTreeWaitsForCertificates(t *testing.T) {
	tree := NewBlockTree()
	parent := types.Hash{2}
	child := types.Hash{3}

	tree.Insert(types.Block{Header: types.Header{Height: 2}}, parent, finality.QuorumCert{})
	tree.Insert(
		types.Block{Header: types.Header{Height: 3, PreviousBlockHash: parent}},
		child,
		finality.QuorumCert{Height: 2, Round: 0, BlockHash: parent},
	)
	if _, found := tree.CommitCandidate(child); found {
		t.Fatal("expected no candidate before the child has its own qc")
	}

	if err := tree.SetQuorumCert(finality.QuorumCert{Height: 3, Round: 1, BlockHash: child}); err != nil {
		t.Fatal(err)
	}
	if _, found := tree.CommitCandidate(parent); found {
		t.Fatal("expected no candidate for a block that carries no justify qc")
	}
	if _, found := tree.CommitCandidate(types.Hash{9}); found {
		t.Fatal("expected no candidate for an unknown block")
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

func TestBlockTreeKeepsTheFirstQuorumCertOfABlock(t *testing.T) {
	tree := NewBlockTree()
	blockHash := types.Hash{1}
	tree.Insert(types.Block{Header: types.Header{Height: 1}}, blockHash, finality.QuorumCert{})

	first := finality.QuorumCert{Height: 1, Round: 0, BlockHash: blockHash, VotingPower: 2}
	later := finality.QuorumCert{Height: 1, Round: 4, BlockHash: blockHash, VotingPower: 3}
	if err := tree.SetQuorumCert(first); err != nil {
		t.Fatal(err)
	}
	if err := tree.SetQuorumCert(later); err != nil {
		t.Fatal(err)
	}

	node, _ := tree.Get(blockHash)
	if node.QuorumCert.Round != 0 {
		t.Fatalf("expected the first certificate to stay attached, got %+v", node.QuorumCert)
	}
	if tree.HighQC().Round != 4 {
		t.Fatalf("expected the later certificate to be the high qc, got %+v", tree.HighQC())
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

func TestBlockTreeTracksHighestQuorumCertByView(t *testing.T) {
	tree := NewBlockTree()
	first := finality.QuorumCert{Height: 2, Round: 2, BlockHash: types.Hash{1}}
	lowerView := finality.QuorumCert{Height: 5, Round: 0, BlockHash: types.Hash{2}}
	sameViewLowerHeight := finality.QuorumCert{Height: 1, Round: 2, BlockHash: types.Hash{3}}
	sameViewHigherHeight := finality.QuorumCert{Height: 3, Round: 2, BlockHash: types.Hash{4}}
	higherView := finality.QuorumCert{Height: 1, Round: 3, BlockHash: types.Hash{5}}

	tree.ObserveQuorumCert(first)
	tree.ObserveQuorumCert(lowerView)
	tree.ObserveQuorumCert(sameViewLowerHeight)
	if tree.HighQC().BlockHash != first.BlockHash {
		t.Fatalf("a higher height must not beat a higher view, got %+v", tree.HighQC())
	}
	tree.ObserveQuorumCert(sameViewHigherHeight)
	if tree.HighQC().BlockHash != sameViewHigherHeight.BlockHash {
		t.Fatalf("expected the higher height to break a tie of views, got %+v", tree.HighQC())
	}
	tree.ObserveQuorumCert(higherView)
	if tree.HighQC().BlockHash != higherView.BlockHash {
		t.Fatalf("expected the higher view to win even at a lower height, got %+v", tree.HighQC())
	}
}

func TestBlockTreeAttachesHeldQuorumCertWhenTheBlockArrives(t *testing.T) {
	tree := NewBlockTree()
	blockHash := types.Hash{1}
	qc := finality.QuorumCert{Height: 1, Round: 0, BlockHash: blockHash}

	tree.ObserveQuorumCert(qc)
	if !sameQC(tree.HighQC(), qc) {
		t.Fatalf("expected the observed certificate to be the high qc, got %+v", tree.HighQC())
	}
	node := tree.Insert(types.Block{Header: types.Header{Height: 1}}, blockHash, finality.QuorumCert{})
	if !sameQC(node.QuorumCert, qc) {
		t.Fatalf("expected the held certificate to attach, got %+v", node.QuorumCert)
	}

	wrongHeight := finality.QuorumCert{Height: 7, Round: 1, BlockHash: types.Hash{2}}
	tree.ObserveQuorumCert(wrongHeight)
	other := tree.Insert(types.Block{Header: types.Header{Height: 2}}, types.Hash{2}, finality.QuorumCert{})
	if other.QuorumCert.Height != 0 {
		t.Fatalf("a certificate for another height must not attach, got %+v", other.QuorumCert)
	}
}

func TestBlockTreeInsertIsIdempotent(t *testing.T) {
	tree := NewBlockTree()
	parent := types.Hash{1}
	child := types.Hash{2}
	justify := finality.QuorumCert{Height: 1, Round: 0, BlockHash: parent}

	tree.Insert(types.Block{Header: types.Header{Height: 1}}, parent, finality.QuorumCert{})
	tree.Insert(types.Block{Header: types.Header{Height: 2, PreviousBlockHash: parent}}, child, finality.QuorumCert{})
	node := tree.Insert(types.Block{Header: types.Header{Height: 2, PreviousBlockHash: parent}}, child, justify)
	if tree.Len() != 2 {
		t.Fatalf("expected two blocks, got %d", tree.Len())
	}
	if !sameQC(node.JustifyQC, justify) {
		t.Fatalf("expected a later insert to fill the missing justify qc, got %+v", node.JustifyQC)
	}
	other := finality.QuorumCert{Height: 1, Round: 5, BlockHash: parent}
	node = tree.Insert(types.Block{Header: types.Header{Height: 2, PreviousBlockHash: parent}}, child, other)
	if !sameQC(node.JustifyQC, justify) {
		t.Fatalf("expected the first justify qc to be kept, got %+v", node.JustifyQC)
	}
}

func TestBlockTreeExtendsAncestor(t *testing.T) {
	tree := NewBlockTree()
	ancestor := types.Hash{1}
	child := types.Hash{2}
	grandchild := types.Hash{3}

	tree.Insert(types.Block{Header: types.Header{Height: 1}}, ancestor, finality.QuorumCert{})
	tree.Insert(types.Block{Header: types.Header{Height: 2, PreviousBlockHash: ancestor}}, child, finality.QuorumCert{})
	tree.Insert(types.Block{Header: types.Header{Height: 3, PreviousBlockHash: child}}, grandchild, finality.QuorumCert{})

	if !tree.Extends(grandchild, ancestor) {
		t.Fatal("expected grandchild to extend ancestor")
	}
	if tree.Extends(types.Hash{9}, ancestor) {
		t.Fatal("unknown descendant must not extend ancestor")
	}
	if tree.Extends(child, grandchild) {
		t.Fatal("ancestor must not extend descendant")
	}
}

func TestBlockTreePrunesBelowHeight(t *testing.T) {
	tree := NewBlockTree()
	hashes := []types.Hash{{1}, {2}, {3}, {4}}
	var parent types.Hash
	for index, hash := range hashes {
		tree.Insert(types.Block{Header: types.Header{Height: types.Height(index + 1), PreviousBlockHash: parent}}, hash, finality.QuorumCert{})
		parent = hash
	}
	tree.ObserveQuorumCert(finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{9}})

	tree.Prune(3)
	if tree.Len() != 2 {
		t.Fatalf("expected the blocks of heights 3 and 4 to remain, got %d", tree.Len())
	}
	if _, found := tree.Get(hashes[1]); found {
		t.Fatal("expected the block below the prune height to be gone")
	}
	if _, found := tree.Get(hashes[2]); !found {
		t.Fatal("expected the block at the prune height to remain")
	}
	if len(tree.certs) != 0 {
		t.Fatal("expected the held certificate below the prune height to be dropped")
	}
	if !tree.Extends(hashes[3], hashes[2]) {
		t.Fatal("expected the remaining blocks to stay linked")
	}
}

package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestStateMachineCreateProposalSortsTxsForFairOrdering(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	machine := cluster.newMachine()

	proposal, err := machine.CreateProposal(types.Block{
		Header: types.Header{Height: 1},
		Txs:    []types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")},
	}, 0, "a", machine.blockTree.HighQC())
	if err != nil {
		t.Fatal(err)
	}
	if !fairordering.IsOrderedWithSalt(proposal.Block.Txs, fairordering.HeightSalt(testChainID, 1)) {
		t.Fatalf("expected ordered proposal txs, got %q", proposal.Block.Txs)
	}
	if proposal.Block.Header.ConsensusHash != dataavailability.Commitment(proposal.Block.Txs) {
		t.Fatal("expected DA commitment over ordered txs")
	}
	if proposal.Block.Header.TxRoot != TxRoot(proposal.Block.Txs) {
		t.Fatal("expected the transaction root over the ordered txs")
	}
}

func TestStateMachineRejectsReorderedProposalTxs(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	machine := cluster.newMachine()

	ordered := fairordering.SortTxsWithSalt(
		[]types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")},
		fairordering.HeightSalt(testChainID, 1),
	)
	reordered := []types.Tx{ordered[1], ordered[0], ordered[2]}
	block := dataavailability.AttachCommitment(types.Block{
		Header: types.Header{ChainID: testChainID, Height: 1, ValidatorSetHash: cluster.set.Hash(), TxRoot: TxRoot(reordered), ValidatorSetHeight: finality.FirstValidatorSetHeight},
		Txs:    reordered,
	})
	err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) || !errors.Is(err, ErrInvalidBlockBody) {
		t.Fatalf("expected invalid proposal for reordered txs, got %v", err)
	}

	// The same transactions in fair order are accepted, so the rejection above is about
	// the ordering and nothing else.
	valid := dataavailability.AttachCommitment(types.Block{
		Header: types.Header{ChainID: testChainID, Height: 1, ValidatorSetHash: cluster.set.Hash(), TxRoot: TxRoot(ordered), ValidatorSetHeight: finality.FirstValidatorSetHeight},
		Txs:    ordered,
	})
	if err := machine.OnProposal(context.Background(), cluster.proposal("a", valid, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatalf("expected the fair-ordered block to be accepted, got %v", err)
	}
}

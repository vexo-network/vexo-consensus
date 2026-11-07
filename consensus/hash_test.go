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

// TestHashBlockCommitsToTheTransactions is the regression for HashBlock trusting a
// non-zero header.TxRoot: a block could keep a vouched-for root while carrying other
// transactions, so the hash the validators voted on did not bind the block body.
func TestHashBlockCommitsToTheTransactions(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	honest := cluster.block(1, types.Hash{}, "pay alice")

	swapped := honest
	swapped.Txs = []types.Tx{[]byte("pay mallory")}
	if swapped.Header.TxRoot != honest.Header.TxRoot {
		t.Fatal("setup: the swapped block keeps the claimed root")
	}
	if HashBlock(swapped) == HashBlock(honest) {
		t.Fatal("two bodies sharing a claimed transaction root must not share a block hash")
	}

	lying := honest
	lying.Header.TxRoot = types.Hash{0xBA, 0xD0}
	if HashBlock(lying) != HashBlock(honest) {
		t.Fatal("the hash must ignore the claimed root and derive it from the transactions")
	}

	empty := honest
	empty.Txs = nil
	empty.Header.TxRoot = types.Hash{0xBA, 0xD0}
	bare := honest.Header
	bare.TxRoot = types.Hash{}
	if HashBlock(empty) != HashBlock(types.Block{Header: bare}) {
		t.Fatal("a block without transactions hashes with a zero transaction root")
	}
}

func TestVerifyBlockBodyChecksRootCommitmentAndOrdering(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	honest := cluster.block(3, types.Hash{1}, "charlie", "alpha", "bravo")
	if err := VerifyBlockBody(honest); err != nil {
		t.Fatalf("a well-formed block must verify: %v", err)
	}
	if err := VerifyBlockBody(cluster.block(3, types.Hash{1})); err != nil {
		t.Fatalf("a block without transactions must verify: %v", err)
	}

	t.Run("claimed root does not match the transactions", func(t *testing.T) {
		block := honest
		block.Header.TxRoot = types.Hash{0xBA, 0xD0}
		if err := VerifyBlockBody(block); !errors.Is(err, ErrInvalidBlockBody) {
			t.Fatalf("expected an invalid block body, got %v", err)
		}
	})
	t.Run("transactions replaced after the header was built", func(t *testing.T) {
		block := honest
		block.Txs = []types.Tx{[]byte("pay mallory")}
		if err := VerifyBlockBody(block); !errors.Is(err, ErrInvalidBlockBody) {
			t.Fatalf("expected an invalid block body, got %v", err)
		}
	})
	t.Run("transactions appended", func(t *testing.T) {
		block := honest
		block.Txs = append(append([]types.Tx(nil), honest.Txs...), types.Tx("extra"))
		if err := VerifyBlockBody(block); !errors.Is(err, ErrInvalidBlockBody) {
			t.Fatalf("expected an invalid block body, got %v", err)
		}
	})
	t.Run("transactions without a data availability commitment", func(t *testing.T) {
		block := honest
		block.Header.ConsensusHash = types.Hash{}
		if err := VerifyBlockBody(block); !errors.Is(err, dataavailability.ErrMissingData) {
			t.Fatalf("expected missing data, got %v", err)
		}
	})
	t.Run("wrong data availability commitment", func(t *testing.T) {
		block := honest
		block.Header.ConsensusHash = dataavailability.Commitment([]types.Tx{[]byte("other")})
		if err := VerifyBlockBody(block); !errors.Is(err, dataavailability.ErrCommitmentMismatch) {
			t.Fatalf("expected a commitment mismatch, got %v", err)
		}
	})
	t.Run("transactions out of fair order", func(t *testing.T) {
		ordered := fairordering.SortTxsWithSalt(
			[]types.Tx{[]byte("charlie"), []byte("alpha"), []byte("bravo")},
			fairordering.HeightSalt(testChainID, 3),
		)
		block := dataavailability.AttachCommitment(types.Block{
			Header: types.Header{ChainID: testChainID, Height: 3, PreviousBlockHash: types.Hash{1}, ValidatorSetHash: cluster.set.Hash()},
			Txs:    []types.Tx{ordered[1], ordered[0], ordered[2]},
		})
		block.Header.TxRoot = TxRoot(block.Txs)
		if err := VerifyBlockBody(block); !errors.Is(err, ErrInvalidBlockBody) {
			t.Fatalf("expected an invalid block body, got %v", err)
		}
	})
}

// A proposal that lies about its body is rejected by the state machine and never reaches the
// block tree, even when the proposer's signature over it is genuine.
func TestProposalWithSwappedTransactionsIsRejected(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	block := cluster.block(1, types.Hash{}, "pay alice")
	block.Txs = []types.Tx{[]byte("pay mallory")}

	err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) || !errors.Is(err, ErrInvalidBlockBody) {
		t.Fatalf("expected an invalid proposal with an invalid body, got %v", err)
	}
	if machine.blockTree.Len() != 0 {
		t.Fatal("a proposal with a forged body was stored")
	}
}

func TestObserveCommittedBlockVerifiesTheBody(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	block := cluster.block(1, types.Hash{}, "pay alice")
	qc := cluster.qc(1, 0, HashBlock(block))

	tampered := block
	tampered.Txs = []types.Tx{[]byte("pay mallory")}
	if err := machine.ObserveCommittedBlock(tampered, qc); !errors.Is(err, ErrInvalidBlockCert) {
		t.Fatalf("expected a committed block with a forged body to be refused, got %v", err)
	}
	if machine.floorHeight != 0 {
		t.Fatal("a forged committed block raised the finalized height")
	}
	if err := machine.ObserveCommittedBlock(block, qc); err != nil {
		t.Fatalf("the genuine block must be accepted, got %v", err)
	}
}

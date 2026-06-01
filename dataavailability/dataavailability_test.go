package dataavailability

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestBuildProofComputesCommitmentMetadata(t *testing.T) {
	txs := []types.Tx{[]byte("one"), []byte("two")}
	proof := BuildProof(txs)
	if proof.Commitment == (types.Hash{}) {
		t.Fatal("expected non-zero commitment")
	}
	if proof.TxCount != 2 || proof.TotalBytes != 6 {
		t.Fatalf("unexpected proof: %+v", proof)
	}
}

func TestVerifyAcceptsMatchingCommitment(t *testing.T) {
	block := AttachCommitment(types.Block{Txs: []types.Tx{[]byte("tx")}})
	if err := Verify(block.Header, block.Txs); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsMismatchedCommitment(t *testing.T) {
	block := AttachCommitment(types.Block{Txs: []types.Tx{[]byte("tx")}})
	err := Verify(block.Header, []types.Tx{[]byte("other")})
	if !errors.Is(err, ErrCommitmentMismatch) {
		t.Fatalf("expected commitment mismatch, got %v", err)
	}
}

func TestVerifyRejectsMissingCommitmentWithData(t *testing.T) {
	err := Verify(types.Header{}, []types.Tx{[]byte("tx")})
	if !errors.Is(err, ErrMissingData) {
		t.Fatalf("expected missing data commitment, got %v", err)
	}
}

func TestVerifyAllowsEmptyDataWithoutCommitment(t *testing.T) {
	if err := Verify(types.Header{}, nil); err != nil {
		t.Fatal(err)
	}
}

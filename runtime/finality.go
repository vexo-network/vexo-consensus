package runtime

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrFinalityProofHeightMismatch = errors.New("finality proof height mismatch")
	ErrFinalityProofBlockMismatch  = errors.New("finality proof block mismatch")
)

// FinalityProof returns the proof skeleton of the block stored at height: its header and quorumCert,
// QC(B). A skeleton only shows that the block is certified; the 2-chain commit rule makes it final
// once the caller attaches the child of the block with finality.Proof.WithChild.
func (runtime *Runtime) FinalityProof(ctx context.Context, height types.Height, quorumCert finality.QuorumCert) (finality.Proof, error) {
	record, err := runtime.BlockByHeight(ctx, height)
	if err != nil {
		return finality.Proof{}, err
	}
	return NewFinalityProof(record.Block.Header, record.Hash, quorumCert)
}

// NewFinalityProof returns the proof skeleton of the block with header and blockHash, the skeleton
// FinalityProof returns once the block is stored. A caller that holds the block it just executed
// builds the skeleton with it instead of reading the block back.
func NewFinalityProof(header types.Header, blockHash types.Hash, quorumCert finality.QuorumCert) (finality.Proof, error) {
	if quorumCert.Height != header.Height {
		return finality.Proof{}, ErrFinalityProofHeightMismatch
	}
	if quorumCert.BlockHash != blockHash {
		return finality.Proof{}, ErrFinalityProofBlockMismatch
	}
	proof := finality.NewProof(header, quorumCert)
	proof.BlockHash = blockHash
	return proof, nil
}

// VerifyFinalityProof verifies that proof proves final, under the 2-chain commit rule, the block this
// runtime executed at proof.Header.Height: the proof must be for that very block, and QC(B) and the
// QC(C) of its child must each verify against the validator set its header names. A
// skeleton from FinalityProof without its child fails; VerifyBlockCert checks a certificate alone.
func (runtime *Runtime) VerifyFinalityProof(ctx context.Context, proof finality.Proof) error {
	record, err := runtime.BlockByHeight(ctx, proof.Header.Height)
	if err != nil {
		return err
	}
	if proof.BlockHash != record.Hash {
		return ErrFinalityProofBlockMismatch
	}
	if proof.Header != record.Block.Header {
		return ErrFinalityProofBlockMismatch
	}
	return runtime.NewFinalityVerifier().VerifyFinalityProofWithContext(ctx, proof)
}

// VerifyBlockCert verifies that quorumCert is a quorum certificate of the validator set the header
// of the block at height names, for the block this runtime executed there. It shows that a quorum voted for the block, not
// that the block is final; VerifyFinalityProof does.
func (runtime *Runtime) VerifyBlockCert(ctx context.Context, height types.Height, quorumCert finality.QuorumCert) error {
	record, err := runtime.BlockByHeight(ctx, height)
	if err != nil {
		return err
	}
	if quorumCert.Height != height {
		return ErrFinalityProofHeightMismatch
	}
	if quorumCert.BlockHash != record.Hash {
		return ErrFinalityProofBlockMismatch
	}
	return runtime.NewFinalityVerifier().VerifyBlockCertWithContext(ctx, record.Block.Header, quorumCert)
}

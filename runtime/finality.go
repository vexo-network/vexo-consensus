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

func (runtime *Runtime) FinalityProof(ctx context.Context, height types.Height, quorumCert finality.QuorumCert) (finality.Proof, error) {
	record, err := runtime.BlockByHeight(ctx, height)
	if err != nil {
		return finality.Proof{}, err
	}
	if quorumCert.Height != height {
		return finality.Proof{}, ErrFinalityProofHeightMismatch
	}

	proof := finality.NewProof(record.Block.Header, quorumCert)
	if quorumCert.BlockHash != proof.HeaderHash() {
		return finality.Proof{}, ErrFinalityProofBlockMismatch
	}
	return proof, nil
}

func (runtime *Runtime) VerifyFinalityProof(ctx context.Context, height types.Height, quorumCert finality.QuorumCert) error {
	proof, err := runtime.FinalityProof(ctx, height, quorumCert)
	if err != nil {
		return err
	}
	verifier, err := runtime.NewFinalityVerifier(ctx, height)
	if err != nil {
		return err
	}
	return verifier.VerifyFinalityProofWithContext(ctx, proof)
}

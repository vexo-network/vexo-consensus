package finality

import "github.com/vexo-network/vexo-consensus/types"

type QuorumCert struct {
	Height      types.Height
	Round       types.Round
	BlockHash   types.Hash
	Signers     types.Bitmap
	Signature   types.AggregateSignature
	VotingPower types.VotingPower
}

type TimeoutCert struct {
	Height    types.Height
	Round     types.Round
	HighQC    QuorumCert
	Signers   types.Bitmap
	Signature types.AggregateSignature
}

type Proof struct {
	Header             types.Header
	BlockHash          types.Hash
	QuorumCert         QuorumCert
	ValidatorSetHeight types.Height
	ValidatorSetHash   types.Hash
}

type LightVerifier interface {
	VerifyFinalityProof(proof Proof) error
}

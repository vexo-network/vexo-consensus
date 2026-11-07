package finality

import "github.com/vexo-network/vexo-consensus/types"

// QuorumCert is a quorum certificate: an aggregate of the votes of a quorum of validators for one
// block in one view. Round carries the view in which the block was proposed; views are global and
// never reset when the height advances.
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

// Proof proves that the block B of Header is final under the 2-chain commit rule of
// consensus.TwoChainCommitRule.
//
//   - BlockHash is the hash of B and must equal HeaderHash(Header).
//   - QuorumCert is QC(B), a quorum certificate for B formed in some view b.
//   - CommitChain holds exactly one link, the child C of B. The child's QuorumCert is QC(C), formed
//     in view b+1. A proof without the child only shows that B is certified, not that it is final.
type Proof struct {
	Header             types.Header
	BlockHash          types.Hash
	QuorumCert         QuorumCert
	CommitChain        []CommitLink
	ValidatorSetHeight types.Height
	ValidatorSetHash   types.Hash
}

// CommitLink is the child C of the block B that a Proof proves final. Header is the header of C,
// BlockHash its hash (HeaderHash(Header)) and QuorumCert is QC(C): the certificate that C itself
// gathered, in the view directly after the view of QC(B). The certificate C carries for B is the
// proof's own QuorumCert.
type CommitLink struct {
	Header     types.Header
	BlockHash  types.Hash
	QuorumCert QuorumCert
}

type LightVerifier interface {
	VerifyFinalityProof(proof Proof) error
}

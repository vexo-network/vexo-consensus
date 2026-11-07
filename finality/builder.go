package finality

import "github.com/vexo-network/vexo-consensus/types"

// NewProof builds the proof skeleton of the block with the given header and its quorum
// certificate. The proof is not final until the child link is attached with WithChild.
func NewProof(header types.Header, quorumCert QuorumCert) Proof {
	return Proof{
		Header:             header,
		BlockHash:          HeaderHash(header),
		QuorumCert:         quorumCert,
		ValidatorSetHeight: header.ValidatorSetHeight,
		ValidatorSetHash:   header.ValidatorSetHash,
	}
}

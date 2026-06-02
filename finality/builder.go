package finality

import "github.com/vexo-network/vexo-consensus/types"

func NewProof(header types.Header, quorumCert QuorumCert) Proof {
	return Proof{
		Header:             header,
		QuorumCert:         quorumCert,
		ValidatorSetHeight: header.Height,
		ValidatorSetHash:   header.ValidatorSetHash,
	}
}

func HeaderHash(header types.Header) types.Hash {
	return NewProof(header, QuorumCert{}).HeaderHash()
}

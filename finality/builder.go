package finality

import "github.com/vexo-network/vexo-consensus/types"

func NewProof(header types.Header, quorumCert QuorumCert) Proof {
	blockHash := quorumCert.BlockHash
	if blockHash == (types.Hash{}) {
		blockHash = Proof{Header: header}.HeaderHash()
	}
	return Proof{
		Header:             header,
		BlockHash:          blockHash,
		QuorumCert:         quorumCert,
		ValidatorSetHeight: header.Height,
		ValidatorSetHash:   header.ValidatorSetHash,
	}
}

func HeaderHash(header types.Header) types.Hash {
	return NewProof(header, QuorumCert{}).HeaderHash()
}

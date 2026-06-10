package finality

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/signbytes"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrEmptySigner = errors.New("empty signer in quorum certificate")

func (proof Proof) HeaderHash() types.Hash {
	return hashHeader(proof.Header)
}

func (link CommitLink) HeaderHash() types.Hash {
	return hashHeader(link.Header)
}

func hashHeader(header types.Header) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(header.ChainID))
	writeUint64(hasher, uint64(header.Height))
	writeUint64(hasher, uint64(header.TimeUnixNano))
	hasher.Write(header.PreviousBlockHash[:])
	hasher.Write(header.AppHash[:])
	hasher.Write(header.ValidatorSetHash[:])
	hasher.Write(header.ConsensusHash[:])
	hasher.Write(header.TxRoot[:])

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

func (proof Proof) SignBytes() []byte {
	blockHash := proof.BlockHash
	if blockHash == (types.Hash{}) {
		blockHash = proof.QuorumCert.BlockHash
	}
	if blockHash == (types.Hash{}) {
		blockHash = proof.HeaderHash()
	}
	return signbytes.Vote(proof.QuorumCert.Height, proof.QuorumCert.Round, blockHash)
}

func (proof Proof) HasThreeChainCommitProof() bool {
	return len(proof.CommitChain) >= 2
}

func (link CommitLink) SignBytes() []byte {
	blockHash := link.QuorumCert.BlockHash
	if blockHash == (types.Hash{}) {
		blockHash = link.BlockHash
	}
	return signbytes.Vote(link.QuorumCert.Height, link.QuorumCert.Round, blockHash)
}

func EncodeSigners(signers []types.ValidatorID) types.Bitmap {
	parts := make([]string, 0, len(signers))
	for _, signer := range signers {
		parts = append(parts, string(signer))
	}
	return types.Bitmap(strings.Join(parts, ","))
}

func ParseSigners(signers types.Bitmap) ([]types.ValidatorID, error) {
	if len(signers) == 0 {
		return nil, nil
	}

	parts := strings.Split(string(signers), ",")
	parsed := make([]types.ValidatorID, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, ErrEmptySigner
		}
		parsed = append(parsed, types.ValidatorID(part))
	}
	return parsed, nil
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

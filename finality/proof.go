package finality

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrEmptySigner = errors.New("empty signer in quorum certificate")

func (proof Proof) HeaderHash() types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(proof.Header.ChainID))
	writeUint64(hasher, uint64(proof.Header.Height))
	writeUint64(hasher, uint64(proof.Header.TimeUnixNano))
	hasher.Write(proof.Header.PreviousBlockHash[:])
	hasher.Write(proof.Header.AppHash[:])
	hasher.Write(proof.Header.ValidatorSetHash[:])
	hasher.Write(proof.Header.ConsensusHash[:])

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
	message := make([]byte, 0, len(blockHash)+20)
	message = append(message, []byte("vote")...)

	var buffer [16]byte
	binary.BigEndian.PutUint64(buffer[:8], uint64(proof.QuorumCert.Height))
	binary.BigEndian.PutUint64(buffer[8:], uint64(proof.QuorumCert.Round))
	message = append(message, buffer[:]...)
	message = append(message, blockHash[:]...)
	return message
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

package consensus

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

func ProposalSignBytes(proposal Proposal) []byte {
	blockHash := HashBlock(proposal.Block)
	message := make([]byte, 0, 128)
	message = append(message, []byte("proposal")...)
	message = appendUint64(message, uint64(proposal.Block.Header.Height))
	message = appendUint64(message, uint64(proposal.Round))
	message = append(message, []byte(proposal.Proposer)...)
	message = append(message, blockHash[:]...)
	message = appendQuorumCert(message, proposal.JustifyQC)
	return message
}

func VoteSignBytes(vote Vote) []byte {
	message := make([]byte, 0, 80)
	message = append(message, []byte("vote")...)
	message = appendUint64(message, uint64(vote.Height))
	message = appendUint64(message, uint64(vote.Round))
	message = append(message, vote.BlockHash[:]...)
	return message
}

func TimeoutVoteSignBytes(vote TimeoutVote) []byte {
	message := make([]byte, 0, 112)
	message = append(message, []byte("timeout_vote")...)
	message = appendUint64(message, uint64(vote.Height))
	message = appendUint64(message, uint64(vote.Round))
	message = appendQuorumCert(message, vote.HighQC)
	return message
}

func appendQuorumCert(message []byte, quorumCert finality.QuorumCert) []byte {
	message = appendUint64(message, uint64(quorumCert.Height))
	message = appendUint64(message, uint64(quorumCert.Round))
	message = append(message, quorumCert.BlockHash[:]...)
	message = appendUint64(message, uint64(quorumCert.VotingPower))
	signersHash := sha256.Sum256([]byte(quorumCert.Signers))
	message = append(message, signersHash[:]...)
	return message
}

func appendUint64(message []byte, value uint64) []byte {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	return append(message, buffer[:]...)
}

type signatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

type aggregateSigner interface {
	Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
}

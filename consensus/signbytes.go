package consensus

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/signbytes"
	"github.com/vexo-network/vexo-consensus/types"
)

// ProposalSignBytes binds the proposer's signature to the block, its view, the
// justifying quorum certificate and the timeout certificate that licenses a view jump.
func ProposalSignBytes(proposal Proposal) []byte {
	return proposalSignBytes(proposal, HashBlock(proposal.Block))
}

// proposalSignBytes returns what the proposer of proposal signs, for the block of blockHash.
func proposalSignBytes(proposal Proposal, blockHash types.Hash) []byte {
	message := make([]byte, 0, 192)
	message = append(message, []byte("proposal")...)
	message = appendUint64(message, uint64(proposal.Block.Header.Height))
	message = appendUint64(message, uint64(proposal.Round))
	message = append(message, []byte(proposal.Proposer)...)
	message = append(message, blockHash[:]...)
	message = appendQuorumCert(message, proposal.JustifyQC)
	message = appendTimeoutCert(message, proposal.TimeoutCert)
	return message
}

func VoteSignBytes(vote Vote) []byte {
	return signbytes.Vote(vote.Height, vote.Round, vote.BlockHash)
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

func appendTimeoutCert(message []byte, timeoutCert finality.TimeoutCert) []byte {
	message = appendUint64(message, uint64(timeoutCert.Height))
	message = appendUint64(message, uint64(timeoutCert.Round))
	message = appendQuorumCert(message, timeoutCert.HighQC)
	signersHash := sha256.Sum256([]byte(timeoutCert.Signers))
	message = append(message, signersHash[:]...)
	signatureHash := sha256.Sum256([]byte(timeoutCert.Signature))
	message = append(message, signatureHash[:]...)
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

// aggregateVerifier verifies an aggregate signature over a single message by the
// given public keys. The consensus domain is applied by the caller.
type aggregateVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

package consensus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/lanes"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrUnknownConsensusMessage = errors.New("unknown consensus message")
	// ErrInvalidWireMessage reports bytes that are not the encoding of a consensus message.
	ErrInvalidWireMessage = errors.New("invalid consensus message")
)

type MessageType string

const (
	MessageProposal        MessageType = "proposal"
	MessageLaneProposal    MessageType = "lane_proposal"
	MessageProposalRequest MessageType = "proposal_request"
	MessageVote            MessageType = "vote"
	MessageTimeoutVote     MessageType = "timeout_vote"
)

// WireMessage is a decoded consensus message: Type names the one field that is set.
type WireMessage struct {
	Type            MessageType
	Proposal        *Proposal
	LaneProposal    *LaneProposal
	ProposalRequest *ProposalRequest
	Vote            *Vote
	TimeoutVote     *TimeoutVote
}

// A consensus message is a version byte, a type byte and a body. The body has the fields of its
// struct in order, in the canonical binary encoding of package types, so a message has exactly one
// encoding and the decoder refuses any other: bytes that do not parse, bytes left over, and lengths
// and counts over the limits or beyond the input.
const wireVersion = 1

const (
	wireProposal byte = iota + 1
	wireLaneProposal
	wireProposalRequest
	wireVote
	wireTimeoutVote
)

func EncodeProposal(proposal Proposal) ([]byte, error) {
	return appendProposal(newWireBuffer(wireProposal, proposalHint(proposal)), proposal), nil
}

func EncodeLaneProposal(compact LaneProposal) ([]byte, error) {
	return appendLaneProposal(newWireBuffer(wireLaneProposal, laneProposalHint(compact)), compact), nil
}

func EncodeProposalRequest(request ProposalRequest) ([]byte, error) {
	return append(newWireBuffer(wireProposalRequest, len(request.BlockHash)), request.BlockHash[:]...), nil
}

func EncodeVote(vote Vote) ([]byte, error) {
	return appendVote(newWireBuffer(wireVote, voteHint(vote)), vote), nil
}

func EncodeTimeoutVote(vote TimeoutVote) ([]byte, error) {
	return appendTimeoutVote(newWireBuffer(wireTimeoutVote, timeoutVoteHint(vote)), vote), nil
}

// wireSlack is the room the hints below add for the fixed fields of a message and the lengths of
// its variable parts.
const wireSlack = 512

// newWireBuffer returns a buffer that holds the header of a message of the given type and has room
// for hint more bytes. A hint is an estimate of the length of the body that lets an encoder
// allocate once: a body that outgrows it costs a reallocation, never correctness.
func newWireBuffer(messageType byte, hint int) []byte {
	return append(make([]byte, 0, 2+hint), wireVersion, messageType)
}

func voteHint(vote Vote) int {
	return len(vote.ValidatorID) + len(vote.Signature) + wireSlack
}

func quorumCertHint(qc finality.QuorumCert) int {
	return len(qc.Signers) + len(qc.Signature)
}

func timeoutCertHint(timeoutCert finality.TimeoutCert) int {
	return quorumCertHint(timeoutCert.HighQC) + len(timeoutCert.Signers) + len(timeoutCert.Signature)
}

func timeoutVoteHint(vote TimeoutVote) int {
	return len(vote.ValidatorID) + quorumCertHint(vote.HighQC) + len(vote.Signature) + wireSlack
}

func proposalHint(proposal Proposal) int {
	return types.BlockSize(proposal.Block) + len(proposal.Proposer) + quorumCertHint(proposal.JustifyQC) +
		timeoutCertHint(proposal.TimeoutCert) + len(proposal.Signature) + wireSlack
}

func laneProposalHint(compact LaneProposal) int {
	hint := proposalHint(compact.Proposal) + len(compact.Excluded)*binary.MaxVarintLen32
	for _, ref := range compact.Batches {
		hint += len(ref.ID) + len(ref.Author) + binary.MaxVarintLen32
	}
	for _, tx := range compact.Extra {
		hint += len(tx) + binary.MaxVarintLen32
	}
	return hint
}

func appendVote(dst []byte, vote Vote) []byte {
	dst = binary.AppendUvarint(dst, uint64(vote.Height))
	dst = binary.AppendUvarint(dst, uint64(vote.Round))
	dst = append(dst, vote.BlockHash[:]...)
	dst = types.AppendBytes(dst, vote.ValidatorID)
	return types.AppendBytes(dst, vote.Signature)
}

func readVote(reader *types.Reader) Vote {
	var vote Vote
	vote.Height = types.Height(reader.Uvarint())
	vote.Round = types.Round(reader.Uvarint())
	vote.BlockHash = reader.Hash()
	vote.ValidatorID = types.ValidatorID(reader.String(types.MaxIDBytes))
	vote.Signature = types.Signature(reader.Bytes(types.MaxSignatureBytes))
	return vote
}

func appendTimeoutVote(dst []byte, vote TimeoutVote) []byte {
	dst = binary.AppendUvarint(dst, uint64(vote.Height))
	dst = binary.AppendUvarint(dst, uint64(vote.Round))
	dst = types.AppendBytes(dst, vote.ValidatorID)
	dst = finality.AppendQuorumCert(dst, vote.HighQC)
	return types.AppendBytes(dst, vote.Signature)
}

func readTimeoutVote(reader *types.Reader) TimeoutVote {
	var vote TimeoutVote
	vote.Height = types.Height(reader.Uvarint())
	vote.Round = types.Round(reader.Uvarint())
	vote.ValidatorID = types.ValidatorID(reader.String(types.MaxIDBytes))
	vote.HighQC = finality.ReadQuorumCert(reader)
	vote.Signature = types.Signature(reader.Bytes(types.MaxSignatureBytes))
	return vote
}

func appendProposal(dst []byte, proposal Proposal) []byte {
	dst = types.AppendBlock(dst, proposal.Block)
	dst = binary.AppendUvarint(dst, uint64(proposal.Round))
	dst = types.AppendBytes(dst, proposal.Proposer)
	dst = finality.AppendQuorumCert(dst, proposal.JustifyQC)
	dst = finality.AppendTimeoutCert(dst, proposal.TimeoutCert)
	return types.AppendBytes(dst, proposal.Signature)
}

func readProposal(reader *types.Reader) Proposal {
	var proposal Proposal
	proposal.Block = reader.Block(MaxBlockBytes)
	proposal.Round = types.Round(reader.Uvarint())
	proposal.Proposer = types.ValidatorID(reader.String(types.MaxIDBytes))
	proposal.JustifyQC = finality.ReadQuorumCert(reader)
	proposal.TimeoutCert = finality.ReadTimeoutCert(reader)
	proposal.Signature = types.Signature(reader.Bytes(types.MaxSignatureBytes))
	return proposal
}

// maxLaneExcluded bounds the transactions a lane proposal excludes: the transactions its batches
// can carry.
const maxLaneExcluded = MaxLaneRefs * lanes.MaxBatchTxs

// minLaneRefBytes is the length of the encoding of a reference to a batch of the shortest author.
const minLaneRefBytes = len(types.Hash{}) + 1

func appendLaneProposal(dst []byte, compact LaneProposal) []byte {
	dst = appendProposal(dst, compact.Proposal)
	dst = binary.AppendUvarint(dst, uint64(len(compact.Batches)))
	for _, ref := range compact.Batches {
		dst = append(dst, ref.ID[:]...)
		dst = types.AppendBytes(dst, ref.Author)
	}
	dst = binary.AppendUvarint(dst, uint64(len(compact.Excluded)))
	for _, position := range compact.Excluded {
		dst = binary.AppendUvarint(dst, uint64(position))
	}
	return types.AppendTxs(dst, compact.Extra)
}

func readLaneProposal(reader *types.Reader) LaneProposal {
	var compact LaneProposal
	compact.Proposal = readProposal(reader)
	if count := reader.Count(MaxLaneRefs, minLaneRefBytes); count > 0 {
		compact.Batches = make([]LaneRef, count)
		for index := range compact.Batches {
			compact.Batches[index].ID = reader.Hash()
			compact.Batches[index].Author = types.ValidatorID(reader.String(types.MaxIDBytes))
		}
	}
	if count := reader.Count(maxLaneExcluded, 1); count > 0 {
		compact.Excluded = make([]uint32, count)
		for index := range compact.Excluded {
			position := reader.Uvarint()
			if position > math.MaxUint32 {
				reader.Fail("excluded position over 32 bits")
			}
			compact.Excluded[index] = uint32(position)
		}
	}
	compact.Extra = reader.Txs(MaxBlockBytes)
	return compact
}

// DecodeWireMessage decodes a consensus message that one of the Encode functions encoded. It
// refuses a message of another version or type with ErrUnknownConsensusMessage, and any other
// bytes with ErrInvalidWireMessage.
func DecodeWireMessage(data []byte) (WireMessage, error) {
	reader := types.NewReader(data)
	version, messageType := reader.Byte(), reader.Byte()
	if err := reader.Err(); err != nil {
		return WireMessage{}, fmt.Errorf("%w: %w", ErrInvalidWireMessage, err)
	}
	if version != wireVersion {
		return WireMessage{}, ErrUnknownConsensusMessage
	}
	var message WireMessage
	switch messageType {
	case wireProposal:
		proposal := readProposal(reader)
		message = WireMessage{Type: MessageProposal, Proposal: &proposal}
	case wireLaneProposal:
		compact := readLaneProposal(reader)
		message = WireMessage{Type: MessageLaneProposal, LaneProposal: &compact}
	case wireProposalRequest:
		request := ProposalRequest{BlockHash: reader.Hash()}
		message = WireMessage{Type: MessageProposalRequest, ProposalRequest: &request}
	case wireVote:
		vote := readVote(reader)
		message = WireMessage{Type: MessageVote, Vote: &vote}
	case wireTimeoutVote:
		vote := readTimeoutVote(reader)
		message = WireMessage{Type: MessageTimeoutVote, TimeoutVote: &vote}
	default:
		return WireMessage{}, ErrUnknownConsensusMessage
	}
	if err := reader.Done(); err != nil {
		return WireMessage{}, fmt.Errorf("%w: %w", ErrInvalidWireMessage, err)
	}
	return message, nil
}

func TopicForMessage(messageType MessageType) (p2p.Topic, error) {
	switch messageType {
	case MessageProposal, MessageLaneProposal, MessageProposalRequest:
		return p2p.TopicProposal, nil
	case MessageVote:
		return p2p.TopicVote, nil
	case MessageTimeoutVote:
		return p2p.TopicTimeout, nil
	default:
		return "", ErrUnknownConsensusMessage
	}
}

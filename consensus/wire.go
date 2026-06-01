package consensus

import (
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/p2p"
)

var ErrUnknownConsensusMessage = errors.New("unknown consensus message")

type MessageType string

const (
	MessageProposal    MessageType = "proposal"
	MessageVote        MessageType = "vote"
	MessageTimeoutVote MessageType = "timeout_vote"
)

type WireMessage struct {
	Type        MessageType  `json:"type"`
	Proposal    *Proposal    `json:"proposal,omitempty"`
	Vote        *Vote        `json:"vote,omitempty"`
	TimeoutVote *TimeoutVote `json:"timeout_vote,omitempty"`
}

func EncodeProposal(proposal Proposal) ([]byte, error) {
	return json.Marshal(WireMessage{Type: MessageProposal, Proposal: &proposal})
}

func EncodeVote(vote Vote) ([]byte, error) {
	return json.Marshal(WireMessage{Type: MessageVote, Vote: &vote})
}

func EncodeTimeoutVote(vote TimeoutVote) ([]byte, error) {
	return json.Marshal(WireMessage{Type: MessageTimeoutVote, TimeoutVote: &vote})
}

func DecodeWireMessage(data []byte) (WireMessage, error) {
	var message WireMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return WireMessage{}, err
	}
	switch message.Type {
	case MessageProposal:
		if message.Proposal == nil {
			return WireMessage{}, ErrUnknownConsensusMessage
		}
	case MessageVote:
		if message.Vote == nil {
			return WireMessage{}, ErrUnknownConsensusMessage
		}
	case MessageTimeoutVote:
		if message.TimeoutVote == nil {
			return WireMessage{}, ErrUnknownConsensusMessage
		}
	default:
		return WireMessage{}, ErrUnknownConsensusMessage
	}
	return message, nil
}

func TopicForMessage(messageType MessageType) (p2p.Topic, error) {
	switch messageType {
	case MessageProposal:
		return p2p.TopicProposal, nil
	case MessageVote:
		return p2p.TopicVote, nil
	case MessageTimeoutVote:
		return p2p.TopicTimeout, nil
	default:
		return "", ErrUnknownConsensusMessage
	}
}

package consensus

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestConsensusWireCodecRoundTripsMessages(t *testing.T) {
	proposalData, err := EncodeProposal(Proposal{Round: 1, Proposer: "a"})
	if err != nil {
		t.Fatal(err)
	}
	proposalMessage, err := DecodeWireMessage(proposalData)
	if err != nil {
		t.Fatal(err)
	}
	if proposalMessage.Type != MessageProposal || proposalMessage.Proposal.Proposer != "a" {
		t.Fatalf("unexpected proposal message: %+v", proposalMessage)
	}

	voteData, err := EncodeVote(Vote{Height: 1, Round: 2, BlockHash: types.Hash{1}, ValidatorID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	voteMessage, err := DecodeWireMessage(voteData)
	if err != nil {
		t.Fatal(err)
	}
	if voteMessage.Type != MessageVote || voteMessage.Vote.ValidatorID != "b" {
		t.Fatalf("unexpected vote message: %+v", voteMessage)
	}

	timeoutData, err := EncodeTimeoutVote(TimeoutVote{Height: 1, Round: 3, ValidatorID: "c"})
	if err != nil {
		t.Fatal(err)
	}
	timeoutMessage, err := DecodeWireMessage(timeoutData)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutMessage.Type != MessageTimeoutVote || timeoutMessage.TimeoutVote.ValidatorID != "c" {
		t.Fatalf("unexpected timeout message: %+v", timeoutMessage)
	}
}

func TestConsensusWireRejectsMalformedMessages(t *testing.T) {
	cases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"type":"proposal"}`),
		[]byte(`{"type":"unknown"}`),
		[]byte(`not-json`),
	}
	for _, data := range cases {
		if _, err := DecodeWireMessage(data); err == nil {
			t.Fatalf("expected decode error for %s", data)
		}
	}
}

func TestTopicForMessage(t *testing.T) {
	topic, err := TopicForMessage(MessageProposal)
	if err != nil {
		t.Fatal(err)
	}
	if topic != p2p.TopicProposal {
		t.Fatalf("unexpected proposal topic %s", topic)
	}
	topic, err = TopicForMessage(MessageTimeoutVote)
	if err != nil {
		t.Fatal(err)
	}
	if topic != p2p.TopicTimeout {
		t.Fatalf("unexpected timeout topic %s", topic)
	}
	if _, err := TopicForMessage("bad"); !errors.Is(err, ErrUnknownConsensusMessage) {
		t.Fatalf("expected unknown message, got %v", err)
	}
}

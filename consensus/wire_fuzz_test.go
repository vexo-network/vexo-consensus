package consensus

import "testing"

func FuzzDecodeWireMessage(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"type":"proposal","proposal":{"round":0,"proposer":"alice"}}`),
		[]byte(`{"type":"vote","vote":{"height":1,"round":0,"validator_id":"alice"}}`),
		[]byte(`{"type":"timeout_vote","timeout_vote":{"height":1,"round":0,"validator_id":"alice"}}`),
		[]byte(`{"type":"unknown"}`),
		[]byte(`not-json`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		message, err := DecodeWireMessage(data)
		if err != nil {
			return
		}
		if _, err := TopicForMessage(message.Type); err != nil {
			t.Fatalf("decoded unsupported consensus message type %q", message.Type)
		}
		switch message.Type {
		case MessageProposal:
			if message.Proposal == nil {
				t.Fatal("decoded proposal message without proposal")
			}
		case MessageVote:
			if message.Vote == nil {
				t.Fatal("decoded vote message without vote")
			}
		case MessageTimeoutVote:
			if message.TimeoutVote == nil {
				t.Fatal("decoded timeout message without timeout vote")
			}
		default:
			t.Fatalf("decoded unknown message type %q", message.Type)
		}
	})
}

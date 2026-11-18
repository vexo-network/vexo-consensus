package consensus

import (
	"bytes"
	"testing"
)

// FuzzDecodeWireMessage checks that no input makes the decoder panic, and that what decodes has one
// encoding: the bytes it came from.
func FuzzDecodeWireMessage(f *testing.F) {
	for _, testCase := range wireCases(f) {
		data, err := encodeWireMessage(testCase.message)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
		f.Add(data[:len(data)/2])
	}
	f.Add([]byte{})
	f.Add([]byte{wireVersion})
	f.Add([]byte{wireVersion, wireVote})
	f.Add([]byte{wireVersion + 1, wireVote})
	f.Add([]byte(`{"type":"proposal","proposal":{"round":0,"proposer":"alice"}}`))
	f.Add([]byte(`not-json`))
	f.Fuzz(func(t *testing.T, data []byte) {
		message, err := DecodeWireMessage(data)
		if err != nil {
			return
		}
		if _, err := TopicForMessage(message.Type); err != nil {
			t.Fatalf("decoded unsupported consensus message type %q", message.Type)
		}
		present := 0
		for _, set := range []bool{
			message.Proposal != nil,
			message.LaneProposal != nil,
			message.ProposalRequest != nil,
			message.Vote != nil,
			message.TimeoutVote != nil,
		} {
			if set {
				present++
			}
		}
		if present != 1 {
			t.Fatalf("a %q message must carry exactly one body, got %d", message.Type, present)
		}
		encoded, err := encodeWireMessage(message)
		if err != nil {
			t.Fatalf("a decoded %q message does not encode: %v", message.Type, err)
		}
		if !bytes.Equal(encoded, data) {
			t.Fatalf("a %q message decoded from %x encodes to %x", message.Type, data, encoded)
		}
	})
}

package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

// fillWireFields sets every field of value, recursively, to a distinct non-zero value, so a round
// trip through a codec that leaves a field out of its encoding changes the value. It fails the test
// on a kind it does not know, so a new field of another kind is not skipped silently.
func fillWireFields(tb testing.TB, value reflect.Value, counter *uint64) {
	tb.Helper()
	if !value.CanSet() {
		tb.Fatalf("fillWireFields cannot set a %s: a codec cannot carry an unexported field", value.Type())
	}
	switch value.Kind() {
	case reflect.Struct:
		for index := range value.NumField() {
			fillWireFields(tb, value.Field(index), counter)
		}
	case reflect.Array:
		for index := range value.Len() {
			fillWireFields(tb, value.Index(index), counter)
		}
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 2, 2))
		for index := range value.Len() {
			fillWireFields(tb, value.Index(index), counter)
		}
	case reflect.String:
		*counter++
		value.SetString(fmt.Sprintf("v%d", *counter))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*counter++
		value.SetUint(*counter)
	case reflect.Int64:
		*counter++
		value.SetInt(int64(*counter))
	case reflect.Bool:
		value.SetBool(true)
	default:
		tb.Fatalf("fillWireFields cannot fill a %s: teach it about %s", value.Type(), value.Kind())
	}
}

func filledWire[Value any](tb testing.TB) Value {
	tb.Helper()
	var value Value
	counter := uint64(0)
	fillWireFields(tb, reflect.ValueOf(&value).Elem(), &counter)
	return value
}

type wireCase struct {
	name    string
	message WireMessage
}

// wireCases returns one message of each type with every field set.
func wireCases(tb testing.TB) []wireCase {
	tb.Helper()
	proposal := filledWire[Proposal](tb)
	compact := filledWire[LaneProposal](tb)
	request := filledWire[ProposalRequest](tb)
	vote := filledWire[Vote](tb)
	timeoutVote := filledWire[TimeoutVote](tb)
	return []wireCase{
		{"proposal", WireMessage{Type: MessageProposal, Proposal: &proposal}},
		{"lane proposal", WireMessage{Type: MessageLaneProposal, LaneProposal: &compact}},
		{"proposal request", WireMessage{Type: MessageProposalRequest, ProposalRequest: &request}},
		{"vote", WireMessage{Type: MessageVote, Vote: &vote}},
		{"timeout vote", WireMessage{Type: MessageTimeoutVote, TimeoutVote: &timeoutVote}},
	}
}

// encodeWireMessage encodes a decoded message again with the encoder of its type.
func encodeWireMessage(message WireMessage) ([]byte, error) {
	switch message.Type {
	case MessageProposal:
		return EncodeProposal(*message.Proposal)
	case MessageLaneProposal:
		return EncodeLaneProposal(*message.LaneProposal)
	case MessageProposalRequest:
		return EncodeProposalRequest(*message.ProposalRequest)
	case MessageVote:
		return EncodeVote(*message.Vote)
	case MessageTimeoutVote:
		return EncodeTimeoutVote(*message.TimeoutVote)
	default:
		return nil, ErrUnknownConsensusMessage
	}
}

// A message that gains a field the codec does not carry fails here: the field is set to a non-zero
// value and the decoded message has its zero value.
func TestConsensusWireCodecRoundTripsMessages(t *testing.T) {
	for _, testCase := range wireCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			data, err := encodeWireMessage(testCase.message)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeWireMessage(data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, testCase.message) {
				t.Fatalf("a message changed in a round trip\n got: %+v\nwant: %+v", decoded, testCase.message)
			}
			again, err := encodeWireMessage(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again, data) {
				t.Fatal("a decoded message must encode to the bytes it came from")
			}
		})
	}
}

func TestConsensusWireCodecRoundTripsEmptyAndExtremeValues(t *testing.T) {
	maxID := types.ValidatorID(strings.Repeat("v", types.MaxIDBytes))
	cases := []WireMessage{
		{Type: MessageProposal, Proposal: &Proposal{}},
		{Type: MessageLaneProposal, LaneProposal: &LaneProposal{}},
		{Type: MessageProposalRequest, ProposalRequest: &ProposalRequest{}},
		{Type: MessageVote, Vote: &Vote{}},
		{Type: MessageTimeoutVote, TimeoutVote: &TimeoutVote{}},
		{Type: MessageVote, Vote: &Vote{Height: math.MaxUint64, Round: math.MaxUint64, ValidatorID: maxID, Signature: bytes.Repeat([]byte{1}, types.MaxSignatureBytes)}},
		{Type: MessageProposal, Proposal: &Proposal{Round: math.MaxUint64, Proposer: maxID, Block: types.Block{Header: types.Header{TimeUnixNano: math.MinInt64}}}},
		{Type: MessageLaneProposal, LaneProposal: &LaneProposal{Excluded: []uint32{0, 7, math.MaxUint32}}},
	}
	for index, message := range cases {
		data, err := encodeWireMessage(message)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeWireMessage(data)
		if err != nil {
			t.Fatalf("case %d: %v", index, err)
		}
		if !reflect.DeepEqual(decoded, message) {
			t.Fatalf("case %d: a message changed in a round trip\n got: %+v\nwant: %+v", index, decoded, message)
		}
	}
}

func TestConsensusWireRejectsMalformedMessages(t *testing.T) {
	cases := map[string][]byte{
		"nothing":                nil,
		"a version only":         {wireVersion},
		"json object":            []byte(`{}`),
		"json message":           []byte(`{"type":"proposal"}`),
		"json of an unknown one": []byte(`{"type":"unknown"}`),
		"text":                   []byte(`not-json`),
		"an unknown version":     {wireVersion + 1, wireVote},
		"version zero":           {0, wireVote},
		"type zero":              {wireVersion, 0},
		"an unknown type":        {wireVersion, wireTimeoutVote + 1},
	}
	for name, data := range cases {
		if _, err := DecodeWireMessage(data); err == nil {
			t.Fatalf("%s: expected a decode error for %q", name, data)
		}
	}

	if _, err := DecodeWireMessage([]byte{wireVersion + 1, wireVote}); !errors.Is(err, ErrUnknownConsensusMessage) {
		t.Fatalf("expected an unknown version to be an unknown message, got %v", err)
	}
	if _, err := DecodeWireMessage([]byte{wireVersion, wireTimeoutVote + 1}); !errors.Is(err, ErrUnknownConsensusMessage) {
		t.Fatalf("expected an unknown type to be an unknown message, got %v", err)
	}
	for _, data := range [][]byte{nil, {wireVersion}, {wireVersion, wireVote}} {
		if _, err := DecodeWireMessage(data); !errors.Is(err, ErrInvalidWireMessage) || !errors.Is(err, types.ErrInvalidEncoding) {
			t.Fatalf("expected a message cut short to be an invalid encoding, got %v", err)
		}
	}
}

// A message is complete or it is not a message: no prefix of an encoding decodes, and no byte may
// follow it.
func TestConsensusWireRejectsTruncatedAndExtendedMessages(t *testing.T) {
	for _, testCase := range wireCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			data, err := encodeWireMessage(testCase.message)
			if err != nil {
				t.Fatal(err)
			}
			for length := range len(data) {
				if _, err := DecodeWireMessage(data[:length]); err == nil {
					t.Fatalf("expected the first %d of %d bytes not to decode", length, len(data))
				}
			}
			for _, extra := range [][]byte{{0}, {1}, bytes.Repeat([]byte{0xff}, 16)} {
				extended := append(append([]byte(nil), data...), extra...)
				if _, err := DecodeWireMessage(extended); !errors.Is(err, ErrInvalidWireMessage) {
					t.Fatalf("expected trailing bytes %x to be refused, got %v", extra, err)
				}
			}
		})
	}
}

func TestConsensusWireRejectsNonCanonicalAndOutOfRangeFields(t *testing.T) {
	oversizedID := types.ValidatorID(strings.Repeat("v", types.MaxIDBytes+1))
	oversizedSignature := make(types.Signature, types.MaxSignatureBytes+1)
	noVote, err := EncodeVote(Vote{Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	// The encoding of a vote is the height and the round as uvarints behind the two header bytes.
	overlongHeight := append([]byte{wireVersion, wireVote, 0x81, 0x00}, noVote[3:]...)

	oversizedVote, err := EncodeVote(Vote{ValidatorID: oversizedID})
	if err != nil {
		t.Fatal(err)
	}
	oversizedVoteSignature, err := EncodeVote(Vote{Signature: oversizedSignature})
	if err != nil {
		t.Fatal(err)
	}
	oversizedProposer, err := EncodeProposal(Proposal{Proposer: oversizedID})
	if err != nil {
		t.Fatal(err)
	}
	// A block is judged by the transactions it carries, each charged its framing.
	overBudget, err := EncodeProposal(Proposal{Block: types.Block{Txs: []types.Tx{
		bytes.Repeat([]byte{1}, MaxBlockBytes/2),
		bytes.Repeat([]byte{2}, MaxBlockBytes/2),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	atBudget, err := EncodeProposal(Proposal{Block: types.Block{Txs: []types.Tx{bytes.Repeat([]byte{1}, MaxBlockBytes-types.TxOverheadBytes)}}})
	if err != nil {
		t.Fatal(err)
	}
	overBudgetExtra, err := EncodeLaneProposal(LaneProposal{Extra: []types.Tx{bytes.Repeat([]byte{1}, MaxBlockBytes)}})
	if err != nil {
		t.Fatal(err)
	}

	tooManyRefs, err := EncodeLaneProposal(LaneProposal{Batches: make([]LaneRef, MaxLaneRefs+1)})
	if err != nil {
		t.Fatal(err)
	}
	maxRefs, err := EncodeLaneProposal(LaneProposal{Batches: make([]LaneRef, MaxLaneRefs)})
	if err != nil {
		t.Fatal(err)
	}
	// A lane proposal that counts more batches than it holds is refused before anything is allocated
	// for them.
	emptyLane, err := EncodeLaneProposal(LaneProposal{})
	if err != nil {
		t.Fatal(err)
	}
	batchCountAt := len(emptyLane) - 3
	countedRefs := append(append([]byte(nil), emptyLane[:batchCountAt]...), binary.AppendUvarint(nil, MaxLaneRefs)...)
	countedRefs = append(countedRefs, 0, 0)
	hugeExclusion := append(append([]byte(nil), emptyLane[:batchCountAt+1]...), 1)
	hugeExclusion = binary.AppendUvarint(hugeExclusion, math.MaxUint32+1)
	hugeExclusion = append(hugeExclusion, 0)

	cases := []struct {
		name   string
		data   []byte
		reason string
	}{
		{"an overlong uvarint", overlongHeight, "uvarint is not minimal"},
		{"a validator id over the limit", oversizedVote, "byte string longer than the limit"},
		{"a signature over the limit", oversizedVoteSignature, "byte string longer than the limit"},
		{"a proposer over the limit", oversizedProposer, "byte string longer than the limit"},
		{"transactions over the block budget", overBudget, "transactions cost more than the block limit"},
		{"more batches than the limit", tooManyRefs, "list longer than the limit"},
		{"more batches than the input holds", countedRefs, "list longer than the input"},
		{"an excluded position over 32 bits", hugeExclusion, "excluded position over 32 bits"},
		{"extra transactions over the block budget", overBudgetExtra, "transactions cost more than the block limit"},
	}
	for _, testCase := range cases {
		_, err := DecodeWireMessage(testCase.data)
		if !errors.Is(err, ErrInvalidWireMessage) || !strings.Contains(err.Error(), testCase.reason) {
			t.Fatalf("%s: expected an invalid message because of %q, got %v", testCase.name, testCase.reason, err)
		}
	}

	for name, data := range map[string][]byte{"a block of the whole budget": atBudget, "the most batches": maxRefs} {
		if _, err := DecodeWireMessage(data); err != nil {
			t.Fatalf("%s: expected the limit itself to be accepted, got %v", name, err)
		}
	}
}

// A proposal, the largest message, is encoded into one allocation of the size the encoder estimated.
func TestEncodeProposalAllocatesOnce(t *testing.T) {
	proposal := wireBenchProposal(1000)
	if allocations := testing.AllocsPerRun(20, func() { _, _ = EncodeProposal(proposal) }); allocations > 1 {
		t.Fatalf("expected one allocation to encode a proposal, got %v", allocations)
	}
}

// wireBenchProposal is a proposal of txCount transactions of about a hundred bytes, justified by a
// certificate of a hundred validators after a timeout certificate of theirs: the size a busy
// network proposes.
func wireBenchProposal(txCount int) Proposal {
	ids := make([]types.ValidatorID, 100)
	for index := range ids {
		ids[index] = types.ValidatorID(fmt.Sprintf("validator-%03d", index))
	}
	signature := func(length int, seed byte) []byte {
		data := make([]byte, length)
		for index := range data {
			data[index] = seed + byte(index*7)
		}
		return data
	}
	hash := func(seed byte) types.Hash {
		var value types.Hash
		copy(value[:], signature(len(value), seed))
		return value
	}
	block := types.Block{Header: types.Header{
		ChainID:              "vexo-test",
		Height:               1_000_000,
		TimeUnixNano:         1_700_000_000_000_000_000,
		PreviousBlockHash:    hash(1),
		AppHash:              hash(2),
		ValidatorSetHash:     hash(3),
		NextValidatorSetHash: hash(3),
		ConsensusHash:        hash(4),
		TxRoot:               hash(5),
		Round:                1_000_007,
		JustifyRound:         1_000_006,
		ValidatorSetHeight:   999_990,
		StateHeight:          999_997,
		Benched:              []types.Bench{{Validator: ids[3], Until: 1_000_020}},
	}}
	for index := range txCount {
		block.Txs = append(block.Txs, types.Tx(fmt.Sprintf("bank:send:%08d:%s", index, strings.Repeat("x", 80))))
	}
	qc := func(round types.Round, seed byte) finality.QuorumCert {
		return finality.QuorumCert{
			Height:      999_999,
			Round:       round,
			BlockHash:   hash(seed),
			Signers:     finality.EncodeSigners(ids),
			Signature:   signature(64*len(ids), seed),
			VotingPower: types.VotingPower(len(ids)),
		}
	}
	justify := qc(1_000_005, 6)
	timeoutCert := finality.TimeoutCert{
		Height:  1_000_000,
		Round:   1_000_006,
		HighQC:  justify,
		Signers: finality.EncodeSigners(ids),
	}
	timeoutCert.Signature = encodeTimeoutSignatureGroups([]timeoutSignatureGroup{{
		Height:    1_000_000,
		HighQC:    justify,
		Signers:   ids,
		Signature: signature(64*len(ids), 9),
	}})
	return Proposal{Block: block, Round: 1_000_007, Proposer: ids[0], JustifyQC: justify, TimeoutCert: timeoutCert, Signature: signature(96, 11)}
}

// jsonWireEnvelope is the JSON envelope the consensus messages used before the binary encoding,
// kept only to measure what the binary encoding saves.
type jsonWireEnvelope struct {
	Type     MessageType `json:"type"`
	Proposal *Proposal   `json:"proposal,omitempty"`
}

func TestBinaryProposalIsSmallerThanJSON(t *testing.T) {
	proposal := wireBenchProposal(1000)
	binaryData, err := EncodeProposal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := json.Marshal(jsonWireEnvelope{Type: MessageProposal, Proposal: &proposal})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("a proposal of %d transactions: %d bytes in binary, %d bytes in JSON (%.0f%% of it)",
		len(proposal.Block.Txs), len(binaryData), len(jsonData), 100*float64(len(binaryData))/float64(len(jsonData)))
	if len(binaryData) >= len(jsonData) {
		t.Fatalf("expected the binary proposal of %d bytes to be smaller than the JSON one of %d", len(binaryData), len(jsonData))
	}
	decoded, err := DecodeWireMessage(binaryData)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*decoded.Proposal, proposal) {
		t.Fatal("the proposal changed in a round trip")
	}
}

func BenchmarkProposalWire(b *testing.B) {
	proposal := wireBenchProposal(1000)
	binaryData, err := EncodeProposal(proposal)
	if err != nil {
		b.Fatal(err)
	}
	jsonData, err := json.Marshal(jsonWireEnvelope{Type: MessageProposal, Proposal: &proposal})
	if err != nil {
		b.Fatal(err)
	}
	b.Run("binary/encode", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(binaryData)))
		for range b.N {
			if _, err := EncodeProposal(proposal); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("binary/decode", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(binaryData)))
		for range b.N {
			if _, err := DecodeWireMessage(binaryData); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("json/encode", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(jsonData)))
		for range b.N {
			if _, err := json.Marshal(jsonWireEnvelope{Type: MessageProposal, Proposal: &proposal}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("json/decode", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(jsonData)))
		for range b.N {
			var envelope jsonWireEnvelope
			if err := json.Unmarshal(jsonData, &envelope); err != nil {
				b.Fatal(err)
			}
		}
	})
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

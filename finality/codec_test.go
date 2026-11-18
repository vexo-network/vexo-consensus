package finality

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

// fillFields sets every field of value, recursively, to a distinct non-zero value, so a round trip
// through a codec that leaves a field out of its encoding changes the value. It fails the test on a
// kind it does not know, so a new field of another kind is not skipped silently.
func fillFields(t *testing.T, value reflect.Value, counter *uint64) {
	t.Helper()
	if !value.CanSet() {
		t.Fatalf("fillFields cannot set a %s: a codec cannot carry an unexported field", value.Type())
	}
	switch value.Kind() {
	case reflect.Struct:
		for index := range value.NumField() {
			fillFields(t, value.Field(index), counter)
		}
	case reflect.Array:
		for index := range value.Len() {
			fillFields(t, value.Index(index), counter)
		}
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 2, 2))
		for index := range value.Len() {
			fillFields(t, value.Index(index), counter)
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
		t.Fatalf("fillFields cannot fill a %s: teach it about %s", value.Type(), value.Kind())
	}
}

func filled[Value any](t *testing.T) Value {
	t.Helper()
	var value Value
	counter := uint64(0)
	fillFields(t, reflect.ValueOf(&value).Elem(), &counter)
	return value
}

// codecCase is a codec under test, written against the encoded form of any one of the types.
type codecCase struct {
	name   string
	filled func(t *testing.T) any
	encode func(value any) []byte
	decode func(data []byte) (any, error)
}

func codecCases() []codecCase {
	return []codecCase{
		{
			name:   "quorum certificate",
			filled: func(t *testing.T) any { return filled[QuorumCert](t) },
			encode: func(value any) []byte { return EncodeQuorumCert(value.(QuorumCert)) },
			decode: func(data []byte) (any, error) { return DecodeQuorumCert(data) },
		},
		{
			name:   "timeout certificate",
			filled: func(t *testing.T) any { return filled[TimeoutCert](t) },
			encode: func(value any) []byte { return EncodeTimeoutCert(value.(TimeoutCert)) },
			decode: func(data []byte) (any, error) { return DecodeTimeoutCert(data) },
		},
		{
			name:   "finality proof",
			filled: func(t *testing.T) any { return filled[Proof](t) },
			encode: func(value any) []byte { return EncodeProof(value.(Proof)) },
			decode: func(data []byte) (any, error) { return DecodeProof(data) },
		},
	}
}

// A certificate or a proof that gains a field the codec does not carry fails here: the field is set
// to a non-zero value and the decoded value has its zero value.
func TestCodecsCarryEveryField(t *testing.T) {
	for _, codec := range codecCases() {
		t.Run(codec.name, func(t *testing.T) {
			value := codec.filled(t)
			encoded := codec.encode(value)
			decoded, err := codec.decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value, decoded) {
				t.Fatalf("a value changed in a round trip\n got: %+v\nwant: %+v", decoded, value)
			}
			valueType := reflect.TypeOf(value)
			for index := range valueType.NumField() {
				if reflect.ValueOf(value).Field(index).IsZero() {
					t.Fatalf("fillFields left %s zero, which a round trip cannot show", valueType.Field(index).Name)
				}
			}
			if !bytes.Equal(codec.encode(decoded), encoded) {
				t.Fatal("a decoded value must encode to the bytes it came from")
			}
		})
	}
}

func TestCodecsRoundTripZeroAndExtremeValues(t *testing.T) {
	largest := ^uint64(0)
	qc := QuorumCert{Height: types.Height(largest), Round: types.Round(largest), BlockHash: types.Hash{0xff}, VotingPower: types.VotingPower(largest)}
	proof := Proof{
		Header:      types.Header{TimeUnixNano: -1},
		CommitChain: []CommitLink{{QuorumCert: qc}},
	}
	for _, value := range []any{QuorumCert{}, qc, TimeoutCert{}, TimeoutCert{Height: types.Height(largest), Round: types.Round(largest), HighQC: qc}, Proof{}, proof} {
		var encoded []byte
		var decoded any
		var err error
		switch typed := value.(type) {
		case QuorumCert:
			encoded = EncodeQuorumCert(typed)
			decoded, err = DecodeQuorumCert(encoded)
		case TimeoutCert:
			encoded = EncodeTimeoutCert(typed)
			decoded, err = DecodeTimeoutCert(encoded)
		case Proof:
			encoded = EncodeProof(typed)
			decoded, err = DecodeProof(encoded)
		}
		if err != nil || !reflect.DeepEqual(value, decoded) {
			t.Fatalf("%T changed in a round trip: %v\n got: %+v\nwant: %+v", value, err, decoded, value)
		}
	}
}

func TestCodecsRefuseEveryPrefixAndAnyTrailingBytes(t *testing.T) {
	for _, codec := range codecCases() {
		t.Run(codec.name, func(t *testing.T) {
			encoded := codec.encode(codec.filled(t))
			for length := range encoded {
				if _, err := codec.decode(encoded[:length]); !errors.Is(err, types.ErrInvalidEncoding) {
					t.Fatalf("a prefix of %d of %d bytes must be refused, got %v", length, len(encoded), err)
				}
			}
			if _, err := codec.decode(append(append([]byte(nil), encoded...), 0)); !errors.Is(err, types.ErrInvalidEncoding) || !strings.Contains(err.Error(), "trailing bytes") {
				t.Fatalf("trailing bytes must be refused, got %v", err)
			}
		})
	}
}

func TestCodecsRefuseValuesOverTheirLimits(t *testing.T) {
	cases := []struct {
		name   string
		decode func() error
		why    string
	}{
		{"signers over the limit", func() error {
			_, err := DecodeQuorumCert(EncodeQuorumCert(QuorumCert{Signers: make(types.Bitmap, MaxSignersBytes+1)}))
			return err
		}, "longer than the limit"},
		{"signature over the limit", func() error {
			_, err := DecodeQuorumCert(EncodeQuorumCert(QuorumCert{Signature: make(types.AggregateSignature, MaxAggregateSignatureBytes+1)}))
			return err
		}, "longer than the limit"},
		{"timeout signers over the limit", func() error {
			_, err := DecodeTimeoutCert(EncodeTimeoutCert(TimeoutCert{Signers: make(types.Bitmap, MaxSignersBytes+1)}))
			return err
		}, "longer than the limit"},
		{"timeout signature over the limit", func() error {
			_, err := DecodeTimeoutCert(EncodeTimeoutCert(TimeoutCert{Signature: make(types.AggregateSignature, MaxAggregateSignatureBytes+1)}))
			return err
		}, "longer than the limit"},
		{"commit chain over the limit", func() error {
			_, err := DecodeProof(EncodeProof(Proof{CommitChain: make([]CommitLink, MaxCommitLinks+1)}))
			return err
		}, "longer than the limit"},
		{"commit chain longer than the input", func() error {
			encoded := EncodeProof(Proof{})
			// The encoding ends with the count of links, then the validator set height and hash.
			countAt := len(encoded) - 1 - len(types.Hash{}) - 1
			encoded[countAt] = 3
			_, err := DecodeProof(encoded)
			return err
		}, "longer than the input"},
		{"height spelled with a padding byte", func() error {
			_, err := DecodeQuorumCert([]byte{0x81, 0x00})
			return err
		}, "not minimal"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.decode()
			if !errors.Is(err, types.ErrInvalidEncoding) || !strings.Contains(err.Error(), testCase.why) {
				t.Fatalf("expected a refusal because of %q, got %v", testCase.why, err)
			}
		})
	}
	longest := QuorumCert{Signers: make(types.Bitmap, MaxSignersBytes), Signature: make(types.AggregateSignature, MaxAggregateSignatureBytes)}
	if _, err := DecodeQuorumCert(EncodeQuorumCert(longest)); err != nil {
		t.Fatalf("a certificate at its limits is within them, got %v", err)
	}
	if _, err := DecodeProof(EncodeProof(Proof{CommitChain: make([]CommitLink, MaxCommitLinks)})); err != nil {
		t.Fatalf("a commit chain at its limit is within it, got %v", err)
	}
}

func TestDecodedCertificatesDoNotAliasTheirInput(t *testing.T) {
	qc := QuorumCert{Height: 1, Round: 2, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("aggregate"), VotingPower: 2}
	input := EncodeQuorumCert(qc)
	decoded, err := DecodeQuorumCert(input)
	if err != nil {
		t.Fatal(err)
	}
	for index := range input {
		input[index] = 0xff
	}
	if !reflect.DeepEqual(qc, decoded) {
		t.Fatalf("a decoded certificate must not alias its input, got %+v", decoded)
	}
}

func FuzzDecodeQuorumCert(f *testing.F) {
	f.Add(EncodeQuorumCert(QuorumCert{}))
	f.Add(EncodeQuorumCert(QuorumCert{Height: 4, Round: 7, BlockHash: types.Hash{1}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("aggregate"), VotingPower: 2}))
	f.Add([]byte{0x80, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		qc, err := DecodeQuorumCert(data)
		if err != nil {
			return
		}
		if !bytes.Equal(EncodeQuorumCert(qc), data) {
			t.Fatalf("a decoded certificate must encode to the bytes it came from\n got: %x\nwant: %x", EncodeQuorumCert(qc), data)
		}
	})
}

func FuzzDecodeTimeoutCert(f *testing.F) {
	f.Add(EncodeTimeoutCert(TimeoutCert{}))
	f.Add(EncodeTimeoutCert(TimeoutCert{Height: 4, Round: 8, HighQC: QuorumCert{Height: 3, Round: 7, Signers: types.Bitmap("alice")}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("groups")}))
	f.Fuzz(func(t *testing.T, data []byte) {
		timeoutCert, err := DecodeTimeoutCert(data)
		if err != nil {
			return
		}
		if !bytes.Equal(EncodeTimeoutCert(timeoutCert), data) {
			t.Fatalf("a decoded certificate must encode to the bytes it came from\n got: %x\nwant: %x", EncodeTimeoutCert(timeoutCert), data)
		}
	})
}

func FuzzDecodeProof(f *testing.F) {
	header := types.Header{ChainID: "vexo-test", Height: 1, Round: 3, JustifyRound: 2, ValidatorSetHeight: 1, Benched: []types.Bench{{Validator: "alice", Until: 9}}}
	qc := QuorumCert{Height: 1, Round: 3, Signers: types.Bitmap("alice"), Signature: types.AggregateSignature("aggregate"), VotingPower: 1}
	f.Add(EncodeProof(Proof{}))
	f.Add(EncodeProof(NewProof(header, qc).WithChild(types.Header{ChainID: "vexo-test", Height: 2, Round: 4, JustifyRound: 3}, QuorumCert{Height: 2, Round: 4})))
	f.Fuzz(func(t *testing.T, data []byte) {
		proof, err := DecodeProof(data)
		if err != nil {
			return
		}
		if !bytes.Equal(EncodeProof(proof), data) {
			t.Fatalf("a decoded proof must encode to the bytes it came from\n got: %x\nwant: %x", EncodeProof(proof), data)
		}
	})
}

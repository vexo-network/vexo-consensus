package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
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

func filledHeader(t *testing.T) Header {
	t.Helper()
	var header Header
	counter := uint64(0)
	fillFields(t, reflect.ValueOf(&header).Elem(), &counter)
	return header
}

func filledBlock(t *testing.T) Block {
	t.Helper()
	var block Block
	counter := uint64(0)
	fillFields(t, reflect.ValueOf(&block).Elem(), &counter)
	return block
}

// A header that gains a field the codec does not carry fails here: the field is set to a non-zero
// value and the decoded header has its zero value.
func TestHeaderCodecCoversEveryField(t *testing.T) {
	header := filledHeader(t)
	reader := NewReader(AppendHeader(nil, header))
	decoded := reader.Header()
	if err := reader.Done(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(header, decoded) {
		t.Fatalf("a header changed in a round trip\n got: %+v\nwant: %+v", decoded, header)
	}
	headerType := reflect.TypeOf(header)
	for index := range headerType.NumField() {
		if reflect.ValueOf(header).Field(index).IsZero() {
			t.Fatalf("fillFields left %s zero, which a round trip cannot show", headerType.Field(index).Name)
		}
	}
}

func TestBlockCodecRoundTrips(t *testing.T) {
	cases := []struct {
		name  string
		block Block
	}{
		{"every field set", filledBlock(t)},
		{"no transactions", Block{Header: filledHeader(t)}},
		{"zero block", Block{}},
		{"extreme numbers", Block{Header: Header{
			Height: math.MaxUint64, TimeUnixNano: math.MinInt64, Round: math.MaxUint64, JustifyRound: math.MaxUint64,
			ValidatorSetHeight: math.MaxUint64, StateHeight: math.MaxUint64, Benched: []Bench{{Validator: "v", Until: math.MaxUint64}},
		}}},
		{"negative time", Block{Header: Header{ChainID: "vexo-test", Height: 1, TimeUnixNano: -1}}},
		{"longest ids", Block{Header: Header{
			ChainID: strings.Repeat("c", MaxIDBytes),
			Benched: []Bench{{Validator: ValidatorID(strings.Repeat("v", MaxIDBytes)), Until: 1}},
		}}},
		{"transactions of many sizes", Block{Txs: []Tx{[]byte("a"), bytes.Repeat([]byte{7}, 300), bytes.Repeat([]byte{9}, 70000)}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			encoded := EncodeBlock(testCase.block)
			if len(encoded) != BlockSize(testCase.block) {
				t.Fatalf("BlockSize is %d for an encoding of %d bytes", BlockSize(testCase.block), len(encoded))
			}
			decoded, err := DecodeBlock(encoded, math.MaxInt32)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(testCase.block, decoded) {
				t.Fatalf("a block changed in a round trip\n got: %+v\nwant: %+v", decoded, testCase.block)
			}
			if !bytes.Equal(EncodeBlock(decoded), encoded) {
				t.Fatal("a decoded block must encode to the bytes it came from")
			}
		})
	}
}

func TestAppendBlockAppendsAfterThePrefix(t *testing.T) {
	block := filledBlock(t)
	prefix := []byte("prefix")
	appended := AppendBlock(append([]byte(nil), prefix...), block)
	if !bytes.HasPrefix(appended, prefix) || !bytes.Equal(appended[len(prefix):], EncodeBlock(block)) {
		t.Fatal("AppendBlock must append the encoding after what dst holds")
	}
}

func TestDecodeBlockNormalizesEmptyValues(t *testing.T) {
	block := Block{Header: Header{ChainID: "x", Benched: []Bench{}}, Txs: []Tx{}}
	decoded, err := DecodeBlock(EncodeBlock(block), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.Benched != nil || decoded.Txs != nil {
		t.Fatalf("empty lists decode to nil, got %#v", decoded)
	}
	withEmptyTxs, err := DecodeBlock(EncodeBlock(Block{Txs: []Tx{nil, []byte("a"), {}}}), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(withEmptyTxs.Txs) != 3 || len(withEmptyTxs.Txs[0]) != 0 || string(withEmptyTxs.Txs[1]) != "a" || len(withEmptyTxs.Txs[2]) != 0 {
		t.Fatalf("empty transactions must stay in place, got %#v", withEmptyTxs.Txs)
	}
}

// The transactions of a block share one allocation, so each is capped at its own length.
func TestDecodedBlockCopiesItsInputAndKeepsTransactionsApart(t *testing.T) {
	block := Block{
		Header: Header{ChainID: "chain", Benched: []Bench{{Validator: "benched", Until: 1}}},
		Txs:    []Tx{[]byte("first"), []byte("second"), []byte("third")},
	}
	input := EncodeBlock(block)
	decoded, err := DecodeBlock(input, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	grown := append(decoded.Txs[0], "-overflow"...)
	if string(grown) != "first-overflow" || string(decoded.Txs[1]) != "second" {
		t.Fatalf("appending to a transaction must not overwrite its neighbour, got %q and %q", grown, decoded.Txs[1])
	}
	for index := range input {
		input[index] = 0xff
	}
	if !reflect.DeepEqual(block, decoded) {
		t.Fatalf("a decoded block must not alias its input, got %+v", decoded)
	}
}

func TestDecodeBlockRefusesEveryPrefixOfAnEncoding(t *testing.T) {
	encoded := EncodeBlock(filledBlock(t))
	for length := range encoded {
		if _, err := DecodeBlock(encoded[:length], 1<<20); !errors.Is(err, ErrInvalidEncoding) {
			t.Fatalf("a prefix of %d of %d bytes must be refused, got %v", length, len(encoded), err)
		}
	}
	if _, err := DecodeBlock(encoded, 1<<20); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeBlockIsStrict(t *testing.T) {
	header := AppendHeader(nil, Header{ChainID: "vexo-test", Height: 1})
	// The header encoding ends with the count of benched validators, which is zero.
	beforeBenched := header[:len(header)-1]
	valid := append(append([]byte(nil), header...), 1, 6)
	valid = append(valid, "bank:a"...)
	validCost := TxCost(Tx("bank:a"))
	cases := []struct {
		name  string
		data  []byte
		limit int
		why   string
	}{
		{"empty input", nil, 1 << 20, "truncated input"},
		{"trailing byte", append(append([]byte(nil), valid...), 0), 1 << 20, "trailing bytes"},
		{"trailing block", append(append([]byte(nil), valid...), valid...), 1 << 20, "trailing bytes"},
		{"chain id longer than the input", append(binary.AppendUvarint(nil, 10), 'a', 'b'), 1 << 20, "longer than the input"},
		{"chain id over the limit", binary.AppendUvarint(nil, MaxIDBytes+1), 1 << 20, "longer than the limit"},
		{"height spelled with a padding byte", []byte{0, 0x81, 0x00}, 1 << 20, "not minimal"},
		{"uvarint over 64 bits", append([]byte{0}, bytes.Repeat([]byte{0xff}, 11)...), 1 << 20, "overflows"},
		{"benched list longer than the limit", append(append(append([]byte(nil), beforeBenched...), binary.AppendUvarint(nil, MaxBenched+1)...), make([]byte, 2*(MaxBenched+1))...), 1 << 20, "longer than the limit"},
		{"benched list longer than the input", append(append([]byte(nil), beforeBenched...), 5, 0, 0, 0, 0), 1 << 20, "longer than the input"},
		{"count of transactions longer than the input", append(append([]byte(nil), header...), 200, 1), 1 << 20, "longer than the input"},
		{"transaction longer than the input", append(append([]byte(nil), header...), 1, 50, 'a'), 1 << 20, "longer than the input"},
		{"more transactions than the limit allows", append(append([]byte(nil), header...), 4, 0, 0, 0, 0), 3 * TxOverheadBytes, "longer than the limit"},
		{"transaction over the limit", append(append([]byte(nil), header...), 1, 100, 0), 50, "longer than the limit"},
		{"transactions that cost more than the limit", valid, validCost - 1, "cost more than the block limit"},
		{"transactions that cost more together", EncodeBlock(Block{Txs: []Tx{[]byte("aaaa"), []byte("bbbb")}}), 2*TxCost(Tx("aaaa")) - 1, "cost more than the block limit"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DecodeBlock(testCase.data, testCase.limit)
			if !errors.Is(err, ErrInvalidEncoding) || !strings.Contains(err.Error(), testCase.why) {
				t.Fatalf("expected a refusal because of %q, got %v", testCase.why, err)
			}
		})
	}
	if _, err := DecodeBlock(valid, validCost); err != nil {
		t.Fatalf("a block that costs exactly the limit is within it, got %v", err)
	}
}

func TestReaderUvarintsAreMinimal(t *testing.T) {
	for _, value := range []uint64{0, 1, 127, 128, 16383, 16384, math.MaxUint32, math.MaxUint64} {
		reader := NewReader(binary.AppendUvarint(nil, value))
		if got := reader.Uvarint(); got != value || reader.Done() != nil {
			t.Fatalf("uvarint %d read back as %d: %v", value, got, reader.Err())
		}
	}
	spellings := []struct {
		name string
		data []byte
	}{
		{"zero in two bytes", []byte{0x80, 0x00}},
		{"one in two bytes", []byte{0x81, 0x00}},
		{"maximum in eleven bytes", append(bytes.Repeat([]byte{0xff}, 9), 0x81, 0x00)},
		{"tenth byte over one", append(bytes.Repeat([]byte{0xff}, 9), 0x02)},
		{"continuation at the end", []byte{0x80}},
	}
	for _, spelling := range spellings {
		reader := NewReader(spelling.data)
		reader.Uvarint()
		if !errors.Is(reader.Err(), ErrInvalidEncoding) {
			t.Fatalf("%s: expected the spelling to be refused", spelling.name)
		}
	}
}

func TestReaderFailureSticks(t *testing.T) {
	reader := NewReader([]byte{2, 'a'})
	if got := reader.String(10); got != "" || reader.Err() == nil {
		t.Fatalf("a string longer than the input must fail, got %q %v", got, reader.Err())
	}
	first := reader.Err()
	if reader.Byte() != 0 || reader.Uvarint() != 0 || reader.Int64() != 0 || reader.Hash() != (Hash{}) || reader.Bytes(10) != nil || reader.Count(10, 1) != 0 {
		t.Fatal("a failed reader must return zero values")
	}
	reader.Fail("later reason")
	if reader.Err() != first {
		t.Fatal("the first failure must stick")
	}
}

func TestReaderBoolIsStrict(t *testing.T) {
	for _, value := range []byte{0, 1} {
		reader := NewReader([]byte{value})
		if reader.Bool() != (value == 1) || reader.Done() != nil {
			t.Fatalf("byte %d must read as %t", value, value == 1)
		}
	}
	reader := NewReader([]byte{2})
	reader.Bool()
	if !errors.Is(reader.Err(), ErrInvalidEncoding) {
		t.Fatal("a boolean byte other than 0 and 1 must be refused")
	}
}

// A claim of the most benched validators or transactions with no bytes behind it is refused before
// anything is allocated for it.
func TestDecodeBlockRefusesAClaimWithoutAllocatingForIt(t *testing.T) {
	header := AppendHeader(nil, Header{})
	claims := []struct {
		name string
		data []byte
	}{
		{"benched validators", binary.AppendUvarint(append([]byte(nil), header[:len(header)-1]...), MaxBenched)},
		{"transactions", binary.AppendUvarint(append([]byte(nil), header...), 2<<20/TxOverheadBytes)},
	}
	for _, claim := range claims {
		allocated := testing.AllocsPerRun(10, func() {
			if _, err := DecodeBlock(claim.data, 2<<20); err == nil {
				t.Fatalf("%s: a claim without the bytes must be refused", claim.name)
			}
		})
		if allocated > 8 {
			t.Fatalf("%s: refusing the claim took %.0f allocations", claim.name, allocated)
		}
	}
}

func FuzzDecodeBlock(f *testing.F) {
	f.Add(EncodeBlock(Block{}), 1<<20)
	f.Add(EncodeBlock(Block{Header: Header{ChainID: "vexo-test", Height: 3, Benched: []Bench{{Validator: "alice", Until: 9}}}, Txs: []Tx{[]byte("bank:send:a"), []byte("bank:send:b")}}), 1<<20)
	f.Add(EncodeBlock(Block{Txs: []Tx{[]byte("bank:send:a"), []byte("bank:send:b")}}), 2*TxCost(Tx("bank:send:a")))
	f.Add([]byte{}, 0)
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}, 1<<20)
	f.Fuzz(func(t *testing.T, data []byte, limit int) {
		limit = max(limit, 0)
		block, err := DecodeBlock(data, limit)
		if err != nil {
			if !errors.Is(err, ErrInvalidEncoding) {
				t.Fatalf("a refusal must wrap ErrInvalidEncoding, got %v", err)
			}
			return
		}
		cost := 0
		for _, tx := range block.Txs {
			cost += TxCost(tx)
		}
		if cost > limit {
			t.Fatalf("a block of cost %d decoded under a limit of %d", cost, limit)
		}
		if !bytes.Equal(EncodeBlock(block), data) {
			t.Fatalf("a decoded block must encode to the bytes it came from\n got: %x\nwant: %x", EncodeBlock(block), data)
		}
	})
}

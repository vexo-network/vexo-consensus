package types

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"slices"
)

// The binary encoding of the types of this package, and the primitives the other packages build
// their own encodings from, is canonical: a value has exactly one encoding and a Reader accepts no
// other. Unsigned integers are minimal uvarints, a byte string or a list is its uvarint length or
// count followed by its elements, a hash is its 32 bytes and the block time is 8 bytes, big-endian.
// Fields follow the order of the struct that declares them. Every length and count a Reader reads
// is checked against a limit and against the input that is left before anything is allocated for
// it, so decoding bytes from a peer costs memory in proportion to those bytes.

// ErrInvalidEncoding is wrapped by every error a Reader reports: the bytes are not the canonical
// encoding of the value they were read as.
var ErrInvalidEncoding = errors.New("invalid binary encoding")

const (
	// MaxIDBytes bounds a chain id or a validator id.
	MaxIDBytes = 256
	// MaxSignatureBytes bounds a signature.
	MaxSignatureBytes = 1 << 16
	// MaxBenched bounds the validators a header benches.
	MaxBenched = 1 << 14
)

// AppendBytes appends value behind its uvarint length.
func AppendBytes[Value ~[]byte | ~string](dst []byte, value Value) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

// uvarintSize returns the length of the minimal uvarint of value.
func uvarintSize(value uint64) int {
	return (bits.Len64(value|1) + 6) / 7
}

// Reader reads the canonical binary encoding. The first failure sticks: every read after it returns
// the zero value, so a decoder reads a whole structure and asks for the failure once, with Done.
type Reader struct {
	data []byte
	err  error
}

// NewReader returns a Reader of data. It reads data in place and copies what it returns.
func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

// Fail fails the reader with reason unless it failed already. A decoder calls it for a value its
// format rules out.
func (reader *Reader) Fail(reason string) {
	if reader.err == nil {
		reader.err = fmt.Errorf("%w: %s", ErrInvalidEncoding, reason)
	}
}

// Err returns the first failure.
func (reader *Reader) Err() error {
	return reader.err
}

// Done returns the first failure, or an error when bytes remain: the value read must be all of the
// input.
func (reader *Reader) Done() error {
	if len(reader.data) != 0 {
		reader.Fail("trailing bytes")
	}
	return reader.err
}

// Len returns the bytes that are left.
func (reader *Reader) Len() int {
	return len(reader.data)
}

// Byte reads one byte.
func (reader *Reader) Byte() byte {
	if reader.err != nil {
		return 0
	}
	if len(reader.data) == 0 {
		reader.Fail("truncated input")
		return 0
	}
	value := reader.data[0]
	reader.data = reader.data[1:]
	return value
}

// Bool reads a byte that is 0 or 1.
func (reader *Reader) Bool() bool {
	switch reader.Byte() {
	case 0:
		return false
	case 1:
		return true
	default:
		reader.Fail("boolean is neither 0 nor 1")
		return false
	}
}

// Uvarint reads a uvarint and refuses any spelling but the minimal one.
func (reader *Reader) Uvarint() uint64 {
	if reader.err != nil {
		return 0
	}
	value, size := binary.Uvarint(reader.data)
	switch {
	case size == 0:
		reader.Fail("truncated input")
	case size < 0:
		reader.Fail("uvarint overflows 64 bits")
	case size != uvarintSize(value):
		reader.Fail("uvarint is not minimal")
	default:
		reader.data = reader.data[size:]
		return value
	}
	return 0
}

// Int64 reads an int64 as 8 bytes, big-endian.
func (reader *Reader) Int64() int64 {
	if reader.err != nil {
		return 0
	}
	if len(reader.data) < 8 {
		reader.Fail("truncated input")
		return 0
	}
	value := binary.BigEndian.Uint64(reader.data)
	reader.data = reader.data[8:]
	return int64(value)
}

// Hash reads a hash.
func (reader *Reader) Hash() Hash {
	var hash Hash
	if reader.err != nil {
		return hash
	}
	if len(reader.data) < len(hash) {
		reader.Fail("truncated input")
		return hash
	}
	copy(hash[:], reader.data)
	reader.data = reader.data[len(hash):]
	return hash
}

// Count reads the number of elements of a list: at most limit, and no more than the input that is
// left can hold when an element takes at least minSize bytes (minSize is at least 1). The caller
// allocates the list only after this check.
func (reader *Reader) Count(limit int, minSize int) int {
	count := reader.Uvarint()
	if reader.err != nil {
		return 0
	}
	if count > uint64(max(limit, 0)) {
		reader.Fail("list longer than the limit")
		return 0
	}
	if count > uint64(len(reader.data)/minSize) {
		reader.Fail("list longer than the input")
		return 0
	}
	return int(count)
}

// take reads a length-prefixed byte string of at most limit bytes and returns it in place.
func (reader *Reader) take(limit int) []byte {
	length := reader.Uvarint()
	if reader.err != nil {
		return nil
	}
	if length > uint64(max(limit, 0)) {
		reader.Fail("byte string longer than the limit")
		return nil
	}
	if length > uint64(len(reader.data)) {
		reader.Fail("byte string longer than the input")
		return nil
	}
	// The capacity is capped so that appending to the result never writes into the input.
	value := reader.data[:length:length]
	reader.data = reader.data[length:]
	return value
}

// Bytes reads a byte string of at most limit bytes. It returns a copy, nil for the empty string.
func (reader *Reader) Bytes(limit int) []byte {
	value := reader.take(limit)
	if len(value) == 0 {
		return nil
	}
	return slices.Clone(value)
}

// String reads a string of at most limit bytes.
func (reader *Reader) String(limit int) string {
	return string(reader.take(limit))
}

// Header reads a header.
func (reader *Reader) Header() Header {
	var header Header
	header.ChainID = reader.String(MaxIDBytes)
	header.Height = Height(reader.Uvarint())
	header.TimeUnixNano = reader.Int64()
	header.PreviousBlockHash = reader.Hash()
	header.AppHash = reader.Hash()
	header.ValidatorSetHash = reader.Hash()
	header.NextValidatorSetHash = reader.Hash()
	header.ConsensusHash = reader.Hash()
	header.TxRoot = reader.Hash()
	header.Round = Round(reader.Uvarint())
	header.JustifyRound = Round(reader.Uvarint())
	header.ValidatorSetHeight = Height(reader.Uvarint())
	header.StateHeight = Height(reader.Uvarint())
	if count := reader.Count(MaxBenched, 2); count > 0 {
		header.Benched = make([]Bench, count)
		for index := range header.Benched {
			header.Benched[index].Validator = ValidatorID(reader.String(MaxIDBytes))
			header.Benched[index].Until = Round(reader.Uvarint())
		}
	}
	return header
}

// Txs reads a list of transactions that a block limit of maxCost charges at most maxCost, each
// transaction its TxCost. The transactions are copied into one allocation.
func (reader *Reader) Txs(maxCost int) []Tx {
	count := reader.Count(maxCost/TxOverheadBytes, 1)
	if count == 0 {
		return nil
	}
	// A first pass over a copy of the reader checks the list and sizes the allocation.
	probe := *reader
	total, cost := 0, 0
	for range count {
		tx := probe.take(maxCost)
		if probe.err != nil {
			reader.err = probe.err
			return nil
		}
		total += len(tx)
		if cost += TxCost(tx); cost > maxCost {
			reader.Fail("transactions cost more than the block limit")
			return nil
		}
	}
	txs := make([]Tx, count)
	storage := make([]byte, total)
	for index := range txs {
		tx := reader.take(maxCost)
		if len(tx) > 0 {
			// The capacity is capped so that appending to one transaction cannot overwrite the next.
			txs[index] = storage[:len(tx):len(tx)]
			copy(txs[index], tx)
			storage = storage[len(tx):]
		}
	}
	return txs
}

// Block reads a block whose transactions a limit of maxCost charges at most maxCost, each
// transaction its TxCost.
func (reader *Reader) Block(maxCost int) Block {
	var block Block
	block.Header = reader.Header()
	block.Txs = reader.Txs(maxCost)
	return block
}

// AppendHeader appends the encoding of header.
func AppendHeader(dst []byte, header Header) []byte {
	dst = AppendBytes(dst, header.ChainID)
	dst = binary.AppendUvarint(dst, uint64(header.Height))
	dst = binary.BigEndian.AppendUint64(dst, uint64(header.TimeUnixNano))
	dst = append(dst, header.PreviousBlockHash[:]...)
	dst = append(dst, header.AppHash[:]...)
	dst = append(dst, header.ValidatorSetHash[:]...)
	dst = append(dst, header.NextValidatorSetHash[:]...)
	dst = append(dst, header.ConsensusHash[:]...)
	dst = append(dst, header.TxRoot[:]...)
	dst = binary.AppendUvarint(dst, uint64(header.Round))
	dst = binary.AppendUvarint(dst, uint64(header.JustifyRound))
	dst = binary.AppendUvarint(dst, uint64(header.ValidatorSetHeight))
	dst = binary.AppendUvarint(dst, uint64(header.StateHeight))
	dst = binary.AppendUvarint(dst, uint64(len(header.Benched)))
	for _, bench := range header.Benched {
		dst = AppendBytes(dst, bench.Validator)
		dst = binary.AppendUvarint(dst, uint64(bench.Until))
	}
	return dst
}

// AppendTxs appends the encoding of a list of transactions.
func AppendTxs(dst []byte, txs []Tx) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(txs)))
	for _, tx := range txs {
		dst = AppendBytes(dst, tx)
	}
	return dst
}

// AppendBlock appends the encoding of block: its header and its transactions.
func AppendBlock(dst []byte, block Block) []byte {
	return AppendTxs(AppendHeader(dst, block.Header), block.Txs)
}

// BlockSize returns the length of the encoding of block.
func BlockSize(block Block) int {
	header := block.Header
	size := uvarintSize(uint64(len(header.ChainID))) + len(header.ChainID) +
		uvarintSize(uint64(header.Height)) + 8 + 6*len(Hash{}) +
		uvarintSize(uint64(header.Round)) + uvarintSize(uint64(header.JustifyRound)) +
		uvarintSize(uint64(header.ValidatorSetHeight)) + uvarintSize(uint64(header.StateHeight)) +
		uvarintSize(uint64(len(header.Benched)))
	for _, bench := range header.Benched {
		size += uvarintSize(uint64(len(bench.Validator))) + len(bench.Validator) + uvarintSize(uint64(bench.Until))
	}
	size += uvarintSize(uint64(len(block.Txs)))
	for _, tx := range block.Txs {
		size += uvarintSize(uint64(len(tx))) + len(tx)
	}
	return size
}

// EncodeBlock returns the encoding of block.
func EncodeBlock(block Block) []byte {
	return AppendBlock(make([]byte, 0, BlockSize(block)), block)
}

// DecodeBlock decodes a block that EncodeBlock encoded. It refuses anything else: a block that
// does not parse, that leaves bytes over, or whose transactions a limit of maxCost charges more
// than maxCost, each transaction its TxCost.
func DecodeBlock(data []byte, maxCost int) (Block, error) {
	reader := NewReader(data)
	block := reader.Block(maxCost)
	if err := reader.Done(); err != nil {
		return Block{}, err
	}
	return block, nil
}

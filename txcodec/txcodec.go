// Package txcodec is the single transaction payload parser shared by the
// application runtime and the mempool.
//
// Both layers MUST read the same bytes identically: a transaction whose tags
// parse differently in the mempool (ordering, replacement, fee ranking) and in
// the ante handler (signer, nonce, fee) lets an attacker steer one layer with
// the tags the other never looks at. The parser therefore rejects ambiguity
// instead of picking a winner: duplicate tag keys, empty tag keys and signed
// envelopes that do not decode are errors.
package txcodec

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/types"
)

// SignedPrefix is the magic that opens a signed transaction, the binary record
// described at Envelope.
//
// Its first byte, 0x89, is one no other transaction starts with, so IsSigned is
// a plain prefix test that neither takes another transaction for an envelope
// nor lets a canonical payload pass as one:
//
//   - A canonical payload opens with the ASCII name of its module, and a byte in
//     0x80-0xbf is never the first byte of UTF-8 text.
//   - An Ethereum transaction opens with its EIP-2718 type byte (below 0x80) or,
//     for a legacy transaction, with an RLP list prefix (0xc0 and above). The EVM
//     bridge does not submit those bytes as a transaction anyway: it carries them
//     as hex in the eth_raw tag of a canonical evm payload, and the ante handler
//     refuses an eth_ tag inside a signed payload, so an envelope cannot launder
//     one.
//
// A transaction that opens with the magic is an envelope or an error, never a
// plain payload. The second byte only narrows the reserved space and makes the
// envelope recognizable in a hex dump.
const SignedPrefix = "\x89V"

var signedPrefix = []byte(SignedPrefix)

// envelopeVersion is the version byte that follows SignedPrefix.
const envelopeVersion = 1

// Bounds of the variable-size envelope fields. They are generous for every
// signature scheme a node can verify (Ed25519 uses 32-byte keys and 64-byte
// signatures, BLS12-381 at most 96 bytes of each) and keep a hostile envelope
// from handing an arbitrary blob to a signature verifier.
const (
	MaxChainIDLength   = 128
	MaxPublicKeyLength = 128
	MaxSignatureLength = 256
)

// Names of the encodings a transaction travels in as text: the encoding of an
// RPC submit request and of vexod's --encoding flags. A signed transaction is
// binary, so only EncodingBase64 can carry it.
const (
	EncodingBase64 = "base64"
	EncodingPlain  = "plain"
)

// EthereumTagPrefix is the prefix shared by every Ethereum compatibility tag
// (eth_raw, eth_hash, eth_gas_price, ...). Those tags describe an Ethereum
// transaction and are only meaningful after the EVM module authenticated them.
const EthereumTagPrefix = "eth_"

// MaxAmountLength bounds the length of an amount tag value. The largest valid
// amount is a 256-bit integer (78 decimal digits) plus a denomination suffix,
// so longer values are never valid and must not reach big-integer parsing.
const MaxAmountLength = 128

var (
	ErrInvalidSignedTx    = errors.New("invalid signed transaction")
	ErrInvalidCanonicalTx = errors.New("invalid canonical transaction")
	ErrMissingTxModule    = errors.New("missing transaction module")
	ErrMissingTxAction    = errors.New("missing transaction action")

	errEnvelopeVersion  = fmt.Errorf("%w: unsupported envelope version", ErrInvalidSignedTx)
	errEnvelopeTrailing = fmt.Errorf("%w: bytes follow the payload", ErrInvalidSignedTx)
	errEnvelopeNested   = fmt.Errorf("%w: payload is itself a signed transaction", ErrInvalidSignedTx)
)

// Envelope is a signed transaction: the payload a signer authorized, the
// signer's public key and the signature over the chain id and the payload (see
// app.SignedTxSignBytes).
//
// A signed transaction is the binary record
//
//	SignedPrefix | version | chain id | public key | signature | payload
//
// where the version is one byte and each of the four fields is a uvarint length
// followed by that many bytes. The encoding is canonical, so an envelope has
// exactly one valid encoding and the bytes of a transaction, hence its hash,
// cannot be varied without changing what it says: lengths are minimal, the
// chain id is at most MaxChainIDLength bytes, the public key and signature are
// non-empty and at most MaxPublicKeyLength and MaxSignatureLength bytes, the
// payload is non-empty and not itself a signed transaction, and nothing follows
// the payload.
type Envelope struct {
	ChainID   string
	PublicKey []byte
	Signature []byte
	Payload   []byte
}

// envelopeField bounds one length-prefixed field of an envelope.
type envelopeField struct {
	name      string
	minLength uint64
	maxLength uint64
}

var (
	chainIDField   = envelopeField{name: "chain id", maxLength: MaxChainIDLength}
	publicKeyField = envelopeField{name: "public key", minLength: 1, maxLength: MaxPublicKeyLength}
	signatureField = envelopeField{name: "signature", minLength: 1, maxLength: MaxSignatureLength}
	payloadField   = envelopeField{name: "payload", minLength: 1, maxLength: math.MaxInt}
)

// check reports whether a field of the given length is within bounds.
func (field envelopeField) check(length uint64) error {
	if length < field.minLength || length > field.maxLength {
		return fmt.Errorf("%w: %s length %d is out of range", ErrInvalidSignedTx, field.name, length)
	}
	return nil
}

// read splits the field off the front of data. The returned field has its
// capacity capped, so appending to it can never write into the bytes after it.
func (field envelopeField) read(data []byte) (value []byte, rest []byte, err error) {
	length, prefix := binary.Uvarint(data)
	if prefix <= 0 || prefix != uvarintLen(length) {
		return nil, nil, fmt.Errorf("%w: %s has no canonical length prefix", ErrInvalidSignedTx, field.name)
	}
	data = data[prefix:]
	if err := field.check(length); err != nil {
		return nil, nil, err
	}
	if length > uint64(len(data)) {
		return nil, nil, fmt.Errorf("%w: %s is truncated", ErrInvalidSignedTx, field.name)
	}
	end := int(length)
	return data[:end:end], data[end:], nil
}

// uvarintLen is the length of the shortest uvarint encoding of value.
func uvarintLen(value uint64) int {
	return (bits.Len64(value|1) + 6) / 7
}

// envelopeFields are the fields of a signed transaction, all sub-slices of it.
type envelopeFields struct {
	chainID   []byte
	publicKey []byte
	signature []byte
	payload   []byte
}

// splitEnvelope slices a signed transaction into its fields without copying
// anything. Every deviation from the canonical encoding is an error wrapping
// ErrInvalidSignedTx.
func splitEnvelope(tx types.Tx) (envelopeFields, error) {
	if !IsSigned(tx) {
		return envelopeFields{}, ErrInvalidSignedTx
	}
	rest := []byte(tx)[len(signedPrefix):]
	if len(rest) == 0 || rest[0] != envelopeVersion {
		return envelopeFields{}, errEnvelopeVersion
	}
	rest = rest[1:]
	var (
		fields envelopeFields
		err    error
	)
	if fields.chainID, rest, err = chainIDField.read(rest); err != nil {
		return envelopeFields{}, err
	}
	if fields.publicKey, rest, err = publicKeyField.read(rest); err != nil {
		return envelopeFields{}, err
	}
	if fields.signature, rest, err = signatureField.read(rest); err != nil {
		return envelopeFields{}, err
	}
	if fields.payload, rest, err = payloadField.read(rest); err != nil {
		return envelopeFields{}, err
	}
	if len(rest) != 0 {
		return envelopeFields{}, errEnvelopeTrailing
	}
	if IsSigned(fields.payload) {
		return envelopeFields{}, errEnvelopeNested
	}
	return fields, nil
}

// Canonical is a parsed `module:action:arg...:key=value...` payload.
type Canonical struct {
	Module string            `json:"module"`
	Action string            `json:"action"`
	Args   []string          `json:"args,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// IsSigned reports whether tx opens with SignedPrefix, that is whether it
// claims to be a signed transaction. Whether it decodes is for DecodeEnvelope to
// say.
func IsSigned(tx types.Tx) bool {
	return bytes.HasPrefix(tx, signedPrefix)
}

// EncodeEnvelope encodes envelope as a signed transaction. It refuses exactly
// what DecodeEnvelope refuses, so an encoded transaction always decodes to the
// envelope it came from. The transaction does not share memory with envelope.
func EncodeEnvelope(envelope Envelope) (types.Tx, error) {
	if err := chainIDField.check(uint64(len(envelope.ChainID))); err != nil {
		return nil, err
	}
	if err := publicKeyField.check(uint64(len(envelope.PublicKey))); err != nil {
		return nil, err
	}
	if err := signatureField.check(uint64(len(envelope.Signature))); err != nil {
		return nil, err
	}
	if err := payloadField.check(uint64(len(envelope.Payload))); err != nil {
		return nil, err
	}
	if IsSigned(envelope.Payload) {
		return nil, errEnvelopeNested
	}
	size := len(signedPrefix) + 1
	for _, length := range []int{len(envelope.ChainID), len(envelope.PublicKey), len(envelope.Signature), len(envelope.Payload)} {
		size += uvarintLen(uint64(length)) + length
	}
	tx := make([]byte, 0, size)
	tx = append(tx, signedPrefix...)
	tx = append(tx, envelopeVersion)
	tx = binary.AppendUvarint(tx, uint64(len(envelope.ChainID)))
	tx = append(tx, envelope.ChainID...)
	tx = binary.AppendUvarint(tx, uint64(len(envelope.PublicKey)))
	tx = append(tx, envelope.PublicKey...)
	tx = binary.AppendUvarint(tx, uint64(len(envelope.Signature)))
	tx = append(tx, envelope.Signature...)
	tx = binary.AppendUvarint(tx, uint64(len(envelope.Payload)))
	tx = append(tx, envelope.Payload...)
	return tx, nil
}

// DecodeEnvelope decodes a signed transaction into its envelope and the payload
// the envelope carries, which is envelope.Payload. Decoding copies nothing but
// the chain id: the public key, signature and payload alias tx, so a caller must
// not modify them.
func DecodeEnvelope(tx types.Tx) (Envelope, types.Tx, error) {
	fields, err := splitEnvelope(tx)
	if err != nil {
		return Envelope{}, nil, err
	}
	envelope := Envelope{
		ChainID:   string(fields.chainID),
		PublicKey: fields.publicKey,
		Signature: fields.signature,
		Payload:   fields.payload,
	}
	return envelope, fields.payload, nil
}

// Unwrap returns the payload of a signed transaction, or tx itself for a plain
// payload. A transaction that claims to be signed but does not decode is an
// error; it is never reinterpreted as a plain payload. The payload of a signed
// transaction aliases tx and must not be modified.
func Unwrap(tx types.Tx) (types.Tx, error) {
	if !IsSigned(tx) {
		return tx, nil
	}
	fields, err := splitEnvelope(tx)
	if err != nil {
		return nil, err
	}
	return fields.payload, nil
}

// DecodeTxEncoding decodes text, a transaction carried in the named encoding:
// EncodingBase64 or EncodingPlain. It is the one decoder of every entry point
// that takes a transaction as text, so they all accept the same names. A
// transaction is never empty.
func DecodeTxEncoding(text string, encoding string) (types.Tx, error) {
	var tx []byte
	switch encoding {
	case EncodingBase64:
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 transaction: %w", err)
		}
		tx = decoded
	case EncodingPlain:
		tx = []byte(text)
	default:
		return nil, fmt.Errorf("unsupported transaction encoding %q", encoding)
	}
	if len(tx) == 0 {
		return nil, errors.New("transaction is empty")
	}
	return tx, nil
}

// Names of the tags every layer reads from a transaction.
const (
	TagSigner   = "signer"
	TagNonce    = "nonce"
	TagFee      = "fee"
	TagGas      = "gas"
	TagGasLimit = "gas_limit"
	TagPriority = "priority"

	// TagEthereumRaw and TagEthereumHash identify a transaction decoded from
	// a raw Ethereum transaction. TagEthereumBlobGas carries its blob gas.
	TagEthereumRaw     = EthereumTagPrefix + "raw"
	TagEthereumHash    = EthereumTagPrefix + "hash"
	TagEthereumBlobGas = EthereumTagPrefix + "blob_gas"
)

// Meta is the execution metadata of a transaction, parsed once from the same
// bytes every layer sees.
type Meta struct {
	// Signer is the signer tag; empty when the transaction carries none.
	Signer string
	// Nonce is the nonce tag; HasNonce reports whether it was present.
	Nonce    uint64
	HasNonce bool
	// Fee is the fee tag as an exact amount; zero when absent. It is never nil.
	Fee *big.Int
	// Gas is the gas tag, or the gas_limit tag when gas is absent.
	Gas uint64
	// Priority is the priority tag; zero when absent.
	Priority uint64
	// Tags holds every execution tag of the transaction.
	Tags map[string]string
}

// FeeSaturated returns Fee clamped to the uint64 range.
func (meta Meta) FeeSaturated() uint64 {
	if meta.Fee == nil {
		return 0
	}
	if !meta.Fee.IsUint64() {
		return math.MaxUint64
	}
	return meta.Fee.Uint64()
}

// ParseMeta parses the execution metadata of tx. Present-but-malformed
// metadata (an unparseable nonce, gas, priority or fee, an empty signer,
// conflicting gas and gas_limit tags, duplicate tags) is an error: a layer must
// never fall back to "tag absent" for a tag the sender did provide.
func ParseMeta(tx types.Tx) (Meta, error) {
	tags, err := Tags(tx)
	if err != nil {
		return Meta{}, err
	}
	meta := Meta{Fee: new(big.Int), Tags: tags}
	if signer, found := tags[TagSigner]; found {
		if signer == "" {
			return Meta{}, fmt.Errorf("%w: empty signer", ErrInvalidCanonicalTx)
		}
		meta.Signer = signer
	}
	if value, found := tags[TagNonce]; found {
		nonce, err := parseUint(TagNonce, value)
		if err != nil {
			return Meta{}, err
		}
		meta.Nonce = nonce
		meta.HasNonce = true
	}
	if value, found := tags[TagFee]; found {
		fee, err := ParseAmount(value)
		if err != nil {
			return Meta{}, fmt.Errorf("%w: invalid fee", ErrInvalidCanonicalTx)
		}
		meta.Fee = fee
	}
	gasValue, gasFound := tags[TagGas]
	limitValue, limitFound := tags[TagGasLimit]
	switch {
	case gasFound:
		gas, err := parseUint(TagGas, gasValue)
		if err != nil {
			return Meta{}, err
		}
		if limitFound {
			limit, err := parseUint(TagGasLimit, limitValue)
			if err != nil {
				return Meta{}, err
			}
			if limit != gas {
				return Meta{}, fmt.Errorf("%w: conflicting gas and gas_limit", ErrInvalidCanonicalTx)
			}
		}
		meta.Gas = gas
	case limitFound:
		limit, err := parseUint(TagGasLimit, limitValue)
		if err != nil {
			return Meta{}, err
		}
		meta.Gas = limit
	}
	if value, found := tags[TagPriority]; found {
		priority, err := parseUint(TagPriority, value)
		if err != nil {
			return Meta{}, err
		}
		meta.Priority = priority
	}
	return meta, nil
}

func parseUint(tag string, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid %s", ErrInvalidCanonicalTx, tag)
	}
	return parsed, nil
}

// ParseCanonical strictly parses tx (unwrapping a signed envelope first).
func ParseCanonical(tx types.Tx) (Canonical, error) {
	payload, err := Unwrap(tx)
	if err != nil {
		return Canonical{}, err
	}
	parts := strings.Split(string(payload), ":")
	if len(parts) < 2 || parts[0] == "" {
		return Canonical{}, ErrMissingTxModule
	}
	if parts[1] == "" {
		return Canonical{}, ErrMissingTxAction
	}
	args, tags, err := splitTail(parts[2:])
	if err != nil {
		return Canonical{}, err
	}
	return Canonical{Module: parts[0], Action: parts[1], Args: args, Tags: tags}, nil
}

// Tags returns the execution tags of tx. It accepts opaque payloads that carry
// no module or action (they simply have no tags) but rejects malformed tag
// grammar exactly like ParseCanonical, so every layer that asks for tags sees
// the same answer.
func Tags(tx types.Tx) (map[string]string, error) {
	payload, err := Unwrap(tx)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(payload, '=') < 0 {
		// Only key=value parts are tags, so a payload without '=' has none.
		return nil, nil
	}
	_, tail, found := strings.Cut(string(payload), ":")
	if !found {
		return nil, nil
	}
	if _, tail, found = strings.Cut(tail, ":"); !found {
		return nil, nil
	}
	var tags map[string]string
	for {
		part, rest, more := strings.Cut(tail, ":")
		if _, err := addTag(&tags, part); err != nil {
			return nil, err
		}
		if !more {
			return tags, nil
		}
		tail = rest
	}
}

func splitTail(parts []string) ([]string, map[string]string, error) {
	var args []string
	var tags map[string]string
	for _, part := range parts {
		tag, err := addTag(&tags, part)
		if err != nil {
			return nil, nil, err
		}
		if !tag {
			args = append(args, part)
		}
	}
	return args, tags, nil
}

// addTag records part in tags when it is a key=value tag, and reports whether it was one.
func addTag(tags *map[string]string, part string) (bool, error) {
	key, value, found := strings.Cut(part, "=")
	if !found {
		return false, nil
	}
	if key == "" {
		return true, fmt.Errorf("%w: empty tag key", ErrInvalidCanonicalTx)
	}
	if _, duplicate := (*tags)[key]; duplicate {
		return true, fmt.Errorf("%w: duplicate tag %q", ErrInvalidCanonicalTx, key)
	}
	if *tags == nil {
		*tags = make(map[string]string)
	}
	(*tags)[key] = value
	return true, nil
}

// ParseAmount parses a fee or value tag with the economics amount grammar.
// Values longer than MaxAmountLength are rejected before any big-integer work.
func ParseAmount(value string) (*big.Int, error) {
	if len(value) > MaxAmountLength {
		return nil, economics.ErrInvalidAmount
	}
	return economics.ParseAmountBig(value)
}

// NormalizeAddress returns the canonical identity of an address: 0x-prefixed
// hex addresses of at most 20 bytes are left-padded to 20 bytes and
// lowercased; every other address compares verbatim.
func NormalizeAddress(address string) string {
	if !strings.HasPrefix(address, "0x") {
		return address
	}
	clean := strings.TrimPrefix(address, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 20 {
		return address
	}
	padded := make([]byte, 20)
	copy(padded[20-len(decoded):], decoded)
	return "0x" + hex.EncodeToString(padded)
}

// HasEthereumTag reports whether tags contain any Ethereum compatibility tag.
func HasEthereumTag(tags map[string]string) bool {
	for key := range tags {
		if strings.HasPrefix(key, EthereumTagPrefix) {
			return true
		}
	}
	return false
}

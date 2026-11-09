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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/types"
)

// SignedPrefix marks a transaction wrapped in a signed envelope.
const SignedPrefix = "signed:"

var signedPrefix = []byte(SignedPrefix)

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
)

// Envelope is the JSON document carried by a signed transaction.
type Envelope struct {
	SchemaVersion string `json:"schema_version"`
	ChainID       string `json:"chain_id"`
	Payload       string `json:"payload"`
	PublicKey     string `json:"public_key"`
	Signature     string `json:"signature"`
}

// Canonical is a parsed `module:action:arg...:key=value...` payload.
type Canonical struct {
	Module string            `json:"module"`
	Action string            `json:"action"`
	Args   []string          `json:"args,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// IsSigned reports whether tx is wrapped in a signed envelope.
func IsSigned(tx types.Tx) bool {
	return bytes.HasPrefix(tx, signedPrefix)
}

// DecodeEnvelope decodes a signed transaction into its envelope and the
// payload the envelope carries.
func DecodeEnvelope(tx types.Tx) (Envelope, types.Tx, error) {
	if !IsSigned(tx) {
		return Envelope{}, nil, ErrInvalidSignedTx
	}
	encoded := tx[len(signedPrefix):]
	document := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	written, err := base64.StdEncoding.Decode(document, encoded)
	document = document[:written]
	if err != nil {
		return Envelope{}, nil, ErrInvalidSignedTx
	}
	var envelope Envelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		return Envelope{}, nil, ErrInvalidSignedTx
	}
	if envelope.SchemaVersion != "v1" || envelope.Payload == "" || envelope.PublicKey == "" || envelope.Signature == "" {
		return Envelope{}, nil, ErrInvalidSignedTx
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil || IsSigned(payload) {
		return Envelope{}, nil, ErrInvalidSignedTx
	}
	return envelope, types.Tx(payload), nil
}

// Unwrap returns the payload of a signed transaction, or tx itself for a plain
// payload. A transaction that claims to be signed but does not decode is an
// error; it is never reinterpreted as a plain payload.
func Unwrap(tx types.Tx) (types.Tx, error) {
	if !IsSigned(tx) {
		return tx, nil
	}
	_, payload, err := DecodeEnvelope(tx)
	return payload, err
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

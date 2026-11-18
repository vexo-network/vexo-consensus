package txcodec

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/vexo-network/vexo-consensus/types"
)

func envelopeOf(t *testing.T, envelope Envelope) types.Tx {
	t.Helper()
	tx, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func signedOf(t *testing.T, payload string) types.Tx {
	t.Helper()
	return envelopeOf(t, Envelope{
		ChainID:   "chain",
		PublicKey: []byte("key"),
		Signature: []byte("signature"),
		Payload:   []byte(payload),
	})
}

// rawEnvelope assembles a signed transaction byte by byte, without the checks
// EncodeEnvelope makes, so a test can build the encodings a decoder must refuse.
// The fields are the chain id, public key, signature and payload.
func rawEnvelope(version byte, fields ...[]byte) types.Tx {
	tx := append([]byte(SignedPrefix), version)
	for _, field := range fields {
		tx = binary.AppendUvarint(tx, uint64(len(field)))
		tx = append(tx, field...)
	}
	return tx
}

func TestTagsRejectAmbiguousGrammar(t *testing.T) {
	for name, payload := range map[string]string{
		"duplicate fee":      "bank:send:a:b:1:fee=1000:fee=0",
		"duplicate signer":   "bank:send:a:b:1:signer=alice:signer=bob",
		"duplicate nonce":    "bank:send:a:b:1:nonce=1:nonce=2",
		"duplicate eth tag":  "evm:call:eth_raw=00:eth_raw=01",
		"empty tag key":      "bank:send:a:b:1:=1",
		"duplicate by value": "bank:send:a:b:1:fee=1:fee=1",
	} {
		tx := types.Tx(payload)
		if _, err := Tags(tx); !errors.Is(err, ErrInvalidCanonicalTx) {
			t.Fatalf("%s: Tags must reject the payload, got %v", name, err)
		}
		if _, err := ParseCanonical(tx); !errors.Is(err, ErrInvalidCanonicalTx) {
			t.Fatalf("%s: ParseCanonical must reject the payload, got %v", name, err)
		}
		if _, err := ParseMeta(tx); !errors.Is(err, ErrInvalidCanonicalTx) {
			t.Fatalf("%s: ParseMeta must reject the payload, got %v", name, err)
		}
		signed := signedOf(t, payload)
		if _, err := ParseMeta(signed); !errors.Is(err, ErrInvalidCanonicalTx) {
			t.Fatalf("%s: a signed envelope must not launder the ambiguity, got %v", name, err)
		}
	}
}

func TestParseMetaRejectsPresentButMalformedMetadata(t *testing.T) {
	for name, payload := range map[string]string{
		"empty signer":      "bank:send:signer=",
		"non numeric nonce": "bank:send:signer=a:nonce=x",
		"negative nonce":    "bank:send:signer=a:nonce=-1",
		"overflow nonce":    "bank:send:signer=a:nonce=18446744073709551616",
		"empty fee":         "bank:send:signer=a:fee=",
		"negative fee":      "bank:send:signer=a:fee=-1",
		"overlong fee":      "bank:send:signer=a:fee=" + strings.Repeat("9", MaxAmountLength+1),
		"non numeric gas":   "bank:send:signer=a:gas=x",
		"non numeric limit": "bank:send:signer=a:gas_limit=x",
		"non numeric prio":  "bank:send:signer=a:priority=x",
		"conflicting gas":   "bank:send:signer=a:gas=5:gas_limit=6",
		"bad limit vs gas":  "bank:send:signer=a:gas=5:gas_limit=x",
	} {
		if _, err := ParseMeta(types.Tx(payload)); !errors.Is(err, ErrInvalidCanonicalTx) {
			t.Fatalf("%s: expected ErrInvalidCanonicalTx, got %v", name, err)
		}
	}
}

func TestParseMetaReadsTheSameAnswerFromPlainAndSignedPayloads(t *testing.T) {
	payload := "bank:send:a:b:1:signer=alice:nonce=7:fee=12avxo:gas=21000:priority=3:memo=x"
	plain, err := ParseMeta(types.Tx(payload))
	if err != nil {
		t.Fatal(err)
	}
	signed, err := ParseMeta(signedOf(t, payload))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain, signed) {
		t.Fatalf("plain and signed payloads must parse identically: %+v vs %+v", plain, signed)
	}
	if plain.Signer != "alice" || plain.Nonce != 7 || !plain.HasNonce || plain.Gas != 21000 || plain.Priority != 3 {
		t.Fatalf("unexpected metadata %+v", plain)
	}
	if !plain.Fee.IsUint64() || plain.Fee.Uint64() != 12 || plain.FeeSaturated() != 12 {
		t.Fatalf("unexpected fee %v", plain.Fee)
	}
	if plain.Tags["memo"] != "x" {
		t.Fatalf("custom tags must be preserved, got %v", plain.Tags)
	}
}

func TestParseMetaDefaultsAndGasLimit(t *testing.T) {
	meta, err := ParseMeta(types.Tx("bank:send:a:b:1"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Signer != "" || meta.HasNonce || meta.Gas != 0 || meta.Priority != 0 || meta.Fee == nil || meta.Fee.Sign() != 0 {
		t.Fatalf("an untagged payload must parse to zero metadata, got %+v", meta)
	}
	meta, err = ParseMeta(types.Tx("bank:send:gas_limit=900"))
	if err != nil || meta.Gas != 900 {
		t.Fatalf("gas_limit alone must set the gas, got %+v %v", meta, err)
	}
	meta, err = ParseMeta(types.Tx("bank:send:gas=900:gas_limit=900"))
	if err != nil || meta.Gas != 900 {
		t.Fatalf("matching gas and gas_limit must be accepted, got %+v %v", meta, err)
	}
	meta, err = ParseMeta(types.Tx("bank:send:nonce=0"))
	if err != nil || !meta.HasNonce || meta.Nonce != 0 {
		t.Fatalf("an explicit zero nonce must be distinguishable from no nonce, got %+v %v", meta, err)
	}
}

func TestFeeSaturatedClampsToUint64(t *testing.T) {
	meta, err := ParseMeta(types.Tx("bank:send:fee=18446744073709551616"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Fee.IsUint64() {
		t.Fatal("2^64 must not fit a uint64")
	}
	if meta.FeeSaturated() != math.MaxUint64 {
		t.Fatalf("expected saturation, got %d", meta.FeeSaturated())
	}
	if (Meta{}).FeeSaturated() != 0 {
		t.Fatal("a zero Meta carries no fee")
	}
}

func TestUnwrapNeverReinterpretsABrokenEnvelope(t *testing.T) {
	valid := signedOf(t, "bank:send:a:b:1")
	chain, key, signature, payload := []byte("chain"), []byte("key"), []byte("signature"), []byte("bank:send:a:b:1")
	// withFields returns a transaction that opens like an envelope, then claims the given length prefixes.
	withFields := func(prefixes ...byte) types.Tx {
		return append(append([]byte(SignedPrefix), envelopeVersion), prefixes...)
	}
	for _, broken := range []struct {
		name string
		tx   types.Tx
	}{
		{"magic only", types.Tx(SignedPrefix)},
		{"no fields", rawEnvelope(envelopeVersion)},
		{"unsupported version 0", rawEnvelope(0, chain, key, signature, payload)},
		{"unsupported version 2", rawEnvelope(2, chain, key, signature, payload)},
		{"missing public key", rawEnvelope(envelopeVersion, chain, nil, signature, payload)},
		{"missing signature", rawEnvelope(envelopeVersion, chain, key, nil, payload)},
		{"missing payload", rawEnvelope(envelopeVersion, chain, key, signature, nil)},
		{"no payload field", rawEnvelope(envelopeVersion, chain, key, signature)},
		{"chain id too long", rawEnvelope(envelopeVersion, make([]byte, MaxChainIDLength+1), key, signature, payload)},
		{"public key too long", rawEnvelope(envelopeVersion, chain, make([]byte, MaxPublicKeyLength+1), signature, payload)},
		{"signature too long", rawEnvelope(envelopeVersion, chain, key, make([]byte, MaxSignatureLength+1), payload)},
		{"payload length beyond the input", append(rawEnvelope(envelopeVersion, chain, key, signature), 0xff, 0xff, 0xff, 0x7f)},
		{"length prefix overflows 64 bits", withFields(0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01)},
		{"truncated", valid[:len(valid)-1]},
		{"trailing byte", append(bytes.Clone(valid), 0)},
		{"nested envelope", rawEnvelope(envelopeVersion, chain, key, signature, valid)},
	} {
		name, tx := broken.name, broken.tx
		if _, _, err := DecodeEnvelope(tx); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: DecodeEnvelope must fail, got %v", name, err)
		}
		if _, err := Unwrap(tx); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: Unwrap must fail, got %v", name, err)
		}
		if _, err := Tags(tx); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: Tags must fail, got %v", name, err)
		}
		if _, err := ParseCanonical(tx); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: ParseCanonical must fail, got %v", name, err)
		}
		if _, err := ParseMeta(tx); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: ParseMeta must fail, got %v", name, err)
		}
	}
	plain := types.Tx("bank:send:a:b:1")
	if unwrapped, err := Unwrap(plain); err != nil || string(unwrapped) != string(plain) {
		t.Fatalf("a plain payload unwraps to itself, got %q %v", unwrapped, err)
	}
	envelope, decoded, err := DecodeEnvelope(valid)
	if err != nil || string(decoded) != "bank:send:a:b:1" || envelope.ChainID != "chain" {
		t.Fatalf("unexpected envelope %+v payload %q err %v", envelope, decoded, err)
	}
	if _, _, err := DecodeEnvelope(plain); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("a plain payload is not an envelope, got %v", err)
	}
}

func TestParseCanonicalSplitsModuleActionArgsAndTags(t *testing.T) {
	canonical, err := ParseCanonical(signedOf(t, "bank:send:alice:bob:5:fee=2:signer=alice"))
	if err != nil {
		t.Fatal(err)
	}
	want := Canonical{
		Module: "bank",
		Action: "send",
		Args:   []string{"alice", "bob", "5"},
		Tags:   map[string]string{"fee": "2", "signer": "alice"},
	}
	if !reflect.DeepEqual(canonical, want) {
		t.Fatalf("unexpected canonical %+v", canonical)
	}
	if _, err := ParseCanonical(types.Tx("")); !errors.Is(err, ErrMissingTxModule) {
		t.Fatalf("an empty payload has no module, got %v", err)
	}
	if _, err := ParseCanonical(types.Tx(":send")); !errors.Is(err, ErrMissingTxModule) {
		t.Fatalf("an empty module is rejected, got %v", err)
	}
	if _, err := ParseCanonical(types.Tx("bank")); !errors.Is(err, ErrMissingTxModule) {
		t.Fatalf("a payload without an action has no module:action pair, got %v", err)
	}
	if _, err := ParseCanonical(types.Tx("bank:")); !errors.Is(err, ErrMissingTxAction) {
		t.Fatalf("an empty action is rejected, got %v", err)
	}
}

func TestTagsOfOpaquePayloadsAreEmptyButNotAmbiguous(t *testing.T) {
	for _, payload := range []string{"", "opaque", "module:action"} {
		tags, err := Tags(types.Tx(payload))
		if err != nil || len(tags) != 0 {
			t.Fatalf("%q: expected no tags, got %v %v", payload, tags, err)
		}
	}
}

func TestParseAmountBoundsInputBeforeBigIntegerWork(t *testing.T) {
	max := strings.Repeat("9", MaxAmountLength)
	if _, err := ParseAmount(max); err != nil {
		t.Fatalf("a value at the bound is parsed, got %v", err)
	}
	if _, err := ParseAmount(max + "9"); err == nil {
		t.Fatal("a value beyond the bound must be rejected")
	}
	if amount, err := ParseAmount("12avxo"); err != nil || amount.Uint64() != 12 {
		t.Fatalf("denominated amounts parse, got %v %v", amount, err)
	}
	if _, err := ParseAmount(""); err == nil {
		t.Fatal("an empty amount is invalid")
	}
}

func TestNormalizeAddressIsTheAccountIdentity(t *testing.T) {
	full := "0x00000000000000000000000000000000000000ab"
	for address, want := range map[string]string{
		"alice":                        "alice",
		"Alice":                        "Alice",
		"":                             "",
		"0xAB":                         full,
		"0xab":                         full,
		"0xabc":                        "0x0000000000000000000000000000000000000abc",
		"0x" + strings.Repeat("A", 40): "0x" + strings.Repeat("a", 40),
		"0xzz":                         "0xzz",
		"0x" + strings.Repeat("a", 42): "0x" + strings.Repeat("a", 42),
	} {
		if got := NormalizeAddress(address); got != want {
			t.Fatalf("NormalizeAddress(%q) = %q, want %q", address, got, want)
		}
	}
	if NormalizeAddress("0xAB") != NormalizeAddress(full) {
		t.Fatal("every spelling of one 20-byte address must share one identity")
	}
}

func TestHasEthereumTag(t *testing.T) {
	if HasEthereumTag(nil) || HasEthereumTag(map[string]string{"signer": "a", "fee": "1"}) {
		t.Fatal("ordinary tags are not Ethereum tags")
	}
	for _, key := range []string{TagEthereumRaw, TagEthereumHash, TagEthereumBlobGas, "eth_gas_price", "eth_anything"} {
		if !HasEthereumTag(map[string]string{key: "x"}) {
			t.Fatalf("%s must be recognized as an Ethereum tag", key)
		}
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	envelope := Envelope{
		ChainID:   "vexo-test",
		PublicKey: bytes.Repeat([]byte{0x11}, 32),
		Signature: bytes.Repeat([]byte{0x22}, 64),
		Payload:   []byte("bank:send:a:b:1:fee=1:gas=1:signer=a:nonce=1"),
	}
	original := Envelope{
		ChainID:   envelope.ChainID,
		PublicKey: bytes.Clone(envelope.PublicKey),
		Signature: bytes.Clone(envelope.Signature),
		Payload:   bytes.Clone(envelope.Payload),
	}
	tx, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !IsSigned(tx) {
		t.Fatalf("an encoded envelope must be recognized as signed: %x", tx)
	}
	decoded, payload, err := DecodeEnvelope(tx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, original) || !bytes.Equal(payload, original.Payload) {
		t.Fatalf("round trip changed the envelope: %+v vs %+v", decoded, original)
	}
	if unwrapped, err := Unwrap(tx); err != nil || !bytes.Equal(unwrapped, original.Payload) {
		t.Fatalf("Unwrap must return the payload, got %q %v", unwrapped, err)
	}

	// The transaction does not share memory with the envelope it was encoded from.
	snapshot := bytes.Clone(tx)
	envelope.PublicKey[0]++
	envelope.Signature[0]++
	envelope.Payload[0]++
	if !bytes.Equal(tx, snapshot) {
		t.Fatal("the encoded transaction aliases the envelope it came from")
	}

	// The codec does not judge the chain id; the verifier compares it with its own.
	bare := Envelope{PublicKey: []byte("k"), Signature: []byte("s"), Payload: []byte("p")}
	decoded, _, err = DecodeEnvelope(envelopeOf(t, bare))
	if err != nil || !reflect.DeepEqual(decoded, bare) {
		t.Fatalf("an envelope without a chain id must round trip, got %+v %v", decoded, err)
	}
}

// The layout is part of the consensus rules: it is pinned byte by byte.
func TestEnvelopeWireLayout(t *testing.T) {
	tx := envelopeOf(t, Envelope{ChainID: "c", PublicKey: []byte("k"), Signature: []byte("s"), Payload: []byte("p")})
	want := []byte{0x89, 'V', 0x01, 0x01, 'c', 0x01, 'k', 0x01, 's', 0x01, 'p'}
	if !bytes.Equal(tx, want) {
		t.Fatalf("unexpected layout %x, want %x", tx, want)
	}

	// A length takes one byte below 128 and grows by one byte per 7 bits: 300 is 0xac 0x02.
	long := bytes.Repeat([]byte("p"), 300)
	tx = envelopeOf(t, Envelope{PublicKey: []byte("k"), Signature: []byte("s"), Payload: long})
	want = append([]byte{0x89, 'V', 0x01, 0x00, 0x01, 'k', 0x01, 's', 0xac, 0x02}, long...)
	if !bytes.Equal(tx, want) {
		t.Fatalf("unexpected layout of a long payload %x, want %x", tx[:12], want[:12])
	}
}

// A signed transaction is in every block and every mempool log record: what it adds to its payload
// is the price of authentication. With Ed25519 that is the 96 bytes of key and signature, the magic and
// version, the chain id and one byte for each of the four lengths.
func TestEnvelopeOverheadForA100BytePayload(t *testing.T) {
	envelope := Envelope{
		ChainID:   "vexo-test",
		PublicKey: make([]byte, 32),
		Signature: make([]byte, 64),
		Payload:   bytes.Repeat([]byte("p"), 100),
	}
	tx := envelopeOf(t, envelope)
	overhead := len(tx) - len(envelope.Payload)
	t.Logf("envelope overhead for a %d byte payload: %d bytes (%d byte signed transaction)", len(envelope.Payload), overhead, len(tx))
	if want := len(SignedPrefix) + 1 + 4 + len(envelope.ChainID) + len(envelope.PublicKey) + len(envelope.Signature); overhead != want {
		t.Fatalf("the envelope adds %d bytes, want the fixed fields and the four one byte lengths: %d", overhead, want)
	}
}

// Two encodings of one envelope would be two transaction hashes for one signature.
func TestEnvelopeEncodingIsCanonical(t *testing.T) {
	envelope := Envelope{ChainID: "chain", PublicKey: []byte("key"), Signature: []byte("signature"), Payload: bytes.Repeat([]byte("p"), 200)}
	tx := envelopeOf(t, envelope)
	decoded, _, err := DecodeEnvelope(tx)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := EncodeEnvelope(decoded); err != nil || !bytes.Equal(again, tx) {
		t.Fatalf("decoding then encoding must reproduce the transaction, got %x %v", again, err)
	}

	// Spell the length of each field with one redundant continuation byte.
	offset := len(SignedPrefix) + 1
	for _, field := range []string{"chain id", "public key", "signature", "payload"} {
		length, size := binary.Uvarint(tx[offset:])
		respelled := bytes.Clone(tx[:offset])
		respelled = append(respelled, tx[offset:offset+size-1]...)
		respelled = append(respelled, tx[offset+size-1]|0x80, 0x00)
		respelled = append(respelled, tx[offset+size:]...)
		if _, _, err := DecodeEnvelope(respelled); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: a non-minimal length prefix must be rejected, got %v", field, err)
		}
		if _, err := Unwrap(respelled); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: Unwrap must reject a non-minimal length prefix, got %v", field, err)
		}
		offset += size + int(length)
	}
}

func TestEnvelopeRejectsTruncatedAndTrailingBytes(t *testing.T) {
	tx := envelopeOf(t, Envelope{
		ChainID:   "vexo-test",
		PublicKey: bytes.Repeat([]byte{1}, 32),
		Signature: bytes.Repeat([]byte{2}, 64),
		Payload:   bytes.Repeat([]byte("p"), 150),
	})
	if _, _, err := DecodeEnvelope(tx); err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(tx); cut++ {
		truncated := tx[:cut]
		if _, _, err := DecodeEnvelope(truncated); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("a transaction cut to %d of %d bytes must be rejected, got %v", cut, len(tx), err)
		}
		if IsSigned(truncated) {
			if _, err := Unwrap(truncated); !errors.Is(err, ErrInvalidSignedTx) {
				t.Fatalf("Unwrap must reject a transaction cut to %d of %d bytes, got %v", cut, len(tx), err)
			}
		}
	}
	for _, trailing := range [][]byte{{0}, {1}, []byte(SignedPrefix), bytes.Repeat([]byte("p"), 150)} {
		extended := append(bytes.Clone(tx), trailing...)
		if _, _, err := DecodeEnvelope(extended); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%d trailing bytes must be rejected, got %v", len(trailing), err)
		}
		if _, err := Unwrap(extended); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("Unwrap must reject %d trailing bytes, got %v", len(trailing), err)
		}
	}
}

func TestEnvelopeFieldBounds(t *testing.T) {
	atBounds := func() Envelope {
		return Envelope{
			ChainID:   strings.Repeat("c", MaxChainIDLength),
			PublicKey: bytes.Repeat([]byte{1}, MaxPublicKeyLength),
			Signature: bytes.Repeat([]byte{2}, MaxSignatureLength),
			Payload:   []byte("p"),
		}
	}
	tx := envelopeOf(t, atBounds())
	if decoded, _, err := DecodeEnvelope(tx); err != nil || !reflect.DeepEqual(decoded, atBounds()) {
		t.Fatalf("fields at their bounds must round trip, got %+v %v", decoded, err)
	}
	for _, refused := range []struct {
		name   string
		mutate func(*Envelope)
	}{
		{"chain id beyond the bound", func(e *Envelope) { e.ChainID += "c" }},
		{"public key beyond the bound", func(e *Envelope) { e.PublicKey = append(e.PublicKey, 1) }},
		{"signature beyond the bound", func(e *Envelope) { e.Signature = append(e.Signature, 1) }},
		{"empty public key", func(e *Envelope) { e.PublicKey = nil }},
		{"empty signature", func(e *Envelope) { e.Signature = nil }},
		{"empty payload", func(e *Envelope) { e.Payload = nil }},
		{"payload that is a signed transaction", func(e *Envelope) { e.Payload = tx }},
	} {
		envelope := atBounds()
		refused.mutate(&envelope)
		// The encoder refuses exactly what the decoder refuses.
		if _, err := EncodeEnvelope(envelope); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: EncodeEnvelope must refuse, got %v", refused.name, err)
		}
		raw := rawEnvelope(envelopeVersion, []byte(envelope.ChainID), envelope.PublicKey, envelope.Signature, envelope.Payload)
		if _, _, err := DecodeEnvelope(raw); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: DecodeEnvelope must refuse, got %v", refused.name, err)
		}
	}
}

// A decoded field points into the transaction, which costs no copy. The spare
// capacity behind it is not the field's to use: transactions are often cut from
// a larger buffer, and appending to a payload must not overwrite its neighbour.
func TestEnvelopeFieldsAliasTheTransactionButCannotGrowIntoIt(t *testing.T) {
	tx := signedOf(t, "bank:send:a:b:1")
	buffer := make([]byte, len(tx)+16)
	copy(buffer, tx)
	shared := types.Tx(buffer[:len(tx)])
	want := bytes.Clone(buffer)

	envelope, payload, err := DecodeEnvelope(shared)
	if err != nil {
		t.Fatal(err)
	}
	if &payload[0] != &shared[len(shared)-len(payload)] {
		t.Fatal("the payload must alias the transaction instead of copying it")
	}
	for name, field := range map[string][]byte{"public key": envelope.PublicKey, "signature": envelope.Signature, "payload": envelope.Payload} {
		if cap(field) != len(field) {
			t.Fatalf("%s: capacity %d beyond its length %d would let an append overwrite the next field", name, cap(field), len(field))
		}
		_ = append(field, 0xee)
	}
	if unwrapped, err := Unwrap(shared); err != nil || cap(unwrapped) != len(unwrapped) {
		t.Fatalf("Unwrap must cap the payload too, got cap %d len %d %v", cap(unwrapped), len(unwrapped), err)
	}
	if !bytes.Equal(buffer, want) {
		t.Fatalf("appending to a decoded field overwrote the buffer: %x vs %x", buffer, want)
	}
}

func TestIsSignedNeverClaimsAnotherKindOfTransaction(t *testing.T) {
	if first := SignedPrefix[0]; first < 0x80 || first >= 0xc0 {
		t.Fatalf("the magic must open with a byte that starts no EIP-2718 type, RLP list or UTF-8 text, got %#x", first)
	}
	if !IsSigned(types.Tx(SignedPrefix)) || IsSigned(types.Tx{SignedPrefix[0]}) {
		t.Fatal("a transaction is signed exactly when it opens with the whole magic")
	}

	// A canonical payload opens with the ASCII name of its module. The former text envelope is
	// just another opaque payload now: nothing decodes it.
	for _, payload := range []string{
		"",
		"opaque",
		"bank:send:alice:bob:1:fee=1:signer=alice:nonce=1",
		"evm:call:fee=1:gas=21000:signer=0xab:nonce=0:eth_raw=0x02f8:eth_hash=0xaa",
		"slashing:evidence:abc",
		"signed:eyJzY2hlbWFfdmVyc2lvbiI6InYxIn0=",
	} {
		tx := types.Tx(payload)
		if IsSigned(tx) {
			t.Fatalf("%q must not be taken for a signed transaction", payload)
		}
		if unwrapped, err := Unwrap(tx); err != nil || !bytes.Equal(unwrapped, tx) {
			t.Fatalf("%q must unwrap to itself, got %q %v", payload, unwrapped, err)
		}
		if _, _, err := DecodeEnvelope(tx); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%q must not decode as an envelope, got %v", payload, err)
		}
	}

	// An Ethereum transaction opens with its type byte or, when it is a legacy one, with an
	// RLP list prefix.
	to := gethcommon.HexToAddress("0x00000000000000000000000000000000000000ab")
	one := uint256.NewInt(1)
	for _, ethereum := range []struct {
		name  string
		inner gethtypes.TxData
	}{
		{"legacy", &gethtypes.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(1)}},
		{"access list", &gethtypes.AccessListTx{ChainID: big.NewInt(1), Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(1)}},
		{"dynamic fee", &gethtypes.DynamicFeeTx{ChainID: big.NewInt(1), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000, To: &to, Value: big.NewInt(1)}},
		{"blob", &gethtypes.BlobTx{ChainID: one, Nonce: 1, GasTipCap: one, GasFeeCap: one, Gas: 21000, To: to, Value: one, BlobFeeCap: one, BlobHashes: []gethcommon.Hash{{1}}}},
		{"set code", &gethtypes.SetCodeTx{ChainID: one, Nonce: 1, GasTipCap: one, GasFeeCap: one, Gas: 21000, To: to, Value: one, AuthList: []gethtypes.SetCodeAuthorization{{Address: to}}}},
	} {
		raw, err := gethtypes.NewTx(ethereum.inner).MarshalBinary()
		if err != nil {
			t.Fatalf("%s: %v", ethereum.name, err)
		}
		if first := raw[0]; first >= 0x80 && first < 0xc0 {
			t.Fatalf("%s: an Ethereum transaction opened with %#x, a byte the magic is meant to own", ethereum.name, first)
		}
		if IsSigned(raw) {
			t.Fatalf("%s: a raw Ethereum transaction must not be taken for a signed one: %x", ethereum.name, raw)
		}
	}
}

func TestDecodeTxEncodingCarriesSignedEnvelopes(t *testing.T) {
	signed := signedOf(t, "bank:send:a:b:1")
	text := base64.StdEncoding.EncodeToString(signed)
	if decoded, err := DecodeTxEncoding(text, EncodingBase64); err != nil || !bytes.Equal(decoded, signed) {
		t.Fatalf("base64 must carry a binary transaction, got %x %v", decoded, err)
	}
	if decoded, err := DecodeTxEncoding("bank:send:a:b:1", EncodingPlain); err != nil || string(decoded) != "bank:send:a:b:1" {
		t.Fatalf("plain must carry the text as it is, got %q %v", decoded, err)
	}
	for _, refused := range []struct {
		name     string
		text     string
		encoding string
	}{
		{"invalid base64", "not base64!", EncodingBase64},
		{"empty base64", "", EncodingBase64},
		{"empty plain", "", EncodingPlain},
		{"hex", "6261", "hex"},
		{"no encoding", text, ""},
		{"wrong case", text, "Base64"},
	} {
		if tx, err := DecodeTxEncoding(refused.text, refused.encoding); err == nil {
			t.Fatalf("%s: expected an error, decoded %q", refused.name, tx)
		}
	}
}

func FuzzDecodeEnvelope(f *testing.F) {
	valid, err := EncodeEnvelope(Envelope{ChainID: "chain", PublicKey: []byte("key"), Signature: []byte("signature"), Payload: []byte("bank:send:a:b:1")})
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{
		valid,
		valid[:len(valid)-1],
		append(bytes.Clone(valid), 0),
		[]byte(SignedPrefix),
		rawEnvelope(0, []byte("chain"), []byte("key"), []byte("signature"), []byte("p")),
		rawEnvelope(envelopeVersion, []byte("chain"), []byte("key"), []byte("signature"), valid),
		[]byte("signed:not-base64"),
		[]byte("bank:plain"),
		{},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tx := types.Tx(data)
		envelope, payload, err := DecodeEnvelope(tx)
		unwrapped, unwrapErr := Unwrap(tx)
		// No reader built on Unwrap may panic, whatever the bytes are.
		_, _ = Tags(tx)
		_, _ = ParseMeta(tx)
		_, _ = ParseCanonical(tx)

		if !IsSigned(tx) {
			if err == nil || unwrapErr != nil || !bytes.Equal(unwrapped, data) {
				t.Fatalf("a transaction without the magic is a plain payload: %v %v", err, unwrapErr)
			}
			return
		}
		if err != nil {
			if !errors.Is(err, ErrInvalidSignedTx) || !errors.Is(unwrapErr, ErrInvalidSignedTx) {
				t.Fatalf("a broken envelope must fail DecodeEnvelope and Unwrap alike: %v %v", err, unwrapErr)
			}
			return
		}
		if unwrapErr != nil || !bytes.Equal(unwrapped, payload) || !bytes.Equal(payload, envelope.Payload) {
			t.Fatalf("Unwrap disagrees with DecodeEnvelope: %q vs %q (%v)", unwrapped, payload, unwrapErr)
		}
		if len(envelope.ChainID) > MaxChainIDLength ||
			len(envelope.PublicKey) == 0 || len(envelope.PublicKey) > MaxPublicKeyLength ||
			len(envelope.Signature) == 0 || len(envelope.Signature) > MaxSignatureLength ||
			len(payload) == 0 || IsSigned(payload) {
			t.Fatalf("decoded an envelope that breaks its bounds: %+v", envelope)
		}
		if encoded, err := EncodeEnvelope(envelope); err != nil || !bytes.Equal(encoded, data) {
			t.Fatalf("the encoding is not canonical: %x re-encodes to %x (%v)", data, encoded, err)
		}
	})
}

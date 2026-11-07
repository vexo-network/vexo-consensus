package txcodec

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func envelopeOf(t *testing.T, envelope Envelope) types.Tx {
	t.Helper()
	document, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return types.Tx(SignedPrefix + base64.StdEncoding.EncodeToString(document))
}

func signedOf(t *testing.T, payload string) types.Tx {
	t.Helper()
	return envelopeOf(t, Envelope{
		SchemaVersion: "v1",
		ChainID:       "chain",
		Payload:       base64.StdEncoding.EncodeToString([]byte(payload)),
		PublicKey:     "key",
		Signature:     "signature",
	})
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
	inner := signedOf(t, "bank:send:a:b:1")
	for name, tx := range map[string]types.Tx{
		"not base64":      types.Tx("signed:@@@"),
		"not json":        types.Tx(SignedPrefix + base64.StdEncoding.EncodeToString([]byte("not json"))),
		"wrong schema":    envelopeOf(t, Envelope{SchemaVersion: "v2", Payload: "eA==", PublicKey: "k", Signature: "s"}),
		"missing payload": envelopeOf(t, Envelope{SchemaVersion: "v1", PublicKey: "k", Signature: "s"}),
		"missing key":     envelopeOf(t, Envelope{SchemaVersion: "v1", Payload: "eA==", Signature: "s"}),
		"missing sig":     envelopeOf(t, Envelope{SchemaVersion: "v1", Payload: "eA==", PublicKey: "k"}),
		"payload base64":  envelopeOf(t, Envelope{SchemaVersion: "v1", Payload: "@@@", PublicKey: "k", Signature: "s"}),
		"nested envelope": envelopeOf(t, Envelope{SchemaVersion: "v1", Payload: base64.StdEncoding.EncodeToString([]byte(inner)), PublicKey: "k", Signature: "s"}),
	} {
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
	envelope, payload, err := DecodeEnvelope(signedOf(t, "bank:send:a:b:1"))
	if err != nil || string(payload) != "bank:send:a:b:1" || envelope.ChainID != "chain" {
		t.Fatalf("unexpected envelope %+v payload %q err %v", envelope, payload, err)
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

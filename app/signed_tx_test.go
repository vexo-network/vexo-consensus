package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestSignedTxRoundTripAndVerify(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	signerAddress, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	payload := types.Tx("bank:send:" + string(signerAddress) + ":bob:1:fee=1:gas=1:signer=" + string(signerAddress) + ":nonce=1")
	signedTx, err := SignTx("vexo-test", payload, signer)
	if err != nil {
		t.Fatal(err)
	}
	if !IsSignedTx(signedTx) {
		t.Fatalf("expected signed tx prefix, got %x", signedTx)
	}
	if string(TxPayload(signedTx)) != string(payload) {
		t.Fatalf("unexpected payload: %s", TxPayload(signedTx))
	}
	if err := VerifySignedTx("vexo-test", signedTx, vexocrypto.Ed25519Signer{}); err != nil {
		t.Fatal(err)
	}
	envelope, decoded, err := DecodeSignedTx(signedTx)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.ChainID != "vexo-test" || !bytes.Equal(envelope.PublicKey, signer.PublicKey()) || !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded envelope does not match what was signed: %+v", envelope)
	}
	if again, err := txcodec.EncodeEnvelope(envelope); err != nil || !bytes.Equal(again, signedTx) {
		t.Fatalf("a signed tx must be the canonical encoding of its envelope, got %x %v", again, err)
	}
}

func TestSignedTxRejectsSignerAddressMismatch(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	_, err = SignTx("vexo-test", types.Tx("bank:send:alice:bob:1:signer=alice"), signer)
	if !errors.Is(err, ErrSignerAddressMismatch) {
		t.Fatalf("expected signer address mismatch, got %v", err)
	}
}

// The bytes a signature covers are the one thing the binary envelope must not change: they
// bind a signature to a chain and a payload, whatever carries the signature.
func TestSignedTxSignBytesBindChainAndPayload(t *testing.T) {
	want := "vexo:signed-tx:v1\nvexo-test\nbank:send:alice:bob:1"
	if got := SignedTxSignBytes("vexo-test", types.Tx("bank:send:alice:bob:1")); string(got) != want {
		t.Fatalf("unexpected sign bytes %q, want %q", got, want)
	}

	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	payload := types.Tx("bank:send:alice:bob:1")
	signedTx, err := SignTx("vexo-test", payload, signer)
	if err != nil {
		t.Fatal(err)
	}
	envelope, _, err := DecodeSignedTx(signedTx)
	if err != nil {
		t.Fatal(err)
	}
	if !(vexocrypto.Ed25519Signer{}).Verify(envelope.PublicKey, SignedTxSignBytes("vexo-test", payload), envelope.Signature) {
		t.Fatal("the signature must cover the sign bytes of the chain and the payload")
	}
}

func TestSignedTxRejectsWrongChainAndTamper(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	other, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	verifier := vexocrypto.Ed25519Signer{}
	signedTx, err := SignTx("vexo-test", []byte("bank:send:alice:bob:1"), signer)
	if err != nil {
		t.Fatal(err)
	}
	otherTx, err := SignTx("vexo-test", []byte("bank:send:alice:bob:2"), other)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignedTx("vexo-test", signedTx, verifier); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignedTx("other", signedTx, verifier); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("expected chain mismatch rejection, got %v", err)
	}
	foreign, _, err := DecodeSignedTx(otherTx)
	if err != nil {
		t.Fatal(err)
	}

	// Each field is replaced in turn, and the result is encoded honestly: the transaction is
	// well-formed, and only the signature can tell it is not what the signer signed.
	for _, tampered := range []struct {
		name   string
		mutate func(*SignedTxEnvelope)
	}{
		{"chain id", func(envelope *SignedTxEnvelope) { envelope.ChainID = "other" }},
		{"payload", func(envelope *SignedTxEnvelope) { envelope.Payload = []byte("bank:send:alice:bob:2") }},
		{"payload byte", func(envelope *SignedTxEnvelope) { envelope.Payload[len(envelope.Payload)-1] ^= 1 }},
		{"public key", func(envelope *SignedTxEnvelope) { envelope.PublicKey = foreign.PublicKey }},
		{"public key byte", func(envelope *SignedTxEnvelope) { envelope.PublicKey[0] ^= 1 }},
		{"short public key", func(envelope *SignedTxEnvelope) { envelope.PublicKey = envelope.PublicKey[:31] }},
		{"signature", func(envelope *SignedTxEnvelope) { envelope.Signature = foreign.Signature }},
		{"signature byte", func(envelope *SignedTxEnvelope) { envelope.Signature[0] ^= 1 }},
		{"short signature", func(envelope *SignedTxEnvelope) { envelope.Signature = envelope.Signature[:63] }},
	} {
		envelope, _, err := DecodeSignedTx(signedTx)
		if err != nil {
			t.Fatal(err)
		}
		// The decoded fields alias signedTx: they are copied before they are changed.
		envelope.PublicKey = bytes.Clone(envelope.PublicKey)
		envelope.Signature = bytes.Clone(envelope.Signature)
		envelope.Payload = bytes.Clone(envelope.Payload)
		tampered.mutate(&envelope)
		tx, err := txcodec.EncodeEnvelope(envelope)
		if err != nil {
			t.Fatalf("%s: %v", tampered.name, err)
		}
		for _, chainID := range []string{"vexo-test", "other"} {
			if err := VerifySignedTx(chainID, tx, verifier); err == nil {
				t.Fatalf("%s: a tampered transaction verified for chain %q", tampered.name, chainID)
			}
		}
	}

	// No single flipped bit anywhere in the transaction survives, whether it lands in the magic,
	// a length, or one of the four fields.
	for index := range signedTx {
		for bit := 0; bit < 8; bit++ {
			flipped := bytes.Clone(signedTx)
			flipped[index] ^= 1 << bit
			if err := VerifySignedTx("vexo-test", flipped, verifier); err == nil {
				t.Fatalf("bit %d of byte %d of %x was flipped and the transaction still verified", bit, index, signedTx)
			}
		}
	}
}

func TestSignedTxRejectsTruncatedAndTrailingBytes(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	verifier := vexocrypto.Ed25519Signer{}
	signedTx, err := SignTx("vexo-test", []byte("bank:send:alice:bob:1"), signer)
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(signedTx); cut++ {
		truncated := signedTx[:cut]
		if _, _, err := DecodeSignedTx(truncated); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("a transaction cut to %d of %d bytes must not decode, got %v", cut, len(signedTx), err)
		}
		if err := VerifySignedTx("vexo-test", truncated, verifier); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("a transaction cut to %d of %d bytes must not verify, got %v", cut, len(signedTx), err)
		}
	}
	for _, trailing := range [][]byte{{0}, []byte("x"), signedTx} {
		extended := append(bytes.Clone(signedTx), trailing...)
		if _, _, err := DecodeSignedTx(extended); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%d trailing bytes must not decode, got %v", len(trailing), err)
		}
		if err := VerifySignedTx("vexo-test", extended, verifier); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%d trailing bytes must not verify, got %v", len(trailing), err)
		}
	}
	if err := VerifySignedTx("vexo-test", signedTx, nil); err == nil {
		t.Fatal("a nil verifier must not accept a signed transaction")
	}
}

func TestSignTxRejectsWhatHasNoSignedForm(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	signedTx, err := SignTx("vexo-test", []byte("bank:send:alice:bob:1"), signer)
	if err != nil {
		t.Fatal(err)
	}
	for name, attempt := range map[string]struct {
		chainID string
		payload types.Tx
	}{
		"empty payload":           {"vexo-test", nil},
		"signed payload":          {"vexo-test", signedTx},
		"chain id beyond a bound": {strings.Repeat("c", txcodec.MaxChainIDLength+1), types.Tx("bank:send:alice:bob:1")},
	} {
		if tx, err := SignTx(attempt.chainID, attempt.payload, signer); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: expected an invalid signed transaction error, got %x %v", name, tx, err)
		}
	}
}

// A signed transaction is on the wire, in every block and in the mempool log: this is its cost.
func TestSignedTxEnvelopeOverhead(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "bank:send:alice:bob:1:"
	payload := types.Tx(prefix + strings.Repeat("x", 100-len(prefix)))
	if len(payload) != 100 {
		t.Fatalf("the payload must be 100 bytes, got %d", len(payload))
	}
	signedTx, err := SignTx("vexo-test", payload, signer)
	if err != nil {
		t.Fatal(err)
	}
	overhead := len(signedTx) - len(payload)
	t.Logf("signed tx envelope overhead for a %d byte payload: %d bytes (%d byte signed tx, Ed25519 key and signature are 96 of them)",
		len(payload), overhead, len(signedTx))
	if overhead > 128 {
		t.Fatalf("a signed tx may add 128 bytes to its payload at most, added %d", overhead)
	}
}

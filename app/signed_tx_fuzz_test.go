package app

import (
	"bytes"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

func FuzzDecodeSignedTx(f *testing.F) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		f.Fatal(err)
	}
	validTx, err := SignTx("vexo-test", []byte("bank:send alice bob 1"), signer)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{
		validTx,
		validTx[:len(validTx)-1],
		append(bytes.Clone(validTx), 0),
		[]byte(txcodec.SignedPrefix),
		[]byte(txcodec.SignedPrefix + "\x01"),
		[]byte("signed:not-base64"),
		[]byte("bank:plain"),
		{},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tx := types.Tx(data)
		envelope, payload, err := DecodeSignedTx(tx)
		// Verification is the other reader of an arbitrary signed transaction: it must reject
		// whatever it cannot decode and whatever the key did not sign, and never panic.
		verifyErr := VerifySignedTx("vexo-test", tx, vexocrypto.Ed25519Signer{})
		if err != nil {
			if verifyErr == nil {
				t.Fatal("verified a transaction that does not decode")
			}
			if IsSignedTx(tx) {
				_ = TxPayload(tx)
			}
			return
		}
		if len(payload) == 0 {
			t.Fatal("decoded empty signed tx payload")
		}
		if !bytes.Equal(envelope.Payload, payload) || !bytes.Equal(TxPayload(tx), payload) {
			t.Fatal("TxPayload does not match decoded payload")
		}
		if IsSignedTx(payload) {
			t.Fatal("decoded a signed transaction nested in a signed transaction")
		}
	})
}

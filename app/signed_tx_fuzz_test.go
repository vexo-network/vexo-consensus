package app

import (
	"bytes"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
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
		[]byte("signed:not-base64"),
		[]byte(`signed:eyJzY2hlbWFfdmVyc2lvbiI6InYxIn0=`),
		[]byte("bank:plain"),
		[]byte{},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		envelope, payload, err := DecodeSignedTx(types.Tx(data))
		if err != nil {
			if IsSignedTx(types.Tx(data)) {
				_ = TxPayload(types.Tx(data))
			}
			return
		}
		if envelope.SchemaVersion != "v1" {
			t.Fatalf("decoded unsupported signed tx schema %q", envelope.SchemaVersion)
		}
		if len(payload) == 0 {
			t.Fatal("decoded empty signed tx payload")
		}
		if !bytes.Equal(TxPayload(types.Tx(data)), payload) {
			t.Fatal("TxPayload does not match decoded payload")
		}
	})
}

package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
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
		t.Fatalf("expected signed tx prefix, got %s", signedTx)
	}
	if string(TxPayload(signedTx)) != string(payload) {
		t.Fatalf("unexpected payload: %s", TxPayload(signedTx))
	}
	if err := VerifySignedTx("vexo-test", signedTx, vexocrypto.Ed25519Signer{}); err != nil {
		t.Fatal(err)
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

func TestSignedTxRejectsWrongChainAndTamper(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	signedTx, err := SignTx("vexo-test", []byte("bank:send"), signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignedTx("other", signedTx, vexocrypto.Ed25519Signer{}); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("expected chain mismatch rejection, got %v", err)
	}
	tampered := types.Tx(strings.TrimSuffix(string(signedTx), "A") + "A")
	if string(tampered) == string(signedTx) {
		tampered = append(types.Tx(nil), signedTx...)
		tampered[len(tampered)-1] = 'B'
	}
	if err := VerifySignedTx("vexo-test", tampered, vexocrypto.Ed25519Signer{}); err == nil {
		t.Fatal("expected tampered signed tx rejection")
	}
}

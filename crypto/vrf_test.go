package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestDeterministicVRFProvesAndVerifies(t *testing.T) {
	publicKey := types.PublicKey("alice")
	vrf := NewDeterministicVRF(map[string][]byte{string(publicKey): []byte("secret")})

	output, proof, err := vrf.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !vrf.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected proof to verify")
	}
}

func TestDeterministicVRFRejectsWrongSeedOutputProofOrKey(t *testing.T) {
	publicKey := types.PublicKey("alice")
	vrf := NewDeterministicVRF(map[string][]byte{string(publicKey): []byte("secret")})

	output, proof, err := vrf.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if vrf.Verify(publicKey, []byte("wrong"), output, proof) {
		t.Fatal("wrong seed verified")
	}
	if vrf.Verify(publicKey, []byte("seed"), []byte("bad"), proof) {
		t.Fatal("wrong output verified")
	}
	if vrf.Verify(publicKey, []byte("seed"), output, []byte("bad")) {
		t.Fatal("wrong proof verified")
	}
	if vrf.Verify(types.PublicKey("unknown"), []byte("seed"), output, proof) {
		t.Fatal("unknown key verified")
	}
}

func TestDeterministicVRFRejectsUnknownProver(t *testing.T) {
	vrf := NewDeterministicVRF(nil)
	_, _, err := vrf.Prove(types.PublicKey("alice"), []byte("seed"))
	if !errors.Is(err, ErrInvalidVRFKey) {
		t.Fatalf("expected invalid vrf key, got %v", err)
	}
}

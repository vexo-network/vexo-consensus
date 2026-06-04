package crypto

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
)

func TestECVRFP256AdapterProvesAndVerifies(t *testing.T) {
	privateKey := []byte("01234567890123456789012345678901")
	publicKey, err := ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewECVRFP256Adapter(config.VRFConfig{
		Keys:        map[string][]byte{base64.StdEncoding.EncodeToString(publicKey): privateKey},
		AdapterName: VRFAdapterECVRFP256Name,
		AuditReport: "ecvrf-test-audit",
		KeySource:   "test-key-source",
	})
	if err != nil {
		t.Fatal(err)
	}
	output, proof, err := adapter.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !adapter.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected ECVRF proof to verify")
	}
	if adapter.Verify(publicKey, []byte("other-seed"), output, proof) {
		t.Fatal("expected wrong seed to fail")
	}
	proof[0] ^= 0xff
	if adapter.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected malformed proof to fail")
	}
}

func TestECVRFP256AdapterValidationAndRegistry(t *testing.T) {
	privateKey := []byte("01234567890123456789012345678901")
	publicKey, err := ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.VRFConfig{
		ProductionAdapter: true,
		AdapterName:       VRFAdapterECVRFP256Name,
		AuditReport:       "ecvrf-test-audit",
		KeySource:         "test-key-source",
		Keys:              map[string][]byte{base64.StdEncoding.EncodeToString(publicKey): privateKey},
	}
	vrf, err := NewVRF(cfg)
	if err != nil {
		t.Fatal(err)
	}
	output, proof, err := vrf.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !vrf.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected registered ECVRF adapter to verify")
	}
}

func TestECVRFP256AdapterRejectsBadKeys(t *testing.T) {
	_, err := NewECVRFP256Adapter(config.VRFConfig{Keys: map[string][]byte{"bad": {}}})
	if !errors.Is(err, ErrInvalidECVRFKey) {
		t.Fatalf("expected invalid ecvrf key, got %v", err)
	}
	adapter, err := NewECVRFP256Adapter(config.VRFConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := adapter.Prove([]byte("missing"), []byte("seed")); !errors.Is(err, ErrInvalidVRFKey) {
		t.Fatalf("expected invalid vrf key, got %v", err)
	}
}

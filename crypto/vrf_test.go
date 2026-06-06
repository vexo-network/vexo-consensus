package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
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

func TestNewVRFLoadsRegisteredProductionAdapter(t *testing.T) {
	RegisterVRFAdapter("global-test-vrf", func(cfg config.VRFConfig) (VRFAdapter, error) {
		return testVRFAdapter{name: "global-test-vrf", auditReport: cfg.AuditReport, keySource: cfg.KeySource}, nil
	})

	vrf, err := NewVRF(config.VRFConfig{
		ProductionAdapter: true,
		AdapterName:       "global-test-vrf",
		AuditReport:       "audit-2026",
		KeySource:         "kms",
	})
	if err != nil {
		t.Fatal(err)
	}
	output, proof, err := vrf.Prove(types.PublicKey("alice"), []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !vrf.Verify(types.PublicKey("alice"), []byte("seed"), output, proof) {
		t.Fatal("expected registered vrf proof to verify")
	}
}

func TestNewVRFUsesBuiltInProductionAdapterByDefault(t *testing.T) {
	privateKey := []byte("12345678901234567890123456789012")
	publicKey, err := ECVRFP256PublicKeyFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	vrf, err := NewVRF(config.VRFConfig{
		ProductionAdapter: true,
		AuditReport:       "audit-2026",
		KeySource:         "local-key",
		Keys:              map[string][]byte{string(publicKey): privateKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	output, proof, err := vrf.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !vrf.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected built-in production vrf proof to verify")
	}
}

func TestNewVRFRequiresRegisteredProductionAdapter(t *testing.T) {
	_, err := NewVRF(config.VRFConfig{ProductionAdapter: true, AdapterName: "missing-vrf"})
	if !errors.Is(err, ErrVRFBackendUnavailable) {
		t.Fatalf("expected missing production vrf adapter, got %v", err)
	}
}

type testVRFAdapter struct {
	name        string
	auditReport string
	keySource   string
}

func (adapter testVRFAdapter) Prove(publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error) {
	output = vrfOutput(append([]byte(publicKey), []byte(":vrf")...), seed)
	proof = append([]byte(nil), output...)
	return output, proof, nil
}

func (adapter testVRFAdapter) Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	expected := vrfOutput(append([]byte(publicKey), []byte(":vrf")...), seed)
	return string(expected) == string(output) && string(expected) == string(proof)
}

func (adapter testVRFAdapter) Metadata() VRFAdapterMetadata {
	return VRFAdapterMetadata{
		Name:                 adapter.name,
		Version:              "v1",
		Audited:              true,
		AuditReport:          adapter.auditReport,
		KeySource:            adapter.keySource,
		DomainSeparation:     true,
		ProofVerification:    true,
		DeterministicOutput:  true,
		MalformedInputFuzzed: true,
	}
}

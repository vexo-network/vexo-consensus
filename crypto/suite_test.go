package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestNewRuntimeSuiteDeterministic(t *testing.T) {
	suite, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendDeterministic})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier == nil {
		t.Fatal("expected finality verifier")
	}
}

func TestNewRuntimeSuiteEd25519(t *testing.T) {
	suite, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendEd25519})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier == nil {
		t.Fatal("expected finality verifier")
	}
}

func TestNewRuntimeSuiteBLSRequiresAuditMetadata(t *testing.T) {
	_, err := NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
	if !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected unaudited built-in bls adapter to be unsafe, got %v", err)
	}
}

func TestNewRuntimeSuiteBLSRejectsConfigOnlyAuditMetadata(t *testing.T) {
	_, err := NewRuntimeSuite(config.CryptoConfig{
		Backend:           config.CryptoBackendBLS,
		ProductionAdapter: true,
		AuditReport:       "external-audit-report-id",
		DependencyAudit:   "dependency-audit-id",
	})
	if !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected configuration metadata without audited adapter metadata to remain unsafe, got %v", err)
	}
}

func TestRuntimeSuiteRegistryAcceptsSafeBLSAdapter(t *testing.T) {
	adapter := testBLSAdapter{safe: true}
	suite, err := NewRuntimeSuiteRegistry().
		RegisterBLS(func() (BLSAdapter, error) { return adapter, nil }).
		NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier == nil || suite.ConsensusAggregator == nil {
		t.Fatal("expected bls runtime suite")
	}
}

func TestRuntimeSuiteRegistryWrapsBLSFinalityWithValidatedCredentials(t *testing.T) {
	suite, err := NewRuntimeSuiteRegistry().
		RegisterBLS(func() (BLSAdapter, error) { return testBLSAdapter{safe: true}, nil }).
		RegisterBLSValidatorCredentials([]BLSValidatorCredential{
			{ValidatorID: "alice", PublicKey: []byte("alice-bls"), ProofOfPossession: []byte("alice-pop")},
		}).
		NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
	if err != nil {
		t.Fatal(err)
	}
	if !suite.FinalityVerifier.VerifyAggregate([]types.PublicKey{[]byte("alice-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected registered bls key to verify")
	}
	if suite.FinalityVerifier.VerifyAggregate([]types.PublicKey{[]byte("mallory-bls")}, []byte("message"), []byte("aggregate")) {
		t.Fatal("expected unregistered bls key to fail")
	}
}

func TestRuntimeSuiteRegistryRejectsUnsafeBLSAdapter(t *testing.T) {
	_, err := NewRuntimeSuiteRegistry().
		RegisterBLS(func() (BLSAdapter, error) { return testBLSAdapter{}, nil }).
		NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
	if !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected unsafe bls adapter, got %v", err)
	}
}

func TestRuntimeSuiteLoadsRegisteredBLSAdapterByName(t *testing.T) {
	RegisterBLSAdapter("global-test-bls", func() (BLSAdapter, error) {
		return namedTestBLSAdapter{name: "global-test-bls"}, nil
	})

	suite, err := NewRuntimeSuite(config.CryptoConfig{
		Backend:     config.CryptoBackendBLS,
		AdapterName: "global-test-bls",
	})
	if err != nil {
		t.Fatal(err)
	}
	if suite.FinalityVerifier == nil || suite.ConsensusVerifier == nil {
		t.Fatal("expected registered bls adapter to be wired")
	}
}

func TestRuntimeSuiteRejectsRegisteredBLSAdapterNameMismatch(t *testing.T) {
	RegisterBLSAdapter("global-test-bls-mismatch", func() (BLSAdapter, error) {
		return namedTestBLSAdapter{name: "other-bls"}, nil
	})

	_, err := NewRuntimeSuite(config.CryptoConfig{
		Backend:     config.CryptoBackendBLS,
		AdapterName: "global-test-bls-mismatch",
	})
	if !errors.Is(err, ErrBLSAdapterUnsafe) {
		t.Fatalf("expected bls name mismatch to be unsafe, got %v", err)
	}
}

func TestNewRuntimeSuiteRejectsUnsupportedBackend(t *testing.T) {
	_, err := NewRuntimeSuite(config.CryptoConfig{Backend: "unknown"})
	if !errors.Is(err, ErrUnsupportedCryptoBackend) {
		t.Fatalf("expected unsupported backend, got %v", err)
	}
}

type testBLSAdapter struct {
	safe            bool
	rejectPublicKey bool
	rejectProof     bool
}

func (adapter testBLSAdapter) PublicKey() types.PublicKey {
	return types.PublicKey("bls-public")
}

func (adapter testBLSAdapter) Sign(message []byte) (types.Signature, error) {
	return types.Signature(append([]byte("bls:"), message...)), nil
}

func (adapter testBLSAdapter) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	return len(publicKey) > 0 && len(message) > 0 && len(signature) > 0
}

func (adapter testBLSAdapter) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	if len(signatures) == 0 {
		return nil, ErrEmptySignature
	}
	return types.AggregateSignature("bls-aggregate"), nil
}

func (adapter testBLSAdapter) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return len(publicKeys) > 0 && len(message) > 0 && len(signature) > 0
}

func (adapter testBLSAdapter) ValidatePublicKey(publicKey types.PublicKey) error {
	if adapter.rejectPublicKey {
		return ErrInvalidBLSPublicKey
	}
	if len(publicKey) == 0 {
		return ErrMissingRemotePublicKey
	}
	return nil
}

func (adapter testBLSAdapter) VerifyProofOfPossession(publicKey types.PublicKey, proof types.Signature) bool {
	if adapter.rejectProof {
		return false
	}
	return len(publicKey) > 0 && len(proof) > 0
}

func (adapter testBLSAdapter) Metadata() BLSAdapterMetadata {
	if !adapter.safe {
		return BLSAdapterMetadata{Name: "unsafe-bls"}
	}
	return BLSAdapterMetadata{
		Name:                  "test-bls",
		Version:               "v1",
		Audited:               true,
		AuditReport:           "audit-report-id",
		DependencyAudit:       "go-mod-audit-id",
		DomainSeparation:      true,
		PublicKeyValidation:   true,
		SubgroupChecks:        true,
		RogueKeyDefense:       true,
		DeterministicEncoding: true,
		MalformedInputFuzzed:  true,
		ProofOfPossession:     true,
	}
}

type namedTestBLSAdapter struct {
	testBLSAdapter
	name string
}

func (adapter namedTestBLSAdapter) Metadata() BLSAdapterMetadata {
	metadata := testBLSAdapter{safe: true}.Metadata()
	metadata.Name = adapter.name
	return metadata
}

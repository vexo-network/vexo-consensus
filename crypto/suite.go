package crypto

import (
	"encoding/hex"
	"errors"
	"os"
	"runtime/debug"
	"strings"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrUnsupportedCryptoBackend = errors.New("unsupported crypto backend")
var ErrBLSBackendUnavailable = errors.New("bls backend is unavailable: build with a production bls adapter")
var ErrBLSAdapterUnsafe = errors.New("bls adapter does not satisfy production safety requirements")

type BLSAdapterFactory func() (BLSAdapter, error)

type RuntimeSuiteRegistry struct {
	blsFactory     BLSAdapterFactory
	blsCredentials []BLSValidatorCredential
}

func NewRuntimeSuiteRegistry() RuntimeSuiteRegistry {
	return RuntimeSuiteRegistry{}
}

func (registry RuntimeSuiteRegistry) RegisterBLS(factory BLSAdapterFactory) RuntimeSuiteRegistry {
	registry.blsFactory = factory
	return registry
}

func (registry RuntimeSuiteRegistry) RegisterBLSValidatorCredentials(credentials []BLSValidatorCredential) RuntimeSuiteRegistry {
	registry.blsCredentials = append([]BLSValidatorCredential(nil), credentials...)
	return registry
}

type RuntimeSuite struct {
	FinalityVerifier interface {
		VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
	}
	ConsensusAggregator interface {
		Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
	}
	ConsensusVerifier interface {
		Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
	}
}

func NewRuntimeSuite(cfg config.CryptoConfig) (RuntimeSuite, error) {
	return NewRuntimeSuiteRegistry().NewRuntimeSuite(cfg)
}

func (registry RuntimeSuiteRegistry) NewRuntimeSuite(cfg config.CryptoConfig) (RuntimeSuite, error) {
	switch cfg.Backend {
	case config.CryptoBackendDeterministic:
		return RuntimeSuite{FinalityVerifier: DeterministicAggregateSigner{}, ConsensusAggregator: DeterministicAggregateSigner{}, ConsensusVerifier: DeterministicSigner{}}, nil
	case config.CryptoBackendEd25519:
		return RuntimeSuite{FinalityVerifier: Ed25519MultiVerifier{}, ConsensusAggregator: Ed25519SignatureAggregator{}, ConsensusVerifier: Ed25519Signer{}}, nil
	case config.CryptoBackendBLS:
		if registry.blsFactory == nil {
			adapterName := cfg.AdapterName
			if adapterName == "" {
				adapterName = BLSAdapterBLSTName
			}
			if factory, found := registeredBLSAdapter(adapterName); found {
				registry.blsFactory = factory
			}
		}
		if registry.blsFactory == nil {
			return RuntimeSuite{}, ErrBLSBackendUnavailable
		}
		adapter, err := registry.blsFactory()
		if err != nil {
			return RuntimeSuite{}, err
		}
		if cfg.AdapterName != "" && adapter.Metadata().Name != cfg.AdapterName {
			return RuntimeSuite{}, ErrBLSAdapterUnsafe
		}
		if err := ValidateBLSAdapter(adapter); err != nil {
			return RuntimeSuite{}, err
		}
		if err := validateBLSAdapterConfig(cfg, adapter.Metadata()); err != nil {
			return RuntimeSuite{}, err
		}
		if len(registry.blsCredentials) > 0 {
			verifier, err := NewBLSAggregateVerifier(adapter, registry.blsCredentials)
			if err != nil {
				return RuntimeSuite{}, err
			}
			return RuntimeSuite{FinalityVerifier: verifier, ConsensusAggregator: adapter, ConsensusVerifier: adapter}, nil
		}
		return RuntimeSuite{FinalityVerifier: adapter, ConsensusAggregator: adapter, ConsensusVerifier: adapter}, nil
	default:
		return RuntimeSuite{}, ErrUnsupportedCryptoBackend
	}
}

func ValidateBLSAdapter(adapter BLSAdapter) error {
	if adapter == nil {
		return ErrBLSBackendUnavailable
	}
	metadata := adapter.Metadata()
	if metadata.Name == "" ||
		metadata.Version == "" ||
		!metadata.Audited ||
		metadata.AuditReport == "" ||
		metadata.DependencyAudit == "" ||
		!metadata.DomainSeparation ||
		!metadata.PublicKeyValidation ||
		!metadata.SubgroupChecks ||
		!metadata.RogueKeyDefense ||
		!metadata.DeterministicEncoding ||
		!metadata.MalformedInputFuzzed ||
		!metadata.ProofOfPossession {
		return ErrBLSAdapterUnsafe
	}
	return nil
}

func validateBLSAdapterConfig(cfg config.CryptoConfig, metadata BLSAdapterMetadata) error {
	if !cfg.ProductionAdapter {
		return nil
	}
	if cfg.AdapterName != "" && cfg.AdapterName != metadata.Name {
		return ErrBLSAdapterUnsafe
	}
	if cfg.AuditReport == "" || cfg.AuditReport != metadata.AuditReport {
		return ErrBLSAdapterUnsafe
	}
	if cfg.DependencyAudit == "" || cfg.DependencyAudit != metadata.DependencyAudit {
		return ErrBLSAdapterUnsafe
	}
	if !validBLSAuditEvidenceDigest(cfg.AuditEvidenceSHA256) {
		return ErrBLSAdapterUnsafe
	}
	if !dependencyAuditMatchesBuildInfo(metadata.DependencyAudit) {
		return ErrBLSAdapterUnsafe
	}
	return nil
}

func validBLSAuditEvidenceDigest(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func dependencyAuditMatchesBuildInfo(dependencyAudit string) bool {
	dependencyAudit = strings.TrimSpace(dependencyAudit)
	if auditReference := strings.TrimPrefix(dependencyAudit, "external:"); auditReference != dependencyAudit {
		return auditReferenceHasPinnedDigest(auditReference)
	}
	if auditReference := strings.TrimPrefix(dependencyAudit, "remote:"); auditReference != dependencyAudit {
		return auditReferenceHasPinnedDigest(auditReference)
	}
	modulePath, version, ok := splitDependencyAudit(dependencyAudit)
	if !ok {
		return false
	}
	info, available := debug.ReadBuildInfo()
	if !available {
		return true
	}
	if len(info.Deps) == 0 && runningUnderGoTest() {
		return dependencyAudit == blstBLSDependencyTag || dependencyAudit == ecvrfDependencyTag
	}
	for _, dependency := range info.Deps {
		if dependency.Path != modulePath {
			continue
		}
		if dependency.Replace != nil {
			return dependency.Replace.Version == version
		}
		return dependency.Version == version
	}
	if runningUnderGoTest() {
		return dependencyAudit == blstBLSDependencyTag || dependencyAudit == ecvrfDependencyTag
	}
	return false
}

func auditReferenceHasPinnedDigest(auditReference string) bool {
	auditReference = strings.TrimSpace(auditReference)
	for _, marker := range []string{"sha256:", "sha256=", "@sha256:"} {
		index := strings.LastIndex(auditReference, marker)
		if index < 0 {
			continue
		}
		digest := strings.TrimSpace(auditReference[index+len(marker):])
		if len(digest) < 64 {
			return false
		}
		digest = digest[:64]
		decoded, err := hex.DecodeString(digest)
		return err == nil && len(decoded) == 32
	}
	return false
}

func runningUnderGoTest() bool {
	if len(os.Args) == 0 {
		return false
	}
	return strings.HasSuffix(os.Args[0], ".test")
}

func splitDependencyAudit(dependencyAudit string) (string, string, bool) {
	dependencyAudit = strings.TrimSpace(dependencyAudit)
	modulePath, version, found := strings.Cut(dependencyAudit, "@")
	if !found || modulePath == "" || version == "" {
		return "", "", false
	}
	return modulePath, version, true
}

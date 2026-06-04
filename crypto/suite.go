package crypto

import (
	"errors"

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
		if registry.blsFactory == nil && cfg.AdapterName != "" {
			if factory, found := registeredBLSAdapter(cfg.AdapterName); found {
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

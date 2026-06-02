package crypto

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrUnsupportedCryptoBackend = errors.New("unsupported crypto backend")

type RuntimeSuite struct {
	FinalityVerifier interface {
		VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
	}
	ConsensusAggregator interface {
		Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
	}
}

func NewRuntimeSuite(cfg config.CryptoConfig) (RuntimeSuite, error) {
	switch cfg.Backend {
	case config.CryptoBackendDeterministic:
		return RuntimeSuite{FinalityVerifier: DeterministicAggregateSigner{}, ConsensusAggregator: DeterministicAggregateSigner{}}, nil
	case config.CryptoBackendEd25519:
		return RuntimeSuite{FinalityVerifier: Ed25519MultiVerifier{}, ConsensusAggregator: Ed25519SignatureAggregator{}}, nil
	default:
		return RuntimeSuite{}, ErrUnsupportedCryptoBackend
	}
}

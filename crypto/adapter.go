package crypto

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/config"
)

var ErrUnsupportedSignerBackend = errors.New("unsupported signer backend")

type SignerFactory func() (Signer, error)

type SignerRegistry struct {
	factories map[config.CryptoBackend]SignerFactory
}

func NewSignerRegistry() SignerRegistry {
	return SignerRegistry{factories: map[config.CryptoBackend]SignerFactory{
		config.CryptoBackendEd25519: func() (Signer, error) {
			return GenerateEd25519Signer()
		},
		config.CryptoBackendBLS: func() (Signer, error) {
			return nil, ErrBLSBackendUnavailable
		},
	}}
}

func (registry SignerRegistry) Register(backend config.CryptoBackend, factory SignerFactory) SignerRegistry {
	if registry.factories == nil {
		registry.factories = make(map[config.CryptoBackend]SignerFactory)
	}
	registry.factories[backend] = factory
	return registry
}

func (registry SignerRegistry) RegisterBLSAdapter(factory BLSAdapterFactory) SignerRegistry {
	return registry.Register(config.CryptoBackendBLS, func() (Signer, error) {
		adapter, err := factory()
		if err != nil {
			return nil, err
		}
		if err := ValidateBLSAdapter(adapter); err != nil {
			return nil, err
		}
		return adapter, nil
	})
}

func (registry SignerRegistry) Generate(backend config.CryptoBackend) (Signer, error) {
	factory, found := registry.factories[backend]
	if !found {
		return nil, ErrUnsupportedSignerBackend
	}
	return factory()
}

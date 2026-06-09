package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidVRFKey = errors.New("invalid vrf key")
var ErrVRFBackendUnavailable = errors.New("vrf backend is unavailable: build with a production vrf adapter")
var ErrVRFAdapterUnsafe = errors.New("vrf adapter does not satisfy production safety requirements")

type DeterministicVRF struct {
	keys map[string][]byte
}

func NewDeterministicVRF(keys map[string][]byte) DeterministicVRF {
	copied := make(map[string][]byte, len(keys))
	for publicKey, privateKey := range keys {
		copied[publicKey] = append([]byte(nil), privateKey...)
	}
	return DeterministicVRF{keys: copied}
}

func NewVRF(cfg config.VRFConfig) (VRF, error) {
	if cfg.ProductionAdapter || cfg.AdapterName != "" {
		adapterName := cfg.AdapterName
		if adapterName == "" {
			adapterName = VRFAdapterECVRFP256Name
		}
		factory, found := registeredVRFAdapter(adapterName)
		if !found {
			return nil, ErrVRFBackendUnavailable
		}
		adapter, err := factory(cfg)
		if err != nil {
			return nil, err
		}
		if err := ValidateVRFAdapter(adapter, cfg); err != nil {
			return nil, err
		}
		return adapter, nil
	}
	return NewDeterministicVRF(cfg.Keys), nil
}

func NewProductionVRF(cfg config.VRFConfig) (VRF, error) {
	if !cfg.ProductionAdapter {
		return nil, ErrVRFAdapterUnsafe
	}
	return NewVRF(cfg)
}

func ValidateVRFAdapter(adapter VRFAdapter, cfg config.VRFConfig) error {
	if adapter == nil {
		return ErrVRFBackendUnavailable
	}
	metadata := adapter.Metadata()
	if metadata.Name == "" ||
		metadata.Version == "" ||
		!metadata.Audited ||
		metadata.AuditReport == "" ||
		metadata.KeySource == "" ||
		!metadata.DomainSeparation ||
		!metadata.ProofVerification ||
		!metadata.DeterministicOutput ||
		!metadata.MalformedInputFuzzed {
		return ErrVRFAdapterUnsafe
	}
	if cfg.AdapterName != "" && metadata.Name != cfg.AdapterName {
		return ErrVRFAdapterUnsafe
	}
	if cfg.AuditReport != "" && metadata.AuditReport != cfg.AuditReport {
		return ErrVRFAdapterUnsafe
	}
	if cfg.KeySource != "" && metadata.KeySource != cfg.KeySource {
		return ErrVRFAdapterUnsafe
	}
	return nil
}

func (vrf DeterministicVRF) Prove(publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error) {
	privateKey, found := vrf.keys[string(publicKey)]
	if !found || len(privateKey) == 0 {
		return nil, nil, ErrInvalidVRFKey
	}
	output = vrfOutput(privateKey, seed)
	proof = append([]byte(nil), output...)
	return output, proof, nil
}

func (vrf DeterministicVRF) Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	privateKey, found := vrf.keys[string(publicKey)]
	if !found || len(privateKey) == 0 {
		return false
	}
	expected := vrfOutput(privateKey, seed)
	return hmac.Equal(expected, output) && hmac.Equal(expected, proof)
}

func vrfOutput(privateKey []byte, seed []byte) []byte {
	mac := hmac.New(sha256.New, privateKey)
	mac.Write(seed)
	return mac.Sum(nil)
}

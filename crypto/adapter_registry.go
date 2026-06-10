package crypto

import (
	"sync"

	"github.com/vexo-network/vexo-consensus/config"
)

type VRFAdapter interface {
	VRF
	Metadata() VRFAdapterMetadata
}

type VRFAdapterMetadata struct {
	Name                 string
	Version              string
	Audited              bool
	AuditReport          string
	DependencyAudit      string
	KeySource            string
	DomainSeparation     bool
	ProofVerification    bool
	DeterministicOutput  bool
	MalformedInputFuzzed bool
}

type VRFAdapterFactory func(config.VRFConfig) (VRFAdapter, error)

var globalAdapters = struct {
	sync.RWMutex
	bls map[string]BLSAdapterFactory
	vrf map[string]VRFAdapterFactory
}{
	bls: make(map[string]BLSAdapterFactory),
	vrf: make(map[string]VRFAdapterFactory),
}

func RegisterBLSAdapter(name string, factory BLSAdapterFactory) {
	if name == "" || factory == nil {
		return
	}
	globalAdapters.Lock()
	defer globalAdapters.Unlock()
	globalAdapters.bls[name] = factory
}

func RegisterVRFAdapter(name string, factory VRFAdapterFactory) {
	if name == "" || factory == nil {
		return
	}
	globalAdapters.Lock()
	defer globalAdapters.Unlock()
	globalAdapters.vrf[name] = factory
}

func registeredBLSAdapter(name string) (BLSAdapterFactory, bool) {
	globalAdapters.RLock()
	defer globalAdapters.RUnlock()
	factory, found := globalAdapters.bls[name]
	return factory, found
}

func registeredVRFAdapter(name string) (VRFAdapterFactory, bool) {
	globalAdapters.RLock()
	defer globalAdapters.RUnlock()
	factory, found := globalAdapters.vrf[name]
	return factory, found
}

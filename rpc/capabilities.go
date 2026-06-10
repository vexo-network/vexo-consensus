package rpc

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrMissingRequiredCapability = errors.New("missing required rpc capability")

type providerCapability struct {
	name        string
	description string
	available   func(StatusProvider) bool
}

var providerCapabilityCatalog = []providerCapability{
	{name: "status", description: "node status and readiness", available: func(provider StatusProvider) bool { return provider != nil }},
	{name: "metrics", description: "node metrics and Prometheus text metrics", available: func(provider StatusProvider) bool { _, ok := provider.(MetricsProvider); return ok }},
	{name: "snapshot", description: "state snapshot export and chunk export", available: func(provider StatusProvider) bool { _, ok := provider.(SnapshotProvider); return ok }},
	{name: "recovery", description: "storage recovery report and repair", available: func(provider StatusProvider) bool { _, ok := provider.(RecoveryProvider); return ok }},
	{name: "tx", description: "transaction submission", available: func(provider StatusProvider) bool { _, ok := provider.(TxSubmitter); return ok }},
	{name: "pending_txs", description: "pending transaction hashes and payloads", available: func(provider StatusProvider) bool { _, ok := provider.(PendingTxsProvider); return ok }},
	{name: "evidence", description: "evidence submission and slashing lifecycle", available: func(provider StatusProvider) bool { _, ok := provider.(EvidenceSubmitter); return ok }},
	{name: "blocks", description: "block lookup and block index", available: func(provider StatusProvider) bool { _, ok := provider.(ChainQueryProvider); return ok }},
	{name: "state_by_height", description: "historical state lookup", available: func(provider StatusProvider) bool { _, ok := provider.(StateByHeightProvider); return ok }},
	{name: "events", description: "indexed event query", available: func(provider StatusProvider) bool { _, ok := provider.(EventQueryProvider); return ok }},
	{name: "proofs", description: "state and IBC query proofs", available: func(provider StatusProvider) bool { _, ok := provider.(QueryProofProvider); return ok }},
	{name: "ibc", description: "IBC module query paths", available: func(provider StatusProvider) bool { _, ok := provider.(IBCQueryProvider); return ok }},
	{name: "app_query", description: "application module query paths", available: func(provider StatusProvider) bool { _, ok := provider.(AppQueryProvider); return ok }},
	{name: "accounts", description: "account sequence query", available: func(provider StatusProvider) bool { _, ok := provider.(AccountQueryProvider); return ok }},
	{name: "finality", description: "latest and height-specific finality proofs", available: func(provider StatusProvider) bool { _, ok := provider.(FinalityProvider); return ok }},
	{name: "prune", description: "admin pruning", available: func(provider StatusProvider) bool { _, ok := provider.(PruneProvider); return ok }},
	{name: "replay", description: "state replay verification", available: func(provider StatusProvider) bool { _, ok := provider.(ReplayProvider); return ok }},
	{name: "strict_replay", description: "strict replay verification", available: func(provider StatusProvider) bool { _, ok := provider.(StrictReplayProvider); return ok }},
	{name: "consensus_control", description: "admin consensus loop start/stop", available: func(provider StatusProvider) bool { _, ok := provider.(ConsensusLoopController); return ok }},
	{name: "validators", description: "validator set and committee query", available: func(provider StatusProvider) bool { _, ok := provider.(ValidatorQueryProvider); return ok }},
}

func providerCapabilities(provider StatusProvider, cfg Config) CapabilityResponse {
	required := requiredCapabilitySet(cfg)
	response := CapabilityResponse{
		Complete:     true,
		Capabilities: make([]CapabilitySnapshot, 0, len(providerCapabilityCatalog)),
	}
	for _, capability := range providerCapabilityCatalog {
		available := capability.available(provider)
		requiredByConfig := required[capability.name]
		if requiredByConfig && !available {
			response.Complete = false
			response.Missing = append(response.Missing, capability.name)
		}
		response.Capabilities = append(response.Capabilities, CapabilitySnapshot{
			Name:        capability.name,
			Available:   available,
			Required:    requiredByConfig,
			Description: capability.description,
		})
	}
	sort.Strings(response.Missing)
	return response
}

func validateRequiredCapabilities(provider StatusProvider, cfg Config) error {
	capabilities := providerCapabilities(provider, cfg)
	if capabilities.Complete {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrMissingRequiredCapability, strings.Join(capabilities.Missing, ","))
}

func RequiredCapabilityNames() []string {
	names := make([]string, 0, len(providerCapabilityCatalog))
	for _, capability := range providerCapabilityCatalog {
		names = append(names, capability.name)
	}
	sort.Strings(names)
	return names
}

func NetworkSafeConfig(cfg Config) Config {
	cfg.RequireAllCapabilities = true
	return cfg
}

func requiredCapabilitySet(cfg Config) map[string]bool {
	required := make(map[string]bool)
	if cfg.RequireAllCapabilities {
		for _, capability := range providerCapabilityCatalog {
			required[capability.name] = true
		}
	}
	for _, name := range cfg.RequiredCapabilities {
		name = strings.TrimSpace(name)
		if name != "" {
			required[name] = true
		}
	}
	return required
}

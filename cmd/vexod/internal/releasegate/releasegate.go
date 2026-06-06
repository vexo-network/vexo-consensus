package releasegate

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
)

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type Document struct {
	SchemaVersion string   `json:"schema_version"`
	OK            bool     `json:"ok"`
	Version       string   `json:"version"`
	Checks        []Check  `json:"checks"`
	NextActions   []string `json:"next_actions,omitempty"`
}

type PackCheck struct {
	Name    string
	OK      bool
	Message string
}

type Pack struct {
	OK     bool
	Checks []PackCheck
}

type Evidence struct {
	Chaos                string
	KMS                  string
	Snapshot             string
	P2PScale             string
	StateSyncLightClient string
	ValidatorEconomics   string
	UpgradeGovernance    string
	MEVFeeMarket         string
	OpsRunbook           string
	FormalSafety         string
	SDKConformance       string
	ExternalAudit        string
	BLSAudit             string
	AllowExternalPending bool
	Exists               func(string) bool
	ReadFile             func(string) ([]byte, error)
}

func Build(version string, pack Pack, evidence Evidence) Document {
	if evidence.Exists == nil {
		evidence.Exists = func(string) bool { return false }
	}
	document := Document{
		SchemaVersion: "v1",
		OK:            true,
		Version:       version,
	}
	document.addCheck("release_pack", pack.OK, "release pack must include manifest, checksums, SBOM, signature when required, and core RC evidence")
	for _, check := range pack.Checks {
		document.addCheck("pack_"+check.Name, check.OK, check.Message)
	}
	document.addFileCheck("chaos_evidence", evidence.Chaos, "chaos test evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("kms_signer_evidence", evidence.KMS, "KMS/remote signer policy and double-sign guard evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("snapshot_replay_evidence", evidence.Snapshot, "snapshot restore and replay consistency evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("p2p_scale_evidence", evidence.P2PScale, "large validator P2P discovery, reconnect, backpressure, NAT, seed, and addrbook evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("state_sync_light_client_evidence", evidence.StateSyncLightClient, "state sync, snapshot restore, replay, and light-client finality proof evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("validator_economics_evidence", evidence.ValidatorEconomics, "staking, rewards, commission, jail, tombstone, unbonding, and slashing accounting evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("upgrade_governance_evidence", evidence.UpgradeGovernance, "governance-approved upgrade, migration, halt, rollback, and failed-upgrade evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("mev_fee_market_evidence", evidence.MEVFeeMarket, "fee market, fair ordering, MEV mitigation, spam-cost, and mempool replay evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("ops_runbook_evidence", evidence.OpsRunbook, "operator runbook, alert thresholds, incident drill, and multi-region observability evidence must exist and contain passing evidence", evidence)
	document.addFileCheck("formal_safety_evidence", evidence.FormalSafety, "formal-ish safety argument, invariant tests, adversarial simulations, and property output must exist and contain passing evidence", evidence)
	document.addFileCheck("sdk_conformance_evidence", evidence.SDKConformance, "app module, storage, crypto, transport, RPC, and upgrade API conformance evidence must exist and contain passing evidence", evidence)
	document.addExternalCheck("external_security_audit", evidence.ExternalAudit, evidence.AllowExternalPending, "external audit disposition must exist before public production release", evidence)
	document.addExternalCheck("bls_adapter_audit", evidence.BLSAudit, evidence.AllowExternalPending, "audited BLS adapter and dependency audit evidence must exist when BLS is enabled", evidence)
	if !document.OK {
		document.NextActions = []string{
			"collect missing evidence artifacts and rerun release gate",
			"do not publish production release artifacts until all required checks pass",
			"use --allow-external-pending only for private release candidates; never use it for public value-bearing launches",
			"treat missing scale, state-sync, economics, MEV, or governance evidence as a release blocker, not a documentation gap",
		}
	}
	return document
}

func (document *Document) addCheck(name string, ok bool, message string) {
	if !ok {
		document.OK = false
	}
	document.Checks = append(document.Checks, Check{Name: name, OK: ok, Message: message})
}

func (document *Document) addFileCheck(name string, path string, message string, evidence Evidence) {
	document.addCheck(name, evidenceFileOK(path, evidence), message)
}

func (document *Document) addExternalCheck(name string, path string, allowPending bool, message string, evidence Evidence) {
	if evidenceFileOK(path, evidence) {
		document.addCheck(name, true, message)
		return
	}
	document.addCheck(name, allowPending, message)
}

func evidenceFileOK(path string, evidence Evidence) bool {
	if path == "" || !evidence.Exists(path) {
		return false
	}
	if evidence.ReadFile == nil {
		return true
	}
	data, err := evidence.ReadFile(path)
	if err != nil {
		return false
	}
	return EvidenceContentOK(path, data)
}

func EvidenceContentOK(path string, data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		var value any
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return false
		}
		return jsonEvidenceOK(value)
	default:
		return len(trimmed) >= 8
	}
}

func jsonEvidenceOK(value any) bool {
	switch item := value.(type) {
	case map[string]any:
		if okValue, found := item["ok"].(bool); found && !okValue {
			return false
		}
		if status, found := item["status"].(string); found {
			normalized := strings.ToLower(status)
			if strings.Contains(normalized, "fail") || strings.Contains(normalized, "error") {
				return false
			}
		}
		if checks, found := item["checks"].([]any); found {
			if len(checks) == 0 {
				return false
			}
			for _, check := range checks {
				if !jsonEvidenceOK(check) {
					return false
				}
			}
		}
		if evidence, found := item["evidence"].([]any); found && len(evidence) == 0 {
			return false
		}
	case []any:
		if len(item) == 0 {
			return false
		}
		for _, value := range item {
			if !jsonEvidenceOK(value) {
				return false
			}
		}
	}
	return true
}

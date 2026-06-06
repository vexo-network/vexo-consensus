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
	document.addFileCheck("chaos_evidence", evidence.Chaos, "chaos test evidence must exist and semantically cover chaos/fault scenarios", evidence)
	document.addFileCheck("kms_signer_evidence", evidence.KMS, "KMS/remote signer evidence must cover signer policy and double-sign protection", evidence)
	document.addFileCheck("snapshot_replay_evidence", evidence.Snapshot, "snapshot evidence must cover snapshot restore and replay consistency", evidence)
	document.addFileCheck("p2p_scale_evidence", evidence.P2PScale, "P2P scale evidence must cover peer discovery/reconnect/scale behavior", evidence)
	document.addFileCheck("state_sync_light_client_evidence", evidence.StateSyncLightClient, "state-sync evidence must cover light-client finality proof verification", evidence)
	document.addFileCheck("validator_economics_evidence", evidence.ValidatorEconomics, "validator economics evidence must cover validator accounting and slashing/rewards/unbonding", evidence)
	document.addFileCheck("upgrade_governance_evidence", evidence.UpgradeGovernance, "upgrade evidence must cover governance, migration, and rollback/halt behavior", evidence)
	document.addFileCheck("mev_fee_market_evidence", evidence.MEVFeeMarket, "MEV/fee evidence must cover fee market, fair ordering, or mempool pressure", evidence)
	document.addFileCheck("ops_runbook_evidence", evidence.OpsRunbook, "ops evidence must cover runbooks plus alert/incident/metrics handling", evidence)
	document.addFileCheck("formal_safety_evidence", evidence.FormalSafety, "formal safety evidence must cover safety plus invariants/adversarial/property output", evidence)
	document.addFileCheck("sdk_conformance_evidence", evidence.SDKConformance, "SDK evidence must cover SDK/API conformance for modules/RPC/storage/crypto/transport", evidence)
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
	document.addCheck(name, evidenceFileOK(name, path, evidence), message)
}

func (document *Document) addExternalCheck(name string, path string, allowPending bool, message string, evidence Evidence) {
	if evidenceFileOK(name, path, evidence) {
		document.addCheck(name, true, message)
		return
	}
	document.addCheck(name, allowPending, message)
}

func evidenceFileOK(name string, path string, evidence Evidence) bool {
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
	return EvidenceCheckContentOK(name, path, data)
}

func EvidenceContentOK(path string, data []byte) bool {
	return EvidenceCheckContentOK("", path, data)
}

func EvidenceCheckContentOK(name string, path string, data []byte) bool {
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
		return jsonEvidenceOK(value) && evidenceCovers(name, path, value)
	default:
		return len(trimmed) >= 8 && evidenceTextCovers(name, path, string(trimmed))
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

func evidenceCovers(name string, path string, value any) bool {
	return evidenceTextCovers(name, path, flattenEvidenceText(value))
}

func evidenceTextCovers(name string, path string, text string) bool {
	requirements := semanticRequirements(name)
	if len(requirements) == 0 {
		return true
	}
	normalized := strings.ToLower(path + " " + strings.ReplaceAll(text, "_", " "))
	for _, group := range requirements {
		matched := false
		for _, term := range group {
			if strings.Contains(normalized, strings.ToLower(term)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func semanticRequirements(name string) [][]string {
	switch name {
	case "longrun_evidence":
		return [][]string{{"longrun", "long run", "soak"}, {"duration", "height", "validator"}, {"per_node", "per-node", "distributed"}}
	case "adversarial_evidence":
		return [][]string{{"adversarial"}, {"consensus", "simulation", "partition"}}
	case "fuzz_evidence":
		return [][]string{{"fuzz", "property"}}
	case "chaos_evidence":
		return [][]string{{"chaos"}, {"fault", "partition", "restart", "kill"}}
	case "kms_signer_evidence":
		return [][]string{{"kms", "signer"}, {"double sign", "double-sign", "policy", "guard"}}
	case "snapshot_replay_evidence":
		return [][]string{{"snapshot"}, {"replay", "restore"}}
	case "p2p_scale_evidence":
		return [][]string{{"p2p", "peer"}, {"scale", "discovery", "reconnect", "backpressure"}}
	case "state_sync_light_client_evidence":
		return [][]string{{"state sync", "state-sync"}, {"light client", "light-client", "finality"}}
	case "validator_economics_evidence":
		return [][]string{{"validator"}, {"slashing", "reward", "commission", "unbonding", "jail"}}
	case "upgrade_governance_evidence":
		return [][]string{{"upgrade"}, {"governance", "migration", "rollback", "halt"}, {"allow_noop", "no-op", "noop"}}
	case "mev_fee_market_evidence":
		return [][]string{{"fee", "mev"}, {"market", "fair", "mempool", "ordering"}, {"replacement", "replace"}}
	case "ops_runbook_evidence":
		return [][]string{{"ops", "runbook"}, {"alert", "incident", "metrics"}}
	case "formal_safety_evidence":
		return [][]string{{"safety"}, {"invariant", "adversarial", "property", "proof"}}
	case "sdk_conformance_evidence":
		return [][]string{{"sdk", "api"}, {"conformance", "module", "rpc", "storage", "crypto", "transport"}}
	case "external_security_audit":
		return [][]string{{"external", "security"}, {"audit", "disposition"}}
	case "bls_adapter_audit":
		return [][]string{{"bls"}, {"audit", "dependency", "subgroup", "rogue"}}
	default:
		return nil
	}
}

func flattenEvidenceText(value any) string {
	var builder strings.Builder
	writeEvidenceText(&builder, value)
	return builder.String()
}

func writeEvidenceText(builder *strings.Builder, value any) {
	switch item := value.(type) {
	case map[string]any:
		for key, value := range item {
			builder.WriteByte(' ')
			builder.WriteString(key)
			writeEvidenceText(builder, value)
		}
	case []any:
		for _, value := range item {
			writeEvidenceText(builder, value)
		}
	case string:
		builder.WriteByte(' ')
		builder.WriteString(item)
	case float64, bool:
		builder.WriteByte(' ')
		builder.WriteString(strings.TrimSpace(strings.ReplaceAll(strings.ToLower(jsonScalar(item)), "\"", "")))
	}
}

func jsonScalar(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

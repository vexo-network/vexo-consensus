package releasegate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	Manifest             string
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
	BLSAuditSHA256       string
	VRFAudit             string
	AllowExternalPending bool
	Exists               func(string) bool
	ReadFile             func(string) ([]byte, error)
}

type EvidenceManifest struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at,omitempty"`
	Evidence      []EvidenceManifestEntry `json:"evidence"`
}

type EvidenceManifestEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
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
	document.addCheck("evidence_manifest", evidenceManifestOK(evidence), "evidence manifest must exist and bind evidence artifact names, paths, and sha256 hashes")
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
	if strings.TrimSpace(evidence.BLSAuditSHA256) != "" {
		document.addCheck("bls_adapter_audit_digest", evidenceFileSHA256OK(evidence.BLSAudit, evidence.BLSAuditSHA256, evidence), "BLS audit evidence digest must match the configured crypto audit_evidence_sha256 pin")
	}
	document.addExternalCheck("vrf_adapter_audit", evidence.VRFAudit, evidence.AllowExternalPending, "audited VRF adapter/KMS evidence must cover TLS/mTLS, auth, replay protection, and key custody", evidence)
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
		return evidence.Manifest == ""
	}
	data, err := evidence.ReadFile(path)
	if err != nil {
		return false
	}
	return EvidenceCheckContentOK(name, path, data) && evidenceFileManifestOK(name, path, data, evidence)
}

func evidenceFileSHA256OK(path string, expected string, evidence Evidence) bool {
	if path == "" || evidence.ReadFile == nil || !evidence.Exists(path) {
		return false
	}
	normalizedExpected := strings.ToLower(strings.TrimSpace(expected))
	if len(normalizedExpected) != 64 {
		return false
	}
	if _, err := hex.DecodeString(normalizedExpected); err != nil {
		return false
	}
	data, err := evidence.ReadFile(path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == normalizedExpected
}

func evidenceManifestOK(evidence Evidence) bool {
	manifest, ok := readEvidenceManifest(evidence)
	if !ok {
		return false
	}
	if strings.TrimSpace(manifest.SchemaVersion) == "" || len(manifest.Evidence) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(manifest.Evidence))
	for _, entry := range manifest.Evidence {
		if strings.TrimSpace(entry.Name) == "" || strings.TrimSpace(entry.Path) == "" || strings.TrimSpace(entry.SHA256) == "" {
			return false
		}
		normalizedHash := strings.ToLower(strings.TrimSpace(entry.SHA256))
		if len(normalizedHash) != 64 {
			return false
		}
		if _, err := hex.DecodeString(normalizedHash); err != nil {
			return false
		}
		key := strings.ToLower(entry.Name + "\x00" + entry.Path)
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func evidenceFileManifestOK(name string, path string, data []byte, evidence Evidence) bool {
	if evidence.Manifest == "" {
		return true
	}
	manifest, ok := readEvidenceManifest(evidence)
	if !ok {
		return false
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	for _, entry := range manifest.Evidence {
		if !manifestEntryMatches(entry, name, path) {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(entry.SHA256), actual)
	}
	return false
}

func readEvidenceManifest(evidence Evidence) (EvidenceManifest, bool) {
	if evidence.Manifest == "" || evidence.ReadFile == nil || !evidence.Exists(evidence.Manifest) {
		return EvidenceManifest{}, false
	}
	data, err := evidence.ReadFile(evidence.Manifest)
	if err != nil {
		return EvidenceManifest{}, false
	}
	var manifest EvidenceManifest
	if err := json.Unmarshal(bytes.TrimSpace(data), &manifest); err != nil {
		return EvidenceManifest{}, false
	}
	return manifest, true
}

func manifestEntryMatches(entry EvidenceManifestEntry, name string, path string) bool {
	if entry.Name == name {
		return true
	}
	entryPath := filepath.Clean(entry.Path)
	targetPath := filepath.Clean(path)
	return entryPath == targetPath || filepath.Base(entryPath) == filepath.Base(targetPath)
}

func EvidenceContentOK(path string, data []byte) bool {
	return EvidenceCheckContentOK("", path, data)
}

func EvidenceCheckContentOK(name string, path string, data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	if evidenceContainsUnsafeClaim(name, path, string(trimmed)) {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		var value any
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return false
		}
		return jsonEvidenceOK(value) && typedEvidenceOK(name, value) && evidenceCovers(name, path, value)
	default:
		return len(trimmed) >= 8 && evidenceTextCovers(name, path, string(trimmed))
	}
}

func evidenceContainsUnsafeClaim(name string, path string, text string) bool {
	normalized := strings.ToLower(path + " " + strings.ReplaceAll(text, "_", " "))
	if strings.Contains(normalized, "operator supplied audit required") ||
		strings.Contains(normalized, "operator-supplied-audit-required") ||
		strings.Contains(normalized, "audit pending") ||
		strings.Contains(normalized, "unaudited") ||
		strings.Contains(normalized, "audited false") ||
		strings.Contains(normalized, `"audited":false`) {
		return true
	}
	if name == "bls_adapter_audit" && strings.Contains(normalized, "circl-bls12381") {
		return true
	}
	return false
}

func jsonEvidenceOK(value any) bool {
	switch item := value.(type) {
	case map[string]any:
		if okValue, found := item["ok"].(bool); found && !okValue {
			return false
		}
		if coverageOK, found := item["coverage_ok"].(bool); found && !coverageOK {
			return false
		}
		if failed, found := numericEvidenceValue(item["failed"]); found && failed > 0 {
			return false
		}
		if total, found := numericEvidenceValue(item["total"]); found && total == 0 {
			return false
		}
		if missing, found := item["missing_categories"].([]any); found && len(missing) > 0 {
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
		if results, found := item["results"].([]any); found {
			if len(results) == 0 {
				return false
			}
			for _, result := range results {
				if !jsonEvidenceOK(result) {
					return false
				}
			}
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

func typedEvidenceOK(name string, value any) bool {
	item, ok := value.(map[string]any)
	if !ok {
		return true
	}
	if evidenceType, found := evidenceString(item["evidence_type"]); found {
		if name != "" && evidenceType != "" && evidenceType != name {
			return false
		}
		if !collectedEvidenceOK(item) {
			return false
		}
	}
	switch name {
	case "longrun_evidence":
		return networkLongRunEvidenceOK(item)
	case "snapshot_replay_evidence":
		return snapshotReplayEvidenceOK(item)
	case "p2p_scale_evidence":
		return p2pScaleEvidenceOK(item)
	case "state_sync_light_client_evidence":
		return stateSyncLightClientEvidenceOK(item)
	case "ops_runbook_evidence":
		return opsRunbookEvidenceOK(item)
	case "sdk_conformance_evidence":
		return sdkConformanceEvidenceOK(item)
	default:
		return true
	}
}

func collectedEvidenceOK(item map[string]any) bool {
	if schemaVersion, found := evidenceString(item["schema_version"]); !found || strings.TrimSpace(schemaVersion) == "" {
		return false
	}
	if okValue, found := item["ok"].(bool); !found || !okValue {
		return false
	}
	checks, found := evidenceArray(item["checks"])
	if !found || len(checks) == 0 {
		return false
	}
	rpcs, found := evidenceArray(item["rpcs"])
	if !found || len(rpcs) == 0 {
		return false
	}
	for _, check := range checks {
		if !jsonEvidenceOK(check) {
			return false
		}
	}
	return true
}

func networkLongRunEvidenceOK(item map[string]any) bool {
	if _, found := item["nodes"]; !found {
		return true
	}
	nodes, found := evidenceArray(item["nodes"])
	if !found || len(nodes) == 0 {
		return false
	}
	if load, found := evidenceMap(item["load"]); found {
		if submitted, found := numericEvidenceValue(load["submitted"]); found && submitted == 0 {
			return false
		}
		if failed, found := numericEvidenceValue(load["failed"]); found && failed > 0 {
			return false
		}
	}
	for _, node := range nodes {
		nodeMap, ok := evidenceMap(node)
		if !ok {
			return false
		}
		if nodeError, found := evidenceString(nodeMap["error"]); found && strings.TrimSpace(nodeError) != "" {
			return false
		}
		before, beforeOK := evidenceMap(nodeMap["before"])
		after, afterOK := evidenceMap(nodeMap["after"])
		if beforeOK && afterOK {
			beforeHeight, beforeFound := numericEvidenceValue(before["latest_height"])
			afterHeight, afterFound := numericEvidenceValue(after["latest_height"])
			if beforeFound && afterFound && afterHeight <= beforeHeight {
				return false
			}
		}
		if report, found := evidenceMap(nodeMap["report"]); found {
			if okValue, found := report["ok"].(bool); found && !okValue {
				return false
			}
		}
	}
	return true
}

func snapshotReplayEvidenceOK(item map[string]any) bool {
	if _, found := item["evidence_type"]; !found {
		return true
	}
	for _, rpcValue := range evidenceRPCs(item) {
		final, found := evidenceMap(rpcValue["final"])
		if !found {
			return false
		}
		snapshot, snapshotFound := evidenceMap(final["snapshot"])
		diagnostics, diagnosticsFound := evidenceMap(final["diagnostics"])
		replayStatus, replayFound := evidenceMap(final["replay_status"])
		if snapshotFound {
			if height, found := numericEvidenceValue(snapshot["height"]); !found || height == 0 {
				return false
			}
		}
		if diagnosticsFound && evidenceBoolFalse(diagnostics["replay_healthy"]) {
			return false
		}
		if replayFound && !evidenceReplayHealthy(replayStatus) {
			return false
		}
		if !snapshotFound && !diagnosticsFound && !replayFound {
			return false
		}
	}
	return true
}

func p2pScaleEvidenceOK(item map[string]any) bool {
	if _, found := item["evidence_type"]; !found {
		return true
	}
	for _, rpcValue := range evidenceRPCs(item) {
		final, found := evidenceMap(rpcValue["final"])
		if !found {
			return false
		}
		status, statusFound := evidenceMap(final["status"])
		peers, peersFound := evidenceMap(final["peers"])
		if statusFound {
			if peerCount, found := numericEvidenceValue(status["peer_count"]); found && peerCount > 0 {
				continue
			}
		}
		if peersFound {
			if peerList, found := evidenceArray(peers["peers"]); found && len(peerList) > 0 {
				continue
			}
		}
		return false
	}
	return true
}

func stateSyncLightClientEvidenceOK(item map[string]any) bool {
	if _, found := item["evidence_type"]; !found {
		return true
	}
	for _, rpcValue := range evidenceRPCs(item) {
		final, found := evidenceMap(rpcValue["final"])
		if !found {
			return false
		}
		if _, found := evidenceMap(final["finality"]); !found {
			return false
		}
	}
	return true
}

func opsRunbookEvidenceOK(item map[string]any) bool {
	if _, found := item["evidence_type"]; !found {
		return true
	}
	for _, rpcValue := range evidenceRPCs(item) {
		final, found := evidenceMap(rpcValue["final"])
		if !found {
			return false
		}
		if _, found := evidenceMap(final["metrics"]); !found {
			return false
		}
	}
	return true
}

func sdkConformanceEvidenceOK(item map[string]any) bool {
	if evmFixtures, found := evidenceMap(item["evm_fixtures"]); found && !jsonEvidenceOK(evmFixtures) {
		return false
	}
	if evmExecution, found := evidenceMap(item["evm_execution"]); found && !jsonEvidenceOK(evmExecution) {
		return false
	}
	return true
}

func evidenceRPCs(item map[string]any) []map[string]any {
	values, found := evidenceArray(item["rpcs"])
	if !found {
		return nil
	}
	rpcs := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if rpc, ok := evidenceMap(value); ok {
			rpcs = append(rpcs, rpc)
		}
	}
	return rpcs
}

func evidenceMap(value any) (map[string]any, bool) {
	item, ok := value.(map[string]any)
	return item, ok
}

func evidenceArray(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func evidenceString(value any) (string, bool) {
	item, ok := value.(string)
	return item, ok
}

func evidenceBoolFalse(value any) bool {
	boolean, found := value.(bool)
	return found && !boolean
}

func evidenceReplayHealthy(item map[string]any) bool {
	if okValue, found := item["ok"].(bool); found {
		return okValue
	}
	if healthy, found := item["healthy"].(bool); found {
		return healthy
	}
	if status, found := evidenceString(item["status"]); found {
		normalized := strings.ToLower(status)
		return !strings.Contains(normalized, "fail") && !strings.Contains(normalized, "error") && !strings.Contains(normalized, "unhealthy")
	}
	return false
}

func numericEvidenceValue(value any) (uint64, bool) {
	switch number := value.(type) {
	case float64:
		if number < 0 {
			return 0, false
		}
		return uint64(number), true
	case json.Number:
		parsed, err := number.Int64()
		if err != nil || parsed < 0 {
			return 0, false
		}
		return uint64(parsed), true
	default:
		return 0, false
	}
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
		return [][]string{{"kms", "signer"}, {"double sign", "double-sign", "policy", "guard"}, {"nonce", "replay"}, {"audit"}}
	case "snapshot_replay_evidence":
		return [][]string{{"snapshot"}, {"replay", "restore"}}
	case "p2p_scale_evidence":
		return [][]string{{"p2p", "peer"}, {"scale", "discovery", "reconnect", "backpressure"}}
	case "state_sync_light_client_evidence":
		return [][]string{{"state sync", "state-sync"}, {"light client", "light-client", "finality"}}
	case "validator_economics_evidence":
		return [][]string{{"validator"}, {"slashing", "reward", "commission", "unbonding", "jail"}, {"custody", "stake accounting", "stake-accounting"}, {"tombstone"}, {"false slashing", "false-slashing"}}
	case "upgrade_governance_evidence":
		return [][]string{{"upgrade"}, {"governance", "migration", "rollback", "halt"}, {"allow_noop", "no-op", "noop"}, {"authority"}, {"rollback-required", "rollback required"}, {"last safe height", "last-safe-height"}}
	case "mev_fee_market_evidence":
		return [][]string{{"fee", "mev"}, {"market", "fair", "mempool", "ordering"}, {"replacement", "replace"}}
	case "ops_runbook_evidence":
		return [][]string{{"ops", "runbook"}, {"alert", "incident", "metrics"}}
	case "formal_safety_evidence":
		return [][]string{{"safety"}, {"invariant", "adversarial", "property", "proof"}}
	case "sdk_conformance_evidence":
		return [][]string{{"sdk", "api"}, {"conformance", "module", "rpc", "storage", "crypto", "transport"}, {"ibc", "relayer", "proof"}, {"evm", "web3", "ethereum"}, {"fixture", "fixtures"}, {"transaction", "raw tx", "raw transaction"}, {"execution", "vm", "opcode"}}
	case "external_security_audit":
		return [][]string{{"external", "security"}, {"audit", "disposition"}}
	case "bls_adapter_audit":
		return [][]string{{"bls"}, {"adapter", "implementation", "blst", "supranational"}, {"audit", "dependency"}, {"version", "dependency version", "pinned"}, {"subgroup", "rogue"}, {"proof of possession", "proof-of-possession", "pop"}, {"key validation", "key-validation"}}
	case "vrf_adapter_audit":
		return [][]string{{"vrf"}, {"adapter", "implementation", "remote", "ecvrf"}, {"audit", "dependency"}, {"tls", "mtls", "mutual tls", "certificate"}, {"auth", "token", "authorization"}, {"nonce", "replay"}, {"custody", "kms", "hsm"}}
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

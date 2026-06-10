package releasegate

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

var releaseGateTestPrivateKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))

func TestBuildPassesWithAllEvidence(t *testing.T) {
	evidence := completeReleaseGateEvidence(false)
	document := Build("v1.2.3", Pack{
		OK: true,
		Checks: []PackCheck{
			{Name: "manifest", OK: true, Message: "manifest exists"},
		},
	}, evidence)

	if !document.OK {
		t.Fatalf("expected release gate to pass: %+v", document)
	}
	if document.Version != "v1.2.3" {
		t.Fatalf("unexpected version %q", document.Version)
	}
	if len(document.NextActions) != 0 {
		t.Fatalf("unexpected next actions: %+v", document.NextActions)
	}
}

func TestBuildFailsWhenEvidenceMissing(t *testing.T) {
	document := Build("v1.2.3", Pack{OK: true}, Evidence{})

	if document.OK {
		t.Fatalf("expected release gate to fail")
	}
	if len(document.NextActions) == 0 {
		t.Fatalf("expected failed gate to include next actions")
	}
}

func TestBuildAllowsExternalPendingOnlyForExternalChecks(t *testing.T) {
	evidence := completeReleaseGateEvidence(true)
	evidence.ExternalAudit = ""
	evidence.BLSAudit = ""
	evidence.AllowExternalPending = true
	document := Build("rc", Pack{OK: true}, evidence)

	if !document.OK {
		t.Fatalf("expected pending external checks to pass for private RCs: %+v", document)
	}
}

func TestBuildRejectsExternalPendingForPublicVersion(t *testing.T) {
	evidence := completeReleaseGateEvidence(true)
	evidence.ExternalAudit = ""
	evidence.BLSAudit = ""
	evidence.VRFAudit = ""
	evidence.AllowExternalPending = true
	document := Build("v1.2.3", Pack{OK: true}, evidence)

	if document.OK {
		t.Fatalf("expected public version to reject pending external checks")
	}
	if checkOK(document.Checks, "external_pending_scope") {
		t.Fatalf("expected external pending scope check to fail: %+v", document.Checks)
	}
}

func TestBuildFailsInvalidEvidenceContent(t *testing.T) {
	evidence := completeReleaseGateEvidence(false)
	evidenceFiles := completeReleaseGateEvidenceFiles(false)
	evidenceFiles["chaos.json"] = []byte(`{"ok":false,"checks":[{"ok":false}]}`)
	evidenceFiles["evidence-manifest.json"] = releaseGateManifestForEvidence(evidenceFiles)
	evidence.ReadFile = func(path string) ([]byte, error) {
		return evidenceFiles[path], nil
	}
	document := Build("rc", Pack{OK: true}, evidence)

	if document.OK {
		t.Fatalf("expected release gate to fail invalid evidence")
	}
	if checkOK(document.Checks, "chaos_evidence") {
		t.Fatalf("expected chaos evidence check to fail: %+v", document.Checks)
	}
}

func TestBuildFailsWhenBLSAuditDigestDoesNotMatchPin(t *testing.T) {
	evidence := completeReleaseGateEvidence(false)
	files := completeReleaseGateEvidenceFiles(false)
	sum := sha256.Sum256(files["bls.pdf"])
	evidence.BLSAuditSHA256 = hex.EncodeToString(sum[:])
	document := Build("rc", Pack{OK: true}, evidence)
	if !document.OK || !checkOK(document.Checks, "bls_adapter_audit_digest") {
		t.Fatalf("expected matching BLS audit digest to pass: %+v", document)
	}

	evidence.BLSAuditSHA256 = strings.Repeat("0", 64)
	document = Build("rc", Pack{OK: true}, evidence)
	if document.OK || checkOK(document.Checks, "bls_adapter_audit_digest") {
		t.Fatalf("expected mismatched BLS audit digest to fail: %+v", document)
	}
}

func TestEvidenceContentValidation(t *testing.T) {
	for _, testCase := range []struct {
		name string
		path string
		data []byte
		ok   bool
	}{
		{name: "json ok", path: "evidence.json", data: []byte(`{"ok":true,"checks":[{"ok":true}]}`), ok: true},
		{name: "json false", path: "evidence.json", data: []byte(`{"ok":false}`), ok: false},
		{name: "json failed status", path: "evidence.json", data: []byte(`{"status":"failed"}`), ok: false},
		{name: "json coverage false", path: "evidence.json", data: []byte(`{"ok":true,"coverage_ok":false}`), ok: false},
		{name: "json failed count", path: "evidence.json", data: []byte(`{"ok":true,"total":12,"failed":1}`), ok: false},
		{name: "json missing categories", path: "evidence.json", data: []byte(`{"ok":true,"total":12,"failed":0,"coverage_ok":false,"missing_categories":["blob_tx"]}`), ok: false},
		{name: "json nested result false", path: "evidence.json", data: []byte(`{"ok":true,"results":[{"ok":true},{"ok":false}]}`), ok: false},
		{name: "empty text", path: "evidence.txt", data: []byte(` `), ok: false},
		{name: "text ok", path: "evidence.txt", data: []byte(`fuzz evidence passed`), ok: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := EvidenceContentOK(testCase.path, testCase.data); got != testCase.ok {
				t.Fatalf("expected %t got %t", testCase.ok, got)
			}
		})
	}
}

func TestEvidenceSemanticValidation(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		checkName string
		path      string
		data      []byte
		ok        bool
	}{
		{name: "chaos semantic json", checkName: "chaos_evidence", path: "chaos.json", data: []byte(`{"ok":true,"summary":"chaos partition fault drill passed"}`), ok: true},
		{name: "chaos generic json rejected", checkName: "chaos_evidence", path: "chaos.json", data: []byte(`{"ok":true,"checks":[{"ok":true}]}`), ok: false},
		{name: "bls semantic text", checkName: "bls_adapter_audit", path: "bls.pdf", data: []byte(`BLS blst adapter implementation audit covered pinned dependency version review, subgroup validation, rogue-key tests, proof-of-possession, and key-validation.`), ok: true},
		{name: "bls generic audit rejected", checkName: "bls_adapter_audit", path: "bls.pdf", data: []byte(`BLS adapter audit covered dependency review, subgroup validation, rogue-key tests, proof-of-possession, and key-validation.`), ok: false},
		{name: "bls builtin reference rejected", checkName: "bls_adapter_audit", path: "bls.pdf", data: []byte(`circl-bls12381 operator-supplied-audit-required dependency subgroup rogue-key proof-of-possession key-validation version`), ok: false},
		{name: "bls audit pending rejected", checkName: "bls_adapter_audit", path: "bls.pdf", data: []byte(`BLS audit pending blst adapter dependency version subgroup rogue-key proof-of-possession key-validation`), ok: false},
		{name: "sdk conformance requires fixtures", checkName: "sdk_conformance_evidence", path: "sdk.json", data: []byte(`{"ok":true,"summary":"sdk api conformance module rpc storage crypto transport ibc relayer proof evm web3 ethereum evidence passed"}`), ok: false},
		{name: "sdk conformance fixtures accepted", checkName: "sdk_conformance_evidence", path: "sdk.json", data: []byte(`{"ok":true,"summary":"sdk api conformance module rpc storage crypto transport ibc relayer proof evm web3 ethereum raw transaction fixtures vm execution opcode passed","evm_fixtures":{"ok":true,"total":1,"passed":1,"failed":0,"coverage_ok":true,"required_categories":["raw_transaction"],"covered_categories":["raw_transaction"],"results":[{"ok":true,"name":"tx"}]},"evm_execution":{"ok":true,"total":1,"passed":1,"failed":0,"coverage_ok":true,"required_categories":["opcode"],"covered_categories":["opcode"],"results":[{"ok":true,"name":"vm"}]}}`), ok: true},
		{name: "audit generic text rejected", checkName: "external_security_audit", path: "audit.pdf", data: []byte(`evidence passed`), ok: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := EvidenceCheckContentOK(testCase.checkName, testCase.path, testCase.data); got != testCase.ok {
				t.Fatalf("expected %t got %t", testCase.ok, got)
			}
		})
	}
}

func TestTypedNetworkLongRunEvidenceValidation(t *testing.T) {
	passing := []byte(`{
		"ok":true,
		"summary":"longrun duration height validator per-node distributed soak evidence passed",
		"load":{"submitted":10,"failed":0},
		"nodes":[
			{"validator_id":"validator-1","before":{"latest_height":1},"after":{"latest_height":5},"report":{"ok":true}},
			{"validator_id":"validator-2","before":{"latest_height":2},"after":{"latest_height":6},"report":{"ok":true}}
		]
	}`)
	if !EvidenceCheckContentOK("longrun_evidence", "longrun.json", passing) {
		t.Fatalf("expected longrun evidence with height growth to pass")
	}

	noGrowth := []byte(`{
		"ok":true,
		"summary":"longrun duration height validator per-node distributed soak evidence passed",
		"load":{"submitted":10,"failed":0},
		"nodes":[{"validator_id":"validator-1","before":{"latest_height":5},"after":{"latest_height":5},"report":{"ok":true}}]
	}`)
	if EvidenceCheckContentOK("longrun_evidence", "longrun.json", noGrowth) {
		t.Fatalf("expected longrun evidence without height growth to fail")
	}

	loadFailure := []byte(`{
		"ok":true,
		"summary":"longrun duration height validator per-node distributed soak evidence passed",
		"load":{"submitted":10,"failed":1},
		"nodes":[{"validator_id":"validator-1","before":{"latest_height":1},"after":{"latest_height":5},"report":{"ok":true}}]
	}`)
	if EvidenceCheckContentOK("longrun_evidence", "longrun.json", loadFailure) {
		t.Fatalf("expected longrun evidence with failed load to fail")
	}
}

func TestTypedCollectedEvidenceValidation(t *testing.T) {
	passingSnapshot := []byte(`{
		"schema_version":"v1",
		"evidence_type":"snapshot_replay_evidence",
		"ok":true,
		"summary":"snapshot replay restore evidence passed",
		"checks":[{"name":"snapshot","ok":true}],
		"rpcs":[{"rpc":"http://127.0.0.1:26657","final":{"snapshot":{"height":10},"diagnostics":{"replay_healthy":true}}}]
	}`)
	if !EvidenceCheckContentOK("snapshot_replay_evidence", "snapshot.json", passingSnapshot) {
		t.Fatalf("expected collected snapshot evidence to pass")
	}

	failedReplay := []byte(`{
		"schema_version":"v1",
		"evidence_type":"snapshot_replay_evidence",
		"ok":true,
		"summary":"snapshot replay restore evidence passed",
		"checks":[{"name":"snapshot","ok":true}],
		"rpcs":[{"rpc":"http://127.0.0.1:26657","final":{"snapshot":{"height":10},"diagnostics":{"replay_healthy":false}}}]
	}`)
	if EvidenceCheckContentOK("snapshot_replay_evidence", "snapshot.json", failedReplay) {
		t.Fatalf("expected collected snapshot evidence with unhealthy replay to fail")
	}

	noPeers := []byte(`{
		"schema_version":"v1",
		"evidence_type":"p2p_scale_evidence",
		"ok":true,
		"summary":"p2p peer scale discovery reconnect backpressure evidence passed",
		"checks":[{"name":"p2p","ok":true}],
		"rpcs":[{"rpc":"http://127.0.0.1:26657","final":{"status":{"peer_count":0},"peers":{"peers":[]}}}]
	}`)
	if EvidenceCheckContentOK("p2p_scale_evidence", "p2p.json", noPeers) {
		t.Fatalf("expected collected p2p evidence without peers to fail")
	}

	wrongType := []byte(`{
		"schema_version":"v1",
		"evidence_type":"p2p_scale_evidence",
		"ok":true,
		"summary":"snapshot replay restore evidence passed",
		"checks":[{"name":"snapshot","ok":true}],
		"rpcs":[{"rpc":"http://127.0.0.1:26657","final":{"snapshot":{"height":10},"diagnostics":{"replay_healthy":true}}}]
	}`)
	if EvidenceCheckContentOK("snapshot_replay_evidence", "snapshot.json", wrongType) {
		t.Fatalf("expected evidence_type mismatch to fail")
	}
}

func TestBuildValidatesEvidenceManifestHash(t *testing.T) {
	chaos := []byte(`{"ok":true,"summary":"chaos partition fault drill passed"}`)
	sum := sha256.Sum256(chaos)
	manifest, err := json.Marshal(EvidenceManifest{
		SchemaVersion: "v1",
		Evidence: []EvidenceManifestEntry{{
			Name:          "chaos_evidence",
			Path:          "chaos.json",
			SHA256:        hex.EncodeToString(sum[:]),
			SchemaVersion: "v1",
			Provenance:    "test harness",
			Signature:     "test-signature",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	document := Build("rc", Pack{OK: false}, Evidence{
		Manifest: "evidence-manifest.json",
		Chaos:    "chaos.json",
		Exists: func(path string) bool {
			return path == "chaos.json" || path == "evidence-manifest.json"
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case "chaos.json":
				return chaos, nil
			case "evidence-manifest.json":
				return manifest, nil
			default:
				return nil, nil
			}
		},
	})
	if !releaseGateCheckOK(document.Checks, "evidence_manifest") || !releaseGateCheckOK(document.Checks, "chaos_evidence") {
		t.Fatalf("expected manifest-bound chaos evidence to pass: %+v", document.Checks)
	}
}

func TestBuildRejectsEvidenceManifestMismatch(t *testing.T) {
	manifest, err := json.Marshal(EvidenceManifest{
		SchemaVersion: "v1",
		Evidence: []EvidenceManifestEntry{{
			Name:          "chaos_evidence",
			Path:          "chaos.json",
			SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SchemaVersion: "v1",
			Provenance:    "test harness",
			Signature:     "test-signature",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	document := Build("rc", Pack{OK: false}, Evidence{
		Manifest: "evidence-manifest.json",
		Chaos:    "chaos.json",
		Exists: func(path string) bool {
			return path == "chaos.json" || path == "evidence-manifest.json"
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case "chaos.json":
				return []byte(`{"ok":true,"summary":"chaos partition fault drill passed"}`), nil
			case "evidence-manifest.json":
				return manifest, nil
			default:
				return nil, nil
			}
		},
	})
	if releaseGateCheckOK(document.Checks, "chaos_evidence") {
		t.Fatalf("expected manifest hash mismatch to fail chaos evidence: %+v", document.Checks)
	}
}

func TestBuildRequiresAttestedEvidenceManifestForPublicRelease(t *testing.T) {
	evidence := completeReleaseGateEvidence(false)
	files := completeReleaseGateEvidenceFiles(false)
	files["evidence-manifest.json"] = releaseGateManifestWithoutAttestation(files)
	evidence.ReadFile = func(path string) ([]byte, error) {
		return files[path], nil
	}
	document := Build("v1.2.3", Pack{OK: true}, evidence)
	if document.OK || releaseGateCheckOK(document.Checks, "evidence_manifest_attestation") {
		t.Fatalf("expected public release to reject unattested evidence manifest: %+v", document.Checks)
	}
	document = Build("v1.2.3-rc1", Pack{OK: true}, evidence)
	if !releaseGateCheckOK(document.Checks, "evidence_manifest_attestation") {
		t.Fatalf("expected private/RC release to allow unattested manifest during evidence collection: %+v", document.Checks)
	}
}

func TestBuildRejectsTamperedEvidenceManifestSignature(t *testing.T) {
	evidence := completeReleaseGateEvidence(false)
	files := completeReleaseGateEvidenceFiles(false)
	var manifest EvidenceManifest
	if err := json.Unmarshal(releaseGateManifestForEvidence(files), &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Evidence[0].Signature = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0}, ed25519.SignatureSize))
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["evidence-manifest.json"] = data
	evidence.ReadFile = func(path string) ([]byte, error) {
		return files[path], nil
	}
	document := Build("v1.2.3", Pack{OK: true}, evidence)
	if document.OK || releaseGateCheckOK(document.Checks, "evidence_manifest_attestation") {
		t.Fatalf("expected public release to reject tampered evidence signature: %+v", document.Checks)
	}
}

func checkOK(checks []Check, name string) bool {
	return releaseGateCheckOK(checks, name)
}

func releaseGateCheckOK(checks []Check, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return check.OK
		}
	}
	return false
}

func completeReleaseGateEvidence(skipExternal bool) Evidence {
	files := completeReleaseGateEvidenceFiles(skipExternal)
	files["evidence-manifest.json"] = releaseGateManifestForEvidence(files)
	evidence := Evidence{
		Manifest:             "evidence-manifest.json",
		Chaos:                "chaos.json",
		KMS:                  "kms.json",
		Snapshot:             "snapshot.json",
		P2PScale:             "p2p.json",
		StateSyncLightClient: "state-sync-light-client.json",
		ValidatorEconomics:   "validator-economics.json",
		UpgradeGovernance:    "upgrade-governance.json",
		MEVFeeMarket:         "mev-fee-market.json",
		OpsRunbook:           "ops-runbook.json",
		FormalSafety:         "formal-safety.json",
		SDKConformance:       "sdk-conformance.json",
		Exists: func(path string) bool {
			_, found := files[path]
			return found
		},
		ReadFile: func(path string) ([]byte, error) {
			return files[path], nil
		},
	}
	if !skipExternal {
		evidence.ExternalAudit = "audit.pdf"
		evidence.BLSAudit = "bls.pdf"
		evidence.VRFAudit = "vrf.pdf"
	}
	return evidence
}

func completeReleaseGateEvidenceFiles(skipExternal bool) map[string][]byte {
	files := map[string][]byte{
		"chaos.json":                   []byte(`{"ok":true,"summary":"chaos partition fault drill passed"}`),
		"kms.json":                     semanticEvidenceContentForPath("kms.json"),
		"snapshot.json":                semanticEvidenceContentForPath("snapshot.json"),
		"p2p.json":                     semanticEvidenceContentForPath("p2p.json"),
		"state-sync-light-client.json": semanticEvidenceContentForPath("state-sync-light-client.json"),
		"validator-economics.json":     semanticEvidenceContentForPath("validator-economics.json"),
		"upgrade-governance.json":      semanticEvidenceContentForPath("upgrade-governance.json"),
		"mev-fee-market.json":          semanticEvidenceContentForPath("mev-fee-market.json"),
		"ops-runbook.json":             semanticEvidenceContentForPath("ops-runbook.json"),
		"formal-safety.json":           semanticEvidenceContentForPath("formal-safety.json"),
		"sdk-conformance.json":         semanticEvidenceContentForPath("sdk-conformance.json"),
	}
	if !skipExternal {
		files["audit.pdf"] = semanticEvidenceContentForPath("audit.pdf")
		files["bls.pdf"] = semanticEvidenceContentForPath("bls.pdf")
		files["vrf.pdf"] = semanticEvidenceContentForPath("vrf.pdf")
	}
	return files
}

func releaseGateManifestForEvidence(files map[string][]byte) []byte {
	entries := make([]EvidenceManifestEntry, 0, len(files))
	for path, data := range files {
		if path == "evidence-manifest.json" {
			continue
		}
		sum := sha256.Sum256(data)
		entries = append(entries, EvidenceManifestEntry{
			Name:          releaseGateEvidenceName(path),
			Path:          path,
			SHA256:        hex.EncodeToString(sum[:]),
			SchemaVersion: "v1",
			Provenance:    "test harness",
		})
		last := len(entries) - 1
		signReleaseGateEvidenceEntry(&entries[last])
	}
	encoded, _ := json.Marshal(EvidenceManifest{SchemaVersion: "v1", Evidence: entries})
	return encoded
}

func signReleaseGateEvidenceEntry(entry *EvidenceManifestEntry) {
	publicKey := releaseGateTestPrivateKey.Public().(ed25519.PublicKey)
	signature := ed25519.Sign(releaseGateTestPrivateKey, EvidenceManifestEntrySigningMessage(*entry))
	entry.SignatureAlgo = evidenceManifestSignatureAlgorithmEd25519
	entry.SignaturePubKey = base64.StdEncoding.EncodeToString(publicKey)
	entry.Signature = base64.StdEncoding.EncodeToString(signature)
}

func releaseGateManifestWithoutAttestation(files map[string][]byte) []byte {
	entries := make([]EvidenceManifestEntry, 0, len(files))
	for path, data := range files {
		if path == "evidence-manifest.json" {
			continue
		}
		sum := sha256.Sum256(data)
		entries = append(entries, EvidenceManifestEntry{
			Name:   releaseGateEvidenceName(path),
			Path:   path,
			SHA256: hex.EncodeToString(sum[:]),
		})
	}
	encoded, _ := json.Marshal(EvidenceManifest{SchemaVersion: "v1", Evidence: entries})
	return encoded
}

func releaseGateEvidenceName(path string) string {
	switch path {
	case "chaos.json":
		return "chaos_evidence"
	case "kms.json":
		return "kms_signer_evidence"
	case "snapshot.json":
		return "snapshot_replay_evidence"
	case "p2p.json":
		return "p2p_scale_evidence"
	case "state-sync-light-client.json":
		return "state_sync_light_client_evidence"
	case "validator-economics.json":
		return "validator_economics_evidence"
	case "upgrade-governance.json":
		return "upgrade_governance_evidence"
	case "mev-fee-market.json":
		return "mev_fee_market_evidence"
	case "ops-runbook.json":
		return "ops_runbook_evidence"
	case "formal-safety.json":
		return "formal_safety_evidence"
	case "sdk-conformance.json":
		return "sdk_conformance_evidence"
	case "audit.pdf":
		return "external_security_audit"
	case "bls.pdf":
		return "bls_adapter_audit"
	case "vrf.pdf":
		return "vrf_adapter_audit"
	default:
		return path
	}
}

func semanticEvidenceContentForPath(path string) []byte {
	switch path {
	case "kms.json":
		return []byte(`{"ok":true,"summary":"KMS signer policy double-sign guard nonce replay audit evidence passed"}`)
	case "snapshot.json":
		return []byte(`{"ok":true,"summary":"snapshot replay restore consistency evidence passed"}`)
	case "p2p.json":
		return []byte(`{"ok":true,"summary":"p2p peer scale discovery reconnect backpressure evidence passed"}`)
	case "state-sync-light-client.json":
		return []byte(`{"ok":true,"summary":"state-sync light-client finality proof evidence passed"}`)
	case "validator-economics.json":
		return []byte(`{"ok":true,"summary":"validator slashing reward commission unbonding jail custody stake-accounting tombstone false-slashing evidence passed"}`)
	case "upgrade-governance.json":
		return []byte(`{"ok":true,"summary":"upgrade governance migration rollback halt allow_noop no-op authority rollback-required last-safe-height evidence passed"}`)
	case "mev-fee-market.json":
		return []byte(`{"ok":true,"summary":"mev fee market fair mempool ordering replacement evidence passed"}`)
	case "ops-runbook.json":
		return []byte(`{"ok":true,"summary":"ops runbook alert incident metrics evidence passed"}`)
	case "formal-safety.json":
		return []byte(`{"ok":true,"summary":"safety invariant adversarial property proof evidence passed"}`)
	case "sdk-conformance.json":
		return []byte(`{"ok":true,"summary":"sdk api conformance module rpc storage crypto transport ibc relayer proof evm web3 ethereum raw transaction fixtures vm execution opcode evidence passed","evm_fixtures":{"ok":true,"total":1,"passed":1,"failed":0,"coverage_ok":true,"required_categories":["raw_transaction"],"covered_categories":["raw_transaction"],"results":[{"ok":true,"name":"tx"}]},"evm_execution":{"ok":true,"total":1,"passed":1,"failed":0,"coverage_ok":true,"required_categories":["opcode"],"covered_categories":["opcode"],"results":[{"ok":true,"name":"vm"}]}}`)
	case "audit.pdf":
		return []byte(`external security audit disposition evidence passed`)
	case "bls.pdf":
		return []byte(`bls blst adapter implementation audit pinned dependency version subgroup rogue-key proof-of-possession key-validation evidence passed`)
	case "vrf.pdf":
		return []byte(`vrf remote adapter implementation ecvrf audit dependency tls mtls certificate auth token nonce replay custody kms hsm evidence passed`)
	default:
		return []byte(`{"ok":true,"summary":"longrun duration height validator soak evidence passed"}`)
	}
}

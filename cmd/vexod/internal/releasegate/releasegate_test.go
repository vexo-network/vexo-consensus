package releasegate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestBuildPassesWithAllEvidence(t *testing.T) {
	document := Build("v1.2.3", Pack{
		OK: true,
		Checks: []PackCheck{
			{Name: "manifest", OK: true, Message: "manifest exists"},
		},
	}, Evidence{
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
		ExternalAudit:        "audit.json",
		BLSAudit:             "bls.json",
		Exists: func(path string) bool {
			return path != ""
		},
	})

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
	document := Build("rc", Pack{OK: true}, Evidence{
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
		AllowExternalPending: true,
		Exists: func(path string) bool {
			return path != ""
		},
	})

	if !document.OK {
		t.Fatalf("expected pending external checks to pass for private RCs: %+v", document)
	}
}

func TestBuildFailsInvalidEvidenceContent(t *testing.T) {
	document := Build("rc", Pack{OK: true}, Evidence{
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
		ExternalAudit:        "audit.pdf",
		BLSAudit:             "bls.pdf",
		Exists: func(path string) bool {
			return path != ""
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "chaos.json" {
				return []byte(`{"ok":false,"checks":[{"ok":false}]}`), nil
			}
			return semanticEvidenceContentForPath(path), nil
		},
	})

	if document.OK {
		t.Fatalf("expected release gate to fail invalid evidence")
	}
	if checkOK(document.Checks, "chaos_evidence") {
		t.Fatalf("expected chaos evidence check to fail: %+v", document.Checks)
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
		{name: "bls semantic text", checkName: "bls_adapter_audit", path: "bls.pdf", data: []byte(`BLS adapter audit covered dependency review, subgroup validation, rogue-key tests, proof-of-possession, and key-validation.`), ok: true},
		{name: "audit generic text rejected", checkName: "external_security_audit", path: "audit.pdf", data: []byte(`evidence passed`), ok: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := EvidenceCheckContentOK(testCase.checkName, testCase.path, testCase.data); got != testCase.ok {
				t.Fatalf("expected %t got %t", testCase.ok, got)
			}
		})
	}
}

func TestBuildValidatesEvidenceManifestHash(t *testing.T) {
	chaos := []byte(`{"ok":true,"summary":"chaos partition fault drill passed"}`)
	sum := sha256.Sum256(chaos)
	manifest, err := json.Marshal(EvidenceManifest{
		SchemaVersion: "v1",
		Evidence: []EvidenceManifestEntry{{
			Name:   "chaos_evidence",
			Path:   "chaos.json",
			SHA256: hex.EncodeToString(sum[:]),
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
			Name:   "chaos_evidence",
			Path:   "chaos.json",
			SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
		return []byte(`{"ok":true,"summary":"sdk api conformance module rpc storage crypto transport evm web3 ethereum evidence passed"}`)
	case "audit.pdf":
		return []byte(`external security audit disposition evidence passed`)
	case "bls.pdf":
		return []byte(`bls adapter audit dependency subgroup rogue-key proof-of-possession key-validation evidence passed`)
	default:
		return []byte(`{"ok":true,"summary":"longrun duration height validator soak evidence passed"}`)
	}
}

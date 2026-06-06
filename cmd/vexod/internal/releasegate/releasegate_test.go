package releasegate

import "testing"

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
			if path == "audit.pdf" || path == "bls.pdf" {
				return []byte("external evidence passed"), nil
			}
			return []byte(`{"ok":true,"checks":[{"ok":true}]}`), nil
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

func checkOK(checks []Check, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return check.OK
		}
	}
	return false
}

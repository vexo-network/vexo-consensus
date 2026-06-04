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

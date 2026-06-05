package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/cmd/vexod/internal/releasegate"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestRunCommandHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		var stdout bytes.Buffer
		if err := runCommand(&stdout, &bytes.Buffer{}, args); err != nil {
			t.Fatal(err)
		}
		output := stdout.String()
		for _, expected := range []string{"Usage:", "init", "config paths", "start", "network", "consensus", "snapshot", "doctor", "version", "Module Commands:", "bank tx mint", "bank query balance"} {
			if !strings.Contains(output, expected) {
				t.Fatalf("expected help output to contain %q, got:\n%s", expected, output)
			}
		}
	}

	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "vexod "+version) || !strings.Contains(stdout.String(), "commit: ") || !strings.Contains(stdout.String(), "build_date: ") {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestRunConsensusAdversarial(t *testing.T) {
	var output bytes.Buffer
	if err := runConsensus(&output, []string{"adversarial"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"consensus adversarial simulation", "safety_ok: true", "scenario: offline_minority", "scenario: split_partition_no_dual_quorum"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected adversarial output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunConsensusAdversarialJSON(t *testing.T) {
	var output bytes.Buffer
	if err := runConsensus(&output, []string{"adversarial", "--json"}); err != nil {
		t.Fatal(err)
	}
	var report struct {
		SafetyOK  bool `json:"safety_ok"`
		Scenarios []struct {
			Name string `json:"name"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.SafetyOK || len(report.Scenarios) == 0 {
		t.Fatalf("unexpected adversarial JSON report: %+v", report)
	}
}

func TestRunCommandDispatchesModuleCLI(t *testing.T) {
	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"bank", "tx", "send", "alice", "bob", "25"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "tx: bank:send:alice:bob:25" {
		t.Fatalf("unexpected module cli output: %q", stdout.String())
	}
}

func TestRunCommandShowsModuleHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"bank", "--help"}); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, expected := range []string{"bank module commands", "Usage:", "Commands:", "tx", "query", "bank tx mint"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected module help to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunCommandShowsNestedModuleHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"bank", "tx", "mint", "--help"}); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, expected := range []string{"build a mint transaction payload", "Arguments:", "to", "amount", "bank tx mint <to> <amount>"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected nested module help to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunCommandReportsModuleCLIError(t *testing.T) {
	var stderr bytes.Buffer
	if err := runCommand(&bytes.Buffer{}, &stderr, []string{"bank", "tx", "mint", "alice", "0"}); err == nil {
		t.Fatal("expected module cli error")
	}
	if !strings.Contains(stderr.String(), "bank failed") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunCommandRejectsUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	if err := runCommand(&bytes.Buffer{}, &stderr, []string{"nope"}); err == nil {
		t.Fatal("expected unknown command error")
	}
	if !strings.Contains(stderr.String(), "unknown command") || !strings.Contains(stderr.String(), "vexod help") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunOpsThresholdsAndAlerts(t *testing.T) {
	var thresholds bytes.Buffer
	if err := runCommand(&thresholds, &bytes.Buffer{}, []string{"ops", "thresholds", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(thresholds.String(), "min_height_rate_per_minute") {
		t.Fatalf("unexpected thresholds output:\n%s", thresholds.String())
	}

	var alerts bytes.Buffer
	if err := runCommand(&alerts, &bytes.Buffer{}, []string{
		"ops", "alerts",
		"--height-rate", "0",
		"--round-timeouts", "10",
		"--proposal-latency", "1s",
		"--vote-latency", "1s",
		"--peer-bans", "1",
		"--mempool-size", "20000",
		"--commit-latency", "2s",
		"--signing-failures", "1",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(alerts.String(), "ops alerts alert") || !strings.Contains(alerts.String(), "height_rate") {
		t.Fatalf("unexpected alerts output:\n%s", alerts.String())
	}

	previousMetrics := filepath.Join(t.TempDir(), "previous.json")
	currentMetrics := filepath.Join(t.TempDir(), "current.json")
	if err := os.WriteFile(previousMetrics, []byte(`{"latest_height":10,"round_timeouts":1,"snapshot_healthy":true,"replay_healthy":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentMetrics, []byte(`{"latest_height":16,"round_timeouts":2,"snapshot_healthy":true,"replay_healthy":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var metricsAlerts bytes.Buffer
	if err := runCommand(&metricsAlerts, &bytes.Buffer{}, []string{
		"ops", "alerts",
		"--metrics-file", currentMetrics,
		"--previous-metrics-file", previousMetrics,
		"--window", "2m",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metricsAlerts.String(), "ops alerts ok") {
		t.Fatalf("unexpected metrics-file alerts output:\n%s", metricsAlerts.String())
	}

	var incident bytes.Buffer
	if err := runCommand(&incident, &bytes.Buffer{}, []string{
		"ops", "incident",
		"--metrics-file", currentMetrics,
		"--previous-metrics-file", previousMetrics,
		"--window", "1m",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(incident.String(), "ops incident report") || !strings.Contains(incident.String(), "severity: none") {
		t.Fatalf("unexpected incident output:\n%s", incident.String())
	}
}

func TestRunUpgradePlan(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"upgrade", "plan",
		"--name", "v0.2.0",
		"--height", "100",
		"--binary-version", "v0.2.0",
		"--config-from", "1",
		"--config-to", "2",
		"--store-from", "1",
		"--store-to", "2",
		"--app-from", "1",
		"--app-to", "2",
		"--proposal", "42",
		"--rollback-binary", "v0.1.0",
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"upgrade plan", "height: 100", "config_schema: 1 -> 2", "governance_proposal: 42", "rollback_binary: v0.1.0"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected upgrade output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunUpgradeApplyPersistsExecutionRecord(t *testing.T) {
	home := t.TempDir()
	planFile := filepath.Join(home, "plan.json")
	recordFile := filepath.Join(home, "records.json")
	plan := []byte(`{
		"name":"v0.2.0",
		"height":100,
		"binary_version":"v0.2.0",
		"config_schema_from":1,
		"config_schema_to":2,
		"store_schema_from":1,
		"store_schema_to":2,
		"app_state_schema_from":1,
		"app_state_schema_to":2
	}`)
	if err := os.WriteFile(planFile, plan, 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"upgrade", "apply",
		"--plan-file", planFile,
		"--record-file", recordFile,
		"--height", "100",
		"--binary-version", "v0.1.0",
		"--config-version", "1",
		"--store-version", "1",
		"--app-version", "1",
		"--allow-empty-migrations",
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"upgrade apply applied", "binary_version: v0.1.0 -> v0.2.0", "store_schema: 1 -> 2"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected upgrade apply output to contain %q, got:\n%s", expected, output.String())
		}
	}
	record, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(record), `"status": "applied"`) || !strings.Contains(string(record), `"v0.2.0"`) {
		t.Fatalf("unexpected record file:\n%s", string(record))
	}
}

func TestRunTxBuildAndParseCanonicalPayload(t *testing.T) {
	var buildOutput bytes.Buffer
	if err := runCommand(&buildOutput, &bytes.Buffer{}, []string{
		"tx", "build",
		"--module", "bank",
		"--action", "send",
		"--args", "alice,bob,1",
		"--tags", "nonce=7,signer=alice,gas=100,fee=2",
	}); err != nil {
		t.Fatal(err)
	}
	expectedTx := "tx: bank:send:alice:bob:1:fee=2:gas=100:signer=alice:nonce=7"
	if strings.TrimSpace(buildOutput.String()) != expectedTx {
		t.Fatalf("expected %q, got %q", expectedTx, strings.TrimSpace(buildOutput.String()))
	}

	var parseOutput bytes.Buffer
	if err := runCommand(&parseOutput, &bytes.Buffer{}, []string{
		"tx", "parse",
		"--tx", strings.TrimPrefix(strings.TrimSpace(buildOutput.String()), "tx: "),
		"--json",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parseOutput.String(), `"module": "bank"`) || !strings.Contains(parseOutput.String(), `"nonce": "7"`) {
		t.Fatalf("unexpected tx parse output:\n%s", parseOutput.String())
	}
}

func TestRunUpgradeRollbackPlan(t *testing.T) {
	home := t.TempDir()
	planFile := filepath.Join(home, "plan.json")
	plan := []byte(`{
		"name":"v0.2.0",
		"height":100,
		"binary_version":"v0.2.0",
		"config_schema_from":1,
		"config_schema_to":2,
		"store_schema_from":1,
		"store_schema_to":2,
		"app_state_schema_from":1,
		"app_state_schema_to":2,
		"rollback_binary":"v0.1.0"
	}`)
	if err := os.WriteFile(planFile, plan, 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"upgrade", "rollback-plan",
		"--plan-file", planFile,
		"--last-safe-height", "99",
		"--snapshot", filepath.Join(home, "snapshot.json"),
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"upgrade rollback plan", "upgrade_height: 100", "rollback_binary: v0.1.0", "last_safe_height: 99", "rollback_binary ok=true", "restore state snapshot", "light-client finality proofs"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected rollback plan output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunUpgradeRollbackPlanJSONWarnsOnUnsafeInputs(t *testing.T) {
	home := t.TempDir()
	planFile := filepath.Join(home, "plan.json")
	plan := []byte(`{
		"name":"unsafe-upgrade",
		"height":100,
		"binary_version":"v0.2.0",
		"config_schema_from":1,
		"config_schema_to":1,
		"store_schema_from":1,
		"store_schema_to":1,
		"app_state_schema_from":1,
		"app_state_schema_to":1
	}`)
	if err := os.WriteFile(planFile, plan, 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"upgrade", "rollback-plan",
		"--plan-file", planFile,
		"--last-safe-height", "100",
		"--json",
	}); err != nil {
		t.Fatal(err)
	}
	var document upgradeRollbackPlanDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "v1" || document.PlanName != "unsafe-upgrade" || len(document.Warnings) < 3 {
		t.Fatalf("unexpected rollback document: %+v", document)
	}
	if document.Checks[0].OK || document.Checks[1].OK || document.Checks[2].OK {
		t.Fatalf("expected unsafe rollback checks to fail: %+v", document.Checks)
	}
}

func TestRunSlashingLifecyclePlan(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"slashing", "lifecycle-plan", "--type", "double_sign", "--validator", "validator-1", "--height", "10", "--current-height", "200", "--current-power", "100"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"slashing lifecycle plan", "type: double_sign", "plan_only: false", "validator: validator-1", "power: 100 -> 95", "runtime_proof_verifier ok=true", "stake slash"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected slashing lifecycle output to contain %q, got:\n%s", expected, output.String())
		}
	}

	var jsonOutput bytes.Buffer
	if err := runCommand(&jsonOutput, &bytes.Buffer{}, []string{"slashing", "lifecycle-plan", "--type", "double_sign", "--validator", "validator-1", "--height", "10", "--current-height", "11", "--json"}); err != nil {
		t.Fatal(err)
	}
	var document slashingLifecyclePlanDocument
	if err := json.Unmarshal(jsonOutput.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "v1" || document.PlanOnly || len(document.Warnings) == 0 {
		t.Fatalf("expected early penalty warning, got %+v", document)
	}

	var finalityOutput bytes.Buffer
	if err := runCommand(&finalityOutput, &bytes.Buffer{}, []string{"slashing", "lifecycle-plan", "--type", "finality_conflict", "--validator", "validator-1", "--height", "10", "--current-height", "200", "--current-power", "100"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"type: finality_conflict", "plan_only: false", "runtime_proof_verifier ok=true"} {
		if !strings.Contains(finalityOutput.String(), expected) {
			t.Fatalf("expected finality slashing output to contain %q, got:\n%s", expected, finalityOutput.String())
		}
	}
}

func TestRunReleasePackWritesAuditManifest(t *testing.T) {
	dist := t.TempDir()
	files := map[string]string{
		"vexod-test-linux-amd64": "binary",
		"checksums.txt":          "checksum",
		"sbom-go-modules.json":   `{"Path":"github.com/vexo-network/vexo-consensus"}`,
		"sbom-go-version.txt":    "go version test",
		"release-manifest.json":  `{"version":"test"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outputPath := filepath.Join(t.TempDir(), "release-audit-pack.json")
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"release", "pack",
		"--dist", dist,
		"--version", "test",
		"--output", outputPath,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "release audit pack written") || !strings.Contains(output.String(), "ok: true") {
		t.Fatalf("unexpected release pack output:\n%s", output.String())
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var document releaseAuditPack
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "v1" || !document.OK || len(document.Artifacts) != len(files) || len(document.Audit.Commands) == 0 {
		t.Fatalf("unexpected release audit pack: %+v", document)
	}
}

func TestRunReleasePackCanRequireSignature(t *testing.T) {
	dist := t.TempDir()
	for _, name := range []string{"checksums.txt", "sbom-go-modules.json", "sbom-go-version.txt", "release-manifest.json"} {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"release", "pack",
		"--dist", dist,
		"--version", "test",
		"--require-signature",
	}); err != nil {
		t.Fatal(err)
	}
	var document releaseAuditPack
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.OK || document.Required.SignatureFound {
		t.Fatalf("expected unsigned release to fail signature check: %+v", document)
	}
}

func TestRunReleasePackIncludesEvidenceFiles(t *testing.T) {
	dist := t.TempDir()
	for _, name := range []string{"checksums.txt", "sbom-go-modules.json", "sbom-go-version.txt", "release-manifest.json"} {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	longrun := filepath.Join(dist, "longrun-evidence.json")
	adversarial := filepath.Join(dist, "adversarial-evidence.json")
	fuzz := filepath.Join(dist, "fuzz-evidence.txt")
	for _, path := range []string{longrun, adversarial, fuzz} {
		if err := os.WriteFile(path, []byte("evidence"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"release", "pack",
		"--dist", dist,
		"--version", "test",
		"--longrun-evidence", longrun,
		"--adversarial-evidence", adversarial,
		"--fuzz-evidence", fuzz,
	}); err != nil {
		t.Fatal(err)
	}
	var document releaseAuditPack
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if !document.OK || document.Required.LongRunEvidence != "longrun-evidence.json" || document.Required.AdversarialEvidence != "adversarial-evidence.json" || document.Required.FuzzEvidence != "fuzz-evidence.txt" {
		t.Fatalf("unexpected release evidence pack: %+v", document)
	}
	if !releaseCheckOK(document, "longrun_evidence") || !releaseCheckOK(document, "adversarial_evidence") || !releaseCheckOK(document, "fuzz_evidence") {
		t.Fatalf("expected evidence checks ok: %+v", document.Checks)
	}
}

func TestRunReleaseLaunchChecklist(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"release", "launch-checklist"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"release launch checklist", "prelaunch:", "release-candidate:", "genesis:", "launch-window:", "postlaunch:", "remote signer double-sign guard"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected launch checklist output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunReleaseLaunchChecklistJSON(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"release", "launch-checklist", "--json"}); err != nil {
		t.Fatal(err)
	}
	var document launchChecklistDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "v1" || len(document.Phases) != 5 {
		t.Fatalf("unexpected launch checklist document: %+v", document)
	}
	if document.Phases[0].Name != "prelaunch" || !strings.Contains(strings.Join(document.Phases[0].Items, "\n"), "network scale-plan") {
		t.Fatalf("unexpected prelaunch phase: %+v", document.Phases[0])
	}
}

func TestRunReleaseReadiness(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"release", "readiness"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"release readiness sweep", "ok: true", "state_sync_drill", "slashing_lifecycle", "observability_incident"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected readiness output to contain %q, got:\n%s", expected, output.String())
		}
	}

	var jsonOutput bytes.Buffer
	if err := runCommand(&jsonOutput, &bytes.Buffer{}, []string{"release", "readiness", "--json"}); err != nil {
		t.Fatal(err)
	}
	var document productionReadinessDocument
	if err := json.Unmarshal(jsonOutput.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if !document.OK || len(document.Commands) == 0 || len(document.Documents) == 0 {
		t.Fatalf("unexpected readiness document: %+v", document)
	}
}

func TestRunReleaseGateRequiresOperationalEvidence(t *testing.T) {
	dist := t.TempDir()
	for _, name := range []string{"checksums.txt", "checksums.txt.asc", "sbom-go-modules.json", "sbom-go-version.txt", "release-manifest.json"} {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"release", "gate",
		"--dist", dist,
		"--version", "test",
		"--json",
	}); err != nil {
		t.Fatal(err)
	}
	var document releaseGateDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.OK || releaseReadinessCheckOK(document.Checks, "chaos_evidence") {
		t.Fatalf("expected release gate to fail missing evidence: %+v", document)
	}
}

func TestRunReleaseGatePassesWithEvidence(t *testing.T) {
	dist := t.TempDir()
	required := []string{
		"checksums.txt",
		"checksums.txt.asc",
		"sbom-go-modules.json",
		"sbom-go-version.txt",
		"release-manifest.json",
		"longrun-evidence.json",
		"adversarial-evidence.json",
		"fuzz-evidence.txt",
		"chaos-evidence.json",
		"kms-evidence.json",
		"snapshot-replay-evidence.json",
		"p2p-scale-evidence.json",
		"state-sync-light-client-evidence.json",
		"validator-economics-evidence.json",
		"upgrade-governance-evidence.json",
		"mev-fee-market-evidence.json",
		"ops-runbook-evidence.json",
		"formal-safety-evidence.json",
		"sdk-conformance-evidence.json",
		"external-audit.pdf",
		"bls-audit.pdf",
	}
	for _, name := range required {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"release", "gate",
		"--dist", dist,
		"--version", "test",
		"--longrun-evidence", filepath.Join(dist, "longrun-evidence.json"),
		"--chaos-evidence", filepath.Join(dist, "chaos-evidence.json"),
		"--adversarial-evidence", filepath.Join(dist, "adversarial-evidence.json"),
		"--fuzz-evidence", filepath.Join(dist, "fuzz-evidence.txt"),
		"--kms-evidence", filepath.Join(dist, "kms-evidence.json"),
		"--snapshot-evidence", filepath.Join(dist, "snapshot-replay-evidence.json"),
		"--p2p-scale-evidence", filepath.Join(dist, "p2p-scale-evidence.json"),
		"--state-sync-light-client-evidence", filepath.Join(dist, "state-sync-light-client-evidence.json"),
		"--validator-economics-evidence", filepath.Join(dist, "validator-economics-evidence.json"),
		"--upgrade-governance-evidence", filepath.Join(dist, "upgrade-governance-evidence.json"),
		"--mev-fee-market-evidence", filepath.Join(dist, "mev-fee-market-evidence.json"),
		"--ops-runbook-evidence", filepath.Join(dist, "ops-runbook-evidence.json"),
		"--formal-safety-evidence", filepath.Join(dist, "formal-safety-evidence.json"),
		"--sdk-conformance-evidence", filepath.Join(dist, "sdk-conformance-evidence.json"),
		"--external-audit", filepath.Join(dist, "external-audit.pdf"),
		"--bls-audit", filepath.Join(dist, "bls-audit.pdf"),
		"--json",
	}); err != nil {
		t.Fatal(err)
	}
	var document releaseGateDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if !document.OK || !releaseReadinessCheckOK(document.Checks, "external_security_audit") || !releaseReadinessCheckOK(document.Checks, "bls_adapter_audit") {
		t.Fatalf("expected release gate to pass: %+v", document)
	}
}

func releaseCheckOK(document releaseAuditPack, name string) bool {
	for _, check := range document.Checks {
		if check.Name == name {
			return check.OK
		}
	}
	return false
}

func releaseReadinessCheckOK(checks []releasegate.Check, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return check.OK
		}
	}
	return false
}

func TestRunProofVerifyCommand(t *testing.T) {
	proof := queryproof.Proof{
		SchemaVersion: queryproof.SchemaVersionV1,
		ChainID:       "vexo-test",
		Height:        7,
		Namespace:     "bank",
		Key:           []byte("alice"),
		Value:         []byte("100"),
		Exists:        true,
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Set(context.Background(), proof.Namespace, proof.Key, proof.Value); err != nil {
		t.Fatal(err)
	}
	built, err := queryproof.Build(context.Background(), storage, proof.ChainID, proof.Height, proof.Namespace, proof.Key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "proof.json")
	encoded, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"proof", "verify", "--input", path, "--chain-id", "vexo-test", "--height", "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "query proof verified") {
		t.Fatalf("unexpected proof output: %s", output.String())
	}
}

func TestRunProofVerifyIBCCommand(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(resolveConfigPath(home, ""))
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	proof, err := queryproof.Build(context.Background(), storage, "counterparty", 9, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ibckeeper.NewKeeper(storage).SetClient(context.Background(), ibckeeper.ClientState{
		ClientID:         "07-vexo-0",
		ChainID:          "counterparty",
		LatestHeight:     9,
		ValidatorSetHash: types.Hash{1},
		LatestStateRoot:  proof.StateRoot,
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ibc-proof.json")
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"proof", "verify-ibc", "--home", home, "--client-id", "07-vexo-0", "--input", path}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "IBC proof verified") || !strings.Contains(output.String(), "client_id: 07-vexo-0") {
		t.Fatalf("unexpected IBC proof output: %s", output.String())
	}
}

func TestRunIBCPacketSendCommand(t *testing.T) {
	var output bytes.Buffer
	hash := strings.Repeat("01", 32)
	root := strings.Repeat("02", 32)
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "tx", "client-update",
		"07-vexo-0", "6", hash, root,
		"--fee", "1", "--gas", "1000", "--signer", "relayer", "--nonce", "1",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:client-update:07-vexo-0:6:"+hash+":"+root+":fee=1:gas=1000:signer=relayer:nonce=1") {
		t.Fatalf("unexpected ibc client update output: %s", output.String())
	}
	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "tx", "connection-open-init",
		"connection-0", "07-vexo-0", "connection-1",
		"--fee", "1", "--gas", "1000", "--signer", "relayer", "--nonce", "1",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:connection-open-init:connection-0:07-vexo-0:connection-1:fee=1:gas=1000:signer=relayer:nonce=1") {
		t.Fatalf("unexpected ibc connection init output: %s", output.String())
	}
	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "tx", "channel-open-confirm",
		"transfer", "channel-0",
		"--fee", "1", "--gas", "1000", "--signer", "relayer", "--nonce", "1",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:channel-open-confirm:transfer:channel-0:fee=1:gas=1000:signer=relayer:nonce=1") {
		t.Fatalf("unexpected ibc channel confirm output: %s", output.String())
	}
	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "tx", "packet-send",
		"1", "transfer", "channel-0", "transfer", "channel-1", "payload",
		"--fee", "1", "--gas", "1000", "--signer", "relayer", "--nonce", "1",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-send:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:fee=1:gas=1000:signer=relayer:nonce=1") {
		t.Fatalf("unexpected ibc tx output: %s", output.String())
	}
	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "tx", "packet-ack",
		"1", "transfer", "channel-0", "transfer", "channel-1", "payload", "ack",
		"--fee", "1", "--gas", "1000", "--signer", "relayer", "--nonce", "2",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-ack:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr:fee=1:gas=1000:signer=relayer:nonce=2") {
		t.Fatalf("unexpected ibc ack output: %s", output.String())
	}
	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "tx", "packet-timeout",
		"1", "transfer", "channel-0", "transfer", "channel-1", "payload", "10",
		"--fee", "1", "--gas", "1000", "--signer", "relayer", "--nonce", "3",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:10:fee=1:gas=1000:signer=relayer:nonce=3") {
		t.Fatalf("unexpected ibc timeout output: %s", output.String())
	}
	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{
		"ibc", "packet", "send",
		"--sequence", "1",
		"--source-port", "transfer",
		"--source-channel", "channel-0",
		"--destination-port", "transfer",
		"--destination-channel", "channel-1",
		"--data", "payload",
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"ibc_packet:", "sequence: 1", "source: transfer/channel-0", "destination: transfer/channel-1"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in output:\n%s", expected, output.String())
		}
	}
}

func TestRunRelayerBuildsFetchesAndSubmitsIBCTx(t *testing.T) {
	submitted := false
	proofPath := ""
	proofBody, err := json.Marshal(map[string]queryproof.Proof{
		"proof": {
			SchemaVersion: queryproof.SchemaVersionV1,
			ChainID:       "counterparty",
			Height:        9,
			Namespace:     "ibc",
			Key:           []byte("packets"),
			Exists:        true,
			StateRoot:     types.Hash{1},
			LeafHash:      types.Hash{2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/ibc/proof/packet/"):
			proofPath = request.URL.Path
			return jsonHTTPResponse(http.StatusOK, string(proofBody)), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			submitted = true
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, err
			}
			decoded, err := base64.StdEncoding.DecodeString(payload["tx"])
			if err != nil {
				return nil, err
			}
			if !strings.Contains(string(decoded), "ibc:packet-ack:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:YWNr") {
				t.Fatalf("unexpected submitted tx: %s", decoded)
			}
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	var output bytes.Buffer
	err = runRelayerPacketAck(&output, []string{
		"--rpc", "http://dest.example",
		"--proof-rpc", "http://source.example",
		"--sequence", "1",
		"--source-port", "transfer",
		"--source-channel", "channel-0",
		"--destination-port", "transfer",
		"--destination-channel", "channel-1",
		"--data", "payload",
		"--ack", "ack",
		"--fee", "1",
		"--gas", "1000",
		"--signer", "relayer",
		"--nonce", "2",
		"--submit",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !submitted || proofPath != "/v1/ibc/proof/packet/1/transfer/channel-0/transfer/channel-1" {
		t.Fatalf("expected proof fetch and tx submit, submitted=%t proofPath=%s", submitted, proofPath)
	}
	for _, expected := range []string{"proof_height: 9", "tx: ibc:packet-ack", "submitted: true"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in relayer output:\n%s", expected, output.String())
		}
	}
}

func TestRunRelayerClientUpdateAndPacketTimeoutDryRun(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatalf("dry-run relayer command should not call HTTP: %s", request.URL.String())
		return nil, nil
	})}
	var output bytes.Buffer
	err := runRelayerClientUpdate(&output, []string{
		"--client-id", "07-vexo-0",
		"--height", "9",
		"--validator-set-hash", strings.Repeat("01", 32),
		"--state-root", strings.Repeat("02", 32),
		"--fee", "1",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:client-update:07-vexo-0:9:") {
		t.Fatalf("unexpected client update output: %s", output.String())
	}
	output.Reset()
	err = runRelayerPacketTimeout(&output, []string{
		"--sequence", "1",
		"--source-port", "transfer",
		"--source-channel", "channel-0",
		"--destination-port", "transfer",
		"--destination-channel", "channel-1",
		"--data", "payload",
		"--timeout-height", "100",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:100") {
		t.Fatalf("unexpected timeout output: %s", output.String())
	}
}

func TestRunRelayerClientUpdateFetchesSourceStateAndSubmits(t *testing.T) {
	hash := strings.Repeat("01", 32)
	root := strings.Repeat("02", 32)
	submitted := false
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/state/latest":
			return jsonHTTPResponse(http.StatusOK, `{"height":12,"app_hash":"`+root+`","validator_set_hash":"`+hash+`"}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			submitted = true
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, err
			}
			decoded, err := base64.StdEncoding.DecodeString(payload["tx"])
			if err != nil {
				return nil, err
			}
			expected := "ibc:client-update:07-vexo-0:12:" + hash + ":" + root + ":fee=1"
			if string(decoded) != expected {
				t.Fatalf("unexpected client update tx: %s", decoded)
			}
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}
	var output bytes.Buffer
	err := runRelayerClientUpdate(&output, []string{
		"--source-rpc", "http://source.example",
		"--rpc", "http://dest.example",
		"--client-id", "07-vexo-0",
		"--fee", "1",
		"--submit",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !submitted {
		t.Fatal("expected client update submit")
	}
	for _, expected := range []string{"source_height: 12", "source_validator_set_hash: " + hash, "source_state_root: " + root, "submitted: true"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in output:\n%s", expected, output.String())
		}
	}
}

func TestRunRelayerDiscoverFindsIndexedPackets(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/events" {
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
		if request.URL.Query().Get("key") != "ibc_packet_event" || request.URL.Query().Get("value") != "send" {
			t.Fatalf("unexpected event query: %s", request.URL.RawQuery)
		}
		return jsonHTTPResponse(http.StatusOK, `{
			"key":"ibc_packet_event",
			"value":"send",
			"records":[{
				"height":7,
				"tx_index":0,
				"event":{
					"type":"ibc_packet-send",
					"attributes":[
						{"key":"ibc_packet_event","value":"send","index":true},
						{"key":"ibc_sequence","value":"2","index":true},
						{"key":"ibc_source_port","value":"transfer","index":true},
						{"key":"ibc_source_channel","value":"channel-0","index":true},
						{"key":"ibc_destination_port","value":"transfer","index":true},
						{"key":"ibc_destination_channel","value":"channel-1","index":true},
						{"key":"ibc_data","value":"cGF5bG9hZA"},
						{"key":"ibc_timeout_height","value":"10","index":true}
					]
				}
			}]
		}`), nil
	})}
	var output bytes.Buffer
	if err := runRelayerDiscover(&output, []string{"--rpc", "http://source.example"}, client); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"height: 7", "sequence: 2", "source: transfer/channel-0", "destination: transfer/channel-1", "data: payload", "timeout_height: 10"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in discover output:\n%s", expected, output.String())
		}
	}
	output.Reset()
	if err := runRelayerDiscover(&output, []string{"--rpc", "http://source.example", "--json"}, client); err != nil {
		t.Fatal(err)
	}
	var packets []relayerDiscoveredPacket
	if err := json.Unmarshal(output.Bytes(), &packets); err != nil {
		t.Fatal(err)
	}
	if len(packets) != 1 || packets[0].Sequence != 2 || packets[0].TimeoutHeight != 10 {
		t.Fatalf("unexpected discovered packets: %+v", packets)
	}
}

func TestRunRelayerLoopRetriesProofAndSubmits(t *testing.T) {
	proofBody, err := json.Marshal(map[string]queryproof.Proof{
		"proof": {
			SchemaVersion: queryproof.SchemaVersionV1,
			ChainID:       "counterparty",
			Height:        11,
			Namespace:     "ibc",
			Key:           []byte("packets"),
			Exists:        true,
			StateRoot:     types.Hash{1},
			LeafHash:      types.Hash{2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	proofCalls := 0
	submitCalls := 0
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/ibc/proof/packet/"):
			proofCalls++
			if proofCalls == 1 {
				return jsonHTTPResponse(http.StatusNotFound, `{"error":"not ready"}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, string(proofBody)), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			submitCalls++
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, err
			}
			decoded, err := base64.StdEncoding.DecodeString(payload["tx"])
			if err != nil {
				return nil, err
			}
			if !strings.Contains(string(decoded), "ibc:packet-timeout:1:transfer:channel-0:transfer:channel-1:cGF5bG9hZA:100") {
				t.Fatalf("unexpected loop tx: %s", decoded)
			}
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}
	var output bytes.Buffer
	err = runRelayerLoop(&output, []string{
		"--mode", "timeout",
		"--rpc", "http://dest.example",
		"--proof-rpc", "http://source.example",
		"--sequence", "1",
		"--source-port", "transfer",
		"--source-channel", "channel-0",
		"--destination-port", "transfer",
		"--destination-channel", "channel-1",
		"--data", "payload",
		"--timeout-height", "100",
		"--interval", "0s",
		"--max-iterations", "2",
		"--continue-on-error",
		"--submit",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if proofCalls != 2 || submitCalls != 1 {
		t.Fatalf("expected two proof polls and one submit, proofCalls=%d submitCalls=%d", proofCalls, submitCalls)
	}
	for _, expected := range []string{"proof_error:", "proof_height: 11", "submitted: true"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in relayer loop output:\n%s", expected, output.String())
		}
	}
}

func TestRunRelayerLoopCheckpointSkipsDuplicateSubmit(t *testing.T) {
	proofBody, err := json.Marshal(map[string]queryproof.Proof{
		"proof": {
			SchemaVersion: queryproof.SchemaVersionV1,
			ChainID:       "counterparty",
			Height:        11,
			Namespace:     "ibc",
			Key:           []byte("packets"),
			Exists:        true,
			StateRoot:     types.Hash{1},
			LeafHash:      types.Hash{2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	proofCalls := 0
	submitCalls := 0
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/ibc/proof/packet/"):
			proofCalls++
			return jsonHTTPResponse(http.StatusOK, string(proofBody)), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			submitCalls++
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}
	statePath := filepath.Join(t.TempDir(), "relayer_state.json")
	args := []string{
		"--mode", "timeout",
		"--rpc", "http://dest.example",
		"--proof-rpc", "http://source.example",
		"--sequence", "1",
		"--source-port", "transfer",
		"--source-channel", "channel-0",
		"--destination-port", "transfer",
		"--destination-channel", "channel-1",
		"--data", "payload",
		"--timeout-height", "100",
		"--interval", "0s",
		"--max-iterations", "1",
		"--submit",
		"--state", statePath,
	}
	var output bytes.Buffer
	if err := runRelayerLoop(&output, args, client); err != nil {
		t.Fatal(err)
	}
	if proofCalls != 1 || submitCalls != 1 {
		t.Fatalf("expected first run to fetch proof and submit once, proofCalls=%d submitCalls=%d", proofCalls, submitCalls)
	}
	for _, expected := range []string{"submitted: true", "checkpoint_saved: true"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in first output:\n%s", expected, output.String())
		}
	}
	output.Reset()
	if err := runRelayerLoop(&output, args, client); err != nil {
		t.Fatal(err)
	}
	if proofCalls != 1 || submitCalls != 1 {
		t.Fatalf("expected second run to skip without network calls, proofCalls=%d submitCalls=%d", proofCalls, submitCalls)
	}
	if !strings.Contains(output.String(), "checkpoint_skipped: true") {
		t.Fatalf("expected checkpoint skip output:\n%s", output.String())
	}
}

func TestRunRelayerConfigRunsMultipleJobs(t *testing.T) {
	proofBody, err := json.Marshal(map[string]queryproof.Proof{
		"proof": {
			SchemaVersion: queryproof.SchemaVersionV1,
			ChainID:       "counterparty",
			Height:        12,
			Namespace:     "ibc",
			Key:           []byte("packets"),
			Exists:        true,
			StateRoot:     types.Hash{1},
			LeafHash:      types.Hash{2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	submitted := map[string]int{}
	var submittedMu sync.Mutex
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/ibc/proof/packet/"):
			return jsonHTTPResponse(http.StatusOK, string(proofBody)), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, err
			}
			decoded, err := base64.StdEncoding.DecodeString(payload["tx"])
			if err != nil {
				return nil, err
			}
			submittedMu.Lock()
			submitted[string(decoded)]++
			submittedMu.Unlock()
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}
	document := relayerConfigDocument{
		SchemaVersion: relayerConfigSchemaVersion,
		Jobs: []relayerJobConfig{
			{
				Name:            "ack-transfer",
				Mode:            "ack",
				RPC:             "dest.example",
				ProofRPC:        "source.example",
				Ack:             "ack",
				Submit:          true,
				StatePath:       filepath.Join(t.TempDir(), "ack_state.json"),
				Interval:        "0s",
				MaxIterations:   1,
				ContinueOnError: true,
				Fee:             "1",
				Packet: relayerPacketConfig{
					Sequence:           1,
					SourcePort:         "transfer",
					SourceChannel:      "channel-0",
					DestinationPort:    "transfer",
					DestinationChannel: "channel-1",
					Data:               "payload",
				},
			},
			{
				Name:            "timeout-transfer",
				Mode:            "timeout",
				RPC:             "dest.example",
				ProofRPC:        "source.example",
				Submit:          true,
				StatePath:       filepath.Join(t.TempDir(), "timeout_state.json"),
				Interval:        "0s",
				MaxIterations:   1,
				ContinueOnError: true,
				Packet: relayerPacketConfig{
					Sequence:           2,
					SourcePort:         "transfer",
					SourceChannel:      "channel-0",
					DestinationPort:    "transfer",
					DestinationChannel: "channel-1",
					Data:               "payload",
					TimeoutHeight:      100,
				},
			},
		},
	}
	var output bytes.Buffer
	if err := runRelayerConfig(context.Background(), &output, client, document); err != nil {
		t.Fatal(err)
	}
	submittedMu.Lock()
	defer submittedMu.Unlock()
	if len(submitted) != 2 {
		t.Fatalf("expected two submitted txs, got %+v", submitted)
	}
	for _, expected := range []string{"job=ack-transfer submitted: true", "job=timeout-transfer submitted: true"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected %q in output:\n%s", expected, output.String())
		}
	}
}

func TestReadRelayerConfigDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "relayer_config.json")
	writeTestJSON(t, path, relayerConfigDocument{
		SchemaVersion: relayerConfigSchemaVersion,
		Jobs: []relayerJobConfig{{
			Name:          "timeout-transfer",
			Mode:          "timeout",
			RPC:           "dest.example",
			ProofRPC:      "source.example",
			Submit:        true,
			StatePath:     "relayer_state.json",
			Interval:      "0s",
			MaxIterations: 1,
			Packet: relayerPacketConfig{
				Sequence:           2,
				SourcePort:         "transfer",
				SourceChannel:      "channel-0",
				DestinationPort:    "transfer",
				DestinationChannel: "channel-1",
				Data:               "payload",
				TimeoutHeight:      100,
			},
		}},
	})
	document, err := readRelayerConfigDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Jobs) != 1 || document.Jobs[0].Name != "timeout-transfer" {
		t.Fatalf("unexpected document: %+v", document)
	}
	if document.Jobs[0].StatePath != filepath.Join(dir, "relayer_state.json") {
		t.Fatalf("expected state path relative to config dir, got %q", document.Jobs[0].StatePath)
	}
}

func TestRunNetworkInitAndStartDryRun(t *testing.T) {
	home := t.TempDir()
	var initOutput bytes.Buffer
	if err := runCommand(&initOutput, &bytes.Buffer{}, []string{"network", "init", "--home", home, "--chain-id", "vexo-test", "--validators", "3"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(initOutput.String(), "initialized vexo network") || !strings.Contains(initOutput.String(), "validators: 3") {
		t.Fatalf("unexpected network init output:\n%s", initOutput.String())
	}

	var startOutput bytes.Buffer
	if err := runCommand(&startOutput, &bytes.Buffer{}, []string{"network", "start", "--home", home, "--validators", "3", "--binary", "/bin/vexod", "--p2p-base-port", "27656", "--rpc-base-port", "27657", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	output := startOutput.String()
	for _, expected := range []string{"network start plan", "validator-1", "validator-2", "validator-3", "start --home"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected dry-run output to contain %q, got:\n%s", expected, output)
		}
	}
	for _, forbidden := range []string{"--rpc-address", "--p2p-listen"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("expected dry-run output to avoid %q, got:\n%s", forbidden, output)
		}
	}
}

func TestRunNetworkUpDryRun(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	err := runCommand(&output, &bytes.Buffer{}, []string{
		"network", "up",
		"--home", home,
		"--chain-id", "vexo-test",
		"--validators", "2",
		"--binary", "/bin/vexod",
		"--p2p-base-port", "28656",
		"--rpc-base-port", "28657",
		"--timeout", "3s",
		"--tx", "bank:dry-run",
		"--overwrite",
		"--dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"network up plan",
		"chain-id: vexo-test",
		"validators: 2",
		"p2p-base-port: 28656",
		"rpc-base-port: 28657",
		"network init",
		"--overwrite",
		"network start",
		"network smoke",
		"network stop",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected network up dry-run output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunNetworkLoadAndChaosPlans(t *testing.T) {
	home := t.TempDir()
	var loadOutput bytes.Buffer
	if err := runCommand(&loadOutput, &bytes.Buffer{}, []string{"network", "load", "--home", home, "--validators", "3", "--duration", "10s", "--rate", "7", "--timeout", "1s", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"network load plan", "validators: 3", "rate: 7 tx/s", "estimated_transactions: 70"} {
		if !strings.Contains(loadOutput.String(), expected) {
			t.Fatalf("expected network load output to contain %q, got:\n%s", expected, loadOutput.String())
		}
	}

	var chaosOutput bytes.Buffer
	if err := runCommand(&chaosOutput, &bytes.Buffer{}, []string{"network", "chaos-plan", "--home", home, "--validators", "4", "--duration", "24h", "--regions", "2"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"network chaos plan", "regions: 2", "validator-1: region=1", "no conflicting finality"} {
		if !strings.Contains(chaosOutput.String(), expected) {
			t.Fatalf("expected network chaos output to contain %q, got:\n%s", expected, chaosOutput.String())
		}
	}

	var chaosRunOutput bytes.Buffer
	if err := runCommand(&chaosRunOutput, &bytes.Buffer{}, []string{"network", "chaos", "--home", home, "--validators", "4", "--timeout", "10s", "--stop-index", "2", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"network chaos run plan", "target: validator-3", "keep quorum online", "require height increase", "require catch-up"} {
		if !strings.Contains(chaosRunOutput.String(), expected) {
			t.Fatalf("expected network chaos run output to contain %q, got:\n%s", expected, chaosRunOutput.String())
		}
	}
}

func TestRunNetworkScalePlan(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"network", "scale-plan", "--home", home, "--validators", "64", "--regions", "4", "--hosts", "8", "--duration", "1m", "--rate", "60", "--inbound-peers", "128", "--outbound-peers", "64"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"network scale plan",
		"validators: 64",
		"regions: 4",
		"hosts: 8",
		"estimated_transactions: 3600",
		"fault_tolerance: 21",
		"quorum_power: 43",
		"full_mesh_connections: 2016",
		"peer_budget: inbound=128 outbound=64",
		"validator-64:",
		"rotate validators during load",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected network scale output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunNetworkScalePlanJSON(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"network", "scale-plan", "--validators", "7", "--regions", "2", "--hosts", "3", "--duration", "10s", "--rate", "5", "--inbound-peers", "2", "--outbound-peers", "1", "--json"}); err != nil {
		t.Fatal(err)
	}
	var plan networkScalePlan
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != "v1" || plan.Validators != 7 || plan.FaultTolerance != 2 || plan.QuorumPower != 5 || plan.FullMeshConnections != 21 {
		t.Fatalf("unexpected scale plan summary: %+v", plan)
	}
	if plan.EstimatedTransactions != 50 || len(plan.Nodes) != 7 {
		t.Fatalf("unexpected scale plan load/nodes: %+v", plan)
	}
	if !plan.Nodes[0].Seed || !plan.Nodes[1].Seed || !plan.Nodes[2].Seed || plan.Nodes[3].Seed {
		t.Fatalf("unexpected seed assignment: %+v", plan.Nodes[:4])
	}
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected peer budget warning")
	}
}

func TestRunNetworkScalePlanRejectsInvalidInputs(t *testing.T) {
	for _, args := range [][]string{
		{"network", "scale-plan", "--validators", "0"},
		{"network", "scale-plan", "--regions", "0"},
		{"network", "scale-plan", "--hosts", "0"},
		{"network", "scale-plan", "--rate", "0"},
		{"network", "scale-plan", "--inbound-peers", "-1"},
	} {
		var output bytes.Buffer
		if err := runCommand(&output, &bytes.Buffer{}, args); err == nil {
			t.Fatalf("expected error for args %v", args)
		}
	}
}

func TestRunNetworkLongRunPlan(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"network", "longrun-plan", "--validators", "4", "--duration", "168h", "--regions", "3", "--hosts", "4"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"network longrun plan", "duration: 168h0m0s", "hosts: 4", "host=node-1 region=1", "state sync recovery"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected network longrun output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunNetworkLongRunDryRun(t *testing.T) {
	var output bytes.Buffer
	if err := runCommand(&output, &bytes.Buffer{}, []string{"network", "longrun", "--validators", "4", "--duration", "10m", "--rate", "25", "--output", "evidence.json", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"network longrun harness plan", "duration: 10m0s", "rate: 25 tx/s", "evidence_output: evidence.json", "emit machine-readable long-run evidence JSON"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected network longrun dry-run output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestRunNetworkLongRunEvidenceEvaluatesMetrics(t *testing.T) {
	plan, err := buildNetworkRuntimePlan(t.TempDir(), 2, "/bin/vexod")
	if err != nil {
		t.Fatal(err)
	}
	metricsCalls := make(map[string]int)
	txSubmitted := uint64(0)
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		address := request.URL.Host
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/metrics":
			metricsCalls[address]++
			height := uint64(10)
			if metricsCalls[address] > 1 {
				height = 12
			}
			return jsonHTTPResponse(http.StatusOK, `{"latest_height":`+strconv.FormatUint(height, 10)+`,"snapshot_healthy":true,"replay_healthy":true}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			txSubmitted++
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	evidence := runNetworkLongRunEvidence(context.Background(), client, plan, 100*time.Millisecond, 20, "bank:send:alice:bob:1:fee=1:gas=1000:signer=alice:nonce")
	if !evidence.OK || evidence.SchemaVersion != "v1" || evidence.Load.Submitted == 0 || txSubmitted == 0 || len(evidence.Nodes) != 2 {
		t.Fatalf("unexpected longrun evidence: %+v", evidence)
	}
	for _, node := range evidence.Nodes {
		if node.Sample.HeightRatePerMinute <= 0 || !node.Report.OK {
			t.Fatalf("expected healthy node evidence, got %+v", node)
		}
	}
}

func TestParseNetworkDurationUsesHumanUnits(t *testing.T) {
	for _, testCase := range []struct {
		value    string
		expected time.Duration
	}{
		{value: "250ms", expected: 250 * time.Duration(1_000_000)},
		{value: "3s", expected: 3 * time.Duration(1_000_000_000)},
		{value: "2m", expected: 2 * time.Duration(60_000_000_000)},
	} {
		actual, err := parseNetworkDuration(testCase.value)
		if err != nil {
			t.Fatalf("expected %s to parse: %v", testCase.value, err)
		}
		if actual != testCase.expected {
			t.Fatalf("expected %s to parse as %s, got %s", testCase.value, testCase.expected, actual)
		}
	}
	if _, err := parseNetworkDuration("0s"); err == nil {
		t.Fatal("expected zero duration to fail")
	}
}

func TestEstimatedNetworkTransactionsUsesWallSeconds(t *testing.T) {
	duration, err := parseNetworkDuration("1h")
	if err != nil {
		t.Fatal(err)
	}
	if actual := estimatedNetworkTransactions(duration, 50); actual != 180_000 {
		t.Fatalf("expected 180000 transactions for 1h at 50 tx/s, got %d", actual)
	}
}

func TestNetworkLoadPayloadUsesRealisticNonce(t *testing.T) {
	payload := networkLoadPayload("bank:send:alice:bob:1:fee=1:gas=1000:signer=alice:nonce", 7)
	if string(payload) != "bank:send:alice:bob:1:fee=1:gas=1000:signer=alice:nonce=7" {
		t.Fatalf("unexpected load payload: %s", payload)
	}
}

func TestRunInitWritesNetworkFilesWithCustomPorts(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	if err := runInit(&output, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2", "--p2p-base-port", "27656", "--rpc-base-port", "27657"}); err != nil {
		t.Fatal(err)
	}
	genesis, err := loadGenesis(filepath.Join(home, "validator-2", genesisFileName))
	if err != nil {
		t.Fatal(err)
	}
	if genesis.Validators[0].Metadata["p2p_address"] != "127.0.0.1:27656" || genesis.Validators[1].Metadata["rpc_address"] != "127.0.0.1:27667" {
		t.Fatalf("unexpected custom port metadata: %+v", genesis.Validators)
	}
	if !strings.Contains(output.String(), "p2p=127.0.0.1:27666") || !strings.Contains(output.String(), "rpc=127.0.0.1:27667") {
		t.Fatalf("unexpected custom port output:\n%s", output.String())
	}
}

func TestRunInitWritesNetworkConfigPeers(t *testing.T) {
	home := t.TempDir()
	topologyPath := filepath.Join(t.TempDir(), "topology.json")
	if err := os.WriteFile(topologyPath, []byte(`{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "public-validator-%d.example.com",
  "rpc_advertise_host_template": "public-rpc-%d.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runInit(&output, []string{
		"--home", home,
		"--chain-id", "vexo-test",
		"--validators", "2",
		"--network-config", topologyPath,
	}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, "validator-1", networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if networkDocument.P2P.ListenAddress != "0.0.0.0:26656" || networkDocument.RPC.Address != "0.0.0.0:26657" {
		t.Fatalf("unexpected listen config: %+v", networkDocument)
	}
	if networkDocument.P2P.Peers["validator-2"] != "validator-2:26656" {
		t.Fatalf("expected config peer address, got %+v", networkDocument.P2P.Peers)
	}
	genesis, err := loadGenesis(filepath.Join(home, "validator-2", genesisFileName))
	if err != nil {
		t.Fatal(err)
	}
	if genesis.Validators[0].Metadata["p2p_address"] != "public-validator-1.example.com:26656" || genesis.Validators[1].Metadata["rpc_address"] != "public-rpc-2.example.com:26657" {
		t.Fatalf("unexpected advertised metadata: %+v", genesis.Validators)
	}
}

func TestRunInitWritesDefaultConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Chain.Committee.CommitteeSize != 128 || cfg.Chain.Governance.Timelock != 10 {
		t.Fatalf("expected default config, got %+v", cfg.Chain)
	}
}

func TestRunInitValidatorAndArchiveRoles(t *testing.T) {
	validatorHome := t.TempDir()
	var validatorOutput bytes.Buffer
	if err := runInit(&validatorOutput, []string{"validator", "--home", validatorHome, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	validatorDocument, err := readConfigDocument(filepath.Join(validatorHome, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	validatorConsensus, err := readConsensusConfigDocument(filepath.Join(validatorHome, consensusConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if validatorDocument.ValidatorID != "alice" || !validatorConsensus.Consensus.LoopEnabled {
		t.Fatalf("unexpected validator config: %+v consensus=%+v", validatorDocument, validatorConsensus)
	}
	if _, err := os.Stat(filepath.Join(validatorHome, keyFileName)); err != nil {
		t.Fatalf("expected validator key: %v", err)
	}

	archiveHome := t.TempDir()
	var archiveOutput bytes.Buffer
	if err := runInit(&archiveOutput, []string{"archive", "--home", archiveHome, "--chain-id", "vexo-test", "--bootstrap-peer", "validator-1=seed.example.com:26656"}); err != nil {
		t.Fatal(err)
	}
	archiveDocument, err := readConfigDocument(filepath.Join(archiveHome, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	archiveConsensus, err := readConsensusConfigDocument(filepath.Join(archiveHome, consensusConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if archiveDocument.ValidatorID != "" || archiveConsensus.Consensus.LoopEnabled {
		t.Fatalf("unexpected archive config: %+v consensus=%+v", archiveDocument, archiveConsensus)
	}
	archiveNetwork, err := readNetworkConfigDocument(filepath.Join(archiveHome, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if archiveNetwork.P2P.Peers["validator-1"] != "seed.example.com:26656" {
		t.Fatalf("expected bootstrap peer in config, got %+v", archiveNetwork.P2P.Peers)
	}
	if _, err := os.Stat(filepath.Join(archiveHome, keyFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive init must not create validator key, got %v", err)
	}
	inputs, err := loadStartInputs(archiveHome, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if inputs.Signer != nil || inputs.Config.ValidatorID != "" {
		t.Fatalf("expected archive start inputs without signer: %+v", inputs)
	}
}

func TestRunInitValidatorBLSWritesProofOfPossession(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"validator", "--home", home, "--chain-id", "vexo-test", "--validator", "alice", "--key-type", "bls"}); err != nil {
		t.Fatal(err)
	}
	genesis, err := loadGenesis(filepath.Join(home, genesisFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(genesis.Validators) != 1 || genesis.Validators[0].Metadata[vexocrypto.BLSProofOfPossessionMetadataKey] == "" {
		t.Fatalf("expected BLS proof in genesis: %+v", genesis.Validators)
	}
	keyDocument, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if keyDocument.Type != vexocrypto.KeyTypeBLS {
		t.Fatalf("expected BLS key document: %+v", keyDocument)
	}
}

func TestWriteOperationalLogJSON(t *testing.T) {
	var output bytes.Buffer
	writeOperationalLog(&output, "json", "info", "node_running", map[string]any{"chain_id": "vexo-test"})
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["event"] != "node_running" || record["level"] != "info" || record["chain_id"] != "vexo-test" || record["version"] == "" || record["ts"] == "" {
		t.Fatalf("unexpected structured log: %+v", record)
	}
}

func TestRunSnapshotExportAndRestore(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(context.Background(), store.StateRecord{
		Height:           3,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := storage.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveStateRoot(context.Background(), store.StateRootRecord{Height: 3, Namespace: "bank", Root: sourceRoot}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(t.TempDir(), "snapshot.json")
	var exportOutput bytes.Buffer
	if err := runSnapshot(&exportOutput, []string{"export", "--home", home, "--output", snapshotPath}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exportOutput.String(), "snapshot exported") {
		t.Fatalf("unexpected export output:\n%s", exportOutput.String())
	}
	var verifyOutput bytes.Buffer
	if err := runSnapshot(&verifyOutput, []string{"verify", "--home", home, "--input", snapshotPath}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(verifyOutput.String(), "snapshot verified") || !strings.Contains(verifyOutput.String(), "kv_pairs: 1") {
		t.Fatalf("unexpected verify output:\n%s", verifyOutput.String())
	}
	var drillOutput bytes.Buffer
	if err := runSnapshot(&drillOutput, []string{"drill-plan", "--input", snapshotPath, "--chain-id", "vexo-test", "--min-height", "3"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"snapshot drill plan", "height: 3", "kv_pairs: 1", "checksum ok=true", "height catch-up"} {
		if !strings.Contains(drillOutput.String(), expected) {
			t.Fatalf("expected snapshot drill output to contain %q, got:\n%s", expected, drillOutput.String())
		}
	}

	restoreHome := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", restoreHome, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	var restoreOutput bytes.Buffer
	if err := runSnapshot(&restoreOutput, []string{"restore", "--home", restoreHome, "--input", snapshotPath}); err != nil {
		t.Fatal(err)
	}
	restoredConfig, err := loadNodeConfig(filepath.Join(restoreHome, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	restoredStore, err := store.OpenLevelDB(restoredConfig.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restoredStore.Close()
	state, err := restoredStore.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 3 || state.AppHash != (types.Hash{1}) {
		t.Fatalf("unexpected restored state: %+v", state)
	}
	root, err := restoredStore.StateRoot(context.Background(), 3, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if root.Root != sourceRoot {
		t.Fatalf("unexpected restored root: %+v", root)
	}
	value, err := restoredStore.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "100" {
		t.Fatalf("unexpected restored bank value %q", value)
	}
}

func TestRunSnapshotChunkExportRestore(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	for account, balance := range map[string]string{"alice": "100", "bob": "70", "carol": "30"} {
		if err := storage.Set(context.Background(), "bank", []byte(account), []byte(balance)); err != nil {
			t.Fatal(err)
		}
	}
	root, err := storage.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(context.Background(), store.StateRecord{
		Height:           11,
		AppHash:          types.Hash{11},
		LastBlockHash:    types.Hash{12},
		ValidatorSetHash: types.Hash{13},
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveStateRoot(context.Background(), store.StateRootRecord{Height: 11, Namespace: "bank", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	chunkDir := filepath.Join(t.TempDir(), "chunks")
	var exportOutput bytes.Buffer
	if err := runSnapshot(&exportOutput, []string{"chunk-export", "--home", home, "--output-dir", chunkDir, "--chunk-size", "2"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exportOutput.String(), "chunks: 2") {
		t.Fatalf("unexpected chunk export output:\n%s", exportOutput.String())
	}

	restoreHome := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", restoreHome, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	var restoreOutput bytes.Buffer
	if err := runSnapshot(&restoreOutput, []string{"chunk-restore", "--home", restoreHome, "--input-dir", chunkDir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restoreOutput.String(), "snapshot chunks restored") || !strings.Contains(restoreOutput.String(), "chunks: 2") {
		t.Fatalf("unexpected chunk restore output:\n%s", restoreOutput.String())
	}
	restoredConfig, err := loadNodeConfig(filepath.Join(restoreHome, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	restoredStore, err := store.OpenLevelDB(restoredConfig.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	defer restoredStore.Close()
	state, err := restoredStore.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	restoredRoot, err := restoredStore.StateRoot(context.Background(), 11, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 11 || restoredRoot.Root != root {
		t.Fatalf("unexpected restored chunk state=%+v root=%+v", state, restoredRoot)
	}
	value, err := restoredStore.Get(context.Background(), "bank", []byte("bob"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "70" {
		t.Fatalf("unexpected restored bank value %q", value)
	}
}

func TestSnapshotChunksRejectTamperingAndMissingChunks(t *testing.T) {
	sourceStore, err := store.OpenLevelDB(filepath.Join(t.TempDir(), "source-store"))
	if err != nil {
		t.Fatal(err)
	}
	for account, balance := range map[string]string{"alice": "100", "bob": "70", "carol": "30"} {
		if err := sourceStore.Set(context.Background(), "bank", []byte(account), []byte(balance)); err != nil {
			t.Fatal(err)
		}
	}
	root, err := sourceStore.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceStore.Close(); err != nil {
		t.Fatal(err)
	}
	document := snapshotDocumentFromState("vexo-test", []string{"bank"}, store.StateRecord{
		Height:           7,
		AppHash:          types.Hash{7},
		LastBlockHash:    types.Hash{8},
		ValidatorSetHash: types.Hash{9},
	}, []store.StateRootRecord{{Height: 7, Namespace: "bank", Root: root}}, []store.KVPair{
		{Namespace: "bank", Key: []byte("alice"), Value: []byte("100")},
		{Namespace: "bank", Key: []byte("bob"), Value: []byte("70")},
		{Namespace: "bank", Key: []byte("carol"), Value: []byte("30")},
	})
	chunks, err := snapshotChunks(document, 2)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := snapshotDocumentFromChunks(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Checksum != document.Checksum || len(rebuilt.KV) != 3 {
		t.Fatalf("unexpected rebuilt snapshot checksum=%s kv=%d", rebuilt.Checksum, len(rebuilt.KV))
	}
	if _, err := snapshotDocumentFromChunks(chunks[:1]); err == nil {
		t.Fatal("expected missing chunk rejection")
	}
	tampered := append([]snapshotChunkDocument(nil), chunks...)
	tampered[0].KV[0].Value = []byte("999")
	if _, err := snapshotDocumentFromChunks(tampered); err == nil {
		t.Fatal("expected tampered chunk rejection")
	}
}

func TestRunSnapshotVerifyRejectsChecksumMismatch(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	document := snapshotDocumentFromState("vexo-test", []string{"bank"}, store.StateRecord{
		Height:           3,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
	}, []store.StateRootRecord{{Height: 3, Namespace: "bank", Root: types.Hash{4}}}, []store.KVPair{{Namespace: "bank", Key: []byte("alice"), Value: []byte("100")}})
	document.KV[0].Value = []byte("999")
	path := filepath.Join(t.TempDir(), "corrupt-snapshot.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSnapshotDocument(file, document); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runSnapshot(&bytes.Buffer{}, []string{"verify", "--home", home, "--input", path}); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestValidateSnapshotRejectsMissingRootAndUnknownKVNamespace(t *testing.T) {
	document := snapshotDocument{
		SchemaVersion: "v1",
		ChainID:       "vexo-test",
		Modules:       []string{"bank"},
		State: store.StateRecord{
			Height:           3,
			AppHash:          types.Hash{1},
			LastBlockHash:    types.Hash{2},
			ValidatorSetHash: types.Hash{3},
		},
	}
	document.Checksum = snapshotChecksum(document)
	if err := validateSnapshotDocument(document, "vexo-test"); err == nil {
		t.Fatal("expected missing root rejection")
	}

	document = snapshotDocumentFromState("vexo-test", []string{"bank"}, store.StateRecord{
		Height:           3,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
	}, []store.StateRootRecord{{Height: 3, Namespace: "bank", Root: types.Hash{4}}}, []store.KVPair{{Namespace: "evil", Key: []byte("alice"), Value: []byte("100")}})
	if err := validateSnapshotDocument(document, "vexo-test"); err == nil {
		t.Fatal("expected unknown KV namespace rejection")
	}
}

func TestRunSnapshotFetchAndSyncFromRPCExport(t *testing.T) {
	sourceStore, err := store.OpenLevelDB(filepath.Join(t.TempDir(), "source-store"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceStore.Set(context.Background(), "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := sourceStore.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceStore.Close(); err != nil {
		t.Fatal(err)
	}
	document := snapshotDocumentFromState("vexo-test", []string{"bank"}, store.StateRecord{
		Height:           7,
		AppHash:          types.Hash{7},
		LastBlockHash:    types.Hash{8},
		ValidatorSetHash: types.Hash{9},
	}, []store.StateRootRecord{{Height: 7, Namespace: "bank", Root: sourceRoot}}, []store.KVPair{{Namespace: "bank", Key: []byte("alice"), Value: []byte("100")}})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/snapshot/export" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := writeSnapshotDocument(writer, document); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	snapshotPath := filepath.Join(t.TempDir(), "remote-snapshot.json")
	var fetchOutput bytes.Buffer
	if err := runSnapshot(&fetchOutput, []string{"fetch", "--url", server.URL + "/snapshot/export", "--output", snapshotPath}); err != nil {
		t.Fatal(err)
	}
	fetched, err := readSnapshotDocument(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.State.Height != 7 || len(fetched.StateRoots) != 1 {
		t.Fatalf("unexpected fetched snapshot: %+v", fetched)
	}

	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	var syncOutput bytes.Buffer
	if err := runSnapshot(&syncOutput, []string{"sync", "--home", home, "--url", server.URL + "/snapshot/export"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	state, err := storage.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	root, err := storage.StateRoot(context.Background(), 7, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 7 || root.Root != sourceRoot || !strings.Contains(syncOutput.String(), "snapshot synced") {
		t.Fatalf("unexpected synced snapshot state=%+v root=%+v output=%s", state, root, syncOutput.String())
	}
	value, err := storage.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "100" {
		t.Fatalf("unexpected synced bank value %q", value)
	}
}

func TestRunDoctorReportsOperationalReadinessAndRepairsIndexes(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveBlock(context.Background(), store.BlockRecord{
		Block:   types.Block{Header: types.Header{Height: 3, ChainID: "vexo-test"}},
		Hash:    types.Hash{3},
		AppHash: types.Hash{4},
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(context.Background(), store.StateRecord{
		Height:           3,
		AppHash:          types.Hash{4},
		LastBlockHash:    types.Hash{3},
		ValidatorSetHash: types.Hash{5},
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveStateRoot(context.Background(), store.StateRootRecord{Height: 3, Namespace: "bank", Root: types.Hash{6}}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runDoctor(&output, []string{"--home", home, "--repair-indexes", "--json"}); err != nil {
		t.Fatal(err)
	}
	var document doctorDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if !document.OK || document.RecoverResult == nil || document.RecoverResult.BlockIndexKeys != 1 || document.RecoverResult.RecoveredIndexes == 0 {
		t.Fatalf("unexpected doctor document: %+v", document)
	}

	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{"doctor", "--home", home}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "doctor ok") || !strings.Contains(output.String(), "check snapshot: ok") {
		t.Fatalf("unexpected doctor output:\n%s", output.String())
	}
}

func TestRunNetworkUpDryRunCanKeepRunning(t *testing.T) {
	var output bytes.Buffer
	err := runCommand(&output, &bytes.Buffer{}, []string{
		"network", "up",
		"--home", t.TempDir(),
		"--validators", "1",
		"--binary", "/bin/vexod",
		"--keep-running",
		"--dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "keep nodes running") || strings.Contains(output.String(), "network stop") {
		t.Fatalf("expected keep-running dry-run to skip stop, got:\n%s", output.String())
	}
}

func TestNetworkRuntimePlanAndPIDHelpers(t *testing.T) {
	home := t.TempDir()
	plan, err := buildNetworkRuntimePlan(home, 2, "/bin/vexod")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Binary != "/bin/vexod" || len(plan.Nodes) != 2 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Nodes[0].ValidatorID != "validator-1" || plan.Nodes[0].RPCAddress != networkRPCAddress(1) || plan.Nodes[1].P2PAddress != networkP2PAddress(2) {
		t.Fatalf("unexpected nodes: %+v", plan.Nodes)
	}
	if plan.Nodes[0].LogPath != filepath.Join(home, "validator-1", "vexod.log") {
		t.Fatalf("unexpected log path: %s", plan.Nodes[0].LogPath)
	}
	if _, err := buildNetworkRuntimePlan(home, 0, "/bin/vexod"); err == nil {
		t.Fatal("expected invalid validator count")
	}
	pidPath := filepath.Join(home, networkPIDFileName)
	if err := os.WriteFile(pidPath, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, err := readNetworkPID(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 12345 {
		t.Fatalf("expected pid 12345, got %d", pid)
	}
}

func TestNetworkRuntimePlanWithCustomPorts(t *testing.T) {
	home := t.TempDir()
	plan, err := buildNetworkRuntimePlanWithPorts(home, 2, "/bin/vexod", 27656, 27657)
	if err != nil {
		t.Fatal(err)
	}
	if plan.P2PBasePort != 27656 || plan.RPCBasePort != 27657 {
		t.Fatalf("unexpected base ports: %+v", plan)
	}
	if plan.Nodes[0].P2PAddress != "127.0.0.1:27656" || plan.Nodes[1].RPCAddress != "127.0.0.1:27667" {
		t.Fatalf("unexpected custom addresses: %+v", plan.Nodes)
	}
	if _, err := buildNetworkRuntimePlanWithPorts(home, 2, "/bin/vexod", 0, 27657); err == nil {
		t.Fatal("expected invalid custom port")
	}
}

func TestStartNetworkNodeRefusesExistingPIDFile(t *testing.T) {
	plan, err := buildNetworkRuntimePlan(t.TempDir(), 1, "/bin/vexod")
	if err != nil {
		t.Fatal(err)
	}
	localNode := plan.Nodes[0]
	if err := os.MkdirAll(localNode.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localNode.PIDPath, []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = startNetworkNode("/bin/vexod", localNode)
	if err == nil || !strings.Contains(err.Error(), "already has pid file") {
		t.Fatalf("expected existing pid file error, got %v", err)
	}
}

func TestNetworkHealthOK(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/healthz" {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}
	if !networkHealthOK(context.Background(), client, "127.0.0.1:26657") {
		t.Fatal("expected network health ok")
	}
	failingClient := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	if networkHealthOK(context.Background(), failingClient, "127.0.0.1:26657") {
		t.Fatal("expected unreachable network health to fail")
	}
}

func TestRunNetworkSmokePlanSubmitsTxAndWaitsForHeight(t *testing.T) {
	plan, err := buildNetworkRuntimePlan(t.TempDir(), 2, "/bin/vexod")
	if err != nil {
		t.Fatal(err)
	}
	heights := map[string]uint64{
		plan.Nodes[0].RPCAddress: 7,
		plan.Nodes[1].RPCAddress: 7,
	}
	txSubmitted := false
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		address := request.URL.Host
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/healthz":
			return jsonHTTPResponse(http.StatusOK, `{"ok":true}`), nil
		case request.Method == http.MethodGet && request.URL.Path == "/v1/status":
			return jsonHTTPResponse(http.StatusOK, `{"chain_id":"vexo-test","running":true,"latest_height":`+strconv.FormatUint(heights[address], 10)+`}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/v1/tx":
			txSubmitted = true
			heights[plan.Nodes[0].RPCAddress] = 8
			heights[plan.Nodes[1].RPCAddress] = 8
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}
	results, err := runNetworkSmokePlan(context.Background(), client, plan, []byte("bank:smoke"))
	if err != nil {
		t.Fatal(err)
	}
	if !txSubmitted || len(results) != 2 {
		t.Fatalf("expected tx submitted and two results: submitted=%t results=%+v", txSubmitted, results)
	}
	for _, result := range results {
		if !result.Healthy || result.Height != 8 {
			t.Fatalf("unexpected smoke result: %+v", result)
		}
	}
}

func jsonHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestRunConfigPathsAndShow(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}

	var pathsOutput bytes.Buffer
	if err := runConfig(&pathsOutput, []string{"paths", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var paths pathDocument
	if err := json.Unmarshal(pathsOutput.Bytes(), &paths); err != nil {
		t.Fatal(err)
	}
	if paths.Config == "" || paths.Genesis == "" || paths.Key == "" || paths.DataDir == "" {
		t.Fatalf("unexpected paths document: %+v", paths)
	}

	var configOutput bytes.Buffer
	if err := runConfig(&configOutput, []string{"show", "--home", home}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(configOutput.String(), `"ChainID": "vexo-test"`) {
		t.Fatalf("unexpected config output:\n%s", configOutput.String())
	}
}

func TestRunConfigRejectsInvalidSubcommand(t *testing.T) {
	if err := runConfig(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("expected missing config subcommand error")
	}
	if err := runConfig(&bytes.Buffer{}, []string{"unknown"}); err == nil {
		t.Fatal("expected unknown config subcommand error")
	}
}

func TestRunConfigAuditReportsProductionWarnings(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runConfig(&output, []string{"audit", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var document auditDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if !document.OK || document.Strict || !auditContains(document, "key_encrypted_or_remote", false) || !auditContains(document, "p2p_auth_token", false) {
		t.Fatalf("unexpected non-strict audit document: %+v", document)
	}
}

func TestRunConfigAuditStrictFailsUnsafeDeployment(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	err := runConfig(&bytes.Buffer{}, []string{"audit", "--home", home, "--strict"})
	if err == nil {
		t.Fatal("expected strict audit to fail unsafe deployment")
	}
}

func TestRunConfigAuditPackAndDeploymentTemplate(t *testing.T) {
	var auditOutput bytes.Buffer
	if err := runConfig(&auditOutput, []string{"audit-pack", "--json"}); err != nil {
		t.Fatal(err)
	}
	var auditPack auditPackDocument
	if err := json.Unmarshal(auditOutput.Bytes(), &auditPack); err != nil {
		t.Fatal(err)
	}
	if auditPack.SchemaVersion != "v1" || len(auditPack.Commands) == 0 || !strings.Contains(strings.Join(auditPack.Commands, "\n"), "network longrun-plan") || !strings.Contains(strings.Join(auditPack.Commands, "\n"), "config tune") {
		t.Fatalf("unexpected audit pack: %+v", auditPack)
	}

	var deploymentOutput bytes.Buffer
	if err := runConfig(&deploymentOutput, []string{"deployment-template", "--json"}); err != nil {
		t.Fatal(err)
	}
	var deployment deploymentTemplateDocument
	if err := json.Unmarshal(deploymentOutput.Bytes(), &deployment); err != nil {
		t.Fatal(err)
	}
	if !deployment.Chain.Execution.RequireSigned || !deployment.Chain.Mempool.EnablePriority || !deployment.Runtime.P2PAuthTokenRequired {
		t.Fatalf("unexpected deployment template: %+v", deployment)
	}
}

func TestRunConfigTune(t *testing.T) {
	var output bytes.Buffer
	if err := runConfig(&output, []string{"tune", "--validators", "100", "--tps", "5000", "--regions", "3", "--latency", "120ms", "--json"}); err != nil {
		t.Fatal(err)
	}
	var document struct {
		SchemaVersion string `json:"schema_version"`
		Inputs        struct {
			Validators        int `json:"validators"`
			QuorumVotingPower int `json:"quorum_voting_power"`
		} `json:"inputs"`
		Consensus struct {
			CommitteeSize int `json:"committee_size"`
		} `json:"consensus"`
		Mempool struct {
			MaxTxs int `json:"max_txs"`
		} `json:"mempool"`
	}
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "v1" || document.Inputs.Validators != 100 || document.Inputs.QuorumVotingPower != 67 {
		t.Fatalf("unexpected tuning document: %+v", document)
	}
	if document.Consensus.CommitteeSize < 64 || document.Mempool.MaxTxs == 0 {
		t.Fatalf("unexpected tuning recommendation: %+v", document)
	}
}

func auditContains(document auditDocument, name string, ok bool) bool {
	for _, check := range document.Checks {
		if check.Name == name && check.OK == ok {
			return true
		}
	}
	return false
}

func TestRunStartDryRun(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runStart(&output, []string{"--home", home, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var plan startPlanDocument
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.ChainID != "vexo-test" || plan.ValidatorID != "alice" || plan.ValidatorN != 1 || plan.KeyType == "" || plan.PublicKey == "" || !plan.DryRun {
		t.Fatalf("unexpected start plan: %+v", plan)
	}

	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{"start", "--home", home, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "startup dry-run valid") || !strings.Contains(output.String(), "validator_id: alice") {
		t.Fatalf("unexpected start output:\n%s", output.String())
	}
}

func TestRunStartDryRunWithRotationKeys(t *testing.T) {
	home := t.TempDir()
	rotationKeyPath := filepath.Join(home, "rotation.key.json")
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--id", "key-1", "--active-from", "1", "--active-until", "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", rotationKeyPath, "--id", "key-2", "--active-from", "11"}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runStart(&output, []string{"--home", home, "--rotation-key", rotationKeyPath, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var plan startPlanDocument
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.KeyType != "keyring" || len(plan.RotationKeyPaths) != 1 || !strings.HasSuffix(plan.RotationKeyPaths[0], "rotation.key.json") {
		t.Fatalf("unexpected rotation start plan: %+v", plan)
	}
	inputs, err := loadStartInputs(home, "", "", "", []string{rotationKeyPath}, false)
	if err != nil {
		t.Fatal(err)
	}
	policySigner, ok := inputs.Signer.(vexocrypto.PolicySigner)
	if !ok {
		t.Fatalf("expected policy signer, got %T", inputs.Signer)
	}
	message := []byte("rotation-signing-check")
	signature, err := policySigner.SignWithPolicy(vexocrypto.SignPolicy{
		ChainID: "vexo-test",
		Height:  11,
		Round:   0,
		Type:    vexocrypto.SignTypeConsensusVote,
		Domain:  vexocrypto.DomainConsensusVote,
	}, message)
	if err != nil {
		t.Fatal(err)
	}
	secondDocument, err := vexocrypto.LoadKeyDocument(rotationKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	secondPublicKey, err := base64.StdEncoding.DecodeString(secondDocument.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !inputs.Signer.Verify(secondPublicKey, message, signature) {
		t.Fatal("expected height 11 signature to verify against rotated key")
	}
}

func TestKeysRotationPlanDetectsContiguousWindows(t *testing.T) {
	home := t.TempDir()
	firstKeyPath := filepath.Join(home, "key-1.json")
	secondKeyPath := filepath.Join(home, "key-2.json")
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", firstKeyPath, "--id", "key-1", "--active-from", "1", "--active-until", "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", secondKeyPath, "--id", "key-2", "--active-from", "11"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runKeys(&output, []string{"rotation-plan", "--home", home, "--key", firstKeyPath, "--key", secondKeyPath, "--json"}); err != nil {
		t.Fatal(err)
	}
	var plan keyRotationPlanDocument
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.OK || len(plan.Keys) != 2 || len(plan.Gaps) != 0 || len(plan.Overlaps) != 0 {
		t.Fatalf("unexpected rotation plan: %+v", plan)
	}
}

func TestKeysRotationPlanReportsGapsAndOverlaps(t *testing.T) {
	home := t.TempDir()
	firstKeyPath := filepath.Join(home, "key-1.json")
	secondKeyPath := filepath.Join(home, "key-2.json")
	thirdKeyPath := filepath.Join(home, "key-3.json")
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", firstKeyPath, "--id", "key-1", "--active-from", "1", "--active-until", "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", secondKeyPath, "--id", "key-2", "--active-from", "12", "--active-until", "20"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", thirdKeyPath, "--id", "key-3", "--active-from", "20"}); err != nil {
		t.Fatal(err)
	}
	plan, err := buildKeyRotationPlan(home, []string{firstKeyPath, secondKeyPath, thirdKeyPath}, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.OK || len(plan.Gaps) != 1 || len(plan.Overlaps) != 1 {
		t.Fatalf("expected gap and overlap, got %+v", plan)
	}
}

func TestOpsConformanceIncludesAuditAndRotationPlan(t *testing.T) {
	home := t.TempDir()
	rotationKeyPath := filepath.Join(home, "rotation.key.json")
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--id", "key-1", "--active-from", "1", "--active-until", "10"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--path", rotationKeyPath, "--id", "key-2", "--active-from", "11"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runOps(&output, []string{"conformance", "--home", home, "--rotation-key", rotationKeyPath, "--json"}); err != nil {
		t.Fatal(err)
	}
	var document opsConformanceDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if !document.OK || document.StartPlan.ValidatorID != "alice" || document.RotationPlan == nil || !document.RotationPlan.OK {
		t.Fatalf("unexpected conformance document: %+v", document)
	}
	if len(document.Audit.Checks) == 0 || len(document.Checks) == 0 {
		t.Fatalf("expected audit and conformance checks: %+v", document)
	}
}

func TestBuildStartNodeLoadsValidatorSigner(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Genesis.Validators) != 1 || len(inputs.Genesis.Validators[0].PublicKey) == 0 {
		t.Fatalf("expected local validator public key to be patched from key file: %+v", inputs.Genesis.Validators)
	}
	if string(inputs.Genesis.Validators[0].PublicKey) != string(inputs.Signer.PublicKey()) {
		t.Fatal("expected genesis public key to match loaded signer")
	}
	startNode, err := buildStartNode(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if startNode == nil {
		t.Fatal("expected start node")
	}

	var output bytes.Buffer
	if err := runStart(&output, []string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "validator signer loaded") {
		t.Fatalf("expected signer loaded output, got:\n%s", output.String())
	}
}

func TestBuildStartNodeLoadsRemoteValidatorSigner(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"remote", "--home", home, "--public-key", base64.StdEncoding.EncodeToString(signer.PublicKey()), "--url", "http://127.0.0.1:9000/sign"}); err != nil {
		t.Fatal(err)
	}

	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(inputs.Signer.PublicKey()) != string(signer.PublicKey()) {
		t.Fatal("expected remote signer public key")
	}
	if string(inputs.Genesis.Validators[0].PublicKey) != string(signer.PublicKey()) {
		t.Fatal("expected genesis public key to be patched from remote signer")
	}
}

func TestRunStartRunStartsAndStopsNode(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	networkPath := filepath.Join(home, networkConfigFileName)
	networkDocument, err := readNetworkConfigDocument(networkPath)
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.RPC.Address = "127.0.0.1:0"
	networkDocument.P2P.ListenAddress = "127.0.0.1:0"
	if err := writeJSONFile(networkPath, networkDocument); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	output := &rpcHealthCheckWriter{
		cancel: cancel,
		client: http.Client{Timeout: 5 * time.Second},
	}
	if err := runStartWithContext(ctx, output, []string{"--home", home, "--run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "rpc listening") ||
		!strings.Contains(output.String(), "p2p listening") ||
		!strings.Contains(output.String(), "consensus loop running") ||
		!strings.Contains(output.String(), "node running") ||
		!strings.Contains(output.String(), "shutdown requested") ||
		!strings.Contains(output.String(), "node stopped") {
		t.Fatalf("unexpected run output:\n%s", output.String())
	}
	if !output.rpcOK {
		t.Fatalf("expected RPC health and metrics checks to pass, got:\n%s", output.String())
	}
}

func TestShutdownSignalsIncludeInterrupt(t *testing.T) {
	signals := shutdownSignals()
	if len(signals) == 0 {
		t.Fatal("expected shutdown signals")
	}
	for _, signal := range signals {
		if signal == os.Interrupt {
			return
		}
	}
	t.Fatalf("expected shutdown signals to include os.Interrupt, got %+v", signals)
}

func TestStartPeerFlagsParsePersistentPeers(t *testing.T) {
	peers := peerFlags{}
	if err := peers.Set("bob=127.0.0.1:26657"); err != nil {
		t.Fatal(err)
	}
	if err := peers.Set("carol=127.0.0.1:26658"); err != nil {
		t.Fatal(err)
	}
	if peers["bob"] != "127.0.0.1:26657" || peers["carol"] != "127.0.0.1:26658" {
		t.Fatalf("unexpected peers: %+v", peers)
	}
	if err := peers.Set("bad"); err == nil {
		t.Fatal("expected invalid peer format error")
	}
	if err := peers.Set("bad=0.0.0.0:26656"); err == nil {
		t.Fatal("expected invalid advertised peer address error")
	}
}

func TestStartSeedFlagsMergeIntoGRPCPeers(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	_, wire, err := buildRuntimeNode(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		P2PPeers:         map[p2p.PeerID]string{"bob": "127.0.0.1:26657"},
		P2PSeeds:         map[p2p.PeerID]string{"seed-1": "127.0.0.1:36657"},
	})
	if err != nil {
		t.Fatal(err)
	}
	known := wire.KnownPeers()
	if known["bob"] != "127.0.0.1:26657" || known["seed-1"] != "127.0.0.1:36657" {
		t.Fatalf("expected peers and seeds to be configured, got %+v", known)
	}
}

func TestBuildRuntimeNodePersistsAddrBookPeers(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	addrBookPath := filepath.Join(home, "addrbook.json")
	_, wire, err := buildRuntimeNode(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		P2PPeers:         map[p2p.PeerID]string{"bob": "127.0.0.1:26657"},
		AddrBookPath:     addrBookPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.KnownPeers()["bob"] != "127.0.0.1:26657" {
		t.Fatalf("expected bob configured in wire, got %+v", wire.KnownPeers())
	}
	book, err := p2p.OpenAddrBook(addrBookPath)
	if err != nil {
		t.Fatal(err)
	}
	if book.PeerMap("")["bob"] != "127.0.0.1:26657" {
		t.Fatalf("expected bob persisted in addrbook, got %+v", book.PeerMap(""))
	}
}

func TestBuildRuntimeNodeFiltersBannedAddrBookPeers(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	addrBookPath := filepath.Join(home, "addrbook.json")
	book, err := p2p.OpenAddrBookWithPolicy(addrBookPath, 1)
	if err != nil {
		t.Fatal(err)
	}
	book.Add("bad", "127.0.0.1:26699", "handshake", false)
	book.MarkFailure("bad", time.Hour)
	if err := book.Save(); err != nil {
		t.Fatal(err)
	}

	_, wire, err := buildRuntimeNode(inputs, startRuntimeConfig{
		P2PEnabled:          true,
		P2PListenAddress:    "127.0.0.1:0",
		AddrBookPath:        addrBookPath,
		AddrBookMaxFailures: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := wire.KnownPeers()["bad"]; found {
		t.Fatalf("expected banned addrbook peer filtered, got %+v", wire.KnownPeers())
	}
}

func TestBuildRuntimeNodeConfiguresGRPCTransport(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	startNode, wire, err := buildRuntimeNode(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		P2PNetworkID:     "network",
		P2PPeers:         map[p2p.PeerID]string{"bob": "127.0.0.1:26657"},
		P2PAuthToken:     "shared-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if startNode == nil || wire == nil {
		t.Fatal("expected node and grpc wire")
	}
	if wire.PeerID() != "alice" {
		t.Fatalf("expected peer id alice, got %s", wire.PeerID())
	}
	if wire.Address() != "127.0.0.1:0" {
		t.Fatalf("expected configured listen address before start, got %s", wire.Address())
	}
	if !wire.AuthTokenConfigured() {
		t.Fatal("expected grpc wire auth token to be configured")
	}
}

func TestBuildRuntimeNodeDerivesNetworkPeers(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "3"}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(filepath.Join(home, "validator-2"), "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	runtimeConfig, err := loadStartRuntimeConfig(filepath.Join(home, "validator-2"), "")
	if err != nil {
		t.Fatal(err)
	}
	if runtimeConfig.P2PListenAddress != networkP2PAddress(2) {
		t.Fatalf("expected validator-2 p2p address, got %s", runtimeConfig.P2PListenAddress)
	}
	if runtimeConfig.RPCAddress != networkRPCAddress(2) {
		t.Fatalf("expected validator-2 rpc address, got %s", runtimeConfig.RPCAddress)
	}
	if len(runtimeConfig.P2PPeers) != 2 || runtimeConfig.P2PPeers["validator-1"] != networkP2PAddress(1) || runtimeConfig.P2PPeers["validator-3"] != networkP2PAddress(3) {
		t.Fatalf("unexpected derived peers: %+v", runtimeConfig.P2PPeers)
	}

	startNode, wire, err := buildRuntimeNode(inputs, runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if startNode == nil || wire == nil {
		t.Fatal("expected network node and grpc wire")
	}
	if wire.Address() != networkP2PAddress(2) {
		t.Fatalf("expected network p2p listen address, got %s", wire.Address())
	}
}

func TestConfigBackedPeersRejectDifferentGenesisHash(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2"}); err != nil {
		t.Fatal(err)
	}
	secondGenesisPath := filepath.Join(home, "validator-2", genesisFileName)
	document, err := readGenesisDocument(secondGenesisPath)
	if err != nil {
		t.Fatal(err)
	}
	if document.AppState == nil {
		document.AppState = map[string]string{}
	}
	document.AppState["tampered"] = base64.StdEncoding.EncodeToString([]byte("unexpected"))
	writeTestJSON(t, secondGenesisPath, document)

	firstInputs, err := loadStartInputs(filepath.Join(home, "validator-1"), "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	secondInputs, err := loadStartInputs(filepath.Join(home, "validator-2"), "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	firstWire, err := buildGRPCTransport(firstInputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		AddrBookPath:     filepath.Join(t.TempDir(), "addrbook-1.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	secondWire, err := buildGRPCTransport(secondInputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		AddrBookPath:     filepath.Join(t.TempDir(), "addrbook-2.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := firstWire.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer firstWire.Stop(context.Background())
	if err := secondWire.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer secondWire.Stop(context.Background())

	firstWire.SetPeer("validator-2", secondWire.Address())
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = firstWire.Send(context.Background(), "validator-2", p2p.TopicTx, []byte("tx"))
		if errors.Is(err, transport.ErrGenesisHashMismatch) {
			break
		}
		if !errors.Is(err, context.DeadlineExceeded) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !errors.Is(err, transport.ErrGenesisHashMismatch) {
		t.Fatalf("expected genesis hash mismatch, got %v", err)
	}
}

func TestRunStartRequiresKey(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if err := runStart(&bytes.Buffer{}, []string{"--home", home, "--dry-run"}); err == nil {
		t.Fatal("expected missing key error")
	}
}

type rpcHealthCheckWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
	client http.Client
	rpcOK  bool
}

func (writer *rpcHealthCheckWriter) Write(data []byte) (int, error) {
	count, err := writer.Buffer.Write(data)
	if writer.rpcOK {
		return count, err
	}
	match := regexp.MustCompile(`rpc_address: ([^\s]+)`).FindStringSubmatch(writer.Buffer.String())
	if len(match) != 2 {
		return count, err
	}
	response, requestErr := writer.client.Get("http://" + match[1] + "/healthz")
	if requestErr != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			_ = response.Body.Close()
		}
		return count, err
	}
	_ = response.Body.Close()
	metricsResponse, requestErr := writer.client.Get("http://" + match[1] + "/metrics")
	if requestErr != nil {
		return count, err
	}
	body, readErr := io.ReadAll(metricsResponse.Body)
	_ = metricsResponse.Body.Close()
	if readErr == nil && metricsResponse.StatusCode == http.StatusOK && strings.Contains(string(body), `"consensus_loop_running":true`) {
		writer.rpcOK = true
		writer.cancel()
	}
	return count, err
}

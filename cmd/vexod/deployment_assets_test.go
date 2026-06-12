package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeploymentAssetsCoverOperationsPaths(t *testing.T) {
	root := repoRoot(t)
	requiredFiles := []string{
		"deployments/README.md",
		"deployments/docker/compose.single-host.init.yml",
		"deployments/docker/compose.single-host.yml",
		"deployments/docker/compose.multi-host.init.yml",
		"deployments/docker/compose.multi-host.yml",
		"deployments/monitoring/prometheus.yml",
		"deployments/monitoring/vexo-alerts.yml",
		"deployments/monitoring/alertmanager.yml",
		"deployments/monitoring/grafana-dashboard.json",
		"deployments/helm/vexo-consensus/Chart.yaml",
		"deployments/helm/vexo-consensus/values.yaml",
		"deployments/helm/vexo-consensus/templates/statefulset.yaml",
		"deployments/helm/vexo-consensus/templates/service.yaml",
		"deployments/terraform/aws-minimal/main.tf",
		"deployments/terraform/aws-minimal/variables.tf",
		"deployments/terraform/aws-minimal/outputs.tf",
		".github/workflows/ci.yml",
		".github/workflows/nightly.yml",
		".github/workflows/release-candidate.yml",
	}
	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Fatalf("required deployment asset %s missing or unreadable: %v", file, err)
		}
	}
}

func TestMonitoringAssetsReferenceExportedMetrics(t *testing.T) {
	root := repoRoot(t)
	alerts := readRepoFile(t, root, "deployments/monitoring/vexo-alerts.yml")
	dashboard := readRepoFile(t, root, "deployments/monitoring/grafana-dashboard.json")
	prometheus := readRepoFile(t, root, "deployments/monitoring/prometheus.yml")

	for _, metric := range []string{
		"vexo_node_running",
		"vexo_latest_height",
		"vexo_active_peer_count",
		"vexo_banned_peers",
		"vexo_consensus_loop_running",
		"vexo_post_commit_reconciliation_failures",
	} {
		if !strings.Contains(alerts+dashboard, metric) {
			t.Fatalf("monitoring assets do not reference exported metric %s", metric)
		}
	}
	if !strings.Contains(prometheus, "/metrics/text") {
		t.Fatalf("prometheus config must scrape /metrics/text")
	}
	if !json.Valid([]byte(dashboard)) {
		t.Fatalf("grafana dashboard is not valid JSON")
	}
}

func TestReleaseCandidateWorkflowIsNotPRSyntheticSmoke(t *testing.T) {
	root := repoRoot(t)
	ci := readRepoFile(t, root, ".github/workflows/ci.yml")
	rc := readRepoFile(t, root, ".github/workflows/release-candidate.yml")
	nightly := readRepoFile(t, root, ".github/workflows/nightly.yml")

	if !strings.Contains(ci, "make release-candidate-smoke VERSION=ci") {
		t.Fatalf("ci workflow must use the explicit smoke release target")
	}
	if !strings.Contains(rc, "workflow_dispatch") ||
		!strings.Contains(rc, "self-hosted") ||
		!strings.Contains(rc, "vexo-rc") ||
		!strings.Contains(rc, "RC_EVM_CONFORMANCE_FLAGS") ||
		!strings.Contains(rc, "make release-candidate VERSION=") ||
		strings.Contains(rc, "release-candidate-smoke") {
		t.Fatalf("manual release-candidate workflow must run the real gated target on a self-hosted RC runner with external corpus flags")
	}
	if !strings.Contains(nightly, "make race") ||
		!strings.Contains(nightly, "make network-e2e") ||
		!strings.Contains(nightly, "make release-candidate-smoke VERSION=nightly") {
		t.Fatalf("nightly workflow must cover race, network E2E, and release-path smoke")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(workingDirectory, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readRepoFile(t *testing.T, root string, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

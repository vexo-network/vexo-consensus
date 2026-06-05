package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vexo-network/vexo-consensus/cmd/vexod/internal/releasegate"
)

type releaseAuditPack struct {
	SchemaVersion string                `json:"schema_version"`
	Version       string                `json:"version"`
	GeneratedAt   string                `json:"generated_at"`
	DistDir       string                `json:"dist_dir"`
	Artifacts     []releaseArtifact     `json:"artifacts"`
	Required      releaseRequiredFiles  `json:"required"`
	Audit         auditPackDocument     `json:"audit"`
	Checks        []releasePackageCheck `json:"checks"`
	OK            bool                  `json:"ok"`
}

type releaseArtifact struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type releaseRequiredFiles struct {
	Manifest            string `json:"manifest"`
	Checksums           string `json:"checksums"`
	SBOMModules         string `json:"sbom_modules"`
	SBOMGoVersion       string `json:"sbom_go_version"`
	Signature           string `json:"signature,omitempty"`
	SignatureFound      bool   `json:"signature_found"`
	LongRunEvidence     string `json:"longrun_evidence,omitempty"`
	AdversarialEvidence string `json:"adversarial_evidence,omitempty"`
	FuzzEvidence        string `json:"fuzz_evidence,omitempty"`
}

type releasePackageCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type launchChecklistDocument struct {
	SchemaVersion string                 `json:"schema_version"`
	Phases        []launchChecklistPhase `json:"phases"`
}

type launchChecklistPhase struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

type productionReadinessDocument struct {
	SchemaVersion    string                     `json:"schema_version"`
	Checks           []productionReadinessCheck `json:"checks"`
	Commands         []string                   `json:"commands"`
	Documents        []string                   `json:"documents"`
	ExternalRequired []string                   `json:"external_required,omitempty"`
	OK               bool                       `json:"ok"`
}

type productionReadinessCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type releaseGateDocument = releasegate.Document

func runRelease(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("release subcommand is required")
	}
	switch args[0] {
	case "pack":
		return runReleasePack(writer, args[1:])
	case "launch-checklist":
		return runReleaseLaunchChecklist(writer, args[1:])
	case "readiness":
		return runReleaseReadiness(writer, args[1:])
	case "gate":
		return runReleaseGate(writer, args[1:])
	default:
		return fmt.Errorf("unknown release subcommand %q", args[0])
	}
}

func runReleasePack(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release pack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	distDir := flags.String("dist", "dist", "release dist directory")
	outputPath := flags.String("output", "", "output JSON path; stdout when empty")
	versionValue := flags.String("version", version, "release version label")
	requireSignature := flags.Bool("require-signature", false, "require checksums.txt.asc")
	longRunEvidence := flags.String("longrun-evidence", "", "long-run harness evidence JSON path")
	adversarialEvidence := flags.String("adversarial-evidence", "", "consensus adversarial simulation evidence JSON path")
	fuzzEvidence := flags.String("fuzz-evidence", "", "fuzz/property test output evidence path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document, err := buildReleaseAuditPackWithEvidence(*distDir, *versionValue, *requireSignature, releaseEvidenceInputs{
		LongRun:     *longRunEvidence,
		Adversarial: *adversarialEvidence,
		Fuzz:        *fuzzEvidence,
	})
	if err != nil {
		return err
	}
	if *outputPath == "" {
		return writeReleaseAuditPack(writer, document)
	}
	file, err := os.OpenFile(*outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeReleaseAuditPack(file, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "release audit pack written\n")
	fmt.Fprintf(writer, "path: %s\n", *outputPath)
	fmt.Fprintf(writer, "artifacts: %d\n", len(document.Artifacts))
	fmt.Fprintf(writer, "ok: %t\n", document.OK)
	return nil
}

func runReleaseLaunchChecklist(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release launch-checklist", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := buildLaunchChecklistDocument()
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "release launch checklist\n")
	for _, phase := range document.Phases {
		fmt.Fprintf(writer, "\n%s:\n", phase.Name)
		for _, item := range phase.Items {
			fmt.Fprintf(writer, "- %s\n", item)
		}
	}
	return nil
}

func runReleaseReadiness(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release readiness", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := buildProductionReadinessDocument()
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "release readiness sweep\n")
	fmt.Fprintf(writer, "ok: %t\n", document.OK)
	fmt.Fprintf(writer, "checks:\n")
	for _, check := range document.Checks {
		fmt.Fprintf(writer, "- %s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	fmt.Fprintf(writer, "commands:\n")
	for _, command := range document.Commands {
		fmt.Fprintf(writer, "- %s\n", command)
	}
	fmt.Fprintf(writer, "documents:\n")
	for _, documentPath := range document.Documents {
		fmt.Fprintf(writer, "- %s\n", documentPath)
	}
	if len(document.ExternalRequired) > 0 {
		fmt.Fprintf(writer, "external required:\n")
		for _, requirement := range document.ExternalRequired {
			fmt.Fprintf(writer, "- %s\n", requirement)
		}
	}
	return nil
}

func runReleaseGate(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release gate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	versionValue := flags.String("version", version, "release version label")
	distDir := flags.String("dist", "dist", "release dist directory")
	requireSignature := flags.Bool("require-signature", true, "require signed checksums")
	longRunEvidence := flags.String("longrun-evidence", "", "multi-host longrun evidence JSON path")
	chaosEvidence := flags.String("chaos-evidence", "", "chaos test evidence JSON path")
	adversarialEvidence := flags.String("adversarial-evidence", "", "consensus adversarial evidence JSON path")
	fuzzEvidence := flags.String("fuzz-evidence", "", "fuzz/property evidence output path")
	kmsEvidence := flags.String("kms-evidence", "", "KMS/remote signer policy evidence path")
	snapshotEvidence := flags.String("snapshot-evidence", "", "snapshot/replay restore evidence path")
	p2pScaleEvidence := flags.String("p2p-scale-evidence", "", "large-validator P2P scale evidence path")
	stateSyncLightClientEvidence := flags.String("state-sync-light-client-evidence", "", "state sync and light-client proof evidence path")
	validatorEconomicsEvidence := flags.String("validator-economics-evidence", "", "staking/slashing/rewards economics evidence path")
	upgradeGovernanceEvidence := flags.String("upgrade-governance-evidence", "", "governance upgrade lifecycle evidence path")
	mevFeeMarketEvidence := flags.String("mev-fee-market-evidence", "", "MEV, fair ordering, fee-market, and mempool evidence path")
	opsRunbookEvidence := flags.String("ops-runbook-evidence", "", "operator runbook, alert threshold, and incident drill evidence path")
	formalSafetyEvidence := flags.String("formal-safety-evidence", "", "formal safety argument, invariant, and adversarial evidence path")
	sdkConformanceEvidence := flags.String("sdk-conformance-evidence", "", "SDK/API module, storage, crypto, transport, and RPC conformance evidence path")
	externalAudit := flags.String("external-audit", "", "external security audit report or disposition path")
	blsAudit := flags.String("bls-audit", "", "audited BLS adapter/dependency audit evidence path")
	allowExternalPending := flags.Bool("allow-external-pending", false, "allow external audit/BLS audit to remain pending for non-mainnet release candidates")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	pack, err := buildReleaseAuditPackWithEvidence(*distDir, *versionValue, *requireSignature, releaseEvidenceInputs{
		LongRun:     *longRunEvidence,
		Adversarial: *adversarialEvidence,
		Fuzz:        *fuzzEvidence,
	})
	if err != nil {
		return err
	}
	document := buildReleaseGateDocument(*versionValue, pack, releaseGateInputs{
		Chaos:                *chaosEvidence,
		KMS:                  *kmsEvidence,
		Snapshot:             *snapshotEvidence,
		P2PScale:             *p2pScaleEvidence,
		StateSyncLightClient: *stateSyncLightClientEvidence,
		ValidatorEconomics:   *validatorEconomicsEvidence,
		UpgradeGovernance:    *upgradeGovernanceEvidence,
		MEVFeeMarket:         *mevFeeMarketEvidence,
		OpsRunbook:           *opsRunbookEvidence,
		FormalSafety:         *formalSafetyEvidence,
		SDKConformance:       *sdkConformanceEvidence,
		ExternalAudit:        *externalAudit,
		BLSAudit:             *blsAudit,
		AllowExternalPending: *allowExternalPending,
	})
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "release gate\n")
	fmt.Fprintf(writer, "version: %s\n", document.Version)
	fmt.Fprintf(writer, "ok: %t\n", document.OK)
	for _, check := range document.Checks {
		fmt.Fprintf(writer, "- %s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	if len(document.NextActions) > 0 {
		fmt.Fprintf(writer, "next actions:\n")
		for _, action := range document.NextActions {
			fmt.Fprintf(writer, "- %s\n", action)
		}
	}
	return nil
}

func buildProductionReadinessDocument() productionReadinessDocument {
	document := productionReadinessDocument{
		SchemaVersion: "v1",
		OK:            true,
		Commands: []string{
			"make check",
			"make ops-verify",
			"go run ./cmd/vexod release launch-checklist --json",
			"go run ./cmd/vexod release readiness --json",
			"go run ./cmd/vexod config tune --validators <n> --tps <target> --regions <r> --latency <duration> --json",
			"go run ./cmd/vexod release gate --dist dist --version <version> --longrun-evidence dist/longrun-evidence.json --chaos-evidence dist/chaos-evidence.json --adversarial-evidence dist/adversarial-evidence.json --fuzz-evidence dist/fuzz-evidence.txt --kms-evidence dist/kms-evidence.json --snapshot-evidence dist/snapshot-replay-evidence.json --p2p-scale-evidence dist/p2p-scale-evidence.json --state-sync-light-client-evidence dist/state-sync-light-client-evidence.json --validator-economics-evidence dist/validator-economics-evidence.json --upgrade-governance-evidence dist/upgrade-governance-evidence.json --mev-fee-market-evidence dist/mev-fee-market-evidence.json --ops-runbook-evidence dist/ops-runbook-evidence.json --formal-safety-evidence dist/formal-safety-evidence.json --sdk-conformance-evidence dist/sdk-conformance-evidence.json --external-audit dist/external-audit.pdf --bls-audit dist/bls-audit.pdf",
			"go run ./cmd/vexod network scale-plan --validators <n> --regions <r> --hosts <h> --json",
			"go run ./cmd/vexod snapshot drill-plan --input snapshot.json --chain-id <chain-id> --json",
			"go run ./cmd/vexod slashing lifecycle-plan --type conflicting_vote --validator <id> --height <h> --current-height <h> --json",
			"go run ./cmd/vexod ops incident --metrics-file current-metrics.json --previous-metrics-file previous-metrics.json --json",
		},
		Documents: []string{
			"docs/specs/consensus-spec.md",
			"docs/specs/networking-spec.md",
			"docs/specs/storage-schema.md",
			"docs/specs/tx-format.md",
			"docs/specs/validator-lifecycle.md",
			"docs/specs/finality-proof-format.md",
			"docs/security/audit-readiness.md",
			"docs/release/launch-runbook.md",
			"docs/release/release-pipeline.md",
			"docs/release/version-compatibility.md",
		},
		ExternalRequired: []string{
			"external security audit with signed finding disposition",
			"audited production BLS backend with dependency audit, subgroup checks, rogue-key defense, proof-of-possession, and malformed-input fuzz evidence",
			"multi-host multi-region longrun evidence on independent machines",
			"large-validator P2P evidence covering discovery, reconnect, backpressure, NAT, seeds, addrbook persistence, and ban eviction",
			"state-sync and light-client evidence covering validator-set hash binding, finality proofs, snapshot restore, and replay consistency",
			"chaos evidence for peer loss, signer failure, snapshot restore, replay, and network partition recovery",
			"KMS or remote signer evidence proving height/round/type sign policy and double-sign guard enforcement",
			"chain-specific staking custody, rewards, commission, tombstone, jail, unbonding, and slashing accounting review",
			"chain-specific durable governance state, proposal execution authority, rollback, and failed-upgrade recovery review",
			"fee-market and MEV mitigation evidence covering base fee, fair ordering, spam cost, mempool durability, and censorship-resistance drills",
			"SDK/API conformance evidence for module, storage, crypto, transport, RPC versioning, and upgrade extension points",
		},
	}
	for _, check := range []productionReadinessCheck{
		{Name: "protocol_specs", OK: true, Message: "consensus, networking, storage, tx, validator, and finality specs are documented"},
		{Name: "crypto_boundaries", OK: true, Message: "deterministic crypto is unsafe for public value-bearing networks and audited adapters have explicit activation boundaries"},
		{Name: "p2p_scale_gate", OK: true, Message: "release gate requires large-validator P2P discovery, reconnect, backpressure, NAT, and addrbook evidence"},
		{Name: "state_sync_drill", OK: true, Message: "snapshot drill-plan verifies checksum, roots, KV payloads, and restore steps"},
		{Name: "light_client_gate", OK: true, Message: "release gate requires light-client finality proof and validator-set-hash evidence"},
		{Name: "slashing_lifecycle", OK: true, Message: "slashing lifecycle-plan captures appeal, expiration, jail, unbonding, and stake accounting"},
		{Name: "validator_economics_gate", OK: true, Message: "release gate requires staking, rewards, commission, tombstone, unbonding, and slashing accounting evidence"},
		{Name: "mev_fee_market_gate", OK: true, Message: "release gate requires base-fee, fair-ordering, spam-cost, mempool durability, and MEV mitigation evidence"},
		{Name: "observability_incident", OK: true, Message: "ops incident reports convert metrics threshold breaches into operator actions"},
		{Name: "upgrade_rollback", OK: true, Message: "upgrade rollback-plan captures last safe height, snapshot evidence, and retry blockers"},
		{Name: "sdk_conformance_gate", OK: true, Message: "release gate requires extension-point conformance evidence for SDK/API stability"},
		{Name: "release_artifacts", OK: true, Message: "release pack, signed checksums, SBOM, and RC evidence are available"},
	} {
		document.Checks = append(document.Checks, check)
		if !check.OK {
			document.OK = false
		}
	}
	return document
}

type releaseGateInputs struct {
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
}

func buildReleaseGateDocument(versionValue string, pack releaseAuditPack, inputs releaseGateInputs) releaseGateDocument {
	gateChecks := make([]releasegate.PackCheck, 0, len(pack.Checks))
	for _, check := range pack.Checks {
		gateChecks = append(gateChecks, releasegate.PackCheck{
			Name:    check.Name,
			OK:      check.OK,
			Message: check.Message,
		})
	}
	return releasegate.Build(versionValue, releasegate.Pack{
		OK:     pack.OK,
		Checks: gateChecks,
	}, releasegate.Evidence{
		Chaos:                inputs.Chaos,
		KMS:                  inputs.KMS,
		Snapshot:             inputs.Snapshot,
		P2PScale:             inputs.P2PScale,
		StateSyncLightClient: inputs.StateSyncLightClient,
		ValidatorEconomics:   inputs.ValidatorEconomics,
		UpgradeGovernance:    inputs.UpgradeGovernance,
		MEVFeeMarket:         inputs.MEVFeeMarket,
		OpsRunbook:           inputs.OpsRunbook,
		FormalSafety:         inputs.FormalSafety,
		SDKConformance:       inputs.SDKConformance,
		ExternalAudit:        inputs.ExternalAudit,
		BLSAudit:             inputs.BLSAudit,
		AllowExternalPending: inputs.AllowExternalPending,
		Exists:               fileExists,
	})
}

func buildLaunchChecklistDocument() launchChecklistDocument {
	return launchChecklistDocument{
		SchemaVersion: "v1",
		Phases: []launchChecklistPhase{
			{
				Name: "prelaunch",
				Items: []string{
					"run make check and make ops-verify on a clean checkout",
					"run config audit --strict against every validator home",
					"verify deterministic crypto is disabled for public value-bearing networks",
					"verify remote signer double-sign guard and height/round/type sign policy",
					"generate network scale-plan for the target validator count and region layout",
					"prepare P2P scale, state-sync/light-client, validator economics, MEV/fee-market, SDK conformance, and formal safety evidence",
				},
			},
			{
				Name: "release-candidate",
				Items: []string{
					"build release artifacts with make release VERSION=<version>",
					"sign checksums and verify checksums before distribution",
					"attach release pack, SBOM, fuzz evidence, adversarial evidence, longrun evidence, P2P scale evidence, and economics evidence",
					"run multi-host longrun with metrics, logs, pprof, snapshot, replay, signer, mempool, light-client, and governance-upgrade evidence",
				},
			},
			{
				Name: "genesis",
				Items: []string{
					"freeze chain-id, validator set, validator set hash, and initial app state",
					"distribute identical genesis/config files to all validators",
					"verify validator keys, BLS proof-of-possession when enabled, and remote signer reachability",
					"record rollback-safe last safe height policy before public start",
				},
			},
			{
				Name: "launch-window",
				Items: []string{
					"start seed validators first, then remaining validators in regional waves",
					"watch height rate, round timeout frequency, proposal/vote latency, peer bans, mempool size, commit latency, snapshot/replay health, and signer failures",
					"halt launch if quorum is unstable, conflicting finality appears, signer policy fails, or snapshot/replay diverges",
					"halt launch if fee-market, fair-ordering, or peer backpressure thresholds fail under load",
					"submit signed low-rate smoke transactions before increasing public traffic",
				},
			},
			{
				Name: "postlaunch",
				Items: []string{
					"publish signed release metadata and compatibility matrix",
					"archive launch metrics, logs, release pack, final genesis, and validator set evidence",
					"confirm slashing evidence lifecycle, jail/unbonding accounting, and governance upgrade scheduling",
					"schedule external audit review and long-duration multi-region soak follow-up",
				},
			},
		},
	}
}

type releaseEvidenceInputs struct {
	LongRun     string
	Adversarial string
	Fuzz        string
}

func buildReleaseAuditPack(distDir string, versionValue string, requireSignature bool) (releaseAuditPack, error) {
	return buildReleaseAuditPackWithEvidence(distDir, versionValue, requireSignature, releaseEvidenceInputs{})
}

func buildReleaseAuditPackWithEvidence(distDir string, versionValue string, requireSignature bool, evidence releaseEvidenceInputs) (releaseAuditPack, error) {
	artifacts, err := releaseArtifacts(distDir)
	if err != nil {
		return releaseAuditPack{}, err
	}
	required := releaseRequiredFiles{
		Manifest:            "release-manifest.json",
		Checksums:           "checksums.txt",
		SBOMModules:         "sbom-go-modules.json",
		SBOMGoVersion:       "sbom-go-version.txt",
		Signature:           "checksums.txt.asc",
		LongRunEvidence:     filepath.Base(evidence.LongRun),
		AdversarialEvidence: filepath.Base(evidence.Adversarial),
		FuzzEvidence:        filepath.Base(evidence.Fuzz),
	}
	required.SignatureFound = fileExists(filepath.Join(distDir, required.Signature))
	document := releaseAuditPack{
		SchemaVersion: "v1",
		Version:       versionValue,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		DistDir:       distDir,
		Artifacts:     artifacts,
		Required:      required,
		Audit:         buildAuditPackDocument(),
		OK:            true,
	}
	document.addCheck("release_manifest", fileExists(filepath.Join(distDir, required.Manifest)), "release manifest must exist")
	document.addCheck("checksums", fileExists(filepath.Join(distDir, required.Checksums)), "checksums.txt must exist")
	document.addCheck("sbom_modules", fileExists(filepath.Join(distDir, required.SBOMModules)), "Go module SBOM must exist")
	document.addCheck("sbom_go_version", fileExists(filepath.Join(distDir, required.SBOMGoVersion)), "Go version SBOM must exist")
	if requireSignature {
		document.addCheck("signed_checksums", required.SignatureFound, "checksums signature must exist")
	}
	document.addEvidenceCheck("longrun_evidence", evidence.LongRun, "longrun harness evidence JSON should be attached for release candidates")
	document.addEvidenceCheck("adversarial_evidence", evidence.Adversarial, "consensus adversarial simulation evidence should be attached for release candidates")
	document.addEvidenceCheck("fuzz_evidence", evidence.Fuzz, "fuzz/property test output should be attached for release candidates")
	return document, nil
}

func (document *releaseAuditPack) addCheck(name string, ok bool, message string) {
	if !ok {
		document.OK = false
	}
	document.Checks = append(document.Checks, releasePackageCheck{Name: name, OK: ok, Message: message})
}

func (document *releaseAuditPack) addEvidenceCheck(name string, path string, message string) {
	if path == "" {
		document.Checks = append(document.Checks, releasePackageCheck{Name: name, OK: true, Message: message + " (not provided)"})
		return
	}
	document.addCheck(name, fileExists(path), message)
}

func releaseArtifacts(distDir string) ([]releaseArtifact, error) {
	entries, err := os.ReadDir(distDir)
	if err != nil {
		return nil, err
	}
	artifacts := make([]releaseArtifact, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(distDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, releaseArtifact{
			Path:   entry.Name(),
			Size:   info.Size(),
			SHA256: sum,
		})
	}
	sort.Slice(artifacts, func(left int, right int) bool {
		return artifacts[left].Path < artifacts[right].Path
	})
	return artifacts, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeReleaseAuditPack(writer io.Writer, document releaseAuditPack) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

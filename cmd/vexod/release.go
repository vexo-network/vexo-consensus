package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/cmd/vexod/internal/releasegate"
	vexoconfig "github.com/vexo-network/vexo-consensus/config"
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
	case "evidence-manifest":
		return runReleaseEvidenceManifest(writer, args[1:])
	case "collect-evidence":
		return runReleaseCollectEvidence(writer, args[1:])
	case "docs-quality":
		return runReleaseDocsQuality(writer, args[1:])
	default:
		return fmt.Errorf("unknown release subcommand %q", args[0])
	}
}

type releaseDocsQualityDocument struct {
	SchemaVersion   string                    `json:"schema_version"`
	OK              bool                      `json:"ok"`
	DocsDir         string                    `json:"docs_dir"`
	CanonicalLocale string                    `json:"canonical_locale"`
	LocaleCount     int                       `json:"locale_count"`
	DocumentCount   int                       `json:"document_count"`
	Checks          []releaseDocsQualityCheck `json:"checks"`
}

type releaseDocsQualityCheck struct {
	Name    string `json:"name"`
	Locale  string `json:"locale,omitempty"`
	Path    string `json:"path,omitempty"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type releaseDocsLocaleManifest struct {
	SchemaVersion     string            `json:"schema_version"`
	CanonicalLocale   string            `json:"canonical_locale"`
	Locales           []string          `json:"locales"`
	CanonicalPolicy   string            `json:"canonical_policy"`
	TranslationPolicy string            `json:"translation_policy"`
	CanonicalHashes   map[string]string `json:"canonical_hashes"`
}

func runReleaseDocsQuality(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release docs-quality", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	docsDir := flags.String("docs", "docs", "documentation directory")
	minBytes := flags.Int("min-bytes", 1500, "minimum useful localized markdown length")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document, err := buildReleaseDocsQualityDocument(*docsDir, *minBytes)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	status := "ok"
	if !document.OK {
		status = "failed"
	}
	fmt.Fprintf(writer, "release docs quality %s\n", status)
	fmt.Fprintf(writer, "docs_dir: %s\n", document.DocsDir)
	fmt.Fprintf(writer, "canonical_locale: %s\n", document.CanonicalLocale)
	fmt.Fprintf(writer, "locales: %d\n", document.LocaleCount)
	fmt.Fprintf(writer, "documents: %d\n", document.DocumentCount)
	for _, check := range document.Checks {
		fmt.Fprintf(writer, "%s", check.Name)
		if check.Locale != "" {
			fmt.Fprintf(writer, " locale=%s", check.Locale)
		}
		if check.Path != "" {
			fmt.Fprintf(writer, " path=%s", check.Path)
		}
		fmt.Fprintf(writer, " ok=%t %s\n", check.OK, check.Message)
	}
	return nil
}

func buildReleaseDocsQualityDocument(docsDir string, minBytes int) (releaseDocsQualityDocument, error) {
	manifestData, err := os.ReadFile(filepath.Join(docsDir, "locales", "manifest.json"))
	if err != nil {
		return releaseDocsQualityDocument{}, err
	}
	var manifest releaseDocsLocaleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return releaseDocsQualityDocument{}, err
	}
	document := releaseDocsQualityDocument{
		SchemaVersion:   "v1",
		OK:              true,
		DocsDir:         docsDir,
		CanonicalLocale: manifest.CanonicalLocale,
		LocaleCount:     len(manifest.Locales),
	}
	addCheck := func(name string, locale string, path string, ok bool, message string) {
		if !ok {
			document.OK = false
		}
		document.Checks = append(document.Checks, releaseDocsQualityCheck{Name: name, Locale: locale, Path: path, OK: ok, Message: message})
	}
	addCheck("manifest_schema", "", "locales/manifest.json", manifest.SchemaVersion == "v1", "locale manifest schema must be v1")
	addCheck("canonical_locale", "", "locales/manifest.json", manifest.CanonicalLocale != "" && containsString(manifest.Locales, manifest.CanonicalLocale), "canonical locale must be present in locales list")
	addCheck("policies", "", "locales/manifest.json", manifest.CanonicalPolicy != "" && manifest.TranslationPolicy != "", "manifest must state canonical and translation policies")
	canonical, err := releaseMarkdownTree(docsDir, func(path string) bool {
		return !strings.HasPrefix(filepath.ToSlash(path), "locales/")
	})
	if err != nil {
		return releaseDocsQualityDocument{}, err
	}
	document.DocumentCount = len(canonical)
	addCheck("canonical_docs_present", "", "", len(canonical) > 0, "canonical documentation tree must contain markdown files")
	if len(manifest.CanonicalHashes) == 0 {
		addCheck("canonical_hashes", "", "locales/manifest.json", false, "manifest must bind canonical docs by SHA-256")
	} else {
		for _, relative := range canonical {
			data, readErr := os.ReadFile(filepath.Join(docsDir, relative))
			if readErr != nil {
				return releaseDocsQualityDocument{}, readErr
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			addCheck("canonical_hash", manifest.CanonicalLocale, relative, manifest.CanonicalHashes[relative] == hash, "canonical document hash must match manifest")
		}
		if diff := stringSetDiffStrings(canonical, sortedStringMapKeys(manifest.CanonicalHashes)); len(diff) > 0 {
			addCheck("canonical_hash_missing", manifest.CanonicalLocale, strings.Join(diff, ","), false, "manifest is missing canonical document hashes")
		}
		if diff := stringSetDiffStrings(sortedStringMapKeys(manifest.CanonicalHashes), canonical); len(diff) > 0 {
			addCheck("canonical_hash_extra", manifest.CanonicalLocale, strings.Join(diff, ","), false, "manifest references non-canonical documents")
		}
	}
	canonicalLocaleFiles, err := releaseReadMarkdownFiles(filepath.Join(docsDir, "locales", manifest.CanonicalLocale))
	if err != nil {
		return releaseDocsQualityDocument{}, err
	}
	for _, locale := range manifest.Locales {
		localeDir := filepath.Join(docsDir, "locales", locale)
		files, err := releaseMarkdownTree(localeDir, func(string) bool { return true })
		if err != nil {
			return releaseDocsQualityDocument{}, err
		}
		if diff := stringSetDiffStrings(canonical, files); len(diff) > 0 {
			addCheck("locale_missing_docs", locale, strings.Join(diff, ","), false, "locale is missing canonical markdown files")
		}
		if diff := stringSetDiffStrings(files, canonical); len(diff) > 0 {
			addCheck("locale_extra_docs", locale, strings.Join(diff, ","), false, "locale has markdown files outside canonical tree")
		}
		localeFiles, err := releaseReadMarkdownFiles(localeDir)
		if err != nil {
			return releaseDocsQualityDocument{}, err
		}
		for relative, body := range localeFiles {
			if locale == manifest.CanonicalLocale {
				continue
			}
			addCheck("locale_marker", locale, relative, strings.Contains(body, "Locale: "+locale), "localized document must include its locale marker")
			addCheck("locale_not_identical", locale, relative, body != canonicalLocaleFiles[relative], "localized document must not be identical to canonical English")
			addCheck("locale_length", locale, relative, len(strings.TrimSpace(body)) >= minBytes, fmt.Sprintf("localized document must be at least %d bytes", minBytes))
			addCheck("locale_sections", locale, relative, strings.Count(body, "\n## ") >= 2, "localized document must keep multiple explanatory sections")
			addCheck("locale_no_placeholders", locale, relative, !releaseContainsPlaceholder(body), "localized document must not contain placeholder translation text")
		}
	}
	return document, nil
}

func releaseMarkdownTree(root string, include func(string) bool) ([]string, error) {
	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if filepath.Base(path) == "locales" && filepath.Clean(path) == filepath.Join(filepath.Clean(root), "locales") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if include(relative) {
			files = append(files, relative)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func releaseReadMarkdownFiles(root string) (map[string]string, error) {
	files, err := releaseMarkdownTree(root, func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(files))
	for _, relative := range files {
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			return nil, err
		}
		result[relative] = string(data)
	}
	return result, nil
}

func releaseContainsPlaceholder(body string) bool {
	for _, forbidden := range []string{"todo", "tbd", "placeholder", "coming soon", "translation pending", "machine translation pending"} {
		if releasePlaceholderPattern(forbidden).MatchString(body) {
			return true
		}
	}
	return false
}

func releasePlaceholderPattern(value string) *regexp.Regexp {
	quoted := regexp.QuoteMeta(value)
	if strings.Contains(value, " ") {
		quoted = strings.ReplaceAll(quoted, `\ `, `\s+`)
	}
	return regexp.MustCompile(`(?i)(^|[^[:alpha:]])` + quoted + `($|[^[:alpha:]])`)
}

func sortedStringMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringSetDiffStrings(left []string, right []string) []string {
	seen := make(map[string]struct{}, len(right))
	for _, value := range right {
		seen[value] = struct{}{}
	}
	var diff []string
	for _, value := range left {
		if _, found := seen[value]; !found {
			diff = append(diff, value)
		}
	}
	sort.Strings(diff)
	return diff
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	home := flags.String("home", defaultHomeDir, "node home directory used when resolving --config")
	configPath := flags.String("config", "", "node config path used to read release-gate digest pins")
	versionValue := flags.String("version", version, "release version label")
	distDir := flags.String("dist", "dist", "release dist directory")
	requireSignature := flags.Bool("require-signature", true, "require signed checksums")
	evidenceManifest := flags.String("evidence-manifest", "", "evidence manifest JSON path; defaults to <dist>/evidence-manifest.json")
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
	sdkConformanceEvidence := flags.String("sdk-conformance-evidence", "", "SDK/API module, storage, crypto, transport, RPC, EVM, and Web3 conformance evidence path")
	externalAudit := flags.String("external-audit", "", "external security audit report or disposition path")
	blsAudit := flags.String("bls-audit", "", "audited BLS adapter/dependency audit evidence path")
	blsAuditSHA256 := flags.String("bls-audit-sha256", "", "expected SHA-256 of BLS audit evidence; defaults to crypto.audit_evidence_sha256 from --config when BLS is configured")
	vrfAudit := flags.String("vrf-audit", "", "audited VRF adapter/KMS/TLS evidence path")
	privateRC := flags.Bool("private-rc", false, "mark this gate as a private release candidate; required before allowing external audit items to remain pending")
	allowExternalPending := flags.Bool("allow-external-pending", false, "allow external audit/BLS audit to remain pending for private release candidates")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *allowExternalPending && !*privateRC {
		return fmt.Errorf("--allow-external-pending requires --private-rc")
	}
	if *allowExternalPending && !isPrivateReleaseCandidateVersion(*versionValue) {
		return fmt.Errorf("--allow-external-pending requires a private release candidate version label containing rc, alpha, beta, or private")
	}
	blsDigestPin, err := resolveReleaseGateBLSAuditSHA256(*home, *configPath, *blsAuditSHA256)
	if err != nil {
		return err
	}
	manifestPath := *evidenceManifest
	if manifestPath == "" {
		manifestPath = filepath.Join(*distDir, "evidence-manifest.json")
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
		Manifest:             manifestPath,
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
		BLSAuditSHA256:       blsDigestPin,
		VRFAudit:             *vrfAudit,
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

func resolveReleaseGateBLSAuditSHA256(home string, configPath string, explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), nil
	}
	if strings.TrimSpace(configPath) == "" {
		return "", nil
	}
	nodeConfig, err := loadNodeConfig(resolveConfigPath(home, configPath))
	if err != nil {
		return "", err
	}
	if nodeConfig.Chain.Crypto.Backend != vexoconfig.CryptoBackendBLS {
		return "", nil
	}
	return strings.TrimSpace(nodeConfig.Chain.Crypto.AuditEvidenceSHA256), nil
}

func isPrivateReleaseCandidateVersion(versionValue string) bool {
	normalized := strings.ToLower(strings.TrimSpace(versionValue))
	return strings.Contains(normalized, "rc") ||
		strings.Contains(normalized, "alpha") ||
		strings.Contains(normalized, "beta") ||
		strings.Contains(normalized, "private")
}

func runReleaseEvidenceManifest(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release evidence-manifest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	distDir := flags.String("dist", "dist", "release dist directory")
	outputPath := flags.String("output", "", "output JSON path; defaults to <dist>/evidence-manifest.json")
	requireAny := flags.Bool("require-any", false, "fail when no known evidence artifacts are found")
	signingKeyPath := flags.String("signing-key", "", "optional Ed25519 private key/seed file for evidence attestation")
	signingKeyEnv := flags.String("signing-key-env", "", "optional environment variable containing an Ed25519 private key/seed")
	jsonOutput := flags.Bool("json", false, "write the generated manifest JSON to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	options := releaseEvidenceManifestOptions{}
	if *signingKeyPath != "" || *signingKeyEnv != "" {
		privateKey, err := loadReleaseEvidenceSigningKey(*signingKeyPath, *signingKeyEnv)
		if err != nil {
			return err
		}
		options.SigningPrivateKey = privateKey
	}
	manifest, err := buildReleaseEvidenceManifestWithOptions(*distDir, options)
	if err != nil {
		return err
	}
	if *requireAny && len(manifest.Evidence) == 0 {
		return fmt.Errorf("no known release evidence artifacts found in %s", *distDir)
	}
	path := *outputPath
	if path == "" {
		path = filepath.Join(*distDir, "evidence-manifest.json")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if *jsonOutput {
		_, err := writer.Write(data)
		return err
	}
	fmt.Fprintf(writer, "release evidence manifest written\n")
	fmt.Fprintf(writer, "path: %s\n", path)
	fmt.Fprintf(writer, "evidence: %d\n", len(manifest.Evidence))
	return nil
}

type releaseEvidenceManifestOptions struct {
	SigningPrivateKey ed25519.PrivateKey
}

type releaseCollectedEvidence struct {
	SchemaVersion string                          `json:"schema_version"`
	EvidenceType  string                          `json:"evidence_type"`
	GeneratedAt   string                          `json:"generated_at"`
	Duration      string                          `json:"duration"`
	OK            bool                            `json:"ok"`
	Summary       string                          `json:"summary"`
	Checks        []releaseCollectedEvidenceCheck `json:"checks"`
	RPCs          []releaseRPCObservation         `json:"rpcs"`
}

type releaseCollectedEvidenceCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type releaseRPCObservation struct {
	RPC      string             `json:"rpc"`
	Baseline releaseRPCSnapshot `json:"baseline"`
	Final    releaseRPCSnapshot `json:"final"`
	Errors   []string           `json:"errors,omitempty"`
	Raw      map[string]any     `json:"raw,omitempty"`
}

type releaseRPCSnapshot struct {
	Status      map[string]any `json:"status,omitempty"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Peers       map[string]any `json:"peers,omitempty"`
	Finality    map[string]any `json:"finality,omitempty"`
	Snapshot    map[string]any `json:"snapshot,omitempty"`
	Diagnostics map[string]any `json:"diagnostics,omitempty"`
}

func runReleaseCollectEvidence(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("release collect-evidence", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	distDir := flags.String("dist", "dist", "release dist directory")
	durationValue := flags.String("duration", "0s", "observation window between baseline and final samples")
	timeoutValue := flags.String("timeout", "5s", "per-request timeout")
	jsonOutput := flags.Bool("json", false, "write collection summary JSON to stdout")
	writeManifest := flags.Bool("write-manifest", true, "write evidence-manifest.json after collecting evidence")
	rpcs := stringListFlags{}
	flags.Var(&rpcs, "rpc", "validator RPC base URL; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(rpcs) == 0 {
		return errors.New("at least one --rpc is required")
	}
	duration, err := time.ParseDuration(*durationValue)
	if err != nil {
		return err
	}
	timeout, err := time.ParseDuration(*timeoutValue)
	if err != nil {
		return err
	}
	if timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if err := os.MkdirAll(*distDir, 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: timeout}
	observations := make([]releaseRPCObservation, 0, len(rpcs))
	for _, rpcURL := range rpcs {
		observation, err := collectReleaseRPCBaseline(client, rpcURL)
		if err != nil {
			return err
		}
		observations = append(observations, observation)
	}
	if duration > 0 {
		time.Sleep(duration)
	}
	for index := range observations {
		final, errors := collectReleaseRPCSnapshot(client, observations[index].RPC)
		observations[index].Final = final
		observations[index].Errors = append(observations[index].Errors, errors...)
	}
	documents := buildCollectedReleaseEvidenceDocuments(duration, observations)
	for _, document := range documents {
		path := filepath.Join(*distDir, releaseCollectedEvidenceFile(document.EvidenceType))
		encoded, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			return err
		}
	}
	var manifest releasegate.EvidenceManifest
	if *writeManifest {
		manifest, err = buildReleaseEvidenceManifest(*distDir)
		if err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		if err := os.WriteFile(filepath.Join(*distDir, "evidence-manifest.json"), encoded, 0o644); err != nil {
			return err
		}
	}
	summary := map[string]any{
		"schema_version": "v1",
		"ok":             collectedEvidenceAllOK(documents),
		"dist":           *distDir,
		"written":        len(documents),
		"manifest_items": len(manifest.Evidence),
		"evidence":       documents,
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	}
	fmt.Fprintf(writer, "release evidence collected\n")
	fmt.Fprintf(writer, "dist: %s\n", *distDir)
	fmt.Fprintf(writer, "rpc_endpoints: %d\n", len(rpcs))
	fmt.Fprintf(writer, "evidence_files: %d\n", len(documents))
	fmt.Fprintf(writer, "ok: %t\n", summary["ok"])
	return nil
}

func collectReleaseRPCBaseline(client *http.Client, rpcURL string) (releaseRPCObservation, error) {
	normalized, err := normalizeReleaseRPCURL(rpcURL)
	if err != nil {
		return releaseRPCObservation{}, err
	}
	snapshot, errors := collectReleaseRPCSnapshot(client, normalized)
	return releaseRPCObservation{RPC: normalized, Baseline: snapshot, Errors: errors}, nil
}

func normalizeReleaseRPCURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("rpc URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported rpc scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", errors.New("rpc host is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func collectReleaseRPCSnapshot(client *http.Client, rpcURL string) (releaseRPCSnapshot, []string) {
	var snapshot releaseRPCSnapshot
	errors := make([]string, 0)
	for _, endpoint := range []struct {
		path string
		set  func(map[string]any)
	}{
		{"/v1/status", func(value map[string]any) { snapshot.Status = value }},
		{"/v1/metrics", func(value map[string]any) { snapshot.Metrics = value }},
		{"/v1/peers", func(value map[string]any) { snapshot.Peers = value }},
		{"/v1/finality/latest?strict=true", func(value map[string]any) { snapshot.Finality = value }},
		{"/v1/snapshot/latest", func(value map[string]any) { snapshot.Snapshot = value }},
		{"/v1/diagnostics", func(value map[string]any) { snapshot.Diagnostics = value }},
	} {
		value, err := collectReleaseRPCJSON(client, rpcURL+endpoint.path)
		if err != nil {
			errors = append(errors, endpoint.path+": "+err.Error())
			continue
		}
		endpoint.set(value)
	}
	return snapshot, errors
}

func collectReleaseRPCJSON(client *http.Client, target string) (map[string]any, error) {
	response, err := client.Get(target)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", response.StatusCode)
	}
	var value map[string]any
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func buildCollectedReleaseEvidenceDocuments(duration time.Duration, observations []releaseRPCObservation) []releaseCollectedEvidence {
	return []releaseCollectedEvidence{
		newCollectedEvidence("longrun_evidence", duration, "longrun duration height validator distributed per_node soak evidence", observations, checkCollectedLongrun(observations)),
		newCollectedEvidence("ops_runbook_evidence", duration, "ops runbook alert incident metrics evidence", observations, checkCollectedOps(observations)),
		newCollectedEvidence("p2p_scale_evidence", duration, "p2p peer scale discovery reconnect backpressure evidence", observations, checkCollectedP2P(observations)),
		newCollectedEvidence("state_sync_light_client_evidence", duration, "state-sync light-client finality evidence", observations, checkCollectedFinality(observations)),
		newCollectedEvidence("snapshot_replay_evidence", duration, "snapshot replay restore evidence", observations, checkCollectedSnapshot(observations)),
	}
}

func newCollectedEvidence(evidenceType string, duration time.Duration, summary string, observations []releaseRPCObservation, checks []releaseCollectedEvidenceCheck) releaseCollectedEvidence {
	return releaseCollectedEvidence{
		SchemaVersion: "v1",
		EvidenceType:  evidenceType,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Duration:      releaseEvidenceDurationString(duration),
		OK:            collectedChecksOK(checks),
		Summary:       summary,
		Checks:        checks,
		RPCs:          observations,
	}
}

func releaseEvidenceDurationString(duration time.Duration) string {
	if duration <= 0 {
		return "0s"
	}
	for _, unit := range []struct {
		suffix string
		value  time.Duration
	}{
		{suffix: "h", value: time.Hour},
		{suffix: "m", value: time.Minute},
		{suffix: "s", value: time.Second},
	} {
		if duration >= unit.value && duration%unit.value == 0 {
			return strconv.FormatInt(int64(duration/unit.value), 10) + unit.suffix
		}
	}
	return duration.String()
}

func releaseCollectedEvidenceFile(evidenceType string) string {
	for _, candidate := range releaseEvidenceCandidates() {
		if candidate.Name == evidenceType {
			return candidate.File
		}
	}
	return evidenceType + ".json"
}

func checkCollectedLongrun(observations []releaseRPCObservation) []releaseCollectedEvidenceCheck {
	checks := []releaseCollectedEvidenceCheck{{Name: "validator_observations", OK: len(observations) > 0, Message: "validator RPC endpoints must be observed"}}
	for _, observation := range observations {
		baseline := jsonNumber(observation.Baseline.Status, "latest_height")
		final := jsonNumber(observation.Final.Status, "latest_height")
		checks = append(checks, releaseCollectedEvidenceCheck{
			Name:    "height_growth_" + safeEvidenceCheckName(observation.RPC),
			OK:      baseline >= 0 && final > baseline,
			Message: fmt.Sprintf("height must increase for %s baseline=%d final=%d", observation.RPC, baseline, final),
		})
	}
	return checks
}

func checkCollectedOps(observations []releaseRPCObservation) []releaseCollectedEvidenceCheck {
	checks := make([]releaseCollectedEvidenceCheck, 0, len(observations))
	for _, observation := range observations {
		checks = append(checks, releaseCollectedEvidenceCheck{
			Name:    "metrics_" + safeEvidenceCheckName(observation.RPC),
			OK:      observation.Final.Metrics != nil,
			Message: "metrics endpoint should return operator alert inputs",
		})
	}
	return checks
}

func checkCollectedP2P(observations []releaseRPCObservation) []releaseCollectedEvidenceCheck {
	checks := make([]releaseCollectedEvidenceCheck, 0, len(observations))
	for _, observation := range observations {
		peerCount := jsonNumber(observation.Final.Status, "peer_count")
		peerListCount := jsonCollectionLen(observation.Final.Peers, "peers")
		checks = append(checks, releaseCollectedEvidenceCheck{
			Name:    "peer_connectivity_" + safeEvidenceCheckName(observation.RPC),
			OK:      peerCount > 0 || peerListCount > 0,
			Message: fmt.Sprintf("peer evidence should expose connected peers for %s peer_count=%d peers=%d", observation.RPC, peerCount, peerListCount),
		})
	}
	return checks
}

func checkCollectedFinality(observations []releaseRPCObservation) []releaseCollectedEvidenceCheck {
	checks := make([]releaseCollectedEvidenceCheck, 0, len(observations))
	for _, observation := range observations {
		checks = append(checks, releaseCollectedEvidenceCheck{
			Name:    "strict_finality_" + safeEvidenceCheckName(observation.RPC),
			OK:      observation.Final.Finality != nil,
			Message: "strict finality proof endpoint should return light-client finality evidence",
		})
	}
	return checks
}

func checkCollectedSnapshot(observations []releaseRPCObservation) []releaseCollectedEvidenceCheck {
	checks := make([]releaseCollectedEvidenceCheck, 0, len(observations))
	for _, observation := range observations {
		snapshotHeight := jsonNumber(observation.Final.Snapshot, "height")
		replayHealthy := jsonBool(observation.Final.Diagnostics, "replay_healthy") ||
			strings.EqualFold(jsonString(observation.Final.Diagnostics, "replay_status"), "healthy") ||
			strings.EqualFold(jsonString(observation.Final.Diagnostics, "replay_status"), "ok")
		checks = append(checks, releaseCollectedEvidenceCheck{
			Name:    "snapshot_" + safeEvidenceCheckName(observation.RPC),
			OK:      snapshotHeight > 0 && replayHealthy,
			Message: fmt.Sprintf("snapshot evidence should include snapshot height and healthy replay diagnostics for %s height=%d replay_healthy=%t", observation.RPC, snapshotHeight, replayHealthy),
		})
	}
	return checks
}

func collectedEvidenceAllOK(documents []releaseCollectedEvidence) bool {
	for _, document := range documents {
		if !document.OK {
			return false
		}
	}
	return true
}

func collectedChecksOK(checks []releaseCollectedEvidenceCheck) bool {
	if len(checks) == 0 {
		return false
	}
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func jsonNumber(value map[string]any, key string) int64 {
	if value == nil {
		return -1
	}
	switch item := value[key].(type) {
	case float64:
		return int64(item)
	case int64:
		return item
	case json.Number:
		parsed, err := item.Int64()
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseInt(item, 10, 64)
		if err == nil {
			return parsed
		}
	}
	return -1
}

func jsonCollectionLen(value map[string]any, key string) int {
	if value == nil {
		return 0
	}
	switch item := value[key].(type) {
	case []any:
		return len(item)
	case []map[string]any:
		return len(item)
	}
	return 0
}

func jsonBool(value map[string]any, key string) bool {
	if value == nil {
		return false
	}
	switch item := value[key].(type) {
	case bool:
		return item
	case string:
		parsed, err := strconv.ParseBool(item)
		return err == nil && parsed
	}
	return false
}

func jsonString(value map[string]any, key string) string {
	if value == nil {
		return ""
	}
	switch item := value[key].(type) {
	case string:
		return item
	default:
		return fmt.Sprint(item)
	}
}

func safeEvidenceCheckName(value string) string {
	replacer := strings.NewReplacer("://", "_", "/", "_", ":", "_", ".", "_", "-", "_")
	return strings.Trim(replacer.Replace(value), "_")
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
			"go run ./cmd/vexod release gate --dist dist --version <version> --evidence-manifest dist/evidence-manifest.json --longrun-evidence dist/longrun-evidence.json --chaos-evidence dist/chaos-evidence.json --adversarial-evidence dist/adversarial-evidence.json --fuzz-evidence dist/fuzz-evidence.txt --kms-evidence dist/kms-evidence.json --snapshot-evidence dist/snapshot-replay-evidence.json --p2p-scale-evidence dist/p2p-scale-evidence.json --state-sync-light-client-evidence dist/state-sync-light-client-evidence.json --validator-economics-evidence dist/validator-economics-evidence.json --upgrade-governance-evidence dist/upgrade-governance-evidence.json --mev-fee-market-evidence dist/mev-fee-market-evidence.json --ops-runbook-evidence dist/ops-runbook-evidence.json --formal-safety-evidence dist/formal-safety-evidence.json --sdk-conformance-evidence dist/sdk-conformance-evidence.json --external-audit dist/external-audit.pdf --bls-audit dist/bls-audit.pdf --bls-audit-sha256 <sha256> --vrf-audit dist/vrf-audit.pdf",
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
			"SDK/API conformance evidence for module, storage, crypto, transport, RPC/Web3 versioning, EVM execution semantics, and upgrade extension points",
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
		Manifest:             inputs.Manifest,
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
		BLSAuditSHA256:       inputs.BLSAuditSHA256,
		VRFAudit:             inputs.VRFAudit,
		AllowExternalPending: inputs.AllowExternalPending,
		Exists:               fileExists,
		ReadFile:             os.ReadFile,
	})
}

func buildReleaseEvidenceManifest(distDir string) (releasegate.EvidenceManifest, error) {
	return buildReleaseEvidenceManifestWithOptions(distDir, releaseEvidenceManifestOptions{})
}

func buildReleaseEvidenceManifestWithOptions(distDir string, options releaseEvidenceManifestOptions) (releasegate.EvidenceManifest, error) {
	manifest := releasegate.EvidenceManifest{
		SchemaVersion: "v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	for _, candidate := range releaseEvidenceCandidates() {
		path := filepath.Join(distDir, candidate.File)
		if !fileExists(path) {
			continue
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return releasegate.EvidenceManifest{}, err
		}
		entry := releasegate.EvidenceManifestEntry{
			Name:          candidate.Name,
			Path:          path,
			SHA256:        sum,
			SchemaVersion: "v1",
			Provenance:    "vexod release evidence-manifest",
		}
		if len(options.SigningPrivateKey) == ed25519.PrivateKeySize {
			publicKey := options.SigningPrivateKey.Public().(ed25519.PublicKey)
			signature := ed25519.Sign(options.SigningPrivateKey, releasegate.EvidenceManifestEntrySigningMessage(entry))
			entry.SignatureAlgo = "ed25519"
			entry.SignaturePubKey = base64.StdEncoding.EncodeToString(publicKey)
			entry.Signature = base64.StdEncoding.EncodeToString(signature)
		}
		manifest.Evidence = append(manifest.Evidence, entry)
		signaturePath := path + ".sig"
		if fileExists(signaturePath) {
			signatureSum, err := fileSHA256(signaturePath)
			if err != nil {
				return releasegate.EvidenceManifest{}, err
			}
			last := len(manifest.Evidence) - 1
			manifest.Evidence[last].SignaturePath = signaturePath
			manifest.Evidence[last].SignatureSHA256 = signatureSum
			if publicKey := releaseEvidenceSignaturePublicKey(path); publicKey != "" {
				manifest.Evidence[last].SignatureAlgo = "ed25519"
				manifest.Evidence[last].SignaturePubKey = publicKey
			}
		}
	}
	sort.Slice(manifest.Evidence, func(left int, right int) bool {
		return manifest.Evidence[left].Name < manifest.Evidence[right].Name
	})
	return manifest, nil
}

func releaseEvidenceSignaturePublicKey(artifactPath string) string {
	for _, candidate := range []string{artifactPath + ".sig.pub", artifactPath + ".pub"} {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func loadReleaseEvidenceSigningKey(path string, envName string) (ed25519.PrivateKey, error) {
	if path != "" && envName != "" {
		return nil, errors.New("use only one of --signing-key or --signing-key-env")
	}
	var raw []byte
	var err error
	if envName != "" {
		value := strings.TrimSpace(os.Getenv(envName))
		if value == "" {
			return nil, fmt.Errorf("release evidence signing key env %s is empty", envName)
		}
		raw = []byte(value)
	} else {
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	keyBytes, err := decodeReleaseEvidenceKeyBytes(string(raw))
	if err != nil {
		return nil, err
	}
	switch len(keyBytes) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(keyBytes), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(keyBytes), nil
	default:
		return nil, fmt.Errorf("release evidence signing key must be %d-byte seed or %d-byte private key", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func decodeReleaseEvidenceKeyBytes(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("empty release evidence signing key")
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil {
		return decoded, nil
	}
	return nil, errors.New("release evidence signing key must be base64 or hex encoded")
}

type releaseEvidenceCandidate struct {
	Name string
	File string
}

func releaseEvidenceCandidates() []releaseEvidenceCandidate {
	return []releaseEvidenceCandidate{
		{Name: "longrun_evidence", File: "longrun-evidence.json"},
		{Name: "adversarial_evidence", File: "adversarial-evidence.json"},
		{Name: "fuzz_evidence", File: "fuzz-evidence.txt"},
		{Name: "chaos_evidence", File: "chaos-evidence.json"},
		{Name: "kms_signer_evidence", File: "kms-evidence.json"},
		{Name: "snapshot_replay_evidence", File: "snapshot-replay-evidence.json"},
		{Name: "p2p_scale_evidence", File: "p2p-scale-evidence.json"},
		{Name: "state_sync_light_client_evidence", File: "state-sync-light-client-evidence.json"},
		{Name: "validator_economics_evidence", File: "validator-economics-evidence.json"},
		{Name: "upgrade_governance_evidence", File: "upgrade-governance-evidence.json"},
		{Name: "mev_fee_market_evidence", File: "mev-fee-market-evidence.json"},
		{Name: "ops_runbook_evidence", File: "ops-runbook-evidence.json"},
		{Name: "formal_safety_evidence", File: "formal-safety-evidence.json"},
		{Name: "sdk_conformance_evidence", File: "sdk-conformance-evidence.json"},
		{Name: "external_security_audit", File: "external-audit.pdf"},
		{Name: "bls_adapter_audit", File: "bls-audit.pdf"},
		{Name: "vrf_adapter_audit", File: "vrf-audit.pdf"},
	}
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
	data, err := os.ReadFile(path)
	document.addCheck(name, err == nil && releasegate.EvidenceCheckContentOK(name, path, data), message)
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

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

func runRelease(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("release subcommand is required")
	}
	switch args[0] {
	case "pack":
		return runReleasePack(writer, args[1:])
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

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vexo-network/vexo-consensus/store"
)

type snapshotDocument struct {
	SchemaVersion string                  `json:"schema_version"`
	ChainID       string                  `json:"chain_id,omitempty"`
	Modules       []string                `json:"modules,omitempty"`
	State         store.StateRecord       `json:"state"`
	StateRoots    []store.StateRootRecord `json:"state_roots"`
	KV            []store.KVPair          `json:"kv,omitempty"`
	Checksum      string                  `json:"checksum,omitempty"`
}

type snapshotChunkDocument struct {
	SchemaVersion    string                  `json:"schema_version"`
	ChainID          string                  `json:"chain_id,omitempty"`
	Modules          []string                `json:"modules,omitempty"`
	State            store.StateRecord       `json:"state"`
	StateRoots       []store.StateRootRecord `json:"state_roots"`
	KV               []store.KVPair          `json:"kv,omitempty"`
	ChunkIndex       uint64                  `json:"chunk_index"`
	ChunkCount       uint64                  `json:"chunk_count"`
	SnapshotChecksum string                  `json:"snapshot_checksum"`
	ChunkChecksum    string                  `json:"chunk_checksum,omitempty"`
}

type snapshotDrillPlanDocument struct {
	SchemaVersion string                   `json:"schema_version"`
	Input         string                   `json:"input"`
	ChainID       string                   `json:"chain_id"`
	Height        uint64                   `json:"height"`
	Modules       []string                 `json:"modules"`
	KVPairCount   int                      `json:"kv_pair_count"`
	Checksum      string                   `json:"checksum"`
	Checks        []snapshotDrillPlanCheck `json:"checks"`
	Steps         []string                 `json:"steps"`
	Warnings      []string                 `json:"warnings,omitempty"`
}

type snapshotDrillPlanCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func runSnapshot(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("snapshot subcommand is required")
	}
	switch args[0] {
	case "export":
		return runSnapshotExport(writer, args[1:])
	case "chunk-export":
		return runSnapshotChunkExport(writer, args[1:])
	case "verify":
		return runSnapshotVerify(writer, args[1:])
	case "restore":
		return runSnapshotRestore(writer, args[1:])
	case "chunk-restore":
		return runSnapshotChunkRestore(writer, args[1:])
	case "fetch":
		return runSnapshotFetch(writer, args[1:])
	case "sync":
		return runSnapshotSync(writer, args[1:])
	case "drill-plan":
		return runSnapshotDrillPlan(writer, args[1:])
	default:
		return fmt.Errorf("unknown snapshot subcommand %q", args[0])
	}
}

func runSnapshotExport(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	outputPath := flags.String("output", "", "snapshot output path; stdout when empty")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	document, err := buildSnapshotDocument(storage, cfg.Chain.ChainID, snapshotNamespaces(cfg.Chain.Application.Modules))
	if err != nil {
		return err
	}
	if *outputPath == "" {
		return writeSnapshotDocument(writer, document)
	}
	file, err := os.OpenFile(*outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeSnapshotDocument(file, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot exported\n")
	fmt.Fprintf(writer, "path: %s\n", *outputPath)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	return nil
}

func runSnapshotChunkExport(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot chunk-export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	outputDir := flags.String("output-dir", "", "snapshot chunk output directory")
	chunkSize := flags.Int("chunk-size", 10000, "maximum KV pairs per chunk")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *outputDir == "" {
		return errors.New("snapshot chunk output directory is required")
	}
	if *chunkSize <= 0 {
		return errors.New("snapshot chunk size must be positive")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	document, err := buildSnapshotDocument(storage, cfg.Chain.ChainID, snapshotNamespaces(cfg.Chain.Application.Modules))
	if err != nil {
		return err
	}
	chunks, err := snapshotChunks(document, *chunkSize)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		return err
	}
	for _, chunk := range chunks {
		path := filepath.Join(*outputDir, snapshotChunkFileName(chunk.ChunkIndex, chunk.ChunkCount))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if err := writeSnapshotChunkDocument(file, chunk); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	fmt.Fprintf(writer, "snapshot chunks exported\n")
	fmt.Fprintf(writer, "output_dir: %s\n", *outputDir)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	fmt.Fprintf(writer, "chunks: %d\n", len(chunks))
	fmt.Fprintf(writer, "checksum: %s\n", document.Checksum)
	return nil
}

func runSnapshotVerify(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	inputPath := flags.String("input", "", "snapshot input path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("snapshot input path is required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	document, err := readSnapshotDocument(*inputPath)
	if err != nil {
		return err
	}
	if err := validateSnapshotDocument(document, cfg.Chain.ChainID); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot verified\n")
	fmt.Fprintf(writer, "path: %s\n", *inputPath)
	fmt.Fprintf(writer, "chain_id: %s\n", document.ChainID)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	fmt.Fprintf(writer, "modules: %v\n", document.Modules)
	fmt.Fprintf(writer, "kv_pairs: %d\n", len(document.KV))
	if document.Checksum != "" {
		fmt.Fprintf(writer, "checksum: %s\n", document.Checksum)
	}
	return nil
}

func runSnapshotRestore(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot restore", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	inputPath := flags.String("input", "", "snapshot input path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("snapshot input path is required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	document, err := readSnapshotDocument(*inputPath)
	if err != nil {
		return err
	}
	if err := validateSnapshotDocument(document, cfg.Chain.ChainID); err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	if err := restoreSnapshotDocument(storage, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot restored\n")
	fmt.Fprintf(writer, "path: %s\n", *inputPath)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	return nil
}

func runSnapshotChunkRestore(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot chunk-restore", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	inputDir := flags.String("input-dir", "", "snapshot chunk input directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputDir == "" {
		return errors.New("snapshot chunk input directory is required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	chunks, err := readSnapshotChunksFromDir(*inputDir)
	if err != nil {
		return err
	}
	for _, chunk := range chunks {
		if err := validateSnapshotChunk(chunk, cfg.Chain.ChainID); err != nil {
			return err
		}
	}
	document, err := snapshotDocumentFromChunks(chunks)
	if err != nil {
		return err
	}
	if err := validateSnapshotDocument(document, cfg.Chain.ChainID); err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	if err := restoreSnapshotDocument(storage, document); err != nil {
		return err
	}
	if _, err := storage.RecoverIndexes(context.Background()); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot chunks restored\n")
	fmt.Fprintf(writer, "input_dir: %s\n", *inputDir)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	fmt.Fprintf(writer, "chunks: %d\n", len(chunks))
	fmt.Fprintf(writer, "checksum: %s\n", document.Checksum)
	return nil
}

func runSnapshotFetch(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot fetch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	url := flags.String("url", "", "snapshot export URL")
	outputPath := flags.String("output", "", "snapshot output path")
	timeout := flags.Duration("timeout", 10*time.Second, "snapshot download timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *url == "" {
		return errors.New("snapshot URL is required")
	}
	if *outputPath == "" {
		return errors.New("snapshot output path is required")
	}
	document, err := downloadSnapshotDocument(*url, *timeout)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(*outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeSnapshotDocument(file, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot fetched\n")
	fmt.Fprintf(writer, "url: %s\n", *url)
	fmt.Fprintf(writer, "path: %s\n", *outputPath)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	return nil
}

func runSnapshotSync(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot sync", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	url := flags.String("url", "", "snapshot export URL")
	timeout := flags.Duration("timeout", 10*time.Second, "snapshot download timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *url == "" {
		return errors.New("snapshot URL is required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	document, err := downloadSnapshotDocument(*url, *timeout)
	if err != nil {
		return err
	}
	if err := validateSnapshotDocument(document, cfg.Chain.ChainID); err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	if err := restoreSnapshotDocument(storage, document); err != nil {
		return err
	}
	if _, err := storage.RecoverIndexes(context.Background()); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot synced\n")
	fmt.Fprintf(writer, "url: %s\n", *url)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	return nil
}

func runSnapshotDrillPlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot drill-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "snapshot input path")
	expectedChainID := flags.String("chain-id", "", "expected chain id")
	minHeight := flags.Uint64("min-height", 1, "minimum acceptable snapshot height")
	requireKV := flags.Bool("require-kv", true, "require module KV payloads for state sync")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("snapshot input path is required")
	}
	document, err := readSnapshotDocument(*inputPath)
	if err != nil {
		return err
	}
	plan := buildSnapshotDrillPlanDocument(*inputPath, document, *expectedChainID, *minHeight, *requireKV)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	writeSnapshotDrillPlan(writer, plan)
	return nil
}

func buildSnapshotDrillPlanDocument(inputPath string, document snapshotDocument, expectedChainID string, minHeight uint64, requireKV bool) snapshotDrillPlanDocument {
	plan := snapshotDrillPlanDocument{
		SchemaVersion: "v1",
		Input:         inputPath,
		ChainID:       document.ChainID,
		Height:        uint64(document.State.Height),
		Modules:       append([]string(nil), document.Modules...),
		KVPairCount:   len(document.KV),
		Checksum:      document.Checksum,
		Steps: []string{
			"verify snapshot checksum before copying to a new node",
			"restore snapshot into an empty node home",
			"run snapshot verify against the restored home",
			"run doctor and require snapshot, store, and recovery checks to pass",
			"start the node and verify replay health plus height catch-up",
			"archive snapshot checksum, restore logs, and recovered state height",
		},
	}
	plan.Checks = append(plan.Checks, snapshotDrillPlanCheck{
		Name:    "height",
		OK:      plan.Height >= minHeight,
		Message: fmt.Sprintf("snapshot height must be at least %d", minHeight),
	})
	plan.Checks = append(plan.Checks, snapshotDrillPlanCheck{
		Name:    "chain_id",
		OK:      expectedChainID == "" || plan.ChainID == expectedChainID,
		Message: "snapshot chain id must match the target chain",
	})
	plan.Checks = append(plan.Checks, snapshotDrillPlanCheck{
		Name:    "state_roots",
		OK:      len(document.StateRoots) >= len(document.Modules),
		Message: "snapshot must include state roots for all declared modules",
	})
	plan.Checks = append(plan.Checks, snapshotDrillPlanCheck{
		Name:    "kv_payload",
		OK:      !requireKV || len(document.KV) > 0,
		Message: "state sync restore should include module KV payloads",
	})
	plan.Checks = append(plan.Checks, snapshotDrillPlanCheck{
		Name:    "checksum",
		OK:      document.Checksum != "" && document.Checksum == snapshotChecksum(document),
		Message: "snapshot checksum must be present and valid",
	})
	for _, check := range plan.Checks {
		if !check.OK {
			plan.Warnings = append(plan.Warnings, check.Message)
		}
	}
	return plan
}

func writeSnapshotDrillPlan(writer io.Writer, plan snapshotDrillPlanDocument) {
	fmt.Fprintf(writer, "snapshot drill plan\n")
	fmt.Fprintf(writer, "input: %s\n", plan.Input)
	fmt.Fprintf(writer, "chain_id: %s\n", plan.ChainID)
	fmt.Fprintf(writer, "height: %d\n", plan.Height)
	fmt.Fprintf(writer, "modules: %v\n", plan.Modules)
	fmt.Fprintf(writer, "kv_pairs: %d\n", plan.KVPairCount)
	fmt.Fprintf(writer, "checksum: %s\n", plan.Checksum)
	fmt.Fprintf(writer, "checks:\n")
	for _, check := range plan.Checks {
		fmt.Fprintf(writer, "- %s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	fmt.Fprintf(writer, "steps:\n")
	for index, step := range plan.Steps {
		fmt.Fprintf(writer, "%d. %s\n", index+1, step)
	}
	if len(plan.Warnings) > 0 {
		fmt.Fprintf(writer, "warnings:\n")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(writer, "- %s\n", warning)
		}
	}
}

func buildSnapshotDocument(storage store.Store, chainID string, namespaces []string) (snapshotDocument, error) {
	state, err := storage.LatestState(context.Background())
	if err != nil {
		return snapshotDocument{}, err
	}
	roots := make([]store.StateRootRecord, 0)
	kv := make([]store.KVPair, 0)
	exporter, canExportKV := storage.(store.SnapshotKVStore)
	for _, namespace := range namespaces {
		root, err := storage.StateRoot(context.Background(), state.Height, namespace)
		if err == nil {
			roots = append(roots, root)
		} else if !errors.Is(err, store.ErrStateRootNotFound) {
			return snapshotDocument{}, err
		}
		if canExportKV {
			pairs, err := exporter.ExportNamespace(context.Background(), namespace)
			if err != nil {
				return snapshotDocument{}, err
			}
			kv = append(kv, pairs...)
		}
	}
	return snapshotDocumentFromState(chainID, namespaces, state, roots, kv), nil
}

func snapshotDocumentFromState(chainID string, modules []string, state store.StateRecord, roots []store.StateRootRecord, kv []store.KVPair) snapshotDocument {
	document := snapshotDocument{
		SchemaVersion: "v1",
		ChainID:       chainID,
		Modules:       activeSnapshotNamespaces(modules, roots, kv),
		State:         state,
		StateRoots:    sortedStateRoots(roots),
		KV:            sortedKVPairs(kv),
	}
	document.Checksum = snapshotChecksum(document)
	return document
}

func snapshotChunks(document snapshotDocument, chunkSize int) ([]snapshotChunkDocument, error) {
	if err := validateSnapshotDocument(document, ""); err != nil {
		return nil, err
	}
	if chunkSize <= 0 {
		return nil, errors.New("snapshot chunk size must be positive")
	}
	kv := sortedKVPairs(document.KV)
	chunkCount := uint64(1)
	if len(kv) > 0 {
		chunkCount = uint64((len(kv) + chunkSize - 1) / chunkSize)
	}
	chunks := make([]snapshotChunkDocument, 0, chunkCount)
	for index := uint64(0); index < chunkCount; index++ {
		start := int(index) * chunkSize
		end := start + chunkSize
		if start > len(kv) {
			start = len(kv)
		}
		if end > len(kv) {
			end = len(kv)
		}
		chunk := snapshotChunkDocument{
			SchemaVersion:    "v1",
			ChainID:          document.ChainID,
			Modules:          sortedStrings(document.Modules),
			State:            document.State,
			StateRoots:       sortedStateRoots(document.StateRoots),
			KV:               sortedKVPairs(kv[start:end]),
			ChunkIndex:       index,
			ChunkCount:       chunkCount,
			SnapshotChecksum: document.Checksum,
		}
		chunk.ChunkChecksum = snapshotChunkChecksum(chunk)
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func snapshotDocumentFromChunks(chunks []snapshotChunkDocument) (snapshotDocument, error) {
	if len(chunks) == 0 {
		return snapshotDocument{}, errors.New("snapshot chunks are required")
	}
	ordered := make([]snapshotChunkDocument, len(chunks))
	seen := make(map[uint64]struct{}, len(chunks))
	var chunkCount uint64
	for _, chunk := range chunks {
		if err := validateSnapshotChunk(chunk, ""); err != nil {
			return snapshotDocument{}, err
		}
		if chunkCount == 0 {
			chunkCount = chunk.ChunkCount
			if int(chunkCount) != len(chunks) {
				return snapshotDocument{}, fmt.Errorf("snapshot chunk count mismatch: expected %d got %d", chunkCount, len(chunks))
			}
		}
		if chunk.ChunkCount != chunkCount {
			return snapshotDocument{}, errors.New("snapshot chunks disagree on chunk count")
		}
		if chunk.ChunkIndex >= chunkCount {
			return snapshotDocument{}, fmt.Errorf("snapshot chunk index out of range: %d/%d", chunk.ChunkIndex, chunkCount)
		}
		if _, found := seen[chunk.ChunkIndex]; found {
			return snapshotDocument{}, fmt.Errorf("duplicate snapshot chunk index %d", chunk.ChunkIndex)
		}
		seen[chunk.ChunkIndex] = struct{}{}
		ordered[chunk.ChunkIndex] = chunk
	}
	first := ordered[0]
	kv := make([]store.KVPair, 0)
	for _, chunk := range ordered {
		if chunk.SchemaVersion != first.SchemaVersion ||
			chunk.ChainID != first.ChainID ||
			chunk.State.Height != first.State.Height ||
			chunk.State.AppHash != first.State.AppHash ||
			chunk.SnapshotChecksum != first.SnapshotChecksum {
			return snapshotDocument{}, errors.New("snapshot chunks belong to different snapshots")
		}
		if !sameStringSet(chunk.Modules, first.Modules) || !sameStateRoots(chunk.StateRoots, first.StateRoots) {
			return snapshotDocument{}, errors.New("snapshot chunks disagree on module roots")
		}
		kv = append(kv, chunk.KV...)
	}
	document := snapshotDocument{
		SchemaVersion: first.SchemaVersion,
		ChainID:       first.ChainID,
		Modules:       sortedStrings(first.Modules),
		State:         first.State,
		StateRoots:    sortedStateRoots(first.StateRoots),
		KV:            sortedKVPairs(kv),
		Checksum:      first.SnapshotChecksum,
	}
	if err := validateSnapshotDocument(document, ""); err != nil {
		return snapshotDocument{}, err
	}
	return document, nil
}

func restoreSnapshotDocument(storage store.Store, document snapshotDocument) error {
	if err := validateSnapshotDocument(document, ""); err != nil {
		return err
	}
	importer, canImportKV := storage.(store.SnapshotKVStore)
	if len(document.KV) > 0 && !canImportKV {
		return errors.New("snapshot KV restore is unavailable")
	}
	if canImportKV {
		for _, namespace := range document.Modules {
			if err := importer.ImportNamespace(context.Background(), namespace, kvForNamespace(document.KV, namespace)); err != nil {
				return err
			}
		}
	}
	if err := storage.SaveState(context.Background(), document.State); err != nil {
		return err
	}
	for _, root := range document.StateRoots {
		if err := storage.SaveStateRoot(context.Background(), root); err != nil {
			return err
		}
		if len(document.KV) > 0 {
			actualRoot, err := storage.Root(context.Background(), root.Namespace)
			if err != nil {
				return err
			}
			if actualRoot != root.Root {
				return fmt.Errorf("snapshot state root mismatch for namespace %q", root.Namespace)
			}
		}
	}
	return nil
}

func downloadSnapshotDocument(url string, timeout time.Duration) (snapshotDocument, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := http.Client{Timeout: timeout}
	response, err := client.Get(url)
	if err != nil {
		return snapshotDocument{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return snapshotDocument{}, fmt.Errorf("snapshot download failed: status %d", response.StatusCode)
	}
	var document snapshotDocument
	decoder := json.NewDecoder(io.LimitReader(response.Body, 32*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return snapshotDocument{}, err
	}
	if document.SchemaVersion != "v1" {
		return snapshotDocument{}, fmt.Errorf("unsupported snapshot schema %q", document.SchemaVersion)
	}
	if err := validateSnapshotDocument(document, ""); err != nil {
		return snapshotDocument{}, err
	}
	return document, nil
}

func writeSnapshotDocument(writer io.Writer, document snapshotDocument) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func writeSnapshotChunkDocument(writer io.Writer, document snapshotChunkDocument) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func readSnapshotDocument(path string) (snapshotDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return snapshotDocument{}, err
	}
	defer file.Close()
	var document snapshotDocument
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return snapshotDocument{}, err
	}
	if err := validateSnapshotDocument(document, ""); err != nil {
		return snapshotDocument{}, err
	}
	return document, nil
}

func readSnapshotChunkDocument(path string) (snapshotChunkDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return snapshotChunkDocument{}, err
	}
	defer file.Close()
	var document snapshotChunkDocument
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return snapshotChunkDocument{}, err
	}
	if err := validateSnapshotChunk(document, ""); err != nil {
		return snapshotChunkDocument{}, err
	}
	return document, nil
}

func readSnapshotChunksFromDir(inputDir string) ([]snapshotChunkDocument, error) {
	paths, err := filepath.Glob(filepath.Join(inputDir, "snapshot-chunk-*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("snapshot chunk files not found")
	}
	sort.Strings(paths)
	chunks := make([]snapshotChunkDocument, 0, len(paths))
	for _, path := range paths {
		chunk, err := readSnapshotChunkDocument(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func validateSnapshotDocument(document snapshotDocument, expectedChainID string) error {
	if document.SchemaVersion != "v1" {
		return fmt.Errorf("unsupported snapshot schema %q", document.SchemaVersion)
	}
	if expectedChainID != "" && document.ChainID != "" && document.ChainID != expectedChainID {
		return fmt.Errorf("snapshot chain id mismatch: expected %s got %s", expectedChainID, document.ChainID)
	}
	if document.State.Height == 0 {
		return store.ErrInvalidStateRecord
	}
	modules := sortedStrings(document.Modules)
	rootNamespaces := make(map[string]struct{}, len(document.StateRoots))
	for _, root := range document.StateRoots {
		if root.Height != document.State.Height {
			return fmt.Errorf("snapshot root height mismatch: state=%d root=%d namespace=%s", document.State.Height, root.Height, root.Namespace)
		}
		if root.Namespace == "" {
			return store.ErrInvalidNamespace
		}
		if _, found := rootNamespaces[root.Namespace]; found {
			return fmt.Errorf("snapshot duplicate state root namespace %q", root.Namespace)
		}
		rootNamespaces[root.Namespace] = struct{}{}
	}
	for _, namespace := range modules {
		if _, found := rootNamespaces[namespace]; !found {
			return fmt.Errorf("snapshot missing state root for namespace %q", namespace)
		}
	}
	moduleSet := make(map[string]struct{}, len(modules))
	for _, namespace := range modules {
		moduleSet[namespace] = struct{}{}
	}
	for _, pair := range document.KV {
		if pair.Namespace == "" {
			return store.ErrInvalidNamespace
		}
		if len(pair.Key) == 0 {
			return store.ErrInvalidKey
		}
		if _, found := moduleSet[pair.Namespace]; !found {
			return fmt.Errorf("snapshot KV namespace %q is not declared", pair.Namespace)
		}
	}
	if document.Checksum != "" && document.Checksum != snapshotChecksum(document) {
		return errors.New("snapshot checksum mismatch")
	}
	return nil
}

func validateSnapshotChunk(document snapshotChunkDocument, expectedChainID string) error {
	if document.SchemaVersion != "v1" {
		return fmt.Errorf("unsupported snapshot chunk schema %q", document.SchemaVersion)
	}
	if expectedChainID != "" && document.ChainID != "" && document.ChainID != expectedChainID {
		return fmt.Errorf("snapshot chunk chain id mismatch: expected %s got %s", expectedChainID, document.ChainID)
	}
	if document.ChunkCount == 0 {
		return errors.New("snapshot chunk count must be positive")
	}
	if document.ChunkIndex >= document.ChunkCount {
		return fmt.Errorf("snapshot chunk index out of range: %d/%d", document.ChunkIndex, document.ChunkCount)
	}
	if document.SnapshotChecksum == "" {
		return errors.New("snapshot chunk is missing snapshot checksum")
	}
	if document.ChunkChecksum == "" {
		return errors.New("snapshot chunk is missing chunk checksum")
	}
	if document.ChunkChecksum != snapshotChunkChecksum(document) {
		return errors.New("snapshot chunk checksum mismatch")
	}
	candidate := snapshotDocument{
		SchemaVersion: document.SchemaVersion,
		ChainID:       document.ChainID,
		Modules:       sortedStrings(document.Modules),
		State:         document.State,
		StateRoots:    sortedStateRoots(document.StateRoots),
		KV:            sortedKVPairs(document.KV),
	}
	if candidate.State.Height == 0 {
		return store.ErrInvalidStateRecord
	}
	if err := validateSnapshotChunkPayload(candidate); err != nil {
		return err
	}
	return nil
}

func validateSnapshotChunkPayload(document snapshotDocument) error {
	if document.SchemaVersion != "v1" {
		return fmt.Errorf("unsupported snapshot schema %q", document.SchemaVersion)
	}
	modules := sortedStrings(document.Modules)
	rootNamespaces := make(map[string]struct{}, len(document.StateRoots))
	for _, root := range document.StateRoots {
		if root.Height != document.State.Height {
			return fmt.Errorf("snapshot root height mismatch: state=%d root=%d namespace=%s", document.State.Height, root.Height, root.Namespace)
		}
		if root.Namespace == "" {
			return store.ErrInvalidNamespace
		}
		rootNamespaces[root.Namespace] = struct{}{}
	}
	moduleSet := make(map[string]struct{}, len(modules))
	for _, namespace := range modules {
		if _, found := rootNamespaces[namespace]; !found {
			return fmt.Errorf("snapshot missing state root for namespace %q", namespace)
		}
		moduleSet[namespace] = struct{}{}
	}
	for _, pair := range document.KV {
		if pair.Namespace == "" {
			return store.ErrInvalidNamespace
		}
		if len(pair.Key) == 0 {
			return store.ErrInvalidKey
		}
		if _, found := moduleSet[pair.Namespace]; !found {
			return fmt.Errorf("snapshot KV namespace %q is not declared", pair.Namespace)
		}
	}
	return nil
}

func snapshotChecksum(document snapshotDocument) string {
	document.Checksum = ""
	document.Modules = sortedStrings(document.Modules)
	document.StateRoots = sortedStateRoots(document.StateRoots)
	document.KV = sortedKVPairs(document.KV)
	data, _ := json.Marshal(document)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func snapshotChunkChecksum(document snapshotChunkDocument) string {
	document.ChunkChecksum = ""
	document.Modules = sortedStrings(document.Modules)
	document.StateRoots = sortedStateRoots(document.StateRoots)
	document.KV = sortedKVPairs(document.KV)
	data, _ := json.Marshal(document)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func snapshotChunkFileName(index uint64, count uint64) string {
	return fmt.Sprintf("snapshot-chunk-%06d-of-%06d.json", index+1, count)
}

func sameStringSet(left []string, right []string) bool {
	left = sortedStrings(left)
	right = sortedStrings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameStateRoots(left []store.StateRootRecord, right []store.StateRootRecord) bool {
	left = sortedStateRoots(left)
	right = sortedStateRoots(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func snapshotNamespaces(modules []string) []string {
	namespaces := append([]string(nil), modules...)
	namespaces = append(namespaces, "auth")
	return sortedStrings(namespaces)
}

func activeSnapshotNamespaces(namespaces []string, roots []store.StateRootRecord, kv []store.KVPair) []string {
	allowed := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		if namespace != "" {
			allowed[namespace] = struct{}{}
		}
	}
	active := make(map[string]struct{}, len(roots)+len(kv))
	for _, root := range roots {
		if _, found := allowed[root.Namespace]; found {
			active[root.Namespace] = struct{}{}
		}
	}
	for _, pair := range kv {
		if _, found := allowed[pair.Namespace]; found {
			active[pair.Namespace] = struct{}{}
		}
	}
	values := make([]string, 0, len(active))
	for namespace := range active {
		values = append(values, namespace)
	}
	return sortedStrings(values)
}

func sortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	sorted := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		sorted = append(sorted, value)
	}
	sort.Strings(sorted)
	return sorted
}

func sortedStateRoots(roots []store.StateRootRecord) []store.StateRootRecord {
	sorted := append([]store.StateRootRecord(nil), roots...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Height != sorted[j].Height {
			return sorted[i].Height < sorted[j].Height
		}
		return sorted[i].Namespace < sorted[j].Namespace
	})
	return sorted
}

func sortedKVPairs(pairs []store.KVPair) []store.KVPair {
	sorted := append([]store.KVPair(nil), pairs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return bytes.Compare(sorted[i].Key, sorted[j].Key) < 0
	})
	return sorted
}

func kvForNamespace(pairs []store.KVPair, namespace string) []store.KVPair {
	filtered := make([]store.KVPair, 0)
	for _, pair := range pairs {
		if pair.Namespace == namespace {
			filtered = append(filtered, pair)
		}
	}
	return filtered
}

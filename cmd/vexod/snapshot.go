package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexoconfig "github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	appmodules "github.com/vexo-network/vexo-consensus/modules"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// Snapshots.
//
// A snapshot is the replicated state a committed block header commits the app hash of
// (finality.AppHashHeight), anchored to that block. It is a manifest (the anchor header, the
// headers linking it back to the block of the state, that block, the state record and the root of
// every namespace) followed by the pairs of every namespace in key order, page by page. The RPC
// snapshot endpoints serve that stream one request per page, and a snapshot file holds it as one
// JSON value after another, so neither exporting nor restoring a snapshot holds the state in memory.
//
// Whoever serves a snapshot is not trusted. It is restored only when it proves to be the state of a
// block the operator trusts (checkSnapshotManifest, verifySnapshotTrust), and every imported
// namespace has the root the manifest names; it is imported into a staging store that replaces the
// node's store only once complete, so a refused or interrupted restore leaves the store as it was.

type snapshotManifest = vexorpc.SnapshotManifestResponse

type snapshotPage = vexorpc.SnapshotPairsResponse

// snapshotPageSize is the number of pairs a snapshot page holds when this node writes or asks for one.
const snapshotPageSize = 1_000

// snapshotSource yields a snapshot: its manifest first, then the pages of every namespace of the
// manifest in order, each after the last key of the page before.
type snapshotSource interface {
	Manifest(ctx context.Context) (snapshotManifest, error)
	Page(ctx context.Context, namespace string, after []byte) (snapshotPage, error)
}

type snapshotDrillPlanDocument struct {
	SchemaVersion string                   `json:"schema_version"`
	Input         string                   `json:"input"`
	ChainID       string                   `json:"chain_id"`
	Height        uint64                   `json:"height"`
	AnchorHeight  uint64                   `json:"anchor_height"`
	Modules       []string                 `json:"modules"`
	KVPairCount   uint64                   `json:"kv_pair_count"`
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
	case "verify":
		return runSnapshotVerify(writer, args[1:])
	case "restore":
		return runSnapshotRestore(writer, args[1:])
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
	blockHeight := flags.Uint64("height", 0, "committed block the snapshot is anchored to; the latest when 0")
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
	source, err := newStoreSnapshotSource(storage, cfg.Chain, types.Height(*blockHeight))
	if err != nil {
		return err
	}
	if *outputPath == "" {
		_, _, err := writeSnapshotStream(context.Background(), writer, source)
		return err
	}
	manifest, pairs, err := writeSnapshotFile(context.Background(), *outputPath, source)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot exported\n")
	fmt.Fprintf(writer, "path: %s\n", *outputPath)
	writeSnapshotSummary(writer, manifest, pairs)
	return nil
}

func runSnapshotVerify(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	inputPath := flags.String("input", "", "snapshot input path")
	trustHeight, trustHash := snapshotTrustFlags(flags)
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
	// Without a trusted block the check is of the snapshot's own consistency: its manifest, and
	// the roots of its pairs.
	trusted := *trustHeight != 0 || *trustHash != ""
	var trust *snapshotTrust
	if trusted {
		parsed, err := parseSnapshotTrust(*trustHeight, *trustHash)
		if err != nil {
			return err
		}
		trust = &parsed
	}
	source, closeSource, err := openSnapshotFile(*inputPath)
	if err != nil {
		return err
	}
	defer closeSource()
	ctx := context.Background()
	manifest, err := source.Manifest(ctx)
	if err != nil {
		return err
	}
	if trust != nil {
		if err := verifySnapshotTrust(manifest, *trust); err != nil {
			return err
		}
	}
	if err := checkSnapshotManifest(ctx, manifest, cfg.Chain); err != nil {
		return err
	}
	scratch, err := os.MkdirTemp("", "vexo-snapshot-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	pairs, err := importSnapshot(ctx, filepath.Join(scratch, "store"), source, manifest)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot verified\n")
	fmt.Fprintf(writer, "trusted: %t\n", trusted)
	fmt.Fprintf(writer, "path: %s\n", *inputPath)
	fmt.Fprintf(writer, "chain_id: %s\n", manifest.ChainID)
	writeSnapshotSummary(writer, manifest, pairs)
	return nil
}

func runSnapshotRestore(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot restore", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	inputPath := flags.String("input", "", "snapshot input path")
	trustHeight, trustHash := snapshotTrustFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("snapshot input path is required")
	}
	trust, err := parseSnapshotTrust(*trustHeight, *trustHash)
	if err != nil {
		return err
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	source, closeSource, err := openSnapshotFile(*inputPath)
	if err != nil {
		return err
	}
	defer closeSource()
	manifest, pairs, err := restoreSnapshot(context.Background(), cfg.StoreDir(), source, cfg.Chain, trust, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot restored\n")
	fmt.Fprintf(writer, "path: %s\n", *inputPath)
	writeSnapshotSummary(writer, manifest, pairs)
	return nil
}

func runSnapshotFetch(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot fetch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rpcURL := flags.String("url", "", "RPC URL of the snapshot source")
	outputPath := flags.String("output", "", "snapshot output path")
	blockHeight := flags.Uint64("height", 0, "committed block the snapshot is anchored to; the latest when 0")
	timeout := flags.Duration("timeout", 10*time.Second, "timeout of each snapshot request")
	maxBytes := flags.Int64("max-bytes", defaultStateSyncSnapshotMaxBytes, "most bytes the whole snapshot may take")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *rpcURL == "" {
		return errors.New("snapshot URL is required")
	}
	if *outputPath == "" {
		return errors.New("snapshot output path is required")
	}
	source, err := newHTTPSnapshotSource(*rpcURL, types.Height(*blockHeight), *timeout, *maxBytes)
	if err != nil {
		return err
	}
	manifest, pairs, err := writeSnapshotFile(context.Background(), *outputPath, source)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot fetched\n")
	fmt.Fprintf(writer, "url: %s\n", *rpcURL)
	fmt.Fprintf(writer, "path: %s\n", *outputPath)
	writeSnapshotSummary(writer, manifest, pairs)
	return nil
}

func runSnapshotSync(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot sync", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	rpcURL := flags.String("url", "", "RPC URL of the snapshot source")
	timeout := flags.Duration("timeout", 10*time.Second, "timeout of each snapshot request")
	maxBytes := flags.Int64("max-bytes", defaultStateSyncSnapshotMaxBytes, "most bytes the whole snapshot may take")
	trustHeight, trustHash := snapshotTrustFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *rpcURL == "" {
		return errors.New("snapshot URL is required")
	}
	trust, err := parseSnapshotTrust(*trustHeight, *trustHash)
	if err != nil {
		return err
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	source, err := newHTTPSnapshotSource(*rpcURL, trust.Height, *timeout, *maxBytes)
	if err != nil {
		return err
	}
	manifest, pairs, err := restoreSnapshot(context.Background(), cfg.StoreDir(), source, cfg.Chain, trust, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot synced\n")
	fmt.Fprintf(writer, "url: %s\n", *rpcURL)
	writeSnapshotSummary(writer, manifest, pairs)
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
	source, closeSource, err := openSnapshotFile(*inputPath)
	if err != nil {
		return err
	}
	defer closeSource()
	ctx := context.Background()
	manifest, err := source.Manifest(ctx)
	if err != nil {
		return err
	}
	var pairs uint64
	pagesErr := readSnapshotPages(ctx, source, manifest, func(page snapshotPage) error {
		pairs += uint64(len(page.Pairs))
		return nil
	})
	plan := buildSnapshotDrillPlanDocument(*inputPath, manifest, pairs, pagesErr, *expectedChainID, *minHeight, *requireKV)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	writeSnapshotDrillPlan(writer, plan)
	return nil
}

func buildSnapshotDrillPlanDocument(inputPath string, manifest snapshotManifest, pairs uint64, pagesErr error, expectedChainID string, minHeight uint64, requireKV bool) snapshotDrillPlanDocument {
	plan := snapshotDrillPlanDocument{
		SchemaVersion: "v2",
		Input:         inputPath,
		ChainID:       manifest.ChainID,
		Height:        uint64(manifest.State.Height),
		AnchorHeight:  uint64(manifest.Header.Height),
		Modules:       append([]string(nil), manifest.Modules...),
		KVPairCount:   pairs,
		Steps: []string{
			"run snapshot verify with the trusted block before copying to a new node",
			"restore snapshot into an empty node home",
			"run doctor and require snapshot, store, and recovery checks to pass",
			"start the node and verify replay health plus height catch-up",
			"archive the trusted block, restore logs, and recovered state height",
		},
	}
	pagesMessage := "snapshot pages must cover every namespace in key order"
	if pagesErr != nil {
		pagesMessage += ": " + pagesErr.Error()
	}
	plan.Checks = []snapshotDrillPlanCheck{
		{Name: "height", OK: plan.Height >= minHeight, Message: fmt.Sprintf("snapshot height must be at least %d", minHeight)},
		{Name: "chain_id", OK: expectedChainID == "" || plan.ChainID == expectedChainID, Message: "snapshot chain id must match the target chain"},
		{Name: "state_roots", OK: snapshotRootsCover(manifest), Message: "snapshot must include one state root for every declared module"},
		{Name: "kv_payload", OK: !requireKV || pairs > 0, Message: "state sync restore should include module KV payloads"},
		{Name: "pages", OK: pagesErr == nil, Message: pagesMessage},
	}
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
	fmt.Fprintf(writer, "anchor_height: %d\n", plan.AnchorHeight)
	fmt.Fprintf(writer, "modules: %v\n", plan.Modules)
	fmt.Fprintf(writer, "kv_pairs: %d\n", plan.KVPairCount)
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

func writeSnapshotSummary(writer io.Writer, manifest snapshotManifest, pairs uint64) {
	fmt.Fprintf(writer, "height: %d\n", manifest.State.Height)
	fmt.Fprintf(writer, "anchor_height: %d\n", manifest.Header.Height)
	fmt.Fprintf(writer, "modules: %v\n", manifest.Modules)
	fmt.Fprintf(writer, "kv_pairs: %d\n", pairs)
}

// storeSnapshotSource reads the snapshot anchored to a committed block of a local store.
type storeSnapshotSource struct {
	source vexonode.SnapshotSource
	height types.Height
}

func newStoreSnapshotSource(storage store.Store, chain vexoconfig.Config, blockHeight types.Height) (*storeSnapshotSource, error) {
	namespaces, err := replicatedNamespaces(chain)
	if err != nil {
		return nil, err
	}
	return &storeSnapshotSource{source: vexonode.SnapshotSource{Store: storage, ChainID: chain.ChainID, Namespaces: namespaces}, height: blockHeight}, nil
}

func (source *storeSnapshotSource) Manifest(ctx context.Context) (snapshotManifest, error) {
	manifest, err := source.source.Manifest(ctx, source.height)
	if err != nil {
		return snapshotManifest{}, err
	}
	source.height = manifest.Header.Height
	return vexorpc.NewSnapshotManifestResponse(manifest), nil
}

func (source *storeSnapshotSource) Page(ctx context.Context, namespace string, after []byte) (snapshotPage, error) {
	pairs, more, err := source.source.Pairs(ctx, source.height, namespace, after, snapshotPageSize)
	if err != nil {
		return snapshotPage{}, err
	}
	return snapshotPage{SchemaVersion: vexorpc.SnapshotSchemaVersion, Height: uint64(source.height), Namespace: namespace, Pairs: pairs, More: more}, nil
}

// errSnapshotSourcePruned reports a source that no longer retains the state of the block asked for.
var errSnapshotSourcePruned = errors.New("the snapshot source has pruned the state of the block")

// httpSnapshotSource reads a snapshot from the RPC snapshot endpoints of a node, within a budget
// of bytes for the whole snapshot.
type httpSnapshotSource struct {
	base      *url.URL
	height    types.Height
	client    http.Client
	maxBytes  int64
	remaining int64
}

func newHTTPSnapshotSource(rawURL string, blockHeight types.Height, timeout time.Duration, maxBytes int64) (*httpSnapshotSource, error) {
	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("snapshot URL %q: %w", rawURL, err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("snapshot URL %q must be an http or https RPC URL", rawURL)
	}
	if timeout <= 0 {
		timeout = defaultStateSyncTimeout
	}
	if maxBytes <= 0 {
		maxBytes = defaultStateSyncSnapshotMaxBytes
	}
	return &httpSnapshotSource{base: base, height: blockHeight, client: http.Client{Timeout: timeout}, maxBytes: maxBytes, remaining: maxBytes}, nil
}

func (source *httpSnapshotSource) Manifest(ctx context.Context) (snapshotManifest, error) {
	query := url.Values{}
	query.Set("height", strconv.FormatUint(uint64(source.height), 10))
	var manifest snapshotManifest
	if err := source.get(ctx, "/snapshot/manifest", query, &manifest); err != nil {
		return snapshotManifest{}, err
	}
	if manifest.SchemaVersion != vexorpc.SnapshotSchemaVersion {
		return snapshotManifest{}, fmt.Errorf("unsupported snapshot schema %q", manifest.SchemaVersion)
	}
	if source.height != 0 && manifest.Header.Height != source.height {
		return snapshotManifest{}, fmt.Errorf("%w: asked for the snapshot of block %d, got block %d", ErrUntrustedSnapshot, source.height, manifest.Header.Height)
	}
	source.height = manifest.Header.Height
	return manifest, nil
}

func (source *httpSnapshotSource) Page(ctx context.Context, namespace string, after []byte) (snapshotPage, error) {
	query := url.Values{}
	query.Set("height", strconv.FormatUint(uint64(source.height), 10))
	query.Set("namespace", namespace)
	query.Set("limit", strconv.Itoa(snapshotPageSize))
	if after != nil {
		query.Set("after", hex.EncodeToString(after))
	}
	var page snapshotPage
	if err := source.get(ctx, "/snapshot/pairs", query, &page); err != nil {
		return snapshotPage{}, err
	}
	if len(page.Pairs) > snapshotPageSize {
		return snapshotPage{}, fmt.Errorf("%w: a page of %d pairs, %d were asked for", ErrUntrustedSnapshot, len(page.Pairs), snapshotPageSize)
	}
	return page, nil
}

func (source *httpSnapshotSource) get(ctx context.Context, endpoint string, query url.Values, into any) error {
	target := *source.base
	target.Path = strings.TrimSuffix(target.Path, "/") + endpoint
	target.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	response, err := source.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, source.remaining+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > source.remaining {
		return fmt.Errorf("snapshot download exceeds max bytes %d", source.maxBytes)
	}
	source.remaining -= int64(len(data))
	if response.StatusCode != http.StatusOK {
		var refusal vexorpc.SnapshotErrorResponse
		_ = json.Unmarshal(data, &refusal)
		if response.StatusCode == http.StatusGone {
			return fmt.Errorf("%w %d: %s; it serves snapshots from block %d on", errSnapshotSourcePruned, source.height, refusal.Error, refusal.EarliestHeight)
		}
		return fmt.Errorf("snapshot download failed: status %d: %s", response.StatusCode, refusal.Error)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into)
}

// fileSnapshotSource reads a snapshot file: the manifest, then the pages of every namespace in order.
type fileSnapshotSource struct {
	decoder *json.Decoder
	height  types.Height
}

func openSnapshotFile(path string) (*fileSnapshotSource, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	decoder := json.NewDecoder(bufio.NewReader(file))
	decoder.DisallowUnknownFields()
	return &fileSnapshotSource{decoder: decoder}, func() { file.Close() }, nil
}

func (source *fileSnapshotSource) Manifest(context.Context) (snapshotManifest, error) {
	var manifest snapshotManifest
	if err := source.decoder.Decode(&manifest); err != nil {
		return snapshotManifest{}, fmt.Errorf("snapshot manifest: %w", err)
	}
	if manifest.SchemaVersion != vexorpc.SnapshotSchemaVersion {
		return snapshotManifest{}, fmt.Errorf("unsupported snapshot schema %q", manifest.SchemaVersion)
	}
	source.height = manifest.Header.Height
	return manifest, nil
}

func (source *fileSnapshotSource) Page(_ context.Context, namespace string, _ []byte) (snapshotPage, error) {
	var page snapshotPage
	if err := source.decoder.Decode(&page); err != nil {
		return snapshotPage{}, fmt.Errorf("snapshot page of namespace %q: %w", namespace, err)
	}
	return page, nil
}

// readSnapshotPages reads the pages of every namespace of manifest from source in order, handing
// each to visit. Each page must be of the namespace and anchor asked for, and its keys
// must ascend strictly after those of the pages before.
func readSnapshotPages(ctx context.Context, source snapshotSource, manifest snapshotManifest, visit func(page snapshotPage) error) error {
	for _, namespace := range manifest.Modules {
		var after []byte
		for {
			page, err := source.Page(ctx, namespace, after)
			if err != nil {
				return err
			}
			if page.SchemaVersion != vexorpc.SnapshotSchemaVersion || page.Namespace != namespace || page.Height != uint64(manifest.Header.Height) {
				return fmt.Errorf("%w: a %s page of namespace %q of block %d where namespace %q of block %d was asked for", ErrUntrustedSnapshot, page.SchemaVersion, page.Namespace, page.Height, namespace, manifest.Header.Height)
			}
			if page.More && len(page.Pairs) == 0 {
				return fmt.Errorf("%w: an empty page of namespace %q announces more pairs", ErrUntrustedSnapshot, namespace)
			}
			for _, pair := range page.Pairs {
				if pair.Namespace != namespace || len(pair.Key) == 0 || (after != nil && bytes.Compare(pair.Key, after) <= 0) {
					return fmt.Errorf("%w: pair %x of namespace %q is out of the namespace's key order", ErrUntrustedSnapshot, pair.Key, pair.Namespace)
				}
				after = pair.Key
			}
			if err := visit(page); err != nil {
				return err
			}
			if !page.More {
				break
			}
		}
	}
	return nil
}

// writeSnapshotStream writes the snapshot of source to writer as a snapshot file.
func writeSnapshotStream(ctx context.Context, writer io.Writer, source snapshotSource) (snapshotManifest, uint64, error) {
	buffered := bufio.NewWriter(writer)
	encoder := json.NewEncoder(buffered)
	manifest, err := source.Manifest(ctx)
	if err != nil {
		return snapshotManifest{}, 0, err
	}
	if err := encoder.Encode(manifest); err != nil {
		return snapshotManifest{}, 0, err
	}
	var pairs uint64
	err = readSnapshotPages(ctx, source, manifest, func(page snapshotPage) error {
		pairs += uint64(len(page.Pairs))
		return encoder.Encode(page)
	})
	if err != nil {
		return snapshotManifest{}, 0, err
	}
	return manifest, pairs, buffered.Flush()
}

// writeSnapshotFile writes the snapshot of source to the file at path, which it replaces only once
// the snapshot is complete.
func writeSnapshotFile(ctx context.Context, path string, source snapshotSource) (snapshotManifest, uint64, error) {
	partial := path + ".partial"
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return snapshotManifest{}, 0, err
	}
	manifest, pairs, err := writeSnapshotStream(ctx, file, source)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(partial, path)
	}
	if err != nil {
		os.Remove(partial)
		return snapshotManifest{}, 0, err
	}
	return manifest, pairs, nil
}

// restoreSnapshot restores the snapshot of source as the store at storeDir once its manifest proves
// to be the state of the trusted block and accept, when set, takes it. The snapshot is imported into
// a staging store that replaces the store at storeDir only once every namespace has its root, so the
// store is left as it was when the snapshot is refused or the restore fails.
func restoreSnapshot(ctx context.Context, storeDir string, source snapshotSource, chain vexoconfig.Config, trust snapshotTrust, accept func(snapshotManifest) error) (snapshotManifest, uint64, error) {
	manifest, err := source.Manifest(ctx)
	if err != nil {
		return snapshotManifest{}, 0, err
	}
	if err := verifySnapshotTrust(manifest, trust); err != nil {
		return snapshotManifest{}, 0, err
	}
	if err := checkSnapshotManifest(ctx, manifest, chain); err != nil {
		return snapshotManifest{}, 0, err
	}
	if accept != nil {
		if err := accept(manifest); err != nil {
			return snapshotManifest{}, 0, err
		}
	}
	storeDir = filepath.Clean(storeDir)
	staging, replaced := storeDir+".restore", storeDir+".replaced"
	for _, leftover := range []string{staging, replaced} {
		if err := os.RemoveAll(leftover); err != nil {
			return snapshotManifest{}, 0, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(storeDir), 0o755); err != nil {
		return snapshotManifest{}, 0, err
	}
	pairs, err := importSnapshot(ctx, staging, source, manifest)
	if err != nil {
		os.RemoveAll(staging)
		return snapshotManifest{}, 0, err
	}
	if _, err := os.Stat(storeDir); err == nil {
		if err := os.Rename(storeDir, replaced); err != nil {
			os.RemoveAll(staging)
			return snapshotManifest{}, 0, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		os.RemoveAll(staging)
		return snapshotManifest{}, 0, err
	}
	if err := os.Rename(staging, storeDir); err != nil {
		return snapshotManifest{}, 0, err
	}
	if err := os.RemoveAll(replaced); err != nil {
		return snapshotManifest{}, 0, err
	}
	return manifest, pairs, nil
}

// importSnapshot imports the pages of the snapshot of manifest from source into a new store at dir,
// checks the root of every namespace against the manifest, and saves the state, its roots and the
// block of the state, which the restored node continues the chain from. It returns the number of
// pairs imported. The block is saved without its quorum certificate: the hash links from the
// trusted anchor prove the block, not the certificate a source hands over.
func importSnapshot(ctx context.Context, dir string, source snapshotSource, manifest snapshotManifest) (uint64, error) {
	target, err := store.OpenLevelDB(dir)
	if err != nil {
		return 0, err
	}
	defer target.Close()
	roots := make(map[string]types.Hash, len(manifest.StateRoots))
	for _, root := range manifest.StateRoots {
		roots[root.Namespace] = root.Root
	}
	var (
		pairs    uint64
		current  string
		importer *store.NamespaceImport
	)
	finish := func() error {
		if importer == nil {
			return nil
		}
		root, err := importer.Root(ctx)
		if err != nil {
			return err
		}
		if root != roots[current] {
			return fmt.Errorf("%w: the pairs of namespace %q have root %x, the snapshot names %x", ErrUntrustedSnapshot, current, root, roots[current])
		}
		return nil
	}
	err = readSnapshotPages(ctx, source, manifest, func(page snapshotPage) error {
		if importer == nil || page.Namespace != current {
			if err := finish(); err != nil {
				return err
			}
			if importer, err = target.BeginNamespaceImport(ctx, page.Namespace); err != nil {
				return err
			}
			current = page.Namespace
		}
		pairs += uint64(len(page.Pairs))
		if err := importer.Add(ctx, page.Pairs); err != nil {
			return fmt.Errorf("%w: %w", ErrUntrustedSnapshot, err)
		}
		return nil
	})
	if err == nil {
		err = finish()
	}
	if err != nil {
		return 0, err
	}
	for _, root := range manifest.StateRoots {
		if err := target.SaveStateRoot(ctx, root); err != nil {
			return 0, err
		}
	}
	block := store.BlockRecord{Block: manifest.Block.Block, Hash: manifest.Block.Hash, AppHash: manifest.State.AppHash, StateRoots: manifest.StateRoots}
	if err := target.SaveBlock(ctx, block); err != nil {
		return 0, err
	}
	if err := target.SaveState(ctx, manifest.State); err != nil {
		return 0, err
	}
	if _, err := target.RecoverIndexes(ctx); err != nil {
		return 0, err
	}
	return pairs, nil
}

// replicatedNamespaces is every namespace of the replicated state of chain, as runtime.StateNamespaces
// names them.
func replicatedNamespaces(chain vexoconfig.Config) ([]string, error) {
	application, err := appmodules.NewRuntimeWithChainConfig(chain.ChainID, chain)
	if err != nil {
		return nil, err
	}
	modules := make([]string, 0, len(application.Modules()))
	for _, module := range application.Modules() {
		modules = append(modules, module.Name())
	}
	return snapshotNamespaces(modules), nil
}

// checkSnapshotManifest checks that manifest is consistent and is the state its anchor header
// commits:
//
//   - it carries exactly the namespaces of the replicated state of chain, each with one root of the
//     state's height, so no namespace outside what the checks below cover is imported;
//   - those roots give the state's app hash, recomputed the way the application of chain computes
//     it, and with the validator registry root the header app hash the anchor commits
//     (finality.HeaderAppHash), so every namespace, the registry with the validator sets of past
//     heights included, is exactly the committed state;
//   - the links and the block of the state chain back from the anchor by their hashes, so the block
//     the restored node continues from is the committed block of the state's height.
//
// The roots of the pairs themselves are checked as they are imported (importSnapshot).
func checkSnapshotManifest(ctx context.Context, manifest snapshotManifest, chain vexoconfig.Config) error {
	untrusted := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrUntrustedSnapshot}, args...)...)
	}
	header, state := manifest.Header, manifest.State
	if manifest.ChainID != chain.ChainID || header.ChainID != chain.ChainID {
		return untrusted("snapshot of chain %q anchored to a block of chain %q, node runs %q", manifest.ChainID, header.ChainID, chain.ChainID)
	}
	height, ok := finality.AppHashHeight(header)
	if !ok || height == 0 || height != state.Height || header.Height <= state.Height {
		return untrusted("header of block %d does not commit the state of height %d", header.Height, state.Height)
	}
	want, err := replicatedNamespaces(chain)
	if err != nil {
		return err
	}
	if !slices.Equal(manifest.Modules, want) {
		return untrusted("snapshot namespaces %v, the replicated state has %v", manifest.Modules, want)
	}
	if !snapshotRootsCover(manifest) {
		return untrusted("snapshot does not name one root of height %d for each of its namespaces", state.Height)
	}
	application, err := appmodules.NewRuntimeWithChainConfig(chain.ChainID, chain)
	if err != nil {
		return err
	}
	appHash, err := application.AppHashOfRoots(ctx, height, snapshotRoots(manifest.StateRoots))
	if err != nil {
		return untrusted("%v", err)
	}
	if appHash != state.AppHash {
		return untrusted("snapshot roots give app hash %x, the snapshot state names %x", appHash, state.AppHash)
	}
	headerAppHash, err := snapshotHeaderAppHash(appHash, manifest.StateRoots)
	if err != nil {
		return untrusted("%v", err)
	}
	if headerAppHash != header.AppHash {
		return untrusted("snapshot state has header app hash %x, block %d commits %x", headerAppHash, header.Height, header.AppHash)
	}
	if uint64(len(manifest.Links)) != uint64(header.Height-state.Height-1) {
		return untrusted("%d headers link block %d to block %d", len(manifest.Links), header.Height, state.Height)
	}
	expected := header.PreviousBlockHash
	for index := len(manifest.Links) - 1; index >= 0; index-- {
		link := manifest.Links[index]
		if link.ChainID != chain.ChainID || link.Height != state.Height+1+types.Height(index) || consensus.HashHeader(link) != expected {
			return untrusted("header of height %d does not link to block %d", link.Height, header.Height)
		}
		expected = link.PreviousBlockHash
	}
	block := manifest.Block
	if block.Block.Header.Height != state.Height || consensus.HashBlock(block.Block) != expected || block.Hash != expected || state.LastBlockHash != expected {
		return untrusted("the block of the snapshot state is not block %d of the anchored chain", state.Height)
	}
	return nil
}

// snapshotRootsCover reports whether manifest names exactly one root of its state's height for
// every namespace it carries, and no other.
func snapshotRootsCover(manifest snapshotManifest) bool {
	if len(manifest.StateRoots) != len(manifest.Modules) {
		return false
	}
	seen := make(map[string]struct{}, len(manifest.StateRoots))
	for _, root := range manifest.StateRoots {
		if root.Height != manifest.State.Height {
			return false
		}
		seen[root.Namespace] = struct{}{}
	}
	for _, namespace := range manifest.Modules {
		if _, found := seen[namespace]; !found {
			return false
		}
	}
	return len(seen) == len(manifest.Modules)
}

// snapshotNamespaces is every namespace of the replicated state of a chain with modules, as
// runtime.StateNamespaces names them, in sorted order.
func snapshotNamespaces(modules []string) []string {
	namespaces := append([]string(nil), modules...)
	namespaces = append(namespaces, vexoapp.CoreNamespaces()...)
	namespaces = append(namespaces, validator.RegistryNamespace)
	return sortedStrings(namespaces)
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

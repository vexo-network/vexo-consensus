package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/ops"
	"github.com/vexo-network/vexo-consensus/types"
)

const networkPIDFileName = "vexod.pid"
const networkSecond = time.Duration(1_000_000_000)
const defaultNetworkSmokeTxPrefix = "bank:send:{account}:smoke:1:fee=1000:gas=1000:signer={account}:nonce"
const defaultNetworkLoadTxPrefix = "bank:send:{account}:load-dst:1:fee=1000:gas=1000:signer={account}:nonce"

type networkRuntimePlan struct {
	Home        string
	Binary      string
	Validators  int
	P2PBasePort int
	RPCBasePort int
	Nodes       []networkNodeRuntimePlan
}

type networkNodeRuntimePlan struct {
	ValidatorID string
	Home        string
	RPCAddress  string
	P2PAddress  string
	PIDPath     string
	LogPath     string
	Args        []string
	command     *exec.Cmd
}

type networkNonceTracker struct {
	mu            sync.Mutex
	nextByAccount map[string]uint64
}

func newNetworkNonceTracker() *networkNonceTracker {
	return &networkNonceTracker{nextByAccount: make(map[string]uint64)}
}

func (tracker *networkNonceTracker) Next(account string) uint64 {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	next := tracker.nextByAccount[account]
	if next == 0 {
		next = 1
	}
	tracker.nextByAccount[account] = next + 1
	return next
}

func runNetwork(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("network subcommand is required")
	}
	switch args[0] {
	case "up":
		return runNetworkUp(context.Background(), writer, args[1:])
	case "init":
		return runNetworkInit(writer, args[1:])
	case "start":
		return runNetworkStart(writer, args[1:])
	case "status":
		return runNetworkStatus(context.Background(), writer, args[1:])
	case "smoke":
		return runNetworkSmoke(context.Background(), writer, args[1:])
	case "load":
		return runNetworkLoad(context.Background(), writer, args[1:])
	case "scale-plan":
		return runNetworkScalePlan(writer, args[1:])
	case "metrics":
		return runNetworkMetrics(context.Background(), writer, args[1:])
	case "chaos":
		return runNetworkChaos(context.Background(), writer, args[1:])
	case "chaos-plan":
		return runNetworkChaosPlan(writer, args[1:])
	case "longrun-plan":
		return runNetworkLongRunPlan(writer, args[1:])
	case "longrun":
		return runNetworkLongRun(context.Background(), writer, args[1:])
	case "analyze-longrun":
		return runNetworkAnalyzeLongRun(writer, args[1:])
	case "stop":
		return runNetworkStop(writer, args[1:])
	default:
		return fmt.Errorf("unknown network subcommand %q", args[0])
	}
}

type networkLoadResult struct {
	Submitted uint64
	Failed    uint64
	Duration  time.Duration
	PerNode   map[string]uint64
}

type networkLongRunEvidence struct {
	SchemaVersion string                       `json:"schema_version"`
	OK            bool                         `json:"ok"`
	Home          string                       `json:"home"`
	Validators    int                          `json:"validators"`
	Duration      string                       `json:"duration"`
	Rate          int                          `json:"rate"`
	StartedAtUnix int64                        `json:"started_at_unix"`
	EndedAtUnix   int64                        `json:"ended_at_unix"`
	Load          networkLongRunLoadEvidence   `json:"load"`
	Nodes         []networkLongRunNodeEvidence `json:"nodes"`
}

type networkLongRunLoadEvidence struct {
	Submitted uint64            `json:"submitted"`
	Failed    uint64            `json:"failed"`
	Duration  string            `json:"duration"`
	PerNode   map[string]uint64 `json:"per_node,omitempty"`
}

type networkLongRunNodeEvidence struct {
	ValidatorID string              `json:"validator_id"`
	RPCAddress  string              `json:"rpc_address"`
	Before      ops.MetricsSnapshot `json:"before,omitempty"`
	After       ops.MetricsSnapshot `json:"after,omitempty"`
	Sample      ops.Sample          `json:"sample,omitempty"`
	Report      ops.Report          `json:"report,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type networkLongRunAnalysis struct {
	SchemaVersion string                  `json:"schema_version"`
	OK            bool                    `json:"ok"`
	Input         string                  `json:"input"`
	Summary       []string                `json:"summary"`
	Checks        []networkLongRunCheck   `json:"checks"`
	Evidence      *networkLongRunEvidence `json:"evidence,omitempty"`
}

type networkLongRunCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type networkScalePlan struct {
	SchemaVersion             string                 `json:"schema_version"`
	Home                      string                 `json:"home"`
	Validators                int                    `json:"validators"`
	Regions                   int                    `json:"regions"`
	Hosts                     int                    `json:"hosts"`
	Duration                  string                 `json:"duration"`
	Rate                      int                    `json:"rate"`
	EstimatedTransactions     uint64                 `json:"estimated_transactions"`
	FaultTolerance            int                    `json:"fault_tolerance"`
	QuorumPower               int                    `json:"quorum_power"`
	TotalPeers                int                    `json:"total_peers"`
	FullMeshConnections       int                    `json:"full_mesh_connections"`
	PerNodeInboundPeerBudget  int                    `json:"per_node_inbound_peer_budget"`
	PerNodeOutboundPeerBudget int                    `json:"per_node_outbound_peer_budget"`
	Nodes                     []networkScaleNodePlan `json:"nodes"`
	Warnings                  []string               `json:"warnings,omitempty"`
}

type networkScaleNodePlan struct {
	ValidatorID string `json:"validator_id"`
	Host        string `json:"host"`
	Region      string `json:"region"`
	RPCAddress  string `json:"rpc_address"`
	P2PAddress  string `json:"p2p_address"`
	Seed        bool   `json:"seed"`
}

func runNetworkLoad(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network load", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	durationValue := flags.String("duration", "30s", "load test duration")
	rate := flags.Int("rate", 10, "transactions per second")
	timeoutValue := flags.String("timeout", "2s", "per-request timeout")
	txPrefix := flags.String("tx-prefix", defaultNetworkLoadTxPrefix, "transaction payload prefix; nonce suffix is appended; {validator}, {node}, and {account} are replaced per target node")
	dryRun := flags.Bool("dry-run", false, "print load plan without submitting transactions")
	if err := flags.Parse(args); err != nil {
		return err
	}
	duration, err := parseNetworkDuration(*durationValue)
	if err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	if *rate <= 0 {
		return errors.New("rate must be positive")
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	if *dryRun {
		writeNetworkLoadPlan(writer, plan, duration, *rate, timeout, *txPrefix)
		return nil
	}
	loadCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	client := http.Client{Timeout: timeout}
	chainID, err := networkPlanChainID(plan)
	if err != nil {
		return err
	}
	result := runNetworkLoadPlan(loadCtx, client, plan, chainID, *rate, *txPrefix)
	fmt.Fprintf(writer, "network load complete\n")
	fmt.Fprintf(writer, "submitted: %d\n", result.Submitted)
	fmt.Fprintf(writer, "failed: %d\n", result.Failed)
	fmt.Fprintf(writer, "duration: %s\n", result.Duration)
	return nil
}

func runNetworkScalePlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network scale-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 64, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	durationValue := flags.String("duration", "24h", "target scale validation duration")
	rate := flags.Int("rate", 50, "target transactions per second")
	regionCount := flags.Int("regions", 3, "number of logical regions")
	hostCount := flags.Int("hosts", 8, "number of independent machines")
	inboundPeers := flags.Int("inbound-peers", 128, "per-node inbound peer budget")
	outboundPeers := flags.Int("outbound-peers", 64, "per-node outbound peer budget")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	duration, err := parseNetworkDuration(*durationValue)
	if err != nil {
		return err
	}
	if *rate <= 0 {
		return errors.New("rate must be positive")
	}
	if *regionCount <= 0 {
		return errors.New("regions must be positive")
	}
	if *hostCount <= 0 {
		return errors.New("hosts must be positive")
	}
	if *inboundPeers < 0 || *outboundPeers < 0 {
		return errors.New("peer budgets must not be negative")
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	scalePlan := buildNetworkScalePlan(plan, duration, *rate, *regionCount, *hostCount, *inboundPeers, *outboundPeers)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(scalePlan)
	}
	writeNetworkScalePlan(writer, scalePlan)
	return nil
}

func runNetworkChaosPlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network chaos-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	durationValue := flags.String("duration", "24h", "target chaos run duration")
	regionCount := flags.Int("regions", 3, "number of logical regions to spread validators across")
	if err := flags.Parse(args); err != nil {
		return err
	}
	duration, err := parseNetworkDuration(*durationValue)
	if err != nil {
		return err
	}
	if *regionCount <= 0 {
		return errors.New("regions must be positive")
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	writeNetworkChaosPlan(writer, plan, duration, *regionCount)
	return nil
}

func runNetworkLongRunPlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network longrun-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	durationValue := flags.String("duration", "168h", "target long-run duration")
	regionCount := flags.Int("regions", 3, "number of logical regions")
	hostCount := flags.Int("hosts", 4, "number of independent machines")
	if err := flags.Parse(args); err != nil {
		return err
	}
	duration, err := parseNetworkDuration(*durationValue)
	if err != nil {
		return err
	}
	if *regionCount <= 0 {
		return errors.New("regions must be positive")
	}
	if *hostCount <= 0 {
		return errors.New("hosts must be positive")
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	writeNetworkLongRunPlan(writer, plan, duration, *regionCount, *hostCount)
	return nil
}

func runNetworkLongRun(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network longrun", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	durationValue := flags.String("duration", "1h", "long-run execution duration")
	timeoutValue := flags.String("timeout", "2s", "per-request timeout")
	rate := flags.Int("rate", 10, "transactions per second during long-run")
	txPrefix := flags.String("tx-prefix", defaultNetworkLoadTxPrefix, "transaction payload prefix; nonce suffix is appended; {validator}, {node}, and {account} are replaced per target node")
	outputPath := flags.String("output", "", "optional JSON evidence output path")
	jsonOutput := flags.Bool("json", false, "write JSON evidence to stdout")
	dryRun := flags.Bool("dry-run", false, "print long-run harness plan without querying or submitting")
	if err := flags.Parse(args); err != nil {
		return err
	}
	duration, err := parseNetworkDuration(*durationValue)
	if err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	if *rate <= 0 {
		return errors.New("rate must be positive")
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	if *dryRun {
		writeNetworkLongRunHarnessPlan(writer, plan, duration, timeout, *rate, *txPrefix, *outputPath)
		return nil
	}
	client := http.Client{Timeout: timeout}
	chainID, err := networkPlanChainID(plan)
	if err != nil {
		return err
	}
	evidence := runNetworkLongRunEvidence(ctx, client, plan, chainID, duration, *rate, *txPrefix)
	if *outputPath != "" {
		encoded, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*outputPath, append(encoded, '\n'), 0o644); err != nil {
			return err
		}
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(evidence)
	}
	status := "ok"
	if !evidence.OK {
		status = "alert"
	}
	fmt.Fprintf(writer, "network longrun %s\n", status)
	fmt.Fprintf(writer, "home: %s\n", evidence.Home)
	fmt.Fprintf(writer, "validators: %d\n", evidence.Validators)
	fmt.Fprintf(writer, "duration: %s\n", evidence.Duration)
	fmt.Fprintf(writer, "submitted: %d\n", evidence.Load.Submitted)
	fmt.Fprintf(writer, "failed: %d\n", evidence.Load.Failed)
	if *outputPath != "" {
		fmt.Fprintf(writer, "evidence: %s\n", *outputPath)
	}
	for _, node := range evidence.Nodes {
		if node.Error != "" {
			fmt.Fprintf(writer, "%s rpc=%s error=%s\n", node.ValidatorID, node.RPCAddress, node.Error)
			continue
		}
		nodeStatus := "ok"
		if !node.Report.OK {
			nodeStatus = "alert"
		}
		fmt.Fprintf(writer, "%s rpc=%s height_before=%d height_after=%d rate=%.2f/min status=%s\n", node.ValidatorID, node.RPCAddress, node.Before.LatestHeight, node.After.LatestHeight, node.Sample.HeightRatePerMinute, nodeStatus)
	}
	return nil
}

func runNetworkAnalyzeLongRun(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network analyze-longrun", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "long-run evidence JSON file")
	minValidators := flags.Int("min-validators", 4, "minimum expected validators")
	minDurationValue := flags.String("min-duration", "1h", "minimum observed long-run duration")
	includeEvidence := flags.Bool("include-evidence", false, "include original evidence in JSON output")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("input is required")
	}
	minDuration, err := parseNetworkDuration(*minDurationValue)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(*inputPath)
	if err != nil {
		return err
	}
	var evidence networkLongRunEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return err
	}
	analysis := analyzeNetworkLongRunEvidence(*inputPath, evidence, *minValidators, minDuration)
	if *includeEvidence {
		analysis.Evidence = &evidence
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(analysis)
	}
	status := "ok"
	if !analysis.OK {
		status = "failed"
	}
	fmt.Fprintf(writer, "network longrun analysis %s\n", status)
	fmt.Fprintf(writer, "input: %s\n", analysis.Input)
	for _, summary := range analysis.Summary {
		fmt.Fprintf(writer, "- %s\n", summary)
	}
	for _, check := range analysis.Checks {
		fmt.Fprintf(writer, "%s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	return nil
}

func analyzeNetworkLongRunEvidence(inputPath string, evidence networkLongRunEvidence, minValidators int, minDuration time.Duration) networkLongRunAnalysis {
	analysis := networkLongRunAnalysis{
		SchemaVersion: "v1",
		OK:            true,
		Input:         inputPath,
	}
	addCheck := func(name string, ok bool, message string) {
		if !ok {
			analysis.OK = false
		}
		analysis.Checks = append(analysis.Checks, networkLongRunCheck{Name: name, OK: ok, Message: message})
	}
	addCheck("schema_version", strings.TrimSpace(evidence.SchemaVersion) == "v1", "long-run evidence schema version must be v1")
	addCheck("evidence_ok", evidence.OK, "long-run harness must report ok=true")
	addCheck("validator_count", evidence.Validators >= minValidators, fmt.Sprintf("validators must be at least %d", minValidators))
	addCheck("node_count", len(evidence.Nodes) >= minValidators, fmt.Sprintf("node evidence must include at least %d validators", minValidators))
	addCheck("load_submitted", evidence.Load.Submitted > 0, "long-run load must submit transactions")
	addCheck("load_failed", evidence.Load.Failed == 0, "long-run load must not record failed transactions")
	observedDuration := observedLongRunDuration(evidence)
	addCheck("duration", observedDuration >= minDuration, fmt.Sprintf("observed duration %s must be at least %s", observedDuration, minDuration))
	thresholds := ops.DefaultThresholds()
	for _, node := range evidence.Nodes {
		prefix := "node_" + node.ValidatorID
		addCheck(prefix+"_error", strings.TrimSpace(node.Error) == "", "node evidence must not include collection errors")
		addCheck(prefix+"_height_growth", node.After.LatestHeight > node.Before.LatestHeight, "latest height must increase during the long run")
		addCheck(prefix+"_report_ok", node.Report.OK, "ops report must satisfy alert thresholds")
		addCheck(prefix+"_height_rate", node.Sample.HeightRatePerMinute >= thresholds.MinHeightRatePerMinute, fmt.Sprintf("height rate %.2f/min must meet threshold %.2f/min", node.Sample.HeightRatePerMinute, thresholds.MinHeightRatePerMinute))
		if thresholds.SnapshotRequired {
			addCheck(prefix+"_snapshot", node.Sample.SnapshotHealthy, "snapshot health must be true")
		}
		if thresholds.ReplayHealthyRequired {
			addCheck(prefix+"_replay", node.Sample.ReplayHealthy, "replay health must be true")
		}
	}
	analysis.Summary = append(analysis.Summary,
		fmt.Sprintf("validators=%d nodes=%d", evidence.Validators, len(evidence.Nodes)),
		fmt.Sprintf("load submitted=%d failed=%d", evidence.Load.Submitted, evidence.Load.Failed),
		fmt.Sprintf("observed_duration=%s", observedDuration),
	)
	return analysis
}

func observedLongRunDuration(evidence networkLongRunEvidence) time.Duration {
	if duration, err := parseNetworkDuration(evidence.Duration); err == nil {
		return duration
	}
	if evidence.StartedAtUnix > 0 && evidence.EndedAtUnix > evidence.StartedAtUnix {
		return time.Duration(evidence.EndedAtUnix-evidence.StartedAtUnix) * time.Second
	}
	if duration, err := parseNetworkDuration(evidence.Load.Duration); err == nil {
		return duration
	}
	return 0
}

type networkSmokeResult struct {
	ValidatorID string
	RPCAddress  string
	Healthy     bool
	Height      uint64
}

type networkStatusResponse struct {
	LatestHeight        uint64 `json:"latest_height"`
	PeerCount           int    `json:"peer_count"`
	ActivePeerCount     int    `json:"active_peer_count"`
	ConfiguredPeerCount int    `json:"configured_peer_count"`
	ScoredPeerCount     int    `json:"scored_peer_count"`
}

type networkMetricsResponse struct {
	ValidatorID string              `json:"validator_id"`
	RPCAddress  string              `json:"rpc_address"`
	Metrics     ops.MetricsSnapshot `json:"metrics"`
	Report      ops.Report          `json:"report,omitempty"`
	Error       string              `json:"error,omitempty"`
}

func runNetworkUp(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network up", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	keyType := flags.String("key-type", vexocrypto.KeyTypeBLS, "validator consensus key type: bls or ed25519")
	binaryPath := flags.String("binary", "", "vexod binary path; defaults to current executable")
	timeoutValue := flags.String("timeout", "60s", "startup and smoke test timeout")
	tx := flags.String("tx", defaultNetworkSmokeTxPrefix, "transaction payload template to submit; nonce suffix is appended when needed")
	overwrite := flags.Bool("overwrite", false, "overwrite existing network files")
	keepRunning := flags.Bool("keep-running", false, "leave nodes running after smoke test")
	dryRun := flags.Bool("dry-run", false, "print orchestration plan without writing files or spawning processes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, *binaryPath, *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	if *dryRun {
		writeNetworkUpPlan(writer, plan, *chainID, timeout, *tx, *overwrite, *keepRunning)
		return nil
	}
	networkFiles, err := writeNetworkFilesWithOptionsAndKeyType(*home, *chainID, *validators, *overwrite, networkAddressOptions{
		P2PBasePort:     *p2pBasePort,
		RPCBasePort:     *rpcBasePort,
		P2PPortStep:     10,
		RPCPortStep:     10,
		P2PHostTemplate: "127.0.0.1",
		RPCHostTemplate: "127.0.0.1",
	}, *keyType, false, "", true)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "initialized vexo network home=%s validators=%d\n", networkFiles.Home, len(networkFiles.Nodes))
	nodesStarted := false
	if err := startNetworkPlan(writer, plan); err != nil {
		return err
	}
	nodesStarted = true
	if !*keepRunning {
		defer func() {
			if nodesStarted {
				_ = stopNetworkPlan(writer, plan)
			}
		}()
	}
	smokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := http.Client{Timeout: timeout}
	results, err := runNetworkSmokePlan(smokeCtx, client, plan, *chainID, *tx)
	if err != nil {
		writeNetworkLogTails(writer, plan, 4096)
		return err
	}
	for _, result := range results {
		fmt.Fprintf(writer, "%s rpc=%s healthy=%t height=%d\n", result.ValidatorID, result.RPCAddress, result.Healthy, result.Height)
	}
	if *keepRunning {
		fmt.Fprintf(writer, "network up ok; nodes are running\n")
		return nil
	}
	fmt.Fprintf(writer, "network up ok; stopping nodes\n")
	return nil
}

func runNetworkSmoke(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network smoke", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	timeoutValue := flags.String("timeout", "10s", "smoke test timeout")
	tx := flags.String("tx", defaultNetworkSmokeTxPrefix, "transaction payload template to submit; nonce suffix is appended when needed")
	if err := flags.Parse(args); err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := http.Client{Timeout: timeout}
	chainID, err := networkPlanChainID(plan)
	if err != nil {
		return err
	}
	results, err := runNetworkSmokePlan(ctx, client, plan, chainID, *tx)
	if err != nil {
		writeNetworkLogTails(writer, plan, 4096)
		return err
	}
	for _, result := range results {
		fmt.Fprintf(writer, "%s rpc=%s healthy=%t height=%d\n", result.ValidatorID, result.RPCAddress, result.Healthy, result.Height)
	}
	fmt.Fprintf(writer, "network smoke ok\n")
	return nil
}

func runNetworkInit(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	p2pPortStep := flags.Int("p2p-port-step", 10, "P2P port increment per validator")
	rpcPortStep := flags.Int("rpc-port-step", 10, "RPC port increment per validator")
	networkConfigPath := flags.String("network-config", "", "network topology config file for generated validator addresses")
	web3DevAccounts := flags.Bool("web3-dev-accounts", false, "generate prefunded Web3 managed accounts for local Remix deployment")
	encryptKeys := flags.Bool("encrypt-keys", false, "encrypt generated validator and VRF key files")
	passphrase := flags.String("passphrase", "", "key encryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	overwrite := flags.Bool("overwrite", false, "overwrite existing network files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	initArgs := []string{
		"--home", *home,
		"--chain-id", *chainID,
		"--validators", strconv.Itoa(*validators),
	}
	if *encryptKeys {
		initArgs = append(initArgs, "--encrypt-keys")
	}
	if *passphrase != "" {
		initArgs = append(initArgs, "--passphrase", *passphrase)
	}
	if *networkConfigPath != "" {
		initArgs = append(initArgs, "--network-config", *networkConfigPath)
	} else {
		initArgs = append(initArgs,
			"--p2p-base-port", strconv.Itoa(*p2pBasePort),
			"--rpc-base-port", strconv.Itoa(*rpcBasePort),
			"--p2p-port-step", strconv.Itoa(*p2pPortStep),
			"--rpc-port-step", strconv.Itoa(*rpcPortStep),
		)
	}
	if *web3DevAccounts {
		initArgs = append(initArgs, "--web3-dev-accounts")
	}
	if *overwrite {
		initArgs = append(initArgs, "--overwrite")
	}
	return runInit(writer, initArgs)
}

func runNetworkStart(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	binaryPath := flags.String("binary", "", "vexod binary path; defaults to current executable")
	dryRun := flags.Bool("dry-run", false, "print node start commands without spawning processes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, *binaryPath, *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	if *dryRun {
		writeNetworkPlan(writer, plan)
		return nil
	}
	return startNetworkPlan(writer, plan)
}

func runNetworkStatus(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	timeoutValue := flags.String("timeout", "2s", "per-node health check timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: timeout}
	for _, localNode := range plan.Nodes {
		ok := networkHealthOK(ctx, client, localNode.RPCAddress)
		fmt.Fprintf(writer, "%s rpc=%s healthy=%t\n", localNode.ValidatorID, localNode.RPCAddress, ok)
	}
	return nil
}

func runNetworkMetrics(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network metrics", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	timeoutValue := flags.String("timeout", "2s", "per-node metrics query timeout")
	evaluate := flags.Bool("evaluate", false, "evaluate each metrics response against default alert thresholds")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: timeout}
	results := collectNetworkMetrics(ctx, client, plan, *evaluate)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	}
	for _, result := range results {
		if result.Error != "" {
			fmt.Fprintf(writer, "%s rpc=%s metrics_error=%s\n", result.ValidatorID, result.RPCAddress, result.Error)
			continue
		}
		status := "collected"
		if *evaluate {
			if result.Report.OK {
				status = "ok"
			} else {
				status = "alert"
			}
		}
		fmt.Fprintf(writer, "%s rpc=%s height=%d mempool=%d bans=%d status=%s\n", result.ValidatorID, result.RPCAddress, result.Metrics.LatestHeight, result.Metrics.MempoolSize, result.Metrics.BannedPeers, status)
	}
	return nil
}

func runNetworkChaos(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network chaos", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	binaryPath := flags.String("binary", "", "vexod binary path; defaults to current executable")
	timeoutValue := flags.String("timeout", "20s", "chaos scenario timeout")
	stopIndex := flags.Int("stop-index", -1, "zero-based validator index to stop; defaults to last validator")
	tx := flags.String("tx", defaultNetworkSmokeTxPrefix, "transaction payload template to submit while one validator is stopped")
	restart := flags.Bool("restart", true, "restart the stopped validator and wait for catch-up")
	dryRun := flags.Bool("dry-run", false, "print chaos execution steps without stopping or starting processes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	timeout, err := parseNetworkDuration(*timeoutValue)
	if err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, *binaryPath, *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	if len(plan.Nodes) < 2 {
		return errors.New("chaos run requires at least two validators")
	}
	targetIndex := *stopIndex
	if targetIndex < 0 {
		targetIndex = len(plan.Nodes) - 1
	}
	if targetIndex < 0 || targetIndex >= len(plan.Nodes) {
		return fmt.Errorf("stop-index must be between 0 and %d", len(plan.Nodes)-1)
	}
	if *dryRun {
		writeNetworkChaosRunPlan(writer, plan, targetIndex, timeout, *tx, *restart)
		return nil
	}
	chaosCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := http.Client{Timeout: timeout}
	chainID, err := networkPlanChainID(plan)
	if err != nil {
		return err
	}
	return runNetworkChaosPlanExecution(chaosCtx, writer, client, plan, chainID, targetIndex, *tx, *restart)
}

func runNetworkStop(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("network stop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-network", "network home directory")
	validators := flags.Int("validators", 4, "validator count")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := buildNetworkRuntimePlanWithPorts(*home, *validators, "", *p2pBasePort, *rpcBasePort)
	if err != nil {
		return err
	}
	return stopNetworkPlan(writer, plan)
}

func startNetworkPlan(writer io.Writer, plan networkRuntimePlan) error {
	started := networkRuntimePlan{Nodes: make([]networkNodeRuntimePlan, 0, len(plan.Nodes))}
	for index, localNode := range plan.Nodes {
		command, err := startNetworkNode(plan.Binary, localNode)
		if err != nil {
			_ = stopNetworkPlan(io.Discard, started)
			return err
		}
		localNode.command = command
		plan.Nodes[index].command = command
		started.Nodes = append(started.Nodes, localNode)
		fmt.Fprintf(writer, "started %s pid=%s log=%s rpc=%s p2p=%s\n", localNode.ValidatorID, localNode.PIDPath, localNode.LogPath, localNode.RPCAddress, localNode.P2PAddress)
	}
	return nil
}

func stopNetworkPlan(writer io.Writer, plan networkRuntimePlan) error {
	for _, localNode := range plan.Nodes {
		pid, err := readNetworkPID(localNode.PIDPath)
		if err != nil {
			fmt.Fprintf(writer, "stopped %s pid=missing\n", localNode.ValidatorID)
			continue
		}
		if localNode.command != nil {
			if err := stopNetworkCommand(localNode.command); err != nil {
				return err
			}
		} else if err := stopNetworkPID(pid); err != nil {
			return err
		}
		waitNetworkRPCDown(localNode.RPCAddress, 2*time.Second)
		_ = os.Remove(localNode.PIDPath)
		fmt.Fprintf(writer, "stopped %s pid=%d\n", localNode.ValidatorID, pid)
	}
	return nil
}

func buildNetworkRuntimePlan(home string, validators int, binaryPath string) (networkRuntimePlan, error) {
	return buildNetworkRuntimePlanWithPorts(home, validators, binaryPath, defaultP2PBasePort, defaultRPCBasePort)
}

func buildNetworkRuntimePlanWithPorts(home string, validators int, binaryPath string, p2pBasePort int, rpcBasePort int) (networkRuntimePlan, error) {
	if validators <= 0 {
		return networkRuntimePlan{}, fmt.Errorf("validators must be positive")
	}
	if p2pBasePort <= 0 || rpcBasePort <= 0 {
		return networkRuntimePlan{}, fmt.Errorf("base ports must be positive")
	}
	if home == "" {
		home = ".vexo-network"
	}
	if binaryPath == "" {
		executable, err := os.Executable()
		if err != nil {
			return networkRuntimePlan{}, err
		}
		binaryPath = executable
	}
	plan := networkRuntimePlan{
		Home:        home,
		Binary:      binaryPath,
		Validators:  validators,
		P2PBasePort: p2pBasePort,
		RPCBasePort: rpcBasePort,
		Nodes:       make([]networkNodeRuntimePlan, 0, validators),
	}
	for index := 1; index <= validators; index++ {
		validatorID := networkValidatorID(index)
		nodeHome := filepath.Join(home, validatorID)
		rpcAddress := networkRPCAddressWithBasePort(index, rpcBasePort)
		p2pAddress := networkP2PAddressWithBasePort(index, p2pBasePort)
		configPath := filepath.Join(nodeHome, configFileName)
		if document, err := readConfigDocument(configPath); err == nil {
			if networkDocument, err := loadNetworkConfigForConfig(configPath, document); err == nil {
				if networkDocument.RPC.Address != "" {
					rpcAddress = networkDocument.RPC.Address
				}
				if networkDocument.P2P.ListenAddress != "" {
					p2pAddress = networkDocument.P2P.ListenAddress
				}
			}
		}
		args := []string{
			"start",
			"--home", nodeHome,
		}
		plan.Nodes = append(plan.Nodes, networkNodeRuntimePlan{
			ValidatorID: validatorID,
			Home:        nodeHome,
			RPCAddress:  rpcAddress,
			P2PAddress:  p2pAddress,
			PIDPath:     filepath.Join(nodeHome, networkPIDFileName),
			LogPath:     filepath.Join(nodeHome, "vexod.log"),
			Args:        args,
		})
	}
	return plan, nil
}

func writeNetworkPlan(writer io.Writer, plan networkRuntimePlan) {
	fmt.Fprintf(writer, "network start plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "p2p-base-port: %d\n", plan.P2PBasePort)
	fmt.Fprintf(writer, "rpc-base-port: %d\n", plan.RPCBasePort)
	for _, localNode := range plan.Nodes {
		fmt.Fprintf(writer, "%s: %s %s # log=%s pid=%s\n", localNode.ValidatorID, plan.Binary, joinArgs(localNode.Args), localNode.LogPath, localNode.PIDPath)
	}
}

func writeNetworkUpPlan(writer io.Writer, plan networkRuntimePlan, chainID string, timeout time.Duration, tx string, overwrite bool, keepRunning bool) {
	fmt.Fprintf(writer, "network up plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "chain-id: %s\n", chainID)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "p2p-base-port: %d\n", plan.P2PBasePort)
	fmt.Fprintf(writer, "rpc-base-port: %d\n", plan.RPCBasePort)
	fmt.Fprintf(writer, "timeout: %s\n", timeout)
	fmt.Fprintf(writer, "tx: %s\n", tx)
	initCommand := fmt.Sprintf("network init --home %s --chain-id %s --validators %d --p2p-base-port %d --rpc-base-port %d --web3-dev-accounts", plan.Home, chainID, plan.Validators, plan.P2PBasePort, plan.RPCBasePort)
	if overwrite {
		initCommand += " --overwrite"
	}
	fmt.Fprintf(writer, "1. %s\n", initCommand)
	fmt.Fprintf(writer, "2. network start --home %s --validators %d --p2p-base-port %d --rpc-base-port %d --binary %s\n", plan.Home, plan.Validators, plan.P2PBasePort, plan.RPCBasePort, plan.Binary)
	fmt.Fprintf(writer, "3. network smoke --home %s --validators %d --p2p-base-port %d --rpc-base-port %d --timeout %s --tx %s\n", plan.Home, plan.Validators, plan.P2PBasePort, plan.RPCBasePort, timeout, tx)
	if keepRunning {
		fmt.Fprintf(writer, "4. keep nodes running\n")
		return
	}
	fmt.Fprintf(writer, "4. network stop --home %s --validators %d --p2p-base-port %d --rpc-base-port %d\n", plan.Home, plan.Validators, plan.P2PBasePort, plan.RPCBasePort)
}

func writeNetworkLoadPlan(writer io.Writer, plan networkRuntimePlan, duration time.Duration, rate int, timeout time.Duration, txPrefix string) {
	fmt.Fprintf(writer, "network load plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "duration: %s\n", duration)
	fmt.Fprintf(writer, "rate: %d tx/s\n", rate)
	fmt.Fprintf(writer, "request_timeout: %s\n", timeout)
	fmt.Fprintf(writer, "tx_prefix: %s\n", txPrefix)
	fmt.Fprintf(writer, "target_rpc: %s\n", plan.Nodes[0].RPCAddress)
	fmt.Fprintf(writer, "estimated_transactions: %d\n", estimatedNetworkTransactions(duration, rate))
}

func buildNetworkScalePlan(plan networkRuntimePlan, duration time.Duration, rate int, regions int, hosts int, inboundPeers int, outboundPeers int) networkScalePlan {
	scalePlan := networkScalePlan{
		SchemaVersion:             "v1",
		Home:                      plan.Home,
		Validators:                plan.Validators,
		Regions:                   regions,
		Hosts:                     hosts,
		Duration:                  duration.String(),
		Rate:                      rate,
		EstimatedTransactions:     estimatedNetworkTransactions(duration, rate),
		FaultTolerance:            networkFaultTolerance(plan.Validators),
		QuorumPower:               networkQuorumPower(plan.Validators),
		TotalPeers:                plan.Validators,
		FullMeshConnections:       networkFullMeshConnections(plan.Validators),
		PerNodeInboundPeerBudget:  inboundPeers,
		PerNodeOutboundPeerBudget: outboundPeers,
		Nodes:                     make([]networkScaleNodePlan, 0, len(plan.Nodes)),
	}
	seedLimit := 3
	if seedLimit > len(plan.Nodes) {
		seedLimit = len(plan.Nodes)
	}
	for index, localNode := range plan.Nodes {
		scalePlan.Nodes = append(scalePlan.Nodes, networkScaleNodePlan{
			ValidatorID: localNode.ValidatorID,
			Host:        fmt.Sprintf("node-%d", (index%hosts)+1),
			Region:      fmt.Sprintf("region-%d", (index%regions)+1),
			RPCAddress:  localNode.RPCAddress,
			P2PAddress:  localNode.P2PAddress,
			Seed:        index < seedLimit,
		})
	}
	scalePlan.Warnings = networkScaleWarnings(scalePlan)
	return scalePlan
}

func writeNetworkScalePlan(writer io.Writer, plan networkScalePlan) {
	fmt.Fprintf(writer, "network scale plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "regions: %d\n", plan.Regions)
	fmt.Fprintf(writer, "hosts: %d\n", plan.Hosts)
	fmt.Fprintf(writer, "duration: %s\n", plan.Duration)
	fmt.Fprintf(writer, "rate: %d tx/s\n", plan.Rate)
	fmt.Fprintf(writer, "estimated_transactions: %d\n", plan.EstimatedTransactions)
	fmt.Fprintf(writer, "fault_tolerance: %d\n", plan.FaultTolerance)
	fmt.Fprintf(writer, "quorum_power: %d\n", plan.QuorumPower)
	fmt.Fprintf(writer, "full_mesh_connections: %d\n", plan.FullMeshConnections)
	fmt.Fprintf(writer, "peer_budget: inbound=%d outbound=%d\n", plan.PerNodeInboundPeerBudget, plan.PerNodeOutboundPeerBudget)
	for _, localNode := range plan.Nodes {
		fmt.Fprintf(writer, "%s: host=%s region=%s seed=%t rpc=%s p2p=%s\n", localNode.ValidatorID, localNode.Host, localNode.Region, localNode.Seed, localNode.RPCAddress, localNode.P2PAddress)
	}
	fmt.Fprintf(writer, "checks:\n")
	fmt.Fprintf(writer, "1. run network init/start with matching ports and copied genesis files\n")
	fmt.Fprintf(writer, "2. require at least %d live validators before accepting finality\n", plan.QuorumPower)
	fmt.Fprintf(writer, "3. sustain %d tx/s for %s and submit about %d txs\n", plan.Rate, plan.Duration, plan.EstimatedTransactions)
	fmt.Fprintf(writer, "4. collect metrics for commit latency, round timeouts, peer bans, mempool, signer failures, snapshots, and replay\n")
	fmt.Fprintf(writer, "5. isolate up to %d faulty validators and require no conflicting finality\n", plan.FaultTolerance)
	fmt.Fprintf(writer, "6. rotate validators during load and verify light-client finality proofs by height-specific validator set\n")
	if len(plan.Warnings) > 0 {
		fmt.Fprintf(writer, "warnings:\n")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(writer, "- %s\n", warning)
		}
	}
}

func networkScaleWarnings(plan networkScalePlan) []string {
	warnings := make([]string, 0)
	if plan.Validators < 4 {
		warnings = append(warnings, "fewer than 4 validators cannot tolerate one Byzantine validator with 2f+1 quorum")
	}
	if plan.Hosts > plan.Validators {
		warnings = append(warnings, "hosts exceed validators; some hosts will be unused")
	}
	if plan.PerNodeInboundPeerBudget+plan.PerNodeOutboundPeerBudget < plan.Validators-1 {
		warnings = append(warnings, "peer budget is below full mesh; verify gossip fanout and seed coverage")
	}
	if plan.EstimatedTransactions == 0 {
		warnings = append(warnings, "estimated transaction count is zero; increase duration or rate")
	}
	return warnings
}

func networkFaultTolerance(validators int) int {
	if validators <= 0 {
		return 0
	}
	return (validators - 1) / 3
}

func networkQuorumPower(validators int) int {
	if validators <= 0 {
		return 0
	}
	return ((validators * 2) / 3) + 1
}

func networkFullMeshConnections(validators int) int {
	if validators <= 1 {
		return 0
	}
	return validators * (validators - 1) / 2
}

func writeNetworkChaosPlan(writer io.Writer, plan networkRuntimePlan, duration time.Duration, regions int) {
	fmt.Fprintf(writer, "network chaos plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "duration: %s\n", duration)
	fmt.Fprintf(writer, "regions: %d\n", regions)
	for index, localNode := range plan.Nodes {
		region := (index % regions) + 1
		fmt.Fprintf(writer, "%s: region=%d rpc=%s p2p=%s\n", localNode.ValidatorID, region, localNode.RPCAddress, localNode.P2PAddress)
	}
	fmt.Fprintf(writer, "steps:\n")
	fmt.Fprintf(writer, "1. start network with --keep-running\n")
	fmt.Fprintf(writer, "2. run network load during the full window\n")
	fmt.Fprintf(writer, "3. stop one non-quorum validator and confirm height still increases\n")
	fmt.Fprintf(writer, "4. stop quorum-sized partition and confirm no conflicting finality\n")
	fmt.Fprintf(writer, "5. restart stopped validators and confirm state catches up\n")
	fmt.Fprintf(writer, "6. run snapshot export/verify/restore from a surviving node\n")
}

func writeNetworkChaosRunPlan(writer io.Writer, plan networkRuntimePlan, targetIndex int, timeout time.Duration, tx string, restart bool) {
	target := plan.Nodes[targetIndex]
	survivor := networkSurvivorNode(plan, targetIndex)
	fmt.Fprintf(writer, "network chaos run plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "target: %s\n", target.ValidatorID)
	fmt.Fprintf(writer, "survivor_rpc: %s\n", survivor.RPCAddress)
	fmt.Fprintf(writer, "timeout: %s\n", timeout)
	fmt.Fprintf(writer, "tx: %s\n", tx)
	fmt.Fprintf(writer, "restart: %t\n", restart)
	fmt.Fprintf(writer, "steps:\n")
	fmt.Fprintf(writer, "1. wait all validators healthy\n")
	fmt.Fprintf(writer, "2. stop %s and keep quorum online\n", target.ValidatorID)
	fmt.Fprintf(writer, "3. submit tx through %s and require height increase\n", survivor.ValidatorID)
	if restart {
		fmt.Fprintf(writer, "4. restart %s and require catch-up\n", target.ValidatorID)
	} else {
		fmt.Fprintf(writer, "4. leave %s stopped for operator inspection\n", target.ValidatorID)
	}
}

func writeNetworkLongRunPlan(writer io.Writer, plan networkRuntimePlan, duration time.Duration, regions int, hosts int) {
	fmt.Fprintf(writer, "network longrun plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "duration: %s\n", duration)
	fmt.Fprintf(writer, "regions: %d\n", regions)
	fmt.Fprintf(writer, "hosts: %d\n", hosts)
	for index, localNode := range plan.Nodes {
		region := (index % regions) + 1
		host := (index % hosts) + 1
		fmt.Fprintf(writer, "%s: host=node-%d region=%d rpc=%s p2p=%s\n", localNode.ValidatorID, host, region, localNode.RPCAddress, localNode.P2PAddress)
	}
	fmt.Fprintf(writer, "phases:\n")
	fmt.Fprintf(writer, "1. provision independent machines and copy matching genesis/config/key files\n")
	fmt.Fprintf(writer, "2. start all validators and confirm peer connectivity plus height growth\n")
	fmt.Fprintf(writer, "3. run sustained load for %s and collect metrics, logs, and pprof samples\n", duration)
	fmt.Fprintf(writer, "4. inject restarts, packet loss, latency, and one-region isolation\n")
	fmt.Fprintf(writer, "5. verify no conflicting finality, state sync recovery, snapshots, and KMS challenge signatures\n")
}

func writeNetworkLongRunHarnessPlan(writer io.Writer, plan networkRuntimePlan, duration time.Duration, timeout time.Duration, rate int, txPrefix string, outputPath string) {
	fmt.Fprintf(writer, "network longrun harness plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	fmt.Fprintf(writer, "duration: %s\n", duration)
	fmt.Fprintf(writer, "request_timeout: %s\n", timeout)
	fmt.Fprintf(writer, "rate: %d tx/s\n", rate)
	fmt.Fprintf(writer, "tx_prefix: %s\n", txPrefix)
	if outputPath != "" {
		fmt.Fprintf(writer, "evidence_output: %s\n", outputPath)
	}
	fmt.Fprintf(writer, "steps:\n")
	fmt.Fprintf(writer, "1. collect baseline /v1/metrics from all validators\n")
	fmt.Fprintf(writer, "2. submit realistic signed-shape load payloads for %s\n", duration)
	fmt.Fprintf(writer, "3. collect final /v1/metrics from all validators\n")
	fmt.Fprintf(writer, "4. evaluate height rate, round timeouts, latency, bans, mempool, snapshot, replay, and signer failures\n")
	fmt.Fprintf(writer, "5. emit machine-readable long-run evidence JSON\n")
}

func parseNetworkDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("duration is required")
	}
	units := []struct {
		suffix string
		scale  time.Duration
	}{
		{suffix: "ms", scale: time.Duration(1_000_000)},
		{suffix: "s", scale: time.Duration(1_000_000_000)},
		{suffix: "m", scale: time.Duration(60_000_000_000)},
		{suffix: "h", scale: time.Duration(3_600_000_000_000)},
	}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		raw := strings.TrimSuffix(value, unit.suffix)
		amount, err := strconv.ParseFloat(raw, 64)
		if err != nil || amount <= 0 {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		return time.Duration(amount * float64(unit.scale)), nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	return duration, nil
}

func startNetworkNode(binaryPath string, localNode networkNodeRuntimePlan) (*exec.Cmd, error) {
	if err := os.MkdirAll(localNode.Home, 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(localNode.PIDPath); err == nil {
		return nil, fmt.Errorf("%s already has pid file %s", localNode.ValidatorID, localNode.PIDPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	logFile, err := os.OpenFile(localNode.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()
	command := exec.Command(binaryPath, localNode.Args...)
	command.Stdout = logFile
	command.Stderr = logFile
	configureNetworkChildProcess(command)
	if err := command.Start(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(localNode.PIDPath, []byte(strconv.Itoa(command.Process.Pid)), 0o644); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, err
	}
	return command, nil
}

func networkHealthOK(ctx context.Context, client http.Client, address string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/healthz", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func waitNetworkRPCDown(address string, timeout time.Duration) {
	if address == "" || timeout <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := http.Client{Timeout: 100 * time.Millisecond}
	for {
		if !networkHealthOK(ctx, client, address) {
			time.Sleep(100 * time.Millisecond)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func runNetworkSmokePlan(ctx context.Context, client http.Client, plan networkRuntimePlan, chainID string, txPrefix string) ([]networkSmokeResult, error) {
	if len(plan.Nodes) == 0 {
		return nil, errors.New("network has no nodes")
	}
	nonceTracker := newNetworkNonceTracker()
	for _, localNode := range plan.Nodes {
		if err := waitNetworkHealth(ctx, client, localNode.RPCAddress); err != nil {
			return nil, fmt.Errorf("%s health: %w", localNode.ValidatorID, err)
		}
	}
	if len(plan.Nodes) > 1 {
		for _, localNode := range plan.Nodes {
			if err := waitNetworkPeerCount(ctx, client, localNode.RPCAddress, len(plan.Nodes)-1); err != nil {
				return nil, fmt.Errorf("%s peers: %w", localNode.ValidatorID, err)
			}
		}
	}
	firstNode := plan.Nodes[0]
	statusBefore, err := networkStatus(ctx, client, firstNode.RPCAddress)
	if err != nil {
		return nil, err
	}
	for _, localNode := range plan.Nodes {
		tx, err := signedNetworkPayload(chainID, localNode, txPrefix, nonceTracker.Next(localNode.ValidatorID))
		if err != nil {
			return nil, err
		}
		if err := submitNetworkTx(ctx, client, localNode.RPCAddress, tx); err != nil {
			return nil, fmt.Errorf("%s tx: %w", localNode.ValidatorID, err)
		}
	}
	targetHeight := statusBefore.LatestHeight + 1
	results, err := waitNetworkHeights(ctx, client, plan.Nodes, targetHeight)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func runNetworkChaosPlanExecution(ctx context.Context, writer io.Writer, client http.Client, plan networkRuntimePlan, chainID string, targetIndex int, txPrefix string, restart bool) error {
	if len(plan.Nodes) < 2 {
		return errors.New("chaos run requires at least two validators")
	}
	nonceTracker := newNetworkNonceTracker()
	target := plan.Nodes[targetIndex]
	survivor := networkSurvivorNode(plan, targetIndex)
	for _, localNode := range plan.Nodes {
		if err := waitNetworkHealth(ctx, client, localNode.RPCAddress); err != nil {
			return fmt.Errorf("%s health before chaos: %w", localNode.ValidatorID, err)
		}
	}
	statusBefore, err := networkStatus(ctx, client, survivor.RPCAddress)
	if err != nil {
		return err
	}
	pid, err := readNetworkPID(target.PIDPath)
	if err != nil {
		return fmt.Errorf("%s pid: %w", target.ValidatorID, err)
	}
	if err := stopNetworkPID(pid); err != nil {
		return fmt.Errorf("%s stop: %w", target.ValidatorID, err)
	}
	_ = os.Remove(target.PIDPath)
	fmt.Fprintf(writer, "chaos stopped %s pid=%d\n", target.ValidatorID, pid)
	tx, err := signedNetworkPayload(chainID, survivor, txPrefix, nonceTracker.Next(survivor.ValidatorID))
	if err != nil {
		return fmt.Errorf("%s tx during chaos: %w", survivor.ValidatorID, err)
	}
	if err := submitNetworkTx(ctx, client, survivor.RPCAddress, tx); err != nil {
		return fmt.Errorf("%s tx during chaos: %w", survivor.ValidatorID, err)
	}
	statusAfter, err := waitNetworkHeight(ctx, client, survivor.RPCAddress, statusBefore.LatestHeight+1)
	if err != nil {
		return fmt.Errorf("%s height during chaos: %w", survivor.ValidatorID, err)
	}
	fmt.Fprintf(writer, "chaos survivor %s height_before=%d height_after=%d\n", survivor.ValidatorID, statusBefore.LatestHeight, statusAfter.LatestHeight)
	if !restart {
		fmt.Fprintf(writer, "network chaos ok; %s remains stopped\n", target.ValidatorID)
		return nil
	}
	command, err := startNetworkNode(plan.Binary, target)
	if err != nil {
		return fmt.Errorf("%s restart: %w", target.ValidatorID, err)
	}
	target.command = command
	fmt.Fprintf(writer, "chaos restarted %s\n", target.ValidatorID)
	if err := waitNetworkHealth(ctx, client, target.RPCAddress); err != nil {
		return fmt.Errorf("%s health after restart: %w", target.ValidatorID, err)
	}
	if _, err := waitNetworkHeight(ctx, client, target.RPCAddress, statusAfter.LatestHeight); err != nil {
		return fmt.Errorf("%s catch-up after restart: %w", target.ValidatorID, err)
	}
	fmt.Fprintf(writer, "network chaos ok\n")
	return nil
}

func collectNetworkMetrics(ctx context.Context, client http.Client, plan networkRuntimePlan, evaluate bool) []networkMetricsResponse {
	results := make([]networkMetricsResponse, 0, len(plan.Nodes))
	for _, localNode := range plan.Nodes {
		metrics, err := networkMetrics(ctx, client, localNode.RPCAddress)
		result := networkMetricsResponse{
			ValidatorID: localNode.ValidatorID,
			RPCAddress:  localNode.RPCAddress,
			Metrics:     metrics,
		}
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if evaluate {
			sample, err := ops.SampleFromMetricsSnapshot(nil, metrics, 0)
			if err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}
			report, err := ops.Evaluate(sample, ops.DefaultThresholds())
			if err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}
			result.Report = report
		}
		results = append(results, result)
	}
	return results
}

func runNetworkLongRunEvidence(ctx context.Context, client http.Client, plan networkRuntimePlan, chainID string, duration time.Duration, rate int, txPrefix string) networkLongRunEvidence {
	started := time.Now()
	before := collectNetworkMetrics(ctx, client, plan, false)
	loadCtx, cancel := context.WithTimeout(ctx, duration)
	load := runNetworkLoadPlan(loadCtx, client, plan, chainID, rate, txPrefix)
	cancel()
	after := collectNetworkMetrics(ctx, client, plan, false)
	evidence := networkLongRunEvidence{
		SchemaVersion: "v1",
		OK:            true,
		Home:          plan.Home,
		Validators:    plan.Validators,
		Duration:      duration.String(),
		Rate:          rate,
		StartedAtUnix: started.Unix(),
		EndedAtUnix:   time.Now().Unix(),
		Load: networkLongRunLoadEvidence{
			Submitted: load.Submitted,
			Failed:    load.Failed,
			Duration:  load.Duration.String(),
			PerNode:   load.PerNode,
		},
		Nodes: make([]networkLongRunNodeEvidence, 0, len(plan.Nodes)),
	}
	beforeByValidator := make(map[string]networkMetricsResponse, len(before))
	for _, result := range before {
		beforeByValidator[result.ValidatorID] = result
	}
	afterByValidator := make(map[string]networkMetricsResponse, len(after))
	for _, result := range after {
		afterByValidator[result.ValidatorID] = result
	}
	for _, localNode := range plan.Nodes {
		nodeEvidence := networkLongRunNodeEvidence{
			ValidatorID: localNode.ValidatorID,
			RPCAddress:  localNode.RPCAddress,
		}
		beforeResult := beforeByValidator[localNode.ValidatorID]
		afterResult := afterByValidator[localNode.ValidatorID]
		if beforeResult.Error != "" {
			nodeEvidence.Error = "before metrics: " + beforeResult.Error
			evidence.OK = false
			evidence.Nodes = append(evidence.Nodes, nodeEvidence)
			continue
		}
		if afterResult.Error != "" {
			nodeEvidence.Error = "after metrics: " + afterResult.Error
			evidence.OK = false
			evidence.Nodes = append(evidence.Nodes, nodeEvidence)
			continue
		}
		nodeEvidence.Before = beforeResult.Metrics
		nodeEvidence.After = afterResult.Metrics
		sample, err := ops.SampleFromMetricsSnapshot(&beforeResult.Metrics, afterResult.Metrics, duration)
		if err != nil {
			nodeEvidence.Error = err.Error()
			evidence.OK = false
			evidence.Nodes = append(evidence.Nodes, nodeEvidence)
			continue
		}
		report, err := ops.Evaluate(sample, ops.DefaultThresholds())
		if err != nil {
			nodeEvidence.Error = err.Error()
			evidence.OK = false
			evidence.Nodes = append(evidence.Nodes, nodeEvidence)
			continue
		}
		nodeEvidence.Sample = sample
		nodeEvidence.Report = report
		if !report.OK {
			evidence.OK = false
		}
		evidence.Nodes = append(evidence.Nodes, nodeEvidence)
	}
	if load.Failed > 0 {
		evidence.OK = false
	}
	return evidence
}

func runNetworkLoadPlan(ctx context.Context, client http.Client, plan networkRuntimePlan, chainID string, rate int, txPrefix string) networkLoadResult {
	started := time.Now()
	if len(plan.Nodes) == 0 {
		return networkLoadResult{Duration: time.Since(started), Failed: 1}
	}
	nonceTracker := newNetworkNonceTracker()
	interval := networkSecond / time.Duration(rate)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	result := networkLoadResult{PerNode: make(map[string]uint64, len(plan.Nodes))}
	for {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(started)
			return result
		case <-ticker.C:
			sequence := result.Submitted + result.Failed + 1
			target := plan.Nodes[int((sequence-1)%uint64(len(plan.Nodes)))]
			payload, err := signedNetworkPayload(chainID, target, txPrefix, nonceTracker.Next(target.ValidatorID))
			if err != nil {
				result.Failed++
				continue
			}
			if err := submitNetworkTx(ctx, client, target.RPCAddress, payload); err != nil {
				result.Failed++
				continue
			}
			result.Submitted++
			result.PerNode[target.ValidatorID]++
		}
	}
}

func networkSurvivorNode(plan networkRuntimePlan, targetIndex int) networkNodeRuntimePlan {
	if targetIndex != 0 {
		return plan.Nodes[0]
	}
	return plan.Nodes[1]
}

func signedNetworkPayload(chainID string, localNode networkNodeRuntimePlan, txPrefix string, sequence uint64) (types.Tx, error) {
	keyDocument, err := loadNetworkTxKeyDocument(localNode)
	if err != nil {
		return nil, err
	}
	signer, err := keyDocument.SignerWithPassphrase(resolvePassphrase(""))
	if err != nil {
		return nil, err
	}
	account, err := keyDocumentAccountAddress(keyDocument)
	if err != nil {
		return nil, err
	}
	payload := networkLoadPayloadForAccount(txPrefix, sequence, localNode.ValidatorID, string(account))
	if vexoapp.IsSignedTx(payload) {
		return payload, nil
	}
	return vexoapp.SignTx(chainID, payload, signer)
}

func loadNetworkTxKeyDocument(localNode networkNodeRuntimePlan) (vexocrypto.KeyDocument, error) {
	nodeKeyDocument, err := vexocrypto.LoadKeyDocument(filepath.Join(localNode.Home, nodeKeyFileName))
	if err == nil {
		return nodeKeyDocument, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return vexocrypto.KeyDocument{}, err
	}
	return vexocrypto.LoadKeyDocument(filepath.Join(localNode.Home, keyFileName))
}

func networkPlanChainID(plan networkRuntimePlan) (string, error) {
	if len(plan.Nodes) == 0 {
		return "", errors.New("network has no nodes")
	}
	document, err := readConfigDocument(filepath.Join(plan.Nodes[0].Home, configFileName))
	if err != nil {
		return "", err
	}
	if document.ChainID == "" {
		return defaultChainID, nil
	}
	return document.ChainID, nil
}

func networkLoadPayload(txPrefix string, sequence uint64, validatorID string) types.Tx {
	return networkLoadPayloadForAccount(txPrefix, sequence, validatorID, validatorID)
}

func networkLoadPayloadForAccount(txPrefix string, sequence uint64, validatorID string, account string) types.Tx {
	txPrefix = strings.ReplaceAll(txPrefix, "{validator}", validatorID)
	txPrefix = strings.ReplaceAll(txPrefix, "{node}", validatorID)
	txPrefix = strings.ReplaceAll(txPrefix, "{account}", account)
	if strings.Contains(txPrefix, "nonce") && !strings.Contains(txPrefix, "nonce=") {
		return types.Tx(fmt.Sprintf("%s=%d", txPrefix, sequence))
	}
	return types.Tx(fmt.Sprintf("%s:%d", txPrefix, sequence))
}

func estimatedNetworkTransactions(duration time.Duration, rate int) uint64 {
	if duration <= 0 || rate <= 0 {
		return 0
	}
	return uint64(duration/networkSecond) * uint64(rate)
}

func waitNetworkHealth(ctx context.Context, client http.Client, address string) error {
	for {
		if networkHealthOK(ctx, client, address) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitNetworkHeight(ctx context.Context, client http.Client, address string, targetHeight uint64) (networkStatusResponse, error) {
	for {
		status, err := networkStatus(ctx, client, address)
		if err == nil && status.LatestHeight >= targetHeight {
			return status, nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return networkStatusResponse{}, err
			}
			return networkStatusResponse{}, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitNetworkPeerCount(ctx context.Context, client http.Client, address string, minPeers int) error {
	var lastStatus networkStatusResponse
	for {
		status, err := networkStatus(ctx, client, address)
		if err == nil {
			lastStatus = status
		}
		if err == nil && effectiveNetworkActivePeerCount(status) >= minPeers {
			return nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return err
			}
			return fmt.Errorf("active peer count below target: active=%d configured=%d scored=%d compatible=%d target=%d", lastStatus.ActivePeerCount, lastStatus.ConfiguredPeerCount, lastStatus.ScoredPeerCount, lastStatus.PeerCount, minPeers)
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func effectiveNetworkActivePeerCount(status networkStatusResponse) int {
	if status.ActivePeerCount > 0 || status.ConfiguredPeerCount > 0 || status.ScoredPeerCount > 0 {
		return status.ActivePeerCount
	}
	return status.PeerCount
}

func waitNetworkHeights(ctx context.Context, client http.Client, nodes []networkNodeRuntimePlan, targetHeight uint64) ([]networkSmokeResult, error) {
	pending := make(map[string]networkNodeRuntimePlan, len(nodes))
	for _, localNode := range nodes {
		pending[localNode.ValidatorID] = localNode
	}
	results := make([]networkSmokeResult, 0, len(nodes))
	var lastErr error
	for len(pending) > 0 {
		for validatorID, localNode := range pending {
			status, err := networkStatus(ctx, client, localNode.RPCAddress)
			if err != nil {
				lastErr = fmt.Errorf("%s status: %w", validatorID, err)
				continue
			}
			if status.LatestHeight < targetHeight {
				lastErr = fmt.Errorf("%s height %d below target %d", validatorID, status.LatestHeight, targetHeight)
				continue
			}
			results = append(results, networkSmokeResult{
				ValidatorID: validatorID,
				RPCAddress:  localNode.RPCAddress,
				Healthy:     true,
				Height:      status.LatestHeight,
			})
			delete(pending, validatorID)
		}
		if len(pending) == 0 {
			return results, nil
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return results, nil
}

func networkStatus(ctx context.Context, client http.Client, address string) (networkStatusResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/v1/status", nil)
	if err != nil {
		return networkStatusResponse{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return networkStatusResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return networkStatusResponse{}, fmt.Errorf("status returned HTTP %d", response.StatusCode)
	}
	var status networkStatusResponse
	return status, json.NewDecoder(response.Body).Decode(&status)
}

func writeNetworkLogTails(writer io.Writer, plan networkRuntimePlan, maxBytes int) {
	if maxBytes <= 0 {
		return
	}
	for _, localNode := range plan.Nodes {
		data, err := os.ReadFile(localNode.LogPath)
		if err != nil {
			fmt.Fprintf(writer, "--- %s log unavailable: %v\n", localNode.ValidatorID, err)
			continue
		}
		if len(data) > maxBytes {
			data = data[len(data)-maxBytes:]
			if newline := bytes.IndexByte(data, '\n'); newline >= 0 && newline+1 < len(data) {
				data = data[newline+1:]
			}
		}
		fmt.Fprintf(writer, "--- %s log tail (%s)\n%s", localNode.ValidatorID, localNode.LogPath, data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			fmt.Fprintln(writer)
		}
	}
}

func networkMetrics(ctx context.Context, client http.Client, address string) (ops.MetricsSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/v1/metrics", nil)
	if err != nil {
		return ops.MetricsSnapshot{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return ops.MetricsSnapshot{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ops.MetricsSnapshot{}, fmt.Errorf("metrics returned HTTP %d", response.StatusCode)
	}
	var metrics ops.MetricsSnapshot
	return metrics, json.NewDecoder(response.Body).Decode(&metrics)
}

func submitNetworkTx(ctx context.Context, client http.Client, address string, tx []byte) error {
	body, err := json.Marshal(map[string]string{"tx": base64.StdEncoding.EncodeToString(tx)})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/v1/tx", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("tx returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func readNetworkPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func stopNetworkPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(os.Interrupt); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return process.Kill()
	}
	return nil
}

func stopNetworkCommand(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	process := command.Process
	if err := process.Signal(os.Interrupt); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		_ = process.Kill()
	}
	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()
	select {
	case err := <-done:
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return err
	case <-time.After(3 * time.Second):
		_ = process.Kill()
		<-done
		return nil
	}
}

func joinArgs(args []string) string {
	joined := ""
	for index, arg := range args {
		if arg == "" {
			continue
		}
		if index > 0 && joined != "" {
			joined += " "
		}
		joined += arg
	}
	return joined
}

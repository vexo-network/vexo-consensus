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
	"time"

	"github.com/vexo-network/vexo-consensus/ops"
	"github.com/vexo-network/vexo-consensus/types"
)

const networkPIDFileName = "vexod.pid"
const networkSecond = time.Duration(1_000_000_000)

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
	case "metrics":
		return runNetworkMetrics(context.Background(), writer, args[1:])
	case "chaos":
		return runNetworkChaos(context.Background(), writer, args[1:])
	case "chaos-plan":
		return runNetworkChaosPlan(writer, args[1:])
	case "longrun-plan":
		return runNetworkLongRunPlan(writer, args[1:])
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
	txPrefix := flags.String("tx-prefix", "bank:send:load-src:load-dst:1:fee=1:gas=1000:signer=load-src:nonce", "transaction payload prefix; nonce suffix is appended for realistic load")
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
	result := runNetworkLoadPlan(loadCtx, client, plan, *rate, *txPrefix)
	fmt.Fprintf(writer, "network load complete\n")
	fmt.Fprintf(writer, "submitted: %d\n", result.Submitted)
	fmt.Fprintf(writer, "failed: %d\n", result.Failed)
	fmt.Fprintf(writer, "duration: %s\n", result.Duration)
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

type networkSmokeResult struct {
	ValidatorID string
	RPCAddress  string
	Healthy     bool
	Height      uint64
}

type networkStatusResponse struct {
	LatestHeight uint64 `json:"latest_height"`
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
	binaryPath := flags.String("binary", "", "vexod binary path; defaults to current executable")
	timeoutValue := flags.String("timeout", "20s", "startup and smoke test timeout")
	tx := flags.String("tx", "bank:mint:smoke:1", "transaction payload to submit")
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
	networkFiles, err := writeNetworkFilesWithPorts(*home, *chainID, *validators, *overwrite, *p2pBasePort, *rpcBasePort)
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
	results, err := runNetworkSmokePlan(smokeCtx, client, plan, []byte(*tx))
	if err != nil {
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
	tx := flags.String("tx", "bank:mint:smoke:1", "transaction payload to submit")
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
	results, err := runNetworkSmokePlan(ctx, client, plan, []byte(*tx))
	if err != nil {
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
	overwrite := flags.Bool("overwrite", false, "overwrite existing network files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	initArgs := []string{
		"--home", *home,
		"--chain-id", *chainID,
		"--validators", strconv.Itoa(*validators),
		"--p2p-base-port", strconv.Itoa(*p2pBasePort),
		"--rpc-base-port", strconv.Itoa(*rpcBasePort),
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
	tx := flags.String("tx", "bank:mint:chaos:1", "transaction payload to submit while one validator is stopped")
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
	return runNetworkChaosPlanExecution(chaosCtx, writer, client, plan, targetIndex, []byte(*tx), *restart)
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
	for _, localNode := range plan.Nodes {
		if err := startNetworkNode(plan.Binary, localNode); err != nil {
			_ = stopNetworkPlan(io.Discard, started)
			return err
		}
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
		if err := stopNetworkPID(pid); err != nil {
			return err
		}
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
		args := []string{
			"start",
			"--home", nodeHome,
			"--run",
			"--rpc-address", networkRPCAddressWithBasePort(index, rpcBasePort),
			"--p2p-listen", networkP2PAddressWithBasePort(index, p2pBasePort),
		}
		plan.Nodes = append(plan.Nodes, networkNodeRuntimePlan{
			ValidatorID: validatorID,
			Home:        nodeHome,
			RPCAddress:  networkRPCAddressWithBasePort(index, rpcBasePort),
			P2PAddress:  networkP2PAddressWithBasePort(index, p2pBasePort),
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
	initCommand := fmt.Sprintf("network init --home %s --chain-id %s --validators %d --p2p-base-port %d --rpc-base-port %d", plan.Home, chainID, plan.Validators, plan.P2PBasePort, plan.RPCBasePort)
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

func startNetworkNode(binaryPath string, localNode networkNodeRuntimePlan) error {
	if err := os.MkdirAll(localNode.Home, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(localNode.PIDPath); err == nil {
		return fmt.Errorf("%s already has pid file %s", localNode.ValidatorID, localNode.PIDPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	logFile, err := os.OpenFile(localNode.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	command := exec.Command(binaryPath, localNode.Args...)
	command.Stdout = logFile
	command.Stderr = logFile
	configureNetworkChildProcess(command)
	if err := command.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(localNode.PIDPath, []byte(strconv.Itoa(command.Process.Pid)), 0o644); err != nil {
		_ = command.Process.Kill()
		return err
	}
	return command.Process.Release()
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

func runNetworkSmokePlan(ctx context.Context, client http.Client, plan networkRuntimePlan, tx []byte) ([]networkSmokeResult, error) {
	if len(plan.Nodes) == 0 {
		return nil, errors.New("network has no nodes")
	}
	for _, localNode := range plan.Nodes {
		if err := waitNetworkHealth(ctx, client, localNode.RPCAddress); err != nil {
			return nil, fmt.Errorf("%s health: %w", localNode.ValidatorID, err)
		}
	}
	firstNode := plan.Nodes[0]
	statusBefore, err := networkStatus(ctx, client, firstNode.RPCAddress)
	if err != nil {
		return nil, err
	}
	if err := submitNetworkTx(ctx, client, firstNode.RPCAddress, tx); err != nil {
		return nil, err
	}
	targetHeight := statusBefore.LatestHeight + 1
	results := make([]networkSmokeResult, 0, len(plan.Nodes))
	for _, localNode := range plan.Nodes {
		status, err := waitNetworkHeight(ctx, client, localNode.RPCAddress, targetHeight)
		if err != nil {
			return nil, fmt.Errorf("%s height: %w", localNode.ValidatorID, err)
		}
		results = append(results, networkSmokeResult{
			ValidatorID: localNode.ValidatorID,
			RPCAddress:  localNode.RPCAddress,
			Healthy:     true,
			Height:      status.LatestHeight,
		})
	}
	return results, nil
}

func runNetworkChaosPlanExecution(ctx context.Context, writer io.Writer, client http.Client, plan networkRuntimePlan, targetIndex int, tx []byte, restart bool) error {
	if len(plan.Nodes) < 2 {
		return errors.New("chaos run requires at least two validators")
	}
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
	if err := startNetworkNode(plan.Binary, target); err != nil {
		return fmt.Errorf("%s restart: %w", target.ValidatorID, err)
	}
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

func runNetworkLoadPlan(ctx context.Context, client http.Client, plan networkRuntimePlan, rate int, txPrefix string) networkLoadResult {
	started := time.Now()
	if len(plan.Nodes) == 0 {
		return networkLoadResult{Duration: time.Since(started), Failed: 1}
	}
	interval := networkSecond / time.Duration(rate)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var result networkLoadResult
	for {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(started)
			return result
		case <-ticker.C:
			payload := networkLoadPayload(txPrefix, result.Submitted+result.Failed+1)
			if err := submitNetworkTx(ctx, client, plan.Nodes[0].RPCAddress, payload); err != nil {
				result.Failed++
				continue
			}
			result.Submitted++
		}
	}
}

func networkSurvivorNode(plan networkRuntimePlan, targetIndex int) networkNodeRuntimePlan {
	if targetIndex != 0 {
		return plan.Nodes[0]
	}
	return plan.Nodes[1]
}

func networkLoadPayload(txPrefix string, sequence uint64) types.Tx {
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
		return fmt.Errorf("tx returned HTTP %d", response.StatusCode)
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

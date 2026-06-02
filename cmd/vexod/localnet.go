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
	"time"
)

const localnetPIDFileName = "vexod.pid"

type localnetRuntimePlan struct {
	Home       string
	Binary     string
	Validators int
	Nodes      []localnetNodeRuntimePlan
}

type localnetNodeRuntimePlan struct {
	ValidatorID string
	Home        string
	RPCAddress  string
	P2PAddress  string
	PIDPath     string
	Args        []string
}

func runLocalnet(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("localnet subcommand is required")
	}
	switch args[0] {
	case "init":
		return runLocalnetInit(writer, args[1:])
	case "start":
		return runLocalnetStart(writer, args[1:])
	case "status":
		return runLocalnetStatus(context.Background(), writer, args[1:])
	case "smoke":
		return runLocalnetSmoke(context.Background(), writer, args[1:])
	case "stop":
		return runLocalnetStop(writer, args[1:])
	default:
		return fmt.Errorf("unknown localnet subcommand %q", args[0])
	}
}

type localnetSmokeResult struct {
	ValidatorID string
	RPCAddress  string
	Healthy     bool
	Height      uint64
}

type localnetStatusResponse struct {
	LatestHeight uint64 `json:"latest_height"`
}

func runLocalnetSmoke(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("localnet smoke", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-localnet", "localnet home directory")
	validators := flags.Int("validators", 4, "validator count")
	timeout := flags.Duration("timeout", 10*time.Second, "smoke test timeout")
	tx := flags.String("tx", "bank:smoke", "transaction payload to submit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := buildLocalnetRuntimePlan(*home, *validators, "")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	client := http.Client{Timeout: *timeout}
	results, err := runLocalnetSmokePlan(ctx, client, plan, []byte(*tx))
	if err != nil {
		return err
	}
	for _, result := range results {
		fmt.Fprintf(writer, "%s rpc=%s healthy=%t height=%d\n", result.ValidatorID, result.RPCAddress, result.Healthy, result.Height)
	}
	fmt.Fprintf(writer, "localnet smoke ok\n")
	return nil
}

func runLocalnetInit(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("localnet init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-localnet", "localnet home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validators := flags.Int("validators", 4, "validator count")
	overwrite := flags.Bool("overwrite", false, "overwrite existing localnet files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	initArgs := []string{
		"--home", *home,
		"--chain-id", *chainID,
		"--validators", strconv.Itoa(*validators),
	}
	if *overwrite {
		initArgs = append(initArgs, "--overwrite")
	}
	return runInit(writer, initArgs)
}

func runLocalnetStart(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("localnet start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-localnet", "localnet home directory")
	validators := flags.Int("validators", 4, "validator count")
	binaryPath := flags.String("binary", "", "vexod binary path; defaults to current executable")
	dryRun := flags.Bool("dry-run", false, "print node start commands without spawning processes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := buildLocalnetRuntimePlan(*home, *validators, *binaryPath)
	if err != nil {
		return err
	}
	if *dryRun {
		writeLocalnetPlan(writer, plan)
		return nil
	}
	for _, localNode := range plan.Nodes {
		if err := startLocalnetNode(plan.Binary, localNode); err != nil {
			return err
		}
		fmt.Fprintf(writer, "started %s pid=%s rpc=%s p2p=%s\n", localNode.ValidatorID, localNode.PIDPath, localNode.RPCAddress, localNode.P2PAddress)
	}
	return nil
}

func runLocalnetStatus(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("localnet status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-localnet", "localnet home directory")
	validators := flags.Int("validators", 4, "validator count")
	timeout := flags.Duration("timeout", 2*time.Second, "per-node health check timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := buildLocalnetRuntimePlan(*home, *validators, "")
	if err != nil {
		return err
	}
	client := http.Client{Timeout: *timeout}
	for _, localNode := range plan.Nodes {
		ok := localnetHealthOK(ctx, client, localNode.RPCAddress)
		fmt.Fprintf(writer, "%s rpc=%s healthy=%t\n", localNode.ValidatorID, localNode.RPCAddress, ok)
	}
	return nil
}

func runLocalnetStop(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("localnet stop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", ".vexo-localnet", "localnet home directory")
	validators := flags.Int("validators", 4, "validator count")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := buildLocalnetRuntimePlan(*home, *validators, "")
	if err != nil {
		return err
	}
	for _, localNode := range plan.Nodes {
		pid, err := readLocalnetPID(localNode.PIDPath)
		if err != nil {
			fmt.Fprintf(writer, "stopped %s pid=missing\n", localNode.ValidatorID)
			continue
		}
		if err := stopLocalnetPID(pid); err != nil {
			return err
		}
		_ = os.Remove(localNode.PIDPath)
		fmt.Fprintf(writer, "stopped %s pid=%d\n", localNode.ValidatorID, pid)
	}
	return nil
}

func buildLocalnetRuntimePlan(home string, validators int, binaryPath string) (localnetRuntimePlan, error) {
	if validators <= 0 {
		return localnetRuntimePlan{}, fmt.Errorf("validators must be positive")
	}
	if home == "" {
		home = ".vexo-localnet"
	}
	if binaryPath == "" {
		executable, err := os.Executable()
		if err != nil {
			return localnetRuntimePlan{}, err
		}
		binaryPath = executable
	}
	plan := localnetRuntimePlan{
		Home:       home,
		Binary:     binaryPath,
		Validators: validators,
		Nodes:      make([]localnetNodeRuntimePlan, 0, validators),
	}
	for index := 1; index <= validators; index++ {
		validatorID := localnetValidatorID(index)
		nodeHome := filepath.Join(home, validatorID)
		args := []string{
			"start",
			"--home", nodeHome,
			"--run",
			"--rpc-address", localnetRPCAddress(index),
			"--p2p-listen", localnetP2PAddress(index),
		}
		plan.Nodes = append(plan.Nodes, localnetNodeRuntimePlan{
			ValidatorID: validatorID,
			Home:        nodeHome,
			RPCAddress:  localnetRPCAddress(index),
			P2PAddress:  localnetP2PAddress(index),
			PIDPath:     filepath.Join(nodeHome, localnetPIDFileName),
			Args:        args,
		})
	}
	return plan, nil
}

func writeLocalnetPlan(writer io.Writer, plan localnetRuntimePlan) {
	fmt.Fprintf(writer, "localnet start plan\n")
	fmt.Fprintf(writer, "home: %s\n", plan.Home)
	fmt.Fprintf(writer, "validators: %d\n", plan.Validators)
	for _, localNode := range plan.Nodes {
		fmt.Fprintf(writer, "%s: %s %s\n", localNode.ValidatorID, plan.Binary, joinArgs(localNode.Args))
	}
}

func startLocalnetNode(binaryPath string, localNode localnetNodeRuntimePlan) error {
	if err := os.MkdirAll(localNode.Home, 0o755); err != nil {
		return err
	}
	command := exec.Command(binaryPath, localNode.Args...)
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(localNode.PIDPath, []byte(strconv.Itoa(command.Process.Pid)), 0o644); err != nil {
		_ = command.Process.Kill()
		return err
	}
	return command.Process.Release()
}

func localnetHealthOK(ctx context.Context, client http.Client, address string) bool {
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

func runLocalnetSmokePlan(ctx context.Context, client http.Client, plan localnetRuntimePlan, tx []byte) ([]localnetSmokeResult, error) {
	if len(plan.Nodes) == 0 {
		return nil, errors.New("localnet has no nodes")
	}
	for _, localNode := range plan.Nodes {
		if err := waitLocalnetHealth(ctx, client, localNode.RPCAddress); err != nil {
			return nil, fmt.Errorf("%s health: %w", localNode.ValidatorID, err)
		}
	}
	firstNode := plan.Nodes[0]
	statusBefore, err := localnetStatus(ctx, client, firstNode.RPCAddress)
	if err != nil {
		return nil, err
	}
	if err := submitLocalnetTx(ctx, client, firstNode.RPCAddress, tx); err != nil {
		return nil, err
	}
	targetHeight := statusBefore.LatestHeight + 1
	results := make([]localnetSmokeResult, 0, len(plan.Nodes))
	for _, localNode := range plan.Nodes {
		status, err := waitLocalnetHeight(ctx, client, localNode.RPCAddress, targetHeight)
		if err != nil {
			return nil, fmt.Errorf("%s height: %w", localNode.ValidatorID, err)
		}
		results = append(results, localnetSmokeResult{
			ValidatorID: localNode.ValidatorID,
			RPCAddress:  localNode.RPCAddress,
			Healthy:     true,
			Height:      status.LatestHeight,
		})
	}
	return results, nil
}

func waitLocalnetHealth(ctx context.Context, client http.Client, address string) error {
	for {
		if localnetHealthOK(ctx, client, address) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func waitLocalnetHeight(ctx context.Context, client http.Client, address string, targetHeight uint64) (localnetStatusResponse, error) {
	for {
		status, err := localnetStatus(ctx, client, address)
		if err == nil && status.LatestHeight >= targetHeight {
			return status, nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return localnetStatusResponse{}, err
			}
			return localnetStatusResponse{}, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func localnetStatus(ctx context.Context, client http.Client, address string) (localnetStatusResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/status", nil)
	if err != nil {
		return localnetStatusResponse{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return localnetStatusResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return localnetStatusResponse{}, fmt.Errorf("status returned HTTP %d", response.StatusCode)
	}
	var status localnetStatusResponse
	return status, json.NewDecoder(response.Body).Decode(&status)
}

func submitLocalnetTx(ctx context.Context, client http.Client, address string, tx []byte) error {
	body, err := json.Marshal(map[string]string{"tx": base64.StdEncoding.EncodeToString(tx)})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/tx", bytes.NewReader(body))
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

func readLocalnetPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func stopLocalnetPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(os.Interrupt); err != nil {
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

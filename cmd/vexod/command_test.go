package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func TestRunCommandHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		var stdout bytes.Buffer
		if err := runCommand(&stdout, &bytes.Buffer{}, args); err != nil {
			t.Fatal(err)
		}
		output := stdout.String()
		for _, expected := range []string{"Usage:", "init", "config paths", "start", "localnet", "version", "Module Commands:", "bank tx mint", "bank query balance"} {
			if !strings.Contains(output, expected) {
				t.Fatalf("expected help output to contain %q, got:\n%s", expected, output)
			}
		}
	}

	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "vexod "+version {
		t.Fatalf("unexpected version output: %q", stdout.String())
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

func TestRunLocalnetInitAndStartDryRun(t *testing.T) {
	home := t.TempDir()
	var initOutput bytes.Buffer
	if err := runCommand(&initOutput, &bytes.Buffer{}, []string{"localnet", "init", "--home", home, "--chain-id", "vexo-test", "--validators", "3"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(initOutput.String(), "initialized vexo localnet") || !strings.Contains(initOutput.String(), "validators: 3") {
		t.Fatalf("unexpected localnet init output:\n%s", initOutput.String())
	}

	var startOutput bytes.Buffer
	if err := runCommand(&startOutput, &bytes.Buffer{}, []string{"localnet", "start", "--home", home, "--validators", "3", "--binary", "/bin/vexod", "--p2p-base-port", "27656", "--rpc-base-port", "27657", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	output := startOutput.String()
	for _, expected := range []string{"localnet start plan", "validator-1", "validator-2", "validator-3", "--rpc-address 127.0.0.1:27657", "--p2p-listen 127.0.0.1:27656"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected dry-run output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunLocalnetUpDryRun(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	err := runCommand(&output, &bytes.Buffer{}, []string{
		"localnet", "up",
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
		"localnet up plan",
		"chain-id: vexo-test",
		"validators: 2",
		"p2p-base-port: 28656",
		"rpc-base-port: 28657",
		"localnet init",
		"--overwrite",
		"localnet start",
		"localnet smoke",
		"localnet stop",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected localnet up dry-run output to contain %q, got:\n%s", expected, output.String())
		}
	}
}

func TestParseLocalnetDurationUsesHumanUnits(t *testing.T) {
	for _, testCase := range []struct {
		value    string
		expected time.Duration
	}{
		{value: "250ms", expected: 250 * time.Duration(1_000_000)},
		{value: "3s", expected: 3 * time.Duration(1_000_000_000)},
		{value: "2m", expected: 2 * time.Duration(60_000_000_000)},
	} {
		actual, err := parseLocalnetDuration(testCase.value)
		if err != nil {
			t.Fatalf("expected %s to parse: %v", testCase.value, err)
		}
		if actual != testCase.expected {
			t.Fatalf("expected %s to parse as %s, got %s", testCase.value, testCase.expected, actual)
		}
	}
	if _, err := parseLocalnetDuration("0s"); err == nil {
		t.Fatal("expected zero duration to fail")
	}
}

func TestRunInitWritesLocalnetFilesWithCustomPorts(t *testing.T) {
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

func TestRunLocalnetUpDryRunCanKeepRunning(t *testing.T) {
	var output bytes.Buffer
	err := runCommand(&output, &bytes.Buffer{}, []string{
		"localnet", "up",
		"--home", t.TempDir(),
		"--validators", "1",
		"--binary", "/bin/vexod",
		"--keep-running",
		"--dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "keep nodes running") || strings.Contains(output.String(), "localnet stop") {
		t.Fatalf("expected keep-running dry-run to skip stop, got:\n%s", output.String())
	}
}

func TestLocalnetRuntimePlanAndPIDHelpers(t *testing.T) {
	home := t.TempDir()
	plan, err := buildLocalnetRuntimePlan(home, 2, "/bin/vexod")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Binary != "/bin/vexod" || len(plan.Nodes) != 2 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Nodes[0].ValidatorID != "validator-1" || plan.Nodes[0].RPCAddress != localnetRPCAddress(1) || plan.Nodes[1].P2PAddress != localnetP2PAddress(2) {
		t.Fatalf("unexpected nodes: %+v", plan.Nodes)
	}
	if plan.Nodes[0].LogPath != filepath.Join(home, "validator-1", "vexod.log") {
		t.Fatalf("unexpected log path: %s", plan.Nodes[0].LogPath)
	}
	if _, err := buildLocalnetRuntimePlan(home, 0, "/bin/vexod"); err == nil {
		t.Fatal("expected invalid validator count")
	}
	pidPath := filepath.Join(home, localnetPIDFileName)
	if err := os.WriteFile(pidPath, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, err := readLocalnetPID(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 12345 {
		t.Fatalf("expected pid 12345, got %d", pid)
	}
}

func TestLocalnetRuntimePlanWithCustomPorts(t *testing.T) {
	home := t.TempDir()
	plan, err := buildLocalnetRuntimePlanWithPorts(home, 2, "/bin/vexod", 27656, 27657)
	if err != nil {
		t.Fatal(err)
	}
	if plan.P2PBasePort != 27656 || plan.RPCBasePort != 27657 {
		t.Fatalf("unexpected base ports: %+v", plan)
	}
	if plan.Nodes[0].P2PAddress != "127.0.0.1:27656" || plan.Nodes[1].RPCAddress != "127.0.0.1:27667" {
		t.Fatalf("unexpected custom addresses: %+v", plan.Nodes)
	}
	if _, err := buildLocalnetRuntimePlanWithPorts(home, 2, "/bin/vexod", 0, 27657); err == nil {
		t.Fatal("expected invalid custom port")
	}
}

func TestStartLocalnetNodeRefusesExistingPIDFile(t *testing.T) {
	plan, err := buildLocalnetRuntimePlan(t.TempDir(), 1, "/bin/vexod")
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
	err = startLocalnetNode("/bin/vexod", localNode)
	if err == nil || !strings.Contains(err.Error(), "already has pid file") {
		t.Fatalf("expected existing pid file error, got %v", err)
	}
}

func TestLocalnetHealthOK(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/healthz" {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}
	if !localnetHealthOK(context.Background(), client, "127.0.0.1:26657") {
		t.Fatal("expected localnet health ok")
	}
	failingClient := http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	if localnetHealthOK(context.Background(), failingClient, "127.0.0.1:26657") {
		t.Fatal("expected unreachable localnet health to fail")
	}
}

func TestRunLocalnetSmokePlanSubmitsTxAndWaitsForHeight(t *testing.T) {
	plan, err := buildLocalnetRuntimePlan(t.TempDir(), 2, "/bin/vexod")
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
		case request.Method == http.MethodGet && request.URL.Path == "/status":
			return jsonHTTPResponse(http.StatusOK, `{"chain_id":"vexo-test","running":true,"latest_height":`+strconv.FormatUint(heights[address], 10)+`}`), nil
		case request.Method == http.MethodPost && request.URL.Path == "/tx":
			txSubmitted = true
			heights[plan.Nodes[0].RPCAddress] = 8
			heights[plan.Nodes[1].RPCAddress] = 8
			return jsonHTTPResponse(http.StatusAccepted, `{"accepted":true}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}
	results, err := runLocalnetSmokePlan(context.Background(), client, plan, []byte("bank:smoke"))
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

func TestBuildStartNodeLoadsValidatorSigner(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	inputs, err := loadStartInputs(home, "", "", "", false)
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

func TestRunStartRunStartsAndStopsNode(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	output := &rpcHealthCheckWriter{
		cancel: cancel,
		client: http.Client{Timeout: 5 * time.Second},
	}
	if err := runStartWithContext(ctx, output, []string{"--home", home, "--run", "--rpc-address", "127.0.0.1:0", "--p2p-listen", "127.0.0.1:0"}); err != nil {
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
}

func TestBuildRuntimeNodeConfiguresGRPCTransport(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	startNode, wire, err := buildRuntimeNode(inputs, startRuntimeConfig{
		P2PEnabled:       true,
		P2PListenAddress: "127.0.0.1:0",
		P2PNetworkID:     "localnet",
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

func TestBuildRuntimeNodeDerivesLocalnetPeers(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "3"}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(filepath.Join(home, "validator-2"), "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	runtimeConfig := applyLocalnetRuntimeDefaults(inputs, startRuntimeConfig{P2PEnabled: true, RPCEnabled: true})
	if runtimeConfig.P2PListenAddress != localnetP2PAddress(2) {
		t.Fatalf("expected validator-2 p2p address, got %s", runtimeConfig.P2PListenAddress)
	}
	if runtimeConfig.RPCAddress != localnetRPCAddress(2) {
		t.Fatalf("expected validator-2 rpc address, got %s", runtimeConfig.RPCAddress)
	}
	if len(runtimeConfig.P2PPeers) != 2 || runtimeConfig.P2PPeers["validator-1"] != localnetP2PAddress(1) || runtimeConfig.P2PPeers["validator-3"] != localnetP2PAddress(3) {
		t.Fatalf("unexpected derived peers: %+v", runtimeConfig.P2PPeers)
	}

	startNode, wire, err := buildRuntimeNode(inputs, startRuntimeConfig{P2PEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if startNode == nil || wire == nil {
		t.Fatal("expected localnet node and grpc wire")
	}
	if wire.Address() != localnetP2PAddress(2) {
		t.Fatalf("expected localnet p2p listen address, got %s", wire.Address())
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

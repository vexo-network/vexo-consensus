package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNetworkUpBuiltBinaryE2E(t *testing.T) {
	if os.Getenv("VEXO_NETWORK_E2E") != "1" {
		t.Skip("set VEXO_NETWORK_E2E=1 to run built-binary network e2e")
	}

	binaryPath := networkE2EBinary(t)

	p2pBasePort, rpcBasePort := reserveNetworkE2EPorts(t)
	home := networkE2ETempHome(t)
	run := exec.Command(binaryPath,
		"network", "up",
		"--home", home,
		"--validators", "4",
		"--p2p-base-port", strconv.Itoa(p2pBasePort),
		"--rpc-base-port", strconv.Itoa(rpcBasePort),
		"--timeout", "60s",
		"--overwrite",
		"--keep-running",
	)
	stopped := false
	defer func() {
		if stopped {
			return
		}
		_ = stopNetworkE2E(binaryPath, home, p2pBasePort, rpcBasePort)
	}()
	var output bytes.Buffer
	run.Stdout = &output
	run.Stderr = &output
	if err := run.Run(); err != nil {
		t.Fatalf("network up failed: %v\n%s", err, output.String())
	}
	for validatorIndex := 1; validatorIndex <= 4; validatorIndex++ {
		expected := fmt.Sprintf("validator-%d rpc=127.0.0.1:%d healthy=true height=", validatorIndex, rpcBasePort+(validatorIndex-1)*10)
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output.String())
		}
	}
	if !strings.Contains(output.String(), "network up ok; nodes are running") {
		t.Fatalf("expected running network confirmation, got:\n%s", output.String())
	}

	web3URL := fmt.Sprintf("http://127.0.0.1:%d/web3", rpcBasePort)
	client := http.Client{Timeout: time.Duration(10_000_000_000)}
	if chainID := networkE2EWeb3Call[string](t, client, web3URL, "eth_chainId"); chainID != "0x147f8" {
		t.Fatalf("unexpected EVM chain id %q", chainID)
	}
	accounts := networkE2EWeb3Call[[]string](t, client, web3URL, "eth_accounts")
	if len(accounts) == 0 {
		t.Fatal("network did not expose a managed EVM development account")
	}
	from := accounts[0]
	send := func(to string, data []byte, gas string) string {
		t.Helper()
		transaction := map[string]any{
			"from":     from,
			"data":     "0x" + hex.EncodeToString(data),
			"gas":      gas,
			"gasPrice": "0x1",
		}
		if to != "" {
			transaction["to"] = to
		}
		return networkE2EWeb3Call[string](t, client, web3URL, "eth_sendTransaction", transaction)
	}
	receiptFor := func(hash string) map[string]any {
		t.Helper()
		receipt := waitNetworkE2EReceipt(t, client, web3URL, hash)
		if receipt == nil {
			latestNonce := networkE2EWeb3Call[string](t, client, web3URL, "eth_getTransactionCount", from, "latest")
			pendingNonce := networkE2EWeb3Call[string](t, client, web3URL, "eth_getTransactionCount", from, "pending")
			stored := networkE2EWeb3Call[map[string]any](t, client, web3URL, "eth_getTransactionByHash", hash)
			plan, _ := buildNetworkRuntimePlanWithPorts(home, 4, binaryPath, p2pBasePort, rpcBasePort)
			var logs bytes.Buffer
			writeNetworkLogTails(&logs, plan, 64*1024)
			t.Fatalf("transaction %s did not produce a receipt: latest_nonce=%s pending_nonce=%s stored=%+v\n%s", hash, latestNonce, pendingNonce, stored, logs.String())
		}
		if receipt["status"] != "0x1" {
			t.Fatalf("transaction %s failed: %+v", hash, receipt)
		}
		return receipt
	}
	submit := func(to string, data []byte, gas string) map[string]any {
		t.Helper()
		return receiptFor(send(to, data, gas))
	}

	tpsCode, err := os.ReadFile(filepath.Join("..", "..", "rpc", "testdata", "tps_check_creation.hex"))
	if err != nil {
		t.Fatal(err)
	}
	tpsCreation, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(string(tpsCode)), "0x"))
	if err != nil {
		t.Fatal(err)
	}
	tpsReceipt := submit("", tpsCreation, "0x7a1200")
	tpsAddress := networkE2EContractAddress(t, tpsReceipt)
	if code := networkE2EWeb3Call[string](t, client, web3URL, "eth_getCode", tpsAddress, "latest"); code == "0x" {
		t.Fatal("TpsCheck deployment produced no runtime code")
	}
	tpsHash, _ := tpsReceipt["transactionHash"].(string)
	storedTransaction := networkE2EWeb3Call[map[string]any](t, client, web3URL, "eth_getTransactionByHash", tpsHash)
	if storedTransaction["to"] != nil || storedTransaction["r"] == nil || storedTransaction["s"] == nil || storedTransaction["v"] == nil {
		t.Fatalf("invalid Ethereum creation transaction response: %+v", storedTransaction)
	}

	v1Receipt := submit("", networkE2EUUPSImplementationCreation(t, 1), "0x2dc6c0")
	v1Address := networkE2EContractAddress(t, v1Receipt)
	v2Receipt := submit("", networkE2EUUPSImplementationCreation(t, 2), "0x2dc6c0")
	v2Address := networkE2EContractAddress(t, v2Receipt)
	proxyReceipt := submit("", networkE2EDelegateProxyCreation(t, v1Address), "0x2dc6c0")
	proxyAddress := networkE2EContractAddress(t, proxyReceipt)
	versionCall := map[string]any{"from": from, "to": proxyAddress, "data": "0x54fd4d50"}
	if version := networkE2EWeb3Call[string](t, client, web3URL, "eth_call", versionCall, "latest"); version != networkE2EUint256(1) {
		t.Fatalf("unexpected proxy version before upgrade %q", version)
	}
	submit(proxyAddress, []byte{0x81, 0x29, 0xfc, 0x1c}, "0x7a120")
	upgradeInput := "3659cfe6" + strings.Repeat("0", 24) + strings.TrimPrefix(v2Address, "0x")
	decodedUpgrade, err := hex.DecodeString(upgradeInput)
	if err != nil {
		t.Fatal(err)
	}
	upgradeReceipt := submit(proxyAddress, decodedUpgrade, "0x7a120")
	if version := networkE2EWeb3Call[string](t, client, web3URL, "eth_call", versionCall, "latest"); version != networkE2EUint256(2) {
		t.Fatalf("unexpected proxy version after upgrade %q", version)
	}

	firstPendingHash := send(from, nil, "0x5208")
	secondPendingHash := send(from, nil, "0x5208")
	firstPendingTx := networkE2EWeb3Call[map[string]any](t, client, web3URL, "eth_getTransactionByHash", firstPendingHash)
	secondPendingTx := networkE2EWeb3Call[map[string]any](t, client, web3URL, "eth_getTransactionByHash", secondPendingHash)
	firstPendingNonce := networkE2EHexUint64(t, firstPendingTx["nonce"])
	secondPendingNonce := networkE2EHexUint64(t, secondPendingTx["nonce"])
	if secondPendingNonce != firstPendingNonce+1 {
		t.Fatalf("managed account reused a pending nonce: first=%d second=%d", firstPendingNonce, secondPendingNonce)
	}
	receiptFor(firstPendingHash)
	secondPendingReceipt := receiptFor(secondPendingHash)
	latestBlock, _ := secondPendingReceipt["blockNumber"].(string)
	if latestBlock == "" {
		latestBlock, _ = upgradeReceipt["blockNumber"].(string)
	}
	waitNetworkE2EValidatorConvergence(t, client, rpcBasePort, latestBlock)

	stopOutput := stopNetworkE2E(binaryPath, home, p2pBasePort, rpcBasePort)
	stopped = true
	if !strings.Contains(stopOutput, "stopped validator-4") {
		t.Fatalf("expected network stop confirmation, got:\n%s", stopOutput)
	}
	for validatorIndex := 1; validatorIndex <= 4; validatorIndex++ {
		logPath := filepath.Join(home, networkValidatorID(validatorIndex), "vexod.log")
		logData, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		logText := string(logData)
		for _, forbidden := range []string{
			"invalid transaction nonce",
			"nonce too high",
			"nonce too low",
			"invalid EVM transaction",
			"double sign detected",
			"unsafe proposal",
			"block height is already committed",
			"block commit height is not sequential",
			"justify qc height exceeds proposal height",
		} {
			if strings.Contains(logText, forbidden) {
				t.Fatalf("%s emitted forbidden consensus error %q:\n%s", networkValidatorID(validatorIndex), forbidden, logText)
			}
		}
	}
}

func networkE2EHexUint64(t *testing.T, value any) uint64 {
	t.Helper()
	text, ok := value.(string)
	if !ok {
		t.Fatalf("expected hex quantity, got %#v", value)
	}
	parsed, err := strconv.ParseUint(strings.TrimPrefix(text, "0x"), 16, 64)
	if err != nil {
		t.Fatalf("invalid hex quantity %q: %v", text, err)
	}
	return parsed
}

func stopNetworkE2E(binaryPath string, home string, p2pBasePort int, rpcBasePort int) string {
	command := exec.Command(binaryPath,
		"network", "stop",
		"--home", home,
		"--validators", "4",
		"--p2p-base-port", strconv.Itoa(p2pBasePort),
		"--rpc-base-port", strconv.Itoa(rpcBasePort),
	)
	output, _ := command.CombinedOutput()
	return string(output)
}

func networkE2EWeb3Call[T any](t *testing.T, client http.Client, endpoint string, method string, params ...any) T {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s request failed: %v", method, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s returned HTTP %d", method, response.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	if envelope.Error != nil {
		t.Fatalf("%s failed: code=%d message=%s", method, envelope.Error.Code, envelope.Error.Message)
	}
	var result T
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return result
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode %s result: %v", method, err)
	}
	return result
}

func waitNetworkE2EReceipt(t *testing.T, client http.Client, endpoint string, hash string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(time.Duration(30_000_000_000))
	for time.Now().Before(deadline) {
		receipt := networkE2EWeb3Call[map[string]any](t, client, endpoint, "eth_getTransactionReceipt", hash)
		if receipt != nil {
			return receipt
		}
		time.Sleep(time.Duration(20_000_000))
	}
	return nil
}

func waitNetworkE2EValidatorConvergence(t *testing.T, client http.Client, rpcBasePort int, targetBlock string) {
	t.Helper()
	deadline := time.Now().Add(time.Duration(30_000_000_000))
	for time.Now().Before(deadline) {
		var expectedHash string
		var expectedStateRoot string
		converged := targetBlock != ""
		for validatorIndex := 1; validatorIndex <= 4; validatorIndex++ {
			endpoint := fmt.Sprintf("http://127.0.0.1:%d/web3", rpcBasePort+(validatorIndex-1)*10)
			block := networkE2EWeb3Call[map[string]any](t, client, endpoint, "eth_getBlockByNumber", targetBlock, false)
			hash, _ := block["hash"].(string)
			stateRoot, _ := block["stateRoot"].(string)
			if hash == "" || stateRoot == "" {
				converged = false
				break
			}
			if validatorIndex == 1 {
				expectedHash = hash
				expectedStateRoot = stateRoot
				continue
			}
			if hash != expectedHash || stateRoot != expectedStateRoot {
				converged = false
				break
			}
		}
		if converged {
			return
		}
		time.Sleep(time.Duration(20_000_000))
	}
	t.Fatalf("validators did not converge on EVM block %s", targetBlock)
}

func networkE2EContractAddress(t *testing.T, receipt map[string]any) string {
	t.Helper()
	address, ok := receipt["contractAddress"].(string)
	if !ok || address == "" {
		t.Fatalf("missing contract address: %+v", receipt)
	}
	return address
}

func networkE2EUint256(value byte) string {
	return "0x" + strings.Repeat("0", 62) + hex.EncodeToString([]byte{value})
}

func networkE2EUUPSImplementationCreation(t *testing.T, version byte) []byte {
	t.Helper()
	builder := newNetworkE2EBytecodeBuilder()
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x54, 0xfd, 0x4d, 0x50, 0x14)
	builder.pushLabel("version")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x81, 0x29, 0xfc, 0x1c, 0x14)
	builder.pushLabel("initialize")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x36, 0x59, 0xcf, 0xe6, 0x14)
	builder.pushLabel("upgrade")
	builder.op(0x57, 0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("version")
	builder.op(0x60, version, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3)
	builder.label("initialize")
	builder.op(0x33, 0x60, 0x01, 0x55, 0x00)
	builder.label("upgrade")
	builder.op(0x33, 0x60, 0x01, 0x54, 0x14)
	builder.pushLabel("upgrade_ok")
	builder.op(0x57, 0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("upgrade_ok")
	builder.op(0x60, 0x04, 0x35, 0x73)
	builder.op(make([]byte, 20)...)
	for index := len(builder.code) - 20; index < len(builder.code); index++ {
		builder.code[index] = 0xff
	}
	builder.op(0x16, 0x60, 0x00, 0x55, 0x00)
	return networkE2ECreationCode(t, nil, builder.bytes(t))
}

func networkE2EDelegateProxyCreation(t *testing.T, implementation string) []byte {
	t.Helper()
	implementationBytes, err := hex.DecodeString(strings.TrimPrefix(implementation, "0x"))
	if err != nil || len(implementationBytes) != 20 {
		t.Fatalf("invalid implementation address %q", implementation)
	}
	builder := newNetworkE2EBytecodeBuilder()
	builder.op(0x36, 0x60, 0x00, 0x60, 0x00, 0x37)
	builder.op(0x60, 0x00, 0x60, 0x00, 0x36, 0x60, 0x00, 0x60, 0x00, 0x54, 0x5a, 0xf4)
	builder.op(0x3d, 0x60, 0x00, 0x60, 0x00, 0x3e)
	builder.pushLabel("success")
	builder.op(0x57, 0x3d, 0x60, 0x00, 0xfd)
	builder.label("success")
	builder.op(0x3d, 0x60, 0x00, 0xf3)
	prefix := append([]byte{0x73}, implementationBytes...)
	prefix = append(prefix, 0x60, 0x00, 0x55)
	return networkE2ECreationCode(t, prefix, builder.bytes(t))
}

func networkE2ECreationCode(t *testing.T, prefix []byte, runtime []byte) []byte {
	t.Helper()
	offset := len(prefix) + 12
	if len(runtime) > 0xff || offset > 0xff {
		t.Fatal("network E2E bytecode exceeds PUSH1 creation wrapper")
	}
	code := append([]byte(nil), prefix...)
	code = append(code, 0x60, byte(len(runtime)), 0x60, byte(offset), 0x60, 0x00, 0x39, 0x60, byte(len(runtime)), 0x60, 0x00, 0xf3)
	return append(code, runtime...)
}

type networkE2EBytecodeBuilder struct {
	code    []byte
	labels  map[string]int
	patches []networkE2EBytecodePatch
}

type networkE2EBytecodePatch struct {
	position int
	label    string
}

func newNetworkE2EBytecodeBuilder() *networkE2EBytecodeBuilder {
	return &networkE2EBytecodeBuilder{labels: make(map[string]int)}
}

func (builder *networkE2EBytecodeBuilder) op(values ...byte) {
	builder.code = append(builder.code, values...)
}

func (builder *networkE2EBytecodeBuilder) label(name string) {
	builder.labels[name] = len(builder.code)
	builder.op(0x5b)
}

func (builder *networkE2EBytecodeBuilder) pushLabel(name string) {
	builder.op(0x61, 0x00, 0x00)
	builder.patches = append(builder.patches, networkE2EBytecodePatch{position: len(builder.code) - 2, label: name})
}

func (builder *networkE2EBytecodeBuilder) bytes(t *testing.T) []byte {
	t.Helper()
	result := append([]byte(nil), builder.code...)
	for _, patch := range builder.patches {
		offset, ok := builder.labels[patch.label]
		if !ok || offset > 0xffff {
			t.Fatalf("invalid bytecode label %q", patch.label)
		}
		result[patch.position] = byte(offset >> 8)
		result[patch.position+1] = byte(offset)
	}
	return result
}

func networkE2ETempHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("", "vexo-network-e2e-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var err error
		for attempt := 0; attempt < 50; attempt++ {
			err = os.RemoveAll(home)
			if err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("cleanup network e2e home %s: %v", home, err)
	})
	return home
}

func networkE2EBinary(t *testing.T) string {
	t.Helper()
	if binaryPath := os.Getenv("VEXO_NETWORK_E2E_BINARY"); binaryPath != "" {
		if _, err := os.Stat(binaryPath); err != nil {
			t.Fatalf("VEXO_NETWORK_E2E_BINARY is not readable: %v", err)
		}
		return binaryPath
	}
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	binaryPath := filepath.Join(t.TempDir(), "vexod")
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/vexod")
	build.Dir = repoRoot
	build.Env = append(os.Environ(),
		"GOCACHE="+filepath.Join(t.TempDir(), "gocache"),
	)
	buildOutput, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build vexod: %v\n%s", err, buildOutput)
	}
	return binaryPath
}

func reserveNetworkE2EPorts(t *testing.T) (int, int) {
	t.Helper()
	for basePort := 35056; basePort < 39000; basePort += 100 {
		if portsAvailable(basePort, basePort+1, 4) {
			return basePort, basePort + 1
		}
	}
	t.Fatal("no free network e2e port range found")
	return 0, 0
}

func portsAvailable(p2pBasePort int, rpcBasePort int, validators int) bool {
	listeners := make([]net.Listener, 0, validators*2)
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	for index := 0; index < validators; index++ {
		for _, port := range []int{p2pBasePort + index*10, rpcBasePort + index*10} {
			listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
			if err != nil {
				return false
			}
			listeners = append(listeners, listener)
		}
	}
	return true
}

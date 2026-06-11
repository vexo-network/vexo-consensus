package main

import (
	"bytes"
	"fmt"
	"net"
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
	)
	var output bytes.Buffer
	run.Stdout = &output
	run.Stderr = &output
	if err := run.Run(); err != nil {
		t.Fatalf("network up failed: %v\n%s", err, output.String())
	}
	for validatorIndex := 1; validatorIndex <= 4; validatorIndex++ {
		expected := fmt.Sprintf("validator-%d rpc=127.0.0.1:%d healthy=true height=1", validatorIndex, rpcBasePort+(validatorIndex-1)*10)
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output.String())
		}
	}
	if !strings.Contains(output.String(), "network up ok; stopping nodes") || !strings.Contains(output.String(), "stopped validator-4") {
		t.Fatalf("expected network stop confirmation, got:\n%s", output.String())
	}
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

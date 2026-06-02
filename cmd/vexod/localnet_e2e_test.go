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
)

func TestLocalnetUpBuiltBinaryE2E(t *testing.T) {
	if os.Getenv("VEXO_LOCALNET_E2E") != "1" {
		t.Skip("set VEXO_LOCALNET_E2E=1 to run built-binary localnet e2e")
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

	p2pBasePort, rpcBasePort := reserveLocalnetE2EPorts(t)
	home := t.TempDir()
	run := exec.Command(binaryPath,
		"localnet", "up",
		"--home", home,
		"--validators", "4",
		"--p2p-base-port", strconv.Itoa(p2pBasePort),
		"--rpc-base-port", strconv.Itoa(rpcBasePort),
		"--timeout", "20s",
		"--overwrite",
	)
	var output bytes.Buffer
	run.Stdout = &output
	run.Stderr = &output
	if err := run.Run(); err != nil {
		t.Fatalf("localnet up failed: %v\n%s", err, output.String())
	}
	for validatorIndex := 1; validatorIndex <= 4; validatorIndex++ {
		expected := fmt.Sprintf("validator-%d rpc=127.0.0.1:%d healthy=true height=1", validatorIndex, rpcBasePort+(validatorIndex-1)*10)
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output.String())
		}
	}
	if !strings.Contains(output.String(), "localnet up ok; stopping nodes") || !strings.Contains(output.String(), "stopped validator-4") {
		t.Fatalf("expected localnet stop confirmation, got:\n%s", output.String())
	}
}

func reserveLocalnetE2EPorts(t *testing.T) (int, int) {
	t.Helper()
	for basePort := 35056; basePort < 39000; basePort += 100 {
		if portsAvailable(basePort, basePort+1, 4) {
			return basePort, basePort + 1
		}
	}
	t.Fatal("no free localnet e2e port range found")
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

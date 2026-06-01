package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/config"
)

func TestWriteStatus(t *testing.T) {
	var buffer bytes.Buffer
	writeStatus(&buffer, config.Default("vexo-test"))

	output := buffer.String()
	expectedParts := []string{
		"vexo-consensus status",
		"chain_id: vexo-test",
		"validator.permissionless: true",
		"committee.size: 128",
	}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Fatalf("expected output to contain %q, got:\n%s", part, output)
		}
	}
	hiddenStatusTerm := "pro" + "gress"
	if strings.Contains(output, hiddenStatusTerm) {
		t.Fatalf("unexpected status term in output:\n%s", output)
	}
}

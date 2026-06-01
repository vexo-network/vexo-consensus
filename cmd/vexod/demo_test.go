package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteDemo(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeDemo(&buffer); err != nil {
		t.Fatal(err)
	}

	output := buffer.String()
	expectedParts := []string{
		"vexo-consensus demo",
		"executed_height: 1",
		"tx_results: 1",
		"app_hash:",
	}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Fatalf("expected output to contain %q, got:\n%s", part, output)
		}
	}
}

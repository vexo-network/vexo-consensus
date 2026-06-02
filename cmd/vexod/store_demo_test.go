package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteStoreDemo(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeStoreDemo(&buffer, t.TempDir()); err != nil {
		t.Fatal(err)
	}

	output := buffer.String()
	expectedParts := []string{
		"vexo-consensus store demo",
		"stored_height: 1",
		"stored_block_hash:",
		"latest_state_height: 1",
		"state_roots:",
		"bob_balance: 25",
	}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Fatalf("expected output to contain %q, got:\n%s", part, output)
		}
	}
}

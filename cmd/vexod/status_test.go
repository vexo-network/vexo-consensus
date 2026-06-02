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
		"fair_ordering.deterministic: true",
		"data_availability.commitments: true",
		"storage.backend: leveldb",
		"p2p.initial_score: 100",
		"p2p.invalid_message_cost: 10",
		"p2p.rate_limit_cost: 5",
		"p2p.ban_threshold: 0",
		"p2p.max_messages_per_window: 1000",
		"p2p.window_reset_interval:",
		"p2p.score_recovery: 1",
		"p2p.ban_duration:",
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

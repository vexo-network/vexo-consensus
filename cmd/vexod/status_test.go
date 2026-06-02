package main

import (
	"bytes"
	"encoding/json"
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

func TestWriteStatusJSON(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeStatusJSON(&buffer, config.Default("vexo-test")); err != nil {
		t.Fatal(err)
	}

	var document statusDocument
	if err := json.Unmarshal(buffer.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "v1" || document.ChainID != "vexo-test" {
		t.Fatalf("unexpected document identity: %+v", document)
	}
	if !document.Validator.Permissionless || document.Validator.MinStake != 1 {
		t.Fatalf("unexpected validator status: %+v", document.Validator)
	}
	if document.Committee.Size != 128 || document.Committee.Backend != "deterministic" {
		t.Fatalf("unexpected committee status: %+v", document.Committee)
	}
	if document.Storage.Backend != "leveldb" || !document.Features["peer_scoring"] {
		t.Fatalf("unexpected feature/storage status: %+v", document)
	}
	if document.P2P.InitialScore != 100 || document.P2P.InvalidMessageCost != 10 || !document.P2P.PeerSnapshotsEnabled {
		t.Fatalf("unexpected p2p status: %+v", document.P2P)
	}
	if document.OperationalHints.PeerMetricsLocation != "node.Status().Peers" {
		t.Fatalf("unexpected operational hints: %+v", document.OperationalHints)
	}
}

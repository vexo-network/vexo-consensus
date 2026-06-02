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
		"application.modules: [bank]",
		"execution.max_gas: 10000000",
		"execution.require_signed: false",
		"execution.fee_collector: fee_collector",
		"validator.permissionless: true",
		"committee.size: 128",
		"mempool.min_fee: 0",
		"mempool.priority_enabled: false",
		"fair_ordering.deterministic: true",
		"fair_ordering.height_salted: true",
		"data_availability.commitments: true",
		"storage.backend: leveldb",
		"addr_book.persistent: true",
		"addr_book.dial_failure_tracking: true",
		"addr_book.ban_eviction_policy: true",
		"p2p.transport_peer_gate: true",
		"p2p.consensus_gossip_scoring: true",
		"p2p.banned_peer_disconnect: true",
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
	if len(document.Application.Modules) != 1 || document.Application.Modules[0] != "bank" {
		t.Fatalf("unexpected application status: %+v", document.Application)
	}
	if !document.Validator.Permissionless || document.Validator.MinStake != 1 {
		t.Fatalf("unexpected validator status: %+v", document.Validator)
	}
	if document.Committee.Size != 128 || document.Committee.Backend != "deterministic" {
		t.Fatalf("unexpected committee status: %+v", document.Committee)
	}
	if document.Mempool.MinFee != 0 || document.Mempool.EnablePriority {
		t.Fatalf("unexpected mempool status: %+v", document.Mempool)
	}
	if document.Storage.Backend != "leveldb" || !document.Features["peer_scoring"] || !document.Features["height_salted_order"] || !document.Features["deployment_audit"] || !document.Features["addr_book"] || !document.Features["addr_book_ban_evict"] || !document.Features["peer_dial_tracking"] || !document.Features["transport_peer_gate"] || !document.Features["consensus_gossip_scoring"] || !document.Features["banned_peer_disconnect"] {
		t.Fatalf("unexpected feature/storage status: %+v", document)
	}
	if document.P2P.InitialScore != 100 || document.P2P.InvalidMessageCost != 10 || !document.P2P.PeerSnapshotsEnabled {
		t.Fatalf("unexpected p2p status: %+v", document.P2P)
	}
	if document.OperationalHints.PeerMetricsLocation != "node.Status().Peers" {
		t.Fatalf("unexpected operational hints: %+v", document.OperationalHints)
	}
}

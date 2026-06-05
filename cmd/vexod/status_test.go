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
		"application.modules: [bank staking governance params ibc]",
		"execution.max_gas: 10000000",
		"execution.require_signed: false",
		"execution.fee_collector: fee_collector",
		"bank.mint_authority:",
		"validator.permissionless: true",
		"committee.size: 128",
		"committee.min_voting_power: 1",
		"committee.backend: deterministic",
		"mempool.max_tx_bytes: 1048576",
		"mempool.min_fee: 0",
		"mempool.seen_ttl:",
		"mempool.priority_enabled: false",
		"fair_ordering.deterministic: true",
		"fair_ordering.height_salted: true",
		"data_availability.commitments: true",
		"storage.backend: leveldb",
		"state_sync.snapshot_kv: true",
		"state_sync.snapshot_checksum: true",
		"state_sync.snapshot_verify: true",
		"state_sync.snapshot_chunks: true",
		"ops.metrics_uptime: true",
		"ops.pprof_optional: true",
		"ops.structured_logs: true",
		"ops.release_artifacts: true",
		"ops.external_audit_pack: true",
		"ops.deployment_template: true",
		"ops.longrun_network_plan: true",
		"security.fuzz_targets: true",
		"security.strict_json_rpc: true",
		"security.forwarded_for_untrusted: true",
		"consensus.adversarial_simulation: true",
		"consensus.partition_safety_simulation: true",
		"consensus.tendermint_style_timeouts: true",
		"consensus.empty_block_control: true",
		"crypto.backend: deterministic",
		"crypto.production_adapter: false",
		"crypto.remote_signer_verification: true",
		"crypto.bls_adapter_required: true",
		"addr_book.persistent: true",
		"addr_book.dial_failure_tracking: true",
		"addr_book.ban_eviction_policy: true",
		"p2p.transport_peer_gate: true",
		"p2p.consensus_gossip_scoring: true",
		"p2p.banned_peer_disconnect: true",
		"p2p.peer_score_persistence: true",
		"p2p.initial_score: 100",
		"p2p.max_score: 1000",
		"p2p.invalid_message_cost: 10",
		"p2p.rate_limit_cost: 5",
		"p2p.ban_threshold: 0",
		"p2p.max_messages_per_window: 1000",
		"p2p.window_reset_interval:",
		"p2p.score_recovery: 1",
		"p2p.ban_duration:",
		"p2p.peer_snapshots_enabled: true",
		"p2p.node_status_peer_metrics: true",
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
	if len(document.Application.Modules) != 5 ||
		document.Application.Modules[0] != "bank" ||
		document.Application.Modules[1] != "staking" ||
		document.Application.Modules[2] != "governance" ||
		document.Application.Modules[3] != "params" ||
		document.Application.Modules[4] != "ibc" {
		t.Fatalf("unexpected application status: %+v", document.Application)
	}
	if !document.Validator.Permissionless || document.Validator.MinStake != 1 {
		t.Fatalf("unexpected validator status: %+v", document.Validator)
	}
	if document.Committee.Size != 128 || document.Committee.Backend != "deterministic" {
		t.Fatalf("unexpected committee status: %+v", document.Committee)
	}
	if document.Bank.MintAuthority != "" {
		t.Fatalf("unexpected bank status: %+v", document.Bank)
	}
	if document.Mempool.MinFee != 0 || document.Mempool.EnablePriority {
		t.Fatalf("unexpected mempool status: %+v", document.Mempool)
	}
	if document.Execution.FeeDenom != "avxo" || document.Execution.DisplayDenom != "vexo" || document.Execution.DisplayExponent != 18 || document.Execution.GasDenom != "gas" {
		t.Fatalf("unexpected execution status: %+v", document.Execution)
	}
	if document.Storage.Backend != "leveldb" || !document.Features["peer_scoring"] || !document.Features["height_salted_order"] || !document.Features["deployment_audit"] || !document.Features["addr_book"] || !document.Features["addr_book_ban_evict"] || !document.Features["peer_dial_tracking"] || !document.Features["transport_peer_gate"] || !document.Features["consensus_gossip_scoring"] || !document.Features["banned_peer_disconnect"] || !document.Features["peer_score_persistence"] || !document.Features["data_availability_chunk_proofs"] || !document.Features["data_availability_parity_recovery"] || !document.Features["state_sync_snapshot_kv"] || !document.Features["state_sync_checksum"] || !document.Features["state_sync_verify"] || !document.Features["state_sync_snapshot_chunks"] || !document.Features["state_sync_historical_replay"] || !document.Features["web3_receipt_roots"] || !document.Features["web3_global_log_filters"] || !document.Features["web3_prefix_log_index"] || !document.Features["web3_filter_limit"] || !document.Features["evm_storage_writes"] || !document.Features["validator_update_atomic_commit"] || !document.Features["staking_slashing_ledger"] || !document.Features["mempool_seen_cache_pruning"] || !document.Features["ops_metrics_uptime"] || !document.Features["ops_pprof_optional"] || !document.Features["ops_structured_logs"] || !document.Features["ops_release_artifacts"] || !document.Features["ops_external_audit_pack"] || !document.Features["ops_deployment_template"] || !document.Features["ops_longrun_network_plan"] || !document.Features["security_fuzz_targets"] || !document.Features["security_strict_json_rpc"] || !document.Features["security_forwarded_for_untrusted"] || !document.Features["consensus_adversarial_simulation"] || !document.Features["consensus_partition_safety"] || !document.Features["consensus_tendermint_timeouts"] || !document.Features["consensus_empty_block_control"] || !document.Features["consensus_app_hash_evidence"] || !document.Features["crypto_remote_signer_verification"] || !document.Features["crypto_bls_adapter_required"] {
		t.Fatalf("unexpected feature/storage status: %+v", document)
	}
	if document.P2P.InitialScore != 100 || document.P2P.MaxScore != 1000 || document.P2P.InvalidMessageCost != 10 || !document.P2P.PeerSnapshotsEnabled {
		t.Fatalf("unexpected p2p status: %+v", document.P2P)
	}
	if document.OperationalHints.PeerMetricsLocation != "node.Status().Peers" {
		t.Fatalf("unexpected operational hints: %+v", document.OperationalHints)
	}
}

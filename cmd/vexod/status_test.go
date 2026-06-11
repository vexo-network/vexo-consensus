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
		"execution.allow_unprotected_legacy_tx: false",
		"bank.mint_authority:",
		"governance.quorum_power: 1",
		"governance.require_deposit: false",
		"governance.deposit_denom: avxo",
		"validator.permissionless: true",
		"committee.size: 128",
		"committee.min_voting_power: 1",
		"committee.backend: deterministic",
		"mempool.max_tx_bytes: 1048576",
		"mempool.min_fee: 0",
		"mempool.seen_ttl:",
		"mempool.priority_enabled: false",
		"mempool.replacement_enabled: true",
		"mempool.replacement_bump_bps: 1000",
		"fair_ordering.deterministic: true",
		"fair_ordering.height_salted: true",
		"data_availability.commitments: true",
		"data_availability.reed_solomon_recovery: true",
		"data_availability.deterministic_sampling: true",
		"storage.backend: leveldb",
		"state_sync.snapshot_kv: true",
		"state_sync.snapshot_checksum: true",
		"state_sync.snapshot_verify: true",
		"state_sync.snapshot_chunks: true",
		"staking.tombstone_ledger: true",
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
		"crypto.bls_adapter_required: false",
		"web3.raw_tx_access_list_prewarm: false",
		"web3.call_vm_trace_state_diff: false",
		"web3.prestate_and_4byte_tracers: false",
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
	if document.Governance.QuorumPower != 1 || document.Governance.RequireDeposit || document.Governance.DepositDenom != "avxo" {
		t.Fatalf("unexpected governance status: %+v", document.Governance)
	}
	if document.Mempool.MinFee != 0 || document.Mempool.EnablePriority || !document.Mempool.EnableReplacement || document.Mempool.ReplacementBumpBPS != 1000 {
		t.Fatalf("unexpected mempool status: %+v", document.Mempool)
	}
	if document.Execution.FeeDenom != "avxo" || document.Execution.DisplayDenom != "vexo" || document.Execution.DisplayExponent != 18 || document.Execution.GasDenom != "gas" || document.Execution.AllowUnprotectedLegacyTx {
		t.Fatalf("unexpected execution status: %+v", document.Execution)
	}
	if document.Storage.Backend != "leveldb" {
		t.Fatalf("unexpected storage status: %+v", document.Storage)
	}
	expectedFeatures := []string{
		"peer_scoring",
		"height_salted_order",
		"deployment_audit",
		"addr_book",
		"addr_book_ban_evict",
		"peer_dial_tracking",
		"transport_peer_gate",
		"consensus_gossip_scoring",
		"banned_peer_disconnect",
		"peer_score_persistence",
		"data_availability_chunk_proofs",
		"data_availability_parity_recovery",
		"data_availability_reed_solomon_recovery",
		"data_availability_deterministic_sampling",
		"state_sync_snapshot_kv",
		"state_sync_checksum",
		"state_sync_verify",
		"state_sync_snapshot_chunks",
		"state_sync_historical_replay",
		"staking_tombstone_ledger",
		"ibc_ordering_validation",
		"execution_context_aware_app_calls",
		"validator_update_atomic_commit",
		"staking_slashing_ledger",
		"ops_metrics_uptime",
		"ops_pprof_optional",
		"ops_structured_logs",
		"ops_release_artifacts",
		"ops_release_evidence_content_gate",
		"ops_release_evidence_semantic_gate",
		"ops_external_audit_pack",
		"ops_deployment_template",
		"ops_longrun_network_plan",
		"security_fuzz_targets",
		"security_strict_json_rpc",
		"security_forwarded_for_untrusted",
		"consensus_adversarial_simulation",
		"consensus_partition_safety",
		"consensus_tendermint_timeouts",
		"consensus_empty_block_control",
		"consensus_app_hash_evidence",
		"consensus_tx_validity_evidence",
		"crypto_remote_signer_verification",
	}
	for _, feature := range expectedFeatures {
		if !document.Features[feature] {
			t.Fatalf("expected feature %q in status document: %+v", feature, document.Features)
		}
	}
	if document.P2P.InitialScore != 100 || document.P2P.MaxScore != 1000 || document.P2P.InvalidMessageCost != 10 || !document.P2P.PeerSnapshotsEnabled {
		t.Fatalf("unexpected p2p status: %+v", document.P2P)
	}
	if document.OperationalHints.PeerMetricsLocation != "node.Status().Peers" {
		t.Fatalf("unexpected operational hints: %+v", document.OperationalHints)
	}
	if document.FeatureAssurance["ops_longrun_network_plan"].State != "requires_operator_artifact" {
		t.Fatalf("expected operator evidence assurance for longrun plan, got %+v", document.FeatureAssurance["ops_longrun_network_plan"])
	}
	if document.Features["mempool_seen_cache_pruning"] ||
		document.Features["crypto_bls_adapter_required"] ||
		document.Features["web3_geth_compat_methods"] ||
		document.Features["evm_geth_vm_adapter"] {
		t.Fatalf("expected optional features disabled by default: %+v", document.Features)
	}
}

func TestStatusFeaturesReflectConfiguredModules(t *testing.T) {
	cfg := config.Default("vexo-test")
	features := statusFeatures(cfg)
	evmFeatures := []string{
		"web3_geth_compat_methods",
		"web3_trace_api",
		"evm_geth_vm_adapter",
		"evm_ethereum_raw_tx",
	}
	for _, feature := range evmFeatures {
		if features[feature] {
			t.Fatalf("expected EVM feature %q disabled without evm module", feature)
		}
	}
	if !features["staking_slashing_ledger"] || !features["ibc_ordering_validation"] {
		t.Fatalf("expected configured staking/ibc features enabled: %+v", features)
	}

	cfg.Application.Modules = append(cfg.Application.Modules, "evm")
	features = statusFeatures(cfg)
	for _, feature := range evmFeatures {
		if !features[feature] {
			t.Fatalf("expected EVM feature %q enabled with evm module", feature)
		}
	}
	assurance := statusFeatureAssurance(cfg)
	if assurance["web3_geth_compat_methods"].State != "requires_release_evidence" ||
		assurance["evm_geth_vm_adapter"].State != "requires_release_evidence" {
		t.Fatalf("expected EVM assurance to require release evidence: %+v", assurance)
	}
	if assurance["web3_pow_uncle_compat_responses"].State != "compatibility_boundary" {
		t.Fatalf("expected PoW/uncle compatibility boundary assurance: %+v", assurance["web3_pow_uncle_compat_responses"])
	}
	if features["crypto_bls_production_adapter"] {
		t.Fatalf("expected BLS production feature disabled on deterministic backend")
	}

	cfg.Crypto.Backend = config.CryptoBackendBLS
	cfg.Crypto.ProductionAdapter = true
	features = statusFeatures(cfg)
	if !features["crypto_bls_adapter_required"] || !features["crypto_bls_production_adapter"] {
		t.Fatalf("expected BLS features enabled for production BLS backend: %+v", features)
	}
	assurance = statusFeatureAssurance(cfg)
	if assurance["crypto_bls_production_adapter"].State != "requires_external_audit_evidence" {
		t.Fatalf("expected BLS assurance to require external audit evidence: %+v", assurance["crypto_bls_production_adapter"])
	}
}

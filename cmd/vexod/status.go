package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

func writeStatus(writer io.Writer, cfg config.Config) {
	features := statusFeatures(cfg)
	fmt.Fprintf(writer, "vexo-consensus status\n")
	fmt.Fprintf(writer, "chain_id: %s\n", cfg.ChainID)
	fmt.Fprintf(writer, "application.modules: %v\n", cfg.Application.Modules)
	fmt.Fprintf(writer, "execution.min_fee: %d\n", cfg.Execution.MinFee)
	fmt.Fprintf(writer, "execution.base_fee: %d\n", cfg.Execution.BaseFee)
	fmt.Fprintf(writer, "execution.blob_base_fee: %d\n", cfg.Execution.BlobBaseFee)
	fmt.Fprintf(writer, "execution.evm_chain_id: %d\n", cfg.Execution.EVMChainID)
	fmt.Fprintf(writer, "execution.dynamic_base_fee: %t\n", cfg.Execution.DynamicBaseFee)
	fmt.Fprintf(writer, "execution.dynamic_blob_base_fee: %t\n", cfg.Execution.DynamicBlobBaseFee)
	fmt.Fprintf(writer, "execution.target_gas: %d\n", cfg.Execution.TargetGas)
	fmt.Fprintf(writer, "execution.target_blob_gas: %d\n", cfg.Execution.TargetBlobGas)
	fmt.Fprintf(writer, "execution.max_blob_gas: %d\n", cfg.Execution.MaxBlobGas)
	fmt.Fprintf(writer, "execution.base_fee_change_denominator: %d\n", cfg.Execution.BaseFeeChangeDenominator)
	fmt.Fprintf(writer, "execution.blob_fee_change_denominator: %d\n", cfg.Execution.BlobFeeChangeDenominator)
	fmt.Fprintf(writer, "execution.min_base_fee: %d\n", cfg.Execution.MinBaseFee)
	fmt.Fprintf(writer, "execution.max_base_fee: %d\n", cfg.Execution.MaxBaseFee)
	fmt.Fprintf(writer, "execution.min_blob_base_fee: %d\n", cfg.Execution.MinBlobBaseFee)
	fmt.Fprintf(writer, "execution.max_blob_base_fee: %d\n", cfg.Execution.MaxBlobBaseFee)
	fmt.Fprintf(writer, "execution.min_gas: %d\n", cfg.Execution.MinGas)
	fmt.Fprintf(writer, "execution.max_gas: %d\n", cfg.Execution.MaxGas)
	fmt.Fprintf(writer, "execution.require_nonce: %t\n", cfg.Execution.RequireNonce)
	fmt.Fprintf(writer, "execution.require_signed: %t\n", cfg.Execution.RequireSigned)
	fmt.Fprintf(writer, "execution.fee_collector: %s\n", cfg.Execution.FeeCollector)
	fmt.Fprintf(writer, "execution.fee_denom: %s\n", cfg.Execution.FeeDenom)
	fmt.Fprintf(writer, "execution.display_denom: %s\n", cfg.Execution.DisplayDenom)
	fmt.Fprintf(writer, "execution.display_exponent: %d\n", cfg.Execution.DisplayExponent)
	fmt.Fprintf(writer, "execution.gas_denom: %s\n", cfg.Execution.GasDenom)
	fmt.Fprintf(writer, "execution.evm_fork_preset: %s\n", cfg.Execution.EVMForkPreset)
	fmt.Fprintf(writer, "execution.evm_chain_config_json: %t\n", cfg.Execution.EVMChainConfigJSON != "")
	fmt.Fprintf(writer, "execution.strict_evm_state_root: %t\n", cfg.Execution.StrictEVMStateRoot)
	fmt.Fprintf(writer, "execution.allow_unprotected_legacy_tx: %t\n", cfg.Execution.AllowUnprotectedLegacyTx)
	fmt.Fprintf(writer, "execution.max_blob_sidecar_blobs: %d\n", cfg.Execution.MaxBlobSidecarBlobs)
	fmt.Fprintf(writer, "execution.max_blob_sidecar_bytes: %d\n", cfg.Execution.MaxBlobSidecarBytes)
	fmt.Fprintf(writer, "bank.mint_authority: %s\n", cfg.Bank.MintAuthority)
	fmt.Fprintf(writer, "validator.permissionless: %t\n", cfg.Validator.Permissionless)
	fmt.Fprintf(writer, "validator.min_stake: %d\n", cfg.Validator.MinStake)
	fmt.Fprintf(writer, "committee.epoch_length: %d\n", cfg.Committee.EpochLength)
	fmt.Fprintf(writer, "committee.size: %d\n", cfg.Committee.CommitteeSize)
	fmt.Fprintf(writer, "committee.min_voting_power: %d\n", cfg.Committee.MinVotingPower)
	fmt.Fprintf(writer, "committee.backend: %s\n", cfg.Committee.Backend)
	fmt.Fprintf(writer, "mempool.max_tx_bytes: %d\n", cfg.Mempool.MaxTxBytes)
	fmt.Fprintf(writer, "mempool.max_txs: %d\n", cfg.Mempool.MaxTxs)
	fmt.Fprintf(writer, "mempool.seen_ttl: %s\n", cfg.Mempool.SeenTTL)
	fmt.Fprintf(writer, "mempool.min_fee: %d\n", cfg.Mempool.MinFee)
	fmt.Fprintf(writer, "mempool.priority_enabled: %t\n", cfg.Mempool.EnablePriority)
	fmt.Fprintf(writer, "mempool.replacement_enabled: %t\n", cfg.Mempool.EnableReplacement)
	fmt.Fprintf(writer, "mempool.replacement_bump_bps: %d\n", cfg.Mempool.ReplacementBumpBPS)
	fmt.Fprintf(writer, "mempool.wal_path: %s\n", cfg.Mempool.WALPath)
	fmt.Fprintf(writer, "fair_ordering.deterministic: true\n")
	fmt.Fprintf(writer, "fair_ordering.height_salted: true\n")
	fmt.Fprintf(writer, "data_availability.commitments: true\n")
	fmt.Fprintf(writer, "data_availability.chunk_proofs: true\n")
	fmt.Fprintf(writer, "data_availability.parity_recovery: true\n")
	fmt.Fprintf(writer, "data_availability.reed_solomon_recovery: true\n")
	fmt.Fprintf(writer, "data_availability.deterministic_sampling: true\n")
	fmt.Fprintf(writer, "storage.backend: leveldb\n")
	fmt.Fprintf(writer, "state_sync.snapshot_kv: true\n")
	fmt.Fprintf(writer, "state_sync.snapshot_checksum: true\n")
	fmt.Fprintf(writer, "state_sync.snapshot_verify: true\n")
	fmt.Fprintf(writer, "state_sync.snapshot_chunks: true\n")
	fmt.Fprintf(writer, "staking.slashing_ledger: %t\n", features["staking_slashing_ledger"])
	fmt.Fprintf(writer, "staking.tombstone_ledger: %t\n", features["staking_tombstone_ledger"])
	fmt.Fprintf(writer, "ops.metrics_uptime: true\n")
	fmt.Fprintf(writer, "ops.pprof_optional: true\n")
	fmt.Fprintf(writer, "ops.structured_logs: true\n")
	fmt.Fprintf(writer, "ops.release_artifacts: true\n")
	fmt.Fprintf(writer, "ops.external_audit_pack: true\n")
	fmt.Fprintf(writer, "ops.deployment_template: true\n")
	fmt.Fprintf(writer, "ops.longrun_network_plan: true\n")
	fmt.Fprintf(writer, "security.fuzz_targets: true\n")
	fmt.Fprintf(writer, "security.strict_json_rpc: true\n")
	fmt.Fprintf(writer, "security.forwarded_for_untrusted: true\n")
	fmt.Fprintf(writer, "web3.raw_tx_access_list_prewarm: %t\n", features["web3_raw_tx_access_list_prewarm"])
	fmt.Fprintf(writer, "web3.call_vm_trace_state_diff: %t\n", features["web3_call_vm_trace_state_diff"])
	fmt.Fprintf(writer, "web3.prestate_and_4byte_tracers: %t\n", features["web3_prestate_and_4byte_tracers"])
	fmt.Fprintf(writer, "web3.blob_tx_fee_accounting: %t\n", features["web3_blob_tx_fee_accounting"])
	fmt.Fprintf(writer, "consensus.adversarial_simulation: true\n")
	fmt.Fprintf(writer, "consensus.partition_safety_simulation: true\n")
	fmt.Fprintf(writer, "consensus.tendermint_style_timeouts: true\n")
	fmt.Fprintf(writer, "consensus.empty_block_control: true\n")
	fmt.Fprintf(writer, "consensus.tx_validity_evidence: true\n")
	fmt.Fprintf(writer, "crypto.backend: %s\n", cfg.Crypto.Backend)
	fmt.Fprintf(writer, "crypto.production_adapter: %t\n", cfg.Crypto.ProductionAdapter)
	fmt.Fprintf(writer, "crypto.remote_signer_verification: true\n")
	fmt.Fprintf(writer, "crypto.bls_adapter_required: %t\n", features["crypto_bls_adapter_required"])
	fmt.Fprintf(writer, "addr_book.persistent: true\n")
	fmt.Fprintf(writer, "addr_book.dial_failure_tracking: true\n")
	fmt.Fprintf(writer, "addr_book.ban_eviction_policy: true\n")
	fmt.Fprintf(writer, "p2p.transport_peer_gate: true\n")
	fmt.Fprintf(writer, "p2p.consensus_gossip_scoring: true\n")
	fmt.Fprintf(writer, "p2p.banned_peer_disconnect: true\n")
	fmt.Fprintf(writer, "p2p.peer_score_persistence: true\n")
	fmt.Fprintf(writer, "p2p.initial_score: %d\n", cfg.P2P.InitialScore)
	fmt.Fprintf(writer, "p2p.max_score: %d\n", cfg.P2P.MaxScore)
	fmt.Fprintf(writer, "p2p.valid_message_reward: %d\n", cfg.P2P.ValidMessageReward)
	fmt.Fprintf(writer, "p2p.invalid_message_cost: %d\n", cfg.P2P.InvalidMessageCost)
	fmt.Fprintf(writer, "p2p.rate_limit_cost: %d\n", cfg.P2P.RateLimitCost)
	fmt.Fprintf(writer, "p2p.ban_threshold: %d\n", cfg.P2P.BanThreshold)
	fmt.Fprintf(writer, "p2p.max_messages_per_window: %d\n", cfg.P2P.MaxMessagesPerWindow)
	fmt.Fprintf(writer, "p2p.window_reset_interval: %s\n", cfg.P2P.WindowResetInterval)
	fmt.Fprintf(writer, "p2p.score_recovery: %d\n", cfg.P2P.ScoreRecovery)
	fmt.Fprintf(writer, "p2p.ban_duration: %s\n", cfg.P2P.BanDuration)
	fmt.Fprintf(writer, "p2p.peer_snapshots_enabled: true\n")
	fmt.Fprintf(writer, "p2p.node_status_peer_metrics: true\n")
}

type statusDocument struct {
	SchemaVersion    string                     `json:"schema_version"`
	ChainID          string                     `json:"chain_id"`
	Application      applicationStatus          `json:"application"`
	Execution        executionStatus            `json:"execution"`
	Bank             bankStatus                 `json:"bank"`
	Validator        validatorStatus            `json:"validator"`
	Committee        committeeStatus            `json:"committee"`
	Mempool          mempoolStatus              `json:"mempool"`
	Features         map[string]bool            `json:"features"`
	FeatureAssurance map[string]assuranceStatus `json:"feature_assurance"`
	Storage          storageStatus              `json:"storage"`
	P2P              p2pStatus                  `json:"p2p"`
	OperationalHints operationalHintsStatus     `json:"operational_hints"`
}

type assuranceStatus struct {
	State string `json:"state"`
	Note  string `json:"note"`
}

type applicationStatus struct {
	Modules []string `json:"modules"`
}

type executionStatus struct {
	MinFee                   uint64 `json:"min_fee"`
	BaseFee                  uint64 `json:"base_fee"`
	BlobBaseFee              uint64 `json:"blob_base_fee"`
	EVMChainID               uint64 `json:"evm_chain_id"`
	DynamicBaseFee           bool   `json:"dynamic_base_fee"`
	DynamicBlobBaseFee       bool   `json:"dynamic_blob_base_fee"`
	TargetGas                uint64 `json:"target_gas"`
	TargetBlobGas            uint64 `json:"target_blob_gas"`
	MaxBlobGas               uint64 `json:"max_blob_gas"`
	BaseFeeChangeDenominator uint64 `json:"base_fee_change_denominator"`
	BlobFeeChangeDenominator uint64 `json:"blob_fee_change_denominator"`
	MinBaseFee               uint64 `json:"min_base_fee"`
	MaxBaseFee               uint64 `json:"max_base_fee"`
	MinBlobBaseFee           uint64 `json:"min_blob_base_fee"`
	MaxBlobBaseFee           uint64 `json:"max_blob_base_fee"`
	MinGas                   uint64 `json:"min_gas"`
	MaxGas                   uint64 `json:"max_gas"`
	RequireNonce             bool   `json:"require_nonce"`
	RequireSigned            bool   `json:"require_signed"`
	FeeCollector             string `json:"fee_collector"`
	FeeDenom                 string `json:"fee_denom"`
	DisplayDenom             string `json:"display_denom"`
	DisplayExponent          uint8  `json:"display_exponent"`
	GasDenom                 string `json:"gas_denom"`
	EVMForkPreset            string `json:"evm_fork_preset"`
	EVMChainConfigJSON       bool   `json:"evm_chain_config_json"`
	StrictEVMStateRoot       bool   `json:"strict_evm_state_root"`
	AllowUnprotectedLegacyTx bool   `json:"allow_unprotected_legacy_tx"`
	MaxBlobSidecarBlobs      uint64 `json:"max_blob_sidecar_blobs"`
	MaxBlobSidecarBytes      uint64 `json:"max_blob_sidecar_bytes"`
}

type bankStatus struct {
	MintAuthority string `json:"mint_authority,omitempty"`
}

type validatorStatus struct {
	Permissionless bool   `json:"permissionless"`
	MinStake       uint64 `json:"min_stake"`
}

type committeeStatus struct {
	EpochLength    uint64            `json:"epoch_length"`
	Size           uint64            `json:"size"`
	MinVotingPower types.VotingPower `json:"min_voting_power"`
	Backend        string            `json:"backend"`
}

type mempoolStatus struct {
	MaxTxBytes         int64  `json:"max_tx_bytes"`
	MaxTxs             int    `json:"max_txs"`
	SeenTTL            string `json:"seen_ttl"`
	MinFee             uint64 `json:"min_fee"`
	EnablePriority     bool   `json:"enable_priority"`
	EnableReplacement  bool   `json:"enable_replacement"`
	ReplacementBumpBPS uint64 `json:"replacement_bump_bps"`
	WALPath            string `json:"wal_path"`
}

type storageStatus struct {
	Backend string `json:"backend"`
}

type p2pStatus struct {
	InitialScore          int64  `json:"initial_score"`
	MaxScore              int64  `json:"max_score"`
	ValidMessageReward    int64  `json:"valid_message_reward"`
	InvalidMessageCost    int64  `json:"invalid_message_cost"`
	RateLimitCost         int64  `json:"rate_limit_cost"`
	BanThreshold          int64  `json:"ban_threshold"`
	MaxMessagesPerWindow  uint64 `json:"max_messages_per_window"`
	WindowResetInterval   string `json:"window_reset_interval"`
	ScoreRecovery         int64  `json:"score_recovery"`
	BanDuration           string `json:"ban_duration"`
	PeerSnapshotsEnabled  bool   `json:"peer_snapshots_enabled"`
	NodeStatusPeerMetrics bool   `json:"node_status_peer_metrics"`
}

type operationalHintsStatus struct {
	UseJSONStatus       string `json:"use_json_status"`
	PeerMetricsLocation string `json:"peer_metrics_location"`
}

func writeStatusJSON(writer io.Writer, cfg config.Config) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(newStatusDocument(cfg))
}

func newStatusDocument(cfg config.Config) statusDocument {
	return statusDocument{
		SchemaVersion: "v1",
		ChainID:       cfg.ChainID,
		Application: applicationStatus{
			Modules: append([]string(nil), cfg.Application.Modules...),
		},
		Execution: executionStatus{
			MinFee:                   cfg.Execution.MinFee,
			BaseFee:                  cfg.Execution.BaseFee,
			BlobBaseFee:              cfg.Execution.BlobBaseFee,
			EVMChainID:               cfg.Execution.EVMChainID,
			DynamicBaseFee:           cfg.Execution.DynamicBaseFee,
			DynamicBlobBaseFee:       cfg.Execution.DynamicBlobBaseFee,
			TargetGas:                cfg.Execution.TargetGas,
			TargetBlobGas:            cfg.Execution.TargetBlobGas,
			MaxBlobGas:               cfg.Execution.MaxBlobGas,
			BaseFeeChangeDenominator: cfg.Execution.BaseFeeChangeDenominator,
			BlobFeeChangeDenominator: cfg.Execution.BlobFeeChangeDenominator,
			MinBaseFee:               cfg.Execution.MinBaseFee,
			MaxBaseFee:               cfg.Execution.MaxBaseFee,
			MinBlobBaseFee:           cfg.Execution.MinBlobBaseFee,
			MaxBlobBaseFee:           cfg.Execution.MaxBlobBaseFee,
			MinGas:                   cfg.Execution.MinGas,
			MaxGas:                   cfg.Execution.MaxGas,
			RequireNonce:             cfg.Execution.RequireNonce,
			RequireSigned:            cfg.Execution.RequireSigned,
			FeeCollector:             cfg.Execution.FeeCollector,
			FeeDenom:                 cfg.Execution.FeeDenom,
			DisplayDenom:             cfg.Execution.DisplayDenom,
			DisplayExponent:          cfg.Execution.DisplayExponent,
			GasDenom:                 cfg.Execution.GasDenom,
			EVMForkPreset:            cfg.Execution.EVMForkPreset,
			EVMChainConfigJSON:       cfg.Execution.EVMChainConfigJSON != "",
			StrictEVMStateRoot:       cfg.Execution.StrictEVMStateRoot,
			AllowUnprotectedLegacyTx: cfg.Execution.AllowUnprotectedLegacyTx,
			MaxBlobSidecarBlobs:      cfg.Execution.MaxBlobSidecarBlobs,
			MaxBlobSidecarBytes:      cfg.Execution.MaxBlobSidecarBytes,
		},
		Bank: bankStatus{
			MintAuthority: cfg.Bank.MintAuthority,
		},
		Validator: validatorStatus{
			Permissionless: cfg.Validator.Permissionless,
			MinStake:       cfg.Validator.MinStake,
		},
		Committee: committeeStatus{
			EpochLength:    cfg.Committee.EpochLength,
			Size:           cfg.Committee.CommitteeSize,
			MinVotingPower: cfg.Committee.MinVotingPower,
			Backend:        string(cfg.Committee.Backend),
		},
		Mempool: mempoolStatus{
			MaxTxBytes:         cfg.Mempool.MaxTxBytes,
			MaxTxs:             cfg.Mempool.MaxTxs,
			SeenTTL:            cfg.Mempool.SeenTTL.String(),
			MinFee:             cfg.Mempool.MinFee,
			EnablePriority:     cfg.Mempool.EnablePriority,
			EnableReplacement:  cfg.Mempool.EnableReplacement,
			ReplacementBumpBPS: cfg.Mempool.ReplacementBumpBPS,
			WALPath:            cfg.Mempool.WALPath,
		},
		Features:         statusFeatures(cfg),
		FeatureAssurance: statusFeatureAssurance(cfg),
		Storage:          storageStatus{Backend: "leveldb"},
		P2P: p2pStatus{
			InitialScore:          cfg.P2P.InitialScore,
			MaxScore:              cfg.P2P.MaxScore,
			ValidMessageReward:    cfg.P2P.ValidMessageReward,
			InvalidMessageCost:    cfg.P2P.InvalidMessageCost,
			RateLimitCost:         cfg.P2P.RateLimitCost,
			BanThreshold:          cfg.P2P.BanThreshold,
			MaxMessagesPerWindow:  cfg.P2P.MaxMessagesPerWindow,
			WindowResetInterval:   cfg.P2P.WindowResetInterval.String(),
			ScoreRecovery:         cfg.P2P.ScoreRecovery,
			BanDuration:           cfg.P2P.BanDuration.String(),
			PeerSnapshotsEnabled:  true,
			NodeStatusPeerMetrics: true,
		},
		OperationalHints: operationalHintsStatus{
			UseJSONStatus:       "vexod status --json",
			PeerMetricsLocation: "node.Status().Peers",
		},
	}
}

func statusFeatures(cfg config.Config) map[string]bool {
	evmEnabled := moduleEnabled(cfg, "evm")
	return map[string]bool{
		"fair_ordering":                            true,
		"height_salted_order":                      true,
		"data_availability":                        true,
		"data_availability_chunk_proofs":           true,
		"data_availability_parity_recovery":        true,
		"data_availability_reed_solomon_recovery":  true,
		"data_availability_deterministic_sampling": true,
		"state_sync_snapshot_kv":                   true,
		"state_sync_checksum":                      true,
		"state_sync_verify":                        true,
		"state_sync_snapshot_chunks":               true,
		"state_sync_historical_replay":             true,
		"web3_receipt_roots":                       evmEnabled,
		"web3_ethereum_trie_roots":                 evmEnabled,
		"web3_global_log_filters":                  evmEnabled,
		"web3_prefix_log_index":                    evmEnabled,
		"web3_filter_limit":                        evmEnabled,
		"web3_geth_compat_methods":                 evmEnabled,
		"web3_txpool_debug_trace":                  evmEnabled,
		"web3_trace_api":                           evmEnabled,
		"web3_access_list_call_trace":              evmEnabled,
		"web3_raw_tx_access_list_prewarm":          evmEnabled,
		"web3_call_vm_trace_state_diff":            evmEnabled,
		"web3_prestate_and_4byte_tracers":          evmEnabled,
		"web3_raw_tx_replay_trace":                 evmEnabled,
		"web3_pending_tx_compat":                   evmEnabled,
		"web3_safe_finalized_tags":                 evmEnabled,
		"web3_jsonrpc_batch_notifications":         evmEnabled,
		"web3_eip1898_block_selectors":             evmEnabled,
		"web3_post_merge_block_fields":             evmEnabled,
		"web3_block_scan_tx_lookup":                evmEnabled,
		"web3_receipt_trace_block_fallback":        evmEnabled,
		"web3_ws_full_pending_transactions":        evmEnabled,
		"evm_geth_vm_adapter":                      evmEnabled,
		"evm_ethereum_raw_tx":                      evmEnabled,
		"evm_storage_writes":                       evmEnabled,
		"evm_code_writes":                          evmEnabled,
		"evm_nonce_writes":                         evmEnabled,
		"evm_selfdestruct_account_deletion":        evmEnabled,
		"evm_actual_gas_accounting":                evmEnabled,
		"web3_call_block_context":                  evmEnabled,
		"web3_historical_code_storage":             evmEnabled,
		"web3_historical_account_state":            evmEnabled,
		"web3_txpool_pending_queued":               evmEnabled,
		"web3_receipt_location_index":              evmEnabled,
		"web3_replay_state_diff":                   evmEnabled,
		"web3_blob_tx_fee_accounting":              evmEnabled,
		"execution_context_aware_app_calls":        true,
		"validator_update_atomic_commit":           true,
		"staking_slashing_ledger":                  moduleEnabled(cfg, "staking"),
		"staking_tombstone_ledger":                 moduleEnabled(cfg, "staking"),
		"ibc_ordering_validation":                  moduleEnabled(cfg, "ibc"),
		"mempool_seen_cache_pruning":               cfg.Mempool.SeenTTL > 0,
		"mempool_wal":                              cfg.Mempool.WALPath != "",
		"mempool_wal_compaction":                   cfg.Mempool.WALPath != "",
		"mempool_priority":                         cfg.Mempool.EnablePriority,
		"mempool_replacement":                      cfg.Mempool.EnableReplacement,
		"ops_metrics_uptime":                       true,
		"ops_pprof_optional":                       true,
		"ops_structured_logs":                      true,
		"ops_release_artifacts":                    true,
		"ops_release_evidence_content_gate":        true,
		"ops_release_evidence_semantic_gate":       true,
		"ops_external_audit_pack":                  true,
		"ops_deployment_template":                  true,
		"ops_longrun_network_plan":                 true,
		"security_fuzz_targets":                    true,
		"security_strict_json_rpc":                 true,
		"security_forwarded_for_untrusted":         true,
		"consensus_adversarial_simulation":         true,
		"consensus_partition_safety":               true,
		"consensus_tendermint_timeouts":            true,
		"consensus_empty_block_control":            true,
		"consensus_app_hash_evidence":              true,
		"consensus_tx_validity_evidence":           true,
		"crypto_remote_signer_verification":        true,
		"crypto_bls_adapter_required":              cfg.Crypto.Backend == config.CryptoBackendBLS,
		"crypto_bls_production_adapter":            cfg.Crypto.Backend == config.CryptoBackendBLS && cfg.Crypto.ProductionAdapter,
		"crypto_vrf_production_adapter":            cfg.VRF.ProductionAdapter,
		"deployment_audit":                         true,
		"addr_book":                                true,
		"addr_book_ban_evict":                      cfg.P2P.BanDuration > 0,
		"peer_dial_tracking":                       true,
		"transport_peer_gate":                      true,
		"transport_tls_configurable":               true,
		"consensus_gossip_scoring":                 true,
		"banned_peer_disconnect":                   true,
		"peer_score_persistence":                   true,
		"leveldb_storage":                          true,
		"peer_scoring":                             true,
		"temporary_peer_bans":                      cfg.P2P.BanDuration > 0,
		"peer_score_recovery":                      cfg.P2P.ScoreRecovery > 0,
	}
}

func statusFeatureAssurance(cfg config.Config) map[string]assuranceStatus {
	features := statusFeatures(cfg)
	assurance := make(map[string]assuranceStatus, len(features))
	for feature, enabled := range features {
		if !enabled {
			assurance[feature] = assuranceStatus{State: "disabled", Note: "feature is not enabled by current config"}
			continue
		}
		assurance[feature] = assuranceStatus{State: "implemented", Note: "code path is enabled; release evidence may still be required"}
	}
	for _, feature := range []string{
		"web3_geth_compat_methods",
		"web3_txpool_debug_trace",
		"web3_trace_api",
		"web3_raw_tx_replay_trace",
		"web3_blob_tx_fee_accounting",
		"evm_geth_vm_adapter",
		"evm_ethereum_raw_tx",
		"evm_actual_gas_accounting",
	} {
		if features[feature] {
			assurance[feature] = assuranceStatus{
				State: "requires_release_evidence",
				Note:  "EVM/Web3 compatibility should be backed by fixture/conformance evidence before release claims",
			}
		}
	}
	for _, feature := range []string{
		"ops_longrun_network_plan",
		"ops_release_evidence_content_gate",
		"ops_release_evidence_semantic_gate",
		"ops_external_audit_pack",
	} {
		if features[feature] {
			assurance[feature] = assuranceStatus{
				State: "requires_operator_artifact",
				Note:  "the tool exists, but release readiness depends on generated evidence artifacts",
			}
		}
	}
	for _, feature := range []string{
		"crypto_bls_adapter_required",
		"crypto_bls_production_adapter",
		"crypto_vrf_production_adapter",
	} {
		if features[feature] {
			assurance[feature] = assuranceStatus{
				State: "requires_external_audit_evidence",
				Note:  "cryptographic adapter safety depends on configured audited adapter metadata and attached audit evidence",
			}
		}
	}
	return assurance
}

func moduleEnabled(cfg config.Config, name string) bool {
	for _, module := range cfg.Application.Modules {
		if module == name {
			return true
		}
	}
	return false
}

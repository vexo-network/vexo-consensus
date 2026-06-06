package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/types"
)

func writeStatus(writer io.Writer, cfg config.Config) {
	fmt.Fprintf(writer, "vexo-consensus status\n")
	fmt.Fprintf(writer, "chain_id: %s\n", cfg.ChainID)
	fmt.Fprintf(writer, "application.modules: %v\n", cfg.Application.Modules)
	fmt.Fprintf(writer, "execution.min_fee: %d\n", cfg.Execution.MinFee)
	fmt.Fprintf(writer, "execution.base_fee: %d\n", cfg.Execution.BaseFee)
	fmt.Fprintf(writer, "execution.dynamic_base_fee: %t\n", cfg.Execution.DynamicBaseFee)
	fmt.Fprintf(writer, "execution.target_gas: %d\n", cfg.Execution.TargetGas)
	fmt.Fprintf(writer, "execution.base_fee_change_denominator: %d\n", cfg.Execution.BaseFeeChangeDenominator)
	fmt.Fprintf(writer, "execution.min_base_fee: %d\n", cfg.Execution.MinBaseFee)
	fmt.Fprintf(writer, "execution.max_base_fee: %d\n", cfg.Execution.MaxBaseFee)
	fmt.Fprintf(writer, "execution.min_gas: %d\n", cfg.Execution.MinGas)
	fmt.Fprintf(writer, "execution.max_gas: %d\n", cfg.Execution.MaxGas)
	fmt.Fprintf(writer, "execution.require_nonce: %t\n", cfg.Execution.RequireNonce)
	fmt.Fprintf(writer, "execution.require_signed: %t\n", cfg.Execution.RequireSigned)
	fmt.Fprintf(writer, "execution.fee_collector: %s\n", cfg.Execution.FeeCollector)
	fmt.Fprintf(writer, "execution.fee_denom: %s\n", cfg.Execution.FeeDenom)
	fmt.Fprintf(writer, "execution.display_denom: %s\n", cfg.Execution.DisplayDenom)
	fmt.Fprintf(writer, "execution.display_exponent: %d\n", cfg.Execution.DisplayExponent)
	fmt.Fprintf(writer, "execution.gas_denom: %s\n", cfg.Execution.GasDenom)
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
	fmt.Fprintf(writer, "mempool.wal_path: %s\n", cfg.Mempool.WALPath)
	fmt.Fprintf(writer, "fair_ordering.deterministic: true\n")
	fmt.Fprintf(writer, "fair_ordering.height_salted: true\n")
	fmt.Fprintf(writer, "data_availability.commitments: true\n")
	fmt.Fprintf(writer, "data_availability.chunk_proofs: true\n")
	fmt.Fprintf(writer, "data_availability.parity_recovery: true\n")
	fmt.Fprintf(writer, "data_availability.reed_solomon_recovery: true\n")
	fmt.Fprintf(writer, "storage.backend: leveldb\n")
	fmt.Fprintf(writer, "state_sync.snapshot_kv: true\n")
	fmt.Fprintf(writer, "state_sync.snapshot_checksum: true\n")
	fmt.Fprintf(writer, "state_sync.snapshot_verify: true\n")
	fmt.Fprintf(writer, "state_sync.snapshot_chunks: true\n")
	fmt.Fprintf(writer, "staking.slashing_ledger: true\n")
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
	fmt.Fprintf(writer, "consensus.adversarial_simulation: true\n")
	fmt.Fprintf(writer, "consensus.partition_safety_simulation: true\n")
	fmt.Fprintf(writer, "consensus.tendermint_style_timeouts: true\n")
	fmt.Fprintf(writer, "consensus.empty_block_control: true\n")
	fmt.Fprintf(writer, "consensus.tx_validity_evidence: true\n")
	fmt.Fprintf(writer, "crypto.backend: %s\n", cfg.Crypto.Backend)
	fmt.Fprintf(writer, "crypto.production_adapter: %t\n", cfg.Crypto.ProductionAdapter)
	fmt.Fprintf(writer, "crypto.remote_signer_verification: true\n")
	fmt.Fprintf(writer, "crypto.bls_adapter_required: true\n")
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
	SchemaVersion    string                 `json:"schema_version"`
	ChainID          string                 `json:"chain_id"`
	Application      applicationStatus      `json:"application"`
	Execution        executionStatus        `json:"execution"`
	Bank             bankStatus             `json:"bank"`
	Validator        validatorStatus        `json:"validator"`
	Committee        committeeStatus        `json:"committee"`
	Mempool          mempoolStatus          `json:"mempool"`
	Features         map[string]bool        `json:"features"`
	Storage          storageStatus          `json:"storage"`
	P2P              p2pStatus              `json:"p2p"`
	OperationalHints operationalHintsStatus `json:"operational_hints"`
}

type applicationStatus struct {
	Modules []string `json:"modules"`
}

type executionStatus struct {
	MinFee                   uint64 `json:"min_fee"`
	BaseFee                  uint64 `json:"base_fee"`
	DynamicBaseFee           bool   `json:"dynamic_base_fee"`
	TargetGas                uint64 `json:"target_gas"`
	BaseFeeChangeDenominator uint64 `json:"base_fee_change_denominator"`
	MinBaseFee               uint64 `json:"min_base_fee"`
	MaxBaseFee               uint64 `json:"max_base_fee"`
	MinGas                   uint64 `json:"min_gas"`
	MaxGas                   uint64 `json:"max_gas"`
	RequireNonce             bool   `json:"require_nonce"`
	RequireSigned            bool   `json:"require_signed"`
	FeeCollector             string `json:"fee_collector"`
	FeeDenom                 string `json:"fee_denom"`
	DisplayDenom             string `json:"display_denom"`
	DisplayExponent          uint8  `json:"display_exponent"`
	GasDenom                 string `json:"gas_denom"`
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
	MaxTxBytes     int64  `json:"max_tx_bytes"`
	MaxTxs         int    `json:"max_txs"`
	SeenTTL        string `json:"seen_ttl"`
	MinFee         uint64 `json:"min_fee"`
	EnablePriority bool   `json:"enable_priority"`
	WALPath        string `json:"wal_path"`
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
			DynamicBaseFee:           cfg.Execution.DynamicBaseFee,
			TargetGas:                cfg.Execution.TargetGas,
			BaseFeeChangeDenominator: cfg.Execution.BaseFeeChangeDenominator,
			MinBaseFee:               cfg.Execution.MinBaseFee,
			MaxBaseFee:               cfg.Execution.MaxBaseFee,
			MinGas:                   cfg.Execution.MinGas,
			MaxGas:                   cfg.Execution.MaxGas,
			RequireNonce:             cfg.Execution.RequireNonce,
			RequireSigned:            cfg.Execution.RequireSigned,
			FeeCollector:             cfg.Execution.FeeCollector,
			FeeDenom:                 cfg.Execution.FeeDenom,
			DisplayDenom:             cfg.Execution.DisplayDenom,
			DisplayExponent:          cfg.Execution.DisplayExponent,
			GasDenom:                 cfg.Execution.GasDenom,
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
			MaxTxBytes:     cfg.Mempool.MaxTxBytes,
			MaxTxs:         cfg.Mempool.MaxTxs,
			SeenTTL:        cfg.Mempool.SeenTTL.String(),
			MinFee:         cfg.Mempool.MinFee,
			EnablePriority: cfg.Mempool.EnablePriority,
			WALPath:        cfg.Mempool.WALPath,
		},
		Features: map[string]bool{
			"fair_ordering":                           true,
			"height_salted_order":                     true,
			"data_availability":                       true,
			"data_availability_chunk_proofs":          true,
			"data_availability_parity_recovery":       true,
			"data_availability_reed_solomon_recovery": true,
			"state_sync_snapshot_kv":                  true,
			"state_sync_checksum":                     true,
			"state_sync_verify":                       true,
			"state_sync_snapshot_chunks":              true,
			"state_sync_historical_replay":            true,
			"web3_receipt_roots":                      true,
			"web3_ethereum_trie_roots":                true,
			"web3_global_log_filters":                 true,
			"web3_prefix_log_index":                   true,
			"web3_filter_limit":                       true,
			"web3_geth_compat_methods":                true,
			"web3_txpool_debug_trace":                 true,
			"web3_trace_api":                          true,
			"web3_access_list_call_trace":             true,
			"web3_raw_tx_replay_trace":                true,
			"evm_geth_vm_adapter":                     true,
			"evm_ethereum_raw_tx":                     true,
			"evm_storage_writes":                      true,
			"evm_code_writes":                         true,
			"evm_selfdestruct_account_deletion":       true,
			"evm_actual_gas_accounting":               true,
			"validator_update_atomic_commit":          true,
			"staking_slashing_ledger":                 true,
			"mempool_seen_cache_pruning":              true,
			"ops_metrics_uptime":                      true,
			"ops_pprof_optional":                      true,
			"ops_structured_logs":                     true,
			"ops_release_artifacts":                   true,
			"ops_external_audit_pack":                 true,
			"ops_deployment_template":                 true,
			"ops_longrun_network_plan":                true,
			"security_fuzz_targets":                   true,
			"security_strict_json_rpc":                true,
			"security_forwarded_for_untrusted":        true,
			"consensus_adversarial_simulation":        true,
			"consensus_partition_safety":              true,
			"consensus_tendermint_timeouts":           true,
			"consensus_empty_block_control":           true,
			"consensus_app_hash_evidence":             true,
			"consensus_tx_validity_evidence":          true,
			"crypto_remote_signer_verification":       true,
			"crypto_bls_adapter_required":             true,
			"deployment_audit":                        true,
			"addr_book":                               true,
			"addr_book_ban_evict":                     true,
			"peer_dial_tracking":                      true,
			"transport_peer_gate":                     true,
			"consensus_gossip_scoring":                true,
			"banned_peer_disconnect":                  true,
			"peer_score_persistence":                  true,
			"leveldb_storage":                         true,
			"peer_scoring":                            true,
			"temporary_peer_bans":                     true,
			"peer_score_recovery":                     true,
		},
		Storage: storageStatus{Backend: "leveldb"},
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

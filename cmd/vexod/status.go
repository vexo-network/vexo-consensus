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
	fmt.Fprintf(writer, "execution.min_gas: %d\n", cfg.Execution.MinGas)
	fmt.Fprintf(writer, "execution.max_gas: %d\n", cfg.Execution.MaxGas)
	fmt.Fprintf(writer, "execution.require_nonce: %t\n", cfg.Execution.RequireNonce)
	fmt.Fprintf(writer, "execution.require_signed: %t\n", cfg.Execution.RequireSigned)
	fmt.Fprintf(writer, "execution.fee_collector: %s\n", cfg.Execution.FeeCollector)
	fmt.Fprintf(writer, "validator.permissionless: %t\n", cfg.Validator.Permissionless)
	fmt.Fprintf(writer, "validator.min_stake: %d\n", cfg.Validator.MinStake)
	fmt.Fprintf(writer, "committee.epoch_length: %d\n", cfg.Committee.EpochLength)
	fmt.Fprintf(writer, "committee.size: %d\n", cfg.Committee.CommitteeSize)
	fmt.Fprintf(writer, "mempool.max_txs: %d\n", cfg.Mempool.MaxTxs)
	fmt.Fprintf(writer, "mempool.min_fee: %d\n", cfg.Mempool.MinFee)
	fmt.Fprintf(writer, "mempool.priority_enabled: %t\n", cfg.Mempool.EnablePriority)
	fmt.Fprintf(writer, "fair_ordering.deterministic: true\n")
	fmt.Fprintf(writer, "fair_ordering.height_salted: true\n")
	fmt.Fprintf(writer, "data_availability.commitments: true\n")
	fmt.Fprintf(writer, "storage.backend: leveldb\n")
	fmt.Fprintf(writer, "addr_book.persistent: true\n")
	fmt.Fprintf(writer, "addr_book.dial_failure_tracking: true\n")
	fmt.Fprintf(writer, "addr_book.ban_eviction_policy: true\n")
	fmt.Fprintf(writer, "p2p.initial_score: %d\n", cfg.P2P.InitialScore)
	fmt.Fprintf(writer, "p2p.valid_message_reward: %d\n", cfg.P2P.ValidMessageReward)
	fmt.Fprintf(writer, "p2p.invalid_message_cost: %d\n", cfg.P2P.InvalidMessageCost)
	fmt.Fprintf(writer, "p2p.rate_limit_cost: %d\n", cfg.P2P.RateLimitCost)
	fmt.Fprintf(writer, "p2p.ban_threshold: %d\n", cfg.P2P.BanThreshold)
	fmt.Fprintf(writer, "p2p.max_messages_per_window: %d\n", cfg.P2P.MaxMessagesPerWindow)
	fmt.Fprintf(writer, "p2p.window_reset_interval: %s\n", cfg.P2P.WindowResetInterval)
	fmt.Fprintf(writer, "p2p.score_recovery: %d\n", cfg.P2P.ScoreRecovery)
	fmt.Fprintf(writer, "p2p.ban_duration: %s\n", cfg.P2P.BanDuration)
}

type statusDocument struct {
	SchemaVersion    string                 `json:"schema_version"`
	ChainID          string                 `json:"chain_id"`
	Application      applicationStatus      `json:"application"`
	Execution        executionStatus        `json:"execution"`
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
	MinFee        uint64 `json:"min_fee"`
	MinGas        uint64 `json:"min_gas"`
	MaxGas        uint64 `json:"max_gas"`
	RequireNonce  bool   `json:"require_nonce"`
	RequireSigned bool   `json:"require_signed"`
	FeeCollector  string `json:"fee_collector"`
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
}

type storageStatus struct {
	Backend string `json:"backend"`
}

type p2pStatus struct {
	InitialScore          int64  `json:"initial_score"`
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
			MinFee:        cfg.Execution.MinFee,
			MinGas:        cfg.Execution.MinGas,
			MaxGas:        cfg.Execution.MaxGas,
			RequireNonce:  cfg.Execution.RequireNonce,
			RequireSigned: cfg.Execution.RequireSigned,
			FeeCollector:  cfg.Execution.FeeCollector,
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
		},
		Features: map[string]bool{
			"fair_ordering":       true,
			"height_salted_order": true,
			"data_availability":   true,
			"deployment_audit":    true,
			"addr_book":           true,
			"addr_book_ban_evict": true,
			"peer_dial_tracking":  true,
			"leveldb_storage":     true,
			"peer_scoring":        true,
			"temporary_peer_bans": true,
			"peer_score_recovery": true,
		},
		Storage: storageStatus{Backend: "leveldb"},
		P2P: p2pStatus{
			InitialScore:          cfg.P2P.InitialScore,
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

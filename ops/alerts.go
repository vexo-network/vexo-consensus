package ops

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

var ErrInvalidThresholds = errors.New("invalid alert thresholds")

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Thresholds struct {
	MinHeightRatePerMinute      float64       `json:"min_height_rate_per_minute"`
	MaxRoundTimeoutsPerMinute   float64       `json:"max_round_timeouts_per_minute"`
	MaxProposalLatency          time.Duration `json:"max_proposal_latency"`
	MaxVoteLatency              time.Duration `json:"max_vote_latency"`
	MaxPeerBans                 uint64        `json:"max_peer_bans"`
	MaxMempoolSize              uint64        `json:"max_mempool_size"`
	MaxCommitLatency            time.Duration `json:"max_commit_latency"`
	SnapshotRequired            bool          `json:"snapshot_required"`
	ReplayHealthyRequired       bool          `json:"replay_healthy_required"`
	MaxValidatorSigningFailures uint64        `json:"max_validator_signing_failures"`
	MaxReconciliationFailures   uint64        `json:"max_post_commit_reconciliation_failures"`
}

type Sample struct {
	HeightRatePerMinute      float64       `json:"height_rate_per_minute"`
	RoundTimeoutsPerMinute   float64       `json:"round_timeouts_per_minute"`
	ProposalLatency          time.Duration `json:"proposal_latency"`
	VoteLatency              time.Duration `json:"vote_latency"`
	PeerBans                 uint64        `json:"peer_bans"`
	MempoolSize              uint64        `json:"mempool_size"`
	CommitLatency            time.Duration `json:"commit_latency"`
	SnapshotHealthy          bool          `json:"snapshot_healthy"`
	ReplayHealthy            bool          `json:"replay_healthy"`
	ValidatorSigningFailures uint64        `json:"validator_signing_failures"`
	ReconciliationFailures   uint64        `json:"post_commit_reconciliation_failures"`
}

type Alert struct {
	Name      string   `json:"name"`
	Severity  Severity `json:"severity"`
	Value     string   `json:"value"`
	Threshold string   `json:"threshold"`
	Message   string   `json:"message"`
}

type Report struct {
	OK     bool    `json:"ok"`
	Alerts []Alert `json:"alerts"`
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		MinHeightRatePerMinute:      1,
		MaxRoundTimeoutsPerMinute:   3,
		MaxProposalLatency:          250 * time.Millisecond,
		MaxVoteLatency:              250 * time.Millisecond,
		MaxPeerBans:                 0,
		MaxMempoolSize:              10_000,
		MaxCommitLatency:            500 * time.Millisecond,
		SnapshotRequired:            true,
		ReplayHealthyRequired:       true,
		MaxValidatorSigningFailures: 0,
		MaxReconciliationFailures:   0,
	}
}

func (thresholds Thresholds) Validate() error {
	if thresholds.MinHeightRatePerMinute < 0 ||
		thresholds.MaxRoundTimeoutsPerMinute < 0 ||
		thresholds.MaxProposalLatency < 0 ||
		thresholds.MaxVoteLatency < 0 ||
		thresholds.MaxCommitLatency < 0 {
		return ErrInvalidThresholds
	}
	return nil
}

func Evaluate(sample Sample, thresholds Thresholds) (Report, error) {
	if err := thresholds.Validate(); err != nil {
		return Report{}, err
	}
	report := Report{OK: true}
	add := func(alert Alert) {
		report.OK = false
		report.Alerts = append(report.Alerts, alert)
	}
	if sample.HeightRatePerMinute < thresholds.MinHeightRatePerMinute {
		add(Alert{Name: "height_rate", Severity: SeverityCritical, Value: formatFloat(sample.HeightRatePerMinute), Threshold: ">=" + formatFloat(thresholds.MinHeightRatePerMinute), Message: "block production is stalled or too slow"})
	}
	if sample.RoundTimeoutsPerMinute > thresholds.MaxRoundTimeoutsPerMinute {
		add(Alert{Name: "round_timeouts", Severity: SeverityWarning, Value: formatFloat(sample.RoundTimeoutsPerMinute), Threshold: "<=" + formatFloat(thresholds.MaxRoundTimeoutsPerMinute), Message: "round timeout frequency is above normal"})
	}
	if sample.ProposalLatency > thresholds.MaxProposalLatency {
		add(Alert{Name: "proposal_latency", Severity: SeverityWarning, Value: sample.ProposalLatency.String(), Threshold: "<=" + thresholds.MaxProposalLatency.String(), Message: "proposal processing latency is high"})
	}
	if sample.VoteLatency > thresholds.MaxVoteLatency {
		add(Alert{Name: "vote_latency", Severity: SeverityWarning, Value: sample.VoteLatency.String(), Threshold: "<=" + thresholds.MaxVoteLatency.String(), Message: "vote processing latency is high"})
	}
	if sample.PeerBans > thresholds.MaxPeerBans {
		add(Alert{Name: "peer_bans", Severity: SeverityWarning, Value: formatUint(sample.PeerBans), Threshold: "<=" + formatUint(thresholds.MaxPeerBans), Message: "peer ban count indicates networking abuse or misconfiguration"})
	}
	if sample.MempoolSize > thresholds.MaxMempoolSize {
		add(Alert{Name: "mempool_size", Severity: SeverityWarning, Value: formatUint(sample.MempoolSize), Threshold: "<=" + formatUint(thresholds.MaxMempoolSize), Message: "mempool backlog is above target"})
	}
	if sample.CommitLatency > thresholds.MaxCommitLatency {
		add(Alert{Name: "commit_latency", Severity: SeverityCritical, Value: sample.CommitLatency.String(), Threshold: "<=" + thresholds.MaxCommitLatency.String(), Message: "commit latency threatens fast finality"})
	}
	if thresholds.SnapshotRequired && !sample.SnapshotHealthy {
		add(Alert{Name: "snapshot", Severity: SeverityCritical, Value: "false", Threshold: "true", Message: "snapshot export/verify path is unhealthy"})
	}
	if thresholds.ReplayHealthyRequired && !sample.ReplayHealthy {
		add(Alert{Name: "replay", Severity: SeverityCritical, Value: "false", Threshold: "true", Message: "replay or recovery verification is unhealthy"})
	}
	if sample.ValidatorSigningFailures > thresholds.MaxValidatorSigningFailures {
		add(Alert{Name: "validator_signing_failures", Severity: SeverityCritical, Value: formatUint(sample.ValidatorSigningFailures), Threshold: "<=" + formatUint(thresholds.MaxValidatorSigningFailures), Message: "validator signer or KMS is failing"})
	}
	if sample.ReconciliationFailures > thresholds.MaxReconciliationFailures {
		add(Alert{Name: "post_commit_reconciliation", Severity: SeverityCritical, Value: formatUint(sample.ReconciliationFailures), Threshold: "<=" + formatUint(thresholds.MaxReconciliationFailures), Message: "post-commit reconciliation recovered from a partial durable commit and needs operator review"})
	}
	return report, nil
}

func MarshalThresholdsJSON(thresholds Thresholds) ([]byte, error) {
	return json.MarshalIndent(thresholds, "", "  ")
}

func formatFloat(value float64) string {
	return trimFloat(value)
}

func formatUint(value uint64) string {
	return trimFloat(float64(value))
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

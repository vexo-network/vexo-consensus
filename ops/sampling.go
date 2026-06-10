package ops

import (
	"errors"
	"time"
)

var ErrInvalidSampleWindow = errors.New("invalid metrics sample window")

type MetricsSnapshot struct {
	LatestHeight         uint64  `json:"latest_height"`
	HeightRatePerMinute  float64 `json:"height_rate_per_minute"`
	RoundTimeouts        uint64  `json:"round_timeouts"`
	ProposalLatencyNanos uint64  `json:"proposal_latency_nanos"`
	VoteLatencyNanos     uint64  `json:"vote_latency_nanos"`
	BannedPeers          int     `json:"banned_peers"`
	MempoolSize          uint64  `json:"mempool_size"`
	CommitLatencyNanos   uint64  `json:"commit_latency_nanos"`
	SnapshotHealthy      bool    `json:"snapshot_healthy"`
	ReplayHealthy        bool    `json:"replay_healthy"`
	SigningFailures      uint64  `json:"validator_signing_failures"`
	ReconciliationFails  uint64  `json:"post_commit_reconciliation_failures"`
}

func SampleFromMetricsSnapshot(previous *MetricsSnapshot, current MetricsSnapshot, window time.Duration) (Sample, error) {
	sample := Sample{
		HeightRatePerMinute:      current.HeightRatePerMinute,
		ProposalLatency:          time.Duration(current.ProposalLatencyNanos),
		VoteLatency:              time.Duration(current.VoteLatencyNanos),
		PeerBans:                 uint64(nonNegativeInt(current.BannedPeers)),
		MempoolSize:              current.MempoolSize,
		CommitLatency:            time.Duration(current.CommitLatencyNanos),
		SnapshotHealthy:          current.SnapshotHealthy,
		ReplayHealthy:            current.ReplayHealthy,
		ValidatorSigningFailures: current.SigningFailures,
		ReconciliationFailures:   current.ReconciliationFails,
	}
	if previous == nil {
		sample.RoundTimeoutsPerMinute = float64(current.RoundTimeouts)
		return sample, nil
	}
	if window <= 0 {
		return Sample{}, ErrInvalidSampleWindow
	}
	minutes := window.Minutes()
	if current.LatestHeight >= previous.LatestHeight {
		sample.HeightRatePerMinute = float64(current.LatestHeight-previous.LatestHeight) / minutes
	}
	if current.RoundTimeouts >= previous.RoundTimeouts {
		sample.RoundTimeoutsPerMinute = float64(current.RoundTimeouts-previous.RoundTimeouts) / minutes
	}
	return sample, nil
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

package ops

import (
	"errors"
	"testing"
	"time"
)

func TestEvaluateReportsOperationalRisk(t *testing.T) {
	report, err := Evaluate(Sample{
		HeightRatePerMinute:      0,
		RoundTimeoutsPerMinute:   10,
		ProposalLatency:          time.Duration(1_000_000_000),
		VoteLatency:              time.Duration(1_000_000_000),
		PeerBans:                 2,
		MempoolSize:              20_000,
		CommitLatency:            time.Duration(2_000_000_000),
		SnapshotHealthy:          false,
		ReplayHealthy:            false,
		ValidatorSigningFailures: 1,
		ReconciliationFailures:   1,
	}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.Alerts) != 11 {
		t.Fatalf("expected 11 alerts, got %+v", report)
	}
}

func TestEvaluateAcceptsHealthySample(t *testing.T) {
	report, err := Evaluate(Sample{
		HeightRatePerMinute: 10,
		SnapshotHealthy:     true,
		ReplayHealthy:       true,
	}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || len(report.Alerts) != 0 {
		t.Fatalf("expected healthy report, got %+v", report)
	}
}

func TestThresholdsValidate(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.MaxCommitLatency = -time.Duration(1_000_000_000)
	if _, err := Evaluate(Sample{}, thresholds); !errors.Is(err, ErrInvalidThresholds) {
		t.Fatalf("expected invalid thresholds, got %v", err)
	}
}

func TestSampleFromMetricsSnapshotCalculatesRates(t *testing.T) {
	previous := MetricsSnapshot{LatestHeight: 10, RoundTimeouts: 2}
	current := MetricsSnapshot{
		LatestHeight:         16,
		RoundTimeouts:        5,
		ProposalLatencyNanos: uint64((250 * time.Millisecond).Nanoseconds()),
		VoteLatencyNanos:     uint64((100 * time.Millisecond).Nanoseconds()),
		BannedPeers:          2,
		MempoolSize:          9,
		CommitLatencyNanos:   uint64((400 * time.Millisecond).Nanoseconds()),
		SnapshotHealthy:      true,
		ReplayHealthy:        true,
		SigningFailures:      1,
		ReconciliationFails:  2,
	}
	sample, err := SampleFromMetricsSnapshot(&previous, current, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if sample.HeightRatePerMinute != 3 || sample.RoundTimeoutsPerMinute != 1.5 {
		t.Fatalf("unexpected rates: %+v", sample)
	}
	if sample.ProposalLatency != 250*time.Millisecond || sample.PeerBans != 2 || sample.ValidatorSigningFailures != 1 || sample.ReconciliationFailures != 2 {
		t.Fatalf("unexpected sample fields: %+v", sample)
	}
}

func TestSampleFromMetricsSnapshotRequiresWindowForDelta(t *testing.T) {
	previous := MetricsSnapshot{LatestHeight: 1}
	if _, err := SampleFromMetricsSnapshot(&previous, MetricsSnapshot{LatestHeight: 2}, 0); !errors.Is(err, ErrInvalidSampleWindow) {
		t.Fatalf("expected invalid sample window, got %v", err)
	}
}

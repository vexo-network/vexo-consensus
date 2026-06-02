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
	}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.Alerts) != 10 {
		t.Fatalf("expected 10 alerts, got %+v", report)
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

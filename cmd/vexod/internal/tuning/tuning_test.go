package tuning

import (
	"strings"
	"testing"
	"time"
)

func TestRecommendProducesSafeDefaults(t *testing.T) {
	recommendation := Recommend(Inputs{
		Validators:     100,
		TargetTPS:      5_000,
		Regions:        3,
		AverageLatency: 120 * time.Millisecond,
		BlockBytes:     8 * 1024 * 1024,
	})

	if recommendation.SchemaVersion != "v1" {
		t.Fatalf("unexpected schema version %q", recommendation.SchemaVersion)
	}
	if recommendation.Inputs.QuorumVotingPower != 67 {
		t.Fatalf("unexpected quorum: %d", recommendation.Inputs.QuorumVotingPower)
	}
	if recommendation.Consensus.CommitteeSize < 64 {
		t.Fatalf("committee size should scale up, got %d", recommendation.Consensus.CommitteeSize)
	}
	if recommendation.Networking.OutboundPeers <= 0 || recommendation.Mempool.MaxTxs <= 0 {
		t.Fatalf("expected positive network and mempool recommendations: %+v", recommendation)
	}
	for _, check := range recommendation.Validation {
		if !check.OK {
			t.Fatalf("expected safe input validation to pass, failed %s: %+v", check.Name, recommendation)
		}
	}
}

func TestRecommendWarnsWhenFaultBudgetExceeded(t *testing.T) {
	recommendation := Recommend(Inputs{
		Validators:      10,
		TargetTPS:       100,
		Regions:         2,
		AverageLatency:  100 * time.Millisecond,
		FaultPercentage: 40,
	})

	if len(recommendation.Warnings) == 0 {
		t.Fatalf("expected warning for excessive fault percentage")
	}
	if !strings.Contains(strings.Join(recommendation.Warnings, "\n"), "fault percentage") {
		t.Fatalf("unexpected warnings: %+v", recommendation.Warnings)
	}
}

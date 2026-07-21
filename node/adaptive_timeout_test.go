package node

import (
	"testing"
	"time"
)

func TestRecommendAdaptiveRoundTimeoutGrowsOnTimeout(t *testing.T) {
	base := 100 * time.Millisecond
	current := 100 * time.Millisecond
	snapshot := nodeMetricsSnapshot{
		proposalP95Nanos: uint64((15 * time.Millisecond).Nanoseconds()),
		voteP95Nanos:     uint64((10 * time.Millisecond).Nanoseconds()),
		commitP95Nanos:   uint64((5 * time.Millisecond).Nanoseconds()),
	}

	next := recommendAdaptiveRoundTimeout(base, current, snapshot, false, true, 4, 4)
	if next <= current {
		t.Fatalf("expected timeout to grow, current=%s next=%s", current, next)
	}
	if next < 90*time.Millisecond {
		t.Fatalf("expected next timeout to respect observed budget, got %s", next)
	}
}

func TestRecommendAdaptiveRoundTimeoutShrinksOnProgress(t *testing.T) {
	base := 100 * time.Millisecond
	current := 400 * time.Millisecond
	snapshot := nodeMetricsSnapshot{
		proposalP95Nanos: uint64((5 * time.Millisecond).Nanoseconds()),
		voteP95Nanos:     uint64((5 * time.Millisecond).Nanoseconds()),
		commitP95Nanos:   uint64((5 * time.Millisecond).Nanoseconds()),
	}

	next := recommendAdaptiveRoundTimeout(base, current, snapshot, true, false, 4, 4)
	if next >= current {
		t.Fatalf("expected timeout to shrink, current=%s next=%s", current, next)
	}
	if next < base {
		t.Fatalf("expected timeout to stay above base, got %s", next)
	}
}

func TestRecommendAdaptiveRoundTimeoutCapsGrowth(t *testing.T) {
	base := 100 * time.Millisecond
	current := 700 * time.Millisecond
	snapshot := nodeMetricsSnapshot{
		proposalP95Nanos: uint64((100 * time.Millisecond).Nanoseconds()),
		voteP95Nanos:     uint64((100 * time.Millisecond).Nanoseconds()),
		commitP95Nanos:   uint64((100 * time.Millisecond).Nanoseconds()),
	}

	next := recommendAdaptiveRoundTimeout(base, current, snapshot, false, true, 4, 4)
	if next > 800*time.Millisecond {
		t.Fatalf("expected timeout to cap at 8x base, got %s", next)
	}
}

func TestRecommendAdaptiveRoundTimeoutWidensWhenPeersAreMissing(t *testing.T) {
	base := 100 * time.Millisecond
	current := 100 * time.Millisecond
	snapshot := nodeMetricsSnapshot{}

	next := recommendAdaptiveRoundTimeout(base, current, snapshot, false, false, 0, 4)
	if next <= base {
		t.Fatalf("expected timeout to widen when peers are missing, base=%s next=%s", base, next)
	}
	if next < 200*time.Millisecond {
		t.Fatalf("expected missing peers to at least double the floor, got %s", next)
	}
}

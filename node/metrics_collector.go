package node

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const metricsDurationWindowSize = 256

type nodeMetrics struct {
	roundTimeouts        atomic.Uint64
	adaptiveTimeoutNanos atomic.Uint64
	recoveryDeferrals    atomic.Uint64
	proposalLatencyNanos atomic.Uint64
	voteLatencyNanos     atomic.Uint64
	commitLatencyNanos   atomic.Uint64
	signingFailures      atomic.Uint64
	committedBlocks      atomic.Uint64
	firstCommitUnixNano  atomic.Int64
	latestCommitUnixNano atomic.Int64
	proposalWindow       durationWindow
	voteWindow           durationWindow
	commitWindow         durationWindow
}

type nodeMetricsSnapshot struct {
	roundTimeouts        uint64
	adaptiveTimeoutNanos uint64
	recoveryDeferrals    uint64
	proposalLatencyNanos uint64
	voteLatencyNanos     uint64
	commitLatencyNanos   uint64
	proposalP95Nanos     uint64
	proposalP99Nanos     uint64
	voteP95Nanos         uint64
	voteP99Nanos         uint64
	commitP95Nanos       uint64
	commitP99Nanos       uint64
	signingFailures      uint64
	committedBlocks      uint64
	firstCommitUnixNano  int64
	latestCommitUnixNano int64
}

func (metrics *nodeMetrics) observeRoundTimeout() {
	metrics.roundTimeouts.Add(1)
}

func (metrics *nodeMetrics) observeAdaptiveTimeout(duration time.Duration) {
	storeDurationNanos(&metrics.adaptiveTimeoutNanos, duration)
}

func (metrics *nodeMetrics) observeRecoveryFinalityDeferral() {
	metrics.recoveryDeferrals.Add(1)
}

func (metrics *nodeMetrics) observeProposalLatency(duration time.Duration) {
	storeDurationNanos(&metrics.proposalLatencyNanos, duration)
	metrics.proposalWindow.observe(duration)
}

func (metrics *nodeMetrics) observeVoteLatency(duration time.Duration) {
	storeDurationNanos(&metrics.voteLatencyNanos, duration)
	metrics.voteWindow.observe(duration)
}

func (metrics *nodeMetrics) observeCommitLatency(duration time.Duration) {
	storeDurationNanos(&metrics.commitLatencyNanos, duration)
	metrics.commitWindow.observe(duration)
	now := time.Now().UnixNano()
	if metrics.firstCommitUnixNano.Load() == 0 {
		metrics.firstCommitUnixNano.CompareAndSwap(0, now)
	}
	metrics.latestCommitUnixNano.Store(now)
	metrics.committedBlocks.Add(1)
}

func (metrics *nodeMetrics) observeSigningFailure() {
	metrics.signingFailures.Add(1)
}

func (metrics *nodeMetrics) snapshot() nodeMetricsSnapshot {
	proposalP95, proposalP99 := metrics.proposalWindow.percentiles()
	voteP95, voteP99 := metrics.voteWindow.percentiles()
	commitP95, commitP99 := metrics.commitWindow.percentiles()
	return nodeMetricsSnapshot{
		roundTimeouts:        metrics.roundTimeouts.Load(),
		adaptiveTimeoutNanos: metrics.adaptiveTimeoutNanos.Load(),
		recoveryDeferrals:    metrics.recoveryDeferrals.Load(),
		proposalLatencyNanos: metrics.proposalLatencyNanos.Load(),
		voteLatencyNanos:     metrics.voteLatencyNanos.Load(),
		commitLatencyNanos:   metrics.commitLatencyNanos.Load(),
		proposalP95Nanos:     proposalP95,
		proposalP99Nanos:     proposalP99,
		voteP95Nanos:         voteP95,
		voteP99Nanos:         voteP99,
		commitP95Nanos:       commitP95,
		commitP99Nanos:       commitP99,
		signingFailures:      metrics.signingFailures.Load(),
		committedBlocks:      metrics.committedBlocks.Load(),
		firstCommitUnixNano:  metrics.firstCommitUnixNano.Load(),
		latestCommitUnixNano: metrics.latestCommitUnixNano.Load(),
	}
}

func storeDurationNanos(target *atomic.Uint64, duration time.Duration) {
	if duration <= 0 {
		return
	}
	target.Store(uint64(duration.Nanoseconds()))
}

type durationWindow struct {
	mu     sync.Mutex
	values [metricsDurationWindowSize]uint64
	next   int
	count  int
}

func (window *durationWindow) observe(duration time.Duration) {
	if duration <= 0 {
		return
	}
	window.mu.Lock()
	defer window.mu.Unlock()
	window.values[window.next] = uint64(duration.Nanoseconds())
	window.next = (window.next + 1) % len(window.values)
	if window.count < len(window.values) {
		window.count++
	}
}

func (window *durationWindow) percentiles() (uint64, uint64) {
	window.mu.Lock()
	defer window.mu.Unlock()
	if window.count == 0 {
		return 0, 0
	}
	values := make([]uint64, window.count)
	copy(values, window.values[:window.count])
	sort.Slice(values, func(first int, second int) bool { return values[first] < values[second] })
	return percentile(values, 95), percentile(values, 99)
}

func percentile(values []uint64, pct int) uint64 {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*pct + 99) / 100
	if index == 0 {
		return values[0]
	}
	if index > len(values) {
		index = len(values)
	}
	return values[index-1]
}

package node

import (
	"sync/atomic"
	"time"
)

type nodeMetrics struct {
	roundTimeouts        atomic.Uint64
	proposalLatencyNanos atomic.Uint64
	voteLatencyNanos     atomic.Uint64
	commitLatencyNanos   atomic.Uint64
	signingFailures      atomic.Uint64
	committedBlocks      atomic.Uint64
	firstCommitUnixNano  atomic.Int64
	latestCommitUnixNano atomic.Int64
}

type nodeMetricsSnapshot struct {
	roundTimeouts        uint64
	proposalLatencyNanos uint64
	voteLatencyNanos     uint64
	commitLatencyNanos   uint64
	signingFailures      uint64
	committedBlocks      uint64
	firstCommitUnixNano  int64
	latestCommitUnixNano int64
}

func (metrics *nodeMetrics) observeRoundTimeout() {
	metrics.roundTimeouts.Add(1)
}

func (metrics *nodeMetrics) observeProposalLatency(duration time.Duration) {
	storeDurationNanos(&metrics.proposalLatencyNanos, duration)
}

func (metrics *nodeMetrics) observeVoteLatency(duration time.Duration) {
	storeDurationNanos(&metrics.voteLatencyNanos, duration)
}

func (metrics *nodeMetrics) observeCommitLatency(duration time.Duration) {
	storeDurationNanos(&metrics.commitLatencyNanos, duration)
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
	return nodeMetricsSnapshot{
		roundTimeouts:        metrics.roundTimeouts.Load(),
		proposalLatencyNanos: metrics.proposalLatencyNanos.Load(),
		voteLatencyNanos:     metrics.voteLatencyNanos.Load(),
		commitLatencyNanos:   metrics.commitLatencyNanos.Load(),
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

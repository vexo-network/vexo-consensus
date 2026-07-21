package node

import "time"

const (
	adaptiveRoundTimeoutLatencyMultiplier = 3
	adaptiveRoundTimeoutGrowthNumerator   = 3
	adaptiveRoundTimeoutGrowthDenominator = 2
	adaptiveRoundTimeoutDecayNumerator    = 4
	adaptiveRoundTimeoutDecayDenominator  = 5
	adaptiveRoundTimeoutMaxFactor         = 8
)

func recommendAdaptiveRoundTimeout(base time.Duration, current time.Duration, snapshot nodeMetricsSnapshot, progressed bool, timedOut bool, activePeers int, configuredPeers int) time.Duration {
	if base <= 0 {
		base = defaultConsensusTimeoutPropose + defaultConsensusTimeoutPrevote + defaultConsensusTimeoutPrecommit
	}
	if base <= 0 {
		base = time.Second
	}
	if current <= 0 {
		current = base
	}

	candidate := base
	observed := time.Duration(snapshot.proposalP95Nanos + snapshot.voteP95Nanos + snapshot.commitP95Nanos)
	if observed > 0 {
		observedBudget := observed * adaptiveRoundTimeoutLatencyMultiplier
		if observedBudget > candidate {
			candidate = observedBudget
		}
	}
	if peerBudget := peerAwareAdaptiveRoundTimeoutFloor(base, activePeers, configuredPeers); peerBudget > candidate {
		candidate = peerBudget
	}

	next := current
	switch {
	case timedOut:
		grown := growDuration(current, adaptiveRoundTimeoutGrowthNumerator, adaptiveRoundTimeoutGrowthDenominator)
		if grown < candidate {
			grown = candidate
		}
		next = grown
	case progressed:
		shrunk := shrinkDuration(current, adaptiveRoundTimeoutDecayNumerator, adaptiveRoundTimeoutDecayDenominator)
		if shrunk < candidate {
			shrunk = candidate
		}
		next = shrunk
	default:
		if next < candidate {
			next = candidate
		}
	}

	maxTimeout := base * adaptiveRoundTimeoutMaxFactor
	if maxTimeout < base {
		maxTimeout = base
	}
	if next < base {
		next = base
	}
	if next > maxTimeout {
		next = maxTimeout
	}
	return next
}

func peerAwareAdaptiveRoundTimeoutFloor(base time.Duration, activePeers int, configuredPeers int) time.Duration {
	if base <= 0 {
		return 0
	}
	if configuredPeers <= 0 {
		return base
	}
	if activePeers <= 0 {
		return growDuration(base, 2, 1)
	}
	if activePeers >= configuredPeers {
		return base
	}
	deficit := configuredPeers - activePeers
	floor := base * time.Duration(configuredPeers+deficit) / time.Duration(configuredPeers)
	if floor < base {
		return base
	}
	return floor
}

func growDuration(value time.Duration, numerator int, denominator int) time.Duration {
	if value <= 0 {
		return value
	}
	if denominator <= 0 {
		return value
	}
	return value * time.Duration(numerator) / time.Duration(denominator)
}

func shrinkDuration(value time.Duration, numerator int, denominator int) time.Duration {
	if value <= 0 {
		return value
	}
	if denominator <= 0 {
		return value
	}
	return value * time.Duration(numerator) / time.Duration(denominator)
}

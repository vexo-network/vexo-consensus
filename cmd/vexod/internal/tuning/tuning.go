package tuning

import (
	"math"
	"strconv"
	"strings"
	"time"
)

type Inputs struct {
	Validators      int
	TargetTPS       int
	Regions         int
	AverageLatency  time.Duration
	BlockBytes      int64
	FaultPercentage int
}

type Recommendation struct {
	SchemaVersion string            `json:"schema_version"`
	Inputs        InputsDocument    `json:"inputs"`
	Consensus     ConsensusTuning   `json:"consensus"`
	Networking    NetworkingTuning  `json:"networking"`
	Mempool       MempoolTuning     `json:"mempool"`
	Economics     EconomicsTuning   `json:"economics"`
	Alerts        AlertTuning       `json:"alerts"`
	Validation    []ValidationCheck `json:"validation"`
	Warnings      []string          `json:"warnings,omitempty"`
	Notes         []string          `json:"notes"`
}

type InputsDocument struct {
	Validators          int    `json:"validators"`
	TargetTPS           int    `json:"target_tps"`
	Regions             int    `json:"regions"`
	AverageLatency      string `json:"average_latency"`
	BlockBytes          int64  `json:"block_bytes"`
	FaultPercentage     int    `json:"fault_percentage"`
	FaultTolerance      int    `json:"fault_tolerance"`
	QuorumVotingPower   int    `json:"quorum_voting_power"`
	EstimatedByzantines int    `json:"estimated_byzantines"`
}

type ConsensusTuning struct {
	TargetBlockTime      string `json:"target_block_time"`
	ProposalTimeout      string `json:"proposal_timeout"`
	VoteTimeout          string `json:"vote_timeout"`
	CommitTimeout        string `json:"commit_timeout"`
	TimeoutBackoffFactor string `json:"timeout_backoff_factor"`
	CommitteeSize        int    `json:"committee_size"`
	EpochLength          int    `json:"epoch_length"`
}

type NetworkingTuning struct {
	OutboundPeers       int    `json:"outbound_peers"`
	InboundPeerBudget   int    `json:"inbound_peer_budget"`
	MaxMessageBytes     int64  `json:"max_message_bytes"`
	HandshakeTimeout    string `json:"handshake_timeout"`
	ReconnectMinBackoff string `json:"reconnect_min_backoff"`
	ReconnectMaxBackoff string `json:"reconnect_max_backoff"`
	RateLimitPerPeer    int    `json:"rate_limit_per_peer"`
	BanThreshold        int    `json:"ban_threshold"`
}

type MempoolTuning struct {
	MaxTxs        int    `json:"max_txs"`
	MaxTxBytes    int64  `json:"max_tx_bytes"`
	MinFee        uint64 `json:"min_fee"`
	BaseFee       uint64 `json:"base_fee"`
	MinGas        uint64 `json:"min_gas"`
	SeenTTL       string `json:"seen_ttl"`
	EnableWAL     bool   `json:"enable_wal"`
	EnableRecheck bool   `json:"enable_recheck"`
}

type EconomicsTuning struct {
	MinValidatorStake uint64 `json:"min_validator_stake"`
	SlashFractionBps  uint64 `json:"slash_fraction_bps"`
	JailDuration      string `json:"jail_duration"`
	UnbondingPeriod   string `json:"unbonding_period"`
}

type AlertTuning struct {
	MinHeightRatePerMinute int    `json:"min_height_rate_per_minute"`
	MaxRoundTimeouts       int    `json:"max_round_timeouts"`
	MaxCommitLatency       string `json:"max_commit_latency"`
	MaxMempoolSize         int    `json:"max_mempool_size"`
	MaxPeerBansPerHour     int    `json:"max_peer_bans_per_hour"`
	MaxSignerFailures      int    `json:"max_signer_failures"`
}

type ValidationCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func Recommend(inputs Inputs) Recommendation {
	normalized := normalize(inputs)
	faultTolerance := (normalized.Validators - 1) / 3
	quorum := (normalized.Validators*2)/3 + 1
	estimatedByzantines := int(math.Ceil(float64(normalized.Validators*normalized.FaultPercentage) / 100))
	latency := normalized.AverageLatency
	proposalTimeout := clampDuration(latency*6, duration("800ms"), duration("8s"))
	voteTimeout := clampDuration(latency*4, duration("600ms"), duration("6s"))
	commitTimeout := clampDuration(latency*2, duration("300ms"), duration("3s"))
	blockTime := clampDuration(proposalTimeout+voteTimeout+commitTimeout, duration("1200ms"), duration("12s"))
	committeeSize := recommendCommitteeSize(normalized.Validators)
	outboundPeers := clampInt(int(math.Ceil(math.Sqrt(float64(normalized.Validators))*4)), 8, 96)
	inboundBudget := clampInt(outboundPeers*2, 16, 192)
	maxTxs := clampInt(normalized.TargetTPS*int(blockTime/duration("1s")+1)*2, 5_000, 250_000)
	rateLimit := clampInt(normalized.TargetTPS/maxInt(1, normalized.Validators/4), 100, 10_000)

	recommendation := Recommendation{
		SchemaVersion: "v1",
		Inputs: InputsDocument{
			Validators:          normalized.Validators,
			TargetTPS:           normalized.TargetTPS,
			Regions:             normalized.Regions,
			AverageLatency:      latency.String(),
			BlockBytes:          normalized.BlockBytes,
			FaultPercentage:     normalized.FaultPercentage,
			FaultTolerance:      faultTolerance,
			QuorumVotingPower:   quorum,
			EstimatedByzantines: estimatedByzantines,
		},
		Consensus: ConsensusTuning{
			TargetBlockTime:      blockTime.String(),
			ProposalTimeout:      proposalTimeout.String(),
			VoteTimeout:          voteTimeout.String(),
			CommitTimeout:        commitTimeout.String(),
			TimeoutBackoffFactor: "1.5",
			CommitteeSize:        committeeSize,
			EpochLength:          clampInt(normalized.Validators*16, 256, 10_000),
		},
		Networking: NetworkingTuning{
			OutboundPeers:       outboundPeers,
			InboundPeerBudget:   inboundBudget,
			MaxMessageBytes:     maxInt64(normalized.BlockBytes*2, 1_048_576),
			HandshakeTimeout:    clampDuration(latency*8, duration("1s"), duration("10s")).String(),
			ReconnectMinBackoff: clampDuration(latency*2, duration("250ms"), duration("2s")).String(),
			ReconnectMaxBackoff: clampDuration(latency*40, duration("5s"), duration("60s")).String(),
			RateLimitPerPeer:    rateLimit,
			BanThreshold:        clampInt(rateLimit*4, 500, 50_000),
		},
		Mempool: MempoolTuning{
			MaxTxs:        maxTxs,
			MaxTxBytes:    minInt64(normalized.BlockBytes/8, 256_000),
			MinFee:        uint64(clampInt(normalized.TargetTPS/10_000+1, 1, 100)),
			BaseFee:       uint64(clampInt(normalized.TargetTPS/20_000+1, 1, 100)),
			MinGas:        1,
			SeenTTL:       (blockTime * 120).String(),
			EnableWAL:     true,
			EnableRecheck: true,
		},
		Economics: EconomicsTuning{
			MinValidatorStake: uint64(clampInt(normalized.Validators*100, 1_000, 1_000_000)),
			SlashFractionBps:  uint64(clampInt(normalized.FaultPercentage*10, 50, 1_000)),
			JailDuration:      duration("336h").String(),
			UnbondingPeriod:   duration("504h").String(),
		},
		Alerts: AlertTuning{
			MinHeightRatePerMinute: clampInt(60/int(blockTime/duration("1s")+1), 1, 60),
			MaxRoundTimeouts:       clampInt(normalized.Validators/8, 2, 50),
			MaxCommitLatency:       (blockTime * 2).String(),
			MaxMempoolSize:         maxTxs * 8 / 10,
			MaxPeerBansPerHour:     clampInt(normalized.Validators/10, 1, 20),
			MaxSignerFailures:      0,
		},
		Notes: []string{
			"Treat this as a launch-safe starting point, not a substitute for multi-host measurements.",
			"Increase timeouts if observed p95 proposal/vote latency exceeds the recommended timeout budget.",
			"Lower mempool limits or raise fees if CheckTx CPU, memory, or queue latency becomes unstable.",
		},
	}
	recommendation.Validation = validate(normalized, estimatedByzantines, faultTolerance)
	recommendation.Warnings = warnings(normalized, estimatedByzantines, faultTolerance)
	return recommendation
}

func normalize(inputs Inputs) Inputs {
	if inputs.Validators <= 0 {
		inputs.Validators = 4
	}
	if inputs.TargetTPS <= 0 {
		inputs.TargetTPS = 100
	}
	if inputs.Regions <= 0 {
		inputs.Regions = 1
	}
	if inputs.AverageLatency <= 0 {
		inputs.AverageLatency = 150 * time.Millisecond
	}
	if inputs.BlockBytes <= 0 {
		inputs.BlockBytes = 4 * 1024 * 1024
	}
	if inputs.FaultPercentage <= 0 {
		inputs.FaultPercentage = 20
	}
	return inputs
}

func validate(inputs Inputs, estimatedByzantines int, faultTolerance int) []ValidationCheck {
	return []ValidationCheck{
		{Name: "validator_count", OK: inputs.Validators >= 4, Message: "at least four validators are recommended for BFT fault tolerance"},
		{Name: "fault_budget", OK: estimatedByzantines <= faultTolerance, Message: "estimated Byzantine validators must fit within f < n/3"},
		{Name: "region_spread", OK: inputs.Regions >= 1 && inputs.Regions <= inputs.Validators, Message: "regions must be positive and not exceed validator count"},
		{Name: "block_size", OK: inputs.BlockBytes >= 1_048_576 && inputs.BlockBytes <= 64*1024*1024, Message: "block size should stay within safe gossip and memory bounds"},
		{Name: "latency_budget", OK: inputs.AverageLatency <= duration("2s"), Message: "average latency above 2s requires conservative timeout tuning and launch review"},
	}
}

func warnings(inputs Inputs, estimatedByzantines int, faultTolerance int) []string {
	var result []string
	if estimatedByzantines > faultTolerance {
		result = append(result, "fault percentage exceeds the BFT safety budget")
	}
	if inputs.TargetTPS > 10_000 {
		result = append(result, "target TPS is high; require real load evidence before launch")
	}
	if inputs.Validators > 256 {
		result = append(result, "large validator sets should use committee sampling and aggressive peer budgeting")
	}
	if inputs.Regions > 1 && inputs.AverageLatency < duration("80ms") {
		result = append(result, "multi-region average latency looks optimistic; verify with p95 measurements")
	}
	return result
}

func recommendCommitteeSize(validators int) int {
	if validators <= 64 {
		return validators
	}
	return clampInt(int(math.Ceil(math.Sqrt(float64(validators))*16)), 64, minInt(validators, 512))
}

func clampDuration(value time.Duration, low time.Duration, high time.Duration) time.Duration {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func clampInt(value int, low int, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func duration(value string) time.Duration {
	switch {
	case strings.HasSuffix(value, "ms"):
		amount, err := strconv.ParseInt(strings.TrimSuffix(value, "ms"), 10, 64)
		if err != nil {
			panic(err)
		}
		return time.Duration(amount) * time.Millisecond
	case strings.HasSuffix(value, "s"):
		amount, err := strconv.ParseInt(strings.TrimSuffix(value, "s"), 10, 64)
		if err != nil {
			panic(err)
		}
		return time.Duration(amount*1000) * time.Millisecond
	case strings.HasSuffix(value, "h"):
		amount, err := strconv.ParseInt(strings.TrimSuffix(value, "h"), 10, 64)
		if err != nil {
			panic(err)
		}
		return time.Duration(amount*60*60*1000) * time.Millisecond
	default:
		panic("unsupported duration " + value)
	}
}

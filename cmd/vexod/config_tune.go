package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/vexo-network/vexo-consensus/cmd/vexod/internal/tuning"
)

func runConfigTune(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config tune", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	validators := flags.Int("validators", 4, "validator count")
	targetTPS := flags.Int("tps", 100, "target transactions per second")
	regions := flags.Int("regions", 1, "deployment region count")
	averageLatency := flags.Duration("latency", 150*time.Millisecond, "average inter-validator latency")
	blockBytes := flags.Int64("block-bytes", 4*1024*1024, "target maximum block bytes")
	faultPercentage := flags.Int("fault-percent", 20, "expected Byzantine/offline validator percentage")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}

	document := tuning.Recommend(tuning.Inputs{
		Validators:      *validators,
		TargetTPS:       *targetTPS,
		Regions:         *regions,
		AverageLatency:  *averageLatency,
		BlockBytes:      *blockBytes,
		FaultPercentage: *faultPercentage,
	})
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	writeTuningRecommendation(writer, document)
	return nil
}

func writeTuningRecommendation(writer io.Writer, document tuning.Recommendation) {
	fmt.Fprintf(writer, "mainnet tuning recommendation\n")
	fmt.Fprintf(writer, "inputs: validators=%d tps=%d regions=%d latency=%s block_bytes=%d fault_percent=%d\n", document.Inputs.Validators, document.Inputs.TargetTPS, document.Inputs.Regions, document.Inputs.AverageLatency, document.Inputs.BlockBytes, document.Inputs.FaultPercentage)
	fmt.Fprintf(writer, "consensus: block_time=%s proposal_timeout=%s vote_timeout=%s commit_timeout=%s committee_size=%d epoch_length=%d\n", document.Consensus.TargetBlockTime, document.Consensus.ProposalTimeout, document.Consensus.VoteTimeout, document.Consensus.CommitTimeout, document.Consensus.CommitteeSize, document.Consensus.EpochLength)
	fmt.Fprintf(writer, "networking: outbound_peers=%d inbound_budget=%d max_message_bytes=%d rate_limit_per_peer=%d ban_threshold=%d\n", document.Networking.OutboundPeers, document.Networking.InboundPeerBudget, document.Networking.MaxMessageBytes, document.Networking.RateLimitPerPeer, document.Networking.BanThreshold)
	fmt.Fprintf(writer, "mempool: max_txs=%d max_tx_bytes=%d min_fee=%d seen_ttl=%s wal=%t recheck=%t\n", document.Mempool.MaxTxs, document.Mempool.MaxTxBytes, document.Mempool.MinFee, document.Mempool.SeenTTL, document.Mempool.EnableWAL, document.Mempool.EnableRecheck)
	fmt.Fprintf(writer, "economics: min_validator_stake=%d slash_fraction_bps=%d jail=%s unbonding=%s\n", document.Economics.MinValidatorStake, document.Economics.SlashFractionBps, document.Economics.JailDuration, document.Economics.UnbondingPeriod)
	fmt.Fprintf(writer, "alerts: min_height_rate_per_minute=%d max_round_timeouts=%d max_commit_latency=%s max_mempool_size=%d max_peer_bans_per_hour=%d signer_failures=%d\n", document.Alerts.MinHeightRatePerMinute, document.Alerts.MaxRoundTimeouts, document.Alerts.MaxCommitLatency, document.Alerts.MaxMempoolSize, document.Alerts.MaxPeerBansPerHour, document.Alerts.MaxSignerFailures)
	fmt.Fprintf(writer, "validation:\n")
	for _, check := range document.Validation {
		fmt.Fprintf(writer, "- %s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	if len(document.Warnings) > 0 {
		fmt.Fprintf(writer, "warnings:\n")
		for _, warning := range document.Warnings {
			fmt.Fprintf(writer, "- %s\n", warning)
		}
	}
}

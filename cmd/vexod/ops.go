package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/vexo-network/vexo-consensus/ops"
)

func runOps(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("ops subcommand is required")
	}
	switch args[0] {
	case "thresholds":
		return runOpsThresholds(writer, args[1:])
	case "alerts":
		return runOpsAlerts(writer, args[1:])
	default:
		return fmt.Errorf("unknown ops subcommand %q", args[0])
	}
}

func runOpsThresholds(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("ops thresholds", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	thresholds := ops.DefaultThresholds()
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(thresholds)
	}
	fmt.Fprintf(writer, "ops alert thresholds\n")
	fmt.Fprintf(writer, "height_rate_per_minute >= %.2f\n", thresholds.MinHeightRatePerMinute)
	fmt.Fprintf(writer, "round_timeouts_per_minute <= %.2f\n", thresholds.MaxRoundTimeoutsPerMinute)
	fmt.Fprintf(writer, "proposal_latency <= %s\n", thresholds.MaxProposalLatency)
	fmt.Fprintf(writer, "vote_latency <= %s\n", thresholds.MaxVoteLatency)
	fmt.Fprintf(writer, "peer_bans <= %d\n", thresholds.MaxPeerBans)
	fmt.Fprintf(writer, "mempool_size <= %d\n", thresholds.MaxMempoolSize)
	fmt.Fprintf(writer, "commit_latency <= %s\n", thresholds.MaxCommitLatency)
	fmt.Fprintf(writer, "snapshot_required: %t\n", thresholds.SnapshotRequired)
	fmt.Fprintf(writer, "replay_healthy_required: %t\n", thresholds.ReplayHealthyRequired)
	fmt.Fprintf(writer, "validator_signing_failures <= %d\n", thresholds.MaxValidatorSigningFailures)
	return nil
}

func runOpsAlerts(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("ops alerts", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	heightRate := flags.Float64("height-rate", 0, "observed height increase rate per minute")
	roundTimeouts := flags.Float64("round-timeouts", 0, "observed round timeouts per minute")
	proposalLatency := flags.Duration("proposal-latency", 0, "observed proposal processing latency")
	voteLatency := flags.Duration("vote-latency", 0, "observed vote processing latency")
	peerBans := flags.Uint64("peer-bans", 0, "observed peer ban count")
	mempoolSize := flags.Uint64("mempool-size", 0, "observed mempool size")
	commitLatency := flags.Duration("commit-latency", 0, "observed commit latency")
	snapshotHealthy := flags.Bool("snapshot-healthy", false, "whether snapshot export/verify is healthy")
	replayHealthy := flags.Bool("replay-healthy", false, "whether replay/recovery is healthy")
	signingFailures := flags.Uint64("signing-failures", 0, "validator signing failures")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	report, err := ops.Evaluate(ops.Sample{
		HeightRatePerMinute:      *heightRate,
		RoundTimeoutsPerMinute:   *roundTimeouts,
		ProposalLatency:          durationOrZero(*proposalLatency),
		VoteLatency:              durationOrZero(*voteLatency),
		PeerBans:                 *peerBans,
		MempoolSize:              *mempoolSize,
		CommitLatency:            durationOrZero(*commitLatency),
		SnapshotHealthy:          *snapshotHealthy,
		ReplayHealthy:            *replayHealthy,
		ValidatorSigningFailures: *signingFailures,
	}, ops.DefaultThresholds())
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	status := "ok"
	if !report.OK {
		status = "alert"
	}
	fmt.Fprintf(writer, "ops alerts %s\n", status)
	for _, alert := range report.Alerts {
		fmt.Fprintf(writer, "%s [%s] value=%s threshold=%s %s\n", alert.Name, alert.Severity, alert.Value, alert.Threshold, alert.Message)
	}
	return nil
}

func durationOrZero(duration time.Duration) time.Duration {
	if duration < 0 {
		return 0
	}
	return duration
}

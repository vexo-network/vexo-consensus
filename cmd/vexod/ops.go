package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vexo-network/vexo-consensus/ops"
	"github.com/vexo-network/vexo-consensus/p2p"
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
	case "incident":
		return runOpsIncident(writer, args[1:])
	case "conformance":
		return runOpsConformance(writer, args[1:])
	default:
		return fmt.Errorf("unknown ops subcommand %q", args[0])
	}
}

type opsIncidentDocument struct {
	SchemaVersion string     `json:"schema_version"`
	Title         string     `json:"title"`
	Severity      string     `json:"severity"`
	Report        ops.Report `json:"report"`
	Summary       []string   `json:"summary"`
	Actions       []string   `json:"actions"`
}

type opsConformanceDocument struct {
	SchemaVersion string                   `json:"schema_version"`
	OK            bool                     `json:"ok"`
	StartPlan     startPlanDocument        `json:"start_plan"`
	Audit         auditDocument            `json:"audit"`
	RotationPlan  *keyRotationPlanDocument `json:"rotation_plan,omitempty"`
	Metrics       *ops.Report              `json:"metrics,omitempty"`
	Checks        []auditCheckDocument     `json:"checks"`
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

func runOpsConformance(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("ops conformance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	metricsFile := flags.String("metrics-file", "", "current /metrics JSON file to evaluate")
	previousMetricsFile := flags.String("previous-metrics-file", "", "previous /metrics JSON file for rate deltas")
	windowValue := flags.String("window", "1m", "elapsed time between previous and current metrics files")
	strict := flags.Bool("strict", false, "use strict network-safety audit severities")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	rotationKeys := stringListFlags{}
	flags.Var(&rotationKeys, "rotation-key", "additional validator key file path with active-from/active-until metadata; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	inputs, err := loadStartInputs(*home, *configPath, *genesisPath, *keyPath, []string(rotationKeys), true)
	if err != nil {
		return err
	}
	runtimeConfig, err := loadStartRuntimeConfig(*home, *configPath)
	if err != nil {
		return err
	}
	runtimeConfig = applyNetworkRuntimeDefaults(inputs, runtimeConfig)
	document := opsConformanceDocument{
		SchemaVersion: "v1",
		OK:            true,
		StartPlan:     inputs.Plan,
		Audit:         auditDeployment(inputs, runtimeConfig, *strict),
	}
	document.addCheck("start_inputs", "error", true, "config, genesis, and validator signer inputs are loadable")
	for _, check := range validateConformancePeerAddresses("peer", runtimeConfig.P2PPeers) {
		document.addCheck(check.Name, check.Severity, check.OK, check.Message)
	}
	for _, check := range validateConformancePeerAddresses("seed", runtimeConfig.P2PSeeds) {
		document.addCheck(check.Name, check.Severity, check.OK, check.Message)
	}
	if len(rotationKeys) > 0 {
		rotationPaths := append([]string{inputs.Plan.KeyPath}, []string(rotationKeys)...)
		rotationPlan, err := buildKeyRotationPlan(*home, rotationPaths, resolvePassphrase(""))
		if err != nil {
			return err
		}
		document.RotationPlan = &rotationPlan
		document.addCheck("key_rotation_windows", "error", rotationPlan.OK, "validator key active windows must be contiguous and non-overlapping")
	}
	if *metricsFile != "" {
		sample, err := readOpsMetricsSample(*metricsFile, *previousMetricsFile, *windowValue)
		if err != nil {
			return err
		}
		report, err := ops.Evaluate(sample, ops.DefaultThresholds())
		if err != nil {
			return err
		}
		document.Metrics = &report
		document.addCheck("metrics_thresholds", "warning", report.OK, "operator metrics should stay within alert thresholds")
	}
	document.OK = document.Audit.OK && conformanceChecksOK(document.Checks)
	if document.RotationPlan != nil {
		document.OK = document.OK && document.RotationPlan.OK
	}
	if document.Metrics != nil {
		document.OK = document.OK && document.Metrics.OK
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	writeOpsConformance(writer, document)
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
	metricsFile := flags.String("metrics-file", "", "current /metrics JSON file to evaluate")
	previousMetricsFile := flags.String("previous-metrics-file", "", "previous /metrics JSON file for rate deltas")
	windowValue := flags.String("window", "1m", "elapsed time between previous and current metrics files")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	sample := ops.Sample{
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
	}
	if *metricsFile != "" {
		derived, err := readOpsMetricsSample(*metricsFile, *previousMetricsFile, *windowValue)
		if err != nil {
			return err
		}
		sample = derived
	}
	report, err := ops.Evaluate(sample, ops.DefaultThresholds())
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

func (document *opsConformanceDocument) addCheck(name string, severity string, ok bool, message string) {
	document.Checks = append(document.Checks, auditCheckDocument{Name: name, Severity: severity, OK: ok, Message: message})
}

func validateConformancePeerAddresses(prefix string, peers map[p2p.PeerID]string) []auditCheckDocument {
	checks := make([]auditCheckDocument, 0, len(peers))
	for peerID, address := range peers {
		err := p2p.ValidatePeerAddress(address)
		checks = append(checks, auditCheckDocument{
			Name:     prefix + "_address_" + string(peerID),
			Severity: "error",
			OK:       err == nil,
			Message:  fmt.Sprintf("%s %s advertised address must be dialable host:port", prefix, peerID),
		})
	}
	return checks
}

func conformanceChecksOK(checks []auditCheckDocument) bool {
	for _, check := range checks {
		if !check.OK && check.Severity == "error" {
			return false
		}
	}
	return true
}

func writeOpsConformance(writer io.Writer, document opsConformanceDocument) {
	status := "ok"
	if !document.OK {
		status = "failed"
	}
	fmt.Fprintf(writer, "ops conformance %s\n", status)
	fmt.Fprintf(writer, "chain_id: %s\n", document.StartPlan.ChainID)
	fmt.Fprintf(writer, "validator_id: %s\n", document.StartPlan.ValidatorID)
	for _, check := range document.Checks {
		fmt.Fprintf(writer, "%s [%s] ok=%t %s\n", check.Name, check.Severity, check.OK, check.Message)
	}
	if document.RotationPlan != nil {
		fmt.Fprintf(writer, "key_rotation_ok: %t\n", document.RotationPlan.OK)
	}
	if document.Metrics != nil {
		fmt.Fprintf(writer, "metrics_ok: %t\n", document.Metrics.OK)
	}
}

func runOpsIncident(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("ops incident", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	title := flags.String("title", "vexo operational incident", "incident title")
	metricsFile := flags.String("metrics-file", "", "current /metrics JSON file to evaluate")
	previousMetricsFile := flags.String("previous-metrics-file", "", "previous /metrics JSON file for rate deltas")
	windowValue := flags.String("window", "1m", "elapsed time between previous and current metrics files")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *metricsFile == "" {
		return errors.New("metrics-file is required")
	}
	sample, err := readOpsMetricsSample(*metricsFile, *previousMetricsFile, *windowValue)
	if err != nil {
		return err
	}
	report, err := ops.Evaluate(sample, ops.DefaultThresholds())
	if err != nil {
		return err
	}
	document := buildOpsIncidentDocument(*title, report)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	writeOpsIncident(writer, document)
	return nil
}

func buildOpsIncidentDocument(title string, report ops.Report) opsIncidentDocument {
	severity := "none"
	if !report.OK {
		severity = "warning"
		for _, alert := range report.Alerts {
			if alert.Severity == ops.SeverityCritical {
				severity = "critical"
				break
			}
		}
	}
	document := opsIncidentDocument{
		SchemaVersion: "v1",
		Title:         title,
		Severity:      severity,
		Report:        report,
		Actions: []string{
			"freeze deploys and preserve current logs, metrics, pprof, and config files",
			"check quorum, finality, signer failures, peer bans, mempool, snapshot, and replay health",
			"if finality or replay is unsafe, halt validators and use the last safe height recovery procedure",
			"attach incident report to release/launch audit evidence",
		},
	}
	if report.OK {
		document.Summary = append(document.Summary, "no alert thresholds exceeded")
		return document
	}
	for _, alert := range report.Alerts {
		document.Summary = append(document.Summary, fmt.Sprintf("%s severity=%s value=%s threshold=%s", alert.Name, alert.Severity, alert.Value, alert.Threshold))
	}
	return document
}

func writeOpsIncident(writer io.Writer, document opsIncidentDocument) {
	fmt.Fprintf(writer, "ops incident report\n")
	fmt.Fprintf(writer, "title: %s\n", document.Title)
	fmt.Fprintf(writer, "severity: %s\n", document.Severity)
	fmt.Fprintf(writer, "alerts: %d\n", len(document.Report.Alerts))
	for _, summary := range document.Summary {
		fmt.Fprintf(writer, "- %s\n", summary)
	}
	fmt.Fprintf(writer, "actions:\n")
	for index, action := range document.Actions {
		fmt.Fprintf(writer, "%d. %s\n", index+1, action)
	}
}

func durationOrZero(duration time.Duration) time.Duration {
	if duration < 0 {
		return 0
	}
	return duration
}

func readOpsMetricsSample(metricsFile string, previousMetricsFile string, windowValue string) (ops.Sample, error) {
	current, err := readOpsMetricsSnapshot(metricsFile)
	if err != nil {
		return ops.Sample{}, err
	}
	var previous *ops.MetricsSnapshot
	if previousMetricsFile != "" {
		previousValue, err := readOpsMetricsSnapshot(previousMetricsFile)
		if err != nil {
			return ops.Sample{}, err
		}
		previous = &previousValue
	}
	window, err := time.ParseDuration(windowValue)
	if err != nil {
		return ops.Sample{}, err
	}
	return ops.SampleFromMetricsSnapshot(previous, current, window)
}

func readOpsMetricsSnapshot(path string) (ops.MetricsSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ops.MetricsSnapshot{}, err
	}
	var snapshot ops.MetricsSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return ops.MetricsSnapshot{}, err
	}
	return snapshot, nil
}

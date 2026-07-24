package rpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func statusResponse(status node.Status) StatusResponse {
	startedAtUnix := int64(0)
	if !status.StartedAt.IsZero() {
		startedAtUnix = status.StartedAt.Unix()
	}
	response := StatusResponse{
		ChainID:             status.ChainID,
		EVMChainID:          status.EVMChainID,
		Running:             status.Running,
		StartedAtUnix:       startedAtUnix,
		LatestHeight:        uint64(status.LatestHeight),
		LatestAppHash:       hex.EncodeToString(status.LatestAppHash[:]),
		DataDir:             status.DataDir,
		PeerCount:           status.PeerCount,
		ActivePeerCount:     status.ActivePeerCount,
		ConfiguredPeerCount: status.ConfiguredPeerCount,
		ScoredPeerCount:     status.ScoredPeerCount,
		BannedPeers:         status.BannedPeers,
	}
	if status.ConfiguredPeerCount > 0 {
		response.QuorumHealthRatio = float64(status.ActivePeerCount) / float64(status.ConfiguredPeerCount)
	}
	if status.LatestFinalizedHeight > 0 {
		response.LatestFinalizedHeight = uint64(status.LatestFinalizedHeight)
		response.LatestFinalizedHash = hex.EncodeToString(status.LatestFinalizedHash[:])
	}
	return response
}

func metricsResponse(metrics node.Metrics) MetricsResponse {
	return MetricsResponse{
		ChainID:                     metrics.ChainID,
		Running:                     metrics.Running,
		StartedAtUnix:               metrics.StartedAtUnix,
		UptimeSeconds:               metrics.UptimeSeconds,
		DataDir:                     metrics.DataDir,
		AdaptiveRoundTimeoutEnabled: metrics.AdaptiveRoundTimeoutEnabled,
		RecoveryFinalityGateEnabled: metrics.RecoveryFinalityGateEnabled,
		LatestHeight:                uint64(metrics.LatestHeight),
		LatestAppHash:               hex.EncodeToString(metrics.LatestAppHash[:]),
		EarliestBlockHeight:         uint64(metrics.EarliestBlockHeight),
		LatestBlockHeight:           uint64(metrics.LatestBlockHeight),
		TotalBlocks:                 metrics.TotalBlocks,
		ValidatorCount:              metrics.ValidatorCount,
		TotalVotingPower:            metrics.TotalVotingPower,
		ValidatorSetHash:            hex.EncodeToString(metrics.ValidatorSetHash[:]),
		PeerCount:                   metrics.PeerCount,
		ActivePeerCount:             metrics.ActivePeerCount,
		ConfiguredPeerCount:         metrics.ConfiguredPeerCount,
		ScoredPeerCount:             metrics.ScoredPeerCount,
		BannedPeers:                 metrics.BannedPeers,
		QuorumHealthRatio:           metrics.QuorumHealthRatio,
		PeerWindowMessages:          metrics.PeerWindowMessages,
		ConsensusLoopRunning:        metrics.ConsensusLoopRunning,
		HeightRatePerMinute:         metrics.HeightRatePerMinute,
		RoundTimeouts:               metrics.RoundTimeouts,
		AdaptiveRoundTimeoutNanos:   metrics.AdaptiveRoundTimeoutNanos,
		RecoveryFinalityDeferrals:   metrics.RecoveryFinalityDeferrals,
		ProposalLatencyNanos:        metrics.ProposalLatencyNanos,
		ProposalLatencyP95Nanos:     metrics.ProposalLatencyP95Nanos,
		ProposalLatencyP99Nanos:     metrics.ProposalLatencyP99Nanos,
		VoteLatencyNanos:            metrics.VoteLatencyNanos,
		VoteLatencyP95Nanos:         metrics.VoteLatencyP95Nanos,
		VoteLatencyP99Nanos:         metrics.VoteLatencyP99Nanos,
		MempoolSize:                 metrics.MempoolSize,
		CommitLatencyNanos:          metrics.CommitLatencyNanos,
		CommitLatencyP95Nanos:       metrics.CommitLatencyP95Nanos,
		CommitLatencyP99Nanos:       metrics.CommitLatencyP99Nanos,
		SnapshotHealthy:             metrics.SnapshotHealthy,
		ReplayHealthy:               metrics.ReplayHealthy,
		SigningFailures:             metrics.SigningFailures,
		ReconciliationFailures:      metrics.ReconciliationFailures,
	}
}

func diagnosticsResponse(ctx context.Context, provider StatusProvider, cfg Config) DiagnosticsResponse {
	status := provider.Status(ctx)
	capabilities := providerCapabilities(provider, cfg)
	response := DiagnosticsResponse{
		OK:     true,
		Status: "healthy",
		Checks: []DiagnosticCheckResponse{
			{Name: "status", OK: true},
			{Name: "ready", OK: status.Running},
		},
		Node:  statusResponse(status),
		Peers: peerResponses(status.Peers),
	}
	if !status.Running {
		response.OK = false
		response.Status = "not_ready"
		response.Checks[1].Error = "node is not running"
	}

	if metricsProvider, ok := provider.(MetricsProvider); ok {
		metrics, err := metricsProvider.Metrics(ctx)
		if err != nil {
			response.addDiagnosticFailure("metrics", err)
		} else {
			metricsResponse := metricsResponse(metrics)
			response.Metrics = &metricsResponse
			response.Checks = append(response.Checks, DiagnosticCheckResponse{Name: "metrics", OK: true})
		}
	} else {
		response.addDiagnosticFailure("metrics", errors.New("metrics query is unavailable"))
	}

	if queryProvider, ok := provider.(ChainQueryProvider); ok {
		index, err := queryProvider.BlockIndex(ctx)
		if err != nil {
			response.addDiagnosticFailure("storage", err)
		} else {
			storageResponse := blockIndexResponse(index)
			response.Storage = &storageResponse
			response.Checks = append(response.Checks, DiagnosticCheckResponse{Name: "storage", OK: true})
		}
	} else {
		response.addDiagnosticFailure("storage", errors.New("block index query is unavailable"))
	}
	if capabilities.Complete {
		response.Checks = append(response.Checks, DiagnosticCheckResponse{Name: "capabilities", OK: true})
	} else {
		response.addDiagnosticFailure("capabilities", fmt.Errorf("missing required rpc capabilities: %s", strings.Join(capabilities.Missing, ",")))
	}
	return response
}

func (response *DiagnosticsResponse) addDiagnosticFailure(name string, err error) {
	response.OK = false
	if response.Status == "healthy" {
		response.Status = "degraded"
	}
	response.Checks = append(response.Checks, DiagnosticCheckResponse{
		Name:  name,
		OK:    false,
		Error: err.Error(),
	})
}

func metricsText(metrics node.Metrics) string {
	var builder strings.Builder
	writeGauge := func(name string, help string, value uint64) {
		fmt.Fprintf(&builder, "# HELP %s %s\n", name, help)
		fmt.Fprintf(&builder, "# TYPE %s gauge\n", name)
		fmt.Fprintf(&builder, "%s %d\n", name, value)
	}
	writeFloatGauge := func(name string, help string, value float64) {
		fmt.Fprintf(&builder, "# HELP %s %s\n", name, help)
		fmt.Fprintf(&builder, "# TYPE %s gauge\n", name)
		fmt.Fprintf(&builder, "%s %.6f\n", name, value)
	}
	writeGauge("vexo_node_running", "Whether the node is running.", boolGauge(metrics.Running))
	writeGauge("vexo_started_at_unix", "Node process start timestamp in unix seconds.", uint64(metrics.StartedAtUnix))
	writeGauge("vexo_uptime_seconds", "Node uptime in seconds.", metrics.UptimeSeconds)
	writeGauge("vexo_latest_height", "Latest committed application height.", uint64(metrics.LatestHeight))
	writeGauge("vexo_earliest_block_height", "Earliest locally stored block height.", uint64(metrics.EarliestBlockHeight))
	writeGauge("vexo_latest_block_height", "Latest locally stored block height.", uint64(metrics.LatestBlockHeight))
	writeGauge("vexo_total_blocks", "Total locally stored blocks.", metrics.TotalBlocks)
	writeGauge("vexo_validator_count", "Current validator count.", uint64(metrics.ValidatorCount))
	writeGauge("vexo_total_voting_power", "Current total validator voting power.", metrics.TotalVotingPower)
	writeGauge("vexo_peer_count", "Connected peer count when available, otherwise scored peer count for backwards compatibility.", uint64(metrics.PeerCount))
	writeGauge("vexo_active_peer_count", "Active transport peer session count.", uint64(metrics.ActivePeerCount))
	writeGauge("vexo_configured_peer_count", "Configured or learned transport peer count.", uint64(metrics.ConfiguredPeerCount))
	writeGauge("vexo_scored_peer_count", "Peer score table entry count.", uint64(metrics.ScoredPeerCount))
	writeGauge("vexo_banned_peers", "Banned peer count.", uint64(metrics.BannedPeers))
	writeFloatGauge("vexo_quorum_health_ratio", "Active peer count divided by configured peer count.", metrics.QuorumHealthRatio)
	writeGauge("vexo_peer_window_messages", "Peer messages observed in the current score window.", metrics.PeerWindowMessages)
	writeGauge("vexo_consensus_loop_running", "Whether the local consensus loop is running.", boolGauge(metrics.ConsensusLoopRunning))
	writeGauge("vexo_adaptive_round_timeout_enabled", "Whether the adaptive consensus round timeout policy is enabled.", boolGauge(metrics.AdaptiveRoundTimeoutEnabled))
	writeGauge("vexo_recovery_finality_gate_enabled", "Whether the recovery finality gate is enabled.", boolGauge(metrics.RecoveryFinalityGateEnabled))
	writeGauge("vexo_round_timeouts", "Observed consensus round timeouts.", metrics.RoundTimeouts)
	writeGauge("vexo_adaptive_round_timeout_nanos", "Current adaptive consensus round timeout in nanoseconds.", metrics.AdaptiveRoundTimeoutNanos)
	writeGauge("vexo_recovery_finality_deferrals", "Finalized commits deferred by recovery consistency checks.", metrics.RecoveryFinalityDeferrals)
	writeGauge("vexo_proposal_latency_nanos", "Observed proposal processing latency in nanoseconds.", metrics.ProposalLatencyNanos)
	writeGauge("vexo_proposal_latency_p95_nanos", "Rolling p95 proposal processing latency in nanoseconds.", metrics.ProposalLatencyP95Nanos)
	writeGauge("vexo_proposal_latency_p99_nanos", "Rolling p99 proposal processing latency in nanoseconds.", metrics.ProposalLatencyP99Nanos)
	writeGauge("vexo_vote_latency_nanos", "Observed vote processing latency in nanoseconds.", metrics.VoteLatencyNanos)
	writeGauge("vexo_vote_latency_p95_nanos", "Rolling p95 vote processing latency in nanoseconds.", metrics.VoteLatencyP95Nanos)
	writeGauge("vexo_vote_latency_p99_nanos", "Rolling p99 vote processing latency in nanoseconds.", metrics.VoteLatencyP99Nanos)
	writeGauge("vexo_mempool_size", "Current mempool size.", metrics.MempoolSize)
	writeGauge("vexo_commit_latency_nanos", "Observed commit latency in nanoseconds.", metrics.CommitLatencyNanos)
	writeGauge("vexo_commit_latency_p95_nanos", "Rolling p95 commit latency in nanoseconds.", metrics.CommitLatencyP95Nanos)
	writeGauge("vexo_commit_latency_p99_nanos", "Rolling p99 commit latency in nanoseconds.", metrics.CommitLatencyP99Nanos)
	writeGauge("vexo_snapshot_healthy", "Whether snapshot verification is healthy.", boolGauge(metrics.SnapshotHealthy))
	writeGauge("vexo_replay_healthy", "Whether replay verification is healthy.", boolGauge(metrics.ReplayHealthy))
	writeGauge("vexo_validator_signing_failures", "Validator signing failures.", metrics.SigningFailures)
	writeGauge("vexo_post_commit_reconciliation_failures", "Post-commit reconciliation failures recovered from durable state.", metrics.ReconciliationFailures)
	fmt.Fprintf(&builder, "# HELP vexo_height_rate_per_minute Height increase rate per minute.\n")
	fmt.Fprintf(&builder, "# TYPE vexo_height_rate_per_minute gauge\n")
	fmt.Fprintf(&builder, "vexo_height_rate_per_minute %.6f\n", metrics.HeightRatePerMinute)
	return builder.String()
}

func boolGauge(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func peerResponses(peers []p2p.PeerSnapshot) []PeerResponse {
	responses := make([]PeerResponse, 0, len(peers))
	for _, peer := range peers {
		response := PeerResponse{
			Peer:           string(peer.Peer),
			Score:          peer.Score,
			Banned:         peer.Banned,
			WindowMessages: peer.WindowMessages,
		}
		if !peer.BannedUntil.IsZero() {
			response.BannedUntil = peer.BannedUntil.UTC().Format(time.RFC3339Nano)
		}
		responses = append(responses, response)
	}
	return responses
}

func blockResponse(record store.BlockRecord) BlockResponse {
	txs := make([]string, 0, len(record.Block.Txs))
	for _, tx := range record.Block.Txs {
		txs = append(txs, base64.StdEncoding.EncodeToString(tx))
	}
	stateRoots := make([]StateRootResponse, 0, len(record.StateRoots))
	for _, root := range record.StateRoots {
		stateRoots = append(stateRoots, StateRootResponse{
			Height:    uint64(root.Height),
			Namespace: root.Namespace,
			Root:      hex.EncodeToString(root.Root[:]),
		})
	}
	return BlockResponse{
		Height:       uint64(record.Block.Header.Height),
		Hash:         hex.EncodeToString(record.Hash[:]),
		AppHash:      hex.EncodeToString(record.AppHash[:]),
		ChainID:      record.Block.Header.ChainID,
		TxCount:      len(record.Block.Txs),
		Txs:          txs,
		StateRoots:   stateRoots,
		TimeUnixNano: record.Block.Header.TimeUnixNano,
	}
}

func blockIndexResponse(index store.BlockIndex) BlockIndexResponse {
	return BlockIndexResponse{
		EarliestHeight: uint64(index.EarliestHeight),
		LatestHeight:   uint64(index.LatestHeight),
		TotalBlocks:    index.TotalBlocks,
	}
}

func stateResponse(state store.StateRecord) StateResponse {
	return StateResponse{
		Height:           uint64(state.Height),
		AppHash:          hex.EncodeToString(state.AppHash[:]),
		LastBlockHash:    hex.EncodeToString(state.LastBlockHash[:]),
		ValidatorSetHash: hex.EncodeToString(state.ValidatorSetHash[:]),
	}
}

func finalityProofResponse(proof finality.Proof) FinalityProofResponse {
	commitChain := make([]CommitLinkResponse, 0, len(proof.CommitChain))
	for _, link := range proof.CommitChain {
		commitChain = append(commitChain, CommitLinkResponse{
			Header:     headerResponse(link.Header),
			BlockHash:  hex.EncodeToString(link.BlockHash[:]),
			QuorumCert: quorumCertResponse(link.QuorumCert),
		})
	}
	return FinalityProofResponse{
		Height:             uint64(proof.Header.Height),
		BlockHash:          hex.EncodeToString(proof.BlockHash[:]),
		ValidatorSetHeight: uint64(proof.ValidatorSetHeight),
		ValidatorSetHash:   hex.EncodeToString(proof.ValidatorSetHash[:]),
		Strict:             proof.HasThreeChainCommitProof(),
		Header:             headerResponse(proof.Header),
		QuorumCert:         quorumCertResponse(proof.QuorumCert),
		CommitChain:        commitChain,
	}
}

func headerResponse(header types.Header) HeaderResponse {
	return HeaderResponse{
		ChainID:           header.ChainID,
		Height:            uint64(header.Height),
		TimeUnixNano:      header.TimeUnixNano,
		PreviousBlockHash: hex.EncodeToString(header.PreviousBlockHash[:]),
		AppHash:           hex.EncodeToString(header.AppHash[:]),
		ValidatorSetHash:  hex.EncodeToString(header.ValidatorSetHash[:]),
		ConsensusHash:     hex.EncodeToString(header.ConsensusHash[:]),
	}
}

func quorumCertResponse(cert finality.QuorumCert) QuorumCertResponse {
	return QuorumCertResponse{
		Height:      uint64(cert.Height),
		Round:       uint64(cert.Round),
		BlockHash:   hex.EncodeToString(cert.BlockHash[:]),
		Signers:     hex.EncodeToString(cert.Signers),
		Signature:   hex.EncodeToString(cert.Signature),
		VotingPower: uint64(cert.VotingPower),
	}
}

func eventsResponse(key string, value string, records []events.Record) EventsResponse {
	responses := make([]EventRecordResponse, 0, len(records))
	for _, record := range records {
		attributes := make([]EventAttributeResponse, 0, len(record.Event.Attributes))
		for _, attribute := range record.Event.Attributes {
			attributes = append(attributes, EventAttributeResponse{
				Key:   attribute.Key,
				Value: attribute.Value,
				Index: attribute.Index,
			})
		}
		responses = append(responses, EventRecordResponse{
			Height:  record.Height,
			TxIndex: record.TxIndex,
			Event: EventResponse{
				Type:       record.Event.Type,
				Attributes: attributes,
			},
		})
	}
	return EventsResponse{
		Key:     key,
		Value:   value,
		Records: responses,
	}
}

func stateSnapshotResponse(snapshot node.StateSnapshot) StateSnapshotResponse {
	roots := make([]StateRootResponse, 0, len(snapshot.StateRoots))
	for _, root := range snapshot.StateRoots {
		roots = append(roots, stateRootResponse(root))
	}
	return StateSnapshotResponse{
		Height:           uint64(snapshot.Height),
		AppHash:          hex.EncodeToString(snapshot.AppHash[:]),
		LastBlockHash:    hex.EncodeToString(snapshot.LastBlockHash[:]),
		ValidatorSetHash: hex.EncodeToString(snapshot.ValidatorSetHash[:]),
		StateRoots:       roots,
	}
}

func snapshotExportResponse(snapshot node.StateSnapshot) SnapshotExportResponse {
	response := SnapshotExportResponse{
		SchemaVersion: "v1",
		State: store.StateRecord{
			Height:           snapshot.Height,
			AppHash:          snapshot.AppHash,
			LastBlockHash:    snapshot.LastBlockHash,
			ValidatorSetHash: snapshot.ValidatorSetHash,
		},
		StateRoots: sortedStateRoots(snapshot.StateRoots),
		KV:         sortedKVPairs(snapshot.KV),
	}
	response.Checksum = snapshotChecksum(response)
	return response
}

func snapshotChunkResponse(snapshot node.StateSnapshot, index uint64, size uint64) (SnapshotChunkResponse, error) {
	export := snapshotExportResponse(snapshot)
	kv := sortedKVPairs(export.KV)
	chunkCount := uint64(1)
	if len(kv) > 0 {
		chunkCount = uint64((len(kv) + int(size) - 1) / int(size))
	}
	if index >= chunkCount {
		return SnapshotChunkResponse{}, fmt.Errorf("snapshot chunk index out of range: %d/%d", index, chunkCount)
	}
	start := int(index * size)
	end := start + int(size)
	if start > len(kv) {
		start = len(kv)
	}
	if end > len(kv) {
		end = len(kv)
	}
	chunk := SnapshotChunkResponse{
		SchemaVersion:    "v1",
		State:            export.State,
		StateRoots:       append([]store.StateRootRecord(nil), export.StateRoots...),
		KV:               sortedKVPairs(kv[start:end]),
		ChunkIndex:       index,
		ChunkCount:       chunkCount,
		SnapshotChecksum: export.Checksum,
	}
	chunk.ChunkChecksum = snapshotChunkChecksum(chunk)
	return chunk, nil
}

func recoveryReportResponse(report node.RecoveryReport) RecoveryReportResponse {
	response := RecoveryReportResponse{
		OK:                report.OK,
		Running:           report.Running,
		LatestHeight:      uint64(report.LatestHeight),
		LatestStateHeight: uint64(report.LatestStateHeight),
		SafeHeight:        uint64(report.SafeHeight),
		EarliestBlock:     uint64(report.EarliestBlock),
		LatestBlock:       uint64(report.LatestBlock),
		TotalBlocks:       report.TotalBlocks,
		SnapshotAvailable: report.SnapshotAvailable,
		Repaired:          report.Repaired,
		Problems:          append([]string(nil), report.Problems...),
	}
	if report.Repaired {
		recoverResult := recoverIndexesResponse(report.RecoverResult)
		response.RecoverResult = &recoverResult
	}
	return response
}

func recoverIndexesResponse(result store.RecoverResult) RecoverIndexesResponse {
	return RecoverIndexesResponse{
		BlockIndexKeys:   result.BlockIndexKeys,
		EvidenceKeys:     result.EvidenceKeys,
		EarliestHeight:   uint64(result.EarliestHeight),
		LatestHeight:     uint64(result.LatestHeight),
		RecoveredIndexes: result.RecoveredIndexes,
	}
}

func writeRecoveryReport(writer http.ResponseWriter, report node.RecoveryReport, err error) {
	if err != nil && len(report.Problems) == 0 {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	statusCode := http.StatusOK
	if !report.OK {
		statusCode = http.StatusServiceUnavailable
	}
	writeJSON(writer, statusCode, recoveryReportResponse(report))
}

func stateRootResponse(root store.StateRootRecord) StateRootResponse {
	return StateRootResponse{
		Height:    uint64(root.Height),
		Namespace: root.Namespace,
		Root:      hex.EncodeToString(root.Root[:]),
	}
}

func snapshotChecksum(response SnapshotExportResponse) string {
	response.Checksum = ""
	response.StateRoots = sortedStateRoots(response.StateRoots)
	response.KV = sortedKVPairs(response.KV)
	data, _ := json.Marshal(response)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func snapshotChunkChecksum(response SnapshotChunkResponse) string {
	response.ChunkChecksum = ""
	response.StateRoots = sortedStateRoots(response.StateRoots)
	response.KV = sortedKVPairs(response.KV)
	data, _ := json.Marshal(response)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortedStateRoots(roots []store.StateRootRecord) []store.StateRootRecord {
	sorted := append([]store.StateRootRecord(nil), roots...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Height != sorted[j].Height {
			return sorted[i].Height < sorted[j].Height
		}
		return sorted[i].Namespace < sorted[j].Namespace
	})
	return sorted
}

func sortedKVPairs(pairs []store.KVPair) []store.KVPair {
	sorted := append([]store.KVPair(nil), pairs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return bytes.Compare(sorted[i].Key, sorted[j].Key) < 0
	})
	return sorted
}

func pruneResponse(result store.PruneResult) PruneResponse {
	return PruneResponse{
		RetainFromHeight: uint64(result.RetainFromHeight),
		PrunedBlocks:     result.PrunedBlocks,
		PrunedStates:     result.PrunedStates,
		PrunedStateRoots: result.PrunedStateRoots,
	}
}

func replayResponse(result vexoruntime.ReplayResult) ReplayResponse {
	return ReplayResponse{
		FromHeight: uint64(result.FromHeight),
		ToHeight:   uint64(result.ToHeight),
		LastHash:   hex.EncodeToString(result.LastHash[:]),
		Blocks:     result.Blocks,
	}
}

func consensusLoopResponse(action string, running bool, cfg node.ConsensusLoopConfig) ConsensusLoopResponse {
	return ConsensusLoopResponse{
		Running:            running,
		Action:             action,
		IntervalMillis:     uint64(cfg.Interval / time.Millisecond),
		RoundTimeoutMillis: uint64(cfg.RoundTimeout / time.Millisecond),
		MaxBlockBytes:      cfg.MaxBlockBytes,
	}
}

func writeConsensusLoopError(writer http.ResponseWriter, err error) {
	if errors.Is(err, node.ErrNodeNotRunning) ||
		errors.Is(err, node.ErrLoopAlreadyRunning) ||
		errors.Is(err, node.ErrLoopNotRunning) {
		writeError(writer, http.StatusConflict, err.Error())
		return
	}
	writeError(writer, http.StatusInternalServerError, err.Error())
}

func evidenceResponse(evidence slashing.Evidence, result consensus.SlashResult, applied bool) SubmitEvidenceResponse {
	response := SubmitEvidenceResponse{
		Accepted:  true,
		Applied:   applied,
		Type:      string(evidence.Type),
		Validator: string(evidence.Validator),
		Height:    uint64(evidence.Height),
		Round:     uint64(evidence.Round),
	}
	if applied {
		response.PreviousPower = uint64(result.PreviousPower)
		response.RemainingPower = uint64(result.RemainingPower)
		response.Penalty = PenaltyResponse{
			SlashFraction: result.Receipt.Penalty.SlashFraction,
			JailDuration:  result.Receipt.Penalty.JailDuration,
		}
	}
	return response
}

func validatorSetResponse(height types.Height, validatorSet validator.Set) ValidatorSetResponse {
	validators := validatorSet.List()
	responses := make([]ValidatorResponse, 0, len(validators))
	var totalPower uint64
	for _, validatorInfo := range validators {
		totalPower = types.MustAddUint64Saturating(totalPower, uint64(validatorInfo.VotingPower))
		responses = append(responses, validatorResponse(validatorInfo))
	}
	hash := validatorSet.Hash()
	return ValidatorSetResponse{
		Height:           uint64(height),
		TotalValidators:  len(responses),
		TotalPower:       totalPower,
		ValidatorSetHash: hex.EncodeToString(hash[:]),
		Validators:       responses,
	}
}

func validatorResponse(validatorInfo validator.Validator) ValidatorResponse {
	return ValidatorResponse{
		ID:          string(validatorInfo.ID),
		Address:     string(validatorInfo.Address),
		VotingPower: uint64(validatorInfo.VotingPower),
		Stake:       validatorInfo.Stake,
		Metadata:    validatorInfo.Metadata,
	}
}

func committeeResponse(height types.Height, seed types.Hash, committeeResult committee.Committee) CommitteeResponse {
	members := make([]CommitteeMemberResponse, 0, len(committeeResult.Members))
	for _, member := range committeeResult.Members {
		members = append(members, CommitteeMemberResponse{
			Validator: validatorResponse(member.Validator),
			Weight:    uint64(member.Weight),
			Proof:     hex.EncodeToString(member.Proof),
		})
	}
	return CommitteeResponse{
		Height:  uint64(height),
		Epoch:   committeeResult.Epoch,
		Round:   uint64(committeeResult.Round),
		Seed:    hex.EncodeToString(seed[:]),
		Members: members,
	}
}

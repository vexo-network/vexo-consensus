package rpc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type fakeStatusProvider struct {
	status            node.Status
	statusDeadline    chan bool
	statusWaitCancel  chan struct{}
	metrics           node.Metrics
	metricsErr        error
	snapshot          node.StateSnapshot
	snapshotErr       error
	recoveryReport    node.RecoveryReport
	recoveryErr       error
	recoveryRepairs   int
	submitErr         error
	submitted         []types.Tx
	blocks            map[types.Height]store.BlockRecord
	blocksByHash      map[types.Hash]store.BlockRecord
	latest            types.Height
	blockErr          error
	index             store.BlockIndex
	state             store.StateRecord
	roots             map[string]store.StateRootRecord
	stateErr          error
	eventRecords      []events.Record
	eventErr          error
	queryProof        queryproof.Proof
	queryProofErr     error
	ibcQueryResponse  vexoapp.QueryResponse
	ibcQueryErr       error
	ibcQueryPath      []string
	appQueryResponse  vexoapp.QueryResponse
	appQueryErr       error
	appQueryPath      []string
	appQueryData      []byte
	pruneResult       store.PruneResult
	pruneErr          error
	prunedHeights     []types.Height
	replayResult      vexoruntime.ReplayResult
	replayErr         error
	replayAllCalled   bool
	replayRanges      [][2]types.Height
	loopStartErr      error
	loopStopErr       error
	loopRunning       bool
	loopStartConfigs  []node.ConsensusLoopConfig
	validators        validator.Set
	committee         committee.Committee
	validatorErr      error
	evidenceResult    consensus.SlashResult
	evidenceApplied   bool
	evidenceErr       error
	evidenceSubmitted []slashing.Evidence
	accountSequence   uint64
	accountErr        error
	accountAddress    types.Address
	finalityProof     finality.Proof
	finalityErr       error
	finalityHeight    types.Height
}

func (provider fakeStatusProvider) Status(ctx context.Context) node.Status {
	if provider.statusDeadline != nil {
		_, ok := ctx.Deadline()
		provider.statusDeadline <- ok
	}
	if provider.statusWaitCancel != nil {
		<-ctx.Done()
		close(provider.statusWaitCancel)
	}
	return provider.status
}

func (provider fakeStatusProvider) Metrics(ctx context.Context) (node.Metrics, error) {
	if provider.metricsErr != nil {
		return node.Metrics{}, provider.metricsErr
	}
	return provider.metrics, nil
}

func (provider fakeStatusProvider) StateSnapshot(ctx context.Context) (node.StateSnapshot, error) {
	if provider.snapshotErr != nil {
		return node.StateSnapshot{}, provider.snapshotErr
	}
	return provider.snapshot, nil
}

func (provider *fakeStatusProvider) RecoveryReport(ctx context.Context, repairIndexes bool) (node.RecoveryReport, error) {
	if repairIndexes {
		provider.recoveryRepairs++
	}
	if provider.recoveryErr != nil {
		return provider.recoveryReport, provider.recoveryErr
	}
	return provider.recoveryReport, nil
}

func (provider *fakeStatusProvider) SubmitTx(ctx context.Context, tx types.Tx) error {
	if provider.submitErr != nil {
		return provider.submitErr
	}
	provider.submitted = append(provider.submitted, append(types.Tx(nil), tx...))
	return nil
}

func (provider fakeStatusProvider) BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error) {
	if provider.blockErr != nil {
		return store.BlockRecord{}, provider.blockErr
	}
	record, ok := provider.blocks[height]
	if !ok {
		return store.BlockRecord{}, store.ErrBlockNotFound
	}
	return record, nil
}

func (provider fakeStatusProvider) BlockByHash(ctx context.Context, hash types.Hash) (store.BlockRecord, error) {
	if provider.blockErr != nil {
		return store.BlockRecord{}, provider.blockErr
	}
	record, ok := provider.blocksByHash[hash]
	if !ok {
		return store.BlockRecord{}, store.ErrBlockNotFound
	}
	return record, nil
}

func (provider fakeStatusProvider) LatestBlock(ctx context.Context) (store.BlockRecord, error) {
	if provider.blockErr != nil {
		return store.BlockRecord{}, provider.blockErr
	}
	if provider.latest == 0 {
		return store.BlockRecord{}, store.ErrBlockIndexNotFound
	}
	return provider.BlockByHeight(ctx, provider.latest)
}

func (provider fakeStatusProvider) BlockIndex(ctx context.Context) (store.BlockIndex, error) {
	if provider.blockErr != nil {
		return store.BlockIndex{}, provider.blockErr
	}
	if provider.index.TotalBlocks == 0 {
		return store.BlockIndex{}, store.ErrBlockIndexNotFound
	}
	return provider.index, nil
}

func (provider fakeStatusProvider) LatestState(ctx context.Context) (store.StateRecord, error) {
	if provider.stateErr != nil {
		return store.StateRecord{}, provider.stateErr
	}
	if provider.state.Height == 0 {
		return store.StateRecord{}, store.ErrStateNotFound
	}
	return provider.state, nil
}

func (provider fakeStatusProvider) StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error) {
	if provider.stateErr != nil {
		return store.StateRootRecord{}, provider.stateErr
	}
	record, ok := provider.roots[stateRootKey(height, namespace)]
	if !ok {
		return store.StateRootRecord{}, store.ErrStateRootNotFound
	}
	return record, nil
}

func (provider fakeStatusProvider) QueryEvents(ctx context.Context, key string, value string) ([]events.Record, error) {
	if provider.eventErr != nil {
		return nil, provider.eventErr
	}
	return append([]events.Record(nil), provider.eventRecords...), nil
}

func (provider fakeStatusProvider) QueryProof(ctx context.Context, height types.Height, namespace string, key []byte) (queryproof.Proof, error) {
	if provider.queryProofErr != nil {
		return queryproof.Proof{}, provider.queryProofErr
	}
	return provider.queryProof, nil
}

func (provider *fakeStatusProvider) IBCQuery(ctx context.Context, path []string) (vexoapp.QueryResponse, error) {
	provider.ibcQueryPath = append([]string(nil), path...)
	if provider.ibcQueryErr != nil {
		return vexoapp.QueryResponse{}, provider.ibcQueryErr
	}
	return provider.ibcQueryResponse, nil
}

func (provider *fakeStatusProvider) AppQuery(ctx context.Context, path []string, data []byte) (vexoapp.QueryResponse, error) {
	provider.appQueryPath = append([]string(nil), path...)
	provider.appQueryData = append([]byte(nil), data...)
	if provider.appQueryErr != nil {
		return vexoapp.QueryResponse{}, provider.appQueryErr
	}
	return provider.appQueryResponse, nil
}

func (provider *fakeStatusProvider) AccountSequence(ctx context.Context, address types.Address) (uint64, error) {
	provider.accountAddress = address
	if provider.accountErr != nil {
		return 0, provider.accountErr
	}
	return provider.accountSequence, nil
}

func (provider *fakeStatusProvider) FinalityProof(ctx context.Context, height types.Height) (finality.Proof, error) {
	provider.finalityHeight = height
	if provider.finalityErr != nil {
		return finality.Proof{}, provider.finalityErr
	}
	return provider.finalityProof, nil
}

func (provider *fakeStatusProvider) LatestFinalityProof(ctx context.Context) (finality.Proof, error) {
	if provider.finalityErr != nil {
		return finality.Proof{}, provider.finalityErr
	}
	return provider.finalityProof, nil
}

func (provider *fakeStatusProvider) PruneBelow(ctx context.Context, retainFrom types.Height) (store.PruneResult, error) {
	if provider.pruneErr != nil {
		return store.PruneResult{}, provider.pruneErr
	}
	provider.prunedHeights = append(provider.prunedHeights, retainFrom)
	return provider.pruneResult, nil
}

func (provider *fakeStatusProvider) Replay(ctx context.Context, from types.Height, to types.Height) (vexoruntime.ReplayResult, error) {
	if provider.replayErr != nil {
		return vexoruntime.ReplayResult{}, provider.replayErr
	}
	provider.replayRanges = append(provider.replayRanges, [2]types.Height{from, to})
	return provider.replayResult, nil
}

func (provider *fakeStatusProvider) ReplayAll(ctx context.Context) (vexoruntime.ReplayResult, error) {
	if provider.replayErr != nil {
		return vexoruntime.ReplayResult{}, provider.replayErr
	}
	provider.replayAllCalled = true
	return provider.replayResult, nil
}

func (provider *fakeStatusProvider) StartConsensusLoop(ctx context.Context, cfg node.ConsensusLoopConfig) error {
	if provider.loopStartErr != nil {
		return provider.loopStartErr
	}
	provider.loopStartConfigs = append(provider.loopStartConfigs, cfg)
	provider.loopRunning = true
	return nil
}

func (provider *fakeStatusProvider) StopConsensusLoop(ctx context.Context) error {
	if provider.loopStopErr != nil {
		return provider.loopStopErr
	}
	provider.loopRunning = false
	return nil
}

func (provider *fakeStatusProvider) ConsensusLoopRunning() bool {
	return provider.loopRunning
}

func (provider fakeStatusProvider) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	if provider.validatorErr != nil {
		return nil, provider.validatorErr
	}
	if provider.validators == nil {
		return nil, errors.New("validator set failed")
	}
	return provider.validators, nil
}

func (provider fakeStatusProvider) Committee(ctx context.Context, height types.Height, round types.Round, seed types.Hash) (committee.Committee, error) {
	if provider.validatorErr != nil {
		return committee.Committee{}, provider.validatorErr
	}
	if len(provider.committee.Members) == 0 {
		return committee.Committee{}, errors.New("committee failed")
	}
	return provider.committee, nil
}

func (provider *fakeStatusProvider) SubmitEvidence(ctx context.Context, evidence slashing.Evidence) (consensus.SlashResult, bool, error) {
	if provider.evidenceErr != nil {
		return consensus.SlashResult{}, false, provider.evidenceErr
	}
	provider.evidenceSubmitted = append(provider.evidenceSubmitted, evidence)
	return provider.evidenceResult, provider.evidenceApplied, nil
}

func TestHandlerReportsHealthStatusAndPeers(t *testing.T) {
	appHash := types.Hash{1, 2, 3}
	bannedUntil := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(fakeStatusProvider{status: node.Status{
		ChainID:       "vexo-test",
		Running:       true,
		LatestHeight:  7,
		LatestAppHash: appHash,
		DataDir:       "/tmp/vexo",
		PeerCount:     2,
		BannedPeers:   1,
		Peers: []p2p.PeerSnapshot{
			{Peer: "alice", Score: 12, WindowMessages: 3},
			{Peer: "mallory", Score: -1, Banned: true, BannedUntil: bannedUntil, WindowMessages: 9},
		},
	}})

	var health HealthResponse
	getJSON(t, handler, "/healthz", http.StatusOK, &health)
	if !health.OK {
		t.Fatal("expected health ok")
	}

	var ready HealthResponse
	getJSON(t, handler, "/readyz", http.StatusOK, &ready)
	if !ready.OK {
		t.Fatal("expected ready ok")
	}

	var status StatusResponse
	getJSON(t, handler, "/status", http.StatusOK, &status)
	if status.ChainID != "vexo-test" || !status.Running || status.LatestHeight != 7 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.LatestAppHash[:6] != "010203" || status.PeerCount != 2 || status.BannedPeers != 1 {
		t.Fatalf("unexpected status metrics: %+v", status)
	}

	var peers PeersResponse
	getJSON(t, handler, "/peers", http.StatusOK, &peers)
	if len(peers.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers.Peers))
	}
	if peers.Peers[0].Peer != "alice" || peers.Peers[0].Score != 12 || peers.Peers[0].Banned {
		t.Fatalf("unexpected first peer: %+v", peers.Peers[0])
	}
	if peers.Peers[1].Peer != "mallory" || !peers.Peers[1].Banned || peers.Peers[1].BannedUntil == "" {
		t.Fatalf("unexpected banned peer: %+v", peers.Peers[1])
	}
}

func TestHandlerExposesStableV1Routes(t *testing.T) {
	record := store.BlockRecord{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 2, TimeUnixNano: 42},
			Txs:    []types.Tx{[]byte("bank:first"), []byte("bank:second")},
		},
		Hash:    types.Hash{2},
		AppHash: types.Hash{3},
		StateRoots: []store.StateRootRecord{
			{Height: 2, Namespace: "bank", Root: types.Hash{4}},
		},
	}
	handler := NewHandler(fakeStatusProvider{
		status: node.Status{ChainID: "vexo-test", Running: true, LatestHeight: 2},
		blocks: map[types.Height]store.BlockRecord{2: record},
		latest: 2,
		index:  store.BlockIndex{EarliestHeight: 1, LatestHeight: 2, TotalBlocks: 2},
	})

	var status StatusResponse
	getJSON(t, handler, "/v1/status", http.StatusOK, &status)
	if status.ChainID != "vexo-test" || status.LatestHeight != 2 {
		t.Fatalf("unexpected v1 status: %+v", status)
	}

	var latest BlockResponse
	getJSON(t, handler, "/v1/blocks/latest", http.StatusOK, &latest)
	assertBlockResponse(t, latest)

	request := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("X-Vexo-RPC-Version") != "v1" {
		t.Fatalf("expected v1 response header, got %q", response.Header().Get("X-Vexo-RPC-Version"))
	}
}

func TestHandlerReportsNotReadyWhenNodeStopped(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{status: node.Status{ChainID: "vexo-test"}})

	var ready HealthResponse
	getJSON(t, handler, "/readyz", http.StatusServiceUnavailable, &ready)
	if ready.OK {
		t.Fatal("expected not ready")
	}
}

func TestHandlerReportsMetrics(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{metrics: node.Metrics{
		ChainID:              "vexo-test",
		Running:              true,
		StartedAtUnix:        1710000000,
		UptimeSeconds:        42,
		DataDir:              "/tmp/vexo",
		LatestHeight:         9,
		LatestAppHash:        types.Hash{1, 2, 3},
		EarliestBlockHeight:  1,
		LatestBlockHeight:    9,
		TotalBlocks:          9,
		ValidatorCount:       4,
		TotalVotingPower:     100,
		ValidatorSetHash:     types.Hash{4, 5, 6},
		PeerCount:            3,
		BannedPeers:          1,
		PeerWindowMessages:   12,
		ConsensusLoopRunning: true,
	}})

	var metrics MetricsResponse
	getJSON(t, handler, "/metrics", http.StatusOK, &metrics)
	if metrics.ChainID != "vexo-test" || !metrics.Running || metrics.StartedAtUnix != 1710000000 || metrics.UptimeSeconds != 42 || metrics.LatestHeight != 9 || metrics.TotalBlocks != 9 {
		t.Fatalf("unexpected metrics identity: %+v", metrics)
	}
	if metrics.ValidatorCount != 4 || metrics.TotalVotingPower != 100 || metrics.PeerCount != 3 || metrics.BannedPeers != 1 || !metrics.ConsensusLoopRunning {
		t.Fatalf("unexpected metrics counters: %+v", metrics)
	}
	if metrics.LatestAppHash[:6] != "010203" || metrics.ValidatorSetHash[:6] != "040506" {
		t.Fatalf("unexpected metrics hashes: %+v", metrics)
	}
}

func TestHandlerReportsMetricsText(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{metrics: node.Metrics{
		Running:              true,
		StartedAtUnix:        1710000000,
		UptimeSeconds:        42,
		LatestHeight:         9,
		EarliestBlockHeight:  1,
		LatestBlockHeight:    9,
		TotalBlocks:          9,
		ValidatorCount:       4,
		TotalVotingPower:     100,
		PeerCount:            3,
		BannedPeers:          1,
		PeerWindowMessages:   12,
		ConsensusLoopRunning: true,
	}})

	body := getText(t, handler, "/metrics/text", http.StatusOK)
	for _, expected := range []string{
		"# TYPE vexo_node_running gauge",
		"vexo_node_running 1",
		"vexo_started_at_unix 1710000000",
		"vexo_uptime_seconds 42",
		"vexo_latest_height 9",
		"vexo_total_blocks 9",
		"vexo_validator_count 4",
		"vexo_total_voting_power 100",
		"vexo_peer_count 3",
		"vexo_banned_peers 1",
		"vexo_peer_window_messages 12",
		"vexo_consensus_loop_running 1",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected metrics text to contain %q, got:\n%s", expected, body)
		}
	}
}

func TestHandlerRejectsUnavailableMetricsProvider(t *testing.T) {
	var response map[string]string
	getJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/metrics", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable metrics error, got %+v", response)
	}
}

func TestHandlerReportsMetricsProviderErrors(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{metricsErr: errors.New("metrics failed")})

	var response map[string]string
	getJSON(t, handler, "/metrics", http.StatusInternalServerError, &response)
	if response["error"] != "metrics failed" {
		t.Fatalf("unexpected metrics error: %+v", response)
	}

	getJSON(t, handler, "/metrics/text", http.StatusInternalServerError, &response)
	if response["error"] != "metrics failed" {
		t.Fatalf("unexpected metrics text error: %+v", response)
	}
}

func TestHandlerReportsHealthyDiagnostics(t *testing.T) {
	bannedUntil := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(fakeStatusProvider{
		status: node.Status{
			ChainID:       "vexo-test",
			Running:       true,
			LatestHeight:  9,
			LatestAppHash: types.Hash{1, 2, 3},
			DataDir:       "/tmp/vexo",
			PeerCount:     2,
			BannedPeers:   1,
			Peers: []p2p.PeerSnapshot{
				{Peer: "alice", Score: 10, WindowMessages: 3},
				{Peer: "mallory", Score: -5, Banned: true, BannedUntil: bannedUntil, WindowMessages: 9},
			},
		},
		metrics: node.Metrics{
			ChainID:              "vexo-test",
			Running:              true,
			LatestHeight:         9,
			TotalBlocks:          9,
			ValidatorCount:       4,
			TotalVotingPower:     100,
			PeerCount:            2,
			BannedPeers:          1,
			PeerWindowMessages:   12,
			ConsensusLoopRunning: true,
		},
		index: store.BlockIndex{EarliestHeight: 1, LatestHeight: 9, TotalBlocks: 9},
	})

	var diagnostics DiagnosticsResponse
	getJSON(t, handler, "/diagnostics", http.StatusOK, &diagnostics)
	if !diagnostics.OK || diagnostics.Status != "healthy" {
		t.Fatalf("expected healthy diagnostics, got %+v", diagnostics)
	}
	if diagnostics.Node.ChainID != "vexo-test" || diagnostics.Node.LatestHeight != 9 {
		t.Fatalf("unexpected node diagnostics: %+v", diagnostics.Node)
	}
	if diagnostics.Metrics == nil || diagnostics.Metrics.ValidatorCount != 4 || diagnostics.Metrics.TotalVotingPower != 100 {
		t.Fatalf("unexpected metrics diagnostics: %+v", diagnostics.Metrics)
	}
	if diagnostics.Storage == nil || diagnostics.Storage.EarliestHeight != 1 || diagnostics.Storage.LatestHeight != 9 || diagnostics.Storage.TotalBlocks != 9 {
		t.Fatalf("unexpected storage diagnostics: %+v", diagnostics.Storage)
	}
	if len(diagnostics.Peers) != 2 || diagnostics.Peers[1].Peer != "mallory" || !diagnostics.Peers[1].Banned {
		t.Fatalf("unexpected peer diagnostics: %+v", diagnostics.Peers)
	}
	assertDiagnosticCheck(t, diagnostics.Checks, "status", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "ready", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "metrics", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "storage", true)
}

func TestHandlerReportsDegradedDiagnostics(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{
		status:     node.Status{ChainID: "vexo-test", Running: true},
		metricsErr: errors.New("metrics failed"),
		blockErr:   errors.New("storage failed"),
	})

	var diagnostics DiagnosticsResponse
	getJSON(t, handler, "/diagnostics", http.StatusServiceUnavailable, &diagnostics)
	if diagnostics.OK || diagnostics.Status != "degraded" {
		t.Fatalf("expected degraded diagnostics, got %+v", diagnostics)
	}
	assertDiagnosticCheck(t, diagnostics.Checks, "status", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "ready", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "metrics", false)
	assertDiagnosticCheck(t, diagnostics.Checks, "storage", false)
	if diagnostics.Metrics != nil || diagnostics.Storage != nil {
		t.Fatalf("expected missing failed optional sections, got metrics=%+v storage=%+v", diagnostics.Metrics, diagnostics.Storage)
	}
}

func TestHandlerReportsNotReadyDiagnostics(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{
		status:  node.Status{ChainID: "vexo-test", Running: false},
		metrics: node.Metrics{ChainID: "vexo-test", Running: false},
		index:   store.BlockIndex{EarliestHeight: 1, LatestHeight: 1, TotalBlocks: 1},
	})

	var diagnostics DiagnosticsResponse
	getJSON(t, handler, "/diagnostics", http.StatusServiceUnavailable, &diagnostics)
	if diagnostics.OK || diagnostics.Status != "not_ready" {
		t.Fatalf("expected not_ready diagnostics, got %+v", diagnostics)
	}
	assertDiagnosticCheck(t, diagnostics.Checks, "status", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "ready", false)
	assertDiagnosticCheck(t, diagnostics.Checks, "metrics", true)
	assertDiagnosticCheck(t, diagnostics.Checks, "storage", true)
}

func TestHandlerAppliesRequestTimeoutContext(t *testing.T) {
	deadline := make(chan bool, 1)
	cancelled := make(chan struct{})
	handler := NewHandlerWithConfig(fakeStatusProvider{
		statusDeadline:   deadline,
		statusWaitCancel: cancelled,
	}, Config{RequestTimeout: time.Nanosecond})

	var ready HealthResponse
	getJSON(t, handler, "/readyz", http.StatusServiceUnavailable, &ready)

	if !<-deadline {
		t.Fatal("expected status context deadline")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request context cancellation")
	}
}

func TestHandlerRateLimitsRequests(t *testing.T) {
	handler := NewHandlerWithConfig(fakeStatusProvider{status: node.Status{Running: true}}, Config{
		RateLimitWindow:      time.Hour,
		RateLimitMaxRequests: 2,
	})

	var health HealthResponse
	getJSON(t, handler, "/healthz", http.StatusOK, &health)
	getJSON(t, handler, "/healthz", http.StatusOK, &health)

	var response map[string]string
	getJSON(t, handler, "/healthz", http.StatusTooManyRequests, &response)
	if response["error"] != "rate limit exceeded" {
		t.Fatalf("unexpected rate limit response: %+v", response)
	}
}

func TestHandlerServesPprofWhenEnabled(t *testing.T) {
	enabled := NewHandlerWithConfig(fakeStatusProvider{}, Config{EnablePprof: true})
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	response := httptest.NewRecorder()
	enabled.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected pprof enabled status 200, got %d", response.Code)
	}

	disabled := NewHandler(fakeStatusProvider{})
	request = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	response = httptest.NewRecorder()
	disabled.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected pprof disabled status 404, got %d", response.Code)
	}
}

func TestHandlerRateLimitSeparatesClientIPs(t *testing.T) {
	handler := NewHandlerWithConfig(fakeStatusProvider{status: node.Status{Running: true}}, Config{
		RateLimitWindow:      time.Hour,
		RateLimitMaxRequests: 1,
	})

	var health HealthResponse
	requestJSON(t, handler, http.MethodGet, "/healthz", "", "10.0.0.1:1000", http.StatusOK, &health)

	var response map[string]string
	requestJSON(t, handler, http.MethodGet, "/healthz", "", "10.0.0.1:1001", http.StatusTooManyRequests, &response)
	requestJSON(t, handler, http.MethodGet, "/healthz", "", "10.0.0.2:1000", http.StatusOK, &health)
}

func TestHandlerRateLimitIgnoresForwardedForHeader(t *testing.T) {
	handler := NewHandlerWithConfig(fakeStatusProvider{status: node.Status{Running: true}}, Config{
		RateLimitWindow:      time.Hour,
		RateLimitMaxRequests: 1,
	})

	var health HealthResponse
	requestJSONWithHeaders(t, handler, http.MethodGet, "/healthz", "", "10.0.0.1:1000", map[string]string{"X-Forwarded-For": "203.0.113.1"}, http.StatusOK, &health)

	var response map[string]string
	requestJSONWithHeaders(t, handler, http.MethodGet, "/healthz", "", "10.0.0.1:1001", map[string]string{"X-Forwarded-For": "203.0.113.2"}, http.StatusTooManyRequests, &response)
	if response["error"] != "rate limit exceeded" {
		t.Fatalf("unexpected rate limit response: %+v", response)
	}
}

func TestHandlerRejectsNonGET(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/status", nil)
	response := httptest.NewRecorder()

	NewHandler(fakeStatusProvider{}).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected allow GET, got %q", allow)
	}
}

func TestHandlerSubmitsBase64Transaction(t *testing.T) {
	provider := &fakeStatusProvider{status: node.Status{Running: true}}
	handler := NewHandler(provider)

	var response SubmitTxResponse
	body := `{"tx":"` + base64.StdEncoding.EncodeToString([]byte("bank:send")) + `"}`
	postJSON(t, handler, "/tx", body, http.StatusAccepted, &response)

	if !response.Accepted || response.TxHash == "" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(provider.submitted) != 1 || string(provider.submitted[0]) != "bank:send" {
		t.Fatalf("unexpected submitted txs: %+v", provider.submitted)
	}
}

func TestHandlerServesWeb3JSONRPC(t *testing.T) {
	blockHash := types.Hash{0xab}
	parentHash := types.Hash{0xcd}
	block := store.BlockRecord{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-chain", Height: 12, PreviousBlockHash: parentHash, TimeUnixNano: int64(1700000000 * time.Second)},
			Txs:    []types.Tx{[]byte("bank:send")},
		},
		Hash:    blockHash,
		AppHash: types.Hash{0xef},
	}
	provider := &fakeStatusProvider{
		status:           node.Status{ChainID: "vexo-chain", LatestHeight: 12},
		state:            store.StateRecord{Height: 12, BaseFee: 9, NextBaseFee: 11},
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"0xabc","status":1,"gas_used":7,"logs":[{"address":"0xcontract","data":"0x01"}]}`)},
		blocks:           map[types.Height]store.BlockRecord{12: block},
		blocksByHash:     map[types.Hash]store.BlockRecord{blockHash: block},
		latest:           12,
		index:            store.BlockIndex{EarliestHeight: 12, LatestHeight: 12, TotalBlocks: 1},
		accountSequence:  7,
	}
	handler := NewHandler(provider)

	var blockNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`, http.StatusOK, &blockNumber)
	if blockNumber.Error != nil || blockNumber.Result != "0xc" {
		t.Fatalf("unexpected block number response: %+v", blockNumber)
	}

	var sendRaw JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":2,"method":"eth_sendRawTransaction","params":["0x62616e6b3a73656e64"]}`, http.StatusOK, &sendRaw)
	if sendRaw.Error != nil || len(provider.submitted) != 1 || string(provider.submitted[0]) != "bank:send" {
		t.Fatalf("unexpected sendRaw response=%+v submitted=%q", sendRaw, provider.submitted)
	}
	if result, ok := sendRaw.Result.(string); !ok || !strings.HasPrefix(result, "0x") {
		t.Fatalf("expected tx hash result, got %+v", sendRaw.Result)
	}

	var gasPrice JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":30,"method":"eth_gasPrice","params":[]}`, http.StatusOK, &gasPrice)
	if gasPrice.Error != nil || gasPrice.Result != "0xb" {
		t.Fatalf("unexpected gas price response: %+v", gasPrice)
	}

	var blockByNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":6,"method":"eth_getBlockByNumber","params":["latest",true]}`, http.StatusOK, &blockByNumber)
	if blockByNumber.Error != nil {
		t.Fatalf("unexpected block by number error: %+v", blockByNumber)
	}
	blockResult, ok := blockByNumber.Result.(map[string]any)
	if !ok || blockResult["number"] != "0xc" || blockResult["hash"] != "0xab00000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("unexpected block by number response: %+v", blockByNumber.Result)
	}

	var blockByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":7,"method":"eth_getBlockByHash","params":["0xab00000000000000000000000000000000000000000000000000000000000000",false]}`, http.StatusOK, &blockByHash)
	if blockByHash.Error != nil {
		t.Fatalf("unexpected block by hash error: %+v", blockByHash)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`123`)}
	var balance JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":31,"method":"eth_getBalance","params":["0xaaaa","latest"]}`, http.StatusOK, &balance)
	if balance.Error != nil || balance.Result != "0x7b" {
		t.Fatalf("unexpected balance response: %+v", balance)
	}
	if provider.appQueryPath[0] != "bank" || provider.appQueryPath[1] != "balance" || provider.appQueryPath[2] != "0xaaaa" {
		t.Fatalf("unexpected balance query path: %+v", provider.appQueryPath)
	}

	var txCount JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":33,"method":"eth_getTransactionCount","params":["0xaaaa","latest"]}`, http.StatusOK, &txCount)
	if txCount.Error != nil || txCount.Result != "0x7" || provider.accountAddress != "0xaaaa" {
		t.Fatalf("unexpected transaction count response: %+v address=%s", txCount, provider.accountAddress)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xbbbb","code":"60016002"}`)}
	var code JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":34,"method":"eth_getCode","params":["0xbbbb","latest"]}`, http.StatusOK, &code)
	if code.Error != nil || code.Result != "0x60016002" {
		t.Fatalf("unexpected code response: %+v", code)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xbbbb","slot":"0x0","value":"0x01"}`)}
	var storageAt JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":35,"method":"eth_getStorageAt","params":["0xbbbb","0x0","latest"]}`, http.StatusOK, &storageAt)
	if storageAt.Error != nil || storageAt.Result != "0x01" {
		t.Fatalf("unexpected storage response: %+v", storageAt)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"0xabc","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234"}`)}
	var txByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":32,"method":"eth_getTransactionByHash","params":["0xabc"]}`, http.StatusOK, &txByHash)
	if txByHash.Error != nil {
		t.Fatalf("unexpected transaction error: %+v", txByHash)
	}
	txResult, ok := txByHash.Result.(map[string]any)
	if !ok || txResult["hash"] != "0xabc" || txResult["blockNumber"] != "0xc" {
		t.Fatalf("unexpected transaction response: %+v", txByHash.Result)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"0xabc","status":1,"gas_used":7,"logs":[{"address":"0xcontract","data":"0x01"}]}`)}
	var receipt JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":3,"method":"eth_getTransactionReceipt","params":["0xabc"]}`, http.StatusOK, &receipt)
	if receipt.Error != nil {
		t.Fatalf("unexpected receipt error: %+v", receipt)
	}
	if provider.appQueryPath[0] != "evm" || provider.appQueryPath[1] != "receipt" || provider.appQueryPath[2] != "0xabc" {
		t.Fatalf("unexpected app query path: %+v", provider.appQueryPath)
	}

	var filterID JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":8,"method":"eth_newFilter","params":[{"address":"0xcontract"}]}`, http.StatusOK, &filterID)
	filterText, ok := filterID.Result.(string)
	if filterID.Error != nil || !ok || filterText == "" {
		t.Fatalf("unexpected filter id: %+v", filterID)
	}
	var filterLogs JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":9,"method":"eth_getFilterLogs","params":["`+filterText+`"]}`, http.StatusOK, &filterLogs)
	if filterLogs.Error != nil {
		t.Fatalf("unexpected filter logs: %+v", filterLogs)
	}
	var uninstall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":10,"method":"eth_uninstallFilter","params":["`+filterText+`"]}`, http.StatusOK, &uninstall)
	if uninstall.Error != nil || uninstall.Result != true {
		t.Fatalf("unexpected uninstall response: %+v", uninstall)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"output":"0x1234","gas_used":9}`)}
	var call JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":4,"method":"eth_call","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x1234","gas":"0x5208"},"latest"]}`, http.StatusOK, &call)
	if call.Error != nil || call.Result != "0x1234" {
		t.Fatalf("unexpected eth_call response: %+v", call)
	}
	if !strings.Contains(string(provider.appQueryData), `"input":"0x1234"`) {
		t.Fatalf("unexpected call query data: %s", provider.appQueryData)
	}

	var estimate JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":5,"method":"eth_estimateGas","params":[{"to":"0xbbbb","gas":"0x100"}]}`, http.StatusOK, &estimate)
	if estimate.Error != nil || estimate.Result != "0x100" {
		t.Fatalf("unexpected estimate response: %+v", estimate)
	}
}

func TestHandlerSubmitsPlainTransaction(t *testing.T) {
	provider := &fakeStatusProvider{status: node.Status{Running: true}}
	handler := NewHandler(provider)

	var response SubmitTxResponse
	postJSON(t, handler, "/tx", `{"tx":"bank:plain","encoding":"plain"}`, http.StatusAccepted, &response)

	if !response.Accepted || len(provider.submitted) != 1 || string(provider.submitted[0]) != "bank:plain" {
		t.Fatalf("unexpected plain submit: response=%+v txs=%+v", response, provider.submitted)
	}
}

func TestHandlerRejectsUnavailableTxSubmitter(t *testing.T) {
	var response map[string]string
	postJSON(t, NewHandler(fakeStatusProvider{}), "/tx", `{"tx":"YmFuaw=="}`, http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected error response, got %+v", response)
	}
}

func TestHandlerRejectsInvalidTransactionRequests(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{status: node.Status{Running: true}})
	cases := []string{
		`{}`,
		`{"tx":"not-base64"}`,
		`{"tx":"bank:send","encoding":"unknown"}`,
		`{"tx":"YmFuaw==","extra":true}`,
		`{"tx":"YmFuaw=="} {"tx":"YmFuaw=="}`,
	}
	for _, body := range cases {
		var response map[string]string
		postJSON(t, handler, "/tx", body, http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerRejectsOversizedTransactionRequest(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{status: node.Status{Running: true}}, Config{MaxRequestBytes: 8})

	var response map[string]string
	postJSON(t, handler, "/tx", `{"tx":"YmFuaw=="}`, http.StatusBadRequest, &response)
	if response["error"] == "" {
		t.Fatalf("expected oversized error, got %+v", response)
	}
}

func TestHandlerReportsSubmitTxError(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{status: node.Status{Running: true}, submitErr: errors.New("mempool full")})

	var response map[string]string
	postJSON(t, handler, "/tx", `{"tx":"YmFuaw=="}`, http.StatusBadRequest, &response)
	if response["error"] != "mempool full" {
		t.Fatalf("unexpected submit error: %+v", response)
	}
}

func TestHandlerRejectsNonPOSTTx(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/tx", nil)
	response := httptest.NewRecorder()

	NewHandler(&fakeStatusProvider{}).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected allow POST, got %q", allow)
	}
}

func TestHandlerAcceptsEvidenceSubmission(t *testing.T) {
	provider := &fakeStatusProvider{
		evidenceApplied: true,
		evidenceResult: consensus.SlashResult{
			Receipt: slashing.PenaltyReceipt{
				Penalty: slashing.Penalty{SlashFraction: "0.05", JailDuration: 120},
			},
			PreviousPower:  100,
			RemainingPower: 95,
		},
	}
	handler := NewHandler(provider)

	var response SubmitEvidenceResponse
	postJSON(t, handler, "/evidence", `{"type":"invalid_proposal","validator":"alice","height":7,"round":2,"proof":"cHJvb2Y="}`, http.StatusAccepted, &response)

	if !response.Accepted || !response.Applied || response.Type != "invalid_proposal" || response.Validator != "alice" || response.Height != 7 || response.Round != 2 {
		t.Fatalf("unexpected evidence response: %+v", response)
	}
	if response.PreviousPower != 100 || response.RemainingPower != 95 || response.Penalty.SlashFraction != "0.05" || response.Penalty.JailDuration != 120 {
		t.Fatalf("unexpected slashing result: %+v", response)
	}
	if len(provider.evidenceSubmitted) != 1 || string(provider.evidenceSubmitted[0].Proof) != "proof" {
		t.Fatalf("unexpected submitted evidence: %+v", provider.evidenceSubmitted)
	}
}

func TestHandlerAcceptsDuplicateEvidenceAsNotApplied(t *testing.T) {
	provider := &fakeStatusProvider{evidenceApplied: false}
	handler := NewHandler(provider)

	var response SubmitEvidenceResponse
	postJSON(t, handler, "/evidence", `{"type":"double_sign","validator":"alice","height":7,"proof":"plain-proof","encoding":"plain"}`, http.StatusAccepted, &response)

	if !response.Accepted || response.Applied || response.PreviousPower != 0 || response.Penalty.SlashFraction != "" {
		t.Fatalf("unexpected duplicate evidence response: %+v", response)
	}
	if len(provider.evidenceSubmitted) != 1 || string(provider.evidenceSubmitted[0].Proof) != "plain-proof" {
		t.Fatalf("unexpected submitted duplicate evidence: %+v", provider.evidenceSubmitted)
	}
}

func TestHandlerRejectsUnavailableEvidenceSubmitter(t *testing.T) {
	var response map[string]string
	postJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/evidence", `{"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y="}`, http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable evidence error, got %+v", response)
	}
}

func TestHandlerRejectsInvalidEvidenceRequests(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{})
	cases := []string{
		`{}`,
		`{"type":"double_sign","validator":"alice","height":1,"proof":"bad-base64"}`,
		`{"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y=","encoding":"unknown"}`,
		`{"type":"double_sign","validator":"","height":1,"proof":"cHJvb2Y="}`,
		`{"type":"double_sign","validator":"alice","height":0,"proof":"cHJvb2Y="}`,
		`{"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y=","extra":true}`,
		`{"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y="} {"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y="}`,
	}
	for _, body := range cases {
		var response map[string]string
		postJSON(t, handler, "/evidence", body, http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected evidence error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerReportsSubmitEvidenceError(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{evidenceErr: errors.New("invalid evidence")})

	var response map[string]string
	postJSON(t, handler, "/evidence", `{"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y="}`, http.StatusBadRequest, &response)
	if response["error"] != "invalid evidence" {
		t.Fatalf("unexpected evidence error: %+v", response)
	}
}

func TestHandlerRejectsNonPOSTEvidence(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/evidence", nil)
	response := httptest.NewRecorder()

	NewHandler(&fakeStatusProvider{}).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected allow POST, got %q", allow)
	}
}

func TestHandlerReportsBlocksByHeightAndLatest(t *testing.T) {
	record := store.BlockRecord{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 2, TimeUnixNano: 42},
			Txs:    []types.Tx{[]byte("bank:first"), []byte("bank:second")},
		},
		Hash:    types.Hash{2},
		AppHash: types.Hash{3},
		StateRoots: []store.StateRootRecord{
			{Height: 2, Namespace: "bank", Root: types.Hash{4}},
		},
	}
	handler := NewHandler(fakeStatusProvider{
		blocks: map[types.Height]store.BlockRecord{2: record},
		latest: 2,
		index:  store.BlockIndex{EarliestHeight: 1, LatestHeight: 2, TotalBlocks: 2},
	})

	var index BlockIndexResponse
	getJSON(t, handler, "/blocks", http.StatusOK, &index)
	if index.EarliestHeight != 1 || index.LatestHeight != 2 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected block index: %+v", index)
	}

	var byHeight BlockResponse
	getJSON(t, handler, "/blocks/2", http.StatusOK, &byHeight)
	assertBlockResponse(t, byHeight)

	var latest BlockResponse
	getJSON(t, handler, "/blocks/latest", http.StatusOK, &latest)
	assertBlockResponse(t, latest)
}

func TestHandlerRejectsInvalidBlockRequests(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{blocks: map[types.Height]store.BlockRecord{}})
	cases := map[string]int{
		"/blocks/0":       http.StatusBadRequest,
		"/blocks/not-num": http.StatusBadRequest,
		"/blocks/9":       http.StatusNotFound,
		"/blocks/latest":  http.StatusNotFound,
	}
	for path, expectedStatus := range cases {
		var response map[string]string
		getJSON(t, handler, path, expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected error for %s, got %+v", path, response)
		}
	}
}

func TestHandlerRejectsUnavailableBlockProvider(t *testing.T) {
	var response map[string]string
	getJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/blocks/latest", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable block error, got %+v", response)
	}
}

func TestHandlerReportsBlockProviderErrors(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{blockErr: errors.New("store failed")})

	var response map[string]string
	getJSON(t, handler, "/blocks/latest", http.StatusInternalServerError, &response)
	if response["error"] != "store failed" {
		t.Fatalf("unexpected block error: %+v", response)
	}
}

func TestHandlerReportsLatestStateAndStateRoot(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{
		state: store.StateRecord{
			Height:           3,
			AppHash:          types.Hash{3},
			LastBlockHash:    types.Hash{4},
			ValidatorSetHash: types.Hash{5},
		},
		roots: map[string]store.StateRootRecord{
			stateRootKey(3, "bank"): {Height: 3, Namespace: "bank", Root: types.Hash{6}},
		},
	})

	var state StateResponse
	getJSON(t, handler, "/state/latest", http.StatusOK, &state)
	if state.Height != 3 || state.AppHash[:2] != "03" || state.LastBlockHash[:2] != "04" || state.ValidatorSetHash[:2] != "05" {
		t.Fatalf("unexpected state response: %+v", state)
	}

	var root StateRootResponse
	getJSON(t, handler, "/state/3/bank", http.StatusOK, &root)
	if root.Height != 3 || root.Namespace != "bank" || root.Root[:2] != "06" {
		t.Fatalf("unexpected state root response: %+v", root)
	}
}

func TestHandlerReportsFinalityProof(t *testing.T) {
	proof := finality.Proof{
		Header: types.Header{
			ChainID:          "vexo-test",
			Height:           7,
			ValidatorSetHash: types.Hash{3},
		},
		BlockHash:          types.Hash{1},
		ValidatorSetHeight: 7,
		ValidatorSetHash:   types.Hash{3},
		QuorumCert: finality.QuorumCert{
			Height:      7,
			Round:       2,
			BlockHash:   types.Hash{1},
			Signers:     []byte{0x03},
			Signature:   []byte{0x04},
			VotingPower: 20,
		},
	}
	provider := &fakeStatusProvider{
		status: node.Status{
			ChainID:               "vexo-test",
			Running:               true,
			LatestHeight:          9,
			LatestFinalizedHeight: 7,
			LatestFinalizedHash:   types.Hash{1},
		},
		finalityProof: proof,
	}
	handler := NewHandler(provider)

	var status StatusResponse
	getJSON(t, handler, "/v1/status", http.StatusOK, &status)
	if status.LatestFinalizedHeight != 7 || status.LatestFinalizedHash[:2] != "01" {
		t.Fatalf("unexpected finalized status: %+v", status)
	}

	var latest FinalityProofResponse
	getJSON(t, handler, "/v1/finality/latest", http.StatusOK, &latest)
	if latest.Height != 7 || latest.BlockHash[:2] != "01" || latest.QuorumCert.Round != 2 {
		t.Fatalf("unexpected latest finality proof: %+v", latest)
	}

	var byHeight FinalityProofResponse
	getJSON(t, handler, "/v1/finality/7", http.StatusOK, &byHeight)
	if provider.finalityHeight != 7 || byHeight.ValidatorSetHash[:2] != "03" {
		t.Fatalf("unexpected finality by height: proof=%+v requested=%d", byHeight, provider.finalityHeight)
	}
}

func TestHandlerReportsEventsAndQueryProof(t *testing.T) {
	proof := queryproof.Proof{
		SchemaVersion: queryproof.SchemaVersionV1,
		ChainID:       "vexo-test",
		Height:        9,
		Namespace:     "bank",
		Key:           []byte("alice"),
		Value:         []byte("100"),
		Exists:        true,
		StateRoot:     types.Hash{9},
		LeafHash:      types.Hash{8},
	}
	handler := NewHandler(fakeStatusProvider{
		eventRecords: []events.Record{
			{
				Height:  9,
				TxIndex: 1,
				Event: events.Event{
					Type: "transfer",
					Attributes: []events.Attribute{
						{Key: "sender", Value: "alice", Index: true},
						{Key: "recipient", Value: "bob", Index: true},
					},
				},
			},
		},
		queryProof: proof,
	})

	var eventsResponse EventsResponse
	getJSON(t, handler, "/v1/events?key=sender&value=alice", http.StatusOK, &eventsResponse)
	if eventsResponse.Key != "sender" || eventsResponse.Value != "alice" || len(eventsResponse.Records) != 1 {
		t.Fatalf("unexpected events response: %+v", eventsResponse)
	}
	if eventsResponse.Records[0].Height != 9 || eventsResponse.Records[0].Event.Type != "transfer" {
		t.Fatalf("unexpected event record: %+v", eventsResponse.Records[0])
	}

	var proofResponse QueryProofResponse
	getJSON(t, handler, "/v1/proof?namespace=bank&key=alice&height=9", http.StatusOK, &proofResponse)
	if proofResponse.Proof.ChainID != "vexo-test" || proofResponse.Proof.Height != 9 || string(proofResponse.Proof.Key) != "alice" {
		t.Fatalf("unexpected proof response: %+v", proofResponse)
	}
}

func TestHandlerReportsIBCQueries(t *testing.T) {
	provider := &fakeStatusProvider{
		ibcQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"client_id":"07-vexo-0","chain_id":"counterparty"}`)},
	}
	handler := NewHandler(provider)

	var response IBCQueryResponse
	getJSON(t, handler, "/v1/ibc/client/07-vexo-0", http.StatusOK, &response)
	if strings.Join(response.Path, "/") != "client/07-vexo-0" || string(response.Value) != `{"client_id":"07-vexo-0","chain_id":"counterparty"}` {
		t.Fatalf("unexpected IBC response: %+v", response)
	}
	if strings.Join(provider.ibcQueryPath, "/") != "client/07-vexo-0" {
		t.Fatalf("unexpected provider path: %v", provider.ibcQueryPath)
	}
}

func TestHandlerReportsIBCPacketProof(t *testing.T) {
	proof := queryproof.Proof{
		SchemaVersion: queryproof.SchemaVersionV1,
		ChainID:       "vexo-test",
		Height:        12,
		Namespace:     "ibc",
		Key:           []byte("packets/transfer/channel-0/1"),
		Value:         []byte(`{"acknowledged":true}`),
		Exists:        true,
		StateRoot:     types.Hash{9},
		LeafHash:      types.Hash{8},
	}
	handler := NewHandler(fakeStatusProvider{queryProof: proof})

	var response QueryProofResponse
	getJSON(t, handler, "/v1/ibc/proof/packet/1/transfer/channel-0/transfer/channel-1", http.StatusOK, &response)
	if response.Proof.Namespace != "ibc" || string(response.Proof.Key) != "packets/transfer/channel-0/1" {
		t.Fatalf("unexpected IBC proof response: %+v", response)
	}
}

func TestHandlerRejectsInvalidIBCQueries(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{})
	cases := map[string]int{
		"/ibc/unknown/value":               http.StatusBadRequest,
		"/ibc/client":                      http.StatusBadRequest,
		"/ibc/channel/transfer":            http.StatusBadRequest,
		"/ibc/packet/1/transfer/channel-0": http.StatusBadRequest,
		"/ibc/client/07-vexo-0/extra":      http.StatusBadRequest,
	}
	for path, expectedStatus := range cases {
		var response map[string]string
		getJSON(t, handler, path, expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected error for %s, got %+v", path, response)
		}
	}
	notFoundHandler := NewHandler(&fakeStatusProvider{ibcQueryResponse: vexoapp.QueryResponse{Code: 3, Log: "IBC state not found"}})
	var response map[string]string
	getJSON(t, notFoundHandler, "/ibc/client/missing", http.StatusNotFound, &response)
	if response["error"] != "IBC state not found" {
		t.Fatalf("unexpected not found response: %+v", response)
	}
	var proofResponse map[string]string
	getJSON(t, handler, "/ibc/proof/packet/0/transfer/channel-0/transfer/channel-1", http.StatusBadRequest, &proofResponse)
	if proofResponse["error"] == "" {
		t.Fatalf("expected proof path error, got %+v", proofResponse)
	}
}

func TestHandlerRejectsInvalidEventAndProofRequests(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{})
	cases := map[string]int{
		"/events":               http.StatusBadRequest,
		"/events?key=sender":    http.StatusBadRequest,
		"/proof":                http.StatusBadRequest,
		"/proof?namespace=bank": http.StatusBadRequest,
		"/proof?namespace=bank&key=alice&height=bad": http.StatusBadRequest,
	}
	for path, expectedStatus := range cases {
		var response map[string]string
		getJSON(t, handler, path, expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected error for %s, got %+v", path, response)
		}
	}
	proofHandler := NewHandler(fakeStatusProvider{queryProofErr: store.ErrStateNotFound})
	var response map[string]string
	getJSON(t, proofHandler, "/proof?namespace=bank&key=alice", http.StatusNotFound, &response)
	if response["error"] == "" {
		t.Fatalf("expected proof not found error, got %+v", response)
	}
}

func TestHandlerRejectsInvalidStateRequests(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{})
	cases := map[string]int{
		"/state/latest":       http.StatusNotFound,
		"/state/0/bank":       http.StatusBadRequest,
		"/state/not-num/bank": http.StatusBadRequest,
		"/state/3":            http.StatusBadRequest,
		"/state/3/bank":       http.StatusNotFound,
	}
	for path, expectedStatus := range cases {
		var response map[string]string
		getJSON(t, handler, path, expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected error for %s, got %+v", path, response)
		}
	}
}

func TestHandlerRejectsUnavailableChainQueryProvider(t *testing.T) {
	handler := NewHandler(struct{ StatusProvider }{fakeStatusProvider{}})
	for _, path := range []string{"/blocks", "/state/latest", "/state/3/bank"} {
		var response map[string]string
		getJSON(t, handler, path, http.StatusNotImplemented, &response)
		if response["error"] == "" {
			t.Fatalf("expected unavailable query error for %s, got %+v", path, response)
		}
	}
}

func TestHandlerReportsStateProviderErrors(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{stateErr: errors.New("state failed")})

	var response map[string]string
	getJSON(t, handler, "/state/latest", http.StatusInternalServerError, &response)
	if response["error"] != "state failed" {
		t.Fatalf("unexpected state error: %+v", response)
	}
}

func TestHandlerReportsLatestSnapshot(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{snapshot: node.StateSnapshot{
		Height:           5,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
		StateRoots: []store.StateRootRecord{
			{Height: 5, Namespace: "bank", Root: types.Hash{4}},
			{Height: 5, Namespace: "staking", Root: types.Hash{5}},
		},
	}})

	var snapshot StateSnapshotResponse
	getJSON(t, handler, "/snapshot/latest", http.StatusOK, &snapshot)
	if snapshot.Height != 5 || snapshot.AppHash[:2] != "01" || snapshot.LastBlockHash[:2] != "02" || snapshot.ValidatorSetHash[:2] != "03" {
		t.Fatalf("unexpected snapshot identity: %+v", snapshot)
	}
	if len(snapshot.StateRoots) != 2 || snapshot.StateRoots[0].Namespace != "bank" || snapshot.StateRoots[0].Root[:2] != "04" {
		t.Fatalf("unexpected snapshot roots: %+v", snapshot.StateRoots)
	}
}

func TestHandlerExportsRestorableSnapshotDocument(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{snapshot: node.StateSnapshot{
		Height:           5,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
		StateRoots: []store.StateRootRecord{
			{Height: 5, Namespace: "bank", Root: types.Hash{4}},
		},
		KV: []store.KVPair{{Namespace: "bank", Key: []byte("alice"), Value: []byte("100")}},
	}})

	var snapshot SnapshotExportResponse
	getJSON(t, handler, "/snapshot/export", http.StatusOK, &snapshot)
	if snapshot.SchemaVersion != "v1" || snapshot.State.Height != 5 || snapshot.State.AppHash != (types.Hash{1}) {
		t.Fatalf("unexpected snapshot export identity: %+v", snapshot)
	}
	if len(snapshot.StateRoots) != 1 || snapshot.StateRoots[0].Namespace != "bank" || snapshot.StateRoots[0].Root != (types.Hash{4}) {
		t.Fatalf("unexpected snapshot export roots: %+v", snapshot.StateRoots)
	}
	if len(snapshot.KV) != 1 || snapshot.KV[0].Namespace != "bank" || string(snapshot.KV[0].Key) != "alice" {
		t.Fatalf("unexpected snapshot export kv: %+v", snapshot.KV)
	}
}

func TestHandlerRejectsUnavailableSnapshotProvider(t *testing.T) {
	var response map[string]string
	getJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/snapshot/latest", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable snapshot error, got %+v", response)
	}
}

func TestHandlerReportsSnapshotErrors(t *testing.T) {
	cases := []struct {
		err            error
		expectedStatus int
	}{
		{err: store.ErrStateNotFound, expectedStatus: http.StatusNotFound},
		{err: store.ErrStateRootNotFound, expectedStatus: http.StatusNotFound},
		{err: errors.New("snapshot failed"), expectedStatus: http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		handler := NewHandler(fakeStatusProvider{snapshotErr: testCase.err})
		var response map[string]string
		getJSON(t, handler, "/snapshot/latest", testCase.expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected snapshot error for %v, got %+v", testCase.err, response)
		}
	}
}

func TestHandlerReportsRecoveryAndRepairsIndexes(t *testing.T) {
	provider := &fakeStatusProvider{recoveryReport: node.RecoveryReport{
		OK:                true,
		Running:           true,
		LatestHeight:      9,
		LatestStateHeight: 9,
		EarliestBlock:     3,
		LatestBlock:       9,
		TotalBlocks:       7,
		SnapshotAvailable: true,
		Repaired:          true,
		RecoverResult: store.RecoverResult{
			BlockIndexKeys:   7,
			EvidenceKeys:     2,
			EarliestHeight:   3,
			LatestHeight:     9,
			RecoveredIndexes: 2,
		},
	}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response RecoveryReportResponse
	requestJSON(t, handler, http.MethodPost, "/recovery", ``, "127.0.0.1:1", http.StatusUnauthorized, &map[string]string{})
	postJSONWithToken(t, handler, "/recovery", ``, "secret", http.StatusOK, &response)
	if provider.recoveryRepairs != 1 {
		t.Fatalf("expected one repair request, got %d", provider.recoveryRepairs)
	}
	if !response.OK || !response.SnapshotAvailable || !response.Repaired || response.TotalBlocks != 7 || response.RecoverResult == nil || response.RecoverResult.RecoveredIndexes != 2 {
		t.Fatalf("unexpected recovery response: %+v", response)
	}
}

func TestHandlerReportsDegradedRecovery(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{recoveryReport: node.RecoveryReport{
		OK:       false,
		Problems: []string{"block index not found"},
	}})

	var response RecoveryReportResponse
	getJSON(t, handler, "/recovery", http.StatusServiceUnavailable, &response)
	if response.OK || len(response.Problems) != 1 {
		t.Fatalf("expected degraded recovery response, got %+v", response)
	}
}

func TestHandlerRejectsUnavailableRecoveryProvider(t *testing.T) {
	var response map[string]string
	getJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/recovery", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable recovery error, got %+v", response)
	}
}

func TestHandlerPrunesBlocksAndStateRoots(t *testing.T) {
	provider := &fakeStatusProvider{pruneResult: store.PruneResult{
		RetainFromHeight: 3,
		PrunedBlocks:     2,
		PrunedStates:     1,
		PrunedStateRoots: 4,
	}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response PruneResponse
	postJSONWithToken(t, handler, "/prune", `{"retain_from_height":3}`, "secret", http.StatusOK, &response)

	if response.RetainFromHeight != 3 || response.PrunedBlocks != 2 || response.PrunedStates != 1 || response.PrunedStateRoots != 4 {
		t.Fatalf("unexpected prune response: %+v", response)
	}
	if len(provider.prunedHeights) != 1 || provider.prunedHeights[0] != 3 {
		t.Fatalf("unexpected prune heights: %+v", provider.prunedHeights)
	}
}

func TestHandlerRequiresAdminTokenForPrune(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	for _, testCase := range []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{name: "missing", expectedStatus: http.StatusUnauthorized},
		{name: "wrong", token: "wrong", expectedStatus: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var response map[string]string
			postJSONWithToken(t, handler, "/prune", `{"retain_from_height":3}`, testCase.token, testCase.expectedStatus, &response)
			if response["error"] == "" {
				t.Fatalf("expected admin auth error, got %+v", response)
			}
		})
	}
}

func TestHandlerRejectsMalformedAdminAuthorizationScheme(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	request := httptest.NewRequest(http.MethodPost, "/prune", strings.NewReader(`{"retain_from_height":3}`))
	request.Header.Set("Authorization", "Basic secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected malformed admin scheme to be unauthorized, got %d", response.Code)
	}
}

func TestHandlerAcceptsAdminTokenForPrune(t *testing.T) {
	provider := &fakeStatusProvider{pruneResult: store.PruneResult{RetainFromHeight: 3, PrunedBlocks: 1}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response PruneResponse
	postJSONWithToken(t, handler, "/prune", `{"retain_from_height":3}`, "secret", http.StatusOK, &response)
	if response.RetainFromHeight != 3 || len(provider.prunedHeights) != 1 {
		t.Fatalf("unexpected authorized prune: response=%+v heights=%+v", response, provider.prunedHeights)
	}
}

func TestHandlerRejectsInvalidPruneRequests(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	cases := []string{
		`{}`,
		`{"retain_from_height":0}`,
		`{"retain_from_height":1,"extra":true}`,
		`{"retain_from_height":1} {"retain_from_height":2}`,
	}
	for _, body := range cases {
		var response map[string]string
		postJSONWithToken(t, handler, "/prune", body, "secret", http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected prune error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerRejectsUnavailablePruneProvider(t *testing.T) {
	var response map[string]string
	postJSONWithToken(t, NewHandlerWithConfig(struct{ StatusProvider }{fakeStatusProvider{}}, Config{AdminToken: "secret"}), "/prune", `{"retain_from_height":2}`, "secret", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable prune error, got %+v", response)
	}
}

func TestHandlerReportsPruneErrors(t *testing.T) {
	cases := []struct {
		err            error
		expectedStatus int
	}{
		{err: store.ErrInvalidPruneHeight, expectedStatus: http.StatusBadRequest},
		{err: store.ErrBlockIndexNotFound, expectedStatus: http.StatusNotFound},
		{err: errors.New("prune failed"), expectedStatus: http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		handler := NewHandlerWithConfig(&fakeStatusProvider{pruneErr: testCase.err}, Config{AdminToken: "secret"})
		var response map[string]string
		postJSONWithToken(t, handler, "/prune", `{"retain_from_height":2}`, "secret", testCase.expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected prune error for %v, got %+v", testCase.err, response)
		}
	}
}

func TestHandlerRejectsNonPOSTPrune(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/prune", nil)
	response := httptest.NewRecorder()

	NewHandler(&fakeStatusProvider{}).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected allow POST, got %q", allow)
	}
}

func TestHandlerReplaysStoredBlocks(t *testing.T) {
	provider := &fakeStatusProvider{replayResult: vexoruntime.ReplayResult{
		FromHeight: 2,
		ToHeight:   4,
		LastHash:   types.Hash{9},
		Blocks:     3,
	}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ReplayResponse
	postJSONWithToken(t, handler, "/replay", `{"from_height":2,"to_height":4}`, "secret", http.StatusOK, &response)

	if response.FromHeight != 2 || response.ToHeight != 4 || response.Blocks != 3 || response.LastHash[:2] != "09" {
		t.Fatalf("unexpected replay response: %+v", response)
	}
	if len(provider.replayRanges) != 1 || provider.replayRanges[0] != [2]types.Height{2, 4} {
		t.Fatalf("unexpected replay ranges: %+v", provider.replayRanges)
	}
}

func TestHandlerReplaysAllStoredBlocks(t *testing.T) {
	provider := &fakeStatusProvider{replayResult: vexoruntime.ReplayResult{
		FromHeight: 1,
		ToHeight:   5,
		LastHash:   types.Hash{7},
		Blocks:     5,
	}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ReplayResponse
	postJSONWithToken(t, handler, "/replay", `{"all":true}`, "secret", http.StatusOK, &response)

	if !provider.replayAllCalled || response.FromHeight != 1 || response.ToHeight != 5 || response.Blocks != 5 {
		t.Fatalf("unexpected replay all response: called=%v response=%+v", provider.replayAllCalled, response)
	}
}

func TestHandlerRequiresAdminTokenForReplay(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	for _, testCase := range []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{name: "missing", expectedStatus: http.StatusUnauthorized},
		{name: "wrong", token: "wrong", expectedStatus: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var response map[string]string
			postJSONWithToken(t, handler, "/replay", `{"all":true}`, testCase.token, testCase.expectedStatus, &response)
			if response["error"] == "" {
				t.Fatalf("expected admin auth error, got %+v", response)
			}
		})
	}
}

func TestHandlerAcceptsAdminTokenForReplay(t *testing.T) {
	provider := &fakeStatusProvider{replayResult: vexoruntime.ReplayResult{FromHeight: 1, ToHeight: 1, LastHash: types.Hash{1}, Blocks: 1}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ReplayResponse
	postJSONWithToken(t, handler, "/replay", `{"all":true}`, "secret", http.StatusOK, &response)
	if !provider.replayAllCalled || response.Blocks != 1 {
		t.Fatalf("unexpected authorized replay: called=%v response=%+v", provider.replayAllCalled, response)
	}
}

func TestHandlerRejectsInvalidReplayRequests(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	cases := []string{
		`{"from_height":1}`,
		`{"to_height":2}`,
		`{"all":true,"from_height":1}`,
		`{"from_height":1,"to_height":2,"extra":true}`,
		`{"all":true} {"all":true}`,
	}
	for _, body := range cases {
		var response map[string]string
		postJSONWithToken(t, handler, "/replay", body, "secret", http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected replay error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerRejectsUnavailableReplayProvider(t *testing.T) {
	var response map[string]string
	postJSONWithToken(t, NewHandlerWithConfig(struct{ StatusProvider }{fakeStatusProvider{}}, Config{AdminToken: "secret"}), "/replay", `{"all":true}`, "secret", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable replay error, got %+v", response)
	}
}

func TestHandlerReportsReplayErrors(t *testing.T) {
	cases := []struct {
		err            error
		expectedStatus int
	}{
		{err: vexoruntime.ErrInvalidReplayRange, expectedStatus: http.StatusBadRequest},
		{err: store.ErrBlockNotFound, expectedStatus: http.StatusNotFound},
		{err: store.ErrBlockIndexNotFound, expectedStatus: http.StatusNotFound},
		{err: vexoruntime.ErrReplayAppHashMismatch, expectedStatus: http.StatusConflict},
		{err: errors.New("replay failed"), expectedStatus: http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		handler := NewHandlerWithConfig(&fakeStatusProvider{replayErr: testCase.err}, Config{AdminToken: "secret"})
		var response map[string]string
		postJSONWithToken(t, handler, "/replay", `{"all":true}`, "secret", testCase.expectedStatus, &response)
		if response["error"] == "" {
			t.Fatalf("expected replay error for %v, got %+v", testCase.err, response)
		}
	}
}

func TestHandlerRejectsNonPOSTReplay(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/replay", nil)
	response := httptest.NewRecorder()

	NewHandler(&fakeStatusProvider{}).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected allow POST, got %q", allow)
	}
}

func TestHandlerStartsConsensusLoop(t *testing.T) {
	provider := &fakeStatusProvider{}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ConsensusLoopResponse
	postJSONWithToken(t, handler, "/consensus/start", `{"interval_millis":25,"round_timeout_millis":250,"max_block_bytes":4096}`, "secret", http.StatusOK, &response)

	if !response.Running || response.Action != "start" || response.IntervalMillis != 25 || response.RoundTimeoutMillis != 250 || response.MaxBlockBytes != 4096 {
		t.Fatalf("unexpected consensus start response: %+v", response)
	}
	if !provider.loopRunning || len(provider.loopStartConfigs) != 1 {
		t.Fatalf("expected loop start to be recorded: running=%v configs=%+v", provider.loopRunning, provider.loopStartConfigs)
	}
	startConfig := provider.loopStartConfigs[0]
	if startConfig.Interval != 25*time.Millisecond || startConfig.RoundTimeout != 250*time.Millisecond || startConfig.MaxBlockBytes != 4096 {
		t.Fatalf("unexpected loop start config: %+v", startConfig)
	}
}

func TestHandlerStartsConsensusLoopWithDefaults(t *testing.T) {
	provider := &fakeStatusProvider{}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ConsensusLoopResponse
	postJSONWithToken(t, handler, "/consensus/start", ``, "secret", http.StatusOK, &response)

	if !response.Running || response.Action != "start" {
		t.Fatalf("unexpected default consensus start response: %+v", response)
	}
	if len(provider.loopStartConfigs) != 1 || provider.loopStartConfigs[0] != (node.ConsensusLoopConfig{}) {
		t.Fatalf("expected zero config for node defaults, got %+v", provider.loopStartConfigs)
	}
}

func TestHandlerStopsConsensusLoop(t *testing.T) {
	provider := &fakeStatusProvider{loopRunning: true}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ConsensusLoopResponse
	postJSONWithToken(t, handler, "/consensus/stop", `{}`, "secret", http.StatusOK, &response)

	if response.Running || response.Action != "stop" || provider.loopRunning {
		t.Fatalf("unexpected consensus stop response: response=%+v running=%v", response, provider.loopRunning)
	}
}

func TestHandlerRequiresAdminTokenForConsensusControl(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	for _, testCase := range []struct {
		name           string
		path           string
		token          string
		expectedStatus int
	}{
		{name: "start missing", path: "/consensus/start", expectedStatus: http.StatusUnauthorized},
		{name: "start wrong", path: "/consensus/start", token: "wrong", expectedStatus: http.StatusForbidden},
		{name: "stop missing", path: "/consensus/stop", expectedStatus: http.StatusUnauthorized},
		{name: "stop wrong", path: "/consensus/stop", token: "wrong", expectedStatus: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var response map[string]string
			postJSONWithToken(t, handler, testCase.path, `{}`, testCase.token, testCase.expectedStatus, &response)
			if response["error"] == "" {
				t.Fatalf("expected admin auth error, got %+v", response)
			}
		})
	}
}

func TestHandlerReportsConsensusControlErrors(t *testing.T) {
	cases := []struct {
		name           string
		path           string
		provider       *fakeStatusProvider
		expectedStatus int
	}{
		{name: "start node stopped", path: "/consensus/start", provider: &fakeStatusProvider{loopStartErr: node.ErrNodeNotRunning}, expectedStatus: http.StatusConflict},
		{name: "start already running", path: "/consensus/start", provider: &fakeStatusProvider{loopStartErr: node.ErrLoopAlreadyRunning}, expectedStatus: http.StatusConflict},
		{name: "stop not running", path: "/consensus/stop", provider: &fakeStatusProvider{loopStopErr: node.ErrLoopNotRunning}, expectedStatus: http.StatusConflict},
		{name: "start internal", path: "/consensus/start", provider: &fakeStatusProvider{loopStartErr: errors.New("start failed")}, expectedStatus: http.StatusInternalServerError},
		{name: "stop internal", path: "/consensus/stop", provider: &fakeStatusProvider{loopStopErr: errors.New("stop failed")}, expectedStatus: http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var response map[string]string
			postJSONWithToken(t, NewHandlerWithConfig(testCase.provider, Config{AdminToken: "secret"}), testCase.path, `{}`, "secret", testCase.expectedStatus, &response)
			if response["error"] == "" {
				t.Fatalf("expected consensus control error, got %+v", response)
			}
		})
	}
}

func TestHandlerRejectsInvalidConsensusStartRequests(t *testing.T) {
	handler := NewHandlerWithConfig(&fakeStatusProvider{}, Config{AdminToken: "secret"})
	for _, body := range []string{
		`{"interval_millis":1,"extra":true}`,
		`{"interval_millis":1} {"interval_millis":2}`,
		`{`,
	} {
		var response map[string]string
		postJSONWithToken(t, handler, "/consensus/start", body, "secret", http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected consensus start error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerRejectsUnavailableConsensusController(t *testing.T) {
	var response map[string]string
	postJSONWithToken(t, NewHandlerWithConfig(struct{ StatusProvider }{fakeStatusProvider{}}, Config{AdminToken: "secret"}), "/consensus/start", `{}`, "secret", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable consensus start error, got %+v", response)
	}
	postJSONWithToken(t, NewHandlerWithConfig(struct{ StatusProvider }{fakeStatusProvider{}}, Config{AdminToken: "secret"}), "/consensus/stop", `{}`, "secret", http.StatusNotImplemented, &response)
	if response["error"] == "" {
		t.Fatalf("expected unavailable consensus stop error, got %+v", response)
	}
}

func TestHandlerRejectsNonPOSTConsensusControl(t *testing.T) {
	for _, path := range []string{"/consensus/start", "/consensus/stop"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()

		NewHandler(&fakeStatusProvider{}).ServeHTTP(response, request)

		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for %s, got %d", path, response.Code)
		}
		if allow := response.Header().Get("Allow"); allow != http.MethodPost {
			t.Fatalf("expected allow POST for %s, got %q", path, allow)
		}
	}
}

func TestHandlerReportsValidatorsAndCommittee(t *testing.T) {
	validatorSet := newTestValidatorSet(t, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 10, Stake: 10, Metadata: map[string]string{"region": "ap"}},
		{ID: "bob", Address: "bob", VotingPower: 20, Stake: 20},
	})
	seed := types.Hash{9}
	handler := NewHandler(fakeStatusProvider{
		validators: validatorSet,
		committee: committee.Committee{
			Epoch: 2,
			Round: 3,
			Seed:  seed,
			Members: []committee.Member{
				{Validator: validator.Validator{ID: "bob", Address: "bob", VotingPower: 20, Stake: 20}, Weight: 20, Proof: []byte{1, 2}},
			},
		},
	})

	var validators ValidatorSetResponse
	getJSON(t, handler, "/validators/7", http.StatusOK, &validators)
	if validators.Height != 7 || validators.TotalValidators != 2 || validators.TotalPower != 30 || validators.ValidatorSetHash == "" {
		t.Fatalf("unexpected validator set: %+v", validators)
	}
	if validators.Validators[0].ID != "alice" || validators.Validators[0].Metadata["region"] != "ap" {
		t.Fatalf("unexpected first validator: %+v", validators.Validators[0])
	}

	var committeeResponse CommitteeResponse
	getJSON(t, handler, "/committee/7/3?seed="+hexHash(seed), http.StatusOK, &committeeResponse)
	if committeeResponse.Height != 7 || committeeResponse.Epoch != 2 || committeeResponse.Round != 3 || committeeResponse.Seed != hexHash(seed) {
		t.Fatalf("unexpected committee identity: %+v", committeeResponse)
	}
	if len(committeeResponse.Members) != 1 || committeeResponse.Members[0].Validator.ID != "bob" || committeeResponse.Members[0].Weight != 20 || committeeResponse.Members[0].Proof != "0102" {
		t.Fatalf("unexpected committee members: %+v", committeeResponse.Members)
	}
}

func TestHandlerRejectsInvalidValidatorRequests(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{validators: newTestValidatorSet(t, nil)})
	cases := []string{
		"/validators/0",
		"/validators/not-num",
		"/committee/0/1",
		"/committee/1/not-num",
		"/committee/1",
		"/committee/1/0?seed=bad",
	}
	for _, path := range cases {
		var response map[string]string
		getJSON(t, handler, path, http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected error for %s, got %+v", path, response)
		}
	}
}

func TestHandlerRejectsUnavailableValidatorQueryProvider(t *testing.T) {
	handler := NewHandler(struct{ StatusProvider }{fakeStatusProvider{}})
	for _, path := range []string{"/validators/1", "/committee/1/0"} {
		var response map[string]string
		getJSON(t, handler, path, http.StatusNotImplemented, &response)
		if response["error"] == "" {
			t.Fatalf("expected unavailable validator query error for %s, got %+v", path, response)
		}
	}
}

func TestHandlerReportsValidatorQueryErrors(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{validatorErr: errors.New("validator failed")})

	var response map[string]string
	getJSON(t, handler, "/validators/1", http.StatusInternalServerError, &response)
	if response["error"] != "validator failed" {
		t.Fatalf("unexpected validator error: %+v", response)
	}
}

func assertBlockResponse(t *testing.T, response BlockResponse) {
	t.Helper()
	if response.Height != 2 || response.ChainID != "vexo-test" || response.TxCount != 2 {
		t.Fatalf("unexpected block response: %+v", response)
	}
	if response.Hash[:2] != "02" || response.AppHash[:2] != "03" {
		t.Fatalf("unexpected block hashes: %+v", response)
	}
	if len(response.Txs) != 2 || response.Txs[0] != base64.StdEncoding.EncodeToString([]byte("bank:first")) {
		t.Fatalf("unexpected txs: %+v", response.Txs)
	}
	if len(response.StateRoots) != 1 || response.StateRoots[0].Namespace != "bank" || response.StateRoots[0].Root[:2] != "04" {
		t.Fatalf("unexpected state roots: %+v", response.StateRoots)
	}
}

func stateRootKey(height types.Height, namespace string) string {
	return strconv.FormatUint(uint64(height), 10) + "/" + namespace
}

func newTestValidatorSet(t *testing.T, validators []validator.Validator) validator.Set {
	t.Helper()
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func hexHash(hash types.Hash) string {
	return hex.EncodeToString(hash[:])
}

func TestServerStartAndShutdown(t *testing.T) {
	listener := newBlockingListener()
	server := NewServer(fakeStatusProvider{status: node.Status{Running: true}}, Config{})
	errs := make(chan error, 1)
	go func() {
		errs <- server.Start(listener)
	}()

	<-listener.ready
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errs:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server shutdown")
	}
}

type blockingListener struct {
	ready chan struct{}
	done  chan struct{}
	once  sync.Once
}

func newBlockingListener() *blockingListener {
	return &blockingListener{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
}

func (listener *blockingListener) Accept() (net.Conn, error) {
	listener.once.Do(func() {
		close(listener.ready)
	})
	<-listener.done
	return nil, net.ErrClosed
}

func (listener *blockingListener) Close() error {
	select {
	case <-listener.done:
	default:
		close(listener.done)
	}
	return nil
}

func (listener *blockingListener) Addr() net.Addr {
	return staticAddr("test-listener")
}

type staticAddr string

func (addr staticAddr) Network() string {
	return "test"
}

func (addr staticAddr) String() string {
	return string(addr)
}

func getJSON(t *testing.T, handler http.Handler, path string, expectedStatus int, value any) {
	t.Helper()
	requestJSON(t, handler, http.MethodGet, path, "", "192.0.2.1:1234", expectedStatus, value)
}

func getText(t *testing.T, handler http.Handler, path string, expectedStatus int) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = "192.0.2.1:1234"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d body=%s", expectedStatus, response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain, got %q", contentType)
	}
	return response.Body.String()
}

func assertDiagnosticCheck(t *testing.T, checks []DiagnosticCheckResponse, name string, ok bool) {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			if check.OK != ok {
				t.Fatalf("expected diagnostic check %q ok=%v, got %+v", name, ok, check)
			}
			if !ok && check.Error == "" {
				t.Fatalf("expected diagnostic check %q to include error", name)
			}
			return
		}
	}
	t.Fatalf("missing diagnostic check %q in %+v", name, checks)
}

func postJSON(t *testing.T, handler http.Handler, path string, body string, expectedStatus int, value any) {
	t.Helper()
	requestJSON(t, handler, http.MethodPost, path, body, "192.0.2.1:1234", expectedStatus, value)
}

func postJSONWithToken(t *testing.T, handler http.Handler, path string, body string, token string, expectedStatus int, value any) {
	t.Helper()
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	requestJSONWithHeaders(t, handler, http.MethodPost, path, body, "192.0.2.1:1234", headers, expectedStatus, value)
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body string, remoteAddr string, expectedStatus int, value any) {
	t.Helper()
	requestJSONWithHeaders(t, handler, method, path, body, remoteAddr, nil, expectedStatus, value)
}

func requestJSONWithHeaders(t *testing.T, handler http.Handler, method string, path string, body string, remoteAddr string, headers map[string]string, expectedStatus int, value any) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = remoteAddr
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d body=%s", expectedStatus, response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json, got %q", contentType)
	}
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		t.Fatal(err)
	}
}

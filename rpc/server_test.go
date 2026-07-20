package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gethaccounts "github.com/ethereum/go-ethereum/accounts"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/mempool"
	evmmodule "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
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
	status                node.Status
	statusDeadline        chan bool
	statusWaitCancel      chan struct{}
	metrics               node.Metrics
	metricsErr            error
	snapshot              node.StateSnapshot
	snapshotErr           error
	recoveryReport        node.RecoveryReport
	recoveryErr           error
	recoveryRepairs       int
	submitErr             error
	submitted             []types.Tx
	blocks                map[types.Height]store.BlockRecord
	blocksByHash          map[types.Hash]store.BlockRecord
	latest                types.Height
	blockErr              error
	index                 store.BlockIndex
	state                 store.StateRecord
	states                map[types.Height]store.StateRecord
	roots                 map[string]store.StateRootRecord
	stateErr              error
	eventRecords          []events.Record
	eventErr              error
	queryProof            queryproof.Proof
	queryProofErr         error
	ibcQueryResponse      vexoapp.QueryResponse
	ibcQueryErr           error
	ibcQueryPath          []string
	appQueryResponse      vexoapp.QueryResponse
	appQueryResponses     map[string]vexoapp.QueryResponse
	appQueryErr           error
	appQueryPath          []string
	appQueryData          []byte
	pruneResult           store.PruneResult
	pruneErr              error
	prunedHeights         []types.Height
	replayResult          vexoruntime.ReplayResult
	replayErr             error
	replayAllCalled       bool
	replayRanges          [][2]types.Height
	strictReplayAllCalled bool
	strictReplayRanges    [][2]types.Height
	loopStartErr          error
	loopStopErr           error
	loopRunning           bool
	loopStartConfigs      []node.ConsensusLoopConfig
	validators            validator.Set
	committee             committee.Committee
	validatorErr          error
	evidenceResult        consensus.SlashResult
	evidenceApplied       bool
	evidenceErr           error
	evidenceSubmitted     []slashing.Evidence
	accountSequence       uint64
	accountErr            error
	accountAddress        types.Address
	finalityProof         finality.Proof
	finalityErr           error
	finalityHeight        types.Height
	pendingHashes         []types.Hash
	pendingTxs            []types.Tx
	pendingErr            error
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

func (provider *fakeStatusProvider) PendingTxHashes(ctx context.Context) ([]types.Hash, error) {
	if provider.pendingErr != nil {
		return nil, provider.pendingErr
	}
	if len(provider.pendingHashes) == 0 && len(provider.pendingTxs) > 0 {
		hashes := make([]types.Hash, 0, len(provider.pendingTxs))
		for _, tx := range provider.pendingTxs {
			hashes = append(hashes, mempool.HashTx(tx))
		}
		return hashes, nil
	}
	return append([]types.Hash(nil), provider.pendingHashes...), nil
}

func (provider *fakeStatusProvider) PendingTxs(ctx context.Context) ([]types.Tx, error) {
	if provider.pendingErr != nil {
		return nil, provider.pendingErr
	}
	txs := make([]types.Tx, 0, len(provider.pendingTxs))
	for _, tx := range provider.pendingTxs {
		txs = append(txs, append(types.Tx(nil), tx...))
	}
	return txs, nil
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

func (provider fakeStatusProvider) StateByHeight(ctx context.Context, height types.Height) (store.StateRecord, error) {
	if provider.stateErr != nil {
		return store.StateRecord{}, provider.stateErr
	}
	if provider.states != nil {
		if record, ok := provider.states[height]; ok {
			return record, nil
		}
	}
	if provider.state.Height == height && height != 0 {
		return provider.state, nil
	}
	return store.StateRecord{}, store.ErrStateNotFound
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
	if provider.appQueryResponses != nil {
		if response, ok := provider.appQueryResponses[strings.Join(path, "/")]; ok {
			return response, nil
		}
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

func (provider *fakeStatusProvider) ReplayStrict(ctx context.Context, from types.Height, to types.Height) (vexoruntime.ReplayResult, error) {
	if provider.replayErr != nil {
		return vexoruntime.ReplayResult{}, provider.replayErr
	}
	provider.strictReplayRanges = append(provider.strictReplayRanges, [2]types.Height{from, to})
	return provider.replayResult, nil
}

func (provider *fakeStatusProvider) ReplayAllStrict(ctx context.Context) (vexoruntime.ReplayResult, error) {
	if provider.replayErr != nil {
		return vexoruntime.ReplayResult{}, provider.replayErr
	}
	provider.strictReplayAllCalled = true
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
		ChainID:             "vexo-test",
		Running:             true,
		LatestHeight:        7,
		LatestAppHash:       appHash,
		DataDir:             "/tmp/vexo",
		PeerCount:           2,
		ActivePeerCount:     1,
		ConfiguredPeerCount: 3,
		ScoredPeerCount:     2,
		BannedPeers:         1,
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
	if status.LatestAppHash[:6] != "010203" || status.PeerCount != 2 || status.ActivePeerCount != 1 || status.ConfiguredPeerCount != 3 || status.ScoredPeerCount != 2 || status.BannedPeers != 1 {
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
		ChainID:                     "vexo-test",
		Running:                     true,
		StartedAtUnix:               1710000000,
		UptimeSeconds:               42,
		DataDir:                     "/tmp/vexo",
		LatestHeight:                9,
		LatestAppHash:               types.Hash{1, 2, 3},
		EarliestBlockHeight:         1,
		LatestBlockHeight:           9,
		TotalBlocks:                 9,
		ValidatorCount:              4,
		TotalVotingPower:            100,
		ValidatorSetHash:            types.Hash{4, 5, 6},
		PeerCount:                   3,
		ActivePeerCount:             2,
		ConfiguredPeerCount:         4,
		ScoredPeerCount:             3,
		BannedPeers:                 1,
		QuorumHealthRatio:           0.5,
		PeerWindowMessages:          12,
		ConsensusLoopRunning:        true,
		AdaptiveRoundTimeoutEnabled: true,
		RecoveryFinalityGateEnabled: true,
		AdaptiveRoundTimeoutNanos:   250000000,
		RecoveryFinalityDeferrals:   3,
		ReconciliationFailures:      2,
	}})

	var metrics MetricsResponse
	getJSON(t, handler, "/metrics", http.StatusOK, &metrics)
	if metrics.ChainID != "vexo-test" || !metrics.Running || metrics.StartedAtUnix != 1710000000 || metrics.UptimeSeconds != 42 || metrics.LatestHeight != 9 || metrics.TotalBlocks != 9 {
		t.Fatalf("unexpected metrics identity: %+v", metrics)
	}
	if metrics.ValidatorCount != 4 || metrics.TotalVotingPower != 100 || metrics.PeerCount != 3 || metrics.ActivePeerCount != 2 || metrics.ConfiguredPeerCount != 4 || metrics.ScoredPeerCount != 3 || metrics.BannedPeers != 1 || metrics.QuorumHealthRatio != 0.5 || !metrics.ConsensusLoopRunning || metrics.ReconciliationFailures != 2 || metrics.AdaptiveRoundTimeoutNanos != 250000000 || metrics.RecoveryFinalityDeferrals != 3 {
		t.Fatalf("unexpected metrics counters: %+v", metrics)
	}
	if !metrics.AdaptiveRoundTimeoutEnabled || !metrics.RecoveryFinalityGateEnabled {
		t.Fatalf("expected policy toggles in metrics: %+v", metrics)
	}
	if metrics.LatestAppHash[:6] != "010203" || metrics.ValidatorSetHash[:6] != "040506" {
		t.Fatalf("unexpected metrics hashes: %+v", metrics)
	}
}

func TestHandlerReportsMetricsText(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{metrics: node.Metrics{
		Running:                     true,
		StartedAtUnix:               1710000000,
		UptimeSeconds:               42,
		LatestHeight:                9,
		EarliestBlockHeight:         1,
		LatestBlockHeight:           9,
		TotalBlocks:                 9,
		ValidatorCount:              4,
		TotalVotingPower:            100,
		PeerCount:                   3,
		ActivePeerCount:             2,
		ConfiguredPeerCount:         4,
		ScoredPeerCount:             3,
		BannedPeers:                 1,
		QuorumHealthRatio:           0.5,
		PeerWindowMessages:          12,
		ConsensusLoopRunning:        true,
		AdaptiveRoundTimeoutEnabled: true,
		RecoveryFinalityGateEnabled: true,
		AdaptiveRoundTimeoutNanos:   250000000,
		RecoveryFinalityDeferrals:   3,
		ReconciliationFailures:      2,
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
		"vexo_active_peer_count 2",
		"vexo_configured_peer_count 4",
		"vexo_scored_peer_count 3",
		"vexo_banned_peers 1",
		"vexo_quorum_health_ratio 0.500000",
		"vexo_peer_window_messages 12",
		"vexo_consensus_loop_running 1",
		"vexo_adaptive_round_timeout_enabled 1",
		"vexo_recovery_finality_gate_enabled 1",
		"vexo_adaptive_round_timeout_nanos 250000000",
		"vexo_recovery_finality_deferrals 3",
		"vexo_post_commit_reconciliation_failures 2",
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

func TestHandlerReportsProviderCapabilities(t *testing.T) {
	handler := NewHandlerWithConfig(fakeStatusProvider{
		status:  node.Status{ChainID: "vexo-test", Running: true},
		metrics: node.Metrics{ChainID: "vexo-test", Running: true},
		index:   store.BlockIndex{EarliestHeight: 1, LatestHeight: 1, TotalBlocks: 1},
	}, Config{RequiredCapabilities: []string{"metrics", "blocks", "finality"}})

	var capabilities CapabilityResponse
	getJSON(t, handler, "/capabilities", http.StatusOK, &capabilities)
	if capabilities.Complete || len(capabilities.Missing) != 1 || capabilities.Missing[0] != "finality" {
		t.Fatalf("expected missing finality capability, got %+v", capabilities)
	}
	foundMetrics := false
	for _, capability := range capabilities.Capabilities {
		if capability.Name == "metrics" {
			foundMetrics = capability.Available && capability.Required
		}
	}
	if !foundMetrics {
		t.Fatalf("expected required metrics capability to be available: %+v", capabilities)
	}
}

func TestServerStartupFailsWhenRequiredCapabilitiesAreMissing(t *testing.T) {
	server := NewServer(fakeStatusProvider{status: node.Status{Running: true}}, Config{RequiredCapabilities: []string{"finality"}})
	if !errors.Is(server.StartupError(), ErrMissingRequiredCapability) {
		t.Fatalf("expected missing capability startup error, got %v", server.StartupError())
	}
}

func TestNetworkSafeServerRequiresAllCapabilities(t *testing.T) {
	_, err := NewNetworkSafeServer(struct{ StatusProvider }{fakeStatusProvider{status: node.Status{Running: true}}}, Config{})
	if !errors.Is(err, ErrMissingRequiredCapability) {
		t.Fatalf("expected missing capability error, got %v", err)
	}
}

func TestNetworkSafeServerAcceptsFullProvider(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	validatorSet, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewNetworkSafeServer(&fakeStatusProvider{
		status:     node.Status{Running: true},
		metrics:    node.Metrics{Running: true},
		index:      store.BlockIndex{EarliestHeight: 1, LatestHeight: 1, TotalBlocks: 1},
		validators: validatorSet,
		committee: committee.Committee{Members: []committee.Member{
			{Validator: validator.Validator{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1}, Weight: 1},
		}},
	}, Config{})
	if err != nil {
		t.Fatalf("expected full provider to pass network-safe startup: %v", err)
	}
	if server.StartupError() != nil {
		t.Fatalf("unexpected startup error: %v", server.StartupError())
	}
}

func TestNetworkSafeHandlerRequiresAllCapabilities(t *testing.T) {
	_, err := NewNetworkSafeHandlerWithConfig(struct{ StatusProvider }{fakeStatusProvider{status: node.Status{Running: true}}}, Config{})
	if !errors.Is(err, ErrMissingRequiredCapability) {
		t.Fatalf("expected missing capability error, got %v", err)
	}
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
	blockTxHashText := "0x8888888888888888888888888888888888888888888888888888888888888888"
	blockTx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "evm",
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "1234", "7", "0"},
		Tags: map[string]string{
			"fee":             "14",
			"gas":             "7",
			ethcompat.TagRaw:  "0xabcdef",
			ethcompat.TagHash: blockTxHashText,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	block := store.BlockRecord{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-chain", Height: 12, PreviousBlockHash: parentHash, TimeUnixNano: int64(1700000000 * time.Second)},
			Txs:    []types.Tx{blockTx},
		},
		Hash:    blockHash,
		AppHash: types.Hash{0xef},
		TxResults: []types.Result{
			{GasUsed: 7, Data: []byte(`{"tx_hash":"` + blockTxHashText + `","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234","state_diff":{"0xbbbb":{"storage":{"0x0":{"to":"0x1"}}}}}`)},
		},
	}
	finalizedHash := types.Hash{0xac}
	finalizedBlock := store.BlockRecord{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-chain", Height: 11, TimeUnixNano: int64(1699999999 * time.Second)},
		},
		Hash:    finalizedHash,
		AppHash: types.Hash{0xee},
	}
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", Running: true, LatestHeight: 12, LatestFinalizedHeight: 11, PeerCount: 2},
		state:  store.StateRecord{Height: 12, BaseFee: 9, NextBaseFee: 11},
		states: map[types.Height]store.StateRecord{
			11: {Height: 11, BaseFee: 11, NextBaseFee: 9},
			12: {Height: 12, BaseFee: 9, NextBaseFee: 11},
		},
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"0xabc","status":1,"gas_used":7,"logs":[{"address":"0xcontract","data":"0x01"}]}`)},
		blocks:           map[types.Height]store.BlockRecord{11: finalizedBlock, 12: block},
		blocksByHash:     map[types.Hash]store.BlockRecord{blockHash: block, finalizedHash: finalizedBlock},
		latest:           12,
		index:            store.BlockIndex{EarliestHeight: 11, LatestHeight: 12, TotalBlocks: 2},
		accountSequence:  7,
		pendingHashes:    []types.Hash{{0xfa}},
		finalityProof:    finality.Proof{Header: types.Header{Height: 11}, BlockHash: finalizedHash},
		loopRunning:      true,
	}
	pendingTx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "evm",
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "abcd", "21000", "5"},
		Tags: map[string]string{
			"signer":             "0xaaaa",
			"nonce":              "7",
			"gas":                "21000",
			"fee":                "42000",
			ethcompat.TagHash:    "0x9999999999999999999999999999999999999999999999999999999999999999",
			ethcompat.TagRaw:     "0xbeef",
			ethcompat.TagChainID: "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	queuedTx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "evm",
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xcccc", "call", "dcba", "21000", "6"},
		Tags: map[string]string{
			"signer":             "0xaaaa",
			"nonce":              "9",
			"gas":                "21000",
			"fee":                "42000",
			ethcompat.TagHash:    "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ethcompat.TagRaw:     "0xcafe",
			ethcompat.TagChainID: "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.pendingTxs = []types.Tx{pendingTx, queuedTx}
	handler := NewHandler(provider)

	var blockNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`, http.StatusOK, &blockNumber)
	if blockNumber.Error != nil || blockNumber.Result != "0xc" {
		t.Fatalf("unexpected block number response: %+v", blockNumber)
	}

	for _, testCase := range []struct {
		method   string
		params   string
		expected any
	}{
		{method: "web3_sha3", params: `["0x68656c6c6f"]`, expected: "0x1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"},
		{method: "net_listening", params: `[]`, expected: true},
		{method: "net_peerCount", params: `[]`, expected: "0x2"},
		{method: "eth_protocolVersion", params: `[]`, expected: "0x1"},
		{method: "eth_mining", params: `[]`, expected: true},
		{method: "eth_hashrate", params: `[]`, expected: "0x0"},
		{method: "eth_coinbase", params: `[]`, expected: "0x0000000000000000000000000000000000000000"},
	} {
		var response JSONRPCResponse
		postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":66,"method":"`+testCase.method+`","params":`+testCase.params+`}`, http.StatusOK, &response)
		if response.Error != nil || response.Result != testCase.expected {
			t.Fatalf("unexpected %s response: %+v", testCase.method, response)
		}
	}
	var syncing JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":66,"method":"eth_syncing","params":[]}`, http.StatusOK, &syncing)
	syncingObject, ok := syncing.Result.(map[string]any)
	if syncing.Error != nil || !ok || syncingObject["currentBlock"] != "0xb" || syncingObject["highestBlock"] != "0xc" {
		t.Fatalf("unexpected eth_syncing response: %+v", syncing)
	}
	var accounts JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":67,"method":"eth_accounts","params":[]}`, http.StatusOK, &accounts)
	accountList, ok := accounts.Result.([]any)
	if accounts.Error != nil || !ok || len(accountList) != 0 {
		t.Fatalf("unexpected accounts response: %+v", accounts)
	}
	var modules JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":95,"method":"rpc_modules","params":[]}`, http.StatusOK, &modules)
	moduleResult, ok := modules.Result.(map[string]any)
	if modules.Error != nil || !ok || moduleResult["eth"] != "1.0" || moduleResult["txpool"] != "1.0" || moduleResult["trace"] != "1.0" {
		t.Fatalf("unexpected rpc modules response: %+v", modules)
	}
	var capabilities JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":96,"method":"vexo_web3Capabilities","params":[]}`, http.StatusOK, &capabilities)
	capabilityResult, ok := capabilities.Result.(map[string]any)
	if capabilities.Error != nil || !ok || capabilityResult["native_vexo_network"] != true || capabilityResult["ethereum_p2p"] != false || capabilityResult["trace_reexecution"] == "" {
		t.Fatalf("unexpected web3 capabilities response: %+v", capabilities)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xaaaa","balance":123,"nonce":7,"code":""}`)}
	var txpoolStatus JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":75,"method":"txpool_status","params":[]}`, http.StatusOK, &txpoolStatus)
	statusResult, ok := txpoolStatus.Result.(map[string]any)
	if txpoolStatus.Error != nil || !ok || statusResult["pending"] != "0x1" || statusResult["queued"] != "0x1" {
		t.Fatalf("unexpected txpool status: %+v", txpoolStatus)
	}
	var txpoolContent JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":76,"method":"txpool_content","params":[]}`, http.StatusOK, &txpoolContent)
	contentResult, ok := txpoolContent.Result.(map[string]any)
	pendingContent, _ := contentResult["pending"].(map[string]any)
	fromContent, _ := pendingContent["0xaaaa"].(map[string]any)
	pendingItem, _ := fromContent["0x7"].(map[string]any)
	queuedContent, _ := contentResult["queued"].(map[string]any)
	queuedFromContent, _ := queuedContent["0xaaaa"].(map[string]any)
	queuedItem, _ := queuedFromContent["0x9"].(map[string]any)
	if txpoolContent.Error != nil || !ok || pendingItem["hash"] != "0x9999999999999999999999999999999999999999999999999999999999999999" || pendingItem["to"] != "0xbbbb" {
		t.Fatalf("unexpected txpool content: %+v", txpoolContent)
	}
	if queuedItem["hash"] != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || queuedItem["to"] != "0xcccc" {
		t.Fatalf("unexpected queued txpool content: %+v", txpoolContent)
	}
	var txpoolContentFrom JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":88,"method":"txpool_contentFrom","params":["0xaaaa"]}`, http.StatusOK, &txpoolContentFrom)
	contentFromResult, ok := txpoolContentFrom.Result.(map[string]any)
	contentFromPending, _ := contentFromResult["pending"].(map[string]any)
	contentFromQueued, _ := contentFromResult["queued"].(map[string]any)
	if txpoolContentFrom.Error != nil || !ok || len(contentFromPending) != 1 || len(contentFromQueued) != 1 {
		t.Fatalf("unexpected txpool contentFrom: %+v", txpoolContentFrom)
	}
	var pendingTransactions JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":96,"method":"eth_pendingTransactions","params":[]}`, http.StatusOK, &pendingTransactions)
	pendingItems, ok := pendingTransactions.Result.([]any)
	if pendingTransactions.Error != nil || !ok || len(pendingItems) != 2 {
		t.Fatalf("unexpected pending transactions: %+v", pendingTransactions)
	}
	pendingObject, ok := pendingItems[0].(map[string]any)
	if !ok || pendingObject["hash"] != "0x9999999999999999999999999999999999999999999999999999999999999999" || pendingObject["blockNumber"] != nil {
		t.Fatalf("unexpected pending transaction object: %+v", pendingItems[0])
	}
	var txpoolInspect JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":77,"method":"txpool_inspect","params":[]}`, http.StatusOK, &txpoolInspect)
	inspectResult, ok := txpoolInspect.Result.(map[string]any)
	if txpoolInspect.Error != nil || !ok || inspectResult["pending"] == nil {
		t.Fatalf("unexpected txpool inspect: %+v", txpoolInspect)
	}

	var sendRaw JSONRPCResponse
	rawEthTx, rawEthHash := signedTestEthereumTx(t, chainNumericID("vexo-chain"))
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":2,"method":"eth_sendRawTransaction","params":["`+rawEthTx+`"]}`, http.StatusOK, &sendRaw)
	if sendRaw.Error != nil || len(provider.submitted) != 1 || !strings.HasPrefix(string(provider.submitted[0]), "evm:call:evm:") || !strings.Contains(string(provider.submitted[0]), "eth_hash="+rawEthHash) {
		t.Fatalf("unexpected sendRaw response=%+v submitted=%q", sendRaw, provider.submitted)
	}
	if result, ok := sendRaw.Result.(string); !ok || result != rawEthHash {
		t.Fatalf("expected tx hash result, got %+v", sendRaw.Result)
	}
	rawUnprotectedTx, _ := unprotectedLegacyEthereumTx(t)
	var sendUnprotected JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":104,"method":"eth_sendRawTransaction","params":["`+rawUnprotectedTx+`"]}`, http.StatusOK, &sendUnprotected)
	if sendUnprotected.Error == nil || len(provider.submitted) != 1 {
		t.Fatalf("expected unprotected legacy tx rejection, response=%+v submitted=%d", sendUnprotected, len(provider.submitted))
	}
	legacyProvider := &fakeStatusProvider{status: node.Status{ChainID: "vexo-chain", Running: true}}
	legacyHandler := NewHandlerWithConfig(legacyProvider, Config{AllowUnprotectedLegacyTx: true})
	var sendAllowedUnprotected JSONRPCResponse
	postJSON(t, legacyHandler, "/web3", `{"jsonrpc":"2.0","id":105,"method":"eth_sendRawTransaction","params":["`+rawUnprotectedTx+`"]}`, http.StatusOK, &sendAllowedUnprotected)
	if sendAllowedUnprotected.Error != nil || len(legacyProvider.submitted) != 1 {
		t.Fatalf("expected explicitly allowed unprotected legacy tx, response=%+v submitted=%d", sendAllowedUnprotected, len(legacyProvider.submitted))
	}
	rawBlobTx, rawBlobHash, blobSidecar := signedTestEthereumBlobTx(t, chainNumericID("vexo-chain"))
	blobSidecarJSON, err := json.Marshal(blobSidecar)
	if err != nil {
		t.Fatal(err)
	}
	var sendBlobWithoutSidecar JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":101,"method":"eth_sendRawTransaction","params":["`+rawBlobTx+`"]}`, http.StatusOK, &sendBlobWithoutSidecar)
	if sendBlobWithoutSidecar.Error == nil || len(provider.submitted) != 1 {
		t.Fatalf("expected eth_sendRawTransaction to reject blob tx without sidecar, response=%+v submitted=%q", sendBlobWithoutSidecar, provider.submitted)
	}
	var sendBlob JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":102,"method":"vexo_sendRawBlobTransaction","params":["`+rawBlobTx+`",`+string(blobSidecarJSON)+`]}`, http.StatusOK, &sendBlob)
	if sendBlob.Error != nil || sendBlob.Result != rawBlobHash || len(provider.submitted) != 2 || !strings.Contains(string(provider.submitted[1]), ethcompat.TagBlobSidecar+"=") {
		t.Fatalf("unexpected blob tx response=%+v submitted=%q", sendBlob, provider.submitted)
	}
	blobDetails := web3TransactionDetails(provider.submitted[1])
	if blobDetails.BlobGasFeeCap != 9 || len(blobDetails.BlobHashes) != 1 || !strings.EqualFold(blobDetails.BlobHashes[0], blobSidecar.BlobHashes[0]) {
		t.Fatalf("expected Web3 blob transaction details, got %+v sidecar=%+v", blobDetails, blobSidecar)
	}
	var sendBlobAlias JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":106,"method":"eth_sendRawBlobTransaction","params":["`+rawBlobTx+`",`+string(blobSidecarJSON)+`]}`, http.StatusOK, &sendBlobAlias)
	if sendBlobAlias.Error != nil || sendBlobAlias.Result != rawBlobHash || len(provider.submitted) != 3 {
		t.Fatalf("unexpected blob tx alias response=%+v submitted=%q", sendBlobAlias, provider.submitted)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"` + rawBlobHash + `","sidecar":{"blob_hashes":["` + blobSidecar.BlobHashes[0] + `"],"blobs":[],"commitments":[],"proofs":[]}}`)}
	var getBlob JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":103,"method":"vexo_getBlobSidecarByTxHash","params":["`+rawBlobHash+`"]}`, http.StatusOK, &getBlob)
	if getBlob.Error != nil {
		t.Fatalf("unexpected blob sidecar query response: %+v", getBlob)
	}

	var gasPrice JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":30,"method":"eth_gasPrice","params":[]}`, http.StatusOK, &gasPrice)
	if gasPrice.Error != nil || gasPrice.Result != "0xb" {
		t.Fatalf("unexpected gas price response: %+v", gasPrice)
	}
	var blobBaseFee JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":100,"method":"eth_blobBaseFee","params":[]}`, http.StatusOK, &blobBaseFee)
	if blobBaseFee.Error != nil || blobBaseFee.Result != "0x0" {
		t.Fatalf("unexpected blob base fee response: %+v", blobBaseFee)
	}

	var priorityFee JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":44,"method":"eth_maxPriorityFeePerGas","params":[]}`, http.StatusOK, &priorityFee)
	if priorityFee.Error != nil || priorityFee.Result != "0x1" {
		t.Fatalf("unexpected priority fee response: %+v", priorityFee)
	}

	var feeHistory JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":45,"method":"eth_feeHistory","params":["0x2","latest",[10,50]]}`, http.StatusOK, &feeHistory)
	feeHistoryResult, ok := feeHistory.Result.(map[string]any)
	if feeHistory.Error != nil || !ok || feeHistoryResult["oldestBlock"] != "0xb" {
		t.Fatalf("unexpected fee history response: %+v", feeHistory)
	}
	baseFees, ok := feeHistoryResult["baseFeePerGas"].([]any)
	if !ok || len(baseFees) != 3 || baseFees[0] != "0xb" || baseFees[1] != "0x9" || baseFees[2] != "0xb" {
		t.Fatalf("unexpected fee history base fees: %+v", feeHistoryResult)
	}
	rewards, ok := feeHistoryResult["reward"].([]any)
	if !ok || len(rewards) != 2 {
		t.Fatalf("unexpected fee history rewards: %+v", feeHistoryResult)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"state_root":"0x1111111111111111111111111111111111111111111111111111111111111111"}`)}
	var blockByNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":6,"method":"eth_getBlockByNumber","params":["latest",true]}`, http.StatusOK, &blockByNumber)
	if blockByNumber.Error != nil {
		t.Fatalf("unexpected block by number error: %+v", blockByNumber)
	}
	blockResult, ok := blockByNumber.Result.(map[string]any)
	if !ok || blockResult["number"] != "0xc" || blockResult["hash"] != "0xab00000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("unexpected block by number response: %+v", blockByNumber.Result)
	}
	if blockResult["stateRoot"] != "0x1111111111111111111111111111111111111111111111111111111111111111" {
		t.Fatalf("expected Ethereum state root, got %+v", blockResult["stateRoot"])
	}
	if blockResult["transactionsRoot"] == "0x0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("expected non-zero transactions root for block with txs: %+v", blockResult)
	}
	if blockResult["receiptsRoot"] == "0x0000000000000000000000000000000000000000000000000000000000000000" || blockResult["gasUsed"] != "0x7" {
		t.Fatalf("expected execution-backed receipts root and gas used: %+v", blockResult)
	}
	if blockResult["baseFeePerGas"] != "0x9" || blockResult["blobGasUsed"] != "0x0" || blockResult["withdrawals"] == nil || blockResult["gasLimit"] == "0x0" {
		t.Fatalf("expected post-merge/cancun compatible block fields: %+v", blockResult)
	}
	fullTxs, ok := blockResult["transactions"].([]any)
	if !ok || len(fullTxs) != 1 {
		t.Fatalf("expected full transaction response: %+v", blockResult["transactions"])
	}
	fullTx, ok := fullTxs[0].(map[string]any)
	if !ok || fullTx["from"] != "0xaaaa" || fullTx["to"] != "0xbbbb" || fullTx["gas"] != "0x7" || fullTx["gasPrice"] != "0x2" {
		t.Fatalf("expected receipt-backed transaction fields: %+v", fullTxs[0])
	}

	var txCountByNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":68,"method":"eth_getBlockTransactionCountByNumber","params":["latest"]}`, http.StatusOK, &txCountByNumber)
	if txCountByNumber.Error != nil || txCountByNumber.Result != "0x1" {
		t.Fatalf("unexpected tx count by number: %+v", txCountByNumber)
	}
	var finalizedBlockByNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":101,"method":"eth_getBlockByNumber","params":["finalized",false]}`, http.StatusOK, &finalizedBlockByNumber)
	finalizedBlockResult, ok := finalizedBlockByNumber.Result.(map[string]any)
	if finalizedBlockByNumber.Error != nil || !ok || finalizedBlockResult["number"] != "0xb" {
		t.Fatalf("unexpected finalized block response: %+v", finalizedBlockByNumber)
	}
	var safeTxCountByNumber JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":102,"method":"eth_getBlockTransactionCountByNumber","params":["safe"]}`, http.StatusOK, &safeTxCountByNumber)
	if safeTxCountByNumber.Error != nil || safeTxCountByNumber.Result != "0x0" {
		t.Fatalf("unexpected safe tx count by number: %+v", safeTxCountByNumber)
	}
	var txByNumberAndIndex JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":69,"method":"eth_getTransactionByBlockNumberAndIndex","params":["latest","0x0"]}`, http.StatusOK, &txByNumberAndIndex)
	txByNumberResult, ok := txByNumberAndIndex.Result.(map[string]any)
	if txByNumberAndIndex.Error != nil || !ok || txByNumberResult["hash"] != blockTxHashText || txByNumberResult["transactionIndex"] != "0x0" {
		t.Fatalf("unexpected tx by number/index: %+v", txByNumberAndIndex)
	}
	var rawByNumberAndIndex JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":89,"method":"eth_getRawTransactionByBlockNumberAndIndex","params":["latest","0x0"]}`, http.StatusOK, &rawByNumberAndIndex)
	if rawByNumberAndIndex.Error != nil || rawByNumberAndIndex.Result != "0xabcdef" {
		t.Fatalf("unexpected raw tx by number/index: %+v", rawByNumberAndIndex)
	}
	var missingTxByNumberAndIndex JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":70,"method":"eth_getTransactionByBlockNumberAndIndex","params":["latest","0x9"]}`, http.StatusOK, &missingTxByNumberAndIndex)
	if missingTxByNumberAndIndex.Error != nil || missingTxByNumberAndIndex.Result != nil {
		t.Fatalf("unexpected missing tx by number/index: %+v", missingTxByNumberAndIndex)
	}

	var blockByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":7,"method":"eth_getBlockByHash","params":["0xab00000000000000000000000000000000000000000000000000000000000000",false]}`, http.StatusOK, &blockByHash)
	if blockByHash.Error != nil {
		t.Fatalf("unexpected block by hash error: %+v", blockByHash)
	}

	var txCountByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":71,"method":"eth_getBlockTransactionCountByHash","params":["0xab00000000000000000000000000000000000000000000000000000000000000"]}`, http.StatusOK, &txCountByHash)
	if txCountByHash.Error != nil || txCountByHash.Result != "0x1" {
		t.Fatalf("unexpected tx count by hash: %+v", txCountByHash)
	}
	var txByHashAndIndex JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":72,"method":"eth_getTransactionByBlockHashAndIndex","params":["0xab00000000000000000000000000000000000000000000000000000000000000","0x0"]}`, http.StatusOK, &txByHashAndIndex)
	txByHashResult, ok := txByHashAndIndex.Result.(map[string]any)
	if txByHashAndIndex.Error != nil || !ok || txByHashResult["hash"] != blockTxHashText || txByHashResult["blockHash"] != "0xab00000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("unexpected tx by hash/index: %+v", txByHashAndIndex)
	}
	var rawByHashAndIndex JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":90,"method":"eth_getRawTransactionByBlockHashAndIndex","params":["0xab00000000000000000000000000000000000000000000000000000000000000","0x0"]}`, http.StatusOK, &rawByHashAndIndex)
	if rawByHashAndIndex.Error != nil || rawByHashAndIndex.Result != "0xabcdef" {
		t.Fatalf("unexpected raw tx by hash/index: %+v", rawByHashAndIndex)
	}
	var rawByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":91,"method":"eth_getRawTransactionByHash","params":["`+blockTxHashText+`"]}`, http.StatusOK, &rawByHash)
	if rawByHash.Error != nil || rawByHash.Result != "0xabcdef" {
		t.Fatalf("unexpected raw tx by hash: %+v", rawByHash)
	}
	var pendingRawByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":98,"method":"eth_getRawTransactionByHash","params":["0x9999999999999999999999999999999999999999999999999999999999999999"]}`, http.StatusOK, &pendingRawByHash)
	if pendingRawByHash.Error != nil || pendingRawByHash.Result != "0xbeef" {
		t.Fatalf("unexpected pending raw tx by hash: %+v", pendingRawByHash)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Code: 3}
	var scannedReceipt JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":104,"method":"eth_getTransactionReceipt","params":["`+blockTxHashText+`"]}`, http.StatusOK, &scannedReceipt)
	scannedReceiptResult, ok := scannedReceipt.Result.(map[string]any)
	if scannedReceipt.Error != nil || !ok || scannedReceiptResult["transactionHash"] != blockTxHashText || scannedReceiptResult["blockNumber"] != "0xc" {
		t.Fatalf("unexpected scanned receipt: %+v", scannedReceipt)
	}
	var scannedTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":105,"method":"debug_traceTransaction","params":["`+blockTxHashText+`"]}`, http.StatusOK, &scannedTrace)
	scannedTraceResult, ok := scannedTrace.Result.(map[string]any)
	if scannedTrace.Error != nil || !ok || scannedTraceResult["gas"] != float64(7) || scannedTraceResult["returnValue"] != "1234" {
		t.Fatalf("unexpected scanned debug trace: %+v", scannedTrace)
	}
	var scannedTransactionTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":106,"method":"trace_transaction","params":["`+blockTxHashText+`"]}`, http.StatusOK, &scannedTransactionTrace)
	scannedTransactionTraceItems, ok := scannedTransactionTrace.Result.([]any)
	if scannedTransactionTrace.Error != nil || !ok || len(scannedTransactionTraceItems) != 1 {
		t.Fatalf("unexpected scanned trace transaction: %+v", scannedTransactionTrace)
	}
	var pendingTxByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":99,"method":"eth_getTransactionByHash","params":["0x9999999999999999999999999999999999999999999999999999999999999999"]}`, http.StatusOK, &pendingTxByHash)
	pendingTxByHashResult, ok := pendingTxByHash.Result.(map[string]any)
	if pendingTxByHash.Error != nil || !ok || pendingTxByHashResult["hash"] != "0x9999999999999999999999999999999999999999999999999999999999999999" || pendingTxByHashResult["blockHash"] != nil {
		t.Fatalf("unexpected pending tx by hash: %+v", pendingTxByHash)
	}
	provider.pendingTxs = nil
	var scannedTxByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":103,"method":"eth_getTransactionByHash","params":["`+blockTxHashText+`"]}`, http.StatusOK, &scannedTxByHash)
	scannedTxByHashResult, ok := scannedTxByHash.Result.(map[string]any)
	if scannedTxByHash.Error != nil || !ok || scannedTxByHashResult["hash"] != blockTxHashText || scannedTxByHashResult["blockNumber"] != "0xc" {
		t.Fatalf("unexpected scanned tx by hash: %+v", scannedTxByHash)
	}
	provider.pendingTxs = []types.Tx{pendingTx}
	var uncleCount JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":73,"method":"eth_getUncleCountByBlockNumber","params":["latest"]}`, http.StatusOK, &uncleCount)
	if uncleCount.Error != nil || uncleCount.Result != "0x0" {
		t.Fatalf("unexpected uncle count: %+v", uncleCount)
	}
	var uncle JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":74,"method":"eth_getUncleByBlockNumberAndIndex","params":["latest","0x0"]}`, http.StatusOK, &uncle)
	if uncle.Error != nil || uncle.Result != nil {
		t.Fatalf("unexpected uncle response: %+v", uncle)
	}

	var blockReceipts JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":46,"method":"eth_getBlockReceipts","params":["latest"]}`, http.StatusOK, &blockReceipts)
	blockReceiptItems, ok := blockReceipts.Result.([]any)
	if blockReceipts.Error != nil || !ok || len(blockReceiptItems) != 1 {
		t.Fatalf("unexpected block receipts: %+v", blockReceipts)
	}
	blockReceipt, ok := blockReceiptItems[0].(map[string]any)
	if !ok || blockReceipt["transactionHash"] != blockTxHashText || blockReceipt["blockNumber"] != "0xc" {
		t.Fatalf("unexpected block receipt item: %+v", blockReceiptItems[0])
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xaaaa","balance":123,"nonce":7,"code":""}`)}
	var balance JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":31,"method":"eth_getBalance","params":["0xaaaa","latest"]}`, http.StatusOK, &balance)
	if balance.Error != nil || balance.Result != "0x7b" {
		t.Fatalf("unexpected balance response: %+v", balance)
	}
	if provider.appQueryPath[0] != "evm" || provider.appQueryPath[1] != "account" || provider.appQueryPath[2] != "0xaaaa" {
		t.Fatalf("unexpected balance query path: %+v", provider.appQueryPath)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xaaaa","balance":0,"balance_hex":"0x100000000000000000000","nonce":7,"code":""}`)}
	var largeBalance JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":131,"method":"eth_getBalance","params":["0xaaaa","latest"]}`, http.StatusOK, &largeBalance)
	if largeBalance.Error != nil || largeBalance.Result != "0x100000000000000000000" {
		t.Fatalf("unexpected large balance response: %+v", largeBalance)
	}
	var historicalBalance JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":109,"method":"eth_getBalance","params":["0xaaaa",{"blockNumber":"0xb"}]}`, http.StatusOK, &historicalBalance)
	if historicalBalance.Error != nil || historicalBalance.Result != "0x100000000000000000000" || !strings.Contains(string(provider.appQueryData), `"height":11`) {
		t.Fatalf("unexpected historical balance response=%+v data=%s", historicalBalance, provider.appQueryData)
	}

	var txCount JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":33,"method":"eth_getTransactionCount","params":["0xaaaa","latest"]}`, http.StatusOK, &txCount)
	if txCount.Error != nil || txCount.Result != "0x7" {
		t.Fatalf("unexpected transaction count response: %+v", txCount)
	}
	var pendingTxCount JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":97,"method":"eth_getTransactionCount","params":["0xaaaa","pending"]}`, http.StatusOK, &pendingTxCount)
	if pendingTxCount.Error != nil || pendingTxCount.Result != "0x8" {
		t.Fatalf("unexpected pending transaction count response: %+v", pendingTxCount)
	}
	var historicalTxCount JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":110,"method":"eth_getTransactionCount","params":["0xaaaa",{"blockHash":"0xac00000000000000000000000000000000000000000000000000000000000000"}]}`, http.StatusOK, &historicalTxCount)
	if historicalTxCount.Error != nil || historicalTxCount.Result != "0x7" || !strings.Contains(string(provider.appQueryData), `"height":11`) {
		t.Fatalf("unexpected historical transaction count response=%+v error=%+v data=%s", historicalTxCount, historicalTxCount.Error, provider.appQueryData)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xbbbb","code":"60016002"}`)}
	var code JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":34,"method":"eth_getCode","params":["0xbbbb","latest"]}`, http.StatusOK, &code)
	if code.Error != nil || code.Result != "0x60016002" {
		t.Fatalf("unexpected code response: %+v", code)
	}
	var historicalCode JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":107,"method":"eth_getCode","params":["0xbbbb","0xb"]}`, http.StatusOK, &historicalCode)
	if historicalCode.Error != nil || !strings.Contains(string(provider.appQueryData), `"height":11`) {
		t.Fatalf("unexpected historical code response=%+v data=%s", historicalCode, provider.appQueryData)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0xbbbb","slot":"0x0","value":"0x01"}`)}
	var storageAt JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":35,"method":"eth_getStorageAt","params":["0xbbbb","0x0","latest"]}`, http.StatusOK, &storageAt)
	if storageAt.Error != nil || storageAt.Result != "0x01" {
		t.Fatalf("unexpected storage response: %+v", storageAt)
	}
	var historicalStorageAt JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":108,"method":"eth_getStorageAt","params":["0xbbbb","0x0","finalized"]}`, http.StatusOK, &historicalStorageAt)
	if historicalStorageAt.Error != nil || !strings.Contains(string(provider.appQueryData), `"height":11`) {
		t.Fatalf("unexpected historical storage response=%+v data=%s", historicalStorageAt, provider.appQueryData)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"address":"0x000000000000000000000000000000000000bbbb","accountProof":["0xf8"],"balance":"0x7b","codeHash":"0xabc","nonce":"0x1","storageHash":"0xdef","storageProof":[{"key":"0x0","value":"0x1","proof":["0xc1"]}],"stateRoot":"0x1111111111111111111111111111111111111111111111111111111111111111"}`)}
	var proof JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":47,"method":"eth_getProof","params":["0xbbbb",["0x0"],"latest"]}`, http.StatusOK, &proof)
	proofResult, ok := proof.Result.(map[string]any)
	if proof.Error != nil || !ok || proofResult["balance"] != "0x7b" || provider.appQueryPath[0] != "evm" || provider.appQueryPath[1] != "eth_proof" {
		t.Fatalf("unexpected get proof response=%+v path=%+v", proof, provider.appQueryPath)
	}
	if !strings.Contains(string(provider.appQueryData), `"storage_keys":["0x0"]`) {
		t.Fatalf("unexpected proof query data: %s", provider.appQueryData)
	}
	var historicalProof JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":48,"method":"eth_getProof","params":["0xbbbb",[],"0xb"]}`, http.StatusOK, &historicalProof)
	if historicalProof.Error != nil || !strings.Contains(string(provider.appQueryData), `"height":11`) {
		t.Fatalf("unexpected historical proof response=%+v data=%s", historicalProof, provider.appQueryData)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"` + blockTxHashText + `","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234"}`)}
	var txByHash JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":32,"method":"eth_getTransactionByHash","params":["`+blockTxHashText+`"]}`, http.StatusOK, &txByHash)
	if txByHash.Error != nil {
		t.Fatalf("unexpected transaction error: %+v", txByHash)
	}
	txResult, ok := txByHash.Result.(map[string]any)
	if !ok || txResult["hash"] != blockTxHashText || txResult["blockNumber"] != "0xc" || txResult["gasPrice"] != "0x2" {
		t.Fatalf("unexpected transaction response: %+v", txByHash.Result)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"` + blockTxHashText + `","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234"}`)}
	var trace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":78,"method":"debug_traceTransaction","params":["`+blockTxHashText+`"]}`, http.StatusOK, &trace)
	traceResult, ok := trace.Result.(map[string]any)
	if trace.Error != nil || !ok || traceResult["gas"] != float64(7) || traceResult["failed"] != false || traceResult["returnValue"] != "1234" {
		t.Fatalf("unexpected debug trace: %+v", trace)
	}
	var callTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":79,"method":"debug_traceTransaction","params":["`+blockTxHashText+`",{"tracer":"callTracer"}]}`, http.StatusOK, &callTrace)
	callTraceResult, ok := callTrace.Result.(map[string]any)
	if callTrace.Error != nil || !ok || callTraceResult["type"] != "CALL" || callTraceResult["gasUsed"] != "0x7" {
		t.Fatalf("unexpected call trace: %+v", callTrace)
	}
	var prestateTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":114,"method":"debug_traceTransaction","params":["`+blockTxHashText+`",{"tracer":"prestateTracer"}]}`, http.StatusOK, &prestateTrace)
	prestateResult, ok := prestateTrace.Result.(map[string]any)
	if prestateTrace.Error != nil || !ok || len(prestateResult) == 0 {
		t.Fatalf("unexpected prestate tracer response: %+v", prestateTrace)
	}
	var fourByteTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":116,"method":"debug_traceTransaction","params":["`+blockTxHashText+`",{"tracer":"4byteTracer"}]}`, http.StatusOK, &fourByteTrace)
	_, ok = fourByteTrace.Result.(map[string]any)
	if fourByteTrace.Error != nil || !ok {
		t.Fatalf("unexpected 4byte tracer response: %+v", fourByteTrace)
	}
	var customTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":117,"method":"debug_traceTransaction","params":["`+blockTxHashText+`",{"tracer":"function(){return {}}"}]}`, http.StatusOK, &customTrace)
	customTraceResult, ok := customTrace.Result.(map[string]any)
	if customTrace.Error != nil || !ok || customTraceResult["gas"] != float64(7) {
		t.Fatalf("expected custom tracer to fall back to struct logger, got %+v", customTrace)
	}
	var debugBlockTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":80,"method":"debug_traceBlockByNumber","params":["latest"]}`, http.StatusOK, &debugBlockTrace)
	debugBlockItems, ok := debugBlockTrace.Result.([]any)
	if debugBlockTrace.Error != nil || !ok || len(debugBlockItems) != 1 {
		t.Fatalf("unexpected debug block trace: %+v", debugBlockTrace)
	}
	var debugBlockHashTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":81,"method":"debug_traceBlockByHash","params":["0xab00000000000000000000000000000000000000000000000000000000000000"]}`, http.StatusOK, &debugBlockHashTrace)
	debugBlockHashItems, ok := debugBlockHashTrace.Result.([]any)
	if debugBlockHashTrace.Error != nil || !ok || len(debugBlockHashItems) != 1 {
		t.Fatalf("unexpected debug block hash trace: %+v", debugBlockHashTrace)
	}
	var transactionTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":82,"method":"trace_transaction","params":["`+blockTxHashText+`"]}`, http.StatusOK, &transactionTrace)
	transactionTraceItems, ok := transactionTrace.Result.([]any)
	if transactionTrace.Error != nil || !ok || len(transactionTraceItems) != 1 {
		t.Fatalf("unexpected transaction trace: %+v", transactionTrace)
	}
	transactionTraceItem, ok := transactionTraceItems[0].(map[string]any)
	if !ok || transactionTraceItem["transactionHash"] != blockTxHashText || transactionTraceItem["type"] != "call" {
		t.Fatalf("unexpected transaction trace item: %+v", transactionTraceItems[0])
	}
	var blockTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":83,"method":"trace_block","params":["latest"]}`, http.StatusOK, &blockTrace)
	blockTraceItems, ok := blockTrace.Result.([]any)
	if blockTrace.Error != nil || !ok || len(blockTraceItems) != 1 {
		t.Fatalf("unexpected block trace: %+v", blockTrace)
	}
	var filterTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":84,"method":"trace_filter","params":[{"fromBlock":"0xc","toBlock":"0xc"}]}`, http.StatusOK, &filterTrace)
	filterTraceItems, ok := filterTrace.Result.([]any)
	if filterTrace.Error != nil || !ok || len(filterTraceItems) != 1 {
		t.Fatalf("unexpected filter trace: %+v", filterTrace)
	}
	var filteredByFrom JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":841,"method":"trace_filter","params":[{"fromBlock":"0xc","toBlock":"0xc","fromAddress":["0xaaaa"],"toAddress":"0xbbbb","count":1}]}`, http.StatusOK, &filteredByFrom)
	filteredByFromItems, ok := filteredByFrom.Result.([]any)
	if filteredByFrom.Error != nil || !ok || len(filteredByFromItems) != 1 {
		t.Fatalf("unexpected address-filtered trace: %+v", filteredByFrom)
	}
	var filteredAfter JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":842,"method":"trace_filter","params":[{"fromBlock":"0xc","toBlock":"0xc","fromAddress":"0xaaaa","after":1}]}`, http.StatusOK, &filteredAfter)
	filteredAfterItems, ok := filteredAfter.Result.([]any)
	if filteredAfter.Error != nil || !ok || len(filteredAfterItems) != 0 {
		t.Fatalf("unexpected paged trace result: %+v", filteredAfter)
	}
	var filteredMiss JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":843,"method":"trace_filter","params":[{"fromBlock":"0xc","toBlock":"0xc","toAddress":"0xcccc"}]}`, http.StatusOK, &filteredMiss)
	filteredMissItems, ok := filteredMiss.Result.([]any)
	if filteredMiss.Error != nil || !ok || len(filteredMissItems) != 0 {
		t.Fatalf("unexpected miss trace result: %+v", filteredMiss)
	}
	var traceGet JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":92,"method":"trace_get","params":["`+blockTxHashText+`",[]]}`, http.StatusOK, &traceGet)
	traceGetResult, ok := traceGet.Result.(map[string]any)
	if traceGet.Error != nil || !ok || traceGetResult["transactionHash"] != blockTxHashText {
		t.Fatalf("unexpected trace_get: %+v", traceGet)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"` + blockTxHashText + `","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234","vm_trace":{"calls":[{"type":"CALL","from":"0xaaaa","to":"0xcccc","gas":"0x5","gasUsed":"0x3","input":"0x12","output":"0x34","value":"0x2"}]}}`)}
	var nestedTransactionTrace JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":112,"method":"trace_transaction","params":["`+blockTxHashText+`"]}`, http.StatusOK, &nestedTransactionTrace)
	nestedTransactionTraceItems, ok := nestedTransactionTrace.Result.([]any)
	if nestedTransactionTrace.Error != nil || !ok || len(nestedTransactionTraceItems) != 2 {
		t.Fatalf("unexpected nested transaction trace: %+v", nestedTransactionTrace)
	}
	nestedChildTrace, ok := nestedTransactionTraceItems[1].(map[string]any)
	if !ok || !web3TraceAddressEqual(nestedChildTrace["traceAddress"], []uint64{0}) {
		t.Fatalf("unexpected nested child trace: %+v", nestedTransactionTraceItems[1])
	}
	var nestedTraceGet JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":113,"method":"trace_get","params":["`+blockTxHashText+`",[0]]}`, http.StatusOK, &nestedTraceGet)
	nestedTraceGetResult, ok := nestedTraceGet.Result.(map[string]any)
	nestedTraceGetAction, _ := nestedTraceGetResult["action"].(map[string]any)
	if nestedTraceGet.Error != nil || !ok || nestedTraceGetAction["to"] != "0xcccc" {
		t.Fatalf("unexpected nested trace_get: %+v", nestedTraceGet)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"` + blockTxHashText + `","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234"}`)}
	var replayTransaction JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":93,"method":"trace_replayTransaction","params":["`+blockTxHashText+`",["trace"]]}`, http.StatusOK, &replayTransaction)
	replayTransactionResult, ok := replayTransaction.Result.(map[string]any)
	replayTransactionTrace, _ := replayTransactionResult["trace"].([]any)
	_, replayTransactionStateDiff := replayTransactionResult["stateDiff"]
	_, replayTransactionVMTrace := replayTransactionResult["vmTrace"]
	if replayTransaction.Error != nil || !ok || len(replayTransactionTrace) != 1 || replayTransactionStateDiff || replayTransactionVMTrace {
		t.Fatalf("unexpected trace_replayTransaction: %+v", replayTransaction)
	}
	var replayTransactionCustomType JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":118,"method":"trace_replayTransaction","params":["`+blockTxHashText+`",["trace","customTrace"]]}`, http.StatusOK, &replayTransactionCustomType)
	replayCustomResult, ok := replayTransactionCustomType.Result.(map[string]any)
	if replayTransactionCustomType.Error != nil || !ok {
		t.Fatalf("expected custom replay type to be accepted, got %+v", replayTransactionCustomType)
	}
	if _, found := replayCustomResult["customTrace"]; !found {
		t.Fatalf("expected custom replay type key in response: %+v", replayCustomResult)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"` + blockTxHashText + `","height":12,"status":1,"from":"0xaaaa","to":"0xbbbb","gas_used":7,"output":"0x1234","state_diff":{"0xbbbb":{"storage":{"0x0":{"to":"0x1"}}}},"vm_trace":{"gas":7,"failed":false,"returnValue":"0x1234","structLogs":[{"pc":0,"op":"STOP"}]}}`)}
	var replayTransactionWithDiff JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":111,"method":"trace_replayTransaction","params":["`+blockTxHashText+`"]}`, http.StatusOK, &replayTransactionWithDiff)
	replayTransactionWithDiffResult, ok := replayTransactionWithDiff.Result.(map[string]any)
	replayStateDiff, stateDiffOK := replayTransactionWithDiffResult["stateDiff"].(map[string]any)
	replayVMTrace, vmTraceOK := replayTransactionWithDiffResult["vmTrace"].(map[string]any)
	if replayTransactionWithDiff.Error != nil || !ok || !stateDiffOK || replayStateDiff["0xbbbb"] == nil || !vmTraceOK || replayVMTrace["structLogs"] == nil {
		t.Fatalf("unexpected replay transaction state diff: %+v", replayTransactionWithDiff)
	}
	var replayBlock JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":94,"method":"trace_replayBlockTransactions","params":["latest",["trace"]]}`, http.StatusOK, &replayBlock)
	replayBlockItems, ok := replayBlock.Result.([]any)
	if replayBlock.Error != nil || !ok || len(replayBlockItems) != 1 {
		t.Fatalf("unexpected trace_replayBlockTransactions: %+v", replayBlock)
	}
	replayBlockItem, ok := replayBlockItems[0].(map[string]any)
	_, replayBlockStateDiff := replayBlockItem["stateDiff"]
	_, replayBlockVMTrace := replayBlockItem["vmTrace"]
	if !ok || replayBlockStateDiff || replayBlockVMTrace {
		t.Fatalf("unexpected replay block item fields: %+v", replayBlockItems[0])
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"tx_hash":"0xabc","status":1,"gas_used":7,"logs":[{"address":"0xcontract","data":"0x01"}]}`)}
	var receipt JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":3,"method":"eth_getTransactionReceipt","params":["0xabc"]}`, http.StatusOK, &receipt)
	if receipt.Error != nil {
		t.Fatalf("unexpected receipt error: %+v", receipt)
	}
	receiptResult, ok := receipt.Result.(map[string]any)
	if !ok || receiptResult["transactionHash"] != "0xabc" || receiptResult["status"] != "0x1" || receiptResult["gasUsed"] != "0x7" {
		t.Fatalf("unexpected web3 receipt: %+v", receipt.Result)
	}
	if provider.appQueryPath[0] != "evm" || provider.appQueryPath[1] != "receipt" || provider.appQueryPath[2] != "0xabc" {
		t.Fatalf("unexpected app query path: %+v", provider.appQueryPath)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`[
		{"address":"0xcontract","topics":["0xaaa","0xbbb"],"data":"0x01","block_number":13,"transaction_hash":"0xabc","log_index":0},
		{"address":"0xcontract","topics":["0xccc"],"data":"0x02","block_number":13,"transaction_hash":"0xdef","log_index":1}
	]`)}
	var getLogs JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":41,"method":"eth_getLogs","params":[{"address":["0xcontract"],"fromBlock":"0xd","toBlock":"0xd","topics":["0xaaa"]}]}`, http.StatusOK, &getLogs)
	filteredLogs, ok := getLogs.Result.([]any)
	if getLogs.Error != nil || !ok || len(filteredLogs) != 1 {
		t.Fatalf("unexpected filtered logs: %+v", getLogs)
	}
	filteredLog, ok := filteredLogs[0].(map[string]any)
	if !ok || filteredLog["blockNumber"] != "0xd" || filteredLog["transactionHash"] != "0xabc" {
		t.Fatalf("unexpected normalized log: %+v", filteredLogs[0])
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`[
		{"address":"0xcontract","topics":["0xaaa"],"data":"0x01","block_number":13,"transaction_hash":"0xabc","log_index":0}
	]`)}
	var allLogs JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":43,"method":"eth_getLogs","params":[{}]}`, http.StatusOK, &allLogs)
	allLogItems, ok := allLogs.Result.([]any)
	if allLogs.Error != nil || !ok || len(allLogItems) != 1 || len(provider.appQueryPath) != 2 || provider.appQueryPath[0] != "evm" || provider.appQueryPath[1] != "logs" {
		t.Fatalf("unexpected global logs response=%+v path=%+v", allLogs, provider.appQueryPath)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`[
		{"address":"0xcontract","topics":["0xaaa"],"data":"0x01","block_number":13,"transaction_hash":"0xabc","log_index":0},
		{"address":"0xcontract","topics":["0xbbb"],"data":"0x02","block_number":14,"transaction_hash":"0xdef","log_index":1}
	]`)}
	limitedHandler := NewHandlerWithConfig(provider, Config{Web3LogMaxResults: 1})
	var tooManyLogs JSONRPCResponse
	postJSON(t, limitedHandler, "/", `{"jsonrpc":"2.0","id":44,"method":"eth_getLogs","params":[{"address":"0xcontract"}]}`, http.StatusOK, &tooManyLogs)
	if tooManyLogs.Error == nil || tooManyLogs.Error.Code != -32005 {
		t.Fatalf("expected log result limit error, got %+v", tooManyLogs)
	}
	rangeLimitedHandler := NewHandlerWithConfig(provider, Config{Web3LogMaxBlockRange: 1})
	var tooWideLogs JSONRPCResponse
	postJSON(t, rangeLimitedHandler, "/", `{"jsonrpc":"2.0","id":45,"method":"eth_getLogs","params":[{"fromBlock":"0x1","toBlock":"0x3"}]}`, http.StatusOK, &tooWideLogs)
	if tooWideLogs.Error == nil || !strings.Contains(tooWideLogs.Error.Message, "block range") {
		t.Fatalf("expected log block range error, got %+v", tooWideLogs)
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
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`[
		{"address":"0xcontract","topics":["0xaaa"],"data":"0x01","block_number":13,"transaction_hash":"0xabc","log_index":0},
		{"address":"0xcontract","topics":["0xddd"],"data":"0x03","block_number":14,"transaction_hash":"0xghi","log_index":2}
	]`)}
	var logChanges JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":42,"method":"eth_getFilterChanges","params":["`+filterText+`"]}`, http.StatusOK, &logChanges)
	changedLogs, ok := logChanges.Result.([]any)
	if logChanges.Error != nil || !ok || len(changedLogs) != 1 {
		t.Fatalf("unexpected log filter changes: %+v", logChanges)
	}
	var uninstall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":10,"method":"eth_uninstallFilter","params":["`+filterText+`"]}`, http.StatusOK, &uninstall)
	if uninstall.Error != nil || uninstall.Result != true {
		t.Fatalf("unexpected uninstall response: %+v", uninstall)
	}

	var blockFilterID JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":36,"method":"eth_newBlockFilter","params":[]}`, http.StatusOK, &blockFilterID)
	blockFilterText, ok := blockFilterID.Result.(string)
	if blockFilterID.Error != nil || !ok || blockFilterText == "" {
		t.Fatalf("unexpected block filter id: %+v", blockFilterID)
	}
	nextBlockHash := types.Hash{0xac}
	provider.status.LatestHeight = 13
	provider.latest = 13
	provider.blocks[13] = store.BlockRecord{
		Block:   types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 13}},
		Hash:    nextBlockHash,
		AppHash: types.Hash{0xee},
	}
	var blockChanges JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":37,"method":"eth_getFilterChanges","params":["`+blockFilterText+`"]}`, http.StatusOK, &blockChanges)
	changes, ok := blockChanges.Result.([]any)
	if blockChanges.Error != nil || !ok || len(changes) != 1 || changes[0] != "0xac00000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("unexpected block filter changes: %+v", blockChanges)
	}

	var pendingFilterID JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":39,"method":"eth_newPendingTransactionFilter","params":[]}`, http.StatusOK, &pendingFilterID)
	pendingFilterText, ok := pendingFilterID.Result.(string)
	if pendingFilterID.Error != nil || !ok || pendingFilterText == "" {
		t.Fatalf("unexpected pending filter id: %+v", pendingFilterID)
	}
	provider.pendingHashes = []types.Hash{{0xfa}, {0xfb}}
	var pendingChanges JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":40,"method":"eth_getFilterChanges","params":["`+pendingFilterText+`"]}`, http.StatusOK, &pendingChanges)
	pending, ok := pendingChanges.Result.([]any)
	if pendingChanges.Error != nil || !ok || len(pending) != 1 || pending[0] != "0xfb00000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("unexpected pending filter changes: %+v", pendingChanges)
	}

	var subscribe JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":38,"method":"eth_subscribe","params":["newHeads"]}`, http.StatusOK, &subscribe)
	if subscribe.Error == nil || !strings.Contains(subscribe.Error.Message, "WebSocket") {
		t.Fatalf("expected WebSocket transport error, got %+v", subscribe)
	}

	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"output":"0x1234","gas_used":9,"access_list":[{"address":"0xbbbb","storage_keys":["0x01"]}],"state_diff":{"0xbbbb":{"storage":{"0x01":{"from":"0x00","to":"0x02"}}}},"vm_trace":{"structLogs":[{"op":"STOP","pc":0}]}}`)}
	var call JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":4,"method":"eth_call","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x1234","gas":"0x5208","gasPrice":"0x7","accessList":[{"address":"0xbbbb","storageKeys":["0x01"]}]},{"blockNumber":"0xb"}]}`, http.StatusOK, &call)
	if call.Error != nil || call.Result != "0x1234" {
		t.Fatalf("unexpected eth_call response: %+v", call)
	}
	if !strings.Contains(string(provider.appQueryData), `"input":"0x1234"`) {
		t.Fatalf("unexpected call query data: %s", provider.appQueryData)
	}
	if !strings.Contains(string(provider.appQueryData), `"height":11`) || !strings.Contains(string(provider.appQueryData), `"gas_price":7`) || !strings.Contains(string(provider.appQueryData), `"base_fee":11`) {
		t.Fatalf("expected call query to include block height, gas price, and base fee: %s", provider.appQueryData)
	}
	if !strings.Contains(string(provider.appQueryData), `"access_list":[{"address":"0xbbbb","storage_keys":["0x01"]}]`) {
		t.Fatalf("expected call query to include access list: %s", provider.appQueryData)
	}
	var overrideCall JSONRPCResponse
	overrideAddress := "0x000000000000000000000000000000000000bbbb"
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":44,"method":"eth_call","params":[{"from":"0xaaaa","to":"`+overrideAddress+`","data":"0x1234","authorizationList":[{"chainId":"0x1","address":"`+overrideAddress+`","nonce":"0x0","yParity":"0x0","r":"0x1","s":"0x1"}]},"latest",{"`+overrideAddress+`":{"balance":"0x64","nonce":"0x9","code":"0x6001","stateDiff":{"0x01":"0x02"}}},{"number":"0x63","time":"0x3039","gasLimit":"0x6acfc0","baseFeePerGas":"0x2a","blobBaseFee":"0x2b"}]}`, http.StatusOK, &overrideCall)
	if overrideCall.Error != nil {
		t.Fatalf("unexpected override eth_call response: %+v", overrideCall)
	}
	var overrideRequest web3CallRequest
	if err := json.Unmarshal(provider.appQueryData, &overrideRequest); err != nil {
		t.Fatal(err)
	}
	if overrideRequest.BlockOverride.Number != 99 || overrideRequest.BlockOverride.Timestamp != 12345 || overrideRequest.BlockOverride.GasLimit != 7_000_000 || overrideRequest.BaseFee != 42 || overrideRequest.BlobBaseFee != 43 {
		t.Fatalf("expected block override in call request, got %+v", overrideRequest)
	}
	if overrideRequest.SetCodeAuthorizationsJSON == "" || len(overrideRequest.StateOverrides) != 1 || overrideRequest.StateOverrides[overrideAddress].Balance != "0x64" || overrideRequest.StateOverrides[overrideAddress].Nonce == nil || *overrideRequest.StateOverrides[overrideAddress].Nonce != 9 {
		t.Fatalf("expected state override and authorization list in call request, got %+v", overrideRequest)
	}
	var invalidOverride JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":46,"method":"eth_call","params":[{"to":"`+overrideAddress+`"},"latest",{"`+overrideAddress+`":{"code":"6001","state":{"0x01":"0x02"},"stateDiff":{"0x01":"0x03"}}}]}`, http.StatusOK, &invalidOverride)
	if invalidOverride.Error == nil || invalidOverride.Error.Code != -32602 {
		t.Fatalf("expected invalid state override to be rejected, got %+v", invalidOverride)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"output":"0x602a60005260206000f3","gas_used":53000}`)}
	var createCall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":45,"method":"eth_call","params":[{"from":"0xaaaa","data":"0x600a600c600039600a6000f3602a60005260206000f3","gas":"0x10000"},"latest"]}`, http.StatusOK, &createCall)
	if createCall.Error != nil || createCall.Result != "0x602a60005260206000f3" || !strings.Contains(string(provider.appQueryData), `"method":"deploy"`) {
		t.Fatalf("unexpected contract creation eth_call response=%+v query=%s", createCall, provider.appQueryData)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"output":"0x1234","gas_used":9,"access_list":[{"address":"0xbbbb","storage_keys":["0x01"]}],"state_diff":{"0xbbbb":{"storage":{"0x01":{"from":"0x00","to":"0x02"}}}},"vm_trace":{"structLogs":[{"op":"STOP","pc":0}]}}`)}

	var estimate JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":5,"method":"eth_estimateGas","params":[{"to":"0xbbbb","gas":"0x100"}]}`, http.StatusOK, &estimate)
	if estimate.Error != nil || estimate.Result != "0x5208" {
		t.Fatalf("unexpected estimate response: %+v", estimate)
	}
	var intrinsicEstimate JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":56,"method":"eth_estimateGas","params":[{"to":"0xbbbb","data":"0x0001","accessList":[{"address":"0xbbbb","storageKeys":["0x01"]}],"gas":"0x100"}]}`, http.StatusOK, &intrinsicEstimate)
	if intrinsicEstimate.Error != nil || intrinsicEstimate.Result != "0x62e8" {
		t.Fatalf("expected estimate to honor calldata/access-list intrinsic gas, got %+v", intrinsicEstimate)
	}
	var deployEstimate JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":57,"method":"eth_estimateGas","params":[{"data":"0x00","gas":"0x100"}]}`, http.StatusOK, &deployEstimate)
	if deployEstimate.Error != nil || deployEstimate.Result != "0xcf0e" {
		t.Fatalf("expected create estimate to honor deploy intrinsic gas, got %+v", deployEstimate)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"output":"0x","gas_used":9,"failed":true,"error":"execution reverted"}`)}
	var failedEstimate JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":55,"method":"eth_estimateGas","params":[{"to":"0xbbbb","gas":"0x100"}]}`, http.StatusOK, &failedEstimate)
	if failedEstimate.Error == nil || failedEstimate.Error.Message != "execution reverted" {
		t.Fatalf("expected failed estimate to surface EVM failure, got %+v", failedEstimate)
	}
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`{"output":"0x1234","gas_used":9,"access_list":[{"address":"0xbbbb","storage_keys":["0x01"]}],"state_diff":{"0xbbbb":{"storage":{"0x01":{"from":"0x00","to":"0x02"}}}},"vm_trace":{"structLogs":[{"op":"STOP","pc":0}]}}`)}

	var accessList JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":85,"method":"eth_createAccessList","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x1234"},"latest"]}`, http.StatusOK, &accessList)
	accessResult, ok := accessList.Result.(map[string]any)
	accessEntries, _ := accessResult["accessList"].([]any)
	if accessList.Error != nil || !ok || accessResult["gasUsed"] != "0x9" || len(accessEntries) != 1 {
		t.Fatalf("unexpected access list response: %+v", accessList)
	}
	accessEntry, ok := accessEntries[0].(map[string]any)
	storageKeys, _ := accessEntry["storageKeys"].([]any)
	if !ok || accessEntry["address"] != "0xbbbb" || len(storageKeys) != 1 || storageKeys[0] != "0x01" {
		t.Fatalf("unexpected access list entry: %+v", accessEntries[0])
	}

	var debugCall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":86,"method":"debug_traceCall","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x1234"},"latest",{"tracer":"callTracer"}]}`, http.StatusOK, &debugCall)
	debugCallResult, ok := debugCall.Result.(map[string]any)
	if debugCall.Error != nil || !ok || debugCallResult["type"] != "CALL" || debugCallResult["gasUsed"] != "0x9" {
		t.Fatalf("unexpected debug trace call: %+v", debugCall)
	}

	var debugStruct JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":88,"method":"debug_traceCall","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x1234"},"latest",{}]}`, http.StatusOK, &debugStruct)
	debugStructResult, ok := debugStruct.Result.(map[string]any)
	structLogs, _ := debugStructResult["structLogs"].([]any)
	if debugStruct.Error != nil || !ok || len(structLogs) != 1 {
		t.Fatalf("unexpected debug struct trace call: %+v", debugStruct)
	}
	var prestateDebugCall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":115,"method":"debug_traceCall","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x12345678"},"latest",{"tracer":"prestateTracer"}]}`, http.StatusOK, &prestateDebugCall)
	prestateDebugCallResult, ok := prestateDebugCall.Result.(map[string]any)
	if prestateDebugCall.Error != nil || !ok || len(prestateDebugCallResult) != 2 {
		t.Fatalf("unexpected prestate debug_traceCall response: %+v", prestateDebugCall)
	}
	var fourByteDebugCall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":116,"method":"debug_traceCall","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x12345678"},"latest",{"tracer":"4byteTracer"}]}`, http.StatusOK, &fourByteDebugCall)
	fourByteDebugCallResult, ok := fourByteDebugCall.Result.(map[string]any)
	if fourByteDebugCall.Error != nil || !ok || fourByteDebugCallResult["0x12345678-0"] != float64(1) {
		t.Fatalf("unexpected 4byte debug_traceCall response: %+v", fourByteDebugCall)
	}

	var traceCall JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":87,"method":"trace_call","params":[{"from":"0xaaaa","to":"0xbbbb","data":"0x1234"},["trace"],"latest"]}`, http.StatusOK, &traceCall)
	traceCallResult, ok := traceCall.Result.(map[string]any)
	traceItems, _ := traceCallResult["trace"].([]any)
	traceStateDiff, _ := traceCallResult["stateDiff"].(map[string]any)
	traceVMTrace, _ := traceCallResult["vmTrace"].(map[string]any)
	if traceCall.Error != nil || !ok || traceCallResult["output"] != "0x1234" || len(traceItems) != 1 || len(traceStateDiff) == 0 || len(traceVMTrace) == 0 {
		t.Fatalf("unexpected trace call: %+v", traceCall)
	}
}

func TestWeb3ReceiptUsesBlockCumulativeGas(t *testing.T) {
	txHash1 := "0x1111111111111111111111111111111111111111111111111111111111111111"
	txHash2 := "0x2222222222222222222222222222222222222222222222222222222222222222"
	receipt1 := web3Receipt{TxHash: txHash1, Height: 3, Status: 1, From: "0xaaaa", To: "0xbbbb", GasUsed: 7}
	receipt2 := web3Receipt{TxHash: txHash2, Height: 3, Status: 1, From: "0xcccc", To: "0xdddd", GasUsed: 11}
	encoded1, err := json.Marshal(receipt1)
	if err != nil {
		t.Fatal(err)
	}
	encoded2, err := json.Marshal(receipt2)
	if err != nil {
		t.Fatal(err)
	}
	provider := fakeStatusProvider{
		blocks: map[types.Height]store.BlockRecord{
			3: {
				Block: types.Block{
					Header: types.Header{Height: 3},
					Txs: []types.Tx{
						types.Tx("evm:call:fee=7:gas=21000:signer=0xaaaa:nonce=0:eth_hash=" + txHash1),
						types.Tx("evm:call:fee=11:gas=21000:signer=0xcccc:nonce=0:eth_hash=" + txHash2),
					},
				},
				Hash: types.Hash{3},
				TxResults: []types.Result{
					{Data: encoded1, GasUsed: 7},
					{Data: encoded2, GasUsed: 11},
				},
			},
		},
	}
	object, rpcErr := web3ReceiptObject(context.Background(), provider, encoded2)
	if rpcErr != nil {
		t.Fatalf("unexpected receipt error: %+v", rpcErr)
	}
	receiptObject, ok := object.(map[string]any)
	if !ok || receiptObject["transactionIndex"] != "0x1" || receiptObject["gasUsed"] != "0xb" || receiptObject["cumulativeGasUsed"] != "0x12" {
		t.Fatalf("unexpected cumulative receipt object: %+v", object)
	}
}

func TestHandlerWeb3StateRootFailsClosedWhenUnavailable(t *testing.T) {
	blockHash := types.Hash{0xab}
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", EVMChainID: 7, Running: true, LatestHeight: 1},
		blocks: map[types.Height]store.BlockRecord{
			1: {
				Block:   types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 1}},
				Hash:    blockHash,
				AppHash: types.Hash{0xcd},
			},
		},
		latest:           1,
		appQueryResponse: vexoapp.QueryResponse{Code: 1, Log: "missing EVM snapshot"},
	}
	handler := NewHandlerWithConfig(provider, Config{StrictEVMStateRoot: true})

	var response JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["latest",false]}`, http.StatusOK, &response)
	if response.Error == nil || response.Error.Code != -32000 || !strings.Contains(response.Error.Message, "EVM state root is unavailable") {
		t.Fatalf("expected strict EVM state-root failure, got %+v", response)
	}

	handler = NewHandlerWithConfig(provider, Config{})
	response = JSONRPCResponse{}
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":2,"method":"eth_getBlockByNumber","params":["latest",false]}`, http.StatusOK, &response)
	if response.Error == nil || response.Error.Code != -32000 || !strings.Contains(response.Error.Message, "EVM state root is unavailable") {
		t.Fatalf("expected EVM state-root failure without fallback, got %+v", response)
	}
}

func TestWeb3EVMCallUsesHistoricalFeeContext(t *testing.T) {
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", EVMChainID: 7, Running: true, LatestHeight: 9},
		state:  store.StateRecord{Height: 9, BaseFee: 11, NextBaseFee: 12, BlobBaseFee: 13, NextBlobBaseFee: 14},
		states: map[types.Height]store.StateRecord{
			7: {Height: 7, BaseFee: 3, NextBaseFee: 4, BlobBaseFee: 5, NextBlobBaseFee: 6},
		},
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"output":"0x"}`)},
	}
	handler := NewHandler(provider)

	var response JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":1,"method":"eth_call","params":[{"to":"0x000000000000000000000000000000000000beef","data":"0x"},"0x7"]}`, http.StatusOK, &response)
	if response.Error != nil {
		t.Fatalf("unexpected eth_call response: %+v", response)
	}
	var call web3CallRequest
	if err := json.Unmarshal(provider.appQueryData, &call); err != nil {
		t.Fatal(err)
	}
	if call.Height != 7 || call.BaseFee != 3 || call.BlobBaseFee != 5 || call.GasPrice != 0 {
		t.Fatalf("expected historical call fee context, got %+v", call)
	}
	var dynamicFee JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":2,"method":"eth_call","params":[{"to":"0x000000000000000000000000000000000000beef","data":"0x","maxFeePerGas":"0xa","maxPriorityFeePerGas":"0x2"},"0x7"]}`, http.StatusOK, &dynamicFee)
	if dynamicFee.Error != nil {
		t.Fatalf("unexpected dynamic fee eth_call response: %+v", dynamicFee)
	}
	if err := json.Unmarshal(provider.appQueryData, &call); err != nil {
		t.Fatal(err)
	}
	if call.GasPrice != 5 || call.MaxFeePerGas != 10 || call.MaxPriorityFeePerGas != 2 {
		t.Fatalf("expected EIP-1559 effective call gas price, got %+v", call)
	}
	largeMaxFee := new(big.Int).Lsh(big.NewInt(1), 80)
	largePriority := new(big.Int).Lsh(big.NewInt(1), 79)
	largePayload := `{"jsonrpc":"2.0","id":3,"method":"eth_call","params":[{"to":"0x000000000000000000000000000000000000beef","data":"0x","maxFeePerGas":"0x` + largeMaxFee.Text(16) + `","maxPriorityFeePerGas":"0x` + largePriority.Text(16) + `"},"0x7"]}`
	var largeFee JSONRPCResponse
	postJSON(t, handler, "/web3", largePayload, http.StatusOK, &largeFee)
	if largeFee.Error != nil {
		t.Fatalf("unexpected large dynamic fee eth_call response: %+v", largeFee)
	}
	call = web3CallRequest{}
	if err := json.Unmarshal(provider.appQueryData, &call); err != nil {
		t.Fatal(err)
	}
	expectedEffective := new(big.Int).Add(largePriority, big.NewInt(3))
	if call.GasPrice != 0 || call.GasPriceHex != "0x"+expectedEffective.Text(16) || call.MaxFeePerGasHex != "0x"+largeMaxFee.Text(16) || call.MaxPriorityFeePerGasHex != "0x"+largePriority.Text(16) {
		t.Fatalf("expected uint256 EIP-1559 fee context, got %+v", call)
	}
	tooLargeGasPrice := new(big.Int).Lsh(big.NewInt(1), 256)
	var tooLarge JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":4,"method":"eth_call","params":[{"to":"0x000000000000000000000000000000000000beef","data":"0x","gasPrice":"0x`+tooLargeGasPrice.Text(16)+`"},"0x7"]}`, http.StatusOK, &tooLarge)
	if tooLarge.Error == nil || !strings.Contains(tooLarge.Error.Message, "invalid gasPrice quantity") {
		t.Fatalf("expected oversized gasPrice rejection, got %+v", tooLarge)
	}
}

func TestHandlerWeb3UsesConfiguredEVMChainID(t *testing.T) {
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", EVMChainID: 77, Running: true},
	}
	handler := NewHandler(provider)

	var netVersion JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":1,"method":"net_version","params":[]}`, http.StatusOK, &netVersion)
	if netVersion.Error != nil || netVersion.Result != "77" {
		t.Fatalf("unexpected net_version response: %+v", netVersion)
	}
	var chainID JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":2,"method":"eth_chainId","params":[]}`, http.StatusOK, &chainID)
	if chainID.Error != nil || chainID.Result != "0x4d" {
		t.Fatalf("unexpected eth_chainId response: %+v", chainID)
	}

	rawConfiguredTx, configuredHash := signedTestEthereumTx(t, 77)
	var sendConfigured JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":3,"method":"eth_sendRawTransaction","params":["`+rawConfiguredTx+`"]}`, http.StatusOK, &sendConfigured)
	if sendConfigured.Error != nil || sendConfigured.Result != configuredHash || len(provider.submitted) != 1 {
		t.Fatalf("expected configured chain tx to be accepted, response=%+v submitted=%d", sendConfigured, len(provider.submitted))
	}

	rawFallbackTx, _ := signedTestEthereumTx(t, chainNumericID("vexo-chain"))
	var sendFallback JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":4,"method":"eth_sendRawTransaction","params":["`+rawFallbackTx+`"]}`, http.StatusOK, &sendFallback)
	if sendFallback.Error == nil || len(provider.submitted) != 1 {
		t.Fatalf("expected fallback-derived chain tx to be rejected, response=%+v submitted=%d", sendFallback, len(provider.submitted))
	}
}

func TestHandlerAllowsWeb3CORSPreflightForRemix(t *testing.T) {
	handler := NewHandlerWithConfig(fakeStatusProvider{}, Config{CORSAllowedOrigins: []string{"https://remix.ethereum.org"}})
	request := httptest.NewRequest(http.MethodOptions, "/web3", nil)
	request.Header.Set("Origin", "https://remix.ethereum.org")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected preflight success, got %d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://remix.ethereum.org" {
		t.Fatalf("unexpected allow origin %q", got)
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("expected POST in CORS methods, got %q", response.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestHandlerAddsCORSHeadersToWeb3ChainID(t *testing.T) {
	handler := NewHandlerWithConfig(fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", EVMChainID: 2026, Running: true},
	}, Config{})
	request := httptest.NewRequest(http.MethodPost, "/web3", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://remix.ethereum.org")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected eth_chainId success, got %d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected default permissive CORS origin for Web3 tooling, got %q", got)
	}
	var payload JSONRPCResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != nil || payload.Result != "0x7ea" {
		t.Fatalf("unexpected chain id payload: %+v", payload)
	}
}

func TestHandlerWeb3ManagedAccountSigning(t *testing.T) {
	const privateKeyHex = "4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5"
	key, err := gethcrypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	address := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	provider := &fakeStatusProvider{
		status:           node.Status{ChainID: "vexo-chain", EVMChainID: 77, Running: true, LatestHeight: 3},
		state:            store.StateRecord{Height: 3, BaseFee: 9, NextBaseFee: 11},
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"address":"` + address + `","balance":1000000000000,"nonce":7,"code":""}`)},
	}
	disabledHandler := NewHandlerWithConfig(provider, Config{EVMAccountPrivateKeys: []string{privateKeyHex}})
	var disabledAccounts JSONRPCResponse
	postJSON(t, disabledHandler, "/web3", `{"jsonrpc":"2.0","id":0,"method":"eth_accounts","params":[]}`, http.StatusOK, &disabledAccounts)
	if disabledAccounts.Error != nil || len(disabledAccounts.Result.([]any)) != 0 {
		t.Fatalf("expected managed accounts to be disabled by default, got %+v", disabledAccounts)
	}

	handler := NewHandlerWithConfig(provider, Config{EnableEVMManagedAccounts: true, EVMAccountPrivateKeys: []string{privateKeyHex}})

	var accounts JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":1,"method":"eth_accounts","params":[]}`, http.StatusOK, &accounts)
	accountList, ok := accounts.Result.([]any)
	if accounts.Error != nil || !ok || len(accountList) != 1 || !strings.EqualFold(accountList[0].(string), address) {
		t.Fatalf("unexpected eth_accounts response: %+v", accounts)
	}

	var coinbase JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":2,"method":"eth_coinbase","params":[]}`, http.StatusOK, &coinbase)
	if coinbase.Error != nil || !strings.EqualFold(coinbase.Result.(string), address) {
		t.Fatalf("unexpected eth_coinbase response: %+v", coinbase)
	}

	var sign JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":3,"method":"eth_sign","params":["`+address+`","0x6869"]}`, http.StatusOK, &sign)
	assertWeb3TextSignature(t, sign, address, []byte("hi"))

	var personalSign JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":4,"method":"personal_sign","params":["0x6869","`+address+`"]}`, http.StatusOK, &personalSign)
	assertWeb3TextSignature(t, personalSign, address, []byte("hi"))

	txJSON := `{"from":"` + address + `","to":"0x000000000000000000000000000000000000beef","value":"0x5","gas":"0x5208","maxFeePerGas":"0x14","maxPriorityFeePerGas":"0x2","data":"0x1234"}`
	var signedTx JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":5,"method":"eth_signTransaction","params":[`+txJSON+`]}`, http.StatusOK, &signedTx)
	signedTxResult, ok := signedTx.Result.(map[string]any)
	if signedTx.Error != nil || !ok || signedTxResult["raw"] == "" {
		t.Fatalf("unexpected eth_signTransaction response: %+v", signedTx)
	}
	rawText, ok := signedTxResult["raw"].(string)
	if !ok {
		t.Fatalf("expected raw signed tx, got %+v", signedTxResult)
	}
	decoded, err := ethcompat.DecodeRawTransaction(rawText, ethcompat.DecodeOptions{ChainID: 77, BaseFee: 11})
	if err != nil {
		t.Fatalf("expected signed tx to decode: %v", err)
	}
	if decoded.Nonce != 7 || decoded.Gas != 21000 || decoded.MaxFeePerGas != 20 || decoded.MaxPriorityFeePerGas != 2 || !strings.EqualFold(string(decoded.From), address) {
		t.Fatalf("unexpected decoded signed tx: %+v", decoded)
	}
	txObject, ok := signedTxResult["tx"].(map[string]any)
	if !ok || txObject["chainId"] != "0x4d" || txObject["nonce"] != "0x7" {
		t.Fatalf("unexpected signed transaction object: %+v", signedTxResult["tx"])
	}

	var sendTx JSONRPCResponse
	postJSON(t, handler, "/web3", `{"jsonrpc":"2.0","id":6,"method":"eth_sendTransaction","params":[`+txJSON+`]}`, http.StatusOK, &sendTx)
	if sendTx.Error != nil || len(provider.submitted) != 1 {
		t.Fatalf("unexpected eth_sendTransaction response=%+v submitted=%d", sendTx, len(provider.submitted))
	}
	submitted := string(provider.submitted[0])
	if result, ok := sendTx.Result.(string); !ok || !strings.HasPrefix(result, "0x") || !strings.Contains(submitted, "eth_hash="+result) || !strings.Contains(submitted, ethcompat.TagRaw+"=") {
		t.Fatalf("unexpected send transaction result=%+v submitted=%q", sendTx.Result, submitted)
	}
}

func assertWeb3TextSignature(t *testing.T, response JSONRPCResponse, expectedAddress string, payload []byte) {
	t.Helper()
	if response.Error != nil {
		t.Fatalf("unexpected signing error: %+v", response)
	}
	signatureText, ok := response.Result.(string)
	if !ok || !strings.HasPrefix(signatureText, "0x") {
		t.Fatalf("expected hex signature, got %+v", response.Result)
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(signatureText, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(signature) != 65 {
		t.Fatalf("expected 65-byte signature, got %d", len(signature))
	}
	if signature[64] >= 27 {
		signature[64] -= 27
	}
	publicKey, err := gethcrypto.SigToPub(gethaccounts.TextHash(payload), signature)
	if err != nil {
		t.Fatal(err)
	}
	if recovered := gethcrypto.PubkeyToAddress(*publicKey).Hex(); !strings.EqualFold(recovered, expectedAddress) {
		t.Fatalf("expected recovered %s, got %s", expectedAddress, recovered)
	}
}

func TestWeb3CallPrestateTraceIncludesStateOverride(t *testing.T) {
	nonce := uint64(9)
	address := "0x000000000000000000000000000000000000bbbb"
	trace := web3CallPrestateTrace(web3CallRequest{
		From: address,
		StateOverrides: map[string]evmmodule.CallStateOverride{
			address: {
				Balance: "100",
				Nonce:   &nonce,
				Code:    "0x6001",
				StateDiff: map[string]string{
					"0x01": "0x02",
				},
			},
		},
	})
	account, ok := trace[address].(map[string]any)
	storage, _ := account["storage"].(map[string]any)
	if !ok || account["balance"] != "0x64" || account["nonce"] != "0x9" || account["code"] != "0x6001" {
		t.Fatalf("expected prestate override account, got %+v", trace)
	}
	if storage["0x0000000000000000000000000000000000000000000000000000000000000001"] != "0x0000000000000000000000000000000000000000000000000000000000000002" {
		t.Fatalf("expected normalized override storage, got %+v", storage)
	}
}

func TestWeb3FilterStoreEvictsOldestFilters(t *testing.T) {
	filters := newWeb3FilterStore()
	filters.max = 2
	first := filters.addBlock(1)
	second := filters.addPending(nil)
	third := filters.addLog(web3Filter{}, nil, 3)

	if _, found := filters.get(first); found {
		t.Fatalf("expected oldest filter %s to be evicted", first)
	}
	if _, found := filters.get(second); !found {
		t.Fatalf("expected second filter %s to remain", second)
	}
	if _, found := filters.get(third); !found {
		t.Fatalf("expected third filter %s to remain", third)
	}
}

func TestWeb3FilterStoreSnapshotRestore(t *testing.T) {
	filters := newWeb3FilterStore()
	logFilterID := filters.addLog(web3Filter{
		Addresses: []string{"0xcontract"},
		Topics:    [][]string{{"0xaaa"}},
		FromBlock: 7,
		ToBlock:   9,
	}, []any{map[string]any{"transactionHash": "0xabc", "logIndex": "0x0"}}, 9)
	pendingFilterID := filters.addPending([]types.Hash{{1, 2, 3}})
	snapshot := filters.Snapshot()

	restored := newWeb3FilterStore()
	restored.Restore(snapshot)
	logFilter, found := restored.get(logFilterID)
	if !found || logFilter.LastHeight != 9 || len(logFilter.Addresses) != 1 || len(logFilter.Topics) != 1 || len(logFilter.SeenLogs) != 1 {
		t.Fatalf("expected restored log filter, got found=%t filter=%+v", found, logFilter)
	}
	pendingFilter, found := restored.get(pendingFilterID)
	if !found || len(pendingFilter.SeenPending) != 1 {
		t.Fatalf("expected restored pending filter, got found=%t filter=%+v", found, pendingFilter)
	}
	snapshot.Filters[0].Addresses[0] = "0xmutated"
	logFilter, _ = restored.get(logFilterID)
	if logFilter.Addresses[0] != "0xcontract" {
		t.Fatalf("expected restore to deep-copy filters, got %+v", logFilter)
	}
}

func TestWeb3FilterStoreSnapshotPathRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	filters := newWeb3FilterStore()
	firstID := filters.addBlock(7)
	if err := saveWeb3FilterStoreSnapshotAtomic(path, filters.Snapshot()); err != nil {
		t.Fatal(err)
	}
	restored, err := newWeb3FilterStoreWithConfig(Config{Web3FilterSnapshotPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := restored.get(firstID); !found {
		t.Fatalf("expected restored filter %s", firstID)
	}
	if nextID := restored.addBlock(8); nextID != "0x2" {
		t.Fatalf("expected persisted next filter id, got %s", nextID)
	}
}

func TestServerShutdownPersistsWeb3FilterStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	server := NewServer(fakeStatusProvider{status: node.Status{Running: true}}, Config{Web3FilterSnapshotPath: path})
	filterID := server.filterStore.addBlock(11)
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	restored, err := newWeb3FilterStoreWithConfig(Config{Web3FilterSnapshotPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := restored.get(filterID); !found {
		t.Fatalf("expected shutdown to persist filter %s", filterID)
	}
}

func TestServerPersistsWeb3FilterStoreOnMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	server := NewServer(fakeStatusProvider{status: node.Status{Running: true}}, Config{Web3FilterSnapshotPath: path})
	filterID := server.filterStore.addBlock(12)

	restored, err := newWeb3FilterStoreWithConfig(Config{Web3FilterSnapshotPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := restored.get(filterID); !found {
		t.Fatalf("expected mutation to persist filter %s", filterID)
	}
}

func TestWeb3FilterSnapshotPersistErrorFailsRPC(t *testing.T) {
	filters := newWeb3FilterStore()
	filters.setOnChange(func(Web3FilterStoreSnapshot) error {
		return errors.New("disk full")
	})
	provider := fakeStatusProvider{status: node.Status{Running: true, LatestHeight: 7}}

	result, rpcErr := executeWeb3Method(context.Background(), provider, Config{}, filters, "eth_newBlockFilter", nil)
	if rpcErr == nil || !strings.Contains(rpcErr.Message, "web3 filter snapshot persistence failed") {
		t.Fatalf("expected persist failure, result=%v err=%+v", result, rpcErr)
	}
	if _, found := filters.get("0x1"); !found {
		t.Fatal("expected in-memory filter to remain for operator inspection")
	}
	_, rpcErr = executeWeb3Method(context.Background(), provider, Config{}, filters, "eth_getFilterChanges", []json.RawMessage{json.RawMessage(`"0x1"`)})
	if rpcErr == nil || !strings.Contains(rpcErr.Message, "web3 filter snapshot persistence failed") {
		t.Fatalf("expected subsequent filter RPC to fail while persistence is unhealthy, got %+v", rpcErr)
	}
}

func TestServerReportsCorruptWeb3FilterSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := NewServer(fakeStatusProvider{status: node.Status{Running: true}}, Config{Web3FilterSnapshotPath: path})
	if server.StartupError() == nil {
		t.Fatal("expected corrupt filter snapshot to fail server startup")
	}
}

func TestWeb3TransactionDetailsPreservesUint256Value(t *testing.T) {
	value := new(big.Int).Lsh(big.NewInt(1), 80)
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "evm",
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "00", "21000", value.String()},
		Tags: map[string]string{
			ethcompat.TagValue: value.String(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	details := web3TransactionDetails(tx)
	if details.Value != 0 || details.ValueHex != "0x"+value.Text(16) || web3TxValueHex(details) != "0x"+value.Text(16) {
		t.Fatalf("unexpected uint256 transaction value details: %+v", details)
	}
}

func TestWeb3TransactionDetailsPreservesUint256FeeQuantities(t *testing.T) {
	gasPrice := new(big.Int).Lsh(big.NewInt(1), 80)
	maxFee := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 81), big.NewInt(7))
	maxPriority := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 79), big.NewInt(3))
	blobFeeCap := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 78), big.NewInt(5))
	fee := new(big.Int).Mul(gasPrice, big.NewInt(21000))
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "evm",
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "00", "21000", "0"},
		Tags: map[string]string{
			"fee":                             fee.String(),
			"gas":                             "21000",
			ethcompat.TagGasPrice:             gasPrice.String(),
			ethcompat.TagMaxFeePerGas:         maxFee.String(),
			ethcompat.TagMaxPriorityFeePerGas: maxPriority.String(),
			ethcompat.TagBlobGasFeeCap:        blobFeeCap.String(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	details := web3TransactionDetails(tx)
	if details.GasPrice != 0 || web3TxGasPriceHex(details) != "0x"+gasPrice.Text(16) {
		t.Fatalf("unexpected gas price details: %+v", details)
	}
	if details.MaxFeePerGas != 0 || web3TxMaxFeePerGasHex(details) != "0x"+maxFee.Text(16) {
		t.Fatalf("unexpected max fee details: %+v", details)
	}
	if details.MaxPriorityFeePerGas != 0 || web3TxMaxPriorityFeePerGasHex(details) != "0x"+maxPriority.Text(16) {
		t.Fatalf("unexpected priority fee details: %+v", details)
	}
	if details.BlobGasFeeCap != 0 || web3TxBlobGasFeeCapHex(details) != "0x"+blobFeeCap.Text(16) {
		t.Fatalf("unexpected blob fee cap details: %+v", details)
	}
	record := store.BlockRecord{
		Block: types.Block{
			Header: types.Header{Height: 3},
			Txs:    []types.Tx{tx},
		},
	}
	rendered, ok := web3TransactionFromBlockRecord(record, 0, "0xhash", tx).(map[string]any)
	if !ok {
		t.Fatal("expected transaction object")
	}
	if rendered["gasPrice"] != "0x"+gasPrice.Text(16) ||
		rendered["maxFeePerGas"] != "0x"+maxFee.Text(16) ||
		rendered["maxPriorityFeePerGas"] != "0x"+maxPriority.Text(16) ||
		rendered["maxFeePerBlobGas"] != "0x"+blobFeeCap.Text(16) {
		t.Fatalf("unexpected rendered EVM fee quantities: %+v", rendered)
	}
}

func TestWeb3JSONRPCBatchAndNotifications(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", Running: true, LatestHeight: 12, PeerCount: 2},
	})
	var batch []JSONRPCResponse
	postJSON(t, handler, "/", `[
		{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]},
		{"jsonrpc":"2.0","method":"net_listening","params":[]},
		{"jsonrpc":"2.0","id":2,"method":"net_peerCount","params":[]}
	]`, http.StatusOK, &batch)
	if len(batch) != 2 || string(batch[0].ID) != "1" || batch[0].Result != "0xc" || string(batch[1].ID) != "2" || batch[1].Result != "0x2" {
		t.Fatalf("unexpected batch response: %+v", batch)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","method":"net_listening","params":[]}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || strings.TrimSpace(recorder.Body.String()) != "" {
		t.Fatalf("unexpected notification response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerServesWeb3WebSocketNewHeads(t *testing.T) {
	firstHash := types.Hash{0x01}
	secondHash := types.Hash{0x02}
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", LatestHeight: 1},
		blocks: map[types.Height]store.BlockRecord{
			1: {
				Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 1}},
				Hash:  firstHash,
			},
			2: {
				Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 2}},
				Hash:  secondHash,
			},
		},
		latest: 1,
		appQueryResponses: map[string]vexoapp.QueryResponse{
			"evm/eth_state_root": {Value: []byte(`{"state_root":"0x1111111111111111111111111111111111111111111111111111111111111111"}`)},
		},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{},
		send:     func(value any) { sent = append(sent, value) },
	}
	subscriptionID, rpcErr := session.subscribe([]json.RawMessage{json.RawMessage(`"newHeads"`)})
	if rpcErr != nil || subscriptionID == "" {
		t.Fatalf("unexpected subscription response id=%q err=%+v", subscriptionID, rpcErr)
	}

	provider.status.LatestHeight = 2
	provider.latest = 2
	session.publish()
	if len(sent) != 1 {
		t.Fatalf("expected one notification, got %+v", sent)
	}
	notification, ok := sent[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected notification type: %+v", sent[0])
	}
	if notification["method"] != "eth_subscription" {
		t.Fatalf("unexpected notification: %+v", notification)
	}
	params, ok := notification["params"].(map[string]any)
	if !ok || params["subscription"] != subscriptionID {
		t.Fatalf("unexpected subscription params: %+v", notification)
	}
	result, ok := params["result"].(map[string]any)
	if !ok || result["number"] != "0x2" || result["hash"] != "0x0200000000000000000000000000000000000000000000000000000000000000" || result["stateRoot"] != "0x1111111111111111111111111111111111111111111111111111111111111111" {
		t.Fatalf("unexpected head result: %+v", result)
	}
}

func TestHandlerWeb3WebSocketStrictStateRootSkipsUnavailableHead(t *testing.T) {
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", LatestHeight: 2},
		blocks: map[types.Height]store.BlockRecord{
			1: {Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 1}}, Hash: types.Hash{0x01}},
			2: {Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 2}}, Hash: types.Hash{0x02}, AppHash: types.Hash{0xcd}},
		},
		latest:           2,
		appQueryResponse: vexoapp.QueryResponse{Code: 1, Log: "missing EVM snapshot"},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		cfg:      Config{StrictEVMStateRoot: true},
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{"0xsub": {ID: "0xsub", Type: "newHeads", LastHeight: 1}},
		send:     func(value any) { sent = append(sent, value) },
	}
	session.publish()
	if len(sent) != 0 {
		t.Fatalf("expected strict state-root policy to skip head without EVM snapshot, got %+v", sent)
	}
}

func TestHandlerBoundsWeb3WebSocketCatchUp(t *testing.T) {
	blocks := make(map[types.Height]store.BlockRecord)
	for height := types.Height(1); height <= types.Height(web3SubscriptionMaxCatchUp+2); height++ {
		blocks[height] = store.BlockRecord{Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: height}}}
	}
	provider := &fakeStatusProvider{
		status:           node.Status{ChainID: "vexo-chain", LatestHeight: types.Height(web3SubscriptionMaxCatchUp + 2)},
		blocks:           blocks,
		latest:           types.Height(web3SubscriptionMaxCatchUp + 2),
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"state_root":"0x1111111111111111111111111111111111111111111111111111111111111111"}`)},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{},
		send:     func(value any) { sent = append(sent, value) },
	}
	session.subs["0xsub"] = web3Subscription{ID: "0xsub", Type: "newHeads"}

	session.publish()
	if len(sent) != web3SubscriptionMaxCatchUp {
		t.Fatalf("expected bounded catch-up batch, got %d", len(sent))
	}
	if session.subs["0xsub"].LastHeight != web3SubscriptionMaxCatchUp {
		t.Fatalf("expected bounded last height, got %d", session.subs["0xsub"].LastHeight)
	}
}

func TestHandlerUsesConfiguredWeb3WebSocketCatchUpLimit(t *testing.T) {
	blocks := make(map[types.Height]store.BlockRecord)
	for height := types.Height(1); height <= 5; height++ {
		blocks[height] = store.BlockRecord{Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: height}}}
	}
	provider := &fakeStatusProvider{
		status:           node.Status{ChainID: "vexo-chain", LatestHeight: 5},
		blocks:           blocks,
		latest:           5,
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`{"state_root":"0x1111111111111111111111111111111111111111111111111111111111111111"}`)},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		cfg:      Config{Web3SubscriptionMaxCatchUp: 2},
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{"0xsub": {ID: "0xsub", Type: "newHeads"}},
		send:     func(value any) { sent = append(sent, value) },
	}

	session.publish()
	if len(sent) != 2 || session.subs["0xsub"].LastHeight != 2 {
		t.Fatalf("expected configured catch-up limit, sent=%d sub=%+v", len(sent), session.subs["0xsub"])
	}
}

func TestHandlerLimitsWeb3WebSocketSubscriptionsPerConnection(t *testing.T) {
	provider := &fakeStatusProvider{status: node.Status{ChainID: "vexo-chain", LatestHeight: 1}}
	session := &web3SubscriptionSession{
		provider: provider,
		cfg:      Config{Web3SubscriptionMaxPerConn: 1},
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{},
		send:     func(value any) {},
	}
	if id, rpcErr := session.subscribe([]json.RawMessage{json.RawMessage(`"newHeads"`)}); rpcErr != nil || id == "" {
		t.Fatalf("unexpected first subscription id=%q err=%+v", id, rpcErr)
	}
	if id, rpcErr := session.subscribe([]json.RawMessage{json.RawMessage(`"newHeads"`)}); rpcErr == nil || id != "" || rpcErr.Code != -32005 {
		t.Fatalf("expected subscription limit rejection, id=%q err=%+v", id, rpcErr)
	}
}

func TestHandlerServesWeb3WebSocketLogSubscriptions(t *testing.T) {
	provider := &fakeStatusProvider{
		status:           node.Status{ChainID: "vexo-chain", LatestHeight: 1},
		appQueryResponse: vexoapp.QueryResponse{Value: []byte(`[]`)},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{},
		send:     func(value any) { sent = append(sent, value) },
	}
	subscriptionID, rpcErr := session.subscribe([]json.RawMessage{
		json.RawMessage(`"logs"`),
		json.RawMessage(`{"address":"0xcontract"}`),
	})
	if rpcErr != nil || subscriptionID == "" {
		t.Fatalf("unexpected log subscription response id=%q err=%+v", subscriptionID, rpcErr)
	}

	provider.status.LatestHeight = 2
	provider.appQueryResponse = vexoapp.QueryResponse{Value: []byte(`[{"address":"0xcontract","data":"0x01"}]`)}
	session.publish()
	if len(sent) != 1 {
		t.Fatalf("expected one log notification, got %+v", sent)
	}
	notification, ok := sent[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected notification type: %+v", sent[0])
	}
	params, ok := notification["params"].(map[string]any)
	if notification["method"] != "eth_subscription" || !ok || params["subscription"] != subscriptionID {
		t.Fatalf("unexpected log notification: %+v", notification)
	}
	result, ok := params["result"].(map[string]any)
	if !ok || result["address"] != "0xcontract" || result["data"] != "0x01" {
		t.Fatalf("unexpected log result: %+v", params["result"])
	}
}

func TestHandlerServesWeb3WebSocketPendingTransactionSubscriptions(t *testing.T) {
	firstHash := types.Hash{0x01}
	secondHash := types.Hash{0x02}
	provider := &fakeStatusProvider{
		status:        node.Status{ChainID: "vexo-chain", LatestHeight: 1},
		pendingHashes: []types.Hash{firstHash},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{},
		send:     func(value any) { sent = append(sent, value) },
	}
	subscriptionID, rpcErr := session.subscribe([]json.RawMessage{json.RawMessage(`"newPendingTransactions"`)})
	if rpcErr != nil || subscriptionID == "" {
		t.Fatalf("unexpected pending subscription response id=%q err=%+v", subscriptionID, rpcErr)
	}

	provider.pendingHashes = []types.Hash{firstHash, secondHash}
	session.publish()
	if len(sent) != 1 {
		t.Fatalf("expected one pending tx notification, got %+v", sent)
	}
	notification, ok := sent[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected notification type: %+v", sent[0])
	}
	params, ok := notification["params"].(map[string]any)
	if notification["method"] != "eth_subscription" || !ok || params["subscription"] != subscriptionID {
		t.Fatalf("unexpected pending notification: %+v", notification)
	}
	if params["result"] != "0x0200000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("unexpected pending hash: %+v", params["result"])
	}
}

func TestHandlerRetriesPendingWebSocketTransactionsBeyondConfiguredRunLimit(t *testing.T) {
	firstHash := types.Hash{0x01}
	secondHash := types.Hash{0x02}
	provider := &fakeStatusProvider{
		status:        node.Status{ChainID: "vexo-chain", LatestHeight: 1},
		pendingHashes: []types.Hash{firstHash, secondHash},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		cfg:      Config{Web3SubscriptionMaxPendingRun: 1},
		ctx:      context.Background(),
		subs: map[string]web3Subscription{
			"0xsub": {ID: "0xsub", Type: "newPendingTransactions", SeenPending: map[string]bool{}},
		},
		send: func(value any) { sent = append(sent, value) },
	}

	session.publish()
	session.publish()
	if len(sent) != 2 {
		t.Fatalf("expected pending overflow to be retried on next tick, got %+v", sent)
	}
}

func TestHandlerServesWeb3WebSocketFullPendingTransactionSubscriptions(t *testing.T) {
	pendingTx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "evm",
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "abcd", "21000", "5"},
		Tags: map[string]string{
			"signer":             "0xaaaa",
			"nonce":              "3",
			"gas":                "21000",
			"fee":                "42000",
			ethcompat.TagHash:    "0x9999999999999999999999999999999999999999999999999999999999999999",
			ethcompat.TagChainID: "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", LatestHeight: 1},
	}
	sent := make([]any, 0)
	session := &web3SubscriptionSession{
		provider: provider,
		ctx:      context.Background(),
		subs:     map[string]web3Subscription{},
		send:     func(value any) { sent = append(sent, value) },
	}
	subscriptionID, rpcErr := session.subscribe([]json.RawMessage{json.RawMessage(`"newPendingTransactions"`), json.RawMessage(`true`)})
	if rpcErr != nil || subscriptionID == "" {
		t.Fatalf("unexpected pending subscription response id=%q err=%+v", subscriptionID, rpcErr)
	}

	provider.pendingTxs = []types.Tx{pendingTx}
	session.publish()
	if len(sent) != 1 {
		t.Fatalf("expected one full pending tx notification, got %+v", sent)
	}
	notification, ok := sent[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected notification type: %+v", sent[0])
	}
	params, ok := notification["params"].(map[string]any)
	if notification["method"] != "eth_subscription" || !ok || params["subscription"] != subscriptionID {
		t.Fatalf("unexpected full pending notification: %+v", notification)
	}
	result, ok := params["result"].(map[string]any)
	if !ok || result["hash"] != "0x9999999999999999999999999999999999999999999999999999999999999999" || result["from"] != "0xaaaa" || result["blockHash"] != nil {
		t.Fatalf("unexpected full pending result: %+v", params["result"])
	}
}

func TestHandlerWeb3GasPriceFailsClosedWhenBaseFeeUnavailable(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{status: node.Status{ChainID: "vexo-chain", LatestHeight: 7}})

	var response JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":1,"method":"eth_gasPrice","params":[]}`, http.StatusOK, &response)
	if response.Error != nil || response.Result != "0x0" {
		t.Fatalf("expected zero gas price fallback, got %+v", response)
	}
}

func TestHandlerWeb3LatestBlockFallsBackToGenesisShapeWhenNoBlocksExist(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", Running: true, LatestHeight: 0},
	})

	var response JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["latest",false]}`, http.StatusOK, &response)
	if response.Error != nil {
		t.Fatalf("unexpected latest block error: %+v", response)
	}
	block, ok := response.Result.(map[string]any)
	if !ok || block["number"] != "0x0" || block["parentHash"] != "0x0000000000000000000000000000000000000000000000000000000000000000" || block["hash"] == nil {
		t.Fatalf("unexpected genesis-shaped latest block: %+v", response.Result)
	}

	response = JSONRPCResponse{}
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":2,"method":"eth_getBlockTransactionCountByNumber","params":["latest"]}`, http.StatusOK, &response)
	if response.Error != nil || response.Result != "0x0" {
		t.Fatalf("unexpected latest block transaction count: %+v", response)
	}
}

func TestHandlerWeb3FinalizedTagFailsClosedWithoutFinalityProof(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{
		status: node.Status{ChainID: "vexo-chain", LatestHeight: 7},
		latest: 7,
		blocks: map[types.Height]store.BlockRecord{
			7: {Block: types.Block{Header: types.Header{ChainID: "vexo-chain", Height: 7}}, Hash: types.Hash{7}},
		},
	})

	var response JSONRPCResponse
	postJSON(t, handler, "/", `{"jsonrpc":"2.0","id":2,"method":"eth_getBlockByNumber","params":["finalized",false]}`, http.StatusOK, &response)
	if response.Error == nil || !strings.Contains(response.Error.Message, "finalized block is unavailable") {
		t.Fatalf("expected finalized fail-closed error, got %+v", response)
	}
}

func TestHandlerDetectsWebSocketUpgrade(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/web3", nil)
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Connection", "keep-alive, Upgrade")
	if !isWebSocketUpgrade(request) {
		t.Fatal("expected websocket upgrade to be detected")
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
	if latest.Height != 7 || latest.BlockHash[:2] != "01" || latest.QuorumCert.Round != 2 || latest.Strict {
		t.Fatalf("unexpected latest finality proof: %+v", latest)
	}
	var strictError map[string]string
	getJSON(t, handler, "/v1/finality/latest?strict=true", http.StatusNotFound, &strictError)
	if strictError["error"] == "" {
		t.Fatalf("expected strict finality error, got %+v", strictError)
	}

	var byHeight FinalityProofResponse
	getJSON(t, handler, "/v1/finality/7", http.StatusOK, &byHeight)
	if provider.finalityHeight != 7 || byHeight.ValidatorSetHash[:2] != "03" {
		t.Fatalf("unexpected finality by height: proof=%+v requested=%d", byHeight, provider.finalityHeight)
	}

	provider.finalityProof.CommitChain = []finality.CommitLink{
		{Header: types.Header{ChainID: "vexo-test", Height: 8}, BlockHash: types.Hash{8}, QuorumCert: finality.QuorumCert{Height: 8, Round: 0, BlockHash: types.Hash{8}}},
		{Header: types.Header{ChainID: "vexo-test", Height: 9}, BlockHash: types.Hash{9}, QuorumCert: finality.QuorumCert{Height: 9, Round: 0, BlockHash: types.Hash{9}}},
	}
	var strictLatest FinalityProofResponse
	getJSON(t, handler, "/v1/finality/latest?strict=1", http.StatusOK, &strictLatest)
	if !strictLatest.Strict || len(strictLatest.CommitChain) != 2 {
		t.Fatalf("unexpected strict finality proof: %+v", strictLatest)
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
	if snapshot.Checksum == "" {
		t.Fatal("expected snapshot checksum")
	}
}

func TestHandlerExportsSnapshotChunks(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{snapshot: node.StateSnapshot{
		Height:           5,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
		StateRoots: []store.StateRootRecord{
			{Height: 5, Namespace: "bank", Root: types.Hash{4}},
		},
		KV: []store.KVPair{
			{Namespace: "bank", Key: []byte("carol"), Value: []byte("30")},
			{Namespace: "bank", Key: []byte("alice"), Value: []byte("100")},
			{Namespace: "bank", Key: []byte("bob"), Value: []byte("70")},
		},
	}})

	var first SnapshotChunkResponse
	getJSON(t, handler, "/snapshot/chunk?index=0&size=2", http.StatusOK, &first)
	if first.SchemaVersion != "v1" || first.ChunkIndex != 0 || first.ChunkCount != 2 || len(first.KV) != 2 {
		t.Fatalf("unexpected first chunk: %+v", first)
	}
	if first.SnapshotChecksum == "" || first.ChunkChecksum == "" {
		t.Fatalf("expected chunk checksums: %+v", first)
	}
	if string(first.KV[0].Key) != "alice" || string(first.KV[1].Key) != "bob" {
		t.Fatalf("expected deterministic chunk ordering, got %+v", first.KV)
	}

	var second SnapshotChunkResponse
	getJSON(t, handler, "/snapshot/chunk?index=1&size=2", http.StatusOK, &second)
	if second.ChunkIndex != 1 || second.ChunkCount != 2 || len(second.KV) != 1 || string(second.KV[0].Key) != "carol" {
		t.Fatalf("unexpected second chunk: %+v", second)
	}
	if second.SnapshotChecksum != first.SnapshotChecksum {
		t.Fatalf("expected same snapshot checksum, first=%s second=%s", first.SnapshotChecksum, second.SnapshotChecksum)
	}
}

func TestHandlerRejectsInvalidSnapshotChunkQuery(t *testing.T) {
	handler := NewHandler(fakeStatusProvider{snapshot: node.StateSnapshot{
		Height:           5,
		AppHash:          types.Hash{1},
		LastBlockHash:    types.Hash{2},
		ValidatorSetHash: types.Hash{3},
		StateRoots:       []store.StateRootRecord{{Height: 5, Namespace: "bank", Root: types.Hash{4}}},
	}})

	var response map[string]string
	getJSON(t, handler, "/snapshot/chunk?index=0&size=0", http.StatusBadRequest, &response)
	if response["error"] == "" {
		t.Fatalf("expected invalid size error, got %+v", response)
	}
	getJSON(t, handler, "/snapshot/chunk?index=1&size=10", http.StatusBadRequest, &response)
	if response["error"] == "" {
		t.Fatalf("expected out of range error, got %+v", response)
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

func TestHandlerAllowsAdminRouteWhenNoTokenConfigured(t *testing.T) {
	provider := &fakeStatusProvider{pruneResult: store.PruneResult{RetainFromHeight: 3, PrunedBlocks: 1}}
	handler := NewHandlerWithConfig(provider, Config{})

	var response PruneResponse
	postJSON(t, handler, "/prune", `{"retain_from_height":3}`, http.StatusOK, &response)

	if response.RetainFromHeight != 3 || len(provider.prunedHeights) != 1 {
		t.Fatalf("unexpected tokenless admin prune: response=%+v heights=%+v", response, provider.prunedHeights)
	}
}

func TestHandlerEnforcesScopedAdminTokensAndAudits(t *testing.T) {
	provider := &fakeStatusProvider{
		pruneResult:  store.PruneResult{RetainFromHeight: 3, PrunedBlocks: 1},
		replayResult: vexoruntime.ReplayResult{FromHeight: 1, ToHeight: 1, LastHash: types.Hash{1}, Blocks: 1},
	}
	var events []AdminAuditEvent
	handler := NewHandlerWithConfig(provider, Config{
		AdminTokens: map[string][]string{
			"prune-token":  {"prune"},
			"replay-token": {"replay"},
			"root-token":   {"*"},
		},
		AdminAuditSink: func(event AdminAuditEvent) {
			events = append(events, event)
		},
	})

	var forbidden map[string]string
	postJSONWithToken(t, handler, "/prune", `{"retain_from_height":3}`, "replay-token", http.StatusForbidden, &forbidden)
	if forbidden["error"] == "" {
		t.Fatalf("expected scoped auth failure, got %+v", forbidden)
	}
	var prune PruneResponse
	postJSONWithToken(t, handler, "/prune", `{"retain_from_height":3}`, "prune-token", http.StatusOK, &prune)
	var replay ReplayResponse
	postJSONWithToken(t, handler, "/replay", `{"all":true}`, "root-token", http.StatusOK, &replay)

	if len(events) != 3 {
		t.Fatalf("expected three audit events, got %+v", events)
	}
	if events[0].Authorized || events[0].Scope != "prune" || events[0].Reason == "" {
		t.Fatalf("unexpected failed audit event: %+v", events[0])
	}
	if !events[1].Authorized || events[1].Scope != "prune" {
		t.Fatalf("unexpected prune audit event: %+v", events[1])
	}
	if !events[2].Authorized || events[2].Scope != "replay" {
		t.Fatalf("unexpected replay audit event: %+v", events[2])
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

func TestHandlerReplaysStrictStoredBlocks(t *testing.T) {
	provider := &fakeStatusProvider{replayResult: vexoruntime.ReplayResult{
		FromHeight: 2,
		ToHeight:   4,
		LastHash:   types.Hash{9},
		Blocks:     3,
	}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ReplayResponse
	postJSONWithToken(t, handler, "/replay", `{"from_height":2,"to_height":4,"strict":true}`, "secret", http.StatusOK, &response)

	if response.FromHeight != 2 || response.ToHeight != 4 || response.Blocks != 3 {
		t.Fatalf("unexpected strict replay response: %+v", response)
	}
	if len(provider.strictReplayRanges) != 1 || provider.strictReplayRanges[0] != [2]types.Height{2, 4} || len(provider.replayRanges) != 0 {
		t.Fatalf("unexpected replay calls: strict=%+v normal=%+v", provider.strictReplayRanges, provider.replayRanges)
	}
}

func TestHandlerReplaysAllStrictStoredBlocks(t *testing.T) {
	provider := &fakeStatusProvider{replayResult: vexoruntime.ReplayResult{
		FromHeight: 1,
		ToHeight:   5,
		LastHash:   types.Hash{7},
		Blocks:     5,
	}}
	handler := NewHandlerWithConfig(provider, Config{AdminToken: "secret"})

	var response ReplayResponse
	postJSONWithToken(t, handler, "/replay", `{"all":true,"strict":true}`, "secret", http.StatusOK, &response)

	if !provider.strictReplayAllCalled || provider.replayAllCalled || response.FromHeight != 1 || response.ToHeight != 5 {
		t.Fatalf("unexpected strict replay all response: strict=%v normal=%v response=%+v", provider.strictReplayAllCalled, provider.replayAllCalled, response)
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

func signedTestEthereumTx(t *testing.T, chainID uint64) (string, string) {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     7,
		GasTipCap: big.NewInt(2),
		GasFeeCap: big.NewInt(20),
		Gas:       21_000,
		To:        &to,
		Value:     big.NewInt(3),
		Data:      []byte{0x12, 0x34},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex()
}

func unprotectedLegacyEthereumTx(t *testing.T) (string, string) {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	tx := gethtypes.NewTransaction(8, to, big.NewInt(1), 21_000, big.NewInt(1), nil)
	signed, err := gethtypes.SignTx(tx, gethtypes.HomesteadSigner{}, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex()
}

func signedTestEthereumBlobTx(t *testing.T, chainID uint64) (string, string, ethcompat.BlobSidecarBundle) {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	var blob kzg4844.Blob
	blob[0] = 1
	blob[31] = 2
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	if err != nil {
		t.Fatal(err)
	}
	blobHash := gethcommon.Hash(kzg4844.CalcBlobHashV1(sha256.New(), &commitment))
	sidecar := &gethtypes.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{blob},
		Commitments: []kzg4844.Commitment{commitment},
		Proofs:      []kzg4844.Proof{proof},
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	tx := gethtypes.NewTx(&gethtypes.BlobTx{
		ChainID:    uint256.NewInt(chainID),
		Nonce:      8,
		GasTipCap:  uint256.NewInt(2),
		GasFeeCap:  uint256.NewInt(20),
		Gas:        50_000,
		To:         to,
		Value:      uint256.NewInt(3),
		Data:       []byte{0x12, 0x34},
		BlobFeeCap: uint256.NewInt(9),
		BlobHashes: []gethcommon.Hash{blobHash},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ethcompat.BlobSidecarBundleFromGeth(sidecar, []string{blobHash.Hex()})
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex(), bundle
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

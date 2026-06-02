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

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
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
	submitErr         error
	submitted         []types.Tx
	blocks            map[types.Height]store.BlockRecord
	latest            types.Height
	blockErr          error
	index             store.BlockIndex
	state             store.StateRecord
	roots             map[string]store.StateRootRecord
	stateErr          error
	pruneResult       store.PruneResult
	pruneErr          error
	prunedHeights     []types.Height
	replayResult      vexoruntime.ReplayResult
	replayErr         error
	replayAllCalled   bool
	replayRanges      [][2]types.Height
	validators        validator.Set
	committee         committee.Committee
	validatorErr      error
	evidenceResult    consensus.SlashResult
	evidenceApplied   bool
	evidenceErr       error
	evidenceSubmitted []slashing.Evidence
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
	if metrics.ChainID != "vexo-test" || !metrics.Running || metrics.LatestHeight != 9 || metrics.TotalBlocks != 9 {
		t.Fatalf("unexpected metrics identity: %+v", metrics)
	}
	if metrics.ValidatorCount != 4 || metrics.TotalVotingPower != 100 || metrics.PeerCount != 3 || metrics.BannedPeers != 1 || !metrics.ConsensusLoopRunning {
		t.Fatalf("unexpected metrics counters: %+v", metrics)
	}
	if metrics.LatestAppHash[:6] != "010203" || metrics.ValidatorSetHash[:6] != "040506" {
		t.Fatalf("unexpected metrics hashes: %+v", metrics)
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

func TestHandlerPrunesBlocksAndStateRoots(t *testing.T) {
	provider := &fakeStatusProvider{pruneResult: store.PruneResult{
		RetainFromHeight: 3,
		PrunedBlocks:     2,
		PrunedStateRoots: 4,
	}}
	handler := NewHandler(provider)

	var response PruneResponse
	postJSON(t, handler, "/prune", `{"retain_from_height":3}`, http.StatusOK, &response)

	if response.RetainFromHeight != 3 || response.PrunedBlocks != 2 || response.PrunedStateRoots != 4 {
		t.Fatalf("unexpected prune response: %+v", response)
	}
	if len(provider.prunedHeights) != 1 || provider.prunedHeights[0] != 3 {
		t.Fatalf("unexpected prune heights: %+v", provider.prunedHeights)
	}
}

func TestHandlerRejectsInvalidPruneRequests(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{})
	cases := []string{
		`{}`,
		`{"retain_from_height":0}`,
		`{"retain_from_height":1,"extra":true}`,
	}
	for _, body := range cases {
		var response map[string]string
		postJSON(t, handler, "/prune", body, http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected prune error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerRejectsUnavailablePruneProvider(t *testing.T) {
	var response map[string]string
	postJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/prune", `{"retain_from_height":2}`, http.StatusNotImplemented, &response)
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
		handler := NewHandler(&fakeStatusProvider{pruneErr: testCase.err})
		var response map[string]string
		postJSON(t, handler, "/prune", `{"retain_from_height":2}`, testCase.expectedStatus, &response)
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
	handler := NewHandler(provider)

	var response ReplayResponse
	postJSON(t, handler, "/replay", `{"from_height":2,"to_height":4}`, http.StatusOK, &response)

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
	handler := NewHandler(provider)

	var response ReplayResponse
	postJSON(t, handler, "/replay", `{"all":true}`, http.StatusOK, &response)

	if !provider.replayAllCalled || response.FromHeight != 1 || response.ToHeight != 5 || response.Blocks != 5 {
		t.Fatalf("unexpected replay all response: called=%v response=%+v", provider.replayAllCalled, response)
	}
}

func TestHandlerRejectsInvalidReplayRequests(t *testing.T) {
	handler := NewHandler(&fakeStatusProvider{})
	cases := []string{
		`{"from_height":1}`,
		`{"to_height":2}`,
		`{"all":true,"from_height":1}`,
		`{"from_height":1,"to_height":2,"extra":true}`,
	}
	for _, body := range cases {
		var response map[string]string
		postJSON(t, handler, "/replay", body, http.StatusBadRequest, &response)
		if response["error"] == "" {
			t.Fatalf("expected replay error for %s, got %+v", body, response)
		}
	}
}

func TestHandlerRejectsUnavailableReplayProvider(t *testing.T) {
	var response map[string]string
	postJSON(t, NewHandler(struct{ StatusProvider }{fakeStatusProvider{}}), "/replay", `{"all":true}`, http.StatusNotImplemented, &response)
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
		handler := NewHandler(&fakeStatusProvider{replayErr: testCase.err})
		var response map[string]string
		postJSON(t, handler, "/replay", `{"all":true}`, testCase.expectedStatus, &response)
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

func postJSON(t *testing.T, handler http.Handler, path string, body string, expectedStatus int, value any) {
	t.Helper()
	requestJSON(t, handler, http.MethodPost, path, body, "192.0.2.1:1234", expectedStatus, value)
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body string, remoteAddr string, expectedStatus int, value any) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = remoteAddr
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
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

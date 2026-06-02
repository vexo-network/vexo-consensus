package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type fakeStatusProvider struct {
	status    node.Status
	submitErr error
	submitted []types.Tx
	blocks    map[types.Height]store.BlockRecord
	latest    types.Height
	blockErr  error
}

func (provider fakeStatusProvider) Status(ctx context.Context) node.Status {
	return provider.status
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
	})

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
	request := httptest.NewRequest(http.MethodGet, path, nil)
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

func postJSON(t *testing.T, handler http.Handler, path string, body string, expectedStatus int, value any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
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

package rpc

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

type fakeStatusProvider struct {
	status node.Status
}

func (provider fakeStatusProvider) Status(ctx context.Context) node.Status {
	return provider.status
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

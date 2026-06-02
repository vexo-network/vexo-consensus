package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
)

const defaultReadHeaderTimeout = 5 * time.Second

type StatusProvider interface {
	Status(ctx context.Context) node.Status
}

type Config struct {
	Address           string
	ReadHeaderTimeout time.Duration
}

type Server struct {
	provider StatusProvider
	server   *http.Server
}

type HealthResponse struct {
	OK bool `json:"ok"`
}

type StatusResponse struct {
	ChainID       string `json:"chain_id"`
	Running       bool   `json:"running"`
	LatestHeight  uint64 `json:"latest_height"`
	LatestAppHash string `json:"latest_app_hash"`
	DataDir       string `json:"data_dir"`
	PeerCount     int    `json:"peer_count"`
	BannedPeers   int    `json:"banned_peers"`
}

type PeerResponse struct {
	Peer           string `json:"peer"`
	Score          int64  `json:"score"`
	Banned         bool   `json:"banned"`
	BannedUntil    string `json:"banned_until,omitempty"`
	WindowMessages uint64 `json:"window_messages"`
}

type PeersResponse struct {
	Peers []PeerResponse `json:"peers"`
}

func NewServer(provider StatusProvider, cfg Config) *Server {
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	handler := NewHandler(provider)
	return &Server{
		provider: provider,
		server: &http.Server{
			Addr:              cfg.Address,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		},
	}
}

func (server *Server) Start(listener net.Listener) error {
	if listener == nil {
		return errors.New("listener is required")
	}
	err := server.server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (server *Server) Shutdown(ctx context.Context) error {
	return server.server.Shutdown(ctx)
}

func NewHandler(provider StatusProvider) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		writeJSON(writer, http.StatusOK, HealthResponse{OK: true})
	})
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		status := provider.Status(request.Context())
		if !status.Running {
			writeJSON(writer, http.StatusServiceUnavailable, HealthResponse{OK: false})
			return
		}
		writeJSON(writer, http.StatusOK, HealthResponse{OK: true})
	})
	mux.HandleFunc("/status", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		writeJSON(writer, http.StatusOK, statusResponse(provider.Status(request.Context())))
	})
	mux.HandleFunc("/peers", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		status := provider.Status(request.Context())
		writeJSON(writer, http.StatusOK, PeersResponse{Peers: peerResponses(status.Peers)})
	})
	return mux
}

func allowGet(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet {
		return true
	}
	writer.Header().Set("Allow", http.MethodGet)
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func statusResponse(status node.Status) StatusResponse {
	return StatusResponse{
		ChainID:       status.ChainID,
		Running:       status.Running,
		LatestHeight:  uint64(status.LatestHeight),
		LatestAppHash: hex.EncodeToString(status.LatestAppHash[:]),
		DataDir:       status.DataDir,
		PeerCount:     status.PeerCount,
		BannedPeers:   status.BannedPeers,
	}
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

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

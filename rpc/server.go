package rpc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const defaultReadHeaderTimeout = 5 * time.Second
const defaultMaxRequestBytes = 1024 * 1024

type StatusProvider interface {
	Status(ctx context.Context) node.Status
}

type TxSubmitter interface {
	SubmitTx(ctx context.Context, tx types.Tx) error
}

type BlockProvider interface {
	BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error)
	LatestBlock(ctx context.Context) (store.BlockRecord, error)
}

type Config struct {
	Address           string
	ReadHeaderTimeout time.Duration
	MaxRequestBytes   int64
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

type SubmitTxRequest struct {
	Tx       string `json:"tx"`
	Encoding string `json:"encoding,omitempty"`
}

type SubmitTxResponse struct {
	Accepted bool   `json:"accepted"`
	TxHash   string `json:"tx_hash"`
}

type BlockResponse struct {
	Height       uint64              `json:"height"`
	Hash         string              `json:"hash"`
	AppHash      string              `json:"app_hash"`
	ChainID      string              `json:"chain_id"`
	TxCount      int                 `json:"tx_count"`
	Txs          []string            `json:"txs"`
	StateRoots   []StateRootResponse `json:"state_roots"`
	TimeUnixNano int64               `json:"time_unix_nano"`
}

type StateRootResponse struct {
	Height    uint64 `json:"height"`
	Namespace string `json:"namespace"`
	Root      string `json:"root"`
}

func NewServer(provider StatusProvider, cfg Config) *Server {
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	handler := NewHandlerWithConfig(provider, cfg)
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
	return NewHandlerWithConfig(provider, Config{})
}

func NewHandlerWithConfig(provider StatusProvider, cfg Config) http.Handler {
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = defaultMaxRequestBytes
	}
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
	mux.HandleFunc("/tx", func(writer http.ResponseWriter, request *http.Request) {
		if !allowPost(writer, request) {
			return
		}
		submitter, ok := provider.(TxSubmitter)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "transaction submission is unavailable")
			return
		}
		tx, err := decodeSubmitTxRequest(writer, request, cfg.MaxRequestBytes)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if err := submitter.SubmitTx(request.Context(), tx); err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		hash := mempool.HashTx(tx)
		writeJSON(writer, http.StatusAccepted, SubmitTxResponse{
			Accepted: true,
			TxHash:   hex.EncodeToString(hash[:]),
		})
	})
	mux.HandleFunc("/blocks/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		blockProvider, ok := provider.(BlockProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "block query is unavailable")
			return
		}
		selector := strings.TrimPrefix(request.URL.Path, "/blocks/")
		if selector == "" {
			writeError(writer, http.StatusNotFound, "block selector is required")
			return
		}
		var (
			record store.BlockRecord
			err    error
		)
		if selector == "latest" {
			record, err = blockProvider.LatestBlock(request.Context())
		} else {
			height, parseErr := strconv.ParseUint(selector, 10, 64)
			if parseErr != nil || height == 0 {
				writeError(writer, http.StatusBadRequest, "invalid block height")
				return
			}
			record, err = blockProvider.BlockByHeight(request.Context(), types.Height(height))
		}
		if errors.Is(err, store.ErrBlockNotFound) || errors.Is(err, store.ErrBlockIndexNotFound) {
			writeError(writer, http.StatusNotFound, "block not found")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, blockResponse(record))
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

func allowPost(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodPost {
		return true
	}
	writer.Header().Set("Allow", http.MethodPost)
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

func decodeSubmitTxRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (types.Tx, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload SubmitTxRequest
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}
	if payload.Tx == "" {
		return nil, errors.New("transaction is required")
	}
	encoding := payload.Encoding
	if encoding == "" {
		encoding = "base64"
	}
	var tx []byte
	switch encoding {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(payload.Tx)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 transaction: %w", err)
		}
		tx = decoded
	case "plain":
		tx = []byte(payload.Tx)
	default:
		return nil, fmt.Errorf("unsupported transaction encoding %q", payload.Encoding)
	}
	if len(tx) == 0 {
		return nil, errors.New("transaction is empty")
	}
	return types.Tx(tx), nil
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

func writeError(writer http.ResponseWriter, statusCode int, message string) {
	writeJSON(writer, statusCode, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

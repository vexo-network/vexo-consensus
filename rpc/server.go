package rpc

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const defaultReadHeaderTimeout = 5 * time.Second
const defaultMaxRequestBytes = 1024 * 1024

type StatusProvider interface {
	Status(ctx context.Context) node.Status
}

type MetricsProvider interface {
	Metrics(ctx context.Context) (node.Metrics, error)
}

type SnapshotProvider interface {
	StateSnapshot(ctx context.Context) (node.StateSnapshot, error)
}

type TxSubmitter interface {
	SubmitTx(ctx context.Context, tx types.Tx) error
}

type EvidenceSubmitter interface {
	SubmitEvidence(ctx context.Context, evidence slashing.Evidence) (consensus.SlashResult, bool, error)
}

type BlockProvider interface {
	BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error)
	LatestBlock(ctx context.Context) (store.BlockRecord, error)
}

type ChainQueryProvider interface {
	BlockProvider
	BlockIndex(ctx context.Context) (store.BlockIndex, error)
	LatestState(ctx context.Context) (store.StateRecord, error)
	StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error)
}

type PruneProvider interface {
	PruneBelow(ctx context.Context, retainFrom types.Height) (store.PruneResult, error)
}

type ReplayProvider interface {
	Replay(ctx context.Context, from types.Height, to types.Height) (vexoruntime.ReplayResult, error)
	ReplayAll(ctx context.Context) (vexoruntime.ReplayResult, error)
}

type ConsensusLoopController interface {
	StartConsensusLoop(ctx context.Context, cfg node.ConsensusLoopConfig) error
	StopConsensusLoop(ctx context.Context) error
	ConsensusLoopRunning() bool
}

type ValidatorQueryProvider interface {
	ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error)
	Committee(ctx context.Context, height types.Height, round types.Round, seed types.Hash) (committee.Committee, error)
}

type Config struct {
	Address              string
	ReadHeaderTimeout    time.Duration
	RequestTimeout       time.Duration
	MaxRequestBytes      int64
	RateLimitWindow      time.Duration
	RateLimitMaxRequests int
	AdminToken           string
	EnablePprof          bool
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

type MetricsResponse struct {
	ChainID              string `json:"chain_id"`
	Running              bool   `json:"running"`
	DataDir              string `json:"data_dir"`
	LatestHeight         uint64 `json:"latest_height"`
	LatestAppHash        string `json:"latest_app_hash"`
	EarliestBlockHeight  uint64 `json:"earliest_block_height"`
	LatestBlockHeight    uint64 `json:"latest_block_height"`
	TotalBlocks          uint64 `json:"total_blocks"`
	ValidatorCount       int    `json:"validator_count"`
	TotalVotingPower     uint64 `json:"total_voting_power"`
	ValidatorSetHash     string `json:"validator_set_hash"`
	PeerCount            int    `json:"peer_count"`
	BannedPeers          int    `json:"banned_peers"`
	PeerWindowMessages   uint64 `json:"peer_window_messages"`
	ConsensusLoopRunning bool   `json:"consensus_loop_running"`
}

type DiagnosticsResponse struct {
	OK      bool                      `json:"ok"`
	Status  string                    `json:"status"`
	Checks  []DiagnosticCheckResponse `json:"checks"`
	Node    StatusResponse            `json:"node"`
	Metrics *MetricsResponse          `json:"metrics,omitempty"`
	Storage *BlockIndexResponse       `json:"storage,omitempty"`
	Peers   []PeerResponse            `json:"peers"`
}

type DiagnosticCheckResponse struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
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

type SubmitEvidenceRequest struct {
	Type      string `json:"type"`
	Validator string `json:"validator"`
	Height    uint64 `json:"height"`
	Round     uint64 `json:"round,omitempty"`
	Proof     string `json:"proof"`
	Encoding  string `json:"encoding,omitempty"`
}

type SubmitEvidenceResponse struct {
	Accepted       bool            `json:"accepted"`
	Applied        bool            `json:"applied"`
	Type           string          `json:"type"`
	Validator      string          `json:"validator"`
	Height         uint64          `json:"height"`
	Round          uint64          `json:"round"`
	PreviousPower  uint64          `json:"previous_power,omitempty"`
	RemainingPower uint64          `json:"remaining_power,omitempty"`
	Penalty        PenaltyResponse `json:"penalty,omitempty"`
}

type PenaltyResponse struct {
	SlashFraction string `json:"slash_fraction,omitempty"`
	JailDuration  uint64 `json:"jail_duration,omitempty"`
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

type BlockIndexResponse struct {
	EarliestHeight uint64 `json:"earliest_height"`
	LatestHeight   uint64 `json:"latest_height"`
	TotalBlocks    uint64 `json:"total_blocks"`
}

type StateResponse struct {
	Height           uint64 `json:"height"`
	AppHash          string `json:"app_hash"`
	LastBlockHash    string `json:"last_block_hash"`
	ValidatorSetHash string `json:"validator_set_hash"`
}

type StateSnapshotResponse struct {
	Height           uint64              `json:"height"`
	AppHash          string              `json:"app_hash"`
	LastBlockHash    string              `json:"last_block_hash"`
	ValidatorSetHash string              `json:"validator_set_hash"`
	StateRoots       []StateRootResponse `json:"state_roots"`
}

type PruneRequest struct {
	RetainFromHeight uint64 `json:"retain_from_height"`
}

type PruneResponse struct {
	RetainFromHeight uint64 `json:"retain_from_height"`
	PrunedBlocks     uint64 `json:"pruned_blocks"`
	PrunedStateRoots uint64 `json:"pruned_state_roots"`
}

type ReplayRequest struct {
	All        bool   `json:"all,omitempty"`
	FromHeight uint64 `json:"from_height,omitempty"`
	ToHeight   uint64 `json:"to_height,omitempty"`
}

type ReplayResponse struct {
	FromHeight uint64 `json:"from_height"`
	ToHeight   uint64 `json:"to_height"`
	LastHash   string `json:"last_hash"`
	Blocks     uint64 `json:"blocks"`
}

type ConsensusLoopRequest struct {
	IntervalMillis     uint64 `json:"interval_millis,omitempty"`
	RoundTimeoutMillis uint64 `json:"round_timeout_millis,omitempty"`
	MaxBlockBytes      int64  `json:"max_block_bytes,omitempty"`
}

type ConsensusLoopResponse struct {
	Running            bool   `json:"running"`
	Action             string `json:"action"`
	IntervalMillis     uint64 `json:"interval_millis,omitempty"`
	RoundTimeoutMillis uint64 `json:"round_timeout_millis,omitempty"`
	MaxBlockBytes      int64  `json:"max_block_bytes,omitempty"`
}

type ValidatorSetResponse struct {
	Height           uint64              `json:"height"`
	TotalValidators  int                 `json:"total_validators"`
	TotalPower       uint64              `json:"total_power"`
	ValidatorSetHash string              `json:"validator_set_hash"`
	Validators       []ValidatorResponse `json:"validators"`
}

type ValidatorResponse struct {
	ID          string            `json:"id"`
	Address     string            `json:"address"`
	VotingPower uint64            `json:"voting_power"`
	Stake       uint64            `json:"stake"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type CommitteeResponse struct {
	Height  uint64                    `json:"height"`
	Epoch   uint64                    `json:"epoch"`
	Round   uint64                    `json:"round"`
	Seed    string                    `json:"seed"`
	Members []CommitteeMemberResponse `json:"members"`
}

type CommitteeMemberResponse struct {
	Validator ValidatorResponse `json:"validator"`
	Weight    uint64            `json:"weight"`
	Proof     string            `json:"proof,omitempty"`
}

type rateBucket struct {
	WindowStart time.Time
	Count       int
}

type rateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	window  time.Duration
	max     int
	buckets map[string]rateBucket
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
	if cfg.EnablePprof {
		registerPprofHandlers(mux)
	}
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
	mux.HandleFunc("/diagnostics", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		diagnostics := diagnosticsResponse(request.Context(), provider)
		statusCode := http.StatusOK
		if !diagnostics.OK {
			statusCode = http.StatusServiceUnavailable
		}
		writeJSON(writer, statusCode, diagnostics)
	})
	mux.HandleFunc("/metrics/text", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		metricsProvider, ok := provider.(MetricsProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "metrics query is unavailable")
			return
		}
		metrics, err := metricsProvider.Metrics(request.Context())
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeText(writer, http.StatusOK, metricsText(metrics))
	})
	mux.HandleFunc("/metrics", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		metricsProvider, ok := provider.(MetricsProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "metrics query is unavailable")
			return
		}
		metrics, err := metricsProvider.Metrics(request.Context())
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, metricsResponse(metrics))
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
	mux.HandleFunc("/evidence", func(writer http.ResponseWriter, request *http.Request) {
		if !allowPost(writer, request) {
			return
		}
		submitter, ok := provider.(EvidenceSubmitter)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "evidence submission is unavailable")
			return
		}
		evidence, err := decodeSubmitEvidenceRequest(writer, request, cfg.MaxRequestBytes)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		result, applied, err := submitter.SubmitEvidence(request.Context(), evidence)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, evidenceResponse(evidence, result, applied))
	})
	mux.HandleFunc("/blocks", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(ChainQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "block index query is unavailable")
			return
		}
		index, err := queryProvider.BlockIndex(request.Context())
		if errors.Is(err, store.ErrBlockIndexNotFound) {
			writeError(writer, http.StatusNotFound, "block index not found")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, blockIndexResponse(index))
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
	mux.HandleFunc("/state/latest", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(ChainQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "state query is unavailable")
			return
		}
		state, err := queryProvider.LatestState(request.Context())
		if errors.Is(err, store.ErrStateNotFound) {
			writeError(writer, http.StatusNotFound, "state not found")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, stateResponse(state))
	})
	mux.HandleFunc("/snapshot/latest", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		snapshotProvider, ok := provider.(SnapshotProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "snapshot query is unavailable")
			return
		}
		snapshot, err := snapshotProvider.StateSnapshot(request.Context())
		if errors.Is(err, store.ErrStateNotFound) {
			writeError(writer, http.StatusNotFound, "snapshot not found")
			return
		}
		if errors.Is(err, store.ErrStateRootNotFound) {
			writeError(writer, http.StatusNotFound, "snapshot state root not found")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, stateSnapshotResponse(snapshot))
	})
	mux.HandleFunc("/state/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(ChainQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "state root query is unavailable")
			return
		}
		height, namespace, ok := parseStateRootPath(request.URL.Path)
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid state root path")
			return
		}
		root, err := queryProvider.StateRoot(request.Context(), height, namespace)
		if errors.Is(err, store.ErrStateRootNotFound) {
			writeError(writer, http.StatusNotFound, "state root not found")
			return
		}
		if errors.Is(err, store.ErrInvalidStateRoot) {
			writeError(writer, http.StatusBadRequest, "invalid state root request")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, stateRootResponse(root))
	})
	mux.HandleFunc("/prune", func(writer http.ResponseWriter, request *http.Request) {
		if !allowPost(writer, request) {
			return
		}
		if !allowAdmin(writer, request, cfg.AdminToken) {
			return
		}
		pruner, ok := provider.(PruneProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "prune is unavailable")
			return
		}
		retainFrom, err := decodePruneRequest(writer, request, cfg.MaxRequestBytes)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		result, err := pruner.PruneBelow(request.Context(), retainFrom)
		if errors.Is(err, store.ErrInvalidPruneHeight) {
			writeError(writer, http.StatusBadRequest, "invalid prune height")
			return
		}
		if errors.Is(err, store.ErrBlockIndexNotFound) {
			writeError(writer, http.StatusNotFound, "block index not found")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, pruneResponse(result))
	})
	mux.HandleFunc("/replay", func(writer http.ResponseWriter, request *http.Request) {
		if !allowPost(writer, request) {
			return
		}
		if !allowAdmin(writer, request, cfg.AdminToken) {
			return
		}
		replayer, ok := provider.(ReplayProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "replay is unavailable")
			return
		}
		payload, err := decodeReplayRequest(writer, request, cfg.MaxRequestBytes)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		var result vexoruntime.ReplayResult
		if payload.All || (payload.FromHeight == 0 && payload.ToHeight == 0) {
			result, err = replayer.ReplayAll(request.Context())
		} else {
			if payload.FromHeight == 0 || payload.ToHeight == 0 {
				writeError(writer, http.StatusBadRequest, "from_height and to_height are required")
				return
			}
			result, err = replayer.Replay(request.Context(), types.Height(payload.FromHeight), types.Height(payload.ToHeight))
		}
		if errors.Is(err, vexoruntime.ErrInvalidReplayRange) {
			writeError(writer, http.StatusBadRequest, "invalid replay range")
			return
		}
		if errors.Is(err, store.ErrBlockNotFound) || errors.Is(err, store.ErrBlockIndexNotFound) {
			writeError(writer, http.StatusNotFound, "replay data not found")
			return
		}
		if errors.Is(err, vexoruntime.ErrReplayAppHashMismatch) {
			writeError(writer, http.StatusConflict, "replay app hash mismatch")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, replayResponse(result))
	})
	mux.HandleFunc("/consensus/start", func(writer http.ResponseWriter, request *http.Request) {
		if !allowPost(writer, request) {
			return
		}
		if !allowAdmin(writer, request, cfg.AdminToken) {
			return
		}
		controller, ok := provider.(ConsensusLoopController)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "consensus loop control is unavailable")
			return
		}
		loopConfig, err := decodeConsensusLoopRequest(writer, request, cfg.MaxRequestBytes)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if err := controller.StartConsensusLoop(request.Context(), loopConfig); err != nil {
			writeConsensusLoopError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, consensusLoopResponse("start", true, loopConfig))
	})
	mux.HandleFunc("/consensus/stop", func(writer http.ResponseWriter, request *http.Request) {
		if !allowPost(writer, request) {
			return
		}
		if !allowAdmin(writer, request, cfg.AdminToken) {
			return
		}
		controller, ok := provider.(ConsensusLoopController)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "consensus loop control is unavailable")
			return
		}
		if err := controller.StopConsensusLoop(request.Context()); err != nil {
			writeConsensusLoopError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, consensusLoopResponse("stop", controller.ConsensusLoopRunning(), node.ConsensusLoopConfig{}))
	})
	mux.HandleFunc("/validators/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(ValidatorQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "validator query is unavailable")
			return
		}
		height, ok := parseHeightSelector(request.URL.Path, "/validators/")
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid validator set height")
			return
		}
		validatorSet, err := queryProvider.ValidatorSet(request.Context(), height)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, validatorSetResponse(height, validatorSet))
	})
	mux.HandleFunc("/committee/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(ValidatorQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "committee query is unavailable")
			return
		}
		height, round, ok := parseHeightRoundSelector(request.URL.Path, "/committee/")
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid committee selector")
			return
		}
		seed, ok := parseSeed(request.URL.Query().Get("seed"))
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid committee seed")
			return
		}
		committeeResult, err := queryProvider.Committee(request.Context(), height, round, seed)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, committeeResponse(height, seed, committeeResult))
	})
	return applyMiddleware(mux, cfg)
}

func registerPprofHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func applyMiddleware(handler http.Handler, cfg Config) http.Handler {
	if cfg.RequestTimeout > 0 {
		handler = requestTimeout(handler, cfg.RequestTimeout)
	}
	if cfg.RateLimitMaxRequests > 0 {
		window := cfg.RateLimitWindow
		if window <= 0 {
			window = time.Second
		}
		handler = newRateLimiter(window, cfg.RateLimitMaxRequests).Handler(handler)
	}
	return handler
}

func requestTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), timeout)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func newRateLimiter(window time.Duration, maxRequests int) *rateLimiter {
	return &rateLimiter{
		now:     time.Now,
		window:  window,
		max:     maxRequests,
		buckets: make(map[string]rateBucket),
	}
}

func (limiter *rateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !limiter.Allow(request) {
			writeJSON(writer, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (limiter *rateLimiter) Allow(request *http.Request) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.now()
	key := clientKey(request)
	bucket := limiter.buckets[key]
	if bucket.WindowStart.IsZero() || now.Sub(bucket.WindowStart) >= limiter.window {
		limiter.buckets[key] = rateBucket{WindowStart: now, Count: 1}
		return true
	}
	if bucket.Count >= limiter.max {
		return false
	}
	bucket.Count++
	limiter.buckets[key] = bucket
	return true
}

func clientKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if request.RemoteAddr != "" {
		return request.RemoteAddr
	}
	return "unknown"
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

func allowAdmin(writer http.ResponseWriter, request *http.Request, adminToken string) bool {
	if adminToken == "" {
		return true
	}
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "admin authorization is required"})
		return false
	}
	token := strings.TrimPrefix(header, prefix)
	if subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "admin authorization is invalid"})
		return false
	}
	return true
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

func metricsResponse(metrics node.Metrics) MetricsResponse {
	return MetricsResponse{
		ChainID:              metrics.ChainID,
		Running:              metrics.Running,
		DataDir:              metrics.DataDir,
		LatestHeight:         uint64(metrics.LatestHeight),
		LatestAppHash:        hex.EncodeToString(metrics.LatestAppHash[:]),
		EarliestBlockHeight:  uint64(metrics.EarliestBlockHeight),
		LatestBlockHeight:    uint64(metrics.LatestBlockHeight),
		TotalBlocks:          metrics.TotalBlocks,
		ValidatorCount:       metrics.ValidatorCount,
		TotalVotingPower:     metrics.TotalVotingPower,
		ValidatorSetHash:     hex.EncodeToString(metrics.ValidatorSetHash[:]),
		PeerCount:            metrics.PeerCount,
		BannedPeers:          metrics.BannedPeers,
		PeerWindowMessages:   metrics.PeerWindowMessages,
		ConsensusLoopRunning: metrics.ConsensusLoopRunning,
	}
}

func diagnosticsResponse(ctx context.Context, provider StatusProvider) DiagnosticsResponse {
	status := provider.Status(ctx)
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
	writeGauge("vexo_node_running", "Whether the node is running.", boolGauge(metrics.Running))
	writeGauge("vexo_latest_height", "Latest committed application height.", uint64(metrics.LatestHeight))
	writeGauge("vexo_earliest_block_height", "Earliest locally stored block height.", uint64(metrics.EarliestBlockHeight))
	writeGauge("vexo_latest_block_height", "Latest locally stored block height.", uint64(metrics.LatestBlockHeight))
	writeGauge("vexo_total_blocks", "Total locally stored blocks.", metrics.TotalBlocks)
	writeGauge("vexo_validator_count", "Current validator count.", uint64(metrics.ValidatorCount))
	writeGauge("vexo_total_voting_power", "Current total validator voting power.", metrics.TotalVotingPower)
	writeGauge("vexo_peer_count", "Known peer count.", uint64(metrics.PeerCount))
	writeGauge("vexo_banned_peers", "Banned peer count.", uint64(metrics.BannedPeers))
	writeGauge("vexo_peer_window_messages", "Peer messages observed in the current score window.", metrics.PeerWindowMessages)
	writeGauge("vexo_consensus_loop_running", "Whether the local consensus loop is running.", boolGauge(metrics.ConsensusLoopRunning))
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

func decodeSubmitEvidenceRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (slashing.Evidence, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload SubmitEvidenceRequest
	if err := decoder.Decode(&payload); err != nil {
		return slashing.Evidence{}, fmt.Errorf("invalid evidence request: %w", err)
	}
	if payload.Type == "" {
		return slashing.Evidence{}, errors.New("evidence type is required")
	}
	if payload.Validator == "" {
		return slashing.Evidence{}, errors.New("evidence validator is required")
	}
	if payload.Height == 0 {
		return slashing.Evidence{}, errors.New("evidence height is required")
	}
	if payload.Proof == "" {
		return slashing.Evidence{}, errors.New("evidence proof is required")
	}
	encoding := payload.Encoding
	if encoding == "" {
		encoding = "base64"
	}
	var proof []byte
	switch encoding {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(payload.Proof)
		if err != nil {
			return slashing.Evidence{}, fmt.Errorf("invalid base64 evidence proof: %w", err)
		}
		proof = decoded
	case "plain":
		proof = []byte(payload.Proof)
	default:
		return slashing.Evidence{}, fmt.Errorf("unsupported evidence proof encoding %q", payload.Encoding)
	}
	if len(proof) == 0 {
		return slashing.Evidence{}, errors.New("evidence proof is empty")
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceType(payload.Type),
		Validator: types.ValidatorID(payload.Validator),
		Height:    types.Height(payload.Height),
		Round:     types.Round(payload.Round),
		Proof:     proof,
	}, nil
}

func decodePruneRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (types.Height, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload PruneRequest
	if err := decoder.Decode(&payload); err != nil {
		return 0, fmt.Errorf("invalid prune request: %w", err)
	}
	if payload.RetainFromHeight == 0 {
		return 0, errors.New("retain_from_height is required")
	}
	return types.Height(payload.RetainFromHeight), nil
}

func decodeReplayRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (ReplayRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload ReplayRequest
	if err := decoder.Decode(&payload); err != nil {
		return ReplayRequest{}, fmt.Errorf("invalid replay request: %w", err)
	}
	if payload.All && (payload.FromHeight != 0 || payload.ToHeight != 0) {
		return ReplayRequest{}, errors.New("all cannot be combined with from_height or to_height")
	}
	return payload, nil
}

func decodeConsensusLoopRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (node.ConsensusLoopConfig, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload ConsensusLoopRequest
	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return node.ConsensusLoopConfig{}, nil
		}
		return node.ConsensusLoopConfig{}, fmt.Errorf("invalid consensus loop request: %w", err)
	}
	return node.ConsensusLoopConfig{
		Interval:      time.Duration(payload.IntervalMillis) * time.Millisecond,
		RoundTimeout:  time.Duration(payload.RoundTimeoutMillis) * time.Millisecond,
		MaxBlockBytes: payload.MaxBlockBytes,
	}, nil
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

func stateRootResponse(root store.StateRootRecord) StateRootResponse {
	return StateRootResponse{
		Height:    uint64(root.Height),
		Namespace: root.Namespace,
		Root:      hex.EncodeToString(root.Root[:]),
	}
}

func pruneResponse(result store.PruneResult) PruneResponse {
	return PruneResponse{
		RetainFromHeight: uint64(result.RetainFromHeight),
		PrunedBlocks:     result.PrunedBlocks,
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
		totalPower += uint64(validatorInfo.VotingPower)
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

func parseStateRootPath(path string) (types.Height, string, bool) {
	selector := strings.TrimPrefix(path, "/state/")
	parts := strings.Split(selector, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", false
	}
	height, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || height == 0 {
		return 0, "", false
	}
	return types.Height(height), parts[1], true
}

func parseHeightSelector(path string, prefix string) (types.Height, bool) {
	selector := strings.TrimPrefix(path, prefix)
	height, err := strconv.ParseUint(selector, 10, 64)
	if err != nil || height == 0 {
		return 0, false
	}
	return types.Height(height), true
}

func parseHeightRoundSelector(path string, prefix string) (types.Height, types.Round, bool) {
	selector := strings.TrimPrefix(path, prefix)
	parts := strings.Split(selector, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, false
	}
	height, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || height == 0 {
		return 0, 0, false
	}
	round, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return types.Height(height), types.Round(round), true
}

func parseSeed(value string) (types.Hash, bool) {
	if value == "" {
		return types.Hash{}, true
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(types.Hash{}) {
		return types.Hash{}, false
	}
	var seed types.Hash
	copy(seed[:], decoded)
	return seed, true
}

func writeError(writer http.ResponseWriter, statusCode int, message string) {
	writeJSON(writer, statusCode, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeText(writer http.ResponseWriter, statusCode int, value string) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(statusCode)
	_, _ = writer.Write([]byte(value))
}

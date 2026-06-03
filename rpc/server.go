package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/node"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const defaultReadHeaderTimeout = 5 * time.Second
const defaultMaxRequestBytes = 1024 * 1024
const stableAPIPrefix = "/v1"

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
	StartedAtUnix int64  `json:"started_at_unix,omitempty"`
	LatestHeight  uint64 `json:"latest_height"`
	LatestAppHash string `json:"latest_app_hash"`
	DataDir       string `json:"data_dir"`
	PeerCount     int    `json:"peer_count"`
	BannedPeers   int    `json:"banned_peers"`
}

type MetricsResponse struct {
	ChainID              string  `json:"chain_id"`
	Running              bool    `json:"running"`
	StartedAtUnix        int64   `json:"started_at_unix,omitempty"`
	UptimeSeconds        uint64  `json:"uptime_seconds"`
	DataDir              string  `json:"data_dir"`
	LatestHeight         uint64  `json:"latest_height"`
	LatestAppHash        string  `json:"latest_app_hash"`
	EarliestBlockHeight  uint64  `json:"earliest_block_height"`
	LatestBlockHeight    uint64  `json:"latest_block_height"`
	TotalBlocks          uint64  `json:"total_blocks"`
	ValidatorCount       int     `json:"validator_count"`
	TotalVotingPower     uint64  `json:"total_voting_power"`
	ValidatorSetHash     string  `json:"validator_set_hash"`
	PeerCount            int     `json:"peer_count"`
	BannedPeers          int     `json:"banned_peers"`
	PeerWindowMessages   uint64  `json:"peer_window_messages"`
	ConsensusLoopRunning bool    `json:"consensus_loop_running"`
	HeightRatePerMinute  float64 `json:"height_rate_per_minute"`
	RoundTimeouts        uint64  `json:"round_timeouts"`
	ProposalLatencyNanos uint64  `json:"proposal_latency_nanos"`
	VoteLatencyNanos     uint64  `json:"vote_latency_nanos"`
	MempoolSize          uint64  `json:"mempool_size"`
	CommitLatencyNanos   uint64  `json:"commit_latency_nanos"`
	SnapshotHealthy      bool    `json:"snapshot_healthy"`
	ReplayHealthy        bool    `json:"replay_healthy"`
	SigningFailures      uint64  `json:"validator_signing_failures"`
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

type SnapshotExportResponse struct {
	SchemaVersion string                  `json:"schema_version"`
	State         store.StateRecord       `json:"state"`
	StateRoots    []store.StateRootRecord `json:"state_roots"`
	KV            []store.KVPair          `json:"kv,omitempty"`
}

type RecoveryReportResponse struct {
	OK                bool                    `json:"ok"`
	Running           bool                    `json:"running"`
	LatestHeight      uint64                  `json:"latest_height"`
	LatestStateHeight uint64                  `json:"latest_state_height"`
	SafeHeight        uint64                  `json:"safe_height"`
	EarliestBlock     uint64                  `json:"earliest_block"`
	LatestBlock       uint64                  `json:"latest_block"`
	TotalBlocks       uint64                  `json:"total_blocks"`
	SnapshotAvailable bool                    `json:"snapshot_available"`
	Repaired          bool                    `json:"repaired"`
	RecoverResult     *RecoverIndexesResponse `json:"recover_result,omitempty"`
	Problems          []string                `json:"problems,omitempty"`
}

type RecoverIndexesResponse struct {
	BlockIndexKeys   uint64 `json:"block_index_keys"`
	EvidenceKeys     uint64 `json:"evidence_keys"`
	EarliestHeight   uint64 `json:"earliest_height"`
	LatestHeight     uint64 `json:"latest_height"`
	RecoveredIndexes uint64 `json:"recovered_indexes"`
}

type PruneRequest struct {
	RetainFromHeight uint64 `json:"retain_from_height"`
}

type PruneResponse struct {
	RetainFromHeight uint64 `json:"retain_from_height"`
	PrunedBlocks     uint64 `json:"pruned_blocks"`
	PrunedStates     uint64 `json:"pruned_states"`
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
	mux.HandleFunc("/snapshot/export", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		snapshotProvider, ok := provider.(SnapshotProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "snapshot export is unavailable")
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
		writeJSON(writer, http.StatusOK, snapshotExportResponse(snapshot))
	})
	mux.HandleFunc("/recovery", func(writer http.ResponseWriter, request *http.Request) {
		recoveryProvider, ok := provider.(RecoveryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "recovery report is unavailable")
			return
		}
		switch request.Method {
		case http.MethodGet:
			report, err := recoveryProvider.RecoveryReport(request.Context(), false)
			writeRecoveryReport(writer, report, err)
		case http.MethodPost:
			if !allowAdmin(writer, request, cfg.AdminToken) {
				return
			}
			report, err := recoveryProvider.RecoveryReport(request.Context(), true)
			writeRecoveryReport(writer, report, err)
		default:
			writer.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
			writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
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
	return applyMiddleware(versionedHandler(mux), cfg)
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

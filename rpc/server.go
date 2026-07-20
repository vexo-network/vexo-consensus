package rpc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gethaccounts "github.com/ethereum/go-ethereum/accounts"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/mempool"
	evmmodule "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"golang.org/x/crypto/sha3"
)

const defaultReadHeaderTimeout = 5 * time.Second
const defaultMaxRequestBytes = 1024 * 1024
const defaultMaxWeb3Filters = 1024
const defaultMaxWeb3LogResults = 10_000
const defaultMaxWeb3LogBlockRange = 10_000
const defaultWeb3BlockGasLimit = 10_000_000
const stableAPIPrefix = "/v1"

type Config struct {
	Address                       string
	ReadHeaderTimeout             time.Duration
	RequestTimeout                time.Duration
	MaxRequestBytes               int64
	RateLimitWindow               time.Duration
	RateLimitMaxRequests          int
	AdminToken                    string
	AdminTokens                   map[string][]string
	AdminAuditSink                func(AdminAuditEvent)
	CORSAllowedOrigins            []string
	TLSConfig                     *tls.Config
	EnablePprof                   bool
	AllowUnprotectedLegacyTx      bool
	EVMChainConfigJSON            string
	StrictEVMStateRoot            bool
	EnableEVMManagedAccounts      bool
	EVMAccountPrivateKeys         []string
	Web3SubscriptionInterval      time.Duration
	Web3SubscriptionMaxCatchUp    uint64
	Web3SubscriptionMaxLogBatch   int
	Web3SubscriptionMaxPendingRun int
	Web3SubscriptionMaxPerConn    int
	Web3SubscriptionIdleTimeout   time.Duration
	Web3LogMaxResults             int
	Web3LogMaxBlockRange          uint64
	Web3FilterSnapshotPath        string
	RequiredCapabilities          []string
	RequireAllCapabilities        bool
}

type AdminAuditEvent struct {
	Scope      string
	Path       string
	Method     string
	RemoteAddr string
	Authorized bool
	Reason     string
	At         time.Time
}

type Server struct {
	provider               StatusProvider
	server                 *http.Server
	filterStore            *web3FilterStore
	web3FilterSnapshotPath string
	startupErr             error
}

type HealthResponse struct {
	OK bool `json:"ok"`
}

type StatusResponse struct {
	ChainID               string  `json:"chain_id"`
	EVMChainID            uint64  `json:"evm_chain_id,omitempty"`
	Running               bool    `json:"running"`
	StartedAtUnix         int64   `json:"started_at_unix,omitempty"`
	LatestHeight          uint64  `json:"latest_height"`
	LatestAppHash         string  `json:"latest_app_hash"`
	LatestFinalizedHeight uint64  `json:"latest_finalized_height,omitempty"`
	LatestFinalizedHash   string  `json:"latest_finalized_hash,omitempty"`
	DataDir               string  `json:"data_dir"`
	PeerCount             int     `json:"peer_count"`
	ActivePeerCount       int     `json:"active_peer_count"`
	ConfiguredPeerCount   int     `json:"configured_peer_count"`
	ScoredPeerCount       int     `json:"scored_peer_count"`
	BannedPeers           int     `json:"banned_peers"`
	QuorumHealthRatio     float64 `json:"quorum_health_ratio"`
}

type MetricsResponse struct {
	ChainID                     string  `json:"chain_id"`
	Running                     bool    `json:"running"`
	StartedAtUnix               int64   `json:"started_at_unix,omitempty"`
	UptimeSeconds               uint64  `json:"uptime_seconds"`
	DataDir                     string  `json:"data_dir"`
	AdaptiveRoundTimeoutEnabled bool    `json:"adaptive_round_timeout_enabled"`
	RecoveryFinalityGateEnabled bool    `json:"recovery_finality_gate_enabled"`
	LatestHeight                uint64  `json:"latest_height"`
	LatestAppHash               string  `json:"latest_app_hash"`
	EarliestBlockHeight         uint64  `json:"earliest_block_height"`
	LatestBlockHeight           uint64  `json:"latest_block_height"`
	TotalBlocks                 uint64  `json:"total_blocks"`
	ValidatorCount              int     `json:"validator_count"`
	TotalVotingPower            uint64  `json:"total_voting_power"`
	ValidatorSetHash            string  `json:"validator_set_hash"`
	PeerCount                   int     `json:"peer_count"`
	ActivePeerCount             int     `json:"active_peer_count"`
	ConfiguredPeerCount         int     `json:"configured_peer_count"`
	ScoredPeerCount             int     `json:"scored_peer_count"`
	BannedPeers                 int     `json:"banned_peers"`
	QuorumHealthRatio           float64 `json:"quorum_health_ratio"`
	PeerWindowMessages          uint64  `json:"peer_window_messages"`
	ConsensusLoopRunning        bool    `json:"consensus_loop_running"`
	HeightRatePerMinute         float64 `json:"height_rate_per_minute"`
	RoundTimeouts               uint64  `json:"round_timeouts"`
	AdaptiveRoundTimeoutNanos   uint64  `json:"adaptive_round_timeout_nanos"`
	RecoveryFinalityDeferrals   uint64  `json:"recovery_finality_deferrals"`
	ProposalLatencyNanos        uint64  `json:"proposal_latency_nanos"`
	ProposalLatencyP95Nanos     uint64  `json:"proposal_latency_p95_nanos"`
	ProposalLatencyP99Nanos     uint64  `json:"proposal_latency_p99_nanos"`
	VoteLatencyNanos            uint64  `json:"vote_latency_nanos"`
	VoteLatencyP95Nanos         uint64  `json:"vote_latency_p95_nanos"`
	VoteLatencyP99Nanos         uint64  `json:"vote_latency_p99_nanos"`
	MempoolSize                 uint64  `json:"mempool_size"`
	CommitLatencyNanos          uint64  `json:"commit_latency_nanos"`
	CommitLatencyP95Nanos       uint64  `json:"commit_latency_p95_nanos"`
	CommitLatencyP99Nanos       uint64  `json:"commit_latency_p99_nanos"`
	SnapshotHealthy             bool    `json:"snapshot_healthy"`
	ReplayHealthy               bool    `json:"replay_healthy"`
	SigningFailures             uint64  `json:"validator_signing_failures"`
	ReconciliationFailures      uint64  `json:"post_commit_reconciliation_failures"`
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

type CapabilityResponse struct {
	Complete     bool                 `json:"complete"`
	Capabilities []CapabilitySnapshot `json:"capabilities"`
	Missing      []string             `json:"missing,omitempty"`
}

type CapabilitySnapshot struct {
	Name        string `json:"name"`
	Available   bool   `json:"available"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
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

type HeaderResponse struct {
	ChainID           string `json:"chain_id"`
	Height            uint64 `json:"height"`
	TimeUnixNano      int64  `json:"time_unix_nano"`
	PreviousBlockHash string `json:"previous_block_hash"`
	AppHash           string `json:"app_hash"`
	ValidatorSetHash  string `json:"validator_set_hash"`
	ConsensusHash     string `json:"consensus_hash"`
}

type QuorumCertResponse struct {
	Height      uint64 `json:"height"`
	Round       uint64 `json:"round"`
	BlockHash   string `json:"block_hash"`
	Signers     string `json:"signers"`
	Signature   string `json:"signature"`
	VotingPower uint64 `json:"voting_power"`
}

type FinalityProofResponse struct {
	Height             uint64               `json:"height"`
	BlockHash          string               `json:"block_hash"`
	ValidatorSetHeight uint64               `json:"validator_set_height"`
	ValidatorSetHash   string               `json:"validator_set_hash"`
	Strict             bool                 `json:"strict"`
	Header             HeaderResponse       `json:"header"`
	QuorumCert         QuorumCertResponse   `json:"quorum_cert"`
	CommitChain        []CommitLinkResponse `json:"commit_chain,omitempty"`
}

type CommitLinkResponse struct {
	Header     HeaderResponse     `json:"header"`
	BlockHash  string             `json:"block_hash"`
	QuorumCert QuorumCertResponse `json:"quorum_cert"`
}

type EventAttributeResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Index bool   `json:"index,omitempty"`
}

type EventResponse struct {
	Type       string                   `json:"type"`
	Attributes []EventAttributeResponse `json:"attributes,omitempty"`
}

type EventRecordResponse struct {
	Height  uint64        `json:"height"`
	TxIndex int           `json:"tx_index"`
	Event   EventResponse `json:"event"`
}

type EventsResponse struct {
	Key     string                `json:"key"`
	Value   string                `json:"value"`
	Records []EventRecordResponse `json:"records"`
}

type QueryProofResponse struct {
	Proof queryproof.Proof `json:"proof"`
}

type IBCQueryResponse struct {
	Path  []string        `json:"path"`
	Value json.RawMessage `json:"value"`
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
	Checksum      string                  `json:"checksum,omitempty"`
}

type SnapshotChunkResponse struct {
	SchemaVersion    string                  `json:"schema_version"`
	State            store.StateRecord       `json:"state"`
	StateRoots       []store.StateRootRecord `json:"state_roots"`
	KV               []store.KVPair          `json:"kv,omitempty"`
	ChunkIndex       uint64                  `json:"chunk_index"`
	ChunkCount       uint64                  `json:"chunk_count"`
	SnapshotChecksum string                  `json:"snapshot_checksum"`
	ChunkChecksum    string                  `json:"chunk_checksum,omitempty"`
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
	Strict     bool   `json:"strict,omitempty"`
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

type JSONRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id,omitempty"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type web3FilterStore struct {
	mu             sync.Mutex
	nextID         uint64
	max            int
	order          []string
	filters        map[string]web3Filter
	onChange       func(Web3FilterStoreSnapshot) error
	lastPersistErr error
}

type Web3FilterStoreSnapshot struct {
	NextID  uint64       `json:"next_id"`
	Max     int          `json:"max"`
	Order   []string     `json:"order"`
	Filters []web3Filter `json:"filters"`
}

type web3Filter struct {
	ID          string `json:"id,omitempty"`
	Type        string
	Address     string
	Addresses   []string
	Topics      [][]string
	FromBlock   uint64
	ToBlock     uint64
	LastHeight  uint64
	SeenPending map[string]bool
	SeenLogs    map[string]bool
}

func NewServer(provider StatusProvider, cfg Config) *Server {
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	filters, startupErr := newWeb3FilterStoreWithConfig(cfg)
	startupErr = errors.Join(startupErr, validateRequiredCapabilities(provider, cfg))
	handler := newHandlerWithConfig(provider, cfg, filters)
	return &Server{
		provider:               provider,
		filterStore:            filters,
		web3FilterSnapshotPath: cfg.Web3FilterSnapshotPath,
		startupErr:             startupErr,
		server: &http.Server{
			Addr:              cfg.Address,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			TLSConfig:         cfg.TLSConfig,
		},
	}
}

// NewNetworkSafeServer builds an RPC server with fail-closed capability checks.
//
// Use this constructor for running nodes and SDK integrations that should fail
// during startup when any built-in node RPC capability is missing. NewServer is
// intentionally lower-level and may expose 501 responses for omitted optional
// provider interfaces.
func NewNetworkSafeServer(provider StatusProvider, cfg Config) (*Server, error) {
	server := NewServer(provider, NetworkSafeConfig(cfg))
	if err := server.StartupError(); err != nil {
		return nil, err
	}
	return server, nil
}

func (server *Server) Start(listener net.Listener) error {
	if listener == nil {
		return errors.New("listener is required")
	}
	if server.startupErr != nil {
		_ = listener.Close()
		return server.startupErr
	}
	var err error
	if server.server.TLSConfig != nil {
		err = server.server.ServeTLS(listener, "", "")
	} else {
		err = server.server.Serve(listener)
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (server *Server) StartupError() error {
	if server == nil {
		return nil
	}
	return server.startupErr
}

func (server *Server) Shutdown(ctx context.Context) error {
	shutdownErr := server.server.Shutdown(ctx)
	saveErr := server.saveWeb3FilterSnapshot()
	return errors.Join(shutdownErr, saveErr)
}

func (server *Server) saveWeb3FilterSnapshot() error {
	if server == nil || server.filterStore == nil || server.web3FilterSnapshotPath == "" {
		return nil
	}
	return saveWeb3FilterStoreSnapshotAtomic(server.web3FilterSnapshotPath, server.filterStore.Snapshot())
}

// NewHandler returns a low-level RPC handler for tests and custom embedders.
//
// Running nodes should prefer NewNetworkSafeServer or
// NewNetworkSafeHandlerWithConfig so missing provider capabilities fail at
// startup instead of surfacing as 501 responses after the RPC listener starts.
func NewHandler(provider StatusProvider) http.Handler {
	return NewHandlerWithConfig(provider, Config{})
}

// NewHandlerWithConfig returns a low-level RPC handler for tests and custom
// embedders. It does not enable the network-safety capability set by default.
//
// Use NewNetworkSafeHandlerWithConfig for public or validator node RPC surfaces.
func NewHandlerWithConfig(provider StatusProvider, cfg Config) http.Handler {
	filters, _ := newWeb3FilterStoreWithConfig(cfg)
	return newHandlerWithConfig(provider, cfg, filters)
}

// NewNetworkSafeHandlerWithConfig returns an RPC handler that fails closed when
// any required node capability is missing.
func NewNetworkSafeHandlerWithConfig(provider StatusProvider, cfg Config) (http.Handler, error) {
	cfg = NetworkSafeConfig(cfg)
	if err := validateRequiredCapabilities(provider, cfg); err != nil {
		return nil, err
	}
	filters, err := newWeb3FilterStoreWithConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newHandlerWithConfig(provider, cfg, filters), nil
}

func newHandlerWithConfig(provider StatusProvider, cfg Config, filters *web3FilterStore) http.Handler {
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = defaultMaxRequestBytes
	}
	if cfg.Web3LogMaxResults <= 0 {
		cfg.Web3LogMaxResults = defaultMaxWeb3LogResults
	}
	if cfg.Web3LogMaxBlockRange == 0 {
		cfg.Web3LogMaxBlockRange = defaultMaxWeb3LogBlockRange
	}
	if filters == nil {
		filters = newWeb3FilterStore()
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
		diagnostics := diagnosticsResponse(request.Context(), provider, cfg)
		statusCode := http.StatusOK
		if !diagnostics.OK {
			statusCode = http.StatusServiceUnavailable
		}
		writeJSON(writer, statusCode, diagnostics)
	})
	mux.HandleFunc("/capabilities", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		writeJSON(writer, http.StatusOK, providerCapabilities(provider, cfg))
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
	mux.HandleFunc("/finality/latest", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		finalityProvider, ok := provider.(FinalityProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "finality proof query is unavailable")
			return
		}
		proof, err := finalityProvider.LatestFinalityProof(request.Context())
		writeFinalityProof(writer, proof, err, strictFinalityRequested(request))
	})
	mux.HandleFunc("/finality/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		finalityProvider, ok := provider.(FinalityProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "finality proof query is unavailable")
			return
		}
		height, ok := parseHeightSelector(request.URL.Path, "/finality/")
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid finality height")
			return
		}
		proof, err := finalityProvider.FinalityProof(request.Context(), height)
		writeFinalityProof(writer, proof, err, strictFinalityRequested(request))
	})
	mux.HandleFunc("/events", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(EventQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "event query is unavailable")
			return
		}
		key := request.URL.Query().Get("key")
		value := request.URL.Query().Get("value")
		if key == "" || value == "" {
			writeError(writer, http.StatusBadRequest, "event key and value are required")
			return
		}
		records, err := queryProvider.QueryEvents(request.Context(), key, value)
		if errors.Is(err, events.ErrStoreMissing) {
			writeError(writer, http.StatusNotImplemented, "event query is unavailable")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, eventsResponse(key, value, records))
	})
	mux.HandleFunc("/proof", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		queryProvider, ok := provider.(QueryProofProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "query proof is unavailable")
			return
		}
		namespace := request.URL.Query().Get("namespace")
		key := request.URL.Query().Get("key")
		if namespace == "" || key == "" {
			writeError(writer, http.StatusBadRequest, "namespace and key are required")
			return
		}
		height, ok := parseOptionalHeight(request.URL.Query().Get("height"))
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid proof height")
			return
		}
		proof, err := queryProvider.QueryProof(request.Context(), height, namespace, []byte(key))
		if errors.Is(err, queryproof.ErrInvalidProof) {
			writeError(writer, http.StatusBadRequest, "invalid query proof request")
			return
		}
		if errors.Is(err, store.ErrStateNotFound) {
			writeError(writer, http.StatusNotFound, "state not found")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, QueryProofResponse{Proof: proof})
	})
	mux.HandleFunc("/ibc/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		if strings.HasPrefix(request.URL.Path, "/ibc/proof/") {
			queryProvider, ok := provider.(QueryProofProvider)
			if !ok {
				writeError(writer, http.StatusNotImplemented, "IBC proof query is unavailable")
				return
			}
			packet, ok := parseIBCPacketProofPath(request.URL.Path)
			if !ok {
				writeError(writer, http.StatusBadRequest, "invalid IBC proof path")
				return
			}
			height, ok := parseOptionalHeight(request.URL.Query().Get("height"))
			if !ok {
				writeError(writer, http.StatusBadRequest, "invalid proof height")
				return
			}
			proof, err := queryProvider.QueryProof(request.Context(), height, ibckeeper.Namespace, ibckeeper.PacketCommitmentKey(packet))
			if errors.Is(err, queryproof.ErrInvalidProof) {
				writeError(writer, http.StatusBadRequest, "invalid IBC proof request")
				return
			}
			if errors.Is(err, store.ErrStateNotFound) {
				writeError(writer, http.StatusNotFound, "IBC proof state not found")
				return
			}
			if err != nil {
				writeError(writer, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(writer, http.StatusOK, QueryProofResponse{Proof: proof})
			return
		}
		queryProvider, ok := provider.(IBCQueryProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "IBC query is unavailable")
			return
		}
		path, ok := parseIBCQueryPath(request.URL.Path)
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid IBC query path")
			return
		}
		response, err := queryProvider.IBCQuery(request.Context(), path)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		if response.Code != 0 {
			writeIBCQueryError(writer, response)
			return
		}
		if !json.Valid(response.Value) {
			writeError(writer, http.StatusInternalServerError, "IBC query returned invalid JSON")
			return
		}
		writeJSON(writer, http.StatusOK, IBCQueryResponse{Path: path, Value: append(json.RawMessage(nil), response.Value...)})
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
	mux.HandleFunc("/snapshot/chunk", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		snapshotProvider, ok := provider.(SnapshotProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "snapshot chunk export is unavailable")
			return
		}
		index, err := parseOptionalUintQuery(request, "index", 0)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		size, err := parseOptionalUintQuery(request, "size", 10000)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if size == 0 {
			writeError(writer, http.StatusBadRequest, "snapshot chunk size must be positive")
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
		chunk, err := snapshotChunkResponse(snapshot, index, size)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, chunk)
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
			if !allowAdmin(writer, request, cfg, "recovery") {
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
		if !allowAdmin(writer, request, cfg, "prune") {
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
		if !allowAdmin(writer, request, cfg, "replay") {
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
		if payload.Strict {
			strictReplayer, ok := provider.(StrictReplayProvider)
			if !ok {
				writeError(writer, http.StatusNotImplemented, "strict replay is unavailable")
				return
			}
			if payload.All || (payload.FromHeight == 0 && payload.ToHeight == 0) {
				result, err = strictReplayer.ReplayAllStrict(request.Context())
			} else {
				if payload.FromHeight == 0 || payload.ToHeight == 0 {
					writeError(writer, http.StatusBadRequest, "from_height and to_height are required")
					return
				}
				result, err = strictReplayer.ReplayStrict(request.Context(), types.Height(payload.FromHeight), types.Height(payload.ToHeight))
			}
		} else if payload.All || (payload.FromHeight == 0 && payload.ToHeight == 0) {
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
		if !allowAdmin(writer, request, cfg, "consensus") {
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
		if !allowAdmin(writer, request, cfg, "consensus") {
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
	mux.HandleFunc("/web3", func(writer http.ResponseWriter, request *http.Request) {
		if isWebSocketUpgrade(request) {
			handleWeb3WebSocket(writer, request, provider, cfg, filters)
			return
		}
		handleWeb3JSONRPC(writer, request, provider, cfg, filters)
	})
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			writeError(writer, http.StatusNotFound, "endpoint not found")
			return
		}
		if isWebSocketUpgrade(request) {
			handleWeb3WebSocket(writer, request, provider, cfg, filters)
			return
		}
		handleWeb3JSONRPC(writer, request, provider, cfg, filters)
	})
	return applyMiddleware(versionedHandler(mux), cfg)
}

func handleWeb3JSONRPC(writer http.ResponseWriter, request *http.Request, provider StatusProvider, cfg Config, filters *web3FilterStore) {
	if !allowPost(writer, request) {
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, cfg.MaxRequestBytes)
	defer request.Body.Close()
	var raw json.RawMessage
	if err := decodeStrictJSON(request.Body, &raw); err != nil {
		writeJSONRPC(writer, json.RawMessage("null"), nil, &JSONRPCError{Code: -32700, Message: "parse error"})
		return
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		writeJSONRPC(writer, json.RawMessage("null"), nil, &JSONRPCError{Code: -32700, Message: "parse error"})
		return
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			writeJSONRPC(writer, json.RawMessage("null"), nil, &JSONRPCError{Code: -32600, Message: "invalid JSON-RPC batch"})
			return
		}
		responses := make([]JSONRPCResponse, 0, len(batch))
		for _, item := range batch {
			response, notify := executeWeb3Payload(request.Context(), provider, cfg, filters, item)
			if notify {
				continue
			}
			responses = append(responses, response)
		}
		if len(responses) == 0 {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(writer, http.StatusOK, responses)
		return
	}
	response, notify := executeWeb3Payload(request.Context(), provider, cfg, filters, trimmed)
	if notify {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func executeWeb3Payload(ctx context.Context, provider StatusProvider, cfg Config, filters *web3FilterStore, raw json.RawMessage) (JSONRPCResponse, bool) {
	var payload JSONRPCRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &JSONRPCError{Code: -32600, Message: "invalid request"}}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &JSONRPCError{Code: -32600, Message: "invalid request"}}, false
	}
	if payload.JSONRPC != "" && payload.JSONRPC != "2.0" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Error: &JSONRPCError{Code: -32600, Message: "invalid JSON-RPC version"}}, false
	}
	if payload.Method == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Error: &JSONRPCError{Code: -32600, Message: "method is required"}}, false
	}
	notify := len(payload.ID) == 0
	result, rpcErr := executeWeb3Method(ctx, provider, cfg, filters, payload.Method, payload.Params)
	if notify {
		return JSONRPCResponse{}, true
	}
	if len(payload.ID) == 0 {
		payload.ID = json.RawMessage("null")
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Result: result, Error: rpcErr}, false
}

func writeFinalityProof(writer http.ResponseWriter, proof finality.Proof, err error, requireStrict bool) {
	if errors.Is(err, node.ErrFinalityNotFound) ||
		errors.Is(err, store.ErrBlockNotFound) ||
		errors.Is(err, store.ErrBlockIndexNotFound) {
		writeError(writer, http.StatusNotFound, "finality proof not found")
		return
	}
	if errors.Is(err, vexoruntime.ErrFinalityProofHeightMismatch) ||
		errors.Is(err, vexoruntime.ErrFinalityProofBlockMismatch) {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if requireStrict && !proof.HasThreeChainCommitProof() {
		writeError(writer, http.StatusNotFound, "strict finality proof not found")
		return
	}
	writeJSON(writer, http.StatusOK, finalityProofResponse(proof))
}

func strictFinalityRequested(request *http.Request) bool {
	value := request.URL.Query().Get("strict")
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func executeWeb3Method(ctx context.Context, provider StatusProvider, cfg Config, filters *web3FilterStore, method string, params []json.RawMessage) (any, *JSONRPCError) {
	switch method {
	case "rpc_modules":
		return web3RPCModules(), nil
	case "vexo_web3Capabilities":
		return web3Capabilities(), nil
	case "web3_clientVersion":
		return "vexo-consensus/web3", nil
	case "web3_sha3":
		return web3Sha3(params)
	case "net_version":
		return strconv.FormatUint(web3ChainID(provider.Status(ctx)), 10), nil
	case "net_listening":
		return provider.Status(ctx).Running, nil
	case "net_peerCount":
		return hexQuantity(uint64(provider.Status(ctx).PeerCount)), nil
	case "eth_chainId":
		return hexQuantity(web3ChainID(provider.Status(ctx))), nil
	case "eth_protocolVersion":
		return "0x1", nil
	case "eth_syncing":
		return web3Syncing(provider.Status(ctx)), nil
	case "eth_mining":
		return web3Mining(provider, ctx), nil
	case "eth_hashrate":
		return hexQuantity(0), nil
	case "eth_accounts":
		return web3ConfiguredAccounts(cfg), nil
	case "eth_coinbase":
		accounts := web3ConfiguredAccounts(cfg)
		if len(accounts) == 0 {
			return "0x0000000000000000000000000000000000000000", nil
		}
		return accounts[0], nil
	case "eth_sign":
		return web3EthSign(cfg, params, false)
	case "personal_sign":
		return web3EthSign(cfg, params, true)
	case "eth_signTransaction":
		return web3SignTransaction(ctx, provider, cfg, params)
	case "eth_sendTransaction":
		return web3SendTransaction(ctx, provider, cfg, params)
	case "eth_blockNumber":
		return hexQuantity(uint64(provider.Status(ctx).LatestHeight)), nil
	case "eth_getBlockByNumber":
		record, rpcErr := web3BlockByNumber(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		fullTx := web3FullTransactionParam(params, 1)
		return web3BlockFromRecord(ctx, provider, cfg, record, fullTx)
	case "eth_getBlockTransactionCountByNumber":
		record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{firstParam(params, "null")})
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3BlockTransactionCount(record), nil
	case "eth_getBlockByHash":
		record, rpcErr := web3BlockByHash(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		fullTx := web3FullTransactionParam(params, 1)
		return web3BlockFromRecord(ctx, provider, cfg, record, fullTx)
	case "eth_getBlockTransactionCountByHash":
		record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{firstParam(params, "null")})
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3BlockTransactionCount(record), nil
	case "eth_getTransactionByBlockNumberAndIndex":
		return web3TransactionByBlockNumberAndIndex(ctx, provider, params)
	case "eth_getTransactionByBlockHashAndIndex":
		return web3TransactionByBlockHashAndIndex(ctx, provider, params)
	case "eth_getUncleCountByBlockNumber", "eth_getUncleCountByBlockHash":
		return hexQuantity(0), nil
	case "eth_getUncleByBlockNumberAndIndex", "eth_getUncleByBlockHashAndIndex":
		return nil, nil
	case "eth_gasPrice":
		if query, ok := provider.(ChainQueryProvider); ok {
			state, err := query.LatestState(ctx)
			if err == nil {
				if state.NextBaseFee > 0 {
					return hexQuantity(state.NextBaseFee), nil
				}
				if state.BaseFee > 0 {
					return hexQuantity(state.BaseFee), nil
				}
			}
		}
		return nil, &JSONRPCError{Code: -32000, Message: "gas price is unavailable"}
	case "eth_blobBaseFee":
		return hexQuantity(web3LatestBlobBaseFee(ctx, provider)), nil
	case "eth_maxPriorityFeePerGas":
		return web3MaxPriorityFeePerGas(ctx, provider), nil
	case "eth_feeHistory":
		return web3FeeHistory(ctx, provider, params)
	case "eth_getBalance":
		if len(params) == 0 || len(params) > 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getBalance requires address and optional block tag"}
		}
		address, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		account, rpcErr := web3AccountState(ctx, provider, address, params, 1)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3AccountBalanceHex(account), nil
	case "eth_getTransactionCount":
		if len(params) == 0 || len(params) > 2 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionCount requires address and optional block tag"}
		}
		address, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		account, rpcErr := web3AccountState(ctx, provider, address, params, 1)
		if rpcErr != nil {
			return nil, rpcErr
		}
		sequence := account.Nonce
		if len(params) == 2 {
			tag := ""
			if trimmed := bytes.TrimSpace(params[1]); len(trimmed) > 0 && trimmed[0] != '{' {
				var err error
				tag, err = jsonRPCStringParam(params[1])
				if err != nil {
					return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
				}
			}
			if tag == "pending" {
				sequence, err = web3PendingSequence(ctx, provider, address, sequence)
				if err != nil {
					return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
				}
			}
		}
		return hexQuantity(sequence), nil
	case "eth_getCode":
		code, rpcErr := web3Code(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return code, nil
	case "eth_getStorageAt":
		value, rpcErr := web3StorageAt(ctx, provider, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return value, nil
	case "eth_getProof":
		return web3GetProof(ctx, provider, params)
	case "eth_sendRawTransaction":
		submitter, ok := provider.(TxSubmitter)
		if !ok {
			return nil, &JSONRPCError{Code: -32000, Message: "transaction submission is unavailable"}
		}
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_sendRawTransaction requires one raw transaction parameter"}
		}
		rawTx, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		decoded, err := web3DecodeRawEthereumTx(ctx, provider, cfg, rawTx)
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		if len(decoded.BlobHashes) > 0 {
			return nil, &JSONRPCError{Code: -32602, Message: "blob transactions require eth_sendRawBlobTransaction or vexo_sendRawBlobTransaction with an explicit sidecar"}
		}
		if err := submitter.SubmitTx(ctx, decoded.Tx); err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return decoded.Hash, nil
	case "eth_sendRawBlobTransaction":
		return web3SendRawBlobTransaction(ctx, provider, cfg, params)
	case "vexo_sendRawBlobTransaction":
		return web3SendRawBlobTransaction(ctx, provider, cfg, params)
	case "vexo_getBlobSidecarByTxHash":
		return web3BlobSidecar(ctx, provider, params, "blob_sidecar")
	case "vexo_getBlobSidecarByBlobHash":
		return web3BlobSidecar(ctx, provider, params, "blob_sidecar_by_hash")
	case "eth_getTransactionReceipt":
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionReceipt requires one transaction hash"}
		}
		hash, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if !found {
			return nil, nil
		}
		return web3ReceiptObject(ctx, provider, value)
	case "eth_getBlockReceipts":
		return web3BlockReceipts(ctx, provider, params)
	case "eth_getTransactionByHash":
		return web3TransactionByHash(ctx, provider, params)
	case "eth_getRawTransactionByHash":
		return web3RawTransactionByHash(ctx, provider, params)
	case "eth_getRawTransactionByBlockNumberAndIndex":
		return web3RawTransactionByBlockNumberAndIndex(ctx, provider, params)
	case "eth_getRawTransactionByBlockHashAndIndex":
		return web3RawTransactionByBlockHashAndIndex(ctx, provider, params)
	case "txpool_status":
		return web3TxpoolStatus(ctx, provider)
	case "txpool_content":
		return web3TxpoolContent(ctx, provider)
	case "txpool_contentFrom":
		return web3TxpoolContentFrom(ctx, provider, params)
	case "txpool_inspect":
		return web3TxpoolInspect(ctx, provider)
	case "eth_pendingTransactions":
		return web3PendingTransactions(ctx, provider)
	case "debug_traceTransaction":
		return web3DebugTraceTransaction(ctx, provider, params)
	case "debug_traceBlockByNumber":
		return web3DebugTraceBlockByNumber(ctx, provider, params)
	case "debug_traceBlockByHash":
		return web3DebugTraceBlockByHash(ctx, provider, params)
	case "trace_transaction":
		return web3TraceTransaction(ctx, provider, params)
	case "trace_get":
		return web3TraceGet(ctx, provider, params)
	case "trace_block":
		return web3TraceBlock(ctx, provider, params)
	case "trace_filter":
		return web3TraceFilter(ctx, provider, params)
	case "trace_replayTransaction":
		return web3TraceReplayTransaction(ctx, provider, params)
	case "trace_replayBlockTransactions":
		return web3TraceReplayBlockTransactions(ctx, provider, params)
	case "eth_getLogs":
		filter, rpcErr := web3LogFilterParam(ctx, provider, cfg, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return web3LogsForFilter(ctx, provider, cfg, filter)
	case "eth_newBlockFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		if len(params) != 0 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_newBlockFilter does not accept parameters"}
		}
		filterID := filters.addBlock(uint64(provider.Status(ctx).LatestHeight))
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return filterID, nil
	case "eth_newPendingTransactionFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		pendingProvider, ok := provider.(PendingTxProvider)
		if !ok {
			return nil, &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
		}
		if len(params) != 0 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_newPendingTransactionFilter does not accept parameters"}
		}
		hashes, err := pendingProvider.PendingTxHashes(ctx)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		filterID := filters.addPending(hashes)
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return filterID, nil
	case "eth_newFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		filter, rpcErr := web3LogFilterParam(ctx, provider, cfg, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		logs, rpcErr := web3LogsForFilter(ctx, provider, cfg, filter)
		if rpcErr != nil {
			return nil, rpcErr
		}
		filterID := filters.addLog(filter, logs, uint64(provider.Status(ctx).LatestHeight))
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return filterID, nil
	case "eth_getFilterChanges", "eth_getFilterLogs":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: method + " requires filter id"}
		}
		filterID, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		filter, found := filters.get(filterID)
		if !found {
			return nil, &JSONRPCError{Code: -32000, Message: "filter not found"}
		}
		if filter.Type == "pending" {
			changes, updated, rpcErr := web3PendingFilterChanges(ctx, provider, filter, method == "eth_getFilterChanges")
			if rpcErr != nil {
				return nil, rpcErr
			}
			if method == "eth_getFilterChanges" {
				filters.replace(filterID, updated)
				if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
					return nil, rpcErr
				}
			}
			return changes, nil
		}
		changes, rpcErr := web3FilterChanges(ctx, provider, cfg, filter, method == "eth_getFilterChanges")
		if rpcErr != nil {
			return nil, rpcErr
		}
		if method == "eth_getFilterChanges" {
			if filter.Type == "log" {
				currentLogs, rpcErr := web3LogsForFilter(ctx, provider, cfg, filter)
				if rpcErr != nil {
					return nil, rpcErr
				}
				filter.SeenLogs = web3SeenLogSet(currentLogs)
				filter.LastHeight = uint64(provider.Status(ctx).LatestHeight)
				filters.replace(filterID, filter)
			} else {
				filters.mark(filterID, uint64(provider.Status(ctx).LatestHeight))
			}
			if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
				return nil, rpcErr
			}
		}
		return changes, nil
	case "eth_uninstallFilter":
		if filters == nil {
			return nil, &JSONRPCError{Code: -32000, Message: "filter store is unavailable"}
		}
		if len(params) != 1 {
			return nil, &JSONRPCError{Code: -32602, Message: "eth_uninstallFilter requires filter id"}
		}
		filterID, err := jsonRPCStringParam(params[0])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		removed := filters.remove(filterID)
		if rpcErr := web3FilterPersistRPCError(filters); rpcErr != nil {
			return nil, rpcErr
		}
		return removed, nil
	case "eth_call":
		callResponse, rpcErr := web3EVMCall(ctx, provider, cfg, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return callResponse.Output, nil
	case "eth_estimateGas":
		return web3EstimateGas(ctx, provider, cfg, params)
	case "eth_createAccessList":
		return web3CreateAccessList(ctx, provider, cfg, params)
	case "debug_traceCall":
		return web3DebugTraceCall(ctx, provider, cfg, params)
	case "trace_call":
		return web3TraceCall(ctx, provider, cfg, params)
	case "eth_subscribe", "eth_unsubscribe":
		return nil, &JSONRPCError{Code: -32000, Message: method + " requires a WebSocket transport"}
	default:
		return nil, &JSONRPCError{Code: -32601, Message: "method not found"}
	}
}

func writeJSONRPC(writer http.ResponseWriter, id json.RawMessage, result any, rpcErr *JSONRPCError) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	writeJSON(writer, http.StatusOK, JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
}

func firstParam(params []json.RawMessage, fallback string) json.RawMessage {
	if len(params) == 0 {
		return json.RawMessage(fallback)
	}
	return params[0]
}

func web3RPCModules() map[string]string {
	return map[string]string{
		"debug":  "1.0",
		"eth":    "1.0",
		"net":    "1.0",
		"rpc":    "1.0",
		"trace":  "1.0",
		"txpool": "1.0",
		"vexo":   "1.0",
		"web3":   "1.0",
	}
}

func web3Capabilities() map[string]any {
	return map[string]any{
		"ethereum_p2p":                 false,
		"native_vexo_network":          true,
		"json_rpc_namespaces":          web3RPCModules(),
		"raw_transactions":             true,
		"blob_transactions":            true,
		"native_fee_token_accounting":  true,
		"dynamic_base_fee":             true,
		"dynamic_blob_base_fee":        true,
		"eth_call":                     true,
		"eth_estimateGas":              true,
		"eth_createAccessList":         true,
		"eth_getProof":                 true,
		"websocket_subscriptions":      true,
		"filters":                      true,
		"txpool":                       true,
		"debug_traceTransaction":       true,
		"debug_traceCall":              true,
		"trace_replayTransaction":      true,
		"trace_reexecution":            "receipt_or_call_context_backed",
		"trace_filter_max_block_range": 128,
		"unsupported_namespaces":       []string{"admin", "engine", "miner"},
		"compatibility_target":         "Ethereum JSON-RPC semantics on the Vexo native network; not Ethereum devp2p/engine API node compatibility",
		"recommended_conformance_suites": []string{
			"ethers",
			"web3.js",
			"MetaMask",
			"Hardhat",
			"Foundry",
		},
	}
}

func web3ConfiguredAccounts(cfg Config) []string {
	if !cfg.EnableEVMManagedAccounts {
		return []string{}
	}
	accounts := make([]string, 0, len(cfg.EVMAccountPrivateKeys))
	seen := make(map[string]struct{}, len(cfg.EVMAccountPrivateKeys))
	for _, rawKey := range cfg.EVMAccountPrivateKeys {
		key, err := web3PrivateKey(rawKey)
		if err != nil {
			continue
		}
		address := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
		lower := strings.ToLower(address)
		if _, found := seen[lower]; found {
			continue
		}
		seen[lower] = struct{}{}
		accounts = append(accounts, address)
	}
	return accounts
}

func web3AccountKey(cfg Config, address string) (*ecdsa.PrivateKey, *JSONRPCError) {
	if !cfg.EnableEVMManagedAccounts {
		return nil, &JSONRPCError{Code: -32000, Message: "managed accounts are disabled"}
	}
	parsed, err := parseHexAddress(address)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid account address"}
	}
	target := strings.ToLower(string(parsed))
	for _, rawKey := range cfg.EVMAccountPrivateKeys {
		key, err := web3PrivateKey(rawKey)
		if err != nil {
			continue
		}
		if strings.ToLower(gethcrypto.PubkeyToAddress(key.PublicKey).Hex()) == target {
			return key, nil
		}
	}
	return nil, &JSONRPCError{Code: -32000, Message: "account is not managed by this node"}
}

func web3PrivateKey(rawKey string) (*ecdsa.PrivateKey, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(rawKey, "0x"))
	if trimmed == "" {
		return nil, errors.New("empty private key")
	}
	return gethcrypto.HexToECDSA(trimmed)
}

func web3EthSign(cfg Config, params []json.RawMessage, personalOrder bool) (string, *JSONRPCError) {
	if len(params) != 2 {
		return "", &JSONRPCError{Code: -32602, Message: "signing requires address and data"}
	}
	addressIndex := 0
	dataIndex := 1
	if personalOrder {
		addressIndex = 1
		dataIndex = 0
	}
	address, err := jsonRPCStringParam(params[addressIndex])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	dataHex, err := jsonRPCStringParam(params[dataIndex])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, err := hexBytes(dataHex)
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: "invalid sign data hex"}
	}
	key, rpcErr := web3AccountKey(cfg, address)
	if rpcErr != nil {
		return "", rpcErr
	}
	signature, err := gethcrypto.Sign(gethaccounts.TextHash(data), key)
	if err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	signature[64] += 27
	return "0x" + hex.EncodeToString(signature), nil
}

func web3SignTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	signed, raw, rpcErr := web3SignEthereumTransaction(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"raw": "0x" + hex.EncodeToString(raw),
		"tx":  web3SignedTransactionObject(signed),
	}, nil
}

func web3SendTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	submitter, ok := provider.(TxSubmitter)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "transaction submission is unavailable"}
	}
	signed, raw, rpcErr := web3SignEthereumTransaction(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	rawHex := "0x" + hex.EncodeToString(raw)
	decoded, err := web3DecodeRawEthereumTx(ctx, provider, cfg, rawHex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if len(decoded.BlobHashes) > 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "blob transactions require eth_sendRawBlobTransaction or vexo_sendRawBlobTransaction with an explicit sidecar"}
	}
	if err := submitter.SubmitTx(ctx, decoded.Tx); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return signed.Hash().Hex(), nil
}

func web3SignEthereumTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (*gethtypes.Transaction, []byte, *JSONRPCError) {
	if len(params) != 1 {
		return nil, nil, &JSONRPCError{Code: -32602, Message: "transaction signing requires one transaction object"}
	}
	var payload web3TransactionCall
	if err := json.Unmarshal(params[0], &payload); err != nil {
		return nil, nil, &JSONRPCError{Code: -32602, Message: "invalid transaction object"}
	}
	if payload.From == "" {
		return nil, nil, &JSONRPCError{Code: -32602, Message: "from address is required"}
	}
	key, rpcErr := web3AccountKey(cfg, payload.From)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}
	tx, rpcErr := web3UnsignedEthereumTransaction(ctx, provider, payload)
	if rpcErr != nil {
		return nil, nil, rpcErr
	}
	chainID := new(big.Int).SetUint64(web3ChainID(provider.Status(ctx)))
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		return nil, nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return signed, raw, nil
}

func web3UnsignedEthereumTransaction(ctx context.Context, provider StatusProvider, payload web3TransactionCall) (*gethtypes.Transaction, *JSONRPCError) {
	dataHex := payload.Data
	if dataHex == "" {
		dataHex = payload.Input
	}
	if dataHex == "" {
		dataHex = "0x"
	}
	data, err := hexBytes(dataHex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid data hex"}
	}
	nonce, rpcErr := web3TransactionNonce(ctx, provider, payload)
	if rpcErr != nil {
		return nil, rpcErr
	}
	gas, rpcErr := web3TransactionGas(payload)
	if rpcErr != nil {
		return nil, rpcErr
	}
	value, rpcErr := web3TransactionBig(payload.Value, "invalid value quantity")
	if rpcErr != nil {
		return nil, rpcErr
	}
	accessList, rpcErr := web3GethAccessList(payload.AccessList)
	if rpcErr != nil {
		return nil, rpcErr
	}
	chainID := new(big.Int).SetUint64(web3ChainID(provider.Status(ctx)))
	to, rpcErr := web3OptionalToAddress(payload.To)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if len(bytes.TrimSpace(payload.AuthorizationList)) > 0 && string(bytes.TrimSpace(payload.AuthorizationList)) != "null" {
		authList, rpcErr := web3SetCodeAuthorizations(payload.AuthorizationList)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if to == nil {
			return nil, &JSONRPCError{Code: -32602, Message: "set-code transaction requires to address"}
		}
		gasFeeCap, rpcErr := web3RequiredUint256(payload.MaxFeePerGas, "maxFeePerGas is required for set-code transaction")
		if rpcErr != nil {
			return nil, rpcErr
		}
		gasTipCap, rpcErr := web3OptionalUint256(payload.MaxPriorityFeePerGas)
		if rpcErr != nil {
			return nil, rpcErr
		}
		txValue, rpcErr := web3BigToUint256(value, "value exceeds uint256")
		if rpcErr != nil {
			return nil, rpcErr
		}
		return gethtypes.NewTx(&gethtypes.SetCodeTx{
			ChainID:    uint256.MustFromBig(chainID),
			Nonce:      nonce,
			GasTipCap:  gasTipCap,
			GasFeeCap:  gasFeeCap,
			Gas:        gas,
			To:         *to,
			Value:      txValue,
			Data:       data,
			AccessList: accessList,
			AuthList:   authList,
		}), nil
	}
	if payload.MaxFeePerGas != "" || payload.MaxPriorityFeePerGas != "" {
		gasFeeCap, rpcErr := web3RequiredBig(payload.MaxFeePerGas, "maxFeePerGas is required for dynamic fee transaction")
		if rpcErr != nil {
			return nil, rpcErr
		}
		gasTipCap, rpcErr := web3TransactionBig(payload.MaxPriorityFeePerGas, "invalid maxPriorityFeePerGas quantity")
		if rpcErr != nil {
			return nil, rpcErr
		}
		return gethtypes.NewTx(&gethtypes.DynamicFeeTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasTipCap:  gasTipCap,
			GasFeeCap:  gasFeeCap,
			Gas:        gas,
			To:         to,
			Value:      value,
			Data:       data,
			AccessList: accessList,
		}), nil
	}
	gasPrice, rpcErr := web3TransactionGasPrice(ctx, provider, payload)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if len(accessList) > 0 {
		return gethtypes.NewTx(&gethtypes.AccessListTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasPrice:   gasPrice,
			Gas:        gas,
			To:         to,
			Value:      value,
			Data:       data,
			AccessList: accessList,
		}), nil
	}
	return gethtypes.NewTx(&gethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gas,
		To:       to,
		Value:    value,
		Data:     data,
	}), nil
}

func web3TransactionNonce(ctx context.Context, provider StatusProvider, payload web3TransactionCall) (uint64, *JSONRPCError) {
	if payload.Nonce != "" {
		nonce, err := parseHexQuantity(payload.Nonce)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid nonce quantity"}
		}
		return nonce, nil
	}
	account, rpcErr := web3AccountState(ctx, provider, payload.From, []json.RawMessage{json.RawMessage(strconv.Quote(payload.From)), json.RawMessage(`"pending"`)}, 1)
	if rpcErr != nil {
		return 0, rpcErr
	}
	return account.Nonce, nil
}

func web3TransactionGas(payload web3TransactionCall) (uint64, *JSONRPCError) {
	if payload.Gas == "" {
		return defaultWeb3BlockGasLimit, nil
	}
	gas, err := parseHexQuantity(payload.Gas)
	if err != nil {
		return 0, &JSONRPCError{Code: -32602, Message: "invalid gas quantity"}
	}
	return gas, nil
}

func web3TransactionGasPrice(ctx context.Context, provider StatusProvider, payload web3TransactionCall) (*big.Int, *JSONRPCError) {
	if payload.GasPrice != "" {
		return web3RequiredBig(payload.GasPrice, "invalid gasPrice quantity")
	}
	if query, ok := provider.(ChainQueryProvider); ok {
		if state, err := query.LatestState(ctx); err == nil {
			price := state.NextBaseFee
			if price == 0 {
				price = state.BaseFee
			}
			return new(big.Int).SetUint64(price), nil
		}
	}
	return big.NewInt(0), nil
}

func web3TransactionBig(value string, message string) (*big.Int, *JSONRPCError) {
	if value == "" {
		return big.NewInt(0), nil
	}
	parsed, err := parseHexQuantityBig(value)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return parsed, nil
}

func web3RequiredBig(value string, message string) (*big.Int, *JSONRPCError) {
	if value == "" {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return web3TransactionBig(value, message)
}

func web3OptionalUint256(value string) (*uint256.Int, *JSONRPCError) {
	if value == "" {
		return uint256.NewInt(0), nil
	}
	parsed, rpcErr := web3TransactionBig(value, "invalid uint256 quantity")
	if rpcErr != nil {
		return nil, rpcErr
	}
	if parsed.BitLen() > 256 {
		return nil, &JSONRPCError{Code: -32602, Message: "quantity exceeds uint256"}
	}
	return uint256.MustFromBig(parsed), nil
}

func web3RequiredUint256(value string, message string) (*uint256.Int, *JSONRPCError) {
	if value == "" {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return web3OptionalUint256(value)
}

func web3BigToUint256(value *big.Int, message string) (*uint256.Int, *JSONRPCError) {
	if value == nil {
		return uint256.NewInt(0), nil
	}
	if value.Sign() < 0 || value.BitLen() > 256 {
		return nil, &JSONRPCError{Code: -32602, Message: message}
	}
	return uint256.MustFromBig(value), nil
}

func web3OptionalToAddress(value string) (*gethcommon.Address, *JSONRPCError) {
	if value == "" {
		return nil, nil
	}
	parsed, err := parseHexAddress(value)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid to address"}
	}
	address := gethcommon.HexToAddress(string(parsed))
	return &address, nil
}

func web3GethAccessList(entries []web3AccessListEntry) (gethtypes.AccessList, *JSONRPCError) {
	if len(entries) == 0 {
		return nil, nil
	}
	out := make(gethtypes.AccessList, 0, len(entries))
	for _, entry := range entries {
		parsed, err := parseHexAddress(entry.Address)
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid access list address"}
		}
		item := gethtypes.AccessTuple{Address: gethcommon.HexToAddress(string(parsed))}
		for _, key := range entry.StorageKeys {
			if _, err := parseFixedWidthHex(key, 32); err != nil {
				return nil, &JSONRPCError{Code: -32602, Message: "invalid access list storage key"}
			}
			item.StorageKeys = append(item.StorageKeys, gethcommon.HexToHash(key))
		}
		out = append(out, item)
	}
	return out, nil
}

func web3SetCodeAuthorizations(raw json.RawMessage) ([]gethtypes.SetCodeAuthorization, *JSONRPCError) {
	var authList []gethtypes.SetCodeAuthorization
	if err := json.Unmarshal(raw, &authList); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid authorizationList"}
	}
	return authList, nil
}

func web3SignedTransactionObject(tx *gethtypes.Transaction) map[string]any {
	out := map[string]any{
		"hash":  tx.Hash().Hex(),
		"type":  hexQuantity(uint64(tx.Type())),
		"nonce": hexQuantity(tx.Nonce()),
		"gas":   hexQuantity(tx.Gas()),
		"input": "0x" + hex.EncodeToString(tx.Data()),
		"value": hexQuantityBig(tx.Value()),
	}
	if to := tx.To(); to != nil {
		out["to"] = to.Hex()
	} else {
		out["to"] = nil
	}
	if chainID := tx.ChainId(); chainID != nil {
		out["chainId"] = hexQuantityBig(chainID)
	}
	if gasPrice := tx.GasPrice(); gasPrice != nil {
		out["gasPrice"] = hexQuantityBig(gasPrice)
	}
	if tx.GasFeeCap() != nil {
		out["maxFeePerGas"] = hexQuantityBig(tx.GasFeeCap())
	}
	if tx.GasTipCap() != nil {
		out["maxPriorityFeePerGas"] = hexQuantityBig(tx.GasTipCap())
	}
	if accessList := tx.AccessList(); len(accessList) > 0 {
		items := make([]any, 0, len(accessList))
		for _, entry := range accessList {
			keys := make([]string, 0, len(entry.StorageKeys))
			for _, key := range entry.StorageKeys {
				keys = append(keys, key.Hex())
			}
			items = append(items, map[string]any{"address": entry.Address.Hex(), "storageKeys": keys})
		}
		out["accessList"] = items
	}
	if authList := tx.SetCodeAuthorizations(); len(authList) > 0 {
		out["authorizationList"] = authList
	}
	return out
}

type web3BlobSidecarParam struct {
	BlobHashes      []string `json:"blobHashes,omitempty"`
	BlobHashesSnake []string `json:"blob_hashes,omitempty"`
	Blobs           []string `json:"blobs"`
	Commitments     []string `json:"commitments"`
	Proofs          []string `json:"proofs"`
}

func web3DecodeRawEthereumTx(ctx context.Context, provider StatusProvider, cfg Config, rawTx string) (ethcompat.DecodedTransaction, error) {
	baseFee := uint64(0)
	if query, ok := provider.(ChainQueryProvider); ok {
		if state, err := query.LatestState(ctx); err == nil {
			baseFee = state.BaseFee
			if state.NextBaseFee > 0 {
				baseFee = state.NextBaseFee
			}
		}
	}
	return ethcompat.DecodeRawTransaction(rawTx, ethcompat.DecodeOptions{
		ChainID:                web3ChainID(provider.Status(ctx)),
		BaseFee:                baseFee,
		BlobBaseFee:            web3LatestBlobBaseFee(ctx, provider),
		VM:                     "evm",
		AllowUnprotectedLegacy: cfg.AllowUnprotectedLegacyTx,
	})
}

func web3SendRawBlobTransaction(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	submitter, ok := provider.(TxSubmitter)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "transaction submission is unavailable"}
	}
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "vexo_sendRawBlobTransaction requires raw transaction and blob sidecar parameters"}
	}
	rawTx, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	decoded, err := web3DecodeRawEthereumTx(ctx, provider, cfg, rawTx)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if len(decoded.BlobHashes) == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "raw transaction does not reference blobs"}
	}
	sidecarTag, rpcErr := web3EncodeBlobSidecarParam(params[1], decoded.BlobHashes)
	if rpcErr != nil {
		return nil, rpcErr
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if canonical.Tags == nil {
		canonical.Tags = make(map[string]string)
	}
	canonical.Tags[ethcompat.TagBlobSidecar] = sidecarTag
	tx, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if err := submitter.SubmitTx(ctx, tx); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return decoded.Hash, nil
}

func web3EncodeBlobSidecarParam(raw json.RawMessage, expectedHashes []string) (string, *JSONRPCError) {
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '"' {
		encoded, err := jsonRPCStringParam(raw)
		if err != nil {
			return "", &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		bundle, err := ethcompat.DecodeBlobSidecarBundle(encoded)
		if err != nil {
			return "", &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		if !web3SameBlobHashes(expectedHashes, bundle.BlobHashes) {
			return "", &JSONRPCError{Code: -32602, Message: ethcompat.ErrInvalidBlobSidecar.Error()}
		}
		return encoded, nil
	}
	var param web3BlobSidecarParam
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&param); err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	hashes := param.BlobHashes
	if len(hashes) == 0 {
		hashes = param.BlobHashesSnake
	}
	if len(hashes) == 0 {
		hashes = expectedHashes
	}
	if !web3SameBlobHashes(expectedHashes, hashes) {
		return "", &JSONRPCError{Code: -32602, Message: ethcompat.ErrInvalidBlobSidecar.Error()}
	}
	encoded, err := ethcompat.EncodeBlobSidecarBundle(ethcompat.BlobSidecarBundle{
		BlobHashes:  hashes,
		Blobs:       param.Blobs,
		Commitments: param.Commitments,
		Proofs:      param.Proofs,
	})
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	return encoded, nil
}

func web3SameBlobHashes(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !strings.EqualFold(left[index], right[index]) {
			return false
		}
	}
	return true
}

func web3BlobSidecar(ctx context.Context, provider StatusProvider, params []json.RawMessage, queryPath string) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: queryPath + " requires one hash parameter"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	response, err := query.AppQuery(ctx, []string{"evm", queryPath, hash}, nil)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return nil, nil
	}
	if response.Code != 0 {
		return nil, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	return rawJSONObject(response.Value)
}

func web3Sha3(params []json.RawMessage) (string, *JSONRPCError) {
	if len(params) != 1 {
		return "", &JSONRPCError{Code: -32602, Message: "web3_sha3 requires one data parameter"}
	}
	value, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, err := hexBytes(value)
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: "invalid hex data"}
	}
	hash := sha3.NewLegacyKeccak256()
	_, _ = hash.Write(data)
	return "0x" + hex.EncodeToString(hash.Sum(nil)), nil
}

type web3CallRequest struct {
	VM                        string                                 `json:"vm"`
	From                      string                                 `json:"from"`
	To                        string                                 `json:"to"`
	Method                    string                                 `json:"method"`
	Input                     string                                 `json:"input,omitempty"`
	GasLimit                  uint64                                 `json:"gas_limit,omitempty"`
	Value                     uint64                                 `json:"value,omitempty"`
	ValueHex                  string                                 `json:"value_hex,omitempty"`
	Height                    uint64                                 `json:"height,omitempty"`
	GasPrice                  uint64                                 `json:"gas_price,omitempty"`
	GasPriceHex               string                                 `json:"gas_price_hex,omitempty"`
	MaxFeePerGas              uint64                                 `json:"max_fee_per_gas,omitempty"`
	MaxFeePerGasHex           string                                 `json:"max_fee_per_gas_hex,omitempty"`
	MaxPriorityFeePerGas      uint64                                 `json:"max_priority_fee_per_gas,omitempty"`
	MaxPriorityFeePerGasHex   string                                 `json:"max_priority_fee_per_gas_hex,omitempty"`
	BaseFee                   uint64                                 `json:"base_fee,omitempty"`
	BlobBaseFee               uint64                                 `json:"blob_base_fee,omitempty"`
	BlobHashes                []string                               `json:"blob_hashes,omitempty"`
	Nonce                     uint64                                 `json:"nonce,omitempty"`
	AccessList                []contract.AccessListEntry             `json:"access_list,omitempty"`
	StateOverrides            map[string]evmmodule.CallStateOverride `json:"state_overrides,omitempty"`
	BlockOverride             evmmodule.CallBlockOverride            `json:"block_override,omitempty"`
	SetCodeAuthorizationsJSON string                                 `json:"set_code_authorizations_json,omitempty"`
}

type web3TransactionCall struct {
	From                 string                `json:"from"`
	To                   string                `json:"to"`
	Data                 string                `json:"data"`
	Input                string                `json:"input"`
	Gas                  string                `json:"gas"`
	GasPrice             string                `json:"gasPrice"`
	MaxFeePerGas         string                `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string                `json:"maxPriorityFeePerGas"`
	Nonce                string                `json:"nonce"`
	Value                string                `json:"value"`
	VM                   string                `json:"vm"`
	Method               string                `json:"method"`
	AccessList           []web3AccessListEntry `json:"accessList,omitempty"`
	AuthorizationList    json.RawMessage       `json:"authorizationList,omitempty"`
}

type web3StateOverrideAccount struct {
	Balance   string            `json:"balance"`
	Nonce     string            `json:"nonce"`
	Code      string            `json:"code"`
	State     map[string]string `json:"state"`
	StateDiff map[string]string `json:"stateDiff"`
}

type web3BlockOverride struct {
	Number      string `json:"number"`
	Time        string `json:"time"`
	Timestamp   string `json:"timestamp"`
	GasLimit    string `json:"gasLimit"`
	BaseFee     string `json:"baseFeePerGas"`
	BlobBaseFee string `json:"blobBaseFee"`
}

type web3Receipt struct {
	TxHash          string `json:"tx_hash"`
	Height          uint64 `json:"height"`
	Status          uint32 `json:"status"`
	From            string `json:"from"`
	To              string `json:"to,omitempty"`
	ContractAddress string `json:"contract_address,omitempty"`
	GasUsed         uint64 `json:"gas_used"`
	Error           string `json:"error,omitempty"`
	Output          string `json:"output,omitempty"`
	Logs            []any  `json:"logs,omitempty"`
	StateDiff       any    `json:"state_diff,omitempty"`
	VMTrace         any    `json:"vm_trace,omitempty"`
}

type web3ReceiptIndex struct {
	TxHash  string `json:"tx_hash"`
	Height  uint64 `json:"height"`
	TxIndex uint64 `json:"tx_index"`
}

type web3EVMCallResponse struct {
	Output     string                `json:"output"`
	GasUsed    uint64                `json:"gas_used"`
	Failed     bool                  `json:"failed,omitempty"`
	Error      string                `json:"error,omitempty"`
	AccessList []web3AccessListEntry `json:"access_list,omitempty"`
	StateDiff  any                   `json:"state_diff,omitempty"`
	VMTrace    any                   `json:"vm_trace,omitempty"`
}

type web3AccessListEntry struct {
	Address     string   `json:"address"`
	StorageKeys []string `json:"storage_keys,omitempty"`
}

func (entry *web3AccessListEntry) UnmarshalJSON(data []byte) error {
	var payload struct {
		Address           string   `json:"address"`
		StorageKeys       []string `json:"storageKeys"`
		StorageKeysLegacy []string `json:"storage_keys"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	entry.Address = payload.Address
	entry.StorageKeys = append([]string(nil), payload.StorageKeys...)
	if len(entry.StorageKeys) == 0 {
		entry.StorageKeys = append([]string(nil), payload.StorageKeysLegacy...)
	}
	return nil
}

type web3AccountStateResponse struct {
	Address    string `json:"address"`
	Balance    uint64 `json:"balance"`
	BalanceHex string `json:"balance_hex"`
	Nonce      uint64 `json:"nonce"`
	Code       string `json:"code"`
}

func web3BlockByNumber(ctx context.Context, provider StatusProvider, params []json.RawMessage) (store.BlockRecord, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	if len(params) == 0 || len(params) > 2 {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "eth_getBlockByNumber requires block tag and optional full transaction flag"}
	}
	tag, err := jsonRPCStringParam(params[0])
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	var (
		record   store.BlockRecord
		queryErr error
	)
	switch tag {
	case "latest", "pending":
		record, queryErr = blockProvider.LatestBlock(ctx)
	case "safe", "finalized":
		height, rpcErr := web3FinalizedHeight(ctx, provider)
		if rpcErr != nil {
			return store.BlockRecord{}, rpcErr
		}
		record, queryErr = blockProvider.BlockByHeight(ctx, height)
	case "earliest":
		queryProvider, ok := provider.(ChainQueryProvider)
		if !ok {
			return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block index query is unavailable"}
		}
		index, err := queryProvider.BlockIndex(ctx)
		if err != nil {
			queryErr = err
			break
		}
		record, queryErr = blockProvider.BlockByHeight(ctx, index.EarliestHeight)
	default:
		height, err := parseHexQuantity(tag)
		if err != nil || height == 0 {
			return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "invalid block number tag"}
		}
		record, queryErr = blockProvider.BlockByHeight(ctx, types.Height(height))
	}
	if errors.Is(queryErr, store.ErrBlockNotFound) || errors.Is(queryErr, store.ErrBlockIndexNotFound) {
		return store.BlockRecord{}, nil
	}
	if queryErr != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: queryErr.Error()}
	}
	return record, nil
}

func web3BlockByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (store.BlockRecord, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	if len(params) == 0 || len(params) > 2 {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "eth_getBlockByHash requires block hash and optional full transaction flag"}
	}
	hashText, err := jsonRPCStringParam(params[0])
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	hash, err := parseHexHash(hashText)
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "invalid block hash"}
	}
	record, err := blockProvider.BlockByHash(ctx, hash)
	if errors.Is(err, store.ErrBlockNotFound) {
		return store.BlockRecord{}, nil
	}
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return record, nil
}

func web3FullTransactionParam(params []json.RawMessage, index int) bool {
	if len(params) <= index {
		return false
	}
	var full bool
	_ = json.Unmarshal(params[index], &full)
	return full
}

func web3BlockTransactionCount(record store.BlockRecord) any {
	if record.Block.Header.Height == 0 {
		return nil
	}
	return hexQuantity(uint64(len(record.Block.Txs)))
}

func web3PendingSequence(ctx context.Context, provider StatusProvider, address string, sequence uint64) (uint64, error) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		if rpcErr.Message == "pending transaction query is unavailable" {
			return sequence, nil
		}
		return sequence, errors.New(rpcErr.Message)
	}
	candidates := make([]uint64, 0)
	for _, tx := range txs {
		details := web3TransactionDetails(tx)
		from, ok := details.From.(string)
		if !ok || !strings.EqualFold(from, address) {
			continue
		}
		candidates = append(candidates, details.Nonce)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	next := sequence
	for _, nonce := range candidates {
		if nonce < next {
			continue
		}
		if nonce > next {
			break
		}
		next++
	}
	return next, nil
}

func web3AccountState(ctx context.Context, provider StatusProvider, address string, params []json.RawMessage, tagIndex int) (web3AccountStateResponse, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		if tagIndex >= 0 && len(params) > tagIndex {
			tag, err := jsonRPCStringParam(params[tagIndex])
			if err == nil && tag != "" && tag != "latest" && tag != "pending" {
				return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: "historical EVM account query is unavailable"}
			}
		}
		accountProvider, ok := provider.(AccountQueryProvider)
		if !ok {
			return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: "account query is unavailable"}
		}
		sequence, err := accountProvider.AccountSequence(ctx, types.Address(address))
		if err != nil {
			return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return web3AccountStateResponse{Address: address, Nonce: sequence}, nil
	}
	data, rpcErr := web3HistoricalAccountQueryData(ctx, provider, params, tagIndex)
	if rpcErr != nil {
		return web3AccountStateResponse{}, rpcErr
	}
	response, err := query.AppQuery(ctx, []string{"evm", "account", address}, data)
	if err != nil {
		return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return web3AccountStateResponse{Address: address}, nil
	}
	if response.Code != 0 {
		return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var account web3AccountStateResponse
	if err := json.Unmarshal(response.Value, &account); err != nil {
		return web3AccountStateResponse{}, &JSONRPCError{Code: -32000, Message: "invalid EVM account response"}
	}
	return account, nil
}

func web3TransactionByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByHash requires one transaction hash"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if query, ok := provider.(AppQueryProvider); ok {
		response, err := query.AppQuery(ctx, []string{"evm", "receipt", hash}, nil)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if response.Code != 3 {
			if response.Code != 0 {
				return nil, &JSONRPCError{Code: -32000, Message: response.Log}
			}
			return web3TransactionFromReceipt(ctx, provider, response.Value)
		}
	}
	pending, rpcErr := web3PendingTransactionByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if pending != nil {
		return pending, nil
	}
	return web3CommittedTransactionByHash(ctx, provider, hash)
}

func web3TransactionByBlockNumberAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByBlockNumberAndIndex requires block tag and transaction index"}
	}
	record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3TransactionByBlockIndex(record, params[1])
}

func web3TransactionByBlockHashAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getTransactionByBlockHashAndIndex requires block hash and transaction index"}
	}
	record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3TransactionByBlockIndex(record, params[1])
}

func web3TransactionByBlockIndex(record store.BlockRecord, rawIndex json.RawMessage) (any, *JSONRPCError) {
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	indexText, err := jsonRPCStringParam(rawIndex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	index, err := parseHexQuantity(indexText)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid transaction index"}
	}
	if index >= uint64(len(record.Block.Txs)) {
		return nil, nil
	}
	tx := record.Block.Txs[index]
	return web3TransactionFromBlockRecord(record, int(index), web3TxHash(tx), tx), nil
}

func web3PendingTransactionByHash(ctx context.Context, provider StatusProvider, hash string) (any, *JSONRPCError) {
	tx, found, rpcErr := web3PendingTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3PendingTransaction(tx), nil
}

func web3PendingTxByHash(ctx context.Context, provider StatusProvider, hash string) (types.Tx, bool, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		if rpcErr.Message == "pending transaction query is unavailable" {
			return nil, false, nil
		}
		return nil, false, rpcErr
	}
	for _, tx := range txs {
		if web3TxMatchesHash(tx, hash) {
			return tx, true, nil
		}
	}
	return nil, false, nil
}

func web3TxMatchesHash(tx types.Tx, hash string) bool {
	return strings.EqualFold(web3TxHash(tx), hash) || strings.EqualFold(web3HashString(mempool.HashTx(tx)), hash)
}

func web3CommittedTransactionByHash(ctx context.Context, provider StatusProvider, hash string) (any, *JSONRPCError) {
	record, index, tx, found, rpcErr := web3CommittedTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3TransactionFromBlockRecord(record, index, web3TxHash(tx), tx), nil
}

func web3CommittedTxByHash(ctx context.Context, provider StatusProvider, hash string) (store.BlockRecord, int, types.Tx, bool, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, 0, nil, false, nil
	}
	if index, found, rpcErr := web3ReceiptIndexByHash(ctx, provider, hash); rpcErr != nil || found {
		if rpcErr != nil {
			return store.BlockRecord{}, 0, nil, false, rpcErr
		}
		record, err := blockProvider.BlockByHeight(ctx, types.Height(index.Height))
		if errors.Is(err, store.ErrBlockNotFound) {
			return store.BlockRecord{}, 0, nil, false, nil
		}
		if err != nil {
			return store.BlockRecord{}, 0, nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if index.TxIndex < uint64(len(record.Block.Txs)) {
			tx := record.Block.Txs[index.TxIndex]
			if strings.EqualFold(index.TxHash, hash) || web3TxMatchesHash(tx, hash) {
				return record, int(index.TxIndex), append(types.Tx(nil), tx...), true, nil
			}
		}
	}
	status := provider.Status(ctx)
	for height := status.LatestHeight; height > 0; height-- {
		record, err := blockProvider.BlockByHeight(ctx, height)
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return store.BlockRecord{}, 0, nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		for index, tx := range record.Block.Txs {
			if web3TxMatchesHash(tx, hash) {
				return record, index, tx, true, nil
			}
		}
	}
	return store.BlockRecord{}, 0, nil, false, nil
}

func web3RawTransactionByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getRawTransactionByHash requires one transaction hash"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if raw, rpcErr := web3PendingRawTransactionByHash(ctx, provider, hash); rpcErr != nil || raw != nil {
		return raw, rpcErr
	}
	_, _, tx, found, rpcErr := web3CommittedTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3RawTransaction(tx), nil
}

func web3PendingRawTransactionByHash(ctx context.Context, provider StatusProvider, hash string) (any, *JSONRPCError) {
	tx, found, rpcErr := web3PendingTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, rpcErr
	}
	return web3RawTransaction(tx), nil
}

func web3RawTransactionByBlockNumberAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getRawTransactionByBlockNumberAndIndex requires block tag and transaction index"}
	}
	record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3RawTransactionByBlockIndex(record, params[1])
}

func web3RawTransactionByBlockHashAndIndex(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getRawTransactionByBlockHashAndIndex requires block hash and transaction index"}
	}
	record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3RawTransactionByBlockIndex(record, params[1])
}

func web3RawTransactionByBlockIndex(record store.BlockRecord, rawIndex json.RawMessage) (any, *JSONRPCError) {
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	indexText, err := jsonRPCStringParam(rawIndex)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	index, err := parseHexQuantity(indexText)
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid transaction index"}
	}
	if index >= uint64(len(record.Block.Txs)) {
		return nil, nil
	}
	return web3RawTransaction(record.Block.Txs[index]), nil
}

func web3RawTransaction(tx types.Tx) any {
	if raw, found := vexoapp.TxTag(tx, ethcompat.TagRaw); found && raw != "" {
		return "0x" + strings.TrimPrefix(raw, "0x")
	}
	return "0x" + hex.EncodeToString(tx)
}

func web3BlockFromRecord(ctx context.Context, provider StatusProvider, cfg Config, record store.BlockRecord, fullTransactions bool) (any, *JSONRPCError) {
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	stateRoot, ok := web3StateRoot(ctx, provider, cfg, record)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "EVM state root is unavailable for block"}
	}
	transactions := make([]any, 0, len(record.Block.Txs))
	for index, tx := range record.Block.Txs {
		hashText := web3TxHash(tx)
		if !fullTransactions {
			transactions = append(transactions, hashText)
			continue
		}
		transactions = append(transactions, web3TransactionFromBlockRecord(record, index, hashText, tx))
	}
	return map[string]any{
		"number":           hexQuantity(uint64(record.Block.Header.Height)),
		"hash":             "0x" + hex.EncodeToString(record.Hash[:]),
		"parentHash":       "0x" + hex.EncodeToString(record.Block.Header.PreviousBlockHash[:]),
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"mixHash":          "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logsBloom":        web3LogsBloom(record.Block.Txs, record.TxResults),
		"transactionsRoot": web3TransactionsRoot(record.Block.Txs),
		"stateRoot":        stateRoot,
		"receiptsRoot":     web3ReceiptsRoot(record.Block.Txs, record.TxResults),
		"miner":            "0x0000000000000000000000000000000000000000",
		"difficulty":       "0x0",
		"totalDifficulty":  "0x0",
		"extraData":        "0x",
		"size":             hexQuantity(uint64(len(record.Block.Txs))),
		"gasLimit":         web3BlockGasLimit(record.TxResults),
		"gasUsed":          hexQuantity(web3BlockGasUsed(record.TxResults)),
		"baseFeePerGas":    hexQuantity(web3BlockBaseFee(ctx, provider, record)),
		"blobGasUsed":      hexQuantity(web3BlockBlobGasUsed(ctx, provider, record)),
		"excessBlobGas":    hexQuantity(web3BlockExcessBlobGas(ctx, provider, record)),
		"timestamp":        hexQuantity(uint64(record.Block.Header.TimeUnixNano / int64(time.Second))),
		"transactions":     transactions,
		"uncles":           []any{},
		"withdrawals":      []any{},
		"withdrawalsRoot":  "0x0000000000000000000000000000000000000000000000000000000000000000",
	}, nil
}

func web3GetProof(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	if len(params) < 2 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getProof requires address, storage keys, and optional block tag"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	var storageKeys []string
	if err := json.Unmarshal(params[1], &storageKeys); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "storage keys array is required"}
	}
	height := types.Height(0)
	if len(params) == 3 {
		var rpcErr *JSONRPCError
		height, rpcErr = web3BlockHeightParam(ctx, provider, params[2])
		if rpcErr != nil {
			return nil, rpcErr
		}
	}
	request := map[string]any{"address": address, "storage_keys": storageKeys}
	if height > 0 {
		request["height"] = uint64(height)
	}
	encoded, _ := json.Marshal(request)
	response, err := query.AppQuery(ctx, []string{"evm", "eth_proof"}, encoded)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code != 0 {
		return nil, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	payload, rpcErr := rawJSONObject(response.Value)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var result any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM proof response"}
	}
	return result, nil
}

func web3StateRoot(ctx context.Context, provider StatusProvider, _ Config, record store.BlockRecord) (string, bool) {
	if record.Block.Header.Height == provider.Status(ctx).LatestHeight {
		if query, ok := provider.(AppQueryProvider); ok {
			payload, _ := json.Marshal(map[string]uint64{"height": uint64(record.Block.Header.Height)})
			response, err := query.AppQuery(ctx, []string{"evm", "eth_state_root"}, payload)
			if err == nil && response.Code == 0 {
				var payload struct {
					StateRoot string `json:"state_root"`
				}
				if json.Unmarshal(response.Value, &payload) == nil && payload.StateRoot != "" {
					return payload.StateRoot, true
				}
			}
		}
	} else if query, ok := provider.(AppQueryProvider); ok {
		payload, _ := json.Marshal(map[string]uint64{"height": uint64(record.Block.Header.Height)})
		response, err := query.AppQuery(ctx, []string{"evm", "eth_state_root"}, payload)
		if err == nil && response.Code == 0 {
			var payload struct {
				StateRoot string `json:"state_root"`
			}
			if json.Unmarshal(response.Value, &payload) == nil && payload.StateRoot != "" {
				return payload.StateRoot, true
			}
		}
	}
	return "", false
}

func web3BlockBaseFee(ctx context.Context, provider StatusProvider, record store.BlockRecord) uint64 {
	if stateProvider, ok := provider.(StateByHeightProvider); ok && record.Block.Header.Height != 0 {
		state, err := stateProvider.StateByHeight(ctx, record.Block.Header.Height)
		if err == nil {
			if state.BaseFee > 0 {
				return state.BaseFee
			}
			return state.NextBaseFee
		}
	}
	query, ok := provider.(ChainQueryProvider)
	if !ok {
		return 0
	}
	state, err := query.LatestState(ctx)
	if err != nil {
		return 0
	}
	if state.Height != 0 && record.Block.Header.Height != 0 && state.Height != record.Block.Header.Height {
		return 0
	}
	if state.BaseFee > 0 {
		return state.BaseFee
	}
	return state.NextBaseFee
}

func web3LatestBlobBaseFee(ctx context.Context, provider StatusProvider) uint64 {
	if query, ok := provider.(ChainQueryProvider); ok {
		state, err := query.LatestState(ctx)
		if err == nil {
			if state.NextBlobBaseFee > 0 {
				return state.NextBlobBaseFee
			}
			return state.BlobBaseFee
		}
	}
	return 0
}

func web3BaseFeeAtHeight(ctx context.Context, provider StatusProvider, height types.Height) uint64 {
	if height == 0 {
		return web3LatestBaseFee(ctx, provider)
	}
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, height)
		if err == nil {
			if state.BaseFee > 0 {
				return state.BaseFee
			}
			return state.NextBaseFee
		}
	}
	if blockProvider, ok := provider.(BlockProvider); ok {
		record, err := blockProvider.BlockByHeight(ctx, height)
		if err == nil {
			return web3BlockBaseFee(ctx, provider, record)
		}
	}
	return 0
}

func web3NextBaseFeeAfterHeight(ctx context.Context, provider StatusProvider, height types.Height) uint64 {
	if height == 0 {
		return web3LatestBaseFee(ctx, provider)
	}
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, height)
		if err == nil && state.NextBaseFee > 0 {
			return state.NextBaseFee
		}
		nextState, err := query.StateByHeight(ctx, height+1)
		if err == nil && nextState.BaseFee > 0 {
			return nextState.BaseFee
		}
	}
	return web3BaseFeeAtHeight(ctx, provider, height)
}

func web3BlobBaseFeeAtHeight(ctx context.Context, provider StatusProvider, height types.Height) uint64 {
	if height == 0 {
		return web3LatestBlobBaseFee(ctx, provider)
	}
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, height)
		if err == nil {
			if state.BlobBaseFee > 0 {
				return state.BlobBaseFee
			}
			return state.NextBlobBaseFee
		}
	}
	return web3LatestBlobBaseFee(ctx, provider)
}

func web3BlockBlobGasUsed(ctx context.Context, provider StatusProvider, record store.BlockRecord) uint64 {
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, record.Block.Header.Height)
		if err == nil {
			return state.BlobGasUsed
		}
	}
	return web3TxsBlobGas(record.Block.Txs)
}

func web3BlockExcessBlobGas(ctx context.Context, provider StatusProvider, record store.BlockRecord) uint64 {
	if query, ok := provider.(StateByHeightProvider); ok {
		state, err := query.StateByHeight(ctx, record.Block.Header.Height)
		if err == nil {
			return state.ExcessBlobGas
		}
	}
	return 0
}

func web3TxsBlobGas(txs []types.Tx) uint64 {
	var total uint64
	for _, tx := range txs {
		blobGas, found := vexoapp.TxUintTag(tx, ethcompat.TagBlobGas)
		if !found {
			continue
		}
		if total > ^uint64(0)-blobGas {
			return ^uint64(0)
		}
		total += blobGas
	}
	return total
}

func web3MaxPriorityFeePerGas(ctx context.Context, provider StatusProvider) string {
	if query, ok := provider.(ChainQueryProvider); ok {
		state, err := query.LatestState(ctx)
		if err == nil && (state.BaseFee > 0 || state.NextBaseFee > 0) {
			return hexQuantity(1)
		}
	}
	return hexQuantity(0)
}

func web3FeeHistory(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) < 2 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_feeHistory requires block count, newest block, and optional reward percentiles"}
	}
	blockCount, err := web3QuantityParam(params[0])
	if err != nil || blockCount == 0 {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid fee history block count"}
	}
	if blockCount > 1024 {
		blockCount = 1024
	}
	newest, rpcErr := web3BlockHeightParam(ctx, provider, params[1])
	if rpcErr != nil {
		return nil, rpcErr
	}
	oldest := uint64(0)
	if uint64(newest) >= blockCount {
		oldest = uint64(newest) - blockCount + 1
	}
	baseFees := make([]string, 0, blockCount+1)
	gasUsedRatios := make([]float64, 0, blockCount)
	for index := uint64(0); index < blockCount; index++ {
		height := types.Height(oldest + index)
		baseFees = append(baseFees, hexQuantity(web3BaseFeeAtHeight(ctx, provider, height)))
		gasUsedRatios = append(gasUsedRatios, web3BlockGasUsedRatio(ctx, provider, height))
	}
	baseFees = append(baseFees, hexQuantity(web3NextBaseFeeAfterHeight(ctx, provider, newest)))
	response := map[string]any{
		"oldestBlock":   hexQuantity(oldest),
		"baseFeePerGas": baseFees,
		"gasUsedRatio":  gasUsedRatios,
	}
	if len(params) == 3 && string(params[2]) != "null" {
		percentiles, err := web3RewardPercentiles(params[2])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		reward := make([][]string, 0, blockCount)
		for range blockCount {
			row := make([]string, len(percentiles))
			for index := range row {
				row[index] = web3MaxPriorityFeePerGas(ctx, provider)
			}
			reward = append(reward, row)
		}
		response["reward"] = reward
	}
	return response, nil
}

func web3BlockReceipts(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "eth_getBlockReceipts requires block hash or block tag"}
	}
	record, rpcErr := web3BlockRecordParam(ctx, provider, params[0])
	if rpcErr != nil {
		return nil, rpcErr
	}
	if record.Block.Header.Height == 0 {
		return nil, nil
	}
	receipts := make([]any, 0, len(record.TxResults))
	for _, result := range record.TxResults {
		if _, ok := web3ReceiptFromResult(result); !ok {
			continue
		}
		receipt, rpcErr := web3ReceiptObject(ctx, provider, result.Data)
		if rpcErr != nil {
			return nil, rpcErr
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func web3ReceiptValueByHash(ctx context.Context, provider StatusProvider, hash string) ([]byte, bool, *JSONRPCError) {
	if query, ok := provider.(AppQueryProvider); ok {
		response, err := query.AppQuery(ctx, []string{"evm", "receipt", hash}, nil)
		if err != nil {
			return nil, false, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		if response.Code != 3 {
			if response.Code != 0 {
				return nil, false, &JSONRPCError{Code: -32000, Message: response.Log}
			}
			return response.Value, true, nil
		}
	}
	record, index, _, found, rpcErr := web3CommittedTxByHash(ctx, provider, hash)
	if rpcErr != nil || !found {
		return nil, false, rpcErr
	}
	if index >= len(record.TxResults) {
		return nil, false, nil
	}
	if _, ok := web3ReceiptFromResult(record.TxResults[index]); !ok {
		return nil, false, nil
	}
	return record.TxResults[index].Data, true, nil
}

func web3ReceiptIndexByHash(ctx context.Context, provider StatusProvider, hash string) (web3ReceiptIndex, bool, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return web3ReceiptIndex{}, false, nil
	}
	response, err := query.AppQuery(ctx, []string{"evm", "receipt_index", hash}, nil)
	if err != nil {
		return web3ReceiptIndex{}, false, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return web3ReceiptIndex{}, false, nil
	}
	if response.Code != 0 {
		return web3ReceiptIndex{}, false, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var index web3ReceiptIndex
	if err := json.Unmarshal(response.Value, &index); err != nil {
		return web3ReceiptIndex{}, false, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt index response"}
	}
	if index.TxHash == "" || index.Height == 0 {
		return web3ReceiptIndex{}, false, nil
	}
	return index, true, nil
}

func web3TxpoolStatus(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		return web3PendingTransaction(tx)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"pending": hexQuantity(uint64(web3TxpoolBucketCount(pending))),
		"queued":  hexQuantity(uint64(web3TxpoolBucketCount(queued))),
	}, nil
}

func web3TxpoolContent(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		return web3PendingTransaction(tx)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{"pending": pending, "queued": queued}, nil
}

func web3TxpoolContentFrom(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "txpool_contentFrom requires one address"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		return web3PendingTransaction(tx)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"pending": web3TxpoolBucketForAddress(pending, address),
		"queued":  web3TxpoolBucketForAddress(queued, address),
	}, nil
}

func web3TxpoolInspect(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	pending, queued, rpcErr := web3TxpoolBuckets(ctx, provider, txs, func(tx types.Tx) any {
		details := web3TransactionDetails(tx)
		to := "<contract creation>"
		if raw, ok := details.To.(string); ok && raw != "" {
			to = raw
		}
		return fmt.Sprintf("%s: %s value + %d gas × %d wei", to, web3TxValueDecimal(details), details.Gas, details.GasPrice)
	})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{"pending": pending, "queued": queued}, nil
}

func web3TxpoolBuckets(ctx context.Context, provider StatusProvider, txs []types.Tx, item func(types.Tx) any) (map[string]map[string]any, map[string]map[string]any, *JSONRPCError) {
	grouped := make(map[string][]types.Tx)
	for _, tx := range txs {
		from := web3TxpoolSender(tx)
		grouped[from] = append(grouped[from], tx)
	}
	pending := make(map[string]map[string]any)
	queued := make(map[string]map[string]any)
	for from, items := range grouped {
		sort.SliceStable(items, func(i, j int) bool {
			return web3TransactionDetails(items[i]).Nonce < web3TransactionDetails(items[j]).Nonce
		})
		account, rpcErr := web3AccountState(ctx, provider, from, []json.RawMessage{json.RawMessage(strconv.Quote(from)), json.RawMessage(`"latest"`)}, 1)
		if rpcErr != nil {
			return nil, nil, rpcErr
		}
		expected := account.Nonce
		for _, tx := range items {
			details := web3TransactionDetails(tx)
			target := queued
			if details.Nonce == expected {
				target = pending
				expected++
			}
			if target[from] == nil {
				target[from] = make(map[string]any)
			}
			target[from][hexQuantity(details.Nonce)] = item(tx)
		}
	}
	return pending, queued, nil
}

func web3TxpoolSender(tx types.Tx) string {
	details := web3TransactionDetails(tx)
	if raw, ok := details.From.(string); ok && raw != "" {
		return raw
	}
	return "0x0000000000000000000000000000000000000000"
}

func web3TxpoolBucketCount(bucket map[string]map[string]any) int {
	count := 0
	for _, byNonce := range bucket {
		count += len(byNonce)
	}
	return count
}

func web3TxpoolBucketForAddress(bucket map[string]map[string]any, address string) map[string]any {
	for from, byNonce := range bucket {
		if strings.EqualFold(from, address) {
			return byNonce
		}
	}
	return map[string]any{}
}

func web3PendingTransactions(ctx context.Context, provider StatusProvider) (any, *JSONRPCError) {
	txs, rpcErr := web3PendingTxs(ctx, provider)
	if rpcErr != nil {
		return nil, rpcErr
	}
	transactions := make([]any, 0, len(txs))
	for _, tx := range txs {
		transactions = append(transactions, web3PendingTransaction(tx))
	}
	return transactions, nil
}

func web3PendingTxs(ctx context.Context, provider StatusProvider) ([]types.Tx, *JSONRPCError) {
	pendingProvider, ok := provider.(PendingTxsProvider)
	if !ok {
		hashProvider, ok := provider.(PendingTxProvider)
		if !ok {
			return nil, &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
		}
		hashes, err := hashProvider.PendingTxHashes(ctx)
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		txs := make([]types.Tx, 0, len(hashes))
		for _, hash := range hashes {
			txs = append(txs, types.Tx("hash:"+web3HashString(hash)))
		}
		return txs, nil
	}
	txs, err := pendingProvider.PendingTxs(ctx)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return txs, nil
}

func web3PendingTransaction(tx types.Tx) any {
	details := web3TransactionDetails(tx)
	transaction := map[string]any{
		"hash":             web3TxHash(tx),
		"nonce":            hexQuantity(details.Nonce),
		"blockHash":        nil,
		"blockNumber":      nil,
		"transactionIndex": nil,
		"from":             details.From,
		"to":               details.To,
		"value":            web3TxValueHex(details),
		"gas":              hexQuantity(details.Gas),
		"gasPrice":         web3TxGasPriceHex(details),
		"input":            details.Input,
		"type":             hexQuantity(details.Type),
		"chainId":          hexQuantity(details.ChainID),
	}
	if details.MaxFeePerGas > 0 || details.MaxFeePerGasHex != "" {
		transaction["maxFeePerGas"] = web3TxMaxFeePerGasHex(details)
	}
	if details.MaxPriorityFeePerGas > 0 || details.MaxPriorityFeePerGasHex != "" {
		transaction["maxPriorityFeePerGas"] = web3TxMaxPriorityFeePerGasHex(details)
	}
	return transaction
}

func web3DebugTraceTransaction(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceTransaction requires transaction hash and optional config"}
	}
	tracer, rpcErr := web3DebugTracer(params, 1)
	if rpcErr != nil {
		return nil, rpcErr
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !found {
		return nil, nil
	}
	receipt, ok := web3ReceiptFromResult(types.Result{Data: value})
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt"}
	}
	if tracer == "callTracer" {
		return web3ReceiptCallTrace(ctx, provider, receipt), nil
	}
	if tracer == "prestateTracer" {
		return web3ReceiptPrestateTrace(ctx, provider, receipt), nil
	}
	if tracer == "4byteTracer" {
		return web3Receipt4ByteTrace(ctx, provider, receipt), nil
	}
	return web3ReceiptDebugTraceResult(receipt), nil
}

func web3DebugTraceBlockByNumber(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceBlockByNumber requires block tag and optional config"}
	}
	record, rpcErr := web3BlockByNumber(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3DebugTraceBlockRecord(record), nil
}

func web3DebugTraceBlockByHash(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceBlockByHash requires block hash and optional config"}
	}
	record, rpcErr := web3BlockByHash(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3DebugTraceBlockRecord(record), nil
}

func web3DebugTraceBlockRecord(record store.BlockRecord) any {
	if record.Block.Header.Height == 0 {
		return nil
	}
	traces := make([]any, 0, len(record.TxResults))
	for index, result := range record.TxResults {
		receipt, ok := web3ReceiptFromResult(result)
		if !ok {
			continue
		}
		item := map[string]any{
			"txHash":  receipt.TxHash,
			"result":  web3ReceiptDebugTraceResult(receipt),
			"vmTrace": web3VMTraceFromReceipt(receipt),
		}
		if index < len(record.Block.Txs) {
			item["transaction"] = web3TransactionFromBlockRecord(record, index, receipt.TxHash, record.Block.Txs[index])
		}
		traces = append(traces, item)
	}
	return traces
}

func web3TraceTransaction(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_transaction requires one transaction hash"}
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !found {
		return []any{}, nil
	}
	receipt, ok := web3ReceiptFromResult(types.Result{Data: value})
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt"}
	}
	traces := web3TraceListFromReceipt(ctx, provider, receipt)
	if len(traces) == 0 {
		return []any{}, nil
	}
	return traces, nil
}

func web3TraceGet(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_get requires transaction hash and trace address"}
	}
	traces, rpcErr := web3TraceTransaction(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	items, ok := traces.([]any)
	if !ok || len(items) == 0 {
		return nil, nil
	}
	var traceAddress []uint64
	if err := json.Unmarshal(params[1], &traceAddress); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid trace address"}
	}
	if len(traceAddress) == 0 {
		return items[0], nil
	}
	for _, item := range items {
		trace, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if web3TraceAddressEqual(trace["traceAddress"], traceAddress) {
			return trace, nil
		}
	}
	return nil, nil
}

func web3TraceBlock(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_block requires one block hash or tag"}
	}
	record, rpcErr := web3BlockRecordParam(ctx, provider, params[0])
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3TraceBlockRecord(ctx, provider, record), nil
}

func web3TraceFilter(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) != 1 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_filter requires one filter object"}
	}
	var filter struct {
		FromBlock   json.RawMessage `json:"fromBlock"`
		ToBlock     json.RawMessage `json:"toBlock"`
		FromAddress any             `json:"fromAddress"`
		ToAddress   any             `json:"toAddress"`
		After       uint64          `json:"after"`
		Count       uint64          `json:"count"`
	}
	if err := json.Unmarshal(params[0], &filter); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid trace filter"}
	}
	fromAddresses, rpcErr := web3LogAddresses(filter.FromAddress)
	if rpcErr != nil {
		return nil, rpcErr
	}
	toAddresses, rpcErr := web3LogAddresses(filter.ToAddress)
	if rpcErr != nil {
		return nil, rpcErr
	}
	from := provider.Status(ctx).LatestHeight
	to := from
	if len(filter.FromBlock) > 0 {
		height, rpcErr := web3BlockHeightParam(ctx, provider, filter.FromBlock)
		if rpcErr != nil {
			return nil, rpcErr
		}
		from = height
	}
	if len(filter.ToBlock) > 0 {
		height, rpcErr := web3BlockHeightParam(ctx, provider, filter.ToBlock)
		if rpcErr != nil {
			return nil, rpcErr
		}
		to = height
	}
	if to < from {
		return []any{}, nil
	}
	if uint64(to-from) > 127 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_filter block range exceeds 128 blocks"}
	}
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	traces := make([]any, 0)
	for height := from; height <= to; height++ {
		record, err := blockProvider.BlockByHeight(ctx, height)
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		items := web3TraceBlockRecord(ctx, provider, record)
		for _, item := range items {
			if !web3TraceMatchesFilter(item, fromAddresses, toAddresses) {
				continue
			}
			traces = append(traces, item)
		}
	}
	return web3TracePage(traces, filter.After, filter.Count), nil
}

func web3TraceMatchesFilter(raw any, fromAddresses []string, toAddresses []string) bool {
	trace, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	action, _ := trace["action"].(map[string]any)
	if len(fromAddresses) > 0 && !web3TraceActionAddressMatches(action["from"], fromAddresses) {
		return false
	}
	if len(toAddresses) > 0 && !web3TraceActionAddressMatches(action["to"], toAddresses) {
		return false
	}
	return true
}

func web3TraceActionAddressMatches(raw any, addresses []string) bool {
	address, ok := raw.(string)
	if !ok || address == "" {
		return false
	}
	for _, candidate := range addresses {
		if strings.EqualFold(address, candidate) {
			return true
		}
	}
	return false
}

func web3TracePage(traces []any, after uint64, count uint64) []any {
	if after >= uint64(len(traces)) {
		return []any{}
	}
	start := int(after)
	end := len(traces)
	if count > 0 && count < uint64(end-start) {
		end = start + int(count)
	}
	return traces[start:end]
}

func web3TraceReplayTransaction(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_replayTransaction requires transaction hash and optional trace types"}
	}
	replayTypes, rpcErr := web3ReplayTypes(params, 1)
	if rpcErr != nil {
		return nil, rpcErr
	}
	traces, rpcErr := web3TraceTransaction(ctx, provider, []json.RawMessage{params[0]})
	if rpcErr != nil {
		return nil, rpcErr
	}
	hash, err := jsonRPCStringParam(params[0])
	if err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	value, found, rpcErr := web3ReceiptValueByHash(ctx, provider, hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !found {
		return web3ReplayResponse("0x", traces, replayTypes, nil, nil), nil
	}
	receipt, ok := web3ReceiptFromResult(types.Result{Data: value})
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt"}
	}
	return web3ReplayResponse(receipt.Output, traces, replayTypes, receipt.StateDiff, web3VMTraceFromReceipt(receipt)), nil
}

func web3TraceReplayBlockTransactions(ctx context.Context, provider StatusProvider, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 2 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_replayBlockTransactions requires block hash/tag and optional trace types"}
	}
	replayTypes, rpcErr := web3ReplayTypes(params, 1)
	if rpcErr != nil {
		return nil, rpcErr
	}
	record, rpcErr := web3BlockRecordParam(ctx, provider, params[0])
	if rpcErr != nil {
		return nil, rpcErr
	}
	if record.Block.Header.Height == 0 {
		return []any{}, nil
	}
	responses := make([]any, 0, len(record.TxResults))
	for _, result := range record.TxResults {
		receipt, ok := web3ReceiptFromResult(result)
		if !ok {
			continue
		}
		trace := web3TraceFromReceipt(ctx, provider, receipt)
		if trace == nil {
			continue
		}
		response := web3ReplayResponse(receipt.Output, []any{trace}, replayTypes, receipt.StateDiff, web3VMTraceFromReceipt(receipt))
		response["transactionHash"] = receipt.TxHash
		responses = append(responses, response)
	}
	return responses, nil
}

func web3ReplayTypes(params []json.RawMessage, index int) (map[string]bool, *JSONRPCError) {
	types := map[string]bool{"trace": true, "stateDiff": true, "vmTrace": true}
	if len(params) <= index || string(params[index]) == "null" {
		return types, nil
	}
	var requested []string
	if err := json.Unmarshal(params[index], &requested); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid replay trace types"}
	}
	types = make(map[string]bool, len(requested))
	for _, item := range requested {
		if strings.TrimSpace(item) != "" {
			types[item] = true
		}
	}
	return types, nil
}

func web3ReplayResponse(output string, traces any, types map[string]bool, stateDiff any, vmTrace any) map[string]any {
	response := map[string]any{"output": output}
	if types["trace"] {
		response["trace"] = traces
	}
	if types["stateDiff"] {
		if stateDiff == nil {
			stateDiff = map[string]any{}
		}
		response["stateDiff"] = stateDiff
	}
	if types["vmTrace"] {
		response["vmTrace"] = vmTrace
	}
	for item := range types {
		switch item {
		case "trace", "stateDiff", "vmTrace":
		default:
			response[item] = nil
		}
	}
	return response
}

func web3VMTraceFromReceipt(receipt web3Receipt) any {
	if receipt.VMTrace != nil {
		return receipt.VMTrace
	}
	return nil
}

func web3ReceiptDebugTraceResult(receipt web3Receipt) map[string]any {
	return map[string]any{
		"gas":         receipt.GasUsed,
		"failed":      receipt.Status == 0,
		"returnValue": strings.TrimPrefix(receipt.Output, "0x"),
		"structLogs":  web3StructLogs(receipt.VMTrace),
	}
}

func web3ReceiptCallTrace(ctx context.Context, provider StatusProvider, receipt web3Receipt) map[string]any {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	details := web3TxDetails{}
	if found {
		details = web3TransactionDetails(tx)
	}
	to := receipt.To
	traceType := "CALL"
	if receipt.ContractAddress != "" && receipt.To == "" {
		to = receipt.ContractAddress
		traceType = "CREATE"
	}
	result := map[string]any{
		"type":    traceType,
		"from":    receipt.From,
		"to":      to,
		"value":   web3TxValueHex(details),
		"gas":     hexQuantity(details.Gas),
		"gasUsed": hexQuantity(receipt.GasUsed),
		"input":   details.Input,
		"output":  receipt.Output,
	}
	if !found {
		result["value"] = "0x0"
		result["gas"] = hexQuantity(receipt.GasUsed)
		result["input"] = "0x"
	}
	if calls := web3CallTraceChildren(receipt.VMTrace); len(calls) > 0 {
		result["calls"] = calls
	}
	if receipt.Status == 0 {
		result["error"] = web3ReceiptError(receipt)
	}
	return result
}

func web3ReceiptPrestateTrace(ctx context.Context, provider StatusProvider, receipt web3Receipt) map[string]any {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	details := web3TxDetails{Input: "0x"}
	if found {
		details = web3TransactionDetails(tx)
	}
	out := map[string]any{}
	if receipt.From != "" {
		out[strings.ToLower(receipt.From)] = map[string]any{
			"balance": "0x0",
			"nonce":   hexQuantity(details.Nonce),
			"code":    "0x",
			"storage": map[string]any{},
		}
	}
	to := receipt.To
	if to == "" {
		to = receipt.ContractAddress
	}
	if to != "" {
		out[strings.ToLower(to)] = map[string]any{
			"balance": "0x0",
			"nonce":   "0x0",
			"code":    "0x",
			"storage": map[string]any{},
		}
	}
	return out
}

func web3Receipt4ByteTrace(ctx context.Context, provider StatusProvider, receipt web3Receipt) map[string]any {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return map[string]any{}
	}
	return web3SelectorTrace(web3TransactionDetails(tx).Input)
}

func web3CallPrestateTrace(call web3CallRequest) map[string]any {
	out := map[string]any{}
	if call.From != "" {
		out[strings.ToLower(call.From)] = web3CallPrestateAccount(call.From, call.StateOverrides)
	}
	if call.To != "" {
		out[strings.ToLower(call.To)] = web3CallPrestateAccount(call.To, call.StateOverrides)
	}
	return out
}

func web3CallPrestateAccount(address string, overrides map[string]evmmodule.CallStateOverride) map[string]any {
	account := map[string]any{
		"balance": "0x0",
		"nonce":   "0x0",
		"code":    "0x",
		"storage": map[string]any{},
	}
	override, ok := web3StateOverrideForAddress(address, overrides)
	if !ok {
		return account
	}
	if override.Balance != "" {
		account["balance"] = normalizeHexQuantityString(override.Balance)
	}
	if override.Nonce != nil {
		account["nonce"] = hexQuantity(*override.Nonce)
	}
	if override.Code != "" {
		account["code"] = normalizeHexDataString(override.Code)
	}
	storage := make(map[string]any)
	for slot, value := range override.State {
		storage[normalizeStorageHex(slot)] = normalizeStorageHex(value)
	}
	for slot, value := range override.StateDiff {
		storage[normalizeStorageHex(slot)] = normalizeStorageHex(value)
	}
	account["storage"] = storage
	return account
}

func web3StateOverrideForAddress(address string, overrides map[string]evmmodule.CallStateOverride) (evmmodule.CallStateOverride, bool) {
	if len(overrides) == 0 || address == "" {
		return evmmodule.CallStateOverride{}, false
	}
	candidates := []string{address, strings.ToLower(address), strings.ToUpper(address)}
	if parsed, err := parseHexAddress(address); err == nil {
		canonical := string(parsed)
		candidates = append(candidates, canonical, strings.ToLower(canonical), strings.ToUpper(canonical))
	}
	for _, candidate := range candidates {
		if override, ok := overrides[candidate]; ok {
			return override, true
		}
	}
	return evmmodule.CallStateOverride{}, false
}

func web3SelectorTrace(input string) map[string]any {
	decoded, err := hexBytes(input)
	if err != nil || len(decoded) < 4 {
		return map[string]any{}
	}
	key := "0x" + hex.EncodeToString(decoded[:4]) + "-" + strconv.Itoa(len(decoded)-4)
	return map[string]any{key: 1}
}

func web3TraceBlockRecord(ctx context.Context, provider StatusProvider, record store.BlockRecord) []any {
	if record.Block.Header.Height == 0 {
		return []any{}
	}
	traces := make([]any, 0, len(record.TxResults))
	for _, result := range record.TxResults {
		receipt, ok := web3ReceiptFromResult(result)
		if !ok {
			continue
		}
		traces = append(traces, web3TraceListFromReceipt(ctx, provider, receipt)...)
	}
	return traces
}

func web3TraceListFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) []any {
	trace := web3TraceFromReceipt(ctx, provider, receipt)
	if trace == nil {
		return nil
	}
	traces := []any{trace}
	traces = append(traces, web3ParityTraceChildren(receipt.VMTrace, []uint64{})...)
	return traces
}

func web3TraceFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) any {
	blockHash, txIndex, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return nil
	}
	details := web3TransactionDetails(tx)
	to := details.To
	traceType := "call"
	result := map[string]any{"gasUsed": hexQuantity(receipt.GasUsed), "output": receipt.Output}
	if receipt.ContractAddress != "" && receipt.To == "" {
		traceType = "create"
		to = nil
		result["address"] = receipt.ContractAddress
	}
	action := map[string]any{
		"from":  receipt.From,
		"to":    to,
		"gas":   hexQuantity(details.Gas),
		"input": details.Input,
		"value": web3TxValueHex(details),
	}
	if traceType == "call" {
		action["callType"] = "call"
	}
	trace := map[string]any{
		"action":              action,
		"blockHash":           blockHash,
		"blockNumber":         hexQuantity(receipt.Height),
		"result":              result,
		"subtraces":           0,
		"traceAddress":        []any{},
		"transactionHash":     receipt.TxHash,
		"transactionPosition": hexQuantity(txIndex),
		"type":                traceType,
	}
	if receipt.Status == 0 {
		trace["error"] = web3ReceiptError(receipt)
	}
	return trace
}

func web3ReceiptError(receipt web3Receipt) string {
	if receipt.Error != "" {
		return receipt.Error
	}
	return "execution reverted"
}

func web3CallTraceChildren(vmTrace any) []any {
	trace, ok := vmTrace.(map[string]any)
	if !ok {
		return nil
	}
	calls, ok := trace["calls"].([]any)
	if ok {
		return calls
	}
	calls, ok = trace["children"].([]any)
	if ok {
		return calls
	}
	return nil
}

func web3ParityTraceChildren(vmTrace any, parent []uint64) []any {
	children := web3CallTraceChildren(vmTrace)
	if len(children) == 0 {
		return nil
	}
	out := make([]any, 0, len(children))
	for index, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		address := append(append([]uint64(nil), parent...), uint64(index))
		trace := web3ParityTraceFromCall(childMap, address)
		out = append(out, trace)
		out = append(out, web3ParityTraceChildren(childMap, address)...)
	}
	return out
}

func web3ParityTraceFromCall(call map[string]any, address []uint64) map[string]any {
	callType := strings.ToLower(stringValue(call["type"]))
	if callType == "" {
		callType = strings.ToLower(stringValue(call["callType"]))
	}
	if callType == "" {
		callType = "call"
	}
	action := map[string]any{
		"callType": callType,
		"from":     stringValue(call["from"]),
		"to":       stringValue(call["to"]),
		"gas":      stringValueOrDefault(call["gas"], "0x0"),
		"input":    stringValueOrDefault(call["input"], "0x"),
		"value":    stringValueOrDefault(call["value"], "0x0"),
	}
	result := map[string]any{
		"gasUsed": stringValueOrDefault(call["gasUsed"], "0x0"),
		"output":  stringValueOrDefault(call["output"], "0x"),
	}
	trace := map[string]any{
		"action":       action,
		"result":       result,
		"subtraces":    len(web3CallTraceChildren(call)),
		"traceAddress": uint64SliceToAny(address),
		"type":         callType,
	}
	if errText := stringValue(call["error"]); errText != "" {
		trace["error"] = errText
	}
	return trace
}

func stringValue(value any) string {
	item, _ := value.(string)
	return item
}

func stringValueOrDefault(value any, fallback string) string {
	if item := stringValue(value); item != "" {
		return item
	}
	return fallback
}

func uint64SliceToAny(values []uint64) []any {
	out := make([]any, len(values))
	for index, value := range values {
		out[index] = value
	}
	return out
}

func web3TraceAddressEqual(raw any, expected []uint64) bool {
	items, ok := raw.([]any)
	if !ok || len(items) != len(expected) {
		return false
	}
	for index, item := range items {
		switch value := item.(type) {
		case uint64:
			if value != expected[index] {
				return false
			}
		case float64:
			if value != float64(expected[index]) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func web3TraceWantsCallTracer(params []json.RawMessage) bool {
	if len(params) < 2 || string(params[1]) == "null" {
		return false
	}
	var config struct {
		Tracer string `json:"tracer"`
	}
	if err := json.Unmarshal(params[1], &config); err != nil {
		return false
	}
	return config.Tracer == "callTracer"
}

func web3Code(ctx context.Context, provider StatusProvider, params []json.RawMessage) (string, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return "", &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	if len(params) == 0 || len(params) > 2 {
		return "", &JSONRPCError{Code: -32602, Message: "eth_getCode requires address and optional block tag"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, rpcErr := web3HistoricalAccountQueryData(ctx, provider, params, 1)
	if rpcErr != nil {
		return "", rpcErr
	}
	response, err := query.AppQuery(ctx, []string{"evm", "code", address}, data)
	if err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return "0x", nil
	}
	if response.Code != 0 {
		return "", &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Value, &payload); err != nil {
		return "", &JSONRPCError{Code: -32000, Message: "invalid EVM code response"}
	}
	return "0x" + strings.TrimPrefix(payload.Code, "0x"), nil
}

func web3StorageAt(ctx context.Context, provider StatusProvider, params []json.RawMessage) (string, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return "", &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	if len(params) < 2 || len(params) > 3 {
		return "", &JSONRPCError{Code: -32602, Message: "eth_getStorageAt requires address, slot, and optional block tag"}
	}
	address, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	slot, err := jsonRPCStringParam(params[1])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	data, rpcErr := web3HistoricalAccountQueryData(ctx, provider, params, 2)
	if rpcErr != nil {
		return "", rpcErr
	}
	response, err := query.AppQuery(ctx, []string{"evm", "storage", address, slot}, data)
	if err != nil {
		return "", &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return "0x0", nil
	}
	if response.Code != 0 {
		return "", &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var payload struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(response.Value, &payload); err != nil {
		return "", &JSONRPCError{Code: -32000, Message: "invalid EVM storage response"}
	}
	if payload.Value == "" {
		return "0x0", nil
	}
	return "0x" + strings.TrimPrefix(payload.Value, "0x"), nil
}

func web3HistoricalAccountQueryData(ctx context.Context, provider StatusProvider, params []json.RawMessage, tagIndex int) ([]byte, *JSONRPCError) {
	if len(params) <= tagIndex || len(params[tagIndex]) == 0 || string(params[tagIndex]) == "null" {
		return nil, nil
	}
	trimmed := bytes.TrimSpace(params[tagIndex])
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] != '{' {
		tag, err := jsonRPCStringParam(params[tagIndex])
		if err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: err.Error()}
		}
		if tag == "latest" || tag == "pending" || tag == "" {
			return nil, nil
		}
	}
	height, rpcErr := web3BlockHeightParam(ctx, provider, params[tagIndex])
	if rpcErr != nil {
		return nil, rpcErr
	}
	encoded, err := json.Marshal(map[string]uint64{"height": uint64(height)})
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return encoded, nil
}

func web3EstimateGas(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (string, *JSONRPCError) {
	call, rpcErr := web3PreparedEVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return "", rpcErr
	}
	intrinsicGas, rpcErr := web3IntrinsicGasForCall(call, cfg)
	if rpcErr != nil {
		return "", rpcErr
	}
	high := call.GasLimit
	if high == 0 {
		high = defaultWeb3BlockGasLimit
	}
	if high < intrinsicGas {
		high = intrinsicGas
	}
	probe := call
	probe.GasLimit = high
	callResponse, rpcErr := web3EVMCallRequest(ctx, provider, probe)
	if rpcErr != nil {
		return "", rpcErr
	}
	if callResponse.Failed {
		return "", web3CallFailureError(callResponse)
	}
	low := intrinsicGas
	if callResponse.GasUsed > low {
		low = callResponse.GasUsed
	}
	if low >= high {
		return hexQuantity(high), nil
	}
	left, right := low, high
	for left < right {
		mid := left + (right-left)/2
		probe.GasLimit = mid
		response, rpcErr := web3EVMCallRequest(ctx, provider, probe)
		if rpcErr != nil || response.Failed {
			left = mid + 1
			continue
		}
		right = mid
	}
	return hexQuantity(left), nil
}

func web3IntrinsicGasForCall(call web3CallRequest, cfg Config) (uint64, *JSONRPCError) {
	input, err := hexBytes(call.Input)
	if err != nil {
		return 0, &JSONRPCError{Code: -32602, Message: "invalid data hex"}
	}
	gas, err := ethcompat.IntrinsicGasWithChainConfigJSON(input, call.AccessList, call.Method == "deploy", call.BlockOverride.Timestamp, cfg.EVMChainConfigJSON)
	if err != nil {
		return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return gas, nil
}

func saturatingAddUint64(left uint64, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}

func web3EVMCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (web3EVMCallResponse, *JSONRPCError) {
	call, rpcErr := web3PreparedEVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return web3EVMCallResponse{}, rpcErr
	}
	return web3EVMCallRequest(ctx, provider, call)
}

func web3PreparedEVMCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (web3CallRequest, *JSONRPCError) {
	call, rpcErr := evmCallParam(params)
	if rpcErr != nil {
		return web3CallRequest{}, rpcErr
	}
	if height, rpcErr := web3CallHeight(ctx, provider, params); rpcErr != nil {
		return web3CallRequest{}, rpcErr
	} else {
		call.Height = uint64(height)
	}
	baseFee := web3BaseFeeAtHeight(ctx, provider, types.Height(call.Height))
	call.BaseFee = baseFee
	call.BlobBaseFee = web3BlobBaseFeeAtHeight(ctx, provider, types.Height(call.Height))
	if call.GasPrice == 0 && call.GasPriceHex == "" && (call.MaxFeePerGas > 0 || call.MaxFeePerGasHex != "") {
		gasPrice := web3EffectiveCallGasPriceBig(baseFee, web3CallQuantityBig(call.MaxFeePerGasHex, call.MaxFeePerGas), web3CallQuantityBig(call.MaxPriorityFeePerGasHex, call.MaxPriorityFeePerGas))
		call.GasPriceHex = hexQuantityBig(gasPrice)
		if gasPrice.IsUint64() {
			call.GasPrice = gasPrice.Uint64()
		}
	}
	if overrides, rpcErr := web3StateOverridesParam(params); rpcErr != nil {
		return web3CallRequest{}, rpcErr
	} else {
		call.StateOverrides = overrides
	}
	if override, rpcErr := web3BlockOverrideParam(params); rpcErr != nil {
		return web3CallRequest{}, rpcErr
	} else {
		call.BlockOverride = override
		if override.BaseFee > 0 {
			call.BaseFee = override.BaseFee
		}
		if override.BlobBaseFee > 0 {
			call.BlobBaseFee = override.BlobBaseFee
		}
	}
	_ = cfg
	return call, nil
}

func web3EffectiveCallGasPrice(baseFee uint64, maxFeePerGas uint64, maxPriorityFeePerGas uint64) uint64 {
	if maxFeePerGas == 0 {
		return baseFee
	}
	capWithTip := saturatingAddUint64(baseFee, maxPriorityFeePerGas)
	if capWithTip > maxFeePerGas {
		return maxFeePerGas
	}
	return capWithTip
}

func web3EffectiveCallGasPriceBig(baseFee uint64, maxFeePerGas *big.Int, maxPriorityFeePerGas *big.Int) *big.Int {
	if maxFeePerGas == nil || maxFeePerGas.Sign() == 0 {
		return new(big.Int).SetUint64(baseFee)
	}
	capWithTip := new(big.Int).SetUint64(baseFee)
	if maxPriorityFeePerGas != nil {
		capWithTip.Add(capWithTip, maxPriorityFeePerGas)
	}
	if capWithTip.Cmp(maxFeePerGas) > 0 {
		return new(big.Int).Set(maxFeePerGas)
	}
	return capWithTip
}

func web3CallQuantityBig(hexValue string, fallback uint64) *big.Int {
	if hexValue != "" {
		parsed, ok := new(big.Int).SetString(strings.TrimPrefix(hexValue, "0x"), 16)
		if ok && parsed.Sign() >= 0 {
			return parsed
		}
	}
	return new(big.Int).SetUint64(fallback)
}

func web3EVMCallRequest(ctx context.Context, provider StatusProvider, call web3CallRequest) (web3EVMCallResponse, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	encoded, _ := json.Marshal(call)
	response, err := query.AppQuery(ctx, []string{"evm", "call"}, encoded)
	if err != nil {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code != 0 {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	var callResponse web3EVMCallResponse
	if err := json.Unmarshal(response.Value, &callResponse); err != nil {
		return web3EVMCallResponse{}, &JSONRPCError{Code: -32000, Message: "invalid EVM call response"}
	}
	return callResponse, nil
}

func web3CallFailureError(callResponse web3EVMCallResponse) *JSONRPCError {
	if callResponse.Error == "" {
		callResponse.Error = "execution reverted"
	}
	return &JSONRPCError{Code: -32000, Message: callResponse.Error}
}

func web3CallHeight(ctx context.Context, provider StatusProvider, params []json.RawMessage) (types.Height, *JSONRPCError) {
	for _, index := range []int{1, 2} {
		if len(params) <= index || len(params[index]) == 0 || string(params[index]) == "null" {
			continue
		}
		trimmed := bytes.TrimSpace(params[index])
		if len(trimmed) == 0 {
			continue
		}
		if index == 2 && trimmed[0] == '{' && !web3LooksLikeBlockSelector(trimmed) {
			continue
		}
		if trimmed[0] != '{' {
			var tag string
			if err := json.Unmarshal(params[index], &tag); err != nil || tag == "" {
				continue
			}
		}
		return web3BlockHeightParam(ctx, provider, params[index])
	}
	return provider.Status(ctx).LatestHeight, nil
}

func web3LooksLikeBlockSelector(raw json.RawMessage) bool {
	var selector map[string]json.RawMessage
	if err := json.Unmarshal(raw, &selector); err != nil {
		return false
	}
	_, hasNumber := selector["blockNumber"]
	_, hasHash := selector["blockHash"]
	_, hasCanonical := selector["requireCanonical"]
	return hasNumber || hasHash || hasCanonical
}

func web3StateOverridesParam(params []json.RawMessage) (map[string]evmmodule.CallStateOverride, *JSONRPCError) {
	if len(params) <= 2 || len(bytes.TrimSpace(params[2])) == 0 || string(bytes.TrimSpace(params[2])) == "null" {
		return nil, nil
	}
	raw := bytes.TrimSpace(params[2])
	if raw[0] != '{' || web3LooksLikeBlockSelector(raw) {
		return nil, nil
	}
	var payload map[string]web3StateOverrideAccount
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid state override"}
	}
	overrides := make(map[string]evmmodule.CallStateOverride, len(payload))
	for address, account := range payload {
		if _, err := parseHexAddress(address); err != nil {
			return nil, &JSONRPCError{Code: -32602, Message: "invalid state override address"}
		}
		if rpcErr := validateWeb3StateOverrideAccount(account); rpcErr != nil {
			return nil, rpcErr
		}
		override := evmmodule.CallStateOverride{
			Balance:   account.Balance,
			Code:      account.Code,
			State:     normalizeOverrideStorage(account.State),
			StateDiff: normalizeOverrideStorage(account.StateDiff),
		}
		if account.Nonce != "" {
			nonce, err := parseHexQuantity(account.Nonce)
			if err != nil {
				return nil, &JSONRPCError{Code: -32602, Message: "invalid state override nonce"}
			}
			override.Nonce = &nonce
		}
		overrides[address] = override
	}
	return overrides, nil
}

func validateWeb3StateOverrideAccount(account web3StateOverrideAccount) *JSONRPCError {
	if account.Balance != "" {
		if _, err := parseOverrideBalance(account.Balance); err != nil {
			return &JSONRPCError{Code: -32602, Message: "invalid state override balance"}
		}
	}
	if account.Nonce != "" {
		if _, err := parseHexQuantity(account.Nonce); err != nil {
			return &JSONRPCError{Code: -32602, Message: "invalid state override nonce"}
		}
	}
	if account.Code != "" {
		if err := validateHexData(account.Code, "state override code"); err != nil {
			return &JSONRPCError{Code: -32602, Message: err.Error()}
		}
	}
	if len(account.State) > 0 && len(account.StateDiff) > 0 {
		return &JSONRPCError{Code: -32602, Message: "state override cannot include both state and stateDiff"}
	}
	if err := validateOverrideStorage(account.State, "state override state"); err != nil {
		return &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	if err := validateOverrideStorage(account.StateDiff, "state override stateDiff"); err != nil {
		return &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	return nil
}

func parseOverrideBalance(value string) (*big.Int, error) {
	if strings.HasPrefix(value, "0x") {
		parsed, err := parseHexQuantityBig(value)
		if err != nil {
			return nil, err
		}
		if parsed.BitLen() > 256 {
			return nil, fmt.Errorf("balance exceeds uint256")
		}
		return parsed, nil
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() < 0 || parsed.BitLen() > 256 {
		return nil, fmt.Errorf("invalid decimal balance")
	}
	return parsed, nil
}

func validateHexData(value string, name string) error {
	if !strings.HasPrefix(value, "0x") {
		return fmt.Errorf("%s must use 0x hex data", name)
	}
	_, err := hexBytes(value)
	if err != nil {
		return fmt.Errorf("invalid %s", name)
	}
	return nil
}

func validateOverrideStorage(values map[string]string, name string) error {
	for slot, value := range values {
		slotBytes, err := parseFixedWidthHex(slot, 32)
		if err != nil {
			return fmt.Errorf("invalid %s slot", name)
		}
		if len(slotBytes) > 32 {
			return fmt.Errorf("invalid %s slot", name)
		}
		valueBytes, err := parseFixedWidthHex(value, 32)
		if err != nil {
			return fmt.Errorf("invalid %s value", name)
		}
		if len(valueBytes) > 32 {
			return fmt.Errorf("invalid %s value", name)
		}
	}
	return nil
}

func parseFixedWidthHex(value string, maxBytes int) ([]byte, error) {
	if !strings.HasPrefix(value, "0x") {
		return nil, fmt.Errorf("hex value must use 0x prefix")
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return []byte{}, nil
	}
	if len(trimmed) > maxBytes*2 {
		return nil, fmt.Errorf("hex value too wide")
	}
	return hexBytes(value)
}

func normalizeOverrideStorage(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(values))
	for slot, value := range values {
		normalized[slot] = value
	}
	return normalized
}

func web3BlockOverrideParam(params []json.RawMessage) (evmmodule.CallBlockOverride, *JSONRPCError) {
	if len(params) <= 3 || len(bytes.TrimSpace(params[3])) == 0 || string(bytes.TrimSpace(params[3])) == "null" {
		return evmmodule.CallBlockOverride{}, nil
	}
	var payload web3BlockOverride
	if err := json.Unmarshal(params[3], &payload); err != nil {
		return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override"}
	}
	var override evmmodule.CallBlockOverride
	var err error
	if payload.Number != "" {
		override.Number, err = parseHexQuantity(payload.Number)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override number"}
		}
	}
	timestamp := payload.Timestamp
	if timestamp == "" {
		timestamp = payload.Time
	}
	if timestamp != "" {
		override.Timestamp, err = parseHexQuantity(timestamp)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override timestamp"}
		}
	}
	if payload.GasLimit != "" {
		override.GasLimit, err = parseHexQuantity(payload.GasLimit)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override gasLimit"}
		}
	}
	if payload.BaseFee != "" {
		override.BaseFee, err = parseHexQuantity(payload.BaseFee)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override baseFeePerGas"}
		}
	}
	if payload.BlobBaseFee != "" {
		override.BlobBaseFee, err = parseHexQuantity(payload.BlobBaseFee)
		if err != nil {
			return evmmodule.CallBlockOverride{}, &JSONRPCError{Code: -32602, Message: "invalid block override blobBaseFee"}
		}
	}
	return override, nil
}

func web3LatestBaseFee(ctx context.Context, provider StatusProvider) uint64 {
	query, ok := provider.(ChainQueryProvider)
	if !ok {
		return 0
	}
	state, err := query.LatestState(ctx)
	if err != nil {
		return 0
	}
	if state.NextBaseFee > 0 {
		return state.NextBaseFee
	}
	return state.BaseFee
}

func web3CreateAccessList(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	callResponse, rpcErr := web3EVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return map[string]any{
		"accessList": web3AccessList(callResponse.AccessList),
		"gasUsed":    hexQuantity(callResponse.GasUsed),
	}, nil
}

func web3DebugTraceCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "debug_traceCall requires call object, optional block tag, and optional config"}
	}
	tracer, rpcErr := web3DebugCallTracer(params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	executionParams := web3DebugTraceExecutionParams(params)
	callResponse, rpcErr := web3EVMCall(ctx, provider, cfg, executionParams)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if tracer == "callTracer" {
		call, _ := evmCallParam(params)
		trace := map[string]any{
			"type":       "CALL",
			"from":       call.From,
			"to":         call.To,
			"value":      web3CallValueHex(call),
			"gas":        hexQuantity(call.GasLimit),
			"gasUsed":    hexQuantity(callResponse.GasUsed),
			"input":      call.Input,
			"output":     callResponse.Output,
			"accessList": web3AccessList(callResponse.AccessList),
		}
		if callResponse.Failed {
			trace["error"] = callResponse.Error
		}
		if children := web3CallTraceChildren(callResponse.VMTrace); len(children) > 0 {
			trace["calls"] = children
		}
		return trace, nil
	}
	if tracer == "prestateTracer" {
		call, _ := evmCallParam(params)
		return web3CallPrestateTrace(call), nil
	}
	if tracer == "4byteTracer" {
		call, _ := evmCallParam(params)
		return web3SelectorTrace(call.Input), nil
	}
	return map[string]any{
		"gas":         callResponse.GasUsed,
		"failed":      callResponse.Failed,
		"returnValue": strings.TrimPrefix(callResponse.Output, "0x"),
		"structLogs":  web3StructLogs(callResponse.VMTrace),
	}, nil
}

func web3TraceCall(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (any, *JSONRPCError) {
	if len(params) == 0 || len(params) > 3 {
		return nil, &JSONRPCError{Code: -32602, Message: "trace_call requires call object, optional trace types, and optional block tag"}
	}
	call, rpcErr := evmCallParam(params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	callResponse, rpcErr := web3EVMCall(ctx, provider, cfg, params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	trace := map[string]any{
		"action": map[string]any{
			"callType": "call",
			"from":     call.From,
			"to":       call.To,
			"gas":      hexQuantity(call.GasLimit),
			"input":    call.Input,
			"value":    web3CallValueHex(call),
		},
		"result": map[string]any{
			"gasUsed": hexQuantity(callResponse.GasUsed),
			"output":  callResponse.Output,
		},
		"subtraces":    len(web3CallTraceChildren(callResponse.VMTrace)),
		"traceAddress": []any{},
		"type":         "call",
	}
	if callResponse.Failed {
		trace["error"] = callResponse.Error
	}
	traces := []any{trace}
	traces = append(traces, web3ParityTraceChildren(callResponse.VMTrace, []uint64{})...)
	return map[string]any{
		"output":     callResponse.Output,
		"stateDiff":  web3StateDiff(callResponse.StateDiff),
		"trace":      traces,
		"vmTrace":    callResponse.VMTrace,
		"accessList": web3AccessList(callResponse.AccessList),
	}, nil
}

func web3StructLogs(vmTrace any) any {
	trace, ok := vmTrace.(map[string]any)
	if !ok {
		return []any{}
	}
	if logs, found := trace["structLogs"]; found {
		return logs
	}
	return []any{}
}

func web3DebugCallTracer(params []json.RawMessage) (string, *JSONRPCError) {
	if len(params) >= 3 {
		return web3DebugTracer(params, 2)
	}
	if len(params) == 2 && bytes.Contains(params[1], []byte(`"tracer"`)) {
		return web3DebugTracer(params, 1)
	}
	return "", nil
}

func web3DebugTraceExecutionParams(params []json.RawMessage) []json.RawMessage {
	if len(params) >= 3 {
		return params[:2]
	}
	if len(params) == 2 && bytes.Contains(params[1], []byte(`"tracer"`)) {
		return params[:1]
	}
	return params
}

func web3DebugTracer(params []json.RawMessage, index int) (string, *JSONRPCError) {
	if len(params) <= index || len(bytes.TrimSpace(params[index])) == 0 || string(bytes.TrimSpace(params[index])) == "null" {
		return "", nil
	}
	var config struct {
		Tracer string `json:"tracer"`
	}
	if err := json.Unmarshal(params[index], &config); err != nil {
		return "", &JSONRPCError{Code: -32602, Message: "debug trace config must be an object"}
	}
	switch config.Tracer {
	case "", "callTracer", "structLogger", "prestateTracer", "4byteTracer":
		return config.Tracer, nil
	default:
		return "structLogger", nil
	}
}

func web3StateDiff(stateDiff any) any {
	if stateDiff == nil {
		return map[string]any{}
	}
	return stateDiff
}

func web3AccessList(entries []web3AccessListEntry) []any {
	out := make([]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, map[string]any{
			"address":     entry.Address,
			"storageKeys": append([]string(nil), entry.StorageKeys...),
		})
	}
	return out
}

func lastParam(params []json.RawMessage) json.RawMessage {
	if len(params) == 0 {
		return nil
	}
	return params[len(params)-1]
}

func web3LogsForAddress(ctx context.Context, provider StatusProvider, address string) ([]any, *JSONRPCError) {
	query, ok := provider.(AppQueryProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "application query is unavailable"}
	}
	path := []string{"evm", "logs"}
	if address != "" {
		path = append(path, address)
	}
	response, err := query.AppQuery(ctx, path, nil)
	if err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	if response.Code == 3 {
		return []any{}, nil
	}
	if response.Code != 0 {
		return nil, &JSONRPCError{Code: -32000, Message: response.Log}
	}
	logs, rpcErr := rawJSONObject(response.Value)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return web3LogArray(logs), nil
}

func web3LogsForFilter(ctx context.Context, provider StatusProvider, cfg Config, filter web3Filter) ([]any, *JSONRPCError) {
	maxResults := cfg.Web3LogMaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxWeb3LogResults
	}
	results := make([]any, 0)
	appendMatching := func(logs []any) *JSONRPCError {
		for _, log := range logs {
			if !web3LogMatchesFilter(log, filter) {
				continue
			}
			results = append(results, web3NormalizeLog(log))
			if len(results) > maxResults {
				return &JSONRPCError{Code: -32005, Message: "eth_getLogs result limit exceeded"}
			}
		}
		return nil
	}
	if len(filter.Addresses) == 0 {
		logs, rpcErr := web3LogsForAddress(ctx, provider, "")
		if rpcErr != nil {
			return nil, rpcErr
		}
		if rpcErr := appendMatching(logs); rpcErr != nil {
			return nil, rpcErr
		}
		return results, nil
	}
	for _, address := range filter.Addresses {
		logs, rpcErr := web3LogsForAddress(ctx, provider, address)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if rpcErr := appendMatching(logs); rpcErr != nil {
			return nil, rpcErr
		}
	}
	return results, nil
}

func web3FilterChanges(ctx context.Context, provider StatusProvider, cfg Config, filter web3Filter, onlyChanges bool) (any, *JSONRPCError) {
	switch filter.Type {
	case "block":
		return web3BlockFilterChanges(ctx, provider, filter, onlyChanges)
	default:
		logs, rpcErr := web3LogsForFilter(ctx, provider, cfg, filter)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if !onlyChanges {
			return logs, nil
		}
		changes := make([]any, 0, len(logs))
		for _, log := range logs {
			id := web3LogID(log)
			if id == "" || filter.SeenLogs[id] {
				continue
			}
			changes = append(changes, log)
		}
		return changes, nil
	}
}

func web3PendingFilterChanges(ctx context.Context, provider StatusProvider, filter web3Filter, onlyChanges bool) ([]string, web3Filter, *JSONRPCError) {
	pendingProvider, ok := provider.(PendingTxProvider)
	if !ok {
		return nil, filter, &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
	}
	hashes, err := pendingProvider.PendingTxHashes(ctx)
	if err != nil {
		return nil, filter, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	items := make([]string, 0, len(hashes))
	nextSeen := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		encoded := web3HashString(hash)
		nextSeen[encoded] = true
		if onlyChanges && filter.SeenPending[encoded] {
			continue
		}
		items = append(items, encoded)
	}
	filter.SeenPending = nextSeen
	return items, filter, nil
}

func web3BlockFilterChanges(ctx context.Context, provider StatusProvider, filter web3Filter, onlyChanges bool) ([]string, *JSONRPCError) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return nil, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	latest := uint64(provider.Status(ctx).LatestHeight)
	from := uint64(1)
	if onlyChanges {
		from = filter.LastHeight + 1
	}
	if from > latest {
		return []string{}, nil
	}
	hashes := make([]string, 0, latest-from+1)
	for height := from; height <= latest; height++ {
		record, err := blockProvider.BlockByHeight(ctx, types.Height(height))
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return nil, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		hashes = append(hashes, "0x"+hex.EncodeToString(record.Hash[:]))
	}
	return hashes, nil
}

func web3TransactionFromReceipt(ctx context.Context, provider StatusProvider, value []byte) (any, *JSONRPCError) {
	var receipt web3Receipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt response"}
	}
	if receipt.TxHash == "" {
		return nil, &JSONRPCError{Code: -32000, Message: "missing EVM receipt hash"}
	}
	blockHash, txIndex, tx, foundTx := web3ReceiptBlockLocation(ctx, provider, receipt)
	details := web3TxDetails{Nonce: 0, Gas: receipt.GasUsed, GasPrice: 0, Value: 0, Input: receipt.Output}
	if foundTx {
		details = web3TransactionDetails(tx)
	}
	to := receipt.To
	if to == "" && receipt.ContractAddress != "" {
		to = receipt.ContractAddress
	}
	return map[string]any{
		"hash":             receipt.TxHash,
		"nonce":            hexQuantity(details.Nonce),
		"blockHash":        blockHash,
		"blockNumber":      hexQuantity(receipt.Height),
		"transactionIndex": hexQuantity(txIndex),
		"from":             receipt.From,
		"to":               to,
		"value":            web3TxValueHex(details),
		"gas":              hexQuantity(details.Gas),
		"gasPrice":         web3TxGasPriceHex(details),
		"input":            details.Input,
		"type":             hexQuantity(details.Type),
		"chainId":          hexQuantity(details.ChainID),
	}, nil
}

func web3ReceiptObject(ctx context.Context, provider StatusProvider, value []byte) (any, *JSONRPCError) {
	var receipt web3Receipt
	if err := json.Unmarshal(value, &receipt); err != nil {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid EVM receipt response"}
	}
	if receipt.TxHash == "" {
		return nil, &JSONRPCError{Code: -32000, Message: "missing EVM receipt hash"}
	}
	blockHash, txIndex, _, _ := web3ReceiptBlockLocation(ctx, provider, receipt)
	cumulativeGasUsed := web3CumulativeGasUsed(ctx, provider, receipt, txIndex)
	to := any(receipt.To)
	if receipt.To == "" {
		to = nil
	}
	contractAddress := any(nil)
	if receipt.ContractAddress != "" {
		contractAddress = receipt.ContractAddress
	}
	logs := make([]any, 0, len(receipt.Logs))
	for _, log := range receipt.Logs {
		logs = append(logs, web3NormalizeLog(log))
	}
	return map[string]any{
		"transactionHash":   receipt.TxHash,
		"transactionIndex":  hexQuantity(txIndex),
		"blockHash":         blockHash,
		"blockNumber":       hexQuantity(receipt.Height),
		"from":              receipt.From,
		"to":                to,
		"cumulativeGasUsed": hexQuantity(cumulativeGasUsed),
		"gasUsed":           hexQuantity(receipt.GasUsed),
		"contractAddress":   contractAddress,
		"logs":              logs,
		"logsBloom":         web3ReceiptBloom(ctx, provider, receipt),
		"status":            hexQuantity(uint64(receipt.Status)),
		"effectiveGasPrice": web3EffectiveGasPriceHexFromReceipt(ctx, provider, receipt),
		"type":              hexQuantity(web3TransactionTypeFromReceipt(ctx, provider, receipt)),
	}, nil
}

func web3TransactionFromBlockRecord(record store.BlockRecord, index int, hashText string, tx types.Tx) any {
	details := web3TransactionDetails(tx)
	transaction := map[string]any{
		"hash":             hashText,
		"nonce":            hexQuantity(details.Nonce),
		"blockHash":        "0x" + hex.EncodeToString(record.Hash[:]),
		"blockNumber":      hexQuantity(uint64(record.Block.Header.Height)),
		"transactionIndex": hexQuantity(uint64(index)),
		"from":             details.From,
		"to":               details.To,
		"value":            web3TxValueHex(details),
		"gas":              hexQuantity(details.Gas),
		"gasPrice":         web3TxGasPriceHex(details),
		"input":            details.Input,
		"type":             hexQuantity(details.Type),
		"chainId":          hexQuantity(details.ChainID),
	}
	if details.MaxFeePerGas > 0 || details.MaxFeePerGasHex != "" {
		transaction["maxFeePerGas"] = web3TxMaxFeePerGasHex(details)
	}
	if details.MaxPriorityFeePerGas > 0 || details.MaxPriorityFeePerGasHex != "" {
		transaction["maxPriorityFeePerGas"] = web3TxMaxPriorityFeePerGasHex(details)
	}
	if details.BlobGasFeeCap > 0 || details.BlobGasFeeCapHex != "" {
		transaction["maxFeePerBlobGas"] = web3TxBlobGasFeeCapHex(details)
	}
	if len(details.BlobHashes) > 0 {
		transaction["blobVersionedHashes"] = append([]string(nil), details.BlobHashes...)
	}
	if index >= len(record.TxResults) {
		return transaction
	}
	receipt, ok := web3ReceiptFromResult(record.TxResults[index])
	if !ok {
		transaction["gas"] = hexQuantity(record.TxResults[index].GasUsed)
		return transaction
	}
	to := receipt.To
	if to == "" && receipt.ContractAddress != "" {
		to = receipt.ContractAddress
	}
	transaction["from"] = receipt.From
	transaction["to"] = to
	transaction["gas"] = hexQuantity(receipt.GasUsed)
	return transaction
}

func web3EffectiveGasPrice(tx types.Tx) uint64 {
	price, found := web3EffectiveGasPriceBig(tx)
	if !found || !price.IsUint64() {
		return 0
	}
	return price.Uint64()
}

func web3EffectiveGasPriceBig(tx types.Tx) (*big.Int, bool) {
	if gasPrice, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagGasPrice); found {
		return gasPrice, true
	}
	meta := vexoapp.ParseTxMeta(tx)
	if meta.FeeBig == nil || meta.FeeBig.Sign() == 0 || meta.Gas == 0 {
		return nil, false
	}
	return new(big.Int).Div(new(big.Int).Set(meta.FeeBig), new(big.Int).SetUint64(meta.Gas)), true
}

func web3EffectiveGasPriceHex(tx types.Tx) string {
	price, found := web3EffectiveGasPriceBig(tx)
	if !found {
		return hexQuantity(0)
	}
	return hexQuantityBig(price)
}

type web3TxDetails struct {
	From                    any
	To                      any
	Input                   string
	Nonce                   uint64
	Gas                     uint64
	GasPrice                uint64
	GasPriceHex             string
	MaxFeePerGas            uint64
	MaxFeePerGasHex         string
	MaxPriorityFeePerGas    uint64
	MaxPriorityFeePerGasHex string
	BlobGasFeeCap           uint64
	BlobGasFeeCapHex        string
	BlobHashes              []string
	Value                   uint64
	ValueHex                string
	Type                    uint64
	ChainID                 uint64
}

func web3TransactionDetails(tx types.Tx) web3TxDetails {
	meta := vexoapp.ParseTxMeta(tx)
	details := web3TxDetails{
		From:        nil,
		To:          nil,
		Input:       "0x" + hex.EncodeToString(tx),
		Nonce:       meta.Nonce,
		Gas:         meta.Gas,
		GasPrice:    web3EffectiveGasPrice(tx),
		GasPriceHex: web3EffectiveGasPriceHex(tx),
		Type:        0,
	}
	if meta.Signer != "" {
		details.From = string(meta.Signer)
	}
	if chainID, found := vexoapp.TxUintTag(tx, ethcompat.TagChainID); found {
		details.ChainID = chainID
	}
	if txType, found := vexoapp.TxUintTag(tx, ethcompat.TagType); found {
		details.Type = txType
	}
	if maxFee, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagMaxFeePerGas); found {
		setWeb3TxQuantity(&details.MaxFeePerGas, &details.MaxFeePerGasHex, maxFee)
	}
	if maxPriority, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagMaxPriorityFeePerGas); found {
		setWeb3TxQuantity(&details.MaxPriorityFeePerGas, &details.MaxPriorityFeePerGasHex, maxPriority)
	}
	if blobGasFeeCap, found := vexoapp.TxAmountBigTag(tx, ethcompat.TagBlobGasFeeCap); found {
		setWeb3TxQuantity(&details.BlobGasFeeCap, &details.BlobGasFeeCapHex, blobGasFeeCap)
	}
	details.BlobHashes = web3BlobVersionedHashes(tx)
	if value, found := vexoapp.TxTag(tx, ethcompat.TagValue); found {
		setWeb3TxValue(&details, value)
	}
	if input, found := vexoapp.TxTag(tx, ethcompat.TagInput); found && input != "" {
		details.Input = "0x" + strings.TrimPrefix(input, "0x")
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		return details
	}
	switch canonical.Action {
	case "call":
		if len(canonical.Args) >= 7 {
			details.To = canonical.Args[2]
			if details.Input == "" || details.Input == "0x" {
				details.Input = "0x" + strings.TrimPrefix(canonical.Args[4], "0x")
			}
			setWeb3TxValue(&details, canonical.Args[6])
		}
	case "deploy", "eth_deploy":
		if len(canonical.Args) >= 5 {
			details.To = nil
			if details.Input == "" || details.Input == "0x" {
				details.Input = "0x" + strings.TrimPrefix(canonical.Args[2], "0x")
			}
			setWeb3TxValue(&details, canonical.Args[4])
		}
	}
	return details
}

func setWeb3TxValue(details *web3TxDetails, decimal string) {
	value, ok := new(big.Int).SetString(decimal, 10)
	if !ok || value.Sign() < 0 {
		return
	}
	details.ValueHex = hexQuantityBig(value)
	if value.IsUint64() {
		details.Value = value.Uint64()
	}
}

func setWeb3TxQuantity(compat *uint64, hexValue *string, value *big.Int) {
	if value == nil || value.Sign() < 0 {
		return
	}
	*hexValue = hexQuantityBig(value)
	if value.IsUint64() {
		*compat = value.Uint64()
	}
}

func web3TxValueHex(details web3TxDetails) string {
	if details.ValueHex != "" {
		return details.ValueHex
	}
	return hexQuantity(details.Value)
}

func web3TxGasPriceHex(details web3TxDetails) string {
	if details.GasPriceHex != "" {
		return details.GasPriceHex
	}
	return hexQuantity(details.GasPrice)
}

func web3TxMaxFeePerGasHex(details web3TxDetails) string {
	if details.MaxFeePerGasHex != "" {
		return details.MaxFeePerGasHex
	}
	return hexQuantity(details.MaxFeePerGas)
}

func web3TxMaxPriorityFeePerGasHex(details web3TxDetails) string {
	if details.MaxPriorityFeePerGasHex != "" {
		return details.MaxPriorityFeePerGasHex
	}
	return hexQuantity(details.MaxPriorityFeePerGas)
}

func web3TxBlobGasFeeCapHex(details web3TxDetails) string {
	if details.BlobGasFeeCapHex != "" {
		return details.BlobGasFeeCapHex
	}
	return hexQuantity(details.BlobGasFeeCap)
}

func web3TxValueDecimal(details web3TxDetails) string {
	if details.ValueHex == "" {
		return strconv.FormatUint(details.Value, 10)
	}
	value, ok := new(big.Int).SetString(strings.TrimPrefix(details.ValueHex, "0x"), 16)
	if !ok {
		return "0"
	}
	return value.String()
}

func web3BlobVersionedHashes(tx types.Tx) []string {
	encoded, found := vexoapp.TxTag(tx, ethcompat.TagBlobHashes)
	if !found || encoded == "" {
		return nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var hashes []string
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return nil
	}
	normalized := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		if !strings.HasPrefix(hash, "0x") && !strings.HasPrefix(hash, "0X") {
			hash = "0x" + hash
		}
		normalized = append(normalized, strings.ToLower(hash))
	}
	return normalized
}

func web3AccountBalanceHex(account web3AccountStateResponse) string {
	if account.BalanceHex != "" {
		return account.BalanceHex
	}
	return hexQuantity(account.Balance)
}

func web3CallValueHex(call web3CallRequest) string {
	if call.ValueHex != "" {
		return call.ValueHex
	}
	return hexQuantity(call.Value)
}

func web3TxHash(tx types.Tx) string {
	if hash, found := vexoapp.TxTag(tx, ethcompat.TagHash); found && hash != "" {
		return hash
	}
	hash := mempool.HashTx(tx)
	return "0x" + hex.EncodeToString(hash[:])
}

func web3ReceiptFromResult(result types.Result) (web3Receipt, bool) {
	if len(result.Data) == 0 {
		return web3Receipt{}, false
	}
	var receipt web3Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil || receipt.TxHash == "" {
		return web3Receipt{}, false
	}
	return receipt, true
}

func web3EffectiveGasPriceFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) uint64 {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return 0
	}
	return web3EffectiveGasPrice(tx)
}

func web3EffectiveGasPriceHexFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) string {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return hexQuantity(0)
	}
	return web3EffectiveGasPriceHex(tx)
}

func web3TransactionTypeFromReceipt(ctx context.Context, provider StatusProvider, receipt web3Receipt) uint64 {
	_, _, tx, found := web3ReceiptBlockLocation(ctx, provider, receipt)
	if !found {
		return 0
	}
	if txType, ok := vexoapp.TxUintTag(tx, ethcompat.TagType); ok {
		return txType
	}
	return 0
}

func web3CumulativeGasUsed(ctx context.Context, provider StatusProvider, receipt web3Receipt, txIndex uint64) uint64 {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 {
		return receipt.GasUsed
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return receipt.GasUsed
	}
	if txIndex >= uint64(len(record.TxResults)) {
		return receipt.GasUsed
	}
	var cumulative uint64
	for index := uint64(0); index <= txIndex; index++ {
		gasUsed := record.TxResults[index].GasUsed
		if parsed, ok := web3ReceiptFromResult(record.TxResults[index]); ok {
			gasUsed = parsed.GasUsed
		}
		if ^uint64(0)-cumulative < gasUsed {
			return ^uint64(0)
		}
		cumulative += gasUsed
	}
	if cumulative == 0 {
		return receipt.GasUsed
	}
	return cumulative
}

func web3ReceiptBlockLocation(ctx context.Context, provider StatusProvider, receipt web3Receipt) (any, uint64, types.Tx, bool) {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 {
		return nil, 0, nil, false
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return nil, 0, nil, false
	}
	if index, found, _ := web3ReceiptIndexByHash(ctx, provider, receipt.TxHash); found && index.Height == receipt.Height && index.TxIndex < uint64(len(record.Block.Txs)) {
		tx := record.Block.Txs[index.TxIndex]
		if web3TxMatchesHash(tx, receipt.TxHash) || strings.EqualFold(index.TxHash, receipt.TxHash) {
			return "0x" + hex.EncodeToString(record.Hash[:]), index.TxIndex, append(types.Tx(nil), tx...), true
		}
	}
	for index, tx := range record.Block.Txs {
		if web3TxHash(tx) == receipt.TxHash {
			return "0x" + hex.EncodeToString(record.Hash[:]), uint64(index), append(types.Tx(nil), tx...), true
		}
	}
	if record.Hash != (types.Hash{}) {
		return "0x" + hex.EncodeToString(record.Hash[:]), 0, nil, false
	}
	return nil, 0, nil, false
}

func web3ReceiptsRoot(txs []types.Tx, results []types.Result) string {
	if root, ok := ethcompat.ReceiptRoot(txs, results); ok {
		return root
	}
	hasher := sha256.New()
	for _, result := range results {
		code := make([]byte, 4)
		code[0] = byte(result.Code >> 24)
		code[1] = byte(result.Code >> 16)
		code[2] = byte(result.Code >> 8)
		code[3] = byte(result.Code)
		_, _ = hasher.Write(code)
		_, _ = hasher.Write(result.Data)
		gas := strconv.FormatUint(result.GasUsed, 10)
		_, _ = hasher.Write([]byte(gas))
		_, _ = hasher.Write([]byte{0})
	}
	sum := hasher.Sum(nil)
	return "0x" + hex.EncodeToString(sum)
}

func web3LogsBloom(txs []types.Tx, results []types.Result) string {
	if bloom, ok := ethcompat.LogsBloom(txs, results); ok {
		return bloom
	}
	return "0x" + strings.Repeat("00", 256)
}

func web3ReceiptBloom(ctx context.Context, provider StatusProvider, receipt web3Receipt) string {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || receipt.Height == 0 {
		return "0x" + strings.Repeat("00", 256)
	}
	record, err := blockProvider.BlockByHeight(ctx, types.Height(receipt.Height))
	if err != nil {
		return "0x" + strings.Repeat("00", 256)
	}
	for index, tx := range record.Block.Txs {
		if index >= len(record.TxResults) {
			break
		}
		if web3TxHash(tx) != receipt.TxHash {
			continue
		}
		if bloom, ok := ethcompat.ReceiptBloom(tx, record.TxResults[index]); ok {
			return bloom
		}
	}
	return "0x" + strings.Repeat("00", 256)
}

func web3BlockGasUsed(results []types.Result) uint64 {
	var total uint64
	for _, result := range results {
		if total > ^uint64(0)-result.GasUsed {
			return ^uint64(0)
		}
		total += result.GasUsed
	}
	return total
}

func web3BlockGasLimit(results []types.Result) string {
	return hexQuantity(web3BlockGasLimitValue(results))
}

func web3BlockGasLimitValue(results []types.Result) uint64 {
	used := web3BlockGasUsed(results)
	if used > defaultWeb3BlockGasLimit {
		return used
	}
	return defaultWeb3BlockGasLimit
}

func web3BlockGasUsedRatio(ctx context.Context, provider StatusProvider, height types.Height) float64 {
	blockProvider, ok := provider.(BlockProvider)
	if !ok || height == 0 {
		return 0
	}
	record, err := blockProvider.BlockByHeight(ctx, height)
	if err != nil {
		return 0
	}
	if web3BlockGasUsed(record.TxResults) == 0 {
		return 0
	}
	gasLimit := web3BlockGasLimitValue(record.TxResults)
	if gasLimit == 0 {
		return 0
	}
	return float64(web3BlockGasUsed(record.TxResults)) / float64(gasLimit)
}

func web3BlockRecordParam(ctx context.Context, provider StatusProvider, raw json.RawMessage) (store.BlockRecord, *JSONRPCError) {
	text, err := jsonRPCStringParam(raw)
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	blockProvider, ok := provider.(BlockProvider)
	if !ok {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
	}
	if strings.HasPrefix(text, "0x") && len(text) == 66 {
		hash, err := parseHexHash(text)
		if err != nil {
			return store.BlockRecord{}, &JSONRPCError{Code: -32602, Message: "invalid block hash"}
		}
		record, err := blockProvider.BlockByHash(ctx, hash)
		if errors.Is(err, store.ErrBlockNotFound) {
			return store.BlockRecord{}, nil
		}
		if err != nil {
			return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return record, nil
	}
	height, rpcErr := web3BlockHeightParam(ctx, provider, raw)
	if rpcErr != nil {
		return store.BlockRecord{}, rpcErr
	}
	record, err := blockProvider.BlockByHeight(ctx, height)
	if errors.Is(err, store.ErrBlockNotFound) {
		return store.BlockRecord{}, nil
	}
	if err != nil {
		return store.BlockRecord{}, &JSONRPCError{Code: -32000, Message: err.Error()}
	}
	return record, nil
}

func web3BlockHeightParam(ctx context.Context, provider StatusProvider, raw json.RawMessage) (types.Height, *JSONRPCError) {
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{' {
		var selector struct {
			BlockNumber      string `json:"blockNumber"`
			BlockHash        string `json:"blockHash"`
			RequireCanonical bool   `json:"requireCanonical"`
		}
		if err := json.Unmarshal(raw, &selector); err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid block selector"}
		}
		if selector.BlockNumber != "" {
			encoded, _ := json.Marshal(selector.BlockNumber)
			return web3BlockHeightParam(ctx, provider, encoded)
		}
		if selector.BlockHash == "" {
			return 0, &JSONRPCError{Code: -32602, Message: "block selector requires blockHash or blockNumber"}
		}
		blockProvider, ok := provider.(BlockProvider)
		if !ok {
			return 0, &JSONRPCError{Code: -32000, Message: "block query is unavailable"}
		}
		hash, err := parseHexHash(selector.BlockHash)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid block hash"}
		}
		record, err := blockProvider.BlockByHash(ctx, hash)
		if errors.Is(err, store.ErrBlockNotFound) {
			return 0, &JSONRPCError{Code: -32000, Message: "block not found"}
		}
		if err != nil {
			return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return record.Block.Header.Height, nil
	}
	text, err := jsonRPCStringParam(raw)
	if err != nil {
		return 0, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	switch text {
	case "latest", "pending":
		return provider.Status(ctx).LatestHeight, nil
	case "safe", "finalized":
		return web3FinalizedHeight(ctx, provider)
	case "earliest":
		query, ok := provider.(ChainQueryProvider)
		if !ok {
			return 0, &JSONRPCError{Code: -32000, Message: "block index query is unavailable"}
		}
		index, err := query.BlockIndex(ctx)
		if err != nil {
			return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		return index.EarliestHeight, nil
	default:
		height, err := parseHexQuantity(text)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid block tag"}
		}
		return types.Height(height), nil
	}
}

func web3FinalizedHeight(ctx context.Context, provider StatusProvider) (types.Height, *JSONRPCError) {
	status := provider.Status(ctx)
	if status.LatestFinalizedHeight > 0 {
		return status.LatestFinalizedHeight, nil
	}
	if finalityProvider, ok := provider.(FinalityProvider); ok {
		proof, err := finalityProvider.LatestFinalityProof(ctx)
		if err == nil && proof.Header.Height > 0 {
			return proof.Header.Height, nil
		}
		if err != nil && !errors.Is(err, node.ErrFinalityNotFound) && !errors.Is(err, store.ErrBlockNotFound) && !errors.Is(err, store.ErrBlockIndexNotFound) {
			return 0, &JSONRPCError{Code: -32000, Message: err.Error()}
		}
	}
	return 0, &JSONRPCError{Code: -32000, Message: "finalized block is unavailable"}
}

func web3QuantityParam(raw json.RawMessage) (uint64, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseHexQuantity(text)
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, nil
	}
	return 0, fmt.Errorf("quantity parameter is required")
}

func web3RewardPercentiles(raw json.RawMessage) ([]float64, error) {
	var percentiles []float64
	if err := json.Unmarshal(raw, &percentiles); err != nil {
		return nil, fmt.Errorf("invalid reward percentiles")
	}
	for _, percentile := range percentiles {
		if percentile < 0 || percentile > 100 {
			return nil, fmt.Errorf("reward percentile out of range")
		}
	}
	return percentiles, nil
}

func evmCallParam(params []json.RawMessage) (web3CallRequest, *JSONRPCError) {
	if len(params) == 0 {
		return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "call object is required"}
	}
	var payload web3TransactionCall
	if err := json.Unmarshal(params[0], &payload); err != nil {
		return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid call object"}
	}
	if payload.VM == "" {
		payload.VM = "evm"
	}
	if payload.Method == "" {
		payload.Method = "call"
	}
	if payload.From == "" {
		payload.From = "0x0000000000000000000000000000000000000000"
	}
	if payload.To == "" {
		payload.Method = "deploy"
		payload.To = "0x0000000000000000000000000000000000000000"
	}
	gasLimit := uint64(0)
	if payload.Gas != "" {
		value, err := parseHexQuantity(payload.Gas)
		if err != nil {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid gas quantity"}
		}
		gasLimit = value
	}
	callValue := uint64(0)
	callValueHex := ""
	if payload.Value != "" {
		value, err := parseHexQuantityBig(payload.Value)
		if err != nil {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid value quantity"}
		}
		callValueHex = hexQuantityBig(value)
		if value.IsUint64() {
			callValue = value.Uint64()
		}
	}
	gasPrice := uint64(0)
	gasPriceHex := ""
	maxFeePerGas := uint64(0)
	maxFeePerGasHex := ""
	maxPriorityFeePerGas := uint64(0)
	maxPriorityFeePerGasHex := ""
	nonce := uint64(0)
	switch {
	case payload.GasPrice != "":
		value, err := parseHexQuantityBig(payload.GasPrice)
		if err != nil || value.BitLen() > 256 {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid gasPrice quantity"}
		}
		gasPriceHex = hexQuantityBig(value)
		if value.IsUint64() {
			gasPrice = value.Uint64()
		}
	case payload.MaxFeePerGas != "":
		value, err := parseHexQuantityBig(payload.MaxFeePerGas)
		if err != nil || value.BitLen() > 256 {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid maxFeePerGas quantity"}
		}
		maxFeePerGasHex = hexQuantityBig(value)
		if value.IsUint64() {
			maxFeePerGas = value.Uint64()
		}
		if payload.MaxPriorityFeePerGas != "" {
			priority, err := parseHexQuantityBig(payload.MaxPriorityFeePerGas)
			if err != nil || priority.BitLen() > 256 {
				return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid maxPriorityFeePerGas quantity"}
			}
			maxPriorityFeePerGasHex = hexQuantityBig(priority)
			if priority.IsUint64() {
				maxPriorityFeePerGas = priority.Uint64()
			}
		}
	}
	if payload.Nonce != "" {
		value, err := parseHexQuantity(payload.Nonce)
		if err != nil {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid nonce quantity"}
		}
		nonce = value
	}
	if payload.Data == "" {
		payload.Data = "0x"
	}
	if _, err := hexBytes(payload.Data); err != nil {
		return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid data hex"}
	}
	setCodeAuthorizationsJSON := ""
	if raw := bytes.TrimSpace(payload.AuthorizationList); len(raw) > 0 && string(raw) != "null" {
		if !json.Valid(raw) || raw[0] != '[' {
			return web3CallRequest{}, &JSONRPCError{Code: -32602, Message: "invalid authorizationList"}
		}
		setCodeAuthorizationsJSON = string(raw)
	}
	return web3CallRequest{
		VM:                        payload.VM,
		From:                      payload.From,
		To:                        payload.To,
		Method:                    payload.Method,
		Input:                     payload.Data,
		GasLimit:                  gasLimit,
		Value:                     callValue,
		ValueHex:                  callValueHex,
		GasPrice:                  gasPrice,
		GasPriceHex:               gasPriceHex,
		MaxFeePerGas:              maxFeePerGas,
		MaxFeePerGasHex:           maxFeePerGasHex,
		MaxPriorityFeePerGas:      maxPriorityFeePerGas,
		MaxPriorityFeePerGasHex:   maxPriorityFeePerGasHex,
		Nonce:                     nonce,
		AccessList:                web3ContractAccessList(payload.AccessList),
		SetCodeAuthorizationsJSON: setCodeAuthorizationsJSON,
	}, nil
}

func web3ContractAccessList(entries []web3AccessListEntry) []contract.AccessListEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]contract.AccessListEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, contract.AccessListEntry{
			Address:     types.Address(entry.Address),
			StorageKeys: append([]string(nil), entry.StorageKeys...),
		})
	}
	return out
}

func web3LogFilterParam(ctx context.Context, provider StatusProvider, cfg Config, params []json.RawMessage) (web3Filter, *JSONRPCError) {
	if len(params) != 1 {
		return web3Filter{}, &JSONRPCError{Code: -32602, Message: "filter object is required"}
	}
	var payload struct {
		Address   any             `json:"address"`
		FromBlock string          `json:"fromBlock"`
		ToBlock   string          `json:"toBlock"`
		Topics    json.RawMessage `json:"topics"`
	}
	if err := json.Unmarshal(params[0], &payload); err != nil {
		return web3Filter{}, &JSONRPCError{Code: -32602, Message: "invalid filter object"}
	}
	addresses, rpcErr := web3LogAddresses(payload.Address)
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	latest := uint64(provider.Status(ctx).LatestHeight)
	fromBlock, rpcErr := web3LogBlockBound(ctx, provider, payload.FromBlock, latest, 0)
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	toBlock, rpcErr := web3LogBlockBound(ctx, provider, payload.ToBlock, latest, ^uint64(0))
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	if cfg.Web3LogMaxBlockRange == 0 {
		cfg.Web3LogMaxBlockRange = defaultMaxWeb3LogBlockRange
	}
	if toBlock != ^uint64(0) && toBlock >= fromBlock && toBlock-fromBlock+1 > cfg.Web3LogMaxBlockRange {
		return web3Filter{}, &JSONRPCError{Code: -32602, Message: "eth_getLogs block range exceeds configured limit"}
	}
	topics, rpcErr := web3LogTopics(payload.Topics)
	if rpcErr != nil {
		return web3Filter{}, rpcErr
	}
	return web3Filter{Type: "log", Addresses: addresses, FromBlock: fromBlock, ToBlock: toBlock, Topics: topics}, nil
}

func web3LogAddresses(value any) ([]string, *JSONRPCError) {
	switch address := value.(type) {
	case nil:
		return nil, nil
	case string:
		if address == "" {
			return nil, &JSONRPCError{Code: -32602, Message: "address is required"}
		}
		return []string{address}, nil
	case []any:
		if len(address) == 0 {
			return nil, nil
		}
		addresses := make([]string, 0, len(address))
		for _, item := range address {
			text, ok := item.(string)
			if !ok || text == "" {
				return nil, &JSONRPCError{Code: -32602, Message: "address is required"}
			}
			addresses = append(addresses, text)
		}
		return addresses, nil
	default:
		return nil, &JSONRPCError{Code: -32602, Message: "address is required"}
	}
}

func web3LogBlockBound(ctx context.Context, provider StatusProvider, tag string, latest uint64, empty uint64) (uint64, *JSONRPCError) {
	if tag == "" {
		return empty, nil
	}
	switch tag {
	case "latest", "pending":
		return latest, nil
	case "safe", "finalized":
		height, rpcErr := web3FinalizedHeight(ctx, provider)
		if rpcErr != nil {
			return 0, rpcErr
		}
		return uint64(height), nil
	case "earliest":
		return 0, nil
	default:
		height, err := parseHexQuantity(tag)
		if err != nil {
			return 0, &JSONRPCError{Code: -32602, Message: "invalid log block tag"}
		}
		return height, nil
	}
}

func web3LogTopics(raw json.RawMessage) ([][]string, *JSONRPCError) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, &JSONRPCError{Code: -32602, Message: "invalid log topics"}
	}
	topics := make([][]string, 0, len(values))
	for _, value := range values {
		switch item := value.(type) {
		case nil:
			topics = append(topics, nil)
		case string:
			topics = append(topics, []string{strings.ToLower(item)})
		case []any:
			options := make([]string, 0, len(item))
			for _, option := range item {
				text, ok := option.(string)
				if !ok || text == "" {
					return nil, &JSONRPCError{Code: -32602, Message: "invalid log topic option"}
				}
				options = append(options, strings.ToLower(text))
			}
			topics = append(topics, options)
		default:
			return nil, &JSONRPCError{Code: -32602, Message: "invalid log topic"}
		}
	}
	return topics, nil
}

func jsonRPCStringParam(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("string parameter is required")
	}
	if value == "" {
		return "", fmt.Errorf("string parameter is empty")
	}
	return value, nil
}

func rawJSONObject(value []byte) (json.RawMessage, *JSONRPCError) {
	if !json.Valid(value) {
		return nil, &JSONRPCError{Code: -32000, Message: "invalid application JSON"}
	}
	return append(json.RawMessage(nil), value...), nil
}

func hexBytes(value string) ([]byte, error) {
	value = strings.TrimPrefix(value, "0x")
	if len(value)%2 == 1 {
		value = "0" + value
	}
	return hex.DecodeString(value)
}

func normalizeHexDataString(value string) string {
	decoded, err := hexBytes(value)
	if err != nil {
		return value
	}
	return "0x" + hex.EncodeToString(decoded)
}

func normalizeHexQuantityString(value string) string {
	if strings.HasPrefix(value, "0x") {
		parsed, err := parseHexQuantityBig(value)
		if err != nil {
			return value
		}
		return "0x" + parsed.Text(16)
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return value
	}
	return "0x" + parsed.Text(16)
}

func normalizeStorageHex(value string) string {
	decoded, err := parseFixedWidthHex(value, 32)
	if err != nil {
		return value
	}
	if len(decoded) > 32 {
		return value
	}
	out := make([]byte, 32)
	copy(out[32-len(decoded):], decoded)
	return "0x" + hex.EncodeToString(out)
}

func parseHexQuantity(value string) (uint64, error) {
	if !strings.HasPrefix(value, "0x") {
		return 0, fmt.Errorf("quantity must use 0x prefix")
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return 0, fmt.Errorf("empty quantity")
	}
	return strconv.ParseUint(trimmed, 16, 64)
}

func parseHexQuantityBig(value string) (*big.Int, error) {
	if !strings.HasPrefix(value, "0x") {
		return nil, fmt.Errorf("quantity must use 0x prefix")
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return nil, fmt.Errorf("empty quantity")
	}
	parsed, ok := new(big.Int).SetString(trimmed, 16)
	if !ok || parsed.Sign() < 0 {
		return nil, fmt.Errorf("invalid quantity")
	}
	return parsed, nil
}

func parseHexAddress(value string) (types.Address, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "0x") {
		return "", fmt.Errorf("address must use 0x prefix")
	}
	clean := strings.TrimPrefix(trimmed, "0x")
	if len(clean) != 40 {
		return "", fmt.Errorf("address must be 20 bytes")
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) != 20 {
		return "", fmt.Errorf("invalid address")
	}
	return types.Address("0x" + strings.ToLower(clean)), nil
}

func parseHexHash(value string) (types.Hash, error) {
	var hash types.Hash
	decoded, err := hexBytes(value)
	if err != nil {
		return hash, err
	}
	if len(decoded) != len(hash) {
		return hash, fmt.Errorf("hash must be 32 bytes")
	}
	copy(hash[:], decoded)
	return hash, nil
}

func web3TransactionsRoot(txs []types.Tx) string {
	if root, ok := ethcompat.TransactionRoot(txs); ok {
		return root
	}
	hasher := sha256.New()
	for _, tx := range txs {
		_, _ = hasher.Write([]byte(web3TxHash(tx)))
	}
	return "0x" + hex.EncodeToString(hasher.Sum(nil))
}

func web3LogMatchesFilter(value any, filter web3Filter) bool {
	log, ok := value.(map[string]any)
	if !ok {
		return false
	}
	blockNumber := web3LogBlockNumber(log)
	if blockNumber < filter.FromBlock || blockNumber > filter.ToBlock {
		return false
	}
	if len(filter.Topics) == 0 {
		return true
	}
	topics := web3LogTopicValues(log)
	for index, accepted := range filter.Topics {
		if len(accepted) == 0 {
			continue
		}
		if index >= len(topics) {
			return false
		}
		found := false
		actual := strings.ToLower(topics[index])
		for _, topic := range accepted {
			if actual == topic {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func web3NormalizeLog(value any) any {
	log, ok := value.(map[string]any)
	if !ok {
		return value
	}
	normalized := make(map[string]any, len(log)+3)
	for key, item := range log {
		normalized[key] = item
	}
	if blockNumber := web3LogBlockNumber(log); blockNumber > 0 {
		normalized["blockNumber"] = hexQuantity(blockNumber)
	}
	if txHash := web3LogString(log, "transaction_hash", "transactionHash", "tx_hash"); txHash != "" {
		normalized["transactionHash"] = txHash
	}
	if logIndex := web3LogUint(log, "log_index", "logIndex"); logIndex > 0 {
		normalized["logIndex"] = hexQuantity(logIndex)
	}
	return normalized
}

func web3LogBlockNumber(log map[string]any) uint64 {
	return web3LogUint(log, "block_number", "blockNumber", "height")
}

func web3LogTopicValues(log map[string]any) []string {
	raw, found := log["topics"]
	if !found {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	topics := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		topics = append(topics, text)
	}
	return topics
}

func web3LogString(log map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := log[key].(string)
		if ok {
			return value
		}
	}
	return ""
}

func web3LogUint(log map[string]any, keys ...string) uint64 {
	for _, key := range keys {
		value, found := log[key]
		if !found {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed >= 0 {
				return uint64(typed)
			}
		case uint64:
			return typed
		case string:
			if strings.HasPrefix(typed, "0x") {
				parsed, err := parseHexQuantity(typed)
				if err == nil {
					return parsed
				}
				continue
			}
			parsed, err := strconv.ParseUint(typed, 10, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func web3SeenLogSet(logs []any) map[string]bool {
	seen := make(map[string]bool, len(logs))
	for _, log := range logs {
		id := web3LogID(log)
		if id != "" {
			seen[id] = true
		}
	}
	return seen
}

func web3LogID(value any) string {
	log, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	txHash := web3LogString(log, "transactionHash", "transaction_hash", "tx_hash")
	if txHash == "" {
		return ""
	}
	return txHash + ":" + strconv.FormatUint(web3LogUint(log, "logIndex", "log_index"), 10)
}

func hexQuantity(value uint64) string {
	return "0x" + strconv.FormatUint(value, 16)
}

func hexQuantityBig(value *big.Int) string {
	if value == nil || value.Sign() == 0 {
		return "0x0"
	}
	return "0x" + value.Text(16)
}

func chainNumericID(chainID string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(chainID))
	value := hash.Sum64()
	if value == 0 {
		return 1
	}
	return value
}

func web3ChainID(status node.Status) uint64 {
	if status.EVMChainID != 0 {
		return status.EVMChainID
	}
	return chainNumericID(status.ChainID)
}

func web3Syncing(status node.Status) any {
	if !status.Running || status.LatestHeight == 0 {
		return false
	}
	finalized := status.LatestFinalizedHeight
	if finalized == 0 || finalized >= status.LatestHeight {
		return false
	}
	return map[string]any{
		"startingBlock": hexQuantity(uint64(finalized)),
		"currentBlock":  hexQuantity(uint64(finalized)),
		"highestBlock":  hexQuantity(uint64(status.LatestHeight)),
	}
}

func web3Mining(provider StatusProvider, ctx context.Context) bool {
	status := provider.Status(ctx)
	if !status.Running {
		return false
	}
	if controller, ok := provider.(ConsensusLoopController); ok {
		return controller.ConsensusLoopRunning()
	}
	return true
}

func newWeb3FilterStore() *web3FilterStore {
	return &web3FilterStore{max: defaultMaxWeb3Filters, filters: make(map[string]web3Filter)}
}

func newWeb3FilterStoreWithConfig(cfg Config) (*web3FilterStore, error) {
	filters := newWeb3FilterStore()
	snapshotPath := strings.TrimSpace(cfg.Web3FilterSnapshotPath)
	if snapshotPath == "" {
		return filters, nil
	}
	filters.setOnChange(func(snapshot Web3FilterStoreSnapshot) error {
		return saveWeb3FilterStoreSnapshotAtomic(snapshotPath, snapshot)
	})
	snapshot, err := loadWeb3FilterStoreSnapshot(snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return filters, nil
	}
	if err != nil {
		return filters, err
	}
	filters.Restore(snapshot)
	return filters, nil
}

func loadWeb3FilterStoreSnapshot(path string) (Web3FilterStoreSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Web3FilterStoreSnapshot{}, err
	}
	var snapshot Web3FilterStoreSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Web3FilterStoreSnapshot{}, err
	}
	return snapshot, nil
}

func saveWeb3FilterStoreSnapshotAtomic(path string, snapshot Web3FilterStoreSnapshot) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer os.Remove(tmpPath)
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

func (store *web3FilterStore) addLog(filter web3Filter, logs []any, latestHeight uint64) string {
	store.mu.Lock()
	store.nextID++
	id := hexQuantity(store.nextID)
	filter.Type = "log"
	filter.LastHeight = latestHeight
	filter.SeenLogs = web3SeenLogSet(logs)
	store.addLocked(id, filter)
	store.mu.Unlock()
	store.persistAfterChange()
	return id
}

func (store *web3FilterStore) addBlock(latestHeight uint64) string {
	store.mu.Lock()
	store.nextID++
	id := hexQuantity(store.nextID)
	store.addLocked(id, web3Filter{Type: "block", LastHeight: latestHeight})
	store.mu.Unlock()
	store.persistAfterChange()
	return id
}

func (store *web3FilterStore) addPending(hashes []types.Hash) string {
	store.mu.Lock()
	store.nextID++
	id := hexQuantity(store.nextID)
	seen := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		seen[web3HashString(hash)] = true
	}
	store.addLocked(id, web3Filter{Type: "pending", SeenPending: seen})
	store.mu.Unlock()
	store.persistAfterChange()
	return id
}

func (store *web3FilterStore) get(id string) (web3Filter, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	filter, found := store.filters[id]
	return filter, found
}

func (store *web3FilterStore) mark(id string, height uint64) {
	store.mu.Lock()
	filter, found := store.filters[id]
	if !found {
		store.mu.Unlock()
		return
	}
	filter.LastHeight = height
	store.filters[id] = filter
	store.mu.Unlock()
	store.persistAfterChange()
}

func (store *web3FilterStore) replace(id string, filter web3Filter) {
	store.mu.Lock()
	if _, found := store.filters[id]; !found {
		store.mu.Unlock()
		return
	}
	store.filters[id] = filter
	store.mu.Unlock()
	store.persistAfterChange()
}

func (store *web3FilterStore) remove(id string) bool {
	store.mu.Lock()
	if _, found := store.filters[id]; !found {
		store.mu.Unlock()
		return false
	}
	store.removeLocked(id)
	store.mu.Unlock()
	store.persistAfterChange()
	return true
}

func (store *web3FilterStore) addLocked(id string, filter web3Filter) {
	if _, found := store.filters[id]; !found {
		store.order = append(store.order, id)
	}
	store.filters[id] = filter
	limit := store.max
	if limit <= 0 {
		limit = defaultMaxWeb3Filters
	}
	for len(store.filters) > limit && len(store.order) > 0 {
		store.removeLocked(store.order[0])
	}
}

func (store *web3FilterStore) removeLocked(id string) {
	delete(store.filters, id)
	for index, current := range store.order {
		if current == id {
			store.order = append(store.order[:index], store.order[index+1:]...)
			return
		}
	}
}

func (store *web3FilterStore) Snapshot() Web3FilterStoreSnapshot {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.snapshotLocked()
}

func (store *web3FilterStore) snapshotLocked() Web3FilterStoreSnapshot {
	filters := make([]web3Filter, 0, len(store.order))
	for _, id := range store.order {
		filter, found := store.filters[id]
		if !found {
			continue
		}
		filter.ID = id
		filter.Addresses = append([]string(nil), filter.Addresses...)
		filter.Topics = cloneWeb3Topics(filter.Topics)
		filter.SeenPending = cloneStringBoolMap(filter.SeenPending)
		filter.SeenLogs = cloneStringBoolMap(filter.SeenLogs)
		filters = append(filters, filter)
	}
	return Web3FilterStoreSnapshot{
		NextID:  store.nextID,
		Max:     store.max,
		Order:   append([]string(nil), store.order...),
		Filters: filters,
	}
}

func (store *web3FilterStore) setOnChange(callback func(Web3FilterStoreSnapshot) error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.onChange = callback
}

func (store *web3FilterStore) persistAfterChange() {
	store.mu.Lock()
	callback := store.onChange
	if callback == nil {
		store.mu.Unlock()
		return
	}
	snapshot := store.snapshotLocked()
	store.mu.Unlock()

	err := callback(snapshot)
	store.mu.Lock()
	store.lastPersistErr = err
	store.mu.Unlock()
}

func (store *web3FilterStore) persistError() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.lastPersistErr
}

func web3FilterPersistRPCError(store *web3FilterStore) *JSONRPCError {
	if store == nil {
		return nil
	}
	if err := store.persistError(); err != nil {
		return &JSONRPCError{Code: -32000, Message: "web3 filter snapshot persistence failed: " + err.Error()}
	}
	return nil
}

func (store *web3FilterStore) Restore(snapshot Web3FilterStoreSnapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.nextID = snapshot.NextID
	store.max = snapshot.Max
	store.order = make([]string, 0, len(snapshot.Filters))
	store.filters = make(map[string]web3Filter, len(snapshot.Filters))
	for _, filter := range snapshot.Filters {
		id := filter.ID
		if id == "" {
			continue
		}
		filter.ID = ""
		filter.Addresses = append([]string(nil), filter.Addresses...)
		filter.Topics = cloneWeb3Topics(filter.Topics)
		filter.SeenPending = cloneStringBoolMap(filter.SeenPending)
		filter.SeenLogs = cloneStringBoolMap(filter.SeenLogs)
		store.order = append(store.order, id)
		store.filters[id] = filter
	}
	if len(snapshot.Order) > 0 {
		ordered := make([]string, 0, len(snapshot.Order))
		seen := make(map[string]struct{}, len(snapshot.Order))
		for _, id := range snapshot.Order {
			if _, found := store.filters[id]; !found {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			ordered = append(ordered, id)
			seen[id] = struct{}{}
		}
		for id := range store.filters {
			if _, found := seen[id]; !found {
				ordered = append(ordered, id)
			}
		}
		store.order = ordered
	}
	limit := store.max
	if limit <= 0 {
		limit = defaultMaxWeb3Filters
	}
	for len(store.filters) > limit && len(store.order) > 0 {
		store.removeLocked(store.order[0])
	}
}

func cloneWeb3Topics(topics [][]string) [][]string {
	if topics == nil {
		return nil
	}
	cloned := make([][]string, len(topics))
	for index, topicGroup := range topics {
		cloned[index] = append([]string(nil), topicGroup...)
	}
	return cloned
}

func cloneStringBoolMap(values map[string]bool) map[string]bool {
	if values == nil {
		return nil
	}
	cloned := make(map[string]bool, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
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

func parseOptionalHeight(value string) (types.Height, bool) {
	if value == "" || value == "latest" {
		return 0, true
	}
	height, err := strconv.ParseUint(value, 10, 64)
	if err != nil || height == 0 {
		return 0, false
	}
	return types.Height(height), true
}

func parseOptionalUintQuery(request *http.Request, key string, fallback uint64) (uint64, error) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s query", key)
	}
	return parsed, nil
}

func parseIBCQueryPath(path string) ([]string, bool) {
	selector := strings.TrimPrefix(path, "/ibc/")
	if selector == "" || selector == path {
		return nil, false
	}
	parts := strings.Split(selector, "/")
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
	}
	switch parts[0] {
	case "client", "connection":
		return parts, len(parts) == 2
	case "channel":
		return parts, len(parts) == 3
	case "packet":
		return parts, len(parts) == 6
	default:
		return nil, false
	}
}

func parseIBCPacketProofPath(path string) (ibckeeper.Packet, bool) {
	selector := strings.TrimPrefix(path, "/ibc/proof/packet/")
	if selector == "" || selector == path {
		return ibckeeper.Packet{}, false
	}
	parts := strings.Split(selector, "/")
	if len(parts) != 5 {
		return ibckeeper.Packet{}, false
	}
	for _, part := range parts {
		if part == "" {
			return ibckeeper.Packet{}, false
		}
	}
	sequence, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || sequence == 0 {
		return ibckeeper.Packet{}, false
	}
	return ibckeeper.Packet{
		Sequence:           sequence,
		SourcePort:         parts[1],
		SourceChannel:      parts[2],
		DestinationPort:    parts[3],
		DestinationChannel: parts[4],
		Data:               []byte("proof"),
	}, true
}

func writeIBCQueryError(writer http.ResponseWriter, response vexoapp.QueryResponse) {
	message := response.Log
	if message == "" {
		message = "IBC query failed"
	}
	switch response.Code {
	case 2:
		writeError(writer, http.StatusBadRequest, message)
	case 3:
		writeError(writer, http.StatusNotFound, message)
	default:
		writeError(writer, http.StatusInternalServerError, message)
	}
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

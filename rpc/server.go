package rpc

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const defaultReadHeaderTimeout = 5 * time.Second
const defaultMaxRequestBytes = 1024 * 1024
const defaultMaxWeb3Filters = 1024
const defaultMaxWeb3LogResults = 10_000
const defaultMaxWeb3LogBlockRange = 10_000

// maxWeb3LogAddresses is the most addresses one log filter may name.
const maxWeb3LogAddresses = 32
const defaultWeb3BlockGasLimit = 10_000_000
const stableAPIPrefix = "/v1"

// Config configures the RPC server.
//
// The admin endpoints fail closed: without AdminToken or AdminTokens none is authorized.
// CORSAllowedOrigins lists the browser origins allowed to call the API; empty means no
// cross-origin access, and a WebSocket handshake that carries an unlisted Origin is refused.
// AllowedHosts lists the host names, besides localhost and IP literals, that an admin
// request may address.
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
	AllowedHosts                  []string
	TLSConfig                     *tls.Config
	EnablePprof                   bool
	AllowUnprotectedLegacyTx      bool
	EVMChainConfigJSON            string
	StrictEVMStateRoot            bool
	RPCGasCap                     uint64
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
	managedTransactionMu          *sync.Mutex
}

type Server struct {
	provider               StatusProvider
	server                 *http.Server
	filterStore            *web3FilterStore
	web3FilterSnapshotPath string
	startupErr             error
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
	if cfg.managedTransactionMu == nil {
		cfg.managedTransactionMu = &sync.Mutex{}
	}
	mux := http.NewServeMux()
	if cfg.EnablePprof {
		registerPprofHandlers(mux, cfg)
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
		hash, err := submitter.SubmitEvidence(request.Context(), evidence)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, evidenceResponse(evidence, hash))
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
	mux.HandleFunc("/light/", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		lightProvider, ok := provider.(LightBlockProvider)
		if !ok {
			writeError(writer, http.StatusNotImplemented, "light block query is unavailable")
			return
		}
		height, ok := parseHeightSelector(request.URL.Path, "/light/")
		if !ok {
			writeError(writer, http.StatusBadRequest, "invalid light block height")
			return
		}
		block, err := lightProvider.LightBlock(request.Context(), height)
		if err != nil {
			writeFinalityProof(writer, finality.Proof{}, err, false)
			return
		}
		writeJSON(writer, http.StatusOK, lightBlockResponse(block))
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
	registerSnapshotRoutes(mux, provider)
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
			if !allowAdmin(writer, request, cfg, "recovery") || !requireJSONContentType(writer, request) {
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
		if !allowAdminPost(writer, request, cfg, "prune") {
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
		if !allowAdminPost(writer, request, cfg, "replay") {
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
		if !allowAdminPost(writer, request, cfg, "consensus") {
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
		if !allowAdminPost(writer, request, cfg, "consensus") {
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

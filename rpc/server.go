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

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
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

type ChainQueryProvider interface {
	BlockProvider
	BlockIndex(ctx context.Context) (store.BlockIndex, error)
	LatestState(ctx context.Context) (store.StateRecord, error)
	StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error)
}

type ValidatorQueryProvider interface {
	ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error)
	Committee(ctx context.Context, height types.Height, round types.Round, seed types.Hash) (committee.Committee, error)
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

func stateRootResponse(root store.StateRootRecord) StateRootResponse {
	return StateRootResponse{
		Height:    uint64(root.Height),
		Namespace: root.Namespace,
		Root:      hex.EncodeToString(root.Root[:]),
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

func writeError(writer http.ResponseWriter, statusCode int, message string) {
	writeJSON(writer, statusCode, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

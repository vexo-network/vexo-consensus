package rpc

import (
	"encoding/json"
	"time"

	"github.com/vexo-network/vexo-consensus/queryproof"
)

type AdminAuditEvent struct {
	Scope      string
	Path       string
	Method     string
	RemoteAddr string
	Authorized bool
	Reason     string
	At         time.Time
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
	RetentionPruneFailures      uint64  `json:"retention_prune_failures"`
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

// SubmitEvidenceResponse acknowledges evidence that was verified, pooled and gossiped. Nothing is
// punished yet: EvidenceHash identifies the payload to wrap in a transaction for a block.
type SubmitEvidenceResponse struct {
	Accepted     bool   `json:"accepted"`
	EvidenceHash string `json:"evidence_hash"`
	Type         string `json:"type"`
	Validator    string `json:"validator"`
	Height       uint64 `json:"height"`
	Round        uint64 `json:"round"`
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

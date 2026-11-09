package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vexo-network/vexo-consensus/address"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	defaultHomeDir          = ".vexo"
	defaultChainID          = "vexo-chain"
	defaultValidatorID      = "validator-1"
	defaultVRFKeyFileName   = "validator.vrf.key.json"
	defaultP2PBasePort      = 26656
	defaultRPCBasePort      = 26657
	configFileName          = "config.json"
	moduleConfigFileName    = "module_config.json"
	networkConfigFileName   = "network_config.json"
	consensusConfigFileName = "consensus_config.json"
	mempoolConfigFileName   = "mempool_config.json"
	logConfigFileName       = "log_config.json"
	genesisFileName         = "genesis.json"
	configSchemaVersion     = "v1"
	genesisSchemaVersion    = "v1"
	moduleSchemaVersion     = "v1"
	networkSchemaVersion    = "v1"
	consensusSchemaVersion  = "v1"
	mempoolSchemaVersion    = "v1"
	logSchemaVersion        = "v1"
)

var errValidationFailed = errors.New("validation failed")

type configDocument struct {
	SchemaVersion        string `json:"schema_version"`
	RequireNetworkSafety bool   `json:"require_network_safety,omitempty"`
	DataDir              string `json:"data_dir"`
	ValidatorID          string `json:"validator_id,omitempty"`
	ChainID              string `json:"chain_id"`
	ModuleConfigPath     string `json:"module_config_path,omitempty"`
	NetworkConfigPath    string `json:"network_config_path,omitempty"`
	ConsensusConfigPath  string `json:"consensus_config_path,omitempty"`
	MempoolConfigPath    string `json:"mempool_config_path,omitempty"`
	LogConfigPath        string `json:"log_config_path,omitempty"`
	// RetainBlocks is node.Config.RetainBlocks; without it the node keeps node.DefaultRetainBlocks.
	RetainBlocks    *uint64                 `json:"retain_blocks,omitempty"`
	Runtime         runtimeConfig           `json:"-"`
	Chain           chainConfigDocument     `json:"-"`
	LegacyModule    moduleConfigDocument    `json:"-"`
	LegacyNetwork   networkConfigDocument   `json:"-"`
	LegacyConsensus consensusConfigDocument `json:"-"`
	LegacyMempool   mempoolConfigDocument   `json:"-"`
	LegacyLog       logConfigDocument       `json:"-"`
}

type chainConfigDocument struct {
	ChainID   string                    `json:"ChainID"`
	Crypto    config.CryptoConfig       `json:"Crypto"`
	VRF       config.VRFConfig          `json:"VRF"`
	Validator validator.AdmissionConfig `json:"Validator"`
	Committee committee.RotationPolicy  `json:"Committee"`
	Mempool   mempool.FIFOConfig        `json:"Mempool"`
	P2P       p2p.ScoreConfig           `json:"P2P"`
}

type legacyConfigDocument struct {
	SchemaVersion        string        `json:"schema_version"`
	RequireNetworkSafety bool          `json:"require_network_safety,omitempty"`
	DataDir              string        `json:"data_dir"`
	ValidatorID          string        `json:"validator_id,omitempty"`
	ChainID              string        `json:"chain_id,omitempty"`
	ModuleConfigPath     string        `json:"module_config_path,omitempty"`
	NetworkConfigPath    string        `json:"network_config_path,omitempty"`
	ConsensusConfigPath  string        `json:"consensus_config_path,omitempty"`
	MempoolConfigPath    string        `json:"mempool_config_path,omitempty"`
	LogConfigPath        string        `json:"log_config_path,omitempty"`
	RetainBlocks         *uint64       `json:"retain_blocks,omitempty"`
	Runtime              runtimeConfig `json:"runtime,omitempty"`
	Chain                config.Config `json:"chain,omitempty"`
}

type moduleConfigDocument struct {
	SchemaVersion string                   `json:"schema_version"`
	Application   config.ApplicationConfig `json:"application"`
	Execution     config.ExecutionConfig   `json:"execution"`
	Bank          config.BankConfig        `json:"bank"`
	Staking       config.StakingConfig     `json:"staking"`
	Governance    governance.TallyPolicy   `json:"governance"`
}

type networkConfigDocument struct {
	SchemaVersion string                 `json:"schema_version"`
	RPC           runtimeRPCConfig       `json:"rpc"`
	P2P           runtimeP2PConfig       `json:"p2p"`
	StateSync     runtimeStateSyncConfig `json:"state_sync,omitempty"`
	PeerScoring   p2p.ScoreConfig        `json:"peer_scoring"`
}

type consensusConfigDocument struct {
	SchemaVersion string                    `json:"schema_version"`
	Consensus     runtimeConsensusConfig    `json:"consensus"`
	Crypto        config.CryptoConfig       `json:"crypto"`
	VRF           config.VRFConfig          `json:"vrf"`
	VRFKeyPaths   []string                  `json:"vrf_key_paths,omitempty"`
	Validator     validator.AdmissionConfig `json:"validator"`
	Committee     committee.RotationPolicy  `json:"committee"`
}

type mempoolConfigDocument struct {
	SchemaVersion string             `json:"schema_version"`
	Mempool       mempool.FIFOConfig `json:"mempool"`
}

type logConfigDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Log           runtimeLogConfig `json:"log"`
}

type runtimeConfig struct {
	RPC       runtimeRPCConfig       `json:"rpc,omitempty"`
	P2P       runtimeP2PConfig       `json:"p2p,omitempty"`
	StateSync runtimeStateSyncConfig `json:"state_sync,omitempty"`
	Consensus runtimeConsensusConfig `json:"consensus,omitempty"`
	Log       runtimeLogConfig       `json:"log,omitempty"`
}

type runtimeRPCConfig struct {
	Enabled               bool                `json:"enabled"`
	Address               string              `json:"address,omitempty"`
	AdminToken            string              `json:"admin_token,omitempty"`
	AdminTokens           map[string][]string `json:"admin_tokens,omitempty"`
	CORSAllowedOrigins    []string            `json:"cors_allowed_origins,omitempty"`
	AllowedHosts          []string            `json:"allowed_hosts,omitempty"`
	TLSCertPath           string              `json:"tls_cert_path,omitempty"`
	TLSKeyPath            string              `json:"tls_key_path,omitempty"`
	TLSCAPath             string              `json:"tls_ca_path,omitempty"`
	TLSServerName         string              `json:"tls_server_name,omitempty"`
	EnablePprof           bool                `json:"enable_pprof,omitempty"`
	RequestTimeout        string              `json:"request_timeout,omitempty"`
	ShutdownTimeout       string              `json:"shutdown_timeout,omitempty"`
	MaxRequestBytes       int64               `json:"max_request_bytes,omitempty"`
	RateLimitWindow       string              `json:"rate_limit_window,omitempty"`
	RateLimitMaxRequests  int                 `json:"rate_limit_max_requests,omitempty"`
	Web3MaxSubscriptions  int                 `json:"web3_max_subscriptions_per_connection,omitempty"`
	Web3IdleTimeout       string              `json:"web3_idle_timeout,omitempty"`
	Web3FilterSnapshot    string              `json:"web3_filter_snapshot,omitempty"`
	EVMManagedAccounts    bool                `json:"evm_managed_accounts,omitempty"`
	EVMAccountPrivateKeys []string            `json:"evm_account_private_keys,omitempty"`
	EVMAccountKeyEnvs     []string            `json:"evm_account_key_envs,omitempty"`
}

type runtimeP2PConfig struct {
	Enabled                bool              `json:"enabled"`
	NodeID                 string            `json:"node_id,omitempty"`
	NodeKeyPath            string            `json:"node_key_path,omitempty"`
	ListenAddress          string            `json:"listen_address,omitempty"`
	DialTimeout            string            `json:"dial_timeout,omitempty"`
	NetworkID              string            `json:"network_id,omitempty"`
	MaxMessageBytes        uint64            `json:"max_message_bytes,omitempty"`
	MaxPeers               int               `json:"max_peers,omitempty"`
	AuthToken              string            `json:"auth_token,omitempty"`
	AuthReplayPath         string            `json:"auth_replay_path,omitempty"`
	RequireAuthReplayStore bool              `json:"require_auth_replay_store,omitempty"`
	TLSCertPath            string            `json:"tls_cert_path,omitempty"`
	TLSKeyPath             string            `json:"tls_key_path,omitempty"`
	TLSCAPath              string            `json:"tls_ca_path,omitempty"`
	TLSServerName          string            `json:"tls_server_name,omitempty"`
	AddrBookPath           string            `json:"addr_book_path,omitempty"`
	AddrBookMaxFails       int               `json:"addr_book_max_failures,omitempty"`
	Peers                  map[string]string `json:"peers,omitempty"`
	Seeds                  map[string]string `json:"seeds,omitempty"`
}

type runtimeStateSyncConfig struct {
	Enabled           bool     `json:"enabled,omitempty"`
	SnapshotURLs      []string `json:"snapshot_urls,omitempty"`
	Timeout           string   `json:"timeout,omitempty"`
	MinHeight         uint64   `json:"min_height,omitempty"`
	RequireFresh      bool     `json:"require_fresh,omitempty"`
	TrustLocalHigher  bool     `json:"trust_local_higher,omitempty"`
	MaxSnapshotBytes  int64    `json:"max_snapshot_bytes,omitempty"`
	RetryAllSnapshots bool     `json:"retry_all_snapshots,omitempty"`
	// TrustHeight and TrustHash name the block the operator trusts; state sync restores only the
	// state that block's header commits (see verifySnapshotTrust).
	TrustHeight uint64 `json:"trust_height,omitempty"`
	TrustHash   string `json:"trust_hash,omitempty"`
}

type runtimeConsensusConfig struct {
	LoopEnabled                 bool   `json:"loop_enabled"`
	Interval                    string `json:"interval,omitempty"`
	TimeoutPropose              string `json:"timeout_propose,omitempty"`
	TimeoutPrevote              string `json:"timeout_prevote,omitempty"`
	TimeoutPrecommit            string `json:"timeout_precommit,omitempty"`
	RoundTimeout                string `json:"round_timeout,omitempty"`
	MaxBlockBytes               int64  `json:"max_block_bytes,omitempty"`
	CreateEmptyBlocks           bool   `json:"create_empty_blocks"`
	ExecutionCommit             string `json:"execution_commit,omitempty"`
	AllowUnsafeQCCommit         bool   `json:"allow_unsafe_qc_commit,omitempty"`
	AdaptiveRoundTimeoutEnabled *bool  `json:"adaptive_round_timeout_enabled,omitempty"`
	RecoveryFinalityGateEnabled *bool  `json:"recovery_finality_gate_enabled,omitempty"`
}

type runtimeLogConfig struct {
	Format       string `json:"format,omitempty"`
	Level        string `json:"level,omitempty"`
	CommitEvents *bool  `json:"commit_events,omitempty"`
	PeerEvents   *bool  `json:"peer_events,omitempty"`
}

type genesisDocument struct {
	SchemaVersion string              `json:"schema_version"`
	ChainID       string              `json:"chain_id"`
	Validators    []validatorDocument `json:"validators"`
	AppState      map[string]string   `json:"app_state,omitempty"`
	Governance    map[string]uint64   `json:"governance,omitempty"`
}

type validatorDocument struct {
	ID          string            `json:"id"`
	Address     string            `json:"address"`
	PublicKey   string            `json:"public_key,omitempty"`
	VotingPower uint64            `json:"voting_power"`
	Stake       uint64            `json:"stake"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func readConfigDocument(path string) (configDocument, error) {
	var legacy legacyConfigDocument
	if err := readJSONFile(path, &legacy); err != nil {
		return configDocument{}, err
	}
	if legacy.SchemaVersion != configSchemaVersion {
		return configDocument{}, fmt.Errorf("unsupported config schema %q", legacy.SchemaVersion)
	}
	chainID := legacy.ChainID
	if chainID == "" {
		chainID = legacy.Chain.ChainID
	}
	return configDocument{
		SchemaVersion:        legacy.SchemaVersion,
		RequireNetworkSafety: legacy.RequireNetworkSafety,
		DataDir:              legacy.DataDir,
		ValidatorID:          legacy.ValidatorID,
		ChainID:              chainID,
		ModuleConfigPath:     legacy.ModuleConfigPath,
		NetworkConfigPath:    legacy.NetworkConfigPath,
		ConsensusConfigPath:  legacy.ConsensusConfigPath,
		MempoolConfigPath:    legacy.MempoolConfigPath,
		LogConfigPath:        legacy.LogConfigPath,
		RetainBlocks:         legacy.RetainBlocks,
		Runtime:              legacy.Runtime,
		Chain:                chainConfigFromConfig(legacy.Chain),
		LegacyModule:         moduleConfigFromConfig(legacy.Chain),
		LegacyNetwork:        networkConfigFromConfig(legacy.Chain, legacy.Runtime),
		LegacyConsensus:      consensusConfigFromConfig(legacy.Chain, legacy.Runtime),
		LegacyMempool:        mempoolConfigFromConfig(legacy.Chain),
		LegacyLog:            logConfigFromRuntime(legacy.Runtime),
	}, nil
}

func readModuleConfigDocument(path string) (moduleConfigDocument, error) {
	var document moduleConfigDocument
	if err := readJSONFile(path, &document); err != nil {
		return moduleConfigDocument{}, err
	}
	if document.SchemaVersion != moduleSchemaVersion {
		return moduleConfigDocument{}, fmt.Errorf("unsupported module config schema %q", document.SchemaVersion)
	}
	return document, nil
}

func readNetworkConfigDocument(path string) (networkConfigDocument, error) {
	var document networkConfigDocument
	if err := readJSONFile(path, &document); err != nil {
		return networkConfigDocument{}, err
	}
	if document.SchemaVersion != networkSchemaVersion {
		return networkConfigDocument{}, fmt.Errorf("unsupported network config schema %q", document.SchemaVersion)
	}
	return document, nil
}

func readConsensusConfigDocument(path string) (consensusConfigDocument, error) {
	var document consensusConfigDocument
	if err := readJSONFile(path, &document); err != nil {
		return consensusConfigDocument{}, err
	}
	if document.SchemaVersion != consensusSchemaVersion {
		return consensusConfigDocument{}, fmt.Errorf("unsupported consensus config schema %q", document.SchemaVersion)
	}
	return document, nil
}

func readMempoolConfigDocument(path string) (mempoolConfigDocument, error) {
	var document mempoolConfigDocument
	if err := readJSONFile(path, &document); err != nil {
		return mempoolConfigDocument{}, err
	}
	if document.SchemaVersion != mempoolSchemaVersion {
		return mempoolConfigDocument{}, fmt.Errorf("unsupported mempool config schema %q", document.SchemaVersion)
	}
	return document, nil
}

func readLogConfigDocument(path string) (logConfigDocument, error) {
	var document logConfigDocument
	if err := readJSONFile(path, &document); err != nil {
		return logConfigDocument{}, err
	}
	if document.SchemaVersion != logSchemaVersion {
		return logConfigDocument{}, fmt.Errorf("unsupported log config schema %q", document.SchemaVersion)
	}
	return document, nil
}

func loadNodeConfig(path string) (node.Config, error) {
	document, err := readConfigDocument(path)
	if err != nil {
		return node.Config{}, err
	}
	moduleDocument, err := loadModuleConfigForConfig(path, document)
	if err != nil {
		return node.Config{}, err
	}
	networkDocument, err := loadNetworkConfigForConfig(path, document)
	if err != nil {
		return node.Config{}, err
	}
	consensusDocument, err := loadConsensusConfigForConfig(path, document)
	if err != nil {
		return node.Config{}, err
	}
	mempoolDocument, err := loadMempoolConfigForConfig(path, document)
	if err != nil {
		return node.Config{}, err
	}
	chain := configFromConfigDocuments(document, moduleDocument, networkDocument, consensusDocument, mempoolDocument)
	cfg := node.Config{
		Chain:                chain,
		DataDir:              document.DataDir,
		ValidatorID:          types.ValidatorID(document.ValidatorID),
		RequireNetworkSafety: true,
		RetainBlocks:         node.DefaultRetainBlocks,
	}
	if document.RetainBlocks != nil {
		cfg.RetainBlocks = *document.RetainBlocks
	}
	if err := cfg.Validate(); err != nil {
		return node.Config{}, err
	}
	return cfg, nil
}

func loadModuleConfigForConfig(configPath string, document configDocument) (moduleConfigDocument, error) {
	modulePath := resolveModuleConfigPath(filepath.Dir(configPath), document.ModuleConfigPath)
	moduleDocument, err := readModuleConfigDocument(modulePath)
	if err == nil {
		return moduleDocument, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if hasLegacyModuleConfig(document.LegacyModule) {
			return document.LegacyModule, nil
		}
		return defaultModuleConfigDocument(documentChainID(document)), nil
	}
	return moduleConfigDocument{}, err
}

func loadNetworkConfigForConfig(configPath string, document configDocument) (networkConfigDocument, error) {
	networkPath := resolveNetworkConfigPath(filepath.Dir(configPath), document.NetworkConfigPath)
	networkDocument, err := readNetworkConfigDocument(networkPath)
	if err == nil {
		return networkDocument, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if hasLegacyNetworkConfig(document.LegacyNetwork) {
			return document.LegacyNetwork, nil
		}
		return defaultNetworkConfigDocument(documentChainID(document), document.DataDir, document.ValidatorID), nil
	}
	return networkConfigDocument{}, err
}

func loadConsensusConfigForConfig(configPath string, document configDocument) (consensusConfigDocument, error) {
	consensusPath := resolveConsensusConfigPath(filepath.Dir(configPath), document.ConsensusConfigPath)
	consensusDocument, err := readConsensusConfigDocument(consensusPath)
	if err == nil {
		if err := loadVRFKeyDocuments(filepath.Dir(consensusPath), &consensusDocument); err != nil {
			return consensusConfigDocument{}, err
		}
		return consensusDocument, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if hasLegacyConsensusConfig(document.LegacyConsensus) {
			if err := loadVRFKeyDocuments(filepath.Dir(configPath), &document.LegacyConsensus); err != nil {
				return consensusConfigDocument{}, err
			}
			return document.LegacyConsensus, nil
		}
		return defaultConsensusConfigDocument(documentChainID(document), document.DataDir, document.ValidatorID), nil
	}
	return consensusConfigDocument{}, err
}

func loadVRFKeyDocuments(configDir string, document *consensusConfigDocument) error {
	if len(document.VRFKeyPaths) == 0 {
		return nil
	}
	keys := cloneVRFKeys(document.VRF.Keys)
	for _, configuredPath := range document.VRFKeyPaths {
		keyPath := configuredPath
		if !filepath.IsAbs(keyPath) {
			keyPath = filepath.Join(configDir, configuredPath)
		}
		keyDocument, err := vexocrypto.LoadKeyDocument(keyPath)
		if err != nil {
			return fmt.Errorf("vrf key %q: %w", configuredPath, err)
		}
		privateKey, err := keyDocument.ECVRFP256PrivateKeyWithPassphrase(resolvePassphrase(""))
		if err != nil {
			return fmt.Errorf("vrf key %q: %w", configuredPath, err)
		}
		publicKey, err := decodeOptionalBase64(keyDocument.PublicKey)
		if err != nil {
			return fmt.Errorf("vrf key %q public key: %w", configuredPath, err)
		}
		keys[string(publicKey)] = privateKey
		keys[keyDocument.PublicKey] = privateKey
	}
	document.VRF.Keys = keys
	return nil
}

func cloneVRFKeys(keys map[string][]byte) map[string][]byte {
	copied := make(map[string][]byte, len(keys))
	for publicKey, privateKey := range keys {
		copied[publicKey] = append([]byte(nil), privateKey...)
	}
	return copied
}

func loadMempoolConfigForConfig(configPath string, document configDocument) (mempoolConfigDocument, error) {
	mempoolPath := resolveMempoolConfigPath(filepath.Dir(configPath), document.MempoolConfigPath)
	mempoolDocument, err := readMempoolConfigDocument(mempoolPath)
	if err == nil {
		return mempoolDocument, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if hasLegacyMempoolConfig(document.LegacyMempool) {
			return document.LegacyMempool, nil
		}
		return defaultMempoolConfigDocument(documentChainID(document), document.DataDir), nil
	}
	return mempoolConfigDocument{}, err
}

func loadLogConfigForConfig(configPath string, document configDocument) (logConfigDocument, error) {
	logPath := resolveLogConfigPath(filepath.Dir(configPath), document.LogConfigPath)
	logDocument, err := readLogConfigDocument(logPath)
	if err == nil {
		return logDocument, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if hasLegacyLogConfig(document.LegacyLog) {
			return document.LegacyLog, nil
		}
		return defaultLogConfigDocument(documentChainID(document), document.DataDir, document.ValidatorID), nil
	}
	return logConfigDocument{}, err
}

func documentChainID(document configDocument) string {
	if document.ChainID != "" {
		return document.ChainID
	}
	return document.Chain.ChainID
}

func hasLegacyModuleConfig(document moduleConfigDocument) bool {
	if document.SchemaVersion == "" {
		return false
	}
	if len(document.Application.Modules) > 0 {
		return true
	}
	if document.Execution != (config.ExecutionConfig{}) {
		return true
	}
	if document.Bank != (config.BankConfig{}) {
		return true
	}
	if document.Staking != (config.StakingConfig{}) {
		return true
	}
	return document.Governance != (governance.TallyPolicy{})
}

func hasLegacyNetworkConfig(document networkConfigDocument) bool {
	if document.SchemaVersion == "" {
		return false
	}
	return runtimeRPCConfigSet(document.RPC) ||
		runtimeP2PConfigSet(document.P2P) ||
		runtimeStateSyncConfigSet(document.StateSync) ||
		document.PeerScoring != (p2p.ScoreConfig{})
}

func runtimeRPCConfigSet(rpc runtimeRPCConfig) bool {
	return rpc.Enabled ||
		rpc.Address != "" ||
		rpc.AdminToken != "" ||
		len(rpc.AdminTokens) > 0 ||
		len(rpc.CORSAllowedOrigins) > 0 ||
		len(rpc.AllowedHosts) > 0 ||
		rpc.TLSCertPath != "" ||
		rpc.TLSKeyPath != "" ||
		rpc.TLSCAPath != "" ||
		rpc.TLSServerName != "" ||
		rpc.EnablePprof ||
		rpc.RequestTimeout != "" ||
		rpc.MaxRequestBytes != 0 ||
		rpc.RateLimitWindow != "" ||
		rpc.RateLimitMaxRequests != 0 ||
		rpc.Web3MaxSubscriptions != 0 ||
		rpc.Web3IdleTimeout != "" ||
		rpc.Web3FilterSnapshot != "" ||
		rpc.EVMManagedAccounts ||
		len(rpc.EVMAccountPrivateKeys) > 0 ||
		len(rpc.EVMAccountKeyEnvs) > 0
}

func hasLegacyConsensusConfig(document consensusConfigDocument) bool {
	if document.SchemaVersion == "" {
		return false
	}
	return document.Consensus != (runtimeConsensusConfig{}) ||
		document.Crypto != (config.CryptoConfig{}) ||
		document.VRF.Keys != nil ||
		len(document.VRFKeyPaths) > 0 ||
		validatorAdmissionConfigSet(document.Validator) ||
		committeeRotationPolicySet(document.Committee)
}

func hasLegacyMempoolConfig(document mempoolConfigDocument) bool {
	if document.SchemaVersion == "" {
		return false
	}
	return document.Mempool.Author != "" ||
		document.Mempool.MaxTxBytes != 0 ||
		document.Mempool.MaxTxs != 0 ||
		document.Mempool.MaxTotalBytes != 0 ||
		document.Mempool.MaxTxsPerSender != 0 ||
		document.Mempool.AllowDuplicate ||
		document.Mempool.SeenTTL != 0 ||
		document.Mempool.MinFee != 0 ||
		document.Mempool.EnablePriority ||
		document.Mempool.WALPath != "" ||
		document.Mempool.EnableReplacement ||
		document.Mempool.ReplacementBumpBPS != 0
}

func hasLegacyLogConfig(document logConfigDocument) bool {
	if document.SchemaVersion == "" {
		return false
	}
	return document.Log != (runtimeLogConfig{})
}

func runtimeP2PConfigSet(config runtimeP2PConfig) bool {
	return config.Enabled ||
		config.NodeID != "" ||
		config.NodeKeyPath != "" ||
		config.ListenAddress != "" ||
		config.DialTimeout != "" ||
		config.NetworkID != "" ||
		config.MaxMessageBytes != 0 ||
		config.MaxPeers != 0 ||
		config.AuthToken != "" ||
		config.AuthReplayPath != "" ||
		config.RequireAuthReplayStore ||
		config.TLSCertPath != "" ||
		config.TLSKeyPath != "" ||
		config.TLSCAPath != "" ||
		config.TLSServerName != "" ||
		config.AddrBookPath != "" ||
		config.AddrBookMaxFails != 0 ||
		len(config.Peers) > 0 ||
		len(config.Seeds) > 0
}

func runtimeStateSyncConfigSet(config runtimeStateSyncConfig) bool {
	return config.Enabled ||
		len(config.SnapshotURLs) > 0 ||
		config.Timeout != "" ||
		config.MinHeight != 0 ||
		config.RequireFresh ||
		config.TrustLocalHigher ||
		config.MaxSnapshotBytes != 0 ||
		config.RetryAllSnapshots ||
		config.TrustHeight != 0 ||
		config.TrustHash != ""
}

func validatorAdmissionConfigSet(config validator.AdmissionConfig) bool {
	return config.Permissionless ||
		config.MinStake != 0 ||
		config.MaxValidators != 0 ||
		len(config.Whitelist) > 0 ||
		config.RequirePublicKey
}

func committeeRotationPolicySet(policy committee.RotationPolicy) bool {
	return policy.EpochLength != 0 ||
		policy.CommitteeSize != 0 ||
		len(policy.VRFThreshold) > 0 ||
		policy.MinVotingPower != 0 ||
		policy.Backend != ""
}

func loadGenesis(path string) (node.Genesis, error) {
	document, err := readGenesisDocument(path)
	if err != nil {
		return node.Genesis{}, err
	}
	genesis := node.Genesis{
		ChainID:    document.ChainID,
		Validators: make([]validator.Validator, 0, len(document.Validators)),
		AppState:   make(map[string][]byte, len(document.AppState)),
		Governance: make(map[types.Address]types.VotingPower, len(document.Governance)),
	}
	for _, validatorInfo := range document.Validators {
		publicKey, err := decodeOptionalBase64(validatorInfo.PublicKey)
		if err != nil {
			return node.Genesis{}, fmt.Errorf("validator %q public key: %w", validatorInfo.ID, err)
		}
		if err := validateValidatorDocumentAddress(validatorInfo, publicKey); err != nil {
			return node.Genesis{}, fmt.Errorf("validator %q address: %w", validatorInfo.ID, err)
		}
		genesis.Validators = append(genesis.Validators, validator.Validator{
			ID:          types.ValidatorID(validatorInfo.ID),
			Address:     types.Address(validatorInfo.Address),
			PublicKey:   publicKey,
			VotingPower: types.VotingPower(validatorInfo.VotingPower),
			Stake:       validatorInfo.Stake,
			Metadata:    validatorInfo.Metadata,
		})
	}
	for key, encoded := range document.AppState {
		value, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return node.Genesis{}, fmt.Errorf("app state %q: %w", key, err)
		}
		genesis.AppState[key] = value
	}
	for address, power := range document.Governance {
		genesis.Governance[types.Address(address)] = types.VotingPower(power)
	}
	return genesis, nil
}

func validateValidatorDocumentAddress(validatorInfo validatorDocument, publicKey types.PublicKey) error {
	if validatorInfo.Address == "" || len(publicKey) == 0 {
		return nil
	}
	if !strings.HasPrefix(validatorInfo.Address, address.ValidatorOperatorHRP+"1") {
		return nil
	}
	return address.MatchesPublicKey(types.Address(validatorInfo.Address), address.ValidatorOperatorHRP, publicKey)
}

func resolveConfigPath(home string, path string) string {
	if path != "" {
		return path
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, configFileName)
}

func resolveGenesisPath(home string, path string) string {
	if path != "" {
		return path
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, genesisFileName)
}

func resolveModuleConfigPath(home string, path string) string {
	return resolveHomePath(home, path, moduleConfigFileName)
}

func resolveNetworkConfigPath(home string, path string) string {
	return resolveHomePath(home, path, networkConfigFileName)
}

func resolveConsensusConfigPath(home string, path string) string {
	return resolveHomePath(home, path, consensusConfigFileName)
}

func resolveMempoolConfigPath(home string, path string) string {
	return resolveHomePath(home, path, mempoolConfigFileName)
}

func resolveLogConfigPath(home string, path string) string {
	return resolveHomePath(home, path, logConfigFileName)
}

func resolveHomePath(home string, path string, fileName string) string {
	if path != "" {
		if filepath.IsAbs(path) {
			return path
		}
		if home == "" {
			home = defaultHomeDir
		}
		return filepath.Join(home, path)
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, fileName)
}

func writeJSONFile(path string, value any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func readJSONFile(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func decodeOptionalBase64(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

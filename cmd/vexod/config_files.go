package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	SchemaVersion        string                  `json:"schema_version"`
	RequireNetworkSafety bool                    `json:"require_network_safety,omitempty"`
	DataDir              string                  `json:"data_dir"`
	ValidatorID          string                  `json:"validator_id,omitempty"`
	ChainID              string                  `json:"chain_id"`
	ModuleConfigPath     string                  `json:"module_config_path,omitempty"`
	NetworkConfigPath    string                  `json:"network_config_path,omitempty"`
	ConsensusConfigPath  string                  `json:"consensus_config_path,omitempty"`
	MempoolConfigPath    string                  `json:"mempool_config_path,omitempty"`
	LogConfigPath        string                  `json:"log_config_path,omitempty"`
	Runtime              runtimeConfig           `json:"-"`
	Chain                chainConfigDocument     `json:"-"`
	LegacyModule         moduleConfigDocument    `json:"-"`
	LegacyNetwork        networkConfigDocument   `json:"-"`
	LegacyConsensus      consensusConfigDocument `json:"-"`
	LegacyMempool        mempoolConfigDocument   `json:"-"`
	LegacyLog            logConfigDocument       `json:"-"`
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
	SchemaVersion string           `json:"schema_version"`
	RPC           runtimeRPCConfig `json:"rpc"`
	P2P           runtimeP2PConfig `json:"p2p"`
	PeerScoring   p2p.ScoreConfig  `json:"peer_scoring"`
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
	Consensus runtimeConsensusConfig `json:"consensus,omitempty"`
	Log       runtimeLogConfig       `json:"log,omitempty"`
}

type runtimeRPCConfig struct {
	Enabled               bool                `json:"enabled"`
	Address               string              `json:"address,omitempty"`
	AdminToken            string              `json:"admin_token,omitempty"`
	AdminTokens           map[string][]string `json:"admin_tokens,omitempty"`
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
	EVMManagedAccounts    bool                `json:"evm_managed_accounts,omitempty"`
	EVMAccountPrivateKeys []string            `json:"evm_account_private_keys,omitempty"`
	EVMAccountKeyEnvs     []string            `json:"evm_account_key_envs,omitempty"`
}

type runtimeP2PConfig struct {
	Enabled          bool              `json:"enabled"`
	ListenAddress    string            `json:"listen_address,omitempty"`
	NetworkID        string            `json:"network_id,omitempty"`
	MaxMessageBytes  uint64            `json:"max_message_bytes,omitempty"`
	MaxPeers         int               `json:"max_peers,omitempty"`
	AuthToken        string            `json:"auth_token,omitempty"`
	TLSCertPath      string            `json:"tls_cert_path,omitempty"`
	TLSKeyPath       string            `json:"tls_key_path,omitempty"`
	TLSCAPath        string            `json:"tls_ca_path,omitempty"`
	TLSServerName    string            `json:"tls_server_name,omitempty"`
	AddrBookPath     string            `json:"addr_book_path,omitempty"`
	AddrBookMaxFails int               `json:"addr_book_max_failures,omitempty"`
	Peers            map[string]string `json:"peers,omitempty"`
	Seeds            map[string]string `json:"seeds,omitempty"`
}

type runtimeConsensusConfig struct {
	LoopEnabled       bool   `json:"loop_enabled"`
	Interval          string `json:"interval,omitempty"`
	TimeoutPropose    string `json:"timeout_propose,omitempty"`
	TimeoutPrevote    string `json:"timeout_prevote,omitempty"`
	TimeoutPrecommit  string `json:"timeout_precommit,omitempty"`
	TimeoutCommit     string `json:"timeout_commit,omitempty"`
	RoundTimeout      string `json:"round_timeout,omitempty"`
	MaxBlockBytes     int64  `json:"max_block_bytes,omitempty"`
	CreateEmptyBlocks bool   `json:"create_empty_blocks"`
	ExecutionCommit   string `json:"execution_commit,omitempty"`
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

func runInit(writer io.Writer, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "validator":
			return runInitValidator(writer, args[1:])
		case "archive":
			return runInitArchive(writer, args[1:])
		}
	}
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validatorID := flags.String("validator", defaultValidatorID, "local validator id")
	validatorCount := flags.Int("validators", 1, "number of local validators to initialize")
	keyType := flags.String("key-type", vexocrypto.KeyTypeEd25519, "validator key type: ed25519 or bls")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	p2pPortStep := flags.Int("p2p-port-step", 10, "P2P port increment per validator")
	rpcPortStep := flags.Int("rpc-port-step", 10, "RPC port increment per validator")
	networkConfigPath := flags.String("network-config", "", "network topology config file for generated validator addresses")
	encryptKeys := flags.Bool("encrypt-keys", false, "encrypt generated validator and VRF key files")
	passphrase := flags.String("passphrase", "", "key encryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	overwrite := flags.Bool("overwrite", false, "overwrite existing files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *validatorCount > 1 {
		options := networkAddressOptions{
			P2PBasePort: *p2pBasePort,
			RPCBasePort: *rpcBasePort,
			P2PPortStep: *p2pPortStep,
			RPCPortStep: *rpcPortStep,
		}
		if *networkConfigPath != "" {
			loadedOptions, err := readNetworkAddressOptions(*networkConfigPath)
			if err != nil {
				return err
			}
			options = loadedOptions
		}
		network, err := writeNetworkFilesWithOptionsAndKeyType(*home, *chainID, *validatorCount, *overwrite, options, *keyType, *encryptKeys, resolvePassphrase(*passphrase))
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "initialized vexo network\n")
		fmt.Fprintf(writer, "home: %s\n", network.Home)
		fmt.Fprintf(writer, "validators: %d\n", len(network.Nodes))
		for _, localNode := range network.Nodes {
			fmt.Fprintf(writer, "node: %s config=%s module_config=%s network_config=%s consensus_config=%s mempool_config=%s log_config=%s genesis=%s key=%s vrf_key=%s p2p=%s rpc=%s\n", localNode.ValidatorID, localNode.ConfigPath, localNode.ModuleConfigPath, localNode.NetworkConfigPath, localNode.ConsensusConfigPath, localNode.MempoolConfigPath, localNode.LogConfigPath, localNode.GenesisPath, localNode.KeyPath, localNode.VRFKeyPath, localNode.P2PAddress, localNode.RPCAddress)
		}
		return nil
	}
	configPath, genesisPath, keyPath, err := writeValidatorInitFilesWithKeyType(*home, *chainID, *validatorID, defaultP2PAddress, defaultRPCAddress, *overwrite, *keyType, *encryptKeys, resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "initialized vexo node\n")
	fmt.Fprintf(writer, "home: %s\n", *home)
	fmt.Fprintf(writer, "config: %s\n", configPath)
	fmt.Fprintf(writer, "module_config: %s\n", resolveModuleConfigPath(*home, ""))
	fmt.Fprintf(writer, "network_config: %s\n", resolveNetworkConfigPath(*home, ""))
	fmt.Fprintf(writer, "consensus_config: %s\n", resolveConsensusConfigPath(*home, ""))
	fmt.Fprintf(writer, "mempool_config: %s\n", resolveMempoolConfigPath(*home, ""))
	fmt.Fprintf(writer, "log_config: %s\n", resolveLogConfigPath(*home, ""))
	fmt.Fprintf(writer, "genesis: %s\n", genesisPath)
	fmt.Fprintf(writer, "key: %s\n", keyPath)
	return nil
}

func runInitValidator(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("init validator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "validator node home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validatorID := flags.String("validator", defaultValidatorID, "local validator id")
	keyType := flags.String("key-type", vexocrypto.KeyTypeEd25519, "validator key type: ed25519 or bls")
	encryptKeys := flags.Bool("encrypt-keys", false, "encrypt generated validator and VRF key files")
	passphrase := flags.String("passphrase", "", "key encryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	overwrite := flags.Bool("overwrite", false, "overwrite existing files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	configPath, genesisPath, keyPath, err := writeValidatorInitFilesWithKeyType(*home, *chainID, *validatorID, defaultP2PAddress, defaultRPCAddress, *overwrite, *keyType, *encryptKeys, resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "initialized vexo validator node\n")
	fmt.Fprintf(writer, "home: %s\n", *home)
	fmt.Fprintf(writer, "config: %s\n", configPath)
	fmt.Fprintf(writer, "module_config: %s\n", resolveModuleConfigPath(*home, ""))
	fmt.Fprintf(writer, "network_config: %s\n", resolveNetworkConfigPath(*home, ""))
	fmt.Fprintf(writer, "consensus_config: %s\n", resolveConsensusConfigPath(*home, ""))
	fmt.Fprintf(writer, "mempool_config: %s\n", resolveMempoolConfigPath(*home, ""))
	fmt.Fprintf(writer, "log_config: %s\n", resolveLogConfigPath(*home, ""))
	fmt.Fprintf(writer, "genesis: %s\n", genesisPath)
	fmt.Fprintf(writer, "key: %s\n", keyPath)
	return nil
}

func runInitArchive(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("init archive", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "archive node home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	bootstrapPeer := flags.String("bootstrap-peer", "", "optional bootstrap peer id=host:port stored in config")
	overwrite := flags.Bool("overwrite", false, "overwrite existing files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	configPath, genesisPath, err := writeArchiveInitFiles(*home, *chainID, defaultP2PAddress, defaultRPCAddress, *bootstrapPeer, *overwrite)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "initialized vexo archive node\n")
	fmt.Fprintf(writer, "home: %s\n", *home)
	fmt.Fprintf(writer, "config: %s\n", configPath)
	fmt.Fprintf(writer, "module_config: %s\n", resolveModuleConfigPath(*home, ""))
	fmt.Fprintf(writer, "network_config: %s\n", resolveNetworkConfigPath(*home, ""))
	fmt.Fprintf(writer, "consensus_config: %s\n", resolveConsensusConfigPath(*home, ""))
	fmt.Fprintf(writer, "mempool_config: %s\n", resolveMempoolConfigPath(*home, ""))
	fmt.Fprintf(writer, "log_config: %s\n", resolveLogConfigPath(*home, ""))
	fmt.Fprintf(writer, "genesis: %s\n", genesisPath)
	return nil
}

type networkDocument struct {
	Home  string
	Nodes []networkNodeDocument
}

type networkNodeDocument struct {
	ValidatorID         string
	Home                string
	ConfigPath          string
	ModuleConfigPath    string
	NetworkConfigPath   string
	ConsensusConfigPath string
	MempoolConfigPath   string
	LogConfigPath       string
	GenesisPath         string
	KeyPath             string
	VRFKeyPath          string
	P2PAddress          string
	RPCAddress          string
}

func writeNetworkFiles(home string, chainID string, validatorCount int, overwrite bool) (networkDocument, error) {
	return writeNetworkFilesWithPorts(home, chainID, validatorCount, overwrite, defaultP2PBasePort, defaultRPCBasePort)
}

func writeNetworkFilesWithPorts(home string, chainID string, validatorCount int, overwrite bool, p2pBasePort int, rpcBasePort int) (networkDocument, error) {
	return writeNetworkFilesWithOptions(home, chainID, validatorCount, overwrite, networkAddressOptions{
		P2PBasePort:     p2pBasePort,
		RPCBasePort:     rpcBasePort,
		P2PPortStep:     10,
		RPCPortStep:     10,
		P2PHostTemplate: "127.0.0.1",
		RPCHostTemplate: "127.0.0.1",
	})
}

type networkAddressOptions struct {
	P2PBasePort              int
	RPCBasePort              int
	P2PPortStep              int
	RPCPortStep              int
	P2PHostTemplate          string
	RPCHostTemplate          string
	P2PAdvertiseHostTemplate string
	RPCAdvertiseHostTemplate string
	P2PListenHost            string
	RPCListenHost            string
}

type networkAddressConfig struct {
	P2PBasePort              *int   `json:"p2p_base_port,omitempty"`
	RPCBasePort              *int   `json:"rpc_base_port,omitempty"`
	P2PPortStep              *int   `json:"p2p_port_step,omitempty"`
	RPCPortStep              *int   `json:"rpc_port_step,omitempty"`
	P2PHostTemplate          string `json:"p2p_host_template,omitempty"`
	RPCHostTemplate          string `json:"rpc_host_template,omitempty"`
	P2PAdvertiseHostTemplate string `json:"p2p_advertise_host_template,omitempty"`
	RPCAdvertiseHostTemplate string `json:"rpc_advertise_host_template,omitempty"`
	P2PListenHost            string `json:"p2p_listen_host,omitempty"`
	RPCListenHost            string `json:"rpc_listen_host,omitempty"`
}

func readNetworkAddressOptions(path string) (networkAddressOptions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return networkAddressOptions{}, err
	}
	var document networkAddressConfig
	if err := json.Unmarshal(data, &document); err != nil {
		return networkAddressOptions{}, err
	}
	options := networkAddressOptions{
		P2PHostTemplate:          document.P2PHostTemplate,
		RPCHostTemplate:          document.RPCHostTemplate,
		P2PAdvertiseHostTemplate: document.P2PAdvertiseHostTemplate,
		RPCAdvertiseHostTemplate: document.RPCAdvertiseHostTemplate,
		P2PListenHost:            document.P2PListenHost,
		RPCListenHost:            document.RPCListenHost,
	}
	if document.P2PBasePort != nil {
		options.P2PBasePort = *document.P2PBasePort
	}
	if document.RPCBasePort != nil {
		options.RPCBasePort = *document.RPCBasePort
	}
	if document.P2PPortStep != nil {
		options.P2PPortStep = *document.P2PPortStep
	} else {
		options.P2PPortStep = 10
	}
	if document.RPCPortStep != nil {
		options.RPCPortStep = *document.RPCPortStep
	} else {
		options.RPCPortStep = 10
	}
	return normalizeNetworkAddressOptions(options), nil
}

func writeNetworkFilesWithOptions(home string, chainID string, validatorCount int, overwrite bool, options networkAddressOptions) (networkDocument, error) {
	return writeNetworkFilesWithOptionsAndKeyType(home, chainID, validatorCount, overwrite, options, vexocrypto.KeyTypeEd25519, false, "")
}

func writeNetworkFilesWithOptionsAndKeyType(home string, chainID string, validatorCount int, overwrite bool, options networkAddressOptions, keyType string, encryptKeys bool, passphrase string) (networkDocument, error) {
	if home == "" {
		home = defaultHomeDir
	}
	if chainID == "" {
		chainID = defaultChainID
	}
	if validatorCount <= 0 {
		return networkDocument{}, fmt.Errorf("validators must be positive")
	}
	options = normalizeNetworkAddressOptions(options)
	if options.P2PBasePort <= 0 || options.RPCBasePort <= 0 {
		return networkDocument{}, fmt.Errorf("base ports must be positive")
	}
	if options.P2PPortStep < 0 || options.RPCPortStep < 0 {
		return networkDocument{}, fmt.Errorf("port steps must not be negative")
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return networkDocument{}, err
	}

	validators := make([]validatorDocument, 0, validatorCount)
	governance := make(map[string]uint64, validatorCount)
	appState := make(map[string]string, validatorCount)
	keys := make([]vexocrypto.KeyDocument, 0, validatorCount)
	vrfKeys := make([]vexocrypto.KeyDocument, 0, validatorCount)
	for index := 1; index <= validatorCount; index++ {
		validatorID := networkValidatorID(index)
		keyDocument, err := generateConsensusKeyDocument(keyType)
		if err != nil {
			return networkDocument{}, err
		}
		keyDocument, err = maybeEncryptKeyDocument(keyDocument, encryptKeys, passphrase)
		if err != nil {
			return networkDocument{}, err
		}
		keys = append(keys, keyDocument)
		vrfKeyDocument, err := vexocrypto.GenerateECVRFP256KeyDocument()
		if err != nil {
			return networkDocument{}, err
		}
		vrfKeyDocument, err = maybeEncryptKeyDocument(vrfKeyDocument, encryptKeys, passphrase)
		if err != nil {
			return networkDocument{}, err
		}
		vrfKeys = append(vrfKeys, vrfKeyDocument)
		publicKey, err := decodeOptionalBase64(keyDocument.PublicKey)
		if err != nil {
			return networkDocument{}, err
		}
		operatorAddress, err := address.ValidatorOperatorFromPublicKey(publicKey)
		if err != nil {
			return networkDocument{}, err
		}
		accountAddress, err := address.AccountFromPublicKey(publicKey)
		if err != nil {
			return networkDocument{}, err
		}
		consensusAddress, err := address.ValidatorConsensusFromPublicKey(publicKey)
		if err != nil {
			return networkDocument{}, err
		}
		metadata := map[string]string{
			"account_address":   string(accountAddress),
			"consensus_address": string(consensusAddress),
			"operator_address":  string(operatorAddress),
			"p2p_address":       networkP2PAdvertiseAddressWithOptions(index, options),
			"rpc_address":       networkRPCAdvertiseAddressWithOptions(index, options),
		}
		copyKeyDocumentValidatorMetadata(metadata, keyDocument)
		validators = append(validators, validatorDocument{
			ID:          validatorID,
			Address:     string(operatorAddress),
			PublicKey:   keyDocument.PublicKey,
			VotingPower: 1,
			Stake:       1,
			Metadata:    metadata,
		})
		governance[string(operatorAddress)] = 1
		appState["bank:"+string(accountAddress)] = base64.StdEncoding.EncodeToString([]byte("1000000000000000000000000"))
	}
	genesis := genesisDocument{
		SchemaVersion: genesisSchemaVersion,
		ChainID:       chainID,
		Validators:    validators,
		AppState:      appState,
		Governance:    governance,
	}

	network := networkDocument{Home: home, Nodes: make([]networkNodeDocument, 0, validatorCount)}
	for index := 1; index <= validatorCount; index++ {
		validatorID := networkValidatorID(index)
		nodeHome := filepath.Join(home, validatorID)
		dataDir := filepath.Join(nodeHome, "data")
		if overwrite {
			if err := os.RemoveAll(dataDir); err != nil {
				return networkDocument{}, err
			}
			for _, path := range []string{filepath.Join(nodeHome, networkPIDFileName), filepath.Join(nodeHome, "vexod.log")} {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return networkDocument{}, err
				}
			}
		}
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return networkDocument{}, err
		}
		configPath := filepath.Join(nodeHome, configFileName)
		moduleConfigPath := filepath.Join(nodeHome, moduleConfigFileName)
		networkConfigPath := filepath.Join(nodeHome, networkConfigFileName)
		consensusConfigPath := filepath.Join(nodeHome, consensusConfigFileName)
		mempoolConfigPath := filepath.Join(nodeHome, mempoolConfigFileName)
		logConfigPath := filepath.Join(nodeHome, logConfigFileName)
		genesisPath := filepath.Join(nodeHome, genesisFileName)
		keyPath := filepath.Join(nodeHome, keyFileName)
		vrfKeyPath := filepath.Join(nodeHome, defaultVRFKeyFileName)
		if !overwrite {
			for _, path := range []string{configPath, moduleConfigPath, networkConfigPath, consensusConfigPath, mempoolConfigPath, logConfigPath, genesisPath, keyPath, vrfKeyPath} {
				if _, err := os.Stat(path); err == nil {
					return networkDocument{}, fmt.Errorf("%s already exists", path)
				} else if !errors.Is(err, os.ErrNotExist) {
					return networkDocument{}, err
				}
			}
		} else if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return networkDocument{}, err
		} else if err := os.Remove(vrfKeyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return networkDocument{}, err
		}
		cfg := defaultConfigDocument(chainID, dataDir, validatorID)
		moduleCfg := defaultModuleConfigDocument(chainID)
		networkCfg := defaultNetworkConfigDocument(chainID, dataDir, validatorID)
		consensusCfg := defaultConsensusConfigDocument(chainID, dataDir, validatorID)
		applyConsensusCryptoForKeyType(&consensusCfg, keyType)
		consensusCfg.VRFKeyPaths = []string{defaultVRFKeyFileName}
		mempoolCfg := defaultMempoolConfigDocument(chainID, dataDir)
		logCfg := defaultLogConfigDocument(chainID, dataDir, validatorID)
		networkCfg.RPC.Address = networkRPCListenAddressWithOptions(index, options)
		networkCfg.P2P.ListenAddress = networkP2PListenAddressWithOptions(index, options)
		networkCfg.P2P.Peers = networkConfigPeers(validators, validatorID, options)
		if err := writeJSONFile(configPath, cfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(moduleConfigPath, moduleCfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(networkConfigPath, networkCfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(consensusConfigPath, consensusCfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(mempoolConfigPath, mempoolCfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(logConfigPath, logCfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(genesisPath, genesis); err != nil {
			return networkDocument{}, err
		}
		if err := vexocrypto.SaveKeyDocument(keyPath, keys[index-1]); err != nil {
			return networkDocument{}, err
		}
		if err := vexocrypto.SaveKeyDocument(vrfKeyPath, vrfKeys[index-1]); err != nil {
			return networkDocument{}, err
		}
		network.Nodes = append(network.Nodes, networkNodeDocument{
			ValidatorID:         validatorID,
			Home:                nodeHome,
			ConfigPath:          configPath,
			ModuleConfigPath:    moduleConfigPath,
			NetworkConfigPath:   networkConfigPath,
			ConsensusConfigPath: consensusConfigPath,
			MempoolConfigPath:   mempoolConfigPath,
			LogConfigPath:       logConfigPath,
			GenesisPath:         genesisPath,
			KeyPath:             keyPath,
			VRFKeyPath:          vrfKeyPath,
			P2PAddress:          networkP2PAdvertiseAddressWithOptions(index, options),
			RPCAddress:          networkRPCAdvertiseAddressWithOptions(index, options),
		})
	}
	return network, nil
}

func normalizeNetworkAddressOptions(options networkAddressOptions) networkAddressOptions {
	if options.P2PBasePort == 0 {
		options.P2PBasePort = defaultP2PBasePort
	}
	if options.RPCBasePort == 0 {
		options.RPCBasePort = defaultRPCBasePort
	}
	if options.P2PHostTemplate == "" {
		options.P2PHostTemplate = "127.0.0.1"
	}
	if options.RPCHostTemplate == "" {
		options.RPCHostTemplate = "127.0.0.1"
	}
	return options
}

func networkValidatorID(index int) string {
	return "validator-" + strconv.Itoa(index)
}

func networkP2PAddress(index int) string {
	return networkP2PAddressWithBasePort(index, defaultP2PBasePort)
}

func networkRPCAddress(index int) string {
	return networkRPCAddressWithBasePort(index, defaultRPCBasePort)
}

func networkP2PAddressWithBasePort(index int, basePort int) string {
	return networkAddress(index, "127.0.0.1", basePort, 10)
}

func networkRPCAddressWithBasePort(index int, basePort int) string {
	return networkAddress(index, "127.0.0.1", basePort, 10)
}

func networkP2PAddressWithOptions(index int, options networkAddressOptions) string {
	return networkAddress(index, options.P2PHostTemplate, options.P2PBasePort, options.P2PPortStep)
}

func networkRPCAddressWithOptions(index int, options networkAddressOptions) string {
	return networkAddress(index, options.RPCHostTemplate, options.RPCBasePort, options.RPCPortStep)
}

func networkP2PAdvertiseAddressWithOptions(index int, options networkAddressOptions) string {
	host := options.P2PAdvertiseHostTemplate
	if host == "" {
		host = options.P2PHostTemplate
	}
	return networkAddress(index, host, options.P2PBasePort, options.P2PPortStep)
}

func networkRPCAdvertiseAddressWithOptions(index int, options networkAddressOptions) string {
	host := options.RPCAdvertiseHostTemplate
	if host == "" {
		host = options.RPCHostTemplate
	}
	return networkAddress(index, host, options.RPCBasePort, options.RPCPortStep)
}

func networkP2PListenAddressWithOptions(index int, options networkAddressOptions) string {
	host := options.P2PListenHost
	if host == "" {
		host = options.P2PHostTemplate
	}
	return networkAddress(index, host, options.P2PBasePort, options.P2PPortStep)
}

func networkRPCListenAddressWithOptions(index int, options networkAddressOptions) string {
	host := options.RPCListenHost
	if host == "" {
		host = options.RPCHostTemplate
	}
	return networkAddress(index, host, options.RPCBasePort, options.RPCPortStep)
}

func networkAddress(index int, hostTemplate string, basePort int, portStep int) string {
	host := hostTemplate
	if strings.Contains(hostTemplate, "%d") {
		host = fmt.Sprintf(hostTemplate, index)
	}
	return host + ":" + strconv.Itoa(basePort+(index-1)*portStep)
}

func networkConfigPeers(validators []validatorDocument, self string, options networkAddressOptions) map[string]string {
	peers := make(map[string]string)
	for index, validatorInfo := range validators {
		if validatorInfo.ID == self {
			continue
		}
		address := networkP2PAddressWithOptions(index+1, options)
		if address != "" {
			peers[validatorInfo.ID] = address
		}
	}
	return peers
}

func runValidate(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	resolvedConfigPath := resolveConfigPath(*home, *configPath)
	resolvedGenesisPath := resolveGenesisPath(*home, *genesisPath)
	cfg, err := loadNodeConfig(resolvedConfigPath)
	if err != nil {
		return fmt.Errorf("%w: config: %v", errValidationFailed, err)
	}
	genesis, err := loadGenesis(resolvedGenesisPath)
	if err != nil {
		return fmt.Errorf("%w: genesis: %v", errValidationFailed, err)
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		return fmt.Errorf("%w: genesis: %v", errValidationFailed, err)
	}
	fmt.Fprintf(writer, "configuration valid\n")
	fmt.Fprintf(writer, "config: %s\n", resolvedConfigPath)
	fmt.Fprintf(writer, "genesis: %s\n", resolvedGenesisPath)
	return nil
}

func writeInitFiles(home string, chainID string, validatorID string, overwrite bool) (string, string, error) {
	if home == "" {
		home = defaultHomeDir
	}
	if chainID == "" {
		chainID = defaultChainID
	}
	if validatorID == "" {
		validatorID = defaultValidatorID
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", "", err
	}
	dataDir := filepath.Join(home, "data")
	if overwrite {
		if err := os.RemoveAll(dataDir); err != nil {
			return "", "", err
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", "", err
	}
	configPath := filepath.Join(home, configFileName)
	moduleConfigPath := filepath.Join(home, moduleConfigFileName)
	networkConfigPath := filepath.Join(home, networkConfigFileName)
	consensusConfigPath := filepath.Join(home, consensusConfigFileName)
	mempoolConfigPath := filepath.Join(home, mempoolConfigFileName)
	logConfigPath := filepath.Join(home, logConfigFileName)
	genesisPath := filepath.Join(home, genesisFileName)
	if !overwrite {
		for _, path := range []string{configPath, moduleConfigPath, networkConfigPath, consensusConfigPath, mempoolConfigPath, logConfigPath, genesisPath} {
			if _, err := os.Stat(path); err == nil {
				return "", "", fmt.Errorf("%s already exists", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", err
			}
		}
	}
	cfg := defaultConfigDocument(chainID, dataDir, validatorID)
	moduleCfg := defaultModuleConfigDocument(chainID)
	networkCfg := defaultNetworkConfigDocument(chainID, dataDir, validatorID)
	consensusCfg := defaultConsensusConfigDocument(chainID, dataDir, validatorID)
	applyConsensusCryptoForKeyType(&consensusCfg, vexocrypto.KeyTypeEd25519)
	mempoolCfg := defaultMempoolConfigDocument(chainID, dataDir)
	logCfg := defaultLogConfigDocument(chainID, dataDir, validatorID)
	genesis := defaultGenesisDocument(chainID, validatorID)
	if err := writeJSONFile(configPath, cfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(moduleConfigPath, moduleCfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(networkConfigPath, networkCfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(consensusConfigPath, consensusCfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(mempoolConfigPath, mempoolCfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(logConfigPath, logCfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(genesisPath, genesis); err != nil {
		return "", "", err
	}
	return configPath, genesisPath, nil
}

func writeValidatorInitFiles(home string, chainID string, validatorID string, p2pAddress string, rpcAddress string, overwrite bool) (string, string, string, error) {
	return writeValidatorInitFilesWithKeyType(home, chainID, validatorID, p2pAddress, rpcAddress, overwrite, vexocrypto.KeyTypeEd25519, false, "")
}

func writeValidatorInitFilesWithKeyType(home string, chainID string, validatorID string, p2pAddress string, rpcAddress string, overwrite bool, keyType string, encryptKeys bool, passphrase string) (string, string, string, error) {
	configPath, genesisPath, err := writeInitFiles(home, chainID, validatorID, overwrite)
	if err != nil {
		return "", "", "", err
	}
	keyPath := resolveKeyPath(home, "")
	if !overwrite {
		if _, err := os.Stat(keyPath); err == nil {
			return "", "", "", fmt.Errorf("%s already exists", keyPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", "", err
		}
	} else if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", "", err
	}
	keyDocument, err := generateConsensusKeyDocument(keyType)
	if err != nil {
		return "", "", "", err
	}
	if keyType == vexocrypto.KeyTypeBLS {
		consensusPath := resolveConsensusConfigPath(home, "")
		consensusDocument, err := readConsensusConfigDocument(consensusPath)
		if err != nil {
			return "", "", "", err
		}
		applyConsensusCryptoForKeyType(&consensusDocument, keyType)
		if err := writeJSONFile(consensusPath, consensusDocument); err != nil {
			return "", "", "", err
		}
	}
	keyDocument, err = maybeEncryptKeyDocument(keyDocument, encryptKeys, passphrase)
	if err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, keyDocument); err != nil {
		return "", "", "", err
	}
	vrfKeyPath := filepath.Join(home, defaultVRFKeyFileName)
	if !overwrite {
		if _, err := os.Stat(vrfKeyPath); err == nil {
			return "", "", "", fmt.Errorf("%s already exists", vrfKeyPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", "", err
		}
	} else if err := os.Remove(vrfKeyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", "", err
	}
	vrfKeyDocument, err := vexocrypto.GenerateECVRFP256KeyDocument()
	if err != nil {
		return "", "", "", err
	}
	vrfKeyDocument, err = maybeEncryptKeyDocument(vrfKeyDocument, encryptKeys, passphrase)
	if err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(vrfKeyPath, vrfKeyDocument); err != nil {
		return "", "", "", err
	}
	consensusPath := resolveConsensusConfigPath(home, "")
	consensusDocument, err := readConsensusConfigDocument(consensusPath)
	if err != nil {
		return "", "", "", err
	}
	consensusDocument.VRFKeyPaths = []string{defaultVRFKeyFileName}
	if err := writeJSONFile(consensusPath, consensusDocument); err != nil {
		return "", "", "", err
	}
	genesisDocument, err := readGenesisDocument(genesisPath)
	if err != nil {
		return "", "", "", err
	}
	if err := applyValidatorKeyToGenesisDocument(&genesisDocument, validatorID, keyDocument); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(genesisPath, genesisDocument); err != nil {
		return "", "", "", err
	}
	document, err := readConfigDocument(configPath)
	if err != nil {
		return "", "", "", err
	}
	networkDocument, err := readNetworkConfigDocument(resolveNetworkConfigPath(home, document.NetworkConfigPath))
	if err != nil {
		return "", "", "", err
	}
	networkDocument.RPC.Address = p2pOrDefault(rpcAddress, defaultRPCAddress)
	networkDocument.P2P.ListenAddress = p2pOrDefault(p2pAddress, defaultP2PAddress)
	if err := writeJSONFile(resolveNetworkConfigPath(home, document.NetworkConfigPath), networkDocument); err != nil {
		return "", "", "", err
	}
	return configPath, genesisPath, keyPath, nil
}

func generateConsensusKeyDocument(keyType string) (vexocrypto.KeyDocument, error) {
	switch keyType {
	case vexocrypto.KeyTypeEd25519, vexocrypto.KeyTypeBLS:
		return generateKeyDocument(keyType)
	default:
		return vexocrypto.KeyDocument{}, vexocrypto.ErrUnsupportedKeyType
	}
}

func readGenesisDocument(path string) (genesisDocument, error) {
	var document genesisDocument
	if err := readJSONFile(path, &document); err != nil {
		return genesisDocument{}, err
	}
	if document.SchemaVersion != genesisSchemaVersion {
		return genesisDocument{}, fmt.Errorf("unsupported genesis schema %q", document.SchemaVersion)
	}
	return document, nil
}

func applyValidatorKeyToGenesisDocument(document *genesisDocument, validatorID string, keyDocument vexocrypto.KeyDocument) error {
	publicKey, err := decodeOptionalBase64(keyDocument.PublicKey)
	if err != nil {
		return err
	}
	operatorAddress, err := address.ValidatorOperatorFromPublicKey(publicKey)
	if err != nil {
		return err
	}
	accountAddress, err := address.AccountFromPublicKey(publicKey)
	if err != nil {
		return err
	}
	consensusAddress, err := address.ValidatorConsensusFromPublicKey(publicKey)
	if err != nil {
		return err
	}
	for index := range document.Validators {
		if document.Validators[index].ID != validatorID {
			continue
		}
		document.Validators[index].Address = string(operatorAddress)
		document.Validators[index].PublicKey = keyDocument.PublicKey
		if document.Validators[index].Metadata == nil {
			document.Validators[index].Metadata = map[string]string{}
		}
		document.Validators[index].Metadata["account_address"] = string(accountAddress)
		document.Validators[index].Metadata["consensus_address"] = string(consensusAddress)
		document.Validators[index].Metadata["operator_address"] = string(operatorAddress)
		copyKeyDocumentValidatorMetadata(document.Validators[index].Metadata, keyDocument)
		if document.Governance == nil {
			document.Governance = map[string]uint64{}
		}
		power := document.Governance[validatorID]
		if power == 0 {
			power = document.Validators[index].VotingPower
		}
		delete(document.Governance, validatorID)
		document.Governance[string(operatorAddress)] = power
		return nil
	}
	return fmt.Errorf("validator %q not found in genesis", validatorID)
}

func copyKeyDocumentValidatorMetadata(metadata map[string]string, keyDocument vexocrypto.KeyDocument) {
	if keyDocument.Metadata.BLSProofOfPossession != "" {
		metadata[vexocrypto.BLSProofOfPossessionMetadataKey] = keyDocument.Metadata.BLSProofOfPossession
	}
	if keyDocument.Metadata.BLSAdapter != "" {
		metadata["bls_adapter"] = keyDocument.Metadata.BLSAdapter
	}
}

func writeArchiveInitFiles(home string, chainID string, p2pAddress string, rpcAddress string, bootstrapPeer string, overwrite bool) (string, string, error) {
	if home == "" {
		home = defaultHomeDir
	}
	if chainID == "" {
		chainID = defaultChainID
	}
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
		return "", "", err
	}
	configPath := filepath.Join(home, configFileName)
	moduleConfigPath := filepath.Join(home, moduleConfigFileName)
	networkConfigPath := filepath.Join(home, networkConfigFileName)
	consensusConfigPath := filepath.Join(home, consensusConfigFileName)
	mempoolConfigPath := filepath.Join(home, mempoolConfigFileName)
	logConfigPath := filepath.Join(home, logConfigFileName)
	genesisPath := filepath.Join(home, genesisFileName)
	if !overwrite {
		for _, path := range []string{configPath, moduleConfigPath, networkConfigPath, consensusConfigPath, mempoolConfigPath, logConfigPath, genesisPath} {
			if _, err := os.Stat(path); err == nil {
				return "", "", fmt.Errorf("%s already exists", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", err
			}
		}
	}
	document := defaultConfigDocument(chainID, filepath.Join(home, "data"), "")
	moduleDocument := defaultModuleConfigDocument(chainID)
	networkDocument := defaultNetworkConfigDocument(chainID, filepath.Join(home, "data"), "")
	consensusDocument := defaultConsensusConfigDocument(chainID, filepath.Join(home, "data"), "")
	mempoolDocument := defaultMempoolConfigDocument(chainID, filepath.Join(home, "data"))
	logDocument := defaultLogConfigDocument(chainID, filepath.Join(home, "data"), "")
	networkDocument.RPC.Address = p2pOrDefault(rpcAddress, defaultRPCAddress)
	networkDocument.P2P.ListenAddress = p2pOrDefault(p2pAddress, defaultP2PAddress)
	if bootstrapPeer != "" {
		peerID, address, err := parsePeerAssignment(bootstrapPeer)
		if err != nil {
			return "", "", err
		}
		networkDocument.P2P.Peers[string(peerID)] = address
	}
	if err := writeJSONFile(configPath, document); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(moduleConfigPath, moduleDocument); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(networkConfigPath, networkDocument); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(consensusConfigPath, consensusDocument); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(mempoolConfigPath, mempoolDocument); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(logConfigPath, logDocument); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(genesisPath, defaultGenesisDocument(chainID, defaultValidatorID)); err != nil {
		return "", "", err
	}
	return configPath, genesisPath, nil
}

func p2pOrDefault(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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
		document.PeerScoring != (p2p.ScoreConfig{})
}

func runtimeRPCConfigSet(rpc runtimeRPCConfig) bool {
	return rpc.Enabled ||
		rpc.Address != "" ||
		rpc.AdminToken != "" ||
		len(rpc.AdminTokens) > 0 ||
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
	return document.Mempool != (mempool.FIFOConfig{})
}

func hasLegacyLogConfig(document logConfigDocument) bool {
	if document.SchemaVersion == "" {
		return false
	}
	return document.Log != (runtimeLogConfig{})
}

func runtimeP2PConfigSet(config runtimeP2PConfig) bool {
	return config.Enabled ||
		config.ListenAddress != "" ||
		config.NetworkID != "" ||
		config.MaxMessageBytes != 0 ||
		config.MaxPeers != 0 ||
		config.AuthToken != "" ||
		config.TLSCertPath != "" ||
		config.TLSKeyPath != "" ||
		config.TLSCAPath != "" ||
		config.TLSServerName != "" ||
		config.AddrBookPath != "" ||
		config.AddrBookMaxFails != 0 ||
		len(config.Peers) > 0 ||
		len(config.Seeds) > 0
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

func maybeEncryptKeyDocument(document vexocrypto.KeyDocument, encrypt bool, passphrase string) (vexocrypto.KeyDocument, error) {
	if !encrypt {
		return document, nil
	}
	return document.Encrypted(passphrase)
}

func defaultConfigDocument(chainID string, dataDir string, validatorID string) configDocument {
	return configDocument{
		SchemaVersion:        configSchemaVersion,
		RequireNetworkSafety: true,
		DataDir:              dataDir,
		ValidatorID:          validatorID,
		ChainID:              chainID,
		ModuleConfigPath:     moduleConfigFileName,
		NetworkConfigPath:    networkConfigFileName,
		ConsensusConfigPath:  consensusConfigFileName,
		MempoolConfigPath:    mempoolConfigFileName,
		LogConfigPath:        logConfigFileName,
	}
}

func defaultModuleConfigDocument(chainID string) moduleConfigDocument {
	cfg := config.Default(chainID)
	applyDefaultNetworkSafetyModuleConfig(&cfg)
	return moduleConfigFromConfig(cfg)
}

func applyDefaultNetworkSafetyModuleConfig(cfg *config.Config) {
	cfg.Execution.RequireSigned = true
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinBaseFee = 1
	cfg.Execution.DynamicBaseFee = true
	cfg.Execution.MinGas = 1
	cfg.Execution.BlobBaseFee = 1
	cfg.Execution.MinBlobBaseFee = 1
	cfg.Execution.DynamicBlobBaseFee = true
	cfg.Execution.StrictEVMStateRoot = false
	cfg.Execution.AllowUnprotectedLegacyTx = false
	cfg.Bank.MintAuthority = "governance"
}

func defaultNetworkConfigDocument(chainID string, dataDir string, validatorID string) networkConfigDocument {
	cfg := config.Default(chainID)
	runtime := defaultRuntimeConfig(validatorID)
	return networkConfigDocument{
		SchemaVersion: networkSchemaVersion,
		RPC:           runtime.RPC,
		P2P:           runtime.P2P,
		PeerScoring:   cfg.P2P,
	}
}

func defaultConsensusConfigDocument(chainID string, dataDir string, validatorID string) consensusConfigDocument {
	cfg := config.Default(chainID)
	applyDefaultNetworkSafetyConsensusConfig(&cfg)
	runtime := defaultRuntimeConfig(validatorID)
	return consensusConfigDocument{
		SchemaVersion: consensusSchemaVersion,
		Consensus:     runtime.Consensus,
		Crypto:        cfg.Crypto,
		VRF:           cfg.VRF,
		Validator:     cfg.Validator,
		Committee:     cfg.Committee,
	}
}

func applyDefaultNetworkSafetyConsensusConfig(cfg *config.Config) {
	cfg.Crypto.Backend = config.CryptoBackendEd25519
	cfg.Crypto.ProductionAdapter = false
	cfg.Crypto.AdapterName = ""
	cfg.Crypto.AuditReport = ""
	cfg.Crypto.DependencyAudit = ""
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AdapterName = vexocrypto.VRFAdapterECVRFP256Name
	cfg.VRF.AuditReport = "built-in-ecvrf-p256-runtime"
	cfg.VRF.KeySource = "consensus_config.vrf_key_paths"
}

func applyConsensusCryptoForKeyType(document *consensusConfigDocument, keyType string) {
	if document == nil || keyType != vexocrypto.KeyTypeBLS {
		return
	}
	document.Crypto.Backend = config.CryptoBackendBLS
	document.Crypto.ProductionAdapter = true
	document.Crypto.AdapterName = vexocrypto.BLSAdapterBLSTName
	document.Crypto.AuditReport = "ncc-group-blst-security-assessment"
	document.Crypto.DependencyAudit = "github.com/supranational/blst@v0.3.16"
}

func defaultMempoolConfigDocument(chainID string, dataDir string) mempoolConfigDocument {
	cfg := config.Default(chainID)
	applyDefaultNetworkSafetyMempoolConfig(&cfg, dataDir)
	return mempoolConfigDocument{
		SchemaVersion: mempoolSchemaVersion,
		Mempool:       cfg.Mempool,
	}
}

func applyDefaultNetworkSafetyMempoolConfig(cfg *config.Config, dataDir string) {
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.EnableReplacement = true
	cfg.Mempool.ReplacementBumpBPS = 1000
	cfg.Mempool.SeenTTL = time.Hour
	if dataDir != "" {
		cfg.Mempool.WALPath = filepath.Join(dataDir, "mempool.wal")
	} else {
		cfg.Mempool.WALPath = "mempool.wal"
	}
}

func defaultLogConfigDocument(chainID string, dataDir string, validatorID string) logConfigDocument {
	runtime := defaultRuntimeConfig(validatorID)
	return logConfigDocument{
		SchemaVersion: logSchemaVersion,
		Log:           runtime.Log,
	}
}

func defaultRuntimeConfig(validatorID string) runtimeConfig {
	loopEnabled := validatorID != ""
	return runtimeConfig{
		RPC: runtimeRPCConfig{
			Enabled:              true,
			Address:              defaultRPCAddress,
			ShutdownTimeout:      "10s",
			Web3MaxSubscriptions: 256,
			Web3IdleTimeout:      "2m",
		},
		P2P: runtimeP2PConfig{
			Enabled:          true,
			ListenAddress:    defaultP2PAddress,
			AddrBookMaxFails: 3,
			Peers:            map[string]string{},
			Seeds:            map[string]string{},
		},
		Consensus: runtimeConsensusConfig{
			LoopEnabled:       loopEnabled,
			Interval:          "50ms",
			TimeoutPropose:    "500ms",
			TimeoutPrevote:    "250ms",
			TimeoutPrecommit:  "250ms",
			TimeoutCommit:     "100ms",
			RoundTimeout:      "1s",
			MaxBlockBytes:     1024 * 1024,
			CreateEmptyBlocks: false,
			ExecutionCommit:   string(node.ExecutionCommitModeFinalized),
		},
		Log: runtimeLogConfig{
			Format:       "text",
			Level:        "info",
			CommitEvents: boolPtr(true),
			PeerEvents:   boolPtr(true),
		},
	}
}

func chainConfigFromConfig(cfg config.Config) chainConfigDocument {
	return chainConfigDocument{
		ChainID:   cfg.ChainID,
		Crypto:    cfg.Crypto,
		VRF:       cfg.VRF,
		Validator: cfg.Validator,
		Committee: cfg.Committee,
		Mempool:   cfg.Mempool,
		P2P:       cfg.P2P,
	}
}

func configFromChainConfigDocument(document chainConfigDocument) config.Config {
	cfg := config.Default(document.ChainID)
	cfg.ChainID = document.ChainID
	cfg.Crypto = document.Crypto
	cfg.VRF = document.VRF
	cfg.Validator = document.Validator
	cfg.Committee = document.Committee
	cfg.Mempool = document.Mempool
	cfg.P2P = document.P2P
	return cfg
}

func moduleConfigFromConfig(cfg config.Config) moduleConfigDocument {
	return moduleConfigDocument{
		SchemaVersion: moduleSchemaVersion,
		Application:   cfg.Application,
		Execution:     cfg.Execution,
		Bank:          cfg.Bank,
		Staking:       cfg.Staking,
		Governance:    cfg.Governance,
	}
}

func networkConfigFromConfig(cfg config.Config, runtime runtimeConfig) networkConfigDocument {
	return networkConfigDocument{
		SchemaVersion: networkSchemaVersion,
		RPC:           runtime.RPC,
		P2P:           runtime.P2P,
		PeerScoring:   cfg.P2P,
	}
}

func consensusConfigFromConfig(cfg config.Config, runtime runtimeConfig) consensusConfigDocument {
	return consensusConfigDocument{
		SchemaVersion: consensusSchemaVersion,
		Consensus:     runtime.Consensus,
		Crypto:        cfg.Crypto,
		VRF:           cfg.VRF,
		Validator:     cfg.Validator,
		Committee:     cfg.Committee,
	}
}

func mempoolConfigFromConfig(cfg config.Config) mempoolConfigDocument {
	return mempoolConfigDocument{
		SchemaVersion: mempoolSchemaVersion,
		Mempool:       cfg.Mempool,
	}
}

func logConfigFromRuntime(runtime runtimeConfig) logConfigDocument {
	return logConfigDocument{
		SchemaVersion: logSchemaVersion,
		Log:           runtime.Log,
	}
}

func configFromConfigDocuments(document configDocument, moduleDocument moduleConfigDocument, networkDocument networkConfigDocument, consensusDocument consensusConfigDocument, mempoolDocument mempoolConfigDocument) config.Config {
	chainID := document.ChainID
	if chainID == "" {
		chainID = document.Chain.ChainID
	}
	cfg := config.Default(chainID)
	if moduleDocument.Application.Modules != nil {
		cfg.Application = moduleDocument.Application
	}
	cfg.Execution = moduleDocument.Execution
	cfg.Execution = normalizeExecutionConfig(cfg.Execution)
	cfg.Bank = moduleDocument.Bank
	if moduleDocument.Staking != (config.StakingConfig{}) {
		cfg.Staking = moduleDocument.Staking
	}
	if moduleDocument.Governance != (governance.TallyPolicy{}) {
		cfg.Governance = moduleDocument.Governance
	}
	cfg.P2P = networkDocument.PeerScoring
	cfg.Crypto = consensusDocument.Crypto
	cfg.VRF = consensusDocument.VRF
	cfg.Validator = consensusDocument.Validator
	cfg.Committee = consensusDocument.Committee
	cfg.Mempool = mempoolDocument.Mempool
	return cfg
}

func normalizeExecutionConfig(execution config.ExecutionConfig) config.ExecutionConfig {
	defaults := config.Default(defaultChainID).Execution
	if execution.EVMChainID == 0 {
		execution.EVMChainID = defaults.EVMChainID
	}
	if execution.BlobBaseFee == 0 {
		execution.BlobBaseFee = defaults.BlobBaseFee
	}
	if execution.TargetBlobGas == 0 {
		execution.TargetBlobGas = defaults.TargetBlobGas
	}
	if execution.MaxBlobGas == 0 {
		execution.MaxBlobGas = defaults.MaxBlobGas
	}
	if execution.BlobFeeChangeDenominator == 0 {
		execution.BlobFeeChangeDenominator = defaults.BlobFeeChangeDenominator
	}
	if execution.MinBlobBaseFee == 0 {
		execution.MinBlobBaseFee = defaults.MinBlobBaseFee
	}
	if execution.FeeCollector == "" {
		execution.FeeCollector = defaults.FeeCollector
	}
	if execution.FeeDenom == "" {
		execution.FeeDenom = defaults.FeeDenom
	}
	if execution.DisplayDenom == "" {
		execution.DisplayDenom = defaults.DisplayDenom
	}
	if execution.DisplayExponent == 0 {
		execution.DisplayExponent = defaults.DisplayExponent
	}
	if execution.GasDenom == "" {
		execution.GasDenom = defaults.GasDenom
	}
	if execution.EVMForkPreset == "" {
		execution.EVMForkPreset = defaults.EVMForkPreset
	}
	if execution.MaxBlobSidecarBlobs == 0 {
		execution.MaxBlobSidecarBlobs = defaults.MaxBlobSidecarBlobs
	}
	if execution.MaxBlobSidecarBytes == 0 {
		execution.MaxBlobSidecarBytes = defaults.MaxBlobSidecarBytes
	}
	return execution
}

func boolPtr(value bool) *bool {
	return &value
}

func defaultGenesisDocument(chainID string, validatorID string) genesisDocument {
	return genesisDocument{
		SchemaVersion: genesisSchemaVersion,
		ChainID:       chainID,
		Validators: []validatorDocument{
			{
				ID:          validatorID,
				Address:     validatorID,
				VotingPower: 1,
				Stake:       1,
			},
		},
		Governance: map[string]uint64{validatorID: 1},
	}
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

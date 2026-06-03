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

	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	defaultHomeDir       = ".vexo"
	defaultChainID       = "vexo-chain"
	defaultValidatorID   = "validator-1"
	defaultP2PBasePort   = 26656
	defaultRPCBasePort   = 26657
	configFileName       = "config.json"
	genesisFileName      = "genesis.json"
	configSchemaVersion  = "v1"
	genesisSchemaVersion = "v1"
)

var errValidationFailed = errors.New("validation failed")

type configDocument struct {
	SchemaVersion string        `json:"schema_version"`
	NodeMode      string        `json:"node_mode,omitempty"`
	DataDir       string        `json:"data_dir"`
	ValidatorID   string        `json:"validator_id,omitempty"`
	Runtime       runtimeConfig `json:"runtime,omitempty"`
	Chain         config.Config `json:"chain"`
}

type runtimeConfig struct {
	RPC       runtimeRPCConfig       `json:"rpc,omitempty"`
	P2P       runtimeP2PConfig       `json:"p2p,omitempty"`
	Consensus runtimeConsensusConfig `json:"consensus,omitempty"`
	Log       runtimeLogConfig       `json:"log,omitempty"`
}

type runtimeRPCConfig struct {
	Enabled              bool   `json:"enabled"`
	Address              string `json:"address,omitempty"`
	AdminToken           string `json:"admin_token,omitempty"`
	EnablePprof          bool   `json:"enable_pprof,omitempty"`
	RequestTimeout       string `json:"request_timeout,omitempty"`
	MaxRequestBytes      int64  `json:"max_request_bytes,omitempty"`
	RateLimitWindow      string `json:"rate_limit_window,omitempty"`
	RateLimitMaxRequests int    `json:"rate_limit_max_requests,omitempty"`
}

type runtimeP2PConfig struct {
	Enabled          bool              `json:"enabled"`
	ListenAddress    string            `json:"listen_address,omitempty"`
	NetworkID        string            `json:"network_id,omitempty"`
	MaxMessageBytes  uint64            `json:"max_message_bytes,omitempty"`
	MaxPeers         int               `json:"max_peers,omitempty"`
	AuthToken        string            `json:"auth_token,omitempty"`
	AddrBookPath     string            `json:"addr_book_path,omitempty"`
	AddrBookMaxFails int               `json:"addr_book_max_failures,omitempty"`
	Peers            map[string]string `json:"peers,omitempty"`
	Seeds            map[string]string `json:"seeds,omitempty"`
}

type runtimeConsensusConfig struct {
	LoopEnabled   bool   `json:"loop_enabled"`
	Interval      string `json:"interval,omitempty"`
	RoundTimeout  string `json:"round_timeout,omitempty"`
	MaxBlockBytes int64  `json:"max_block_bytes,omitempty"`
}

type runtimeLogConfig struct {
	Format string `json:"format,omitempty"`
	Level  string `json:"level,omitempty"`
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
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	p2pPortStep := flags.Int("p2p-port-step", 10, "P2P port increment per validator")
	rpcPortStep := flags.Int("rpc-port-step", 10, "RPC port increment per validator")
	networkConfigPath := flags.String("network-config", "", "network topology config file for generated validator addresses")
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
		network, err := writeNetworkFilesWithOptions(*home, *chainID, *validatorCount, *overwrite, options)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "initialized vexo network\n")
		fmt.Fprintf(writer, "home: %s\n", network.Home)
		fmt.Fprintf(writer, "validators: %d\n", len(network.Nodes))
		for _, localNode := range network.Nodes {
			fmt.Fprintf(writer, "node: %s config=%s genesis=%s key=%s p2p=%s rpc=%s\n", localNode.ValidatorID, localNode.ConfigPath, localNode.GenesisPath, localNode.KeyPath, localNode.P2PAddress, localNode.RPCAddress)
		}
		return nil
	}
	configPath, genesisPath, err := writeInitFiles(*home, *chainID, *validatorID, *overwrite)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "initialized vexo node\n")
	fmt.Fprintf(writer, "home: %s\n", *home)
	fmt.Fprintf(writer, "config: %s\n", configPath)
	fmt.Fprintf(writer, "genesis: %s\n", genesisPath)
	return nil
}

func runInitValidator(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("init validator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "validator node home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validatorID := flags.String("validator", defaultValidatorID, "local validator id")
	overwrite := flags.Bool("overwrite", false, "overwrite existing files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	configPath, genesisPath, keyPath, err := writeValidatorInitFiles(*home, *chainID, *validatorID, defaultP2PAddress, defaultRPCAddress, *overwrite)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "initialized vexo validator node\n")
	fmt.Fprintf(writer, "home: %s\n", *home)
	fmt.Fprintf(writer, "config: %s\n", configPath)
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
	fmt.Fprintf(writer, "genesis: %s\n", genesisPath)
	return nil
}

type networkDocument struct {
	Home  string
	Nodes []networkNodeDocument
}

type networkNodeDocument struct {
	ValidatorID string
	Home        string
	ConfigPath  string
	GenesisPath string
	KeyPath     string
	P2PAddress  string
	RPCAddress  string
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
	P2PBasePort     int
	RPCBasePort     int
	P2PPortStep     int
	RPCPortStep     int
	P2PHostTemplate string
	RPCHostTemplate string
	P2PListenHost   string
	RPCListenHost   string
}

type networkAddressConfig struct {
	P2PBasePort     *int   `json:"p2p_base_port,omitempty"`
	RPCBasePort     *int   `json:"rpc_base_port,omitempty"`
	P2PPortStep     *int   `json:"p2p_port_step,omitempty"`
	RPCPortStep     *int   `json:"rpc_port_step,omitempty"`
	P2PHostTemplate string `json:"p2p_host_template,omitempty"`
	RPCHostTemplate string `json:"rpc_host_template,omitempty"`
	P2PListenHost   string `json:"p2p_listen_host,omitempty"`
	RPCListenHost   string `json:"rpc_listen_host,omitempty"`
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
		P2PHostTemplate: document.P2PHostTemplate,
		RPCHostTemplate: document.RPCHostTemplate,
		P2PListenHost:   document.P2PListenHost,
		RPCListenHost:   document.RPCListenHost,
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
	keys := make([]vexocrypto.KeyDocument, 0, validatorCount)
	for index := 1; index <= validatorCount; index++ {
		validatorID := networkValidatorID(index)
		keyDocument, err := vexocrypto.GenerateEd25519KeyDocument()
		if err != nil {
			return networkDocument{}, err
		}
		keys = append(keys, keyDocument)
		validators = append(validators, validatorDocument{
			ID:          validatorID,
			Address:     validatorID,
			PublicKey:   keyDocument.PublicKey,
			VotingPower: 1,
			Stake:       1,
			Metadata: map[string]string{
				"p2p_address": networkP2PAddressWithOptions(index, options),
				"rpc_address": networkRPCAddressWithOptions(index, options),
			},
		})
		governance[validatorID] = 1
	}
	genesis := genesisDocument{
		SchemaVersion: genesisSchemaVersion,
		ChainID:       chainID,
		Validators:    validators,
		Governance:    governance,
	}

	network := networkDocument{Home: home, Nodes: make([]networkNodeDocument, 0, validatorCount)}
	for index := 1; index <= validatorCount; index++ {
		validatorID := networkValidatorID(index)
		nodeHome := filepath.Join(home, validatorID)
		dataDir := filepath.Join(nodeHome, "data")
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return networkDocument{}, err
		}
		configPath := filepath.Join(nodeHome, configFileName)
		genesisPath := filepath.Join(nodeHome, genesisFileName)
		keyPath := filepath.Join(nodeHome, keyFileName)
		if !overwrite {
			for _, path := range []string{configPath, genesisPath, keyPath} {
				if _, err := os.Stat(path); err == nil {
					return networkDocument{}, fmt.Errorf("%s already exists", path)
				} else if !errors.Is(err, os.ErrNotExist) {
					return networkDocument{}, err
				}
			}
		} else if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return networkDocument{}, err
		}
		cfg := defaultConfigDocument(chainID, dataDir, validatorID, "validator")
		cfg.Runtime.RPC.Address = networkRPCListenAddressWithOptions(index, options)
		cfg.Runtime.P2P.ListenAddress = networkP2PListenAddressWithOptions(index, options)
		cfg.Runtime.P2P.Peers = networkConfigPeers(validators, validatorID)
		if err := writeJSONFile(configPath, cfg); err != nil {
			return networkDocument{}, err
		}
		if err := writeJSONFile(genesisPath, genesis); err != nil {
			return networkDocument{}, err
		}
		if err := vexocrypto.SaveKeyDocument(keyPath, keys[index-1]); err != nil {
			return networkDocument{}, err
		}
		network.Nodes = append(network.Nodes, networkNodeDocument{
			ValidatorID: validatorID,
			Home:        nodeHome,
			ConfigPath:  configPath,
			GenesisPath: genesisPath,
			KeyPath:     keyPath,
			P2PAddress:  networkP2PAddressWithOptions(index, options),
			RPCAddress:  networkRPCAddressWithOptions(index, options),
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

func networkConfigPeers(validators []validatorDocument, self string) map[string]string {
	peers := make(map[string]string)
	for _, validatorInfo := range validators {
		if validatorInfo.ID == self {
			continue
		}
		address := validatorInfo.Metadata["p2p_address"]
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
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", "", err
	}
	configPath := filepath.Join(home, configFileName)
	genesisPath := filepath.Join(home, genesisFileName)
	if !overwrite {
		for _, path := range []string{configPath, genesisPath} {
			if _, err := os.Stat(path); err == nil {
				return "", "", fmt.Errorf("%s already exists", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", err
			}
		}
	}
	cfg := defaultConfigDocument(chainID, dataDir, validatorID, "validator")
	genesis := defaultGenesisDocument(chainID, validatorID)
	if err := writeJSONFile(configPath, cfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(genesisPath, genesis); err != nil {
		return "", "", err
	}
	return configPath, genesisPath, nil
}

func writeValidatorInitFiles(home string, chainID string, validatorID string, p2pAddress string, rpcAddress string, overwrite bool) (string, string, string, error) {
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
	keyDocument, err := vexocrypto.GenerateEd25519KeyDocument()
	if err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, keyDocument); err != nil {
		return "", "", "", err
	}
	document, err := readConfigDocument(configPath)
	if err != nil {
		return "", "", "", err
	}
	document.NodeMode = "validator"
	document.Runtime.RPC.Address = p2pOrDefault(rpcAddress, defaultRPCAddress)
	document.Runtime.P2P.ListenAddress = p2pOrDefault(p2pAddress, defaultP2PAddress)
	if err := writeJSONFile(configPath, document); err != nil {
		return "", "", "", err
	}
	return configPath, genesisPath, keyPath, nil
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
	genesisPath := filepath.Join(home, genesisFileName)
	if !overwrite {
		for _, path := range []string{configPath, genesisPath} {
			if _, err := os.Stat(path); err == nil {
				return "", "", fmt.Errorf("%s already exists", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", err
			}
		}
	}
	document := defaultConfigDocument(chainID, filepath.Join(home, "data"), "", "archive")
	document.Runtime.RPC.Address = p2pOrDefault(rpcAddress, defaultRPCAddress)
	document.Runtime.P2P.ListenAddress = p2pOrDefault(p2pAddress, defaultP2PAddress)
	if bootstrapPeer != "" {
		peerID, address, err := parsePeerAssignment(bootstrapPeer)
		if err != nil {
			return "", "", err
		}
		document.Runtime.P2P.Peers[string(peerID)] = address
	}
	if err := writeJSONFile(configPath, document); err != nil {
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
	var document configDocument
	if err := readJSONFile(path, &document); err != nil {
		return configDocument{}, err
	}
	if document.SchemaVersion != configSchemaVersion {
		return configDocument{}, fmt.Errorf("unsupported config schema %q", document.SchemaVersion)
	}
	return document, nil
}

func loadNodeConfig(path string) (node.Config, error) {
	document, err := readConfigDocument(path)
	if err != nil {
		return node.Config{}, err
	}
	cfg := node.Config{
		Chain:       document.Chain,
		DataDir:     document.DataDir,
		ValidatorID: types.ValidatorID(document.ValidatorID),
	}
	if err := cfg.Validate(); err != nil {
		return node.Config{}, err
	}
	return cfg, nil
}

func loadGenesis(path string) (node.Genesis, error) {
	var document genesisDocument
	if err := readJSONFile(path, &document); err != nil {
		return node.Genesis{}, err
	}
	if document.SchemaVersion != genesisSchemaVersion {
		return node.Genesis{}, fmt.Errorf("unsupported genesis schema %q", document.SchemaVersion)
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

func defaultConfigDocument(chainID string, dataDir string, validatorID string, modes ...string) configDocument {
	cfg := config.Default(chainID)
	loopEnabled := validatorID != ""
	mode := "validator"
	if len(modes) > 0 && modes[0] != "" {
		mode = modes[0]
	}
	return configDocument{
		SchemaVersion: configSchemaVersion,
		NodeMode:      mode,
		DataDir:       dataDir,
		ValidatorID:   validatorID,
		Runtime: runtimeConfig{
			RPC: runtimeRPCConfig{
				Enabled: true,
				Address: defaultRPCAddress,
			},
			P2P: runtimeP2PConfig{
				Enabled:          true,
				ListenAddress:    defaultP2PAddress,
				AddrBookMaxFails: 3,
				Peers:            map[string]string{},
				Seeds:            map[string]string{},
			},
			Consensus: runtimeConsensusConfig{
				LoopEnabled: loopEnabled,
			},
			Log: runtimeLogConfig{
				Format: "text",
				Level:  "info",
			},
		},
		Chain: cfg,
	}
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

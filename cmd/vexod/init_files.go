package main

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

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
	keyType := flags.String("key-type", vexocrypto.KeyTypeBLS, "validator key type: bls or ed25519")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first network P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first network RPC port")
	p2pPortStep := flags.Int("p2p-port-step", 10, "P2P port increment per validator")
	rpcPortStep := flags.Int("rpc-port-step", 10, "RPC port increment per validator")
	networkConfigPath := flags.String("network-config", "", "network topology config file for generated validator addresses")
	web3DevAccounts := flags.Bool("web3-dev-accounts", false, "generate prefunded Web3 managed accounts for local Remix deployment")
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
		network, err := writeNetworkFilesWithOptionsAndKeyType(*home, *chainID, *validatorCount, *overwrite, options, *keyType, *encryptKeys, resolvePassphrase(*passphrase), *web3DevAccounts)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "initialized vexo network\n")
		fmt.Fprintf(writer, "home: %s\n", network.Home)
		fmt.Fprintf(writer, "validators: %d\n", len(network.Nodes))
		for _, localNode := range network.Nodes {
			fmt.Fprintf(writer, "node: %s node_id=%s config=%s module_config=%s network_config=%s consensus_config=%s mempool_config=%s log_config=%s genesis=%s key=%s node_key=%s vrf_key=%s p2p=%s rpc=%s\n", localNode.ValidatorID, localNode.NodeID, localNode.ConfigPath, localNode.ModuleConfigPath, localNode.NetworkConfigPath, localNode.ConsensusConfigPath, localNode.MempoolConfigPath, localNode.LogConfigPath, localNode.GenesisPath, localNode.KeyPath, localNode.NodeKeyPath, localNode.VRFKeyPath, localNode.P2PAddress, localNode.RPCAddress)
		}
		return nil
	}
	configPath, genesisPath, keyPath, err := writeValidatorInitFilesWithKeyType(*home, *chainID, *validatorID, defaultP2PAddress, defaultRPCAddress, *overwrite, *keyType, *encryptKeys, resolvePassphrase(*passphrase), *web3DevAccounts)
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
	fmt.Fprintf(writer, "node_key: %s\n", filepath.Join(*home, nodeKeyFileName))
	return nil
}

func runInitValidator(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("init validator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "validator node home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validatorID := flags.String("validator", defaultValidatorID, "local validator id")
	keyType := flags.String("key-type", vexocrypto.KeyTypeBLS, "validator key type: bls or ed25519")
	web3DevAccounts := flags.Bool("web3-dev-accounts", false, "generate prefunded Web3 managed accounts for local Remix deployment")
	encryptKeys := flags.Bool("encrypt-keys", false, "encrypt generated validator and VRF key files")
	passphrase := flags.String("passphrase", "", "key encryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	overwrite := flags.Bool("overwrite", false, "overwrite existing files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	configPath, genesisPath, keyPath, err := writeValidatorInitFilesWithKeyType(*home, *chainID, *validatorID, defaultP2PAddress, defaultRPCAddress, *overwrite, *keyType, *encryptKeys, resolvePassphrase(*passphrase), *web3DevAccounts)
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
	fmt.Fprintf(writer, "node_key: %s\n", filepath.Join(*home, nodeKeyFileName))
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
	configPath, genesisPath, nodeKeyPath, err := writeArchiveInitFiles(*home, *chainID, defaultP2PAddress, defaultRPCAddress, *bootstrapPeer, *overwrite)
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
	fmt.Fprintf(writer, "node_key: %s\n", nodeKeyPath)
	return nil
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

func writeInitFiles(home string, chainID string, validatorID string, overwrite bool, web3DevAccounts bool) (string, string, error) {
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
	moduleCfg := defaultModuleConfigDocumentWithWeb3(chainID, web3DevAccounts)
	networkCfg := defaultNetworkConfigDocument(chainID, dataDir, validatorID)
	consensusCfg := defaultConsensusConfigDocument(chainID, dataDir, validatorID)
	applyConsensusCryptoForKeyType(&consensusCfg, vexocrypto.KeyTypeBLS)
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
	return writeValidatorInitFilesWithKeyType(home, chainID, validatorID, p2pAddress, rpcAddress, overwrite, vexocrypto.KeyTypeBLS, false, "", false)
}

func writeValidatorInitFilesWithKeyType(home string, chainID string, validatorID string, p2pAddress string, rpcAddress string, overwrite bool, keyType string, encryptKeys bool, passphrase string, web3DevAccounts bool) (string, string, string, error) {
	if home == "" {
		home = defaultHomeDir
	}
	keyPath := resolveKeyPath(home, "")
	nodeKeyPath := filepath.Join(home, nodeKeyFileName)
	vrfKeyPath := filepath.Join(home, defaultVRFKeyFileName)
	if !overwrite {
		for _, path := range []string{keyPath, nodeKeyPath, vrfKeyPath} {
			if _, err := os.Stat(path); err == nil {
				return "", "", "", fmt.Errorf("%s already exists", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", "", err
			}
		}
	} else {
		for _, path := range []string{keyPath, nodeKeyPath, vrfKeyPath} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", "", "", err
			}
		}
	}
	var managedEVMKey string
	var managedEVMAddress string
	keyDocument, err := generateConsensusKeyDocument(keyType)
	if err != nil {
		return "", "", "", err
	}
	keyDocument, err = maybeEncryptKeyDocument(keyDocument, encryptKeys, passphrase)
	if err != nil {
		return "", "", "", err
	}
	nodeKeyDocument, err := generateNodeKeyDocument(encryptKeys, passphrase)
	if err != nil {
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
	if web3DevAccounts {
		evmKey, err := gethcrypto.GenerateKey()
		if err != nil {
			return "", "", "", err
		}
		managedEVMKey = "0x" + hex.EncodeToString(gethcrypto.FromECDSA(evmKey))
		managedEVMAddress = strings.ToLower(gethcrypto.PubkeyToAddress(evmKey.PublicKey).Hex())
	}
	configPath, genesisPath, err := writeInitFiles(home, chainID, validatorID, overwrite, web3DevAccounts)
	if err != nil {
		return "", "", "", err
	}
	consensusPath := resolveConsensusConfigPath(home, "")
	consensusDocument, err := readConsensusConfigDocument(consensusPath)
	if err != nil {
		return "", "", "", err
	}
	applyConsensusCryptoForKeyType(&consensusDocument, keyType)
	if err := writeJSONFile(consensusPath, consensusDocument); err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, keyDocument); err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(nodeKeyPath, nodeKeyDocument); err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(vrfKeyPath, vrfKeyDocument); err != nil {
		return "", "", "", err
	}
	consensusDocument, err = readConsensusConfigDocument(consensusPath)
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
	if err := applyAccountKeyFundingToGenesisDocument(&genesisDocument, nodeKeyDocument); err != nil {
		return "", "", "", err
	}
	if web3DevAccounts {
		genesisDocument.AppState["bank:"+managedEVMAddress] = encodedGenesisAccountBalance()
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
	if web3DevAccounts {
		document.RequireNetworkSafety = false
	}
	networkDocument.RPC.Address = p2pOrDefault(rpcAddress, defaultRPCAddress)
	networkDocument.P2P.ListenAddress = p2pOrDefault(p2pAddress, defaultP2PAddress)
	networkDocument.P2P.NodeID = defaultP2PNodeID(validatorID, home)
	networkDocument.P2P.NodeKeyPath = nodeKeyFileName
	if web3DevAccounts {
		networkDocument.RPC.EVMManagedAccounts = true
		networkDocument.RPC.EVMAccountPrivateKeys = []string{managedEVMKey}
		networkDocument.RPC.EVMAccountKeyEnvs = nil
	}
	if err := writeJSONFile(configPath, document); err != nil {
		return "", "", "", err
	}
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

func generateNodeKeyDocument(encryptKey bool, passphrase string) (vexocrypto.KeyDocument, error) {
	document, err := vexocrypto.GenerateEd25519KeyDocument()
	if err != nil {
		return vexocrypto.KeyDocument{}, err
	}
	return maybeEncryptKeyDocument(document, encryptKey, passphrase)
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

func applyAccountKeyFundingToGenesisDocument(document *genesisDocument, keyDocument vexocrypto.KeyDocument) error {
	publicKey, err := decodeOptionalBase64(keyDocument.PublicKey)
	if err != nil {
		return err
	}
	accountAddress, err := address.AccountFromPublicKey(publicKey)
	if err != nil {
		return err
	}
	if document.AppState == nil {
		document.AppState = map[string]string{}
	}
	document.AppState["bank:"+string(accountAddress)] = encodedGenesisAccountBalance()
	return nil
}

func encodedGenesisAccountBalance() string {
	return base64.StdEncoding.EncodeToString([]byte("1000000000000000000000000"))
}

func copyKeyDocumentValidatorMetadata(metadata map[string]string, keyDocument vexocrypto.KeyDocument) {
	if keyDocument.Metadata.BLSProofOfPossession != "" {
		metadata[vexocrypto.BLSProofOfPossessionMetadataKey] = keyDocument.Metadata.BLSProofOfPossession
	}
	if keyDocument.Metadata.BLSAdapter != "" {
		metadata["bls_adapter"] = keyDocument.Metadata.BLSAdapter
	}
}

func writeArchiveInitFiles(home string, chainID string, p2pAddress string, rpcAddress string, bootstrapPeer string, overwrite bool) (string, string, string, error) {
	if home == "" {
		home = defaultHomeDir
	}
	if chainID == "" {
		chainID = defaultChainID
	}
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
		return "", "", "", err
	}
	configPath := filepath.Join(home, configFileName)
	moduleConfigPath := filepath.Join(home, moduleConfigFileName)
	networkConfigPath := filepath.Join(home, networkConfigFileName)
	consensusConfigPath := filepath.Join(home, consensusConfigFileName)
	mempoolConfigPath := filepath.Join(home, mempoolConfigFileName)
	logConfigPath := filepath.Join(home, logConfigFileName)
	genesisPath := filepath.Join(home, genesisFileName)
	nodeKeyPath := filepath.Join(home, nodeKeyFileName)
	if !overwrite {
		for _, path := range []string{configPath, moduleConfigPath, networkConfigPath, consensusConfigPath, mempoolConfigPath, logConfigPath, genesisPath, nodeKeyPath} {
			if _, err := os.Stat(path); err == nil {
				return "", "", "", fmt.Errorf("%s already exists", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", "", "", err
			}
		}
	} else if err := os.Remove(nodeKeyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", "", err
	}
	document := defaultConfigDocument(chainID, filepath.Join(home, "data"), "")
	moduleDocument := defaultModuleConfigDocumentWithWeb3(chainID, false)
	networkDocument := defaultNetworkConfigDocument(chainID, filepath.Join(home, "data"), "")
	consensusDocument := defaultConsensusConfigDocument(chainID, filepath.Join(home, "data"), "")
	mempoolDocument := defaultMempoolConfigDocument(chainID, filepath.Join(home, "data"))
	logDocument := defaultLogConfigDocument(chainID, filepath.Join(home, "data"), "")
	networkDocument.RPC.Address = p2pOrDefault(rpcAddress, defaultRPCAddress)
	networkDocument.P2P.ListenAddress = p2pOrDefault(p2pAddress, defaultP2PAddress)
	networkDocument.P2P.NodeID = defaultP2PNodeID("", home)
	networkDocument.P2P.NodeKeyPath = nodeKeyFileName
	if bootstrapPeer != "" {
		peerID, address, err := parsePeerAssignment(bootstrapPeer)
		if err != nil {
			return "", "", "", err
		}
		networkDocument.P2P.Peers[string(peerID)] = address
	}
	if err := writeJSONFile(configPath, document); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(moduleConfigPath, moduleDocument); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(networkConfigPath, networkDocument); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(consensusConfigPath, consensusDocument); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(mempoolConfigPath, mempoolDocument); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(logConfigPath, logDocument); err != nil {
		return "", "", "", err
	}
	if err := writeJSONFile(genesisPath, defaultGenesisDocument(chainID, defaultValidatorID)); err != nil {
		return "", "", "", err
	}
	nodeKeyDocument, err := generateNodeKeyDocument(false, "")
	if err != nil {
		return "", "", "", err
	}
	if err := vexocrypto.SaveKeyDocument(nodeKeyPath, nodeKeyDocument); err != nil {
		return "", "", "", err
	}
	return configPath, genesisPath, nodeKeyPath, nil
}

func p2pOrDefault(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

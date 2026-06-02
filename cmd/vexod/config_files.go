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

	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	defaultHomeDir       = ".vexo"
	defaultChainID       = "vexo-local"
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
	DataDir       string        `json:"data_dir"`
	ValidatorID   string        `json:"validator_id,omitempty"`
	Chain         config.Config `json:"chain"`
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
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	chainID := flags.String("chain-id", defaultChainID, "chain id")
	validatorID := flags.String("validator", defaultValidatorID, "local validator id")
	validatorCount := flags.Int("validators", 1, "number of local validators to initialize")
	p2pBasePort := flags.Int("p2p-base-port", defaultP2PBasePort, "first localnet P2P port")
	rpcBasePort := flags.Int("rpc-base-port", defaultRPCBasePort, "first localnet RPC port")
	overwrite := flags.Bool("overwrite", false, "overwrite existing files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *validatorCount > 1 {
		localnet, err := writeLocalnetFilesWithPorts(*home, *chainID, *validatorCount, *overwrite, *p2pBasePort, *rpcBasePort)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "initialized vexo localnet\n")
		fmt.Fprintf(writer, "home: %s\n", localnet.Home)
		fmt.Fprintf(writer, "validators: %d\n", len(localnet.Nodes))
		for _, localNode := range localnet.Nodes {
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

type localnetDocument struct {
	Home  string
	Nodes []localnetNodeDocument
}

type localnetNodeDocument struct {
	ValidatorID string
	Home        string
	ConfigPath  string
	GenesisPath string
	KeyPath     string
	P2PAddress  string
	RPCAddress  string
}

func writeLocalnetFiles(home string, chainID string, validatorCount int, overwrite bool) (localnetDocument, error) {
	return writeLocalnetFilesWithPorts(home, chainID, validatorCount, overwrite, defaultP2PBasePort, defaultRPCBasePort)
}

func writeLocalnetFilesWithPorts(home string, chainID string, validatorCount int, overwrite bool, p2pBasePort int, rpcBasePort int) (localnetDocument, error) {
	if home == "" {
		home = defaultHomeDir
	}
	if chainID == "" {
		chainID = defaultChainID
	}
	if validatorCount <= 0 {
		return localnetDocument{}, fmt.Errorf("validators must be positive")
	}
	if p2pBasePort <= 0 || rpcBasePort <= 0 {
		return localnetDocument{}, fmt.Errorf("base ports must be positive")
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return localnetDocument{}, err
	}

	validators := make([]validatorDocument, 0, validatorCount)
	governance := make(map[string]uint64, validatorCount)
	keys := make([]vexocrypto.KeyDocument, 0, validatorCount)
	for index := 1; index <= validatorCount; index++ {
		validatorID := localnetValidatorID(index)
		keyDocument, err := vexocrypto.GenerateEd25519KeyDocument()
		if err != nil {
			return localnetDocument{}, err
		}
		keys = append(keys, keyDocument)
		validators = append(validators, validatorDocument{
			ID:          validatorID,
			Address:     validatorID,
			PublicKey:   keyDocument.PublicKey,
			VotingPower: 1,
			Stake:       1,
			Metadata: map[string]string{
				"p2p_address": localnetP2PAddressWithBasePort(index, p2pBasePort),
				"rpc_address": localnetRPCAddressWithBasePort(index, rpcBasePort),
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

	localnet := localnetDocument{Home: home, Nodes: make([]localnetNodeDocument, 0, validatorCount)}
	for index := 1; index <= validatorCount; index++ {
		validatorID := localnetValidatorID(index)
		nodeHome := filepath.Join(home, validatorID)
		dataDir := filepath.Join(nodeHome, "data")
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return localnetDocument{}, err
		}
		configPath := filepath.Join(nodeHome, configFileName)
		genesisPath := filepath.Join(nodeHome, genesisFileName)
		keyPath := filepath.Join(nodeHome, keyFileName)
		if !overwrite {
			for _, path := range []string{configPath, genesisPath, keyPath} {
				if _, err := os.Stat(path); err == nil {
					return localnetDocument{}, fmt.Errorf("%s already exists", path)
				} else if !errors.Is(err, os.ErrNotExist) {
					return localnetDocument{}, err
				}
			}
		} else if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return localnetDocument{}, err
		}
		cfg := defaultConfigDocument(chainID, dataDir, validatorID)
		if err := writeJSONFile(configPath, cfg); err != nil {
			return localnetDocument{}, err
		}
		if err := writeJSONFile(genesisPath, genesis); err != nil {
			return localnetDocument{}, err
		}
		if err := vexocrypto.SaveKeyDocument(keyPath, keys[index-1]); err != nil {
			return localnetDocument{}, err
		}
		localnet.Nodes = append(localnet.Nodes, localnetNodeDocument{
			ValidatorID: validatorID,
			Home:        nodeHome,
			ConfigPath:  configPath,
			GenesisPath: genesisPath,
			KeyPath:     keyPath,
			P2PAddress:  localnetP2PAddressWithBasePort(index, p2pBasePort),
			RPCAddress:  localnetRPCAddressWithBasePort(index, rpcBasePort),
		})
	}
	return localnet, nil
}

func localnetValidatorID(index int) string {
	return "validator-" + strconv.Itoa(index)
}

func localnetP2PAddress(index int) string {
	return localnetP2PAddressWithBasePort(index, defaultP2PBasePort)
}

func localnetRPCAddress(index int) string {
	return localnetRPCAddressWithBasePort(index, defaultRPCBasePort)
}

func localnetP2PAddressWithBasePort(index int, basePort int) string {
	return "127.0.0.1:" + strconv.Itoa(basePort+(index-1)*10)
}

func localnetRPCAddressWithBasePort(index int, basePort int) string {
	return "127.0.0.1:" + strconv.Itoa(basePort+(index-1)*10)
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
	cfg := defaultConfigDocument(chainID, dataDir, validatorID)
	genesis := defaultGenesisDocument(chainID, validatorID)
	if err := writeJSONFile(configPath, cfg); err != nil {
		return "", "", err
	}
	if err := writeJSONFile(genesisPath, genesis); err != nil {
		return "", "", err
	}
	return configPath, genesisPath, nil
}

func loadNodeConfig(path string) (node.Config, error) {
	var document configDocument
	if err := readJSONFile(path, &document); err != nil {
		return node.Config{}, err
	}
	if document.SchemaVersion != configSchemaVersion {
		return node.Config{}, fmt.Errorf("unsupported config schema %q", document.SchemaVersion)
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

func defaultConfigDocument(chainID string, dataDir string, validatorID string) configDocument {
	return configDocument{
		SchemaVersion: configSchemaVersion,
		DataDir:       dataDir,
		ValidatorID:   validatorID,
		Chain:         config.Default(chainID),
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

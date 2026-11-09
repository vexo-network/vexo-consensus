package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

type networkDocument struct {
	Home  string
	Nodes []networkNodeDocument
}

type networkNodeDocument struct {
	ValidatorID         string
	NodeID              string
	Home                string
	ConfigPath          string
	ModuleConfigPath    string
	NetworkConfigPath   string
	ConsensusConfigPath string
	MempoolConfigPath   string
	LogConfigPath       string
	GenesisPath         string
	KeyPath             string
	NodeKeyPath         string
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
	return writeNetworkFilesWithOptionsAndKeyType(home, chainID, validatorCount, overwrite, options, vexocrypto.KeyTypeBLS, false, "", false)
}

func writeNetworkFilesWithOptionsAndKeyType(home string, chainID string, validatorCount int, overwrite bool, options networkAddressOptions, keyType string, encryptKeys bool, passphrase string, web3DevAccounts bool) (networkDocument, error) {
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
	nodeKeys := make([]vexocrypto.KeyDocument, 0, validatorCount)
	vrfKeys := make([]vexocrypto.KeyDocument, 0, validatorCount)
	managedEVMKeys := make([]string, 0, validatorCount)
	for index := 1; index <= validatorCount; index++ {
		validatorID := networkValidatorID(index)
		nodeID := defaultP2PNodeID(validatorID, filepath.Join(home, validatorID))
		keyDocument, err := generateConsensusKeyDocument(keyType)
		if err != nil {
			return networkDocument{}, err
		}
		keyDocument, err = maybeEncryptKeyDocument(keyDocument, encryptKeys, passphrase)
		if err != nil {
			return networkDocument{}, err
		}
		keys = append(keys, keyDocument)
		nodeKeyDocument, err := generateNodeKeyDocument(encryptKeys, passphrase)
		if err != nil {
			return networkDocument{}, err
		}
		nodeKeys = append(nodeKeys, nodeKeyDocument)
		if web3DevAccounts {
			evmKey, err := gethcrypto.GenerateKey()
			if err != nil {
				return networkDocument{}, err
			}
			managedEVMKeys = append(managedEVMKeys, "0x"+hex.EncodeToString(gethcrypto.FromECDSA(evmKey)))
			evmAddress := strings.ToLower(gethcrypto.PubkeyToAddress(evmKey.PublicKey).Hex())
			appState["bank:"+evmAddress] = encodedGenesisAccountBalance()
		}
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
		nodePublicKey, err := decodeOptionalBase64(nodeKeyDocument.PublicKey)
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
		nodeAccountAddress, err := address.AccountFromPublicKey(nodePublicKey)
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
			"node_id":           nodeID,
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
		appState["bank:"+string(accountAddress)] = encodedGenesisAccountBalance()
		appState["bank:"+string(nodeAccountAddress)] = encodedGenesisAccountBalance()
	}
	genesis := genesisDocument{
		SchemaVersion: genesisSchemaVersion,
		ChainID:       chainID,
		Validators:    validators,
		AppState:      appState,
		Governance:    governance,
	}

	network := networkDocument{Home: home, Nodes: make([]networkNodeDocument, 0, validatorCount)}
	sharedP2PAuthToken, err := generateNetworkAuthToken()
	if err != nil {
		return networkDocument{}, err
	}
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
		nodeKeyPath := filepath.Join(nodeHome, nodeKeyFileName)
		vrfKeyPath := filepath.Join(nodeHome, defaultVRFKeyFileName)
		if !overwrite {
			for _, path := range []string{configPath, moduleConfigPath, networkConfigPath, consensusConfigPath, mempoolConfigPath, logConfigPath, genesisPath, keyPath, nodeKeyPath, vrfKeyPath} {
				if _, err := os.Stat(path); err == nil {
					return networkDocument{}, fmt.Errorf("%s already exists", path)
				} else if !errors.Is(err, os.ErrNotExist) {
					return networkDocument{}, err
				}
			}
		} else if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return networkDocument{}, err
		} else if err := os.Remove(nodeKeyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return networkDocument{}, err
		} else if err := os.Remove(vrfKeyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return networkDocument{}, err
		}
		cfg := defaultConfigDocument(chainID, dataDir, validatorID)
		if web3DevAccounts {
			cfg.RequireNetworkSafety = false
		}
		moduleCfg := defaultModuleConfigDocumentWithWeb3(chainID, web3DevAccounts)
		networkCfg := defaultNetworkConfigDocument(chainID, dataDir, validatorID)
		networkCfg.P2P.NodeID = defaultP2PNodeID(validatorID, nodeHome)
		networkCfg.P2P.NodeKeyPath = nodeKeyFileName
		if web3DevAccounts {
			networkCfg.RPC.EVMManagedAccounts = true
			networkCfg.RPC.EVMAccountPrivateKeys = []string{managedEVMKeys[index-1]}
			networkCfg.RPC.EVMAccountKeyEnvs = nil
		}
		consensusCfg := defaultConsensusConfigDocument(chainID, dataDir, validatorID)
		applyConsensusCryptoForKeyType(&consensusCfg, keyType)
		consensusCfg.VRFKeyPaths = []string{defaultVRFKeyFileName}
		mempoolCfg := defaultMempoolConfigDocument(chainID, dataDir)
		logCfg := defaultLogConfigDocument(chainID, dataDir, validatorID)
		networkCfg.P2P.AuthToken = sharedP2PAuthToken
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
		if err := vexocrypto.SaveKeyDocument(nodeKeyPath, nodeKeys[index-1]); err != nil {
			return networkDocument{}, err
		}
		if err := vexocrypto.SaveKeyDocument(vrfKeyPath, vrfKeys[index-1]); err != nil {
			return networkDocument{}, err
		}
		network.Nodes = append(network.Nodes, networkNodeDocument{
			ValidatorID:         validatorID,
			NodeID:              networkCfg.P2P.NodeID,
			Home:                nodeHome,
			ConfigPath:          configPath,
			ModuleConfigPath:    moduleConfigPath,
			NetworkConfigPath:   networkConfigPath,
			ConsensusConfigPath: consensusConfigPath,
			MempoolConfigPath:   mempoolConfigPath,
			LogConfigPath:       logConfigPath,
			GenesisPath:         genesisPath,
			KeyPath:             keyPath,
			NodeKeyPath:         nodeKeyPath,
			VRFKeyPath:          vrfKeyPath,
			P2PAddress:          networkP2PAdvertiseAddressWithOptions(index, options),
			RPCAddress:          networkRPCAdvertiseAddressWithOptions(index, options),
		})
	}
	return network, nil
}

func generateNetworkAuthToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func generateNetworkTLSCA(commonName string) ([]byte, *x509.Certificate, *ecdsa.PrivateKey, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	caSerial, err := randomSerialNumber()
	if err != nil {
		return nil, nil, nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	return caCertPEM, caTemplate, caKey, nil
}

func generateNetworkTLSMaterial(commonName string, sans networkTLSSubjectAltNames, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) ([]byte, []byte, []byte, error) {
	if caCert == nil || caKey == nil {
		return nil, nil, nil, errors.New("shared network ca is required")
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	leafSerial, err := randomSerialNumber()
	if err != nil {
		return nil, nil, nil, err
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: leafSerial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(3650 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    sans.dnsNames(commonName),
		IPAddresses: sans.ipAddresses(),
	}
	leafCertDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	leafCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafCertDER})
	leafKeyPEM, err := encodeECPrivateKeyPEM(leafKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return leafCertPEM, leafKeyPEM, nil, nil
}

type networkTLSArtifacts struct {
	CAPath   string
	CertPath string
	KeyPath  string
}

func newNetworkTLSArtifacts(validatorID string, index int, options networkAddressOptions) (networkTLSArtifacts, error) {
	_ = validatorID
	_ = index
	_ = options
	return networkTLSArtifacts{
		CAPath:   filepath.Join("tls", "ca.crt"),
		CertPath: filepath.Join("tls", "node.crt"),
		KeyPath:  filepath.Join("tls", "node.key"),
	}, nil
}

func writeNetworkTLSArtifacts(nodeHome string, artifact networkTLSArtifacts, index int, options networkAddressOptions, sharedCA []byte, sharedCACert *x509.Certificate, sharedCAKey *ecdsa.PrivateKey) error {
	tlsDir := filepath.Join(nodeHome, "tls")
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return err
	}
	sans := buildNetworkTLSSubjectAltNames(index, options)
	leafCertPEM, leafKeyPEM, _, err := generateNetworkTLSMaterial(filepath.Base(nodeHome), sans, sharedCACert, sharedCAKey)
	if err != nil {
		return err
	}
	files := map[string][]byte{
		artifact.CAPath:   sharedCA,
		artifact.CertPath: leafCertPEM,
		artifact.KeyPath:  leafKeyPEM,
	}
	for relPath, data := range files {
		path := filepath.Join(nodeHome, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".key") {
			mode = 0o600
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			return err
		}
	}
	return nil
}

type networkTLSSubjectAltNames struct {
	dns []string
	ip  []net.IP
}

func (s networkTLSSubjectAltNames) dnsNames(commonName string) []string {
	values := make([]string, 0, len(s.dns)+2)
	seen := make(map[string]struct{})
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	add(commonName)
	add("localhost")
	for _, value := range s.dns {
		add(value)
	}
	return values
}

func (s networkTLSSubjectAltNames) ipAddresses() []net.IP {
	values := make([]net.IP, 0, len(s.ip)+2)
	seen := make(map[string]struct{})
	add := func(ip net.IP) {
		if ip == nil {
			return
		}
		key := ip.String()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		values = append(values, ip)
	}
	add(net.ParseIP("127.0.0.1"))
	add(net.ParseIP("::1"))
	for _, ip := range s.ip {
		add(ip)
	}
	return values
}

func buildNetworkTLSSubjectAltNames(index int, options networkAddressOptions) networkTLSSubjectAltNames {
	hostValues := []string{
		formatNetworkHostTemplate(options.P2PHostTemplate, index),
		formatNetworkHostTemplate(options.RPCHostTemplate, index),
		formatNetworkHostTemplate(options.P2PAdvertiseHostTemplate, index),
		formatNetworkHostTemplate(options.RPCAdvertiseHostTemplate, index),
	}
	ipValues := make([]net.IP, 0, 2)
	for _, host := range hostValues {
		if ip := net.ParseIP(host); ip != nil {
			ipValues = append(ipValues, ip)
		}
	}
	return networkTLSSubjectAltNames{dns: hostValues, ip: ipValues}
}

func formatNetworkHostTemplate(template string, index int) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	if strings.Contains(template, "%") {
		return fmt.Sprintf(template, index)
	}
	return template
}

func encodeECPrivateKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}), nil
}

func randomSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	if serial.Sign() == 0 {
		return big.NewInt(1), nil
	}
	return serial, nil
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
			peers[validatorP2PNodeID(validatorInfo)] = address
		}
	}
	return peers
}

func validatorP2PNodeID(validatorInfo validatorDocument) string {
	if validatorInfo.Metadata != nil {
		if nodeID := strings.TrimSpace(validatorInfo.Metadata["node_id"]); nodeID != "" {
			return nodeID
		}
	}
	return validatorInfo.ID
}

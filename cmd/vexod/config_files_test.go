package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/governance"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRunInitWritesConfigAndGenesis(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer

	if err := runInit(&buffer, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}

	output := buffer.String()
	if !strings.Contains(output, "initialized vexo node") ||
		!strings.Contains(output, filepath.Join(home, configFileName)) ||
		!strings.Contains(output, filepath.Join(home, keyFileName)) ||
		!strings.Contains(output, filepath.Join(home, nodeKeyFileName)) {
		t.Fatalf("unexpected init output:\n%s", output)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		moduleDocument, readErr := readModuleConfigDocument(filepath.Join(home, moduleConfigFileName))
		t.Logf("read module err=%v", readErr)
		if readErr == nil {
			configDocument, configReadErr := readConfigDocument(filepath.Join(home, configFileName))
			t.Logf("read config err=%v", configReadErr)
			networkDocument, _ := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
			consensusDocument, _ := readConsensusConfigDocument(filepath.Join(home, consensusConfigFileName))
			mempoolDocument, _ := readMempoolConfigDocument(filepath.Join(home, mempoolConfigFileName))
			if configReadErr == nil {
				debugCfg := configFromConfigDocuments(configDocument, moduleDocument, networkDocument, consensusDocument, mempoolDocument)
				t.Logf("debug config: app=%+v exec=%+v bank=%+v staking=%+v gov=%+v mempool=%+v p2p=%+v", debugCfg.Application, debugCfg.Execution, debugCfg.Bank, debugCfg.Staking, debugCfg.Governance, debugCfg.Mempool, debugCfg.P2P)
				t.Logf("debug validate: %v", debugCfg.Validate())
			}
		}
		t.Fatal(err)
	}
	if cfg.Chain.ChainID != "vexo-test" || cfg.DataDir != filepath.Join(home, "data") || cfg.ValidatorID != "alice" {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
	genesis, err := loadGenesis(filepath.Join(home, genesisFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		t.Fatal(err)
	}
	if len(genesis.Validators) != 1 || genesis.Validators[0].ID != "alice" || genesis.Validators[0].Address == "" || genesis.Governance[genesis.Validators[0].Address] != 1 {
		t.Fatalf("unexpected loaded genesis: %+v", genesis)
	}
	for _, fileName := range []string{moduleConfigFileName, networkConfigFileName, consensusConfigFileName, mempoolConfigFileName, logConfigFileName} {
		if _, err := os.Stat(filepath.Join(home, fileName)); err != nil {
			t.Fatalf("expected split config file %s: %v", fileName, err)
		}
	}
	for _, fileName := range []string{keyFileName, nodeKeyFileName, defaultVRFKeyFileName} {
		if _, err := os.Stat(filepath.Join(home, fileName)); err != nil {
			t.Fatalf("expected validator key file %s: %v", fileName, err)
		}
	}
	nodeKeyDocument, err := vexocrypto.LoadKeyDocument(filepath.Join(home, nodeKeyFileName))
	if err != nil {
		t.Fatal(err)
	}
	nodeAccount, err := keyDocumentAccountAddress(nodeKeyDocument)
	if err != nil {
		t.Fatal(err)
	}
	if len(genesis.AppState["bank:"+string(nodeAccount)]) == 0 {
		t.Fatalf("expected node account %s funded in genesis app state", nodeAccount)
	}
	configBytes, err := os.ReadFile(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configBytes), `"Application"`) ||
		strings.Contains(string(configBytes), `"Execution"`) ||
		strings.Contains(string(configBytes), `"Governance"`) ||
		strings.Contains(string(configBytes), `"runtime"`) ||
		strings.Contains(string(configBytes), `"chain"`) {
		t.Fatalf("expected runtime, chain, and module settings to be split out of config.json:\n%s", configBytes)
	}
	moduleDocument, err := readModuleConfigDocument(filepath.Join(home, moduleConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleDocument.Application.CoreModules) != 5 ||
		moduleDocument.Application.CoreModules[0] != "bank" ||
		moduleDocument.Application.CoreModules[1] != "staking" ||
		moduleDocument.Application.CoreModules[2] != "governance" ||
		moduleDocument.Application.CoreModules[3] != "params" ||
		moduleDocument.Application.CoreModules[4] != "ibc" ||
		len(moduleDocument.Application.Modules) != 5 ||
		moduleDocument.Execution.MaxGas == 0 ||
		moduleDocument.Execution.FeeDenom != "avxo" ||
		moduleDocument.Execution.DisplayDenom != "vexo" ||
		moduleDocument.Execution.GasDenom != "gas" ||
		!moduleDocument.Execution.RequireSigned ||
		!moduleDocument.Execution.RequireNonce ||
		moduleDocument.Execution.MinFee == 0 ||
		moduleDocument.Execution.BaseFee == 0 ||
		moduleDocument.Execution.MinGas == 0 ||
		moduleDocument.Execution.AllowUnprotectedLegacyTx ||
		moduleDocument.Bank.MintAuthority != "governance" ||
		moduleDocument.Governance.Timelock == 0 ||
		!moduleDocument.Governance.RequireDeposit ||
		moduleDocument.Governance.MinDeposit != "1avxo" ||
		moduleDocument.Governance.DepositEscrow == "" ||
		moduleDocument.Governance.RejectedDeposits == "" {
		t.Fatalf("unexpected module config: %+v", moduleDocument)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !networkDocument.RPC.Enabled ||
		networkDocument.RPC.Web3FilterSnapshot == "" ||
		!networkDocument.P2P.Enabled ||
		networkDocument.P2P.NodeID != "alice" ||
		networkDocument.P2P.NodeKeyPath != nodeKeyFileName ||
		networkDocument.P2P.AuthReplayPath == "" ||
		networkDocument.StateSync.Enabled ||
		networkDocument.StateSync.Timeout == "" ||
		networkDocument.StateSync.MaxSnapshotBytes == 0 ||
		networkDocument.PeerScoring.InitialScore == 0 {
		t.Fatalf("unexpected network config: %+v", networkDocument)
	}
	consensusDocument, err := readConsensusConfigDocument(filepath.Join(home, consensusConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !consensusDocument.Consensus.LoopEnabled ||
		consensusDocument.Consensus.ExecutionCommit != string(vexonode.ExecutionCommitModeFinalized) ||
		consensusDocument.Consensus.AdaptiveRoundTimeoutEnabled == nil ||
		!(*consensusDocument.Consensus.AdaptiveRoundTimeoutEnabled) ||
		consensusDocument.Consensus.RecoveryFinalityGateEnabled == nil ||
		!(*consensusDocument.Consensus.RecoveryFinalityGateEnabled) ||
		consensusDocument.Committee.CommitteeSize == 0 ||
		consensusDocument.Committee.Backend != committee.BackendVRF ||
		consensusDocument.VRF.AdapterName != vexocrypto.VRFAdapterECVRFP256Name ||
		!consensusDocument.VRF.ProductionAdapter ||
		len(consensusDocument.VRFKeyPaths) != 1 ||
		consensusDocument.VRFKeyPaths[0] != defaultVRFKeyFileName {
		t.Fatalf("unexpected consensus config: %+v", consensusDocument)
	}
	mempoolDocument, err := readMempoolConfigDocument(filepath.Join(home, mempoolConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if mempoolDocument.Mempool.MinFee == 0 ||
		!mempoolDocument.Mempool.EnablePriority ||
		!mempoolDocument.Mempool.EnableReplacement ||
		mempoolDocument.Mempool.WALPath == "" {
		t.Fatalf("unexpected mempool config: %+v", mempoolDocument)
	}
}

func TestRunInitWritesNetworkFiles(t *testing.T) {
	home := t.TempDir()
	var output bytes.Buffer
	if err := runInit(&output, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "4"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "initialized vexo network") || !strings.Contains(output.String(), "validators: 4") {
		t.Fatalf("unexpected network output:\n%s", output.String())
	}
	var sharedP2PAuthToken string

	for index := 1; index <= 4; index++ {
		validatorID := networkValidatorID(index)
		nodeHome := filepath.Join(home, validatorID)
		cfg, err := loadNodeConfig(filepath.Join(nodeHome, configFileName))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ValidatorID != types.ValidatorID(validatorID) || cfg.DataDir != filepath.Join(nodeHome, "data") {
			t.Fatalf("unexpected config for %s: %+v", validatorID, cfg)
		}
		genesis, err := loadGenesis(filepath.Join(nodeHome, genesisFileName))
		if err != nil {
			t.Fatal(err)
		}
		if len(genesis.Validators) != 4 || genesis.Governance[genesis.Validators[index-1].Address] != 1 {
			t.Fatalf("unexpected genesis for %s: %+v", validatorID, genesis)
		}
		nodeKeyDocument, err := vexocrypto.LoadKeyDocument(filepath.Join(nodeHome, nodeKeyFileName))
		if err != nil {
			t.Fatal(err)
		}
		nodeAccount, err := keyDocumentAccountAddress(nodeKeyDocument)
		if err != nil {
			t.Fatal(err)
		}
		if len(genesis.AppState["bank:"+string(nodeAccount)]) == 0 {
			t.Fatalf("expected node account %s funded in network genesis for %s", nodeAccount, validatorID)
		}
		validatorInfo := genesis.Validators[index-1]
		if len(validatorInfo.PublicKey) == 0 ||
			validatorInfo.Metadata["p2p_address"] != networkP2PAddress(index) ||
			validatorInfo.Metadata["rpc_address"] != networkRPCAddress(index) ||
			validatorInfo.Metadata["node_id"] != validatorID {
			t.Fatalf("unexpected validator metadata: %+v", validatorInfo)
		}
		if _, err := loadStartInputs(nodeHome, "", "", "", nil, false); err != nil {
			t.Fatalf("expected start inputs for %s: %v", validatorID, err)
		}
		for _, fileName := range []string{moduleConfigFileName, networkConfigFileName, consensusConfigFileName, mempoolConfigFileName, logConfigFileName} {
			if _, err := os.Stat(filepath.Join(nodeHome, fileName)); err != nil {
				t.Fatalf("expected network split config %s for %s: %v", fileName, validatorID, err)
			}
		}
		for _, fileName := range []string{nodeKeyFileName, defaultVRFKeyFileName} {
			if _, err := os.Stat(filepath.Join(nodeHome, fileName)); err != nil {
				t.Fatalf("expected network key %s for %s: %v", fileName, validatorID, err)
			}
		}
		networkDocument, err := readNetworkConfigDocument(filepath.Join(nodeHome, networkConfigFileName))
		if err != nil {
			t.Fatal(err)
		}
		if networkDocument.P2P.NodeID != validatorID || networkDocument.P2P.NodeKeyPath != nodeKeyFileName {
			t.Fatalf("expected p2p node identity for %s, got %+v", validatorID, networkDocument.P2P)
		}
		if networkDocument.P2P.AuthToken == "" ||
			networkDocument.P2P.AuthReplayPath == "" ||
			networkDocument.P2P.TLSCertPath != filepath.Join("tls", "node.crt") ||
			networkDocument.P2P.TLSKeyPath != filepath.Join("tls", "node.key") ||
			networkDocument.P2P.TLSCAPath != filepath.Join("tls", "ca.crt") ||
			networkDocument.RPC.TLSCertPath != filepath.Join("tls", "node.crt") ||
			networkDocument.RPC.TLSKeyPath != filepath.Join("tls", "node.key") {
			t.Fatalf("expected p2p auth replay path for %s, got %+v", validatorID, networkDocument.P2P)
		}
		if index == 1 {
			sharedP2PAuthToken = networkDocument.P2P.AuthToken
		} else if networkDocument.P2P.AuthToken != sharedP2PAuthToken {
			t.Fatalf("expected shared p2p auth token for %s, got %+v", validatorID, networkDocument.P2P)
		}
		for _, fileName := range []string{"tls/ca.crt", "tls/node.crt", "tls/node.key"} {
			if _, err := os.Stat(filepath.Join(nodeHome, fileName)); err != nil {
				t.Fatalf("expected tls artifact %s for %s: %v", fileName, validatorID, err)
			}
		}
		consensusDocument, err := readConsensusConfigDocument(filepath.Join(nodeHome, consensusConfigFileName))
		if err != nil {
			t.Fatal(err)
		}
		if len(consensusDocument.VRFKeyPaths) != 1 || consensusDocument.VRFKeyPaths[0] != defaultVRFKeyFileName {
			t.Fatalf("expected network VRF key path for %s, got %+v", validatorID, consensusDocument.VRFKeyPaths)
		}
	}

	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "4"}); err == nil {
		t.Fatal("expected network init to reject existing files")
	}
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "4", "--overwrite"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunInitCanEncryptGeneratedValidatorAndVRFKeys(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"validator", "--home", home, "--chain-id", "vexo-test", "--validator", "alice", "--encrypt-keys", "--passphrase", "secret"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(home, keyFileName),
		filepath.Join(home, nodeKeyFileName),
		filepath.Join(home, defaultVRFKeyFileName),
	} {
		document, err := vexocrypto.LoadKeyDocument(path)
		if err != nil {
			t.Fatal(err)
		}
		if document.Encryption == nil || document.PrivateKey != "" {
			t.Fatalf("expected encrypted key document at %s, got %+v", path, document)
		}
	}
}

func TestRunInitNetworkCanEncryptGeneratedValidatorAndVRFKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("VEXO_KEY_PASSPHRASE", "secret")
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2", "--encrypt-keys"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(home, "validator-1", keyFileName),
		filepath.Join(home, "validator-1", nodeKeyFileName),
		filepath.Join(home, "validator-1", defaultVRFKeyFileName),
		filepath.Join(home, "validator-2", keyFileName),
		filepath.Join(home, "validator-2", nodeKeyFileName),
		filepath.Join(home, "validator-2", defaultVRFKeyFileName),
	} {
		document, err := vexocrypto.LoadKeyDocument(path)
		if err != nil {
			t.Fatal(err)
		}
		if document.Encryption == nil || document.PrivateKey != "" {
			t.Fatalf("expected encrypted key document at %s, got %+v", path, document)
		}
	}
}

func TestRunInitNetworkBLSWritesProofOfPossession(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2", "--key-type", "bls"}); err != nil {
		t.Fatal(err)
	}
	genesis, err := loadGenesis(filepath.Join(home, "validator-1", genesisFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(genesis.Validators) != 2 {
		t.Fatalf("unexpected validators: %+v", genesis.Validators)
	}
	for _, validatorInfo := range genesis.Validators {
		if validatorInfo.Metadata[vexocrypto.BLSProofOfPossessionMetadataKey] == "" || validatorInfo.Metadata["bls_adapter"] != vexocrypto.BLSAdapterBLSTName {
			t.Fatalf("expected BLS metadata, got %+v", validatorInfo.Metadata)
		}
	}
	keyDocument, err := vexocrypto.LoadKeyDocument(filepath.Join(home, "validator-1", keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if keyDocument.Type != vexocrypto.KeyTypeBLS || keyDocument.Metadata.BLSProofOfPossession == "" {
		t.Fatalf("unexpected BLS key document: %+v", keyDocument)
	}
	consensusDocument, err := readConsensusConfigDocument(filepath.Join(home, "validator-1", consensusConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if consensusDocument.Crypto.Backend != config.CryptoBackendBLS ||
		!consensusDocument.Crypto.ProductionAdapter ||
		consensusDocument.Crypto.AdapterName != vexocrypto.BLSAdapterBLSTName {
		t.Fatalf("expected BLST consensus crypto config, got %+v", consensusDocument.Crypto)
	}
}

func TestRunInitNetworkEd25519WritesEd25519CryptoConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2", "--key-type", "ed25519"}); err != nil {
		t.Fatal(err)
	}
	keyDocument, err := vexocrypto.LoadKeyDocument(filepath.Join(home, "validator-1", keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if keyDocument.Type != vexocrypto.KeyTypeEd25519 {
		t.Fatalf("unexpected key document: %+v", keyDocument)
	}
	consensusDocument, err := readConsensusConfigDocument(filepath.Join(home, "validator-1", consensusConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if consensusDocument.Crypto.Backend != config.CryptoBackendEd25519 ||
		consensusDocument.Crypto.ProductionAdapter ||
		consensusDocument.Crypto.AdapterName != "" {
		t.Fatalf("expected Ed25519 consensus crypto config, got %+v", consensusDocument.Crypto)
	}
}

func TestRunInitOverwriteClearsRuntimeArtifacts(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2"}); err != nil {
		t.Fatal(err)
	}
	runtimeArtifacts := []string{
		filepath.Join(home, "validator-1", "data", "consensus.wal"),
		filepath.Join(home, "validator-1", "data", "peer_scores.json"),
		filepath.Join(home, "validator-1", "data", "store", "LOCK"),
		filepath.Join(home, "validator-1", networkPIDFileName),
		filepath.Join(home, "validator-1", "vexod.log"),
	}
	for _, path := range runtimeArtifacts {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "2", "--overwrite"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range runtimeArtifacts {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected stale runtime artifact removed %s, got %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "validator-1", "data")); err != nil {
		t.Fatalf("expected data directory recreated: %v", err)
	}
}

func TestRunInitRejectsExistingFilesUnlessOverwrite(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if err := runInit(&bytes.Buffer{}, []string{"--home", home}); err == nil {
		t.Fatal("expected init to reject existing files")
	}
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--overwrite"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunInitRejectsExistingKeyBeforeWritingConfigs(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, nodeKeyFileName), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runInit(&bytes.Buffer{}, []string{"validator", "--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err == nil {
		t.Fatal("expected init to reject existing node key")
	}
	if _, err := os.Stat(filepath.Join(home, configFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("init must not write config after key preflight failure, got %v", err)
	}
}

func TestRunValidateAcceptsGeneratedFiles(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	if err := runValidate(&buffer, []string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "configuration valid") {
		t.Fatalf("unexpected validate output:\n%s", buffer.String())
	}
}

func TestLoadNodeConfigMergesModuleConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	moduleDocument := moduleConfigDocument{
		SchemaVersion: moduleSchemaVersion,
		Application:   config.ApplicationConfig{Modules: append(config.DefaultApplicationModules(), "custom")},
		Execution:     config.ExecutionConfig{RequireSigned: true, RequireNonce: true, MinFee: 7, MinGas: 3, TargetGas: 50, MaxGas: 99, FeeCollector: "collector"},
		Bank:          config.BankConfig{MintAuthority: "governance"},
		Governance:    governance.TallyPolicy{QuorumPower: 2, YesThresholdPower: 2, VotingPeriod: 9, Timelock: 4, VetoPower: 1},
	}
	writeTestJSON(t, filepath.Join(home, moduleConfigFileName), moduleDocument)

	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chain.Application.CoreModules) != 5 || len(cfg.Chain.Application.Modules) != 6 || cfg.Chain.Application.Modules[5] != "custom" {
		t.Fatalf("expected module config override, got %+v", cfg.Chain.Application)
	}
	if !cfg.Chain.Execution.RequireSigned || cfg.Chain.Execution.MinFee != 7 || cfg.Chain.Execution.FeeCollector != "collector" || cfg.Chain.Execution.FeeDenom != "avxo" {
		t.Fatalf("expected execution config override, got %+v", cfg.Chain.Execution)
	}
	if cfg.Chain.Bank.MintAuthority != "governance" {
		t.Fatalf("expected bank config override, got %+v", cfg.Chain.Bank)
	}
	if cfg.Chain.Governance.Timelock != 4 {
		t.Fatalf("expected governance config override, got %+v", cfg.Chain.Governance)
	}
}

func TestLoadNodeConfigMergesSplitConfigFiles(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.PeerScoring.InitialScore = 777
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	consensusDocument, err := readConsensusConfigDocument(filepath.Join(home, consensusConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	consensusDocument.Committee.CommitteeSize = 9
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)
	mempoolDocument, err := readMempoolConfigDocument(filepath.Join(home, mempoolConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	mempoolDocument.Mempool.MaxTxs = 123
	writeTestJSON(t, filepath.Join(home, mempoolConfigFileName), mempoolDocument)

	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Chain.P2P.InitialScore != 777 || cfg.Chain.Committee.CommitteeSize != 9 || cfg.Chain.Mempool.MaxTxs != 123 {
		t.Fatalf("expected split config overrides, got %+v", cfg.Chain)
	}
}

func TestRuntimeConfigLoadsEVMAccountKeysFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	document, err := readConfigDocument(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	document.RequireNetworkSafety = false
	writeTestJSON(t, filepath.Join(home, configFileName), document)
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.RPC.EVMAccountPrivateKeys = []string{"0xabc", "0xdef"}
	networkDocument.RPC.EVMManagedAccounts = true
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if !runtimeConfig.RPCEVMManagedAccounts || len(runtimeConfig.RPCEVMAccountKeys) != 2 || runtimeConfig.RPCEVMAccountKeys[0] != "0xabc" || runtimeConfig.RPCEVMAccountKeys[1] != "0xdef" {
		t.Fatalf("expected enabled RPC EVM account keys from network config, got enabled=%v keys=%+v", runtimeConfig.RPCEVMManagedAccounts, runtimeConfig.RPCEVMAccountKeys)
	}
}

func TestRuntimeConfigLoadsEVMAccountKeyEnvsFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.RPC.EVMAccountKeyEnvs = []string{"VEXO_EVM_KEY_A", "VEXO_EVM_KEY_B"}
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if !runtimeConfig.RPCEVMManagedAccounts || len(runtimeConfig.RPCEVMAccountKeyEnvs) != 2 || runtimeConfig.RPCEVMAccountKeyEnvs[0] != "VEXO_EVM_KEY_A" {
		t.Fatalf("expected enabled RPC EVM account key envs, got enabled=%v envs=%+v", runtimeConfig.RPCEVMManagedAccounts, runtimeConfig.RPCEVMAccountKeyEnvs)
	}
	t.Setenv("VEXO_EVM_KEY_A", "0xabc")
	t.Setenv("VEXO_EVM_KEY_B", "0xdef")
	keys, err := resolveEVMAccountKeys(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "0xabc" || keys[1] != "0xdef" {
		t.Fatalf("expected keys from envs, got %+v", keys)
	}
}

func TestResolveEVMAccountKeysRejectsMissingEnv(t *testing.T) {
	_, err := resolveEVMAccountKeys(startRuntimeConfig{RPCEVMAccountKeyEnvs: []string{"VEXO_MISSING_EVM_KEY"}})
	if err == nil || !strings.Contains(err.Error(), "VEXO_MISSING_EVM_KEY") {
		t.Fatalf("expected missing env error, got %v", err)
	}
}

func TestRuntimeConfigLoadsP2PTLSFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.P2P.TLSCertPath = "tls/node.crt"
	networkDocument.P2P.TLSKeyPath = "tls/node.key"
	networkDocument.P2P.TLSCAPath = "tls/ca.crt"
	networkDocument.P2P.TLSServerName = "validator.internal"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	expectedCert := filepath.Join(home, "tls/node.crt")
	expectedKey := filepath.Join(home, "tls/node.key")
	expectedCA := filepath.Join(home, "tls/ca.crt")
	if runtimeConfig.P2PTLSCertPath != expectedCert ||
		runtimeConfig.P2PTLSKeyPath != expectedKey ||
		runtimeConfig.P2PTLSCAPath != expectedCA ||
		runtimeConfig.P2PTLSServerName != "validator.internal" {
		t.Fatalf("expected resolved p2p TLS config, got %+v", runtimeConfig)
	}
}

func TestRuntimeConfigLoadsP2PAuthReplayPathFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.P2P.AuthReplayPath = "security/p2p-auth-replay.jsonl"
	networkDocument.P2P.RequireAuthReplayStore = true
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtimeConfig.P2PAuthReplayPath != filepath.Join(home, "security/p2p-auth-replay.jsonl") || !runtimeConfig.P2PRequireAuthReplay {
		t.Fatalf("expected resolved p2p auth replay config, got %+v", runtimeConfig)
	}
}

func TestRuntimeConfigLoadsP2PDialTimeoutFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.P2P.DialTimeout = "12s"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtimeConfig.P2PDialTimeout != 12*time.Second {
		t.Fatalf("expected 12s p2p dial timeout, got %s", runtimeConfig.P2PDialTimeout)
	}
}

func TestRuntimeConfigLoadsP2PNodeIdentityFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"archive", "--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.P2P.NodeID = "archive-rpc-1"
	networkDocument.P2P.NodeKeyPath = "keys/node.key.json"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtimeConfig.P2PNodeID != "archive-rpc-1" || runtimeConfig.P2PNodeKeyPath != filepath.Join(home, "keys/node.key.json") {
		t.Fatalf("expected resolved p2p node identity, got %+v", runtimeConfig)
	}
}

func TestRuntimeConfigLoadsRPCTLSFromSplitNetworkConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	networkDocument, err := readNetworkConfigDocument(filepath.Join(home, networkConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	networkDocument.RPC.TLSCertPath = "tls/rpc.crt"
	networkDocument.RPC.TLSKeyPath = "tls/rpc.key"
	networkDocument.RPC.TLSCAPath = "tls/rpc-ca.crt"
	networkDocument.RPC.TLSServerName = "rpc.validator.internal"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	runtimeConfig, err := loadStartRuntimeConfig(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtimeConfig.RPCTLSCertPath != filepath.Join(home, "tls/rpc.crt") ||
		runtimeConfig.RPCTLSKeyPath != filepath.Join(home, "tls/rpc.key") ||
		runtimeConfig.RPCTLSCAPath != filepath.Join(home, "tls/rpc-ca.crt") ||
		runtimeConfig.RPCTLSServerName != "rpc.validator.internal" {
		t.Fatalf("expected resolved rpc TLS config, got %+v", runtimeConfig)
	}
}

func TestLoadP2PTLSConfigRequiresCAForConfiguredIdentity(t *testing.T) {
	_, err := loadP2PTLSConfig(startRuntimeConfig{
		P2PTLSCertPath: "tls/node.crt",
		P2PTLSKeyPath:  "tls/node.key",
	})
	if err == nil {
		t.Fatal("expected p2p tls identity without ca to fail")
	}
}

func TestLoadRPCTLSConfigRejectsPartialIdentity(t *testing.T) {
	_, err := loadRPCTLSConfig(startRuntimeConfig{RPCTLSCertPath: "tls/rpc.crt"})
	if err == nil || !strings.Contains(err.Error(), "cert and key") {
		t.Fatalf("expected rpc tls partial identity error, got %v", err)
	}
}

func TestLoadRPCTLSConfigRequiresCAForServerName(t *testing.T) {
	_, err := loadRPCTLSConfig(startRuntimeConfig{
		RPCTLSCertPath:   "tls/rpc.crt",
		RPCTLSKeyPath:    "tls/rpc.key",
		RPCTLSServerName: "rpc.validator.internal",
	})
	if err == nil || !strings.Contains(err.Error(), "server name requires") {
		t.Fatalf("expected rpc tls server name ca error, got %v", err)
	}
}

func TestApplyStartFlagOverridesEVMAccountKeys(t *testing.T) {
	runtimeConfig := startRuntimeConfig{RPCEVMAccountKeys: []string{"existing"}}
	applyStartFlagOverrides(&runtimeConfig, map[string]bool{"evm-account-key": true}, startFlagValues{
		rpcEVMAccountKeys: []string{"0xabc", "0xdef"},
	})
	if !runtimeConfig.RPCEVMManagedAccounts || len(runtimeConfig.RPCEVMAccountKeys) != 2 || runtimeConfig.RPCEVMAccountKeys[0] != "0xabc" || runtimeConfig.RPCEVMAccountKeys[1] != "0xdef" {
		t.Fatalf("expected enabled flag-provided EVM account keys, got enabled=%v keys=%+v", runtimeConfig.RPCEVMManagedAccounts, runtimeConfig.RPCEVMAccountKeys)
	}
}

func TestApplyStartFlagOverridesEVMAccountKeyEnvs(t *testing.T) {
	runtimeConfig := startRuntimeConfig{}
	applyStartFlagOverrides(&runtimeConfig, map[string]bool{"evm-account-key-env": true}, startFlagValues{
		rpcEVMAccountKeyEnvs: []string{"VEXO_EVM_KEY"},
	})
	if !runtimeConfig.RPCEVMManagedAccounts || len(runtimeConfig.RPCEVMAccountKeyEnvs) != 1 || runtimeConfig.RPCEVMAccountKeyEnvs[0] != "VEXO_EVM_KEY" {
		t.Fatalf("expected enabled flag-provided EVM account key envs, got enabled=%v envs=%+v", runtimeConfig.RPCEVMManagedAccounts, runtimeConfig.RPCEVMAccountKeyEnvs)
	}
}

func TestRunStartRejectsRuntimeFlagOverrides(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	err := runStart(&bytes.Buffer{}, []string{"--home", home, "--dry-run", "--timeout-propose", "250ms"})
	if !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected runtime flag override to be rejected, got %v", err)
	}
}

func TestLoadNodeConfigForcesNetworkSafety(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, configFileName)
	document, err := readConfigDocument(configPath)
	if err != nil {
		t.Fatal(err)
	}
	document.RequireNetworkSafety = false
	writeTestJSON(t, configPath, document)

	cfg, err := loadNodeConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.RequireNetworkSafety {
		t.Fatalf("expected node config load path to force network safety")
	}
}

func TestLoadNodeConfigUsesDefaultModuleConfigWhenSplitFileMissing(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, moduleConfigFileName)); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chain.Application.CoreModules) != 5 || len(cfg.Chain.Application.Modules) != 5 || cfg.Chain.Execution.MaxGas == 0 || cfg.Chain.Governance.Timelock == 0 {
		t.Fatalf("expected default module config fallback, got %+v", cfg.Chain)
	}
}

func TestRunConfigPathsReportsCustomModuleConfig(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	document, err := readConfigDocument(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	document.ModuleConfigPath = "modules/custom.json"
	document.NetworkConfigPath = "modules/network.json"
	document.ConsensusConfigPath = "modules/consensus.json"
	document.MempoolConfigPath = "modules/mempool.json"
	document.LogConfigPath = "modules/log.json"
	if err := os.MkdirAll(filepath.Join(home, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(home, "modules", "custom.json"), defaultModuleConfigDocument("vexo-test"))
	writeTestJSON(t, filepath.Join(home, "modules", "network.json"), defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "validator-1"))
	writeTestJSON(t, filepath.Join(home, "modules", "consensus.json"), defaultConsensusConfigDocument("vexo-test", filepath.Join(home, "data"), "validator-1"))
	writeTestJSON(t, filepath.Join(home, "modules", "mempool.json"), defaultMempoolConfigDocument("vexo-test", filepath.Join(home, "data")))
	writeTestJSON(t, filepath.Join(home, "modules", "log.json"), defaultLogConfigDocument("vexo-test", filepath.Join(home, "data"), "validator-1"))
	writeTestJSON(t, filepath.Join(home, configFileName), document)

	var output bytes.Buffer
	if err := runConfig(&output, []string{"paths", "--home", home}); err != nil {
		t.Fatal(err)
	}
	for label, expected := range map[string]string{
		"module_config":    filepath.Join(home, "modules", "custom.json"),
		"network_config":   filepath.Join(home, "modules", "network.json"),
		"consensus_config": filepath.Join(home, "modules", "consensus.json"),
		"mempool_config":   filepath.Join(home, "modules", "mempool.json"),
		"log_config":       filepath.Join(home, "modules", "log.json"),
	} {
		if !strings.Contains(output.String(), label+": "+expected) {
			t.Fatalf("expected custom %s path %q in output:\n%s", label, expected, output.String())
		}
	}
}

func TestRunValidateRejectsMismatchedGenesis(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	genesisPath := filepath.Join(home, genesisFileName)
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		t.Fatal(err)
	}
	var document genesisDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document.ChainID = "other"
	writeTestJSON(t, genesisPath, document)

	err = runValidate(&bytes.Buffer{}, []string{"--home", home})
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("expected validation failure, got %v", err)
	}
}

func TestLoadGenesisDecodesAppStateAndPublicKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), genesisFileName)
	writeTestJSON(t, path, genesisDocument{
		SchemaVersion: genesisSchemaVersion,
		ChainID:       "vexo-test",
		Validators: []validatorDocument{
			{
				ID:          "alice",
				Address:     "alice",
				PublicKey:   base64.StdEncoding.EncodeToString([]byte("alice-public-key")),
				VotingPower: 7,
				Stake:       70,
				Metadata:    map[string]string{"region": "kr"},
			},
		},
		AppState:   map[string]string{"bank": base64.StdEncoding.EncodeToString([]byte("alice=100"))},
		Governance: map[string]uint64{"alice": 7},
	})

	genesis, err := loadGenesis(path)
	if err != nil {
		t.Fatal(err)
	}
	if genesis.ChainID != "vexo-test" || string(genesis.AppState["bank"]) != "alice=100" {
		t.Fatalf("unexpected genesis app state: %+v", genesis)
	}
	if len(genesis.Validators) != 1 || string(genesis.Validators[0].PublicKey) != "alice-public-key" {
		t.Fatalf("unexpected validator public key: %+v", genesis.Validators)
	}
	if genesis.Validators[0].VotingPower != types.VotingPower(7) || genesis.Validators[0].Metadata["region"] != "kr" {
		t.Fatalf("unexpected validator metadata: %+v", genesis.Validators[0])
	}
}

func TestLoadNodeConfigRejectsInvalidSchemaAndConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFileName)
	document := defaultConfigDocument("vexo-test", t.TempDir(), "alice")
	document.SchemaVersion = "v0"
	writeTestJSON(t, path, document)
	if _, err := loadNodeConfig(path); err == nil {
		t.Fatal("expected invalid config schema")
	}

	document = defaultConfigDocument("", t.TempDir(), "alice")
	writeTestJSON(t, path, document)
	if _, err := loadNodeConfig(path); err == nil {
		t.Fatal("expected invalid chain config")
	}
}

func TestDefaultConfigEnablesOperationalEventLogs(t *testing.T) {
	document := defaultLogConfigDocument("vexo-test", t.TempDir(), "alice")
	if document.Log.CommitEvents == nil || !*document.Log.CommitEvents {
		t.Fatalf("expected commit event logging enabled by default: %+v", document.Log)
	}
	if document.Log.PeerEvents == nil || !*document.Log.PeerEvents {
		t.Fatalf("expected peer event logging enabled by default: %+v", document.Log)
	}
}

func TestDefaultConfigWritesTendermintStyleConsensusTimeouts(t *testing.T) {
	document := defaultConsensusConfigDocument("vexo-test", t.TempDir(), "alice")
	consensus := document.Consensus
	if consensus.TimeoutPropose != "3s" ||
		consensus.TimeoutPrevote != "1s" ||
		consensus.TimeoutPrecommit != "1s" ||
		consensus.TimeoutCommit != "1s" ||
		consensus.RoundTimeout != "3s" {
		t.Fatalf("unexpected consensus timeouts: %+v", consensus)
	}
	if consensus.CreateEmptyBlocks {
		t.Fatalf("expected empty block creation disabled by default: %+v", consensus)
	}
	if consensus.ExecutionCommit != string(vexonode.ExecutionCommitModeFinalized) {
		t.Fatalf("expected finalized execution commit mode by default: %+v", consensus)
	}
}

func TestLoadNodeConfigLoadsEncryptedVRFKeyDocuments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("VEXO_KEY_PASSPHRASE", "secret")
	consensusDir := filepath.Join(home, "modules")
	if err := os.MkdirAll(consensusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keyDocument, err := vexocrypto.GenerateECVRFP256KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	encryptedKeyDocument, err := keyDocument.Encrypted("secret")
	if err != nil {
		t.Fatal(err)
	}
	vrfKeyPath := filepath.Join(consensusDir, "validator.vrf.key.json")
	if err := vexocrypto.SaveKeyDocument(vrfKeyPath, encryptedKeyDocument); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, configFileName)
	configDocument := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	configDocument.ConsensusConfigPath = filepath.Join("modules", "consensus.json")
	writeTestJSON(t, configPath, configDocument)
	consensusDocument := defaultConsensusConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument.Committee.Backend = committee.BackendVRF
	consensusDocument.VRF.ProductionAdapter = true
	consensusDocument.VRF.AdapterName = vexocrypto.VRFAdapterECVRFP256Name
	consensusDocument.VRF.AuditReport = "ecvrf-test-audit"
	consensusDocument.VRF.DependencyAudit = config.NetworkSafeVRFDependencyAudit
	consensusDocument.VRF.AuditEvidenceSHA256 = config.NetworkSafeVRFAuditEvidence
	consensusDocument.VRF.KeySource = "config.vrf.keys"
	consensusDocument.VRFKeyPaths = []string{"validator.vrf.key.json"}
	writeTestJSON(t, filepath.Join(consensusDir, "consensus.json"), consensusDocument)

	cfg, err := loadNodeConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := base64.StdEncoding.DecodeString(keyDocument.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chain.VRF.Keys[string(publicKey)]) == 0 {
		t.Fatalf("expected VRF key loaded from key document, got %+v", cfg.Chain.VRF.Keys)
	}
	vrf, err := vexocrypto.NewVRF(cfg.Chain.VRF)
	if err != nil {
		t.Fatal(err)
	}
	output, proof, err := vrf.Prove(publicKey, []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	if !vrf.Verify(publicKey, []byte("seed"), output, proof) {
		t.Fatal("expected loaded VRF key to prove and verify")
	}
}

func TestLoadStartRuntimeConfigAllowsDisablingOperationalEventLogs(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	logDocument := defaultLogConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	logDocument.Log.CommitEvents = boolPtr(false)
	logDocument.Log.PeerEvents = boolPtr(false)
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, logConfigFileName), logDocument)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogCommitEvents || cfg.LogPeerEvents {
		t.Fatalf("expected operational event logs disabled, got commit=%t peer=%t", cfg.LogCommitEvents, cfg.LogPeerEvents)
	}
}

func TestLoadStartRuntimeConfigParsesShutdownTimeout(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument := defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument.RPC.ShutdownTimeout = "3s"
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("expected shutdown timeout 3s, got %s", cfg.ShutdownTimeout)
	}

	networkDocument.RPC.ShutdownTimeout = "0s"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid zero shutdown timeout, got %v", err)
	}
}

func TestLoadStartRuntimeConfigParsesWeb3SubscriptionLimits(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument := defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument.RPC.Web3MaxSubscriptions = 7
	networkDocument.RPC.Web3IdleTimeout = "45s"
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPCWeb3MaxSubscriptions != 7 || cfg.RPCWeb3IdleTimeout != 45*time.Second {
		t.Fatalf("expected web3 subscription config, got max=%d idle=%s", cfg.RPCWeb3MaxSubscriptions, cfg.RPCWeb3IdleTimeout)
	}

	networkDocument.RPC.Web3IdleTimeout = "0s"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid zero web3 idle timeout, got %v", err)
	}
}

func TestLoadStartRuntimeConfigParsesStateSyncConfig(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument := defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument.StateSync = runtimeStateSyncConfig{
		Enabled:           true,
		SnapshotURLs:      []string{"http://127.0.0.1:26657/v1/snapshot/export"},
		Timeout:           "7s",
		MinHeight:         100,
		RequireFresh:      true,
		TrustLocalHigher:  true,
		MaxSnapshotBytes:  1024,
		RetryAllSnapshots: true,
	}
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.StateSync.Enabled ||
		len(cfg.StateSync.SnapshotURLs) != 1 ||
		cfg.StateSync.SnapshotURLs[0] != "http://127.0.0.1:26657/v1/snapshot/export" ||
		cfg.StateSync.Timeout != "7s" ||
		cfg.StateSync.MinHeight != 100 ||
		!cfg.StateSync.RequireFresh ||
		!cfg.StateSync.TrustLocalHigher ||
		cfg.StateSync.MaxSnapshotBytes != 1024 ||
		!cfg.StateSync.RetryAllSnapshots {
		t.Fatalf("unexpected state sync runtime config: %+v", cfg.StateSync)
	}

	networkDocument.StateSync.Timeout = "0s"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid zero state sync timeout, got %v", err)
	}

	networkDocument.StateSync.Timeout = "7s"
	networkDocument.StateSync.SnapshotURLs = []string{"file:///tmp/snapshot.json"}
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("expected invalid state sync URL scheme, got %v", err)
	}

	networkDocument.StateSync.SnapshotURLs = nil
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing state sync URL rejection, got %v", err)
	}
}

func TestLoadStartRuntimeConfigParsesConsensusTimeoutsAndEmptyBlocks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument := defaultConsensusConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument.Consensus.TimeoutPropose = "4s"
	consensusDocument.Consensus.TimeoutPrevote = "1500ms"
	consensusDocument.Consensus.TimeoutPrecommit = "2s"
	consensusDocument.Consensus.TimeoutCommit = "250ms"
	consensusDocument.Consensus.CreateEmptyBlocks = true
	consensusDocument.Consensus.ExecutionCommit = string(vexonode.ExecutionCommitModeFinalized)
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConsensusLoop.TimeoutPropose != 4*time.Second ||
		cfg.ConsensusLoop.TimeoutPrevote != 1500*time.Millisecond ||
		cfg.ConsensusLoop.TimeoutPrecommit != 2*time.Second ||
		cfg.ConsensusLoop.TimeoutCommit != 250*time.Millisecond ||
		!cfg.ConsensusLoop.CreateEmptyBlocks ||
		!cfg.ConsensusLoop.AdaptiveRoundTimeoutEnabled ||
		!cfg.ConsensusLoop.RecoveryFinalityGateEnabled ||
		cfg.ConsensusLoop.ExecutionCommitMode != vexonode.ExecutionCommitModeFinalized {
		t.Fatalf("unexpected consensus runtime config: %+v", cfg.ConsensusLoop)
	}
}

func TestLoadStartRuntimeConfigHonorsConsensusFeatureDisables(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument := defaultConsensusConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument.Consensus.AdaptiveRoundTimeoutEnabled = boolPtr(false)
	consensusDocument.Consensus.RecoveryFinalityGateEnabled = boolPtr(false)
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConsensusLoop.AdaptiveRoundTimeoutEnabled || cfg.ConsensusLoop.RecoveryFinalityGateEnabled {
		t.Fatalf("expected consensus feature toggles to be loadable, got %+v", cfg.ConsensusLoop)
	}
}

func TestLoadStartRuntimeConfigRequiresFinalizedCommitWhenNetworkSafetyIsRequired(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	document.RequireNetworkSafety = true
	consensusDocument := defaultConsensusConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument.Consensus.ExecutionCommit = string(vexonode.ExecutionCommitModeQC)
	networkDocument := defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument.P2P.ListenAddress = "0.0.0.0:26656"
	networkDocument.P2P.AuthToken = "network-auth-token"
	networkDocument.P2P.TLSCertPath = "tls/node.crt"
	networkDocument.P2P.TLSKeyPath = "tls/node.key"
	networkDocument.P2P.TLSCAPath = "tls/ca.crt"
	networkDocument.P2P.AuthReplayPath = ""
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	_, err := loadStartRuntimeConfig(home, path)
	if !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected unsafe network config for qc commit with safety gate, got %v", err)
	}
	consensusDocument.Consensus.ExecutionCommit = string(vexonode.ExecutionCommitModeFinalized)
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing p2p auth replay path to fail network safety boundary, got %v", err)
	}
	networkDocument.P2P.AuthReplayPath = "data/p2p_auth_replay.jsonl"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); err != nil {
		t.Fatalf("expected durable auth replay path to satisfy runtime safety boundary, got %v", err)
	}

	networkDocument.P2P.TLSCAPath = ""
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected missing p2p mtls ca to fail network safety boundary, got %v", err)
	}
}

func TestLoadStartRuntimeConfigRequiresExplicitUnsafeOptInForQCCommit(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	document.RequireNetworkSafety = false
	consensusDocument := defaultConsensusConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	consensusDocument.Consensus.ExecutionCommit = string(vexonode.ExecutionCommitModeQC)
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)

	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected qc commit without explicit unsafe opt-in to fail, got %v", err)
	}
	consensusDocument.Consensus.AllowUnsafeQCCommit = true
	writeTestJSON(t, filepath.Join(home, consensusConfigFileName), consensusDocument)
	if cfg, err := loadStartRuntimeConfig(home, path); err != nil || !cfg.ConsensusLoop.AllowUnsafeQCCommit || cfg.ConsensusLoop.ExecutionCommitMode != vexonode.ExecutionCommitModeQC {
		t.Fatalf("expected explicit unsafe qc commit to load when network safety gate is off, cfg=%+v err=%v", cfg.ConsensusLoop, err)
	}
	document.RequireNetworkSafety = true
	writeTestJSON(t, path, document)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected qc commit to fail when require_network_safety=true, got %v", err)
	}
}

func TestLoadStartRuntimeConfigRejectsManagedEVMKeysOnPublicRPC(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument := defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument.RPC.Address = "0.0.0.0:26657"
	networkDocument.RPC.AdminToken = "secret"
	networkDocument.RPC.EVMManagedAccounts = true
	networkDocument.RPC.EVMAccountPrivateKeys = []string{"0xabc"}
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected public rpc hot key config to fail network safety boundary, got %v", err)
	}

	networkDocument.RPC.Address = "127.0.0.1:26657"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected require_network_safety to reject direct private key config even on private rpc, got %v", err)
	}

	document.RequireNetworkSafety = false
	writeTestJSON(t, path, document)
	if _, err := loadStartRuntimeConfig(home, path); err != nil {
		t.Fatalf("expected private rpc managed account config to load when network safety gate is disabled, got %v", err)
	}

	document.RequireNetworkSafety = true
	writeTestJSON(t, path, document)
	networkDocument.RPC.Address = "0.0.0.0:26657"
	networkDocument.RPC.EVMAccountPrivateKeys = nil
	networkDocument.RPC.EVMAccountKeyEnvs = []string{"VEXO_EVM_KEY"}
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected public rpc hot key env config to fail network safety boundary, got %v", err)
	}
}

func TestLoadStartRuntimeConfigRequiresTLSOnPublicRPC(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	document.RequireNetworkSafety = false
	networkDocument := defaultNetworkConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	networkDocument.RPC.Address = "0.0.0.0:26657"
	networkDocument.RPC.AdminToken = "secret"
	writeTestJSON(t, path, document)
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)

	if _, err := loadStartRuntimeConfig(home, path); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected public rpc without tls to fail network safety boundary, got %v", err)
	}
	networkDocument.RPC.TLSCertPath = "tls/rpc.crt"
	networkDocument.RPC.TLSKeyPath = "tls/rpc.key"
	writeTestJSON(t, filepath.Join(home, networkConfigFileName), networkDocument)
	if cfg, err := loadStartRuntimeConfig(home, path); err != nil || cfg.RPCTLSCertPath == "" || cfg.RPCTLSKeyPath == "" {
		t.Fatalf("expected public rpc with tls identity to load, cfg=%+v err=%v", cfg, err)
	}
}

func TestValidateStartupKeyCustodyRejectsPlaintextValidatorKeyOnPublicListener(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	runtimeConfig := startRuntimeConfig{
		RequireNetworkSafety: true,
		RPCEnabled:           true,
		RPCAddress:           "0.0.0.0:26657",
		RPCAdminToken:        "secret",
		P2PEnabled:           false,
	}
	if err := validateStartupKeyCustody(inputs, runtimeConfig); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected plaintext local validator key to fail public custody check, got %v", err)
	}
}

func TestNetworkRuntimeDefaultsDoNotBindAdvertisedAddress(t *testing.T) {
	inputs := startInputs{
		Config: vexonode.Config{
			Chain:       config.Default("vexo-test"),
			ValidatorID: "alice",
		},
		Genesis: vexonode.Genesis{
			Validators: []validator.Validator{
				{
					ID: "alice",
					Metadata: map[string]string{
						"p2p_address": "public-validator.example.com:26656",
						"rpc_address": "public-rpc.example.com:26657",
					},
				},
			},
		},
	}
	runtimeConfig := applyNetworkRuntimeDefaults(inputs, startRuntimeConfig{
		RPCAddress:       defaultRPCAddress,
		P2PListenAddress: defaultP2PAddress,
	})
	if runtimeConfig.RPCAddress != defaultRPCAddress || runtimeConfig.P2PListenAddress != defaultP2PAddress {
		t.Fatalf("advertised metadata must not become listen addresses: %+v", runtimeConfig)
	}
}

func TestNetworkRuntimeDefaultsUseGenesisNodeIDPeers(t *testing.T) {
	inputs := startInputs{
		Config: vexonode.Config{
			Chain:       config.Default("vexo-test"),
			ValidatorID: "validator-1",
		},
		Plan: startPlanDocument{ConfigPath: filepath.Join(t.TempDir(), configFileName)},
		Genesis: vexonode.Genesis{
			Validators: []validator.Validator{
				{
					ID: "validator-1",
					Metadata: map[string]string{
						"node_id":     "node-1",
						"p2p_address": "validator-1.example.com:26656",
					},
				},
				{
					ID: "validator-2",
					Metadata: map[string]string{
						"node_id":     "node-2",
						"p2p_address": "validator-2.example.com:26656",
					},
				},
			},
		},
	}
	runtimeConfig := applyNetworkRuntimeDefaults(inputs, startRuntimeConfig{P2PNodeID: "node-1"})
	if runtimeConfig.P2PPeers["node-2"] != "validator-2.example.com:26656" {
		t.Fatalf("expected peer map to use genesis node IDs, got %+v", runtimeConfig.P2PPeers)
	}
	if _, found := runtimeConfig.P2PPeers["validator-2"]; found {
		t.Fatalf("validator IDs must not be used when metadata node_id is present: %+v", runtimeConfig.P2PPeers)
	}
}

func TestLoadGenesisRejectsInvalidSchemaAndBase64(t *testing.T) {
	path := filepath.Join(t.TempDir(), genesisFileName)
	document := defaultGenesisDocument("vexo-test", "alice")
	document.SchemaVersion = "v0"
	writeTestJSON(t, path, document)
	if _, err := loadGenesis(path); err == nil {
		t.Fatal("expected invalid genesis schema")
	}

	document = defaultGenesisDocument("vexo-test", "alice")
	document.Validators[0].PublicKey = "not-base64"
	writeTestJSON(t, path, document)
	if _, err := loadGenesis(path); err == nil {
		t.Fatal("expected invalid public key base64")
	}

	document = defaultGenesisDocument("vexo-test", "alice")
	document.AppState = map[string]string{"bank": "not-base64"}
	writeTestJSON(t, path, document)
	if _, err := loadGenesis(path); err == nil {
		t.Fatal("expected invalid app state base64")
	}
}

func TestResolveConfigAndGenesisPaths(t *testing.T) {
	if got := resolveConfigPath("", "custom.json"); got != "custom.json" {
		t.Fatalf("expected explicit config path, got %q", got)
	}
	if got := resolveGenesisPath("", "custom-genesis.json"); got != "custom-genesis.json" {
		t.Fatalf("expected explicit genesis path, got %q", got)
	}
	if got := resolveConfigPath("/tmp/vexo", ""); got != filepath.Join("/tmp/vexo", configFileName) {
		t.Fatalf("unexpected config path: %q", got)
	}
	if got := resolveGenesisPath("/tmp/vexo", ""); got != filepath.Join("/tmp/vexo", genesisFileName) {
		t.Fatalf("unexpected genesis path: %q", got)
	}
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
}

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

	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestRunInitWritesConfigAndGenesis(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer

	if err := runInit(&buffer, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}

	output := buffer.String()
	if !strings.Contains(output, "initialized vexo node") || !strings.Contains(output, filepath.Join(home, configFileName)) {
		t.Fatalf("unexpected init output:\n%s", output)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
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
	if _, err := os.Stat(filepath.Join(home, moduleConfigFileName)); err != nil {
		t.Fatalf("expected module config file: %v", err)
	}
	configBytes, err := os.ReadFile(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configBytes), `"Application"`) ||
		strings.Contains(string(configBytes), `"Execution"`) ||
		strings.Contains(string(configBytes), `"Governance"`) {
		t.Fatalf("expected module settings to be split out of config.json:\n%s", configBytes)
	}
	moduleDocument, err := readModuleConfigDocument(filepath.Join(home, moduleConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(moduleDocument.Application.Modules) != 3 || moduleDocument.Execution.MaxGas == 0 || moduleDocument.Governance.Timelock == 0 {
		t.Fatalf("unexpected module config: %+v", moduleDocument)
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
		validatorInfo := genesis.Validators[index-1]
		if len(validatorInfo.PublicKey) == 0 || validatorInfo.Metadata["p2p_address"] != networkP2PAddress(index) || validatorInfo.Metadata["rpc_address"] != networkRPCAddress(index) {
			t.Fatalf("unexpected validator metadata: %+v", validatorInfo)
		}
		if _, err := loadStartInputs(nodeHome, "", "", "", false); err != nil {
			t.Fatalf("expected start inputs for %s: %v", validatorID, err)
		}
		if _, err := os.Stat(filepath.Join(nodeHome, moduleConfigFileName)); err != nil {
			t.Fatalf("expected network module config for %s: %v", validatorID, err)
		}
	}

	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "4"}); err == nil {
		t.Fatal("expected network init to reject existing files")
	}
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validators", "4", "--overwrite"}); err != nil {
		t.Fatal(err)
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
		Application:   config.ApplicationConfig{Modules: []string{"bank"}},
		Execution:     config.ExecutionConfig{RequireSigned: true, RequireNonce: true, MinFee: 7, MinGas: 3, MaxGas: 99, FeeCollector: "collector"},
		Governance:    governance.TallyPolicy{QuorumPower: 2, YesThresholdPower: 2, VotingPeriod: 9, Timelock: 4, VetoPower: 1},
	}
	writeTestJSON(t, filepath.Join(home, moduleConfigFileName), moduleDocument)

	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chain.Application.Modules) != 1 || cfg.Chain.Application.Modules[0] != "bank" {
		t.Fatalf("expected module config override, got %+v", cfg.Chain.Application)
	}
	if !cfg.Chain.Execution.RequireSigned || cfg.Chain.Execution.MinFee != 7 || cfg.Chain.Execution.FeeCollector != "collector" {
		t.Fatalf("expected execution config override, got %+v", cfg.Chain.Execution)
	}
	if cfg.Chain.Governance.Timelock != 4 {
		t.Fatalf("expected governance config override, got %+v", cfg.Chain.Governance)
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
	if len(cfg.Chain.Application.Modules) != 3 || cfg.Chain.Execution.MaxGas == 0 || cfg.Chain.Governance.Timelock == 0 {
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
	if err := os.MkdirAll(filepath.Join(home, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(home, "modules", "custom.json"), defaultModuleConfigDocument("vexo-test"))
	writeTestJSON(t, filepath.Join(home, configFileName), document)

	var output bytes.Buffer
	if err := runConfig(&output, []string{"paths", "--home", home}); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(home, "modules", "custom.json")
	if !strings.Contains(output.String(), "module_config: "+expected) {
		t.Fatalf("expected custom module config path %q in output:\n%s", expected, output.String())
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
	document := defaultConfigDocument("vexo-test", t.TempDir(), "alice")
	if document.Runtime.Log.CommitEvents == nil || !*document.Runtime.Log.CommitEvents {
		t.Fatalf("expected commit event logging enabled by default: %+v", document.Runtime.Log)
	}
	if document.Runtime.Log.PeerEvents == nil || !*document.Runtime.Log.PeerEvents {
		t.Fatalf("expected peer event logging enabled by default: %+v", document.Runtime.Log)
	}
}

func TestDefaultConfigWritesTendermintStyleConsensusTimeouts(t *testing.T) {
	document := defaultConfigDocument("vexo-test", t.TempDir(), "alice")
	consensus := document.Runtime.Consensus
	if consensus.TimeoutPropose != "3s" ||
		consensus.TimeoutPrevote != "1s" ||
		consensus.TimeoutPrecommit != "1s" ||
		consensus.TimeoutCommit != "1s" {
		t.Fatalf("unexpected consensus timeouts: %+v", consensus)
	}
	if consensus.CreateEmptyBlocks {
		t.Fatalf("expected empty block creation disabled by default: %+v", consensus)
	}
}

func TestLoadStartRuntimeConfigAllowsDisablingOperationalEventLogs(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	document.Runtime.Log.CommitEvents = boolPtr(false)
	document.Runtime.Log.PeerEvents = boolPtr(false)
	writeTestJSON(t, path, document)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogCommitEvents || cfg.LogPeerEvents {
		t.Fatalf("expected operational event logs disabled, got commit=%t peer=%t", cfg.LogCommitEvents, cfg.LogPeerEvents)
	}
}

func TestLoadStartRuntimeConfigParsesConsensusTimeoutsAndEmptyBlocks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, configFileName)
	document := defaultConfigDocument("vexo-test", filepath.Join(home, "data"), "alice")
	document.Runtime.Consensus.TimeoutPropose = "4s"
	document.Runtime.Consensus.TimeoutPrevote = "1500ms"
	document.Runtime.Consensus.TimeoutPrecommit = "2s"
	document.Runtime.Consensus.TimeoutCommit = "250ms"
	document.Runtime.Consensus.CreateEmptyBlocks = true
	writeTestJSON(t, path, document)

	cfg, err := loadStartRuntimeConfig(home, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConsensusLoop.TimeoutPropose != 4*time.Second ||
		cfg.ConsensusLoop.TimeoutPrevote != 1500*time.Millisecond ||
		cfg.ConsensusLoop.TimeoutPrecommit != 2*time.Second ||
		cfg.ConsensusLoop.TimeoutCommit != 250*time.Millisecond ||
		!cfg.ConsensusLoop.CreateEmptyBlocks {
		t.Fatalf("unexpected consensus runtime config: %+v", cfg.ConsensusLoop)
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

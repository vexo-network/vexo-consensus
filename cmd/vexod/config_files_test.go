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
	if len(genesis.Validators) != 1 || genesis.Validators[0].ID != "alice" || genesis.Governance["alice"] != 1 {
		t.Fatalf("unexpected loaded genesis: %+v", genesis)
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

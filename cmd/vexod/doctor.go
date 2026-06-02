package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/store"
)

type doctorDocument struct {
	OK            bool                         `json:"ok"`
	ConfigPath    string                       `json:"config_path"`
	GenesisPath   string                       `json:"genesis_path"`
	KeyPath       string                       `json:"key_path"`
	DataDir       string                       `json:"data_dir"`
	Checks        []doctorCheckDocument        `json:"checks"`
	RecoverResult *doctorRecoverResultDocument `json:"recover_result,omitempty"`
}

type doctorCheckDocument struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type doctorRecoverResultDocument struct {
	BlockIndexKeys   uint64 `json:"block_index_keys"`
	EvidenceKeys     uint64 `json:"evidence_keys"`
	EarliestHeight   uint64 `json:"earliest_height"`
	LatestHeight     uint64 `json:"latest_height"`
	RecoveredIndexes uint64 `json:"recovered_indexes"`
}

func runDoctor(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	repairIndexes := flags.Bool("repair-indexes", false, "rebuild LevelDB block and evidence indexes")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := buildDoctorDocument(*home, *configPath, *genesisPath, *keyPath, *repairIndexes)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	writeDoctorDocument(writer, document)
	if !document.OK {
		return fmt.Errorf("doctor found %d failed checks", failedDoctorChecks(document.Checks))
	}
	return nil
}

func buildDoctorDocument(home string, configPath string, genesisPath string, keyPath string, repairIndexes bool) doctorDocument {
	resolvedConfigPath := resolveConfigPath(home, configPath)
	resolvedGenesisPath := resolveGenesisPath(home, genesisPath)
	resolvedKeyPath := resolveKeyPath(home, keyPath)
	document := doctorDocument{
		OK:          true,
		ConfigPath:  resolvedConfigPath,
		GenesisPath: resolvedGenesisPath,
		KeyPath:     resolvedKeyPath,
	}

	cfg, err := loadNodeConfig(resolvedConfigPath)
	document.addCheck("config", err)
	if err == nil {
		document.DataDir = cfg.StoreDir()
		genesis, genesisErr := loadGenesis(resolvedGenesisPath)
		if genesisErr == nil {
			genesisErr = genesis.Validate(cfg.Chain.ChainID)
		}
		document.addCheck("genesis", genesisErr)
	} else {
		document.addCheck("genesis", err)
	}

	_, keyErr := vexocrypto.LoadKeyDocument(resolvedKeyPath)
	document.addCheck("key", keyErr)

	if err != nil {
		return document
	}
	storage, storeErr := store.OpenLevelDB(cfg.StoreDir())
	document.addCheck("store_open", storeErr)
	if storeErr != nil {
		return document
	}
	defer storage.Close()

	if repairIndexes {
		result, recoverErr := storage.RecoverIndexes(context.Background())
		document.addCheck("recover_indexes", recoverErr)
		if recoverErr == nil {
			document.RecoverResult = &doctorRecoverResultDocument{
				BlockIndexKeys:   result.BlockIndexKeys,
				EvidenceKeys:     result.EvidenceKeys,
				EarliestHeight:   uint64(result.EarliestHeight),
				LatestHeight:     uint64(result.LatestHeight),
				RecoveredIndexes: result.RecoveredIndexes,
			}
		}
	}

	_, latestStateErr := storage.LatestState(context.Background())
	document.addCheck("latest_state", latestStateErr)
	_, blockIndexErr := storage.BlockIndex(context.Background())
	document.addCheck("block_index", blockIndexErr)
	_, snapshotErr := buildSnapshotDocument(storage, cfg.Chain.ChainID, snapshotNamespaces(cfg.Chain.Application.Modules))
	document.addCheck("snapshot", snapshotErr)
	return document
}

func (document *doctorDocument) addCheck(name string, err error) {
	check := doctorCheckDocument{Name: name, OK: err == nil}
	if err != nil {
		check.Error = err.Error()
		document.OK = false
	}
	document.Checks = append(document.Checks, check)
}

func writeDoctorDocument(writer io.Writer, document doctorDocument) {
	status := "ok"
	if !document.OK {
		status = "failed"
	}
	fmt.Fprintf(writer, "doctor %s\n", status)
	fmt.Fprintf(writer, "config: %s\n", document.ConfigPath)
	fmt.Fprintf(writer, "genesis: %s\n", document.GenesisPath)
	fmt.Fprintf(writer, "key: %s\n", document.KeyPath)
	if document.DataDir != "" {
		fmt.Fprintf(writer, "data_dir: %s\n", document.DataDir)
	}
	for _, check := range document.Checks {
		checkStatus := "ok"
		if !check.OK {
			checkStatus = "failed"
		}
		if check.Error == "" {
			fmt.Fprintf(writer, "check %s: %s\n", check.Name, checkStatus)
		} else {
			fmt.Fprintf(writer, "check %s: %s (%s)\n", check.Name, checkStatus, check.Error)
		}
	}
	if document.RecoverResult != nil {
		fmt.Fprintf(writer, "recovered_indexes: %d\n", document.RecoverResult.RecoveredIndexes)
		fmt.Fprintf(writer, "block_index_keys: %d\n", document.RecoverResult.BlockIndexKeys)
		fmt.Fprintf(writer, "evidence_keys: %d\n", document.RecoverResult.EvidenceKeys)
	}
}

func failedDoctorChecks(checks []doctorCheckDocument) int {
	count := 0
	for _, check := range checks {
		if !check.OK {
			count++
		}
	}
	return count
}

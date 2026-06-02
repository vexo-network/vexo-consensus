package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

type startPlanDocument struct {
	ChainID     string `json:"chain_id"`
	ValidatorID string `json:"validator_id,omitempty"`
	DataDir     string `json:"data_dir"`
	ConfigPath  string `json:"config_path"`
	GenesisPath string `json:"genesis_path"`
	KeyPath     string `json:"key_path"`
	ValidatorN  int    `json:"validator_count"`
	KeyType     string `json:"key_type,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
	DryRun      bool   `json:"dry_run"`
}

func runStart(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	dryRun := flags.Bool("dry-run", false, "validate startup inputs without running a node")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan, err := loadStartPlan(*home, *configPath, *genesisPath, *keyPath, *dryRun)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	if !plan.DryRun {
		fmt.Fprintf(writer, "startup inputs valid\n")
		fmt.Fprintf(writer, "start execution is not enabled yet; rerun with --dry-run for readiness checks\n")
	} else {
		fmt.Fprintf(writer, "startup dry-run valid\n")
	}
	fmt.Fprintf(writer, "chain_id: %s\n", plan.ChainID)
	fmt.Fprintf(writer, "validator_id: %s\n", plan.ValidatorID)
	fmt.Fprintf(writer, "validators: %d\n", plan.ValidatorN)
	fmt.Fprintf(writer, "data_dir: %s\n", plan.DataDir)
	fmt.Fprintf(writer, "key_type: %s\n", plan.KeyType)
	fmt.Fprintf(writer, "public_key: %s\n", plan.PublicKey)
	return nil
}

func loadStartPlan(home string, configPath string, genesisPath string, keyPath string, dryRun bool) (startPlanDocument, error) {
	resolvedConfigPath := resolveConfigPath(home, configPath)
	resolvedGenesisPath := resolveGenesisPath(home, genesisPath)
	resolvedKeyPath := resolveKeyPath(home, keyPath)
	cfg, err := loadNodeConfig(resolvedConfigPath)
	if err != nil {
		return startPlanDocument{}, err
	}
	genesis, err := loadGenesis(resolvedGenesisPath)
	if err != nil {
		return startPlanDocument{}, err
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		return startPlanDocument{}, err
	}
	keyDocument, err := vexocrypto.LoadKeyDocument(resolvedKeyPath)
	if err != nil {
		return startPlanDocument{}, err
	}
	if _, err := keyDocument.Ed25519Signer(); err != nil {
		return startPlanDocument{}, err
	}
	return startPlanDocument{
		ChainID:     cfg.Chain.ChainID,
		ValidatorID: string(cfg.ValidatorID),
		DataDir:     cfg.DataDir,
		ConfigPath:  resolvedConfigPath,
		GenesisPath: resolvedGenesisPath,
		KeyPath:     resolvedKeyPath,
		ValidatorN:  len(genesis.Validators),
		KeyType:     keyDocument.Type,
		PublicKey:   keyDocument.PublicKey,
		DryRun:      dryRun,
	}, nil
}

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/signal"

	appmodules "github.com/vexo-network/vexo-consensus/app/modules"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/types"
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

type startInputs struct {
	Config  vexonode.Config
	Genesis vexonode.Genesis
	Signer  vexocrypto.Ed25519Signer
	Plan    startPlanDocument
}

func runStart(writer io.Writer, args []string) error {
	return runStartWithContext(context.Background(), writer, args)
}

func runStartWithContext(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	dryRun := flags.Bool("dry-run", false, "validate startup inputs without running a node")
	run := flags.Bool("run", false, "start the node and block until context cancellation")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	inputs, err := loadStartInputs(*home, *configPath, *genesisPath, *keyPath, *dryRun)
	if err != nil {
		return err
	}
	plan := inputs.Plan
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	if *run {
		runCtx, stopSignals := signal.NotifyContext(ctx, shutdownSignals()...)
		defer stopSignals()
		return runStartNode(runCtx, writer, inputs)
	}
	if !plan.DryRun {
		if _, err := buildStartNode(inputs); err != nil {
			return err
		}
		fmt.Fprintf(writer, "startup inputs valid\n")
		fmt.Fprintf(writer, "validator signer loaded\n")
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

func runStartNode(ctx context.Context, writer io.Writer, inputs startInputs) error {
	node, err := buildStartNode(inputs)
	if err != nil {
		return err
	}
	if err := node.Start(ctx); err != nil {
		return err
	}
	fmt.Fprintf(writer, "node running\n")
	fmt.Fprintf(writer, "chain_id: %s\n", inputs.Plan.ChainID)
	fmt.Fprintf(writer, "validator_id: %s\n", inputs.Plan.ValidatorID)
	fmt.Fprintf(writer, "data_dir: %s\n", inputs.Plan.DataDir)
	<-ctx.Done()
	fmt.Fprintf(writer, "shutdown requested\n")
	if err := node.Stop(context.Background()); err != nil {
		return err
	}
	fmt.Fprintf(writer, "node stopped\n")
	return nil
}

func loadStartPlan(home string, configPath string, genesisPath string, keyPath string, dryRun bool) (startPlanDocument, error) {
	inputs, err := loadStartInputs(home, configPath, genesisPath, keyPath, dryRun)
	if err != nil {
		return startPlanDocument{}, err
	}
	return inputs.Plan, nil
}

func loadStartInputs(home string, configPath string, genesisPath string, keyPath string, dryRun bool) (startInputs, error) {
	resolvedConfigPath := resolveConfigPath(home, configPath)
	resolvedGenesisPath := resolveGenesisPath(home, genesisPath)
	resolvedKeyPath := resolveKeyPath(home, keyPath)
	cfg, err := loadNodeConfig(resolvedConfigPath)
	if err != nil {
		return startInputs{}, err
	}
	genesis, err := loadGenesis(resolvedGenesisPath)
	if err != nil {
		return startInputs{}, err
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		return startInputs{}, err
	}
	keyDocument, err := vexocrypto.LoadKeyDocument(resolvedKeyPath)
	if err != nil {
		return startInputs{}, err
	}
	signer, err := keyDocument.Ed25519Signer()
	if err != nil {
		return startInputs{}, err
	}
	genesis = withLocalValidatorPublicKey(genesis, cfg.ValidatorID, signer.PublicKey())
	plan := startPlanDocument{
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
	}
	return startInputs{
		Config:  cfg,
		Genesis: genesis,
		Signer:  signer,
		Plan:    plan,
	}, nil
}

func buildStartNode(inputs startInputs) (*vexonode.Node, error) {
	application, err := appmodules.NewRuntime(inputs.Config.Chain.ChainID, inputs.Config.Chain.Application)
	if err != nil {
		return nil, err
	}
	node, err := vexonode.New(inputs.Config, inputs.Genesis, application)
	if err != nil {
		return nil, err
	}
	return node.WithSigner(inputs.Signer), nil
}

func withLocalValidatorPublicKey(genesis vexonode.Genesis, validatorID types.ValidatorID, publicKey types.PublicKey) vexonode.Genesis {
	for index := range genesis.Validators {
		if genesis.Validators[index].ID == validatorID && len(genesis.Validators[index].PublicKey) == 0 {
			genesis.Validators[index].PublicKey = append(types.PublicKey(nil), publicKey...)
		}
	}
	return genesis
}

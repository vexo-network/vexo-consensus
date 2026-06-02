package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

func runUpgrade(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("upgrade subcommand is required")
	}
	switch args[0] {
	case "plan":
		return runUpgradePlan(writer, args[1:])
	case "apply":
		return runUpgradeApply(writer, args[1:])
	default:
		return fmt.Errorf("unknown upgrade subcommand %q", args[0])
	}
}

func runUpgradePlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("upgrade plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "vexo-upgrade", "upgrade name")
	height := flags.Uint64("height", 1, "governance-approved upgrade height")
	binaryVersion := flags.String("binary-version", version, "target binary version")
	configFrom := flags.Uint64("config-from", 1, "current config schema version")
	configTo := flags.Uint64("config-to", 1, "target config schema version")
	storeFrom := flags.Uint64("store-from", 1, "current store schema version")
	storeTo := flags.Uint64("store-to", 1, "target store schema version")
	appFrom := flags.Uint64("app-from", 1, "current app module state schema version")
	appTo := flags.Uint64("app-to", 1, "target app module state schema version")
	governanceProposal := flags.String("proposal", "", "governance proposal identifier")
	rollbackBinary := flags.String("rollback-binary", "", "rollback binary version or artifact")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan := upgrade.Plan{
		Name:               *name,
		Height:             types.Height(*height),
		BinaryVersion:      *binaryVersion,
		ConfigSchemaFrom:   *configFrom,
		ConfigSchemaTo:     *configTo,
		StoreSchemaFrom:    *storeFrom,
		StoreSchemaTo:      *storeTo,
		AppStateSchemaFrom: *appFrom,
		AppStateSchemaTo:   *appTo,
		GovernanceProposal: *governanceProposal,
		RollbackBinary:     *rollbackBinary,
	}
	if err := upgrade.ValidatePlan(plan); err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	fmt.Fprintf(writer, "upgrade plan\n")
	fmt.Fprintf(writer, "name: %s\n", plan.Name)
	fmt.Fprintf(writer, "height: %d\n", plan.Height)
	fmt.Fprintf(writer, "binary_version: %s\n", plan.BinaryVersion)
	fmt.Fprintf(writer, "config_schema: %d -> %d\n", plan.ConfigSchemaFrom, plan.ConfigSchemaTo)
	fmt.Fprintf(writer, "store_schema: %d -> %d\n", plan.StoreSchemaFrom, plan.StoreSchemaTo)
	fmt.Fprintf(writer, "app_state_schema: %d -> %d\n", plan.AppStateSchemaFrom, plan.AppStateSchemaTo)
	if plan.GovernanceProposal != "" {
		fmt.Fprintf(writer, "governance_proposal: %s\n", plan.GovernanceProposal)
	}
	if plan.RollbackBinary != "" {
		fmt.Fprintf(writer, "rollback_binary: %s\n", plan.RollbackBinary)
	}
	return nil
}

func runUpgradeApply(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("upgrade apply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	planFile := flags.String("plan-file", "", "upgrade plan JSON file")
	recordFile := flags.String("record-file", filepath.Join(".vexo", "upgrade-records.json"), "durable upgrade execution record file")
	currentHeight := flags.Uint64("height", 0, "current chain height")
	binaryVersion := flags.String("binary-version", version, "current binary version")
	configVersion := flags.Uint64("config-version", 1, "current config schema version")
	storeVersion := flags.Uint64("store-version", 1, "current store schema version")
	appVersion := flags.Uint64("app-version", 1, "current app state schema version")
	allowEmptyMigrations := flags.Bool("allow-empty-migrations", false, "register no-op migrations for declared schema version jumps")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *planFile == "" {
		return errors.New("plan-file is required")
	}
	plan, err := readUpgradePlanFile(*planFile)
	if err != nil {
		return err
	}
	registry := upgrade.NewRegistry()
	if *allowEmptyMigrations {
		registerNoopUpgradeMigrations(registry, plan)
	}
	state := upgrade.State{
		Height:              types.Height(*currentHeight),
		BinaryVersion:       *binaryVersion,
		ConfigSchemaVersion: *configVersion,
		StoreSchemaVersion:  *storeVersion,
		AppStateVersion:     *appVersion,
	}
	executor := upgrade.NewExecutor(registry, upgrade.JSONFileRecorder{Path: *recordFile})
	record, err := executor.Execute(context.Background(), state, plan)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		encodeErr := encoder.Encode(record)
		if err != nil {
			return err
		}
		return encodeErr
	}
	fmt.Fprintf(writer, "upgrade apply %s\n", record.Status)
	fmt.Fprintf(writer, "name: %s\n", record.Plan.Name)
	fmt.Fprintf(writer, "height: %d\n", record.Result.Height)
	fmt.Fprintf(writer, "binary_version: %s -> %s\n", record.Before.BinaryVersion, record.Result.BinaryVersion)
	fmt.Fprintf(writer, "config_schema: %d -> %d\n", record.Before.ConfigSchemaVersion, record.Result.ConfigSchemaVersion)
	fmt.Fprintf(writer, "store_schema: %d -> %d\n", record.Before.StoreSchemaVersion, record.Result.StoreSchemaVersion)
	fmt.Fprintf(writer, "app_state_schema: %d -> %d\n", record.Before.AppStateVersion, record.Result.AppStateVersion)
	if record.Error != "" {
		fmt.Fprintf(writer, "error: %s\n", record.Error)
	}
	if err != nil {
		return err
	}
	return nil
}

func readUpgradePlanFile(path string) (upgrade.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return upgrade.Plan{}, err
	}
	var plan upgrade.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return upgrade.Plan{}, err
	}
	return plan, upgrade.ValidatePlan(plan)
}

func registerNoopUpgradeMigrations(registry *upgrade.Registry, plan upgrade.Plan) {
	registerNoopMigrationPath(registry.RegisterConfig, plan.ConfigSchemaFrom, plan.ConfigSchemaTo)
	registerNoopMigrationPath(registry.RegisterStore, plan.StoreSchemaFrom, plan.StoreSchemaTo)
	registerNoopMigrationPath(registry.RegisterAppState, plan.AppStateSchemaFrom, plan.AppStateSchemaTo)
}

func registerNoopMigrationPath(register func(upgrade.Migration), from uint64, to uint64) {
	for version := from; version < to; version++ {
		register(upgrade.Migration{From: version, To: version + 1})
	}
}

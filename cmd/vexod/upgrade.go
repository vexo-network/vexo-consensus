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
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

func runUpgrade(writer io.Writer, args []string) error {
	return runUpgradeWithContext(context.Background(), writer, args)
}

func runUpgradeWithContext(ctx context.Context, writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("upgrade subcommand is required")
	}
	switch args[0] {
	case "plan":
		return runUpgradePlan(writer, args[1:])
	case "update":
		return runUpgradeUpdate(writer, args[1:])
	case "apply":
		return runUpgradeApplyWithContext(ctx, writer, args[1:])
	case "rollback-plan":
		return runUpgradeRollbackPlanWithContext(ctx, writer, args[1:])
	default:
		return fmt.Errorf("unknown upgrade subcommand %q", args[0])
	}
}

type upgradeRollbackPlanDocument struct {
	SchemaVersion  string                     `json:"schema_version"`
	PlanName       string                     `json:"plan_name"`
	UpgradeHeight  types.Height               `json:"upgrade_height"`
	CurrentStatus  upgrade.ExecutionStatus    `json:"current_status"`
	RollbackBinary string                     `json:"rollback_binary"`
	LastSafeHeight types.Height               `json:"last_safe_height"`
	SnapshotPath   string                     `json:"snapshot_path,omitempty"`
	RecordFile     string                     `json:"record_file,omitempty"`
	Checks         []upgradeRollbackPlanCheck `json:"checks"`
	Steps          []string                   `json:"steps"`
	Warnings       []string                   `json:"warnings,omitempty"`
}

type upgradeRollbackPlanCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func runUpgradeUpdate(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("upgrade update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "upgrade name")
	versionValue := flags.String("version", "", "target binary version")
	height := flags.Uint64("height", 1, "governance-approved upgrade height")
	configFrom := flags.Uint64("config-from", 1, "current config schema version")
	configTo := flags.Uint64("config-to", 1, "target config schema version")
	storeFrom := flags.Uint64("store-from", 1, "current store schema version")
	storeTo := flags.Uint64("store-to", 1, "target store schema version")
	appFrom := flags.Uint64("app-from", 1, "current app module state schema version")
	appTo := flags.Uint64("app-to", 1, "target app module state schema version")
	governanceProposal := flags.String("proposal", "", "governance proposal identifier")
	rollbackBinary := flags.String("rollback-binary", "", "rollback binary version or artifact")
	allowNoopMigrations := flags.Bool("allow-noop-migrations", true, "explicitly mark this plan as allowing no-op schema migrations")
	planFile := flags.String("plan-file", "", "write the generated plan to this file")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*versionValue) == "" {
		return errors.New("version is required")
	}
	plan := upgrade.Plan{
		Name:                strings.TrimSpace(*name),
		Height:              types.Height(*height),
		BinaryVersion:       strings.TrimSpace(*versionValue),
		ConfigSchemaFrom:    *configFrom,
		ConfigSchemaTo:      *configTo,
		StoreSchemaFrom:     *storeFrom,
		StoreSchemaTo:       *storeTo,
		AppStateSchemaFrom:  *appFrom,
		AppStateSchemaTo:    *appTo,
		GovernanceProposal:  strings.TrimSpace(*governanceProposal),
		RollbackBinary:      strings.TrimSpace(*rollbackBinary),
		AllowNoopMigrations: *allowNoopMigrations,
	}
	if plan.Name == "" {
		plan.Name = plan.BinaryVersion
	}
	if err := upgrade.ValidatePlan(plan); err != nil {
		return err
	}
	if *planFile != "" {
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*planFile, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	fmt.Fprintf(writer, "upgrade update\n")
	fmt.Fprintf(writer, "name: %s\n", plan.Name)
	fmt.Fprintf(writer, "version: %s\n", plan.BinaryVersion)
	fmt.Fprintf(writer, "height: %d\n", plan.Height)
	fmt.Fprintf(writer, "config_schema: %d -> %d\n", plan.ConfigSchemaFrom, plan.ConfigSchemaTo)
	fmt.Fprintf(writer, "store_schema: %d -> %d\n", plan.StoreSchemaFrom, plan.StoreSchemaTo)
	fmt.Fprintf(writer, "app_state_schema: %d -> %d\n", plan.AppStateSchemaFrom, plan.AppStateSchemaTo)
	if plan.GovernanceProposal != "" {
		fmt.Fprintf(writer, "governance_proposal: %s\n", plan.GovernanceProposal)
	}
	if plan.RollbackBinary != "" {
		fmt.Fprintf(writer, "rollback_binary: %s\n", plan.RollbackBinary)
	}
	if *planFile != "" {
		fmt.Fprintf(writer, "plan_file: %s\n", *planFile)
	}
	fmt.Fprintf(writer, "allow_noop_migrations: %t\n", plan.AllowNoopMigrations)
	return nil
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
	allowNoopMigrations := flags.Bool("allow-noop-migrations", false, "explicitly mark this plan as allowing no-op schema migrations")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	plan := upgrade.Plan{
		Name:                *name,
		Height:              types.Height(*height),
		BinaryVersion:       *binaryVersion,
		ConfigSchemaFrom:    *configFrom,
		ConfigSchemaTo:      *configTo,
		StoreSchemaFrom:     *storeFrom,
		StoreSchemaTo:       *storeTo,
		AppStateSchemaFrom:  *appFrom,
		AppStateSchemaTo:    *appTo,
		GovernanceProposal:  *governanceProposal,
		RollbackBinary:      *rollbackBinary,
		AllowNoopMigrations: *allowNoopMigrations,
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
	fmt.Fprintf(writer, "allow_noop_migrations: %t\n", plan.AllowNoopMigrations)
	return nil
}

func runUpgradeApply(writer io.Writer, args []string) error {
	return runUpgradeApplyWithContext(context.Background(), writer, args)
}

func runUpgradeApplyWithContext(ctx context.Context, writer io.Writer, args []string) error {
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
	if err := ctx.Err(); err != nil {
		return err
	}
	plan, err := readUpgradePlanFile(*planFile)
	if err != nil {
		return err
	}
	registry := upgrade.NewRegistry()
	if *allowEmptyMigrations {
		if !plan.AllowNoopMigrations {
			return errors.New("plan must set allow_noop_migrations=true before --allow-empty-migrations can be used")
		}
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
	record, err := executor.Execute(ctx, state, plan)
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

func runUpgradeRollbackPlan(writer io.Writer, args []string) error {
	return runUpgradeRollbackPlanWithContext(context.Background(), writer, args)
}

func runUpgradeRollbackPlanWithContext(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("upgrade rollback-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	planFile := flags.String("plan-file", "", "upgrade plan JSON file")
	recordFile := flags.String("record-file", filepath.Join(".vexo", "upgrade-records.json"), "durable upgrade execution record file")
	lastSafeHeight := flags.Uint64("last-safe-height", 0, "last height known safe for rollback")
	snapshotPath := flags.String("snapshot", "", "snapshot path to restore during rollback")
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
	document := buildUpgradeRollbackPlanDocument(plan, *recordFile, types.Height(*lastSafeHeight), *snapshotPath)
	if *recordFile != "" {
		record, found, err := upgrade.JSONFileRecorder{Path: *recordFile}.Load(ctx, plan.Name)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if found {
			document.CurrentStatus = record.Status
		}
	}
	document.Checks = append(document.Checks, upgradeRollbackPlanCheck{
		Name:    "rollback_binary",
		OK:      document.RollbackBinary != "",
		Message: "rollback binary or artifact must be declared in the upgrade plan",
	})
	document.Checks = append(document.Checks, upgradeRollbackPlanCheck{
		Name:    "last_safe_height",
		OK:      document.LastSafeHeight > 0 && document.LastSafeHeight < document.UpgradeHeight,
		Message: "last safe height must be greater than zero and lower than the upgrade height",
	})
	document.Checks = append(document.Checks, upgradeRollbackPlanCheck{
		Name:    "snapshot",
		OK:      document.SnapshotPath != "",
		Message: "snapshot path should be attached for restore drill evidence",
	})
	document.Warnings = upgradeRollbackPlanWarnings(document)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	writeUpgradeRollbackPlan(writer, document)
	return nil
}

func buildUpgradeRollbackPlanDocument(plan upgrade.Plan, recordFile string, lastSafeHeight types.Height, snapshotPath string) upgradeRollbackPlanDocument {
	return upgradeRollbackPlanDocument{
		SchemaVersion:  "v1",
		PlanName:       plan.Name,
		UpgradeHeight:  plan.Height,
		CurrentStatus:  upgrade.ExecutionPending,
		RollbackBinary: plan.RollbackBinary,
		LastSafeHeight: lastSafeHeight,
		SnapshotPath:   snapshotPath,
		RecordFile:     recordFile,
		Steps: []string{
			"halt validators and public traffic before attempting rollback",
			"confirm no conflicting finality exists above the last safe height",
			"restore state snapshot at or before the last safe height",
			"restart validators with the rollback binary and identical config/genesis inputs",
			"verify height growth, replay health, validator signing policy, and light-client finality proofs",
			"archive rollback evidence and keep the failed upgrade record for audit review",
		},
	}
}

func upgradeRollbackPlanWarnings(document upgradeRollbackPlanDocument) []string {
	warnings := make([]string, 0)
	for _, check := range document.Checks {
		if !check.OK {
			warnings = append(warnings, check.Message)
		}
	}
	if document.CurrentStatus == upgrade.ExecutionApplied {
		warnings = append(warnings, "upgrade record is already applied; rollback requires governance/operator emergency approval")
	}
	if document.CurrentStatus == upgrade.ExecutionRollbackRequired {
		warnings = append(warnings, "upgrade record is rollback_required; operators must not retry apply until rollback is completed")
	}
	return warnings
}

func writeUpgradeRollbackPlan(writer io.Writer, document upgradeRollbackPlanDocument) {
	fmt.Fprintf(writer, "upgrade rollback plan\n")
	fmt.Fprintf(writer, "name: %s\n", document.PlanName)
	fmt.Fprintf(writer, "upgrade_height: %d\n", document.UpgradeHeight)
	fmt.Fprintf(writer, "current_status: %s\n", document.CurrentStatus)
	fmt.Fprintf(writer, "rollback_binary: %s\n", document.RollbackBinary)
	fmt.Fprintf(writer, "last_safe_height: %d\n", document.LastSafeHeight)
	if document.SnapshotPath != "" {
		fmt.Fprintf(writer, "snapshot: %s\n", document.SnapshotPath)
	}
	if document.RecordFile != "" {
		fmt.Fprintf(writer, "record_file: %s\n", document.RecordFile)
	}
	fmt.Fprintf(writer, "checks:\n")
	for _, check := range document.Checks {
		fmt.Fprintf(writer, "- %s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	fmt.Fprintf(writer, "steps:\n")
	for index, step := range document.Steps {
		fmt.Fprintf(writer, "%d. %s\n", index+1, step)
	}
	if len(document.Warnings) > 0 {
		fmt.Fprintf(writer, "warnings:\n")
		for _, warning := range document.Warnings {
			fmt.Fprintf(writer, "- %s\n", warning)
		}
	}
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

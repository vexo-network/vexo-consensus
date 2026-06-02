package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

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

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/cmd/vexod/internal/commandset"
	"github.com/vexo-network/vexo-consensus/config"
	appmodules "github.com/vexo-network/vexo-consensus/modules"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := runCommand(os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func runCommand(stdout io.Writer, stderr io.Writer, args []string) error {
	if len(args) == 0 {
		writeStatus(stdout, config.Default("vexo-chain"))
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		writeHelp(stdout)
		return nil
	case "version", "--version":
		fmt.Fprintf(stdout, "vexod %s\ncommit: %s\nbuild_date: %s\n", version, commit, buildDate)
		return nil
	case "status":
		if len(args) > 1 && args[1] == "--json" {
			if err := writeStatusJSON(stdout, config.Default("vexo-chain")); err != nil {
				return writeCommandError(stderr, "status", err)
			}
			return nil
		}
		writeStatus(stdout, config.Default("vexo-chain"))
		return nil
	case "--json":
		if err := writeStatusJSON(stdout, config.Default("vexo-chain")); err != nil {
			return writeCommandError(stderr, "status", err)
		}
		return nil
	case "demo":
		if err := writeDemo(stdout); err != nil {
			return writeCommandError(stderr, "demo", err)
		}
		return nil
	case "store-demo":
		path := filepath.Join(os.TempDir(), "vexo-consensus-store-demo")
		if len(args) > 1 {
			path = args[1]
		}
		if err := writeStoreDemo(stdout, path); err != nil {
			return writeCommandError(stderr, "store-demo", err)
		}
		return nil
	default:
		if handled, err := coreCommands().Run(args[0], stdout, args[1:]); handled {
			if err != nil {
				return writeCommandError(stderr, args[0], err)
			}
			return nil
		}
		if handled, err := runModuleCommand(stdout, args); handled {
			if err != nil {
				return writeCommandError(stderr, args[0], err)
			}
			return nil
		}
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		fmt.Fprintf(stderr, "run `vexod help` for usage\n")
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func coreCommands() commandset.Registry {
	return commandset.New([]commandset.Command{
		{Name: "init", Description: "initialize config and genesis files", Handler: runInit},
		{Name: "validate", Description: "validate config and genesis files", Handler: runValidate},
		{Name: "config", Description: "audit, inspect, and generate config files", Handler: runConfig},
		{Name: "keys", Description: "manage local and remote validator keys", Handler: runKeys},
		{Name: "tx", Description: "build or parse canonical transaction payloads", Handler: runTx},
		{Name: "proof", Description: "build and verify state query proofs", Handler: runProof},
		{Name: "relayer", Description: "build, prove, and submit IBC relayer transactions", Handler: runRelayer},
		{Name: "start", Description: "validate files, prepare startup, or run a node", Handler: runStart},
		{Name: "network", Description: "manage node networks and local execution harnesses", Handler: runNetwork},
		{Name: "consensus", Description: "run consensus simulations and diagnostics", Handler: runConsensus},
		{Name: "snapshot", Description: "export, verify, fetch, sync, restore, or drill snapshots", Handler: runSnapshot},
		{Name: "slashing", Description: "plan evidence lifecycle and penalty operations", Handler: runSlashing},
		{Name: "doctor", Description: "inspect config, keys, store, snapshot, and recovery readiness", Handler: runDoctor},
		{Name: "ops", Description: "print thresholds, evaluate samples, or build incident reports", Handler: runOps},
		{Name: "upgrade", Description: "build, apply, and rollback-drill upgrade plans", Handler: runUpgrade},
		{Name: "release", Description: "package releases and evaluate release gates", Handler: runRelease},
	})
}

func writeCommandError(writer io.Writer, command string, err error) error {
	if errors.Is(err, errValidationFailed) {
		fmt.Fprintf(writer, "%v\n", err)
		return err
	}
	fmt.Fprintf(writer, "%s failed: %v\n", command, err)
	return err
}

func writeHelp(writer io.Writer) {
	fmt.Fprintf(writer, "vexod %s\n\n", version)
	fmt.Fprintf(writer, "Usage:\n")
	fmt.Fprintf(writer, "  vexod <command> [flags]\n\n")
	fmt.Fprintf(writer, "Commands:\n")
	for _, command := range coreCommands().Commands() {
		fmt.Fprintf(writer, "  %-15s %s\n", command.Name, command.Description)
	}
	fmt.Fprintf(writer, "  init validator  initialize a validator node home and key\n")
	fmt.Fprintf(writer, "  init archive    initialize a non-validator archive node home\n")
	fmt.Fprintf(writer, "  config audit    run deployment and production-readiness checks\n")
	fmt.Fprintf(writer, "  config audit-pack generate external security audit evidence checklist\n")
	fmt.Fprintf(writer, "  config deployment-template print recommended deployment parameters\n")
	fmt.Fprintf(writer, "  config paths    print resolved config, genesis, key, and data paths\n")
	fmt.Fprintf(writer, "  config show     print loaded chain config as JSON\n")
	fmt.Fprintf(writer, "  config tune     recommend launch-safe consensus, network, mempool, fee, and alert parameters\n")
	fmt.Fprintf(writer, "\nCommon Subcommands:\n")
	fmt.Fprintf(writer, "  keys gen        generate an Ed25519 or BLS validator key\n")
	fmt.Fprintf(writer, "  keys remote     register a remote KMS/HSM validator signer\n")
	fmt.Fprintf(writer, "  keys serve-remote serve a policy-enforced KMS/HSM signer endpoint\n")
	fmt.Fprintf(writer, "  keys verify-remote verify remote KMS/HSM challenge signing\n")
	fmt.Fprintf(writer, "  keys sign-tx    sign a raw transaction payload\n")
	fmt.Fprintf(writer, "  keys show       show validator public key\n")
	fmt.Fprintf(writer, "  proof query     build a state-root-bound query proof\n")
	fmt.Fprintf(writer, "  proof verify    verify a query proof envelope\n")
	fmt.Fprintf(writer, "  proof verify-ibc verify a proof against a trusted IBC client\n")
	fmt.Fprintf(writer, "  relayer packet-proof fetch an IBC packet proof from RPC\n")
	fmt.Fprintf(writer, "  relayer discover find IBC packets from indexed RPC events\n")
	fmt.Fprintf(writer, "  relayer packet-ack build or submit an IBC packet acknowledgement\n")
	fmt.Fprintf(writer, "  relayer loop poll packet proofs and submit relay transactions\n")
	fmt.Fprintf(writer, "  relayer run execute relayer jobs from a config file\n")
	fmt.Fprintf(writer, "  ibc tx client-update update a trusted IBC client height/root\n")
	fmt.Fprintf(writer, "  ibc tx connection-open-init start an IBC connection handshake\n")
	fmt.Fprintf(writer, "  ibc tx channel-open-init start an IBC channel handshake\n")
	fmt.Fprintf(writer, "  ibc tx packet-send build an IBC packet send transaction\n")
	fmt.Fprintf(writer, "  ibc tx packet-ack build an IBC packet acknowledgement transaction\n")
	fmt.Fprintf(writer, "  ibc tx packet-timeout build an IBC packet timeout transaction\n")
	fmt.Fprintf(writer, "  ibc packet send build an IBC packet scaffold\n")
	fmt.Fprintf(writer, "  status          print default node capability status\n")
	fmt.Fprintf(writer, "  demo            run an in-memory bank execution demo\n")
	fmt.Fprintf(writer, "  store-demo      run a LevelDB-backed storage demo\n")
	fmt.Fprintf(writer, "  version         print version\n")
	writeModuleHelp(writer)
}

func moduleCLICommands() []vexoapp.CLICommand {
	commands, err := appmodules.BuildCLICommands(config.Default("vexo-chain").Application)
	if err != nil {
		return nil
	}
	return commands
}

func runModuleCommand(writer io.Writer, args []string) (bool, error) {
	for _, command := range moduleCLICommands() {
		if command.Name != args[0] {
			continue
		}
		return true, command.Execute(writer, args[1:])
	}
	return false, nil
}

func writeModuleHelp(writer io.Writer) {
	commands := moduleCLICommands()
	if len(commands) == 0 {
		return
	}
	fmt.Fprintf(writer, "\nModule Commands:\n")
	for _, command := range commands {
		fmt.Fprintf(writer, "  %-14s %s\n", command.Name, command.Description)
		if command.Usage != "" {
			fmt.Fprintf(writer, "                  usage: %s\n", command.Usage)
		}
		for _, example := range command.Examples {
			fmt.Fprintf(writer, "                  example: %s\n", example)
		}
	}
}

func writeModuleCommandHelp(writer io.Writer, command vexoapp.CLICommand) {
	command.WriteHelp(writer)
}

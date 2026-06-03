package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	appmodules "github.com/vexo-network/vexo-consensus/app/modules"
	"github.com/vexo-network/vexo-consensus/config"
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
	case "init":
		if err := runInit(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "init", err)
		}
		return nil
	case "validate":
		if err := runValidate(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "validate", err)
		}
		return nil
	case "config":
		if err := runConfig(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "config", err)
		}
		return nil
	case "keys":
		if err := runKeys(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "keys", err)
		}
		return nil
	case "tx":
		if err := runTx(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "tx", err)
		}
		return nil
	case "start":
		if err := runStart(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "start", err)
		}
		return nil
	case "network":
		if err := runNetwork(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "network", err)
		}
		return nil
	case "consensus":
		if err := runConsensus(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "consensus", err)
		}
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
	case "snapshot":
		if err := runSnapshot(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "snapshot", err)
		}
		return nil
	case "slashing":
		if err := runSlashing(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "slashing", err)
		}
		return nil
	case "doctor":
		if err := runDoctor(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "doctor", err)
		}
		return nil
	case "ops":
		if err := runOps(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "ops", err)
		}
		return nil
	case "upgrade":
		if err := runUpgrade(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "upgrade", err)
		}
		return nil
	case "release":
		if err := runRelease(stdout, args[1:]); err != nil {
			return writeCommandError(stderr, "release", err)
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
	fmt.Fprintf(writer, "  init            initialize config and genesis files\n")
	fmt.Fprintf(writer, "  validate        validate config and genesis files\n")
	fmt.Fprintf(writer, "  config audit    run deployment and production-readiness checks\n")
	fmt.Fprintf(writer, "  config audit-pack generate external security audit evidence checklist\n")
	fmt.Fprintf(writer, "  config deployment-template print recommended deployment parameters\n")
	fmt.Fprintf(writer, "  config paths    print resolved config, genesis, key, and data paths\n")
	fmt.Fprintf(writer, "  config show     print loaded chain config as JSON\n")
	fmt.Fprintf(writer, "  keys gen        generate an Ed25519 validator key\n")
	fmt.Fprintf(writer, "  keys remote     register a remote KMS/HSM validator signer\n")
	fmt.Fprintf(writer, "  keys serve-remote serve a policy-enforced KMS/HSM signer endpoint\n")
	fmt.Fprintf(writer, "  keys verify-remote verify remote KMS/HSM challenge signing\n")
	fmt.Fprintf(writer, "  keys sign-tx    sign a raw transaction payload\n")
	fmt.Fprintf(writer, "  keys show       show validator public key\n")
	fmt.Fprintf(writer, "  tx              build or parse canonical transaction payloads\n")
	fmt.Fprintf(writer, "  start           validate files, prepare startup, or run node with --run; Ctrl+C shuts down gracefully\n")
	fmt.Fprintf(writer, "  network         up, initialize, start, smoke, load, scale-plan, chaos, longrun, status, and stop node networks\n")
	fmt.Fprintf(writer, "  consensus       run consensus simulations and diagnostics\n")
	fmt.Fprintf(writer, "  snapshot        export, verify, fetch, sync, restore, or drill state snapshots\n")
	fmt.Fprintf(writer, "  slashing        plan evidence lifecycle, appeals, jail, and penalty operations\n")
	fmt.Fprintf(writer, "  doctor          inspect config, keys, store, snapshot, and recovery readiness\n")
	fmt.Fprintf(writer, "  ops             print thresholds, evaluate samples, or build incident reports\n")
	fmt.Fprintf(writer, "  upgrade         build, apply, and rollback-drill governance upgrade plans\n")
	fmt.Fprintf(writer, "  release         package release manifests, checklists, readiness sweeps, and release gates\n")
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

package evm

import (
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func (Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{evmCLICommand()}
}

func evmCLICommand() vexoapp.CLICommand {
	return vexoapp.CLICommand{
		Name:        ModuleName,
		Usage:       "evm <command>",
		Description: "contract VM and Web3 bridge module commands",
		Examples: []string{
			"evm tx call evm 0xalice 0xcontract transfer aabbcc 100000",
			"evm tx deploy evm 0xalice 60016000 salt1",
			"evm query receipt <tx_hash>",
		},
		Children: []vexoapp.CLICommand{
			{
				Name:        "tx",
				Usage:       "evm tx <command>",
				Description: "build contract VM transaction payloads",
				Children: []vexoapp.CLICommand{
					{Name: "call", Usage: "evm tx call <vm> <from> <to> <method> <input_hex> <gas_limit> [value]", Description: "build a contract VM call transaction", Run: runEVMCallCLI},
					{Name: "deploy", Usage: "evm tx deploy <vm> <from> <code_hex> <salt> [value]", Description: "build a contract VM deployment transaction", Run: runEVMDeployCLI},
				},
			},
			{
				Name:        "query",
				Usage:       "evm query <command>",
				Description: "build contract VM query paths",
				Children: []vexoapp.CLICommand{
					{Name: "receipt", Usage: "evm query receipt <tx_hash>", Description: "build a contract VM receipt query path", Run: runEVMReceiptQueryCLI},
					{Name: "code", Usage: "evm query code <address>", Description: "build a contract VM code query path", Run: runEVMCodeQueryCLI},
					{Name: "storage", Usage: "evm query storage <address> <slot>", Description: "build a contract VM storage query path", Run: runEVMStorageQueryCLI},
					{Name: "logs", Usage: "evm query logs [address]", Description: "build a contract VM logs query path", Run: runEVMLogsQueryCLI},
				},
			},
		},
	}
}

func runEVMCallCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 6 && len(args) != 7 {
		return vexoapp.ErrCLIUsage("evm tx call <vm> <from> <to> <method> <input_hex> <gas_limit> [value]")
	}
	if _, err := strconv.ParseUint(args[5], 10, 64); err != nil {
		return ErrInvalidEVMTx
	}
	if len(args) == 7 {
		value, err := parseCLIAmount(args[6])
		if err != nil {
			return ErrInvalidEVMTx
		}
		args[6] = value
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "call", Args: args, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runEVMDeployCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 4 && len(args) != 5 {
		return vexoapp.ErrCLIUsage("evm tx deploy <vm> <from> <code_hex> <salt> [value]")
	}
	if len(args) == 5 {
		value, err := parseCLIAmount(args[4])
		if err != nil {
			return ErrInvalidEVMTx
		}
		args[4] = value
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{Module: ModuleName, Action: "deploy", Args: args, Tags: tags})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func parseCLIAmount(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "0", nil
	}
	base := 10
	valueText := trimmed
	if strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
		base = 16
		valueText = trimmed[2:]
	}
	value, ok := new(big.Int).SetString(valueText, base)
	if !ok || value.Sign() < 0 || value.BitLen() > 256 {
		return "", ErrInvalidEVMTx
	}
	return value.String(), nil
}

func runEVMReceiptQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("evm query receipt <tx_hash>")
	}
	fmt.Fprintf(writer, "query_path: %s/receipt/%s\n", ModuleName, args[0])
	return nil
}

func runEVMCodeQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("evm query code <address>")
	}
	fmt.Fprintf(writer, "query_path: %s/code/%s\n", ModuleName, args[0])
	return nil
}

func runEVMStorageQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("evm query storage <address> <slot>")
	}
	fmt.Fprintf(writer, "query_path: %s/storage/%s/%s\n", ModuleName, args[0], args[1])
	return nil
}

func runEVMLogsQueryCLI(writer io.Writer, args []string) error {
	if len(args) > 1 {
		return vexoapp.ErrCLIUsage("evm query logs [address]")
	}
	if len(args) == 0 {
		fmt.Fprintf(writer, "query_path: %s/logs\n", ModuleName)
		return nil
	}
	fmt.Fprintf(writer, "query_path: %s/logs/%s\n", ModuleName, args[0])
	return nil
}

func splitExecutionTags(args []string) ([]string, map[string]string, error) {
	tags := map[string]string{}
	cleaned := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			cleaned = append(cleaned, arg)
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		switch key {
		case "fee", "gas", "signer", "nonce":
			if index+1 >= len(args) {
				return nil, nil, vexoapp.ErrCLIUsage("missing value for --" + key)
			}
			tags[key] = args[index+1]
			index++
		default:
			return nil, nil, vexoapp.ErrCLIUsage("unknown evm flag " + arg)
		}
	}
	if len(tags) == 0 {
		tags = nil
	}
	return cleaned, tags, nil
}

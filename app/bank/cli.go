package bank

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func (Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{
		{
			Name:        ModuleName,
			Usage:       "bank tx mint <to> <amount> | bank tx send <from> <to> <amount> | bank query balance <address>",
			Description: "build bank module transactions and query paths",
			Run:         runBankCLI,
		},
	}
}

func runBankCLI(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("bank subcommand is required")
	}
	switch args[0] {
	case "tx":
		return runBankTxCLI(writer, args[1:])
	case "query":
		return runBankQueryCLI(writer, args[1:])
	default:
		return fmt.Errorf("unknown bank subcommand %q", args[0])
	}
}

func runBankTxCLI(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("bank tx subcommand is required")
	}
	switch args[0] {
	case "mint":
		if len(args) != 3 {
			return errors.New("usage: bank tx mint <to> <amount>")
		}
		amount, err := parseCLIAmount(args[2])
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "tx: %s:mint:%s:%d\n", ModuleName, args[1], amount)
		return nil
	case "send":
		if len(args) != 4 {
			return errors.New("usage: bank tx send <from> <to> <amount>")
		}
		amount, err := parseCLIAmount(args[3])
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "tx: %s:send:%s:%s:%d\n", ModuleName, args[1], args[2], amount)
		return nil
	default:
		return fmt.Errorf("unknown bank tx subcommand %q", args[0])
	}
}

func runBankQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 || args[0] != "balance" {
		return errors.New("usage: bank query balance <address>")
	}
	fmt.Fprintf(writer, "query_path: %s/balance/%s\n", ModuleName, args[1])
	return nil
}

func parseCLIAmount(value string) (uint64, error) {
	amount, err := strconv.ParseUint(value, 10, 64)
	if err != nil || amount == 0 {
		return 0, ErrInvalidBankTx
	}
	return amount, nil
}

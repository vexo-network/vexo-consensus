package bank

import (
	"fmt"
	"io"
	"strconv"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func (Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{bankCLICommand()}
}

func bankCLICommand() vexoapp.CLICommand {
	return vexoapp.CLICommand{
		Name:        ModuleName,
		Usage:       "bank <command>",
		Description: "bank module commands for transactions and balance queries",
		Examples: []string{
			"bank tx mint alice 100",
			"bank tx send alice bob 25",
			"bank query balance alice",
		},
		Children: []vexoapp.CLICommand{
			{
				Name:        "tx",
				Usage:       "bank tx <command>",
				Description: "build bank transaction payloads",
				Children: []vexoapp.CLICommand{
					{
						Name:        "mint",
						Usage:       "bank tx mint <to> <amount>",
						Description: "build a mint transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "to", Description: "recipient account address"},
							{Name: "amount", Description: "positive integer amount to mint"},
						},
						Examples: []string{"bank tx mint alice 100"},
						Run:      runBankMintCLI,
					},
					{
						Name:        "send",
						Usage:       "bank tx send <from> <to> <amount>",
						Description: "build a send transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "from", Description: "sender account address"},
							{Name: "to", Description: "recipient account address"},
							{Name: "amount", Description: "positive integer amount to send"},
						},
						Examples: []string{"bank tx send alice bob 25"},
						Run:      runBankSendCLI,
					},
				},
			},
			{
				Name:        "query",
				Usage:       "bank query <command>",
				Description: "build bank query paths",
				Children: []vexoapp.CLICommand{
					{
						Name:        "balance",
						Usage:       "bank query balance <address>",
						Description: "build a balance query path",
						Args: []vexoapp.CLIArg{
							{Name: "address", Description: "account address to query"},
						},
						Examples: []string{"bank query balance alice"},
						Run:      runBankBalanceCLI,
					},
				},
			},
		},
	}
}

func runBankMintCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("bank tx mint <to> <amount>")
	}
	amount, err := parseCLIAmount(args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s:mint:%s:%d\n", ModuleName, args[0], amount)
	return nil
}

func runBankSendCLI(writer io.Writer, args []string) error {
	if len(args) != 3 {
		return vexoapp.ErrCLIUsage("bank tx send <from> <to> <amount>")
	}
	amount, err := parseCLIAmount(args[2])
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s:send:%s:%s:%d\n", ModuleName, args[0], args[1], amount)
	return nil
}

func runBankBalanceCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("bank query balance <address>")
	}
	fmt.Fprintf(writer, "query_path: %s/balance/%s\n", ModuleName, args[0])
	return nil
}

func parseCLIAmount(value string) (uint64, error) {
	amount, err := strconv.ParseUint(value, 10, 64)
	if err != nil || amount == 0 {
		return 0, ErrInvalidBankTx
	}
	return amount, nil
}

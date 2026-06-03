package staking

import (
	"fmt"
	"io"
	"strconv"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func (module *Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{stakingCLICommand()}
}

func stakingCLICommand() vexoapp.CLICommand {
	return vexoapp.CLICommand{
		Name:        ModuleName,
		Usage:       "staking <command>",
		Description: "staking module commands for delegation, unbonding, and validator queries",
		Examples: []string{
			"staking tx delegate alice validator-1 100 <base64-public-key>",
			"staking tx undelegate alice validator-1 50",
			"staking query validator validator-1",
		},
		Children: []vexoapp.CLICommand{
			{
				Name:        "tx",
				Usage:       "staking tx <command>",
				Description: "build staking transaction payloads",
				Children: []vexoapp.CLICommand{
					{
						Name:        "delegate",
						Usage:       "staking tx delegate <delegator> <validator> <amount> <public_key>",
						Description: "build a delegation transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id"},
							{Name: "amount", Description: "positive integer stake amount"},
							{Name: "public_key", Description: "base64 validator consensus public key"},
						},
						Run: runDelegateCLI,
					},
					{
						Name:        "undelegate",
						Usage:       "staking tx undelegate <delegator> <validator> <amount>",
						Description: "build an undelegation transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id"},
							{Name: "amount", Description: "positive integer stake amount"},
						},
						Run: runUndelegateCLI,
					},
					{
						Name:        "unjail",
						Usage:       "staking tx unjail <validator>",
						Description: "build a validator unjail transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "validator", Description: "validator id"},
						},
						Run: runUnjailCLI,
					},
				},
			},
			{
				Name:        "query",
				Usage:       "staking query <command>",
				Description: "build staking query paths",
				Children: []vexoapp.CLICommand{
					{
						Name:        "stake",
						Usage:       "staking query stake <delegator> <validator>",
						Description: "build a delegated stake query path",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id"},
						},
						Run: runStakeQueryCLI,
					},
					{
						Name:        "validator",
						Usage:       "staking query validator <validator>",
						Description: "build a validator voting power query path",
						Args: []vexoapp.CLIArg{
							{Name: "validator", Description: "validator id"},
						},
						Run: runValidatorQueryCLI,
					},
					{
						Name:        "unbonding",
						Usage:       "staking query unbonding <delegator> <validator>",
						Description: "build an unbonding release-height query path",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id"},
						},
						Run: runUnbondingQueryCLI,
					},
				},
			},
		},
	}
}

func runDelegateCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 4 {
		return vexoapp.ErrCLIUsage("staking tx delegate <delegator> <validator> <amount> <public_key>")
	}
	amount, err := parseCLIAmount(args[2])
	if err != nil {
		return err
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "delegate",
		Args:   []string{args[0], args[1], strconv.FormatUint(amount, 10), args[3]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runUndelegateCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 3 {
		return vexoapp.ErrCLIUsage("staking tx undelegate <delegator> <validator> <amount>")
	}
	amount, err := parseCLIAmount(args[2])
	if err != nil {
		return err
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "undelegate",
		Args:   []string{args[0], args[1], strconv.FormatUint(amount, 10)},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runUnjailCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("staking tx unjail <validator>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "unjail",
		Args:   []string{args[0]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runStakeQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking query stake <delegator> <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/stake/%s/%s\n", ModuleName, args[0], args[1])
	return nil
}

func runValidatorQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("staking query validator <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/validator/%s\n", ModuleName, args[0])
	return nil
}

func runUnbondingQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking query unbonding <delegator> <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/unbonding/%s/%s\n", ModuleName, args[0], args[1])
	return nil
}

func parseCLIAmount(value string) (uint64, error) {
	return parseAmount(value)
}

func splitExecutionTags(args []string) ([]string, map[string]string, error) {
	positional := make([]string, 0, len(args))
	tags := make(map[string]string)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if len(arg) < 2 || arg[:2] != "--" {
			positional = append(positional, arg)
			continue
		}
		key := arg[2:]
		switch key {
		case "fee", "gas", "signer", "nonce":
			if index+1 >= len(args) || len(args[index+1]) >= 2 && args[index+1][:2] == "--" {
				return nil, nil, vexoapp.ErrCLIUsage("--" + key + " <value>")
			}
			tags[key] = args[index+1]
			index++
		default:
			return nil, nil, vexoapp.ErrCLIUsage("unknown staking tx flag " + arg)
		}
	}
	if len(tags) == 0 {
		return positional, nil, nil
	}
	return positional, tags, nil
}

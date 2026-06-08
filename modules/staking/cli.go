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
			"staking tx withdraw-unbonded alice validator-1",
			"staking tx claim-rewards alice validator-1",
			"staking tx set-commission validator-1 500 --signer validator-1",
			"staking query validator validator-1",
			"staking query rewards alice validator-1",
			"staking query tombstone validator-1",
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
					{
						Name:        "claim-rewards",
						Usage:       "staking tx claim-rewards <delegator> <validator>",
						Description: "build a staking reward claim transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id that accrued the reward"},
						},
						Run: runClaimRewardsCLI,
					},
					{
						Name:        "withdraw-unbonded",
						Usage:       "staking tx withdraw-unbonded <delegator> <validator>",
						Description: "build a matured unbonding withdrawal transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id that released the unbonded balance"},
						},
						Run: runWithdrawUnbondedCLI,
					},
					{
						Name:        "set-commission",
						Usage:       "staking tx set-commission <validator> <bps> --signer <validator>",
						Description: "build a validator commission update transaction payload",
						Args: []vexoapp.CLIArg{
							{Name: "validator", Description: "validator id"},
							{Name: "bps", Description: "commission in basis points, 10000 = 100%"},
						},
						Run: runSetCommissionCLI,
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
					{
						Name:        "unbonding-balance",
						Usage:       "staking query unbonding-balance <delegator> <validator>",
						Description: "build an unbonding balance query path",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id"},
						},
						Run: runUnbondingBalanceQueryCLI,
					},
					{
						Name:        "rewards",
						Usage:       "staking query rewards <delegator> <validator>",
						Description: "build a pending staking rewards query path",
						Args: []vexoapp.CLIArg{
							{Name: "delegator", Description: "delegator account address"},
							{Name: "validator", Description: "validator id"},
						},
						Run: runRewardsQueryCLI,
					},
					{
						Name:        "commission",
						Usage:       "staking query commission <validator>",
						Description: "build a validator commission query path",
						Args: []vexoapp.CLIArg{
							{Name: "validator", Description: "validator id"},
						},
						Run: runCommissionQueryCLI,
					},
					{
						Name:        "tombstone",
						Usage:       "staking query tombstone <validator>",
						Description: "build a validator tombstone query path",
						Args: []vexoapp.CLIArg{
							{Name: "validator", Description: "validator id"},
						},
						Run: runTombstoneQueryCLI,
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

func runClaimRewardsCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking tx claim-rewards <delegator> <validator>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "claim-rewards",
		Args:   []string{args[0], args[1]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runWithdrawUnbondedCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking tx withdraw-unbonded <delegator> <validator>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "withdraw-unbonded",
		Args:   []string{args[0], args[1]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runSetCommissionCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking tx set-commission <validator> <bps> --signer <validator>")
	}
	commissionBPS, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil || commissionBPS > commissionDenominatorBPS {
		return ErrInvalidCommission
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "set-commission",
		Args:   []string{args[0], strconv.FormatUint(commissionBPS, 10)},
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

func runUnbondingBalanceQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking query unbonding-balance <delegator> <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/unbonding-balance/%s/%s\n", ModuleName, args[0], args[1])
	return nil
}

func runRewardsQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("staking query rewards <delegator> <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/rewards/%s/%s\n", ModuleName, args[0], args[1])
	return nil
}

func runCommissionQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("staking query commission <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/commission/%s\n", ModuleName, args[0])
	return nil
}

func runTombstoneQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("staking query tombstone <validator>")
	}
	fmt.Fprintf(writer, "query_path: %s/tombstone/%s\n", ModuleName, args[0])
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

package governance

import (
	"fmt"
	"io"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func (module *Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{governanceCLICommand()}
}

func governanceCLICommand() vexoapp.CLICommand {
	return vexoapp.CLICommand{
		Name:        ModuleName,
		Usage:       "governance <command>",
		Description: "governance proposal, voting, execution, and query commands",
		Examples: []string{
			"governance tx submit alice max-gas execution max_gas 20000000",
			"governance tx vote 1 alice yes 10",
			"governance query tally 1",
		},
		Children: []vexoapp.CLICommand{
			{
				Name:        "tx",
				Usage:       "governance tx <command>",
				Description: "build governance transaction payloads",
				Children: []vexoapp.CLICommand{
					{
						Name:        "submit",
						Usage:       "governance tx submit <submitter> <title> <module> <key> <value>",
						Description: "build a parameter-change proposal transaction",
						Args: []vexoapp.CLIArg{
							{Name: "submitter", Description: "proposal submitter address"},
							{Name: "title", Description: "short proposal title without ':'"},
							{Name: "module", Description: "target module name"},
							{Name: "key", Description: "target parameter key"},
							{Name: "value", Description: "new parameter value without ':'"},
						},
						Run: runSubmitCLI,
					},
					{
						Name:        "vote",
						Usage:       "governance tx vote <proposal_id> <voter> <option> <power>",
						Description: "build a governance vote transaction",
						Args: []vexoapp.CLIArg{
							{Name: "proposal_id", Description: "positive proposal id"},
							{Name: "voter", Description: "voter address"},
							{Name: "option", Description: "yes, no, abstain, or veto"},
							{Name: "power", Description: "positive voting power"},
						},
						Run: runVoteCLI,
					},
					{
						Name:        "execute",
						Usage:       "governance tx execute <proposal_id>",
						Description: "build a proposal execution transaction",
						Args: []vexoapp.CLIArg{
							{Name: "proposal_id", Description: "positive proposal id"},
						},
						Run: runExecuteCLI,
					},
				},
			},
			{
				Name:        "query",
				Usage:       "governance query <command>",
				Description: "build governance query paths",
				Children: []vexoapp.CLICommand{
					{
						Name:        "proposal",
						Usage:       "governance query proposal <proposal_id>",
						Description: "build a proposal state query path",
						Args: []vexoapp.CLIArg{
							{Name: "proposal_id", Description: "positive proposal id"},
						},
						Run: runProposalQueryCLI,
					},
					{
						Name:        "tally",
						Usage:       "governance query tally <proposal_id>",
						Description: "build a proposal tally query path",
						Args: []vexoapp.CLIArg{
							{Name: "proposal_id", Description: "positive proposal id"},
						},
						Run: runTallyQueryCLI,
					},
					{
						Name:        "applied",
						Usage:       "governance query applied",
						Description: "build an applied parameter changes query path",
						Run:         runAppliedQueryCLI,
					},
				},
			},
		},
	}
}

func runSubmitCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 5 {
		return vexoapp.ErrCLIUsage("governance tx submit <submitter> <title> <module> <key> <value>")
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "submit",
		Args:   []string{args[0], args[1], args[2], args[3], args[4]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runVoteCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 4 {
		return vexoapp.ErrCLIUsage("governance tx vote <proposal_id> <voter> <option> <power>")
	}
	if _, err := parseProposalID(args[0]); err != nil {
		return err
	}
	if _, err := parseVotingPower(args[3]); err != nil {
		return err
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "vote",
		Args:   []string{args[0], args[1], args[2], args[3]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runExecuteCLI(writer io.Writer, args []string) error {
	args, tags, err := splitExecutionTags(args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("governance tx execute <proposal_id>")
	}
	if _, err := parseProposalID(args[0]); err != nil {
		return err
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "execute",
		Args:   []string{args[0]},
		Tags:   tags,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runProposalQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("governance query proposal <proposal_id>")
	}
	if _, err := parseProposalID(args[0]); err != nil {
		return err
	}
	fmt.Fprintf(writer, "query_path: %s/proposal/%s\n", ModuleName, args[0])
	return nil
}

func runTallyQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 1 {
		return vexoapp.ErrCLIUsage("governance query tally <proposal_id>")
	}
	if _, err := parseProposalID(args[0]); err != nil {
		return err
	}
	fmt.Fprintf(writer, "query_path: %s/tally/%s\n", ModuleName, args[0])
	return nil
}

func runAppliedQueryCLI(writer io.Writer, args []string) error {
	if len(args) != 0 {
		return vexoapp.ErrCLIUsage("governance query applied")
	}
	fmt.Fprintf(writer, "query_path: %s/applied\n", ModuleName)
	return nil
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
			return nil, nil, vexoapp.ErrCLIUsage("unknown governance tx flag " + arg)
		}
	}
	if len(tags) == 0 {
		return positional, nil, nil
	}
	return positional, tags, nil
}

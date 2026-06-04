package params

import (
	"encoding/base64"
	"fmt"
	"io"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
)

func (module *Module) CLICommands() []vexoapp.CLICommand {
	return []vexoapp.CLICommand{
		{
			Name:        Namespace,
			Description: "build parameter governance transactions and query paths",
			Children: []vexoapp.CLICommand{
				{
					Name:        "tx",
					Description: "build parameter transactions",
					Children: []vexoapp.CLICommand{
						{
							Name:        "set",
							Description: "build a parameter set transaction",
							Args: []vexoapp.CLIArg{
								{Name: "authority", Description: "authority address or module authority"},
								{Name: "module", Description: "target module name"},
								{Name: "key", Description: "parameter key"},
								{Name: "value", Description: "raw parameter value"},
							},
							Run: runParamsSet,
						},
					},
				},
				{
					Name:        "query",
					Description: "print parameter query paths",
					Children: []vexoapp.CLICommand{
						{
							Name:        "param",
							Description: "print a parameter query path",
							Args: []vexoapp.CLIArg{
								{Name: "module", Description: "target module name"},
								{Name: "key", Description: "parameter key"},
							},
							Run: runParamsQuery,
						},
					},
				},
			},
		},
	}
}

func runParamsSet(writer io.Writer, args []string) error {
	if len(args) != 4 {
		return vexoapp.ErrCLIUsage("params tx set <authority> <module> <key> <value>")
	}
	fmt.Fprintf(writer, "%s:set:%s:%s:%s:%s\n", Namespace, args[0], args[1], args[2], base64.StdEncoding.EncodeToString([]byte(args[3])))
	return nil
}

func runParamsQuery(writer io.Writer, args []string) error {
	if len(args) != 2 {
		return vexoapp.ErrCLIUsage("params query param <module> <key>")
	}
	fmt.Fprintf(writer, "%s/param/%s/%s\n", Namespace, args[0], args[1])
	return nil
}

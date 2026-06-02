package app

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCLICommandExecutesNestedChildren(t *testing.T) {
	command := CLICommand{
		Name:        "bank",
		Description: "bank module",
		Children: []CLICommand{
			{
				Name:        "tx",
				Description: "transaction commands",
				Children: []CLICommand{
					{
						Name:        "mint",
						Usage:       "bank tx mint <to> <amount>",
						Description: "mint coins",
						Run: func(writer io.Writer, args []string) error {
							_, err := writer.Write([]byte(strings.Join(args, ":")))
							return err
						},
					},
				},
			},
		},
	}

	var output bytes.Buffer
	if err := command.Execute(&output, []string{"tx", "mint", "alice", "100"}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "alice:100" {
		t.Fatalf("unexpected nested command output: %s", output.String())
	}
}

func TestCLICommandWritesNestedHelp(t *testing.T) {
	command := CLICommand{
		Name:        "bank",
		Description: "bank module",
		Children: []CLICommand{
			{Name: "tx", Description: "transaction commands"},
			{Name: "query", Description: "query commands"},
		},
	}

	var output bytes.Buffer
	if err := command.Execute(&output, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"bank module", "Usage:", "bank <command>", "Commands:", "tx", "query"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected help to contain %q, got:\n%s", expected, output.String())
		}
	}
}

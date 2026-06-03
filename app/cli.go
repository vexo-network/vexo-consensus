package app

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type CLIArg struct {
	Name        string
	Description string
}

type CLICommand struct {
	Name        string
	Usage       string
	Description string
	Args        []CLIArg
	Examples    []string
	Children    []CLICommand
	Run         func(writer io.Writer, args []string) error
}

type CLICommandProvider interface {
	CLICommands() []CLICommand
}

func (command CLICommand) Execute(writer io.Writer, args []string) error {
	return command.execute(writer, []string{command.Name}, args)
}

func (command CLICommand) WriteHelp(writer io.Writer) {
	command.writeHelp(writer, []string{command.Name})
}

func (command CLICommand) execute(writer io.Writer, path []string, args []string) error {
	if len(args) == 0 {
		if command.Run != nil {
			return command.Run(writer, args)
		}
		command.writeHelp(writer, path)
		return nil
	}
	if isCLIHelp(args[0]) {
		command.writeHelp(writer, path)
		return nil
	}
	for _, child := range command.Children {
		if child.Name == args[0] {
			return child.execute(writer, append(path, child.Name), args[1:])
		}
	}
	if command.Run != nil {
		return command.Run(writer, args)
	}
	return fmt.Errorf("unknown %s subcommand %q", strings.Join(path, " "), args[0])
}

func (command CLICommand) writeHelp(writer io.Writer, path []string) {
	if command.Description != "" {
		fmt.Fprintf(writer, "%s\n\n", command.Description)
	}
	fmt.Fprintf(writer, "Usage:\n")
	fmt.Fprintf(writer, "  %s\n", command.usage(path))
	if len(command.Args) > 0 {
		fmt.Fprintf(writer, "\nArguments:\n")
		for _, arg := range command.Args {
			fmt.Fprintf(writer, "  %-12s %s\n", arg.Name, arg.Description)
		}
	}
	if len(command.Children) > 0 {
		fmt.Fprintf(writer, "\nCommands:\n")
		for _, child := range command.Children {
			fmt.Fprintf(writer, "  %-12s %s\n", child.Name, child.Description)
		}
	}
	if len(command.Examples) > 0 {
		fmt.Fprintf(writer, "\nExamples:\n")
		for _, example := range command.Examples {
			fmt.Fprintf(writer, "  %s\n", example)
		}
	}
}

func (command CLICommand) usage(path []string) string {
	if command.Usage != "" {
		return command.Usage
	}
	base := strings.Join(path, " ")
	if len(command.Children) > 0 {
		return base + " <command>"
	}
	if len(command.Args) == 0 {
		return base
	}
	parts := []string{base}
	for _, arg := range command.Args {
		parts = append(parts, "<"+arg.Name+">")
	}
	return strings.Join(parts, " ")
}

func isCLIHelp(arg string) bool {
	return arg == "help" || arg == "--help" || arg == "-h"
}

func ErrCLIUsage(usage string) error {
	if usage == "" {
		return errors.New("invalid cli usage")
	}
	return fmt.Errorf("usage: %s", usage)
}

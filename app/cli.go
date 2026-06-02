package app

import "io"

type CLICommand struct {
	Name        string
	Usage       string
	Description string
	Run         func(writer io.Writer, args []string) error
}

type CLICommandProvider interface {
	CLICommands() []CLICommand
}

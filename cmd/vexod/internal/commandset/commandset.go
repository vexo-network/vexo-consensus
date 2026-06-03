package commandset

import (
	"fmt"
	"io"
)

type Handler func(io.Writer, []string) error

type Command struct {
	Name        string
	Description string
	Handler     Handler
}

type Registry struct {
	commands []Command
	byName   map[string]Command
}

func New(commands []Command) Registry {
	registry := Registry{
		commands: append([]Command(nil), commands...),
		byName:   make(map[string]Command, len(commands)),
	}
	for _, command := range commands {
		registry.byName[command.Name] = command
	}
	return registry
}

func (registry Registry) Commands() []Command {
	return append([]Command(nil), registry.commands...)
}

func (registry Registry) Run(name string, writer io.Writer, args []string) (bool, error) {
	command, found := registry.byName[name]
	if !found {
		return false, nil
	}
	if command.Handler == nil {
		return true, fmt.Errorf("command %q has no handler", name)
	}
	return true, command.Handler(writer, args)
}

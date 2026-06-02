package app

import (
	"errors"
	"fmt"
)

var (
	ErrModuleNameRequired = errors.New("module name is required")
	ErrModuleRegistered   = errors.New("module already registered")
	ErrModuleNotFound     = errors.New("module not found")
)

type ModuleFactory func() Module

type Registry struct {
	factories map[string]ModuleFactory
}

func NewRegistry() Registry {
	return Registry{factories: make(map[string]ModuleFactory)}
}

func (registry Registry) Register(name string, factory ModuleFactory) error {
	if name == "" {
		return ErrModuleNameRequired
	}
	if factory == nil {
		return ErrModuleNotFound
	}
	if _, exists := registry.factories[name]; exists {
		return fmt.Errorf("%w: %s", ErrModuleRegistered, name)
	}
	registry.factories[name] = factory
	return nil
}

func (registry Registry) Build(enabled []string) ([]Module, error) {
	modules := make([]Module, 0, len(enabled))
	for _, name := range enabled {
		factory, ok := registry.factories[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrModuleNotFound, name)
		}
		module := factory()
		if module == nil {
			return nil, fmt.Errorf("%w: %s", ErrModuleNotFound, name)
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func (registry Registry) Names() []string {
	names := make([]string, 0, len(registry.factories))
	for name := range registry.factories {
		names = append(names, name)
	}
	return names
}

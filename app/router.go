package app

import (
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrNoModules         = errors.New("no modules registered")
	ErrNoRouteForTx      = errors.New("no module route for transaction")
	ErrMalformedRoutedTx = errors.New("malformed routed transaction")
)

type FirstModuleRouter struct{}

func (FirstModuleRouter) RouteTx(ctx Context, tx types.Tx, modules []Module) (Module, error) {
	if len(modules) == 0 {
		return nil, ErrNoModules
	}
	return modules[0], nil
}

type PrefixRouter struct{}

func (PrefixRouter) RouteTx(ctx Context, tx types.Tx, modules []Module) (Module, error) {
	parts := strings.SplitN(string(tx), ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return nil, ErrMalformedRoutedTx
	}

	for _, module := range modules {
		if module.Name() == parts[0] {
			return module, nil
		}
	}
	return nil, ErrNoRouteForTx
}

package contract

import (
	"context"
	"errors"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrVMNotFound        = errors.New("contract VM not found")
	ErrVMAlreadyExists   = errors.New("contract VM already exists")
	ErrInvalidInvocation = errors.New("invalid contract invocation")
)

type Invocation struct {
	VM       string        `json:"vm"`
	Caller   types.Address `json:"caller"`
	Contract types.Address `json:"contract"`
	Method   string        `json:"method"`
	Input    []byte        `json:"input,omitempty"`
	GasLimit uint64        `json:"gas_limit,omitempty"`
	Value    uint64        `json:"value,omitempty"`
}

type Result struct {
	Output        []byte         `json:"output,omitempty"`
	GasUsed       uint64         `json:"gas_used,omitempty"`
	Logs          []Log          `json:"logs,omitempty"`
	StorageWrites []StorageWrite `json:"storage_writes,omitempty"`
}

type Log struct {
	Address types.Address     `json:"address"`
	Topics  []string          `json:"topics,omitempty"`
	Data    []byte            `json:"data,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

type StorageWrite struct {
	Address types.Address `json:"address,omitempty"`
	Slot    string        `json:"slot"`
	Value   []byte        `json:"value,omitempty"`
	Delete  bool          `json:"delete,omitempty"`
}

type VM interface {
	Name() string
	Execute(ctx context.Context, invocation Invocation) (Result, error)
}

type Registry struct {
	mu  sync.RWMutex
	vms map[string]VM
}

func NewRegistry() *Registry {
	return &Registry{vms: make(map[string]VM)}
}

func (registry *Registry) Register(vm VM) error {
	if vm == nil || vm.Name() == "" {
		return ErrVMNotFound
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, found := registry.vms[vm.Name()]; found {
		return ErrVMAlreadyExists
	}
	registry.vms[vm.Name()] = vm
	return nil
}

func (registry *Registry) Execute(ctx context.Context, invocation Invocation) (Result, error) {
	if invocation.VM == "" || invocation.Caller == "" || invocation.Contract == "" || invocation.Method == "" {
		return Result{}, ErrInvalidInvocation
	}
	registry.mu.RLock()
	vm := registry.vms[invocation.VM]
	registry.mu.RUnlock()
	if vm == nil {
		return Result{}, ErrVMNotFound
	}
	return vm.Execute(ctx, cloneInvocation(invocation))
}

func (registry *Registry) Names() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	names := make([]string, 0, len(registry.vms))
	for name := range registry.vms {
		names = append(names, name)
	}
	return names
}

func cloneInvocation(invocation Invocation) Invocation {
	invocation.Input = append([]byte(nil), invocation.Input...)
	return invocation
}

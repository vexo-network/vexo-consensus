package contract

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

type echoVM struct{}

func (echoVM) Name() string { return "echo" }

func (echoVM) Execute(ctx context.Context, invocation Invocation) (Result, error) {
	return Result{Output: append([]byte(nil), invocation.Input...), GasUsed: 1}, nil
}

func TestRegistryExecutesRegisteredVM(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(echoVM{}); err != nil {
		t.Fatal(err)
	}
	result, err := registry.Execute(context.Background(), Invocation{
		VM:       "echo",
		Caller:   types.Address("alice"),
		Contract: types.Address("contract1"),
		Method:   "call",
		Input:    []byte("hello"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Output) != "hello" || result.GasUsed != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := registry.Execute(context.Background(), Invocation{VM: "missing", Caller: "alice", Contract: "contract1", Method: "call"}); err != ErrVMNotFound {
		t.Fatalf("expected missing VM, got %v", err)
	}
}

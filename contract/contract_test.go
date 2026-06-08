package contract

import (
	"context"
	"math/big"
	"reflect"
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

type mutatingVM struct {
	seen Invocation
}

func (vm *mutatingVM) Name() string { return "mutating" }

func (vm *mutatingVM) Execute(ctx context.Context, invocation Invocation) (Result, error) {
	vm.seen = cloneInvocation(invocation)
	invocation.Input[0] = 'x'
	invocation.Code[0] = 0xff
	invocation.Salt[0] = 0xee
	invocation.BlobHashes[0][0] = 0xdd
	invocation.AccessList[0].StorageKeys[0] = "mutated"
	invocation.ValueBig.SetUint64(999)
	return Result{Output: []byte("ok")}, nil
}

func TestRegistryNamesValidationAndCloneIsolation(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(nil); err != ErrVMNotFound {
		t.Fatalf("expected nil VM rejection, got %v", err)
	}
	if err := registry.Register(echoVM{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(echoVM{}); err != ErrVMAlreadyExists {
		t.Fatalf("expected duplicate VM rejection, got %v", err)
	}
	mutating := &mutatingVM{}
	if err := registry.Register(mutating); err != nil {
		t.Fatal(err)
	}
	if names := registry.Names(); !reflect.DeepEqual(names, []string{"echo", "mutating"}) {
		t.Fatalf("expected stable sorted VM names, got %+v", names)
	}
	original := Invocation{
		VM:       "mutating",
		Caller:   types.Address("alice"),
		Contract: types.Address("contract1"),
		Method:   "call",
		Input:    []byte("hello"),
		Code:     []byte{0x60, 0x00},
		Salt:     []byte{0x01},
		ValueBig: big.NewInt(7),
		BlobHashes: []types.Hash{
			{0x01},
		},
		AccessList: []AccessListEntry{{
			Address:     types.Address("contract1"),
			StorageKeys: []string{"0x01"},
		}},
	}
	if _, err := registry.Execute(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	if string(original.Input) != "hello" || original.Code[0] != 0x60 || original.Salt[0] != 0x01 || original.ValueBig.Uint64() != 7 || original.BlobHashes[0][0] != 0x01 || original.AccessList[0].StorageKeys[0] != "0x01" {
		t.Fatalf("registry leaked mutable invocation data back to caller: %+v", original)
	}
	if string(mutating.seen.Input) != "hello" || mutating.seen.ValueBig.Uint64() != 7 {
		t.Fatalf("unexpected invocation seen by VM: %+v", mutating.seen)
	}
	for _, invalid := range []Invocation{
		{VM: "echo", Contract: "contract1", Method: "call"},
		{VM: "echo", Caller: "alice", Method: "call"},
		{VM: "echo", Caller: "alice", Contract: "contract1"},
		{Caller: "alice", Contract: "contract1", Method: "call"},
	} {
		if _, err := registry.Execute(context.Background(), invalid); err != ErrInvalidInvocation {
			t.Fatalf("expected invalid invocation for %+v, got %v", invalid, err)
		}
	}
}

package geth

import (
	"context"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

type testStateReader struct {
	code     map[types.Address][]byte
	storage  map[string][]byte
	balances map[types.Address]uint64
	nonces   map[types.Address]uint64
}

func (reader testStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	return append([]byte(nil), reader.code[address]...), nil
}

func (reader testStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	return append([]byte(nil), reader.storage[string(address)+"/"+slot]...), nil
}

func (reader testStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	return reader.balances[address], nil
}

func (reader testStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	return reader.nonces[address], nil
}

func TestGethBackendExecutesDeployAndCall(t *testing.T) {
	vm := New()
	if vm.Name() != GethVMName {
		t.Fatalf("unexpected VM name %q", vm.Name())
	}
	salt := [32]byte{1}
	deploy, err := vm.Execute(context.Background(), contract.Invocation{
		Method:   "deploy",
		Caller:   "0x000000000000000000000000000000000000aaaa",
		Contract: "0xde0a5f3a72e81b432378352a6ff6fc71d19df444",
		Input:    []byte{0x60, 0x0a, 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, 0x0a, 0x60, 0x00, 0xf3, 0x60, 0x2a, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3},
		Salt:     salt[:],
		GasLimit: 100_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(deploy.DeployedCode) != string([]byte{0x60, 0x2a, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3}) {
		t.Fatalf("unexpected deployed code %x", deploy.DeployedCode)
	}
	call, err := vm.Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   "0x000000000000000000000000000000000000aaaa",
		Contract: "0xde0a5f3a72e81b432378352a6ff6fc71d19df444",
		Code:     deploy.DeployedCode,
		GasLimit: 100_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(call.Output) != 32 || call.Output[31] != 0x2a {
		t.Fatalf("unexpected call output %x", call.Output)
	}
}

func TestGethBackendImplementsContractVM(t *testing.T) {
	var _ contract.VM = New()
}

func TestGethBackendUsesReaderBalancesAndReturnsBalanceWrites(t *testing.T) {
	vm := New()
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	recipient := types.Address("0x000000000000000000000000000000000000bbbb")
	result, err := vm.Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   caller,
		Contract: recipient,
		GasLimit: 100_000,
		Value:    3,
		State: testStateReader{
			balances: map[types.Address]uint64{caller: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.BalanceWrites) != 2 {
		t.Fatalf("expected caller and recipient balance writes, got %+v", result.BalanceWrites)
	}
	writes := map[types.Address]uint64{}
	for _, write := range result.BalanceWrites {
		writes[types.Address(strings.ToLower(string(write.Address)))] = write.Balance
	}
	if writes[types.Address(strings.ToLower(string(caller)))] != 7 || writes[types.Address(strings.ToLower(string(recipient)))] != 3 {
		t.Fatalf("unexpected balance writes: %+v", result.BalanceWrites)
	}
}

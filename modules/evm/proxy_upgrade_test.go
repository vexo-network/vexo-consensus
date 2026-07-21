package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestModuleSupportsProxyUpgradeFlow(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	module := NewModule()
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	ctx := func(height types.Height) vexoapp.Context {
		return vexoapp.Context{Ctx: context.Background(), Height: height, Store: storage}
	}

	v1Creation, err := buildVersionedImplementationCreationCode(1)
	if err != nil {
		t.Fatal(err)
	}
	v2Creation, err := buildVersionedImplementationCreationCode(2)
	if err != nil {
		t.Fatal(err)
	}

	v1Receipt := deployEVMContract(t, module, ctx(1), caller, v1Creation, "v1")
	v2Receipt := deployEVMContract(t, module, ctx(2), caller, v2Creation, "v2")

	proxyCreation, err := buildProxyCreationCode(v1Receipt.ContractAddress, v2Receipt.ContractAddress)
	if err != nil {
		t.Fatal(err)
	}
	proxyReceipt := deployEVMContract(t, module, ctx(3), caller, proxyCreation, "proxy")
	if proxyCode := queryContractCode(t, module, ctx(3), proxyReceipt.ContractAddress); proxyCode == "" {
		t.Fatalf("expected proxy code to be stored, receipt=%+v", proxyReceipt)
	}

	initReceipt := callEVMContract(t, module, ctx(4), caller, proxyReceipt.ContractAddress, mustUpgradeCalldata(t, v1Receipt.ContractAddress), "upgrade")
	if initReceipt.Error != "" {
		t.Fatalf("unexpected proxy init error: %+v", initReceipt)
	}

	if got := callVersion(t, module, ctx(5), caller, proxyReceipt.ContractAddress); got != 1 {
		t.Fatalf("expected proxy to read version 1 before upgrade, got %d", got)
	}

	upgradeReceipt := callEVMContract(t, module, ctx(6), caller, proxyReceipt.ContractAddress, mustUpgradeCalldata(t, v2Receipt.ContractAddress), "upgrade")
	if upgradeReceipt.Error != "" {
		t.Fatalf("unexpected proxy upgrade error: %+v", upgradeReceipt)
	}

	if got := callVersion(t, module, ctx(7), caller, proxyReceipt.ContractAddress); got != 2 {
		t.Fatalf("expected proxy to read version 2 after upgrade, got %d", got)
	}

	slot0 := queryStorageValue(t, module, ctx(7), proxyReceipt.ContractAddress, "0x0")
	if !strings.EqualFold(slot0, "0x"+leftPadHex(v2Receipt.ContractAddress, 32)) {
		t.Fatalf("expected proxy implementation slot to point at v2, got %s want %s", slot0, "0x"+leftPadHex(v2Receipt.ContractAddress, 32))
	}

	proxyCode := queryContractCode(t, module, ctx(7), proxyReceipt.ContractAddress)
	if proxyCode == "" {
		t.Fatal("expected proxy code to remain deployed")
	}
}

func deployEVMContract(t *testing.T, module Module, ctx vexoapp.Context, caller types.Address, creationCode string, salt string) Receipt {
	t.Helper()
	tx := types.Tx(fmt.Sprintf("evm:deploy:evm:%s:%s:%s", caller, creationCode, salt))
	result := module.DeliverTx(ctx, tx)
	if result.Code != 0 {
		t.Fatalf("deploy failed: %+v", result)
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.ContractAddress == "" {
		t.Fatalf("expected contract address in receipt: %+v", receipt)
	}
	if receipt.Status != 1 {
		t.Fatalf("expected successful deployment receipt, got %+v", receipt)
	}
	return receipt
}

func callEVMContract(t *testing.T, module Module, ctx vexoapp.Context, caller types.Address, contract string, input []byte, method string) Receipt {
	t.Helper()
	tx := types.Tx(fmt.Sprintf("evm:call:evm:%s:%s:%s:0x%s:100000", caller, contract, method, hex.EncodeToString(input)))
	result := module.DeliverTx(ctx, tx)
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func callVersion(t *testing.T, module Module, ctx vexoapp.Context, caller types.Address, contract string) uint64 {
	t.Helper()
	receipt := callEVMContract(t, module, ctx, caller, contract, []byte{0x54, 0xfd, 0x4d, 0x50}, "call")
	if receipt.Error != "" {
		t.Fatalf("unexpected version call error: %+v", receipt)
	}
	if receipt.Output == "" {
		t.Fatalf("expected version output, got %+v", receipt)
	}
	output, err := hex.DecodeString(strings.TrimPrefix(receipt.Output, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 32 {
		t.Fatalf("unexpected version output length: %d", len(output))
	}
	return uint64(output[31])
}

func queryStorageValue(t *testing.T, module Module, ctx vexoapp.Context, address string, slot string) string {
	t.Helper()
	response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"storage", string(address), slot}})
	if response.Code != 0 {
		t.Fatalf("expected storage query success, got %+v", response)
	}
	var payload map[string]string
	if err := json.Unmarshal(response.Value, &payload); err != nil {
		t.Fatal(err)
	}
	return payload["value"]
}

func queryContractCode(t *testing.T, module Module, ctx vexoapp.Context, address string) string {
	t.Helper()
	response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"code", string(address)}})
	if response.Code != 0 {
		t.Fatalf("expected code query success, got %+v", response)
	}
	var payload map[string]string
	if err := json.Unmarshal(response.Value, &payload); err != nil {
		t.Fatal(err)
	}
	return payload["code"]
}

func buildVersionedImplementationCreationCode(version byte) (string, error) {
	runtime, err := buildVersionedImplementationRuntime(version)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(buildCreationCode(nil, runtime)), nil
}

func buildProxyCreationCode(versionOne string, versionTwo string) (string, error) {
	runtime, err := buildProxyRuntime(versionOne, versionTwo)
	if err != nil {
		return "", err
	}
	if _, err = hex.DecodeString(strings.TrimPrefix(versionOne, "0x")); err != nil {
		return "", err
	}
	if _, err = hex.DecodeString(strings.TrimPrefix(versionTwo, "0x")); err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(buildCreationCode(nil, runtime)), nil
}

func buildUpgradeCalldata(implementation string) ([]byte, error) {
	implBytes, err := hex.DecodeString(strings.TrimPrefix(implementation, "0x"))
	if err != nil {
		return nil, err
	}
	if len(implBytes) != 20 {
		return nil, fmt.Errorf("invalid implementation address length: %d", len(implBytes))
	}
	input := append([]byte{0x36, 0x59, 0xcf, 0xe6}, make([]byte, 12)...)
	input = append(input, implBytes...)
	return input, nil
}

func mustUpgradeCalldata(t *testing.T, implementation string) []byte {
	t.Helper()
	input, err := buildUpgradeCalldata(implementation)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func buildVersionedImplementationRuntime(version byte) ([]byte, error) {
	builder := newBytecodeBuilder()
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x54, 0xfd, 0x4d, 0x50, 0x14)
	builder.pushLabel("version")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("version")
	builder.op(0x60, version, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3)
	return builder.bytes()
}

func buildProxyRuntime(versionOne string, versionTwo string) ([]byte, error) {
	oneBytes, err := hex.DecodeString(strings.TrimPrefix(versionOne, "0x"))
	if err != nil {
		return nil, err
	}
	twoBytes, err := hex.DecodeString(strings.TrimPrefix(versionTwo, "0x"))
	if err != nil {
		return nil, err
	}
	if len(oneBytes) != 20 || len(twoBytes) != 20 {
		return nil, fmt.Errorf("invalid proxy implementation address length")
	}
	builder := newBytecodeBuilder()
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x54, 0xfd, 0x4d, 0x50, 0x14)
	builder.pushLabel("version")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x36, 0x59, 0xcf, 0xe6, 0x14)
	builder.pushLabel("upgrade")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("version")
	builder.op(0x60, 0x00, 0x54)
	builder.op(0x73)
	builder.op(oneBytes...)
	builder.op(0x14)
	builder.pushLabel("return_v1")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x54)
	builder.op(0x73)
	builder.op(twoBytes...)
	builder.op(0x14)
	builder.pushLabel("return_v2")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("return_v1")
	builder.op(0x60, 0x01, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3)
	builder.label("return_v2")
	builder.op(0x60, 0x02, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3)
	builder.label("upgrade")
	builder.op(0x60, 0x04, 0x35, 0x60, 0x00, 0x55, 0x60, 0x00, 0x60, 0x00, 0xf3)
	return builder.bytes()
}

func buildCreationCode(prefix, runtime []byte) []byte {
	code := append([]byte(nil), prefix...)
	runtimeOffset := len(prefix) + 12
	if len(runtime) > 0xff || runtimeOffset > 0xff {
		panic("test bytecode too large for PUSH1-based creation wrapper")
	}
	code = append(code,
		0x60, byte(len(runtime)),
		0x60, byte(runtimeOffset),
		0x60, 0x00, 0x39,
		0x60, byte(len(runtime)),
		0x60, 0x00, 0xf3,
	)
	code = append(code, runtime...)
	return code
}

type bytecodeBuilder struct {
	code    []byte
	labels  map[string]int
	patches []bytecodePatch
}

type bytecodePatch struct {
	pos   int
	label string
}

func newBytecodeBuilder() *bytecodeBuilder {
	return &bytecodeBuilder{labels: make(map[string]int)}
}

func (builder *bytecodeBuilder) op(bytes ...byte) {
	builder.code = append(builder.code, bytes...)
}

func (builder *bytecodeBuilder) label(name string) {
	builder.labels[name] = len(builder.code)
	builder.op(0x5b)
}

func (builder *bytecodeBuilder) pushLabel(label string) {
	builder.code = append(builder.code, 0x61, 0x00, 0x00)
	builder.patches = append(builder.patches, bytecodePatch{pos: len(builder.code) - 2, label: label})
}

func (builder *bytecodeBuilder) bytes() ([]byte, error) {
	code := append([]byte(nil), builder.code...)
	for _, patch := range builder.patches {
		offset, ok := builder.labels[patch.label]
		if !ok {
			return nil, fmt.Errorf("unknown label %q", patch.label)
		}
		if offset > 0xffff {
			return nil, fmt.Errorf("label %q too large", patch.label)
		}
		code[patch.pos] = byte(offset >> 8)
		code[patch.pos+1] = byte(offset)
	}
	return code, nil
}

func leftPadHex(value string, size int) string {
	raw := strings.TrimPrefix(value, "0x")
	if len(raw) > size*2 {
		return raw[len(raw)-size*2:]
	}
	if len(raw) < size*2 {
		return strings.Repeat("0", size*2-len(raw)) + raw
	}
	return raw
}

package evm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type testVM struct{}

func (testVM) Name() string { return "evm" }

func (testVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	return contract.Result{
		Output:  append([]byte("out:"), invocation.Input...),
		GasUsed: 7,
		StorageWrites: []contract.StorageWrite{{
			Slot:  "0x0",
			Value: []byte("stored:" + invocation.Method),
		}},
		Logs: []contract.Log{{
			Address: invocation.Contract,
			Topics:  []string{"0x01"},
			Data:    []byte("log"),
			Meta:    map[string]string{"method": invocation.Method},
		}},
	}, nil
}

type deletionVM struct{}

func (deletionVM) Name() string { return "evm" }

func (deletionVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	return contract.Result{
		GasUsed: 11,
		CodeWrites: []contract.CodeWrite{{
			Address: "0x000000000000000000000000000000000000c0de",
			Code:    []byte{0x60, 0x01},
		}},
		AccountDeletions: []contract.AccountDeletion{{
			Address: invocation.Contract,
		}},
		NonceWrites: []contract.NonceWrite{{
			Address: invocation.Contract,
			Nonce:   9,
		}},
		BalanceWrites: []contract.BalanceWrite{{
			Address: invocation.Contract,
			Balance: 88,
		}},
	}, nil
}

type nonceVM struct{}

func (nonceVM) Name() string { return "evm" }

func (nonceVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	return contract.Result{
		GasUsed: 13,
		NonceWrites: []contract.NonceWrite{{
			Address: invocation.Caller,
			Nonce:   8,
		}},
	}, nil
}

type bigBalanceVM struct {
	balance *big.Int
}

func (vm bigBalanceVM) Name() string { return "evm" }

func (vm bigBalanceVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	return contract.Result{
		GasUsed: 5,
		BalanceWrites: []contract.BalanceWrite{{
			Address:    invocation.Contract,
			BalanceBig: new(big.Int).Set(vm.balance),
		}},
	}, nil
}

type recordingInvocationVM struct {
	invocation contract.Invocation
}

func (vm *recordingInvocationVM) Name() string { return "evm" }

func (vm *recordingInvocationVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	vm.invocation = invocation
	return contract.Result{
		Output:  []byte{0x42},
		GasUsed: 17,
		AccessList: []contract.AccessListEntry{{
			Address:     invocation.Contract,
			StorageKeys: []string{"0x01"},
		}},
		StorageWrites: []contract.StorageWrite{{
			Address: invocation.Contract,
			Slot:    "0x01",
			Value:   []byte{0x02},
		}},
		VMTrace: map[string]any{"structLogs": []any{map[string]any{"op": "STOP"}}},
	}, nil
}

type failingVM struct{}

func (failingVM) Name() string { return "evm" }

func (failingVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	return contract.Result{GasUsed: 9, Failed: true, Error: "execution reverted", Output: []byte{0x08}}, nil
}

func TestModuleExecutesAndPersistsReceiptsCodeAndLogs(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry := contract.NewRegistry()
	if err := registry.Register(testVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 3, Store: storage}
	deployTx := types.Tx("evm:deploy:evm:0xaaaa:6001:salt")
	result := module.DeliverTx(ctx, deployTx)
	if result.Code != 0 {
		t.Fatalf("deploy failed: %+v", result)
	}
	var deployReceipt Receipt
	if err := json.Unmarshal(result.Data, &deployReceipt); err != nil {
		t.Fatal(err)
	}
	if deployReceipt.ContractAddress == "" || deployReceipt.GasUsed != 7 || len(deployReceipt.Logs) != 1 {
		t.Fatalf("unexpected deploy receipt: %+v", deployReceipt)
	}
	codeQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"code", deployReceipt.ContractAddress}})
	if codeQuery.Code != 0 || !strings.Contains(string(codeQuery.Value), `"code":"6001"`) {
		t.Fatalf("unexpected code query: %+v", codeQuery)
	}
	callTx := types.Tx("evm:call:evm:0xaaaa:" + deployReceipt.ContractAddress + ":transfer:aabb:100000")
	result = module.DeliverTx(ctx, callTx)
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	var callReceipt Receipt
	if err := json.Unmarshal(result.Data, &callReceipt); err != nil {
		t.Fatal(err)
	}
	if callReceipt.StateDiff == nil || !strings.Contains(mustJSON(t, callReceipt.StateDiff), `"storage"`) {
		t.Fatalf("expected receipt state diff with storage writes, got %+v", callReceipt.StateDiff)
	}
	receiptQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"receipt", callReceipt.TxHash}})
	if receiptQuery.Code != 0 || !strings.Contains(string(receiptQuery.Value), callReceipt.TxHash) {
		t.Fatalf("unexpected receipt query: %+v", receiptQuery)
	}
	logsQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs", deployReceipt.ContractAddress}})
	if logsQuery.Code != 0 || !strings.Contains(string(logsQuery.Value), `"topics":["0x01"]`) {
		t.Fatalf("unexpected logs query: %+v", logsQuery)
	}
	allLogsQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs"}})
	if allLogsQuery.Code != 0 || !strings.Contains(string(allLogsQuery.Value), deployReceipt.ContractAddress) || !strings.Contains(string(allLogsQuery.Value), callReceipt.TxHash) {
		t.Fatalf("unexpected global logs query: %+v", allLogsQuery)
	}
	missingLogsQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs", "0xmissing"}})
	if missingLogsQuery.Code != 0 || strings.TrimSpace(string(missingLogsQuery.Value)) != "[]" {
		t.Fatalf("unexpected missing logs query: %+v", missingLogsQuery)
	}
	storageQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"storage", deployReceipt.ContractAddress, "0x0"}})
	if storageQuery.Code != 0 || !strings.Contains(string(storageQuery.Value), `"value":"0x73746f7265643a7472616e73666572"`) {
		t.Fatalf("unexpected storage query: %+v", storageQuery)
	}
}

func TestModuleEndBlockPersistsReceiptIndexes(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	receipt := Receipt{
		TxHash:  "0xabc123",
		Height:  42,
		Status:  1,
		From:    "0x000000000000000000000000000000000000aaaa",
		To:      "0x000000000000000000000000000000000000bbbb",
		GasUsed: 21,
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	ctx := vexoapp.Context{
		Ctx:       context.Background(),
		Height:    42,
		Store:     storage,
		TxResults: []types.Result{{Data: encoded}},
	}
	if err := module.EndBlock(ctx); err != nil {
		t.Fatal(err)
	}
	query := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"receipt_index", receipt.TxHash}})
	if query.Code != 0 {
		t.Fatalf("unexpected receipt index query: %+v", query)
	}
	var index ReceiptIndex
	if err := json.Unmarshal(query.Value, &index); err != nil {
		t.Fatal(err)
	}
	if index.TxHash != receipt.TxHash || index.Height != 42 || index.TxIndex != 0 {
		t.Fatalf("unexpected receipt index: %+v", index)
	}
}

func TestModuleQueryCallPassesWeb3ExecutionContext(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	vm := &recordingInvocationVM{}
	registry := contract.NewRegistry()
	if err := registry.Register(vm); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}
	if err := persistEthereumStateSnapshot(context.Background(), storage, 77); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(CallRequest{
		VM:       "evm",
		From:     "0x000000000000000000000000000000000000aaaa",
		To:       string(contractAddress),
		Method:   "call",
		Input:    "0x1234",
		GasLimit: 55_000,
		Value:    3,
		Height:   77,
		GasPrice: 9,
		BaseFee:  4,
		AccessList: []contract.AccessListEntry{{
			Address:     contractAddress,
			StorageKeys: []string{"0x01"},
		}},
	})
	response := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 12, Store: storage}, vexoapp.QueryRequest{Path: []string{"call"}, Data: request})
	if response.Code != 0 {
		t.Fatalf("unexpected query response: %+v", response)
	}
	if vm.invocation.BlockNumber != 77 || vm.invocation.GasPrice != 9 || vm.invocation.BaseFee != 4 || vm.invocation.Value != 3 || vm.invocation.GasLimit != 55_000 || vm.invocation.ReadOnly || !vm.invocation.EthereumSimulation {
		t.Fatalf("unexpected invocation context: %+v", vm.invocation)
	}
	if len(vm.invocation.AccessList) != 1 || vm.invocation.AccessList[0].Address != contractAddress || vm.invocation.AccessList[0].StorageKeys[0] != "0x01" {
		t.Fatalf("unexpected invocation access list: %+v", vm.invocation.AccessList)
	}
	var callResponse CallResponse
	if err := json.Unmarshal(response.Value, &callResponse); err != nil {
		t.Fatal(err)
	}
	if len(callResponse.AccessList) != 1 || callResponse.StateDiff == nil || callResponse.VMTrace == nil {
		t.Fatalf("expected call response access list, state diff, and VM trace, got %+v", callResponse)
	}
}

func TestModuleQueryCallAppliesStateBlockOverridesAndAuthorizations(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	vm := &recordingInvocationVM{}
	registry := contract.NewRegistry()
	if err := registry.Register(vm); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}
	setTestEVMBalance(t, storage, contractAddress, 1)
	setTestEVMNonce(t, storage, contractAddress, 2)
	if err := storage.Set(context.Background(), ModuleName, storageKey(contractAddress, "0x01"), []byte{0x01}); err != nil {
		t.Fatal(err)
	}

	nonce := uint64(9)
	request, _ := json.Marshal(CallRequest{
		VM:       "evm",
		From:     string(caller),
		To:       string(contractAddress),
		Method:   "call",
		Input:    "0x",
		GasLimit: 100_000,
		BaseFee:  1,
		StateOverrides: map[string]CallStateOverride{
			string(contractAddress): {
				Balance:   "0x64",
				Nonce:     &nonce,
				Code:      "0x6001",
				StateDiff: map[string]string{"0x01": "0x02"},
			},
		},
		BlockOverride: CallBlockOverride{
			Number:      99,
			Timestamp:   12345,
			GasLimit:    7_000_000,
			BaseFee:     42,
			BlobBaseFee: 43,
		},
		SetCodeAuthorizationsJSON: `[{"chainId":"0x1"}]`,
	})
	response := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 12, Store: storage}, vexoapp.QueryRequest{Path: []string{"call"}, Data: request})
	if response.Code != 0 {
		t.Fatalf("unexpected query response: %+v", response)
	}
	if vm.invocation.BlockNumber != 99 || vm.invocation.Timestamp != 12345 || vm.invocation.BlockGasLimit != 7_000_000 || vm.invocation.BaseFee != 42 || vm.invocation.BlobBaseFee != 43 {
		t.Fatalf("expected block override in invocation, got %+v", vm.invocation)
	}
	if vm.invocation.SetCodeAuthorizationsJSON != `[{"chainId":"0x1"}]` {
		t.Fatalf("expected authorization list to be forwarded, got %q", vm.invocation.SetCodeAuthorizationsJSON)
	}
	if !bytes.Equal(vm.invocation.Code, []byte{0x60, 0x01}) {
		t.Fatalf("expected override code, got %x", vm.invocation.Code)
	}
	reader := vm.invocation.State.(callStateReader)
	balance, err := reader.BalanceBig(context.Background(), contractAddress)
	if err != nil {
		t.Fatal(err)
	}
	readNonce, err := reader.Nonce(context.Background(), contractAddress)
	if err != nil {
		t.Fatal(err)
	}
	value, err := reader.Storage(context.Background(), contractAddress, "0x01")
	if err != nil {
		t.Fatal(err)
	}
	if balance.Uint64() != 100 || readNonce != 9 || !bytes.Equal(value, []byte{0x02}) {
		t.Fatalf("expected override state, balance=%v nonce=%d storage=%x", balance, readNonce, value)
	}
}

func TestModulePersistsNonceWritesIntoEthereumAccountState(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry := contract.NewRegistry()
	if err := registry.Register(nonceVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	caller := types.Address("0x000000000000000000000000000000000000AaAa")
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}

	ctx := vexoapp.Context{Ctx: context.Background(), Height: 4, Store: storage}
	result := module.DeliverTx(ctx, types.Tx("evm:call:evm:"+string(caller)+":"+string(contractAddress)+":call:00:100000"))
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	value, err := storage.Get(context.Background(), "auth", evmNonceKey(caller))
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint64(value) != 8 {
		t.Fatalf("expected persisted nonce 8, got %d", binary.BigEndian.Uint64(value))
	}
	proofRequest, _ := json.Marshal(ProofRequest{Address: string(caller)})
	proofQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: proofRequest})
	if proofQuery.Code != 0 {
		t.Fatalf("unexpected proof query: %+v", proofQuery)
	}
	var proof ethcompat.AccountProof
	if err := json.Unmarshal(proofQuery.Value, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Nonce != "0x8" {
		t.Fatalf("expected proof nonce 0x8, got %+v", proof)
	}
}

func TestModulePersistsUint256EVMBalancesForEthereumProofs(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	large := new(big.Int).Lsh(big.NewInt(1), 80)
	registry := contract.NewRegistry()
	if err := registry.Register(bigBalanceVM{balance: large}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 9, Store: storage}
	result := module.DeliverTx(ctx, types.Tx("evm:call:evm:0x000000000000000000000000000000000000aaaa:"+string(contractAddress)+":call:00:100000"))
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	if err := module.EndBlock(ctx); err != nil {
		t.Fatal(err)
	}
	proofRequest, _ := json.Marshal(ProofRequest{Address: string(contractAddress), Height: 9})
	proofQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: proofRequest})
	if proofQuery.Code != 0 {
		t.Fatalf("unexpected proof query: %+v", proofQuery)
	}
	var proof ethcompat.AccountProof
	if err := json.Unmarshal(proofQuery.Value, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Balance != "0x100000000000000000000" {
		t.Fatalf("expected uint256 balance in proof, got %+v", proof)
	}
}

func TestModulePersistsFailedEVMReceiptWithoutFailingBlock(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry := contract.NewRegistry()
	if err := registry.Register(failingVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 4, Store: storage}
	result := module.DeliverTx(ctx, types.Tx("evm:call:evm:"+string(caller)+":"+string(contractAddress)+":call:00:100000"))
	if result.Code != 0 {
		t.Fatalf("expected failed EVM execution to produce successful block result, got %+v", result)
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != 0 || receipt.Error != "execution reverted" || receipt.GasUsed != 9 {
		t.Fatalf("unexpected failed receipt: %+v", receipt)
	}
}

func TestModuleEstimateEthereumDeployUsesRawTxGasLimit(t *testing.T) {
	vm := &recordingInvocationVM{}
	registry := contract.NewRegistry()
	if err := registry.Register(vm); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	rawTx := signedEthereumCreateTx(t, ethcompat.ChainNumericID("vexo-chain"), "6000")
	decoded, err := ethcompat.DecodeRawTransaction(rawTx, ethcompat.DecodeOptions{ChainID: ethcompat.ChainNumericID("vexo-chain")})
	if err != nil {
		t.Fatal(err)
	}
	ctx := vexoapp.Context{
		Ctx:     context.Background(),
		ChainID: "vexo-chain",
		Height:  1,
		Gas:     vexoapp.NewGasMeter(1_000_000),
	}
	gas, err := module.EstimateGas(ctx, decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	if gas != 17 || vm.invocation.GasLimit != decoded.Gas {
		t.Fatalf("expected estimate to use raw tx gas limit %d, got gas=%d invocation=%+v", decoded.Gas, gas, vm.invocation)
	}
}

func TestModuleQueryCallAcceptsUint256ValueHex(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	vm := &recordingInvocationVM{}
	registry := contract.NewRegistry()
	if err := registry.Register(vm); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}
	value := new(big.Int).Lsh(big.NewInt(1), 80)
	request := CallRequest{
		VM:       "evm",
		From:     "0x000000000000000000000000000000000000aaaa",
		To:       string(contractAddress),
		Method:   "call",
		Input:    "0x00",
		GasLimit: 100000,
		ValueHex: "0x" + value.Text(16),
	}
	encoded, _ := json.Marshal(request)
	response := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 1, Store: storage}, vexoapp.QueryRequest{Path: []string{"call"}, Data: encoded})
	if response.Code != 0 {
		t.Fatalf("query call failed: %+v", response)
	}
	if vm.invocation.Value != 0 || vm.invocation.ValueBig == nil || vm.invocation.ValueBig.Cmp(value) != 0 {
		t.Fatalf("expected uint256 call value, got value=%d big=%v", vm.invocation.Value, vm.invocation.ValueBig)
	}
}

func TestModulePersistsCodeWritesAccountDeletionsAndActualGas(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	registry := contract.NewRegistry()
	if err := registry.Register(deletionVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	contractAddress := types.Address("0x000000000000000000000000000000000000dead")
	if err := storage.Set(context.Background(), ModuleName, codeKey(contractAddress), []byte{0x60, 0x00}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), ModuleName, storageKey(contractAddress, "0x0"), []byte{0x01}); err != nil {
		t.Fatal(err)
	}
	setTestEVMBalance(t, storage, contractAddress, 99)

	ctx := vexoapp.Context{
		Ctx:    context.Background(),
		Height: 12,
		Store:  storage,
		Gas:    vexoapp.NewGasMeter(100),
	}
	result := module.DeliverTx(ctx, types.Tx("evm:call:evm:0x000000000000000000000000000000000000aaaa:"+string(contractAddress)+":call:00:100000"))
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	if result.GasUsed != 11 || ctx.GasUsed() != 11 {
		t.Fatalf("expected actual VM gas to be consumed, result=%d meter=%d", result.GasUsed, ctx.GasUsed())
	}
	if _, err := storage.Get(context.Background(), ModuleName, codeKey(contractAddress)); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected deleted contract code, err=%v", err)
	}
	if _, err := storage.Get(context.Background(), ModuleName, storageKey(contractAddress, "0x0")); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected deleted contract storage, err=%v", err)
	}
	if _, err := storage.Get(context.Background(), "bank", evmBankKey(contractAddress)); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected deleted contract balance, err=%v", err)
	}
	if _, err := storage.Get(context.Background(), "auth", evmNonceKey(contractAddress)); !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatalf("expected deleted contract nonce, err=%v", err)
	}
	codeQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"code", "0x000000000000000000000000000000000000c0de"}})
	if codeQuery.Code != 0 || !strings.Contains(string(codeQuery.Value), `"code":"6001"`) {
		t.Fatalf("expected internal code write, got %+v", codeQuery)
	}
}

func TestModuleQueryCallExecutesReadOnly(t *testing.T) {
	registry := contract.NewRegistry()
	if err := registry.Register(testVM{}); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	request := CallRequest{
		VM:     "evm",
		From:   "0xaaaa",
		To:     "0xbbbb",
		Method: "balanceOf",
		Input:  "0x1234",
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := module.Query(vexoapp.Context{Ctx: context.Background()}, vexoapp.QueryRequest{Path: []string{"call"}, Data: encoded})
	if response.Code != 0 || !strings.Contains(string(response.Value), `"output":"0x6f75743a1234"`) {
		t.Fatalf("unexpected call query: %+v", response)
	}
}

func TestDefaultModuleRunsGethEVMBytecode(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 7, Store: storage}

	initCode := "600a600c600039600a6000f3602a60005260206000f3"
	deployTx := types.Tx("evm:deploy:evm:0x000000000000000000000000000000000000aaaa:" + initCode + ":01")
	result := module.DeliverTx(ctx, deployTx)
	if result.Code != 0 {
		t.Fatalf("deploy failed: %+v", result)
	}
	var deployReceipt Receipt
	if err := json.Unmarshal(result.Data, &deployReceipt); err != nil {
		t.Fatal(err)
	}
	codeQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"code", deployReceipt.ContractAddress}})
	if codeQuery.Code != 0 || !strings.Contains(string(codeQuery.Value), `"code":"602a60005260206000f3"`) {
		t.Fatalf("expected runtime code, got %+v", codeQuery)
	}

	callTx := types.Tx("evm:call:evm:0x000000000000000000000000000000000000aaaa:" + deployReceipt.ContractAddress + ":call:00:100000")
	result = module.DeliverTx(ctx, callTx)
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	var callReceipt Receipt
	if err := json.Unmarshal(result.Data, &callReceipt); err != nil {
		t.Fatal(err)
	}
	if callReceipt.Output != "0x000000000000000000000000000000000000000000000000000000000000002a" {
		t.Fatalf("unexpected EVM output: %+v", callReceipt)
	}
	trace, ok := callReceipt.VMTrace.(map[string]any)
	if !ok || trace["structLogs"] == nil {
		t.Fatalf("expected geth-backed VM trace in receipt, got %+v", callReceipt.VMTrace)
	}
}

func TestDefaultModulePersistsGethEVMStorageWrites(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 8, Store: storage}

	initCode := "6006600c60003960066000f3600160005500"
	deployTx := types.Tx("evm:deploy:evm:0x000000000000000000000000000000000000aaaa:" + initCode + ":02")
	result := module.DeliverTx(ctx, deployTx)
	if result.Code != 0 {
		t.Fatalf("deploy failed: %+v", result)
	}
	var deployReceipt Receipt
	if err := json.Unmarshal(result.Data, &deployReceipt); err != nil {
		t.Fatal(err)
	}
	callTx := types.Tx("evm:call:evm:0x000000000000000000000000000000000000aaaa:" + deployReceipt.ContractAddress + ":call:00:100000")
	result = module.DeliverTx(ctx, callTx)
	if result.Code != 0 {
		t.Fatalf("call failed: %+v", result)
	}
	storageQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"storage", deployReceipt.ContractAddress, "0x0"}})
	if storageQuery.Code != 0 || !strings.Contains(string(storageQuery.Value), `"value":"0x0000000000000000000000000000000000000000000000000000000000000001"`) {
		t.Fatalf("expected EVM SSTORE value, got %+v", storageQuery)
	}
}

func TestDefaultModuleQueryCallAllowsStateChangingSimulationWithoutPersisting(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 8, Store: storage}

	initCode := "6006600c60003960066000f3600160005500"
	deployTx := types.Tx("evm:deploy:evm:0x000000000000000000000000000000000000aaaa:" + initCode + ":03")
	result := module.DeliverTx(ctx, deployTx)
	if result.Code != 0 {
		t.Fatalf("deploy failed: %+v", result)
	}
	var deployReceipt Receipt
	if err := json.Unmarshal(result.Data, &deployReceipt); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(CallRequest{
		VM:       "evm",
		From:     "0x000000000000000000000000000000000000aaaa",
		To:       deployReceipt.ContractAddress,
		Method:   "call",
		Input:    "0x",
		GasLimit: 100_000,
	})
	response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"call"}, Data: request})
	if response.Code != 0 {
		t.Fatalf("expected state-changing eth_call simulation to succeed, got %+v", response)
	}
	var callResponse CallResponse
	if err := json.Unmarshal(response.Value, &callResponse); err != nil {
		t.Fatal(err)
	}
	if callResponse.Failed {
		t.Fatalf("expected call simulation to be non-static, got %+v", callResponse)
	}
	storageQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"storage", deployReceipt.ContractAddress, "0x0"}})
	if storageQuery.Code == 0 {
		t.Fatalf("eth_call simulation must not persist storage writes, got %+v", storageQuery)
	}
}

func TestDefaultModuleQueryCallSupportsContractCreationSimulation(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 8, Store: storage}
	initCode := "600a600c600039600a6000f3602a60005260206000f3"
	request, _ := json.Marshal(CallRequest{
		VM:       "evm",
		From:     "0x000000000000000000000000000000000000aaaa",
		To:       "0x0000000000000000000000000000000000000000",
		Method:   "deploy",
		Input:    "0x" + initCode,
		GasLimit: 100_000,
	})
	response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"call"}, Data: request})
	if response.Code != 0 {
		t.Fatalf("expected contract creation simulation to succeed, got %+v", response)
	}
	var callResponse CallResponse
	if err := json.Unmarshal(response.Value, &callResponse); err != nil {
		t.Fatal(err)
	}
	if callResponse.Output != "0x602a60005260206000f3" {
		t.Fatalf("unexpected create simulation output: %+v", callResponse)
	}
}

func TestModuleQueriesEthereumStateRootAndProof(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 10, Store: storage}
	address := types.Address("0x000000000000000000000000000000000000beef")
	var balance [8]byte
	binary.BigEndian.PutUint64(balance[:], 99)
	if err := storage.Set(context.Background(), "bank", evmBankKey(address), balance[:]); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), ModuleName, codeKey(address), []byte{0x60, 0x2a}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), ModuleName, storageKey(address, "0x01"), []byte{0x2a}); err != nil {
		t.Fatal(err)
	}
	rootQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"eth_state_root"}})
	if rootQuery.Code != 0 || !strings.Contains(string(rootQuery.Value), `"state_root":"0x`) {
		t.Fatalf("unexpected state root query: %+v", rootQuery)
	}
	request, _ := json.Marshal(ProofRequest{Address: string(address), StorageKeys: []string{"0x01"}})
	proofQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: request})
	if proofQuery.Code != 0 {
		t.Fatalf("unexpected proof query: %+v", proofQuery)
	}
	var proof ethcompat.AccountProof
	if err := json.Unmarshal(proofQuery.Value, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Balance != "0x63" || proof.StorageProof[0].Value != "0x2a" || len(proof.AccountProof) == 0 || len(proof.StorageProof[0].Proof) == 0 {
		t.Fatalf("unexpected proof payload: %+v", proof)
	}
}

func TestModulePersistsHistoricalEthereumStateSnapshots(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	address := types.Address("0x000000000000000000000000000000000000beef")
	setTestEVMBalance(t, storage, address, 10)
	setTestEVMNonce(t, storage, address, 1)
	if err := storage.Set(context.Background(), ModuleName, codeKey(address), []byte{0x60, 0x01}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), ModuleName, storageKey(address, "0x01"), []byte{0x01}); err != nil {
		t.Fatal(err)
	}
	if err := module.EndBlock(vexoapp.Context{Ctx: context.Background(), Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	setTestEVMBalance(t, storage, address, 20)
	setTestEVMNonce(t, storage, address, 2)
	if err := storage.Set(context.Background(), ModuleName, codeKey(address), []byte{0x60, 0x02}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(context.Background(), ModuleName, storageKey(address, "0x01"), []byte{0x02}); err != nil {
		t.Fatal(err)
	}
	if err := module.EndBlock(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}); err != nil {
		t.Fatal(err)
	}
	firstRequest, _ := json.Marshal(ProofRequest{Address: string(address), Height: 1})
	firstQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: firstRequest})
	if firstQuery.Code != 0 {
		t.Fatalf("unexpected first proof query: %+v", firstQuery)
	}
	var firstProof ethcompat.AccountProof
	if err := json.Unmarshal(firstQuery.Value, &firstProof); err != nil {
		t.Fatal(err)
	}
	secondRequest, _ := json.Marshal(ProofRequest{Address: string(address), Height: 2})
	secondQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: secondRequest})
	if secondQuery.Code != 0 {
		t.Fatalf("unexpected second proof query: %+v", secondQuery)
	}
	var secondProof ethcompat.AccountProof
	if err := json.Unmarshal(secondQuery.Value, &secondProof); err != nil {
		t.Fatal(err)
	}
	if firstProof.Balance != "0xa" || secondProof.Balance != "0x14" || firstProof.StateRoot == secondProof.StateRoot {
		t.Fatalf("unexpected historical proofs: first=%+v second=%+v", firstProof, secondProof)
	}
	accountRequest, _ := json.Marshal(AccountStateRequest{Height: 1})
	accountQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"account", string(address)}, Data: accountRequest})
	if accountQuery.Code != 0 || !strings.Contains(string(accountQuery.Value), `"balance":10`) || !strings.Contains(string(accountQuery.Value), `"nonce":1`) {
		t.Fatalf("unexpected historical account query: %+v", accountQuery)
	}
	codeRequest, _ := json.Marshal(AccountStateRequest{Height: 1})
	codeQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"code", string(address)}, Data: codeRequest})
	if codeQuery.Code != 0 || !strings.Contains(string(codeQuery.Value), `"code":"6001"`) {
		t.Fatalf("unexpected historical code query: %+v", codeQuery)
	}
	storageRequest, _ := json.Marshal(AccountStateRequest{Height: 1})
	storageQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"storage", string(address), "0x01"}, Data: storageRequest})
	if storageQuery.Code != 0 || !strings.Contains(string(storageQuery.Value), `"value":"0x01"`) {
		t.Fatalf("unexpected historical storage query: %+v", storageQuery)
	}
	rootRequest, _ := json.Marshal(StateRootRequest{Height: 1})
	rootQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"eth_state_root"}, Data: rootRequest})
	if rootQuery.Code != 0 || !strings.Contains(string(rootQuery.Value), firstProof.StateRoot) {
		t.Fatalf("unexpected historical root query: %+v proof=%+v", rootQuery, firstProof)
	}
}

func TestModuleSnapshotsUseStagedStoreOverlay(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	address := types.Address("0x000000000000000000000000000000000000beef")
	setTestEVMBalance(t, storage, address, 10)
	staged := vexoapp.NewStagedStore(storage)
	setTestEVMBalance(t, staged, address, 25)
	if err := module.EndBlock(vexoapp.Context{Ctx: context.Background(), Height: 3, Store: staged}); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(ProofRequest{Address: string(address), Height: 3})
	query := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 3, Store: staged}, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: request})
	if query.Code != 0 {
		t.Fatalf("unexpected staged proof query: %+v", query)
	}
	var proof ethcompat.AccountProof
	if err := json.Unmarshal(query.Value, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Balance != "0x19" {
		t.Fatalf("expected staged balance in snapshot, got %+v", proof)
	}
}

func TestModulePrunesHistoricalEthereumStateSnapshots(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	address := types.Address("0x000000000000000000000000000000000000beef")
	setTestEVMBalance(t, storage, address, 10)
	if err := module.EndBlock(vexoapp.Context{Ctx: context.Background(), Height: 1, Store: storage}); err != nil {
		t.Fatal(err)
	}
	setTestEVMBalance(t, storage, address, 20)
	if err := module.EndBlock(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}); err != nil {
		t.Fatal(err)
	}
	rootBeforePrune, err := storage.Root(context.Background(), ModuleName)
	if err != nil {
		t.Fatal(err)
	}
	if err := module.Prune(vexoapp.Context{Ctx: context.Background(), Store: storage}, 2); err != nil {
		t.Fatal(err)
	}
	rootAfterPrune, err := storage.Root(context.Background(), ModuleName)
	if err != nil {
		t.Fatal(err)
	}
	if rootAfterPrune != rootBeforePrune {
		t.Fatalf("EVM snapshot pruning must not mutate consensus module root: before=%x after=%x", rootBeforePrune, rootAfterPrune)
	}
	firstRequest, _ := json.Marshal(ProofRequest{Address: string(address), Height: 1})
	firstQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: firstRequest})
	if firstQuery.Code == 0 {
		t.Fatalf("expected pruned historical proof to be unavailable, got %+v", firstQuery)
	}
	secondRequest, _ := json.Marshal(ProofRequest{Address: string(address), Height: 2})
	secondQuery := module.Query(vexoapp.Context{Ctx: context.Background(), Height: 2, Store: storage}, vexoapp.QueryRequest{Path: []string{"eth_proof"}, Data: secondRequest})
	if secondQuery.Code != 0 {
		t.Fatalf("expected retained historical proof, got %+v", secondQuery)
	}
}

func TestModulePrunesHistoricalEthereumReceiptsLogsAndBlobSidecars(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := context.Background()
	oldReceipt := Receipt{
		TxHash: "0xold",
		Height: 1,
		Status: 1,
		From:   "0x000000000000000000000000000000000000aaaa",
		To:     "0x000000000000000000000000000000000000bbbb",
		Logs: []Log{{
			Address:         "0x000000000000000000000000000000000000bbbb",
			BlockNumber:     1,
			TransactionHash: "0xold",
			LogIndex:        0,
			Data:            "0x01",
		}},
	}
	newReceipt := Receipt{TxHash: "0xnew", Height: 2, Status: 1, From: oldReceipt.From, To: oldReceipt.To}
	for _, receipt := range []Receipt{oldReceipt, newReceipt} {
		if err := persistReceipt(ctx, storage, receipt); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		if err := persistReceiptIndexes(ctx, storage, []types.Result{{Data: encoded}}); err != nil {
			t.Fatal(err)
		}
	}
	record := BlobSidecarRecord{
		TxHash: "0xold",
		Sidecar: ethcompat.BlobSidecarBundle{
			BlobHashes: []string{"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		},
	}
	encodedRecord, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(ctx, ModuleName, blobSidecarKey(record.TxHash), encodedRecord); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(ctx, ModuleName, blobSidecarHashIndexKey(record.Sidecar.BlobHashes[0]), []byte(record.TxHash)); err != nil {
		t.Fatal(err)
	}
	if err := module.Prune(vexoapp.Context{Ctx: ctx, Store: storage}, 2); err != nil {
		t.Fatal(err)
	}
	for _, key := range [][]byte{
		receiptKey(oldReceipt.TxHash),
		receiptIndexKey(oldReceipt.TxHash),
		globalLogKey(oldReceipt.Logs[0]),
		addressLogKey(oldReceipt.Logs[0]),
		blobSidecarKey(oldReceipt.TxHash),
		blobSidecarHashIndexKey(record.Sidecar.BlobHashes[0]),
	} {
		if _, err := storage.Get(ctx, ModuleName, key); !errors.Is(err, store.ErrKeyNotFound) {
			t.Fatalf("expected old EVM history key %q to be pruned, got %v", key, err)
		}
	}
	if _, err := storage.Get(ctx, ModuleName, receiptKey(newReceipt.TxHash)); err != nil {
		t.Fatalf("expected retained receipt, got %v", err)
	}
	if _, err := storage.Get(ctx, ModuleName, receiptIndexKey(newReceipt.TxHash)); err != nil {
		t.Fatalf("expected retained receipt index, got %v", err)
	}
}

func TestModuleRejectsUnprotectedLegacyEthereumTxUnlessPolicyAllows(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	legacy := gethtypes.NewTransaction(0, to, big.NewInt(1), 21_000, big.NewInt(13), nil)
	signed, err := gethtypes.SignTx(legacy, gethtypes.HomesteadSigner{}, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ethcompat.DecodeRawTransaction("0x"+hex.EncodeToString(raw), ethcompat.DecodeOptions{ChainID: 7, AllowUnprotectedLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	defaultModule, err := NewModuleWithPolicy(Policy{EVMChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := defaultModule.ValidateTx(vexoapp.Context{ChainID: "vexo-chain", EVMChainID: 7}, decoded.Tx); !errors.Is(err, ethcompat.ErrUnprotectedLegacyTx) {
		t.Fatalf("expected default module to reject unprotected legacy tx, got %v", err)
	}
	allowedModule, err := NewModuleWithPolicy(Policy{EVMChainID: 7, AllowUnprotectedLegacyTx: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := allowedModule.ValidateTx(vexoapp.Context{ChainID: "vexo-chain", EVMChainID: 7}, decoded.Tx); err != nil {
		t.Fatalf("expected policy to allow unprotected legacy tx: %v", err)
	}
}

func TestDefaultModuleExecutesEthereumRawContractCreation(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 9, Store: storage}

	initCode := "600a600c600039600a6000f3602a60005260206000f3"
	rawTx := signedEthereumCreateTx(t, ethcompat.ChainNumericID("vexo-chain"), initCode)
	decoded, err := ethcompat.DecodeRawTransaction(rawTx, ethcompat.DecodeOptions{ChainID: ethcompat.ChainNumericID("vexo-chain")})
	if err != nil {
		t.Fatal(err)
	}
	setTestEVMBalance(t, storage, decoded.From, 1_000_000)
	setTestEVMNonce(t, storage, decoded.From, decoded.Nonce)
	result := module.DeliverTx(ctx, decoded.Tx)
	if result.Code != 0 {
		t.Fatalf("ethereum deploy failed: %+v", result)
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.TxHash != decoded.Hash || receipt.To != "" || !strings.EqualFold(receipt.ContractAddress, string(decoded.ContractAddress)) {
		t.Fatalf("unexpected ethereum deploy receipt: %+v decoded=%+v", receipt, decoded)
	}
	codeQuery := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"code", receipt.ContractAddress}})
	if codeQuery.Code != 0 || !strings.Contains(string(codeQuery.Value), `"code":"602a60005260206000f3"`) {
		t.Fatalf("expected Ethereum-created runtime code, got query=%+v receipt=%+v", codeQuery, receipt)
	}
}

func TestModulePersistsAndQueriesBlobSidecar(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	module := NewModule()
	bundle := testBlobSidecarBundle(t)
	encodedSidecar, err := ethcompat.EncodeBlobSidecarBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	encodedHashes, err := json.Marshal(bundle.BlobHashes)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "", "21000", "0"},
		Tags: map[string]string{
			ethcompat.TagHash:        "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ethcompat.TagBlobHashes:  base64.RawStdEncoding.EncodeToString(encodedHashes),
			ethcompat.TagBlobSidecar: encodedSidecar,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := persistBlobSidecar(context.Background(), storage, tx); err != nil {
		t.Fatal(err)
	}
	ctx := vexoapp.Context{Ctx: context.Background(), Store: storage}
	byTx := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"blob_sidecar", "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}})
	if byTx.Code != 0 || !strings.Contains(string(byTx.Value), bundle.BlobHashes[0]) {
		t.Fatalf("expected blob sidecar by tx hash, got %+v", byTx)
	}
	byBlob := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"blob_sidecar_by_hash", bundle.BlobHashes[0]}})
	if byBlob.Code != 0 || string(byBlob.Value) != string(byTx.Value) {
		t.Fatalf("expected blob sidecar by blob hash, got %+v", byBlob)
	}
	noSidecarTx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "", "21000", "0"},
		Tags: map[string]string{
			ethcompat.TagHash:       "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ethcompat.TagBlobHashes: base64.RawStdEncoding.EncodeToString(encodedHashes),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blobSidecarBundleFromTx(noSidecarTx); !errors.Is(err, ethcompat.ErrInvalidBlobSidecar) {
		t.Fatalf("expected blob tx without sidecar to be rejected, got %v", err)
	}
}

func TestModuleRejectsBlobSidecarPolicyViolations(t *testing.T) {
	bundle := testBlobSidecarBundle(t)
	encodedSidecar, err := ethcompat.EncodeBlobSidecarBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	encodedHashes, err := json.Marshal(bundle.BlobHashes)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "", "21000", "0"},
		Tags: map[string]string{
			ethcompat.TagHash:        "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ethcompat.TagBlobHashes:  base64.RawStdEncoding.EncodeToString(encodedHashes),
			ethcompat.TagBlobSidecar: encodedSidecar,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	limitedByBytes, err := NewModuleWithPolicy(Policy{MaxBlobSidecarBlobs: 1, MaxBlobSidecarBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := limitedByBytes.ValidateTx(vexoapp.Context{ChainID: "vexo-chain"}, tx); !errors.Is(err, ethcompat.ErrInvalidBlobSidecar) {
		t.Fatalf("expected sidecar byte limit rejection, got %v", err)
	}
	twoBlobBundle := ethcompat.BlobSidecarBundle{
		BlobHashes:  append(append([]string(nil), bundle.BlobHashes...), bundle.BlobHashes...),
		Blobs:       append(append([]string(nil), bundle.Blobs...), bundle.Blobs...),
		Commitments: append(append([]string(nil), bundle.Commitments...), bundle.Commitments...),
		Proofs:      append(append([]string(nil), bundle.Proofs...), bundle.Proofs...),
	}
	twoBlobSidecar, err := ethcompat.EncodeBlobSidecarBundle(twoBlobBundle)
	if err != nil {
		t.Fatal(err)
	}
	twoBlobHashes, err := json.Marshal(twoBlobBundle.BlobHashes)
	if err != nil {
		t.Fatal(err)
	}
	twoBlobTx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: ModuleName,
		Action: "call",
		Args:   []string{"evm", "0xaaaa", "0xbbbb", "call", "", "21000", "0"},
		Tags: map[string]string{
			ethcompat.TagHash:        "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ethcompat.TagBlobHashes:  base64.RawStdEncoding.EncodeToString(twoBlobHashes),
			ethcompat.TagBlobSidecar: twoBlobSidecar,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	limitedByCount, err := NewModuleWithPolicy(Policy{MaxBlobSidecarBlobs: 1, MaxBlobSidecarBytes: uint64(len(twoBlobSidecar) + 1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := limitedByCount.ValidateTx(vexoapp.Context{ChainID: "vexo-chain"}, twoBlobTx); !errors.Is(err, ethcompat.ErrInvalidBlobSidecar) {
		t.Fatalf("expected sidecar blob count limit rejection, got %v", err)
	}
}

func setTestEVMBalance(t *testing.T, storage vexoapp.StateStore, address types.Address, balance uint64) {
	t.Helper()
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], balance)
	if err := storage.Set(context.Background(), "bank", evmBankKey(address), encoded[:]); err != nil {
		t.Fatal(err)
	}
}

func setTestEVMNonce(t *testing.T, storage vexoapp.StateStore, address types.Address, nonce uint64) {
	t.Helper()
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], nonce)
	if err := storage.Set(context.Background(), "auth", evmNonceKey(address), encoded[:]); err != nil {
		t.Fatal(err)
	}
}

func TestModuleValidateTxRejectsTamperedEthereumRawEconomics(t *testing.T) {
	rawTx := signedEthereumCreateTx(t, ethcompat.ChainNumericID("vexo-chain"), "6000")
	decoded, err := ethcompat.DecodeRawTransaction(rawTx, ethcompat.DecodeOptions{ChainID: ethcompat.ChainNumericID("vexo-chain"), BaseFee: 1})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Tags["fee"] = "1"
	tampered, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := (Module{}).ValidateTx(vexoapp.Context{ChainID: "vexo-chain"}, tampered); !errors.Is(err, ethcompat.ErrSignatureMismatch) {
		t.Fatalf("expected tampered Ethereum raw tx to be rejected, got %v", err)
	}
}

func testBlobSidecarBundle(t *testing.T) ethcompat.BlobSidecarBundle {
	t.Helper()
	var blob kzg4844.Blob
	blob[0] = 1
	blob[31] = 2
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	if err != nil {
		t.Fatal(err)
	}
	versionedHash := gethcommon.Hash(kzg4844.CalcBlobHashV1(sha256.New(), &commitment)).Hex()
	sidecar := &gethtypes.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{blob},
		Commitments: []kzg4844.Commitment{commitment},
		Proofs:      []kzg4844.Proof{proof},
	}
	bundle, err := ethcompat.BlobSidecarBundleFromGeth(sidecar, []string{versionedHash})
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func signedEthereumCreateTx(t *testing.T, chainID uint64, initCode string) string {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	data, err := hex.DecodeString(initCode)
	if err != nil {
		t.Fatal(err)
	}
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     2,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		Gas:       100_000,
		Value:     big.NewInt(0),
		Data:      data,
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

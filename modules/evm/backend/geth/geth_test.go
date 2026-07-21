package geth

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	gethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

type testStateReader struct {
	code     map[types.Address][]byte
	storage  map[string][]byte
	balances map[types.Address]uint64
	big      map[types.Address]*big.Int
	nonces   map[types.Address]uint64
	headers  map[uint64]contract.EthereumHeader
	hashes   map[uint64]types.Hash
	err      error
}

func (reader testStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	return append([]byte(nil), reader.code[address]...), nil
}

func (reader testStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	return append([]byte(nil), reader.storage[string(address)+"/"+slot]...), nil
}

func (reader testStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	return reader.balances[address], nil
}

func (reader testStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	if value := reader.big[address]; value != nil {
		return new(big.Int).Set(value), nil
	}
	return new(big.Int).SetUint64(reader.balances[address]), nil
}

func (reader testStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	if reader.err != nil {
		return 0, reader.err
	}
	return reader.nonces[address], nil
}

func (reader testStateReader) EthereumHeader(ctx context.Context, height uint64) (contract.EthereumHeader, error) {
	if reader.err != nil {
		return contract.EthereumHeader{}, reader.err
	}
	header, found := reader.headers[height]
	if !found {
		return contract.EthereumHeader{}, errors.New("header not found")
	}
	return header, nil
}

func (reader testStateReader) BlockHash(ctx context.Context, height uint64) (types.Hash, error) {
	if reader.err != nil {
		return types.Hash{}, reader.err
	}
	hash, found := reader.hashes[height]
	if !found {
		return types.Hash{}, errors.New("hash not found")
	}
	return hash, nil
}

func TestEthereumMessagePreservesSimulationFlagsForRawTx(t *testing.T) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	chainID := big.NewInt(77)
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000beef")
	tx := gethtypes.NewTransaction(7, to, big.NewInt(0), 21_000, big.NewInt(1), nil)
	signed, err := gethtypes.SignTx(tx, gethtypes.NewEIP155Signer(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	message, err := ethereumMessage(contract.Invocation{
		RawEthereumTx:      "0x" + hex.EncodeToString(raw),
		BaseFee:            1,
		EthereumSimulation: true,
	}, gethcrypto.PubkeyToAddress(key.PublicKey), to, uint256.NewInt(0), 21_000)
	if err != nil {
		t.Fatal(err)
	}
	if !message.SkipNonceChecks || !message.SkipTransactionChecks {
		t.Fatalf("raw transaction dropped simulation flags: %+v", message)
	}
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

func TestGethBackendLatestPresetSupportsCurrentSolidityMcopy(t *testing.T) {
	code := []byte{
		0x60, 0x2a, 0x60, 0x00, 0x52, // mstore(0, 42)
		0x60, 0x20, 0x60, 0x00, 0x60, 0x20, 0x5e, // mcopy(32, 0, 32)
		0x60, 0x20, 0x60, 0x20, 0xf3, // return(32, 32)
	}
	londonResult, err := New().Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   "0x000000000000000000000000000000000000aaaa",
		Contract: "0x000000000000000000000000000000000000bbbb",
		Code:     code,
		GasLimit: 100_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !londonResult.Failed {
		t.Fatal("London must reject Cancun MCOPY bytecode")
	}

	latest, err := NewWithChainConfigPresetJSON(LatestForkPreset, "", 83960)
	if err != nil {
		t.Fatal(err)
	}
	result, err := latest.Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   "0x000000000000000000000000000000000000aaaa",
		Contract: "0x000000000000000000000000000000000000bbbb",
		Code:     code,
		GasLimit: 100_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed || len(result.Output) != 32 || result.Output[31] != 0x2a {
		t.Fatalf("expected current Solidity bytecode to return 42, got failed=%v error=%q output=%x", result.Failed, result.Error, result.Output)
	}
}

func TestRunExecutionFixturesJSON(t *testing.T) {
	raw := []byte(`{
		"schema_version": "v1",
		"required_categories": ["call_return"],
		"fixtures": [
			{
				"name": "json call returns 42",
				"method": "call",
				"code": "0x602a60005260206000f3",
				"gas_limit": 100000,
				"want_output": "0x000000000000000000000000000000000000000000000000000000000000002a",
				"categories": ["call_return"]
			}
		]
	}`)
	report, err := RunExecutionFixturesJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.CoverageOK || report.Passed != 1 || report.Failed != 0 {
		t.Fatalf("unexpected execution fixture report: %+v", report)
	}
}

func TestGethBackendPassesBlobHashesToEVM(t *testing.T) {
	vm, err := NewWithChainConfigPresetJSON(LatestForkPreset, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	blobHash := types.Hash(gethcommon.HexToHash("0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
	result, err := vm.Execute(context.Background(), contract.Invocation{
		Method:      "call",
		Caller:      "0x000000000000000000000000000000000000aaaa",
		Contract:    "0x000000000000000000000000000000000000bbbb",
		Code:        []byte{0x60, 0x00, 0x49, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3},
		GasLimit:    100_000,
		BlockNumber: 1,
		Timestamp:   1,
		BlobHashes:  []types.Hash{blobHash},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed {
		t.Fatalf("expected blobhash opcode to succeed: %s", result.Error)
	}
	if len(result.Output) != 32 || gethcommon.BytesToHash(result.Output) != gethcommon.Hash(blobHash) {
		t.Fatalf("expected blobhash output %x, got %x", blobHash, result.Output)
	}
}

func TestGethBackendEthereumTxUsesStateTransition(t *testing.T) {
	vm := New()
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	coinbase := types.Address("0x000000000000000000000000000000000000c0fe")
	result, err := vm.Execute(context.Background(), contract.Invocation{
		Method:        "call",
		Caller:        caller,
		Contract:      "0x000000000000000000000000000000000000bbbb",
		GasLimit:      21_000,
		GasPrice:      2,
		GasFeeCap:     2,
		GasTipCap:     2,
		Nonce:         7,
		Coinbase:      coinbase,
		EthereumTx:    true,
		BlockGasLimit: 100_000,
		State: testStateReader{
			balances: map[types.Address]uint64{caller: 100_000},
			nonces:   map[types.Address]uint64{caller: 7},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed || result.GasUsed != 21_000 {
		t.Fatalf("expected successful Ethereum state transition, got %+v", result)
	}
	seenNonce := false
	for _, write := range result.NonceWrites {
		if write.Address == caller && write.Nonce == 8 {
			seenNonce = true
		}
	}
	if !seenNonce {
		t.Fatalf("expected Ethereum nonce increment, got %+v", result.NonceWrites)
	}
	balances := map[types.Address]uint64{}
	for _, write := range result.BalanceWrites {
		balances[types.Address(strings.ToLower(string(write.Address)))] = write.Balance
	}
	if balances[types.Address(strings.ToLower(string(caller)))] != 58_000 || balances[types.Address(strings.ToLower(string(coinbase)))] != 42_000 {
		t.Fatalf("expected gas fee transfer through state transition, got %+v", result.BalanceWrites)
	}
}

func TestGethBackendFailsClosedOnStateReaderError(t *testing.T) {
	expected := errors.New("reader failed")
	_, err := New().Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   "0x000000000000000000000000000000000000aaaa",
		Contract: "0x000000000000000000000000000000000000bbbb",
		GasLimit: 21_000,
		State:    testStateReader{err: expected},
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected reader error, got %v", err)
	}
}

func TestGethBackendRejectsHardStateTransitionErrors(t *testing.T) {
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	contractAddress := types.Address("0x000000000000000000000000000000000000bbbb")
	_, err := New().Execute(context.Background(), contract.Invocation{
		Method:        "call",
		Caller:        caller,
		Contract:      contractAddress,
		GasLimit:      1,
		GasPrice:      1,
		EthereumTx:    true,
		BlockGasLimit: 100_000,
		State: testStateReader{
			balances: map[types.Address]uint64{caller: 100_000},
		},
	})
	if err == nil {
		t.Fatal("expected intrinsic gas validation error")
	}
}

func TestGethBackendUsesLegacyCreateWhenSaltIsMissing(t *testing.T) {
	vm := New()
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	result, err := vm.Execute(context.Background(), contract.Invocation{
		Method:   "deploy",
		Caller:   caller,
		Contract: "0x0000000000000000000000000000000000000000",
		Input:    []byte{0x60, 0x0a, 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, 0x0a, 0x60, 0x00, 0xf3, 0x60, 0x2a, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3},
		GasLimit: 100_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := gethcrypto.CreateAddress(gethcommon.HexToAddress(string(caller)), 0).Hex()
	if len(result.CodeWrites) != 1 || !strings.EqualFold(string(result.CodeWrites[0].Address), expected) {
		t.Fatalf("expected legacy CREATE address %s, got %+v", expected, result.CodeWrites)
	}
}

func TestGethBackendImplementsContractVM(t *testing.T) {
	var _ contract.VM = New()
}

func TestGoEthereumDependencyInfo(t *testing.T) {
	info := GoEthereumDependencyInfo()
	if info.Module != GoEthereumModulePath || info.Version == "" {
		t.Fatalf("unexpected go-ethereum dependency info: %+v", info)
	}
}

func TestGethBackendChainConfigJSONValidation(t *testing.T) {
	vm, err := NewWithChainConfigJSON(`{}`, 777)
	if err != nil {
		t.Fatal(err)
	}
	if vm.activeChainConfig().ChainID == nil || vm.activeChainConfig().ChainID.Uint64() != 777 {
		t.Fatalf("expected injected chain id 777, got %+v", vm.activeChainConfig().ChainID)
	}
	vm, err = NewWithChainConfigJSON(`{"chainId":888}`, 777)
	if err != nil {
		t.Fatal(err)
	}
	if vm.activeChainConfig().ChainID == nil || vm.activeChainConfig().ChainID.Uint64() != 888 {
		t.Fatalf("expected JSON chain id 888 to win, got %+v", vm.activeChainConfig().ChainID)
	}
	if _, err := NewWithChainConfigJSON(`{`, 1); err == nil {
		t.Fatalf("expected invalid JSON failure")
	}
	if _, err := NewWithChainConfigJSON(`{"chainId":1,"shanghaiTime":1,"londonBlock":2}`, 1); err == nil {
		t.Fatalf("expected fork order validation failure")
	}
	if normalizedChainConfig(nil) != VexoDefaultChainConfig || VexoDefaultChainConfig != VexoLondonChainConfig {
		t.Fatalf("expected nil config to normalize to Vexo london protocol changes")
	}
	custom := &gethparams.ChainConfig{ChainID: big.NewInt(999)}
	if normalizedChainConfig(custom) != custom {
		t.Fatalf("expected custom config pointer to be preserved")
	}
}

func TestGethBackendChainConfigPresetSelection(t *testing.T) {
	london, err := ChainConfigForPreset(DefaultForkPreset, 777)
	if err != nil {
		t.Fatal(err)
	}
	if london.ChainID == nil || london.ChainID.Uint64() != 777 || london.LondonBlock == nil || london.LondonBlock.Uint64() != 0 || london.ShanghaiTime != nil || london.CancunTime != nil {
		t.Fatalf("unexpected london preset config: %+v", london)
	}
	latest, err := ChainConfigForPreset(LatestForkPreset, 888)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ChainID == nil || latest.ChainID.Uint64() != 888 || latest.LondonBlock == nil || latest.LondonBlock.Uint64() != 0 || latest.ShanghaiTime == nil || latest.CancunTime == nil {
		t.Fatalf("unexpected latest preset config: %+v", latest)
	}
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

func TestGethBackendPreservesUint256BalanceWrites(t *testing.T) {
	vm := New()
	caller := types.Address("0x000000000000000000000000000000000000aaaa")
	recipient := types.Address("0x000000000000000000000000000000000000bbbb")
	large := new(big.Int).Lsh(big.NewInt(1), 80)
	result, err := vm.Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   caller,
		Contract: recipient,
		GasLimit: 100_000,
		Value:    5,
		State: testStateReader{
			big: map[types.Address]*big.Int{caller: large},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var callerWrite contract.BalanceWrite
	for _, write := range result.BalanceWrites {
		if strings.EqualFold(string(write.Address), string(caller)) {
			callerWrite = write
		}
	}
	expected := new(big.Int).Sub(large, big.NewInt(5))
	if callerWrite.BalanceBig == nil || callerWrite.BalanceBig.Cmp(expected) != 0 {
		t.Fatalf("expected caller big balance %s, got %+v", expected, result.BalanceWrites)
	}
}

func TestGethStateDBFinaliseReturnsStateAccessList(t *testing.T) {
	stateDB := newGethStateDB(context.Background(), contract.Invocation{})
	address := gethcommon.HexToAddress("0x000000000000000000000000000000000000aaaa")
	slot := gethcommon.HexToHash("0x01")
	stateDB.AddAddressToAccessList(address)
	stateDB.AddSlotToAccessList(address, slot)
	stateDB.Touch(address)

	accessList := stateDB.Finalise(true)
	if accessList == nil {
		t.Fatal("expected non-nil state access list")
	}
	if accessList.Copy() == nil {
		t.Fatal("expected copyable state access list")
	}
}

func TestGethStateDBDirectShimMethods(t *testing.T) {
	address := types.Address("0x000000000000000000000000000000000000aaaa")
	gethAddress := gethcommon.HexToAddress(string(address))
	slot := gethcommon.HexToHash("0x01")
	initialStorage := gethcommon.HexToHash("0x02")
	blockHash := types.Hash{9}
	db := newGethStateDB(context.Background(), contract.Invocation{
		State: testStateReader{
			code:    map[types.Address][]byte{address: {0x60, 0x00}},
			storage: map[string][]byte{string(address) + "/" + slot.Hex(): initialStorage.Bytes()},
			hashes:  map[uint64]types.Hash{7: blockHash},
		},
	})

	if size := db.GetCodeSize(gethAddress); size != 2 {
		t.Fatalf("expected code size 2, got %d", size)
	}
	db.AddRefund(10)
	db.SubRefund(3)
	if refund := db.GetRefund(); refund != 7 {
		t.Fatalf("expected refund 7, got %d", refund)
	}
	db.SubRefund(100)
	if refund := db.GetRefund(); refund != 0 {
		t.Fatalf("expected refund floor at zero, got %d", refund)
	}
	current, committed := db.GetStateAndCommittedState(gethAddress, slot)
	if current != initialStorage || committed != initialStorage {
		t.Fatalf("unexpected loaded storage current=%s committed=%s", current, committed)
	}
	nextStorage := gethcommon.HexToHash("0x03")
	if previous := db.SetState(gethAddress, slot, nextStorage); previous != initialStorage {
		t.Fatalf("expected previous storage %s, got %s", initialStorage, previous)
	}
	db.SetTransientState(gethAddress, slot, gethcommon.HexToHash("0x04"))
	if transient := db.GetTransientState(gethAddress, slot); transient != gethcommon.HexToHash("0x04") {
		t.Fatalf("unexpected transient storage %s", transient)
	}
	if db.HasSelfDestructed(gethAddress) {
		t.Fatal("unexpected self-destruct before marker")
	}
	db.SelfDestruct(gethAddress)
	if !db.HasSelfDestructed(gethAddress) {
		t.Fatal("expected self-destruct marker")
	}
	db.CreateContract(gethAddress)
	if !db.IsNewContract(gethAddress) || db.Empty(gethAddress) {
		t.Fatal("expected new non-empty contract")
	}
	if db.AddressInAccessList(gethAddress) {
		t.Fatal("unexpected access-list address before prepare")
	}
	db.AddSlotToAccessList(gethAddress, slot)
	addressOK, slotOK := db.SlotInAccessList(gethAddress, slot)
	if !db.AddressInAccessList(gethAddress) || !addressOK || !slotOK {
		t.Fatal("expected address and slot in access list")
	}
	snapshot := db.Snapshot()
	db.AddRefund(5)
	db.SetState(gethAddress, slot, gethcommon.HexToHash("0x05"))
	db.RevertToSnapshot(snapshot)
	if db.GetRefund() != 0 || db.GetState(gethAddress, slot) != nextStorage {
		t.Fatalf("expected snapshot revert refund/storage, refund=%d storage=%s", db.GetRefund(), db.GetState(gethAddress, slot))
	}
	db.RevertToSnapshot(99)
	db.AddLog(&gethtypes.Log{Address: gethAddress, Topics: []gethcommon.Hash{slot}, Data: []byte{1}})
	db.AddLog(nil)
	if logs := db.ContractLogs(); len(logs) != 1 || !strings.EqualFold(string(logs[0].Address), gethAddress.Hex()) {
		t.Fatalf("unexpected logs: %+v", logs)
	}
	db.AddPreimage(slot, []byte("preimage"))
	if events := db.AccessEvents(); events == nil {
		t.Fatal("expected access events")
	}
	if got := db.blockHash(7); got != gethcommon.Hash(blockHash) {
		t.Fatalf("expected block hash %s, got %s", gethcommon.Hash(blockHash), got)
	}
	if got := db.blockHash(8); got != (gethcommon.Hash{}) {
		t.Fatalf("expected missing block hash to return zero, got %s", got)
	}
}

func TestGethStateDBEnablesWitnessWhenHeaderReaderIsAvailable(t *testing.T) {
	parentHash := types.Hash{1}
	stateDB := newGethStateDB(context.Background(), contract.Invocation{
		BlockNumber: 2,
		State: testStateReader{
			headers: map[uint64]contract.EthereumHeader{
				1: {Hash: parentHash, Number: 1, StateRoot: types.Hash{3}},
				2: {ParentHash: parentHash, Number: 2, StateRoot: types.Hash{4}},
			},
		},
	})
	if stateDB.Witness() == nil {
		t.Fatal("expected witness when EthereumHeaderReader is available")
	}
}

func TestGethBackendRejectsValueOverflow(t *testing.T) {
	vm := New()
	_, err := vm.Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   "0x000000000000000000000000000000000000aaaa",
		Contract: "0x000000000000000000000000000000000000bbbb",
		GasLimit: 100_000,
		ValueBig: new(big.Int).Lsh(big.NewInt(1), 256),
	})
	if !errors.Is(err, contract.ErrInvalidInvocation) {
		t.Fatalf("expected invalid invocation, got %v", err)
	}
}

func TestGethStateDBFinaliseReportsCodeStorageAndAccountDeletion(t *testing.T) {
	address := types.Address("0x000000000000000000000000000000000000dead")
	slot := "0x00000000000000000000000000000000000000000000000000000000000000000"
	db := newGethStateDB(context.Background(), contract.Invocation{})
	gethAddress := gethAddress(address)
	account := db.account(gethAddress)
	account.code = []byte{0x60, 0x00}
	account.committedCode = []byte{0x60, 0x00}
	account.storage[gethcommon.HexToHash(slot)] = gethcommon.HexToHash("0x01")
	account.committed[gethcommon.HexToHash(slot)] = gethcommon.HexToHash("0x01")
	db.SelfDestruct(gethAddress)
	db.Finalise(true)

	if deletions := db.AccountDeletions(); len(deletions) != 1 || !strings.EqualFold(string(deletions[0].Address), string(address)) {
		t.Fatalf("unexpected account deletions: %+v", deletions)
	}
	if writes := db.CodeWrites(); len(writes) != 1 || !writes[0].Delete || !strings.EqualFold(string(writes[0].Address), string(address)) {
		t.Fatalf("unexpected code writes: %+v", writes)
	}
	storageWrites := db.StorageWrites()
	if len(storageWrites) != 1 || !storageWrites[0].Delete || !strings.EqualFold(string(storageWrites[0].Address), string(address)) {
		t.Fatalf("unexpected storage writes: %+v", storageWrites)
	}
}

func TestGethStateDBLogsForBurnAccounts(t *testing.T) {
	db := newGethStateDB(context.Background(), contract.Invocation{})
	first := gethcommon.HexToAddress("0x0000000000000000000000000000000000000001")
	second := gethcommon.HexToAddress("0x0000000000000000000000000000000000000002")
	db.CreateAccount(second)
	db.CreateAccount(first)
	db.AddBalance(second, new(uint256.Int).SetUint64(7), 0)
	db.AddBalance(first, new(uint256.Int).SetUint64(3), 0)
	db.SelfDestruct(second)
	db.SelfDestruct(first)

	logs := db.LogsForBurnAccounts()
	if len(logs) != 2 {
		t.Fatalf("expected two burn logs, got %+v", logs)
	}
	if len(logs[0].Topics) < 2 || len(logs[1].Topics) < 2 ||
		gethcommon.BytesToAddress(logs[0].Topics[1].Bytes()) != first ||
		gethcommon.BytesToAddress(logs[1].Topics[1].Bytes()) != second {
		t.Fatalf("expected burn logs sorted by address, got %+v", logs)
	}
	if logs[0].Data == nil || logs[1].Data == nil {
		t.Fatalf("expected burn logs to include encoded balance data, got %+v", logs)
	}
}

func TestGethStateDBBalanceUnderflowFailsClosed(t *testing.T) {
	db := newGethStateDB(context.Background(), contract.Invocation{})
	address := gethcommon.HexToAddress("0x000000000000000000000000000000000000aaaa")
	db.AddBalance(address, new(uint256.Int).SetUint64(3), 0)
	db.SubBalance(address, new(uint256.Int).SetUint64(5), 0)
	if !errors.Is(db.err, contract.ErrInvalidInvocation) {
		t.Fatalf("expected balance underflow to fail closed, got %v", db.err)
	}
	if balance := db.GetBalance(address); !balance.IsZero() {
		t.Fatalf("expected underflowed account to be zeroed, got %s", balance.ToBig())
	}
}

func TestGethStateDBReportsNonceWrites(t *testing.T) {
	address := types.Address("0x000000000000000000000000000000000000aaaa")
	db := newGethStateDB(context.Background(), contract.Invocation{
		State: testStateReader{
			nonces: map[types.Address]uint64{address: 2},
		},
	})
	gethAddress := gethAddress(address)
	if nonce := db.GetNonce(gethAddress); nonce != 2 {
		t.Fatalf("expected loaded nonce 2, got %d", nonce)
	}
	db.SetNonce(gethAddress, 3, 0)

	writes := db.NonceWrites()
	if len(writes) != 1 || writes[0].Nonce != 3 || !strings.EqualFold(string(writes[0].Address), string(address)) {
		t.Fatalf("unexpected nonce writes: %+v", writes)
	}
}

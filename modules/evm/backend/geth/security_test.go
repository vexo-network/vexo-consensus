package geth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtracing "github.com/ethereum/go-ethereum/core/tracing"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	securityCaller = types.Address("0x000000000000000000000000000000000000aaaa")
	securityTarget = types.Address("0x000000000000000000000000000000000000bbbb")
	securityInner  = types.Address("0x000000000000000000000000000000000000cccc")
)

// loopCode jumps to itself forever: JUMPDEST PUSH1 0 JUMP.
var loopCode = []byte{0x5b, 0x60, 0x00, 0x56}

func TestExecutionRecordsNoOpcodeTraceUnlessAsked(t *testing.T) {
	invocation := contract.Invocation{
		Method:   "call",
		Caller:   securityCaller,
		Contract: securityTarget,
		Code:     loopCode,
		GasLimit: 2_000,
	}
	result, err := New().Execute(context.Background(), invocation)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Failed || result.VMTrace != nil {
		t.Fatalf("execution without a trace request must not trace, got failed=%v trace=%v", result.Failed, result.VMTrace)
	}

	invocation.Trace = true
	traced, err := New().Execute(context.Background(), invocation)
	if err != nil {
		t.Fatal(err)
	}
	trace, ok := traced.VMTrace.(map[string]any)
	if !ok || len(trace["structLogs"].([]any)) == 0 {
		t.Fatalf("explicit trace request must produce struct logs, got %+v", traced.VMTrace)
	}
}

func TestExecutionTraceSizeIsBounded(t *testing.T) {
	// The loop produces roughly three struct logs per twelve gas, far more than
	// the trace budget holds; an unbounded logger would keep every one of them.
	result, err := New().Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   securityCaller,
		Contract: securityTarget,
		Code:     loopCode,
		GasLimit: 6_000_000,
		Trace:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.VMTrace)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 || len(encoded) > 2*maxTraceBytes {
		t.Fatalf("trace must stay within its %d byte budget, got %d bytes", maxTraceBytes, len(encoded))
	}
	if !result.Failed {
		t.Fatal("the loop must still run to out-of-gas: tracing never changes execution")
	}
}

func TestExecutionStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := New().Execute(ctx, contract.Invocation{
		Method:   "call",
		Caller:   securityCaller,
		Contract: securityTarget,
		Code:     loopCode,
		GasLimit: 1 << 40,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the context error from a cancelled execution, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("a cancelled execution must stop promptly, ran for %s", elapsed)
	}
}

func TestStateTransitionPreCheckFailuresAreRejections(t *testing.T) {
	base := contract.Invocation{
		Method:        "call",
		Caller:        securityCaller,
		Contract:      securityTarget,
		GasLimit:      21_000,
		GasPrice:      2,
		GasFeeCap:     2,
		GasTipCap:     2,
		Nonce:         7,
		EthereumTx:    true,
		BlockGasLimit: 100_000,
		State: testStateReader{
			balances: map[types.Address]uint64{securityCaller: 1_000_000},
			nonces:   map[types.Address]uint64{securityCaller: 7},
		},
	}
	if _, err := New().Execute(context.Background(), base); err != nil {
		t.Fatalf("control transaction must execute: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*contract.Invocation)
	}{
		{name: "nonce too high", mutate: func(i *contract.Invocation) { i.Nonce = 8 }},
		{name: "nonce too low", mutate: func(i *contract.Invocation) { i.Nonce = 6 }},
		{name: "insufficient funds", mutate: func(i *contract.Invocation) {
			i.State = testStateReader{
				balances: map[types.Address]uint64{securityCaller: 1},
				nonces:   map[types.Address]uint64{securityCaller: 7},
			}
		}},
		{name: "intrinsic gas too low", mutate: func(i *contract.Invocation) { i.GasLimit = 1 }},
		{name: "fee cap below base fee", mutate: func(i *contract.Invocation) { i.BaseFee = 100 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invocation := base
			tt.mutate(&invocation)
			_, err := New().Execute(context.Background(), invocation)
			if err == nil {
				t.Fatal("expected the pre-check failure to reject the invocation")
			}
			if !errors.Is(err, contract.ErrTxRejected) {
				t.Fatalf("a pre-check failure is a deterministic rejection, not an infrastructure error: %v", err)
			}
		})
	}
}

func TestStateReaderFailureIsNotARejection(t *testing.T) {
	expected := errors.New("reader failed")
	_, err := New().Execute(context.Background(), contract.Invocation{
		Method:     "call",
		Caller:     securityCaller,
		Contract:   securityTarget,
		GasLimit:   21_000,
		EthereumTx: true,
		State:      testStateReader{err: expected},
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected the reader error, got %v", err)
	}
	if errors.Is(err, contract.ErrTxRejected) {
		t.Fatalf("an infrastructure failure must never be recorded as a transaction outcome: %v", err)
	}
}

func TestRawTransactionGasLimitCannotExceedExecutionLimit(t *testing.T) {
	raw, sender := signedTestTransfer(t, 9, 100_000)
	_, err := New().Execute(context.Background(), contract.Invocation{
		Method:        "call",
		Caller:        types.Address(sender.Hex()),
		Contract:      securityTarget,
		RawEthereumTx: raw,
		GasLimit:      50_000,
		EthereumTx:    true,
		BlockGasLimit: 10_000_000,
		State: testStateReader{
			balances: map[types.Address]uint64{types.Address(sender.Hex()): 1 << 40},
		},
	})
	if !errors.Is(err, contract.ErrInvalidInvocation) {
		t.Fatalf("a raw transaction asking for more gas than the execution limit must be refused, got %v", err)
	}
}

// outerCallerCode is the runtime code of the contract under test: SSTORE(0, 1)
// followed by a CALL, carrying value wei, into the contract at securityInner
// whose outcome it ignores.
func outerCallerCode(value byte) []byte {
	code := []byte{0x60, 0x01, 0x60, 0x00, 0x55}                        // SSTORE(0, 1)
	code = append(code, 0x60, 0x00, 0x60, 0x00, 0x60, 0x00, 0x60, 0x00) // retSize retOffset argsSize argsOffset
	code = append(code, 0x60, value)                                    // value
	code = append(code, 0x73)                                           // PUSH20 inner
	code = append(code, gethcommon.HexToAddress(string(securityInner)).Bytes()...)
	return append(code, 0x5a, 0xf1, 0x50, 0x00) // GAS CALL POP STOP
}

// revertingCode writes storage, emits a log and then reverts:
// SSTORE(1, 2) LOG0(0, 0) REVERT(0, 0).
var revertingCode = []byte{0x60, 0x02, 0x60, 0x01, 0x55, 0x60, 0x00, 0x60, 0x00, 0xa0, 0x60, 0x00, 0x60, 0x00, 0xfd}

func TestRevertedSubCallUndoesStorageLogsAndTransfers(t *testing.T) {
	result, err := New().Execute(context.Background(), contract.Invocation{
		Method:   "call",
		Caller:   securityCaller,
		Contract: securityTarget,
		Code:     outerCallerCode(1),
		GasLimit: 1_000_000,
		State: testStateReader{
			code:     map[types.Address][]byte{securityInner: revertingCode},
			balances: map[types.Address]uint64{securityTarget: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed {
		t.Fatalf("the outer frame succeeds when the inner call reverts: %+v", result)
	}
	if len(result.Logs) != 0 {
		t.Fatalf("the reverted frame's log must be undone, got %+v", result.Logs)
	}
	if len(result.StorageWrites) != 1 || result.StorageWrites[0].Slot != (gethcommon.Hash{}).Hex() {
		t.Fatalf("only the outer SSTORE may survive, got %+v", result.StorageWrites)
	}
	if len(result.BalanceWrites) != 0 {
		t.Fatalf("the reverted value transfer must be undone, got %+v", result.BalanceWrites)
	}
}

func TestSnapshotRevertRestoresEveryKindOfState(t *testing.T) {
	address := gethcommon.HexToAddress(string(securityTarget))
	other := gethcommon.HexToAddress(string(securityInner))
	slot := gethcommon.HexToHash("0x01")
	// The adapter queries its reader with checksummed addresses.
	key := types.Address(address.Hex())
	db := newGethStateDB(context.Background(), contract.Invocation{State: testStateReader{
		balances: map[types.Address]uint64{key: 100},
		code:     map[types.Address][]byte{key: {0x60, 0x01}},
		nonces:   map[types.Address]uint64{key: 3},
		storage:  map[string][]byte{string(key) + "/" + slot.Hex(): gethcommon.HexToHash("0x0a").Bytes()},
	}})
	db.SetState(address, slot, gethcommon.HexToHash("0x0b"))
	db.AddLog(&gethtypes.Log{Address: address})
	outer := db.Snapshot()

	db.SubBalance(address, uint256.NewInt(40), gethtracing.BalanceChangeTransfer)
	db.AddBalance(other, uint256.NewInt(40), gethtracing.BalanceChangeTransfer)
	db.SetNonce(address, 9, gethtracing.NonceChangeUnspecified)
	db.SetCode(address, []byte{0x00}, gethtracing.CodeChangeUnspecified)
	db.SetState(address, slot, gethcommon.HexToHash("0x0c"))
	db.SetTransientState(address, slot, gethcommon.HexToHash("0x0d"))
	db.AddRefund(77)
	db.AddLog(&gethtypes.Log{Address: other})
	db.SelfDestruct(address)
	db.CreateContract(other)
	db.Touch(other)
	db.AddSlotToAccessList(other, slot)

	inner := db.Snapshot()
	db.SetState(address, slot, gethcommon.HexToHash("0x0e"))
	db.RevertToSnapshot(inner)
	if got := db.GetState(address, slot); got != gethcommon.HexToHash("0x0c") {
		t.Fatalf("nested revert must restore only the inner frame, got %s", got)
	}

	db.RevertToSnapshot(outer)
	if got := db.GetBalance(address); got.Uint64() != 100 {
		t.Fatalf("balance not restored: %s", got)
	}
	if got := db.GetBalance(other); !got.IsZero() {
		t.Fatalf("credit not undone: %s", got)
	}
	if got := db.GetNonce(address); got != 3 {
		t.Fatalf("nonce not restored: %d", got)
	}
	if got := db.GetCode(address); len(got) != 2 {
		t.Fatalf("code not restored: %x", got)
	}
	if got := db.GetState(address, slot); got != gethcommon.HexToHash("0x0b") {
		t.Fatalf("storage must return to its value at the snapshot, got %s", got)
	}
	if got := db.GetTransientState(address, slot); got != (gethcommon.Hash{}) {
		t.Fatalf("transient storage not restored: %s", got)
	}
	if db.GetRefund() != 0 {
		t.Fatalf("refund not restored: %d", db.GetRefund())
	}
	if logs := db.ContractLogs(); len(logs) != 1 {
		t.Fatalf("log added after the snapshot must be dropped, got %+v", logs)
	}
	if db.HasSelfDestructed(address) || db.IsNewContract(other) {
		t.Fatal("self-destruct and creation markers not restored")
	}
	if db.AddressInAccessList(other) {
		t.Fatal("access list entry added after the snapshot must be dropped")
	}
}

func TestSnapshotDoesNotCopyState(t *testing.T) {
	db := newGethStateDB(context.Background(), contract.Invocation{})
	for index := 0; index < 5_000; index++ {
		address := gethcommon.BigToAddress(uint256.NewInt(uint64(index + 1)).ToBig())
		db.AddBalance(address, uint256.NewInt(1), gethtracing.BalanceChangeTransfer)
		db.SetState(address, gethcommon.HexToHash("0x01"), gethcommon.HexToHash("0x02"))
	}
	// A snapshot that deep-copied the touched state would allocate a map entry
	// per account and slot: thousands of allocations per call.
	allocations := testing.AllocsPerRun(50, func() {
		id := db.Snapshot()
		db.RevertToSnapshot(id)
	})
	if allocations > 2 {
		t.Fatalf("Snapshot/RevertToSnapshot must be proportional to the work done in the frame, got %.0f allocations", allocations)
	}
}

func TestEmptyConstructorOutputDeploysNoCode(t *testing.T) {
	// PUSH1 0 PUSH1 0 RETURN: the constructor returns no runtime code.
	initCode := []byte{0x60, 0x00, 0x60, 0x00, 0xf3}
	result, err := New().Execute(context.Background(), contract.Invocation{
		Method:   "deploy",
		Caller:   securityCaller,
		Contract: "0xde0a5f3a72e81b432378352a6ff6fc71d19df444",
		Input:    initCode,
		Salt:     make([]byte, 32),
		GasLimit: 100_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed {
		t.Fatalf("a constructor returning empty code is a valid deployment: %+v", result)
	}
	if len(result.DeployedCode) != 0 {
		t.Fatalf("no runtime code was returned, none may be reported: %x", result.DeployedCode)
	}
	for _, write := range result.CodeWrites {
		if !write.Delete && len(write.Code) != 0 {
			t.Fatalf("nothing but the code the constructor returned may be persisted, got %+v", write)
		}
	}
}

func TestSecurityFixtureAddressesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, address := range []types.Address{securityCaller, securityTarget, securityInner} {
		key := strings.ToLower(string(address))
		if seen[key] {
			t.Fatalf("fixture address %s is reused", address)
		}
		seen[key] = true
	}
}

// signedTestTransfer signs a legacy EIP-155 transfer to securityTarget and
// returns its raw encoding with the address that signed it.
func signedTestTransfer(t *testing.T, nonce uint64, gas uint64) (string, gethcommon.Address) {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	tx := gethtypes.NewTransaction(nonce, gethcommon.HexToAddress(string(securityTarget)), big.NewInt(0), gas, big.NewInt(1), nil)
	signed, err := gethtypes.SignTx(tx, gethtypes.NewEIP155Signer(big.NewInt(77)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw), gethcrypto.PubkeyToAddress(key.PublicKey)
}

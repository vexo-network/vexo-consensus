package evm

import (
	"context"
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
	"github.com/vexo-network/vexo-consensus/address"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	securityVictim   = types.Address("0x0000000000000000000000000000000000001111")
	securityAttacker = types.Address("0x0000000000000000000000000000000000002222")
	securityTarget   = types.Address("0x0000000000000000000000000000000000003333")
)

func openSecurityStore(t *testing.T) *store.LevelDBStore {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.Close() })
	return storage
}

// securityBankBalance reads the bank balance the EVM and the ante handler
// share; a missing key is a zero balance.
func securityBankBalance(t *testing.T, storage vexoapp.StateStore, address types.Address) *big.Int {
	t.Helper()
	value, err := storage.Get(context.Background(), bankNamespace, evmBankKey(address))
	if errors.Is(err, store.ErrKeyNotFound) {
		return new(big.Int)
	}
	if err != nil {
		t.Fatal(err)
	}
	return new(big.Int).SetBytes(value)
}

func securityNonce(t *testing.T, storage vexoapp.StateStore, address types.Address) uint64 {
	t.Helper()
	value, err := storage.Get(context.Background(), authNamespace, evmNonceKey(address))
	if errors.Is(err, store.ErrKeyNotFound) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return binary.BigEndian.Uint64(value)
}

func securityHasReceipt(t *testing.T, storage vexoapp.StateStore, hash string) bool {
	t.Helper()
	_, err := storage.Get(context.Background(), ReceiptNamespace, receiptKey(hash))
	if errors.Is(err, store.ErrKeyNotFound) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

func securityEthereumTx(t *testing.T, tx *gethtypes.Transaction, chainID uint64, baseFee uint64) ethcompat.DecodedTransaction {
	t.Helper()
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ethcompat.DecodeRawTransaction("0x"+hex.EncodeToString(raw), ethcompat.DecodeOptions{ChainID: chainID, BaseFee: baseFee})
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func securityTransfer(t *testing.T, chainID uint64, nonce uint64, value uint64, tip uint64, feeCap uint64, baseFee uint64) ethcompat.DecodedTransaction {
	t.Helper()
	return securityEthereumTx(t, gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     nonce,
		GasTipCap: new(big.Int).SetUint64(tip),
		GasFeeCap: new(big.Int).SetUint64(feeCap),
		Gas:       21_000,
		To:        &gethcommon.Address{0x33},
		Value:     new(big.Int).SetUint64(value),
	}), chainID, baseFee)
}

func rawTag(t *testing.T, tx types.Tx) string {
	t.Helper()
	raw, found := vexoapp.TxTag(tx, ethcompat.TagRaw)
	if !found {
		t.Fatal("transaction carries no raw Ethereum transaction")
	}
	return raw
}

func withTag(t *testing.T, tx types.Tx, key string, value string) types.Tx {
	t.Helper()
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Tags[key] = value
	rebuilt, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	return rebuilt
}

func withArg(t *testing.T, tx types.Tx, index int, value string) types.Tx {
	t.Helper()
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Args[index] = value
	rebuilt, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		t.Fatal(err)
	}
	return rebuilt
}

// A signed native transaction authenticates an ed25519 key whose account is a bech32 address; that
// key owns no EVM account. Claiming the victim's 0x address as the signer breaks the envelope's
// signer binding, and naming the victim as the caller under the key's own signer tag is not
// authorized, so the key cannot spend the victim's balance either way.
func TestSignedNativeTransactionCannotActAsAnEVMAccountItDoesNotOwn(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	account, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	forged := "evm:call:evm:" + string(securityVictim) + ":" + string(securityAttacker) + ":call:00:21000:900"
	if _, err := vexoapp.SignTx("vexo-chain", types.Tx(forged+":signer="+string(securityVictim)), signer); !errors.Is(err, vexoapp.ErrSignerAddressMismatch) {
		t.Fatalf("a key must not sign for an account it does not own, got %v", err)
	}

	signed, err := vexoapp.SignTx("vexo-chain", types.Tx(forged+":signer="+string(account)), signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := vexoapp.VerifySignedTx("vexo-chain", signed, signer); err != nil {
		t.Fatalf("the envelope itself is authentic, got %v", err)
	}
	storage := openSecurityStore(t)
	setTestEVMBalance(t, storage, securityVictim, 1_000)
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	module := NewModule()
	payload := vexoapp.TxPayload(signed)
	if err := module.ValidateTx(ctx, payload); !errors.Is(err, vexoapp.ErrUnauthorizedActor) {
		t.Fatalf("the key's account is not the caller, got %v", err)
	}
	if result := module.DeliverTx(ctx, payload); result.Code == 0 {
		t.Fatalf("a transaction acting as another account must fail: %+v", result)
	}
	if balance := securityBankBalance(t, storage, securityVictim); balance.Uint64() != 1_000 {
		t.Fatalf("victim balance must not move, got %s", balance)
	}
}

func TestNativeTransactionCannotActAsAnotherAccount(t *testing.T) {
	storage := openSecurityStore(t)
	setTestEVMBalance(t, storage, securityVictim, 1_000)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	forged := "evm:call:evm:" + string(securityVictim) + ":" + string(securityAttacker) + ":call:00:21000:900"

	for name, tx := range map[string]types.Tx{
		"no signer tag":      types.Tx(forged),
		"attacker as signer": types.Tx(forged + ":signer=" + string(securityAttacker)),
	} {
		t.Run(name, func(t *testing.T) {
			if err := module.ValidateTx(ctx, tx); !errors.Is(err, vexoapp.ErrUnauthorizedActor) {
				t.Fatalf("admission must reject a caller the signer does not own, got %v", err)
			}
			result := module.DeliverTx(ctx, tx)
			if result.Code == 0 {
				t.Fatalf("a transaction acting as another account must fail: %+v", result)
			}
			if balance := securityBankBalance(t, storage, securityVictim); balance.Uint64() != 1_000 {
				t.Fatalf("victim balance must not move, got %s", balance)
			}
			if balance := securityBankBalance(t, storage, securityAttacker); balance.Sign() != 0 {
				t.Fatalf("attacker must not receive funds, got %s", balance)
			}
		})
	}

	// The owner spells its own address in another case and with a longer
	// zero padding: the same account, so the transaction is authorized.
	owner := types.Tx(forged + ":signer=0x0000000000000000000000000000000000001111")
	if result := module.DeliverTx(ctx, owner); result.Code != 0 {
		t.Fatalf("the owner's own transaction must execute: %+v", result)
	}
	if balance := securityBankBalance(t, storage, securityAttacker); balance.Uint64() != 900 {
		t.Fatalf("owner transfer must arrive, got %s", balance)
	}
}

func TestNativeDeployRequiresTheSignerToOwnTheCaller(t *testing.T) {
	storage := openSecurityStore(t)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	deploy := "evm:deploy:evm:" + string(securityVictim) + ":60006000f3:01"

	if result := module.DeliverTx(ctx, types.Tx(deploy+":signer="+string(securityAttacker))); result.Code == 0 {
		t.Fatalf("a deployment on behalf of another account must fail: %+v", result)
	}
	if result := module.DeliverTx(ctx, types.Tx(deploy)); result.Code == 0 {
		t.Fatalf("an unsigned deployment must fail: %+v", result)
	}
	if result := module.DeliverTx(ctx, types.Tx(deploy+":signer="+string(securityVictim))); result.Code != 0 {
		t.Fatalf("the owner's deployment must execute: %+v", result)
	}
}

func TestEthereumDeploymentRequiresAVerifiedRawTransaction(t *testing.T) {
	storage := openSecurityStore(t)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	tx := types.Tx("evm:eth_deploy:evm:" + string(securityVictim) + ":6000:0:0:signer=" + string(securityVictim))
	if err := module.ValidateTx(ctx, tx); !errors.Is(err, ErrInvalidEVMTx) {
		t.Fatalf("eth_deploy without a signature-verified raw transaction must be refused, got %v", err)
	}
	if result := module.DeliverTx(ctx, tx); result.Code == 0 {
		t.Fatalf("eth_deploy without a raw transaction must fail: %+v", result)
	}
}

func TestEthereumTransactionIsBoundToTheRecoveredSender(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	decoded := securityTransfer(t, chainID, 3, 7, 1, 10, 1)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5}

	if err := module.AuthenticateTx(ctx, decoded.Tx); err != nil {
		t.Fatalf("the genuine transaction must authenticate: %v", err)
	}
	victim := string(securityVictim)
	forgedBoth := withArg(t, withTag(t, decoded.Tx, "signer", victim), 1, victim)
	tests := map[string]types.Tx{
		"signer tag names another account":    withTag(t, decoded.Tx, "signer", victim),
		"caller argument names another one":   withArg(t, decoded.Tx, 1, victim),
		"signer tag and caller both replaced": forgedBoth,
		"nonce tag rewritten":                 withTag(t, decoded.Tx, "nonce", "4"),
		"raw transaction replaced by another": withTag(t, decoded.Tx, ethcompat.TagRaw, rawTag(t, securityTransfer(t, chainID, 3, 8, 1, 10, 1).Tx)),
	}
	for name, tx := range tests {
		t.Run(name, func(t *testing.T) {
			if err := module.AuthenticateTx(ctx, tx); err == nil {
				t.Fatal("the signer of an Ethereum transaction is the account its signature recovers to, nothing else may authenticate")
			}
			if err := module.ValidateTx(ctx, tx); err == nil {
				t.Fatal("admission must refuse a transaction whose signer was not recovered from its signature")
			}
		})
	}
	nativeTx := types.Tx("evm:call:evm:" + victim + ":" + string(securityAttacker) + ":call:00:21000:1:signer=" + victim)
	if err := module.AuthenticateTx(ctx, nativeTx); !errors.Is(err, ErrInvalidEVMTx) {
		t.Fatalf("a native transaction carries no Ethereum signature to authenticate, got %v", err)
	}
}

func TestRejectedEthereumTransactionLeavesNoTrace(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	decoded := securityTransfer(t, chainID, 3, 7, 1, 10, 1)
	module := NewModule()

	t.Run("nonce mismatch", func(t *testing.T) {
		storage := openSecurityStore(t)
		setTestEVMBalance(t, storage, decoded.From, 1_000_000)
		setTestEVMNonce(t, storage, decoded.From, 9)
		ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5, Store: storage, Gas: vexoapp.NewGasMeter(decoded.Gas)}
		result := module.DeliverTx(ctx, decoded.Tx)
		if result.Code == 0 {
			t.Fatalf("a transaction with the wrong nonce must fail, not execute: %+v", result)
		}
		assertNoEthereumTrace(t, storage, decoded, 1_000_000, 9)
	})
	t.Run("insufficient funds", func(t *testing.T) {
		storage := openSecurityStore(t)
		setTestEVMBalance(t, storage, decoded.From, 100)
		setTestEVMNonce(t, storage, decoded.From, decoded.Nonce)
		ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5, Store: storage, Gas: vexoapp.NewGasMeter(decoded.Gas)}
		result := module.DeliverTx(ctx, decoded.Tx)
		if result.Code == 0 {
			t.Fatalf("a transaction the sender cannot pay for must fail, not execute: %+v", result)
		}
		assertNoEthereumTrace(t, storage, decoded, 100, decoded.Nonce)
	})
}

func assertNoEthereumTrace(t *testing.T, storage vexoapp.StateStore, decoded ethcompat.DecodedTransaction, balance uint64, nonce uint64) {
	t.Helper()
	if got := securityBankBalance(t, storage, decoded.From); got.Uint64() != balance {
		t.Fatalf("a rejected transaction must not move funds: balance %s, want %d", got, balance)
	}
	if got := securityNonce(t, storage, decoded.From); got != nonce {
		t.Fatalf("a rejected transaction must not bump the nonce: %d, want %d", got, nonce)
	}
	if securityHasReceipt(t, storage, TxHash(decoded.Tx)) {
		t.Fatal("a rejected transaction must not leave a receipt")
	}
	if got := securityBankBalance(t, storage, defaultFeeCollector); got.Sign() != 0 {
		t.Fatalf("a rejected transaction must not pay the fee collector, got %s", got)
	}
}

func TestEthereumTipsGoToTheBankFeeCollectorAtomically(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	// gas 21000, tip 3 and cap 10 over a base fee of 2: the sender pays 5 per
	// gas, 2 of it is burned and 3 is the priority fee.
	decoded := securityTransfer(t, chainID, 0, 7, 3, 10, 2)
	storage := openSecurityStore(t)
	setTestEVMBalance(t, storage, decoded.From, 1_000_000)
	module, err := NewModuleWithPolicy(Policy{FeeCollector: "treasury"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 2, Height: 5, Store: storage, Gas: vexoapp.NewGasMeter(decoded.Gas)}
	result := module.DeliverTx(ctx, decoded.Tx)
	if result.Code != 0 {
		t.Fatalf("transfer failed: %+v", result)
	}
	if got := securityBankBalance(t, storage, "treasury"); got.Uint64() != 21_000*3 {
		t.Fatalf("the configured fee collector must receive the priority fee, got %s", got)
	}
	if got := securityBankBalance(t, storage, coinbaseAlias); got.Sign() != 0 {
		t.Fatalf("no balance may be stranded on the coinbase alias, got %s", got)
	}
	if got := securityBankBalance(t, storage, decoded.From); got.Uint64() != 1_000_000-7-21_000*5 {
		t.Fatalf("sender must pay value plus gas at the effective price, got %s", got)
	}
	if got := securityBankBalance(t, storage, types.Address(gethcommon.Address{0x33}.Hex())); got.Uint64() != 7 {
		t.Fatalf("recipient must receive the value, got %s", got)
	}
}

func TestEthereumNonceIsTheAnteAccountSequence(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	decoded := securityTransfer(t, chainID, 0, 7, 1, 10, 1)
	storage := openSecurityStore(t)
	setTestEVMBalance(t, storage, decoded.From, 1_000_000)
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5, Store: storage, Gas: vexoapp.NewGasMeter(decoded.Gas)}
	if result := NewModule().DeliverTx(ctx, decoded.Tx); result.Code != 0 {
		t.Fatalf("transfer failed: %+v", result)
	}
	// The ante handler addresses accounts by whatever spelling the signer tag
	// uses; every spelling must find the nonce the EVM advanced.
	for _, spelling := range []types.Address{
		decoded.From,
		types.Address(strings.ToLower(string(decoded.From))),
		types.Address(gethcommon.HexToAddress(string(decoded.From)).Hex()),
	} {
		sequence, err := vexoapp.NewAccountKeeper().Sequence(context.Background(), storage, spelling)
		if err != nil || sequence != 1 {
			t.Fatalf("ante sequence for %s = %d (%v), want the EVM nonce 1", spelling, sequence, err)
		}
	}
	if string(evmNonceKey(decoded.From)) != string(vexoapp.NonceKey(types.Address(gethcommon.HexToAddress(string(decoded.From)).Hex()))) {
		t.Fatal("the EVM and the ante handler must key nonces identically")
	}
}

func TestForgedEthereumHashCannotOverwriteAReceipt(t *testing.T) {
	storage := openSecurityStore(t)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	setTestEVMBalance(t, storage, securityVictim, 1_000)
	victimTx := types.Tx("evm:call:evm:" + string(securityVictim) + ":" + string(securityTarget) + ":call:00:21000:10:signer=" + string(securityVictim))
	first := module.DeliverTx(ctx, victimTx)
	if first.Code != 0 {
		t.Fatalf("victim transaction failed: %+v", first)
	}
	var receipt Receipt
	if err := json.Unmarshal(first.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	before := mustGetStoreValue(t, storage, ReceiptNamespace, receiptKey(receipt.TxHash))

	// An attacker's own native transaction claims the victim's hash. The tag is
	// unsigned and carries no raw transaction to back it, so it is ignored.
	attackerTx := types.Tx("evm:call:evm:" + string(securityAttacker) + ":" + string(securityTarget) + ":call:00:21000:signer=" + string(securityAttacker) + ":eth_hash=" + receipt.TxHash)
	if hash := TxHash(attackerTx); hash == receipt.TxHash {
		t.Fatal("an unbacked eth_hash tag must not name the transaction")
	}
	second := module.DeliverTx(ctx, attackerTx)
	if second.Code != 0 {
		t.Fatalf("attacker transaction failed: %+v", second)
	}
	after := mustGetStoreValue(t, storage, ReceiptNamespace, receiptKey(receipt.TxHash))
	if string(before) != string(after) {
		t.Fatalf("the victim's receipt was overwritten: before=%s after=%s", before, after)
	}

	// Delivering a transaction twice never replaces its receipt either. The
	// outcome does not depend on the receipt store, which nodes prune locally;
	// replay protection is the account nonce's job.
	module.DeliverTx(ctx, victimTx)
	if again := mustGetStoreValue(t, storage, ReceiptNamespace, receiptKey(receipt.TxHash)); string(again) != string(before) {
		t.Fatal("the replay must leave the stored receipt untouched")
	}
}

func TestEthereumHashIsDerivedFromTheSignedBytes(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	decoded := securityTransfer(t, chainID, 0, 7, 1, 10, 1)
	if hash := TxHash(decoded.Tx); !strings.EqualFold(hash, decoded.Hash) {
		t.Fatalf("an Ethereum transaction is known by the keccak of its raw bytes: %s != %s", hash, decoded.Hash)
	}
	forged := withTag(t, decoded.Tx, ethcompat.TagHash, "0x"+strings.Repeat("ab", 32))
	if hash := TxHash(forged); !strings.EqualFold(hash, decoded.Hash) {
		t.Fatalf("a rewritten eth_hash tag must not change the transaction hash, got %s", hash)
	}
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5}
	if err := module.ValidateTx(ctx, forged); err == nil {
		t.Fatal("admission must refuse a transaction whose eth_hash disagrees with its raw bytes")
	}
}

func TestEmptyConstructorOutputStoresNoCode(t *testing.T) {
	storage := openSecurityStore(t)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	// PUSH1 0 PUSH1 0 RETURN: the constructor returns no runtime code.
	result := module.DeliverTx(ctx, types.Tx("evm:deploy:evm:"+string(securityVictim)+":60006000f3:01:signer="+string(securityVictim)))
	if result.Code != 0 {
		t.Fatalf("deployment failed: %+v", result)
	}
	var receipt Receipt
	if err := json.Unmarshal(result.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	code, err := storage.Get(context.Background(), ModuleName, codeKey(types.Address(receipt.ContractAddress)))
	if err == nil && len(code) != 0 {
		t.Fatalf("the init code must never be stored as contract code, stored %x", code)
	}
	if err != nil && !errors.Is(err, store.ErrKeyNotFound) {
		t.Fatal(err)
	}
}

func TestExecutionTraceIsNeverPersisted(t *testing.T) {
	storage := openSecurityStore(t)
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), ChainID: "vexo-chain", Height: 5, Store: storage}
	deployed := module.DeliverTx(ctx, types.Tx("evm:deploy:evm:"+string(securityVictim)+":600a600c600039600a6000f3602a60005260206000f3:01:signer="+string(securityVictim)))
	if deployed.Code != 0 {
		t.Fatalf("deployment failed: %+v", deployed)
	}
	var receipt Receipt
	if err := json.Unmarshal(deployed.Data, &receipt); err != nil {
		t.Fatal(err)
	}
	called := module.DeliverTx(ctx, types.Tx("evm:call:evm:"+string(securityVictim)+":"+receipt.ContractAddress+":call:00:100000:signer="+string(securityVictim)))
	if called.Code != 0 {
		t.Fatalf("call failed: %+v", called)
	}
	for name, data := range map[string][]byte{"deploy result": deployed.Data, "call result": called.Data} {
		for _, forbidden := range []string{"structLogs", "vm_trace", "\"pc\""} {
			if strings.Contains(string(data), forbidden) {
				t.Fatalf("%s must not carry an opcode trace (%s): %s", name, forbidden, data)
			}
		}
	}
	var callReceipt Receipt
	if err := json.Unmarshal(called.Data, &callReceipt); err != nil {
		t.Fatal(err)
	}
	stored := mustGetStoreValue(t, storage, ReceiptNamespace, receiptKey(callReceipt.TxHash))
	if strings.Contains(string(stored), "structLogs") {
		t.Fatalf("the stored receipt must not carry an opcode trace: %s", stored)
	}
}

// Block execution is bounded by the transaction's own gas and by the chain's bound on a
// transaction's gas, a consensus parameter. The node-local RPC cap never applies to it: two
// validators that configured different RPC caps must execute a block identically.
func TestNativeExecutionGasIsBoundedByTheTransactionAndTheConsensusCap(t *testing.T) {
	asked := "evm:call:evm:" + string(securityVictim) + ":" + string(securityTarget) + ":call:00:18446744073709551615:signer=" + string(securityVictim)

	executedLimits := func(rpcGasCap uint64) []uint64 {
		t.Helper()
		storage := openSecurityStore(t)
		if err := storage.Set(context.Background(), ModuleName, codeKey(securityTarget), []byte{0x60, 0x00}); err != nil {
			t.Fatal(err)
		}
		recorder := &recordingInvocationVM{}
		registry := contract.NewRegistry()
		if err := registry.Register(recorder); err != nil {
			t.Fatal(err)
		}
		module := NewModuleWithRegistry(registry)
		module.policy.RPCGasCap = rpcGasCap
		module.policy.MaxTxGas = 1_000_000
		limits := make([]uint64, 0, 4)

		ctx := vexoapp.Context{Ctx: context.Background(), Height: 5, Store: storage, Gas: vexoapp.NewGasMeter(120_000)}
		if result := module.DeliverTx(ctx, types.Tx(asked+":nonce=1")); result.Code != 0 {
			t.Fatalf("call failed: %+v", result)
		}
		limits = append(limits, recorder.invocation.GasLimit)

		ctx.Gas = vexoapp.NewGasMeter(0)
		if result := module.DeliverTx(ctx, types.Tx(asked+":nonce=2:gas=300000")); result.Code != 0 {
			t.Fatalf("call failed: %+v", result)
		}
		limits = append(limits, recorder.invocation.GasLimit)

		ctx.Gas = nil
		if result := module.DeliverTx(ctx, types.Tx(asked+":nonce=3")); result.Code != 0 {
			t.Fatalf("call failed: %+v", result)
		}
		limits = append(limits, recorder.invocation.GasLimit)

		gas, err := module.EstimateGas(vexoapp.Context{Ctx: context.Background(), Height: 5, Store: storage}, types.Tx(asked))
		if err != nil || gas == 0 {
			t.Fatalf("estimate failed: gas=%d err=%v", gas, err)
		}
		limits = append(limits, recorder.invocation.GasLimit)
		return limits
	}

	want := []uint64{120_000, 300_000, 1_000_000, 1_000_000}
	for _, rpcGasCap := range []uint64{21_000, 1_000_000, 50_000_000} {
		got := executedLimits(rpcGasCap)
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("rpc cap %d: execution %d ran with gas %d, want %d (metered, tagged, unbounded, estimate)", rpcGasCap, index, got[index], want[index])
			}
		}
	}
}

func TestQueryCallGasIsCappedAndTracingIsOptIn(t *testing.T) {
	recorder := &recordingInvocationVM{}
	registry := contract.NewRegistry()
	if err := registry.Register(recorder); err != nil {
		t.Fatal(err)
	}
	module := NewModuleWithRegistry(registry)
	module.policy.RPCGasCap = 2_000_000
	storage := openSecurityStore(t)
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 5, Store: storage}
	call := func(request CallRequest) vexoapp.QueryResponse {
		t.Helper()
		request.VM, request.From, request.To, request.Method = "evm", string(securityVictim), string(securityTarget), "call"
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		response := module.Query(ctx, vexoapp.QueryRequest{Path: []string{"call"}, Data: encoded})
		if response.Code != 0 {
			t.Fatalf("query failed: %+v", response)
		}
		return response
	}

	for name, test := range map[string]struct{ asked, want uint64 }{
		"within the cap":          {asked: 500_000, want: 500_000},
		"above the cap":           {asked: 1 << 40, want: 2_000_000},
		"no limit asked":          {asked: 0, want: 2_000_000},
		"exactly the cap":         {asked: 2_000_000, want: 2_000_000},
		"one above the cap":       {asked: 2_000_001, want: 2_000_000},
		"largest representable":   {asked: ^uint64(0), want: 2_000_000},
		"smallest nonzero limit":  {asked: 1, want: 1},
		"just below the cap":      {asked: 1_999_999, want: 1_999_999},
		"intrinsic-sized request": {asked: 21_000, want: 21_000},
	} {
		call(CallRequest{GasLimit: test.asked})
		if recorder.invocation.GasLimit != test.want {
			t.Fatalf("%s: asked %d, executed with %d, want %d", name, test.asked, recorder.invocation.GasLimit, test.want)
		}
	}

	call(CallRequest{GasLimit: 100_000})
	if recorder.invocation.Trace {
		t.Fatal("a plain call must not record an opcode trace")
	}
	call(CallRequest{GasLimit: 100_000, Trace: true})
	if !recorder.invocation.Trace {
		t.Fatal("an explicit trace request must reach the executor")
	}
}

func TestLogsQueryReadsOnlyTheRequestedBlocks(t *testing.T) {
	storage := openSecurityStore(t)
	counting := &countingPrefixStore{LevelDBStore: storage}
	emitter := types.Address("0x000000000000000000000000000000000000AbCd")
	for height := uint64(1); height <= 40; height++ {
		hash := "0x" + strings.Repeat("0", 62) + hex.EncodeToString([]byte{byte(height)})
		receipt := Receipt{
			TxHash: hash,
			Height: height,
			Logs:   []Log{{Address: string(emitter), BlockNumber: height, TransactionHash: hash, Topics: []string{"0x01"}}},
		}
		if err := persistReceipt(context.Background(), storage, receipt); err != nil {
			t.Fatal(err)
		}
	}
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 40, Store: counting}
	query := func(path []string, request LogsRequest) vexoapp.QueryResponse {
		counting.scans = 0
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return module.Query(ctx, vexoapp.QueryRequest{Path: path, Data: data})
	}
	decodeLogs := func(response vexoapp.QueryResponse) []Log {
		t.Helper()
		if response.Code != 0 {
			t.Fatalf("logs query failed: %+v", response)
		}
		var logs []Log
		if err := json.Unmarshal(response.Value, &logs); err != nil {
			t.Fatal(err)
		}
		return logs
	}

	logs := decodeLogs(query([]string{"logs"}, LogsRequest{FromBlock: 10, ToBlock: 12}))
	if len(logs) != 3 || logs[0].BlockNumber != 10 || logs[2].BlockNumber != 12 {
		t.Fatalf("a bounded query returns exactly its blocks, got %+v", logs)
	}
	if counting.scans != 3 {
		t.Fatalf("a three block query must scan three heights, scanned %d", counting.scans)
	}

	logs = decodeLogs(query([]string{"logs"}, LogsRequest{FromBlock: 38}))
	if len(logs) != 3 || counting.scans != 3 {
		t.Fatalf("a missing toBlock defaults to the latest block: logs=%d scans=%d", len(logs), counting.scans)
	}
	logs = decodeLogs(query([]string{"logs"}, LogsRequest{FromBlock: 39, ToBlock: 10_000}))
	if len(logs) != 2 || counting.scans != 2 {
		t.Fatalf("a toBlock past the head stops at the head: logs=%d scans=%d", len(logs), counting.scans)
	}
	if logs = decodeLogs(query([]string{"logs"}, LogsRequest{FromBlock: 30, ToBlock: 20})); len(logs) != 0 || counting.scans != 0 {
		t.Fatalf("an inverted range is empty and reads nothing: logs=%d scans=%d", len(logs), counting.scans)
	}

	// Whatever case a client spells the address in, it finds the logs.
	for _, spelling := range []string{
		string(emitter),
		strings.ToLower(string(emitter)),
		"0x" + strings.ToUpper(strings.TrimPrefix(string(emitter), "0x")),
	} {
		if logs = decodeLogs(query([]string{"logs", spelling}, LogsRequest{FromBlock: 1, ToBlock: 5})); len(logs) != 5 {
			t.Fatalf("address %q must match the emitter, got %d logs", spelling, len(logs))
		}
	}
	if logs = decodeLogs(query([]string{"logs", string(securityTarget)}, LogsRequest{FromBlock: 1, ToBlock: 5})); len(logs) != 0 {
		t.Fatalf("another address has no logs, got %+v", logs)
	}

	// A range wider than the limit is refused before any block is read.
	ctx.Height = 3 * MaxLogBlockRange
	response := query([]string{"logs"}, LogsRequest{FromBlock: 1, ToBlock: 1 + MaxLogBlockRange})
	if response.Code == 0 || counting.scans != 0 {
		t.Fatalf("an over-wide range must be refused without reading: %+v scans=%d", response, counting.scans)
	}
	response = query([]string{"logs"}, LogsRequest{})
	if response.Code == 0 || counting.scans != 0 {
		t.Fatalf("an unbounded query over a long chain must be refused without reading: %+v scans=%d", response, counting.scans)
	}
	response = query([]string{"logs"}, LogsRequest{FromBlock: 1, ToBlock: MaxLogBlockRange})
	if response.Code != 0 {
		t.Fatalf("a range of exactly the limit is allowed: %+v", response)
	}
}

// A logs query filters by topic while it reads and stops once it holds as many logs as it may
// return, so a busy range costs no more than the result it is allowed to produce.
func TestLogsQueryFiltersTopicsAndStopsAtItsLimit(t *testing.T) {
	storage := openSecurityStore(t)
	counting := &countingPrefixStore{LevelDBStore: storage}
	emitter := types.Address("0x000000000000000000000000000000000000AbCd")
	for height := uint64(1); height <= 20; height++ {
		hash := "0x" + strings.Repeat("0", 62) + hex.EncodeToString([]byte{byte(height)})
		topic := "0xAA"
		if height%2 == 0 {
			topic = "0xbb"
		}
		receipt := Receipt{
			TxHash: hash,
			Height: height,
			Logs:   []Log{{Address: string(emitter), BlockNumber: height, TransactionHash: hash, Topics: []string{topic, "0x02"}}},
		}
		if err := persistReceipt(context.Background(), storage, receipt); err != nil {
			t.Fatal(err)
		}
	}
	module := NewModule()
	ctx := vexoapp.Context{Ctx: context.Background(), Height: 20, Store: counting}
	query := func(request LogsRequest) vexoapp.QueryResponse {
		counting.scans = 0
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return module.Query(ctx, vexoapp.QueryRequest{Path: []string{"logs"}, Data: data})
	}
	decodeLogs := func(response vexoapp.QueryResponse) []Log {
		t.Helper()
		if response.Code != 0 {
			t.Fatalf("logs query failed: %+v", response)
		}
		var logs []Log
		if err := json.Unmarshal(response.Value, &logs); err != nil {
			t.Fatal(err)
		}
		return logs
	}

	logs := decodeLogs(query(LogsRequest{FromBlock: 1, ToBlock: 20, Topics: [][]string{{"0xaa"}}}))
	if len(logs) != 10 {
		t.Fatalf("a topic matches whatever case it is spelled in, got %d logs", len(logs))
	}
	for _, log := range logs {
		if log.BlockNumber%2 == 0 {
			t.Fatalf("a log of another topic was returned: %+v", log)
		}
	}
	if logs = decodeLogs(query(LogsRequest{FromBlock: 1, ToBlock: 20, Topics: [][]string{nil, {"0x02"}}})); len(logs) != 20 {
		t.Fatalf("an empty position accepts every topic, got %d logs", len(logs))
	}
	if logs = decodeLogs(query(LogsRequest{FromBlock: 1, ToBlock: 20, Topics: [][]string{nil, nil, {"0x03"}}})); len(logs) != 0 {
		t.Fatalf("a position past the log's topics matches nothing, got %d logs", len(logs))
	}

	logs = decodeLogs(query(LogsRequest{FromBlock: 1, ToBlock: 20, Topics: [][]string{{"0xbb"}}, Limit: 3}))
	if len(logs) != 3 || logs[2].BlockNumber != 6 {
		t.Fatalf("a limited query returns its first matching logs, got %+v", logs)
	}
	if counting.scans != 6 {
		t.Fatalf("a limited query must stop reading once it is full, scanned %d heights", counting.scans)
	}

	tooManyTopics := make([][]string, MaxLogTopics+1)
	if response := query(LogsRequest{FromBlock: 1, ToBlock: 20, Topics: tooManyTopics}); response.Code == 0 || counting.scans != 0 {
		t.Fatalf("more topic positions than a log has must be refused without reading: %+v", response)
	}
	tooManyOptions := make([]string, MaxLogTopicOptions+1)
	for index := range tooManyOptions {
		tooManyOptions[index] = "0x" + hex.EncodeToString([]byte{byte(index)})
	}
	if response := query(LogsRequest{FromBlock: 1, ToBlock: 20, Topics: [][]string{tooManyOptions}}); response.Code == 0 || counting.scans != 0 {
		t.Fatalf("a position with too many options must be refused without reading: %+v", response)
	}
}

// countingPrefixStore counts the prefix scans a query performs.
type countingPrefixStore struct {
	*store.LevelDBStore
	scans int
}

func (counting *countingPrefixStore) ExportPrefix(ctx context.Context, namespace string, prefix []byte) ([]store.KVPair, error) {
	counting.scans++
	return counting.LevelDBStore.ExportPrefix(ctx, namespace, prefix)
}

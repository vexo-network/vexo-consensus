package evm

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/types"
)

// bareAuthenticator offers nothing of an authenticator but AuthenticateTx, as one that names no
// inputs does: the ante handler can neither remember it nor run it on several goroutines.
type bareAuthenticator struct{ vexoapp.TxAuthenticator }

// countRecoveries makes the module count how often it recovers the sender of an Ethereum
// transaction, which is what its check of the raw transaction does.
func countRecoveries(t *testing.T) *atomic.Int64 {
	t.Helper()
	var count atomic.Int64
	previous := validateRawTx
	validateRawTx = func(tx types.Tx, options ethcompat.DecodeOptions) error {
		count.Add(1)
		return previous(tx, options)
	}
	t.Cleanup(func() { validateRawTx = previous })
	return &count
}

func withMaxProcs(t *testing.T, procs int) {
	t.Helper()
	previous := goruntime.GOMAXPROCS(procs)
	t.Cleanup(func() { goruntime.GOMAXPROCS(previous) })
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// transferFrom is a signed Ethereum transfer of key, decoded under baseFee.
func transferFrom(t *testing.T, key *ecdsa.PrivateKey, chainID uint64, nonce uint64, tip uint64, feeCap uint64, baseFee uint64) ethcompat.DecodedTransaction {
	t.Helper()
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     nonce,
		GasTipCap: new(big.Int).SetUint64(tip),
		GasFeeCap: new(big.Int).SetUint64(feeCap),
		Gas:       21_000,
		To:        &gethcommon.Address{0x33},
		Value:     big.NewInt(7),
	})
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

// evmRuntime is a runtime of the chain "vexo-chain" whose only module is module and whose ante
// handler authenticates Ethereum transactions through authenticator.
func evmRuntime(t *testing.T, storage vexoapp.StateStore, module Module, authenticator vexoapp.TxAuthenticator, config vexoapp.AnteConfig) *vexoapp.Runtime {
	t.Helper()
	config.EthereumAuthenticator = authenticator
	config.RequireNonce = true
	config.FeeCollector = "treasury"
	rt, err := vexoapp.NewRuntime("vexo-chain", []vexoapp.Module{module}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	return rt.WithStore(storage).WithAnte(vexoapp.NewAnteKeeper(config))
}

func blockOf(height types.Height, txs ...types.Tx) types.Block {
	return types.Block{
		Header: types.Header{ChainID: "vexo-chain", Height: height},
		Txs:    fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt("vexo-chain", height)),
	}
}

// An Ethereum transaction used to have its sender recovered by every stage that admits it, twice
// each: once to authenticate it and once to validate or deliver it. Now it is recovered once on a
// node, whatever the stages it passes.
func TestEthereumTransactionIsRecoveredOncePerNode(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	tx := transferFrom(t, newKey(t), chainID, 0, 1, 10, 1)
	lifecycle := func(t *testing.T, module Module, authenticator vexoapp.TxAuthenticator) int64 {
		storage := openSecurityStore(t)
		setTestEVMBalance(t, storage, tx.From, 100_000_000)
		rt := evmRuntime(t, storage, module, authenticator, vexoapp.AnteConfig{BaseFee: 1})
		count := countRecoveries(t)

		if response := rt.CheckTx(tx.Tx); response.Result.Code != 0 {
			t.Fatalf("CheckTx rejected the transaction: %+v", response.Result)
		}
		prepared, err := rt.PrepareProposal(vexoapp.PrepareProposalRequest{Height: 1, Txs: []types.Tx{tx.Tx}})
		if err != nil || len(prepared.Txs) != 1 {
			t.Fatalf("PrepareProposal must select the transaction, got %d (%v)", len(prepared.Txs), err)
		}
		block := blockOf(1, prepared.Txs...)
		if response := rt.ProcessProposal(vexoapp.ProcessProposalRequest{Block: block}); !response.Accepted {
			t.Fatalf("ProcessProposal rejected the block: %s", response.Reason)
		}
		response, err := rt.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: block})
		if err != nil || len(response.Results) != 1 || response.Results[0].Code != 0 {
			t.Fatalf("FinalizeBlock must execute the transaction, got %+v (%v)", response.Results, err)
		}
		return count.Load()
	}

	module := NewModule()
	remembered := lifecycle(t, module, module)
	if remembered != 1 {
		t.Fatalf("the sender must be recovered once for the transaction's whole life on a node, got %d recoveries", remembered)
	}
	if got := module.validated.Len(); got != 1 {
		t.Fatalf("the module must remember the transaction, remembers %d", got)
	}

	forgetful := NewModule()
	forgetful.validated = nil
	repeated := lifecycle(t, forgetful, bareAuthenticator{forgetful})
	t.Logf("sender recoveries per Ethereum transaction: %d without the caches, %d with them", repeated, remembered)
	if repeated <= remembered {
		t.Fatalf("without the caches the stages recover the sender again, got %d recoveries", repeated)
	}
}

// The base fee is an input of the check: a fee cap that covered it when a transaction was checked
// may not cover the base fee the transaction is admitted under later, and a transaction that
// authenticated before must be rejected then, with the error it always was rejected with.
func TestRaisedBaseFeeRejectsAnEthereumTransactionThatAuthenticatedBefore(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	burner := transferFrom(t, newKey(t), chainID, 0, 1, 10, 1)
	waiting := transferFrom(t, newKey(t), chainID, 0, 1, 15, 1)
	storage := openSecurityStore(t)
	setTestEVMBalance(t, storage, burner.From, 100_000_000)
	setTestEVMBalance(t, storage, waiting.From, 100_000_000)
	module := NewModule()
	// The block of the burner uses far more gas than the target, so the base fee rises from 1 to 21.
	rt := evmRuntime(t, storage, module, module, vexoapp.AnteConfig{
		BaseFee:   1,
		FeeMarket: vexoapp.FeeMarketConfig{DynamicBaseFee: true, TargetGas: 1000, BaseFeeChangeDenominator: 1},
	})
	count := countRecoveries(t)

	if response := rt.CheckTx(waiting.Tx); response.Result.Code != 0 {
		t.Fatalf("a fee cap of 15 covers a base fee of 1: %+v", response.Result)
	}
	if response := rt.CheckTx(waiting.Tx); response.Result.Code != 0 || count.Load() != 1 {
		t.Fatalf("the second check must be answered from memory, got %+v after %d recoveries", response.Result, count.Load())
	}

	finalized, err := rt.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(1, burner.Tx)})
	if err != nil || len(finalized.Results) != 1 || finalized.Results[0].Code != 0 {
		t.Fatalf("the burner must execute, got %+v (%v)", finalized.Results, err)
	}
	if finalized.FeeMarket.NextBaseFee <= 15 {
		t.Fatalf("the block must raise the base fee above the fee cap of the waiting transaction, got %d", finalized.FeeMarket.NextBaseFee)
	}

	recovered := count.Load()
	for attempt := int64(1); attempt <= 3; attempt++ {
		response := rt.CheckTx(waiting.Tx)
		if response.Result.Code == 0 || !strings.Contains(response.Result.Log, ethcompat.ErrFeeCapTooLow.Error()) {
			t.Fatalf("attempt %d: the raised base fee must reject the transaction for its fee cap, got %+v", attempt, response.Result)
		}
		if got := count.Load() - recovered; got != attempt {
			t.Fatalf("attempt %d: a rejection must be checked afresh, got %d recoveries", attempt, got)
		}
	}
}

// The same holds for the module on its own, for the blob base fee and for each other option of the
// check, and what passed under one set of options is still remembered under it.
func TestModuleChecksEthereumTransactionAgainstTheBaseFeeOfTheContext(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	tx := transferFrom(t, newKey(t), chainID, 0, 1, 10, 1).Tx
	module := NewModule()
	count := countRecoveries(t)
	context := func(baseFee uint64) vexoapp.Context {
		return vexoapp.Context{ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: baseFee, Height: 5}
	}

	if err := module.AuthenticateTx(context(1), tx); err != nil {
		t.Fatal(err)
	}
	if err := module.AuthenticateTx(context(1), tx); err != nil || count.Load() != 1 {
		t.Fatalf("the second authentication must be answered from memory, got %v after %d recoveries", err, count.Load())
	}
	if err := module.ValidateTx(context(1), tx); err != nil || count.Load() != 1 {
		t.Fatalf("validation must find what authentication remembered, got %v after %d recoveries", err, count.Load())
	}
	for attempt := int64(2); attempt <= 4; attempt++ {
		if err := module.AuthenticateTx(context(11), tx); !errors.Is(err, ethcompat.ErrFeeCapTooLow) {
			t.Fatalf("a base fee above the fee cap must reject the transaction, got %v", err)
		}
		if err := module.ValidateTx(context(11), tx); !errors.Is(err, ethcompat.ErrFeeCapTooLow) {
			t.Fatalf("validation must reject it too, got %v", err)
		}
		if got := count.Load(); got != 1+2*(attempt-1) {
			t.Fatalf("attempt %d: rejections must be checked afresh, got %d recoveries", attempt, got)
		}
	}
	before := count.Load()
	if err := module.AuthenticateTx(context(1), tx); err != nil || count.Load() != before {
		t.Fatalf("the success under the first base fee is still remembered, got %v after %d new recoveries", err, count.Load()-before)
	}
	if err := module.AuthenticateTx(context(5), tx); err != nil || count.Load() != before+1 {
		t.Fatalf("another base fee that covers the cap is checked once, got %v after %d new recoveries", err, count.Load()-before)
	}
}

// A signature that does not verify is rejected every time it is presented: the module never
// remembers a failure.
func TestInvalidEthereumSignaturesAreRejectedEveryTime(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	genuine := transferFrom(t, newKey(t), chainID, 0, 1, 10, 1)
	other := transferFrom(t, newKey(t), chainID, 0, 1, 10, 1)
	// The raw transaction of another signer under the tags of this one: its signature recovers to
	// an account that is not the signer the tags name.
	forged := withTag(t, genuine.Tx, ethcompat.TagRaw, rawTag(t, other.Tx))
	module := NewModule()
	count := countRecoveries(t)
	ctx := vexoapp.Context{ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5}

	for attempt := int64(1); attempt <= 3; attempt++ {
		if err := module.AuthenticateTx(ctx, forged); !errors.Is(err, ethcompat.ErrSignatureMismatch) {
			t.Fatalf("attempt %d: expected the forged transaction to be rejected, got %v", attempt, err)
		}
		if got := count.Load(); got != attempt {
			t.Fatalf("attempt %d: a rejected transaction must be checked again, got %d recoveries", attempt, got)
		}
	}
	if got := module.validated.Len(); got != 0 {
		t.Fatalf("a rejection must not be remembered, the module remembers %d", got)
	}

	// Through the runtime the forged transaction is rejected by every stage, each time, and the
	// genuine one beside it is not affected by them.
	storage := openSecurityStore(t)
	setTestEVMBalance(t, storage, genuine.From, 100_000_000)
	rt := evmRuntime(t, storage, module, module, vexoapp.AnteConfig{BaseFee: 1})
	for attempt := 1; attempt <= 3; attempt++ {
		if response := rt.CheckTx(forged); response.Result.Code == 0 {
			t.Fatalf("attempt %d: CheckTx must reject the forged transaction", attempt)
		}
		if response := rt.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(1, forged)}); response.Accepted {
			t.Fatalf("attempt %d: ProcessProposal must reject a block with the forged transaction", attempt)
		}
	}
	if response := rt.CheckTx(genuine.Tx); response.Result.Code != 0 {
		t.Fatalf("the genuine transaction must pass, got %+v", response.Result)
	}
}

// The memo is bounded: what it forgets is checked again.
func TestModuleMemoOfValidatedTransactionsIsBounded(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	key := newKey(t)
	module := NewModule()
	module.validated = vexoapp.NewVerifyCache(3)
	count := countRecoveries(t)
	ctx := vexoapp.Context{ChainID: "vexo-chain", EVMChainID: chainID, BaseFee: 1, Height: 5}
	txs := make([]types.Tx, 0, 6)
	for nonce := range uint64(6) {
		txs = append(txs, transferFrom(t, key, chainID, nonce, 1, 10, 1).Tx)
	}

	for _, tx := range txs {
		if err := module.AuthenticateTx(ctx, tx); err != nil {
			t.Fatal(err)
		}
		if got := module.validated.Len(); got > 3 {
			t.Fatalf("the memo holds %d transactions", got)
		}
	}
	if count.Load() != 6 || module.validated.Len() != 3 {
		t.Fatalf("expected six recoveries and three remembered, got %d and %d", count.Load(), module.validated.Len())
	}
	if err := module.AuthenticateTx(ctx, txs[5]); err != nil || count.Load() != 6 {
		t.Fatalf("the latest transaction is still remembered, got %v after %d recoveries", err, count.Load())
	}
	if err := module.AuthenticateTx(ctx, txs[0]); err != nil || count.Load() != 7 {
		t.Fatalf("the forgotten transaction is checked again and passes, got %v after %d recoveries", err, count.Load())
	}
}

// Warming the checks of a block on several goroutines before the block is admitted one
// transaction after the other changes nothing the block says: the same results and the same state
// as admitting it without any memory, for a block of genuine transfers, a forged one, one under
// the base fee, a replay and junk.
func TestParallelWarmUpOfEthereumTransactionsDoesNotChangeABlock(t *testing.T) {
	chainID := ethcompat.ChainNumericID("vexo-chain")
	const baseFee = 5
	var txs []types.Tx
	var senders []types.Address
	for range 12 {
		decoded := transferFrom(t, newKey(t), chainID, 0, 1, 10, baseFee)
		txs = append(txs, decoded.Tx)
		senders = append(senders, decoded.From)
	}
	victim := transferFrom(t, newKey(t), chainID, 0, 1, 10, baseFee)
	thief := transferFrom(t, newKey(t), chainID, 0, 1, 10, baseFee)
	senders = append(senders, victim.From)
	txs = append(txs, withTag(t, victim.Tx, ethcompat.TagRaw, rawTag(t, thief.Tx)))
	cheap := transferFrom(t, newKey(t), chainID, 0, 1, 3, 0)
	senders = append(senders, cheap.From)
	txs = append(txs, cheap.Tx, txs[0], types.Tx("evm:call:junk"))
	block := blockOf(1, txs...)

	run := func(t *testing.T, remembering bool) (vexoapp.ProcessProposalResponse, vexoapp.FinalizeBlockResponse, Module) {
		storage := openSecurityStore(t)
		for _, sender := range senders {
			setTestEVMBalance(t, storage, sender, 100_000_000)
		}
		module := NewModule()
		var authenticator vexoapp.TxAuthenticator = module
		if !remembering {
			module.validated = nil
			authenticator = bareAuthenticator{module}
		}
		rt := evmRuntime(t, storage, module, authenticator, vexoapp.AnteConfig{BaseFee: baseFee})
		validation := rt.ProcessProposal(vexoapp.ProcessProposalRequest{Block: block})
		execution, err := rt.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: block})
		if err != nil {
			t.Fatal(err)
		}
		return validation, execution, module
	}

	baselineValidation, baseline, _ := run(t, false)
	if baselineValidation.Accepted {
		t.Fatal("the block carries invalid transactions, the baseline must not accept it")
	}
	var executed int
	for _, result := range baseline.Results {
		if result.Code == 0 {
			executed++
		}
	}
	if executed < 12 || executed == len(baseline.Results) {
		t.Fatalf("the block must mix executed and failed transactions, %d of %d executed", executed, len(baseline.Results))
	}

	for _, procs := range []int{2, 4, 8} {
		t.Run(fmt.Sprintf("procs=%d", procs), func(t *testing.T) {
			withMaxProcs(t, procs)
			validation, execution, module := run(t, true)
			if validation != baselineValidation {
				t.Fatalf("validation differs from the sequential one: %+v, want %+v", validation, baselineValidation)
			}
			if !reflect.DeepEqual(execution, baseline) {
				t.Fatalf("execution differs from the sequential one:\n got %+v\nwant %+v", execution, baseline)
			}
			if got := module.validated.Len(); got != 12 {
				t.Fatalf("expected the twelve genuine transactions to be remembered, got %d", got)
			}
		})
	}
}

// The key names every option the check is made under, and nothing the check does not read.
func TestAuthenticationKeyNamesEveryOptionOfTheCheck(t *testing.T) {
	base := authenticationInputs{}.encode()
	inputs := reflect.TypeOf(authenticationInputs{})
	for index := range inputs.NumField() {
		field := inputs.Field(index)
		var changed authenticationInputs
		value := reflect.ValueOf(&changed).Elem().Field(index)
		switch value.Kind() {
		case reflect.Uint64:
			value.SetUint(1)
		case reflect.String:
			value.SetString("x")
		case reflect.Bool:
			value.SetBool(true)
		default:
			t.Fatalf("option %s is a %s: teach the key to encode it", field.Name, value.Kind())
		}
		if bytes.Equal(changed.encode(), base) {
			t.Fatalf("option %s is not part of the key", field.Name)
		}
	}

	module := NewModule()
	ctx := vexoapp.Context{ChainID: "vexo-chain", EVMChainID: 7, BaseFee: 5, BlobBaseFee: 2, Height: 3}
	keyOf := func(module Module, ctx vexoapp.Context) []byte {
		t.Helper()
		key, ok := module.AuthenticationKey(ctx, nil)
		if !ok {
			t.Fatal("a working module must offer a key")
		}
		return key
	}
	key := keyOf(module, ctx)

	same := ctx
	same.Height, same.Header, same.Block = 99, types.Header{Height: 99}, types.Block{Header: types.Header{Height: 99}}
	same.Store, same.Gas = openSecurityStore(t), vexoapp.NewGasMeter(5)
	if !bytes.Equal(keyOf(module, same), key) {
		t.Fatal("what the check does not read must not be in the key")
	}
	for name, changed := range map[string]vexoapp.Context{
		"base fee":      {ChainID: "vexo-chain", EVMChainID: 7, BaseFee: 6, BlobBaseFee: 2},
		"blob base fee": {ChainID: "vexo-chain", EVMChainID: 7, BaseFee: 5, BlobBaseFee: 3},
		"evm chain id":  {ChainID: "vexo-chain", EVMChainID: 8, BaseFee: 5, BlobBaseFee: 2},
	} {
		if bytes.Equal(keyOf(module, changed), key) {
			t.Fatalf("%s must be part of the key", name)
		}
	}
	// Without an explicit EVM chain id the chain id of the context names it.
	derived := vexoapp.Context{ChainID: "vexo-chain", BaseFee: 5}
	elsewhere := vexoapp.Context{ChainID: "other-chain", BaseFee: 5}
	if bytes.Equal(keyOf(module, derived), keyOf(module, elsewhere)) {
		t.Fatal("the chain id of the context must be part of the key when it names the EVM chain")
	}
	permissive := module
	permissive.policy.AllowUnprotectedLegacyTx = true
	if bytes.Equal(keyOf(permissive, ctx), key) {
		t.Fatal("the policy on unprotected legacy transactions must be part of the key")
	}
	broken := Module{policy: DefaultPolicy(), initErr: ErrVMBackendUnavailable}
	if _, ok := broken.AuthenticationKey(ctx, nil); ok {
		t.Fatal("a module that failed to initialize must not offer a key: its verdicts are not to be remembered")
	}
}

// What a module remembers is a function of the transaction and of the options, and the options are
// part of what it is remembered by, so modules that differ in policy can share the memory.
func TestModulesShareWhatTheyValidateSafely(t *testing.T) {
	module := NewModule()
	if clone := module.CloneModule().(Module); clone.validated == nil || clone.validated != module.validated {
		t.Fatal("a clone of a module shares its memory of validated transactions")
	}
	withPolicy, err := NewModuleWithPolicy(Policy{EVMChainID: 7})
	if err != nil {
		t.Fatal(err)
	}
	for name, constructed := range map[string]Module{
		"NewModule":             NewModule(),
		"NewModuleWithRegistry": NewModuleWithRegistry(contract.NewRegistry()),
		"NewModuleWithPolicy":   withPolicy,
	} {
		if constructed.validated == nil {
			t.Fatalf("%s must give the module a memory of validated transactions", name)
		}
	}

	key := newKey(t)
	legacy := gethtypes.NewTransaction(0, gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF"), big.NewInt(1), 21_000, big.NewInt(13), nil)
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
	strict := module
	permissive := module
	permissive.policy.AllowUnprotectedLegacyTx = true
	ctx := vexoapp.Context{ChainID: "vexo-chain", EVMChainID: 7}
	for round := 0; round < 2; round++ {
		if err := permissive.ValidateTx(ctx, decoded.Tx); err != nil {
			t.Fatalf("the permissive module accepts the unprotected legacy transaction, got %v", err)
		}
		if err := strict.ValidateTx(ctx, decoded.Tx); !errors.Is(err, ethcompat.ErrUnprotectedLegacyTx) {
			t.Fatalf("the success of the permissive module must not answer for the strict one, got %v", err)
		}
	}
}

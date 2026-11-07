package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// ValidateTx runs the admission checks that do not depend on state: argument
// shape, strict EVM addresses, signer authorization of the caller and, for
// Ethereum transactions, sender recovery from the raw signature.
func (module Module) ValidateTx(ctx vexoapp.Context, tx types.Tx) error {
	if module.initErr != nil {
		return module.initErr
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return ErrInvalidEVMTx
	}
	if _, _, err := module.admitTx(ctx, tx, canonical); err != nil {
		return err
	}
	_, err = module.blobSidecarBundleFromTx(tx)
	return err
}

// AuthenticateTx proves that the signer tag of an unsigned Ethereum
// transaction names the account that signed it. The ante handler calls it for
// transactions that carry no signed envelope; for those the sender recovered
// from the signature inside eth_raw is the only authentication there can be.
// It requires the signer tag, the caller argument and every tag the executor
// reads to match the raw transaction, and bounds the unsigned fee tag by what
// the signature authorizes.
func (module Module) AuthenticateTx(ctx vexoapp.Context, tx types.Tx) error {
	if module.initErr != nil {
		return module.initErr
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName || !ethcompat.IsEthereumTx(tx) {
		return ErrInvalidEVMTx
	}
	_, _, err = module.admitTx(ctx, tx, canonical)
	return err
}

// DeliverTx executes one transaction. It never fails the block because of
// transaction content: every rejection is a non-zero Result and leaves the
// store untouched, because a transaction's writes are applied in one batch
// after execution succeeded.
func (module Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if module.initErr != nil {
		return types.Result{Code: 1, Log: module.initErr.Error()}
	}
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: ErrStoreMissing.Error()}
	}
	if module.registry == nil {
		return types.Result{Code: 1, Log: ErrVMRegistryEmpty.Error()}
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return types.Result{Code: 2, Log: ErrInvalidEVMTx.Error()}
	}
	switch canonical.Action {
	case "call", "deploy", "eth_deploy":
		return module.deliver(ctx, tx, canonical)
	default:
		return types.Result{Code: 2, Log: ErrInvalidEVMTx.Error()}
	}
}

func (module Module) deliver(ctx vexoapp.Context, tx types.Tx, canonical vexoapp.CanonicalTx) types.Result {
	invocation, created, err := module.admitTx(ctx, tx, canonical)
	if err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	if _, err := module.blobSidecarBundleFromTx(tx); err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	state := evmStateReader{store: ctx.Store}
	if invocation.Method != "deploy" {
		code, err := state.Code(ctx.GoContext(), invocation.Contract)
		if err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		invocation.Code = code
	}
	module.bindEnvironment(ctx, tx, &invocation)
	result, err := module.registry.Execute(ctx.GoContext(), invocation)
	if err != nil {
		return executionFailure(err)
	}
	if err := ctx.ConsumeGas(result.GasUsed); err != nil {
		return types.Result{Code: 6, Log: err.Error()}
	}
	if err := module.sweepCoinbase(ctx.GoContext(), state, &result); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	receipt := receiptFromResult(tx, ctx.Height, invocation, string(created), result)
	writes, err := module.outcomeWrites(ctx.GoContext(), ctx.Store, tx, invocation.Contract, receipt, result)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	if err := applyKVWrites(ctx.GoContext(), ctx.Store, writes); err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	return types.Result{Data: encoded, GasUsed: result.GasUsed}
}

// executionFailure maps an execution error to a failed transaction result.
// The error is either a deterministic rejection (bad nonce, insufficient funds,
// malformed payload, unknown VM) or an infrastructure failure; neither may
// abort the block from here, because the staged store that DeliverTx runs on
// latches store errors and cancellation itself.
func executionFailure(err error) types.Result {
	if errors.Is(err, contract.ErrTxRejected) || errors.Is(err, contract.ErrInvalidInvocation) || errors.Is(err, contract.ErrVMNotFound) {
		return types.Result{Code: 3, Log: err.Error()}
	}
	return types.Result{Code: 4, Log: err.Error()}
}

// admitTx checks everything about a transaction that does not depend on
// state: the action, the sender recovered from an Ethereum signature, the
// strict EVM addresses in the arguments and the signer authorization of the
// caller. It returns the invocation the arguments describe and, for a
// deployment, the address of the contract it creates.
func (module Module) admitTx(ctx vexoapp.Context, tx types.Tx, canonical vexoapp.CanonicalTx) (contract.Invocation, types.Address, error) {
	ethereum := ethcompat.IsEthereumTx(tx)
	switch canonical.Action {
	case "call":
	case "deploy":
		// deploy is the native CREATE2 action; Ethereum transactions are
		// expressed as call or eth_deploy.
		if ethereum {
			return contract.Invocation{}, "", ErrInvalidEVMTx
		}
	case "eth_deploy":
		// eth_deploy derives the contract address from an explicit nonce, so
		// only a signature-verified Ethereum transaction may use it.
		if !ethereum {
			return contract.Invocation{}, "", ErrInvalidEVMTx
		}
	default:
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	if ethereum {
		if err := module.validateEthereumRawTx(ctx, tx); err != nil {
			return contract.Invocation{}, "", err
		}
	}
	invocation, created, err := invocationFromArgs(canonical.Action, canonical.Args)
	if err != nil {
		return contract.Invocation{}, "", err
	}
	if err := vexoapp.AuthorizeActor(tx, invocation.Caller); err != nil {
		return contract.Invocation{}, "", err
	}
	return invocation, created, nil
}

func invocationFromArgs(action string, args []string) (contract.Invocation, types.Address, error) {
	switch action {
	case "call":
		invocation, err := callInvocationFromArgs(args)
		return invocation, "", err
	case "deploy":
		return deployInvocationFromArgs(args)
	case "eth_deploy":
		return ethDeployInvocationFromArgs(args)
	default:
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
}

func callInvocationFromArgs(args []string) (contract.Invocation, error) {
	if len(args) != 6 && len(args) != 7 {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	input, err := hex.DecodeString(strings.TrimPrefix(args[4], "0x"))
	if err != nil {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	gasLimit, err := strconv.ParseUint(args[5], 10, 64)
	if err != nil {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	value := new(big.Int)
	if len(args) == 7 {
		value, err = parseAmountBig(args[6])
		if err != nil {
			return contract.Invocation{}, ErrInvalidEVMTx
		}
	}
	invocation := contract.Invocation{
		VM:       args[0],
		Caller:   types.Address(args[1]),
		Contract: types.Address(args[2]),
		Method:   args[3],
		Input:    input,
		GasLimit: gasLimit,
		Value:    uint64Amount(value),
		ValueBig: value,
	}
	if invocation.VM == "" || invocation.Method == "" || !evmAddress(invocation.Caller) || !evmAddress(invocation.Contract) {
		return contract.Invocation{}, ErrInvalidEVMTx
	}
	return invocation, nil
}

func deployInvocationFromArgs(args []string) (contract.Invocation, types.Address, error) {
	if len(args) != 4 && len(args) != 5 {
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	caller := types.Address(args[1])
	code, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
	if args[0] == "" || !evmAddress(caller) || err != nil || len(code) == 0 {
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	value := new(big.Int)
	if len(args) == 5 {
		value, err = parseAmountBig(args[4])
		if err != nil {
			return contract.Invocation{}, "", ErrInvalidEVMTx
		}
	}
	salt := create2Salt(args[3])
	created := createAddress(caller, code, salt)
	return contract.Invocation{
		VM:       args[0],
		Caller:   caller,
		Contract: created,
		Method:   "deploy",
		Input:    code,
		Value:    uint64Amount(value),
		ValueBig: value,
		Salt:     salt[:],
	}, created, nil
}

func ethDeployInvocationFromArgs(args []string) (contract.Invocation, types.Address, error) {
	if len(args) != 5 {
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	caller := types.Address(args[1])
	code, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
	if args[0] == "" || !evmAddress(caller) || err != nil || len(code) == 0 {
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	nonce, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	value, err := parseAmountBig(args[4])
	if err != nil {
		return contract.Invocation{}, "", ErrInvalidEVMTx
	}
	created := createLegacyAddress(caller, nonce)
	return contract.Invocation{
		VM:       args[0],
		Caller:   caller,
		Contract: created,
		Method:   "deploy",
		Input:    code,
		Value:    uint64Amount(value),
		ValueBig: value,
	}, created, nil
}

// evmAddress reports whether address names an EVM account: 0x followed by one
// to forty hex digits. Any other string would collapse onto the zero address,
// or onto a truncated prefix of itself, inside the EVM, so it can never be a
// caller or a call target.
func evmAddress(address types.Address) bool {
	digits, found := strings.CutPrefix(string(address), "0x")
	if !found || len(digits) == 0 || len(digits) > 40 {
		return false
	}
	for index := 0; index < len(digits); index++ {
		character := digits[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}

// bindEnvironment attaches the block, fee and state context an execution runs
// against. Pricing comes from the block context and from the canonical tags
// the raw Ethereum transaction is bound to; the unsigned eth_base_fee and
// eth_blob_base_fee tags are never consulted, so a relayer cannot steer the
// fee split of someone else's transaction.
func (module Module) bindEnvironment(ctx vexoapp.Context, tx types.Tx, invocation *contract.Invocation) {
	if ctx.Store != nil {
		invocation.State = evmStateReader{store: ctx.Store}
	}
	invocation.GasLimit = module.executionGasLimit(ctx, tx, txGasLimit(tx, invocation.GasLimit))
	invocation.BlockNumber = uint64(ctx.Height)
	invocation.Timestamp = headerUnixSeconds(ctx.Header)
	invocation.BlockGasLimit = ctx.GasLimit()
	invocation.GasPrice = txGasPrice(tx)
	invocation.GasFeeCap = txGasFeeCap(tx)
	invocation.GasTipCap = txGasTipCap(tx)
	invocation.BaseFee = ctx.BaseFee
	invocation.BlobBaseFee = ctx.BlobBaseFee
	invocation.BlobGasFeeCap = txBlobGasFeeCap(tx)
	applyInvocationGasPricing(tx, invocation)
	invocation.BlobHashes = txBlobHashes(tx)
	invocation.Coinbase = coinbaseAlias
	invocation.AccessList = accessListFromTx(tx)
	invocation.Nonce = txNonce(tx)
	invocation.EthereumTx = ethcompat.IsEthereumTx(tx)
	invocation.RawEthereumTx = txRawEthereum(tx)
}

// executionGasLimit is the gas budget of one execution: what the transaction
// asks for, held to the transaction's own gas limit (its gas meter, or its gas
// tag where no meter runs) and to the consensus bound on a transaction's gas.
// The interpreter spends gas before any meter is charged, so an unbounded
// request would let a single transaction occupy every validator for as long as
// it likes. The bound is a consensus parameter, never the node-local RPC cap:
// two validators with different RPC caps must execute a block identically.
func (module Module) executionGasLimit(ctx vexoapp.Context, tx types.Tx, requested uint64) uint64 {
	limit := ctx.GasLimit()
	if limit == 0 {
		limit = vexoapp.ParseTxMeta(tx).Gas
	}
	if txCap := module.txGasCap(); limit == 0 || limit > txCap {
		limit = txCap
	}
	if requested == 0 || requested > limit {
		return limit
	}
	return requested
}

// EstimateGas runs the transaction in simulation at the current state and
// reports the gas it uses. Admission calls it for every transaction, so the
// gas the simulation may spend is bounded like any other execution.
func (module Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	if module.initErr != nil {
		return 0, module.initErr
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil || canonical.Module != ModuleName {
		return 0, ErrInvalidEVMTx
	}
	invocation, _, err := invocationFromArgs(canonical.Action, canonical.Args)
	if err != nil {
		return 0, err
	}
	if canonical.Action == "call" && ctx.Store != nil {
		code, err := evmStateReader{store: ctx.Store}.Code(ctx.GoContext(), invocation.Contract)
		if err != nil {
			return 0, err
		}
		invocation.Code = code
	}
	module.bindEnvironment(ctx, tx, &invocation)
	return module.estimateInvocationGas(ctx, invocation)
}

func (module Module) txGasCap() uint64 {
	if module.policy.MaxTxGas == 0 {
		return DefaultPolicy().MaxTxGas
	}
	return module.policy.MaxTxGas
}

func (module Module) gasCap() uint64 {
	if module.policy.RPCGasCap == 0 {
		return DefaultPolicy().RPCGasCap
	}
	return module.policy.RPCGasCap
}

// sweepCoinbase moves the balance an execution left on the coinbase alias to
// the bank fee collector, so priority fees reach the fee collector in the same
// write batch as the rest of the execution. Anything a contract pays to
// block.coinbase is swept with them.
func (module Module) sweepCoinbase(ctx context.Context, state evmStateReader, result *contract.Result) error {
	aliasKey := canonicalAddressKey(coinbaseAlias)
	collector := module.policy.FeeCollector
	if collector == "" {
		collector = defaultFeeCollector
	}
	collectorKey := canonicalAddressKey(collector)
	if aliasKey == collectorKey {
		return nil
	}
	swept := new(big.Int)
	collectorIndex := -1
	writes := make([]contract.BalanceWrite, 0, len(result.BalanceWrites)+1)
	for _, write := range result.BalanceWrites {
		switch canonicalAddressKey(write.Address) {
		case aliasKey:
			swept.Add(swept, balanceWriteBig(write))
			write = contract.BalanceWrite{Address: write.Address}
		case collectorKey:
			collectorIndex = len(writes)
		}
		writes = append(writes, write)
	}
	if swept.Sign() == 0 {
		return nil
	}
	var balance *big.Int
	if collectorIndex >= 0 {
		balance = balanceWriteBig(writes[collectorIndex])
	} else {
		var err error
		balance, err = state.BalanceBig(ctx, collector)
		if err != nil {
			return err
		}
	}
	balance = new(big.Int).Add(balance, swept)
	if balance.BitLen() > 256 {
		return ErrInvalidEVMTx
	}
	credited := contract.BalanceWrite{Address: collector, BalanceBig: balance}
	if balance.IsUint64() {
		credited.Balance = balance.Uint64()
	}
	if collectorIndex >= 0 {
		writes[collectorIndex] = credited
	} else {
		writes = append(writes, credited)
	}
	result.BalanceWrites = writes
	return nil
}

func balanceWriteBig(write contract.BalanceWrite) *big.Int {
	if write.BalanceBig != nil {
		return new(big.Int).Set(write.BalanceBig)
	}
	return new(big.Int).SetUint64(write.Balance)
}

// outcomeWrites assembles every write one execution produces: the execution
// result, the receipt with its logs and the blob sidecar. DeliverTx applies
// them in a single batch, so a transaction lands completely or not at all.
func (module Module) outcomeWrites(ctx context.Context, store vexoapp.StateStore, tx types.Tx, defaultAddress types.Address, receipt Receipt, result contract.Result) ([]vexostore.KVWrite, error) {
	writes, err := executionResultWrites(ctx, store, defaultAddress, result)
	if err != nil {
		return nil, err
	}
	receiptWrites, err := receiptWrites(ctx, store, receipt)
	if err != nil {
		return nil, err
	}
	writes = append(writes, receiptWrites...)
	sidecarWrites, err := module.blobSidecarWrites(tx)
	if err != nil {
		return nil, err
	}
	return append(writes, sidecarWrites...), nil
}

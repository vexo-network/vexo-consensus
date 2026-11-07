package app

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"

	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrEmptyChainID         = errors.New("chain id is required")
	ErrBlobGasLimitExceeded = errors.New("block blob gas exceeds configured max blob gas")
	ErrExclusiveTxConflict  = errors.New("block carries two transactions that compete for one key")

	errInvalidTransaction = errors.New("invalid transaction")
	errModulePanic        = errors.New("module panicked while executing transaction")
)

type ModuleRouter interface {
	RouteTx(ctx Context, tx types.Tx, modules []Module) (Module, error)
}

type Runtime struct {
	mu         sync.RWMutex
	chainID    string
	evmChainID uint64
	modules    []Module
	router     ModuleRouter
	ante       *AnteKeeper
	store      StateStore
	height     types.Height
	appHash    types.Hash
}

func (runtime *Runtime) WithEVMChainID(chainID uint64) *Runtime {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.evmChainID = chainID
	return runtime
}

func NewRuntime(chainID string, modules []Module, router ModuleRouter) (*Runtime, error) {
	if chainID == "" {
		return nil, ErrEmptyChainID
	}
	if router == nil {
		router = FirstModuleRouter{}
	}
	return &Runtime{
		chainID: chainID,
		modules: append([]Module(nil), modules...),
		router:  router,
	}, nil
}

func (runtime *Runtime) WithStore(store StateStore) *Runtime {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.store = store
	return runtime
}

// WithAnte installs the ante handler. Without one the runtime does not
// authenticate, meter or charge transactions; it still parses them strictly.
func (runtime *Runtime) WithAnte(ante *AnteKeeper) *Runtime {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.ante = ante
	return runtime
}

func (runtime *Runtime) NewReplayApp(store StateStore) (Application, error) {
	runtime.mu.RLock()
	chainID := runtime.chainID
	evmChainID := runtime.evmChainID
	router := runtime.router
	ante := runtime.ante
	runtime.mu.RUnlock()
	modules := make([]Module, 0, len(runtime.modules))
	for _, module := range runtime.modules {
		if cloner, ok := module.(ModuleCloner); ok {
			modules = append(modules, cloner.CloneModule())
			continue
		}
		modules = append(modules, module)
	}
	replayRuntime, err := NewRuntime(chainID, modules, router)
	if err != nil {
		return nil, err
	}
	replayRuntime.evmChainID = evmChainID
	replayRuntime.ante = ante
	if store != nil {
		replayRuntime.WithStore(store)
	}
	return replayRuntime, nil
}

func (runtime *Runtime) InitChain(req InitChainRequest) (InitChainResponse, error) {
	chainID := req.ChainID
	if chainID == "" {
		runtime.mu.RLock()
		chainID = runtime.chainID
		runtime.mu.RUnlock()
	}
	if chainID == "" {
		return InitChainResponse{}, ErrEmptyChainID
	}
	runtime.mu.Lock()
	runtime.chainID = chainID
	runtime.mu.Unlock()

	store := runtime.currentStore()
	ctx, err := runtime.newContext(context.Background(), 0, types.Header{}, store)
	if err != nil {
		return InitChainResponse{}, err
	}
	for _, module := range runtime.modules {
		ctx.GenesisValidators = cloneValidatorUpdates(req.Validators)
		if err := module.InitGenesis(ctx, req.Genesis); err != nil {
			return InitChainResponse{}, err
		}
	}
	appHash, err := runtime.computeAppHash(ctx.GoContext(), 0, store)
	if err != nil {
		return InitChainResponse{}, err
	}
	runtime.mu.Lock()
	runtime.appHash = appHash
	runtime.mu.Unlock()
	return InitChainResponse{AppHash: appHash}, nil
}

func (runtime *Runtime) CheckTx(tx types.Tx) CheckTxResponse {
	return runtime.CheckTxContext(context.Background(), tx)
}

// CheckTxContext admits one transaction against the committed state: its
// envelope, fee, gas and nonce, the payer's balance and the module's own
// validation and gas estimate.
func (runtime *Runtime) CheckTxContext(goCtx context.Context, tx types.Tx) CheckTxResponse {
	if goCtx == nil {
		goCtx = context.Background()
	}
	if err := goCtx.Err(); err != nil {
		return rejectedCheck(err)
	}
	ctx, err := runtime.newContext(goCtx, runtime.currentHeight(), types.Header{}, runtime.currentStore())
	if err != nil {
		return rejectedCheck(err)
	}
	adm, err := runtime.admit(ctx, tx)
	if err != nil {
		return rejectedCheck(err)
	}
	if err := runtime.currentAnte().newSimulation(ctx).apply(adm); err != nil {
		return rejectedCheck(err)
	}
	if err := runtime.checkModule(ctx, adm); err != nil {
		return rejectedCheck(err)
	}
	return CheckTxResponse{Result: types.Result{}}
}

func (runtime *Runtime) CheckTxsContext(goCtx context.Context, txs []types.Tx) CheckTxResponse {
	if err := runtime.validateTxsContext(goCtx, runtime.currentHeight(), types.Header{}, txs); err != nil {
		return rejectedCheck(err)
	}
	return CheckTxResponse{Result: types.Result{}}
}

func rejectedCheck(err error) CheckTxResponse {
	return CheckTxResponse{Result: types.Result{Code: 1, Log: err.Error()}}
}

func (runtime *Runtime) PrepareProposal(req PrepareProposalRequest) (PrepareProposalResponse, error) {
	return runtime.PrepareProposalContext(context.Background(), req)
}

// preparedTx is a candidate transaction with its stateless admission.
type preparedTx struct {
	tx  types.Tx
	adm admission
	// key is what the transaction competes for with the others of the block, "" when for nothing.
	key string
}

// PrepareProposalContext selects the transactions a block may carry: every
// transaction must be admissible after the ones before it, because a block is
// validated cumulatively (consecutive nonces, fees affordable together).
func (runtime *Runtime) PrepareProposalContext(goCtx context.Context, req PrepareProposalRequest) (PrepareProposalResponse, error) {
	if goCtx == nil {
		goCtx = context.Background()
	}
	if err := goCtx.Err(); err != nil {
		return PrepareProposalResponse{}, err
	}
	salt := runtime.orderingSalt(req.Height)
	ordered := fairordering.SortTxsWithSalt(req.Txs, salt)
	ctx, err := runtime.newContext(goCtx, req.Height, types.Header{}, runtime.currentStore())
	if err != nil {
		return PrepareProposalResponse{}, err
	}
	items := make([]preparedTx, 0, len(ordered))
	for _, tx := range ordered {
		if err := goCtx.Err(); err != nil {
			return PrepareProposalResponse{}, err
		}
		adm, err := runtime.admit(ctx, tx)
		if err != nil {
			continue
		}
		module, err := runtime.route(ctx, adm)
		if err != nil {
			continue
		}
		if err := runtime.validateRouted(ctx, adm, module); err != nil {
			continue
		}
		key, err := exclusiveKey(module, adm)
		if err != nil {
			continue
		}
		items = append(items, preparedTx{tx: append(types.Tx(nil), tx...), adm: adm, key: key})
	}
	if len(items) != len(ordered) {
		items = orderPrepared(items, salt)
	}
	ante := runtime.currentAnte()
	for {
		if err := goCtx.Err(); err != nil {
			return PrepareProposalResponse{}, err
		}
		sim := ante.newSimulation(ctx)
		kept := make([]preparedTx, 0, len(items))
		var blobGas uint64
		// The first transaction of a key in the order of the block keeps it, as a validator of the
		// block will find. A transaction that fails the simulation keeps nothing.
		claimed := make(map[string]struct{})
		for _, item := range items {
			if ante != nil && ante.config.FeeMarket.MaxBlobGas > 0 && addSaturating(blobGas, item.adm.blobGas) > ante.config.FeeMarket.MaxBlobGas {
				continue
			}
			if _, taken := claimed[item.key]; taken {
				continue
			}
			if err := sim.apply(item.adm); err != nil {
				continue
			}
			blobGas = addSaturating(blobGas, item.adm.blobGas)
			if item.key != "" {
				claimed[item.key] = struct{}{}
			}
			kept = append(kept, item)
		}
		if len(kept) == len(items) {
			break
		}
		// Dropping a transaction can change the fair order of the rest, and the
		// order is part of what a proposal is validated against.
		items = orderPrepared(kept, salt)
	}
	txs := make([]types.Tx, 0, len(items))
	for _, item := range items {
		txs = append(txs, item.tx)
	}
	return PrepareProposalResponse{Txs: txs}, nil
}

func orderPrepared(items []preparedTx, salt []byte) []preparedTx {
	txs := make([]types.Tx, 0, len(items))
	buckets := make(map[string][]preparedTx, len(items))
	for _, item := range items {
		txs = append(txs, item.tx)
		buckets[string(item.tx)] = append(buckets[string(item.tx)], item)
	}
	ordered := make([]preparedTx, 0, len(items))
	for _, tx := range fairordering.SortTxsWithSalt(txs, salt) {
		bucket := buckets[string(tx)]
		ordered = append(ordered, bucket[0])
		buckets[string(tx)] = bucket[1:]
	}
	return ordered
}

func (runtime *Runtime) ProcessProposal(req ProcessProposalRequest) ProcessProposalResponse {
	return runtime.ProcessProposalContext(context.Background(), req)
}

func (runtime *Runtime) ProcessProposalContext(goCtx context.Context, req ProcessProposalRequest) ProcessProposalResponse {
	if goCtx == nil {
		goCtx = context.Background()
	}
	if err := goCtx.Err(); err != nil {
		return ProcessProposalResponse{Accepted: false, Reason: err.Error()}
	}
	if !fairordering.IsOrderedWithSalt(req.Block.Txs, runtime.orderingSalt(req.Block.Header.Height)) {
		return ProcessProposalResponse{Accepted: false, Reason: "transaction ordering mismatch"}
	}
	if err := runtime.validateTxsContext(goCtx, req.Block.Header.Height, req.Block.Header, req.Block.Txs); err != nil {
		return ProcessProposalResponse{Accepted: false, Reason: err.Error()}
	}
	return ProcessProposalResponse{Accepted: true}
}

// validateTxsContext admits txs as one block against the committed state. The
// account state advances from transaction to transaction, so a sender's nonces
// must be consecutive and its fees affordable together, and the blob gas of
// the whole block must fit the limit.
func (runtime *Runtime) validateTxsContext(goCtx context.Context, height types.Height, header types.Header, txs []types.Tx) error {
	if goCtx == nil {
		goCtx = context.Background()
	}
	if err := goCtx.Err(); err != nil {
		return err
	}
	ctx, err := runtime.newContext(goCtx, height, header, runtime.currentStore())
	if err != nil {
		return err
	}
	ante := runtime.currentAnte()
	sim := ante.newSimulation(ctx)
	var blobGas uint64
	claimed := make(map[string]struct{})
	for _, tx := range txs {
		if err := goCtx.Err(); err != nil {
			return err
		}
		adm, err := runtime.admit(ctx, tx)
		if err != nil {
			return err
		}
		blobGas = addSaturating(blobGas, adm.blobGas)
		if err := sim.apply(adm); err != nil {
			return err
		}
		module, err := runtime.route(ctx, adm)
		if err != nil {
			return errInvalidTransaction
		}
		if err := runtime.validateRouted(ctx, adm, module); err != nil {
			return err
		}
		key, err := exclusiveKey(module, adm)
		if err != nil {
			return err
		}
		if key == "" {
			continue
		}
		if _, taken := claimed[key]; taken {
			return ErrExclusiveTxConflict
		}
		claimed[key] = struct{}{}
	}
	if ante != nil && ante.config.FeeMarket.MaxBlobGas > 0 && blobGas > ante.config.FeeMarket.MaxBlobGas {
		return ErrBlobGasLimitExceeded
	}
	return nil
}

// admit parses and, with an ante handler, authenticates and prices tx.
func (runtime *Runtime) admit(ctx Context, tx types.Tx) (admission, error) {
	if ante := runtime.currentAnte(); ante != nil {
		return ante.admit(ctx, tx)
	}
	return parseAdmission(tx)
}

// route finds the module that executes adm. An Ethereum-authenticated
// transaction may only reach the evm module, whatever the router says.
func (runtime *Runtime) route(ctx Context, adm admission) (Module, error) {
	module, err := runtime.router.RouteTx(ctx, adm.payload, runtime.modules)
	if err != nil {
		return nil, err
	}
	if adm.ethereum && module.Name() != ethereumModuleName {
		return nil, ErrInvalidTxSignature
	}
	return module, nil
}

// checkModule routes adm and runs the module's stateless validation and gas
// estimate against the state in ctx.
func (runtime *Runtime) checkModule(ctx Context, adm admission) error {
	module, err := runtime.route(ctx, adm)
	if err != nil {
		return err
	}
	return runtime.validateRouted(ctx, adm, module)
}

// exclusiveKey returns the key under which adm competes with the other transactions of a block,
// qualified by the module that owns it, or "" when it competes for nothing.
func exclusiveKey(module Module, adm admission) (string, error) {
	exclusive, ok := module.(BlockExclusiveTx)
	if !ok {
		return "", nil
	}
	key, err := exclusive.ExclusiveTxKey(adm.payload)
	if err != nil || key == "" {
		return "", err
	}
	return module.Name() + "/" + key, nil
}

func (runtime *Runtime) validateRouted(ctx Context, adm admission, module Module) error {
	if validator, ok := module.(TxValidator); ok {
		if err := validator.ValidateTx(ctx, adm.payload); err != nil {
			return err
		}
	}
	if runtime.currentAnte() == nil {
		return nil
	}
	estimator, ok := module.(GasEstimator)
	if !ok {
		return nil
	}
	required, err := estimator.EstimateGas(ctx, adm.payload)
	if err != nil {
		return err
	}
	if adm.gasLimit > 0 && required > adm.gasLimit {
		return ErrOutOfGas
	}
	return nil
}

func (runtime *Runtime) FinalizeBlock(req FinalizeBlockRequest) (FinalizeBlockResponse, error) {
	return runtime.FinalizeBlockContext(context.Background(), req)
}

func (runtime *Runtime) FinalizeBlockContext(goCtx context.Context, req FinalizeBlockRequest) (FinalizeBlockResponse, error) {
	response, _, err := runtime.finalizeBlock(goCtx, req, true)
	return response, err
}

func (runtime *Runtime) FinalizeBlockStaged(req FinalizeBlockRequest) (FinalizeBlockResponse, []kvbatch.KVWrite, error) {
	return runtime.FinalizeBlockStagedContext(context.Background(), req)
}

func (runtime *Runtime) FinalizeBlockStagedContext(goCtx context.Context, req FinalizeBlockRequest) (FinalizeBlockResponse, []kvbatch.KVWrite, error) {
	if runtime.currentStore() == nil {
		response, _, err := runtime.finalizeBlock(goCtx, req, true)
		return response, nil, err
	}
	return runtime.finalizeBlock(goCtx, req, false)
}

func (runtime *Runtime) CommitStagedBlock(height types.Height, appHash types.Hash) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.height = height
	runtime.appHash = appHash
}

// txOutcome is what executing one transaction produced.
type txOutcome struct {
	result  types.Result
	events  []events.Event
	blobGas uint64
}

// finalizeBlock executes a block. A transaction never fails the block: one that
// cannot be admitted or whose module rejects it yields a result with a non-zero
// code, and only the writes of successful transactions reach the block. Only
// infrastructure failures (a store error, a cancelled context, a module's
// BeginBlock/EndBlock error) return an error.
//
// With direct set, the writes are applied to the runtime's store and the
// runtime advances; otherwise they are returned for the caller to commit
// atomically with the block.
func (runtime *Runtime) finalizeBlock(goCtx context.Context, req FinalizeBlockRequest, direct bool) (FinalizeBlockResponse, []kvbatch.KVWrite, error) {
	if goCtx == nil {
		goCtx = context.Background()
	}
	if err := goCtx.Err(); err != nil {
		return FinalizeBlockResponse{}, nil, err
	}
	base := runtime.currentStore()
	var layer *StagedStore
	var blockStore StateStore
	if base != nil {
		layer = NewStagedStore(base)
		blockStore = layer
	}
	ctx, err := runtime.newContext(goCtx, req.Block.Header.Height, req.Block.Header, blockStore)
	if err != nil {
		return FinalizeBlockResponse{}, nil, err
	}
	ante := runtime.currentAnte()

	for _, module := range runtime.modules {
		if err := module.BeginBlock(ctx, req.Block.Header); err != nil {
			return FinalizeBlockResponse{}, nil, err
		}
	}

	results := make([]types.Result, 0, len(req.Block.Txs))
	txEvents := make([][]events.Event, 0, len(req.Block.Txs))
	var gasUsed, blobGasUsed uint64
	for _, tx := range req.Block.Txs {
		outcome, err := runtime.deliverTx(ctx, layer, tx)
		if err != nil {
			return FinalizeBlockResponse{}, nil, err
		}
		gasUsed = addSaturating(gasUsed, outcome.result.GasUsed)
		blobGasUsed = addSaturating(blobGasUsed, outcome.blobGas)
		txEvents = append(txEvents, outcome.events)
		results = append(results, outcome.result)
	}

	ctx.Block = req.Block
	ctx.TxResults = cloneResults(results)
	for _, module := range runtime.modules {
		if err := module.EndBlock(ctx); err != nil {
			return FinalizeBlockResponse{}, nil, err
		}
	}
	feeMarket := FeeMarketReport{BaseFee: ctx.BaseFee, NextBaseFee: ctx.BaseFee, BlobBaseFee: ctx.BlobBaseFee, NextBlobBaseFee: ctx.BlobBaseFee, BlobGasUsed: blobGasUsed}
	if ante != nil && blockStore != nil {
		next := ante.nextFeeMarket(feeMarketState{baseFee: ctx.BaseFee, blobBaseFee: ctx.BlobBaseFee}, gasUsed, blobGasUsed)
		if err := ante.storeFeeMarket(goCtx, blockStore, next); err != nil {
			return FinalizeBlockResponse{}, nil, err
		}
		feeMarket.NextBaseFee = next.baseFee
		feeMarket.NextBlobBaseFee = next.blobBaseFee
	}
	if kvStore, ok := blockStore.(vexostore.KVStore); ok {
		if err := events.NewIndexer(kvStore).IndexBlock(goCtx, req.Block.Header.Height, txEvents); err != nil {
			return FinalizeBlockResponse{}, nil, err
		}
	}
	if err := firstError(goCtx.Err(), layer.Fault()); err != nil {
		return FinalizeBlockResponse{}, nil, err
	}

	var rootStore StateStore
	var writes []kvbatch.KVWrite
	if layer != nil {
		rootStore = layer
		writes = layer.Writes()
		if direct {
			if err := layer.ApplyTo(goCtx, base); err != nil {
				return FinalizeBlockResponse{}, nil, err
			}
			rootStore = base
			writes = nil
		}
	}
	appHash, err := runtime.computeAppHash(goCtx, req.Block.Header.Height, rootStore)
	if err != nil {
		return FinalizeBlockResponse{}, nil, err
	}
	if direct {
		runtime.mu.Lock()
		runtime.height = req.Block.Header.Height
		runtime.appHash = appHash
		runtime.mu.Unlock()
	}
	return FinalizeBlockResponse{
		Results:          results,
		TxEvents:         txEvents,
		AppHash:          appHash,
		ValidatorUpdates: runtime.collectValidatorUpdates(ctx),
		FeeMarket:        feeMarket,
	}, writes, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// deliverTx executes one transaction of a block.
//
// The transaction is admitted again against the block's state (the proposal
// check ran against the committed state). One that fails admission, cannot be
// routed or cannot pay its fee is a failed result that changed nothing. Past
// that gate the fee is charged and the sequence advanced in the block, and the
// module runs in a layer of its own that only a successful result merges into
// the block.
func (runtime *Runtime) deliverTx(ctx Context, block *StagedStore, tx types.Tx) (txOutcome, error) {
	ante := runtime.currentAnte()
	adm, err := runtime.admit(ctx, tx)
	if err != nil {
		return rejectedTx(err), nil
	}
	outcome := txOutcome{blobGas: adm.blobGas}
	module, err := runtime.route(ctx, adm)
	if err != nil {
		outcome.result = failedResult(err)
		return outcome, nil
	}
	if ante != nil {
		if err := ante.begin(ctx, adm); err != nil {
			if fault := block.Fault(); fault != nil {
				return txOutcome{}, fault
			}
			outcome.result = failedResult(err)
			return outcome, nil
		}
	}

	txCtx := ctx
	var layer *StagedStore
	if block != nil {
		layer = NewStagedStore(block)
		txCtx.Store = layer
	}
	if ante != nil {
		txCtx = txCtx.WithGasMeter(NewGasMeter(adm.gasLimit))
	}
	result := deliverGuarded(module, txCtx, adm.payload)
	if err := firstError(ctx.GoContext().Err(), layer.Fault(), block.Fault()); err != nil {
		return txOutcome{}, err
	}

	// chargedFee is the fee the transaction paid, nil when it paid none.
	var chargedFee *big.Int
	if ante != nil && !adm.ethereum && adm.payer != "" {
		chargedFee = adm.meta.Fee
	}
	if result.Code == 0 {
		if layer != nil {
			if err := layer.ApplyTo(ctx.GoContext(), block); err != nil {
				return txOutcome{}, err
			}
		}
		if ante != nil {
			if err := ante.finish(ctx, adm); err != nil {
				return txOutcome{}, err
			}
		}
		outcome.events = runtime.collectTxEvents(txCtx, module, adm.payload, result)
	} else if ante != nil && adm.ethereum {
		paid, err := ante.fail(ctx, adm)
		if err != nil {
			return txOutcome{}, err
		}
		chargedFee = paid
	}
	if ante != nil {
		result = settleResult(result, txCtx, adm, chargedFee)
	}
	outcome.result = result
	return outcome, firstError(block.Fault())
}

// settleResult fills the gas and fee a transaction is reported with. A failed
// transaction reports the fee it was charged, nil when it was charged none.
func settleResult(result types.Result, ctx Context, adm admission, chargedFee *big.Int) types.Result {
	gasUsed := ctx.GasUsed()
	if gasUsed == 0 {
		gasUsed = adm.gasLimit
	}
	result.GasUsed = 0
	result.FeePaid = 0
	switch {
	case result.Code == 0:
		result.GasUsed = gasUsed
		result.FeePaid = adm.meta.FeeSaturated()
	case chargedFee != nil:
		result.GasUsed = gasUsed
		result.FeePaid = saturatedUint64(chargedFee)
	}
	if result.Code == 0 && len(result.Data) == 0 {
		result.Data = []byte(fmt.Sprintf("gas_used=%d fee_paid=%d", result.GasUsed, result.FeePaid))
	}
	return result
}

func saturatedUint64(value *big.Int) uint64 {
	if !value.IsUint64() {
		return math.MaxUint64
	}
	return value.Uint64()
}

func rejectedTx(err error) txOutcome {
	return txOutcome{result: failedResult(err)}
}

func failedResult(err error) types.Result {
	return types.Result{Code: 1, Log: err.Error()}
}

// deliverGuarded runs the module and turns a panic into a failed result. A
// panic is deterministic for the same input, and letting it escape would let
// one transaction crash every validator.
func deliverGuarded(module Module, ctx Context, payload types.Tx) (result types.Result) {
	defer func() {
		if recover() != nil {
			result = failedResult(errModulePanic)
		}
	}()
	return module.DeliverTx(ctx, payload)
}

func (runtime *Runtime) collectTxEvents(ctx Context, module Module, tx types.Tx, result types.Result) []events.Event {
	emitter, ok := module.(TxEventEmitter)
	if !ok {
		return nil
	}
	emitted := emitter.Events(ctx, tx, result)
	copied := make([]events.Event, 0, len(emitted))
	for _, event := range emitted {
		event.Attributes = append([]events.Attribute(nil), event.Attributes...)
		copied = append(copied, event)
	}
	return copied
}

func (runtime *Runtime) Commit() (CommitResponse, error) {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return CommitResponse{Height: runtime.height, AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) Restore(height types.Height, appHash types.Hash) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.height = height
	runtime.appHash = appHash
}

func (runtime *Runtime) Query(req QueryRequest) QueryResponse {
	return runtime.QueryContext(context.Background(), req)
}

func (runtime *Runtime) QueryContext(goCtx context.Context, req QueryRequest) QueryResponse {
	if goCtx == nil {
		goCtx = context.Background()
	}
	if err := goCtx.Err(); err != nil {
		return QueryResponse{Code: 1, Log: err.Error()}
	}
	if len(req.Path) == 0 || req.Path[0] == "" {
		return QueryResponse{Code: 1, Log: "query module is required"}
	}
	ctx, err := runtime.newContext(goCtx, runtime.currentHeight(), types.Header{}, runtime.currentStore())
	if err != nil {
		return QueryResponse{Code: 1, Log: err.Error()}
	}
	for _, module := range runtime.modules {
		if module.Name() != req.Path[0] {
			continue
		}
		handler, ok := module.(QueryHandler)
		if !ok {
			return QueryResponse{Code: 2, Log: "module query is unavailable"}
		}
		return handler.Query(ctx, QueryRequest{
			Path: append([]string(nil), req.Path[1:]...),
			Data: append([]byte(nil), req.Data...),
		})
	}
	return QueryResponse{Code: 3, Log: "query module not found"}
}

// newContext builds the context of one operation over store. The fee market
// the operation is priced against is read from that store, so every operation
// sees the base fee of the state it runs on.
func (runtime *Runtime) newContext(goCtx context.Context, height types.Height, header types.Header, store StateStore) (Context, error) {
	if goCtx == nil {
		goCtx = context.Background()
	}
	runtime.mu.RLock()
	ctx := Context{
		Ctx:        goCtx,
		ChainID:    runtime.chainID,
		EVMChainID: runtime.evmChainID,
		Height:     height,
		Header:     header,
		Store:      store,
	}
	ante := runtime.ante
	runtime.mu.RUnlock()
	if ante != nil {
		market, err := ante.loadFeeMarket(goCtx, store)
		if err != nil {
			return Context{}, err
		}
		ctx.BaseFee = market.baseFee
		ctx.BlobBaseFee = market.blobBaseFee
	}
	return ctx, nil
}

func (runtime *Runtime) Modules() []Module {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return append([]Module(nil), runtime.modules...)
}

// collectValidatorUpdates gathers the validator updates of every module. Modules report in
// registration order, and a module registered later sees the effects of the earlier ones, so when
// several modules update one validator only the last update is kept: that is the validator's
// standing at the end of the block, and the registry applies it once.
func (runtime *Runtime) collectValidatorUpdates(ctx Context) []types.ValidatorUpdate {
	collected := make([]types.ValidatorUpdate, 0)
	for _, module := range runtime.modules {
		provider, ok := module.(ValidatorUpdateProvider)
		if !ok {
			continue
		}
		for _, update := range provider.ValidatorUpdates(ctx) {
			collected = append(collected, cloneValidatorUpdate(update))
		}
	}
	last := make(map[types.ValidatorID]int, len(collected))
	for index, update := range collected {
		last[validatorUpdateID(update)] = index
	}
	updates := make([]types.ValidatorUpdate, 0, len(last))
	for index, update := range collected {
		if last[validatorUpdateID(update)] == index {
			updates = append(updates, update)
		}
	}
	return updates
}

// validatorUpdateID is the validator an update is for: its ID, or its address when it names no ID.
func validatorUpdateID(update types.ValidatorUpdate) types.ValidatorID {
	if update.ID != "" {
		return update.ID
	}
	return types.ValidatorID(update.Address)
}

func (runtime *Runtime) orderingSalt(height types.Height) []byte {
	runtime.mu.RLock()
	chainID := runtime.chainID
	runtime.mu.RUnlock()
	return fairordering.HeightSalt(chainID, height)
}

func cloneValidatorUpdates(updates []types.ValidatorUpdate) []types.ValidatorUpdate {
	if updates == nil {
		return nil
	}
	cloned := make([]types.ValidatorUpdate, 0, len(updates))
	for _, update := range updates {
		cloned = append(cloned, cloneValidatorUpdate(update))
	}
	return cloned
}

func cloneValidatorUpdate(update types.ValidatorUpdate) types.ValidatorUpdate {
	update.PublicKey = append(types.PublicKey(nil), update.PublicKey...)
	if update.Metadata != nil {
		metadata := make(map[string]string, len(update.Metadata))
		for key, value := range update.Metadata {
			metadata[key] = value
		}
		update.Metadata = metadata
	}
	return update
}

func cloneResults(results []types.Result) []types.Result {
	cloned := make([]types.Result, len(results))
	for index, result := range results {
		cloned[index] = result
		cloned[index].Data = append([]byte(nil), result.Data...)
	}
	return cloned
}

// computeAppHash commits to the chain, the height and the root of every state
// namespace: the modules' own and the runtime's (account sequences and the fee
// market). Without a store there is no state to commit to and the hash covers
// the chain and the height only. A store that cannot report roots, or fails to
// report one, fails the hash: it must never silently omit a namespace.
//
// The validator registry (validator.RegistryNamespace) is not one of the
// namespaces. The runtime writes it in the same atomic commit as the block,
// after this hash is computed, so a root over it here would describe the
// registry before the block that changed it. The block header commits it
// instead: its AppHash is finality.HeaderAppHash of this hash and the registry
// root after the block, so the validator sets of past heights are as certified
// as the application state.
// AppHashOfRoots returns the app hash of a state of height whose namespaces have the roots reports:
// the hash this runtime commits for a block of height executed into that state. A snapshot restore
// recomputes it from the namespace roots the snapshot carries, without a store.
func (runtime *Runtime) AppHashOfRoots(ctx context.Context, height types.Height, roots NamespaceRoots) (types.Hash, error) {
	if roots == nil {
		return types.Hash{}, errors.New("namespace roots are required")
	}
	return runtime.computeAppHash(ctx, height, roots)
}

func (runtime *Runtime) computeAppHash(goCtx context.Context, height types.Height, stateStore any) (types.Hash, error) {
	if goCtx == nil {
		goCtx = context.Background()
	}
	hasher := sha256.New()
	runtime.mu.RLock()
	chainID := runtime.chainID
	modules := append([]Module(nil), runtime.modules...)
	runtime.mu.RUnlock()
	hasher.Write([]byte(chainID))

	var heightBuffer [8]byte
	binary.BigEndian.PutUint64(heightBuffer[:], uint64(height))
	hasher.Write(heightBuffer[:])

	if stateStore != nil {
		rootStore, ok := stateStore.(NamespaceRoots)
		if !ok {
			return types.Hash{}, fmt.Errorf("state store %T cannot report namespace roots", stateStore)
		}
		namespaces := make([]string, 0, len(modules)+2)
		for _, module := range modules {
			namespaces = append(namespaces, module.Name())
		}
		namespaces = append(namespaces, CoreNamespaces()...)
		for _, namespace := range namespaces {
			root, err := rootStore.Root(goCtx, namespace)
			if err != nil {
				return types.Hash{}, fmt.Errorf("state root of namespace %q: %w", namespace, err)
			}
			hasher.Write([]byte(namespace))
			hasher.Write(root[:])
		}
	}

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash, nil
}

func (runtime *Runtime) currentHeight() types.Height {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.height
}

func (runtime *Runtime) currentStore() StateStore {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.store
}

func (runtime *Runtime) currentAnte() *AnteKeeper {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.ante
}

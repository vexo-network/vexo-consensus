package app

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrEmptyChainID           = errors.New("chain id is required")
	ErrProposalRejected       = errors.New("proposal rejected")
	ErrReplayCloneUnavailable = errors.New("replay clone unavailable")
)

type ModuleRouter interface {
	RouteTx(ctx Context, tx types.Tx, modules []Module) (Module, error)
}

type Runtime struct {
	chainID string
	modules []Module
	router  ModuleRouter
	ante    AnteHandler
	store   StateStore
	height  types.Height
	appHash types.Hash
}

type BaseFeeSetter interface {
	SetBaseFee(baseFee uint64)
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
	runtime.store = store
	runtime.bindStore()
	return runtime
}

func (runtime *Runtime) BindStore() error {
	return runtime.bindStoreWithContext(runtime.newContext(runtime.height, types.Header{}))
}

func (runtime *Runtime) WithAnte(ante AnteHandler) *Runtime {
	runtime.ante = ante
	return runtime
}

func (runtime *Runtime) SetBaseFee(baseFee uint64) {
	setter, ok := runtime.ante.(BaseFeeSetter)
	if !ok {
		return
	}
	setter.SetBaseFee(baseFee)
}

func (runtime *Runtime) NewReplayApp(store StateStore) (Application, error) {
	modules := make([]Module, 0, len(runtime.modules))
	for _, module := range runtime.modules {
		if cloner, ok := module.(ModuleCloner); ok {
			modules = append(modules, cloner.CloneModule())
			continue
		}
		if _, requiresBinding := module.(StoreBinder); requiresBinding {
			return nil, ErrReplayCloneUnavailable
		}
		modules = append(modules, module)
	}
	replayRuntime, err := NewRuntime(runtime.chainID, modules, runtime.router)
	if err != nil {
		return nil, err
	}
	replayRuntime.ante = runtime.ante
	if store != nil {
		replayRuntime.WithStore(store)
	}
	return replayRuntime, nil
}

func (runtime *Runtime) InitChain(req InitChainRequest) (InitChainResponse, error) {
	chainID := req.ChainID
	if chainID == "" {
		chainID = runtime.chainID
	}
	if chainID == "" {
		return InitChainResponse{}, ErrEmptyChainID
	}
	runtime.chainID = chainID

	ctx := runtime.newContext(0, types.Header{})
	if err := runtime.bindStoreWithContext(ctx); err != nil {
		return InitChainResponse{}, err
	}
	for _, module := range runtime.modules {
		if err := module.InitGenesis(ctx, req.Genesis); err != nil {
			return InitChainResponse{}, err
		}
	}
	runtime.appHash = runtime.computeAppHash()
	return InitChainResponse{AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) CheckTx(tx types.Tx) CheckTxResponse {
	ctx := runtime.newContext(runtime.height, types.Header{})
	if runtime.ante != nil {
		if err := runtime.ante.CheckTx(ctx, tx); err != nil {
			return CheckTxResponse{Result: types.Result{Code: 1, Log: err.Error()}}
		}
	}
	payload := TxPayload(tx)
	module, err := runtime.router.RouteTx(ctx, payload, runtime.modules)
	if err != nil {
		return CheckTxResponse{Result: types.Result{Code: 1, Log: err.Error()}}
	}
	if err := runtime.checkEstimatedGas(ctx, tx, payload, module); err != nil {
		return CheckTxResponse{Result: types.Result{Code: 1, Log: err.Error()}}
	}
	return CheckTxResponse{Result: types.Result{}}
}

func (runtime *Runtime) PrepareProposal(req PrepareProposalRequest) (PrepareProposalResponse, error) {
	accepted := make([]types.Tx, 0, len(req.Txs))
	for _, tx := range req.Txs {
		if runtime.CheckTx(tx).Result.Code == 0 {
			accepted = append(accepted, append(types.Tx(nil), tx...))
		}
	}
	return PrepareProposalResponse{Txs: fairordering.SortTxsWithSalt(accepted, runtime.orderingSalt(req.Height))}, nil
}

func (runtime *Runtime) ProcessProposal(req ProcessProposalRequest) ProcessProposalResponse {
	if !fairordering.IsOrderedWithSalt(req.Block.Txs, runtime.orderingSalt(req.Block.Header.Height)) {
		return ProcessProposalResponse{Accepted: false, Reason: "transaction ordering mismatch"}
	}
	ctx := runtime.newContext(req.Block.Header.Height, req.Block.Header)
	if runtime.ante != nil {
		if err := runtime.ante.CheckBlock(ctx, req.Block.Txs); err != nil {
			return ProcessProposalResponse{Accepted: false, Reason: err.Error()}
		}
	}
	for _, tx := range req.Block.Txs {
		payload := TxPayload(tx)
		module, err := runtime.router.RouteTx(ctx, payload, runtime.modules)
		if err != nil {
			return ProcessProposalResponse{Accepted: false, Reason: "invalid transaction"}
		}
		if err := runtime.checkEstimatedGas(ctx, tx, payload, module); err != nil {
			return ProcessProposalResponse{Accepted: false, Reason: err.Error()}
		}
	}
	return ProcessProposalResponse{Accepted: true}
}

func (runtime *Runtime) FinalizeBlock(req FinalizeBlockRequest) (FinalizeBlockResponse, error) {
	return runtime.finalizeBlockWithStore(req, runtime.store, true)
}

func (runtime *Runtime) FinalizeBlockStaged(req FinalizeBlockRequest) (FinalizeBlockResponse, []kvbatch.KVWrite, error) {
	if runtime.store == nil {
		response, err := runtime.finalizeBlockWithStore(req, nil, true)
		return response, nil, err
	}
	staged := NewStagedStore(runtime.store)
	response, err := runtime.finalizeBlockWithStore(req, staged, false)
	if err != nil {
		return FinalizeBlockResponse{}, nil, err
	}
	return response, staged.Writes(), nil
}

func (runtime *Runtime) CommitStagedBlock(height types.Height, appHash types.Hash) {
	runtime.height = height
	runtime.appHash = appHash
}

func (runtime *Runtime) finalizeBlockWithStore(req FinalizeBlockRequest, executionStore StateStore, updateRuntime bool) (FinalizeBlockResponse, error) {
	proposalResponse := runtime.ProcessProposal(ProcessProposalRequest{Block: req.Block})
	if !proposalResponse.Accepted {
		return FinalizeBlockResponse{}, ErrProposalRejected
	}

	ctx := runtime.newContext(req.Block.Header.Height, req.Block.Header)
	ctx.Store = executionStore

	for _, module := range runtime.modules {
		if err := module.BeginBlock(ctx, req.Block.Header); err != nil {
			return FinalizeBlockResponse{}, err
		}
	}

	results := make([]types.Result, 0, len(req.Block.Txs))
	for _, tx := range req.Block.Txs {
		txCtx := ctx
		if runtime.ante != nil {
			if err := runtime.ante.BeforeTx(ctx, tx); err != nil {
				return FinalizeBlockResponse{}, err
			}
			txCtx = ctx.WithGasMeter(NewGasMeter(runtime.ante.GasLimit(tx)))
		}
		payload := TxPayload(tx)
		module, err := runtime.router.RouteTx(txCtx, payload, runtime.modules)
		if err != nil {
			return FinalizeBlockResponse{}, err
		}
		if err := runtime.checkEstimatedGas(txCtx, tx, payload, module); err != nil {
			return FinalizeBlockResponse{}, err
		}
		result := module.DeliverTx(txCtx, payload)
		if result.Code != 0 {
			return FinalizeBlockResponse{}, errors.New(result.Log)
		}
		if runtime.ante != nil {
			if err := runtime.ante.AfterTx(txCtx, tx); err != nil {
				return FinalizeBlockResponse{}, err
			}
			gasUsed := txCtx.GasUsed()
			if gasUsed == 0 {
				gasUsed = runtime.ante.GasUsed(tx)
			}
			result.GasUsed = gasUsed
			result.FeePaid = runtime.ante.FeePaid(tx)
			if len(result.Data) == 0 {
				result.Data = []byte(fmt.Sprintf("gas_used=%d fee_paid=%d", result.GasUsed, result.FeePaid))
			}
		}
		results = append(results, result)
	}

	for _, module := range runtime.modules {
		if err := module.EndBlock(ctx); err != nil {
			return FinalizeBlockResponse{}, err
		}
	}

	appHash := runtime.computeAppHashWithStore(executionStore)
	if updateRuntime {
		runtime.height = req.Block.Header.Height
		runtime.appHash = appHash
	}
	return FinalizeBlockResponse{
		Results:          results,
		AppHash:          appHash,
		ValidatorUpdates: runtime.collectValidatorUpdates(ctx),
	}, nil
}

func (runtime *Runtime) checkEstimatedGas(ctx Context, tx types.Tx, payload types.Tx, module Module) error {
	if runtime.ante == nil {
		return nil
	}
	estimator, ok := module.(GasEstimator)
	if !ok {
		return nil
	}
	required, err := estimator.EstimateGas(ctx, payload)
	if err != nil {
		return err
	}
	limit := runtime.ante.GasLimit(tx)
	if limit > 0 && required > limit {
		return ErrOutOfGas
	}
	return nil
}

func (runtime *Runtime) Commit() (CommitResponse, error) {
	return CommitResponse{Height: runtime.height, AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) Restore(height types.Height, appHash types.Hash) {
	runtime.height = height
	runtime.appHash = appHash
	runtime.bindStore()
}

func (runtime *Runtime) bindStore() {
	_ = runtime.bindStoreWithContext(runtime.newContext(runtime.height, types.Header{}))
}

func (runtime *Runtime) bindStoreWithContext(ctx Context) error {
	if ctx.Store == nil {
		return nil
	}
	for _, module := range runtime.modules {
		binder, ok := module.(StoreBinder)
		if !ok {
			continue
		}
		if err := binder.BindStore(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (runtime *Runtime) Query(req QueryRequest) QueryResponse {
	if len(req.Path) == 0 || req.Path[0] == "" {
		return QueryResponse{Code: 1, Log: "query module is required"}
	}
	ctx := runtime.newContext(runtime.height, types.Header{})
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

func (runtime *Runtime) newContext(height types.Height, header types.Header) Context {
	return Context{
		Ctx:     context.Background(),
		ChainID: runtime.chainID,
		Height:  height,
		Header:  header,
		Store:   runtime.store,
	}
}

func (runtime *Runtime) Modules() []Module {
	return append([]Module(nil), runtime.modules...)
}

func (runtime *Runtime) collectValidatorUpdates(ctx Context) []types.ValidatorUpdate {
	updates := make([]types.ValidatorUpdate, 0)
	for _, module := range runtime.modules {
		provider, ok := module.(ValidatorUpdateProvider)
		if !ok {
			continue
		}
		for _, update := range provider.ValidatorUpdates(ctx) {
			updates = append(updates, cloneValidatorUpdate(update))
		}
	}
	return updates
}

func (runtime *Runtime) orderingSalt(height types.Height) []byte {
	return fairordering.HeightSalt(runtime.chainID, height)
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

func (runtime *Runtime) computeAppHash() types.Hash {
	return runtime.computeAppHashWithStore(runtime.store)
}

func (runtime *Runtime) computeAppHashWithStore(stateStore StateStore) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(runtime.chainID))

	var heightBuffer [8]byte
	binary.BigEndian.PutUint64(heightBuffer[:], uint64(runtime.height))
	hasher.Write(heightBuffer[:])

	rootStore, ok := stateStore.(StateRootStore)
	if ok {
		for _, module := range runtime.modules {
			root, err := rootStore.Root(context.Background(), module.Name())
			if err != nil {
				continue
			}
			hasher.Write([]byte(module.Name()))
			hasher.Write(root[:])
		}
	}

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

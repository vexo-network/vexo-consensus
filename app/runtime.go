package app

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrEmptyChainID     = errors.New("chain id is required")
	ErrProposalRejected = errors.New("proposal rejected")
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
	return runtime
}

func (runtime *Runtime) WithAnte(ante AnteHandler) *Runtime {
	runtime.ante = ante
	return runtime
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

	ctx := Context{ChainID: runtime.chainID, Store: runtime.store}
	for _, module := range runtime.modules {
		if err := module.InitGenesis(ctx, req.Genesis); err != nil {
			return InitChainResponse{}, err
		}
	}
	runtime.appHash = runtime.computeAppHash()
	return InitChainResponse{AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) CheckTx(tx types.Tx) CheckTxResponse {
	ctx := Context{ChainID: runtime.chainID, Height: runtime.height, Store: runtime.store}
	if runtime.ante != nil {
		if err := runtime.ante.CheckTx(ctx, tx); err != nil {
			return CheckTxResponse{Result: types.Result{Code: 1, Log: err.Error()}}
		}
	}
	_, err := runtime.router.RouteTx(ctx, TxPayload(tx), runtime.modules)
	if err != nil {
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
	if runtime.ante != nil {
		ctx := Context{ChainID: runtime.chainID, Height: req.Block.Header.Height, Header: req.Block.Header, Store: runtime.store}
		if err := runtime.ante.CheckBlock(ctx, req.Block.Txs); err != nil {
			return ProcessProposalResponse{Accepted: false, Reason: err.Error()}
		}
	}
	ctx := Context{ChainID: runtime.chainID, Height: req.Block.Header.Height, Header: req.Block.Header, Store: runtime.store}
	for _, tx := range req.Block.Txs {
		if _, err := runtime.router.RouteTx(ctx, TxPayload(tx), runtime.modules); err != nil {
			return ProcessProposalResponse{Accepted: false, Reason: "invalid transaction"}
		}
	}
	return ProcessProposalResponse{Accepted: true}
}

func (runtime *Runtime) FinalizeBlock(req FinalizeBlockRequest) (FinalizeBlockResponse, error) {
	proposalResponse := runtime.ProcessProposal(ProcessProposalRequest{Block: req.Block})
	if !proposalResponse.Accepted {
		return FinalizeBlockResponse{}, ErrProposalRejected
	}

	ctx := Context{
		ChainID: runtime.chainID,
		Height:  req.Block.Header.Height,
		Header:  req.Block.Header,
		Store:   runtime.store,
	}

	for _, module := range runtime.modules {
		if err := module.BeginBlock(ctx, req.Block.Header); err != nil {
			return FinalizeBlockResponse{}, err
		}
	}

	results := make([]types.Result, 0, len(req.Block.Txs))
	for _, tx := range req.Block.Txs {
		if runtime.ante != nil {
			if err := runtime.ante.BeforeTx(ctx, tx); err != nil {
				return FinalizeBlockResponse{}, err
			}
		}
		payload := TxPayload(tx)
		module, err := runtime.router.RouteTx(ctx, payload, runtime.modules)
		if err != nil {
			return FinalizeBlockResponse{}, err
		}
		result := module.DeliverTx(ctx, payload)
		if result.Code != 0 {
			return FinalizeBlockResponse{}, errors.New(result.Log)
		}
		if runtime.ante != nil {
			if err := runtime.ante.AfterTx(ctx, tx); err != nil {
				return FinalizeBlockResponse{}, err
			}
			if len(result.Data) == 0 {
				result.Data = []byte(fmt.Sprintf("gas_used=%d fee_paid=%d", runtime.ante.GasUsed(tx), runtime.ante.FeePaid(tx)))
			}
		}
		results = append(results, result)
	}

	for _, module := range runtime.modules {
		if err := module.EndBlock(ctx); err != nil {
			return FinalizeBlockResponse{}, err
		}
	}

	runtime.height = req.Block.Header.Height
	runtime.appHash = runtime.computeAppHash()
	return FinalizeBlockResponse{
		Results:          results,
		AppHash:          runtime.appHash,
		ValidatorUpdates: runtime.collectValidatorUpdates(ctx),
	}, nil
}

func (runtime *Runtime) Commit() (CommitResponse, error) {
	return CommitResponse{Height: runtime.height, AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) Restore(height types.Height, appHash types.Hash) {
	runtime.height = height
	runtime.appHash = appHash
}

func (runtime *Runtime) Query(req QueryRequest) QueryResponse {
	if len(req.Path) == 0 || req.Path[0] == "" {
		return QueryResponse{Code: 1, Log: "query module is required"}
	}
	ctx := Context{ChainID: runtime.chainID, Height: runtime.height, Store: runtime.store}
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
	hasher := sha256.New()
	hasher.Write([]byte(runtime.chainID))

	var heightBuffer [8]byte
	binary.BigEndian.PutUint64(heightBuffer[:], uint64(runtime.height))
	hasher.Write(heightBuffer[:])

	rootStore, ok := runtime.store.(StateRootStore)
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

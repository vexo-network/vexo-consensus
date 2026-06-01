package app

import (
	"crypto/sha256"
	"errors"

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
	runtime.appHash = runtime.computeAppHash(nil)
	return InitChainResponse{AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) CheckTx(tx types.Tx) CheckTxResponse {
	ctx := Context{ChainID: runtime.chainID, Height: runtime.height, Store: runtime.store}
	_, err := runtime.router.RouteTx(ctx, tx, runtime.modules)
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
	return PrepareProposalResponse{Txs: accepted}, nil
}

func (runtime *Runtime) ProcessProposal(req ProcessProposalRequest) ProcessProposalResponse {
	for _, tx := range req.Block.Txs {
		if runtime.CheckTx(tx).Result.Code != 0 {
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
		module, err := runtime.router.RouteTx(ctx, tx, runtime.modules)
		if err != nil {
			return FinalizeBlockResponse{}, err
		}
		result := module.DeliverTx(ctx, tx)
		results = append(results, result)
		if result.Code != 0 {
			return FinalizeBlockResponse{}, errors.New(result.Log)
		}
	}

	for _, module := range runtime.modules {
		if err := module.EndBlock(ctx); err != nil {
			return FinalizeBlockResponse{}, err
		}
	}

	runtime.height = req.Block.Header.Height
	runtime.appHash = runtime.computeAppHash(req.Block.Txs)
	return FinalizeBlockResponse{Results: results, AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) Commit() (CommitResponse, error) {
	return CommitResponse{Height: runtime.height, AppHash: runtime.appHash}, nil
}

func (runtime *Runtime) Query(req QueryRequest) QueryResponse {
	return QueryResponse{Code: 1, Log: "query is not implemented"}
}

func (runtime *Runtime) Modules() []Module {
	return append([]Module(nil), runtime.modules...)
}

func (runtime *Runtime) computeAppHash(txs []types.Tx) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(runtime.chainID))

	var heightBuffer [8]byte
	height := uint64(runtime.height)
	for index := 7; index >= 0; index-- {
		heightBuffer[index] = byte(height)
		height >>= 8
	}
	hasher.Write(heightBuffer[:])
	for _, tx := range txs {
		hasher.Write(tx)
	}

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

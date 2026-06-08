package app

import (
	"context"

	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/types"
)

type Context struct {
	Ctx        context.Context
	ChainID    string
	EVMChainID uint64
	Height     types.Height
	Header     types.Header
	Block      types.Block
	TxResults  []types.Result
	Store      StateStore
	Gas        *GasMeter
}

func (ctx Context) GoContext() context.Context {
	if ctx.Ctx != nil {
		return ctx.Ctx
	}
	return context.Background()
}

func (ctx Context) WithGasMeter(meter *GasMeter) Context {
	ctx.Gas = meter
	return ctx
}

func (ctx Context) ConsumeGas(amount uint64) error {
	if ctx.Gas == nil {
		return nil
	}
	return ctx.Gas.Consume(amount)
}

func (ctx Context) GasUsed() uint64 {
	return ctx.Gas.Used()
}

func (ctx Context) GasLimit() uint64 {
	return ctx.Gas.Limit()
}

type StateStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
}

type StateRootStore interface {
	StateStore
	Root(ctx context.Context, namespace string) (types.Hash, error)
}

type GenesisState map[string][]byte

type Module interface {
	Name() string
	InitGenesis(ctx Context, genesis GenesisState) error
	BeginBlock(ctx Context, header types.Header) error
	DeliverTx(ctx Context, tx types.Tx) types.Result
	EndBlock(ctx Context) error
}

type ModuleCloner interface {
	CloneModule() Module
}

type StoreBinder interface {
	BindStore(ctx Context) error
}

type ValidatorUpdateProvider interface {
	ValidatorUpdates(ctx Context) []types.ValidatorUpdate
}

type QueryHandler interface {
	Query(ctx Context, req QueryRequest) QueryResponse
}

type GasEstimator interface {
	EstimateGas(ctx Context, tx types.Tx) (uint64, error)
}

type TxValidator interface {
	ValidateTx(ctx Context, tx types.Tx) error
}

type PruneHook interface {
	Prune(ctx Context, retainFrom types.Height) error
}

type TxEventEmitter interface {
	Events(ctx Context, tx types.Tx, result types.Result) []events.Event
}

type InitChainRequest struct {
	ChainID string
	Genesis GenesisState
}

type InitChainResponse struct {
	AppHash types.Hash
}

type CheckTxResponse struct {
	Result types.Result
}

type PrepareProposalRequest struct {
	Height types.Height
	Txs    []types.Tx
}

type PrepareProposalResponse struct {
	Txs []types.Tx
}

type ProcessProposalRequest struct {
	Block types.Block
}

type ProcessProposalResponse struct {
	Accepted bool
	Reason   string
}

type FinalizeBlockRequest struct {
	Block types.Block
}

type FinalizeBlockResponse struct {
	Results          []types.Result
	TxEvents         [][]events.Event
	AppHash          types.Hash
	ValidatorUpdates []types.ValidatorUpdate
}

type CommitResponse struct {
	Height  types.Height
	AppHash types.Hash
}

type QueryRequest struct {
	Path []string
	Data []byte
}

type QueryResponse struct {
	Code  uint32
	Value []byte
	Log   string
}

type Application interface {
	InitChain(req InitChainRequest) (InitChainResponse, error)
	CheckTx(tx types.Tx) CheckTxResponse
	PrepareProposal(req PrepareProposalRequest) (PrepareProposalResponse, error)
	ProcessProposal(req ProcessProposalRequest) ProcessProposalResponse
	FinalizeBlock(req FinalizeBlockRequest) (FinalizeBlockResponse, error)
	Commit() (CommitResponse, error)
	Query(req QueryRequest) QueryResponse
}

type ContextCheckTxApplication interface {
	CheckTxContext(ctx context.Context, tx types.Tx) CheckTxResponse
}

type ContextPrepareProposalApplication interface {
	PrepareProposalContext(ctx context.Context, req PrepareProposalRequest) (PrepareProposalResponse, error)
}

type ContextProcessProposalApplication interface {
	ProcessProposalContext(ctx context.Context, req ProcessProposalRequest) ProcessProposalResponse
}

type ContextFinalizeBlockApplication interface {
	FinalizeBlockContext(ctx context.Context, req FinalizeBlockRequest) (FinalizeBlockResponse, error)
}

type ContextQueryApplication interface {
	QueryContext(ctx context.Context, req QueryRequest) QueryResponse
}

type ContextApplication interface {
	Application
	ContextCheckTxApplication
	ContextPrepareProposalApplication
	ContextProcessProposalApplication
	ContextFinalizeBlockApplication
	ContextQueryApplication
}

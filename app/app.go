package app

import (
	"context"

	"github.com/vexo-network/vexo-consensus/events"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type Context struct {
	Ctx         context.Context
	ChainID     string
	EVMChainID  uint64
	BaseFee     uint64
	BlobBaseFee uint64
	Height      types.Height
	Header      types.Header
	Block       types.Block
	TxResults   []types.Result
	Store       StateStore
	Gas         *GasMeter
	// GenesisValidators is the genesis validator set. It is set only while
	// InitChain runs the modules' InitGenesis and is nil in every other context.
	GenesisValidators []types.ValidatorUpdate
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
	NamespaceRoots
}

// NamespaceRoots reports the state root of a namespace.
type NamespaceRoots interface {
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

// BlockExclusiveTx is implemented by a module whose transactions can be redundant with each other
// within one block: every one of them is valid against the committed state, but only the first to
// execute takes effect. ExclusiveTxKey names what a transaction is exclusive in; a block with two
// transactions of one module and one key is invalid, and a proposer never builds one.
//
// The runtime asks for the key only of a transaction that passed validation, in the order of the
// block, so the key may rely on that validation. The key is a pure function of the transaction.
type BlockExclusiveTx interface {
	ExclusiveTxKey(tx types.Tx) (string, error)
}

type PruneHook interface {
	Prune(ctx Context, retainFrom types.Height) error
}

type ReplayNamespaceProvider interface {
	ReplayNamespaces() []string
}

type TxEventEmitter interface {
	Events(ctx Context, tx types.Tx, result types.Result) []events.Event
}

type InitChainRequest struct {
	ChainID string
	Genesis GenesisState
	// Validators is the genesis validator set handed to every module's
	// InitGenesis through Context.GenesisValidators.
	Validators []types.ValidatorUpdate
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
	// FeeMarket reports the fees the block was priced at and the fees the next
	// block is priced at. Both are application state, so every node derives the
	// same values from the same blocks.
	FeeMarket FeeMarketReport
}

// FeeMarketReport is the fee market of one block: BaseFee and BlobBaseFee are
// what the block's transactions were priced against, NextBaseFee and
// NextBlobBaseFee what the block moved them to, and BlobGasUsed the blob gas
// the block consumed.
type FeeMarketReport struct {
	BaseFee         uint64
	NextBaseFee     uint64
	BlobBaseFee     uint64
	NextBlobBaseFee uint64
	BlobGasUsed     uint64
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

type ContextCheckTxsApplication interface {
	CheckTxsContext(ctx context.Context, txs []types.Tx) CheckTxResponse
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

type AtomicBlockApplication interface {
	Application
	FinalizeBlockStagedContext(ctx context.Context, req FinalizeBlockRequest) (FinalizeBlockResponse, []vexostore.KVWrite, error)
	CommitStagedBlock(height types.Height, appHash types.Hash)
	Modules() []Module
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

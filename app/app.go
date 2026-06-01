package app

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type Context struct {
	ChainID string
	Height  types.Height
	Header  types.Header
	Store   StateStore
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
	Results []types.Result
	AppHash types.Hash
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

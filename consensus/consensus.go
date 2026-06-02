package consensus

import (
	"context"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

type Phase string

const (
	PhasePropose Phase = "propose"
	PhaseVote    Phase = "vote"
	PhaseCommit  Phase = "commit"
)

type Proposal struct {
	Block     types.Block
	Round     types.Round
	Proposer  types.ValidatorID
	JustifyQC finality.QuorumCert
	Signature types.Signature
}

type Vote struct {
	Height      types.Height
	Round       types.Round
	BlockHash   types.Hash
	ValidatorID types.ValidatorID
	Signature   types.Signature
}

type EngineConfig struct {
	ChainID             string
	TargetBlockMillis   uint64
	CommitteeSize       uint64
	ByzantineFaultRatio string
}

type Engine interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SubmitTx(ctx context.Context, tx types.Tx) error
	Status(ctx context.Context) Status
}

type Status struct {
	ChainID          string
	Height           types.Height
	Round            types.Round
	Phase            Phase
	LastFinalized    types.Hash
	ValidatorSetHash types.Hash
	LastTimeoutCert  finality.TimeoutCert
}

type Reactor interface {
	OnProposal(ctx context.Context, proposal Proposal) error
	OnVote(ctx context.Context, vote Vote) error
	OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error)
}

type BlockExecutor interface {
	Execute(ctx context.Context, application app.Application, block types.Block) (app.FinalizeBlockResponse, error)
}

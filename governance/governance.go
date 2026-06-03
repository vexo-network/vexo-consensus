package governance

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type ParameterChange struct {
	Module string
	Key    string
	Value  []byte
}

type Proposal struct {
	ID          uint64
	Title       string
	Description string
	Changes     []ParameterChange
	Submitter   types.Address
}

type VoteOption string

const (
	VoteYes     VoteOption = "yes"
	VoteNo      VoteOption = "no"
	VoteAbstain VoteOption = "abstain"
	VoteVeto    VoteOption = "veto"
)

type Keeper interface {
	SubmitProposal(ctx context.Context, proposal Proposal) (uint64, error)
	Vote(ctx context.Context, proposalID uint64, voter types.Address, option VoteOption) error
	Execute(ctx context.Context, proposalID uint64) error
}

type OperationalKeeper interface {
	Keeper
	SetTime(now uint64)
	SetVotingPower(voter types.Address, power types.VotingPower)
	Proposal(proposalID uint64) (ProposalState, bool)
	AppliedChanges() []ParameterChange
	Tally(proposalID uint64) (TallyResult, bool)
}

type ContextOperationalKeeper interface {
	OperationalKeeper
	SetTimeContext(ctx context.Context, now uint64) error
	SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error
}

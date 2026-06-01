package governance

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingProposalTitle = errors.New("proposal title is required")
	ErrMissingSubmitter     = errors.New("proposal submitter is required")
	ErrNoProposalChanges    = errors.New("proposal changes are required")
	ErrProposalNotFound     = errors.New("proposal not found")
	ErrDuplicateVote        = errors.New("duplicate vote")
	ErrVotingPeriodOpen     = errors.New("voting period is still open")
	ErrTimelockActive       = errors.New("proposal timelock is active")
	ErrProposalRejected     = errors.New("proposal rejected")
	ErrProposalExecuted     = errors.New("proposal already executed")
)

type TallyPolicy struct {
	QuorumPower       types.VotingPower
	YesThresholdPower types.VotingPower
	VetoPower         types.VotingPower
	VotingPeriod      uint64
	Timelock          uint64
}

type VoteRecord struct {
	Voter  types.Address
	Option VoteOption
	Power  types.VotingPower
}

type ProposalState struct {
	Proposal        Proposal
	SubmittedAt     uint64
	VotingEndsAt    uint64
	ExecutableAt    uint64
	Executed        bool
	Votes           map[types.Address]VoteRecord
	ExecutedChanges []ParameterChange
}

type InMemoryKeeper struct {
	policy    TallyPolicy
	nextID    uint64
	proposals map[uint64]*ProposalState
	powers    map[types.Address]types.VotingPower
	now       uint64
	applied   []ParameterChange
}

func NewInMemoryKeeper(policy TallyPolicy, votingPower map[types.Address]types.VotingPower) *InMemoryKeeper {
	powers := make(map[types.Address]types.VotingPower, len(votingPower))
	for address, power := range votingPower {
		powers[address] = power
	}
	return &InMemoryKeeper{
		policy:    policy,
		nextID:    1,
		proposals: make(map[uint64]*ProposalState),
		powers:    powers,
	}
}

func (keeper *InMemoryKeeper) SetTime(now uint64) {
	keeper.now = now
}

func (keeper *InMemoryKeeper) SubmitProposal(ctx context.Context, proposal Proposal) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	if proposal.Title == "" {
		return 0, ErrMissingProposalTitle
	}
	if proposal.Submitter == "" {
		return 0, ErrMissingSubmitter
	}
	if len(proposal.Changes) == 0 {
		return 0, ErrNoProposalChanges
	}

	proposal.ID = keeper.nextID
	keeper.nextID++
	keeper.proposals[proposal.ID] = &ProposalState{
		Proposal:     proposal,
		SubmittedAt:  keeper.now,
		VotingEndsAt: keeper.now + keeper.policy.VotingPeriod,
		ExecutableAt: keeper.now + keeper.policy.VotingPeriod + keeper.policy.Timelock,
		Votes:        make(map[types.Address]VoteRecord),
	}
	return proposal.ID, nil
}

func (keeper *InMemoryKeeper) Vote(ctx context.Context, proposalID uint64, voter types.Address, option VoteOption) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	state, found := keeper.proposals[proposalID]
	if !found {
		return ErrProposalNotFound
	}
	if _, found := state.Votes[voter]; found {
		return ErrDuplicateVote
	}

	state.Votes[voter] = VoteRecord{
		Voter:  voter,
		Option: option,
		Power:  keeper.powers[voter],
	}
	return nil
}

func (keeper *InMemoryKeeper) Execute(ctx context.Context, proposalID uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	state, found := keeper.proposals[proposalID]
	if !found {
		return ErrProposalNotFound
	}
	if state.Executed {
		return ErrProposalExecuted
	}
	if keeper.now < state.VotingEndsAt {
		return ErrVotingPeriodOpen
	}
	if keeper.now < state.ExecutableAt {
		return ErrTimelockActive
	}
	if !keeper.passes(state) {
		return ErrProposalRejected
	}

	state.Executed = true
	state.ExecutedChanges = append([]ParameterChange(nil), state.Proposal.Changes...)
	keeper.applied = append(keeper.applied, state.Proposal.Changes...)
	return nil
}

func (keeper *InMemoryKeeper) Proposal(proposalID uint64) (ProposalState, bool) {
	state, found := keeper.proposals[proposalID]
	if !found {
		return ProposalState{}, false
	}
	return cloneProposalState(*state), true
}

func (keeper *InMemoryKeeper) AppliedChanges() []ParameterChange {
	return append([]ParameterChange(nil), keeper.applied...)
}

func (keeper *InMemoryKeeper) passes(state *ProposalState) bool {
	var yesPower types.VotingPower
	var noPower types.VotingPower
	var vetoPower types.VotingPower
	var totalPower types.VotingPower

	for _, vote := range state.Votes {
		totalPower += vote.Power
		switch vote.Option {
		case VoteYes:
			yesPower += vote.Power
		case VoteNo:
			noPower += vote.Power
		case VoteVeto:
			vetoPower += vote.Power
		case VoteAbstain:
		}
	}

	if keeper.policy.QuorumPower > 0 && totalPower < keeper.policy.QuorumPower {
		return false
	}
	if keeper.policy.VetoPower > 0 && vetoPower >= keeper.policy.VetoPower {
		return false
	}
	if keeper.policy.YesThresholdPower > 0 {
		return yesPower >= keeper.policy.YesThresholdPower
	}
	return yesPower > noPower
}

func cloneProposalState(state ProposalState) ProposalState {
	state.Proposal.Changes = append([]ParameterChange(nil), state.Proposal.Changes...)
	state.Votes = make(map[types.Address]VoteRecord, len(state.Votes))
	for voter, vote := range state.Votes {
		state.Votes[voter] = vote
	}
	state.ExecutedChanges = append([]ParameterChange(nil), state.ExecutedChanges...)
	return state
}

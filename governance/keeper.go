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
	ErrMissingVoter         = errors.New("proposal voter is required")
	ErrInvalidVoteOption    = errors.New("invalid vote option")
)

type TallyPolicy struct {
	QuorumPower       types.VotingPower
	YesThresholdPower types.VotingPower
	VetoPower         types.VotingPower
	VotingPeriod      uint64
	Timelock          uint64
	RequireDeposit    bool
	MinDeposit        string
	DepositDenom      string
	DepositEscrow     types.Address
	RejectedDeposits  types.Address
}

type VoteRecord struct {
	Voter  types.Address
	Option VoteOption
	Power  types.VotingPower
}

type TallyResult struct {
	Yes     types.VotingPower
	No      types.VotingPower
	Abstain types.VotingPower
	Veto    types.VotingPower
	Total   types.VotingPower
	Passed  bool
}

type ProposalState struct {
	Proposal        Proposal
	SubmittedAt     uint64
	VotingEndsAt    uint64
	ExecutableAt    uint64
	Executed        bool
	Rejected        bool
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

func (keeper *InMemoryKeeper) SetTimeContext(ctx context.Context, now uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.SetTime(now)
	return nil
}

func (keeper *InMemoryKeeper) SetVotingPower(voter types.Address, power types.VotingPower) {
	if voter == "" {
		return
	}
	keeper.powers[voter] = power
}

func (keeper *InMemoryKeeper) SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.SetVotingPower(voter, power)
	return nil
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
	if voter == "" {
		return ErrMissingVoter
	}
	if !isValidVoteOption(option) {
		return ErrInvalidVoteOption
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
	if state.Rejected {
		return ErrProposalRejected
	}
	if keeper.now < state.VotingEndsAt {
		return ErrVotingPeriodOpen
	}
	if keeper.now < state.ExecutableAt {
		return ErrTimelockActive
	}
	if !keeper.passes(state) {
		state.Rejected = true
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

func (keeper *InMemoryKeeper) ProposalContext(ctx context.Context, proposalID uint64) (ProposalState, bool, error) {
	select {
	case <-ctx.Done():
		return ProposalState{}, false, ctx.Err()
	default:
	}
	state, found := keeper.Proposal(proposalID)
	return state, found, nil
}

func (keeper *InMemoryKeeper) AppliedChanges() []ParameterChange {
	return append([]ParameterChange(nil), keeper.applied...)
}

func (keeper *InMemoryKeeper) AppliedChangesContext(ctx context.Context) ([]ParameterChange, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return keeper.AppliedChanges(), nil
}

func (keeper *InMemoryKeeper) Tally(proposalID uint64) (TallyResult, bool) {
	state, found := keeper.proposals[proposalID]
	if !found {
		return TallyResult{}, false
	}
	result := keeper.tally(state)
	result.Passed = keeper.passesTally(result)
	return result, true
}

func (keeper *InMemoryKeeper) TallyContext(ctx context.Context, proposalID uint64) (TallyResult, bool, error) {
	select {
	case <-ctx.Done():
		return TallyResult{}, false, ctx.Err()
	default:
	}
	tally, found := keeper.Tally(proposalID)
	return tally, found, nil
}

func (keeper *InMemoryKeeper) passes(state *ProposalState) bool {
	return keeper.passesTally(keeper.tally(state))
}

func (keeper *InMemoryKeeper) tally(state *ProposalState) TallyResult {
	var result TallyResult
	for _, vote := range state.Votes {
		result.Total += vote.Power
		switch vote.Option {
		case VoteYes:
			result.Yes += vote.Power
		case VoteNo:
			result.No += vote.Power
		case VoteVeto:
			result.Veto += vote.Power
		case VoteAbstain:
			result.Abstain += vote.Power
		}
	}
	return result
}

func (keeper *InMemoryKeeper) passesTally(result TallyResult) bool {
	if keeper.policy.QuorumPower > 0 && result.Total < keeper.policy.QuorumPower {
		return false
	}
	if keeper.policy.VetoPower > 0 && result.Veto >= keeper.policy.VetoPower {
		return false
	}
	if keeper.policy.YesThresholdPower > 0 {
		return result.Yes >= keeper.policy.YesThresholdPower
	}
	return result.Yes > result.No
}

func cloneProposalState(state ProposalState) ProposalState {
	state.Proposal.Changes = append([]ParameterChange(nil), state.Proposal.Changes...)
	votes := state.Votes
	state.Votes = make(map[types.Address]VoteRecord, len(state.Votes))
	for voter, vote := range votes {
		state.Votes[voter] = vote
	}
	state.ExecutedChanges = append([]ParameterChange(nil), state.ExecutedChanges...)
	return state
}

func isValidVoteOption(option VoteOption) bool {
	switch option {
	case VoteYes, VoteNo, VoteAbstain, VoteVeto:
		return true
	default:
		return false
	}
}

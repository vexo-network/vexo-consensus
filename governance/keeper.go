package governance

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingProposalTitle = errors.New("proposal title is required")
	ErrMissingSubmitter     = errors.New("proposal submitter is required")
	ErrNoProposalChanges    = errors.New("proposal changes are required")
	ErrProposalNotFound     = errors.New("proposal not found")
	ErrDuplicateVote        = errors.New("duplicate vote")
	ErrVotingPeriodOpen     = errors.New("voting period is still open")
	ErrVotingPeriodClosed   = errors.New("voting period has ended")
	ErrTimelockActive       = errors.New("proposal timelock is active")
	ErrProposalRejected     = errors.New("proposal rejected")
	ErrProposalExecuted     = errors.New("proposal already executed")
	ErrMissingVoter         = errors.New("proposal voter is required")
	ErrInvalidVoteOption    = errors.New("invalid vote option")
)

// BasisPoints is the denominator of the tally thresholds: 10000 basis points are all bonded power.
const BasisPoints uint64 = 10000

// TallyPolicy decides when a proposal passes. The thresholds are basis points of the bonded power
// snapshotted when the proposal was submitted, so they scale with the stake instead of being a fixed
// number that a growing or shrinking validator set makes meaningless.
//
// A proposal passes when
//   - the votes cast reach QuorumBPS of the snapshot (when set),
//   - veto votes stay below VetoBPS of the snapshot (when set),
//   - yes votes reach YesThresholdBPS of the snapshot (when set) and outnumber the no votes.
//
// A non-zero threshold always needs at least one unit of power, however small the snapshot is.
type TallyPolicy struct {
	QuorumBPS        uint64
	YesThresholdBPS  uint64
	VetoBPS          uint64
	VotingPeriod     uint64
	Timelock         uint64
	RequireDeposit   bool
	MinDeposit       string
	DepositDenom     string
	DepositEscrow    types.Address
	RejectedDeposits types.Address
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

// ProposalState is a proposal and everything decided about it. Votes are accepted from the block
// after SubmittedAt up to and including VotingEndsAt; the proposal can be executed after VotingEndsAt
// once the timelock has passed. SnapshotHeight and SnapshotPower fix the base of the tally when the
// proposal is submitted.
type ProposalState struct {
	Proposal        Proposal
	SubmittedAt     uint64
	VotingEndsAt    uint64
	ExecutableAt    uint64
	SnapshotHeight  uint64
	SnapshotPower   types.VotingPower
	Executed        bool
	Rejected        bool
	Votes           map[types.Address]VoteRecord
	ExecutedChanges []ParameterChange
}

type InMemoryKeeper struct {
	policy     TallyPolicy
	nextID     uint64
	proposals  map[uint64]*ProposalState
	powers     map[types.Address]types.VotingPower
	totalPower types.VotingPower
	now        uint64
	applied    []ParameterChange
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

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (keeper *InMemoryKeeper) SetTime(now uint64) {
	keeper.now = now
}

func (keeper *InMemoryKeeper) SetTimeContext(ctx context.Context, now uint64) error {
	if err := contextError(ctx); err != nil {
		return err
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
	if err := contextError(ctx); err != nil {
		return err
	}
	keeper.SetVotingPower(voter, power)
	return nil
}

func (keeper *InMemoryKeeper) SetTotalPower(power types.VotingPower) {
	keeper.totalPower = power
}

func (keeper *InMemoryKeeper) SetTotalPowerContext(ctx context.Context, power types.VotingPower) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	keeper.SetTotalPower(power)
	return nil
}

// snapshotPower is the bonded power a new proposal snapshots.
func (keeper *InMemoryKeeper) snapshotPower() types.VotingPower {
	if keeper.totalPower > 0 {
		return keeper.totalPower
	}
	total := uint64(0)
	for _, power := range keeper.powers {
		if total > math.MaxUint64-uint64(power) {
			return types.VotingPower(math.MaxUint64)
		}
		total += uint64(power)
	}
	return types.VotingPower(total)
}

func (keeper *InMemoryKeeper) SubmitProposal(ctx context.Context, proposal Proposal) (uint64, error) {
	if err := contextError(ctx); err != nil {
		return 0, err
	}
	if err := proposal.Validate(); err != nil {
		return 0, err
	}
	executableAt := keeper.now + keeper.policy.VotingPeriod + keeper.policy.Timelock
	plans, err := upgradePlans(proposal.Changes)
	if err != nil {
		return 0, err
	}
	for _, plan := range plans {
		if uint64(plan.Height) <= executableAt {
			return 0, fmt.Errorf("%w: plan %q height %d, earliest execution %d", ErrUpgradePlanNotInFuture, plan.Name, plan.Height, executableAt)
		}
	}

	proposal.ID = keeper.nextID
	keeper.nextID++
	keeper.proposals[proposal.ID] = &ProposalState{
		Proposal:       proposal,
		SubmittedAt:    keeper.now,
		VotingEndsAt:   keeper.now + keeper.policy.VotingPeriod,
		ExecutableAt:   executableAt,
		SnapshotHeight: keeper.now,
		SnapshotPower:  keeper.snapshotPower(),
		Votes:          make(map[types.Address]VoteRecord),
	}
	return proposal.ID, nil
}

// Vote records a vote with the power the keeper holds for the voter. Votes are refused once the
// voting period has ended and on proposals that were already decided, so a result cannot change after
// it was observed.
func (keeper *InMemoryKeeper) Vote(ctx context.Context, proposalID uint64, voter types.Address, option VoteOption) error {
	if err := contextError(ctx); err != nil {
		return err
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
	if state.Executed {
		return ErrProposalExecuted
	}
	if state.Rejected {
		return ErrProposalRejected
	}
	if keeper.now > state.VotingEndsAt {
		return ErrVotingPeriodClosed
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
	if err := contextError(ctx); err != nil {
		return err
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
	if keeper.now <= state.VotingEndsAt {
		return ErrVotingPeriodOpen
	}
	if keeper.now < state.ExecutableAt {
		return ErrTimelockActive
	}
	if !keeper.policy.passes(state.SnapshotPower, tallyVotes(state)) {
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
	if err := contextError(ctx); err != nil {
		return ProposalState{}, false, err
	}
	state, found := keeper.Proposal(proposalID)
	return state, found, nil
}

func (keeper *InMemoryKeeper) AppliedChanges() []ParameterChange {
	return append([]ParameterChange(nil), keeper.applied...)
}

func (keeper *InMemoryKeeper) AppliedChangesContext(ctx context.Context) ([]ParameterChange, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return keeper.AppliedChanges(), nil
}

func (keeper *InMemoryKeeper) Tally(proposalID uint64) (TallyResult, bool) {
	state, found := keeper.proposals[proposalID]
	if !found {
		return TallyResult{}, false
	}
	result := tallyVotes(state)
	result.Passed = keeper.policy.passes(state.SnapshotPower, result)
	return result, true
}

func (keeper *InMemoryKeeper) TallyContext(ctx context.Context, proposalID uint64) (TallyResult, bool, error) {
	if err := contextError(ctx); err != nil {
		return TallyResult{}, false, err
	}
	tally, found := keeper.Tally(proposalID)
	return tally, found, nil
}

func tallyVotes(state *ProposalState) TallyResult {
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

// passes applies the policy to a tally over a snapshot of bonded power.
func (policy TallyPolicy) passes(snapshot types.VotingPower, result TallyResult) bool {
	if policy.QuorumBPS > 0 && uint64(result.Total) < requiredPower(snapshot, policy.QuorumBPS) {
		return false
	}
	if policy.VetoBPS > 0 && uint64(result.Veto) >= requiredPower(snapshot, policy.VetoBPS) {
		return false
	}
	if policy.YesThresholdBPS > 0 && uint64(result.Yes) < requiredPower(snapshot, policy.YesThresholdBPS) {
		return false
	}
	return result.Yes > result.No
}

// requiredPower is ceil(snapshot * bps / 10000), computed without overflow and never below one.
func requiredPower(snapshot types.VotingPower, bps uint64) uint64 {
	if bps > BasisPoints {
		bps = BasisPoints
	}
	required := new(big.Int).Mul(new(big.Int).SetUint64(uint64(snapshot)), new(big.Int).SetUint64(bps))
	required.Add(required, new(big.Int).SetUint64(BasisPoints-1))
	required.Div(required, new(big.Int).SetUint64(BasisPoints))
	if required.Sign() == 0 {
		return 1
	}
	return required.Uint64()
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

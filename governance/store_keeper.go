package governance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

const governanceNamespace = "governance"

var (
	nextIDKey       = []byte("next_id")
	appliedCountKey = []byte("applied_count")

	ErrGovernanceStoreRequired       = errors.New("governance store is required")
	ErrAtomicGovernanceStoreRequired = errors.New("governance state requires a store that writes batches atomically")
	ErrCorruptGovernanceState        = errors.New("corrupt governance state")

	errUnschedulable = errors.New("upgrade plan cannot be scheduled")
)

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
}

// StoreKeeper is a keeper over a state store. It is built for one operation on the store that
// operation runs against - in a transaction that is the transaction's own layer - and keeps nothing
// between operations except what it writes to that store: the clock and the powers a caller sets
// live in the keeper value only.
type StoreKeeper struct {
	store      KVStore
	policy     TallyPolicy
	powers     map[types.Address]types.VotingPower
	totalPower types.VotingPower
	now        uint64
	lastErr    error
}

func NewStoreKeeper(store KVStore, policy TallyPolicy, votingPower map[types.Address]types.VotingPower) (*StoreKeeper, error) {
	return NewStoreKeeperContext(context.Background(), store, policy, votingPower)
}

// NewStoreKeeperContext opens a keeper over store. Opening never writes: a store without governance
// state simply has no proposals yet.
func NewStoreKeeperContext(ctx context.Context, store KVStore, policy TallyPolicy, votingPower map[types.Address]types.VotingPower) (*StoreKeeper, error) {
	if store == nil {
		return nil, ErrGovernanceStoreRequired
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	powers := make(map[types.Address]types.VotingPower, len(votingPower))
	for address, power := range votingPower {
		powers[address] = power
	}
	return &StoreKeeper{store: store, policy: policy, powers: powers}, nil
}

func (keeper *StoreKeeper) SetTime(now uint64) {
	keeper.now = now
}

func (keeper *StoreKeeper) SetTimeContext(ctx context.Context, now uint64) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	keeper.SetTime(now)
	return nil
}

func (keeper *StoreKeeper) SetVotingPower(voter types.Address, power types.VotingPower) {
	if voter == "" {
		return
	}
	if keeper.powers == nil {
		keeper.powers = make(map[types.Address]types.VotingPower)
	}
	keeper.powers[voter] = power
}

func (keeper *StoreKeeper) SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	keeper.SetVotingPower(voter, power)
	return nil
}

func (keeper *StoreKeeper) SetTotalPower(power types.VotingPower) {
	keeper.totalPower = power
}

func (keeper *StoreKeeper) SetTotalPowerContext(ctx context.Context, power types.VotingPower) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	keeper.SetTotalPower(power)
	return nil
}

func (keeper *StoreKeeper) SubmitProposal(ctx context.Context, proposal Proposal) (uint64, error) {
	nextID, err := keeper.loadNextID(ctx)
	if err != nil {
		return 0, err
	}
	memory := keeper.memory(nextID, nil)
	id, err := memory.SubmitProposal(ctx, proposal)
	if err != nil {
		return 0, err
	}
	encodedNextID, err := json.Marshal(memory.nextID)
	if err != nil {
		return 0, err
	}
	encodedProposal, err := json.Marshal(memory.proposals[id])
	if err != nil {
		return 0, err
	}
	return id, keeper.write(ctx, []kvbatch.KVWrite{
		{Namespace: governanceNamespace, Key: nextIDKey, Value: encodedNextID},
		{Namespace: governanceNamespace, Key: proposalKey(id), Value: encodedProposal},
	})
}

func (keeper *StoreKeeper) Vote(ctx context.Context, proposalID uint64, voter types.Address, option VoteOption) error {
	state, found, err := keeper.loadProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if !found {
		return ErrProposalNotFound
	}
	memory := keeper.memory(0, map[uint64]ProposalState{proposalID: state})
	if err := memory.Vote(ctx, proposalID, voter, option); err != nil {
		return err
	}
	return keeper.writeProposal(ctx, proposalID, *memory.proposals[proposalID], nil, nil)
}

// Execute applies a passed proposal. A proposal that fails its tally, or whose upgrade plan can no
// longer be scheduled, is marked rejected and the rejection is stored together with returning
// ErrProposalRejected. The upgrade plans a proposal carries are written in the same atomic batch as
// the proposal state and the changes it applied.
func (keeper *StoreKeeper) Execute(ctx context.Context, proposalID uint64) error {
	state, found, err := keeper.loadProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if !found {
		return ErrProposalNotFound
	}
	memory := keeper.memory(0, map[uint64]ProposalState{proposalID: state})
	if err := memory.Execute(ctx, proposalID); err != nil {
		if errors.Is(err, ErrProposalRejected) {
			if saveErr := keeper.writeProposal(ctx, proposalID, *memory.proposals[proposalID], nil, nil); saveErr != nil {
				return saveErr
			}
		}
		return err
	}
	plans, err := upgradePlans(state.Proposal.Changes)
	if err != nil {
		return err
	}
	if err := keeper.checkSchedulable(ctx, plans); err != nil {
		if !errors.Is(err, errUnschedulable) {
			return err
		}
		rejected := cloneProposalState(state)
		rejected.Rejected = true
		if err := keeper.writeProposal(ctx, proposalID, rejected, nil, nil); err != nil {
			return err
		}
		return fmt.Errorf("%w: %w", ErrProposalRejected, err)
	}
	return keeper.writeProposal(ctx, proposalID, *memory.proposals[proposalID], memory.applied, plans)
}

// checkSchedulable reports why plans cannot be scheduled by a proposal executing now: a plan for a
// height that has passed would silently never run, and two plans for one height would overwrite each
// other. Such a reason wraps errUnschedulable; any other error is a failure of the store.
func (keeper *StoreKeeper) checkSchedulable(ctx context.Context, plans []upgrade.Plan) error {
	heights := make(map[types.Height]struct{}, len(plans))
	for _, plan := range plans {
		if uint64(plan.Height) <= keeper.now {
			return fmt.Errorf("%w: %w: plan %q height %d, current %d", errUnschedulable, ErrUpgradePlanNotInFuture, plan.Name, plan.Height, keeper.now)
		}
		if _, duplicate := heights[plan.Height]; duplicate {
			return fmt.Errorf("%w: %w: height %d", errUnschedulable, ErrUpgradePlanConflict, plan.Height)
		}
		heights[plan.Height] = struct{}{}
		_, found, err := UpgradePlanAt(ctx, keeper.store, plan.Height)
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("%w: %w: height %d", errUnschedulable, ErrUpgradePlanConflict, plan.Height)
		}
	}
	return nil
}

func upgradePlanKey(height types.Height) []byte {
	return []byte(fmt.Sprintf("upgrade_plan/%020d", uint64(height)))
}

// UpgradePlanAt returns the upgrade plan a passed governance proposal scheduled for height. Plans live
// in the governance namespace of the state the proposal executed in, so a plan of a discarded
// transaction or block never exists.
func UpgradePlanAt(ctx context.Context, store KVStore, height types.Height) (upgrade.Plan, bool, error) {
	if store == nil {
		return upgrade.Plan{}, false, ErrGovernanceStoreRequired
	}
	encoded, err := store.Get(ctx, governanceNamespace, upgradePlanKey(height))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return upgrade.Plan{}, false, nil
	}
	if err != nil {
		return upgrade.Plan{}, false, err
	}
	var plan upgrade.Plan
	if err := json.Unmarshal(encoded, &plan); err != nil {
		return upgrade.Plan{}, false, err
	}
	return plan, true, nil
}

func (keeper *StoreKeeper) Proposal(proposalID uint64) (ProposalState, bool) {
	proposal, found, err := keeper.ProposalContext(context.Background(), proposalID)
	keeper.setLastError(err)
	return proposal, found
}

func (keeper *StoreKeeper) ProposalContext(ctx context.Context, proposalID uint64) (ProposalState, bool, error) {
	return keeper.loadProposal(ctx, proposalID)
}

func (keeper *StoreKeeper) AppliedChanges() []ParameterChange {
	changes, err := keeper.AppliedChangesContext(context.Background())
	keeper.setLastError(err)
	return changes
}

func (keeper *StoreKeeper) AppliedChangesContext(ctx context.Context) ([]ParameterChange, error) {
	count, err := keeper.loadAppliedCount(ctx)
	if err != nil {
		return nil, err
	}
	changes := make([]ParameterChange, 0, count)
	for index := uint64(0); index < count; index++ {
		encoded, err := keeper.store.Get(ctx, governanceNamespace, appliedKey(index))
		if err != nil {
			return nil, fmt.Errorf("governance applied change %d: %w", index, err)
		}
		var change ParameterChange
		if err := json.Unmarshal(encoded, &change); err != nil {
			return nil, fmt.Errorf("governance applied change %d: %w", index, err)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func (keeper *StoreKeeper) Tally(proposalID uint64) (TallyResult, bool) {
	tally, found, err := keeper.TallyContext(context.Background(), proposalID)
	keeper.setLastError(err)
	return tally, found
}

func (keeper *StoreKeeper) LastError() error {
	return keeper.lastErr
}

func (keeper *StoreKeeper) setLastError(err error) {
	keeper.lastErr = err
}

func (keeper *StoreKeeper) TallyContext(ctx context.Context, proposalID uint64) (TallyResult, bool, error) {
	state, found, err := keeper.loadProposal(ctx, proposalID)
	if err != nil || !found {
		return TallyResult{}, false, err
	}
	tally, found := keeper.memory(0, map[uint64]ProposalState{proposalID: state}).Tally(proposalID)
	return tally, found, nil
}

// memory is an in-memory keeper over the slice of governance state one operation reads: the next
// proposal id (when it submits) and the proposals it acts on. Its applied list starts empty, so after
// an execution it holds just the changes that execution applied.
func (keeper *StoreKeeper) memory(nextID uint64, proposals map[uint64]ProposalState) *InMemoryKeeper {
	memory := NewInMemoryKeeper(keeper.policy, keeper.powers)
	memory.totalPower = keeper.totalPower
	memory.nextID = nextID
	memory.now = keeper.now
	memory.proposals = make(map[uint64]*ProposalState, len(proposals))
	for id, state := range proposals {
		cloned := cloneProposalState(state)
		memory.proposals[id] = &cloned
	}
	return memory
}

func proposalKey(id uint64) []byte {
	return []byte(fmt.Sprintf("proposal/%020d", id))
}

func appliedKey(index uint64) []byte {
	return []byte(fmt.Sprintf("applied/%020d", index))
}

// loadNextID reads the id the next proposal gets. A store without governance state starts at 1.
func (keeper *StoreKeeper) loadNextID(ctx context.Context) (uint64, error) {
	nextID, found, err := keeper.loadCounter(ctx, nextIDKey)
	if err != nil {
		return 0, err
	}
	if !found {
		return 1, nil
	}
	if nextID == 0 {
		return 0, fmt.Errorf("%w: next proposal id is zero", ErrCorruptGovernanceState)
	}
	return nextID, nil
}

func (keeper *StoreKeeper) loadAppliedCount(ctx context.Context) (uint64, error) {
	count, _, err := keeper.loadCounter(ctx, appliedCountKey)
	return count, err
}

func (keeper *StoreKeeper) loadCounter(ctx context.Context, key []byte) (uint64, bool, error) {
	encoded, err := keeper.store.Get(ctx, governanceNamespace, key)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, false, contextError(ctx)
	}
	if err != nil {
		return 0, false, err
	}
	var value uint64
	if err := json.Unmarshal(encoded, &value); err != nil {
		return 0, false, fmt.Errorf("%w: %s: %v", ErrCorruptGovernanceState, key, err)
	}
	return value, true, nil
}

// loadProposal reads one proposal. Each proposal is its own record, so an operation on one proposal
// reads and writes that proposal only, whatever the number of proposals the chain holds.
func (keeper *StoreKeeper) loadProposal(ctx context.Context, proposalID uint64) (ProposalState, bool, error) {
	encoded, err := keeper.store.Get(ctx, governanceNamespace, proposalKey(proposalID))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return ProposalState{}, false, contextError(ctx)
	}
	if err != nil {
		return ProposalState{}, false, err
	}
	var state ProposalState
	if err := json.Unmarshal(encoded, &state); err != nil {
		return ProposalState{}, false, fmt.Errorf("%w: proposal %d: %v", ErrCorruptGovernanceState, proposalID, err)
	}
	if state.Proposal.ID != proposalID {
		return ProposalState{}, false, fmt.Errorf("%w: record of proposal %d names proposal %d", ErrCorruptGovernanceState, proposalID, state.Proposal.ID)
	}
	if state.Votes == nil {
		state.Votes = make(map[types.Address]VoteRecord)
	}
	return state, true, nil
}

// writeProposal writes a proposal together with the changes its execution applied and the upgrade
// plans it scheduled, as one batch, so neither a change nor a plan exists without the executed
// proposal that made it.
func (keeper *StoreKeeper) writeProposal(ctx context.Context, proposalID uint64, state ProposalState, applied []ParameterChange, plans []upgrade.Plan) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	writes := []kvbatch.KVWrite{{Namespace: governanceNamespace, Key: proposalKey(proposalID), Value: encoded}}
	if len(applied) > 0 {
		count, err := keeper.loadAppliedCount(ctx)
		if err != nil {
			return err
		}
		for _, change := range applied {
			encodedChange, err := json.Marshal(change)
			if err != nil {
				return err
			}
			writes = append(writes, kvbatch.KVWrite{Namespace: governanceNamespace, Key: appliedKey(count), Value: encodedChange})
			count++
		}
		encodedCount, err := json.Marshal(count)
		if err != nil {
			return err
		}
		writes = append(writes, kvbatch.KVWrite{Namespace: governanceNamespace, Key: appliedCountKey, Value: encodedCount})
	}
	for _, plan := range plans {
		encodedPlan, err := json.Marshal(plan)
		if err != nil {
			return err
		}
		writes = append(writes, kvbatch.KVWrite{Namespace: governanceNamespace, Key: upgradePlanKey(plan.Height), Value: encodedPlan})
	}
	return keeper.write(ctx, writes)
}

// write applies writes atomically. A single write needs no batch; several need a store that applies
// a batch atomically, so governance state is never left half written.
func (keeper *StoreKeeper) write(ctx context.Context, writes []kvbatch.KVWrite) error {
	if len(writes) == 1 {
		return keeper.store.Set(ctx, writes[0].Namespace, writes[0].Key, writes[0].Value)
	}
	batchStore, ok := keeper.store.(kvbatch.BatchKVStore)
	if !ok {
		return ErrAtomicGovernanceStoreRequired
	}
	return batchStore.SetBatch(ctx, writes)
}

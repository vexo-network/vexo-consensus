package governance

import (
	"context"
	"encoding/json"
	"errors"

	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

const governanceNamespace = "governance"

var ErrGovernanceStoreRequired = errors.New("governance store is required")

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
}

type atomicUpgradePlanStore interface {
	SetWithUpgradePlans(ctx context.Context, namespace string, key []byte, value []byte, plans []upgrade.Plan) error
}

type StoreKeeper struct {
	store  KVStore
	policy TallyPolicy
	powers map[types.Address]types.VotingPower
}

type storeDocument struct {
	NextID    uint64                              `json:"next_id"`
	Now       uint64                              `json:"now"`
	Proposals map[uint64]ProposalState            `json:"proposals"`
	Powers    map[types.Address]types.VotingPower `json:"powers"`
	Applied   []ParameterChange                   `json:"applied"`
}

func NewStoreKeeper(store KVStore, policy TallyPolicy, votingPower map[types.Address]types.VotingPower) (*StoreKeeper, error) {
	if store == nil {
		return nil, ErrGovernanceStoreRequired
	}
	powers := make(map[types.Address]types.VotingPower, len(votingPower))
	for address, power := range votingPower {
		powers[address] = power
	}
	keeper := &StoreKeeper{store: store, policy: policy, powers: powers}
	if _, err := keeper.load(context.Background()); err != nil {
		if !errors.Is(err, vexostore.ErrKeyNotFound) {
			return nil, err
		}
		if err := keeper.save(context.Background(), storeDocument{
			NextID:    1,
			Proposals: make(map[uint64]ProposalState),
			Powers:    powers,
		}); err != nil {
			return nil, err
		}
	}
	return keeper, nil
}

func (keeper *StoreKeeper) SetTime(now uint64) {
	_ = keeper.SetTimeContext(context.Background(), now)
}

func (keeper *StoreKeeper) SetTimeContext(ctx context.Context, now uint64) error {
	document, err := keeper.load(ctx)
	if err != nil {
		return err
	}
	document.Now = now
	return keeper.save(ctx, document)
}

func (keeper *StoreKeeper) SetVotingPower(voter types.Address, power types.VotingPower) {
	_ = keeper.SetVotingPowerContext(context.Background(), voter, power)
}

func (keeper *StoreKeeper) SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error {
	if voter == "" {
		return nil
	}
	document, err := keeper.load(ctx)
	if err != nil {
		return err
	}
	if document.Powers == nil {
		document.Powers = make(map[types.Address]types.VotingPower)
	}
	document.Powers[voter] = power
	if keeper.powers == nil {
		keeper.powers = make(map[types.Address]types.VotingPower)
	}
	keeper.powers[voter] = power
	return keeper.save(ctx, document)
}

func (keeper *StoreKeeper) SubmitProposal(ctx context.Context, proposal Proposal) (uint64, error) {
	document, err := keeper.load(ctx)
	if err != nil {
		return 0, err
	}
	memory := keeper.memory(document)
	id, err := memory.SubmitProposal(ctx, proposal)
	if err != nil {
		return 0, err
	}
	return id, keeper.save(ctx, documentFromMemory(memory))
}

func (keeper *StoreKeeper) Vote(ctx context.Context, proposalID uint64, voter types.Address, option VoteOption) error {
	document, err := keeper.load(ctx)
	if err != nil {
		return err
	}
	state, found := document.Proposals[proposalID]
	if !found {
		return ErrProposalNotFound
	}
	if voter == "" {
		return ErrMissingVoter
	}
	if !isValidVoteOption(option) {
		return ErrInvalidVoteOption
	}
	if state.Votes == nil {
		state.Votes = make(map[types.Address]VoteRecord)
	}
	if _, found := state.Votes[voter]; found {
		return ErrDuplicateVote
	}
	power := document.Powers[voter]
	if power == 0 {
		power = keeper.powers[voter]
	}
	state.Votes[voter] = VoteRecord{Voter: voter, Option: option, Power: power}
	document.Proposals[proposalID] = state
	return keeper.save(ctx, document)
}

func (keeper *StoreKeeper) Execute(ctx context.Context, proposalID uint64) error {
	document, err := keeper.load(ctx)
	if err != nil {
		return err
	}
	appliedBefore := len(document.Applied)
	memory := keeper.memory(document)
	if err := memory.Execute(ctx, proposalID); err != nil {
		return err
	}
	updated := documentFromMemory(memory)
	plans, err := keeper.upgradePlansFromChanges(updated.Applied[appliedBefore:])
	if err != nil {
		return err
	}
	if len(plans) > 0 {
		if atomicStore, ok := keeper.store.(atomicUpgradePlanStore); ok {
			encoded, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			return atomicStore.SetWithUpgradePlans(ctx, governanceNamespace, []byte("state"), encoded, plans)
		}
	}
	if err := keeper.save(ctx, updated); err != nil {
		return err
	}
	return keeper.persistUpgradePlans(ctx, plans)
}

func (keeper *StoreKeeper) upgradePlansFromChanges(changes []ParameterChange) ([]upgrade.Plan, error) {
	plans := make([]upgrade.Plan, 0)
	for _, change := range changes {
		if change.Module != "upgrade" || change.Key != "plan" {
			continue
		}
		var plan upgrade.Plan
		if err := json.Unmarshal(change.Value, &plan); err != nil {
			return nil, err
		}
		if err := upgrade.ValidatePlan(plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (keeper *StoreKeeper) persistUpgradePlans(ctx context.Context, plans []upgrade.Plan) error {
	if len(plans) == 0 {
		return nil
	}
	planStore, ok := keeper.store.(upgrade.PlanStore)
	if !ok {
		return nil
	}
	for _, plan := range plans {
		if err := planStore.SaveUpgradePlan(ctx, plan); err != nil {
			return err
		}
	}
	return nil
}

func (keeper *StoreKeeper) Proposal(proposalID uint64) (ProposalState, bool) {
	proposal, found, _ := keeper.ProposalContext(context.Background(), proposalID)
	return proposal, found
}

func (keeper *StoreKeeper) ProposalContext(ctx context.Context, proposalID uint64) (ProposalState, bool, error) {
	document, err := keeper.load(ctx)
	if err != nil {
		return ProposalState{}, false, err
	}
	state, found := document.Proposals[proposalID]
	if !found {
		return ProposalState{}, false, nil
	}
	return cloneProposalState(state), true, nil
}

func (keeper *StoreKeeper) AppliedChanges() []ParameterChange {
	changes, _ := keeper.AppliedChangesContext(context.Background())
	return changes
}

func (keeper *StoreKeeper) AppliedChangesContext(ctx context.Context) ([]ParameterChange, error) {
	document, err := keeper.load(ctx)
	if err != nil {
		return nil, err
	}
	return append([]ParameterChange(nil), document.Applied...), nil
}

func (keeper *StoreKeeper) Tally(proposalID uint64) (TallyResult, bool) {
	tally, found, _ := keeper.TallyContext(context.Background(), proposalID)
	return tally, found
}

func (keeper *StoreKeeper) TallyContext(ctx context.Context, proposalID uint64) (TallyResult, bool, error) {
	document, err := keeper.load(ctx)
	if err != nil {
		return TallyResult{}, false, err
	}
	memory := keeper.memory(document)
	tally, found := memory.Tally(proposalID)
	return tally, found, nil
}

func (keeper *StoreKeeper) memory(document storeDocument) *InMemoryKeeper {
	powers := make(map[types.Address]types.VotingPower, len(document.Powers)+len(keeper.powers))
	for address, power := range document.Powers {
		powers[address] = power
	}
	for address, power := range keeper.powers {
		powers[address] = power
	}
	memory := NewInMemoryKeeper(keeper.policy, powers)
	memory.nextID = document.NextID
	memory.now = document.Now
	memory.applied = append([]ParameterChange(nil), document.Applied...)
	memory.proposals = make(map[uint64]*ProposalState, len(document.Proposals))
	for id, state := range document.Proposals {
		cloned := cloneProposalState(state)
		memory.proposals[id] = &cloned
	}
	return memory
}

func documentFromMemory(memory *InMemoryKeeper) storeDocument {
	proposals := make(map[uint64]ProposalState, len(memory.proposals))
	for id, state := range memory.proposals {
		proposals[id] = cloneProposalState(*state)
	}
	powers := make(map[types.Address]types.VotingPower, len(memory.powers))
	for address, power := range memory.powers {
		powers[address] = power
	}
	return storeDocument{
		NextID:    memory.nextID,
		Now:       memory.now,
		Proposals: proposals,
		Powers:    powers,
		Applied:   append([]ParameterChange(nil), memory.applied...),
	}
}

func (keeper *StoreKeeper) load(ctx context.Context) (storeDocument, error) {
	encoded, err := keeper.store.Get(ctx, governanceNamespace, []byte("state"))
	if err != nil {
		return storeDocument{}, err
	}
	var document storeDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		return storeDocument{}, err
	}
	if document.NextID == 0 {
		document.NextID = 1
	}
	if document.Proposals == nil {
		document.Proposals = make(map[uint64]ProposalState)
	}
	if document.Powers == nil {
		document.Powers = make(map[types.Address]types.VotingPower)
	}
	return document, nil
}

func (keeper *StoreKeeper) save(ctx context.Context, document storeDocument) error {
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return keeper.store.Set(ctx, governanceNamespace, []byte("state"), encoded)
}

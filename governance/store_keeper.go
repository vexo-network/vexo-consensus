package governance

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

const governanceNamespace = "governance"

var ErrGovernanceStoreRequired = errors.New("governance store is required")

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
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
	document, err := keeper.load(context.Background())
	if err != nil {
		return
	}
	document.Now = now
	_ = keeper.save(context.Background(), document)
}

func (keeper *StoreKeeper) SetVotingPower(voter types.Address, power types.VotingPower) {
	if voter == "" {
		return
	}
	document, err := keeper.load(context.Background())
	if err != nil {
		return
	}
	if document.Powers == nil {
		document.Powers = make(map[types.Address]types.VotingPower)
	}
	document.Powers[voter] = power
	if keeper.powers == nil {
		keeper.powers = make(map[types.Address]types.VotingPower)
	}
	keeper.powers[voter] = power
	_ = keeper.save(context.Background(), document)
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
	memory := keeper.memory(document)
	if err := memory.Execute(ctx, proposalID); err != nil {
		return err
	}
	return keeper.save(ctx, documentFromMemory(memory))
}

func (keeper *StoreKeeper) Proposal(proposalID uint64) (ProposalState, bool) {
	document, err := keeper.load(context.Background())
	if err != nil {
		return ProposalState{}, false
	}
	state, found := document.Proposals[proposalID]
	if !found {
		return ProposalState{}, false
	}
	return cloneProposalState(state), true
}

func (keeper *StoreKeeper) AppliedChanges() []ParameterChange {
	document, err := keeper.load(context.Background())
	if err != nil {
		return nil
	}
	return append([]ParameterChange(nil), document.Applied...)
}

func (keeper *StoreKeeper) Tally(proposalID uint64) (TallyResult, bool) {
	document, err := keeper.load(context.Background())
	if err != nil {
		return TallyResult{}, false
	}
	memory := keeper.memory(document)
	return memory.Tally(proposalID)
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

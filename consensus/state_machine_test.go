package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineBuildsQuorumCert(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	block := types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("tx")}}
	blockHash := HashBlock(block)

	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	qc, err := machine.BuildQuorumCert(1, 0, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	if qc.VotingPower != 2 {
		t.Fatalf("expected voting power 2, got %d", qc.VotingPower)
	}
	if machine.Status(context.Background()).Phase != PhaseCommit {
		t.Fatalf("expected commit phase")
	}
}

func TestStateMachineWeightedQuorum(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 4},
		{ID: "b", VotingPower: 2},
		{ID: "c", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	blockHash := HashBlock(types.Block{Header: types.Header{Height: 1}})

	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum with 4/7 power, got %v", err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); err != nil {
		t.Fatalf("expected quorum with 6/7 power, got %v", err)
	}
}

func TestStateMachineRejectsUnknownValidatorVote(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	blockHash := HashBlock(types.Block{Header: types.Header{Height: 1}})
	err = machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "unknown"})
	if !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected unknown validator, got %v", err)
	}
}

func TestStateMachineRejectsUnknownProposalProposer(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}},
		Round:    0,
		Proposer: "unknown",
	})
	if !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected unknown proposer, got %v", err)
	}
}

func TestStateMachineRejectsWrongChainProposal(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "wrong-chain", Height: 1}},
		Round:    0,
		Proposer: "a",
	})
	if err == nil {
		t.Fatal("expected chain id mismatch")
	}
}

func TestStateMachineAllowsSameVoteReplay(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	blockHash := HashBlock(types.Block{Header: types.Header{Height: 1}})
	vote := Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "a"}
	if err := machine.OnVote(context.Background(), vote); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), vote); err != nil {
		t.Fatalf("same vote replay should be idempotent, got %v", err)
	}
}

func TestStateMachineRejectsConflictingVote(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		t.Fatal(err)
	}

	firstHash := HashBlock(types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("first")}})
	secondHash := HashBlock(types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("second")}})

	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: firstHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: secondHash, ValidatorID: "a"}); !errors.Is(err, ErrConflictingVote) {
		t.Fatalf("expected conflicting vote, got %v", err)
	}
	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
	}
}

type testValidatorSet struct {
	validators []validator.Validator
	byID       map[types.ValidatorID]validator.Validator
	totalPower types.VotingPower
}

func newTestValidatorSet(validators []validator.Validator) testValidatorSet {
	byID := make(map[types.ValidatorID]validator.Validator, len(validators))
	var totalPower types.VotingPower
	for _, validatorInfo := range validators {
		byID[validatorInfo.ID] = validatorInfo
		totalPower += validatorInfo.VotingPower
	}
	return testValidatorSet{
		validators: validators,
		byID:       byID,
		totalPower: totalPower,
	}
}

func (set testValidatorSet) Hash() types.Hash {
	return types.Hash{1}
}

func (set testValidatorSet) TotalVotingPower() types.VotingPower {
	return set.totalPower
}

func (set testValidatorSet) Get(id types.ValidatorID) (validator.Validator, bool) {
	validatorInfo, found := set.byID[id]
	return validatorInfo, found
}

func (set testValidatorSet) List() []validator.Validator {
	return set.validators
}

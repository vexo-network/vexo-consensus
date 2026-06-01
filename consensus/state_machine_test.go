package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
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

func TestStateMachineAcceptsValidProposal(t *testing.T) {
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
		Block: types.Block{Header: types.Header{
			ChainID:          "vexo-test",
			Height:           1,
			ValidatorSetHash: set.Hash(),
		}},
		Round:    0,
		Proposer: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if machine.Status(context.Background()).Phase != PhaseVote {
		t.Fatal("expected vote phase")
	}
}

func TestStateMachineRejectsInvalidProposalFields(t *testing.T) {
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

	cases := []struct {
		name     string
		proposal Proposal
		expected error
	}{
		{
			name: "missing height",
			proposal: Proposal{
				Block:    types.Block{Header: types.Header{ChainID: "vexo-test", ValidatorSetHash: set.Hash()}},
				Round:    0,
				Proposer: "a",
			},
			expected: ErrInvalidProposal,
		},
		{
			name: "validator set mismatch",
			proposal: Proposal{
				Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: types.Hash{9}}},
				Round:    0,
				Proposer: "a",
			},
			expected: ErrInvalidProposal,
		},
		{
			name: "qc height exceeds proposal height",
			proposal: Proposal{
				Block:     types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}},
				Round:     0,
				Proposer:  "a",
				JustifyQC: finality.QuorumCert{Height: 2},
			},
			expected: ErrInvalidProposal,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := machine.OnProposal(context.Background(), testCase.proposal)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}

func TestStateMachineRejectsStaleProposal(t *testing.T) {
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
	machine.StartRound(2, 3)

	err = machine.OnProposal(context.Background(), Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}},
		Round:    3,
		Proposer: "a",
	})
	if !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected stale proposal by height, got %v", err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 2, ValidatorSetHash: set.Hash()}},
		Round:    2,
		Proposer: "a",
	})
	if !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected stale proposal by round, got %v", err)
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

func TestStateMachineAdvancesRoundOnTimeoutQuorum(t *testing.T) {
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
	machine.StartRound(1, 0)

	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 0, ValidatorID: "a"}); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 0, ValidatorID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.Round != 0 {
		t.Fatalf("expected timeout cert round 0, got %d", timeoutCert.Round)
	}
	status := machine.Status(context.Background())
	if status.Round != 1 || status.Phase != PhasePropose {
		t.Fatalf("expected round 1 propose, got %+v", status)
	}
}

func TestStateMachineRejectsStaleTimeoutVote(t *testing.T) {
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
	machine.StartRound(2, 1)

	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 1, ValidatorID: "a"}); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("single stale vote should still wait for quorum, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 1, ValidatorID: "b"}); !errors.Is(err, ErrStaleTimeoutCert) {
		t.Fatalf("expected stale timeout cert, got %v", err)
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

package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/dataavailability"
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

	block := dataavailability.AttachCommitment(types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}, Txs: []types.Tx{[]byte("tx")}})
	blockHash := HashBlock(block)
	if err := machine.OnProposal(context.Background(), Proposal{Block: block, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}

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

func TestStateMachineStoresVoteQuorumCertInBlockTree(t *testing.T) {
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

	block := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	blockHash := HashBlock(block)
	if err := machine.OnProposal(context.Background(), Proposal{Block: block, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}

	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	node, found := machine.blockTree.Get(blockHash)
	if !found {
		t.Fatal("expected proposed block in tree")
	}
	if node.QuorumCert.Height != 0 {
		t.Fatal("expected no quorum cert before quorum")
	}

	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}
	node, found = machine.blockTree.Get(blockHash)
	if !found {
		t.Fatal("expected proposed block in tree")
	}
	if node.QuorumCert.BlockHash != blockHash || node.QuorumCert.VotingPower != 2 {
		t.Fatalf("expected stored quorum cert, got %+v", node.QuorumCert)
	}
}

func TestStateMachineCreateProposalUsesHighQC(t *testing.T) {
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

	parent := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	parentHash := HashBlock(parent)
	if err := machine.OnProposal(context.Background(), Proposal{Block: parent, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: parentHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: parentHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 0, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.JustifyQC.BlockHash != parentHash {
		t.Fatalf("expected high qc as justify qc, got %+v", proposal.JustifyQC)
	}
	if proposal.Block.Header.PreviousBlockHash != parentHash {
		t.Fatal("expected proposal parent to follow high qc")
	}
}

func TestStateMachineCreateProposalKeepsExplicitQC(t *testing.T) {
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

	explicitQC := finality.QuorumCert{Height: 7, Round: 1, BlockHash: types.Hash{7}}
	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 8}}, 0, "a", explicitQC)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.JustifyQC.BlockHash != explicitQC.BlockHash {
		t.Fatalf("expected explicit qc, got %+v", proposal.JustifyQC)
	}
	if proposal.Block.Header.PreviousBlockHash != explicitQC.BlockHash {
		t.Fatal("expected proposal parent to follow explicit qc")
	}
}

func TestStateMachineRejectsUnsafeForkBelowLockedQC(t *testing.T) {
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

	lockedBlock := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	lockedHash := HashBlock(lockedBlock)
	if err := machine.OnProposal(context.Background(), Proposal{Block: lockedBlock, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	fork := types.Block{Header: types.Header{
		ChainID:           "vexo-test",
		Height:            2,
		PreviousBlockHash: types.Hash{9},
		ValidatorSetHash:  set.Hash(),
	}}
	err = machine.OnProposal(context.Background(), Proposal{Block: fork, Proposer: "a"})
	if !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("expected unsafe proposal, got %v", err)
	}
}

func TestStateMachineRejectsUnsafeVoteBelowLockedQC(t *testing.T) {
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

	lockedBlock := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	lockedHash := HashBlock(lockedBlock)
	if err := machine.OnProposal(context.Background(), Proposal{Block: lockedBlock, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	fork := types.Block{Header: types.Header{
		ChainID:           "vexo-test",
		Height:            2,
		PreviousBlockHash: types.Hash{9},
		ValidatorSetHash:  set.Hash(),
	}}
	forkHash := HashBlock(fork)
	machine.blockTree.Insert(fork, forkHash, finality.QuorumCert{})
	machine.StartRound(2, 0)

	err = machine.OnVote(context.Background(), Vote{Height: 2, Round: 0, BlockHash: forkHash, ValidatorID: "a"})
	if !errors.Is(err, ErrUnsafeVote) {
		t.Fatalf("expected unsafe vote, got %v", err)
	}
}

func TestStateMachineAcceptsSafeVoteExtendingLockedQC(t *testing.T) {
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

	lockedBlock := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	lockedHash := HashBlock(lockedBlock)
	if err := machine.OnProposal(context.Background(), Proposal{Block: lockedBlock, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	child := types.Block{Header: types.Header{
		ChainID:           "vexo-test",
		Height:            2,
		PreviousBlockHash: lockedHash,
		ValidatorSetHash:  set.Hash(),
	}}
	childHash := HashBlock(child)
	if err := machine.OnProposal(context.Background(), Proposal{Block: child, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 2, Round: 0, BlockHash: childHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineAcceptsProposalExtendingLockedQC(t *testing.T) {
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

	lockedBlock := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	lockedHash := HashBlock(lockedBlock)
	if err := machine.OnProposal(context.Background(), Proposal{Block: lockedBlock, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	child := types.Block{Header: types.Header{
		ChainID:           "vexo-test",
		Height:            2,
		PreviousBlockHash: lockedHash,
		ValidatorSetHash:  set.Hash(),
	}}
	if err := machine.OnProposal(context.Background(), Proposal{Block: child, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineAcceptsProposalWithNewerQCThanLockedQC(t *testing.T) {
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

	lockedBlock := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	lockedHash := HashBlock(lockedBlock)
	if err := machine.OnProposal(context.Background(), Proposal{Block: lockedBlock, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: lockedHash, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}

	newerParent := types.Hash{8}
	proposal := Proposal{
		Block: types.Block{Header: types.Header{
			ChainID:           "vexo-test",
			Height:            3,
			PreviousBlockHash: newerParent,
			ValidatorSetHash:  set.Hash(),
		}},
		Proposer:  "a",
		JustifyQC: finality.QuorumCert{Height: 2, BlockHash: newerParent},
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
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

	block := types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}}
	blockHash := HashBlock(block)
	if err := machine.OnProposal(context.Background(), Proposal{Block: block, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}

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

func TestStateMachineRejectsUnknownVoteTarget(t *testing.T) {
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
	machine.StartRound(1, 0)

	err = machine.OnVote(context.Background(), Vote{Height: 1, Round: 0, BlockHash: types.Hash{1}, ValidatorID: "a"})
	if !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected invalid vote, got %v", err)
	}
}

func TestStateMachineRejectsInvalidVoteFields(t *testing.T) {
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
	machine.StartRound(1, 1)

	cases := []struct {
		name     string
		vote     Vote
		expected error
	}{
		{
			name:     "missing height",
			vote:     Vote{Round: 1, BlockHash: types.Hash{1}, ValidatorID: "a"},
			expected: ErrInvalidVote,
		},
		{
			name:     "missing block hash",
			vote:     Vote{Height: 1, Round: 1, ValidatorID: "a"},
			expected: ErrInvalidVote,
		},
		{
			name:     "future height",
			vote:     Vote{Height: 2, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "a"},
			expected: ErrInvalidVote,
		},
		{
			name:     "future round",
			vote:     Vote{Height: 1, Round: 2, BlockHash: types.Hash{1}, ValidatorID: "a"},
			expected: ErrInvalidVote,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := machine.OnVote(context.Background(), testCase.vote)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}

func TestStateMachineRejectsStaleVote(t *testing.T) {
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
	machine.StartRound(2, 2)

	err = machine.OnVote(context.Background(), Vote{Height: 1, Round: 2, BlockHash: types.Hash{1}, ValidatorID: "a"})
	if !errors.Is(err, ErrStaleVote) {
		t.Fatalf("expected stale vote by height, got %v", err)
	}
	err = machine.OnVote(context.Background(), Vote{Height: 2, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "a"})
	if !errors.Is(err, ErrStaleVote) {
		t.Fatalf("expected stale vote by round, got %v", err)
	}
}

func TestStateMachineObservesTimeoutCertHighQCForNextProposal(t *testing.T) {
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
	machine.StartRound(2, 0)

	highQC := finality.QuorumCert{Height: 1, Round: 2, BlockHash: types.Hash{1}}
	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 2, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 1, BlockHash: types.Hash{2}}}); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 2, Round: 0, ValidatorID: "b", HighQC: highQC})
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != highQC.BlockHash {
		t.Fatalf("expected timeout cert high qc, got %+v", timeoutCert.HighQC)
	}

	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 1, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.JustifyQC.BlockHash != highQC.BlockHash {
		t.Fatalf("expected timeout high qc to feed next proposal, got %+v", proposal.JustifyQC)
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

func TestStateMachineAutoCommitsThreeChainOnProposal(t *testing.T) {
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

	grandparent := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: set.Hash(),
	}}
	grandparentHash := HashBlock(grandparent)
	parent := types.Block{Header: types.Header{
		ChainID:           "vexo-test",
		Height:            2,
		PreviousBlockHash: grandparentHash,
		ValidatorSetHash:  set.Hash(),
	}}
	parentHash := HashBlock(parent)
	block := types.Block{Header: types.Header{
		ChainID:           "vexo-test",
		Height:            3,
		PreviousBlockHash: parentHash,
		ValidatorSetHash:  set.Hash(),
	}}

	if err := machine.OnProposal(context.Background(), Proposal{Block: grandparent, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), Proposal{
		Block:     parent,
		Proposer:  "a",
		JustifyQC: finality.QuorumCert{Height: 1, BlockHash: grandparentHash},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), Proposal{
		Block:     block,
		Proposer:  "a",
		JustifyQC: finality.QuorumCert{Height: 2, BlockHash: parentHash},
	}); err != nil {
		t.Fatal(err)
	}

	if machine.Status(context.Background()).LastFinalized != grandparentHash {
		t.Fatal("expected grandparent to finalize")
	}
	if len(machine.CommitDecisions()) != 1 {
		t.Fatal("expected one commit decision")
	}
}

func TestStateMachineRejectsProposalWithMismatchedJustifyQCParent(t *testing.T) {
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
			ChainID:           "vexo-test",
			Height:            2,
			PreviousBlockHash: types.Hash{1},
			ValidatorSetHash:  set.Hash(),
		}},
		Proposer:  "a",
		JustifyQC: finality.QuorumCert{Height: 1, BlockHash: types.Hash{9}},
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
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

	block := types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}}
	blockHash := HashBlock(block)
	if err := machine.OnProposal(context.Background(), Proposal{Block: block, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
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

	first := dataavailability.AttachCommitment(types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}, Txs: []types.Tx{[]byte("first")}})
	second := dataavailability.AttachCommitment(types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()}, Txs: []types.Tx{[]byte("second")}})
	firstHash := HashBlock(first)
	secondHash := HashBlock(second)
	if err := machine.OnProposal(context.Background(), Proposal{Block: first, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), Proposal{Block: second, Proposer: "a"}); err != nil {
		t.Fatal(err)
	}

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
	if status.LastTimeoutCert.Height != 1 || status.LastTimeoutCert.Round != 0 {
		t.Fatalf("expected last timeout cert in status, got %+v", status.LastTimeoutCert)
	}
}

func TestStateMachineJumpsToFutureTimeoutCertificateRound(t *testing.T) {
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

	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 3, ValidatorID: "a"}); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum for first future timeout vote, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 1, Round: 3, ValidatorID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.Round != 3 {
		t.Fatalf("expected timeout cert round 3, got %d", timeoutCert.Round)
	}
	status := machine.Status(context.Background())
	if status.Height != 1 || status.Round != 4 || status.Phase != PhasePropose {
		t.Fatalf("expected jump to round 4 propose, got %+v", status)
	}
}

func TestStateMachineJumpsToFutureHeightTimeoutCertificate(t *testing.T) {
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

	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 2, Round: 1, ValidatorID: "a"}); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum for first future height timeout vote, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(context.Background(), TimeoutVote{Height: 2, Round: 1, ValidatorID: "b"}); err != nil {
		t.Fatal(err)
	}
	status := machine.Status(context.Background())
	if status.Height != 2 || status.Round != 2 || status.Phase != PhasePropose {
		t.Fatalf("expected jump to height 2 round 2 propose, got %+v", status)
	}
}

func TestStateMachineRecordsConflictingTimeoutVoteEvidence(t *testing.T) {
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

	first := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}}
	second := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}}
	if _, err := machine.OnTimeoutVote(context.Background(), first); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(context.Background(), second); !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}

	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingTimeoutVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRecordsFutureRoundConflictingTimeoutEvidence(t *testing.T) {
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

	first := TimeoutVote{Height: 1, Round: 2, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}}
	second := TimeoutVote{Height: 1, Round: 2, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 1, BlockHash: types.Hash{2}}}
	if _, err := machine.OnTimeoutVote(context.Background(), first); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(context.Background(), second); !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}
	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one future timeout evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingTimeoutVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
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

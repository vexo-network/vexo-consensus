package consensus

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestNewConflictingVoteEvidence(t *testing.T) {
	first := testVote("a", 1, 2, types.Hash{1})
	second := testVote("a", 1, 2, types.Hash{2})

	evidence, err := NewConflictingVoteEvidence(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Type != slashing.EvidenceConflictingVote {
		t.Fatalf("expected conflicting vote evidence, got %s", evidence.Type)
	}
	if evidence.Validator != "a" || evidence.Height != 1 || evidence.Round != 2 {
		t.Fatalf("unexpected evidence metadata: %+v", evidence)
	}
	if err := VerifyConflictingVoteEvidence(evidence); err != nil {
		t.Fatal(err)
	}
}

func TestNewConflictingVoteEvidenceRejectsMismatchedPair(t *testing.T) {
	cases := []struct {
		name   string
		first  Vote
		second Vote
	}{
		{
			name:   "validator mismatch",
			first:  testVote("a", 1, 1, types.Hash{1}),
			second: testVote("b", 1, 1, types.Hash{2}),
		},
		{
			name:   "height mismatch",
			first:  testVote("a", 1, 1, types.Hash{1}),
			second: testVote("a", 2, 1, types.Hash{2}),
		},
		{
			name:   "round mismatch",
			first:  testVote("a", 1, 1, types.Hash{1}),
			second: testVote("a", 1, 2, types.Hash{2}),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewConflictingVoteEvidence(testCase.first, testCase.second)
			if !errors.Is(err, ErrVotePairMismatch) {
				t.Fatalf("expected pair mismatch, got %v", err)
			}
		})
	}
}

func TestNewConflictingVoteEvidenceRejectsSameBlock(t *testing.T) {
	blockHash := types.Hash{1}
	_, err := NewConflictingVoteEvidence(testVote("a", 1, 1, blockHash), testVote("a", 1, 1, blockHash))
	if !errors.Is(err, ErrVotesDoNotConflict) {
		t.Fatalf("expected no conflict, got %v", err)
	}
}

func TestVerifyConflictingVoteEvidenceRejectsTamperedMetadata(t *testing.T) {
	evidence, err := NewConflictingVoteEvidence(
		testVote("a", 1, 1, types.Hash{1}),
		testVote("a", 1, 1, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Validator = "b"

	if err := VerifyConflictingVoteEvidence(evidence); !errors.Is(err, ErrVotePairMismatch) {
		t.Fatalf("expected pair mismatch, got %v", err)
	}
}

func TestVerifyConflictingVoteEvidenceRejectsWrongType(t *testing.T) {
	err := VerifyConflictingVoteEvidence(slashing.Evidence{Type: slashing.EvidenceDoubleSign})
	if !errors.Is(err, slashing.ErrUnknownEvidenceType) {
		t.Fatalf("expected unknown evidence type, got %v", err)
	}
}

func TestVoteConflictFromPrevious(t *testing.T) {
	evidence, err := VoteConflictFromPrevious(types.Hash{1}, testVote("a", 1, 2, types.Hash{2}))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyConflictingVoteEvidence(evidence); err != nil {
		t.Fatal(err)
	}
}

func TestNewConflictingTimeoutVoteEvidence(t *testing.T) {
	first := testTimeoutVote("a", 2, 3, types.Hash{1})
	second := testTimeoutVote("a", 2, 3, types.Hash{2})

	evidence, err := NewConflictingTimeoutVoteEvidence(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Type != slashing.EvidenceConflictingTimeoutVote {
		t.Fatalf("expected conflicting timeout vote evidence, got %s", evidence.Type)
	}
	if evidence.Validator != "a" || evidence.Height != 2 || evidence.Round != 3 {
		t.Fatalf("unexpected evidence metadata: %+v", evidence)
	}
	if err := VerifyConflictingTimeoutVoteEvidence(evidence); err != nil {
		t.Fatal(err)
	}
}

func TestNewConflictingTimeoutVoteEvidenceRejectsMismatchedPair(t *testing.T) {
	cases := []struct {
		name   string
		first  TimeoutVote
		second TimeoutVote
	}{
		{
			name:   "validator mismatch",
			first:  testTimeoutVote("a", 1, 1, types.Hash{1}),
			second: testTimeoutVote("b", 1, 1, types.Hash{2}),
		},
		{
			name:   "height mismatch",
			first:  testTimeoutVote("a", 1, 1, types.Hash{1}),
			second: testTimeoutVote("a", 2, 1, types.Hash{2}),
		},
		{
			name:   "round mismatch",
			first:  testTimeoutVote("a", 1, 1, types.Hash{1}),
			second: testTimeoutVote("a", 1, 2, types.Hash{2}),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewConflictingTimeoutVoteEvidence(testCase.first, testCase.second)
			if !errors.Is(err, ErrVotePairMismatch) {
				t.Fatalf("expected pair mismatch, got %v", err)
			}
		})
	}
}

func TestNewConflictingTimeoutVoteEvidenceRejectsSameHighQC(t *testing.T) {
	first := testTimeoutVote("a", 1, 1, types.Hash{1})
	second := testTimeoutVote("a", 1, 1, types.Hash{1})

	_, err := NewConflictingTimeoutVoteEvidence(first, second)
	if !errors.Is(err, ErrTimeoutVotesDoNotConflict) {
		t.Fatalf("expected no timeout conflict, got %v", err)
	}
}

func TestVerifyConflictingTimeoutVoteEvidenceRejectsWrongType(t *testing.T) {
	err := VerifyConflictingTimeoutVoteEvidence(slashing.Evidence{Type: slashing.EvidenceConflictingVote})
	if !errors.Is(err, slashing.ErrUnknownEvidenceType) {
		t.Fatalf("expected unknown evidence type, got %v", err)
	}
}

func testVote(validatorID types.ValidatorID, height types.Height, round types.Round, blockHash types.Hash) Vote {
	return Vote{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		ValidatorID: validatorID,
	}
}

func testTimeoutVote(validatorID types.ValidatorID, height types.Height, round types.Round, highQCBlockHash types.Hash) TimeoutVote {
	return TimeoutVote{
		Height:      height,
		Round:       round,
		ValidatorID: validatorID,
		HighQC:      finality.QuorumCert{Height: height, Round: round, BlockHash: highQCBlockHash},
	}
}

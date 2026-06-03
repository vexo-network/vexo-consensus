package safety

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestCommitIndexAllowsIdempotentCommit(t *testing.T) {
	index := NewCommitIndex()
	height := types.Height(7)
	hash := types.Hash{1, 2, 3}

	if err := index.Record(height, hash); err != nil {
		t.Fatalf("record first commit: %v", err)
	}
	if err := index.Record(height, hash); err != nil {
		t.Fatalf("record idempotent commit: %v", err)
	}
}

func TestCommitIndexRejectsConflictingCommit(t *testing.T) {
	index := NewCommitIndex()
	height := types.Height(7)

	if err := index.Record(height, types.Hash{1}); err != nil {
		t.Fatalf("record first commit: %v", err)
	}
	err := index.Record(height, types.Hash{2})
	if !errors.Is(err, ErrConflictingCommit) {
		t.Fatalf("expected conflicting commit error, got %v", err)
	}
}

func TestCommitIndexZeroValueIsUsable(t *testing.T) {
	var index CommitIndex

	if err := index.Record(1, types.Hash{1}); err != nil {
		t.Fatalf("record with zero-value index: %v", err)
	}
}

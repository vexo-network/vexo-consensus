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

func TestCommitIndexGetReturnsRecordedHash(t *testing.T) {
	index := NewCommitIndex()
	if _, found := index.Get(3); found {
		t.Fatal("expected no commit at an unrecorded height")
	}
	if err := index.Record(3, types.Hash{3}); err != nil {
		t.Fatal(err)
	}
	hash, found := index.Get(3)
	if !found || hash != (types.Hash{3}) {
		t.Fatalf("expected the recorded hash, got found=%t hash=%x", found, hash)
	}
}

func TestCommitIndexPruneKeepsTheBoundaryAndBoundsMemory(t *testing.T) {
	index := NewCommitIndex()
	for height := types.Height(1); height <= 100; height++ {
		if err := index.Record(height, types.Hash{byte(height)}); err != nil {
			t.Fatal(err)
		}
	}

	index.Prune(90)
	if index.Len() != 11 {
		t.Fatalf("expected heights 90..100 to remain, got %d", index.Len())
	}
	if _, found := index.Get(89); found {
		t.Fatal("expected heights below the prune height to be forgotten")
	}
	if err := index.Record(90, types.Hash{0xFF}); !errors.Is(err, ErrConflictingCommit) {
		t.Fatalf("expected the boundary height to keep rejecting a conflicting commit, got %v", err)
	}
	if err := index.Record(100, types.Hash{100}); err != nil {
		t.Fatalf("expected an idempotent record to stay accepted, got %v", err)
	}

	index.Prune(10)
	if index.Len() != 11 {
		t.Fatalf("pruning below the retained range must change nothing, got %d", index.Len())
	}
}

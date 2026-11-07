package safety

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrConflictingCommit = errors.New("conflicting commit decision")

type CommitIndex struct {
	byHeight map[types.Height]types.Hash
}

func NewCommitIndex() CommitIndex {
	return CommitIndex{byHeight: make(map[types.Height]types.Hash)}
}

func (index *CommitIndex) Record(height types.Height, blockHash types.Hash) error {
	if index.byHeight == nil {
		index.byHeight = make(map[types.Height]types.Hash)
	}
	if existing, found := index.byHeight[height]; found && existing != blockHash {
		return ErrConflictingCommit
	}
	index.byHeight[height] = blockHash
	return nil
}

// Get returns the block recorded as committed at height.
func (index *CommitIndex) Get(height types.Height) (types.Hash, bool) {
	blockHash, found := index.byHeight[height]
	return blockHash, found
}

// Prune forgets every commit recorded strictly below height.
func (index *CommitIndex) Prune(height types.Height) {
	for recorded := range index.byHeight {
		if recorded < height {
			delete(index.byHeight, recorded)
		}
	}
}

// Len reports how many heights are currently tracked.
func (index *CommitIndex) Len() int {
	return len(index.byHeight)
}

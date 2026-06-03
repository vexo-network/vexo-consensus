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

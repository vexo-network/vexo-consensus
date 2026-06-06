package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrCommitRuleNotSatisfied = errors.New("commit rule not satisfied")

type CommitCandidate struct {
	BlockHeight       types.Height
	BlockHash         types.Hash
	ParentHeight      types.Height
	ParentHash        types.Hash
	GrandparentHeight types.Height
	GrandparentHash   types.Hash
	BlockQC           finality.QuorumCert
	ParentQC          finality.QuorumCert
}

type CommitDecision struct {
	CommittedHeight    types.Height
	CommittedBlockHash types.Hash
	CommitQC           finality.QuorumCert
}

type ThreeChainCommitRule struct{}

func (ThreeChainCommitRule) Decide(candidate CommitCandidate) (CommitDecision, error) {
	if candidate.BlockHash == (types.Hash{}) ||
		candidate.ParentHash == (types.Hash{}) ||
		candidate.GrandparentHash == (types.Hash{}) {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockQC.BlockHash != candidate.ParentHash {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.ParentQC.BlockHash != candidate.GrandparentHash {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockHeight == 0 ||
		candidate.ParentHeight == 0 ||
		candidate.GrandparentHeight == 0 ||
		candidate.BlockQC.Height == 0 ||
		candidate.ParentQC.Height == 0 {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockHeight != candidate.ParentHeight+1 ||
		candidate.ParentHeight != candidate.GrandparentHeight+1 ||
		candidate.BlockQC.Height != candidate.ParentHeight ||
		candidate.ParentQC.Height != candidate.GrandparentHeight {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	return CommitDecision{
		CommittedHeight:    candidate.GrandparentHeight,
		CommittedBlockHash: candidate.GrandparentHash,
		CommitQC:           candidate.ParentQC,
	}, nil
}

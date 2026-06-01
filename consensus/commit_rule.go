package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrCommitRuleNotSatisfied = errors.New("commit rule not satisfied")

type CommitCandidate struct {
	BlockHash       types.Hash
	ParentHash      types.Hash
	GrandparentHash types.Hash
	BlockQC         finality.QuorumCert
	ParentQC        finality.QuorumCert
}

type CommitDecision struct {
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
	if candidate.BlockQC.Height == 0 || candidate.ParentQC.Height == 0 {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockQC.Height != candidate.ParentQC.Height+1 {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	return CommitDecision{
		CommittedBlockHash: candidate.GrandparentHash,
		CommitQC:           candidate.ParentQC,
	}, nil
}

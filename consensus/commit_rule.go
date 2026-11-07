package consensus

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrCommitRuleNotSatisfied = errors.New("commit rule not satisfied")

// CommitCandidate describes a block B together with its child C. BlockQC is QC(B) as
// carried by C, and ChildQC is QC(C).
type CommitCandidate struct {
	BlockHeight     types.Height
	BlockHash       types.Hash
	BlockQC         finality.QuorumCert
	ChildHeight     types.Height
	ChildHash       types.Hash
	ChildParentHash types.Hash
	ChildQC         finality.QuorumCert
}

type CommitDecision struct {
	CommittedHeight    types.Height
	CommittedBlockHash types.Hash
	CommitQC           finality.QuorumCert
}

// TwoChainCommitRule is the consensus finality rule: a 2-chain commit over global,
// monotonically increasing views.
//
// Views. Vote.Round, Proposal.Round and QuorumCert.Round all carry the view in which a
// block was proposed. Views are global: they are never reset when the height advances.
// A proposal in view v extends a QC of some view q < v, and v is either exactly q+1 or
// backed by a verified TimeoutCert proving that a quorum gave up on view v-1.
//
// Commit rule. Block B is final when all of the following hold:
//
//  1. QC(B) is a verified quorum certificate for B, formed in view b.
//  2. Block C is the child of B: C.PreviousBlockHash == hash(B) and
//     C.Height == B.Height+1, and C carries QC(B) as its justify certificate.
//  3. QC(C) is a verified quorum certificate for C, formed in view c == b+1.
//
// Every ancestor of a final block is final.
//
// QC verification. A QC counts only when its signers are unique and known in the
// validator set of the certified height, their voting power reaches the two-thirds
// quorum, and the aggregate signature verifies over the vote sign bytes in the
// consensus vote domain. A certificate that fails verification never reaches the block
// tree, the lock, or the commit rule.
//
// Safety. A validator votes only in a view above its last voted view, and only for a
// proposal whose justify QC is not older (by view) than its locked QC. When B and C
// have QCs in consecutive views, a quorum voted for C in view b+1 after locking on
// QC(B); by quorum intersection every later QC extends B.
//
// Decide checks only the structure of the pair; callers pass certificates that were
// already verified.
type TwoChainCommitRule struct{}

func (TwoChainCommitRule) Decide(candidate CommitCandidate) (CommitDecision, error) {
	if candidate.BlockHash == (types.Hash{}) || candidate.ChildHash == (types.Hash{}) {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockHeight == 0 || candidate.ChildHeight == 0 ||
		candidate.BlockQC.Height == 0 || candidate.ChildQC.Height == 0 {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.ChildParentHash != candidate.BlockHash {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockQC.BlockHash != candidate.BlockHash ||
		candidate.BlockQC.Height != candidate.BlockHeight {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.ChildQC.BlockHash != candidate.ChildHash ||
		candidate.ChildQC.Height != candidate.ChildHeight {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.ChildHeight != candidate.BlockHeight+1 {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	if candidate.BlockQC.Round == ^types.Round(0) || candidate.ChildQC.Round != candidate.BlockQC.Round+1 {
		return CommitDecision{}, ErrCommitRuleNotSatisfied
	}
	return CommitDecision{
		CommittedHeight:    candidate.BlockHeight,
		CommittedBlockHash: candidate.BlockHash,
		CommitQC:           candidate.BlockQC,
	}, nil
}

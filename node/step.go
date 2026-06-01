package node

import (
	"context"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/types"
)

type ConsensusStepResult struct {
	Committed bool
	Commit    CommitReadyResult
	Proposed  bool
	Proposal  consensus.Proposal
	BlockHash types.Hash
}

func (node *Node) StepConsensus(ctx context.Context, maxBytes int64) (ConsensusStepResult, error) {
	commit, committed, err := node.CommitReadyBlock(ctx)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	if committed {
		return ConsensusStepResult{
			Committed: true,
			Commit:    commit,
		}, nil
	}

	proposal, blockHash, proposed, err := node.TickConsensus(ctx, maxBytes)
	if err != nil {
		return ConsensusStepResult{}, err
	}
	return ConsensusStepResult{
		Proposed:  proposed,
		Proposal:  proposal,
		BlockHash: blockHash,
	}, nil
}

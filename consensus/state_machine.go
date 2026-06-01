package consensus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrUnknownValidator = errors.New("unknown validator")
	ErrConflictingVote  = errors.New("conflicting vote")
	ErrNoQuorum         = errors.New("not enough voting power for quorum")
	ErrInvalidProposal  = errors.New("invalid proposal")
	ErrStaleProposal    = errors.New("stale proposal")
	ErrInvalidVote      = errors.New("invalid vote")
	ErrStaleVote        = errors.New("stale vote")
)

type StateMachineConfig struct {
	ChainID      string
	ValidatorSet validator.Set
	HashBlock    func(types.Block) types.Hash
}

type StateMachine struct {
	chainID      string
	validatorSet validator.Set
	hashBlock    func(types.Block) types.Hash
	status       Status
	votes        map[types.Height]map[types.Round]map[types.Hash]map[types.ValidatorID]Vote
	votedBlocks  map[types.Height]map[types.Round]map[types.ValidatorID]types.Hash
	evidence     []slashing.Evidence
	timeouts     *TimeoutCollector
	pacemaker    *Pacemaker
	blockTree    *BlockTree
	commitRule   ThreeChainCommitRule
	committed    []CommitDecision
	committedSet map[types.Hash]struct{}
}

func NewStateMachine(config StateMachineConfig) (*StateMachine, error) {
	if config.ChainID == "" {
		return nil, errors.New("chain id is required")
	}
	if config.ValidatorSet == nil {
		return nil, errors.New("validator set is required")
	}
	hashBlock := config.HashBlock
	if hashBlock == nil {
		hashBlock = HashBlock
	}

	return &StateMachine{
		chainID:      config.ChainID,
		validatorSet: config.ValidatorSet,
		hashBlock:    hashBlock,
		status: Status{
			ChainID:          config.ChainID,
			Phase:            PhasePropose,
			ValidatorSetHash: config.ValidatorSet.Hash(),
		},
		votes:        make(map[types.Height]map[types.Round]map[types.Hash]map[types.ValidatorID]Vote),
		votedBlocks:  make(map[types.Height]map[types.Round]map[types.ValidatorID]types.Hash),
		evidence:     make([]slashing.Evidence, 0),
		timeouts:     NewTimeoutCollector(config.ValidatorSet),
		pacemaker:    NewPacemaker(0, 0),
		blockTree:    NewBlockTree(),
		commitRule:   ThreeChainCommitRule{},
		committed:    make([]CommitDecision, 0),
		committedSet: make(map[types.Hash]struct{}),
	}, nil
}

func (machine *StateMachine) StartRound(height types.Height, round types.Round) {
	machine.status.Height = height
	machine.status.Round = round
	machine.status.Phase = PhasePropose
	machine.pacemaker = NewPacemaker(height, round)
}

func (machine *StateMachine) CreateProposal(block types.Block, round types.Round, proposer types.ValidatorID, justifyQC finality.QuorumCert) (Proposal, error) {
	if _, found := machine.validatorSet.Get(proposer); !found {
		return Proposal{}, ErrUnknownValidator
	}

	block.Header.ChainID = machine.chainID
	block.Header.ValidatorSetHash = machine.validatorSet.Hash()

	return Proposal{
		Block:     block,
		Round:     round,
		Proposer:  proposer,
		JustifyQC: justifyQC,
	}, nil
}

func (machine *StateMachine) OnProposal(ctx context.Context, proposal Proposal) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, found := machine.validatorSet.Get(proposal.Proposer); !found {
		return ErrUnknownValidator
	}
	if proposal.Block.Header.ChainID != machine.chainID {
		return fmt.Errorf("proposal chain id mismatch: %s", proposal.Block.Header.ChainID)
	}
	if proposal.Block.Header.Height == 0 {
		return fmt.Errorf("%w: missing height", ErrInvalidProposal)
	}
	if proposal.Block.Header.ValidatorSetHash != machine.validatorSet.Hash() {
		return fmt.Errorf("%w: validator set hash mismatch", ErrInvalidProposal)
	}
	if proposal.Block.Header.Height < machine.status.Height {
		return ErrStaleProposal
	}
	if proposal.Block.Header.Height == machine.status.Height && proposal.Round < machine.status.Round {
		return ErrStaleProposal
	}
	if proposal.JustifyQC.Height > 0 && proposal.JustifyQC.Height > proposal.Block.Header.Height {
		return fmt.Errorf("%w: justify qc height exceeds proposal height", ErrInvalidProposal)
	}
	if proposal.JustifyQC.Height > 0 && proposal.JustifyQC.BlockHash != proposal.Block.Header.PreviousBlockHash {
		return fmt.Errorf("%w: justify qc must match parent block", ErrInvalidProposal)
	}

	blockHash := machine.hashBlock(proposal.Block)
	machine.blockTree.Insert(proposal.Block, blockHash, proposal.JustifyQC)
	if candidate, found := machine.blockTree.CommitCandidate(blockHash); found {
		if _, err := machine.ApplyCommitRule(candidate); err != nil && !errors.Is(err, ErrCommitRuleNotSatisfied) {
			return err
		}
	}

	machine.status.Height = proposal.Block.Header.Height
	machine.status.Round = proposal.Round
	machine.status.Phase = PhaseVote
	return nil
}

func (machine *StateMachine) OnVote(ctx context.Context, vote Vote) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, found := machine.validatorSet.Get(vote.ValidatorID); !found {
		return ErrUnknownValidator
	}
	if err := machine.validateVote(vote); err != nil {
		return err
	}

	if err := machine.recordVote(vote); err != nil {
		return err
	}

	if qc, err := machine.BuildQuorumCert(vote.Height, vote.Round, vote.BlockHash); err == nil {
		if err := machine.blockTree.SetQuorumCert(qc); err != nil && !errors.Is(err, ErrBlockNotFound) {
			return err
		}
		machine.status.Phase = PhaseCommit
	}

	return nil
}

func (machine *StateMachine) validateVote(vote Vote) error {
	if vote.Height == 0 {
		return fmt.Errorf("%w: missing height", ErrInvalidVote)
	}
	if vote.BlockHash == (types.Hash{}) {
		return fmt.Errorf("%w: missing block hash", ErrInvalidVote)
	}
	if machine.status.Height > 0 && vote.Height < machine.status.Height {
		return ErrStaleVote
	}
	if machine.status.Height > 0 && vote.Height > machine.status.Height {
		return fmt.Errorf("%w: future height", ErrInvalidVote)
	}
	if vote.Height == machine.status.Height && vote.Round < machine.status.Round {
		return ErrStaleVote
	}
	if vote.Height == machine.status.Height && vote.Round > machine.status.Round {
		return fmt.Errorf("%w: future round", ErrInvalidVote)
	}
	return nil
}

func (machine *StateMachine) OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error) {
	select {
	case <-ctx.Done():
		return finality.TimeoutCert{}, ctx.Err()
	default:
	}

	if err := machine.timeouts.AddVote(vote); err != nil {
		return finality.TimeoutCert{}, err
	}
	timeoutCert, err := machine.timeouts.BuildTimeoutCert(vote.Height, vote.Round)
	if err != nil {
		return finality.TimeoutCert{}, err
	}
	if err := machine.pacemaker.AdvanceRound(timeoutCert); err != nil {
		return finality.TimeoutCert{}, err
	}
	machine.status.Height = machine.pacemaker.Height()
	machine.status.Round = machine.pacemaker.Round()
	machine.status.Phase = PhasePropose
	return timeoutCert, nil
}

func (machine *StateMachine) BuildQuorumCert(height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, error) {
	blockVotes := machine.votesForBlock(height, round, blockHash)
	if len(blockVotes) == 0 {
		return finality.QuorumCert{}, ErrNoQuorum
	}

	var votingPower types.VotingPower
	signers := make([]string, 0, len(blockVotes))
	for validatorID := range blockVotes {
		validatorInfo, found := machine.validatorSet.Get(validatorID)
		if !found {
			continue
		}
		votingPower += validatorInfo.VotingPower
		signers = append(signers, string(validatorID))
	}

	if !hasQuorum(votingPower, machine.validatorSet.TotalVotingPower()) {
		return finality.QuorumCert{}, ErrNoQuorum
	}

	sort.Strings(signers)
	return finality.QuorumCert{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		Signers:     types.Bitmap(strings.Join(signers, ",")),
		Signature:   types.AggregateSignature("placeholder-aggregate-signature"),
		VotingPower: votingPower,
	}, nil
}

func (machine *StateMachine) Status(ctx context.Context) Status {
	if ctx == nil {
		return machine.status
	}
	select {
	case <-ctx.Done():
		return machine.status
	default:
		return machine.status
	}
}

func (machine *StateMachine) Evidence() []slashing.Evidence {
	return append([]slashing.Evidence(nil), machine.evidence...)
}

func (machine *StateMachine) CommitDecisions() []CommitDecision {
	return append([]CommitDecision(nil), machine.committed...)
}

func (machine *StateMachine) ApplyCommitRule(candidate CommitCandidate) (CommitDecision, error) {
	decision, err := machine.commitRule.Decide(candidate)
	if err != nil {
		return CommitDecision{}, err
	}
	machine.status.LastFinalized = decision.CommittedBlockHash
	if _, found := machine.committedSet[decision.CommittedBlockHash]; !found {
		machine.committed = append(machine.committed, decision)
		machine.committedSet[decision.CommittedBlockHash] = struct{}{}
	}
	return decision, nil
}

func (machine *StateMachine) recordVote(vote Vote) error {
	machine.ensureVoteMaps(vote.Height, vote.Round, vote.BlockHash)

	if previousBlock, found := machine.votedBlocks[vote.Height][vote.Round][vote.ValidatorID]; found && previousBlock != vote.BlockHash {
		evidence, err := VoteConflictFromPrevious(previousBlock, vote)
		if err == nil {
			machine.evidence = append(machine.evidence, evidence)
		}
		return ErrConflictingVote
	}

	machine.votedBlocks[vote.Height][vote.Round][vote.ValidatorID] = vote.BlockHash
	machine.votes[vote.Height][vote.Round][vote.BlockHash][vote.ValidatorID] = vote
	return nil
}

func (machine *StateMachine) votesForBlock(height types.Height, round types.Round, blockHash types.Hash) map[types.ValidatorID]Vote {
	if _, found := machine.votes[height]; !found {
		return nil
	}
	if _, found := machine.votes[height][round]; !found {
		return nil
	}
	return machine.votes[height][round][blockHash]
}

func (machine *StateMachine) ensureVoteMaps(height types.Height, round types.Round, blockHash types.Hash) {
	if _, found := machine.votes[height]; !found {
		machine.votes[height] = make(map[types.Round]map[types.Hash]map[types.ValidatorID]Vote)
	}
	if _, found := machine.votes[height][round]; !found {
		machine.votes[height][round] = make(map[types.Hash]map[types.ValidatorID]Vote)
	}
	if _, found := machine.votes[height][round][blockHash]; !found {
		machine.votes[height][round][blockHash] = make(map[types.ValidatorID]Vote)
	}

	if _, found := machine.votedBlocks[height]; !found {
		machine.votedBlocks[height] = make(map[types.Round]map[types.ValidatorID]types.Hash)
	}
	if _, found := machine.votedBlocks[height][round]; !found {
		machine.votedBlocks[height][round] = make(map[types.ValidatorID]types.Hash)
	}
}

func hasQuorum(power types.VotingPower, total types.VotingPower) bool {
	if total == 0 {
		return false
	}
	return power*3 >= total*2
}

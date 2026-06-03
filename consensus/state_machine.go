package consensus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/vexo-network/vexo-consensus/consensus/internal/safety"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrUnknownValidator  = errors.New("unknown validator")
	ErrConflictingVote   = errors.New("conflicting vote")
	ErrNoQuorum          = errors.New("not enough voting power for quorum")
	ErrInvalidProposal   = errors.New("invalid proposal")
	ErrStaleProposal     = errors.New("stale proposal")
	ErrInvalidVote       = errors.New("invalid vote")
	ErrStaleVote         = errors.New("stale vote")
	ErrUnsafeProposal    = errors.New("unsafe proposal")
	ErrUnsafeVote        = errors.New("unsafe vote")
	ErrConflictingCommit = safety.ErrConflictingCommit
)

type StateMachineConfig struct {
	ChainID      string
	ValidatorSet validator.Set
	HashBlock    func(types.Block) types.Hash
	Signatures   signatureVerifier
	Aggregator   aggregateSigner
}

type StateMachine struct {
	mu           sync.Mutex
	chainID      string
	validatorSet validator.Set
	hashBlock    func(types.Block) types.Hash
	signatures   signatureVerifier
	aggregator   aggregateSigner
	status       Status
	votes        map[types.Height]map[types.Round]map[types.Hash]map[types.ValidatorID]Vote
	votedVotes   map[types.Height]map[types.Round]map[types.ValidatorID]Vote
	evidence     []slashing.Evidence
	timeouts     *TimeoutCollector
	pacemaker    *Pacemaker
	blockTree    *BlockTree
	lockedQC     finality.QuorumCert
	commitRule   ThreeChainCommitRule
	committed    []CommitDecision
	committedSet map[types.Hash]struct{}
	commitIndex  safety.CommitIndex
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
		signatures:   config.Signatures,
		aggregator:   config.Aggregator,
		status: Status{
			ChainID:          config.ChainID,
			Phase:            PhasePropose,
			ValidatorSetHash: config.ValidatorSet.Hash(),
		},
		votes:        make(map[types.Height]map[types.Round]map[types.Hash]map[types.ValidatorID]Vote),
		votedVotes:   make(map[types.Height]map[types.Round]map[types.ValidatorID]Vote),
		evidence:     make([]slashing.Evidence, 0),
		timeouts:     NewTimeoutCollectorWithAggregator(config.ValidatorSet, config.Aggregator),
		pacemaker:    NewPacemaker(0, 0),
		blockTree:    NewBlockTree(),
		commitRule:   ThreeChainCommitRule{},
		committed:    make([]CommitDecision, 0),
		committedSet: make(map[types.Hash]struct{}),
		commitIndex:  safety.NewCommitIndex(),
	}, nil
}

func (machine *StateMachine) StartRound(height types.Height, round types.Round) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	machine.status.Height = height
	machine.status.Round = round
	machine.status.Phase = PhasePropose
	machine.pacemaker = NewPacemaker(height, round)
}

func (machine *StateMachine) UpdateValidatorSet(validatorSet validator.Set) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if validatorSet == nil {
		return errors.New("validator set is required")
	}
	machine.validatorSet = validatorSet
	machine.status.ValidatorSetHash = validatorSet.Hash()
	machine.timeouts = NewTimeoutCollectorWithAggregator(validatorSet, machine.aggregator)
	return nil
}

func (machine *StateMachine) UpdateValidatorSetFromRegistry(ctx context.Context, registry validator.Registry, height types.Height) error {
	validatorSet, err := registry.ValidatorSet(ctx, height)
	if err != nil {
		return err
	}
	return machine.UpdateValidatorSet(validatorSet)
}

func (machine *StateMachine) CreateProposal(block types.Block, round types.Round, proposer types.ValidatorID, justifyQC finality.QuorumCert) (Proposal, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if _, found := machine.validatorSet.Get(proposer); !found {
		return Proposal{}, ErrUnknownValidator
	}

	if justifyQC.Height == 0 {
		justifyQC = machine.blockTree.HighQC()
	}
	block.Header.ChainID = machine.chainID
	block.Header.ValidatorSetHash = machine.validatorSet.Hash()
	block.Txs = fairordering.SortTxsWithSalt(block.Txs, machine.orderingSalt(block.Header.Height))
	if block.Header.ConsensusHash == (types.Hash{}) && len(block.Txs) > 0 {
		block = dataavailability.AttachCommitment(block)
	}
	if block.Header.PreviousBlockHash == (types.Hash{}) && justifyQC.Height > 0 {
		block.Header.PreviousBlockHash = justifyQC.BlockHash
	}

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
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if _, found := machine.validatorSet.Get(proposal.Proposer); !found {
		return ErrUnknownValidator
	}
	if err := machine.verifyProposalSignature(proposal); err != nil {
		return err
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
	if !fairordering.IsOrderedWithSalt(proposal.Block.Txs, machine.orderingSalt(proposal.Block.Header.Height)) {
		return fmt.Errorf("%w: transaction ordering mismatch", ErrInvalidProposal)
	}
	if err := dataavailability.Verify(proposal.Block.Header, proposal.Block.Txs); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProposal, err)
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
	if !machine.isSafeProposal(proposal) {
		return ErrUnsafeProposal
	}

	blockHash := machine.hashBlock(proposal.Block)
	machine.blockTree.Insert(proposal.Block, blockHash, proposal.JustifyQC)
	if candidate, found := machine.blockTree.CommitCandidate(blockHash); found {
		if _, err := machine.applyCommitRule(candidate); err != nil && !errors.Is(err, ErrCommitRuleNotSatisfied) {
			return err
		}
	}

	machine.status.Height = proposal.Block.Header.Height
	machine.status.Round = proposal.Round
	machine.status.Phase = PhaseVote
	return nil
}

func (machine *StateMachine) orderingSalt(height types.Height) []byte {
	return fairordering.HeightSalt(machine.chainID, height)
}

func (machine *StateMachine) OnVote(ctx context.Context, vote Vote) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if _, found := machine.validatorSet.Get(vote.ValidatorID); !found {
		return ErrUnknownValidator
	}
	if err := machine.validateVote(vote); err != nil {
		return err
	}

	if err := machine.recordVote(vote); err != nil {
		return err
	}

	if qc, err := machine.buildQuorumCert(vote.Height, vote.Round, vote.BlockHash); err == nil {
		setErr := machine.blockTree.SetQuorumCert(qc)
		if setErr != nil && !errors.Is(setErr, ErrBlockNotFound) {
			return setErr
		}
		if setErr == nil {
			machine.updateLockedQC(qc)
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
	if err := machine.verifyVoteSignature(vote); err != nil {
		return err
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
	node, found := machine.blockTree.Get(vote.BlockHash)
	if !found {
		return fmt.Errorf("%w: target block not found", ErrInvalidVote)
	}
	if node.Block.Header.Height != vote.Height {
		return fmt.Errorf("%w: target height mismatch", ErrInvalidVote)
	}
	if !machine.isSafeVoteTarget(node) {
		return ErrUnsafeVote
	}
	return nil
}

func (machine *StateMachine) OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error) {
	select {
	case <-ctx.Done():
		return finality.TimeoutCert{}, ctx.Err()
	default:
	}
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if err := machine.verifyTimeoutVoteSignature(vote); err != nil {
		return finality.TimeoutCert{}, err
	}
	previous, conflicting := machine.timeouts.ConflictingVote(vote)
	if err := machine.timeouts.AddVote(vote); err != nil {
		if conflicting && errors.Is(err, ErrConflictingTimeoutVote) {
			if evidence, evidenceErr := NewConflictingTimeoutVoteEvidence(previous, vote); evidenceErr == nil {
				machine.evidence = append(machine.evidence, evidence)
			}
		}
		return finality.TimeoutCert{}, err
	}
	timeoutCert, err := machine.timeouts.BuildTimeoutCert(vote.Height, vote.Round)
	if err != nil {
		return finality.TimeoutCert{}, err
	}
	if err := machine.pacemaker.AdvanceRound(timeoutCert); err != nil {
		return finality.TimeoutCert{}, err
	}
	machine.blockTree.ObserveQuorumCert(timeoutCert.HighQC)
	machine.status.Height = machine.pacemaker.Height()
	machine.status.Round = machine.pacemaker.Round()
	machine.status.Phase = PhasePropose
	machine.status.LastTimeoutCert = timeoutCert
	return timeoutCert, nil
}

func (machine *StateMachine) BuildQuorumCert(height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.buildQuorumCert(height, round, blockHash)
}

func (machine *StateMachine) buildQuorumCert(height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, error) {
	blockVotes := machine.votesForBlock(height, round, blockHash)
	if len(blockVotes) == 0 {
		return finality.QuorumCert{}, ErrNoQuorum
	}

	type signedVote struct {
		validatorID types.ValidatorID
		signature   types.Signature
	}
	var votingPower types.VotingPower
	votes := make([]signedVote, 0, len(blockVotes))
	for validatorID, vote := range blockVotes {
		validatorInfo, found := machine.validatorSet.Get(validatorID)
		if !found {
			continue
		}
		votingPower += validatorInfo.VotingPower
		votes = append(votes, signedVote{validatorID: validatorID, signature: vote.Signature})
	}

	if !hasQuorum(votingPower, machine.validatorSet.TotalVotingPower()) {
		return finality.QuorumCert{}, ErrNoQuorum
	}

	sort.Slice(votes, func(firstIndex int, secondIndex int) bool {
		return votes[firstIndex].validatorID < votes[secondIndex].validatorID
	})
	signers := make([]string, 0, len(votes))
	signatures := make([]types.Signature, 0, len(votes))
	for _, vote := range votes {
		signers = append(signers, string(vote.validatorID))
		signatures = append(signatures, vote.signature)
	}
	aggregateSignature := types.AggregateSignature("placeholder-aggregate-signature")
	if machine.aggregator != nil && allSignaturesPresent(signatures) {
		signature, err := machine.aggregator.Aggregate(signatures)
		if err != nil {
			return finality.QuorumCert{}, err
		}
		aggregateSignature = signature
	}
	return finality.QuorumCert{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		Signers:     types.Bitmap(strings.Join(signers, ",")),
		Signature:   aggregateSignature,
		VotingPower: votingPower,
	}, nil
}

func (machine *StateMachine) Status(ctx context.Context) Status {
	machine.mu.Lock()
	defer machine.mu.Unlock()

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
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return append([]slashing.Evidence(nil), machine.evidence...)
}

func (machine *StateMachine) verifyProposalSignature(proposal Proposal) error {
	if machine.signatures == nil {
		return nil
	}
	if len(proposal.Signature) == 0 {
		return fmt.Errorf("%w: missing proposal signature", ErrInvalidProposal)
	}
	validatorInfo, found := machine.validatorSet.Get(proposal.Proposer)
	if !found {
		return ErrUnknownValidator
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusProposal, ProposalSignBytes(proposal))
	if err != nil {
		return err
	}
	if !machine.signatures.Verify(validatorInfo.PublicKey, message, proposal.Signature) {
		return fmt.Errorf("%w: invalid proposal signature", ErrInvalidProposal)
	}
	return nil
}

func (machine *StateMachine) verifyVoteSignature(vote Vote) error {
	if machine.signatures == nil {
		return nil
	}
	if len(vote.Signature) == 0 {
		return fmt.Errorf("%w: missing vote signature", ErrInvalidVote)
	}
	validatorInfo, found := machine.validatorSet.Get(vote.ValidatorID)
	if !found {
		return ErrUnknownValidator
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusVote, VoteSignBytes(vote))
	if err != nil {
		return err
	}
	if !machine.signatures.Verify(validatorInfo.PublicKey, message, vote.Signature) {
		return fmt.Errorf("%w: invalid vote signature", ErrInvalidVote)
	}
	return nil
}

func (machine *StateMachine) verifyTimeoutVoteSignature(vote TimeoutVote) error {
	if machine.signatures == nil {
		return nil
	}
	if len(vote.Signature) == 0 {
		return fmt.Errorf("%w: missing timeout vote signature", ErrInvalidVote)
	}
	validatorInfo, found := machine.validatorSet.Get(vote.ValidatorID)
	if !found {
		return ErrUnknownValidator
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(vote))
	if err != nil {
		return err
	}
	if !machine.signatures.Verify(validatorInfo.PublicKey, message, vote.Signature) {
		return fmt.Errorf("%w: invalid timeout vote signature", ErrInvalidVote)
	}
	return nil
}

func (machine *StateMachine) CommitDecisions() []CommitDecision {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return append([]CommitDecision(nil), machine.committed...)
}

func (machine *StateMachine) ApplyCommitRule(candidate CommitCandidate) (CommitDecision, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.applyCommitRule(candidate)
}

func (machine *StateMachine) applyCommitRule(candidate CommitCandidate) (CommitDecision, error) {
	decision, err := machine.commitRule.Decide(candidate)
	if err != nil {
		return CommitDecision{}, err
	}
	if err := machine.commitIndex.Record(decision.CommittedHeight, decision.CommittedBlockHash); err != nil {
		return CommitDecision{}, err
	}
	machine.status.LastFinalized = decision.CommittedBlockHash
	if _, found := machine.committedSet[decision.CommittedBlockHash]; !found {
		machine.committed = append(machine.committed, decision)
		machine.committedSet[decision.CommittedBlockHash] = struct{}{}
	}
	return decision, nil
}

func (machine *StateMachine) isSafeProposal(proposal Proposal) bool {
	if machine.lockedQC.Height == 0 {
		return true
	}
	if proposal.JustifyQC.Height >= machine.lockedQC.Height {
		return true
	}
	parentHash := proposal.Block.Header.PreviousBlockHash
	return parentHash == machine.lockedQC.BlockHash || machine.blockTree.Extends(parentHash, machine.lockedQC.BlockHash)
}

func (machine *StateMachine) isSafeVoteTarget(node BlockNode) bool {
	if machine.lockedQC.Height == 0 {
		return true
	}
	if node.JustifyQC.Height >= machine.lockedQC.Height {
		return true
	}
	return node.Hash == machine.lockedQC.BlockHash || machine.blockTree.Extends(node.Hash, machine.lockedQC.BlockHash)
}

func (machine *StateMachine) updateLockedQC(qc finality.QuorumCert) {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return
	}
	if isBetterQC(qc, machine.lockedQC) {
		machine.lockedQC = qc
	}
}

func (machine *StateMachine) recordVote(vote Vote) error {
	machine.ensureVoteMaps(vote.Height, vote.Round, vote.BlockHash)

	if previousVote, found := machine.votedVotes[vote.Height][vote.Round][vote.ValidatorID]; found && previousVote.BlockHash != vote.BlockHash {
		evidence, err := NewConflictingVoteEvidence(previousVote, vote)
		if err == nil {
			machine.evidence = append(machine.evidence, evidence)
		}
		return ErrConflictingVote
	}

	machine.votedVotes[vote.Height][vote.Round][vote.ValidatorID] = vote
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

	if _, found := machine.votedVotes[height]; !found {
		machine.votedVotes[height] = make(map[types.Round]map[types.ValidatorID]Vote)
	}
	if _, found := machine.votedVotes[height][round]; !found {
		machine.votedVotes[height][round] = make(map[types.ValidatorID]Vote)
	}
}

func hasQuorum(power types.VotingPower, total types.VotingPower) bool {
	if total == 0 {
		return false
	}
	return power*3 >= total*2
}

func allSignaturesPresent(signatures []types.Signature) bool {
	if len(signatures) == 0 {
		return false
	}
	for _, signature := range signatures {
		if len(signature) == 0 {
			return false
		}
	}
	return true
}

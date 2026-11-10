package node

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	maxPendingVoteBlocks    = 256
	maxPendingVotesPerBlock = 1024
	// maxBufferedVotesPerValidator bounds the votes of one validator kept for blocks this node has
	// not seen yet. A validator signs one vote per view, so honest traffic stays far below it, and a
	// validator that floods votes only uses up its own budget.
	maxBufferedVotesPerValidator = 64
	// maxCachedLocalVotes bounds the cache of this validator's own votes. A vote that falls out of
	// the cache is still in the WAL, which restores it.
	maxCachedLocalVotes    = 1024
	consensusGossipTimeout = 15 * time.Second
)

type autoVoteReactor struct {
	machine       *consensus.StateMachine
	chainID       string
	validatorID   types.ValidatorID
	signer        vexocrypto.Signer
	broadcastVote func(context.Context, consensus.Vote) error
	// validateProposal asks the application whether it accepts the block of a proposal. The validator
	// votes for a proposal only when it does.
	validateProposal   func(context.Context, consensus.Proposal) error
	onProposalAccepted func(consensus.Proposal, types.Hash)
	onProposalRejected func(consensus.Proposal, error)
	onVoteAccepted     func(context.Context)
	onEvidence         func(context.Context, slashing.Evidence)
	onError            func(string, error)
	onProposalLatency  func(time.Duration)
	onVoteLatency      func(time.Duration)
	onSigningFailure   func()
	wal                *consensus.WAL
	localVoteMu        sync.Mutex
	mu                 sync.Mutex
	pendingVotes       map[types.Hash][]consensus.Vote
	localVotes         map[voteRound]consensus.Vote
	unknownVotes       map[unknownVoteKey]consensus.Vote
	// pendingPerValidator and unknownPerValidator count the buffered votes of each validator.
	pendingPerValidator map[types.ValidatorID]int
	unknownPerValidator map[types.ValidatorID]int
}

type voteRound struct {
	height types.Height
	round  types.Round
}

type unknownVoteKey struct {
	height      types.Height
	round       types.Round
	validatorID types.ValidatorID
}

func (reactor *autoVoteReactor) OnProposal(ctx context.Context, proposal consensus.Proposal) error {
	started := time.Now()
	defer func() {
		if reactor.onProposalLatency != nil {
			reactor.onProposalLatency(time.Since(started))
		}
	}()
	before := reactor.machine.EvidenceCursor()
	if err := reactor.machine.OnProposal(ctx, proposal); err != nil {
		if errors.Is(err, consensus.ErrStaleProposal) {
			blockHash := consensus.HashBlock(proposal.Block)
			if reactor.onProposalAccepted != nil {
				reactor.onProposalAccepted(proposal, blockHash)
			}
			reactor.replayPendingVotes(ctx, blockHash)
			reactor.publishNewEvidence(ctx, before)
			if reactor.onVoteAccepted != nil {
				reactor.onVoteAccepted(ctx)
			}
			return nil
		}
		if proposalRetryable(err) {
			// Judged once its parent arrives or the block its validator set depends on is
			// executed; the proposer delivers it again.
			return err
		}
		reactor.reportError("proposal_rejected", err)
		// A signed proposal whose header breaks a rule every node judges alike is evidence
		// against its proposer.
		reactor.publishNewEvidence(ctx, before)
		if reactor.onProposalRejected != nil {
			reactor.onProposalRejected(proposal, err)
		}
		return err
	}
	blockHash := consensus.HashBlock(proposal.Block)
	if reactor.onProposalAccepted != nil {
		reactor.onProposalAccepted(proposal, blockHash)
	}
	if reactor.validateProposal != nil {
		if err := reactor.validateProposal(ctx, proposal); err != nil {
			// The proposal is authentic and the other validators may still certify it, so it stays
			// cached for execution. This validator just does not vote for it, and the relaying
			// peer is not at fault.
			reactor.reportError("proposal_declined", err)
			reactor.replayPendingVotes(ctx, blockHash)
			reactor.publishNewEvidence(ctx, before)
			return nil
		}
	}
	vote, selected, err := reactor.localVote(proposal.Block.Header.Height, proposal.Round, blockHash)
	if err != nil {
		return err
	}
	if !selected {
		reactor.replayPendingVotes(ctx, blockHash)
		reactor.publishNewEvidence(ctx, before)
		return nil
	}
	if err := reactor.machine.OnVote(ctx, vote); err != nil {
		if errors.Is(err, consensus.ErrStaleVote) {
			return nil
		}
		reactor.reportError("local_vote_rejected", err)
		return err
	}
	reactor.replayPendingVotes(ctx, blockHash)
	reactor.publishNewEvidence(ctx, before)
	if reactor.broadcastVote == nil {
		return nil
	}
	if err := reactor.broadcastVote(ctx, vote); err != nil {
		reactor.reportError("vote_broadcast_failed", err)
	}
	return nil
}

func (reactor *autoVoteReactor) localVote(height types.Height, round types.Round, blockHash types.Hash) (consensus.Vote, bool, error) {
	reactor.localVoteMu.Lock()
	defer reactor.localVoteMu.Unlock()

	if vote, cached := reactor.cachedLocalVote(height, round); cached {
		return vote, vote.BlockHash == blockHash, nil
	}
	if reactor.wal != nil {
		recordedHash, recorded, err := reactor.wal.RecordedVote(reactor.validatorID, height, round)
		if err != nil {
			reactor.reportError("vote_wal_restore_failed", err)
			return consensus.Vote{}, false, err
		}
		if recorded {
			vote, err := reactor.signLocalVote(height, round, recordedHash)
			if err != nil {
				return consensus.Vote{}, false, err
			}
			reactor.cacheLocalVote(vote)
			return vote, recordedHash == blockHash, nil
		}
	}
	// The machine already moved its lock and last voted view when it accepted the proposal. They are
	// persisted before anything is signed, so a restart cannot vote against them.
	if err := reactor.recordSafetyState(); err != nil {
		return consensus.Vote{}, false, err
	}
	vote, err := reactor.signLocalVote(height, round, blockHash)
	if err != nil {
		return consensus.Vote{}, false, err
	}
	if reactor.wal != nil {
		if err := reactor.wal.RecordVote(vote); err != nil {
			reactor.reportError("vote_wal_failed", err)
			return consensus.Vote{}, false, err
		}
	}
	reactor.cacheLocalVote(vote)
	return vote, true, nil
}

func (reactor *autoVoteReactor) hasLocalVote(height types.Height, round types.Round) (bool, error) {
	reactor.localVoteMu.Lock()
	defer reactor.localVoteMu.Unlock()
	if _, cached := reactor.cachedLocalVote(height, round); cached {
		return true, nil
	}
	if reactor.wal == nil {
		return false, nil
	}
	_, recorded, err := reactor.wal.RecordedVote(reactor.validatorID, height, round)
	return recorded, err
}

func (node *Node) hasRecordedLocalVote(height types.Height, round types.Round) (bool, error) {
	node.mu.Lock()
	voter := node.localVoter
	node.mu.Unlock()
	if voter == nil {
		return false, nil
	}
	return voter.hasLocalVote(height, round)
}

func (reactor *autoVoteReactor) signLocalVote(height types.Height, round types.Round, blockHash types.Hash) (consensus.Vote, error) {
	vote := consensus.Vote{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		ValidatorID: reactor.validatorID,
	}
	if err := signConsensusVote(reactor.chainID, reactor.signer, &vote); err != nil {
		if reactor.onSigningFailure != nil {
			reactor.onSigningFailure()
		}
		reactor.reportError("vote_sign_failed", err)
		return consensus.Vote{}, err
	}
	return vote, nil
}

func (reactor *autoVoteReactor) cachedLocalVote(height types.Height, round types.Round) (consensus.Vote, bool) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	vote, ok := reactor.localVotes[voteRound{height: height, round: round}]
	return vote, ok
}

func (reactor *autoVoteReactor) cacheLocalVote(vote consensus.Vote) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	if reactor.localVotes == nil {
		reactor.localVotes = make(map[voteRound]consensus.Vote)
	}
	key := voteRound{height: vote.Height, round: vote.Round}
	if _, cached := reactor.localVotes[key]; !cached && len(reactor.localVotes) >= maxCachedLocalVotes {
		oldest, found := voteRound{}, false
		for candidate := range reactor.localVotes {
			if !found || candidate.height < oldest.height || (candidate.height == oldest.height && candidate.round < oldest.round) {
				oldest, found = candidate, true
			}
		}
		delete(reactor.localVotes, oldest)
	}
	reactor.localVotes[key] = vote
}

// recordSafetyState persists the lock, the highest certificate and the last voted view of the
// machine. The validator calls it before it signs a vote, so what it signed can never be voted
// against after a restart.
func (reactor *autoVoteReactor) recordSafetyState() error {
	if reactor.wal == nil || reactor.machine == nil {
		return nil
	}
	if err := reactor.wal.RecordSafetyState(reactor.machine.SafetyState()); err != nil {
		reactor.reportError("safety_state_wal_failed", err)
		return err
	}
	return nil
}

// pruneAtOrBelow drops everything the reactor buffers for heights that are committed.
func (reactor *autoVoteReactor) pruneAtOrBelow(height types.Height) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	for blockHash, votes := range reactor.pendingVotes {
		kept := votes[:0]
		for _, vote := range votes {
			if vote.Height <= height {
				reactor.releaseBuffered(reactor.pendingPerValidator, vote.ValidatorID)
				continue
			}
			kept = append(kept, vote)
		}
		if len(kept) == 0 {
			delete(reactor.pendingVotes, blockHash)
			continue
		}
		reactor.pendingVotes[blockHash] = kept
	}
	for key := range reactor.unknownVotes {
		if key.height <= height {
			delete(reactor.unknownVotes, key)
			reactor.releaseBuffered(reactor.unknownPerValidator, key.validatorID)
		}
	}
	for key := range reactor.localVotes {
		if key.height <= height {
			delete(reactor.localVotes, key)
		}
	}
}

func (reactor *autoVoteReactor) releaseBuffered(counts map[types.ValidatorID]int, validatorID types.ValidatorID) {
	if counts[validatorID] <= 1 {
		delete(counts, validatorID)
		return
	}
	counts[validatorID]--
}

func (reactor *autoVoteReactor) OnVote(ctx context.Context, vote consensus.Vote) error {
	started := time.Now()
	defer func() {
		if reactor.onVoteLatency != nil {
			reactor.onVoteLatency(time.Since(started))
		}
	}()
	before := reactor.machine.EvidenceCursor()
	err := reactor.machine.OnVote(ctx, vote)
	if errors.Is(err, consensus.ErrUnknownVoteBlock) {
		reactor.publishUnknownVoteEvidence(ctx, vote)
		reactor.cachePendingVote(vote)
		return nil
	}
	if errors.Is(err, consensus.ErrStaleVote) {
		return nil
	}
	if err != nil {
		reactor.reportError("vote_rejected", err)
	}
	reactor.publishNewEvidence(ctx, before)
	if err == nil && reactor.onVoteAccepted != nil {
		reactor.onVoteAccepted(ctx)
	}
	return err
}

func (reactor *autoVoteReactor) publishUnknownVoteEvidence(ctx context.Context, vote consensus.Vote) {
	if reactor.onEvidence == nil {
		return
	}
	reactor.mu.Lock()
	if reactor.unknownVotes == nil {
		reactor.unknownVotes = make(map[unknownVoteKey]consensus.Vote)
	}
	key := unknownVoteKey{height: vote.Height, round: vote.Round, validatorID: vote.ValidatorID}
	previous, found := reactor.unknownVotes[key]
	if !found {
		if reactor.unknownPerValidator[vote.ValidatorID] < maxBufferedVotesPerValidator {
			reactor.unknownVotes[key] = vote
			if reactor.unknownPerValidator == nil {
				reactor.unknownPerValidator = make(map[types.ValidatorID]int)
			}
			reactor.unknownPerValidator[vote.ValidatorID]++
		}
		reactor.mu.Unlock()
		return
	}
	if previous.BlockHash == vote.BlockHash {
		reactor.mu.Unlock()
		return
	}
	reactor.mu.Unlock()
	evidence, err := consensus.NewConflictingVoteEvidence(previous, vote)
	if err != nil {
		return
	}
	reactor.onEvidence(ctx, evidence)
}

func (reactor *autoVoteReactor) reportError(event string, err error) {
	if reactor.onError != nil && err != nil {
		reactor.onError(event, err)
	}
}

func (reactor *autoVoteReactor) cachePendingVote(vote consensus.Vote) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	if reactor.pendingVotes == nil {
		reactor.pendingVotes = make(map[types.Hash][]consensus.Vote)
	}
	if len(reactor.pendingVotes) >= maxPendingVoteBlocks && len(reactor.pendingVotes[vote.BlockHash]) == 0 {
		return
	}
	votes := reactor.pendingVotes[vote.BlockHash]
	if len(votes) >= maxPendingVotesPerBlock {
		return
	}
	for _, existing := range votes {
		if existing.ValidatorID == vote.ValidatorID && existing.Height == vote.Height && existing.Round == vote.Round {
			return
		}
	}
	if reactor.pendingPerValidator[vote.ValidatorID] >= maxBufferedVotesPerValidator {
		return
	}
	if reactor.pendingPerValidator == nil {
		reactor.pendingPerValidator = make(map[types.ValidatorID]int)
	}
	reactor.pendingPerValidator[vote.ValidatorID]++
	reactor.pendingVotes[vote.BlockHash] = append(votes, vote)
}

func (reactor *autoVoteReactor) replayPendingVotes(ctx context.Context, blockHash types.Hash) {
	votes := reactor.takePendingVotes(blockHash)
	for _, vote := range votes {
		before := reactor.machine.EvidenceCursor()
		err := reactor.machine.OnVote(ctx, vote)
		reactor.publishNewEvidence(ctx, before)
		if err == nil && reactor.onVoteAccepted != nil {
			reactor.onVoteAccepted(ctx)
		}
	}
}

func (reactor *autoVoteReactor) takePendingVotes(blockHash types.Hash) []consensus.Vote {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	votes := reactor.pendingVotes[blockHash]
	if len(votes) == 0 {
		return nil
	}
	delete(reactor.pendingVotes, blockHash)
	for _, vote := range votes {
		reactor.releaseBuffered(reactor.pendingPerValidator, vote.ValidatorID)
	}
	return append([]consensus.Vote(nil), votes...)
}

func (node *Node) cachedProposalForRound(height types.Height, round types.Round) (consensus.Proposal, types.Hash, bool) {
	for _, proposal := range node.pendingProposals() {
		if proposal.Block.Header.Height != height || proposal.Round != round || proposal.Proposer != node.cfg.ValidatorID {
			continue
		}
		blockHash := consensus.HashBlock(proposal.Block)
		return proposal, blockHash, true
	}
	return consensus.Proposal{}, types.Hash{}, false
}

func (reactor *autoVoteReactor) OnTimeoutVote(ctx context.Context, vote consensus.TimeoutVote) (finality.TimeoutCert, error) {
	started := time.Now()
	timeoutCert, err := reactor.machine.OnTimeoutVote(ctx, vote)
	if reactor.onVoteLatency != nil {
		reactor.onVoteLatency(time.Since(started))
	}
	return timeoutCert, err
}

func (reactor *autoVoteReactor) publishNewEvidence(ctx context.Context, cursor uint64) {
	if reactor.onEvidence == nil {
		return
	}
	evidence, _ := reactor.machine.EvidenceSince(cursor)
	for _, item := range evidence {
		reactor.onEvidence(ctx, item)
	}
}

func (node *Node) ProposeBlock(ctx context.Context, block types.Block) (consensus.Proposal, types.Hash, error) {
	started := time.Now()
	if node.cfg.ValidatorID == "" {
		return consensus.Proposal{}, types.Hash{}, ErrMissingValidatorID
	}
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, err
	}
	reactor, err := node.ConsensusReactor()
	if err != nil {
		return consensus.Proposal{}, types.Hash{}, ErrConsensusOffline
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		status := machine.Status(ctx)
		height := block.Header.Height
		if height == 0 || height < status.Height {
			height = status.Height
		}
		if height == 0 {
			height = 1
		}
		candidateBlock := block
		candidateBlock.Header.Height = height
		if candidateBlock.Header.TimeUnixNano == 0 {
			candidateBlock.Header.TimeUnixNano = time.Now().UnixNano()
		}
		// Views are global: the proposal goes into the current view whatever its height.
		round := status.Round

		proposal, err := machine.CreateProposal(candidateBlock, round, node.cfg.ValidatorID, finality.QuorumCert{})
		if err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
		// The proposer votes for its own proposal, so the block must pass the application check every
		// other validator runs before it is signed.
		if err := node.processProposal(ctx, proposal); err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
		if err := node.signConsensusProposal(&proposal); err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
		if err := machine.OnProposal(ctx, proposal); err != nil {
			if errors.Is(err, consensus.ErrStaleProposal) {
				lastErr = err
				continue
			}
			return consensus.Proposal{}, types.Hash{}, err
		}
		if err := node.recordConsensusProposal(proposal); err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
		blockHash := consensus.HashBlock(proposal.Block)
		node.markProposed(proposal.Block.Header.Height, proposal.Round)
		node.cacheProposal(proposal, blockHash)
		vote, selected, err := node.voteLocalProposal(ctx, proposal, blockHash)
		if err != nil {
			if errors.Is(err, consensus.ErrStaleVote) {
				lastErr = err
				continue
			}
			return consensus.Proposal{}, types.Hash{}, err
		}
		if !selected {
			node.removePending(blockHash)
			node.removeProposed(proposal.Block.Header.Height, proposal.Round)
			return consensus.Proposal{}, types.Hash{}, ErrLocalVoteRecorded
		}
		// The ancestors went out when they were proposed; a peer that missed one gets it with the
		// retransmission of this proposal (tickConsensusWithProposalOptions).
		node.broadcastConsensusAsync("proposal_broadcast_failed", map[string]any{
			"height": proposal.Block.Header.Height,
			"round":  proposal.Round,
		}, func(ctx context.Context) error {
			return reactor.BroadcastProposal(ctx, proposal)
		})
		node.broadcastConsensusAsync("vote_broadcast_failed", map[string]any{
			"height": vote.Height,
			"round":  vote.Round,
		}, func(ctx context.Context) error {
			return reactor.BroadcastVote(ctx, vote)
		})
		node.logEvent("block_proposed", map[string]any{
			"chain_id":   proposal.Block.Header.ChainID,
			"height":     proposal.Block.Header.Height,
			"round":      proposal.Round,
			"block_hash": fmt.Sprintf("%x", blockHash),
			"tx_count":   len(proposal.Block.Txs),
			"proposer":   proposal.Proposer,
		})
		node.metrics.observeProposalLatency(time.Since(started))
		return proposal, blockHash, nil
	}
	if lastErr != nil {
		return consensus.Proposal{}, types.Hash{}, lastErr
	}
	return consensus.Proposal{}, types.Hash{}, consensus.ErrStaleProposal
}

func (node *Node) broadcastAncestorProposals(reactor *consensus.TransportReactor, beforeHeight types.Height) {
	if reactor == nil || beforeHeight <= 1 {
		return
	}
	ctx := context.Background()
	node.pruneStalePendingProposals(ctx)
	pending := node.pendingProposals()
	proposals := make([]consensus.Proposal, 0, len(pending))
	for _, proposal := range pending {
		proposals = append(proposals, proposal)
	}
	sort.Slice(proposals, func(left, right int) bool {
		if proposals[left].Block.Header.Height == proposals[right].Block.Header.Height {
			return proposals[left].Round < proposals[right].Round
		}
		return proposals[left].Block.Header.Height < proposals[right].Block.Header.Height
	})
	for _, proposal := range proposals {
		if proposal.Block.Header.Height == 0 || proposal.Block.Header.Height >= beforeHeight {
			continue
		}
		if !node.cachedProposalStillValid(ctx, proposal) {
			node.removePending(consensus.HashBlock(proposal.Block))
			node.removeProposed(proposal.Block.Header.Height, proposal.Round)
			continue
		}
		proposal := proposal
		node.broadcastConsensusAsync("ancestor_proposal_broadcast_failed", map[string]any{
			"height": proposal.Block.Header.Height,
			"round":  proposal.Round,
		}, func(ctx context.Context) error {
			return reactor.BroadcastProposal(ctx, proposal)
		})
	}
}

func (node *Node) voteLocalProposal(ctx context.Context, proposal consensus.Proposal, blockHash types.Hash) (consensus.Vote, bool, error) {
	node.mu.Lock()
	voter := node.localVoter
	node.mu.Unlock()
	if voter == nil {
		return consensus.Vote{}, false, ErrMissingValidatorID
	}
	vote, selected, err := voter.localVote(proposal.Block.Header.Height, proposal.Round, blockHash)
	if err != nil || !selected {
		return vote, selected, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Vote{}, false, err
	}
	if err := machine.OnVote(ctx, vote); err != nil {
		return consensus.Vote{}, false, err
	}
	return vote, true, nil
}

func (node *Node) VoteBlock(ctx context.Context, height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, bool, error) {
	started := time.Now()
	if node.cfg.ValidatorID == "" {
		return finality.QuorumCert{}, false, ErrMissingValidatorID
	}
	machine, err := node.Consensus()
	if err != nil {
		return finality.QuorumCert{}, false, err
	}
	reactor, err := node.ConsensusReactor()
	if err != nil {
		return finality.QuorumCert{}, false, ErrConsensusOffline
	}

	node.mu.Lock()
	voter := node.localVoter
	node.mu.Unlock()
	if voter == nil {
		return finality.QuorumCert{}, false, ErrMissingValidatorID
	}
	// A vote is signed only for the proposal the machine accepted for the view: the lock and the last
	// voted view were checked when it did. The application must accept the block as well, unless this
	// validator voted already, which it did only after the application accepted it.
	proposal, accepted := node.acceptedProposalFor(machine, height, round, blockHash)
	if !accepted {
		return finality.QuorumCert{}, false, ErrUnauthorizedVote
	}
	recorded, err := voter.hasLocalVote(height, round)
	if err != nil {
		return finality.QuorumCert{}, false, err
	}
	if !recorded {
		if err := node.processProposal(ctx, proposal); err != nil {
			node.reportInvalidAppHashProposal(ctx, proposal, err)
			return finality.QuorumCert{}, false, err
		}
	}
	vote, selected, err := voter.localVote(height, round, blockHash)
	if err != nil {
		return finality.QuorumCert{}, false, err
	}
	if !selected {
		return finality.QuorumCert{}, false, ErrLocalVoteRecorded
	}
	if err := machine.OnVote(ctx, vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
	node.broadcastConsensusAsync("vote_broadcast_failed", map[string]any{
		"height": vote.Height,
		"round":  vote.Round,
	}, func(ctx context.Context) error {
		return reactor.BroadcastVote(ctx, vote)
	})
	qc, err := machine.BuildQuorumCert(height, round, blockHash)
	if err != nil {
		return finality.QuorumCert{}, false, nil
	}
	node.metrics.observeVoteLatency(time.Since(started))
	return qc, true, nil
}

func (node *Node) TimeoutRound(ctx context.Context) (finality.TimeoutCert, bool, error) {
	started := time.Now()
	if node.cfg.ValidatorID == "" {
		return finality.TimeoutCert{}, false, ErrMissingValidatorID
	}
	machine, err := node.Consensus()
	if err != nil {
		return finality.TimeoutCert{}, false, err
	}
	reactor, err := node.ConsensusReactor()
	if err != nil {
		return finality.TimeoutCert{}, false, ErrConsensusOffline
	}

	status := machine.Status(ctx)
	if status.Height == 0 {
		machine.StartRound(1, status.Round)
		status = machine.Status(ctx)
	}
	if vote, ok := node.cachedTimeoutVote(status.Height, status.Round); ok {
		node.broadcastConsensusAsync("timeout_vote_broadcast_failed", map[string]any{
			"height": vote.Height,
			"round":  vote.Round,
		}, func(ctx context.Context) error {
			return reactor.BroadcastTimeoutVote(ctx, vote)
		})
		return finality.TimeoutCert{}, false, nil
	}
	vote := consensus.TimeoutVote{
		Height:      status.Height,
		Round:       status.Round,
		ValidatorID: node.cfg.ValidatorID,
		HighQC:      machine.HighQC(ctx),
	}
	// Leaving the view is persisted before the timeout vote is signed, so a restart cannot vote in a
	// view this validator has already given up on.
	if err := node.recordTimeoutSafety(machine, vote); err != nil {
		return finality.TimeoutCert{}, false, err
	}
	if err := node.signConsensusTimeoutVote(&vote); err != nil {
		return finality.TimeoutCert{}, false, err
	}
	if err := node.recordConsensusTimeoutVote(vote); err != nil {
		return finality.TimeoutCert{}, false, err
	}
	node.cacheTimeoutVote(vote)
	timeoutCert, err := machine.OnTimeoutVote(ctx, vote)
	if err != nil && !errors.Is(err, consensus.ErrNoQuorum) && !errors.Is(err, consensus.ErrStaleTimeoutCert) {
		return finality.TimeoutCert{}, false, err
	}
	if errors.Is(err, consensus.ErrStaleTimeoutCert) {
		return finality.TimeoutCert{}, false, nil
	}
	node.broadcastConsensusAsync("timeout_vote_broadcast_failed", map[string]any{
		"height": vote.Height,
		"round":  vote.Round,
	}, func(ctx context.Context) error {
		return reactor.BroadcastTimeoutVote(ctx, vote)
	})
	if err != nil {
		return finality.TimeoutCert{}, false, nil
	}
	node.metrics.observeVoteLatency(time.Since(started))
	return timeoutCert, true, nil
}

func (node *Node) logConsensusBroadcastFailure(event string, err error, fields map[string]any) {
	if err == nil {
		return
	}
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["error"] = err.Error()
	node.logEvent(event, fields)
}

func (node *Node) broadcastConsensusAsync(event string, fields map[string]any, broadcast func(context.Context) error) {
	if broadcast == nil {
		return
	}
	key := fmt.Sprintf("%s:%v:%v", event, fields["height"], fields["round"])
	node.broadcastMu.Lock()
	if node.broadcasts == nil {
		node.broadcasts = make(map[string]struct{})
	}
	if _, found := node.broadcasts[key]; found {
		node.broadcastMu.Unlock()
		return
	}
	node.broadcasts[key] = struct{}{}
	node.broadcastMu.Unlock()
	copiedFields := make(map[string]any, len(fields))
	for key, value := range fields {
		copiedFields[key] = value
	}
	go func() {
		defer func() {
			node.broadcastMu.Lock()
			delete(node.broadcasts, key)
			node.broadcastMu.Unlock()
		}()
		deadline := time.Now().Add(consensusGossipTimeout)
		var lastErr error
		for attempt := 0; ; attempt++ {
			if time.Now().After(deadline) {
				break
			}
			attemptCtx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
			lastErr = broadcast(attemptCtx)
			cancel()
			if lastErr == nil {
				return
			}
			sleep := time.Duration(250*(attempt+1)) * time.Millisecond
			if sleep > 2*time.Second {
				sleep = 2 * time.Second
			}
			if time.Now().Add(sleep).After(deadline) {
				break
			}
			time.Sleep(sleep)
		}
		node.logConsensusBroadcastFailure(event, lastErr, copiedFields)
	}()
}

func (node *Node) CommitBlock(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) (app.FinalizeBlockResponse, error) {
	return node.commitBlock(ctx, block, quorumCert, true, true)
}

func (node *Node) recordConsensusProposal(proposal consensus.Proposal) error {
	node.mu.Lock()
	wal := node.consensusWAL
	node.mu.Unlock()
	if wal == nil {
		return nil
	}
	return wal.RecordProposal(proposal)
}

func (node *Node) signConsensusProposal(proposal *consensus.Proposal) error {
	node.mu.Lock()
	signer := node.signer
	node.mu.Unlock()
	if signer == nil {
		node.metrics.observeSigningFailure()
		return ErrMissingSigner
	}
	signature, err := signWithConsensusPolicy(signer, vexocrypto.SignPolicy{
		ChainID: proposal.Block.Header.ChainID,
		Height:  proposal.Block.Header.Height,
		Round:   proposal.Round,
		Type:    vexocrypto.SignTypeConsensusProposal,
		Domain:  vexocrypto.DomainConsensusProposal,
	}, consensus.ProposalSignBytes(*proposal))
	if err != nil {
		node.metrics.observeSigningFailure()
		return err
	}
	proposal.Signature = signature
	return nil
}

func (node *Node) signConsensusVote(vote *consensus.Vote) error {
	node.mu.Lock()
	signer := node.signer
	chainID := node.cfg.Chain.ChainID
	node.mu.Unlock()
	if err := signConsensusVote(chainID, signer, vote); err != nil {
		node.metrics.observeSigningFailure()
		return err
	}
	return nil
}

func signConsensusVote(chainID string, signer vexocrypto.Signer, vote *consensus.Vote) error {
	if signer == nil {
		return ErrMissingSigner
	}
	signature, err := signWithConsensusPolicy(signer, vexocrypto.SignPolicy{
		ChainID: chainID,
		Height:  vote.Height,
		Round:   vote.Round,
		Type:    vexocrypto.SignTypeConsensusVote,
		Domain:  vexocrypto.DomainConsensusVote,
	}, consensus.VoteSignBytes(*vote))
	if err != nil {
		return err
	}
	vote.Signature = signature
	return nil
}

func (node *Node) signConsensusTimeoutVote(vote *consensus.TimeoutVote) error {
	node.mu.Lock()
	signer := node.signer
	chainID := node.cfg.Chain.ChainID
	node.mu.Unlock()
	if signer == nil {
		node.metrics.observeSigningFailure()
		return ErrMissingSigner
	}
	signature, err := signWithConsensusPolicy(signer, vexocrypto.SignPolicy{
		ChainID: chainID,
		Height:  vote.Height,
		Round:   vote.Round,
		Type:    vexocrypto.SignTypeConsensusTimeoutVote,
		Domain:  vexocrypto.DomainConsensusTimeoutVote,
	}, consensus.TimeoutVoteSignBytes(*vote))
	if err != nil {
		node.metrics.observeSigningFailure()
		return err
	}
	vote.Signature = signature
	return nil
}

func (node *Node) recordConsensusVote(vote consensus.Vote) error {
	node.mu.Lock()
	wal := node.consensusWAL
	node.mu.Unlock()
	if wal == nil {
		return nil
	}
	return wal.RecordVote(vote)
}

func signWithConsensusPolicy(signer vexocrypto.Signer, policy vexocrypto.SignPolicy, signBytes []byte) (types.Signature, error) {
	if policy.ChainID == "" {
		return vexocrypto.SignWithDomain(signer, policy.Domain, signBytes)
	}
	policySigner, ok := signer.(vexocrypto.PolicySigner)
	if !ok {
		return vexocrypto.SignWithDomain(signer, policy.Domain, signBytes)
	}
	message, err := vexocrypto.DomainMessage(policy.Domain, signBytes)
	if err != nil {
		return nil, err
	}
	return policySigner.SignWithPolicy(policy, message)
}

func (node *Node) recordConsensusTimeoutVote(vote consensus.TimeoutVote) error {
	node.mu.Lock()
	wal := node.consensusWAL
	node.mu.Unlock()
	if wal == nil {
		return nil
	}
	return wal.RecordTimeoutVote(vote)
}

// commitBlock executes a certified block and finishes its commit. A commit raises the height the
// evidence pool and the mempool judge at, which makes the evidence of that height admissible, so the
// evidence that waits for it is taken up once the commit released its locks. That follows every
// commit that made a block durable, even when finishing it failed or the broadcast of the commit did.
func (node *Node) commitBlock(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, requireLocalQC bool, broadcast bool) (app.FinalizeBlockResponse, error) {
	before, _ := node.currentCommitHeight(ctx)
	response, err := node.executeCommit(ctx, block, quorumCert, requireLocalQC, broadcast)
	if after, ok := node.currentCommitHeight(ctx); ok && after > before {
		node.queueEvidence(ctx)
	}
	return response, err
}

func (node *Node) executeCommit(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, requireLocalQC bool, broadcast bool) (app.FinalizeBlockResponse, error) {
	started := time.Now()
	if quorumCert.Height != block.Header.Height {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: height mismatch", ErrInvalidCommitQC)
	}
	blockHash := consensus.HashBlock(block)
	if quorumCert.BlockHash != blockHash {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: block hash mismatch", ErrInvalidCommitQC)
	}
	// The certificate vouches for the header hash alone. The transactions are bound to the header
	// by the body checks, so a block whose transactions were swapped never reaches execution.
	if err := consensus.VerifyBlockBody(block); err != nil {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: %w", ErrInvalidCommitBlock, err)
	}
	node.commitMu.Lock()
	defer node.commitMu.Unlock()
	node.admissionMu.Lock()
	defer node.admissionMu.Unlock()
	runtime, err := node.Runtime()
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	committedHeight, ok := node.currentCommitHeight(ctx)
	if !ok {
		return app.FinalizeBlockResponse{}, ErrNodeNotRunning
	}
	if block.Header.Height <= committedHeight {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: height=%d committed=%d", ErrBlockAlreadyCommitted, block.Header.Height, committedHeight)
	}
	if expected, _ := nextHeightAfter(committedHeight); block.Header.Height != expected {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: height=%d expected=%d", ErrNonSequentialCommit, block.Header.Height, expected)
	}
	if err := node.checkParentBinding(ctx, runtime, committedHeight, block.Header); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	// A certified header whose app hash differs from the local state means this node's state
	// diverged from the quorum's: it stops here rather than build on a state no one else has. Below
	// the snapshot it was restored from, or its retention, a node compares with the app hash the
	// chain certified for the state (runtime.HeaderAppHashAt); a state no certified block named
	// leaves nothing to compare.
	if err := checkHeaderAppHash(ctx, runtime, block.Header); err != nil && !errors.Is(err, vexoruntime.ErrAppHashUnavailable) {
		return app.FinalizeBlockResponse{}, err
	}
	if requireLocalQC {
		localQC, err := machine.BuildQuorumCert(quorumCert.Height, quorumCert.Round, quorumCert.BlockHash)
		if err != nil {
			return app.FinalizeBlockResponse{}, err
		}
		quorumCert = localQC
	} else if err := node.verifyCommitCertificate(ctx, block, quorumCert); err != nil {
		return app.FinalizeBlockResponse{}, err
	}

	response, err := runtime.ExecuteCertifiedBlock(ctx, block, quorumCert)
	if err != nil {
		if strings.Contains(err.Error(), "invalid transaction nonce") {
			node.removeNonceInvalidProposalTxs(block, quorumCert.Round)
		}
		return app.FinalizeBlockResponse{}, err
	}
	// The block is durable from here on. Everything that follows is repeated by reconcileCommitted
	// when it fails, because the block can never be committed twice.
	node.mu.Lock()
	node.committedHeader = new(block.Header)
	node.mu.Unlock()
	var commitProof finality.Proof
	if broadcast {
		proof, err := node.buildCommitFinalityProof(ctx, runtime, block, quorumCert)
		if err != nil && !errors.Is(err, ErrFinalityNotFound) && !errors.Is(err, store.ErrFinalityNotFound) {
			return app.FinalizeBlockResponse{}, err
		}
		if err == nil {
			commitProof = proof
		}
	}
	if err := node.finishCommit(ctx, runtime, machine, block, &quorumCert); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	node.logEvent("block_committed", map[string]any{
		"chain_id":   block.Header.ChainID,
		"height":     block.Header.Height,
		"round":      quorumCert.Round,
		"block_hash": fmt.Sprintf("%x", blockHash),
		"app_hash":   fmt.Sprintf("%x", response.AppHash),
		"tx_count":   len(block.Txs),
	})
	if broadcast {
		if err := node.broadcastCommit(ctx, block, quorumCert, commitProof); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	node.metrics.observeCommitLatency(time.Since(started))
	return response, nil
}

// persistFinalityDecisions stores the finality proof of every commit decision the machine holds. A
// high-water mark records the height up to which every decision is stored, so a commit does not
// rebuild the proofs of the decisions it already stored. committed, when not nil, is the block the
// caller just made durable; its proof is built from it instead of reading it back.
func (node *Node) persistFinalityDecisions(ctx context.Context, runtime *vexoruntime.Runtime, machine *consensus.StateMachine, committed *types.Block) error {
	proofStore, ok := runtime.Store.(store.FinalityProofStore)
	if !ok || proofStore == nil {
		return nil
	}
	decisions := machine.CommitDecisions()
	sort.Slice(decisions, func(left, right int) bool {
		return decisions[left].CommittedHeight < decisions[right].CommittedHeight
	})
	node.mu.Lock()
	persisted := node.finalityPersisted
	node.mu.Unlock()
	contiguous := true
	for _, decision := range decisions {
		if decision.CommittedHeight <= persisted {
			continue
		}
		stored, err := node.persistFinalityDecision(ctx, runtime, proofStore, decision, committed)
		if errors.Is(err, store.ErrBlockNotFound) {
			// The block is not executed yet, and neither is any block above it.
			break
		}
		if err != nil {
			return err
		}
		if !stored {
			// The child of the block is not known yet. The decision is retried by the next commit.
			contiguous = false
			continue
		}
		if contiguous {
			node.mu.Lock()
			node.finalityPersisted = decision.CommittedHeight
			node.mu.Unlock()
		}
	}
	return nil
}

// persistFinalityDecision stores the proof of one decision. It reports false when the child that
// completes the proof is not known yet.
func (node *Node) persistFinalityDecision(ctx context.Context, runtime *vexoruntime.Runtime, proofStore store.FinalityProofStore, decision consensus.CommitDecision, committed *types.Block) (bool, error) {
	var proof finality.Proof
	var err error
	if committed != nil && committed.Header.Height == decision.CommittedHeight {
		proof, err = vexoruntime.NewFinalityProof(committed.Header, consensus.HashBlock(*committed), decision.CommitQC)
	} else {
		proof, err = runtime.FinalityProof(ctx, decision.CommittedHeight, decision.CommitQC)
	}
	if errors.Is(err, store.ErrFinalityNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	proof, err = node.attachCommitChainProof(ctx, runtime, proof)
	if errors.Is(err, ErrFinalityNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	record := finalityProofRecord(proof)
	stored, err := proofStore.FinalityProof(ctx, decision.CommittedHeight)
	if err == nil && reflect.DeepEqual(stored, record) {
		return true, nil
	}
	if err != nil && !errors.Is(err, store.ErrFinalityNotFound) {
		return false, err
	}
	if err := proofStore.SaveFinalityProof(ctx, record); err != nil {
		return false, err
	}
	return true, nil
}

func (node *Node) buildCommitFinalityProof(ctx context.Context, runtime *vexoruntime.Runtime, block types.Block, quorumCert finality.QuorumCert) (finality.Proof, error) {
	proof, err := vexoruntime.NewFinalityProof(block.Header, consensus.HashBlock(block), quorumCert)
	if err != nil {
		return finality.Proof{}, err
	}
	return node.attachCommitChainProof(ctx, runtime, proof)
}

// attachCommitChainProof completes proof, which proves block B with QC(B), with the single child link
// of the 2-chain commit rule: the child C of B and QC(C), the certificate formed in the view right
// after the view of QC(B). It returns ErrFinalityNotFound while the child is not known, or when no
// child certified in that view exists.
func (node *Node) attachCommitChainProof(ctx context.Context, runtime *vexoruntime.Runtime, proof finality.Proof) (finality.Proof, error) {
	if machine, err := node.Consensus(); err == nil {
		pending := node.pendingProposals()
		if child, found := findCommitChild(pending, proof.Header, proof.BlockHash, proof.QuorumCert.Round); found {
			if childQC, found := childQuorumCert(ctx, machine, pending, child); found {
				return proof.WithChild(child.Block.Header, childQC), nil
			}
		}
	}
	return attachCommitChainProofFromStore(ctx, runtime, proof)
}

// attachCommitChainProofFromStore completes proof with the child that is already executed: its block
// and the certificate the store keeps for it.
func attachCommitChainProofFromStore(ctx context.Context, runtime *vexoruntime.Runtime, proof finality.Proof) (finality.Proof, error) {
	if runtime == nil {
		return finality.Proof{}, ErrFinalityNotFound
	}
	childHeight, ok := nextHeightAfter(proof.Header.Height)
	if !ok {
		return finality.Proof{}, ErrFinalityNotFound
	}
	record, err := runtime.BlockByHeight(ctx, childHeight)
	if err != nil {
		if errors.Is(err, store.ErrBlockNotFound) {
			return finality.Proof{}, ErrFinalityNotFound
		}
		return finality.Proof{}, err
	}
	if record.Hash == (types.Hash{}) ||
		record.Block.Header.ChainID != proof.Header.ChainID ||
		record.Block.Header.Height != childHeight ||
		record.Block.Header.PreviousBlockHash != proof.BlockHash ||
		record.QuorumCert == nil {
		return finality.Proof{}, ErrFinalityNotFound
	}
	childQC := finalityQuorumCertFromRecord(*record.QuorumCert)
	if childQC.Height != childHeight || childQC.BlockHash != record.Hash || !consecutiveViews(proof.QuorumCert, childQC) {
		return finality.Proof{}, ErrFinalityNotFound
	}
	return proof.WithChild(record.Block.Header, childQC), nil
}

// consecutiveViews reports whether childQC was formed in the view right after the view of blockQC,
// which is what makes the child a witness that the block is final.
func consecutiveViews(blockQC finality.QuorumCert, childQC finality.QuorumCert) bool {
	return blockQC.Round != ^types.Round(0) && childQC.Round == blockQC.Round+1
}

// findCommitChild returns the pending proposal that is the child C of the block with the given header
// and hash: it extends the block by one height, names it as its parent, and was proposed in the view
// right after the view of the block's certificate. A view has one proposer, so at most one pending
// proposal matches.
func findCommitChild(pending map[types.Hash]consensus.Proposal, parentHeader types.Header, parentHash types.Hash, blockQCRound types.Round) (consensus.Proposal, bool) {
	childHeight, ok := nextHeightAfter(parentHeader.Height)
	if !ok || blockQCRound == ^types.Round(0) {
		return consensus.Proposal{}, false
	}
	for _, proposal := range pending {
		header := proposal.Block.Header
		if header.ChainID != parentHeader.ChainID ||
			header.Height != childHeight ||
			header.PreviousBlockHash != parentHash ||
			proposal.JustifyQC.Height != parentHeader.Height ||
			proposal.JustifyQC.BlockHash != parentHash ||
			proposal.Round != blockQCRound+1 {
			continue
		}
		return proposal, true
	}
	return consensus.Proposal{}, false
}

// childQuorumCert returns QC(C) of the child proposal: the certificate a pending proposal that extends
// C carries as its justification, the highest certificate of the machine, or one built from the votes
// the machine holds.
func childQuorumCert(ctx context.Context, machine *consensus.StateMachine, pending map[types.Hash]consensus.Proposal, child consensus.Proposal) (finality.QuorumCert, bool) {
	childHash := consensus.HashBlock(child.Block)
	height := child.Block.Header.Height
	for _, proposal := range pending {
		if proposal.JustifyQC.Height == height && proposal.JustifyQC.BlockHash == childHash && proposal.JustifyQC.Round == child.Round {
			return proposal.JustifyQC, true
		}
	}
	if highQC := machine.HighQC(ctx); highQC.Height == height && highQC.BlockHash == childHash && highQC.Round == child.Round {
		return highQC, true
	}
	if built, err := machine.BuildQuorumCert(height, child.Round, childHash); err == nil {
		return built, true
	}
	return finality.QuorumCert{}, false
}

type CommitReadyResult struct {
	Block      types.Block
	BlockHash  types.Hash
	QuorumCert finality.QuorumCert
	Response   app.FinalizeBlockResponse
}

func (node *Node) CommitReadyBlock(ctx context.Context) (CommitReadyResult, bool, error) {
	return CommitReadyResult{}, false, ErrUnsafeQCCommit
}

func (node *Node) UnsafeCommitReadyBlock(ctx context.Context) (CommitReadyResult, bool, error) {
	machine, err := node.Consensus()
	if err != nil {
		return CommitReadyResult{}, false, err
	}
	// The machine leaves a height as soon as a certificate for it forms, so the block to execute
	// next is judged against the committed height, not against the height of the current view.
	committed, ok := node.currentCommitHeight(ctx)
	if !ok {
		return CommitReadyResult{}, false, ErrNodeNotRunning
	}
	next, ok := nextHeightAfter(committed)
	if !ok {
		return CommitReadyResult{}, false, nil
	}
	for blockHash, proposal := range node.pendingProposals() {
		if proposal.Block.Header.Height < next {
			node.removePending(blockHash)
			continue
		}
		if proposal.Block.Header.Height > next {
			continue
		}
		qc, err := machine.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash)
		if err != nil {
			continue
		}
		response, err := node.CommitBlock(ctx, proposal.Block, qc)
		if err != nil {
			return CommitReadyResult{}, false, err
		}
		return CommitReadyResult{
			Block:      proposal.Block,
			BlockHash:  blockHash,
			QuorumCert: qc,
			Response:   response,
		}, true, nil
	}
	return CommitReadyResult{}, false, nil
}

func (node *Node) CommitFinalizedBlock(ctx context.Context) (CommitReadyResult, bool, error) {
	return node.commitFinalizedBlock(ctx, node.recoveryFinalityGateEnabled())
}

// commitFinalizedBlock executes the next block the 2-chain commit rule finalized. Only the block right
// above the committed height can be executed, so every other decision is skipped without being
// examined. With the recovery gate on, the recovery report is consulted, once per committed height.
func (node *Node) commitFinalizedBlock(ctx context.Context, recoveryGate bool) (CommitReadyResult, bool, error) {
	machine, err := node.Consensus()
	if err != nil {
		return CommitReadyResult{}, false, err
	}
	decisions := machine.CommitDecisions()
	if len(decisions) == 0 {
		return CommitReadyResult{}, false, nil
	}
	committed, ok := node.currentCommitHeight(ctx)
	if !ok {
		return CommitReadyResult{}, false, ErrNodeNotRunning
	}
	next, ok := nextHeightAfter(committed)
	if !ok {
		return CommitReadyResult{}, false, nil
	}
	pending := node.pendingProposals()
	for _, decision := range decisions {
		if decision.CommittedHeight <= committed {
			node.removePending(decision.CommittedBlockHash)
			continue
		}
		if decision.CommittedHeight != next {
			continue
		}
		if recoveryGate {
			report, reportErr := node.recoveryReportAt(ctx, committed)
			if reportErr == nil && !recoveryFinalityAllowsCommit(report, decision.CommittedHeight) {
				node.forgetRecoveryReport()
				node.metrics.observeRecoveryFinalityDeferral()
				node.logEvent("recovery_finality_deferred", map[string]any{
					"height":      decision.CommittedHeight,
					"safe_height": report.SafeHeight,
				})
				continue
			}
		}
		proposal, found := pending[decision.CommittedBlockHash]
		if !found {
			continue
		}
		if proposal.Block.Header.Height != decision.CommittedHeight {
			continue
		}
		response, err := node.commitBlock(ctx, proposal.Block, decision.CommitQC, false, true)
		if errors.Is(err, ErrBlockAlreadyCommitted) {
			node.removePending(decision.CommittedBlockHash)
			continue
		}
		if err != nil {
			return CommitReadyResult{}, false, err
		}
		return CommitReadyResult{
			Block:      proposal.Block,
			BlockHash:  decision.CommittedBlockHash,
			QuorumCert: decision.CommitQC,
			Response:   response,
		}, true, nil
	}
	return CommitReadyResult{}, false, nil
}

func (node *Node) cacheProposal(proposal consensus.Proposal, blockHash types.Hash) {
	if node.hasCommitted(context.Background(), proposal.Block.Header.Height) {
		return
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.pending == nil {
		node.pending = make(map[types.Hash]consensus.Proposal)
	}
	node.pending[blockHash] = proposal
}

func (node *Node) removePending(blockHash types.Hash) {
	node.mu.Lock()
	defer node.mu.Unlock()
	delete(node.pending, blockHash)
}

func (node *Node) removePendingAtOrBelow(height types.Height) {
	node.mu.Lock()
	defer node.mu.Unlock()
	for blockHash, proposal := range node.pending {
		if proposal.Block.Header.Height <= height {
			delete(node.pending, blockHash)
		}
	}
}

func (node *Node) hasProposed(height types.Height, round types.Round) bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	_, ok := node.proposed[proposalRound{height: height, round: round}]
	return ok
}

// markProposed records that this node proposed, and broadcast, in round at height now.
func (node *Node) markProposed(height types.Height, round types.Round) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.proposed == nil {
		node.proposed = make(map[proposalRound]time.Time)
	}
	node.proposed[proposalRound{height: height, round: round}] = time.Now()
}

// proposalRetransmitDue reports whether the proposal of round at height was last broadcast at
// least interval ago, and if so restarts its interval.
func (node *Node) proposalRetransmitDue(height types.Height, round types.Round, interval time.Duration) bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	key := proposalRound{height: height, round: round}
	broadcast, ok := node.proposed[key]
	if !ok || time.Since(broadcast) < interval {
		return false
	}
	node.proposed[key] = time.Now()
	return true
}

func (node *Node) removeProposedAtOrBelow(height types.Height) {
	node.mu.Lock()
	defer node.mu.Unlock()
	for key := range node.proposed {
		if key.height <= height {
			delete(node.proposed, key)
		}
	}
}

func (node *Node) removeProposed(height types.Height, round types.Round) {
	node.mu.Lock()
	defer node.mu.Unlock()
	delete(node.proposed, proposalRound{height: height, round: round})
}

func (node *Node) cachedTimeoutVote(height types.Height, round types.Round) (consensus.TimeoutVote, bool) {
	node.mu.Lock()
	defer node.mu.Unlock()
	vote, ok := node.timeoutVotes[proposalRound{height: height, round: round}]
	return vote, ok
}

func (node *Node) cacheTimeoutVote(vote consensus.TimeoutVote) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.timeoutVotes == nil {
		node.timeoutVotes = make(map[proposalRound]consensus.TimeoutVote)
	}
	node.timeoutVotes[proposalRound{height: vote.Height, round: vote.Round}] = vote
}

func (node *Node) removeTimeoutVotesAtOrBelow(height types.Height) {
	node.mu.Lock()
	defer node.mu.Unlock()
	for key := range node.timeoutVotes {
		if key.height <= height {
			delete(node.timeoutVotes, key)
		}
	}
}

func (node *Node) logEvent(event string, fields map[string]any) {
	node.mu.Lock()
	logger := node.eventLogger
	node.mu.Unlock()
	if logger != nil {
		logger(event, fields)
	}
}

func (node *Node) pendingProposals() map[types.Hash]consensus.Proposal {
	node.mu.Lock()
	defer node.mu.Unlock()
	proposals := make(map[types.Hash]consensus.Proposal, len(node.pending))
	for blockHash, proposal := range node.pending {
		proposals[blockHash] = proposal
	}
	return proposals
}

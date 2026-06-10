package node

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
)

type autoVoteReactor struct {
	machine            *consensus.StateMachine
	chainID            string
	validatorID        types.ValidatorID
	signer             vexocrypto.Signer
	broadcastVote      func(context.Context, consensus.Vote) error
	onProposalAccepted func(consensus.Proposal, types.Hash)
	onVoteAccepted     func(context.Context)
	onEvidence         func(context.Context, slashing.Evidence)
	onError            func(string, error)
	onProposalLatency  func(time.Duration)
	onVoteLatency      func(time.Duration)
	onSigningFailure   func()
	wal                *consensus.WAL
	mu                 sync.Mutex
	pendingVotes       map[types.Hash][]consensus.Vote
	localVotes         map[voteRound]consensus.Vote
	unknownVotes       map[unknownVoteKey]consensus.Vote
}

type voteRound struct {
	height    types.Height
	round     types.Round
	blockHash types.Hash
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
	before := len(reactor.machine.Evidence())
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
		reactor.reportError("proposal_rejected", err)
		return err
	}
	blockHash := consensus.HashBlock(proposal.Block)
	if reactor.onProposalAccepted != nil {
		reactor.onProposalAccepted(proposal, blockHash)
	}
	vote, cached := reactor.cachedLocalVote(proposal.Block.Header.Height, proposal.Round, blockHash)
	if !cached {
		vote = consensus.Vote{
			Height:      proposal.Block.Header.Height,
			Round:       proposal.Round,
			BlockHash:   blockHash,
			ValidatorID: reactor.validatorID,
		}
		if err := signConsensusVote(reactor.chainID, reactor.signer, &vote); err != nil {
			if reactor.onSigningFailure != nil {
				reactor.onSigningFailure()
			}
			reactor.reportError("vote_sign_failed", err)
			return err
		}
		if reactor.wal != nil {
			if err := reactor.wal.RecordVote(vote); err != nil {
				reactor.reportError("vote_wal_failed", err)
				return err
			}
		}
		reactor.cacheLocalVote(vote)
	}
	if err := reactor.machine.OnVote(ctx, vote); err != nil {
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
		return err
	}
	return nil
}

func (reactor *autoVoteReactor) cachedLocalVote(height types.Height, round types.Round, blockHash types.Hash) (consensus.Vote, bool) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	vote, ok := reactor.localVotes[voteRound{height: height, round: round, blockHash: blockHash}]
	return vote, ok
}

func (reactor *autoVoteReactor) cacheLocalVote(vote consensus.Vote) {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	if reactor.localVotes == nil {
		reactor.localVotes = make(map[voteRound]consensus.Vote)
	}
	reactor.localVotes[voteRound{height: vote.Height, round: vote.Round, blockHash: vote.BlockHash}] = vote
}

func (reactor *autoVoteReactor) OnVote(ctx context.Context, vote consensus.Vote) error {
	started := time.Now()
	defer func() {
		if reactor.onVoteLatency != nil {
			reactor.onVoteLatency(time.Since(started))
		}
	}()
	before := len(reactor.machine.Evidence())
	err := reactor.machine.OnVote(ctx, vote)
	if errors.Is(err, consensus.ErrUnknownVoteBlock) {
		reactor.publishUnknownVoteEvidence(ctx, vote)
		reactor.cachePendingVote(vote)
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
		reactor.unknownVotes[key] = vote
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
	reactor.pendingVotes[vote.BlockHash] = append(votes, vote)
}

func (reactor *autoVoteReactor) replayPendingVotes(ctx context.Context, blockHash types.Hash) {
	votes := reactor.takePendingVotes(blockHash)
	for _, vote := range votes {
		before := len(reactor.machine.Evidence())
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

func (reactor *autoVoteReactor) publishNewEvidence(ctx context.Context, previousCount int) {
	if reactor.onEvidence == nil {
		return
	}
	evidence := reactor.machine.Evidence()
	if previousCount >= len(evidence) {
		return
	}
	for _, item := range evidence[previousCount:] {
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
		round := status.Round
		if status.Height != height {
			round = 0
		}

		proposal, err := machine.CreateProposal(candidateBlock, round, node.cfg.ValidatorID, finality.QuorumCert{})
		if err != nil {
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
		vote, err := node.voteLocalProposal(ctx, proposal, blockHash)
		if err != nil {
			if errors.Is(err, consensus.ErrStaleVote) {
				lastErr = err
				continue
			}
			return consensus.Proposal{}, types.Hash{}, err
		}
		if err := node.broadcastAncestorProposals(ctx, reactor, proposal.Block.Header.Height); err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
		if err := reactor.BroadcastProposal(ctx, proposal); err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
		if err := reactor.BroadcastVote(ctx, vote); err != nil {
			return consensus.Proposal{}, types.Hash{}, err
		}
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

func (node *Node) broadcastAncestorProposals(ctx context.Context, reactor *consensus.TransportReactor, beforeHeight types.Height) error {
	if reactor == nil || beforeHeight <= 1 {
		return nil
	}
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
		if err := reactor.BroadcastProposal(ctx, proposal); err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) voteLocalProposal(ctx context.Context, proposal consensus.Proposal, blockHash types.Hash) (consensus.Vote, error) {
	vote := consensus.Vote{
		Height:      proposal.Block.Header.Height,
		Round:       proposal.Round,
		BlockHash:   blockHash,
		ValidatorID: node.cfg.ValidatorID,
	}
	if err := node.signConsensusVote(&vote); err != nil {
		return consensus.Vote{}, err
	}
	if err := node.recordConsensusVote(vote); err != nil {
		return consensus.Vote{}, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return consensus.Vote{}, err
	}
	if err := machine.OnVote(ctx, vote); err != nil {
		return consensus.Vote{}, err
	}
	return vote, nil
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

	vote := consensus.Vote{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		ValidatorID: node.cfg.ValidatorID,
	}
	if err := node.signConsensusVote(&vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
	if err := node.recordConsensusVote(vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
	if err := machine.OnVote(ctx, vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
	if err := reactor.BroadcastVote(ctx, vote); err != nil {
		return finality.QuorumCert{}, false, err
	}
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
		if err := reactor.BroadcastTimeoutVote(ctx, vote); err != nil {
			return finality.TimeoutCert{}, false, err
		}
		return finality.TimeoutCert{}, false, nil
	}
	vote := consensus.TimeoutVote{
		Height:      status.Height,
		Round:       status.Round,
		ValidatorID: node.cfg.ValidatorID,
		HighQC:      machine.HighQC(ctx),
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
	if err := reactor.BroadcastTimeoutVote(ctx, vote); err != nil {
		return finality.TimeoutCert{}, false, err
	}
	if err != nil {
		return finality.TimeoutCert{}, false, nil
	}
	node.metrics.observeVoteLatency(time.Since(started))
	return timeoutCert, true, nil
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
		return nil
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
		return nil
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
		return nil
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

func (node *Node) commitBlock(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, requireLocalQC bool, broadcast bool) (app.FinalizeBlockResponse, error) {
	started := time.Now()
	if quorumCert.Height != block.Header.Height {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: height mismatch", ErrInvalidCommitQC)
	}
	blockHash := consensus.HashBlock(block)
	if quorumCert.BlockHash != blockHash {
		return app.FinalizeBlockResponse{}, fmt.Errorf("%w: block hash mismatch", ErrInvalidCommitQC)
	}

	runtime, err := node.Runtime()
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if requireLocalQC {
		if _, err := machine.BuildQuorumCert(quorumCert.Height, quorumCert.Round, quorumCert.BlockHash); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	} else if err := node.verifyCommitCertificate(ctx, block, quorumCert); err != nil {
		return app.FinalizeBlockResponse{}, err
	}

	response, err := runtime.ExecuteBlock(ctx, block)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
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
	if err := runtime.Mempool.MarkCommitted(ctx, block.Txs); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if len(block.Txs) > 0 {
		if err := runtime.Mempool.CompactWAL(ctx); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	if err := machine.ObserveCommittedBlock(block, quorumCert); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if err := node.persistFinalityDecisions(ctx, runtime, machine); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	nextHeight := block.Header.Height + 1
	if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, nextHeight); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	machine.StartRound(nextHeight, 0)
	node.logEvent("block_committed", map[string]any{
		"chain_id":   block.Header.ChainID,
		"height":     block.Header.Height,
		"round":      quorumCert.Round,
		"block_hash": fmt.Sprintf("%x", consensus.HashBlock(block)),
		"app_hash":   fmt.Sprintf("%x", response.AppHash),
		"tx_count":   len(block.Txs),
	})
	node.removePendingAtOrBelow(block.Header.Height)
	node.removeProposedAtOrBelow(block.Header.Height)
	node.removeTimeoutVotesAtOrBelow(block.Header.Height)
	if broadcast {
		if err := node.broadcastCommit(ctx, block, quorumCert, commitProof); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	node.metrics.observeCommitLatency(time.Since(started))
	return response, nil
}

func (node *Node) persistFinalityDecisions(ctx context.Context, runtime *vexoruntime.Runtime, machine *consensus.StateMachine) error {
	proofStore, ok := runtime.Store.(store.FinalityProofStore)
	if !ok || proofStore == nil {
		return nil
	}
	for _, decision := range machine.CommitDecisions() {
		proof, err := runtime.FinalityProof(ctx, decision.CommittedHeight, decision.CommitQC)
		if errors.Is(err, store.ErrFinalityNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if enriched, err := node.attachCommitChainProof(proof); err == nil {
			proof = enriched
		} else if !errors.Is(err, ErrFinalityNotFound) {
			return err
		}
		if err := proofStore.SaveFinalityProof(ctx, finalityProofRecord(proof)); err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) buildCommitFinalityProof(ctx context.Context, runtime *vexoruntime.Runtime, block types.Block, quorumCert finality.QuorumCert) (finality.Proof, error) {
	proof, err := runtime.FinalityProof(ctx, block.Header.Height, quorumCert)
	if err != nil {
		return finality.Proof{}, err
	}
	return node.attachCommitChainProof(proof)
}

func (node *Node) attachCommitChainProof(proof finality.Proof) (finality.Proof, error) {
	pending := node.pendingProposals()
	first, found := findCommitChild(pending, proof.Header, proof.BlockHash)
	if !found {
		return finality.Proof{}, ErrFinalityNotFound
	}
	second, found := findCommitChild(pending, first.Header, first.BlockHash)
	if !found {
		return finality.Proof{}, ErrFinalityNotFound
	}
	proof.CommitChain = []finality.CommitLink{first, second}
	return proof, nil
}

func findCommitChild(pending map[types.Hash]consensus.Proposal, parentHeader types.Header, parentHash types.Hash) (finality.CommitLink, bool) {
	for childHash, proposal := range pending {
		if proposal.Block.Header.ChainID != parentHeader.ChainID {
			continue
		}
		if proposal.Block.Header.Height != parentHeader.Height+1 {
			continue
		}
		if proposal.Block.Header.PreviousBlockHash != parentHash {
			continue
		}
		if proposal.JustifyQC.Height != parentHeader.Height {
			continue
		}
		if proposal.JustifyQC.BlockHash != parentHash {
			continue
		}
		return finality.CommitLink{
			Header:     proposal.Block.Header,
			BlockHash:  childHash,
			QuorumCert: proposal.JustifyQC,
		}, true
	}
	return finality.CommitLink{}, false
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
	status := machine.Status(ctx)
	for blockHash, proposal := range node.pendingProposals() {
		if proposal.Block.Header.Height < status.Height {
			node.removePending(blockHash)
			continue
		}
		if proposal.Block.Header.Height > status.Height {
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
	machine, err := node.Consensus()
	if err != nil {
		return CommitReadyResult{}, false, err
	}
	decisions := machine.CommitDecisions()
	if len(decisions) == 0 {
		return CommitReadyResult{}, false, nil
	}
	pending := node.pendingProposals()
	for _, decision := range decisions {
		proposal, found := pending[decision.CommittedBlockHash]
		if !found {
			continue
		}
		if proposal.Block.Header.Height != decision.CommittedHeight {
			continue
		}
		response, err := node.commitBlock(ctx, proposal.Block, decision.CommitQC, false, true)
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

func (node *Node) markProposed(height types.Height, round types.Round) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.proposed == nil {
		node.proposed = make(map[proposalRound]struct{})
	}
	node.proposed[proposalRound{height: height, round: round}] = struct{}{}
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

package node

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// maxConsensusHeightJump bounds how far above the first uncommitted height a round may start. It
// matches the height window in which the consensus machine accepts blocks and votes.
const maxConsensusHeightJump types.Height = 256

// newConsensusMachine builds the consensus machine of a node whose last committed block is at
// startHeight-1. The machine verifies every message with the runtime's crypto and never with the
// node's own signer, so a full node, which has no signer, verifies exactly like a validator. The last
// committed block anchors the machine: it fixes the finalized height the acceptance window is
// measured from and restores the lock the certificate of that block gives. anchored reports whether
// the store held that certificate.
func (node *Node) newConsensusMachine(ctx context.Context, runtime *vexoruntime.Runtime, startHeight types.Height) (machine *consensus.StateMachine, anchored bool, err error) {
	committed := startHeight - 1
	if committed == 0 {
		machine, err = runtime.NewConsensusStateMachine(ctx, startHeight)
		if err != nil {
			return nil, false, err
		}
		// The machine keeps the registry history from the set of the first block on.
		if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, startHeight); err != nil {
			return nil, false, err
		}
		return machine, false, nil
	}
	machine, err = runtime.NewConsensusStateMachine(ctx, committed)
	if err != nil {
		return nil, false, err
	}
	record, err := runtime.BlockByHeight(ctx, committed)
	if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
		return nil, false, err
	}
	found := err == nil
	if found {
		// The certificate of the committed block was formed by the validator set its header
		// names; the machine holds the registry history from that height on.
		setHeight := record.Block.Header.ValidatorSetHeight
		if setHeight == 0 || setHeight > committed {
			return nil, false, fmt.Errorf("committed block %d names validator set height %d", committed, setHeight)
		}
		if err := runtime.RegisterValidatorsAt(ctx, setHeight); err != nil {
			return nil, false, err
		}
		if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, setHeight); err != nil {
			return nil, false, err
		}
	}
	if found && record.QuorumCert != nil {
		observeErr := machine.ObserveCommittedBlock(record.Block, finalityQuorumCertFromRecord(*record.QuorumCert))
		switch {
		case observeErr == nil:
			anchored = true
		case node.cfg.ValidatorID != "":
			return nil, false, fmt.Errorf("anchor consensus at committed height %d: %w", committed, observeErr)
		}
	}
	switch {
	case anchored:
	case found:
		// Without a certificate the committed block is still the root the next block extends,
		// and its header fixes the validator set of that block.
		if err := machine.AnchorCommittedBlock(record.Block); err != nil {
			return nil, false, fmt.Errorf("anchor consensus at committed height %d: %w", committed, err)
		}
	default:
		machine.Prune(committed)
	}
	if err := runtime.RegisterValidatorsAt(ctx, startHeight); err != nil {
		return nil, false, err
	}
	if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, startHeight); err != nil {
		return nil, false, err
	}
	return machine, anchored, nil
}

// restoreSafetyState reinstates the safety state the validator persisted before its last vote.
// When the machine is anchored at the certificate of the committed block, certificates below the
// committed height are dropped: that certificate supersedes them, and they predate the validator
// sets the machine holds.
func restoreSafetyState(machine *consensus.StateMachine, wal *consensus.WAL, committedHeight types.Height, anchored bool) error {
	state, found, err := wal.SafetyState()
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if anchored {
		if state.LockedQC.Height < committedHeight {
			state.LockedQC = finality.QuorumCert{}
		}
		if state.HighQC.Height < committedHeight {
			state.HighQC = finality.QuorumCert{}
		}
	}
	return machine.RestoreSafetyState(state)
}

// recordTimeoutSafety makes the validator give up voting in the view it times out in, and persists
// that before the timeout vote is signed. A validator that timed out in a view and then voted in it
// would let a certificate form that the timeout certificate of the view does not account for.
func (node *Node) recordTimeoutSafety(machine *consensus.StateMachine, vote consensus.TimeoutVote) error {
	state := machine.SafetyState()
	if state.LastVotedHeight == 0 || vote.Round > state.LastVotedRound {
		if vote.Height > state.LastVotedHeight {
			state.LastVotedHeight = vote.Height
		}
		if vote.Round > state.LastVotedRound {
			state.LastVotedRound = vote.Round
		}
		if err := machine.RestoreSafetyState(state); err != nil {
			return err
		}
	}
	node.mu.Lock()
	wal := node.consensusWAL
	node.mu.Unlock()
	if wal == nil {
		return nil
	}
	return wal.RecordSafetyState(machine.SafetyState())
}

// nextHeightAfter returns height+1, or false when it would overflow.
func nextHeightAfter(height types.Height) (types.Height, bool) {
	if height == ^types.Height(0) {
		return 0, false
	}
	return height + 1, true
}

// boundedNextHeight returns the height a new round starts at once the block at committed is
// executed: the height after the highest certified block, but never below the first uncommitted
// height and never more than maxConsensusHeightJump above it. A certificate far ahead of the
// committed chain, or at the end of the height range, must not move the pacemaker out of the window
// the machine accepts blocks in, because the pacemaker never moves backwards.
func boundedNextHeight(committed types.Height, certified types.Height) types.Height {
	first, ok := nextHeightAfter(committed)
	if !ok {
		return committed
	}
	limit := first + maxConsensusHeightJump
	if limit < first {
		limit = ^types.Height(0)
	}
	if certified < first {
		return first
	}
	if certified >= limit {
		return limit
	}
	return certified + 1
}

// reconcileCommitted completes, idempotently, the steps that follow the durable execution of the last
// committed block. A failure after execution (a full disk, a cancelled context) skips them, and the
// block can never be committed again, so every consensus step first makes sure the machine, the
// validator set, the mempool and the finality records have caught up with the store.
func (node *Node) reconcileCommitted(ctx context.Context) error {
	committed, ok := node.currentCommitHeight(ctx)
	if !ok || committed == 0 || node.isReconciled(committed) {
		return nil
	}
	node.commitMu.Lock()
	defer node.commitMu.Unlock()
	node.admissionMu.Lock()
	defer node.admissionMu.Unlock()

	// A commit that ran while this step waited for the locks has finished the work itself.
	committed, ok = node.currentCommitHeight(ctx)
	if !ok || committed == 0 || node.isReconciled(committed) {
		return nil
	}
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	machine, err := node.Consensus()
	if err != nil {
		return err
	}
	record, err := runtime.BlockByHeight(ctx, committed)
	if err != nil {
		return fmt.Errorf("reconcile committed block %d: %w", committed, err)
	}
	var quorumCert *finality.QuorumCert
	if record.QuorumCert != nil {
		cert := finalityQuorumCertFromRecord(*record.QuorumCert)
		quorumCert = &cert
	}
	return node.finishCommit(ctx, runtime, machine, record.Block, quorumCert)
}

func (node *Node) isReconciled(committed types.Height) bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	return node.reconciledHeight == committed
}

// finishCommit performs everything that follows the durable execution of block, and does it
// idempotently so reconcileCommitted can repeat it after a failure. quorumCert is nil for a block
// that was executed without a certificate.
func (node *Node) finishCommit(ctx context.Context, runtime *vexoruntime.Runtime, machine *consensus.StateMachine, block types.Block, quorumCert *finality.QuorumCert) error {
	height := block.Header.Height
	if err := runtime.Mempool.MarkCommitted(ctx, block.Txs); err != nil {
		return err
	}
	if len(block.Txs) > 0 {
		if err := runtime.Mempool.CompactWAL(ctx); err != nil {
			return err
		}
	}
	if quorumCert != nil {
		if err := machine.ObserveCommittedBlock(block, *quorumCert); err != nil {
			return err
		}
	}
	machine.Prune(height)
	if err := node.persistFinalityDecisions(ctx, runtime, machine); err != nil {
		return err
	}
	nextHeight, ok := nextHeightAfter(height)
	if !ok {
		return fmt.Errorf("committed height %d leaves no next height", height)
	}
	if err := runtime.RegisterValidatorsAt(ctx, nextHeight); err != nil {
		return err
	}
	if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, nextHeight); err != nil {
		return err
	}
	machine.StartRound(boundedNextHeight(height, machine.HighQC(ctx).Height), 0)
	node.removePendingAtOrBelow(height)
	node.removeProposedAtOrBelow(height)
	node.removeTimeoutVotesAtOrBelow(height)
	node.pruneVoteCaches(height)
	node.mu.Lock()
	node.reconciledHeight = height
	node.mu.Unlock()
	return nil
}

// recoveryGate caches the recovery report the finality gate consults, one per committed height.
type recoveryGate struct {
	height types.Height
	cached bool
	report RecoveryReport
	// computed counts the reports built, so tests can tell a cached report from a recomputed one.
	computed uint64
}

// recoveryReportAt returns the recovery report for the node's committed height. The report reads
// the latest state, the block index and the snapshot head, so it is kept for the committed height and
// not recomputed on every consensus step.
func (node *Node) recoveryReportAt(ctx context.Context, committed types.Height) (RecoveryReport, error) {
	node.mu.Lock()
	if node.recoveryGate.cached && node.recoveryGate.height == committed {
		report := node.recoveryGate.report
		node.mu.Unlock()
		return report, nil
	}
	node.mu.Unlock()

	report, err := node.RecoveryReport(ctx, false)

	node.mu.Lock()
	node.recoveryGate.computed++
	node.recoveryGate.cached = err == nil
	if err == nil {
		node.recoveryGate.height = committed
		node.recoveryGate.report = report
	}
	node.mu.Unlock()
	return report, err
}

// forgetRecoveryReport drops the kept report. A gate that defers a commit does so until the next
// commit would replace the report, and the problem it defers on can heal without one, so the next
// step must judge a fresh report.
func (node *Node) forgetRecoveryReport() {
	node.mu.Lock()
	node.recoveryGate.cached = false
	node.mu.Unlock()
}

func (node *Node) recoveryFinalityGateEnabled() bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	return node.loopConfig.RecoveryFinalityGateEnabled
}

// processProposal asks the application whether it accepts the block of proposal, and checks that a
// block built directly on the committed chain extends the last committed block. A validator votes for
// a proposal only when this passes: a vote is a promise that the block is executable.
func (node *Node) processProposal(ctx context.Context, proposal consensus.Proposal) error {
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	header := proposal.Block.Header
	if committed, ok := node.currentCommitHeight(ctx); ok {
		if next, ok := nextHeightAfter(committed); ok && header.Height == next {
			if err := checkParentBinding(ctx, runtime, committed, header); err != nil {
				return fmt.Errorf("%w: %w", ErrProposalRejected, err)
			}
		}
	}
	if err := checkHeaderAppHash(ctx, runtime, header); err != nil {
		return fmt.Errorf("%w: %w", ErrProposalRejected, err)
	}
	request := app.ProcessProposalRequest{Block: proposal.Block}
	var response app.ProcessProposalResponse
	if contextual, ok := runtime.App.(app.ContextProcessProposalApplication); ok {
		response = contextual.ProcessProposalContext(ctx, request)
	} else {
		response = runtime.App.ProcessProposal(request)
	}
	if !response.Accepted {
		return fmt.Errorf("%w: %s", ErrProposalRejected, response.Reason)
	}
	return nil
}

// headerAppHash returns the app hash a header must commit: the header app hash of the state
// finality.AppHashHeight names for it.
func headerAppHash(ctx context.Context, runtime *vexoruntime.Runtime, header types.Header) (types.Hash, error) {
	height, ok := finality.AppHashHeight(header)
	if !ok {
		return types.Hash{}, fmt.Errorf("%w: height %d names no validator set height", ErrBlockParentMismatch, header.Height)
	}
	return runtime.HeaderAppHashAt(ctx, height)
}

// checkHeaderAppHash verifies that header commits the app hash this node computed for the height it
// names. A validator votes only for a header it checked, so a certificate vouches for the
// application state of that height, and a node whose state diverged refuses the chain instead of
// silently forking from it.
func checkHeaderAppHash(ctx context.Context, runtime *vexoruntime.Runtime, header types.Header) error {
	expected, err := headerAppHash(ctx, runtime, header)
	if err != nil {
		return err
	}
	if header.AppHash != expected {
		height, _ := finality.AppHashHeight(header)
		return fmt.Errorf("%w: height %d names app hash %x of height %d, local state has %x", ErrAppHashMismatch, header.Height, header.AppHash, height, expected)
	}
	return nil
}

// checkParentBinding verifies that header extends the last committed block: the block at committed
// must be the one header names as its parent, and the first block names no parent. The header must
// also name the validator set that parent fixes for it. A certificate only vouches for the header it
// signs, so the chain linkage, and with it the set that certified the header, is checked against
// the store.
func checkParentBinding(ctx context.Context, runtime *vexoruntime.Runtime, committed types.Height, header types.Header) error {
	var parent types.Hash
	setHeight := finality.FirstValidatorSetHeight
	if committed > 0 {
		state, err := runtime.LatestState(ctx)
		if err != nil {
			return err
		}
		if state.Height != committed {
			return fmt.Errorf("%w: latest state is at height %d, committed height is %d", ErrBlockParentMismatch, state.Height, committed)
		}
		parent = state.LastBlockHash
		record, err := runtime.BlockByHeight(ctx, committed)
		if err != nil {
			return err
		}
		setHeight = finality.ChildValidatorSetHeight(record.Block.Header)
	}
	if header.PreviousBlockHash != parent {
		return fmt.Errorf("%w: height %d names parent %x, last committed block is %x", ErrBlockParentMismatch, header.Height, header.PreviousBlockHash, parent)
	}
	if header.ValidatorSetHeight != setHeight {
		return fmt.Errorf("%w: height %d names validator set height %d, its parent fixes %d", ErrBlockParentMismatch, header.Height, header.ValidatorSetHeight, setHeight)
	}
	return nil
}

// acceptedProposalFor reports whether blockHash is the proposal the consensus machine accepted for the
// view, and the machine has voted in no later view: only that proposal may be voted for.
func (node *Node) acceptedProposalFor(machine *consensus.StateMachine, height types.Height, round types.Round, blockHash types.Hash) (consensus.Proposal, bool) {
	proposal, found := node.pendingProposals()[blockHash]
	if !found || proposal.Block.Header.Height != height || proposal.Round != round {
		return consensus.Proposal{}, false
	}
	state := machine.SafetyState()
	if state.LastVotedHeight != height || state.LastVotedRound != round {
		return consensus.Proposal{}, false
	}
	return proposal, true
}

// proposalIsAuthentic reports whether proposal carries a valid signature of a proposer of the
// validator set its header names, and the proposer is the one scheduled for its view. A refusal of the
// consensus machine says nothing about the proposal it refused unless the proposal is authentic: most
// refusals happen before the signature is checked.
func (node *Node) proposalIsAuthentic(ctx context.Context, proposal consensus.Proposal) bool {
	if ctx.Err() != nil {
		return false
	}
	runtime, err := node.Runtime()
	if err != nil || len(proposal.Signature) == 0 {
		return false
	}
	machine, err := node.Consensus()
	if err != nil {
		return false
	}
	validatorSet, err := machine.ValidatorSetAt(proposal.Block.Header.ValidatorSetHeight)
	if err != nil || validatorSet.Hash() != proposal.Block.Header.ValidatorSetHash {
		return false
	}
	leader, err := SelectProposer(validatorSet.List(), proposal.Block.Header.Height, proposal.Round)
	if err != nil || leader != proposal.Proposer {
		return false
	}
	proposer, found := validatorSet.Get(leader)
	if !found {
		return false
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusProposal, consensus.ProposalSignBytes(proposal))
	if err != nil {
		return false
	}
	return runtime.Crypto.ConsensusVerifier.Verify(proposer.PublicKey, message, proposal.Signature)
}

// removePendingIfSame drops the cached proposal for blockHash when it is exactly proposal.
func (node *Node) removePendingIfSame(blockHash types.Hash, proposal consensus.Proposal) bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	stored, found := node.pending[blockHash]
	if !found || !sameProposal(stored, proposal) {
		return false
	}
	delete(node.pending, blockHash)
	return true
}

// sameProposal reports whether two proposals are identical, certificates included. The proposer's
// signature does not cover the aggregate signatures of the certificates a proposal carries, so two
// proposals with equal signatures can still differ.
func sameProposal(left consensus.Proposal, right consensus.Proposal) bool {
	return left.Round == right.Round &&
		left.Proposer == right.Proposer &&
		bytes.Equal(left.Signature, right.Signature) &&
		consensus.HashBlock(left.Block) == consensus.HashBlock(right.Block) &&
		sameQuorumCert(left.JustifyQC, right.JustifyQC) &&
		sameTimeoutCert(left.TimeoutCert, right.TimeoutCert)
}

func sameTimeoutCert(left finality.TimeoutCert, right finality.TimeoutCert) bool {
	return left.Height == right.Height &&
		left.Round == right.Round &&
		sameQuorumCert(left.HighQC, right.HighQC) &&
		bytes.Equal(left.Signers, right.Signers) &&
		bytes.Equal(left.Signature, right.Signature)
}

// pruneVoteCaches drops the votes the local validator buffers for committed heights.
func (node *Node) pruneVoteCaches(height types.Height) {
	node.mu.Lock()
	voter := node.localVoter
	node.mu.Unlock()
	if voter != nil {
		voter.pruneAtOrBelow(height)
	}
}

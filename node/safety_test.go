package node

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// proposeAndVote makes the single validator of the test genesis propose a block in the machine's
// current view and vote for it. With one validator the vote alone is a quorum, so the block is
// certified when this returns.
func proposeAndVote(t *testing.T, node *Node, txs ...types.Tx) (consensus.Proposal, types.Hash) {
	t.Helper()
	ctx := context.Background()
	machine, err := node.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	status := machine.Status(ctx)
	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}, Txs: txs}, status.Round, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := node.signConsensusProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := node.localVoter.OnProposal(ctx, proposal); err != nil {
		t.Fatal(err)
	}
	blockHash := consensus.HashBlock(proposal.Block)
	if _, err := machine.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash); err != nil {
		t.Fatalf("expected block %d to be certified: %v", proposal.Block.Header.Height, err)
	}
	return proposal, blockHash
}

func newStartedValidator(t *testing.T, dataDir string) *Node {
	t.Helper()
	node := newTestNodeWithDataDir(t, dataDir)
	node.cfg.ValidatorID = "alice"
	startNode(t, node)
	return node
}

func TestFullNodeConsensusMachineVerifiesSignatures(t *testing.T) {
	ctx := context.Background()
	// A full node has neither a signer nor a validator id.
	node, err := New(DefaultConfig("vexo-test", t.TempDir()), validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	startNode(t, node)
	defer node.Stop(ctx)
	machine, err := node.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}, Txs: []types.Tx{[]byte("bank:first")}}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}

	if err := machine.OnProposal(ctx, proposal); !errors.Is(err, consensus.ErrInvalidProposal) {
		t.Fatalf("expected an unsigned proposal to be rejected by a full node, got %v", err)
	}
	forged := signedNodeTestProposal(t, deterministicSignerForID("mallory"), proposal)
	if err := machine.OnProposal(ctx, forged); !errors.Is(err, consensus.ErrInvalidProposal) {
		t.Fatalf("expected a proposal signed by a foreign key to be rejected by a full node, got %v", err)
	}
	genuine := signedNodeTestProposal(t, deterministicSignerForID("alice"), proposal)
	if err := machine.OnProposal(ctx, genuine); err != nil {
		t.Fatalf("expected the genuine proposal to be accepted, got %v", err)
	}

	blockHash := consensus.HashBlock(genuine.Block)
	forgedVote := signedNodeTestVote(t, deterministicSignerForID("mallory"), "alice", 1, 0, blockHash)
	if err := machine.OnVote(ctx, forgedVote); !errors.Is(err, consensus.ErrInvalidVote) {
		t.Fatalf("expected a vote signed by a foreign key to be rejected by a full node, got %v", err)
	}
	genuineVote := signedNodeTestVote(t, deterministicSignerForID("alice"), "alice", 1, 0, blockHash)
	if err := machine.OnVote(ctx, genuineVote); err != nil {
		t.Fatalf("expected the genuine vote to be accepted, got %v", err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); err != nil {
		t.Fatalf("expected the full node to certify the block from the genuine vote: %v", err)
	}
}

func TestForgedProposalDoesNotEvictPendingProposal(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	machine, err := node.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	genuine, err := machine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}, Txs: []types.Tx{[]byte("bank:first")}}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := node.signConsensusProposal(&genuine); err != nil {
		t.Fatal(err)
	}
	blockHash := consensus.HashBlock(genuine.Block)
	node.cacheProposal(genuine, blockHash)
	node.markProposed(1, 0)

	flipped := genuine
	flipped.Signature = append(types.Signature(nil), genuine.Signature...)
	flipped.Signature[0] ^= 0xff
	unsigned := genuine
	unsigned.Signature = nil
	foreign := signedNodeTestProposal(t, deterministicSignerForID("mallory"), genuine)
	// The proposer signature does not cover the aggregate signature of the certificate it carries.
	altered := genuine
	altered.JustifyQC.Signature = types.AggregateSignature("not the genuine aggregate")
	for name, forgery := range map[string]consensus.Proposal{"flipped": flipped, "unsigned": unsigned, "foreign": foreign, "altered": altered} {
		node.handleRejectedProposal(forgery, consensus.ErrInvalidProposal)
		if _, _, ok := node.cachedProposalForRound(1, 0); !ok {
			t.Fatalf("%s copy of the pending proposal evicted it", name)
		}
		if !node.hasProposed(1, 0) {
			t.Fatalf("%s copy of the pending proposal cleared the proposed marker", name)
		}
	}

	node.handleRejectedProposal(genuine, consensus.ErrUnsafeProposal)
	if _, _, ok := node.cachedProposalForRound(1, 0); ok {
		t.Fatal("expected the authentic rejected proposal to be evicted")
	}
	if node.hasProposed(1, 0) {
		t.Fatal("expected the authentic rejected proposal to clear the proposed marker")
	}
}

func TestRestartedValidatorRefusesToVoteAgainstPersistedLock(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	first := newStartedValidator(t, dataDir)

	// alice certifies B1 in view 0, then votes for B2 in view 1, which locks her on QC(B1).
	proposeAndVote(t, first)
	proposeAndVote(t, first)
	firstMachine, err := first.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	persisted := firstMachine.SafetyState()
	if persisted.LockedQC.Height != 1 || persisted.LastVotedHeight != 2 || persisted.LastVotedRound != 1 {
		t.Fatalf("unexpected safety state before the restart: %+v", persisted)
	}
	if err := first.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	// A proposal in view 3 on top of genesis is justified by a timeout certificate of view 2 whose
	// quorum had not seen QC(B1). It conflicts with the lock.
	control := newStartedValidator(t, t.TempDir())
	defer control.Stop(ctx)
	controlMachine, err := control.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	timeoutVote := consensus.TimeoutVote{Height: 1, Round: 2, ValidatorID: "alice"}
	if err := control.signConsensusTimeoutVote(&timeoutVote); err != nil {
		t.Fatal(err)
	}
	if _, err := controlMachine.OnTimeoutVote(ctx, timeoutVote); err != nil {
		t.Fatal(err)
	}
	conflicting, err := controlMachine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}, Txs: []types.Tx{[]byte("bank:conflicting")}}, 3, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := control.signConsensusProposal(&conflicting); err != nil {
		t.Fatal(err)
	}

	// Without the persisted lock the validator votes for it.
	if err := control.localVoter.OnProposal(ctx, conflicting); err != nil {
		t.Fatalf("expected a validator without a lock to accept the proposal, got %v", err)
	}
	if voted, err := control.hasRecordedLocalVote(1, 3); err != nil || !voted {
		t.Fatalf("expected the control validator to vote, voted=%t err=%v", voted, err)
	}

	restarted := newStartedValidator(t, dataDir)
	defer restarted.Stop(ctx)
	restartedMachine, err := restarted.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	state := restartedMachine.SafetyState()
	if state.LockedQC.BlockHash != persisted.LockedQC.BlockHash || state.LastVotedHeight != 2 || state.LastVotedRound != 1 {
		t.Fatalf("expected the restart to restore the safety state %+v, got %+v", persisted, state)
	}
	err = restarted.localVoter.OnProposal(ctx, conflicting)
	if !errors.Is(err, consensus.ErrUnsafeProposal) {
		t.Fatalf("expected the restarted validator to refuse the proposal against its lock, got %v", err)
	}
	if voted, err := restarted.hasRecordedLocalVote(1, 3); err != nil || voted {
		t.Fatalf("expected the restarted validator not to vote, voted=%t err=%v", voted, err)
	}
}

func TestValidatorRefusesToStartWhenSafetyStateCannotBeRestored(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	cfg := DefaultConfig("vexo-test", dataDir)
	wal, err := consensus.OpenWAL(cfg.ConsensusWALPath())
	if err != nil {
		t.Fatal(err)
	}
	// A lock whose certificate was never signed by the validator set.
	forgedLock := finality.QuorumCert{
		Height:    1,
		Round:     0,
		BlockHash: types.Hash{9},
		Signers:   finality.EncodeSigners([]types.ValidatorID{"alice"}),
		Signature: types.AggregateSignature("forged"),
	}
	if err := wal.RecordSafetyState(consensus.SafetyState{LockedQC: forgedLock, HighQC: forgedLock, LastVotedHeight: 2, LastVotedRound: 1}); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	validatorNode := newTestNodeWithDataDir(t, dataDir)
	validatorNode.cfg.ValidatorID = "alice"
	if err := validatorNode.Start(ctx); err == nil {
		validatorNode.Stop(ctx)
		t.Fatal("expected a validator whose safety state cannot be restored to refuse to start")
	}
	if validatorNode.running {
		t.Fatal("expected the validator not to be running")
	}
}

func TestValidatorDoesNotVoteForProposalRejectedByApplication(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	machine, err := node.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	// No module routes this transaction. The consensus machine does not run the application, so it
	// accepts the block; the application does not.
	rejected, err := machine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}, Txs: []types.Tx{[]byte("unroutable:tx")}}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := node.signConsensusProposal(&rejected); err != nil {
		t.Fatal(err)
	}
	if err := node.processProposal(ctx, rejected); !errors.Is(err, ErrProposalRejected) {
		t.Fatalf("expected the application to reject the block, got %v", err)
	}

	if err := node.localVoter.OnProposal(ctx, rejected); err != nil {
		t.Fatalf("expected an authentic proposal the application declines not to fail the relaying peer, got %v", err)
	}
	if voted, err := node.hasRecordedLocalVote(1, 0); err != nil || voted {
		t.Fatalf("expected no vote for a proposal the application rejected, voted=%t err=%v", voted, err)
	}
	if _, found := node.pendingProposals()[consensus.HashBlock(rejected.Block)]; !found {
		t.Fatal("expected the authentic proposal to stay cached: other validators may still certify it")
	}
	if wal, found, err := node.consensusWAL.SafetyState(); err != nil || found && wal.LastVotedHeight != 0 {
		t.Fatalf("expected nothing to be persisted as voted, found=%t state=%+v err=%v", found, wal, err)
	}
}

func TestTimedOutValidatorDoesNotVoteInTheViewItLeft(t *testing.T) {
	ctx := context.Background()
	alice, _, _ := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(ctx)
	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	if _, _, err := alice.TimeoutRound(ctx); err != nil {
		t.Fatal(err)
	}
	state := machine.SafetyState()
	if state.LastVotedHeight != 1 || state.LastVotedRound != 0 {
		t.Fatalf("expected the timeout to count as a vote in view 0, got %+v", state)
	}
	persisted, found, err := alice.consensusWAL.SafetyState()
	if err != nil || !found || persisted.LastVotedHeight != 1 || persisted.LastVotedRound != 0 {
		t.Fatalf("expected the timeout to be persisted before it was signed, found=%t state=%+v err=%v", found, persisted, err)
	}

	// The proposer of view 0 is late: its proposal arrives after alice gave up on the view.
	proposerID, err := alice.Proposer(ctx, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	late, err := machine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}, Txs: []types.Tx{[]byte("bank:late")}}, 0, proposerID, finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	late = signedNodeTestProposal(t, deterministicSignerForID(proposerID), late)
	if err := alice.localVoter.OnProposal(ctx, late); err != nil {
		t.Fatal(err)
	}
	if voted, err := alice.hasRecordedLocalVote(1, 0); err != nil || voted {
		t.Fatalf("expected no vote in the view alice timed out of, voted=%t err=%v", voted, err)
	}
}

func TestCommitBlockRejectsBodyNotBoundToHeader(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	proposal, blockHash := proposeAndVote(t, node, []byte("bank:first"), []byte("bank:second"))
	qc, err := mustConsensus(t, node).BuildQuorumCert(1, 0, blockHash)
	if err != nil {
		t.Fatal(err)
	}

	swapped := proposal.Block
	swapped.Txs = []types.Tx{[]byte("bank:stolen"), []byte("bank:funds")}
	if _, err := node.commitBlock(ctx, swapped, qc, false, false); !errors.Is(err, ErrInvalidCommitQC) {
		t.Fatalf("expected a block whose transactions were swapped to be rejected, got %v", err)
	}

	// The hash of a block derives its transaction root from the body, so a header that lies about
	// the root still carries the certified hash. Only the body check catches it.
	lying := proposal.Block
	lying.Header.TxRoot = types.Hash{0xaa}
	if consensus.HashBlock(lying) != blockHash {
		t.Fatal("expected the lying header to keep the certified hash")
	}
	if _, err := node.commitBlock(ctx, lying, qc, false, false); !errors.Is(err, ErrInvalidCommitBlock) {
		t.Fatalf("expected a block whose header lies about its transactions to be rejected, got %v", err)
	}

	if committed, _ := node.currentCommitHeight(ctx); committed != 0 {
		t.Fatalf("expected nothing to be executed, committed height is %d", committed)
	}
	if _, err := runtime.LatestState(ctx); !errors.Is(err, store.ErrStateNotFound) {
		t.Fatalf("expected no state to be stored, got %v", err)
	}
}

func TestCommitBlockRejectsBlockThatDoesNotExtendCommittedChain(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	machine := mustConsensus(t, node)

	// The first block names no parent.
	first, firstHash := proposeAndVote(t, node)
	orphan := first.Block
	orphan.Header.PreviousBlockHash = types.Hash{7}
	orphanQC := finality.QuorumCert{Height: 1, Round: 0, BlockHash: consensus.HashBlock(orphan)}
	if _, err := node.commitBlock(ctx, orphan, orphanQC, false, false); !errors.Is(err, ErrBlockParentMismatch) {
		t.Fatalf("expected the first block with a parent to be rejected, got %v", err)
	}

	second, _ := proposeAndVote(t, node)
	decisions := machine.CommitDecisions()
	if len(decisions) != 1 || decisions[0].CommittedBlockHash != firstHash {
		t.Fatalf("expected B1 to be final, got %+v", decisions)
	}
	if _, err := node.CommitBlock(ctx, first.Block, decisions[0].CommitQC); err != nil {
		t.Fatal(err)
	}

	// The second block must name the committed first block.
	forked := second.Block
	forked.Header.PreviousBlockHash = types.Hash{8}
	forkedQC := finality.QuorumCert{Height: 2, Round: 1, BlockHash: consensus.HashBlock(forked)}
	if _, err := node.commitBlock(ctx, forked, forkedQC, false, false); !errors.Is(err, ErrBlockParentMismatch) {
		t.Fatalf("expected a block that does not extend the committed block to be rejected, got %v", err)
	}
	if committed, _ := node.currentCommitHeight(ctx); committed != 1 {
		t.Fatalf("expected the committed height to stay 1, got %d", committed)
	}
}

func TestCommitFinalizedBlockComputesRecoveryReportOncePerCommittedHeight(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)

	// With the gate off the report is never built, even though a block commits.
	first, _ := proposeAndVote(t, node)
	proposeAndVote(t, node)
	result, committed, err := node.commitFinalizedBlock(ctx, false)
	if err != nil || !committed || result.Block.Header.Height != first.Block.Header.Height {
		t.Fatalf("expected B1 to commit with the gate off, committed=%t err=%v", committed, err)
	}
	if node.recoveryGate.computed != 0 {
		t.Fatalf("expected no recovery report with the gate off, built %d", node.recoveryGate.computed)
	}

	// With the gate on the report is built once per committed height, however often it is consulted.
	for index := 0; index < 3; index++ {
		if _, err := node.recoveryReportAt(ctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	if node.recoveryGate.computed != 1 {
		t.Fatalf("expected one report for committed height 1, built %d", node.recoveryGate.computed)
	}
	proposeAndVote(t, node)
	if _, committed, err := node.commitFinalizedBlock(ctx, true); err != nil || !committed {
		t.Fatalf("expected B2 to commit with the gate on, committed=%t err=%v", committed, err)
	}
	if node.recoveryGate.computed != 1 {
		t.Fatalf("expected the gate to reuse the report of committed height 1, built %d", node.recoveryGate.computed)
	}
	for index := 0; index < 3; index++ {
		if _, err := node.recoveryReportAt(ctx, 2); err != nil {
			t.Fatal(err)
		}
	}
	if node.recoveryGate.computed != 2 {
		t.Fatalf("expected one report per committed height, built %d", node.recoveryGate.computed)
	}

	// A gate that defers a commit must judge a fresh report next time.
	node.forgetRecoveryReport()
	if _, err := node.recoveryReportAt(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if node.recoveryGate.computed != 3 {
		t.Fatalf("expected a forgotten report to be rebuilt, built %d", node.recoveryGate.computed)
	}
}

// failingProofStore fails the first SaveFinalityProof calls.
type failingProofStore struct {
	*store.LevelDBStore
	failures int
	saves    int
}

func (proofStore *failingProofStore) SaveFinalityProof(ctx context.Context, record store.FinalityProofRecord) error {
	if proofStore.failures > 0 {
		proofStore.failures--
		return errors.New("disk full")
	}
	proofStore.saves++
	return proofStore.LevelDBStore.SaveFinalityProof(ctx, record)
}

func TestReconcileFinishesCommitAfterPostExecutionFailure(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	levelDB, ok := runtime.Store.(*store.LevelDBStore)
	if !ok {
		t.Fatalf("expected a LevelDB store, got %T", runtime.Store)
	}
	flaky := &failingProofStore{LevelDBStore: levelDB, failures: 1}
	runtime.Store = flaky

	proposeAndVote(t, node)
	proposeAndVote(t, node)
	if _, _, err := node.CommitFinalizedBlock(ctx); err == nil {
		t.Fatal("expected the commit to report the failure after execution")
	}
	if committed, _ := node.currentCommitHeight(ctx); committed != 1 {
		t.Fatalf("expected the block to be durable, committed height is %d", committed)
	}
	if _, err := levelDB.FinalityProof(ctx, 1); !errors.Is(err, store.ErrFinalityNotFound) {
		t.Fatalf("expected the finality proof to be missing after the failure, got %v", err)
	}
	if node.reconciledHeight != 0 {
		t.Fatalf("expected the failed commit not to count as reconciled, got %d", node.reconciledHeight)
	}

	// A step reconciles before it does anything else.
	if _, err := node.StepConsensusWithConfig(ctx, ConsensusLoopConfig{MaxBlockBytes: 1 << 20, ExecutionCommitMode: ExecutionCommitModeFinalized}); err != nil {
		t.Fatal(err)
	}
	if node.reconciledHeight != 1 {
		t.Fatalf("expected the step to finish the commit, reconciled height is %d", node.reconciledHeight)
	}
	if _, err := levelDB.FinalityProof(ctx, 1); err != nil {
		t.Fatalf("expected the reconcile to store the finality proof: %v", err)
	}
	savesAfterReconcile := flaky.saves

	// A reconciled height is not reconciled again.
	if err := node.reconcileCommitted(ctx); err != nil {
		t.Fatal(err)
	}
	if flaky.saves != savesAfterReconcile {
		t.Fatalf("expected no repeated work for a reconciled height, saves %d -> %d", savesAfterReconcile, flaky.saves)
	}
}

func TestPersistFinalityDecisionsSkipsStoredHeights(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	levelDB := runtime.Store.(*store.LevelDBStore)
	counting := &failingProofStore{LevelDBStore: levelDB}
	runtime.Store = counting
	machine := mustConsensus(t, node)

	proposeAndVote(t, node)
	proposeAndVote(t, node)
	if _, committed, err := node.CommitFinalizedBlock(ctx); err != nil || !committed {
		t.Fatalf("expected B1 to commit, committed=%t err=%v", committed, err)
	}
	if node.finalityPersisted != 1 || counting.saves != 1 {
		t.Fatalf("expected the proof of B1 to be stored once, mark=%d saves=%d", node.finalityPersisted, counting.saves)
	}

	// The decision for B1 is still held by the machine until B2 is committed.
	if decisions := machine.CommitDecisions(); len(decisions) != 1 || decisions[0].CommittedHeight != 1 {
		t.Fatalf("expected the decision for B1 to be retained, got %+v", decisions)
	}
	for index := 0; index < 3; index++ {
		if err := node.persistFinalityDecisions(ctx, runtime, machine); err != nil {
			t.Fatal(err)
		}
	}
	if counting.saves != 1 {
		t.Fatalf("expected the stored decision not to be processed again, saves=%d", counting.saves)
	}
}

func TestBoundedNextHeightLimitsTheJump(t *testing.T) {
	maxHeight := ^types.Height(0)
	for name, test := range map[string]struct {
		committed types.Height
		certified types.Height
		expected  types.Height
	}{
		"first block":              {committed: 0, certified: 0, expected: 1},
		"next to the committed":    {committed: 5, certified: 0, expected: 6},
		"above the certified tip":  {committed: 5, certified: 7, expected: 8},
		"certified below the base": {committed: 5, certified: 3, expected: 6},
		"far certified tip":        {committed: 5, certified: 1 << 62, expected: 6 + maxConsensusHeightJump},
		"certified at the maximum": {committed: 5, certified: maxHeight, expected: 6 + maxConsensusHeightJump},
		"committed at the maximum": {committed: maxHeight, certified: maxHeight, expected: maxHeight},
		"near the maximum":         {committed: maxHeight - 10, certified: maxHeight - 1, expected: maxHeight},
	} {
		if actual := boundedNextHeight(test.committed, test.certified); actual != test.expected {
			t.Fatalf("%s: expected %d, got %d", name, test.expected, actual)
		}
	}
}

func TestAutoVoteReactorBoundsBufferedVotesPerValidator(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	defer node.Stop(ctx)
	reactor := node.localVoter

	// A validator signs votes for blocks this node has never seen, in every view of the window.
	for round := types.Round(0); round < 2*maxBufferedVotesPerValidator; round++ {
		vote := signedNodeTestVote(t, deterministicSignerForID("alice"), "alice", 1, round, types.Hash{byte(round + 1)})
		if err := reactor.OnVote(ctx, vote); err != nil {
			t.Fatal(err)
		}
	}
	reactor.mu.Lock()
	buffered := len(reactor.pendingVotes)
	counted := reactor.pendingPerValidator["alice"]
	reactor.mu.Unlock()
	if buffered != maxBufferedVotesPerValidator || counted != maxBufferedVotesPerValidator {
		t.Fatalf("expected %d buffered votes for the validator, got blocks=%d counted=%d", maxBufferedVotesPerValidator, buffered, counted)
	}

	reactor.pruneAtOrBelow(1)
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	if len(reactor.pendingVotes) != 0 || len(reactor.pendingPerValidator) != 0 {
		t.Fatalf("expected the commit to release the buffered votes, got blocks=%d counts=%v", len(reactor.pendingVotes), reactor.pendingPerValidator)
	}
}

func TestAutoVoteReactorBoundsLocalVoteCache(t *testing.T) {
	reactor := &autoVoteReactor{}
	for round := types.Round(0); round < maxCachedLocalVotes+100; round++ {
		reactor.cacheLocalVote(consensus.Vote{Height: 1, Round: round, BlockHash: types.Hash{1}, ValidatorID: "alice"})
	}
	if len(reactor.localVotes) != maxCachedLocalVotes {
		t.Fatalf("expected the local vote cache to hold %d votes, got %d", maxCachedLocalVotes, len(reactor.localVotes))
	}
	if _, cached := reactor.cachedLocalVote(1, maxCachedLocalVotes+99); !cached {
		t.Fatal("expected the newest vote to stay cached")
	}
	if _, cached := reactor.cachedLocalVote(1, 0); cached {
		t.Fatal("expected the oldest vote to be evicted")
	}
}

func TestFullNodeRestartAnchorsConsensusAtCommittedBlock(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	first := newStartedValidator(t, dataDir)
	proposeAndVote(t, first)
	second, _ := proposeAndVote(t, first)
	if _, committed, err := first.CommitFinalizedBlock(ctx); err != nil || !committed {
		t.Fatalf("expected B1 to commit, committed=%t err=%v", committed, err)
	}
	if err := first.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	restarted := newStartedValidator(t, dataDir)
	defer restarted.Stop(ctx)
	machine := mustConsensus(t, restarted)
	state := machine.SafetyState()
	if state.LockedQC.Height < 1 {
		t.Fatalf("expected the restart to restore a lock at the committed block, got %+v", state.LockedQC)
	}
	status := machine.Status(ctx)
	if status.Height < 2 || status.Round < 1 {
		t.Fatalf("expected the restart to continue above the committed block, got %+v", status)
	}
	// The second block was voted for before the restart, not committed, and must not be voted for twice.
	if voted, err := restarted.hasRecordedLocalVote(second.Block.Header.Height, second.Round); err != nil || !voted {
		t.Fatalf("expected the WAL to remember the vote for B2, voted=%t err=%v", voted, err)
	}
}

func mustConsensus(t *testing.T, node *Node) *consensus.StateMachine {
	t.Helper()
	machine, err := node.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	return machine
}

// The proposer stamps the app hash of the state the header's validator set was taken from, and a
// validator refuses to vote for a proposal whose header names another app hash than its own state.
func TestValidatorRefusesToVoteForAProposalNamingAnotherAppHash(t *testing.T) {
	ctx := context.Background()
	node := newStartedValidator(t, t.TempDir())
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := runtime.AppHashAt(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := node.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	status := machine.Status(ctx)
	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{ChainID: "vexo-test"}}, status.Round, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Block.Header.AppHash != genesis || genesis == (types.Hash{}) {
		t.Fatalf("expected the proposal to commit the genesis app hash %x, got %x", genesis, proposal.Block.Header.AppHash)
	}
	if err := node.signConsensusProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := node.processProposal(ctx, proposal); err != nil {
		t.Fatalf("expected the proposal naming the local app hash to be accepted, got %v", err)
	}
	forged := proposal
	forged.Block.Header.AppHash[0] ^= 0xff
	if err := node.signConsensusProposal(&forged); err != nil {
		t.Fatal(err)
	}
	if err := node.processProposal(ctx, forged); !errors.Is(err, ErrAppHashMismatch) {
		t.Fatalf("expected the forged app hash to be refused, got %v", err)
	}
	if err := node.localVoter.OnProposal(ctx, forged); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, status.Round, consensus.HashBlock(forged.Block)); err == nil {
		t.Fatal("expected no vote for the proposal naming a forged app hash")
	}
}

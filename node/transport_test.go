package node

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	transportTestWaitTimeout  = 30 * time.Second
	transportTestWaitAttempts = 300000
)

func TestNodeTransportReactorRoutesProposalBetweenNodes(t *testing.T) {
	alice, bob := newTransportNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)
	proposal, err := aliceConsensus.CreateProposal(types.Block{Header: types.Header{Height: 1}}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.signConsensusProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	aliceReactor, err := alice.ConsensusReactor()
	if err != nil {
		t.Fatal(err)
	}
	if err := aliceReactor.BroadcastProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}

	bobConsensus, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	waitForConsensusStatus(t, bobConsensus, func(status consensus.Status) bool {
		return status.Height == 1 && (status.Phase == consensus.PhaseVote || status.Phase == consensus.PhaseCommit)
	})
}

func TestNodeTransportReactorRoutesVoteBetweenNodes(t *testing.T) {
	alice, bob := newTransportNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	bobConsensus, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)
	bobConsensus.StartRound(1, 0)

	proposal, err := aliceConsensus.CreateProposal(types.Block{Header: types.Header{Height: 1}}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.signConsensusProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := aliceConsensus.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	if err := bobConsensus.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	blockHash := consensus.HashBlock(proposal.Block)

	aliceReactor, err := alice.ConsensusReactor()
	if err != nil {
		t.Fatal(err)
	}
	vote := consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   blockHash,
		ValidatorID: "alice",
	}
	if err := alice.signConsensusVote(&vote); err != nil {
		t.Fatal(err)
	}
	if err := aliceReactor.BroadcastVote(context.Background(), vote); err != nil {
		t.Fatal(err)
	}

	waitForQuorumInput(t, bobConsensus, blockHash)
}

func TestNodeTransportReactorReplaysVoteThatArrivesBeforeProposal(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	bobConsensus, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)
	bobConsensus.StartRound(1, 0)

	proposal, err := aliceConsensus.CreateProposal(types.Block{Header: types.Header{Height: 1}}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.signConsensusProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := aliceConsensus.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	blockHash := consensus.HashBlock(proposal.Block)

	aliceReactor, err := alice.ConsensusReactor()
	if err != nil {
		t.Fatal(err)
	}
	vote := consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   blockHash,
		ValidatorID: "alice",
	}
	if err := alice.signConsensusVote(&vote); err != nil {
		t.Fatal(err)
	}
	if err := aliceReactor.BroadcastVote(context.Background(), vote); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := bobConsensus.BuildQuorumCert(1, 0, blockHash); err == nil {
		t.Fatal("expected no quorum before target proposal is known")
	}

	if err := aliceReactor.BroadcastProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	waitForQuorumCert(t, bobConsensus, 1, 0, blockHash)
}

func TestNodeConsensusLoopBroadcastsProposalAndCollectsVotes(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)

	proposal, blockHash, err := alice.ProposeBlock(context.Background(), types.Block{
		Header: types.Header{Height: 1},
		Txs: []types.Tx{
			[]byte("bank:tx-c"),
			[]byte("bank:tx-a"),
			[]byte("bank:tx-b"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Proposer != "alice" {
		t.Fatalf("unexpected proposer: %s", proposal.Proposer)
	}
	if _, _, err := alice.VoteBlock(context.Background(), proposal.Block.Header.Height, proposal.Round, blockHash); err != nil {
		t.Fatalf("local vote failed: %v", err)
	}

	waitForQuorumCert(t, aliceConsensus, proposal.Block.Header.Height, proposal.Round, blockHash)
	waitForConsensusStatus(t, aliceConsensus, func(status consensus.Status) bool {
		return status.Phase == consensus.PhaseCommit
	})
	quorumCert, err := aliceConsensus.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	response, err := alice.CommitBlock(context.Background(), proposal.Block, quorumCert)
	if err != nil {
		t.Fatal(err)
	}
	if response.AppHash == (types.Hash{}) {
		t.Fatal("expected committed app hash")
	}
	status := alice.Status(context.Background())
	if status.LatestHeight != 1 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("unexpected node status after commit: %+v", status)
	}
	record, err := alice.runtime.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if record.Hash != blockHash {
		t.Fatalf("stored block hash mismatch: %x != %x", record.Hash, blockHash)
	}
	waitForConsensusStatus(t, aliceConsensus, func(status consensus.Status) bool {
		return status.Height == 2 && status.Round == 0 && status.Phase == consensus.PhasePropose
	})
}

func TestNodeProposesFromMempoolAndClearsCommittedTxs(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)

	for _, tx := range []types.Tx{
		[]byte("bank:second"),
		[]byte("bank:first"),
		[]byte("bank:third"),
	} {
		if err := alice.SubmitTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if alice.runtime.Mempool.Len() != 3 {
		t.Fatalf("expected 3 mempool txs, got %d", alice.runtime.Mempool.Len())
	}

	proposal, blockHash, err := alice.ProposeFromMempool(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(proposal.Block.Txs) != 3 {
		t.Fatalf("expected 3 proposal txs, got %d", len(proposal.Block.Txs))
	}
	if !fairordering.IsOrderedWithSalt(proposal.Block.Txs, fairordering.HeightSalt("vexo-test", proposal.Block.Header.Height)) {
		t.Fatalf("expected deterministic proposal ordering, got %q", proposal.Block.Txs)
	}
	quorumCert, ok, err := alice.VoteBlock(context.Background(), proposal.Block.Header.Height, proposal.Round, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		waitForQuorumCert(t, aliceConsensus, proposal.Block.Header.Height, proposal.Round, blockHash)
		quorumCert, err = aliceConsensus.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alice.CommitBlock(context.Background(), proposal.Block, quorumCert); err != nil {
		t.Fatal(err)
	}
	if alice.runtime.Mempool.Len() != 0 {
		t.Fatalf("expected committed txs to be removed, got %d", alice.runtime.Mempool.Len())
	}
}

func TestNodeSubmitTxGossipsToPeers(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	if err := alice.SubmitTx(context.Background(), []byte("bank:gossip")); err != nil {
		t.Fatal(err)
	}

	waitForMempoolLen(t, alice, 1)
	waitForMempoolLen(t, bob, 1)
	waitForMempoolLen(t, carol, 1)

	if err := bob.SubmitTx(context.Background(), []byte("bank:gossip")); err == nil {
		t.Fatal("expected duplicate tx rejection")
	}
	waitForMempoolLen(t, alice, 1)
	waitForMempoolLen(t, bob, 1)
	waitForMempoolLen(t, carol, 1)
}

func TestNodeTickConsensusOnlyProposerBuildsMempoolProposal(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)
	bobConsensus, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	bobConsensus.StartRound(1, 0)

	proposer, err := alice.Proposer(context.Background(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if proposer != "alice" {
		t.Fatalf("expected alice proposer at h1/r0, got %s", proposer)
	}
	nextProposer, err := alice.Proposer(context.Background(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if nextProposer != "bob" {
		t.Fatalf("expected bob proposer at h1/r1, got %s", nextProposer)
	}

	if err := alice.SubmitTx(context.Background(), []byte("bank:auto-propose")); err != nil {
		t.Fatal(err)
	}
	if _, _, proposed, err := bob.TickConsensus(context.Background(), 1024); err != nil || proposed {
		t.Fatalf("non-proposer should skip: proposed=%v err=%v", proposed, err)
	}
	proposal, blockHash, proposed, err := alice.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !proposed {
		t.Fatal("expected proposer to build proposal")
	}
	if proposal.Proposer != "alice" || blockHash == (types.Hash{}) {
		t.Fatalf("unexpected proposal result: proposer=%s hash=%x", proposal.Proposer, blockHash)
	}
	waitForConsensusStatus(t, bobConsensus, func(status consensus.Status) bool {
		return status.Height == 1 && (status.Phase == consensus.PhaseVote || status.Phase == consensus.PhaseCommit)
	})
}

func TestNodeTickConsensusDoesNotReproposeSameRound(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	for _, node := range []*Node{alice, bob, carol} {
		machine, err := node.Consensus()
		if err != nil {
			t.Fatal(err)
		}
		machine.StartRound(1, 0)
	}

	firstProposal, firstHash, proposed, err := alice.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !proposed || firstProposal.Proposer != "alice" || firstHash == (types.Hash{}) {
		t.Fatalf("expected first proposal from alice, proposed=%v proposal=%+v hash=%x", proposed, firstProposal, firstHash)
	}
	secondProposal, secondHash, proposed, err := alice.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if proposed || secondHash != (types.Hash{}) || secondProposal.Proposer != "" {
		t.Fatalf("expected same round reproposal to be suppressed, proposed=%v proposal=%+v hash=%x", proposed, secondProposal, secondHash)
	}

	for _, node := range []*Node{alice, bob, carol} {
		if _, _, err := node.TimeoutRound(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	_, _, proposed, err = alice.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if proposed {
		t.Fatal("expected alice not to propose after round rotation moves proposer to bob")
	}
	nextProposal, nextHash, proposed, err := bob.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !proposed || nextProposal.Proposer != "bob" || nextProposal.Round != 1 || nextHash == (types.Hash{}) {
		t.Fatalf("expected bob to propose after round rotation, proposed=%v proposal=%+v hash=%x", proposed, nextProposal, nextHash)
	}
}

func TestNodeTickConsensusSuppressesReproposalAfterLocalVoteFailure(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	for _, node := range []*Node{alice, bob, carol} {
		machine, err := node.Consensus()
		if err != nil {
			t.Fatal(err)
		}
		machine.StartRound(1, 0)
	}
	if err := alice.recordConsensusVote(consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   types.Hash{9},
		ValidatorID: "alice",
	}); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := alice.TickConsensus(context.Background(), 1024)
	if !errors.Is(err, consensus.ErrDoubleSignDetected) {
		t.Fatalf("expected local vote failure from WAL guard, got %v", err)
	}
	_, _, proposed, err := alice.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if proposed {
		t.Fatal("expected reproposal suppressed after local vote failure")
	}
}

func TestNodeCommitsReadyCachedProposalAfterQC(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)
	bobConsensus, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	bobConsensus.StartRound(1, 0)

	if err := alice.SubmitTx(context.Background(), []byte("bank:cached-commit")); err != nil {
		t.Fatal(err)
	}
	proposal, blockHash, proposed, err := alice.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !proposed {
		t.Fatal("expected alice to propose")
	}
	waitForQuorumCert(t, bobConsensus, proposal.Block.Header.Height, proposal.Round, blockHash)

	result, committed, err := bob.UnsafeCommitReadyBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("expected bob to commit cached proposal")
	}
	if result.BlockHash != blockHash || result.Block.Header.Height != 1 {
		t.Fatalf("unexpected commit result: %+v", result)
	}
	if result.Response.AppHash == (types.Hash{}) {
		t.Fatal("expected committed app hash")
	}
	status := bob.Status(context.Background())
	if status.LatestHeight != 1 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("unexpected bob status after cached commit: %+v", status)
	}
	if _, committed, err := bob.UnsafeCommitReadyBlock(context.Background()); err != nil || committed {
		t.Fatalf("expected no second ready block: committed=%v err=%v", committed, err)
	}
}

func TestNodeStepConsensusProposesThenCommitsReadyBlock(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)
	bobConsensus, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	bobConsensus.StartRound(1, 0)

	if err := alice.SubmitTx(context.Background(), []byte("bank:step")); err != nil {
		t.Fatal(err)
	}
	proposeStep, err := alice.StepConsensusWithConfig(context.Background(), ConsensusLoopConfig{MaxBlockBytes: 1024, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeQC, AllowUnsafeQCCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if proposeStep.Committed || !proposeStep.Proposed {
		t.Fatalf("expected proposal step only, got %+v", proposeStep)
	}
	waitForQuorumCert(t, bobConsensus, proposeStep.Proposal.Block.Header.Height, proposeStep.Proposal.Round, proposeStep.BlockHash)

	commitStep, err := bob.StepConsensusWithConfig(context.Background(), ConsensusLoopConfig{MaxBlockBytes: 1024, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeQC, AllowUnsafeQCCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if !commitStep.Committed || commitStep.Proposed {
		t.Fatalf("expected commit step only, got %+v", commitStep)
	}
	if commitStep.Commit.BlockHash != proposeStep.BlockHash {
		t.Fatalf("commit hash mismatch: %x != %x", commitStep.Commit.BlockHash, proposeStep.BlockHash)
	}
	status := bob.Status(context.Background())
	if status.LatestHeight != 1 {
		t.Fatalf("expected bob committed height 1, got %+v", status)
	}
	emptyProposalStep, err := bob.StepConsensusWithConfig(context.Background(), ConsensusLoopConfig{MaxBlockBytes: 1024, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeQC, AllowUnsafeQCCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if emptyProposalStep.Committed || !emptyProposalStep.Proposed {
		t.Fatalf("expected next-height empty proposal after commit, got %+v", emptyProposalStep)
	}
	if len(emptyProposalStep.Proposal.Block.Txs) != 0 || emptyProposalStep.Proposal.Block.Header.Height != 2 {
		t.Fatalf("unexpected empty proposal after commit: %+v", emptyProposalStep.Proposal.Block)
	}
}

func TestNodeCommitGossipSyncsPeerThatMissedProposal(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	aliceConsensus.StartRound(1, 0)

	if err := alice.SubmitTx(context.Background(), []byte("bank:missed-proposal")); err != nil {
		t.Fatal(err)
	}
	proposal, blockHash, err := alice.ProposeFromMempool(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	quorumCert, ok, err := alice.VoteBlock(context.Background(), proposal.Block.Header.Height, proposal.Round, blockHash)
	if err != nil {
		t.Fatalf("vote block: %v", err)
	}
	if !ok {
		quorumCert = forceTestQuorumCert(t, aliceConsensus, proposal.Block.Header.Height, proposal.Round, blockHash)
	}
	parentProposal, parentHash, err := alice.ProposeBlock(context.Background(), types.Block{
		Header: types.Header{Height: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	parentQC, ok, err := alice.VoteBlock(context.Background(), parentProposal.Block.Header.Height, parentProposal.Round, parentHash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		parentQC = forceTestQuorumCert(t, aliceConsensus, parentProposal.Block.Header.Height, parentProposal.Round, parentHash)
	}
	childProposal, childHash, err := alice.ProposeBlock(context.Background(), types.Block{
		Header: types.Header{Height: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if childProposal.JustifyQC.Height != parentQC.Height || childProposal.JustifyQC.BlockHash != parentQC.BlockHash {
		t.Fatalf("expected child proposal to carry parent QC, got %+v want %+v", childProposal.JustifyQC, parentQC)
	}
	if childProposal.Block.Header.PreviousBlockHash != parentHash || childHash == (types.Hash{}) {
		t.Fatalf("unexpected child proposal: hash=%x proposal=%+v", childHash, childProposal)
	}

	startNode(t, bob)
	defer bob.Stop(context.Background())
	if status := bob.Status(context.Background()); status.LatestHeight != 0 {
		t.Fatalf("expected bob to miss proposal before commit gossip, got %+v", status)
	}
	if _, err := alice.CommitBlock(context.Background(), proposal.Block, quorumCert); err != nil {
		t.Fatal(err)
	}

	waitForNodeHeight(t, bob, 1)
	record := waitForBlockByHeight(t, bob, 1)
	if record.Hash != blockHash {
		t.Fatalf("expected bob to store committed block %x, got %x", blockHash, record.Hash)
	}
	nextProposal, _, proposed, err := bob.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !proposed || nextProposal.Block.Header.Height != 2 {
		t.Fatalf("expected bob to propose next height after commit gossip, proposed=%v proposal=%+v", proposed, nextProposal)
	}
	if nextProposal.JustifyQC.Height != quorumCert.Height || nextProposal.JustifyQC.BlockHash != quorumCert.BlockHash {
		t.Fatalf("expected commit gossip QC to seed next proposal highQC, got %+v want %+v", nextProposal.JustifyQC, quorumCert)
	}
}

func TestNodeLogsCommittedBlockWithRound(t *testing.T) {
	signer := deterministicSignerForID("alice")
	alice := newTestNodeWithSigner(t, signer)
	events := make(chan map[string]any, 1)
	alice.WithEventLogger(func(event string, fields map[string]any) {
		if event == "block_committed" {
			events <- fields
		}
	})
	startNode(t, alice)
	defer alice.Stop(context.Background())

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)
	block := testCommitBlock(t, alice, 1)
	block.Txs = []types.Tx{[]byte("bank:commit-log")}
	proposal, err := machine.CreateProposal(block, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	proposal = signedNodeTestProposal(t, signer, proposal)
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	blockHash := consensus.HashBlock(proposal.Block)
	vote := signedNodeTestVote(t, signer, "alice", 1, 0, blockHash)
	if err := machine.OnVote(context.Background(), vote); err != nil {
		t.Fatal(err)
	}
	quorumCert, err := machine.BuildQuorumCert(1, 0, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alice.CommitBlock(context.Background(), proposal.Block, quorumCert); err != nil {
		t.Fatal(err)
	}

	select {
	case fields := <-events:
		if fields["height"] != types.Height(1) || fields["round"] != types.Round(0) || fields["tx_count"] != 1 {
			t.Fatalf("unexpected commit log fields: %+v", fields)
		}
		if fields["block_hash"] == "" || fields["app_hash"] == "" {
			t.Fatalf("expected block/app hash fields: %+v", fields)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for block commit log")
	}
}

func forceTestQuorumCert(t *testing.T, machine *consensus.StateMachine, height types.Height, round types.Round, blockHash types.Hash) finality.QuorumCert {
	t.Helper()
	for _, validatorID := range []types.ValidatorID{"bob", "carol"} {
		vote := signedNodeTestVote(t, deterministicSignerForID(validatorID), validatorID, height, round, blockHash)
		if err := machine.OnVote(context.Background(), vote); err != nil {
			t.Fatal(err)
		}
	}
	qc, err := machine.BuildQuorumCert(height, round, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	return qc
}

func TestNodeGossipsConflictingVoteEvidenceAndSlashesValidator(t *testing.T) {
	alice, bob, carol := newSlashingNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	aliceConsensus, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	firstProposal, err := aliceConsensus.CreateProposal(types.Block{
		Header: types.Header{Height: 1},
		Txs:    []types.Tx{[]byte("bank:first-conflict-target")},
	}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.signConsensusProposal(&firstProposal); err != nil {
		t.Fatal(err)
	}
	if err := aliceConsensus.OnProposal(context.Background(), firstProposal); err != nil {
		t.Fatal(err)
	}
	secondProposal, err := aliceConsensus.CreateProposal(types.Block{
		Header: types.Header{Height: 1},
		Txs:    []types.Tx{[]byte("bank:second-conflict-target")},
	}, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.signConsensusProposal(&secondProposal); err != nil {
		t.Fatal(err)
	}
	if err := aliceConsensus.OnProposal(context.Background(), secondProposal); err != nil {
		t.Fatal(err)
	}
	firstHash := consensus.HashBlock(firstProposal.Block)
	secondHash := consensus.HashBlock(secondProposal.Block)

	bobReactor, err := bob.ConsensusReactor()
	if err != nil {
		t.Fatal(err)
	}
	firstVote := consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   firstHash,
		ValidatorID: "bob",
	}
	if err := bob.signConsensusVote(&firstVote); err != nil {
		t.Fatal(err)
	}
	if err := bobReactor.BroadcastVote(context.Background(), firstVote); err != nil {
		t.Fatal(err)
	}
	secondVote := consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   secondHash,
		ValidatorID: "bob",
	}
	if err := bob.signConsensusVote(&secondVote); err != nil {
		t.Fatal(err)
	}
	if err := bobReactor.BroadcastVote(context.Background(), secondVote); err != nil {
		t.Fatal(err)
	}
	evidence, err := consensus.NewConflictingVoteEvidence(firstVote, secondVote)
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err := alice.SubmitEvidence(context.Background(), evidence); err != nil || !applied {
		t.Fatalf("expected alice to submit conflicting vote evidence, applied=%t err=%v", applied, err)
	}

	waitForAppliedEvidence(t, alice, "bob")
	waitForValidatorPower(t, alice, "bob", 95)
	waitForValidatorPower(t, bob, "bob", 95)
	waitForValidatorPower(t, carol, "bob", 95)
	runtime, err := alice.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	index, err := runtime.Store.EvidenceIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 {
		t.Fatalf("expected persisted evidence index, got %d", len(index))
	}
	record, err := runtime.Store.EvidenceByKey(context.Background(), index[0])
	if err != nil {
		t.Fatal(err)
	}
	if record.Evidence.Validator != "bob" || !record.Applied {
		t.Fatalf("unexpected persisted evidence: %+v", record)
	}
}

func waitForAppliedEvidence(t *testing.T, node *Node, validatorID types.ValidatorID) {
	t.Helper()
	for attempt := 0; attempt < int(transportTestWaitTimeout/time.Millisecond); attempt++ {
		runtime, err := node.Runtime()
		if err == nil && runtime.Store != nil {
			index, err := runtime.Store.EvidenceIndex(context.Background())
			if err == nil {
				for _, key := range index {
					record, err := runtime.Store.EvidenceByKey(context.Background(), key)
					if err == nil && record.Evidence.Validator == validatorID && record.Applied {
						return
					}
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for applied evidence for %s", validatorID)
}

func TestNodePenalizesAndBansInvalidPeerMessages(t *testing.T) {
	alice, bob, _ := newScoredNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())

	bobWire, ok := bob.wire.(transport.Transport)
	if !ok {
		t.Fatal("expected bob transport")
	}
	if err := bobWire.Publish(context.Background(), p2p.TopicTx, []byte{}); err != nil {
		t.Fatal(err)
	}
	waitForPeerScore(t, alice, "bob", 1)
	if err := bobWire.Publish(context.Background(), p2p.TopicEvidence, []byte("{bad-json")); err != nil {
		t.Fatal(err)
	}
	waitForPeerBanned(t, alice, "bob")

	if err := bobWire.Publish(context.Background(), p2p.TopicTx, []byte("bank:ignored-after-ban")); err != nil {
		t.Fatal(err)
	}
	waitForMempoolLen(t, alice, 0)
}

func TestNodeBansMaliciousFloodWithoutBlockingHonestPeer(t *testing.T) {
	alice, bob, carol := newScoredNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	bobWire, ok := bob.wire.(transport.Transport)
	if !ok {
		t.Fatal("expected bob transport")
	}
	for index := 0; index < 4; index++ {
		if err := bobWire.Publish(context.Background(), p2p.TopicEvidence, []byte("{malicious-flood")); err != nil {
			t.Fatal(err)
		}
	}
	waitForPeerBanned(t, alice, "bob")

	if err := carol.SubmitTx(context.Background(), []byte("bank:honest-after-flood")); err != nil {
		t.Fatal(err)
	}
	waitForMempoolLen(t, alice, 1)
	waitForPeerScore(t, alice, "carol", 4)
}

func TestNodeRewardsValidPeerMessages(t *testing.T) {
	alice, bob, _ := newScoredNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())

	if err := bob.SubmitTx(context.Background(), []byte("bank:valid-score")); err != nil {
		t.Fatal(err)
	}
	waitForPeerScore(t, alice, "bob", 4)
}

func TestNodeReportsPeerScoreSnapshots(t *testing.T) {
	alice, bob, carol := newScoredNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	if err := bob.SubmitTx(context.Background(), []byte("bank:snapshot-valid")); err != nil {
		t.Fatal(err)
	}
	waitForPeerScore(t, alice, "bob", 4)

	carolWire, ok := carol.wire.(transport.Transport)
	if !ok {
		t.Fatal("expected carol transport")
	}
	if err := carolWire.Publish(context.Background(), p2p.TopicTx, []byte{}); err != nil {
		t.Fatal(err)
	}
	waitForPeerScore(t, alice, "carol", 1)

	snapshot, err := alice.PeerScores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 peer snapshots, got %d", len(snapshot))
	}
	if snapshot[0].Peer != "bob" || snapshot[0].Score != 4 || snapshot[0].Banned {
		t.Fatalf("unexpected bob snapshot: %+v", snapshot[0])
	}
	if snapshot[1].Peer != "carol" || snapshot[1].Score != 1 || snapshot[1].Banned {
		t.Fatalf("unexpected carol snapshot: %+v", snapshot[1])
	}
	status := alice.Status(context.Background())
	if status.PeerCount != 2 || status.BannedPeers != 0 || len(status.Peers) != 2 {
		t.Fatalf("unexpected peer status: %+v", status)
	}
	if status.Peers[0].Peer != "bob" || status.Peers[1].Peer != "carol" {
		t.Fatalf("unexpected status peer ordering: %+v", status.Peers)
	}
}

func TestNodeDropsRateLimitedPeerMessages(t *testing.T) {
	alice, bob, _ := newRateLimitedNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())

	if err := bob.SubmitTx(context.Background(), []byte("bank:first")); err != nil {
		t.Fatal(err)
	}
	waitForMempoolLen(t, alice, 1)
	waitForPeerScore(t, alice, "bob", 11)

	if err := bob.SubmitTx(context.Background(), []byte("bank:dropped-by-rate-limit")); err != nil {
		t.Fatal(err)
	}
	waitForPeerScore(t, alice, "bob", 6)
	waitForMempoolLen(t, alice, 1)
}

func TestNodeResetsPeerRateLimitWindow(t *testing.T) {
	alice, bob, _ := newAutoResetRateLimitedNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())

	if err := bob.SubmitTx(context.Background(), []byte("bank:first-window")); err != nil {
		t.Fatal(err)
	}
	waitForMempoolLen(t, alice, 1)

	waitForPeerWindowReset(t, alice, "bob")
	if err := bob.SubmitTx(context.Background(), []byte("bank:accepted-after-reset")); err != nil {
		t.Fatal(err)
	}
	waitForMempoolLen(t, alice, 2)
}

func TestNodeDisconnectsPeerWhenScoreBanApplies(t *testing.T) {
	wire := newDisconnectRecordingTransport("alice")
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	cfg.Chain.P2P.InitialScore = 1
	cfg.Chain.P2P.InvalidMessageCost = 2
	cfg.Chain.P2P.BanThreshold = 0
	cfg.Chain.P2P.BanDuration = time.Hour
	node, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithSigner(deterministicSignerForID("alice"))
	node.WithTransport(wire)
	startNode(t, node)
	defer node.Stop(context.Background())

	if node.observePeerMessage(context.Background(), "bob", false) {
		t.Fatal("expected banned peer observation to return false")
	}
	if !wire.disconnectedPeer("bob") {
		t.Fatalf("expected bob disconnected, got %+v", wire.disconnected)
	}
}

func TestNodeTreatsFutureCommitGossipAsValidRace(t *testing.T) {
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	cfg.Chain.P2P.InitialScore = 2
	cfg.Chain.P2P.ValidMessageReward = 2
	cfg.Chain.P2P.InvalidMessageCost = 5
	cfg.Chain.P2P.BanThreshold = 0
	node, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithSigner(deterministicSignerForID("alice"))
	startNode(t, node)
	defer node.Stop(context.Background())

	block := testCommitBlock(t, node, 2)
	blockHash := consensus.HashBlock(block)
	data, err := encodeCommitMessage(block, finality.QuorumCert{
		Height:    2,
		Round:     0,
		BlockHash: blockHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	node.acceptCommitMessage(context.Background(), "bob", data)

	score, err := node.PeerScore(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	if score != 4 {
		t.Fatalf("expected future commit race to be rewarded as valid, got score %d", score)
	}
	if status := node.Status(context.Background()); status.LatestHeight != 0 {
		t.Fatalf("future commit must not mutate local state, got %+v", status)
	}
}

func TestNodeTreatsInvalidCommitProofAsNonBanningRace(t *testing.T) {
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	cfg.Chain.P2P.InitialScore = 2
	cfg.Chain.P2P.ValidMessageReward = 2
	cfg.Chain.P2P.InvalidMessageCost = 5
	cfg.Chain.P2P.BanThreshold = 0
	node, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithSigner(deterministicSignerForID("alice"))
	startNode(t, node)
	defer node.Stop(context.Background())

	block := testCommitBlock(t, node, 1)
	data, err := encodeCommitMessage(block, finality.QuorumCert{
		Height:    1,
		Round:     0,
		BlockHash: types.Hash{9},
	})
	if err != nil {
		t.Fatal(err)
	}
	node.acceptCommitMessage(context.Background(), "bob", data)

	score, err := node.PeerScore(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	if score != 4 {
		t.Fatalf("expected invalid commit proof to avoid peer ban, got score %d", score)
	}
	if status := node.Status(context.Background()); status.LatestHeight != 0 {
		t.Fatalf("invalid commit proof must not mutate local state, got %+v", status)
	}
}

func TestNodePersistsPeerScoresAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	cfg := DefaultConfig("vexo-test", dataDir)
	cfg.ValidatorID = "alice"
	cfg.Chain.P2P.InitialScore = 1
	cfg.Chain.P2P.InvalidMessageCost = 2
	cfg.Chain.P2P.BanThreshold = 0
	cfg.Chain.P2P.BanDuration = time.Hour
	first, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	first.WithSigner(deterministicSignerForID("alice"))
	startNode(t, first)
	if first.observePeerMessage(context.Background(), "bob", false) {
		t.Fatal("expected bob to be banned")
	}
	if err := first.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	second, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	second.WithSigner(deterministicSignerForID("alice"))
	startNode(t, second)
	defer second.Stop(context.Background())
	runtime, err := second.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	banned, err := runtime.P2PScore.IsBanned(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Fatal("expected restored bob ban")
	}
}

func TestNodeBackgroundConsensusLoopCommitsAcrossPeers(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	loopConfig := ConsensusLoopConfig{Interval: 10 * time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1024, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeQC, AllowUnsafeQCCommit: true}
	for _, node := range []*Node{alice, bob, carol} {
		if err := node.StartConsensusLoop(context.Background(), loopConfig); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
		if err := node.StartConsensusLoop(context.Background(), loopConfig); !errors.Is(err, ErrLoopAlreadyRunning) {
			t.Fatalf("expected loop already running, got %v", err)
		}
	}

	waitForNodeHeight(t, alice, 1)
	waitForNodeHeight(t, bob, 1)
	waitForNodeHeight(t, carol, 1)

	for _, node := range []*Node{alice, bob, carol} {
		if !node.ConsensusLoopRunning() {
			t.Fatal("expected consensus loop to be running")
		}
		if err := node.StopConsensusLoop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if node.ConsensusLoopRunning() {
			t.Fatal("expected consensus loop to stop")
		}
		if err := node.StopConsensusLoop(context.Background()); !errors.Is(err, ErrLoopNotRunning) {
			t.Fatalf("expected loop not running, got %v", err)
		}
	}
}

func TestNodeTimeoutRoundBroadcastsAndAdvancesPeers(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	for _, node := range []*Node{alice, bob, carol} {
		machine, err := node.Consensus()
		if err != nil {
			t.Fatal(err)
		}
		machine.StartRound(1, 0)
	}

	for _, node := range []*Node{alice, bob, carol} {
		if _, _, err := node.TimeoutRound(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	for _, node := range []*Node{alice, bob, carol} {
		machine, err := node.Consensus()
		if err != nil {
			t.Fatal(err)
		}
		waitForConsensusStatus(t, machine, func(status consensus.Status) bool {
			return status.Height == 1 && status.Round >= 1
		})
	}
}

func TestNodeTimeoutRoundRebroadcastsCachedVoteWithoutDuplicateWAL(t *testing.T) {
	alice, _, _ := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	if _, _, err := alice.TimeoutRound(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstWAL, err := os.ReadFile(alice.cfg.ConsensusWALPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := alice.TimeoutRound(context.Background()); err != nil {
		t.Fatal(err)
	}
	secondWAL, err := os.ReadFile(alice.cfg.ConsensusWALPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(firstWAL) != string(secondWAL) {
		t.Fatal("expected cached timeout vote rebroadcast without duplicate WAL append")
	}
}

func TestNodeConsensusLoopCommitsEmptyBlocks(t *testing.T) {
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
	alice := newConsensusLoopNode(t, bus, genesis, "alice")
	startNode(t, alice)
	defer alice.Stop(context.Background())

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	loopConfig := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1024, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeQC, AllowUnsafeQCCommit: true}
	if err := alice.StartConsensusLoop(context.Background(), loopConfig); err != nil {
		t.Fatal(err)
	}
	defer alice.StopConsensusLoop(context.Background())

	waitForNodeHeight(t, alice, 1)
	status := alice.Status(context.Background())
	if status.LatestHeight < 1 {
		t.Fatalf("expected empty block commit, got %+v", status)
	}
}

func TestNodeConsensusLoopSkipsEmptyBlocksWhenDisabled(t *testing.T) {
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
	alice := newConsensusLoopNode(t, bus, genesis, "alice")
	startNode(t, alice)
	defer alice.Stop(context.Background())

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	loopConfig := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1024, CreateEmptyBlocks: false, ExecutionCommitMode: ExecutionCommitModeQC, AllowUnsafeQCCommit: true}
	if err := alice.StartConsensusLoop(context.Background(), loopConfig); err != nil {
		t.Fatal(err)
	}
	defer alice.StopConsensusLoop(context.Background())

	time.Sleep(25 * time.Millisecond)
	if status := alice.Status(context.Background()); status.LatestHeight != 0 {
		t.Fatalf("expected no empty block commits, got %+v", status)
	}
	if err := alice.SubmitTx(context.Background(), []byte("bank:tx-only-block")); err != nil {
		t.Fatal(err)
	}
	waitForNodeHeight(t, alice, 1)
}

func TestNodeConsensusLoopFinalizedModeProgressesTxWithEmptyBlocksDisabled(t *testing.T) {
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
	alice := newConsensusLoopNode(t, bus, genesis, "alice")
	startNode(t, alice)
	defer alice.Stop(context.Background())

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	loopConfig := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1024, CreateEmptyBlocks: false, ExecutionCommitMode: ExecutionCommitModeFinalized}
	if err := alice.StartConsensusLoop(context.Background(), loopConfig); err != nil {
		t.Fatal(err)
	}
	defer alice.StopConsensusLoop(context.Background())

	time.Sleep(25 * time.Millisecond)
	if status := alice.Status(context.Background()); status.LatestHeight != 0 {
		t.Fatalf("expected no gratuitous empty block commits, got %+v", status)
	}
	if err := alice.SubmitTx(context.Background(), []byte("bank:finalized-tx-only-block")); err != nil {
		t.Fatal(err)
	}
	waitForNodeHeight(t, alice, 1)
	if status := alice.Status(context.Background()); status.LatestHeight != 1 {
		t.Fatalf("expected only tx block to commit, got %+v", status)
	}
}

func TestNodeStepFinalizedModeProgressesTxWithEmptyBlocksDisabled(t *testing.T) {
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
	alice := newConsensusLoopNode(t, bus, genesis, "alice")
	startNode(t, alice)
	defer alice.Stop(context.Background())

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)
	if err := alice.SubmitTx(context.Background(), []byte("bank:finalized-step")); err != nil {
		t.Fatal(err)
	}

	loopConfig := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1024, CreateEmptyBlocks: false, ExecutionCommitMode: ExecutionCommitModeFinalized}
	for step := 0; step < 8; step++ {
		result, err := alice.StepConsensusWithConfig(context.Background(), loopConfig)
		if err != nil {
			t.Fatalf("step %d failed: %v status=%+v", step, err, machine.Status(context.Background()))
		}
		if result.Committed {
			break
		}
	}
	if status := alice.Status(context.Background()); status.LatestHeight != 1 {
		t.Fatalf("expected tx block commit through finality progress, got %+v machine=%+v decisions=%+v", status, machine.Status(context.Background()), machine.CommitDecisions())
	}
}

func TestNodeSkippedProposerRecoversOnNextRound(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	if err := bob.SubmitTx(context.Background(), []byte("bank:recover-after-skip")); err != nil {
		t.Fatal(err)
	}
	bobMachine, err := bob.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	bobMachine.StartRound(1, 0)
	if _, _, proposed, err := bob.TickConsensus(context.Background(), 1024); err != nil || proposed {
		t.Fatalf("expected bob not to propose in round 0, proposed=%v err=%v", proposed, err)
	}

	for _, node := range []*Node{alice, bob, carol} {
		machine, err := node.Consensus()
		if err != nil {
			t.Fatal(err)
		}
		machine.StartRound(1, 0)
	}
	for _, node := range []*Node{alice, bob, carol} {
		if _, _, err := node.TimeoutRound(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	waitForConsensusStatus(t, bobMachine, func(status consensus.Status) bool {
		return status.Height == 1 && status.Round >= 1 && status.Phase == consensus.PhasePropose
	})

	proposal, blockHash, proposed, err := bob.TickConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !proposed || proposal.Round != 1 || blockHash == (types.Hash{}) {
		t.Fatalf("expected bob to recover liveness as round 1 proposer, proposed=%v proposal=%+v hash=%x", proposed, proposal, blockHash)
	}
}

func newTransportNodes(t *testing.T) (*Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	aliceWire, err := bus.NewPeer(p2p.PeerID("alice"))
	if err != nil {
		t.Fatal(err)
	}
	bobWire, err := bus.NewPeer(p2p.PeerID("bob"))
	if err != nil {
		t.Fatal(err)
	}
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("bob")},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("carol")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1, "bob": 1, "carol": 1},
	}
	alice, err := New(DefaultConfig("vexo-test", t.TempDir()), genesis, newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := New(DefaultConfig("vexo-test", t.TempDir()), genesis, newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	alice.WithTransport(aliceWire)
	alice.WithSigner(deterministicSignerForID("alice"))
	bob.WithTransport(bobWire)
	bob.WithSigner(deterministicSignerForID("bob"))
	return alice, bob
}

func newConsensusLoopNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("bob")},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("carol")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1, "bob": 1, "carol": 1},
	}
	alice := newConsensusLoopNode(t, bus, genesis, "alice")
	bob := newConsensusLoopNode(t, bus, genesis, "bob")
	carol := newConsensusLoopNode(t, bus, genesis, "carol")
	return alice, bob, carol
}

func newConsensusLoopNode(t *testing.T, bus *transport.InMemoryBus, genesis Genesis, validatorID types.ValidatorID) *Node {
	t.Helper()
	wire, err := bus.NewPeer(p2p.PeerID(validatorID))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = validatorID
	node, err := New(cfg, genesis, newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithTransport(wire)
	node.WithSigner(deterministicSignerForID(validatorID))
	return node
}

func newSlashingNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100, PublicKey: deterministicPublicKeyForID("alice")},
			{ID: "bob", Address: "bob", VotingPower: 100, Stake: 100, PublicKey: deterministicPublicKeyForID("bob")},
			{ID: "carol", Address: "carol", VotingPower: 100, Stake: 100, PublicKey: deterministicPublicKeyForID("carol")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 100, "bob": 100, "carol": 100},
	}
	alice := newConsensusLoopNode(t, bus, genesis, "alice")
	bob := newConsensusLoopNode(t, bus, genesis, "bob")
	carol := newConsensusLoopNode(t, bus, genesis, "carol")
	return alice, bob, carol
}

func newScoredNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("bob")},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("carol")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1, "bob": 1, "carol": 1},
	}
	alice := newScoredNode(t, bus, genesis, "alice")
	bob := newScoredNode(t, bus, genesis, "bob")
	carol := newScoredNode(t, bus, genesis, "carol")
	return alice, bob, carol
}

func newScoredNode(t *testing.T, bus *transport.InMemoryBus, genesis Genesis, validatorID types.ValidatorID) *Node {
	t.Helper()
	wire, err := bus.NewPeer(p2p.PeerID(validatorID))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = validatorID
	cfg.Chain.P2P.InitialScore = 2
	cfg.Chain.P2P.ValidMessageReward = 2
	cfg.Chain.P2P.InvalidMessageCost = 1
	cfg.Chain.P2P.BanThreshold = 0
	cfg.Chain.P2P.WindowResetInterval = time.Hour
	cfg.Chain.P2P.ScoreRecovery = 0
	node, err := New(cfg, genesis, newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithTransport(wire)
	node.WithSigner(deterministicSignerForID(validatorID))
	return node
}

func newRateLimitedNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("bob")},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("carol")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1, "bob": 1, "carol": 1},
	}
	alice := newRateLimitedNode(t, bus, genesis, "alice")
	bob := newRateLimitedNode(t, bus, genesis, "bob")
	carol := newRateLimitedNode(t, bus, genesis, "carol")
	return alice, bob, carol
}

func newRateLimitedNode(t *testing.T, bus *transport.InMemoryBus, genesis Genesis, validatorID types.ValidatorID) *Node {
	t.Helper()
	wire, err := bus.NewPeer(p2p.PeerID(validatorID))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = validatorID
	cfg.Chain.P2P.InitialScore = 10
	cfg.Chain.P2P.ValidMessageReward = 1
	cfg.Chain.P2P.InvalidMessageCost = 1
	cfg.Chain.P2P.RateLimitCost = 5
	cfg.Chain.P2P.BanThreshold = 0
	cfg.Chain.P2P.MaxMessagesPerWindow = 1
	cfg.Chain.P2P.WindowResetInterval = time.Hour
	node, err := New(cfg, genesis, newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithTransport(wire)
	node.WithSigner(deterministicSignerForID(validatorID))
	return node
}

func newAutoResetRateLimitedNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	alice, bob, carol := newRateLimitedNodes(t)
	for _, node := range []*Node{alice, bob, carol} {
		node.cfg.Chain.P2P.WindowResetInterval = time.Nanosecond
	}
	return alice, bob, carol
}

func startNode(t *testing.T, node *Node) {
	t.Helper()
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func waitForConsensusStatus(t *testing.T, machine *consensus.StateMachine, match func(consensus.Status) bool) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		if match(machine.Status(context.Background())) {
			return
		}
		yieldTransportTest()
	}
	finalStatus := machine.Status(context.Background())
	if match(finalStatus) {
		return
	}
	t.Fatalf("timed out waiting for consensus status, got %+v", finalStatus)
}

func waitForQuorumInput(t *testing.T, machine *consensus.StateMachine, blockHash types.Hash) {
	t.Helper()
	vote := consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   blockHash,
		ValidatorID: "bob",
	}
	if err := signConsensusVote("vexo-test", deterministicSignerForID("bob"), &vote); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		_ = machine.OnVote(context.Background(), vote)
		if _, err := machine.BuildQuorumCert(1, 0, blockHash); err == nil {
			return
		}
		yieldTransportTest()
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); err == nil {
		return
	}
	t.Fatal("timed out waiting for vote input")
}

func waitForQuorumCert(t *testing.T, machine *consensus.StateMachine, height types.Height, round types.Round, blockHash types.Hash) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		if _, err := machine.BuildQuorumCert(height, round, blockHash); err == nil {
			return
		}
		yieldTransportTest()
	}
	if _, err := machine.BuildQuorumCert(height, round, blockHash); err == nil {
		return
	}
	t.Fatal("timed out waiting for quorum cert")
}

func waitForMempoolLen(t *testing.T, node *Node, expected int) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		runtime, err := node.Runtime()
		if err == nil && runtime.Mempool.Len() == expected {
			return
		}
		yieldTransportTest()
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if actual := runtime.Mempool.Len(); actual != expected {
		t.Fatalf("expected mempool len %d, got %d", expected, actual)
	}
}

func waitForNodeHeight(t *testing.T, node *Node, height types.Height) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		status := node.Status(context.Background())
		if status.LatestHeight >= height {
			return
		}
		yieldTransportTest()
	}
	status := node.Status(context.Background())
	if status.LatestHeight >= height {
		return
	}
	t.Fatalf("timed out waiting for node height %d, got %+v", height, status)
}

func waitForBlockByHeight(t *testing.T, node *Node, height types.Height) store.BlockRecord {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		record, err := node.runtime.BlockByHeight(context.Background(), height)
		if err == nil {
			return record
		}
		yieldTransportTest()
	}
	record, err := node.runtime.BlockByHeight(context.Background(), height)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func waitForValidatorPower(t *testing.T, node *Node, validatorID types.ValidatorID, expected types.VotingPower) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		runtime, err := node.Runtime()
		if err == nil {
			for _, height := range []types.Height{0, 1} {
				set, err := runtime.Validators.ValidatorSet(context.Background(), height)
				if err == nil {
					validatorInfo, found := set.Get(validatorID)
					if found && validatorInfo.VotingPower == expected {
						return
					}
				}
			}
		}
		yieldTransportTest()
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	set, err := runtime.Validators.ValidatorSet(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	validatorInfo, found := set.Get(validatorID)
	if !found {
		t.Fatalf("validator %s not found", validatorID)
	}
	if validatorInfo.VotingPower != expected {
		t.Fatalf("expected validator %s power %d, got %d", validatorID, expected, validatorInfo.VotingPower)
	}
}

func waitForPeerScore(t *testing.T, node *Node, peer p2p.PeerID, expected int64) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		score, err := node.PeerScore(context.Background(), peer)
		if err == nil && score == expected {
			return
		}
		yieldTransportTest()
	}
	score, err := node.PeerScore(context.Background(), peer)
	if err != nil {
		t.Fatal(err)
	}
	if score != expected {
		t.Fatalf("expected peer %s score %d, got %d", peer, expected, score)
	}
}

func waitForPeerBanned(t *testing.T, node *Node, peer p2p.PeerID) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		runtime, err := node.Runtime()
		if err == nil {
			banned, err := runtime.P2PScore.IsBanned(context.Background(), peer)
			if err == nil && banned {
				return
			}
		}
		yieldTransportTest()
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	banned, err := runtime.P2PScore.IsBanned(context.Background(), peer)
	if err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Fatalf("expected peer %s banned", peer)
	}
}

func waitForPeerWindowReset(t *testing.T, node *Node, peer p2p.PeerID) {
	t.Helper()
	for attempt := 0; attempt < transportTestWaitAttempts; attempt++ {
		runtime, err := node.Runtime()
		if err == nil {
			messages, err := runtime.P2PScore.WindowMessages(context.Background(), peer)
			if err == nil && messages == 0 {
				return
			}
		}
		yieldTransportTest()
	}
	t.Fatalf("timed out waiting for peer %s score window reset", peer)
}

func yieldTransportTest() {
	runtime.Gosched()
	time.Sleep(time.Millisecond)
}

type disconnectRecordingTransport struct {
	peerID       p2p.PeerID
	mu           sync.Mutex
	started      bool
	disconnected map[p2p.PeerID]bool
	gates        []func(context.Context, p2p.PeerID) error
}

func newDisconnectRecordingTransport(peerID p2p.PeerID) *disconnectRecordingTransport {
	return &disconnectRecordingTransport{
		peerID:       peerID,
		disconnected: make(map[p2p.PeerID]bool),
	}
}

func (wire *disconnectRecordingTransport) Start(ctx context.Context) error {
	wire.mu.Lock()
	defer wire.mu.Unlock()
	wire.started = true
	return nil
}

func (wire *disconnectRecordingTransport) Stop(ctx context.Context) error {
	wire.mu.Lock()
	defer wire.mu.Unlock()
	wire.started = false
	return nil
}

func (wire *disconnectRecordingTransport) Publish(ctx context.Context, topic p2p.Topic, data []byte) error {
	return nil
}

func (wire *disconnectRecordingTransport) Send(ctx context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	return nil
}

func (wire *disconnectRecordingTransport) Subscribe(ctx context.Context, topic p2p.Topic) (<-chan transport.Envelope, error) {
	return make(chan transport.Envelope), nil
}

func (wire *disconnectRecordingTransport) PeerID() p2p.PeerID {
	return wire.peerID
}

func (wire *disconnectRecordingTransport) DisconnectPeer(peerID p2p.PeerID) {
	wire.mu.Lock()
	defer wire.mu.Unlock()
	wire.disconnected[peerID] = true
}

func (wire *disconnectRecordingTransport) SetPeerGate(gate func(context.Context, p2p.PeerID) error) {
	wire.mu.Lock()
	defer wire.mu.Unlock()
	wire.gates = nil
	if gate != nil {
		wire.gates = append(wire.gates, gate)
	}
}

func (wire *disconnectRecordingTransport) AddPeerGate(gate func(context.Context, p2p.PeerID) error) {
	if gate == nil {
		return
	}
	wire.mu.Lock()
	defer wire.mu.Unlock()
	wire.gates = append(wire.gates, gate)
}

func (wire *disconnectRecordingTransport) disconnectedPeer(peerID p2p.PeerID) bool {
	wire.mu.Lock()
	defer wire.mu.Unlock()
	return wire.disconnected[peerID]
}

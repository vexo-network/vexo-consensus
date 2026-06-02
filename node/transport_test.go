package node

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const transportTestWaitTimeout = 30 * time.Second

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
	if err := aliceReactor.BroadcastVote(context.Background(), consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   blockHash,
		ValidatorID: "alice",
	}); err != nil {
		t.Fatal(err)
	}

	waitForQuorumInput(t, bobConsensus, blockHash)
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
	if _, ok, err := alice.VoteBlock(context.Background(), proposal.Block.Header.Height, proposal.Round, blockHash); err != nil || ok {
		t.Fatalf("local vote should not have quorum before peer votes: ok=%v err=%v", ok, err)
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
	if !fairordering.IsOrdered(proposal.Block.Txs) {
		t.Fatalf("expected deterministic proposal ordering, got %q", proposal.Block.Txs)
	}
	if _, ok, err := alice.VoteBlock(context.Background(), proposal.Block.Header.Height, proposal.Round, blockHash); err != nil || ok {
		t.Fatalf("local vote should not have quorum before peer votes: ok=%v err=%v", ok, err)
	}

	waitForQuorumCert(t, aliceConsensus, proposal.Block.Header.Height, proposal.Round, blockHash)
	quorumCert, err := aliceConsensus.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash)
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

	result, committed, err := bob.CommitReadyBlock(context.Background())
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
	if _, committed, err := bob.CommitReadyBlock(context.Background()); err != nil || committed {
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
	proposeStep, err := alice.StepConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if proposeStep.Committed || !proposeStep.Proposed {
		t.Fatalf("expected proposal step only, got %+v", proposeStep)
	}
	waitForQuorumCert(t, bobConsensus, proposeStep.Proposal.Block.Header.Height, proposeStep.Proposal.Round, proposeStep.BlockHash)

	commitStep, err := bob.StepConsensus(context.Background(), 1024)
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
	emptyStep, err := bob.StepConsensus(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if emptyStep.Committed || emptyStep.Proposed {
		t.Fatalf("expected idle step after commit, got %+v", emptyStep)
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
		waitForQuorumCert(t, aliceConsensus, proposal.Block.Header.Height, proposal.Round, blockHash)
		quorumCert, err = aliceConsensus.BuildQuorumCert(proposal.Block.Header.Height, proposal.Round, blockHash)
		if err != nil {
			t.Fatal(err)
		}
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
	record, err := bob.runtime.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if record.Hash != blockHash {
		t.Fatalf("expected bob to store committed block %x, got %x", blockHash, record.Hash)
	}
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
	if err := aliceConsensus.OnProposal(context.Background(), secondProposal); err != nil {
		t.Fatal(err)
	}
	firstHash := consensus.HashBlock(firstProposal.Block)
	secondHash := consensus.HashBlock(secondProposal.Block)

	bobReactor, err := bob.ConsensusReactor()
	if err != nil {
		t.Fatal(err)
	}
	if err := bobReactor.BroadcastVote(context.Background(), consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   firstHash,
		ValidatorID: "bob",
	}); err != nil {
		t.Fatal(err)
	}
	if err := bobReactor.BroadcastVote(context.Background(), consensus.Vote{
		Height:      1,
		Round:       0,
		BlockHash:   secondHash,
		ValidatorID: "bob",
	}); err != nil {
		t.Fatal(err)
	}

	waitForValidatorPower(t, alice, "bob", 95)
	waitForValidatorPower(t, bob, "bob", 95)
	waitForValidatorPower(t, carol, "bob", 95)
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

func TestNodeBackgroundConsensusLoopCommitsAcrossPeers(t *testing.T) {
	alice, bob, carol := newConsensusLoopNodes(t)
	startNode(t, alice)
	defer alice.Stop(context.Background())
	startNode(t, bob)
	defer bob.Stop(context.Background())
	startNode(t, carol)
	defer carol.Stop(context.Background())

	loopConfig := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1024}
	for _, node := range []*Node{alice, bob, carol} {
		if err := node.StartConsensusLoop(context.Background(), loopConfig); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
		if err := node.StartConsensusLoop(context.Background(), loopConfig); !errors.Is(err, ErrLoopAlreadyRunning) {
			t.Fatalf("expected loop already running, got %v", err)
		}
	}

	if err := alice.SubmitTx(context.Background(), []byte("bank:background-loop")); err != nil {
		t.Fatal(err)
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

func TestNodeConsensusLoopAdvancesRoundAfterTimeout(t *testing.T) {
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

	loopConfig := ConsensusLoopConfig{
		Interval:      time.Millisecond,
		RoundTimeout:  time.Nanosecond,
		MaxBlockBytes: 1024,
	}
	for _, node := range []*Node{alice, bob, carol} {
		if err := node.StartConsensusLoop(context.Background(), loopConfig); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
	}

	for _, node := range []*Node{alice, bob, carol} {
		machine, err := node.Consensus()
		if err != nil {
			t.Fatal(err)
		}
		waitForConsensusStatus(t, machine, func(status consensus.Status) bool {
			return status.Height == 1 && status.Round >= 1 && status.Phase == consensus.PhasePropose
		})
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
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1},
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
	bob.WithTransport(bobWire)
	return alice, bob
}

func newConsensusLoopNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1},
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
	return node
}

func newSlashingNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100},
			{ID: "bob", Address: "bob", VotingPower: 100, Stake: 100},
			{ID: "carol", Address: "carol", VotingPower: 100, Stake: 100},
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
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1},
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
	return node
}

func newRateLimitedNodes(t *testing.T) (*Node, *Node, *Node) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
			{ID: "bob", Address: "bob", VotingPower: 1, Stake: 1},
			{ID: "carol", Address: "carol", VotingPower: 1, Stake: 1},
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
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		if match(machine.Status(context.Background())) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	finalStatus := machine.Status(context.Background())
	if match(finalStatus) {
		return
	}
	t.Fatalf("timed out waiting for consensus status, got %+v", finalStatus)
}

func waitForQuorumInput(t *testing.T, machine *consensus.StateMachine, blockHash types.Hash) {
	t.Helper()
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		_ = machine.OnVote(context.Background(), consensus.Vote{
			Height:      1,
			Round:       0,
			BlockHash:   blockHash,
			ValidatorID: "bob",
		})
		if _, err := machine.BuildQuorumCert(1, 0, blockHash); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); err == nil {
		return
	}
	t.Fatal("timed out waiting for vote input")
}

func waitForQuorumCert(t *testing.T, machine *consensus.StateMachine, height types.Height, round types.Round, blockHash types.Hash) {
	t.Helper()
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		if _, err := machine.BuildQuorumCert(height, round, blockHash); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := machine.BuildQuorumCert(height, round, blockHash); err == nil {
		return
	}
	t.Fatal("timed out waiting for quorum cert")
}

func waitForMempoolLen(t *testing.T, node *Node, expected int) {
	t.Helper()
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		runtime, err := node.Runtime()
		if err == nil && runtime.Mempool.Len() == expected {
			return
		}
		time.Sleep(time.Millisecond)
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
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		status := node.Status(context.Background())
		if status.LatestHeight >= height {
			return
		}
		time.Sleep(time.Millisecond)
	}
	status := node.Status(context.Background())
	if status.LatestHeight >= height {
		return
	}
	t.Fatalf("timed out waiting for node height %d, got %+v", height, status)
}

func waitForValidatorPower(t *testing.T, node *Node, validatorID types.ValidatorID, expected types.VotingPower) {
	t.Helper()
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		runtime, err := node.Runtime()
		if err == nil {
			set, err := runtime.Validators.ValidatorSet(context.Background(), 1)
			if err == nil {
				validatorInfo, found := set.Get(validatorID)
				if found && validatorInfo.VotingPower == expected {
					return
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	set, err := runtime.Validators.ValidatorSet(context.Background(), 1)
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
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		score, err := node.PeerScore(context.Background(), peer)
		if err == nil && score == expected {
			return
		}
		time.Sleep(time.Millisecond)
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
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		runtime, err := node.Runtime()
		if err == nil {
			banned, err := runtime.P2PScore.IsBanned(context.Background(), peer)
			if err == nil && banned {
				return
			}
		}
		time.Sleep(time.Millisecond)
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
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		runtime, err := node.Runtime()
		if err == nil {
			messages, err := runtime.P2PScore.WindowMessages(context.Background(), peer)
			if err == nil && messages == 0 {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for peer %s score window reset", peer)
}

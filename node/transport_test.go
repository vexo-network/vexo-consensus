package node

import (
	"context"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
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
		return status.Height == 1 && status.Phase == consensus.PhaseVote
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

func startNode(t *testing.T, node *Node) {
	t.Helper()
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func waitForConsensusStatus(t *testing.T, machine *consensus.StateMachine, match func(consensus.Status) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
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
	deadline := time.Now().Add(time.Second)
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
	deadline := time.Now().Add(time.Second)
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

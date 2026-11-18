package node

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

// sentMessage is one message a node sent to a single peer.
type sentMessage struct {
	to    p2p.PeerID
	topic p2p.Topic
	data  []byte
}

// sendRecordingTransport records what a node sends to single peers, so a test can carry it to the
// node it was meant for.
type sendRecordingTransport struct {
	*disconnectRecordingTransport

	sendMu sync.Mutex
	sent   []sentMessage
}

func newSendRecordingTransport(peerID p2p.PeerID) *sendRecordingTransport {
	return &sendRecordingTransport{disconnectRecordingTransport: newDisconnectRecordingTransport(peerID)}
}

func (wire *sendRecordingTransport) Send(_ context.Context, to p2p.PeerID, topic p2p.Topic, data []byte) error {
	wire.sendMu.Lock()
	defer wire.sendMu.Unlock()
	wire.sent = append(wire.sent, sentMessage{to: to, topic: topic, data: append([]byte(nil), data...)})
	return nil
}

// take returns the messages sent since the last call.
func (wire *sendRecordingTransport) take() []sentMessage {
	wire.sendMu.Lock()
	defer wire.sendMu.Unlock()
	sent := wire.sent
	wire.sent = nil
	return sent
}

// onlyCommitMessage returns the one commit message sent since the last call.
func onlyCommitMessage(t *testing.T, wire *sendRecordingTransport, to p2p.PeerID) commitMessage {
	t.Helper()
	sent := wire.take()
	if len(sent) != 1 || sent[0].to != to || sent[0].topic != p2p.TopicCommit {
		t.Fatalf("expected one commit message to %s, got %+v", to, sent)
	}
	message, err := decodeCommitMessage(sent[0].data)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

// A node that missed a block for good, because the announcement came while it was further behind and
// every validator announces a commit once, catches up from the announcements of later heights: once
// it stayed at its height for commitCatchUpDelay it asks the announcer for the next block, which the
// announcer serves from its store, and it asks for each following block as soon as it committed the
// one before. A node only briefly behind asks for nothing: it commits through consensus.
func TestALaggingNodeFetchesTheBlocksItMissedFromThePeersAhead(t *testing.T) {
	ctx := context.Background()
	serverWire := newSendRecordingTransport("server")
	server := newCommitFixtureOn(t, serverWire)
	laggerWire := newSendRecordingTransport("lagger")
	lagger := newCommitFixtureOn(t, laggerWire)

	var blocks []types.Block
	var parent types.Hash
	for height := types.Height(1); height <= 4; height++ {
		block := server.block(height, parent, types.Tx("bank:send:catch-up-"+string(rune('a'+height))))
		if reached, _ := server.deliver(server.commit(block)); reached != height {
			t.Fatalf("expected the server to commit height %d, it is at %d", height, reached)
		}
		blocks = append(blocks, block)
		parent = consensus.HashBlock(block)
	}
	if reached, _ := lagger.deliver(lagger.commit(blocks[0])); reached != 1 {
		t.Fatalf("expected the lagging node to commit height 1, it is at %d", reached)
	}
	serverWire.take()
	laggerWire.take()

	// The announcement of height 3 tells the lagging node that it is behind, but it waits for its own
	// consensus to commit height 2 first.
	announcement := appendCommitMessage(nil, server.commit(blocks[2]))
	lagger.node.acceptCommitMessage(ctx, "server", announcement)
	if sent := laggerWire.take(); len(sent) != 0 {
		t.Fatalf("expected a node just behind to ask for nothing, it sent %+v", sent)
	}

	// Still at height 1 a while later, it asks the announcer for height 2.
	lagger.node.commits.mu.Lock()
	lagger.node.commits.lagSince = lagger.node.commits.lagSince.Add(-commitCatchUpDelay)
	lagger.node.commits.mu.Unlock()
	lagger.node.acceptCommitMessage(ctx, "server", announcement)
	want := onlyCommitMessage(t, laggerWire, "server")
	if want.Want != 2 {
		t.Fatalf("expected the lagging node to want height 2, it wants %d", want.Want)
	}

	// The server answers from its store, and each commit makes the lagging node ask for the next.
	for height := types.Height(2); height <= 3; height++ {
		server.node.acceptCommitMessage(ctx, "lagger", appendCommitMessage(nil, commitMessage{Want: height}))
		whole := onlyCommitMessage(t, serverWire, "lagger")
		if whole.Bodiless || whole.Block.Header.Height != height || consensus.HashBlock(whole.Block) != consensus.HashBlock(blocks[height-1]) {
			t.Fatalf("expected the server to serve the whole block of height %d, got %+v", height, whole.Block.Header)
		}
		lagger.node.acceptCommitMessage(ctx, "server", appendCommitMessage(nil, whole))
		if reached := lagger.node.Status(ctx).LatestHeight; reached != height {
			t.Fatalf("expected the lagging node to commit the served height %d, it is at %d", height, reached)
		}
		sent := laggerWire.take()
		if height < 3 {
			if len(sent) != 1 {
				t.Fatalf("expected the lagging node to ask for height %d at once, it sent %+v", height+1, sent)
			}
			if next, err := decodeCommitMessage(sent[0].data); err != nil || next.Want != height+1 {
				t.Fatalf("expected the lagging node to want height %d, got %+v (%v)", height+1, next, err)
			}
			continue
		}
		// Height 3 is the highest a peer announced: the node caught up and asks no further.
		if len(sent) != 0 {
			t.Fatalf("expected a node that caught up to ask for nothing, it sent %+v", sent)
		}
	}
}

// A peer gets each height at most maxCommitServes times, so it cannot make a node upload a stored
// block over and over.
func TestCommitServerBoundsTheServesOfAHeightToAPeer(t *testing.T) {
	var server commitServer
	for range maxCommitServes {
		if !server.admitServe("bob", 7) {
			t.Fatal("expected a height to be served up to the bound")
		}
	}
	if server.admitServe("bob", 7) {
		t.Fatal("expected a height served as often as the bound allows to be refused")
	}
	if !server.admitServe("bob", 8) || !server.admitServe("carol", 7) {
		t.Fatal("expected another height, or another peer, to be served")
	}
	for height := types.Height(100); len(server.served) < maxTrackedServes; height++ {
		server.admitServe("dave", height)
	}
	server.admitServe("dave", 1)
	if len(server.served) > maxTrackedServes {
		t.Fatalf("expected the tracked serves bounded by %d, got %d", maxTrackedServes, len(server.served))
	}
}

// observeAhead reports a node behind only once it stayed at the same next height for
// commitCatchUpDelay; a node that advanced starts waiting again.
func TestCommitServerWaitsBeforeItCatchesUp(t *testing.T) {
	var server commitServer
	start := time.Unix(1_000, 0)
	if server.observeAhead("bob", 5, 7, start) {
		t.Fatal("expected a node that just learned it is behind to wait")
	}
	if server.observeAhead("bob", 5, 7, start.Add(commitCatchUpDelay-time.Millisecond)) {
		t.Fatal("expected the node to wait for the whole delay")
	}
	if !server.observeAhead("carol", 5, 8, start.Add(commitCatchUpDelay)) {
		t.Fatal("expected a node behind for the whole delay to catch up")
	}
	if peer, behind := server.aheadOf(7); !behind || peer != "carol" {
		t.Fatalf("expected the last announcer of a higher height, got %q %v", peer, behind)
	}
	if server.observeAhead("bob", 6, 8, start.Add(2*commitCatchUpDelay)) {
		t.Fatal("expected a node that advanced to wait again")
	}
	if _, behind := server.aheadOf(8); behind {
		t.Fatal("expected a node at the highest announced height not to be behind")
	}
}

// linkedChain commits on server a chain of six blocks in which the children of blocks 2 and 3 are
// certified only after a timeout, so blocks 2 and 3 have no finality proof of their own: block 4,
// whose child is certified in the next view, is the first one the 2-chain rule finalizes directly.
func linkedChain(t *testing.T, server *commitFixture) []types.Block {
	t.Helper()
	views := []types.Round{3, 4, 6, 8, 9, 10}
	blocks := make([]types.Block, 0, len(views))
	var parent types.Hash
	for index, view := range views {
		height := types.Height(index + 1)
		block := server.block(height, parent, types.Tx("bank:send:linked-"+string(rune('a'+index))))
		block.Header.Round = view
		if index > 0 {
			// A view after a timeout commits no grandparent, so the heights derive from the parent.
			previous := blocks[index-1].Header
			block.Header.JustifyRound = previous.Round
			block.Header.StateHeight = finality.ChildStateHeight(previous)
			block.Header.ValidatorSetHeight = finality.ChildValidatorSetHeight(previous)
			block.Header.AppHash = server.appHashAt(block.Header.StateHeight)
		}
		// The server learns of each block from a commit whose proof names a child in the next view;
		// only the child it then stores tells whether that view certified it.
		if reached, _ := server.deliver(server.commit(block)); reached != height {
			t.Fatalf("expected the server to commit height %d, it is at %d", height, reached)
		}
		blocks = append(blocks, block)
		parent = consensus.HashBlock(block)
	}
	return blocks
}

// A block whose child was certified after a timeout is final as an ancestor of the next block the
// 2-chain rule finalizes directly. A peer serves it with the proof of that block and the headers
// between, and the lagging node commits it.
func TestALaggingNodeCommitsBlocksFinalizedAsAncestors(t *testing.T) {
	ctx := context.Background()
	serverWire := newSendRecordingTransport("server")
	server := newCommitFixtureOn(t, serverWire)
	lagger := newCommitFixtureOn(t, newSendRecordingTransport("lagger"))
	blocks := linkedChain(t, server)
	if reached, _ := lagger.deliver(lagger.commit(blocks[0])); reached != 1 {
		t.Fatalf("expected the lagging node to commit height 1, it is at %d", reached)
	}
	serverWire.take()

	wantLinkage := map[types.Height]int{2: 1, 3: 0, 4: 0}
	wantProven := map[types.Height]types.Height{2: 4, 3: 4, 4: 4}
	for height := types.Height(2); height <= 4; height++ {
		server.node.acceptCommitMessage(ctx, "lagger", appendCommitMessage(nil, commitMessage{Want: height}))
		served := onlyCommitMessage(t, serverWire, "lagger")
		if served.Block.Header.Height != height || len(served.Linkage) != wantLinkage[height] || served.FinalityProof.Header.Height != wantProven[height] {
			t.Fatalf("height %d: expected a proof of height %d over %d linkage headers, got a proof of height %d over %d",
				height, wantProven[height], wantLinkage[height], served.FinalityProof.Header.Height, len(served.Linkage))
		}
		lagger.node.acceptCommitMessage(ctx, "server", appendCommitMessage(nil, served))
		if reached := lagger.node.Status(ctx).LatestHeight; reached != height {
			t.Fatalf("expected the lagging node to commit height %d, it is at %d", height, reached)
		}
	}
}

// A linked commit vouches for its block only through the hash chain to the proven block and the
// block's own certificate: a broken link, a proof of a block off the chain, or a certificate of
// another block is invalid, and the peer that sent it is penalized.
func TestALinkedCommitMustChainItsBlockToTheProvenBlock(t *testing.T) {
	ctx := context.Background()
	serverWire := newSendRecordingTransport("server")
	server := newCommitFixtureOn(t, serverWire)
	blocks := linkedChain(t, server)
	serverWire.take()
	server.node.acceptCommitMessage(ctx, "lagger", appendCommitMessage(nil, commitMessage{Want: 2}))
	genuine := onlyCommitMessage(t, serverWire, "lagger")

	cases := map[string]func(*commitMessage){
		"a linkage header off the chain": func(message *commitMessage) {
			message.Linkage = []types.Header{message.Linkage[0]}
			message.Linkage[0].PreviousBlockHash = types.Hash{9}
		},
		"a linkage header skipped": func(message *commitMessage) {
			message.Linkage = nil
		},
		"a linkage header repeated": func(message *commitMessage) {
			message.Linkage = []types.Header{message.Linkage[0], message.Linkage[0]}
		},
		"the certificate of another block": func(message *commitMessage) {
			message.QuorumCert = server.quorumCert(2, message.QuorumCert.Round, types.Hash{7})
		},
		"a body that does not match the header": func(message *commitMessage) {
			message.Block.Txs = []types.Tx{types.Tx("bank:send:mallory")}
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			lagger := newCommitFixture(t)
			if reached, _ := lagger.deliver(lagger.commit(blocks[0])); reached != 1 {
				t.Fatalf("expected the lagging node to commit height 1, it is at %d", reached)
			}
			message := genuine
			message.Linkage = append([]types.Header(nil), genuine.Linkage...)
			tamper(&message)
			before, err := lagger.node.PeerScore(ctx, "bob")
			if err != nil {
				t.Fatal(err)
			}
			height, after := lagger.deliver(message)
			if height != 1 {
				t.Fatalf("expected the tampered commit refused, node is at height %d", height)
			}
			if after != before-gossipInvalidCost {
				t.Fatalf("expected the peer penalized, score %d after %d", after, before)
			}
		})
	}
}

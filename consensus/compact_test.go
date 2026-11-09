package consensus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

// shortIDTxs serves the transactions it holds by short id; collide makes an id name another
// transaction.
type shortIDTxs struct {
	mu      sync.Mutex
	txs     map[uint64]types.Tx
	collide map[uint64]types.Tx
}

func newShortIDTxs(txs ...types.Tx) *shortIDTxs {
	source := &shortIDTxs{txs: make(map[uint64]types.Tx), collide: make(map[uint64]types.Tx)}
	for _, tx := range txs {
		source.add(tx)
	}
	return source
}

func (source *shortIDTxs) add(tx types.Tx) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.txs[mempool.ShortTxID(mempool.HashTx(tx))] = tx
}

func (source *shortIDTxs) TxByShortID(id uint64) (types.Tx, bool) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if tx, found := source.collide[id]; found {
		return tx, true
	}
	tx, found := source.txs[id]
	return tx, found
}

func compactTestProposal(t *testing.T, count int) Proposal {
	t.Helper()
	cluster := newEqualTestCluster(t, "a", "b", "c")
	txs := make([]string, count)
	for index := range txs {
		txs[index] = "bank:compact-" + string(rune('a'+index%26)) + "-" + time.Duration(index).String()
	}
	return cluster.proposal("a", cluster.block(1, types.Hash{}, txs...), 0, finality.QuorumCert{}, finality.TimeoutCert{})
}

// A compact proposal names the same block as the proposal: rebuilt from a pool that holds its
// transactions it is the proposal the proposer signed, and a pool that misses some, or holds a
// colliding transaction, never yields another block.
func TestCompactProposalRebuildsTheSignedBlock(t *testing.T) {
	proposal := compactTestProposal(t, 40)
	compact := NewCompactProposal(proposal)
	if len(compact.Proposal.Block.Txs) != 0 || len(compact.TxIDs) != 40*shortTxIDSize {
		t.Fatalf("expected the transactions replaced by short ids, got %d txs and %d id bytes", len(compact.Proposal.Block.Txs), len(compact.TxIDs))
	}
	if compact.BlockHash() != HashBlock(proposal.Block) {
		t.Fatal("expected the compact proposal to name the proposal's block")
	}
	full, missing, err := compact.Rebuild(newShortIDTxs(proposal.Block.Txs...))
	if err != nil || len(missing) != 0 {
		t.Fatalf("expected a full rebuild, missing %v err %v", missing, err)
	}
	rebuilt, ok := compact.Complete(full)
	if !ok || HashBlock(rebuilt.Block) != HashBlock(proposal.Block) || string(ProposalSignBytes(rebuilt)) != string(ProposalSignBytes(proposal)) {
		t.Fatal("expected the rebuilt proposal to be the signed one")
	}

	partial := newShortIDTxs(proposal.Block.Txs[5:]...)
	if _, missing, _ := compact.Rebuild(partial); len(missing) != 5 || missing[0] != 0 || missing[4] != 4 {
		t.Fatalf("expected the first five transactions missing, got %v", missing)
	}
	colliding := newShortIDTxs(proposal.Block.Txs...)
	colliding.collide[compact.ShortID(7)] = types.Tx("bank:another")
	full, missing, _ = compact.Rebuild(colliding)
	if len(missing) != 0 {
		t.Fatalf("setup: expected a rebuild with a colliding transaction, missing %v", missing)
	}
	if _, ok := compact.Complete(full); ok {
		t.Fatal("expected a colliding transaction to make another block, which is refused")
	}
	malformed := compact
	malformed.TxIDs = malformed.TxIDs[:len(malformed.TxIDs)-1]
	if _, _, err := malformed.Rebuild(partial); err == nil {
		t.Fatal("expected ids that are not whole to be refused")
	}
}

// compactPeer is a transport reactor on a bus, with the proposals it judged and the peer
// verdicts it gave.
type compactPeer struct {
	transport *TransportReactor
	reactor   *capturingReactor
	verdicts  chan bool
}

type capturingReactor struct {
	recordingReactor
	accepted chan Proposal
}

func (reactor *capturingReactor) OnProposal(ctx context.Context, proposal Proposal) error {
	reactor.accepted <- proposal
	return nil
}

func newCompactPeer(t *testing.T, bus *transport.InMemoryBus, id p2p.PeerID, source TxSource) compactPeer {
	t.Helper()
	wire, err := bus.NewPeer(id)
	if err != nil {
		t.Fatal(err)
	}
	peer := compactPeer{reactor: &capturingReactor{accepted: make(chan Proposal, 8)}, verdicts: make(chan bool, 64)}
	peer.transport = NewTransportReactor(wire, peer.reactor)
	peer.transport.EnableCompactProposals(source)
	peer.transport.SetPeerScoring(nil, func(_ context.Context, _ p2p.PeerID, valid bool) bool {
		peer.verdicts <- valid
		return true
	})
	if err := peer.transport.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.transport.Stop(context.Background()) })
	return peer
}

func (peer compactPeer) accepted(t *testing.T) Proposal {
	t.Helper()
	select {
	case proposal := <-peer.reactor.accepted:
		return proposal
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the proposal")
		return Proposal{}
	}
}

// A validator that holds the block's transactions rebuilds it from the compact proposal; one that
// misses some fetches only those from the proposer, and one that misses most fetches the whole
// proposal. Each judges exactly the block the proposer signed.
func TestTransportReactorFetchesOnlyWhatACompactProposalMisses(t *testing.T) {
	proposal := compactTestProposal(t, 30)
	txs := proposal.Block.Txs
	bus := transport.NewInMemoryBus()
	leader := newCompactPeer(t, bus, "leader", newShortIDTxs(txs...))
	holder := newCompactPeer(t, bus, "holder", newShortIDTxs(txs...))
	partial := newCompactPeer(t, bus, "partial", newShortIDTxs(txs[3:]...))
	empty := newCompactPeer(t, bus, "empty", newShortIDTxs())

	if err := leader.transport.BroadcastProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	for _, peer := range []compactPeer{holder, partial, empty} {
		if got := peer.accepted(t); HashBlock(got.Block) != HashBlock(proposal.Block) || len(got.Block.Txs) != len(txs) {
			t.Fatalf("expected the proposer's block, got %d txs", len(got.Block.Txs))
		}
	}
	if stats := holder.transport.CompactProposalStats(); stats.Rebuilt != 1 || stats.FetchedTxs != 0 {
		t.Fatalf("expected the holder to rebuild the block alone, got %+v", stats)
	}
	if stats := partial.transport.CompactProposalStats(); stats.Completed != 1 || stats.FetchedTxs != 3 {
		t.Fatalf("expected the partial holder to fetch the three missing transactions, got %+v", stats)
	}
	if stats := empty.transport.CompactProposalStats(); stats.Fetched != 1 || stats.FetchedTxs != 30 {
		t.Fatalf("expected the empty pool to fetch the whole proposal, got %+v", stats)
	}
	if stats := leader.transport.CompactProposalStats(); stats.ServedBytes == 0 {
		t.Fatalf("expected the leader to serve the missing transactions, got %+v", stats)
	}
}

// A peer that answers a request with a transaction the compact proposal does not name is at
// fault, and a requester cannot make a node serve a proposal again and again.
func TestTransportReactorRefusesTransactionsTheProposalDoesNotName(t *testing.T) {
	proposal := compactTestProposal(t, 4)
	bus := transport.NewInMemoryBus()
	receiver := newCompactPeer(t, bus, "receiver", newShortIDTxs(proposal.Block.Txs[1:]...))
	liar, err := bus.NewPeer("liar")
	if err != nil {
		t.Fatal(err)
	}
	if err := liar.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	requests, err := liar.Subscribe(context.Background(), p2p.TopicProposal)
	if err != nil {
		t.Fatal(err)
	}
	data, err := EncodeCompactProposal(NewCompactProposal(proposal))
	if err != nil {
		t.Fatal(err)
	}
	if err := liar.Publish(context.Background(), p2p.TopicProposal, data); err != nil {
		t.Fatal(err)
	}
	if valid := <-receiver.verdicts; !valid {
		t.Fatal("a compact proposal the receiver cannot rebuild is not the sender's fault")
	}
	var request WireMessage
	select {
	case envelope := <-requests:
		request, err = DecodeWireMessage(envelope.Data)
		if err != nil || request.Type != MessageProposalRequest || len(request.ProposalRequest.Missing) != 1 || request.ProposalRequest.Missing[0] != 0 {
			t.Fatalf("expected a request for the first transaction, got %+v (%v)", request, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the request")
	}
	answer, err := EncodeProposalTxs(ProposalTxs{BlockHash: request.ProposalRequest.BlockHash, Indexes: []uint32{0}, Txs: []types.Tx{types.Tx("bank:forged")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := liar.Send(context.Background(), "receiver", p2p.TopicProposal, answer); err != nil {
		t.Fatal(err)
	}
	if valid := <-receiver.verdicts; valid {
		t.Fatal("expected a transaction the proposal does not name to count against the peer")
	}

	leader := newCompactPeer(t, bus, "leader", newShortIDTxs())
	leader.transport.hold(proposal)
	for serve := 0; serve < maxProposalServes; serve++ {
		if _, ok := leader.transport.serve("receiver", HashBlock(proposal.Block)); !ok {
			t.Fatalf("expected serve %d to be granted", serve)
		}
	}
	if _, ok := leader.transport.serve("receiver", HashBlock(proposal.Block)); ok {
		t.Fatal("expected a proposal to be served to one peer a bounded number of times")
	}
}

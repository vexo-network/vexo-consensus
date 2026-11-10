package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/lanes"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func laneBatch(author types.ValidatorID, seq uint64, txs ...string) lanes.Batch {
	batch := lanes.Batch{ChainID: testChainID, Author: author, Seq: seq}
	for _, tx := range txs {
		batch.Txs = append(batch.Txs, types.Tx(tx))
	}
	return batch
}

func batchGetter(batches ...lanes.Batch) func(types.Hash) (lanes.Batch, bool) {
	byID := make(map[types.Hash]lanes.Batch, len(batches))
	for _, batch := range batches {
		byID[batch.ID()] = batch
	}
	return func(id types.Hash) (lanes.Batch, bool) {
		batch, found := byID[id]
		return batch, found
	}
}

// A lane proposal names the batches of its block, the positions of their transactions the block
// leaves out, and the transactions no batch carries; a validator holding the batches rebuilds
// exactly the signed block.
func TestLaneProposalRebuildsTheBlockItNames(t *testing.T) {
	cluster := newEqualTestCluster(t, "alice", "bob", "carol")
	block := cluster.block(1, types.Hash{}, "bank:a1", "bank:a3", "bank:b1", "bank:extra")
	proposal := cluster.proposal("alice", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	// alice's batch carries a2, which the leader's application check refused.
	first := laneBatch("alice", 1, "bank:a1", "bank:a2", "bank:a3")
	second := laneBatch("bob", 1, "bank:b1")
	compact := NewLaneProposal(proposal, []lanes.Batch{first, second})
	if len(compact.Proposal.Block.Txs) != 0 || len(compact.Batches) != 2 {
		t.Fatalf("expected the transactions to travel by batch, got %+v", compact)
	}
	if len(compact.Excluded) != 1 || compact.Excluded[0] != 1 || len(compact.Extra) != 1 || string(compact.Extra[0]) != "bank:extra" {
		t.Fatalf("unexpected exclusions %v and extras %q", compact.Excluded, compact.Extra)
	}
	if compact.BlockHash() != HashBlock(proposal.Block) || !bytes.Equal(compact.SignBytes(), ProposalSignBytes(proposal)) {
		t.Fatal("the lane proposal must name the signed block")
	}
	data, err := EncodeLaneProposal(compact)
	if err != nil {
		t.Fatal(err)
	}
	message, err := DecodeWireMessage(data)
	if err != nil || message.Type != MessageLaneProposal {
		t.Fatalf("expected a lane proposal off the wire, got %v %v", message.Type, err)
	}
	rebuilt, missing, complete, err := message.LaneProposal.Rebuild(batchGetter(first, second))
	if err != nil || len(missing) != 0 || !complete {
		t.Fatalf("expected the proposal rebuilt, missing=%v complete=%v err=%v", missing, complete, err)
	}
	if HashBlock(rebuilt.Block) != HashBlock(proposal.Block) || len(rebuilt.Block.Txs) != len(proposal.Block.Txs) {
		t.Fatal("the rebuilt block is another block")
	}
	for index := range rebuilt.Block.Txs {
		if !bytes.Equal(rebuilt.Block.Txs[index], proposal.Block.Txs[index]) {
			t.Fatalf("transaction %d differs", index)
		}
	}

	if _, missing, _, err := compact.Rebuild(batchGetter(first)); err != nil || len(missing) != 1 || missing[0] != second.ID() {
		t.Fatalf("expected bob's batch missing, got %v %v", missing, err)
	}
	// A pool that resolves an id to other transactions rebuilds another block: it is not judged.
	substitute := func(id types.Hash) (lanes.Batch, bool) {
		if id == second.ID() {
			return laneBatch("bob", 1, "bank:b2"), true
		}
		return batchGetter(first)(id)
	}
	if _, _, complete, err := compact.Rebuild(substitute); err != nil || complete {
		t.Fatalf("expected another block not to complete, complete=%v err=%v", complete, err)
	}
}

func TestLaneProposalRefusesWhatNamesNoBlock(t *testing.T) {
	cluster := newEqualTestCluster(t, "alice", "bob", "carol")
	proposal := cluster.proposal("alice", cluster.block(1, types.Hash{}, "bank:a"), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	batch := laneBatch("alice", 1, "bank:a")
	valid := NewLaneProposal(proposal, []lanes.Batch{batch})
	carrying := valid
	carrying.Proposal.Block.Txs = []types.Tx{types.Tx("bank:a")}
	beyond := valid
	beyond.Excluded = []uint32{5}
	repeated := NewLaneProposal(proposal, []lanes.Batch{laneBatch("alice", 1, "bank:a", "bank:b", "bank:c")})
	repeated.Excluded = []uint32{2, 1}
	tooMany := valid
	tooMany.Batches = make([]LaneRef, MaxLaneRefs+1)
	for name, compact := range map[string]LaneProposal{
		"carrying transactions": carrying,
		"no batch":              {Proposal: valid.Proposal},
		"exclusion beyond":      beyond,
		"exclusions unordered":  repeated,
		"too many batches":      tooMany,
	} {
		get := batchGetter(batch, laneBatch("alice", 1, "bank:a", "bank:b", "bank:c"))
		if _, _, _, err := compact.Rebuild(get); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("%s: expected an invalid proposal, got %v", name, err)
		}
	}
}

// laneRecordingReactor records the proposals a transport reactor hands it.
type laneRecordingReactor struct {
	mu        sync.Mutex
	proposals []Proposal
}

func (reactor *laneRecordingReactor) OnProposal(_ context.Context, proposal Proposal) error {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	reactor.proposals = append(reactor.proposals, proposal)
	return nil
}

func (reactor *laneRecordingReactor) OnVote(context.Context, Vote) error { return nil }

func (reactor *laneRecordingReactor) OnTimeoutVote(context.Context, TimeoutVote) (finality.TimeoutCert, error) {
	return finality.TimeoutCert{}, nil
}

func (reactor *laneRecordingReactor) received() []Proposal {
	reactor.mu.Lock()
	defer reactor.mu.Unlock()
	return append([]Proposal(nil), reactor.proposals...)
}

type laneValidator struct {
	reactor  *TransportReactor
	received *laneRecordingReactor
	lanes    *lanes.Lanes
	observed chan bool
}

// newLaneValidators runs a transport reactor with lanes for every member of cluster on one bus.
func newLaneValidators(t *testing.T, cluster *testCluster, authentic LaneAuthenticator) map[types.ValidatorID]*laneValidator {
	t.Helper()
	bus := transport.NewInMemoryBus()
	set := validator.NewSet(cluster.validators("alice", "bob", "carol"))
	validators := map[types.ValidatorID]*laneValidator{}
	for _, id := range []types.ValidatorID{"alice", "bob", "carol"} {
		signer, err := vexocrypto.NewDeterministicSigner([]byte("vexo-simulation-validator-" + string(id)))
		if err != nil {
			t.Fatal(err)
		}
		wire, err := bus.NewPeer(p2p.PeerID(id))
		if err != nil {
			t.Fatal(err)
		}
		received := &laneRecordingReactor{}
		reactor := NewTransportReactor(wire, received)
		observed := make(chan bool, 64)
		reactor.SetPeerScoring(nil, func(_ context.Context, _ p2p.PeerID, valid bool) bool {
			select {
			case observed <- valid:
			default:
			}
			return true
		})
		service := lanes.New(lanes.Config{
			ChainID: testChainID,
			Self:    id,
			Sign: func(_ vexocrypto.SignType, domain vexocrypto.Domain, message []byte) (types.Signature, error) {
				return vexocrypto.SignWithDomain(signer, domain, message)
			},
			Signatures:   signer,
			Aggregator:   vexocrypto.DeterministicAggregateSigner{},
			Aggregates:   vexocrypto.DeterministicAggregateSigner{},
			ValidatorSet: func(context.Context) (validator.Set, error) { return set, nil },
		})
		reactor.EnableLanes(service, authentic)
		if err := reactor.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := service.Start(context.Background(), wire); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			service.Stop()
			_ = reactor.Stop(context.Background())
		})
		validators[id] = &laneValidator{reactor: reactor, received: received, lanes: service, observed: observed}
	}
	return validators
}

func waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// A leader broadcasts a block by the batches its validators already store, and they judge exactly
// the block it signed.
func TestTransportReactorRebuildsALaneProposal(t *testing.T) {
	cluster := newEqualTestCluster(t, "alice", "bob", "carol")
	validators := newLaneValidators(t, cluster, func(context.Context, LaneProposal) error { return nil })
	alice, bob := validators["alice"], validators["bob"]
	alice.lanes.Submit(types.Tx("bank:lane-1"))
	alice.lanes.Submit(types.Tx("bank:lane-2"))
	alice.lanes.Flush(context.Background())
	waitUntil(t, "bob to hold the certified batch", func() bool { return len(bob.lanes.Candidates(1<<20)) == 1 })

	proposal := cluster.proposal("alice", cluster.block(1, types.Hash{}, "bank:lane-1", "bank:lane-2"), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	if err := alice.reactor.BroadcastProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "bob to judge the rebuilt proposal", func() bool { return len(bob.received.received()) == 1 })
	if got := bob.received.received()[0]; HashBlock(got.Block) != HashBlock(proposal.Block) || len(got.Block.Txs) != 2 {
		t.Fatalf("bob judged another block: %+v", got.Block.Header)
	}
	if stats := bob.reactor.LaneProposalStats(); stats.Rebuilt != 1 || stats.Fetched != 0 {
		t.Fatalf("unexpected lane proposal stats %+v", stats)
	}
	if len(bob.lanes.Candidates(1<<20)) != 0 {
		t.Fatal("a batch the pending block names must not be proposed again")
	}
}

// A validator missing a batch fetches it for a proposal of the leader, and asks the sender for the
// whole proposal when it cannot judge the proposal yet. A forged proposal makes it fetch nothing
// and counts against the sender.
func TestTransportReactorFetchesBatchesOnlyForTheLeader(t *testing.T) {
	cluster := newEqualTestCluster(t, "alice", "bob", "carol")
	var verdict error
	var verdictMu sync.Mutex
	validators := newLaneValidators(t, cluster, func(context.Context, LaneProposal) error {
		verdictMu.Lock()
		defer verdictMu.Unlock()
		return verdict
	})
	alice, carol := validators["alice"], validators["carol"]
	proposal := cluster.proposal("alice", cluster.block(1, types.Hash{}, "bank:unseen"), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	missing := laneBatch("alice", 99, "bank:unseen")
	compact := NewLaneProposal(proposal, []lanes.Batch{missing})
	data, err := EncodeLaneProposal(compact)
	if err != nil {
		t.Fatal(err)
	}
	deliver := func() {
		carol.reactor.handleEnvelope(context.Background(), transport.Envelope{Topic: p2p.TopicProposal, From: "alice", To: "carol", Data: data})
	}

	verdictMu.Lock()
	verdict = errors.Join(ErrInvalidProposal, errors.New("forged"))
	verdictMu.Unlock()
	deliver()
	if valid := <-carol.observed; valid {
		t.Fatal("expected the forged lane proposal to count against the sender")
	}
	if requested := carol.lanes.Stats().Requested; requested != 0 {
		t.Fatalf("expected no batch fetched for a forged proposal, got %d", requested)
	}

	verdictMu.Lock()
	verdict = errors.New("validator set pending")
	verdictMu.Unlock()
	alice.reactor.hold(proposal)
	deliver()
	waitUntil(t, "carol to fetch the whole proposal", func() bool { return len(carol.received.received()) == 1 })
	if requested := carol.lanes.Stats().Requested; requested != 0 {
		t.Fatalf("expected the unjudged proposal fetched whole, got %d batch requests", requested)
	}
	if stats := carol.reactor.LaneProposalStats(); stats.Fetched != 1 {
		t.Fatalf("expected one whole fetch, got %+v", stats)
	}
}

// A lane proposal whose validator set waits on a block the node has not executed costs nobody an
// upload: the node neither fetches its batches nor asks for it whole, and judges it once it
// executed that block.
func TestTransportReactorDefersALaneProposalItCannotJudgeYet(t *testing.T) {
	cluster := newEqualTestCluster(t, "alice", "bob", "carol")
	var verdict error
	var verdictMu sync.Mutex
	validators := newLaneValidators(t, cluster, func(context.Context, LaneProposal) error {
		verdictMu.Lock()
		defer verdictMu.Unlock()
		return verdict
	})
	alice, carol := validators["alice"], validators["carol"]
	proposal := cluster.proposal("alice", cluster.block(1, types.Hash{}, "bank:deferred"), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	alice.reactor.hold(proposal)
	// carol does not store the batch the proposal names.
	compact := NewLaneProposal(proposal, []lanes.Batch{laneBatch("alice", 7, "bank:deferred")})
	data, err := EncodeLaneProposal(compact)
	if err != nil {
		t.Fatal(err)
	}
	verdictMu.Lock()
	verdict = fmt.Errorf("%w: registry height 1, executed through 0", ErrValidatorSetPending)
	verdictMu.Unlock()
	carol.reactor.handleEnvelope(context.Background(), transport.Envelope{Topic: p2p.TopicProposal, From: "alice", To: "carol", Data: data})
	time.Sleep(50 * time.Millisecond)
	if requested := carol.lanes.Stats().Requested; requested != 0 {
		t.Fatalf("expected no batch fetched for a proposal carol cannot judge yet, got %d", requested)
	}
	if stats := carol.reactor.LaneProposalStats(); stats.Fetched != 0 || len(carol.received.received()) != 0 {
		t.Fatalf("expected the proposal deferred, not fetched whole: %+v", stats)
	}
	verdictMu.Lock()
	verdict = nil
	verdictMu.Unlock()
	carol.reactor.Retry()
	waitUntil(t, "carol to ask for the missing batch", func() bool { return carol.lanes.Stats().Requested > 0 })
}

package lanes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const testChain = "vexo-test"

func testSigner(t *testing.T, id types.ValidatorID) vexocrypto.DeterministicSigner {
	t.Helper()
	signer, err := vexocrypto.NewDeterministicSigner([]byte(string(id) + "-key"))
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func testSet(t *testing.T, ids ...types.ValidatorID) validator.Set {
	t.Helper()
	members := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		members = append(members, validator.Validator{ID: id, Address: types.Address(id), VotingPower: 1, PublicKey: testSigner(t, id).PublicKey()})
	}
	return validator.NewSet(members)
}

func sign(t *testing.T, id types.ValidatorID, domain vexocrypto.Domain, message []byte) types.Signature {
	t.Helper()
	signature, err := vexocrypto.SignWithDomain(testSigner(t, id), domain, message)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

func signedBatch(t *testing.T, author types.ValidatorID, seq uint64, txs ...string) Batch {
	t.Helper()
	batch := Batch{ChainID: testChain, Author: author, Seq: seq}
	for _, tx := range txs {
		batch.Txs = append(batch.Txs, types.Tx(tx))
	}
	batch.Signature = sign(t, author, vexocrypto.DomainLaneBatch, BatchSignBytes(batch.ID()))
	return batch
}

func certFor(t *testing.T, batch Batch, signers ...types.ValidatorID) Cert {
	t.Helper()
	signatures := make([]types.Signature, 0, len(signers))
	for _, signer := range signers {
		signatures = append(signatures, sign(t, signer, vexocrypto.DomainLaneBatchAck, AckSignBytes(batch.ID())))
	}
	aggregate, err := vexocrypto.DeterministicAggregateSigner{}.Aggregate(signatures)
	if err != nil {
		t.Fatal(err)
	}
	return Cert{BatchID: batch.ID(), Author: batch.Author, Signers: finality.EncodeSigners(signers), Signature: aggregate}
}

func TestWireRoundTripsEveryMessageAndRefusesMalformedOnes(t *testing.T) {
	batch := signedBatch(t, "alice", 7, "bank:a", "bank:b")
	cert := certFor(t, batch, "alice", "bob")
	messages := []Message{
		{Type: MessageBatch, Batch: &batch},
		{Type: MessageAck, Ack: &Ack{BatchID: batch.ID(), Voter: "bob", Signature: types.Signature("sig")}},
		{Type: MessageCert, Cert: &cert},
		{Type: MessageRequest, Request: &Request{IDs: []types.Hash{batch.ID(), {1}}}},
	}
	for _, message := range messages {
		data, err := encode(message)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("%d: %v", message.Type, err)
		}
		again, err := encode(decoded)
		if err != nil || !bytes.Equal(again, data) {
			t.Fatalf("%d: the message does not round-trip", message.Type)
		}
		if _, err := Decode(append(data, 0)); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("%d: expected trailing bytes refused, got %v", message.Type, err)
		}
		if _, err := Decode(data[:len(data)-1]); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("%d: expected a truncated message refused, got %v", message.Type, err)
		}
	}
	if decoded, _ := Decode(mustEncode(t, messages[0])); decoded.Batch.ID() != batch.ID() {
		t.Fatal("a decoded batch names another id")
	}
	for name, data := range map[string][]byte{
		"empty":         nil,
		"version":       {2, byte(MessageAck)},
		"type":          {wireVersion, 9},
		"empty request": {wireVersion, byte(MessageRequest), 0},
		"huge request":  {wireVersion, byte(MessageRequest), 0xff, 0xff, 0x03},
	} {
		if _, err := Decode(data); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("%s: expected refusal, got %v", name, err)
		}
	}
}

func mustEncode(t *testing.T, message Message) []byte {
	t.Helper()
	data, err := encode(message)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestVerifyBatchRefusesWhatNoHonestAuthorSigns(t *testing.T) {
	set := testSet(t, "alice", "bob", "carol")
	verifier := testSigner(t, "alice")
	good := signedBatch(t, "alice", 1, "bank:a")
	if err := VerifyBatch(testChain, good, set, verifier); err != nil {
		t.Fatal(err)
	}
	tampered := good
	tampered.Txs = []types.Tx{types.Tx("bank:b")}
	stranger := signedBatch(t, "mallory", 1, "bank:a")
	forged := good
	forged.Author = "bob"
	for name, batch := range map[string]Batch{
		"tampered":     tampered,
		"stranger":     stranger,
		"forged":       forged,
		"other chain":  {ChainID: "other", Author: "alice", Seq: 1, Txs: good.Txs, Signature: good.Signature},
		"no sequence":  signedBatch(t, "alice", 0, "bank:a"),
		"empty":        signedBatch(t, "alice", 1),
		"empty tx":     signedBatch(t, "alice", 1, ""),
		"no signature": {ChainID: testChain, Author: "alice", Seq: 1, Txs: good.Txs},
	} {
		if err := VerifyBatch(testChain, batch, set, verifier); !errors.Is(err, ErrInvalidBatch) {
			t.Fatalf("%s: expected refusal, got %v", name, err)
		}
	}
}

func TestVerifyCertRequiresAQuorumOfDistinctValidators(t *testing.T) {
	set := testSet(t, "alice", "bob", "carol", "dave")
	aggregates := vexocrypto.DeterministicAggregateSigner{}
	batch := signedBatch(t, "alice", 1, "bank:a")
	if err := VerifyCert(certFor(t, batch, "alice", "bob", "carol"), set, aggregates); err != nil {
		t.Fatal(err)
	}
	duplicated := certFor(t, batch, "alice", "bob", "bob")
	forged := certFor(t, batch, "alice", "bob", "carol")
	forged.BatchID = types.Hash{9}
	for name, cert := range map[string]Cert{
		"two of four":     certFor(t, batch, "alice", "bob"),
		"duplicate":       duplicated,
		"stranger":        certFor(t, batch, "alice", "bob", "mallory"),
		"other batch":     forged,
		"unknown author":  {BatchID: batch.ID(), Author: "mallory", Signers: duplicated.Signers, Signature: duplicated.Signature},
		"empty signature": {BatchID: batch.ID(), Author: "alice", Signers: finality.EncodeSigners([]types.ValidatorID{"alice", "bob", "carol"})},
	} {
		if err := VerifyCert(cert, set, aggregates); !errors.Is(err, ErrInvalidCert) {
			t.Fatalf("%s: expected refusal, got %v", name, err)
		}
	}
}

func TestOwnerGivesEverySenderOneLane(t *testing.T) {
	members := Members(testSet(t, "carol", "alice", "bob"))
	if members[0] != "alice" || members[2] != "carol" {
		t.Fatalf("expected members sorted by id, got %v", members)
	}
	owners := map[types.ValidatorID]int{}
	for index := 0; index < 300; index++ {
		owner := Owner(members, []byte(fmt.Sprintf("sender-%d", index)))
		if owner != Owner(members, []byte(fmt.Sprintf("sender-%d", index))) {
			t.Fatal("the owner of a sender changes")
		}
		owners[owner]++
	}
	for _, member := range members {
		if owners[member] < 60 {
			t.Fatalf("expected the senders spread over the lanes, got %v", owners)
		}
	}
}

// laneNet runs one lane service per validator on an in-memory bus.
type laneNet struct {
	set      validator.Set
	services map[types.ValidatorID]*Lanes
	pending  map[types.ValidatorID]map[types.Hash]bool
	mu       sync.Mutex
}

func newLaneNet(t *testing.T, ids ...types.ValidatorID) *laneNet {
	t.Helper()
	net := &laneNet{set: testSet(t, ids...), services: map[types.ValidatorID]*Lanes{}, pending: map[types.ValidatorID]map[types.Hash]bool{}}
	bus := transport.NewInMemoryBus()
	for _, id := range ids {
		signer := testSigner(t, id)
		net.pending[id] = map[types.Hash]bool{}
		self := id
		service := New(Config{
			ChainID: testChain,
			Self:    id,
			Sign: func(_ vexocrypto.SignType, domain vexocrypto.Domain, message []byte) (types.Signature, error) {
				return vexocrypto.SignWithDomain(signer, domain, message)
			},
			Signatures:   signer,
			Aggregator:   vexocrypto.DeterministicAggregateSigner{},
			Aggregates:   vexocrypto.DeterministicAggregateSigner{},
			ValidatorSet: func(context.Context) (validator.Set, error) { return net.set, nil },
			Pending: func(hash types.Hash) bool {
				net.mu.Lock()
				defer net.mu.Unlock()
				return net.pending[self][hash]
			},
		})
		wire, err := bus.NewPeer(p2p.PeerID(id))
		if err != nil {
			t.Fatal(err)
		}
		if err := wire.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := service.Start(context.Background(), wire); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(service.Stop)
		net.services[id] = service
	}
	return net
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func candidateIDs(service *Lanes) []types.Hash {
	var ids []types.Hash
	for _, batch := range service.Candidates(1 << 20) {
		ids = append(ids, batch.ID())
	}
	return ids
}

// A batch travels to every validator, is certified by a quorum of acknowledgements, and is then
// proposable everywhere; a block that names it consumes it.
func TestLanesCertifyABatchAndOfferItToEveryLeader(t *testing.T) {
	net := newLaneNet(t, "alice", "bob", "carol", "dave")
	alice, bob := net.services["alice"], net.services["bob"]
	alice.Submit(types.Tx("bank:certified"))
	alice.Flush(context.Background())
	waitFor(t, "bob to hold a certified batch", func() bool { return len(candidateIDs(bob)) == 1 })
	batches := bob.Candidates(1 << 20)
	if batches[0].Author != "alice" || string(batches[0].Txs[0]) != "bank:certified" {
		t.Fatalf("unexpected candidate %+v", batches[0])
	}
	if !bob.Carries(mempool.HashTx(types.Tx("bank:certified"))) {
		t.Fatal("expected bob to know the batch carries the transaction")
	}
	if stats := alice.Stats(); stats.Sealed != 1 || stats.Certified != 1 {
		t.Fatalf("unexpected author stats %+v", stats)
	}
	id := batches[0].ID()
	bob.Reference(types.Hash{1}, 5, []types.Hash{id})
	if len(candidateIDs(bob)) != 0 {
		t.Fatal("a batch a pending block names must not be proposed again")
	}
	// The block of height 5 that commits is another one: the batch is proposable again.
	bob.Committed(types.Hash{2}, 5, nil)
	if ids := candidateIDs(bob); len(ids) != 1 || ids[0] != id {
		t.Fatalf("expected the batch of the abandoned block proposable again, got %v", ids)
	}
	bob.Reference(types.Hash{3}, 6, []types.Hash{id})
	bob.Committed(types.Hash{3}, 6, batches[0].Txs)
	if len(candidateIDs(bob)) != 0 || bob.HasProposable() {
		t.Fatal("a committed batch must not be proposed again")
	}
}

// A lane is proposed in sequence order up to its first uncertified batch, and the lanes take turns,
// so no validator's lane starves the others in a full block.
func TestCandidatesKeepLaneOrderAndTakeLanesInTurn(t *testing.T) {
	service := New(Config{ChainID: testChain, Self: "zed"})
	store := func(batch Batch, certified bool) {
		service.mu.Lock()
		stored := service.storeLocked(batch, batch.ID())
		if certified {
			stored.cert = &Cert{BatchID: batch.ID()}
		}
		service.mu.Unlock()
	}
	a1, a2, a3 := signedBatch(t, "alice", 1, "a1"), signedBatch(t, "alice", 2, "a2"), signedBatch(t, "alice", 3, "a3")
	b1, b2 := signedBatch(t, "bob", 1, "b1"), signedBatch(t, "bob", 2, "b2")
	store(a3, true)
	store(a1, true)
	store(a2, false)
	store(b2, true)
	store(b1, true)
	var order []string
	for _, batch := range service.Candidates(1 << 20) {
		order = append(order, string(batch.Txs[0]))
	}
	if fmt.Sprint(order) != "[a1 b1 b2]" {
		t.Fatalf("expected alice's lane to stop at its uncertified batch and the lanes to alternate, got %v", order)
	}
	if limited := service.Candidates(4); len(limited) != 2 {
		t.Fatalf("expected the byte limit to bound the candidates, got %d", len(limited))
	}
}

// A certificate for a batch the node does not hold makes it ask the signers, and the batch they
// serve is stored.
func TestLanesFetchACertifiedBatchTheyMissed(t *testing.T) {
	net := newLaneNet(t, "alice", "bob", "carol")
	batch := signedBatch(t, "alice", 1, "bank:missed")
	alice, carol := net.services["alice"], net.services["carol"]
	alice.mu.Lock()
	alice.storeLocked(batch, batch.ID())
	alice.mu.Unlock()
	cert := certFor(t, batch, "alice", "bob", "carol")
	if err := carol.onCert(context.Background(), cert, net.set); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "carol to fetch the batch", func() bool {
		_, held := carol.Get(batch.ID())
		return held
	})
	if stats := alice.Stats(); stats.Served != 1 {
		t.Fatalf("expected alice to serve the batch once, got %+v", stats)
	}
}

// A validator holding a transaction of another lane batches it itself once the fallback delay
// passed and no stored batch carries it: a crashed owner does not censor its senders.
func TestHeldTransactionsAreBatchedWhenTheirOwnerDoesNot(t *testing.T) {
	net := newLaneNet(t, "alice", "bob", "carol")
	tx := types.Tx("bank:orphan")
	net.mu.Lock()
	net.pending["bob"][mempool.HashTx(tx)] = true
	net.mu.Unlock()
	bob := net.services["bob"]
	bob.Hold(tx)
	time.Sleep(FallbackDelay / 2)
	if bob.Stats().Sealed != 0 {
		t.Fatal("expected bob to wait for the owner")
	}
	waitFor(t, "bob to batch the orphan", func() bool { return bob.Stats().Sealed == 1 })
	waitFor(t, "alice to hold the orphan's batch", func() bool { return net.services["alice"].Carries(mempool.HashTx(tx)) })
}

// A batch over its author's share of a node's storage is not stored or acknowledged: a validator
// flooding its lane fills its own share only.
func TestABatchOverTheAuthorsShareIsNotStored(t *testing.T) {
	set := testSet(t, "alice", "bob")
	service := New(Config{ChainID: testChain, Self: "bob", Signatures: testSigner(t, "bob"), ValidatorSet: func(context.Context) (validator.Set, error) { return set, nil }})
	service.mu.Lock()
	service.lanes["alice"] = &lane{bytes: maxLaneBytes}
	service.mu.Unlock()
	batch := signedBatch(t, "alice", 1, "bank:flood")
	if err := service.onBatch(context.Background(), batch, set); err != nil {
		t.Fatal(err)
	}
	if _, held := service.Get(batch.ID()); held {
		t.Fatal("expected the batch over the author's share to be refused")
	}
}

// A lightly loaded lane waits for the certificate of its previous small batch, at most certWait,
// so the transactions that arrive meanwhile travel in one batch instead of one batch each; a lane
// whose previous batch was large does not wait.
func TestALightlyLoadedLaneWaitsForItsPreviousCertificate(t *testing.T) {
	// alice's batches never gather a quorum of acknowledgements: the other validators are down.
	net := newLaneNet(t, "alice", "bob", "carol", "dave")
	for _, id := range []types.ValidatorID{"bob", "carol", "dave"} {
		net.services[id].Stop()
	}
	alice := net.services["alice"]
	alice.Submit(types.Tx("bank:first"))
	waitFor(t, "the first batch", func() bool { return alice.Stats().Sealed == 1 })
	first := time.Now()
	for index := range 5 {
		alice.Submit(types.Tx(fmt.Sprintf("bank:waiting-%d", index)))
	}
	waitFor(t, "the second batch", func() bool { return alice.Stats().Sealed == 2 })
	if waited := time.Since(first); waited < certWait-SealDelay {
		t.Fatalf("expected the second batch to wait for the certificate of the first, sealed after %s", waited)
	}
	alice.mu.Lock()
	second := alice.entries[alice.batched[mempool.HashTx(types.Tx("bank:waiting-0"))]]
	alice.mu.Unlock()
	if second == nil || len(second.batch.Txs) != 5 {
		t.Fatalf("expected the waiting transactions in one batch, got %+v", second)
	}

	alice.Submit(types.Tx(strings.Repeat("x", paceBytes)))
	waitFor(t, "the large batch", func() bool { return alice.Stats().Sealed == 3 })
	alice.mu.Lock()
	wait := alice.certWaitLocked(time.Now())
	alice.mu.Unlock()
	if wait != 0 {
		t.Fatalf("a lane whose previous batch was large must not wait, waits %s", wait)
	}
}

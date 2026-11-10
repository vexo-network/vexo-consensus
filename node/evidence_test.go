package node

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/mempool"
	appslashing "github.com/vexo-network/vexo-consensus/modules/slashing"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// evidenceStepConfig drives a node one consensus step at a time in finalized mode. Empty blocks keep
// the chain moving when no transaction is pending.
var evidenceStepConfig = ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeFinalized}

const (
	// evidenceBobPower is the power, and the self-stake, of the validator the tests accuse.
	evidenceBobPower = 100
	// evidenceBobStakeAfterSlash is what remains of it after the default penalty of a conflicting
	// vote, 500 basis points.
	evidenceBobStakeAfterSlash = 95
)

// newEvidenceApplication is an application that punishes: the staking module owns the stake, the
// slashing module applies evidence transactions.
func newEvidenceApplication(t *testing.T) vexoapp.Application {
	t.Helper()
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{testModule{name: "bank"}, staking.NewModule(), appslashing.NewModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func evidenceValidator(id types.ValidatorID, power types.VotingPower) validator.Validator {
	return validator.Validator{ID: id, Address: types.Address(id), VotingPower: power, Stake: uint64(power), PublicKey: deterministicPublicKeyForID(id)}
}

// evidenceGenesis is a chain of alice and bob in which alice alone holds a quorum and proposes every
// view, so a test can drive the chain through her while bob stays offline.
func evidenceGenesis() Genesis {
	return Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			evidenceValidator("alice", 1000),
			evidenceValidator("bob", evidenceBobPower),
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1000, "bob": evidenceBobPower},
	}
}

// newEvidenceNode builds a validator that judges and punishes evidence. It stores its data in
// dataDir, so a second node over the same directory is the first one restarted.
func newEvidenceNode(t *testing.T, bus *transport.InMemoryBus, genesis Genesis, validatorID types.ValidatorID, dataDir string, adjust ...func(*Config)) *Node {
	t.Helper()
	wire, err := bus.NewPeer(p2p.PeerID(validatorID))
	if err != nil {
		t.Fatal(err)
	}
	cfg := UnsafeTestConfig("vexo-test", dataDir)
	cfg.Chain.Crypto.Backend = config.CryptoBackendDeterministic
	cfg.ValidatorID = validatorID
	for _, change := range adjust {
		change(&cfg)
	}
	node, err := New(cfg, genesis, newEvidenceApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.WithTransport(wire)
	node.WithSigner(deterministicSignerForID(validatorID))
	return node
}

// startEvidenceNode starts a validator and stops it when the test ends.
func startEvidenceNode(t *testing.T, node *Node) {
	t.Helper()
	startNode(t, node)
	t.Cleanup(func() { _ = node.Stop(context.Background()) })
}

// stepNode runs one consensus step: it either commits a block or proposes one.
func stepNode(t *testing.T, node *Node) ConsensusStepResult {
	t.Helper()
	result, err := node.StepConsensusWithConfig(context.Background(), evidenceStepConfig)
	if err != nil {
		t.Fatalf("consensus step failed: %v", err)
	}
	return result
}

// stepToHeight steps until the node has executed the block of height.
func stepToHeight(t *testing.T, node *Node, height types.Height) {
	t.Helper()
	for step := 0; step < 24; step++ {
		if node.Status(context.Background()).LatestHeight >= height {
			return
		}
		stepNode(t, node)
	}
	t.Fatalf("node did not execute height %d: %+v", height, node.Status(context.Background()))
}

// stepUntil steps until condition holds and stops at once, so a test sees the state of every step.
func stepUntil(t *testing.T, node *Node, what string, condition func() bool) {
	t.Helper()
	for step := 0; step < 24; step++ {
		if condition() {
			return
		}
		stepNode(t, node)
	}
	if !condition() {
		t.Fatalf("consensus did not reach %s: %+v", what, node.Status(context.Background()))
	}
}

func evidenceRuntime(t *testing.T, node *Node) *vexoruntime.Runtime {
	t.Helper()
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

// signedBobVote is a vote by bob, signed with bob's key, for the block hash.
func signedBobVote(t *testing.T, height types.Height, round types.Round, blockHash types.Hash) consensus.Vote {
	t.Helper()
	return signedNodeTestVote(t, deterministicSignerForID("bob"), "bob", height, round, blockHash)
}

// bobEquivocation is evidence that bob signed two different blocks in one view.
func bobEquivocation(t *testing.T, height types.Height, round types.Round) slashing.Evidence {
	t.Helper()
	return bobEquivocationOf(t, height, round, types.Hash{1}, types.Hash{2})
}

func bobEquivocationOf(t *testing.T, height types.Height, round types.Round, first types.Hash, second types.Hash) slashing.Evidence {
	t.Helper()
	evidence, err := consensus.NewConflictingVoteEvidence(signedBobVote(t, height, round, first), signedBobVote(t, height, round, second))
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func evidenceTx(t *testing.T, evidence slashing.Evidence) types.Tx {
	t.Helper()
	tx, err := appslashing.EncodeEvidenceTx(evidence)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

// stakeOf is the stake validator holds in itself.
func stakeOf(t *testing.T, node *Node, validatorID types.ValidatorID) uint64 {
	t.Helper()
	runtime := evidenceRuntime(t, node)
	stake, err := staking.Stake(context.Background(), runtime.Store, types.Address(validatorID), validatorID)
	if err != nil {
		t.Fatal(err)
	}
	return stake
}

func requireStake(t *testing.T, node *Node, validatorID types.ValidatorID, expected uint64) {
	t.Helper()
	if stake := stakeOf(t, node, validatorID); stake != expected {
		t.Fatalf("expected %s to hold stake %d, got %d", validatorID, expected, stake)
	}
}

func jailOf(t *testing.T, node *Node, validatorID types.ValidatorID) (types.Height, bool) {
	t.Helper()
	runtime := evidenceRuntime(t, node)
	until, jailed, err := staking.JailedUntil(context.Background(), runtime.Store, validatorID)
	if err != nil {
		t.Fatal(err)
	}
	return until, jailed
}

// pooledEvidence is the keys of the evidence records the node keeps.
func pooledEvidence(t *testing.T, node *Node) []string {
	t.Helper()
	runtime := evidenceRuntime(t, node)
	keys, err := runtime.Store.EvidenceIndex(context.Background())
	if errors.Is(err, store.ErrEvidenceNotFound) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func requirePooledEvidence(t *testing.T, node *Node, expected ...slashing.Evidence) {
	t.Helper()
	pooled := pooledEvidence(t, node)
	if len(pooled) != len(expected) {
		t.Fatalf("expected %d pooled evidence records, got %v", len(expected), pooled)
	}
	for _, evidence := range expected {
		key := store.EvidenceKey(evidence)
		found := false
		for _, candidate := range pooled {
			found = found || candidate == key
		}
		if !found {
			t.Fatalf("expected evidence %q in the pool, got %v", key, pooled)
		}
	}
}

// mempoolTxs is every transaction the mempool holds, without checking them again.
func mempoolTxs(t *testing.T, node *Node) []types.Tx {
	t.Helper()
	runtime := evidenceRuntime(t, node)
	txs, err := runtime.Mempool.SnapshotTxs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return txs
}

// txHashes names transactions by hash, which keeps a failure message readable.
func txHashes(txs []types.Tx) []string {
	names := make([]string, 0, len(txs))
	for _, tx := range txs {
		hash := mempool.HashTx(tx)
		names = append(names, hex.EncodeToString(hash[:4]))
	}
	return names
}

func requireMempoolTxs(t *testing.T, node *Node, expected ...types.Tx) {
	t.Helper()
	txs := mempoolTxs(t, node)
	if len(txs) != len(expected) {
		t.Fatalf("expected mempool %v, got %v", txHashes(expected), txHashes(txs))
	}
	for index := range expected {
		if !bytes.Equal(txs[index], expected[index]) {
			t.Fatalf("expected mempool %v, got %v", txHashes(expected), txHashes(txs))
		}
	}
}

// blockWithTx is the committed block that carries tx, or false when no executed block does.
func blockWithTx(t *testing.T, node *Node, tx types.Tx) (store.BlockRecord, int, bool) {
	t.Helper()
	latest := node.Status(context.Background()).LatestHeight
	for height := types.Height(1); height <= latest; height++ {
		record, err := node.BlockByHeight(context.Background(), height)
		if err != nil {
			t.Fatal(err)
		}
		for index, candidate := range record.Block.Txs {
			if bytes.Equal(candidate, tx) {
				return record, index, true
			}
		}
	}
	return store.BlockRecord{}, 0, false
}

// requireOffenderSlashed checks what executing the evidence block did to bob and to the validator
// set that applies from the next height.
func requireOffenderSlashed(t *testing.T, node *Node, carrier store.BlockRecord) {
	t.Helper()
	ctx := context.Background()
	runtime := evidenceRuntime(t, node)
	requireStake(t, node, "bob", evidenceBobStakeAfterSlash)
	power, err := staking.ValidatorPower(ctx, runtime.Store, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if power != evidenceBobStakeAfterSlash {
		t.Fatalf("expected bob's power to follow his stake to %d, got %d", evidenceBobStakeAfterSlash, power)
	}
	penalty, err := slashing.DefaultPenaltyPolicy().Penalty(slashing.EvidenceConflictingVote)
	if err != nil {
		t.Fatal(err)
	}
	until, jailed := jailOf(t, node, "bob")
	if !jailed || until != carrier.Block.Header.Height+types.Height(penalty.JailDuration) {
		t.Fatalf("expected bob jailed for %d blocks from %d, got until=%d jailed=%t", penalty.JailDuration, carrier.Block.Header.Height, until, jailed)
	}
	if tombstoned, err := staking.Tombstoned(ctx, runtime.Store, "bob"); err != nil || tombstoned {
		t.Fatalf("a conflicting vote must not tombstone bob: tombstoned=%t err=%v", tombstoned, err)
	}
	requireStake(t, node, "alice", 1000)

	before, err := runtime.Validators.ValidatorSet(ctx, carrier.Block.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := before.Get("bob"); !found {
		t.Fatal("the block that punished bob ran under a set that must still hold him")
	}
	after, err := runtime.Validators.ValidatorSet(ctx, carrier.Block.Header.Height+1)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := after.Get("bob"); found {
		t.Fatal("a jailed validator must leave the set the punishing block reports for the next height")
	}
	if _, found := after.Get("alice"); !found {
		t.Fatal("the honest validator must stay in the set")
	}
}

// requireEvidenceSettled checks that the node holds nothing of an offence a block punished.
func requireEvidenceSettled(t *testing.T, node *Node, evidence slashing.Evidence) {
	t.Helper()
	requirePooledEvidence(t, node)
	requireMempoolTxs(t, node)
	_, err := node.SubmitEvidence(context.Background(), evidence)
	if !errors.Is(err, appslashing.ErrEvidenceAlreadyApplied) {
		t.Fatalf("expected punished evidence to be refused as already applied, got %v", err)
	}
	requirePooledEvidence(t, node)
	requireMempoolTxs(t, node)
}

// A reporter's evidence changes no consensus state. It waits in the pool and in the mempool, and the
// offender pays when the block that carries it executes, on the strength of the proof alone.
func TestNodeSubmittedEvidenceSlashesOffenderWhenItsBlockExecutes(t *testing.T) {
	ctx := context.Background()
	alice := newEvidenceNode(t, transport.NewInMemoryBus(), evidenceGenesis(), "alice", t.TempDir())
	startEvidenceNode(t, alice)
	stepToHeight(t, alice, 1)

	evidence := bobEquivocation(t, 1, 0)
	tx := evidenceTx(t, evidence)
	hash, err := alice.SubmitEvidence(ctx, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if hash != mempool.HashTx(tx) {
		t.Fatalf("expected the hash of the evidence transaction %x, got %x", mempool.HashTx(tx), hash)
	}
	requireMempoolTxs(t, alice, tx)
	requirePooledEvidence(t, alice, evidence)
	requireStake(t, alice, "bob", evidenceBobPower)
	if _, jailed := jailOf(t, alice, "bob"); jailed {
		t.Fatal("submitting evidence must not jail the offender")
	}

	for step := 0; step < 12 && stakeOf(t, alice, "bob") == evidenceBobPower; step++ {
		stepNode(t, alice)
		if _, _, executed := blockWithTx(t, alice, tx); executed != (stakeOf(t, alice, "bob") != evidenceBobPower) {
			t.Fatal("bob's stake must change in the step that executes the block carrying the evidence, and only then")
		}
	}
	carrier, index, executed := blockWithTx(t, alice, tx)
	if !executed {
		t.Fatalf("the evidence never reached a block: %+v", alice.Status(ctx))
	}
	if carrier.Block.Header.Height != alice.Status(ctx).LatestHeight || carrier.Block.Header.Height <= 1 {
		t.Fatalf("expected the evidence block to be the one just executed above height 1, got %d of %+v", carrier.Block.Header.Height, alice.Status(ctx))
	}
	if len(carrier.Block.Txs) != 1 || len(carrier.TxResults) != 1 || carrier.TxResults[index].Code != 0 {
		t.Fatalf("expected the evidence transaction to succeed alone in its block: txs=%v results=%+v", txHashes(carrier.Block.Txs), carrier.TxResults)
	}
	requireOffenderSlashed(t, alice, carrier)
	requireEvidenceSettled(t, alice, evidence)
	// The same offence packaged as other votes is still the same offence.
	requireEvidenceSettled(t, alice, bobEquivocationOf(t, 1, 0, types.Hash{3}, types.Hash{4}))

	t.Run("chain continues after the slash", func(t *testing.T) {
		committed := alice.Status(ctx).LatestHeight
		stepToHeight(t, alice, committed+4)
		latest := alice.Status(ctx).LatestHeight
		last, err := alice.BlockByHeight(ctx, latest)
		if err != nil {
			t.Fatal(err)
		}
		set, err := evidenceRuntime(t, alice).Validators.ValidatorSet(ctx, latest)
		if err != nil {
			t.Fatal(err)
		}
		if _, found := set.Get("bob"); found || last.Block.Header.ValidatorSetHash != set.Hash() {
			t.Fatalf("expected the chain to run on the set without bob, got bob=%t header=%x set=%x", found, last.Block.Header.ValidatorSetHash, set.Hash())
		}
		requireStake(t, alice, "bob", evidenceBobStakeAfterSlash)
	})
}

// Equivocation that a node detects is of the height being decided, which the mempool cannot judge
// yet. The evidence is pooled and gossiped at once and queued by the commit that makes it judgeable,
// so a later block still punishes it.
func TestNodeQueuesEvidenceOfTheHeightInProgressAfterTheNextCommit(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir())
	startEvidenceNode(t, alice)
	observerWire, err := bus.NewPeer("observer")
	if err != nil {
		t.Fatal(err)
	}
	if err := observerWire.Start(ctx); err != nil {
		t.Fatal(err)
	}
	gossip, err := observerWire.Subscribe(ctx, p2p.TopicEvidence)
	if err != nil {
		t.Fatal(err)
	}
	stepToHeight(t, alice, 1)

	evidence := bobEquivocation(t, 2, 1)
	tx := evidenceTx(t, evidence)
	hash, err := alice.SubmitEvidence(ctx, evidence)
	if err != nil {
		t.Fatalf("evidence of the height in progress must be accepted for later, got %v", err)
	}
	if hash != mempool.HashTx(tx) {
		t.Fatalf("expected the hash of the evidence transaction %x, got %x", mempool.HashTx(tx), hash)
	}
	requirePooledEvidence(t, alice, evidence)
	requireMempoolTxs(t, alice)
	select {
	case envelope := <-gossip:
		if !bytes.Equal(envelope.Data, tx) {
			t.Fatalf("expected the evidence transaction on the wire, got %v", txHashes([]types.Tx{envelope.Data}))
		}
	default:
		t.Fatal("deferred evidence must still be gossiped, peers judge it themselves")
	}

	stepToHeight(t, alice, 2)
	requireMempoolTxs(t, alice, tx)
	requirePooledEvidence(t, alice, evidence)
	requireStake(t, alice, "bob", evidenceBobPower)

	for step := 0; step < 12 && stakeOf(t, alice, "bob") == evidenceBobPower; step++ {
		stepNode(t, alice)
		if txs := mempoolTxs(t, alice); len(txs) > 1 {
			t.Fatalf("a commit must not queue the evidence a second time: %v", txHashes(txs))
		}
	}
	carrier, _, executed := blockWithTx(t, alice, tx)
	if !executed || carrier.Block.Header.Height <= 2 {
		t.Fatalf("expected a later block to carry the deferred evidence, got executed=%t height=%d", executed, carrier.Block.Header.Height)
	}
	requireOffenderSlashed(t, alice, carrier)
	requireEvidenceSettled(t, alice, evidence)
}

// Evidence is accepted on the strength of its proof and nothing else. Whatever cannot be verified
// against the validator set of its height, by the rules a block applies it with, is refused before it
// is pooled, queued or gossiped.
func TestNodeRejectsEvidenceItCannotVerify(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir())
	startEvidenceNode(t, alice)
	observerWire, err := bus.NewPeer("observer")
	if err != nil {
		t.Fatal(err)
	}
	if err := observerWire.Start(ctx); err != nil {
		t.Fatal(err)
	}
	gossip, err := observerWire.Subscribe(ctx, p2p.TopicEvidence)
	if err != nil {
		t.Fatal(err)
	}
	stepToHeight(t, alice, 1)

	forgedVote := signedBobVote(t, 1, 0, types.Hash{1})
	forgedVote.Signature[0] ^= 0xff
	forged, err := consensus.NewConflictingVoteEvidence(forgedVote, signedBobVote(t, 1, 0, types.Hash{2}))
	if err != nil {
		t.Fatal(err)
	}
	// Anyone can put bob's name on a vote; only a vote bob's key signed convicts him.
	framed, err := consensus.NewConflictingVoteEvidence(
		signedNodeTestVote(t, deterministicSignerForID("alice"), "bob", 1, 0, types.Hash{1}),
		signedNodeTestVote(t, deterministicSignerForID("alice"), "bob", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := consensus.NewConflictingVoteEvidence(
		signedNodeTestVote(t, deterministicSignerForID("mallory"), "mallory", 1, 0, types.Hash{1}),
		signedNodeTestVote(t, deterministicSignerForID("mallory"), "mallory", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	sameBlock := signedBobVote(t, 1, 0, types.Hash{1})
	sameBlockProof, err := json.Marshal(consensus.ConflictingVoteProof{First: sameBlock, Second: sameBlock})
	if err != nil {
		t.Fatal(err)
	}
	proposal := signedNodeTestProposal(t, deterministicSignerForID("bob"), consensus.Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}},
		Round:    0,
		Proposer: "bob",
	})
	// A timestamp fault is no evidence: nodes could judge it differently.
	timestampProof, err := json.Marshal(consensus.InvalidProposalProof{Proposal: proposal, Reason: "timestamp", ExpectedHash: types.Hash{1}, ActualHash: types.Hash{2}})
	if err != nil {
		t.Fatal(err)
	}
	nodeLocal := slashing.Evidence{Type: slashing.EvidenceInvalidProposal, Validator: "bob", Height: 1, Round: 0, Proof: timestampProof}

	cases := []struct {
		name     string
		evidence slashing.Evidence
		expected error
	}{
		{"forged vote signature", forged, consensus.ErrInvalidEvidenceVoteSignature},
		{"votes signed by another validator", framed, consensus.ErrInvalidEvidenceVoteSignature},
		{"validator outside the set", stranger, consensus.ErrEvidenceValidatorNotFound},
		{"votes for the same block", slashing.Evidence{Type: slashing.EvidenceConflictingVote, Validator: "bob", Height: 1, Round: 0, Proof: sameBlockProof}, consensus.ErrVotesDoNotConflict},
		{"height that has not been reached", bobEquivocation(t, 9, 0), appslashing.ErrEvidenceFromFuture},
		{"fault nodes could judge differently", nodeLocal, consensus.ErrUnsupportedProposalReason},
		{"no proof", slashing.Evidence{Type: slashing.EvidenceConflictingVote, Validator: "bob", Height: 1, Round: 0}, slashing.ErrEmptyProof},
		{"unknown type", slashing.Evidence{Type: "gossip", Validator: "bob", Height: 1, Round: 0, Proof: []byte("{}")}, slashing.ErrUnknownEvidenceType},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := alice.SubmitEvidence(ctx, test.evidence); !errors.Is(err, test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, err)
			}
			requirePooledEvidence(t, alice)
			requireMempoolTxs(t, alice)
			select {
			case envelope := <-gossip:
				t.Fatalf("refused evidence must not be gossiped, got %v", txHashes([]types.Tx{envelope.Data}))
			default:
			}
			requireStake(t, alice, "bob", evidenceBobPower)
		})
	}
}

// withStoppedStore opens the store of a node that is not running, as the node itself would.
func withStoppedStore(t *testing.T, node *Node, use func(storage *store.LevelDBStore)) {
	t.Helper()
	storage, err := store.OpenLevelDB(node.cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	use(storage)
}

// plantEvidenceRecord writes bytes under the key of an evidence record in the store of a node that
// is not running, as a crash or a bug in an older version could leave them.
func plantEvidenceRecord(t *testing.T, node *Node, key string, value []byte) {
	t.Helper()
	db, err := leveldb.OpenFile(node.cfg.StoreDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err := db.Put(append([]byte("evidence:"), key...), value, nil); err != nil {
		t.Fatal(err)
	}
}

// The pool of verified evidence outlives a restart and the mempool does not, so a node queues what
// it pooled for a block again when it starts.
func TestNodeQueuesPooledEvidenceWhenItStarts(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	bus := transport.NewInMemoryBus()
	first := newEvidenceNode(t, bus, evidenceGenesis(), "alice", dataDir)
	startNode(t, first)
	stepToHeight(t, first, 1)
	evidence := bobEquivocation(t, 1, 0)
	// The node stops between pooling the evidence and queueing its transaction.
	admission, err := evidenceRuntime(t, first).AdmitEvidence(ctx, evidence)
	if err != nil || !admission.New {
		t.Fatalf("expected new evidence to be pooled, got new=%t err=%v", admission.New, err)
	}
	requireMempoolTxs(t, first)
	if err := first.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	restarted := newEvidenceNode(t, bus, evidenceGenesis(), "alice", dataDir)
	startEvidenceNode(t, restarted)
	tx := evidenceTx(t, evidence)
	requirePooledEvidence(t, restarted, evidence)
	requireMempoolTxs(t, restarted, tx)
	requireStake(t, restarted, "bob", evidenceBobPower)
	// The queued transaction is one the node would propose: it still passes admission.
	pending, err := restarted.PendingTxs(ctx)
	if err != nil || len(pending) != 1 || !bytes.Equal(pending[0], tx) {
		t.Fatalf("expected the re-queued evidence to be pending, got %v err=%v", txHashes(pending), err)
	}
}

// A restart judges the pool again against the chain. Records that cannot be applied any more, or that
// were never evidence, are dropped; none of them keeps the node from starting, and none is queued.
func TestNodeStartDropsPooledEvidenceThatCannotBeApplied(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	bus := transport.NewInMemoryBus()
	first := newEvidenceNode(t, bus, evidenceGenesis(), "alice", dataDir)
	startNode(t, first)
	stepToHeight(t, first, 1)
	punished := bobEquivocation(t, 1, 0)
	if _, err := first.SubmitEvidence(ctx, punished); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 12 && stakeOf(t, first, "bob") == evidenceBobPower; step++ {
		stepNode(t, first)
	}
	requireStake(t, first, "bob", evidenceBobStakeAfterSlash)
	if err := first.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	forgedVote := signedBobVote(t, 2, 2, types.Hash{1})
	forgedVote.Signature[0] ^= 0xff
	forged, err := consensus.NewConflictingVoteEvidence(forgedVote, signedBobVote(t, 2, 2, types.Hash{2}))
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := consensus.NewConflictingVoteEvidence(
		signedNodeTestVote(t, deterministicSignerForID("mallory"), "mallory", 2, 3, types.Hash{1}),
		signedNodeTestVote(t, deterministicSignerForID("mallory"), "mallory", 2, 3, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Another offence of bob, which no block has punished yet.
	pending := bobEquivocation(t, 2, 1)
	withStoppedStore(t, first, func(storage *store.LevelDBStore) {
		for _, evidence := range []slashing.Evidence{punished, forged, stranger, pending} {
			if err := storage.SaveEvidence(ctx, store.EvidenceRecord{Evidence: evidence}); err != nil {
				t.Fatal(err)
			}
		}
	})
	plantEvidenceRecord(t, first, store.EvidenceKey(bobEquivocation(t, 2, 4)), []byte("{not json"))
	plantEvidenceRecord(t, first, "no-offence-here", []byte(`{"Evidence":{}}`))

	restarted := newEvidenceNode(t, bus, evidenceGenesis(), "alice", dataDir)
	startEvidenceNode(t, restarted)
	requirePooledEvidence(t, restarted, pending)
	requireMempoolTxs(t, restarted, evidenceTx(t, pending))
	requireStake(t, restarted, "bob", evidenceBobStakeAfterSlash)
}

// waitForEvidenceState polls until condition holds, for state that a commit settles after the
// block is already visible.
func waitForEvidenceState(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(transportTestWaitTimeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	if !condition() {
		t.Fatalf("timed out waiting for %s", what)
	}
}

// Verified evidence reaches every node by gossip. Each node verifies it for itself, pools it and
// queues its transaction, whichever node proposes next carries it into a block, and every node
// punishes the offender when it executes that block.
func TestNodeGossipedEvidenceSlashesOffenderOnEveryNode(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir())
	bob := newEvidenceNode(t, bus, evidenceGenesis(), "bob", t.TempDir())
	// Carol follows the chain without voting in it.
	carol := newEvidenceNode(t, bus, evidenceGenesis(), "carol", t.TempDir(), func(cfg *Config) { cfg.ValidatorID = "" })
	nodes := []*Node{alice, bob, carol}
	for _, node := range nodes {
		startEvidenceNode(t, node)
	}
	stepToHeight(t, alice, 1)
	waitForNodeHeight(t, bob, 1)
	waitForNodeHeight(t, carol, 1)

	evidence := bobEquivocation(t, 1, 0)
	tx := evidenceTx(t, evidence)
	if _, err := alice.SubmitEvidence(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		waitForMempoolLen(t, node, 1)
		requireMempoolTxs(t, node, tx)
		requirePooledEvidence(t, node, evidence)
		requireStake(t, node, "bob", evidenceBobPower)
	}

	loop := ConsensusLoopConfig{Interval: 5 * time.Millisecond, RoundTimeout: time.Hour, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: false, ExecutionCommitMode: ExecutionCommitModeFinalized}
	for _, node := range []*Node{alice, bob} {
		if err := node.StartConsensusLoop(ctx, loop); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = node.StopConsensusLoop(ctx) })
	}
	for _, node := range nodes {
		waitForEvidenceState(t, "the offender to be punished on every node", func() bool {
			return stakeOf(t, node, "bob") == evidenceBobStakeAfterSlash
		})
	}

	carrier, _, executed := blockWithTx(t, alice, tx)
	if !executed {
		t.Fatal("the gossiped evidence never reached a block")
	}
	for _, node := range nodes {
		record, _, found := blockWithTx(t, node, tx)
		if !found || record.Hash != carrier.Hash || record.AppHash != carrier.AppHash {
			t.Fatalf("expected every node to execute the same evidence block %x, got found=%t %x", carrier.Hash, found, record.Hash)
		}
		requireOffenderSlashed(t, node, record)
		waitForEvidenceState(t, "the settled evidence to leave the pool and the mempool", func() bool {
			return len(pooledEvidence(t, node)) == 0 && len(mempoolTxs(t, node)) == 0
		})
		requireEvidenceSettled(t, node, evidence)
	}
}

// A peer is judged by the evidence it sends: a message that is not verifiable evidence costs it
// score, while evidence that is merely ahead of this node does not, and evidence that verifies is
// pooled and queued.
func TestNodeScoresPeersByTheEvidenceTheyGossip(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir(), func(cfg *Config) {
		cfg.Chain.P2P.InitialScore = 10
		cfg.Chain.P2P.ValidMessageReward = 2
		cfg.Chain.P2P.InvalidMessageCost = 3
		cfg.Chain.P2P.BanThreshold = 0
		cfg.Chain.P2P.WindowResetInterval = time.Hour
		cfg.Chain.P2P.ScoreRecovery = 0
	})
	startEvidenceNode(t, alice)
	stepToHeight(t, alice, 1)
	mallory, err := bus.NewPeer("mallory")
	if err != nil {
		t.Fatal(err)
	}
	if err := mallory.Start(ctx); err != nil {
		t.Fatal(err)
	}
	publish := func(tx types.Tx) {
		t.Helper()
		if err := mallory.Publish(ctx, p2p.TopicEvidence, tx); err != nil {
			t.Fatal(err)
		}
	}

	forgedVote := signedBobVote(t, 1, 0, types.Hash{1})
	forgedVote.Signature[0] ^= 0xff
	forged, err := consensus.NewConflictingVoteEvidence(forgedVote, signedBobVote(t, 1, 0, types.Hash{2}))
	if err != nil {
		t.Fatal(err)
	}
	valid := bobEquivocation(t, 1, 0)

	publish(types.Tx("slashing:evidence:not-evidence"))
	waitForPeerScore(t, alice, "mallory", 7)
	publish(evidenceTx(t, forged))
	waitForPeerScore(t, alice, "mallory", 4)
	requirePooledEvidence(t, alice)
	requireMempoolTxs(t, alice)

	publish(evidenceTx(t, bobEquivocation(t, 9, 0)))
	waitForPeerScore(t, alice, "mallory", 6)
	requirePooledEvidence(t, alice)
	requireMempoolTxs(t, alice)

	publish(evidenceTx(t, valid))
	waitForPeerScore(t, alice, "mallory", 8)
	waitForMempoolLen(t, alice, 1)
	requireMempoolTxs(t, alice, evidenceTx(t, valid))
	requirePooledEvidence(t, alice, valid)
	requireStake(t, alice, "bob", evidenceBobPower)
}

// The views a node votes in run ahead of the chain it has executed, so it detects equivocation of
// heights its evidence pool cannot judge yet. That evidence is not lost: it is pooled and gossiped
// once the chain catches up, queued once its height commits, and a later block punishes the offender.
func TestNodePunishesEquivocationItDetectedAheadOfTheChain(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir())
	startEvidenceNode(t, alice)
	observerWire, err := bus.NewPeer("observer")
	if err != nil {
		t.Fatal(err)
	}
	if err := observerWire.Start(ctx); err != nil {
		t.Fatal(err)
	}
	gossip, err := observerWire.Subscribe(ctx, p2p.TopicEvidence)
	if err != nil {
		t.Fatal(err)
	}
	stepToHeight(t, alice, 1)

	machine, err := alice.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	view := machine.Status(ctx)
	if view.Height <= alice.Status(ctx).LatestHeight+1 {
		t.Fatalf("expected the view being voted on to run ahead of the executed chain, got view %d and chain %d", view.Height, alice.Status(ctx).LatestHeight)
	}
	evidence := bobEquivocation(t, view.Height, view.Round)
	tx := evidenceTx(t, evidence)
	// Bob's two votes name blocks this node has never seen, as they do when it is a step behind.
	for _, blockHash := range []types.Hash{{1}, {2}} {
		if err := alice.localVoter.OnVote(ctx, signedBobVote(t, view.Height, view.Round, blockHash)); err != nil {
			t.Fatal(err)
		}
	}
	requirePooledEvidence(t, alice)
	requireMempoolTxs(t, alice)

	stepUntil(t, alice, "a chain the pool can judge the evidence against", func() bool { return len(pooledEvidence(t, alice)) > 0 })
	requirePooledEvidence(t, alice, evidence)
	requireMempoolTxs(t, alice)
	select {
	case envelope := <-gossip:
		if !bytes.Equal(envelope.Data, tx) {
			t.Fatalf("expected the evidence transaction on the wire, got %v", txHashes([]types.Tx{envelope.Data}))
		}
	default:
		t.Fatal("pooled evidence must be gossiped")
	}

	stepUntil(t, alice, "the commit of the evidence height", func() bool { return len(mempoolTxs(t, alice)) > 0 })
	requireMempoolTxs(t, alice, tx)
	if committed := alice.Status(ctx).LatestHeight; committed < evidence.Height {
		t.Fatalf("expected the evidence to be queued by the commit of its height %d, got %d", evidence.Height, committed)
	}
	requireStake(t, alice, "bob", evidenceBobPower)

	stepUntil(t, alice, "the punishment of the offender", func() bool { return stakeOf(t, alice, "bob") != evidenceBobPower })
	carrier, _, executed := blockWithTx(t, alice, tx)
	if !executed || carrier.Block.Header.Height <= evidence.Height {
		t.Fatalf("expected a block above the evidence height to carry the evidence, got executed=%t height=%d", executed, carrier.Block.Header.Height)
	}
	requireOffenderSlashed(t, alice, carrier)
	requireEvidenceSettled(t, alice, evidence)
}

// A validator that equivocates in every view cannot make a node hold back unbounded evidence.
func TestNodeBoundsTheEvidenceItHoldsBack(t *testing.T) {
	node := &Node{}
	for round := 0; round < maxDeferredEvidence+8; round++ {
		evidence := bobEquivocation(t, 3, types.Round(round))
		node.deferEvidence(evidence)
		node.deferEvidence(evidence)
	}
	if held := len(node.deferredEvidence); held != maxDeferredEvidence {
		t.Fatalf("expected %d distinct evidence records held back, got %d", maxDeferredEvidence, held)
	}
}

// One offence is one transaction. Other proofs of an offence the pool holds already, whether they
// are submitted as evidence, as a transaction or by a peer, queue nothing new: the mempool keeps
// the transaction of the evidence pooled first, so an equivocator's many conflicting votes cannot
// fill it and the blocks with free transactions of which only the first can apply.
func TestNodeQueuesOneTransactionPerOffence(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir())
	startEvidenceNode(t, alice)
	stepToHeight(t, alice, 1)

	first := bobEquivocationOf(t, 1, 0, types.Hash{1}, types.Hash{2})
	firstTx := evidenceTx(t, first)
	if _, err := alice.SubmitEvidence(ctx, first); err != nil {
		t.Fatal(err)
	}
	requireMempoolTxs(t, alice, firstTx)

	second := bobEquivocationOf(t, 1, 0, types.Hash{1}, types.Hash{3})
	hash, err := alice.SubmitEvidence(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if hash != mempool.HashTx(firstTx) {
		t.Fatalf("expected the transaction of the pooled evidence %x, got %x", mempool.HashTx(firstTx), hash)
	}
	if err := alice.SubmitTx(ctx, evidenceTx(t, bobEquivocationOf(t, 1, 0, types.Hash{2}, types.Hash{3}))); err != nil {
		t.Fatal(err)
	}
	alice.acceptGossipTxBatch(ctx, "carol", encodeTxBatch([]types.Tx{evidenceTx(t, bobEquivocationOf(t, 1, 0, types.Hash{4}, types.Hash{5}))}))
	alice.acceptEvidenceMessage(ctx, "carol", evidenceTx(t, bobEquivocationOf(t, 1, 0, types.Hash{6}, types.Hash{7})))
	requireMempoolTxs(t, alice, firstTx)
	requirePooledEvidence(t, alice, first)

	if err := alice.SubmitTx(ctx, types.Tx("slashing:evidence:not-evidence")); err == nil {
		t.Fatal("expected a transaction that is not evidence to be refused")
	}
	requireMempoolTxs(t, alice, firstTx)
}

// A validator that refuses a proposal because its header names another app hash than the certified
// one reports the proposer: the evidence is pooled once the chain reached its height, and the
// slashing module judges it against the app hash the certified blocks committed.
func TestValidatorReportsAProposalNamingAnotherAppHash(t *testing.T) {
	ctx := context.Background()
	bus := transport.NewInMemoryBus()
	alice := newEvidenceNode(t, bus, evidenceGenesis(), "alice", t.TempDir())
	startEvidenceNode(t, alice)
	stepToHeight(t, alice, 2)

	committed := alice.Status(ctx).LatestHeight
	runtime := evidenceRuntime(t, alice)
	block, err := runtime.Store.BlockByHeight(ctx, committed)
	if err != nil {
		t.Fatal(err)
	}
	forged := consensus.Proposal{Block: block.Block, Round: block.Block.Header.Round, Proposer: "bob"}
	forged.Block.Header.AppHash[0] ^= 0xff
	signature, err := vexocrypto.SignWithDomain(deterministicSignerForID("bob"), vexocrypto.DomainConsensusProposal, consensus.ProposalSignBytes(forged))
	if err != nil {
		t.Fatal(err)
	}
	forged.Signature = signature
	refusal := alice.processProposal(ctx, forged)
	if !errors.Is(refusal, ErrAppHashMismatch) {
		t.Fatalf("expected the forged app hash to be refused, got %v", refusal)
	}
	alice.reportInvalidAppHashProposal(ctx, forged, refusal)

	pooled := pooledEvidence(t, alice)
	if len(pooled) != 1 {
		t.Fatalf("expected the proposal to be reported, pooled %v", pooled)
	}
	record, err := runtime.Store.EvidenceByKey(ctx, pooled[0])
	if err != nil {
		t.Fatal(err)
	}
	if record.Evidence.Type != slashing.EvidenceInvalidProposal || record.Evidence.Validator != "bob" {
		t.Fatalf("expected invalid-proposal evidence against bob, got %+v", record.Evidence)
	}
	stepUntil(t, alice, "the evidence to be applied", func() bool { return len(pooledEvidence(t, alice)) == 0 })
	offence, err := slashing.OffenceKey(record.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Store.Get(ctx, appslashing.ModuleName, []byte("offence/"+offence)); err != nil {
		t.Fatalf("expected a block to punish the offence, got %v", err)
	}

	// The proposer's own proposals and a refusal for another reason are not reported.
	alice.reportInvalidAppHashProposal(ctx, forged, ErrProposalRejected)
	own := forged
	own.Proposer = "alice"
	alice.reportInvalidAppHashProposal(ctx, own, refusal)
	if pooled := pooledEvidence(t, alice); len(pooled) != 0 {
		t.Fatalf("expected nothing more to be reported, pooled %v", pooled)
	}
}

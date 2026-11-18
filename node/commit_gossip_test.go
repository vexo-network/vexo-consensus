package node

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	gossipInitialScore = 10
	gossipValidReward  = 1
	gossipInvalidCost  = 3

	// The views of the honest chain the fixture builds: the block at height h is
	// proposed and certified in view gossipBlockView+h-1, directly after the view
	// of its parent, and QC(C) of its child in the view that follows.
	gossipBlockView types.Round = 3
)

// commitFixture is a running single-validator node and the validator's key, so a
// test can issue quorum certificates the node accepts as genuine.
type commitFixture struct {
	t                *testing.T
	node             *Node
	signer           vexocrypto.Signer
	validatorSetHash types.Hash
}

func newCommitFixture(t *testing.T) *commitFixture {
	t.Helper()
	return newCommitFixtureOn(t, nil)
}

// newCommitFixtureOn is newCommitFixture with the node running on wire, none when wire is nil.
func newCommitFixtureOn(t *testing.T, wire transport.Transport) *commitFixture {
	t.Helper()
	cfg := UnsafeTestConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	cfg.Chain.P2P.InitialScore = gossipInitialScore
	cfg.Chain.P2P.ValidMessageReward = gossipValidReward
	cfg.Chain.P2P.InvalidMessageCost = gossipInvalidCost
	cfg.Chain.P2P.BanThreshold = -1000
	cfg.Chain.P2P.ScoreRecovery = 0
	node, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	signer := deterministicSignerForID("alice")
	node.WithSigner(signer)
	if wire != nil {
		node.WithTransport(wire)
	}
	startNode(t, node)
	t.Cleanup(func() { node.Stop(context.Background()) })

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	validatorSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return &commitFixture{t: t, node: node, signer: signer, validatorSetHash: validatorSet.Hash()}
}

// block builds a block whose body is consistent with its header: the
// transactions in the deterministic fair order, with the transaction root and
// the data availability commitment of that order. It is the block of a direct
// chain: proposed in the view after its parent's, naming the validator set its
// parent fixes for it.
func (fixture *commitFixture) block(height types.Height, parent types.Hash, txs ...types.Tx) types.Block {
	round := gossipBlockView + types.Round(height-1)
	// Every block proves the commit of its grandparent, so a block commits the state one block below
	// its grandparent and is certified by the set after its parent's state.
	var stateHeight types.Height
	if height > 3 {
		stateHeight = height - 3
	}
	validatorSetHeight := finality.FirstValidatorSetHeight
	if height > 4 {
		validatorSetHeight = height - 3
	}
	block := types.Block{
		Header: types.Header{
			ChainID:              "vexo-test",
			Height:               height,
			TimeUnixNano:         int64(height),
			Round:                round,
			JustifyRound:         round - 1,
			PreviousBlockHash:    parent,
			ValidatorSetHeight:   validatorSetHeight,
			StateHeight:          stateHeight,
			ValidatorSetHash:     fixture.validatorSetHash,
			NextValidatorSetHash: fixture.validatorSetHash,
		},
		Txs: fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt("vexo-test", height)),
	}
	// The header commits the app hash of its state.
	block.Header.AppHash = fixture.appHashAt(stateHeight)
	block.Header.TxRoot = consensus.TxRoot(block.Txs)
	if len(block.Txs) > 0 {
		block = dataavailability.AttachCommitment(block)
	}
	return block
}

// appHashAt is the app hash a header committing the state of height names.
func (fixture *commitFixture) appHashAt(height types.Height) types.Hash {
	fixture.t.Helper()
	runtime, err := fixture.node.Runtime()
	if err != nil {
		fixture.t.Fatal(err)
	}
	appHash, err := runtime.HeaderAppHashAt(context.Background(), height)
	if err != nil {
		fixture.t.Fatal(err)
	}
	return appHash
}

// quorumCert is a certificate the validator set signed for a block in a view.
func (fixture *commitFixture) quorumCert(height types.Height, round types.Round, blockHash types.Hash) finality.QuorumCert {
	fixture.t.Helper()
	vote := signedNodeTestVote(fixture.t, fixture.signer, "alice", height, round, blockHash)
	aggregate, err := (vexocrypto.DeterministicAggregateSigner{}).Aggregate([]types.Signature{vote.Signature})
	if err != nil {
		fixture.t.Fatal(err)
	}
	return finality.QuorumCert{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		Signers:     finality.EncodeSigners([]types.ValidatorID{"alice"}),
		Signature:   aggregate,
		VotingPower: 1,
	}
}

// commit is the gossip message for a block that is final under the 2-chain
// rule: QC(B) in the block's view, and its child with QC(C) one view later.
func (fixture *commitFixture) commit(block types.Block) commitMessage {
	fixture.t.Helper()
	return fixture.commitWithChildView(block, block.Header.Round+1)
}

// commitWithChildView is the commit of block whose child is proposed and
// certified in childView, justified by QC(B).
func (fixture *commitFixture) commitWithChildView(block types.Block, childView types.Round) commitMessage {
	fixture.t.Helper()
	blockHash := consensus.HashBlock(block)
	quorumCert := fixture.quorumCert(block.Header.Height, block.Header.Round, blockHash)
	child := fixture.block(block.Header.Height+1, blockHash)
	child.Header.Round = childView
	child.Header.JustifyRound = block.Header.Round
	child.Header.ValidatorSetHeight = finality.ChildValidatorSetHeight(block.Header)
	child.Header.StateHeight = finality.ChildStateHeight(block.Header)
	child.Header.AppHash = fixture.appHashAt(child.Header.StateHeight)
	childQC := fixture.quorumCert(child.Header.Height, childView, finality.HeaderHash(child.Header))
	proof := finality.NewProof(block.Header, quorumCert).WithChild(child.Header, childQC)
	return commitMessage{Block: block, QuorumCert: quorumCert, FinalityProof: proof}
}

// deliver gossips message from a peer and reports the height the node reached
// and the peer's score afterwards.
func (fixture *commitFixture) deliver(message commitMessage) (types.Height, int64) {
	fixture.t.Helper()
	fixture.node.acceptCommitMessage(context.Background(), "bob", appendCommitMessage(nil, message))
	score, err := fixture.node.PeerScore(context.Background(), "bob")
	if err != nil {
		fixture.t.Fatal(err)
	}
	return fixture.node.Status(context.Background()).LatestHeight, score
}

func (fixture *commitFixture) requireRejected(message commitMessage, why string) {
	fixture.t.Helper()
	height, score := fixture.deliver(message)
	if height != 0 {
		fixture.t.Fatalf("%s: the commit must not be applied, node is at height %d", why, height)
	}
	if want := int64(gossipInitialScore - gossipInvalidCost); score != want {
		fixture.t.Fatalf("%s: the peer must be penalized as invalid, score %d want %d", why, score, want)
	}
}

func TestCommitGossipAppliesACommitWithAVerifiedTwoChainProof(t *testing.T) {
	fixture := newCommitFixture(t)
	block := fixture.block(1, types.Hash{}, types.Tx("bank:send:a"), types.Tx("bank:send:b"))

	height, score := fixture.deliver(fixture.commit(block))
	if height != 1 {
		t.Fatalf("expected the verified commit to be applied, node is at height %d", height)
	}
	if want := int64(gossipInitialScore + gossipValidReward); score != want {
		t.Fatalf("expected the peer to be rewarded, score %d want %d", score, want)
	}
	record, err := fixture.node.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Block.Txs) != 2 || record.Hash != consensus.HashBlock(block) {
		t.Fatalf("unexpected committed block: %+v", record)
	}
}

// A quorum certificate vouches for a header hash. A peer that replays a genuine
// commit with other transactions must not get them executed.
func TestCommitGossipRejectsSwappedTransactions(t *testing.T) {
	t.Run("transactions replaced under the genuine header", func(t *testing.T) {
		fixture := newCommitFixture(t)
		message := fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a"), types.Tx("bank:send:b")))
		message.Block.Txs = []types.Tx{types.Tx("bank:send:mallory")}
		fixture.requireRejected(message, "swapped transactions")
	})
	t.Run("transactions replaced with a matching header root", func(t *testing.T) {
		fixture := newCommitFixture(t)
		message := fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a"), types.Tx("bank:send:b")))
		swapped := fixture.block(1, types.Hash{}, types.Tx("bank:send:mallory"))
		message.Block = swapped
		fixture.requireRejected(message, "swapped transactions with a rebuilt header")
	})
	t.Run("a transaction appended", func(t *testing.T) {
		fixture := newCommitFixture(t)
		message := fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a")))
		message.Block.Txs = append(message.Block.Txs, types.Tx("bank:send:mallory"))
		fixture.requireRejected(message, "appended transaction")
	})
	t.Run("transactions dropped", func(t *testing.T) {
		fixture := newCommitFixture(t)
		message := fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a")))
		message.Block.Txs = nil
		fixture.requireRejected(message, "dropped transactions")
	})
}

// Validators vote for a header hash; a body that breaks the deterministic
// ordering or the data availability commitment is invalid however many of them
// signed. The body is checked on its own, apart from the hash binding.
func TestCommitGossipRejectsACertifiedBlockWithAnInvalidBody(t *testing.T) {
	ordered := func(fixture *commitFixture) types.Block {
		return fixture.block(1, types.Hash{}, types.Tx("bank:send:a"), types.Tx("bank:send:b"), types.Tx("bank:send:c"))
	}
	reseal := func(block types.Block) types.Block {
		block.Header.TxRoot = consensus.TxRoot(block.Txs)
		return dataavailability.AttachCommitment(block)
	}
	cases := []struct {
		name   string
		mutate func(block types.Block) types.Block
	}{
		{
			name: "transactions out of the fair order",
			mutate: func(block types.Block) types.Block {
				block.Txs = []types.Tx{block.Txs[2], block.Txs[1], block.Txs[0]}
				return reseal(block)
			},
		},
		{
			name: "no data availability commitment",
			mutate: func(block types.Block) types.Block {
				block.Header.ConsensusHash = types.Hash{}
				return block
			},
		},
		{
			name: "data availability commitment of other transactions",
			mutate: func(block types.Block) types.Block {
				block.Header.ConsensusHash = dataavailability.Commitment([]types.Tx{types.Tx("other")})
				return block
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newCommitFixture(t)
			block := testCase.mutate(ordered(fixture))
			if err := consensus.VerifyBlockBody(block); !errors.Is(err, consensus.ErrInvalidBlockBody) {
				t.Fatalf("the fixture must build an invalid body, got %v", err)
			}
			// The validators did sign this block: the hash binding holds and
			// only the body check can refuse it.
			message := fixture.commit(block)
			err := fixture.node.verifyCommitFinalityProof(context.Background(), message.Block, message.QuorumCert, message.FinalityProof)
			if !errors.Is(err, consensus.ErrInvalidBlockBody) {
				t.Fatalf("expected the invalid body to be refused, got %v", err)
			}
			fixture.requireRejected(message, testCase.name)
		})
	}
}

// Under the 2-chain rule B is final only when QC(C) was formed in the view right
// after QC(B). Two certificates in other views prove no commit, however valid
// their signatures are.
func TestCommitGossipRejectsACommitChainOverNonConsecutiveViews(t *testing.T) {
	cases := []struct {
		name      string
		childView types.Round
	}{
		{"a view skipped", gossipBlockView + 2},
		{"the same view", gossipBlockView},
		{"an earlier view", gossipBlockView - 1},
		{"a distant view", gossipBlockView + 1000},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newCommitFixture(t)
			block := fixture.block(1, types.Hash{}, types.Tx("bank:send:a"))
			message := fixture.commitWithChildView(block, testCase.childView)
			err := fixture.node.verifyCommitFinalityProof(context.Background(), message.Block, message.QuorumCert, message.FinalityProof)
			if !errors.Is(err, finality.ErrViewsNotConsecutive) {
				t.Fatalf("expected non-consecutive views to be refused, got %v", err)
			}
			fixture.requireRejected(message, testCase.name)
		})
	}
}

// The validator set that must have signed is the one the block's header names,
// and that is the one its parent fixes for it. A proof naming another set would
// pick which keys judge it, and a header naming another set than its parent
// fixes is not a block of this chain.
func TestCommitGossipRequiresTheValidatorSetHeightOfTheBlock(t *testing.T) {
	t.Run("a proof naming another set than its header", func(t *testing.T) {
		for _, validatorSetHeight := range []types.Height{0, 2, 99} {
			fixture := newCommitFixture(t)
			message := fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a")))
			message.FinalityProof.ValidatorSetHeight = validatorSetHeight
			err := fixture.node.verifyCommitFinalityProof(context.Background(), message.Block, message.QuorumCert, message.FinalityProof)
			if !errors.Is(err, finality.ErrHeightMismatch) {
				t.Fatalf("validator set height %d: expected a height mismatch, got %v", validatorSetHeight, err)
			}
			fixture.requireRejected(message, "validator set height")
		}
	})
	t.Run("a header naming another set than its parent fixes", func(t *testing.T) {
		fixture := newCommitFixture(t)
		first := fixture.block(1, types.Hash{}, types.Tx("bank:send:a"))
		if height, _ := fixture.deliver(fixture.commit(first)); height != 1 {
			t.Fatalf("expected the first block to be applied, node is at height %d", height)
		}
		second := fixture.block(2, consensus.HashBlock(first), types.Tx("bank:send:b"))
		if height, _ := fixture.deliver(fixture.commit(second)); height != 2 {
			t.Fatalf("expected the second block to be applied, node is at height %d", height)
		}
		third := fixture.block(3, consensus.HashBlock(second), types.Tx("bank:send:c"))
		if height, _ := fixture.deliver(fixture.commit(third)); height != 3 {
			t.Fatalf("expected the third block to be applied, node is at height %d", height)
		}
		// Block 3 directly follows the view of its parent, so it proves block 2 committed: block 4
		// commits the state one block below it, after block 1, and names the set after it as next,
		// the set of registry height 2 that certifies block 5. A quorum of an older set certifying a
		// block 5 that keeps registry height 1 does not make it a block of this chain.
		fourth := fixture.block(4, consensus.HashBlock(third), types.Tx("bank:send:d"))
		if fourth.Header.ValidatorSetHeight != 1 || fourth.Header.StateHeight != 1 {
			t.Fatalf("expected block 4 to commit state 1 on registry height 1, got %+v", fourth.Header)
		}
		if height, _ := fixture.deliver(fixture.commit(fourth)); height != 4 {
			t.Fatalf("expected the fourth block to be applied, node is at height %d", height)
		}
		fifth := fixture.block(5, consensus.HashBlock(fourth), types.Tx("bank:send:e"))
		if fifth.Header.ValidatorSetHeight != 2 {
			t.Fatalf("expected block 5 to name registry height 2, got %d", fifth.Header.ValidatorSetHeight)
		}
		stale := fifth
		stale.Header.ValidatorSetHeight = 1
		if height, _ := fixture.deliver(fixture.commit(stale)); height != 4 {
			t.Fatalf("expected the block naming a stale validator set to be refused, node is at height %d", height)
		}
		misnamed := fifth
		misnamed.Header.NextValidatorSetHash = types.Hash{0xEE}
		if height, _ := fixture.deliver(fixture.commit(misnamed)); height != 4 {
			t.Fatalf("expected the block naming another next set to be refused, node is at height %d", height)
		}
		if height, _ := fixture.deliver(fixture.commit(fifth)); height != 5 {
			t.Fatalf("expected the block naming the set its parent fixes to be applied, node is at height %d", height)
		}
	})
}

// Every way a gossiped commit can fail verification is the sender's fault.
func TestCommitGossipPenalizesEveryFailedVerification(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(fixture *commitFixture, message *commitMessage)
	}{
		{"no finality proof", func(_ *commitFixture, message *commitMessage) {
			message.FinalityProof = finality.Proof{}
		}},
		{"proof of another block", func(fixture *commitFixture, message *commitMessage) {
			other := fixture.block(1, types.Hash{}, types.Tx("bank:send:other"))
			message.FinalityProof = fixture.commit(other).FinalityProof
		}},
		{"proof certificate differs from the commit certificate", func(fixture *commitFixture, message *commitMessage) {
			message.QuorumCert = fixture.quorumCert(1, gossipBlockView+10, message.QuorumCert.BlockHash)
		}},
		{"forged signature on QC(B)", func(_ *commitFixture, message *commitMessage) {
			message.QuorumCert.Signature = types.AggregateSignature("forged")
			message.FinalityProof.QuorumCert = message.QuorumCert
		}},
		{"forged signature on QC(C)", func(_ *commitFixture, message *commitMessage) {
			message.FinalityProof.CommitChain[0].QuorumCert.Signature = types.AggregateSignature("forged")
		}},
		{"certificate signed by an unknown validator", func(_ *commitFixture, message *commitMessage) {
			message.FinalityProof.CommitChain[0].QuorumCert.Signers = finality.EncodeSigners([]types.ValidatorID{"mallory"})
		}},
		{"no child", func(_ *commitFixture, message *commitMessage) {
			message.FinalityProof.CommitChain = nil
		}},
		{"two children", func(_ *commitFixture, message *commitMessage) {
			chain := message.FinalityProof.CommitChain
			message.FinalityProof.CommitChain = append(chain, chain[0])
		}},
		{"child of another parent", func(_ *commitFixture, message *commitMessage) {
			link := &message.FinalityProof.CommitChain[0]
			link.Header.PreviousBlockHash = types.Hash{0xAB}
			link.BlockHash = finality.HeaderHash(link.Header)
		}},
		{"child two heights up", func(_ *commitFixture, message *commitMessage) {
			link := &message.FinalityProof.CommitChain[0]
			link.Header.Height++
			link.BlockHash = finality.HeaderHash(link.Header)
		}},
		{"proof of another validator set", func(_ *commitFixture, message *commitMessage) {
			message.FinalityProof.ValidatorSetHash = types.Hash{0xCD}
		}},
		{"bodiless announcement carrying transactions", func(_ *commitFixture, message *commitMessage) {
			message.Bodiless = true
		}},
		{"bodiless announcement with a forged proof", func(_ *commitFixture, message *commitMessage) {
			message.Block.Txs = nil
			message.Bodiless = true
			message.FinalityProof.CommitChain[0].QuorumCert.Signature = types.AggregateSignature("forged")
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newCommitFixture(t)
			message := fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a")))
			testCase.mutate(fixture, &message)
			fixture.requireRejected(message, testCase.name)
		})
	}
}

// A node that cannot run the verification judges nobody: a missing signature
// verifier would otherwise ban every honest peer that sends a genuine commit.
func TestCommitGossipDoesNotPenalizeAPeerForALocalVerificationFault(t *testing.T) {
	fixture := newCommitFixture(t)
	runtime, err := fixture.node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	runtime.Crypto.FinalityVerifier = nil

	height, score := fixture.deliver(fixture.commit(fixture.block(1, types.Hash{}, types.Tx("bank:send:a"))))
	if height != 0 {
		t.Fatalf("a commit that could not be verified must not be applied, node is at height %d", height)
	}
	if score != gossipInitialScore {
		t.Fatalf("the peer is not at fault when the node cannot verify, score %d want %d", score, gossipInitialScore)
	}
}

func TestCommitVerificationUnavailableSeparatesLocalFaultsFromBadProofs(t *testing.T) {
	local := []error{
		context.Canceled,
		context.DeadlineExceeded,
		finality.ErrMissingSignatureVerifier,
		errors.Join(errCommitVerifierUnavailable, ErrNodeNotRunning),
		// The set the header names depends on a block this node has not executed.
		consensus.ErrValidatorSetPending,
	}
	for _, err := range local {
		if !commitVerificationUnavailable(err) {
			t.Fatalf("%v is a failure of the node, not of the proof", err)
		}
	}
	bad := []error{
		ErrInvalidCommitQC,
		consensus.ErrInvalidBlockBody,
		finality.ErrViewsNotConsecutive,
		finality.ErrMissingQCSignature,
		finality.ErrValidatorSetMismatch,
		finality.ErrCommitChainTooShort,
	}
	for _, err := range bad {
		if commitVerificationUnavailable(err) {
			t.Fatalf("%v is a failure of the proof, not of the node", err)
		}
	}
}

// A header commits the app hash of the state its validator set was taken from: genesis for the
// blocks that keep the first set, the state after block 1 for block 3. A certified block whose
// header names another app hash is not a block of the chain this node executed, and the node
// refuses it instead of building on a state no one vouched for.
func TestCommitGossipRefusesAHeaderNamingAnotherAppHash(t *testing.T) {
	ctx := context.Background()
	fixture := newCommitFixture(t)
	runtime, err := fixture.node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := runtime.HeaderAppHashAt(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if genesis == (types.Hash{}) {
		t.Fatal("expected the genesis app hash to be persisted")
	}

	forged := fixture.block(1, types.Hash{}, types.Tx("bank:send:a"))
	forged.Header.AppHash[0] ^= 0xff
	// The quorum certified the forged header, so the relaying peer did not vouch for anything the
	// quorum did not; the node does not apply it, since its own state disagrees.
	if height, _ := fixture.deliver(fixture.commit(forged)); height != 0 {
		t.Fatalf("expected the block naming a forged app hash to be refused, node is at height %d", height)
	}

	first := fixture.block(1, types.Hash{}, types.Tx("bank:send:a"))
	if first.Header.AppHash != genesis {
		t.Fatalf("expected block 1 to commit the genesis app hash %x, got %x", genesis, first.Header.AppHash)
	}
	if height, _ := fixture.deliver(fixture.commit(first)); height != 1 {
		t.Fatalf("expected the first block to be applied, node is at height %d", height)
	}
	second := fixture.block(2, consensus.HashBlock(first), types.Tx("bank:send:b"))
	if height, _ := fixture.deliver(fixture.commit(second)); height != 2 {
		t.Fatalf("expected the second block to be applied, node is at height %d", height)
	}
	third := fixture.block(3, consensus.HashBlock(second), types.Tx("bank:send:c"))
	if height, _ := fixture.deliver(fixture.commit(third)); height != 3 {
		t.Fatalf("expected the third block to be applied, node is at height %d", height)
	}
	afterFirst, err := runtime.HeaderAppHashAt(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	fourth := fixture.block(4, consensus.HashBlock(third), types.Tx("bank:send:d"))
	if fourth.Header.AppHash != afterFirst || afterFirst == genesis {
		t.Fatalf("expected block 4 to commit the app hash after block 1 %x, got %x", afterFirst, fourth.Header.AppHash)
	}
	stale := fourth
	stale.Header.AppHash = genesis
	if height, _ := fixture.deliver(fixture.commit(stale)); height != 3 {
		t.Fatalf("expected the block naming a stale app hash to be refused, node is at height %d", height)
	}
	// The application's own app hash leaves the validator registry out; the header commits both.
	stateOne, err := runtime.StateByHeight(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	registryOne, err := runtime.StateRoot(ctx, 1, validator.RegistryNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if afterFirst != finality.HeaderAppHash(stateOne.AppHash, registryOne.Root) {
		t.Fatal("expected block 4 to commit the app hash and the validator registry root of height 1")
	}
	applicationOnly := fourth
	applicationOnly.Header.AppHash = stateOne.AppHash
	if height, _ := fixture.deliver(fixture.commit(applicationOnly)); height != 3 {
		t.Fatalf("expected the block naming only the application app hash to be refused, node is at height %d", height)
	}
	if height, _ := fixture.deliver(fixture.commit(fourth)); height != 4 {
		t.Fatalf("expected the block naming the executed app hash to be applied, node is at height %d", height)
	}
}

// Every validator announces each commit; a node missing the body asks one announcer, and the
// next only once that request had time to be answered.
func TestCommitServerAsksOnceForABodyPerRetry(t *testing.T) {
	var server commitServer
	if !server.shouldAsk(5) {
		t.Fatal("expected the first announcement of a missing body to ask")
	}
	for range 8 {
		if server.shouldAsk(5) {
			t.Fatal("expected the other announcers not to be asked again before the retry")
		}
	}
	if !server.shouldAsk(6) {
		t.Fatal("expected a new height to be asked at once")
	}
	server.asked = server.asked.Add(-commitBodyRetry)
	if !server.shouldAsk(6) {
		t.Fatal("expected an unanswered request to be retried")
	}
}

// sampleCommitMessage is a commit with every field of its kinds set: a block of transactions, its
// certificate, and a proof that carries the child that makes it final.
func sampleCommitMessage() commitMessage {
	header := types.Header{
		ChainID:              "vexo-test",
		Height:               5,
		TimeUnixNano:         1_700_000_000_000_000_005,
		PreviousBlockHash:    types.Hash{1},
		AppHash:              types.Hash{2},
		ValidatorSetHash:     types.Hash{3},
		NextValidatorSetHash: types.Hash{4},
		ConsensusHash:        types.Hash{5},
		TxRoot:               types.Hash{6},
		Round:                7,
		JustifyRound:         6,
		ValidatorSetHeight:   4,
		StateHeight:          3,
		Benched:              []types.Bench{{Validator: "bob", Until: 9}},
	}
	block := types.Block{Header: header, Txs: []types.Tx{types.Tx("bank:send:a"), types.Tx("bank:send:b")}}
	quorumCert := finality.QuorumCert{Height: 5, Round: 7, BlockHash: types.Hash{7}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("aggregate"), VotingPower: 2}
	child := header
	child.Height, child.Round, child.JustifyRound = 6, 8, 7
	childCert := finality.QuorumCert{Height: 6, Round: 8, BlockHash: types.Hash{8}, Signers: types.Bitmap("alice,bob"), Signature: types.AggregateSignature("child"), VotingPower: 2}
	proof := finality.NewProof(header, quorumCert).WithChild(child, childCert)
	proof.ValidatorSetHeight, proof.ValidatorSetHash = 4, types.Hash{9}
	return commitMessage{Block: block, QuorumCert: quorumCert, FinalityProof: proof}
}

func sampleCommitMessages() map[string]commitMessage {
	whole := sampleCommitMessage()
	bodiless := whole
	bodiless.Block.Txs = nil
	bodiless.Bodiless = true
	unproven := whole
	unproven.FinalityProof = finality.Proof{}
	linked := whole
	linked.Linkage = []types.Header{whole.FinalityProof.Header, whole.Block.Header}
	return map[string]commitMessage{"whole": whole, "bodiless": bodiless, "unproven": unproven, "linked": linked, "request": {Want: 42}}
}

func TestCommitMessageCodecRoundTrips(t *testing.T) {
	for name, message := range sampleCommitMessages() {
		t.Run(name, func(t *testing.T) {
			data := appendCommitMessage(nil, message)
			decoded, err := decodeCommitMessage(data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, message) {
				t.Fatalf("a message changed in a round trip\n got: %+v\nwant: %+v", decoded, message)
			}
			if !bytes.Equal(appendCommitMessage(nil, decoded), data) {
				t.Fatal("a decoded message must encode to the bytes it came from")
			}
		})
	}
}

// A request names a height and nothing else: a message that wants a height carries no block.
func TestCommitMessageThatWantsAHeightIsARequestOnly(t *testing.T) {
	message := sampleCommitMessage()
	message.Want = 42
	decoded, err := decodeCommitMessage(appendCommitMessage(nil, message))
	if err != nil {
		t.Fatal(err)
	}
	if want := (commitMessage{Want: 42}); !reflect.DeepEqual(decoded, want) {
		t.Fatalf("expected only the height wanted, got %+v", decoded)
	}
}

func TestCommitMessageDecodingIsStrict(t *testing.T) {
	for name, message := range sampleCommitMessages() {
		t.Run(name, func(t *testing.T) {
			data := appendCommitMessage(nil, message)
			for length := range len(data) {
				if _, err := decodeCommitMessage(data[:length]); !errors.Is(err, ErrInvalidCommitMessage) {
					t.Fatalf("expected the first %d of %d bytes not to decode, got %v", length, len(data), err)
				}
			}
			for _, extra := range [][]byte{{0}, {1}, bytes.Repeat([]byte{0xff}, 16)} {
				if _, err := decodeCommitMessage(append(append([]byte(nil), data...), extra...)); !errors.Is(err, ErrInvalidCommitMessage) {
					t.Fatalf("expected trailing bytes %x to be refused, got %v", extra, err)
				}
			}
		})
	}

	whole := appendCommitMessage(nil, sampleCommitMessage())
	notBoolean := append([]byte(nil), whole...)
	notBoolean[1] = 2
	noHeight := sampleCommitMessage()
	noHeight.Block.Header.Height = 0
	noCertificateHeight := sampleCommitMessage()
	noCertificateHeight.QuorumCert.Height = 0
	overBudget := sampleCommitMessage()
	overBudget.Block.Txs = []types.Tx{bytes.Repeat([]byte{1}, consensus.MaxBlockBytes/2), bytes.Repeat([]byte{2}, consensus.MaxBlockBytes/2)}
	tooLong := sampleCommitMessage()
	tooLong.Block.Header.ChainID = strings.Repeat("c", types.MaxIDBytes+1)
	overLinked := sampleCommitMessage()
	overLinked.Linkage = make([]types.Header, maxCommitLinkage+1)
	for index := range overLinked.Linkage {
		overLinked.Linkage[index] = overLinked.Block.Header
	}
	cases := map[string][]byte{
		"nothing":                            nil,
		"json":                               []byte(`{"block":{}}`),
		"an unknown kind":                    append([]byte{commitKindWant + 1}, whole[1:]...),
		"kind zero":                          append([]byte{0}, whole[1:]...),
		"a bodiless flag that is no boolean": notBoolean,
		"a request for no height":            {commitKindWant, 0},
		"a height that is not minimal":       {commitKindWant, 0x81, 0x00},
		"a height past 64 bits":              append([]byte{commitKindWant}, bytes.Repeat([]byte{0xff}, 11)...),
		"a block without a height":           appendCommitMessage(nil, noHeight),
		"a certificate without a height":     appendCommitMessage(nil, noCertificateHeight),
		"a block over the byte limit":        appendCommitMessage(nil, overBudget),
		"a chain id over the limit":          appendCommitMessage(nil, tooLong),
		"a linkage over the limit":           appendCommitMessage(nil, overLinked),
	}
	for name, data := range cases {
		if _, err := decodeCommitMessage(data); !errors.Is(err, ErrInvalidCommitMessage) {
			t.Fatalf("%s: expected an invalid commit message, got %v", name, err)
		}
	}
	atLimit := sampleCommitMessage()
	atLimit.Block.Txs = []types.Tx{bytes.Repeat([]byte{1}, consensus.MaxBlockBytes-types.TxOverheadBytes)}
	if _, err := decodeCommitMessage(appendCommitMessage(nil, atLimit)); err != nil {
		t.Fatalf("expected a block of the whole byte limit to be accepted, got %v", err)
	}
	extreme := sampleCommitMessage()
	extreme.Want, extreme.Block, extreme.QuorumCert = math.MaxUint64, types.Block{}, finality.QuorumCert{}
	if decoded, err := decodeCommitMessage(appendCommitMessage(nil, extreme)); err != nil || decoded.Want != math.MaxUint64 {
		t.Fatalf("expected the largest height to be wanted, got %+v %v", decoded, err)
	}
}

// FuzzDecodeCommitMessage checks that no input makes the decoder panic, and that what decodes has
// one encoding: the bytes it came from.
func FuzzDecodeCommitMessage(f *testing.F) {
	for _, message := range sampleCommitMessages() {
		data := appendCommitMessage(nil, message)
		f.Add(data)
		f.Add(data[:len(data)/2])
	}
	f.Add([]byte{})
	f.Add([]byte{commitKindWant, 0})
	f.Add([]byte(`{"block":{}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		message, err := decodeCommitMessage(data)
		if err != nil {
			if !errors.Is(err, ErrInvalidCommitMessage) {
				t.Fatalf("a commit message was refused with an error that is not ErrInvalidCommitMessage: %v", err)
			}
			return
		}
		if message.Want == 0 && (message.Block.Header.Height == 0 || message.QuorumCert.Height == 0) {
			t.Fatalf("decoded a commit without a block height or a certificate height: %+v", message)
		}
		if message.Want != 0 && !reflect.DeepEqual(message, commitMessage{Want: message.Want}) {
			t.Fatalf("decoded a request that carries more than a height: %+v", message)
		}
		if encoded := appendCommitMessage(nil, message); !bytes.Equal(encoded, data) {
			t.Fatalf("a message decoded from %x encodes to %x", data, encoded)
		}
	})
}

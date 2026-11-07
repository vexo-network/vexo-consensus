package node

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
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
	cfg := DefaultConfig("vexo-test", t.TempDir())
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
	validatorSetHeight := finality.FirstValidatorSetHeight
	if height > 2 {
		validatorSetHeight = height - 1
	}
	block := types.Block{
		Header: types.Header{
			ChainID:            "vexo-test",
			Height:             height,
			Round:              round,
			JustifyRound:       round - 1,
			PreviousBlockHash:  parent,
			ValidatorSetHeight: validatorSetHeight,
			ValidatorSetHash:   fixture.validatorSetHash,
		},
		Txs: fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt("vexo-test", height)),
	}
	// The header commits the app hash of the state its validator set was taken from.
	runtime, err := fixture.node.Runtime()
	if err != nil {
		fixture.t.Fatal(err)
	}
	appHash, err := runtime.HeaderAppHashAt(context.Background(), validatorSetHeight-1)
	if err != nil {
		fixture.t.Fatal(err)
	}
	block.Header.AppHash = appHash
	block.Header.TxRoot = consensus.TxRoot(block.Txs)
	if len(block.Txs) > 0 {
		block = dataavailability.AttachCommitment(block)
	}
	return block
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
	childQC := fixture.quorumCert(child.Header.Height, childView, finality.HeaderHash(child.Header))
	proof := finality.NewProof(block.Header, quorumCert).WithChild(child.Header, childQC)
	return commitMessage{Block: block, QuorumCert: quorumCert, FinalityProof: proof}
}

// deliver gossips message from a peer and reports the height the node reached
// and the peer's score afterwards.
func (fixture *commitFixture) deliver(message commitMessage) (types.Height, int64) {
	fixture.t.Helper()
	data, err := encodeCommitMessage(message.Block, message.QuorumCert, message.FinalityProof)
	if err != nil {
		fixture.t.Fatal(err)
	}
	fixture.node.acceptCommitMessage(context.Background(), "bob", data)
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
		// Block 2 directly follows the view of its parent, so it proves block 1 committed and
		// fixes the set of registry height 2 for block 3. A quorum of an older set certifying
		// a block 3 that keeps registry height 1 does not make it a block of this chain.
		third := fixture.block(3, consensus.HashBlock(second), types.Tx("bank:send:c"))
		if third.Header.ValidatorSetHeight != 2 {
			t.Fatalf("expected block 3 to name registry height 2, got %d", third.Header.ValidatorSetHeight)
		}
		stale := third
		stale.Header.ValidatorSetHeight = 1
		if height, _ := fixture.deliver(fixture.commit(stale)); height != 2 {
			t.Fatalf("expected the block naming a stale validator set to be refused, node is at height %d", height)
		}
		if height, _ := fixture.deliver(fixture.commit(third)); height != 3 {
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
	afterFirst, err := runtime.HeaderAppHashAt(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	third := fixture.block(3, consensus.HashBlock(second), types.Tx("bank:send:c"))
	if third.Header.AppHash != afterFirst || afterFirst == genesis {
		t.Fatalf("expected block 3 to commit the app hash after block 1 %x, got %x", afterFirst, third.Header.AppHash)
	}
	stale := third
	stale.Header.AppHash = genesis
	if height, _ := fixture.deliver(fixture.commit(stale)); height != 2 {
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
		t.Fatal("expected block 3 to commit the app hash and the validator registry root of height 1")
	}
	applicationOnly := third
	applicationOnly.Header.AppHash = stateOne.AppHash
	if height, _ := fixture.deliver(fixture.commit(applicationOnly)); height != 2 {
		t.Fatalf("expected the block naming only the application app hash to be refused, node is at height %d", height)
	}
	if height, _ := fixture.deliver(fixture.commit(third)); height != 3 {
		t.Fatalf("expected the block naming the executed app hash to be applied, node is at height %d", height)
	}
}

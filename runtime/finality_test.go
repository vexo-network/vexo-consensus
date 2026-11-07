package runtime

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// twoChainFixture is a runtime that executed block 1 under the validators a, b and c. Each has power
// 1, so any two of them are a quorum. It holds the keys needed to certify blocks for them.
type twoChainFixture struct {
	runtime *Runtime
	signers map[types.ValidatorID]vexocrypto.DeterministicSigner
	block   types.Block
}

func newTwoChainFixture(t *testing.T) *twoChainFixture {
	t.Helper()
	signers := make(map[types.ValidatorID]vexocrypto.DeterministicSigner)
	validators := make([]validator.Validator, 0, 3)
	for _, id := range []types.ValidatorID{"a", "b", "c"} {
		signer, err := vexocrypto.NewDeterministicSigner([]byte("signer-" + string(id)))
		if err != nil {
			t.Fatal(err)
		}
		signers[id] = signer
		validators = append(validators, validator.Validator{ID: id, Address: types.Address(id), VotingPower: 1, Stake: 1, PublicKey: signer.PublicKey()})
	}
	runtime := newFinalityRuntime(t, validators)
	validatorSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	block := types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHeight: finality.FirstValidatorSetHeight, ValidatorSetHash: validatorSet.Hash()}}
	if _, err := runtime.ExecuteBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}
	if consensus.HashBlock(block) != finality.HeaderHash(block.Header) {
		t.Fatal("a block without transactions must be certified by the hash of its header")
	}
	return &twoChainFixture{runtime: runtime, signers: signers, block: block}
}

// childOf returns the header of a block that extends parent by one height, proposed in the view
// after the parent's with the parent's certificate, under the same chain and validator set.
func (fixture *twoChainFixture) childOf(parent types.Header) types.Header {
	return types.Header{
		ChainID:            parent.ChainID,
		Height:             parent.Height + 1,
		Round:              parent.Round + 1,
		JustifyRound:       parent.Round,
		PreviousBlockHash:  finality.HeaderHash(parent),
		ValidatorSetHeight: finality.ChildValidatorSetHeight(parent),
		ValidatorSetHash:   parent.ValidatorSetHash,
	}
}

// cert returns the certificate of the block with the given header, formed in view round and signed
// by the named validators.
func (fixture *twoChainFixture) cert(t *testing.T, header types.Header, round types.Round, ids ...types.ValidatorID) finality.QuorumCert {
	t.Helper()
	cert := finality.QuorumCert{
		Height:      header.Height,
		Round:       round,
		BlockHash:   finality.HeaderHash(header),
		Signers:     finality.EncodeSigners(ids),
		VotingPower: types.VotingPower(len(ids)),
	}
	keys := make([]vexocrypto.DeterministicSigner, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, fixture.signers[id])
	}
	cert.Signature = signRuntimeCert(t, cert, keys...)
	return cert
}

// proof returns a valid proof that block 1 is final: QC(B) from a and b in view 0 and QC(C) of its
// child from b and c in view 1.
func (fixture *twoChainFixture) proof(t *testing.T) finality.Proof {
	t.Helper()
	child := fixture.childOf(fixture.block.Header)
	return finality.NewProof(fixture.block.Header, fixture.cert(t, fixture.block.Header, 0, "a", "b")).
		WithChild(child, fixture.cert(t, child, 1, "b", "c"))
}

func TestRuntimeBuildsAndVerifiesStoredFinalityProof(t *testing.T) {
	ctx := context.Background()
	fixture := newTwoChainFixture(t)
	qc := fixture.cert(t, fixture.block.Header, 0, "a", "b")

	skeleton, err := fixture.runtime.FinalityProof(ctx, 1, qc)
	if err != nil {
		t.Fatal(err)
	}
	if skeleton.Header != fixture.block.Header || skeleton.BlockHash != qc.BlockHash || skeleton.HasCommitProof() {
		t.Fatalf("expected the proof skeleton of the stored block, got %+v", skeleton)
	}
	if err := fixture.runtime.VerifyBlockCert(ctx, 1, qc); err != nil {
		t.Fatalf("a quorum certificate of the stored block must verify: %v", err)
	}
	// QC(B) shows that the block is certified, never that it is final.
	if err := fixture.runtime.VerifyFinalityProof(ctx, skeleton); !errors.Is(err, finality.ErrCommitChainTooShort) {
		t.Fatalf("a proof without the child of its block must not verify, got %v", err)
	}

	child := fixture.childOf(fixture.block.Header)
	proof := skeleton.WithChild(child, fixture.cert(t, child, 1, "b", "c"))
	if !proof.HasCommitProof() {
		t.Fatal("expected the proof to carry the child link")
	}
	if err := fixture.runtime.VerifyFinalityProof(ctx, proof); err != nil {
		t.Fatalf("a 2-chain proof of the stored block must verify: %v", err)
	}
}

// The 2-chain commit rule is what makes a block final, and the runtime must hold a proof to all of
// it: certificates from a quorum of the validator set that is in effect, a child that extends the
// block, and consecutive views.
func TestRuntimeVerifyFinalityProofRejectsAnUnsoundCommitChain(t *testing.T) {
	fixture := newTwoChainFixture(t)
	header := fixture.block.Header
	child := fixture.childOf(header)
	blockCert := fixture.cert(t, header, 0, "a", "b")
	outsiderB, err := vexocrypto.NewDeterministicSigner([]byte("outsider-b"))
	if err != nil {
		t.Fatal(err)
	}
	outsiderC, err := vexocrypto.NewDeterministicSigner([]byte("outsider-c"))
	if err != nil {
		t.Fatal(err)
	}
	forgedChildCert := fixture.cert(t, child, 1, "b", "c")
	forgedChildCert.Signature = signRuntimeCert(t, forgedChildCert, outsiderB, outsiderC)
	detachedChild := child
	detachedChild.PreviousBlockHash = types.Hash{9}
	skippedChild := fixture.childOf(header)
	skippedChild.Height = header.Height + 2
	twoLinks := fixture.proof(t)
	twoLinks.CommitChain = append(twoLinks.CommitChain, twoLinks.CommitChain[0])

	for _, test := range []struct {
		name  string
		proof finality.Proof
		want  error
	}{
		{
			name:  "child certified in the same view as the block",
			proof: finality.NewProof(header, blockCert).WithChild(child, fixture.cert(t, child, 0, "b", "c")),
			want:  finality.ErrViewsNotConsecutive,
		},
		{
			name:  "child certified after a skipped view",
			proof: finality.NewProof(header, blockCert).WithChild(child, fixture.cert(t, child, 2, "b", "c")),
			want:  finality.ErrViewsNotConsecutive,
		},
		{
			name:  "child certified below quorum",
			proof: finality.NewProof(header, blockCert).WithChild(child, fixture.cert(t, child, 1, "c")),
			want:  finality.ErrInsufficientQuorum,
		},
		{
			name:  "block certified below quorum",
			proof: finality.NewProof(header, fixture.cert(t, header, 0, "a")).WithChild(child, fixture.cert(t, child, 1, "b", "c")),
			want:  finality.ErrInsufficientQuorum,
		},
		{
			name:  "child certificate signed by keys outside the validator set",
			proof: finality.NewProof(header, blockCert).WithChild(child, forgedChildCert),
			want:  finality.ErrMissingQCSignature,
		},
		{
			name:  "child that does not extend the block",
			proof: finality.NewProof(header, blockCert).WithChild(detachedChild, fixture.cert(t, detachedChild, 1, "b", "c")),
			want:  finality.ErrCommitChainBroken,
		},
		{
			name:  "child that skips a height",
			proof: finality.NewProof(header, blockCert).WithChild(skippedChild, fixture.cert(t, skippedChild, 1, "b", "c")),
			want:  finality.ErrCommitChainBroken,
		},
		{
			name:  "more than one child link",
			proof: twoLinks,
			want:  finality.ErrCommitChainTooLong,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := fixture.runtime.VerifyFinalityProof(context.Background(), test.proof); !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
	if err := fixture.runtime.VerifyFinalityProof(context.Background(), fixture.proof(t)); err != nil {
		t.Fatalf("the unmodified proof must verify: %v", err)
	}
}

// When validators equivocate, two blocks of one height can both gather a quorum. The proof of the
// block this runtime did not execute is valid on its own, but it proves nothing about the executed
// block, and the runtime must not vouch for it.
func TestRuntimeVerifyFinalityProofRejectsAProofOfAnotherBlock(t *testing.T) {
	ctx := context.Background()
	fixture := newTwoChainFixture(t)
	other := fixture.block.Header
	other.AppHash = types.Hash{0xEE}
	child := fixture.childOf(other)
	forged := finality.NewProof(other, fixture.cert(t, other, 0, "a", "b")).WithChild(child, fixture.cert(t, child, 1, "b", "c"))

	verifier := fixture.runtime.NewFinalityVerifier()
	if err := verifier.VerifyFinalityProofWithContext(ctx, forged); err != nil {
		t.Fatalf("the proof of the other block is valid for the validator set: %v", err)
	}
	if err := fixture.runtime.VerifyFinalityProof(ctx, forged); !errors.Is(err, ErrFinalityProofBlockMismatch) {
		t.Fatalf("expected block mismatch, got %v", err)
	}
	if err := fixture.runtime.VerifyBlockCert(ctx, 1, forged.QuorumCert); !errors.Is(err, ErrFinalityProofBlockMismatch) {
		t.Fatalf("expected block mismatch, got %v", err)
	}
	// The block the runtime executed is still provable.
	if err := fixture.runtime.VerifyFinalityProof(ctx, fixture.proof(t)); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeVerifyFinalityProofRejectsABlockItNeverExecuted(t *testing.T) {
	ctx := context.Background()
	fixture := newTwoChainFixture(t)
	next := fixture.childOf(fixture.block.Header)
	nextChild := fixture.childOf(next)
	proof := finality.NewProof(next, fixture.cert(t, next, 1, "a", "b")).WithChild(nextChild, fixture.cert(t, nextChild, 2, "b", "c"))
	if err := fixture.runtime.VerifyFinalityProof(ctx, proof); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found for a height the runtime has not executed, got %v", err)
	}
	if err := fixture.runtime.VerifyBlockCert(ctx, next.Height, proof.QuorumCert); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found for a height the runtime has not executed, got %v", err)
	}
}

func TestRuntimeVerifyBlockCertRejectsAnUnsoundCertificate(t *testing.T) {
	ctx := context.Background()
	fixture := newTwoChainFixture(t)
	header := fixture.block.Header
	valid := fixture.cert(t, header, 0, "a", "b")
	if err := fixture.runtime.VerifyBlockCert(ctx, 1, valid); err != nil {
		t.Fatal(err)
	}

	wrongHeight := valid
	wrongHeight.Height = 2
	unsigned := valid
	unsigned.Signature = nil
	outsider, err := vexocrypto.NewDeterministicSigner([]byte("outsider-a"))
	if err != nil {
		t.Fatal(err)
	}
	forged := valid
	forged.Signature = signRuntimeCert(t, forged, outsider, fixture.signers["b"])
	otherView := valid
	otherView.Round = 1
	otherBlock := header
	otherBlock.AppHash = types.Hash{0xEE}
	duplicated := fixture.cert(t, header, 0, "a", "a")

	for _, test := range []struct {
		name string
		cert finality.QuorumCert
		want error
	}{
		{name: "another height", cert: wrongHeight, want: ErrFinalityProofHeightMismatch},
		{name: "another block at this height", cert: fixture.cert(t, otherBlock, 0, "a", "b"), want: ErrFinalityProofBlockMismatch},
		{name: "no signature", cert: unsigned, want: finality.ErrMissingQCSignature},
		{name: "certificate of another view than the block's", cert: otherView, want: finality.ErrViewsNotConsecutive},
		{name: "signature of a key outside the validator set", cert: forged, want: finality.ErrMissingQCSignature},
		{name: "below quorum", cert: fixture.cert(t, header, 0, "a"), want: finality.ErrInsufficientQuorum},
		{name: "one validator counted twice", cert: duplicated, want: finality.ErrDuplicateSigner},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := fixture.runtime.VerifyBlockCert(ctx, 1, test.cert); !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestRuntimeExecuteCertifiedBlockPersistsQuorumCert(t *testing.T) {
	runtime := newFinalityRuntime(t, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
	})
	block := types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}}
	blockHash := consensus.HashBlock(block)
	qc := finality.QuorumCert{
		Height:      1,
		Round:       2,
		BlockHash:   blockHash,
		Signers:     finality.EncodeSigners([]types.ValidatorID{"a"}),
		VotingPower: 1,
	}
	if _, err := runtime.ExecuteCertifiedBlock(context.Background(), block, qc); err != nil {
		t.Fatal(err)
	}
	record, err := runtime.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if record.QuorumCert == nil || record.QuorumCert.Height != 1 || record.QuorumCert.Round != 2 || record.QuorumCert.BlockHash != blockHash {
		t.Fatalf("expected persisted block quorum cert, got %+v", record.QuorumCert)
	}
}

func TestRuntimeFinalityProofRejectsWrongHeight(t *testing.T) {
	runtime := newFinalityRuntime(t, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
	})
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}}); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.FinalityProof(context.Background(), 1, finality.QuorumCert{Height: 2, BlockHash: types.Hash{1}})
	if !errors.Is(err, ErrFinalityProofHeightMismatch) {
		t.Fatalf("expected height mismatch, got %v", err)
	}
}

func TestRuntimeFinalityProofRejectsWrongBlockHash(t *testing.T) {
	runtime := newFinalityRuntime(t, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
	})
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}}); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.FinalityProof(context.Background(), 1, finality.QuorumCert{Height: 1, BlockHash: types.Hash{9}})
	if !errors.Is(err, ErrFinalityProofBlockMismatch) {
		t.Fatalf("expected block mismatch, got %v", err)
	}
}

func TestRuntimeFinalityProofRequiresStore(t *testing.T) {
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEphemeral(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.FinalityProof(context.Background(), 1, finality.QuorumCert{Height: 1})
	if !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found without store, got %v", err)
	}
	if err := runtime.VerifyBlockCert(context.Background(), 1, finality.QuorumCert{Height: 1}); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found without store, got %v", err)
	}
	proof := finality.NewProof(types.Header{ChainID: "vexo-test", Height: 1}, finality.QuorumCert{Height: 1})
	if err := runtime.VerifyFinalityProof(context.Background(), proof); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected block not found without store, got %v", err)
	}
}

func newFinalityRuntime(t *testing.T, validators []validator.Validator) *Runtime {
	t.Helper()
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{&runtimeModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewWithStore(config.Default("vexo-test"), application, validators, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return runtime
}

// signRuntimeCert signs the vote of cert with every signer, in the consensus vote domain, and
// aggregates the signatures in the order of the signers.
func signRuntimeCert(t *testing.T, cert finality.QuorumCert, signers ...vexocrypto.DeterministicSigner) types.AggregateSignature {
	t.Helper()
	signatures := make([]types.Signature, 0, len(signers))
	for _, signer := range signers {
		domainSigner, err := vexocrypto.NewDomainSigner(signer, vexocrypto.DomainConsensusVote)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := domainSigner.Sign(cert.SignBytes())
		if err != nil {
			t.Fatal(err)
		}
		signatures = append(signatures, signature)
	}
	aggregate, err := (vexocrypto.DeterministicAggregateSigner{}).Aggregate(signatures)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}

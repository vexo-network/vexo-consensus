package runtime

import (
	"context"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRuntimeBuildsAndVerifiesStoredFinalityProof(t *testing.T) {
	firstSigner, err := vexocrypto.NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	secondSigner, err := vexocrypto.NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	thirdSigner, err := vexocrypto.NewDeterministicSigner([]byte("third"))
	if err != nil {
		t.Fatal(err)
	}
	validators := []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1, PublicKey: firstSigner.PublicKey()},
		{ID: "b", Address: "b", VotingPower: 1, Stake: 1, PublicKey: secondSigner.PublicKey()},
		{ID: "c", Address: "c", VotingPower: 1, Stake: 1, PublicKey: thirdSigner.PublicKey()},
	}
	runtime := newFinalityRuntime(t, validators)
	validatorSet, err := runtime.Validators.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	block := types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           1,
		ValidatorSetHash: validatorSet.Hash(),
	}}
	if _, err := runtime.ExecuteBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}

	qc := finality.QuorumCert{
		Height:      1,
		Round:       0,
		BlockHash:   finality.HeaderHash(block.Header),
		Signers:     finality.EncodeSigners([]types.ValidatorID{"a", "b"}),
		VotingPower: 2,
	}
	proof := finality.NewProof(block.Header, qc)
	qc.Signature = signRuntimeProof(t, proof, firstSigner, secondSigner)

	proof, err = runtime.FinalityProof(context.Background(), 1, qc)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Header.Height != 1 || proof.QuorumCert.BlockHash != qc.BlockHash {
		t.Fatalf("unexpected proof: %+v", proof)
	}
	if err := runtime.VerifyFinalityProof(context.Background(), 1, qc); err != nil {
		t.Fatal(err)
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
	runtime, err := New(config.Default("vexo-test"), application, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.FinalityProof(context.Background(), 1, finality.QuorumCert{Height: 1})
	if !errors.Is(err, store.ErrBlockNotFound) {
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

func signRuntimeProof(t *testing.T, proof finality.Proof, signers ...vexocrypto.DeterministicSigner) types.AggregateSignature {
	t.Helper()
	signatures := make([]types.Signature, 0, len(signers))
	for _, signer := range signers {
		signature, err := signer.Sign(proof.SignBytes())
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

package finality

import (
	"bytes"
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func abcSet(t *testing.T) validator.Set {
	t.Helper()
	return testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
}

func TestVerifierAcceptsTwoChainFinalityProof(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	if err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
	if !proof.HasCommitProof() {
		t.Fatal("a complete proof must report its commit proof")
	}
}

func TestVerifierRejectsProofWithoutChild(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.CommitChain = nil

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrCommitChainTooShort) {
		t.Fatalf("a certified block without its child is not final, got %v", err)
	}
	if proof.HasCommitProof() {
		t.Fatal("a proof without a child must not report a commit proof")
	}
}

func TestVerifierRejectsMoreThanOneChildLink(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.CommitChain = append(proof.CommitChain, proof.CommitChain[0])

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrCommitChainTooLong) {
		t.Fatalf("expected exactly one child link, got %v", err)
	}
}

func TestVerifierRejectsNilSignatureVerifier(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	// The signature is garbage: a verifier that skips signatures would accept this proof.
	proof.QuorumCert.Signature = types.AggregateSignature("forged")
	proof.CommitChain[0].QuorumCert.Signature = types.AggregateSignature("forged")

	err := NewVerifier(set, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrMissingSignatureVerifier) {
		t.Fatalf("expected a nil verifier to fail closed, got %v", err)
	}
	err = NewVerifier(set, nil).VerifyBlockCertWithContext(context.Background(), proof.Header, proof.QuorumCert)
	if !errors.Is(err, ErrMissingSignatureVerifier) {
		t.Fatalf("expected a nil verifier to fail closed for a block certificate, got %v", err)
	}
	registry := staticRegistry{set: set}
	err = NewRegistryVerifier(registry, nil).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrMissingSignatureVerifier) {
		t.Fatalf("expected a nil registry verifier to fail closed, got %v", err)
	}
	if err := (Verifier{}).VerifyFinalityProof(proof); !errors.Is(err, ErrMissingSignatureVerifier) {
		t.Fatalf("expected the zero verifier to fail closed, got %v", err)
	}
	if err := (Verifier{signatures: acceptSignatureVerifier{}}).VerifyFinalityProof(proof); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected the verifier without a validator set to fail closed, got %v", err)
	}
}

func TestVerifierRejectsSwappedHeader(t *testing.T) {
	set := abcSet(t)
	// acceptSignatureVerifier models an attacker who holds valid certificates: only the
	// header-to-hash binding can stop the swap.
	verifier := NewVerifier(set, acceptSignatureVerifier{})

	forgedHeader := validProof(set, []types.ValidatorID{"a", "b"})
	forgedHeader.Header.AppHash = types.Hash{9}
	if err := verifier.VerifyFinalityProof(forgedHeader); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected a swapped header to be rejected, got %v", err)
	}

	forgedChild := validProof(set, []types.ValidatorID{"a", "b"})
	forgedChild.CommitChain[0].Header.AppHash = types.Hash{9}
	if err := verifier.VerifyFinalityProof(forgedChild); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected a swapped child header to be rejected, got %v", err)
	}

	// Rewriting the proof's own hash to match the forged header still leaves the certificate
	// certifying the original block.
	rehashed := validProof(set, []types.ValidatorID{"a", "b"})
	rehashed.Header.AppHash = types.Hash{9}
	rehashed.BlockHash = HeaderHash(rehashed.Header)
	if err := verifier.VerifyFinalityProof(rehashed); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected the certificate to stay bound to the original block, got %v", err)
	}

	// An explicit hash that is not the hash of the header is never accepted.
	arbitrary := validProof(set, []types.ValidatorID{"a", "b"})
	arbitrary.BlockHash = types.Hash{7}
	arbitrary.QuorumCert.BlockHash = arbitrary.BlockHash
	if err := verifier.VerifyFinalityProof(arbitrary); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected a hash that does not commit to the header to be rejected, got %v", err)
	}
}

func TestVerifierRejectsNonConsecutiveViews(t *testing.T) {
	set := abcSet(t)
	verifier := NewVerifier(set, acceptSignatureVerifier{})
	signers := []types.ValidatorID{"a", "b"}

	for name, childRound := range map[string]types.Round{"gap": 7, "same view": 5, "earlier view": 4, "much later": 6 + 1<<40} {
		proof := proofAt(set, 1, types.Hash{}, 5, signers)
		proof.CommitChain[0].QuorumCert.Round = childRound
		if err := verifier.VerifyFinalityProof(proof); !errors.Is(err, ErrViewsNotConsecutive) {
			t.Fatalf("%s: expected a non-consecutive view chain to be rejected, got %v", name, err)
		}
	}

	proof := proofAt(set, 1, types.Hash{}, 5, signers)
	proof.CommitChain[0].QuorumCert.Round = 6
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatalf("consecutive views must verify, got %v", err)
	}

	overflow := proofAt(set, 1, types.Hash{}, ^types.Round(0), signers)
	overflow.CommitChain[0].QuorumCert.Round = 0
	if err := verifier.VerifyFinalityProof(overflow); !errors.Is(err, ErrViewsNotConsecutive) {
		t.Fatalf("expected the view overflow to be rejected, got %v", err)
	}
}

func TestVerifierRejectsChildThatDoesNotExtendTheBlock(t *testing.T) {
	set := abcSet(t)
	verifier := NewVerifier(set, acceptSignatureVerifier{})
	signers := []types.ValidatorID{"a", "b"}

	cases := map[string]struct {
		mutate func(link *CommitLink)
		want   error
	}{
		"fork": {
			mutate: func(link *CommitLink) { link.Header.PreviousBlockHash = types.Hash{5} },
			want:   ErrCommitChainBroken,
		},
		"height gap": {
			mutate: func(link *CommitLink) { link.Header.Height += 1 },
			want:   ErrCommitChainBroken,
		},
		"same height": {
			mutate: func(link *CommitLink) { link.Header.Height -= 1 },
			want:   ErrCommitChainBroken,
		},
		"other chain": {
			mutate: func(link *CommitLink) { link.Header.ChainID = "other-chain" },
			want:   ErrCommitChainBroken,
		},
		"other validator set": {
			mutate: func(link *CommitLink) { link.Header.ValidatorSetHash = types.Hash{6} },
			want:   ErrValidatorSetMismatch,
		},
	}
	for name, testCase := range cases {
		proof := proofAt(set, 1, types.Hash{}, 0, signers)
		testCase.mutate(&proof.CommitChain[0])
		reseatChild(&proof)
		if err := verifier.VerifyFinalityProof(proof); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: expected %v, got %v", name, testCase.want, err)
		}
	}
}

func TestVerifierRejectsChildCertificateForAnotherBlock(t *testing.T) {
	set := abcSet(t)
	verifier := NewVerifier(set, acceptSignatureVerifier{})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	// The child link must carry the child's own certificate, not a second copy of QC(B).
	proof.CommitChain[0].QuorumCert = proof.QuorumCert
	if err := verifier.VerifyFinalityProof(proof); !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected QC(B) in the child link to be rejected, got %v", err)
	}

	proof = validProof(set, []types.ValidatorID{"a", "b"})
	proof.CommitChain[0].QuorumCert.BlockHash = types.Hash{8}
	if err := verifier.VerifyFinalityProof(proof); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected a child certificate for another block to be rejected, got %v", err)
	}
}

func TestVerifierChecksBothCertificates(t *testing.T) {
	set := abcSet(t)
	signers := []types.ValidatorID{"a", "b"}

	proof := validProof(set, signers)
	both := messageVerifier{accepted: [][]byte{voteMessage(t, proof.QuorumCert), voteMessage(t, proof.CommitChain[0].QuorumCert)}}
	if err := NewVerifier(set, both).VerifyFinalityProof(proof); err != nil {
		t.Fatalf("expected signatures over both certificates to verify, got %v", err)
	}

	onlyBlock := messageVerifier{accepted: [][]byte{voteMessage(t, proof.QuorumCert)}}
	if err := NewVerifier(set, onlyBlock).VerifyFinalityProof(proof); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected an unsigned child certificate to be rejected, got %v", err)
	}
	onlyChild := messageVerifier{accepted: [][]byte{voteMessage(t, proof.CommitChain[0].QuorumCert)}}
	if err := NewVerifier(set, onlyChild).VerifyFinalityProof(proof); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected an unsigned block certificate to be rejected, got %v", err)
	}

	weakChild := validProof(set, signers)
	weakChild.CommitChain[0].QuorumCert = certFor(set, weakChild.CommitChain[0].Header, 1, []types.ValidatorID{"a"})
	if err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(weakChild); !errors.Is(err, ErrInsufficientQuorum) {
		t.Fatalf("expected a child certificate below quorum to be rejected, got %v", err)
	}

	unsignedChild := validProof(set, signers)
	unsignedChild.CommitChain[0].QuorumCert.Signature = nil
	if err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(unsignedChild); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected a child certificate without a signature to be rejected, got %v", err)
	}
}

func TestVerifierRejectsValidatorSetMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.ValidatorSetHash = types.Hash{9}

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected validator set mismatch, got %v", err)
	}
}

func TestVerifierRejectsBlockHashMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.QuorumCert.BlockHash = types.Hash{9}

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected block hash mismatch, got %v", err)
	}
}

func TestVerifierRejectsHeightMismatch(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.QuorumCert.Height = proof.Header.Height + 1

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected height mismatch, got %v", err)
	}
}

func TestVerifierRejectsMissingValidatorSetHeight(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.ValidatorSetHeight = 0

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected height mismatch, got %v", err)
	}
}

func TestVerifierAcceptsHistoricalValidatorSetHeight(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	// Block 10 keeps the set of registry height 7 when no block since proved a commit.
	header := types.Header{ChainID: "vexo-test", Height: 10, Round: 4, JustifyRound: 1, ValidatorSetHeight: 7, StateHeight: 6, ValidatorSetHash: set.Hash(), NextValidatorSetHash: set.Hash()}
	child := types.Header{
		ChainID:              header.ChainID,
		Height:               11,
		Round:                5,
		JustifyRound:         4,
		PreviousBlockHash:    HeaderHash(header),
		ValidatorSetHeight:   ChildValidatorSetHeight(header),
		StateHeight:          ChildStateHeight(header),
		ValidatorSetHash:     set.Hash(),
		NextValidatorSetHash: set.Hash(),
	}
	if child.ValidatorSetHeight != 7 || child.StateHeight != 6 {
		t.Fatalf("expected the child of a block after a view change to keep registry height 7 and state 6, got %d and %d", child.ValidatorSetHeight, child.StateHeight)
	}
	proof := NewProof(header, certFor(set, header, 4, []types.ValidatorID{"a"})).WithChild(child, certFor(set, child, 5, []types.ValidatorID{"a"}))
	if err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof); err != nil {
		t.Fatalf("expected historical validator set height to verify, got %v", err)
	}
}

func TestVerifierRejectsFutureValidatorSetHeight(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.ValidatorSetHeight = proof.Header.Height + 1

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected future validator set height rejection, got %v", err)
	}
}

func TestVerifyBlockCertWithContext(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	verifier := NewVerifier(set, acceptSignatureVerifier{})
	ctx := context.Background()

	// A certified block needs no child: this is the QC-only check, not a finality proof.
	if err := verifier.VerifyBlockCertWithContext(ctx, proof.Header, proof.QuorumCert); err != nil {
		t.Fatal(err)
	}

	forged := proof.Header
	forged.AppHash = types.Hash{9}
	if err := verifier.VerifyBlockCertWithContext(ctx, forged, proof.QuorumCert); !errors.Is(err, ErrBlockHashMismatch) {
		t.Fatalf("expected a certificate to stay bound to its header, got %v", err)
	}

	otherSet := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 5, PublicKey: []byte("a-pub")}})
	if err := NewVerifier(otherSet, acceptSignatureVerifier{}).VerifyBlockCertWithContext(ctx, proof.Header, proof.QuorumCert); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected a certificate of another validator set to be rejected, got %v", err)
	}

	weak := certFor(set, proof.Header, 0, []types.ValidatorID{"a"})
	if err := verifier.VerifyBlockCertWithContext(ctx, proof.Header, weak); !errors.Is(err, ErrInsufficientQuorum) {
		t.Fatalf("expected a certificate below quorum to be rejected, got %v", err)
	}

	if err := NewVerifier(set, rejectSignatureVerifier{}).VerifyBlockCertWithContext(ctx, proof.Header, proof.QuorumCert); !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected an invalid aggregate to be rejected, got %v", err)
	}
}

func TestRegistryVerifierLoadsValidatorSetAtProofHeight(t *testing.T) {
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	if err := NewRegistryVerifier(registry, acceptSignatureVerifier{}).VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryVerifierRejectsWrongHeightValidatorSet(t *testing.T) {
	correctSet := abcSet(t)
	wrongSet := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 10, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})
	proof := validProof(correctSet, []types.ValidatorID{"a", "b"})

	err := NewRegistryVerifier(staticRegistry{set: wrongSet}, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected validator set mismatch, got %v", err)
	}
}

// A prover must not choose the validator set that verifies its proof: the set is the one the
// certified header names, the proof's own fields must repeat it, and a header cannot name a set
// its height cannot have.
func TestRegistryVerifierUsesTheValidatorSetTheHeaderNames(t *testing.T) {
	staleSet := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("stale-a")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("stale-b")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("stale-c")},
	})
	currentSet := abcSet(t)
	registry := heightRegistry{sets: map[types.Height]validator.Set{7: staleSet, 9: currentSet, 10: currentSet}}
	verifier := NewRegistryVerifier(registry, acceptSignatureVerifier{})

	honest := proofAt(currentSet, 10, types.Hash{}, 0, []types.ValidatorID{"a", "b"})
	if honest.Header.ValidatorSetHeight != 9 {
		t.Fatalf("expected the block at height 10 to name registry height 9, got %d", honest.Header.ValidatorSetHeight)
	}
	if err := verifier.VerifyFinalityProof(honest); err != nil {
		t.Fatal(err)
	}

	claimed := proofAt(currentSet, 10, types.Hash{}, 0, []types.ValidatorID{"a", "b"})
	claimed.ValidatorSetHeight = 7
	if err := verifier.VerifyFinalityProof(claimed); !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected a proof naming another set than its header to be rejected, got %v", err)
	}

	// The stale set certifies a header that names the current set's height: its hash differs.
	staleSigned := proofAt(staleSet, 10, types.Hash{}, 0, []types.ValidatorID{"a", "b"})
	if err := verifier.VerifyFinalityProof(staleSigned); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected the proof signed by a stale validator set to be rejected, got %v", err)
	}

	future := proofAt(currentSet, 10, types.Hash{}, 0, []types.ValidatorID{"a", "b"})
	future.Header.ValidatorSetHeight = 10
	future.ValidatorSetHeight = 10
	future.BlockHash = HeaderHash(future.Header)
	future.QuorumCert.BlockHash = future.BlockHash
	if err := verifier.VerifyFinalityProof(future); !errors.Is(err, ErrHeightMismatch) {
		t.Fatalf("expected a header naming the set of its own height to be rejected, got %v", err)
	}
}

// The child is certified by the next set the proven block names, which may differ from the
// proven block's own set: the set after the state the block commits.
func TestRegistryVerifierChecksTheChildWithItsOwnValidatorSet(t *testing.T) {
	oldSet := abcSet(t)
	newSet := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "d", VotingPower: 1, PublicKey: []byte("d-pub")},
	})
	registry := heightRegistry{sets: map[types.Height]validator.Set{4: oldSet, 5: newSet}}
	verifier := NewRegistryVerifier(registry, acceptSignatureVerifier{})

	// Block 6 commits the state after block 4, whose updates produce the set of registry height 5:
	// it names that set as next, and its child is certified by it.
	header := types.Header{ChainID: "vexo-test", Height: 6, Round: 3, JustifyRound: 2, ValidatorSetHeight: 4, StateHeight: 4, ValidatorSetHash: oldSet.Hash(), NextValidatorSetHash: newSet.Hash()}
	child := types.Header{
		ChainID:              header.ChainID,
		Height:               7,
		Round:                4,
		JustifyRound:         3,
		PreviousBlockHash:    HeaderHash(header),
		ValidatorSetHeight:   ChildValidatorSetHeight(header),
		StateHeight:          ChildStateHeight(header),
		ValidatorSetHash:     newSet.Hash(),
		NextValidatorSetHash: newSet.Hash(),
	}
	if child.ValidatorSetHeight != 5 || child.StateHeight != 5 {
		t.Fatalf("expected the child to name registry height 5 and state 5, got %d and %d", child.ValidatorSetHeight, child.StateHeight)
	}
	proof := NewProof(header, certFor(oldSet, header, 3, []types.ValidatorID{"a", "b"})).
		WithChild(child, certFor(newSet, child, 4, []types.ValidatorID{"b", "d"}))
	if err := verifier.VerifyFinalityProof(proof); err != nil {
		t.Fatal(err)
	}

	// A child that keeps the old set although the block fixes the new one is not the child the
	// chain can have.
	stale := child
	stale.ValidatorSetHeight = 4
	stale.ValidatorSetHash = oldSet.Hash()
	staleProof := NewProof(header, certFor(oldSet, header, 3, []types.ValidatorID{"a", "b"})).
		WithChild(stale, certFor(oldSet, stale, 4, []types.ValidatorID{"b", "c"}))
	if err := verifier.VerifyFinalityProof(staleProof); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected a child naming the wrong set to be rejected, got %v", err)
	}

	// The proven block's quorum signed the next set: a block naming another one cannot be completed
	// by the child of the new set.
	misnamed := header
	misnamed.NextValidatorSetHash = oldSet.Hash()
	misnamedChild := child
	misnamedChild.PreviousBlockHash = HeaderHash(misnamed)
	misnamedProof := NewProof(misnamed, certFor(oldSet, misnamed, 3, []types.ValidatorID{"a", "b"})).
		WithChild(misnamedChild, certFor(newSet, misnamedChild, 4, []types.ValidatorID{"b", "d"}))
	if err := verifier.VerifyFinalityProof(misnamedProof); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected a child outside the next set its parent names to be rejected, got %v", err)
	}

	// The child must be justified by the certificate of the proven block's view.
	unjustified := child
	unjustified.JustifyRound = 2
	unjustifiedProof := NewProof(header, certFor(oldSet, header, 3, []types.ValidatorID{"a", "b"})).
		WithChild(unjustified, certFor(newSet, unjustified, 4, []types.ValidatorID{"b", "d"}))
	if err := verifier.VerifyFinalityProof(unjustifiedProof); !errors.Is(err, ErrViewsNotConsecutive) {
		t.Fatalf("expected a child not justified by the block's view to be rejected, got %v", err)
	}
}

func TestVerifierRejectsMissingSignature(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	proof.QuorumCert.Signature = nil

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected missing signature, got %v", err)
	}
}

func TestVerifierRejectsInsufficientQuorum(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a"})

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrInsufficientQuorum) {
		t.Fatalf("expected insufficient quorum, got %v", err)
	}
}

func TestVerifierRejectsUnknownSigner(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a", "unknown"})

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrUnknownSigner) {
		t.Fatalf("expected unknown signer, got %v", err)
	}
}

func TestVerifierRejectsDuplicateSigner(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a", "a", "b"})

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrDuplicateSigner) {
		t.Fatalf("expected duplicate signer, got %v", err)
	}
}

func TestVerifierRejectsVotingPowerOverflow(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{
		{ID: "a", VotingPower: ^types.VotingPower(0)},
		{ID: "b", VotingPower: 1},
	})
	proof := validProof(set, []types.ValidatorID{"a", "b"})

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, types.ErrVotingPowerOverflow) {
		t.Fatalf("expected voting power overflow, got %v", err)
	}
}

func TestVerifierRejectsVotingPowerMismatch(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	proof.QuorumCert.VotingPower = 3

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrVotingPowerMismatch) {
		t.Fatalf("expected voting power mismatch, got %v", err)
	}
}

func TestVerifierRejectsInvalidAggregateSignature(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")}})
	proof := validProof(set, []types.ValidatorID{"a"})

	err := NewVerifier(set, rejectSignatureVerifier{}).VerifyFinalityProof(proof)
	if !errors.Is(err, ErrMissingQCSignature) {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestVerifierContextCancellation(t *testing.T) {
	set := testValidatorSet(t, []validator.Validator{{ID: "a", VotingPower: 1}})
	proof := validProof(set, []types.ValidatorID{"a"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProofWithContext(ctx, proof)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestParseSignersRejectsEmptySigner(t *testing.T) {
	_, err := ParseSigners(types.Bitmap("a,,b"))
	if !errors.Is(err, ErrEmptySigner) {
		t.Fatalf("expected empty signer, got %v", err)
	}
}

func TestHasQuorum(t *testing.T) {
	cases := []struct {
		power    types.VotingPower
		total    types.VotingPower
		expected bool
	}{
		{power: 0, total: 0, expected: false},
		{power: 1, total: 3, expected: false},
		{power: 2, total: 3, expected: true},
		{power: 6, total: 7, expected: true},
		{power: 4, total: 7, expected: false},
		{power: ^types.VotingPower(0), total: ^types.VotingPower(0), expected: true},
		{power: ^types.VotingPower(0)/3*2 + 1, total: ^types.VotingPower(0), expected: true},
	}
	for _, testCase := range cases {
		if got := HasQuorum(testCase.power, testCase.total); got != testCase.expected {
			t.Fatalf("power %d total %d: expected %v, got %v", testCase.power, testCase.total, testCase.expected, got)
		}
	}
}

func TestWithChildDerivesTheChildHash(t *testing.T) {
	set := abcSet(t)
	proof := validProof(set, []types.ValidatorID{"a", "b"})
	child := proof.CommitChain[0]

	rebuilt := NewProof(proof.Header, proof.QuorumCert).WithChild(child.Header, child.QuorumCert)
	if rebuilt.BlockHash != HeaderHash(proof.Header) || rebuilt.CommitChain[0].BlockHash != HeaderHash(child.Header) {
		t.Fatalf("expected hashes derived from the headers, got %+v", rebuilt)
	}
	if err := NewVerifier(set, acceptSignatureVerifier{}).VerifyFinalityProof(rebuilt); err != nil {
		t.Fatal(err)
	}
}

// validProof is a complete 2-chain proof for the block at height 1, certified in view 0, with its
// child certified in view 1. Signatures are placeholders for acceptSignatureVerifier.
func validProof(set validator.Set, signers []types.ValidatorID) Proof {
	return proofAt(set, 1, types.Hash{}, 0, signers)
}

// proofAt builds a complete 2-chain proof for the block at height with the given application hash,
// proposed and certified in view round and naming the latest set its height can have; the child at
// height+1 is proposed and certified in view round+1.
func proofAt(set validator.Set, height types.Height, appHash types.Hash, round types.Round, signers []types.ValidatorID) Proof {
	setHeight := FirstValidatorSetHeight
	var stateHeight types.Height
	if height > 1 {
		setHeight = height - 1
		stateHeight = height - 2
	}
	header := types.Header{
		ChainID:              "vexo-test",
		Height:               height,
		Round:                round,
		AppHash:              appHash,
		ValidatorSetHeight:   setHeight,
		StateHeight:          stateHeight,
		ValidatorSetHash:     set.Hash(),
		NextValidatorSetHash: set.Hash(),
	}
	proof := NewProof(header, certFor(set, header, round, signers))
	childHeader := types.Header{
		ChainID:              header.ChainID,
		Height:               header.Height + 1,
		Round:                round + 1,
		JustifyRound:         round,
		PreviousBlockHash:    proof.BlockHash,
		ValidatorSetHeight:   ChildValidatorSetHeight(header),
		StateHeight:          ChildStateHeight(header),
		ValidatorSetHash:     set.Hash(),
		NextValidatorSetHash: set.Hash(),
	}
	return proof.WithChild(childHeader, certFor(set, childHeader, round+1, signers))
}

// certFor builds a placeholder certificate for the block with the given header.
func certFor(set validator.Set, header types.Header, round types.Round, signers []types.ValidatorID) QuorumCert {
	cert := QuorumCert{
		Height:    header.Height,
		Round:     round,
		BlockHash: HeaderHash(header),
		Signers:   EncodeSigners(signers),
		Signature: types.AggregateSignature("signature"),
	}
	for _, signer := range signers {
		if validatorInfo, found := set.Get(signer); found {
			cert.VotingPower = types.MustAddVotingPowerSaturating(cert.VotingPower, validatorInfo.VotingPower)
		}
	}
	return cert
}

// reseatChild recomputes the hashes that follow from the child header, so a test can change one
// header field and still hand the verifier an internally consistent link.
func reseatChild(proof *Proof) {
	link := &proof.CommitChain[0]
	link.BlockHash = HeaderHash(link.Header)
	link.QuorumCert.BlockHash = link.BlockHash
	link.QuorumCert.Height = link.Header.Height
}

func voteMessage(t *testing.T, cert QuorumCert) []byte {
	t.Helper()
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusVote, cert.SignBytes())
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func testValidatorSet(t *testing.T, validators []validator.Validator) validator.Set {
	t.Helper()
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

type acceptSignatureVerifier struct{}

func (acceptSignatureVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return true
}

type rejectSignatureVerifier struct{}

func (rejectSignatureVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	return false
}

// messageVerifier accepts an aggregate only over one of the given (domain-prefixed) messages.
type messageVerifier struct {
	accepted [][]byte
}

func (verifier messageVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	for _, accepted := range verifier.accepted {
		if bytes.Equal(accepted, message) {
			return true
		}
	}
	return false
}

type staticRegistry struct {
	set validator.Set
}

func (registry staticRegistry) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	return registry.set, nil
}

func (registry staticRegistry) ApplyJoin(ctx context.Context, candidate validator.Candidate) (validator.Validator, error) {
	return validator.Validator{}, nil
}

func (registry staticRegistry) ApplyLeave(ctx context.Context, id types.ValidatorID) error {
	return nil
}

func (registry staticRegistry) UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error {
	return nil
}

// heightRegistry serves a different validator set per height.
type heightRegistry struct {
	staticRegistry
	sets map[types.Height]validator.Set
}

func (registry heightRegistry) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	set, found := registry.sets[height]
	if !found {
		return nil, ErrFinalitySetUnavailable
	}
	return set, nil
}

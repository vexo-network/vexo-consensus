package consensus

import (
	"encoding/json"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// evidenceFixture is a validator set of four members that all hold real signing keys. Every test
// judges evidence the way the slashing module does: with VerifyEvidence, from the proof and the
// validator set alone. A test starts from evidence that verifies and breaks exactly one thing.
//
// The fixture is built with the test's own t, so a subtest must not call its fatal helpers.
type evidenceFixture struct {
	t        *testing.T
	cluster  *testCluster
	set      validator.Set
	verifier EvidenceVerifierBundle
}

func newEvidenceFixture(t *testing.T) evidenceFixture {
	t.Helper()
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	return evidenceFixture{
		t:        t,
		cluster:  cluster,
		set:      validator.NewSet(cluster.validators(cluster.ids...)),
		verifier: NewEvidenceVerifier(vexocrypto.DeterministicSigner{}, vexocrypto.DeterministicAggregateSigner{}),
	}
}

func (fixture evidenceFixture) verify(evidence slashing.Evidence) error {
	return VerifyEvidence(evidence, fixture.set, fixture.verifier)
}

func (fixture evidenceFixture) mustVerify(evidence slashing.Evidence) {
	fixture.t.Helper()
	if err := fixture.verify(evidence); err != nil {
		fixture.t.Fatalf("expected %s evidence to verify, got %v", evidence.Type, err)
	}
}

// need returns evidence a constructor built, or fails the test.
func (fixture evidenceFixture) need(evidence slashing.Evidence, err error) slashing.Evidence {
	fixture.t.Helper()
	if err != nil {
		fixture.t.Fatal(err)
	}
	return evidence
}

// handmade assembles evidence around an arbitrary proof, bypassing the constructors that refuse
// to build evidence that proves nothing.
func (fixture evidenceFixture) handmade(evidenceType slashing.EvidenceType, validatorID types.ValidatorID, height types.Height, round types.Round, proof any) slashing.Evidence {
	fixture.t.Helper()
	encoded, err := json.Marshal(proof)
	if err != nil {
		fixture.t.Fatal(err)
	}
	return slashing.Evidence{Type: evidenceType, Validator: validatorID, Height: height, Round: round, Proof: encoded}
}

// conflictingVotes is evidence that the validator signed two different blocks in one view.
func (fixture evidenceFixture) conflictingVotes(validatorID types.ValidatorID, height types.Height, round types.Round) slashing.Evidence {
	fixture.t.Helper()
	return fixture.need(NewConflictingVoteEvidence(
		fixture.cluster.vote(validatorID, height, round, types.Hash{1}),
		fixture.cluster.vote(validatorID, height, round, types.Hash{2}),
	))
}

// conflictingTimeoutVotes is evidence that the validator timed out in one view with two different
// high QCs.
func (fixture evidenceFixture) conflictingTimeoutVotes(validatorID types.ValidatorID, height types.Height, round types.Round) slashing.Evidence {
	fixture.t.Helper()
	return fixture.need(NewConflictingTimeoutVoteEvidence(
		fixture.cluster.timeoutVote(validatorID, height, round, finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}),
		fixture.cluster.timeoutVote(validatorID, height, round, finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}),
	))
}

// mismatchedCommitmentBlock names a data availability commitment that is not the commitment of its
// transactions.
func mismatchedCommitmentBlock() types.Block {
	return types.Block{
		Header: types.Header{ChainID: testChainID, Height: 1, ConsensusHash: dataavailability.Commitment([]types.Tx{[]byte("other")})},
		Txs:    []types.Tx{[]byte("tx")},
	}
}

// uncommittedBlock carries transactions but no data availability commitment.
func uncommittedBlock() types.Block {
	return types.Block{
		Header: types.Header{ChainID: testChainID, Height: 1},
		Txs:    []types.Tx{[]byte("tx")},
	}
}

// staleValidatorSetHash is the validator set hash a proposal names when it is built on the wrong
// set.
var staleValidatorSetHash = types.Hash{7}

// staleSetBlock names a validator set hash that is not the hash of the fixture's set.
func staleSetBlock() types.Block {
	return types.Block{Header: types.Header{ChainID: testChainID, Height: 1, ValidatorSetHash: staleValidatorSetHash}}
}

// proposalOf returns the proposal of block, signed by proposer.
func (fixture evidenceFixture) proposalOf(proposer types.ValidatorID, block types.Block) Proposal {
	fixture.t.Helper()
	return fixture.cluster.proposal(proposer, block, 0, finality.QuorumCert{}, finality.TimeoutCert{})
}

// forgedProposal is a proposal that names blamed as its proposer but is signed by signer.
func (fixture evidenceFixture) forgedProposal(blamed types.ValidatorID, signer types.ValidatorID, block types.Block) Proposal {
	fixture.t.Helper()
	proposal := Proposal{Block: block, Proposer: blamed}
	signature, err := vexocrypto.SignWithDomain(fixture.cluster.signers[signer], vexocrypto.DomainConsensusProposal, ProposalSignBytes(proposal))
	if err != nil {
		fixture.t.Fatal(err)
	}
	proposal.Signature = signature
	return proposal
}

// proposalEvidenceCase builds one kind of evidence from a proposal. Every kind here is decidable
// from the proof and the validator set alone.
type proposalEvidenceCase struct {
	name  string
	block types.Block
	build func(fixture evidenceFixture, proposal Proposal) (slashing.Evidence, error)
}

func daMismatchCase() proposalEvidenceCase {
	return proposalEvidenceCase{
		name:  "data availability mismatch",
		block: mismatchedCommitmentBlock(),
		build: func(_ evidenceFixture, proposal Proposal) (slashing.Evidence, error) {
			return NewInvalidProposalEvidence(proposal, string(InvalidProposalReasonDAMismatch))
		},
	}
}

func missingDataCase() proposalEvidenceCase {
	return proposalEvidenceCase{
		name:  "missing data",
		block: uncommittedBlock(),
		build: func(_ evidenceFixture, proposal Proposal) (slashing.Evidence, error) {
			return NewDataAvailabilityInvalidProposalEvidence(proposal, InvalidProposalReasonMissingData)
		},
	}
}

func validatorSetHashCase() proposalEvidenceCase {
	return proposalEvidenceCase{
		name:  "validator set hash",
		block: staleSetBlock(),
		build: func(fixture evidenceFixture, proposal Proposal) (slashing.Evidence, error) {
			context := InvalidProposalVerificationContext{ExpectedValidatorSetHash: fixture.set.Hash()}
			return NewInvalidProposalEvidenceWithContext(proposal, context, InvalidProposalReasonValidatorSetHash, staleValidatorSetHash, "")
		},
	}
}

func unavailableDataCase() proposalEvidenceCase {
	return proposalEvidenceCase{
		name:  "unavailable data",
		block: uncommittedBlock(),
		build: func(_ evidenceFixture, proposal Proposal) (slashing.Evidence, error) {
			return NewUnavailableDataEvidence(proposal, "missing data availability commitment")
		},
	}
}

func proposalEvidenceCases() []proposalEvidenceCase {
	return []proposalEvidenceCase{daMismatchCase(), missingDataCase(), validatorSetHashCase(), unavailableDataCase()}
}

// proposalEvidence is evidence of the case against proposer, signed by proposer.
func (fixture evidenceFixture) proposalEvidence(testCase proposalEvidenceCase, proposer types.ValidatorID) slashing.Evidence {
	fixture.t.Helper()
	return fixture.need(testCase.build(fixture, fixture.proposalOf(proposer, testCase.block)))
}

func (fixture evidenceFixture) daMismatch(proposer types.ValidatorID) slashing.Evidence {
	fixture.t.Helper()
	return fixture.proposalEvidence(daMismatchCase(), proposer)
}

func (fixture evidenceFixture) unavailableData(proposer types.ValidatorID) slashing.Evidence {
	fixture.t.Helper()
	return fixture.proposalEvidence(unavailableDataCase(), proposer)
}

// finalityProof is a complete 2-chain proof for the block at height with the given application
// hash: QC(B) formed in view round and QC(C) of its child formed in view round+1, both signed by
// signers with their real keys.
func (fixture evidenceFixture) finalityProof(height types.Height, appHash types.Hash, round types.Round, signers ...types.ValidatorID) finality.Proof {
	fixture.t.Helper()
	header := types.Header{ChainID: testChainID, Height: height, Round: round, AppHash: appHash, ValidatorSetHeight: finality.FirstValidatorSetHeight, ValidatorSetHash: fixture.set.Hash()}
	proof := finality.NewProof(header, fixture.cluster.qc(height, round, finality.HeaderHash(header), signers...))
	child := types.Header{ChainID: testChainID, Height: height + 1, Round: round + 1, JustifyRound: round, PreviousBlockHash: proof.BlockHash, ValidatorSetHeight: finality.ChildValidatorSetHeight(header), ValidatorSetHash: fixture.set.Hash()}
	return proof.WithChild(child, fixture.cluster.qc(child.Height, round+1, finality.HeaderHash(child), signers...))
}

// conflictingFinalityProofs returns two proofs that finalize different blocks at height 1: a, b
// and c finalized the first in view 0, a, b and d finalized the second in view 2. Only a and b
// voted for both.
func (fixture evidenceFixture) conflictingFinalityProofs() (finality.Proof, finality.Proof) {
	fixture.t.Helper()
	first := fixture.finalityProof(1, types.Hash{1}, 0, "a", "b", "c")
	second := fixture.finalityProof(1, types.Hash{2}, 2, "a", "b", "d")
	return first, second
}

// finalityEvidence assembles finality conflict evidence by hand, so that it does not pass through
// the checks of finality.NewConflictEvidence.
func (fixture evidenceFixture) finalityEvidence(first finality.Proof, second finality.Proof, validatorID types.ValidatorID) slashing.Evidence {
	fixture.t.Helper()
	return fixture.handmade(slashing.EvidenceFinalityConflict, validatorID, first.Header.Height, first.QuorumCert.Round, finality.ConflictProof{First: first, Second: second})
}

// finalityConflict is evidence that the validator voted for two conflicting finalized blocks.
func (fixture evidenceFixture) finalityConflict(validatorID types.ValidatorID) slashing.Evidence {
	fixture.t.Helper()
	first, second := fixture.conflictingFinalityProofs()
	return fixture.need(finality.NewConflictEvidence(fixture.set, first, second, validatorID))
}

// cloneProof copies the commit chain, which a shallow copy of a proof would share.
func cloneProof(proof finality.Proof) finality.Proof {
	proof.CommitChain = append([]finality.CommitLink(nil), proof.CommitChain...)
	return proof
}

func requireSameOffence(t *testing.T, first slashing.Evidence, second slashing.Evidence) {
	t.Helper()
	firstKey, err := slashing.OffenceKey(first)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := slashing.OffenceKey(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey != secondKey {
		t.Fatalf("expected one offence key, got %q and %q", firstKey, secondKey)
	}
}

func TestVerifyEvidenceAcceptsConflictingVotes(t *testing.T) {
	fixture := newEvidenceFixture(t)
	evidence := fixture.conflictingVotes("a", 3, 2)
	fixture.mustVerify(evidence)

	// Which vote is listed first does not matter: it is one offence either way.
	swapped := fixture.need(NewConflictingVoteEvidence(
		fixture.cluster.vote("a", 3, 2, types.Hash{2}),
		fixture.cluster.vote("a", 3, 2, types.Hash{1}),
	))
	fixture.mustVerify(swapped)
	requireSameOffence(t, evidence, swapped)
}

func TestVerifyEvidenceTreatsTheLegacyDoubleSignLabelAsConflictingVote(t *testing.T) {
	fixture := newEvidenceFixture(t)
	evidence := fixture.conflictingVotes("a", 1, 0)
	legacy := evidence
	legacy.Type = slashing.EvidenceDoubleSign

	fixture.mustVerify(legacy)
	requireSameOffence(t, evidence, legacy)

	// The legacy label opens no second way to punish the same offence for a proof that is not a
	// conflicting vote.
	timeoutProof := fixture.conflictingTimeoutVotes("a", 1, 0)
	timeoutProof.Type = slashing.EvidenceDoubleSign
	if err := fixture.verify(timeoutProof); err == nil {
		t.Fatal("expected timeout vote evidence under the double_sign label to be rejected")
	}
}

func TestVerifyEvidenceRejectsUnknownValidator(t *testing.T) {
	fixture := newEvidenceFixture(t)
	outsider := newEqualTestCluster(t, "x")
	evidence := fixture.need(NewConflictingVoteEvidence(
		outsider.vote("x", 1, 0, types.Hash{1}),
		outsider.vote("x", 1, 0, types.Hash{2}),
	))
	if err := fixture.verify(evidence); !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected unknown evidence validator, got %v", err)
	}

	// Naming an outsider in genuine evidence does not help either.
	relabelled := fixture.conflictingVotes("a", 1, 0)
	relabelled.Validator = "x"
	if err := fixture.verify(relabelled); !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected unknown evidence validator, got %v", err)
	}
}

func TestVerifyEvidenceRejectsEvidenceRelabelledToAnotherMember(t *testing.T) {
	fixture := newEvidenceFixture(t)
	evidence := fixture.conflictingVotes("a", 1, 0)

	// Naming b in the envelope contradicts the votes inside it.
	relabelled := evidence
	relabelled.Validator = "b"
	if err := fixture.verify(relabelled); !errors.Is(err, ErrVotePairMismatch) {
		t.Fatalf("expected pair mismatch, got %v", err)
	}

	// Rewriting the votes as well leaves a's signatures, which b's key does not verify.
	framed := fixture.need(NewConflictingVoteEvidence(
		withValidator(fixture.cluster.vote("a", 1, 0, types.Hash{1}), "b"),
		withValidator(fixture.cluster.vote("a", 1, 0, types.Hash{2}), "b"),
	))
	if err := fixture.verify(framed); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}
}

func TestVerifyEvidenceRejectsAVoteTheValidatorDidNotSign(t *testing.T) {
	fixture := newEvidenceFixture(t)
	// b signs a second vote in a's name to frame a for equivocating.
	forged := withValidator(fixture.cluster.vote("b", 1, 0, types.Hash{2}), "a")
	evidence := fixture.need(NewConflictingVoteEvidence(fixture.cluster.vote("a", 1, 0, types.Hash{1}), forged))

	if err := fixture.verify(evidence); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}
}

func TestVerifyEvidenceRejectsUnsignedVotes(t *testing.T) {
	fixture := newEvidenceFixture(t)
	unsigned := fixture.need(NewConflictingVoteEvidence(
		testVote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	))
	if err := fixture.verify(unsigned); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}

	halfSigned := fixture.need(NewConflictingVoteEvidence(
		fixture.cluster.vote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	))
	if err := fixture.verify(halfSigned); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}
}

func TestVerifyEvidenceRejectsVotesAlteredAfterSigning(t *testing.T) {
	fixture := newEvidenceFixture(t)

	// a signed the same block twice; pointing one copy at another block makes the pair conflict,
	// but a never signed that vote.
	first := fixture.cluster.vote("a", 1, 0, types.Hash{1})
	second := fixture.cluster.vote("a", 1, 0, types.Hash{1})
	second.BlockHash = types.Hash{2}
	repointed := fixture.need(NewConflictingVoteEvidence(first, second))
	if err := fixture.verify(repointed); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}

	// Genuine votes of view 0 moved to view 1 are not a double vote of view 1.
	moved := fixture.conflictingVotes("a", 1, 0)
	decoded, err := DecodeConflictingVoteProof(moved.Proof)
	if err != nil {
		t.Fatal(err)
	}
	decoded.First.Round, decoded.Second.Round = 1, 1
	replayed := fixture.handmade(slashing.EvidenceConflictingVote, "a", 1, 1, decoded)
	if err := fixture.verify(replayed); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}
}

func TestVerifyEvidenceRejectsVotesThatDoNotConflict(t *testing.T) {
	fixture := newEvidenceFixture(t)
	vote := fixture.cluster.vote("a", 1, 0, types.Hash{1})

	// One signed vote replayed twice proves nothing.
	replay := fixture.handmade(slashing.EvidenceConflictingVote, "a", 1, 0, ConflictingVoteProof{First: vote, Second: vote})
	if err := fixture.verify(replay); !errors.Is(err, ErrVotesDoNotConflict) {
		t.Fatalf("expected no vote conflict, got %v", err)
	}

	// Votes of different views are not an equivocation: a votes once per view.
	otherView := fixture.cluster.vote("a", 1, 1, types.Hash{2})
	crossView := fixture.handmade(slashing.EvidenceConflictingVote, "a", 1, 0, ConflictingVoteProof{First: vote, Second: otherView})
	if err := fixture.verify(crossView); !errors.Is(err, ErrVotePairMismatch) {
		t.Fatalf("expected pair mismatch, got %v", err)
	}

	// Votes of different heights likewise.
	otherHeight := fixture.cluster.vote("a", 2, 0, types.Hash{2})
	crossHeight := fixture.handmade(slashing.EvidenceConflictingVote, "a", 1, 0, ConflictingVoteProof{First: vote, Second: otherHeight})
	if err := fixture.verify(crossHeight); !errors.Is(err, ErrVotePairMismatch) {
		t.Fatalf("expected pair mismatch, got %v", err)
	}
}

func TestVerifyEvidenceAcceptsConflictingTimeoutVotes(t *testing.T) {
	fixture := newEvidenceFixture(t)
	evidence := fixture.conflictingTimeoutVotes("a", 1, 0)
	fixture.mustVerify(evidence)

	// The timeout offence is distinct from the vote offence of the same view.
	voteKey, err := slashing.OffenceKey(fixture.conflictingVotes("a", 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	timeoutKey, err := slashing.OffenceKey(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if voteKey == timeoutKey {
		t.Fatalf("expected distinct offence keys, got %q", voteKey)
	}
}

func TestVerifyEvidenceRejectsForgedTimeoutVotes(t *testing.T) {
	fixture := newEvidenceFixture(t)
	highQC := finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}
	otherHighQC := finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}

	unsigned := fixture.need(NewConflictingTimeoutVoteEvidence(
		testTimeoutVote("a", 1, 0, types.Hash{1}),
		testTimeoutVote("a", 1, 0, types.Hash{2}),
	))
	if err := fixture.verify(unsigned); !errors.Is(err, ErrInvalidEvidenceTimeoutSignature) {
		t.Fatalf("expected invalid timeout signature, got %v", err)
	}

	// a signed one high QC; the second vote was re-pointed at another after signing.
	first := fixture.cluster.timeoutVote("a", 1, 0, highQC)
	second := fixture.cluster.timeoutVote("a", 1, 0, highQC)
	second.HighQC = otherHighQC
	repointed := fixture.need(NewConflictingTimeoutVoteEvidence(first, second))
	if err := fixture.verify(repointed); !errors.Is(err, ErrInvalidEvidenceTimeoutSignature) {
		t.Fatalf("expected invalid timeout signature, got %v", err)
	}

	// b timed out in a's name.
	forged := fixture.cluster.timeoutVote("b", 1, 0, otherHighQC)
	forged.ValidatorID = "a"
	framed := fixture.need(NewConflictingTimeoutVoteEvidence(fixture.cluster.timeoutVote("a", 1, 0, highQC), forged))
	if err := fixture.verify(framed); !errors.Is(err, ErrInvalidEvidenceTimeoutSignature) {
		t.Fatalf("expected invalid timeout signature, got %v", err)
	}

	// Timeout votes with the same high QC are a replay, not a conflict.
	replay := fixture.handmade(slashing.EvidenceConflictingTimeoutVote, "a", 1, 0, ConflictingTimeoutVoteProof{First: first, Second: first})
	if err := fixture.verify(replay); !errors.Is(err, ErrTimeoutVotesDoNotConflict) {
		t.Fatalf("expected no timeout conflict, got %v", err)
	}
}

func TestVerifyEvidenceAcceptsProposalFaultsDecidableFromTheProof(t *testing.T) {
	fixture := newEvidenceFixture(t)
	for _, testCase := range proposalEvidenceCases() {
		evidence := fixture.proposalEvidence(testCase, "a")
		if err := fixture.verify(evidence); err != nil {
			t.Fatalf("%s: expected evidence to verify, got %v", testCase.name, err)
		}
	}
}

func TestVerifyEvidenceRejectsProposalFaultsTheProposerDidNotSign(t *testing.T) {
	fixture := newEvidenceFixture(t)
	for _, testCase := range proposalEvidenceCases() {
		// b signs a block in a's name to frame a.
		forged := fixture.need(testCase.build(fixture, fixture.forgedProposal("a", "b", testCase.block)))
		if err := fixture.verify(forged); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("%s: expected invalid proposal for a forged signature, got %v", testCase.name, err)
		}

		unsigned := fixture.need(testCase.build(fixture, Proposal{Block: testCase.block, Proposer: "a"}))
		if err := fixture.verify(unsigned); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("%s: expected invalid proposal for an unsigned proposal, got %v", testCase.name, err)
		}

		// A header changed after the proposer signed it is not what the proposer proposed.
		proposal := fixture.proposalOf("a", testCase.block)
		proposal.Block.Header.TimeUnixNano++
		altered := fixture.need(testCase.build(fixture, proposal))
		if err := fixture.verify(altered); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("%s: expected invalid proposal for an altered header, got %v", testCase.name, err)
		}
	}
}

func TestVerifyEvidenceRejectsDataMismatchFramedOntoAnHonestProposal(t *testing.T) {
	fixture := newEvidenceFixture(t)
	honest := fixture.proposalOf("a", fixture.cluster.block(1, types.Hash{}, "tx-one", "tx-two"))
	if err := VerifyBlockBody(honest.Block); err != nil {
		t.Fatalf("expected a well-formed honest block, got %v", err)
	}

	// The attacker keeps the signed header and swaps the body to fake a commitment mismatch.
	swapped := honest
	swapped.Block.Txs = []types.Tx{[]byte("swapped")}
	evidence := fixture.need(NewInvalidProposalEvidence(swapped, string(InvalidProposalReasonDAMismatch)))
	if err := fixture.verify(evidence); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal for a swapped body, got %v", err)
	}

	// Stripping the commitment from the signed header fakes unavailable data the same way.
	withheld := honest
	withheld.Block.Header.ConsensusHash = types.Hash{}
	unavailable := fixture.need(NewUnavailableDataEvidence(withheld, "stripped commitment"))
	if err := fixture.verify(unavailable); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal for a stripped commitment, got %v", err)
	}
}

func TestVerifyEvidenceChecksValidatorSetHashClaimsAgainstTheSet(t *testing.T) {
	fixture := newEvidenceFixture(t)
	proposal := fixture.proposalOf("a", staleSetBlock())

	// The claim must name the hash of the set the evidence is judged against.
	wrongExpectation := InvalidProposalVerificationContext{ExpectedValidatorSetHash: types.Hash{8}}
	claimed := fixture.need(NewInvalidProposalEvidenceWithContext(proposal, wrongExpectation, InvalidProposalReasonValidatorSetHash, staleValidatorSetHash, ""))
	if err := fixture.verify(claimed); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal for a wrong expected hash, got %v", err)
	}

	// A proposer that used the right hash cannot be blamed for using the wrong one.
	right := fixture.proposalOf("a", types.Block{Header: types.Header{ChainID: testChainID, Height: 1, ValidatorSetHash: fixture.set.Hash()}})
	framing := InvalidProposalVerificationContext{ExpectedValidatorSetHash: staleValidatorSetHash}
	framed := fixture.need(NewInvalidProposalEvidenceWithContext(right, framing, InvalidProposalReasonValidatorSetHash, fixture.set.Hash(), ""))
	if err := fixture.verify(framed); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal for a proposer that used the right hash, got %v", err)
	}

	// Evidence that is not bound to its context proves nothing.
	correct := InvalidProposalVerificationContext{ExpectedValidatorSetHash: fixture.set.Hash()}
	unbound := fixture.need(NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), correct.ExpectedValidatorSetHash, staleValidatorSetHash))
	if err := fixture.verify(unbound); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected a missing context, got %v", err)
	}
}

func TestVerifyEvidenceRejectsProposalFaultsThatNeedNodeLocalData(t *testing.T) {
	fixture := newEvidenceFixture(t)
	expectedAppHash := types.Hash{9}
	actualAppHash := types.Hash{1}
	appHashProposal := fixture.proposalOf("a", types.Block{Header: types.Header{ChainID: testChainID, Height: 1, AppHash: actualAppHash}})
	appHashEvidence := fixture.need(NewInvalidProposalHashEvidence(appHashProposal, string(InvalidProposalReasonAppHash), expectedAppHash, actualAppHash))

	txProposal := fixture.proposalOf("a", types.Block{Header: types.Header{ChainID: testChainID, Height: 1}, Txs: []types.Tx{[]byte("bad-tx")}})
	expectedResults := []types.Result{{Code: 1, Log: "ante rejected tx"}}
	actualResults := []types.Result{{Code: 0, Log: "accepted"}}
	txEvidence := fixture.need(NewInvalidProposalTxExecutionEvidence(txProposal, expectedResults, actualResults, 0, "ante rejected tx"))

	timestampProposal := fixture.proposalOf("a", types.Block{Header: types.Header{ChainID: testChainID, Height: 1, TimeUnixNano: 200}})
	timestampEvidence := fixture.need(NewInvalidProposalTimestampEvidence(timestampProposal, 100, 200))

	// The application hash, the result of a transaction and the time of a block are facts of the
	// chain, which a node that pruned them cannot check. A node must not slash on them.
	appHashContext := InvalidProposalVerificationContext{ExpectedAppHash: expectedAppHash}
	appHashContext.ContextProofHash = appHashContext.ProofHash()
	txContext := InvalidProposalVerificationContext{ExpectedTxResultsHash: HashTxResults(expectedResults)}
	txContext.ContextProofHash = txContext.ProofHash()
	timestampContext := InvalidProposalVerificationContext{ExpectedTimeUnixNano: 100}
	timestampContext.ContextProofHash = timestampContext.ProofHash()
	cases := []struct {
		name     string
		evidence slashing.Evidence
		context  InvalidProposalVerificationContext
	}{
		{"app hash", appHashEvidence, appHashContext},
		{"tx validity", txEvidence, txContext},
		{"timestamp", timestampEvidence, timestampContext},
	}
	for _, testCase := range cases {
		if err := fixture.verify(testCase.evidence); !errors.Is(err, ErrEvidenceNotSelfContained) {
			t.Fatalf("%s: expected evidence that is not self-contained, got %v", testCase.name, err)
		}
		bound, err := BindInvalidProposalEvidenceContext(testCase.evidence, testCase.context)
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if err := fixture.verify(bound); !errors.Is(err, ErrEvidenceNotSelfContained) {
			t.Fatalf("%s: binding a context must not make the claim decidable, got %v", testCase.name, err)
		}
	}
}

func TestVerifyInvalidProposalAppHashEvidenceWithContextBindsTheContextProofHash(t *testing.T) {
	fixture := newEvidenceFixture(t)
	expectedAppHash := types.Hash{9}
	actualAppHash := types.Hash{1}
	proposal := fixture.proposalOf("a", types.Block{Header: types.Header{ChainID: testChainID, Height: 1, AppHash: actualAppHash}})
	evidence := fixture.need(NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonAppHash), expectedAppHash, actualAppHash))

	context := InvalidProposalVerificationContext{ExpectedAppHash: expectedAppHash}
	context.ContextProofHash = context.ProofHash()
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, context); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected an unbound claim to need its context, got %v", err)
	}
	bound := fixture.need(BindInvalidProposalEvidenceContext(evidence, context))

	if err := VerifyInvalidProposalEvidenceWithContext(bound, InvalidProposalVerificationContext{ExpectedAppHash: expectedAppHash}); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected a missing context proof hash, got %v", err)
	}
	wrongProofHash := InvalidProposalVerificationContext{ExpectedAppHash: expectedAppHash, ContextProofHash: types.Hash{8}}
	if err := VerifyInvalidProposalEvidenceWithContext(bound, wrongProofHash); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected a context proof hash mismatch, got %v", err)
	}
	otherExpectation := InvalidProposalVerificationContext{ExpectedAppHash: types.Hash{5}}
	otherExpectation.ContextProofHash = otherExpectation.ProofHash()
	if err := VerifyInvalidProposalEvidenceWithContext(bound, otherExpectation); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a context for another expected hash to be rejected, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(bound, context); err != nil {
		t.Fatalf("expected evidence with its matching context to verify, got %v", err)
	}
}

func TestVerifyInvalidProposalTxValidityEvidenceWithContextBindsTheExpectedResults(t *testing.T) {
	fixture := newEvidenceFixture(t)
	proposal := fixture.proposalOf("a", types.Block{Header: types.Header{ChainID: testChainID, Height: 1}, Txs: []types.Tx{[]byte("bad-tx")}})
	expectedResults := []types.Result{{Code: 1, Log: "ante rejected tx"}}
	actualResults := []types.Result{{Code: 0, Log: "accepted"}}
	evidence := fixture.need(NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, actualResults, 0, "ante rejected tx"))

	context := InvalidProposalVerificationContext{ExpectedTxResultsHash: HashTxResults(expectedResults)}
	context.ContextProofHash = context.ProofHash()
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, context); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected an unbound claim to need its context, got %v", err)
	}
	bound := fixture.need(BindInvalidProposalEvidenceContext(evidence, context))

	if err := VerifyInvalidProposalEvidenceWithContext(bound, InvalidProposalVerificationContext{ExpectedTxResultsHash: types.Hash{9}}); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected a tx result hash mismatch, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(bound, context); err != nil {
		t.Fatalf("expected evidence with its matching context to verify, got %v", err)
	}
}

func TestVerifyEvidenceAcceptsFinalityConflict(t *testing.T) {
	fixture := newEvidenceFixture(t)
	// a and b voted for both finalized blocks and are accountable for the conflict.
	for _, accountable := range []types.ValidatorID{"a", "b"} {
		fixture.mustVerify(fixture.finalityConflict(accountable))
	}
}

func TestVerifyEvidenceRejectsFinalityConflictAgainstAValidatorOutsideBothQuorums(t *testing.T) {
	fixture := newEvidenceFixture(t)
	first, second := fixture.conflictingFinalityProofs()
	// c voted only for the first block and d only for the second.
	for _, innocent := range []types.ValidatorID{"c", "d"} {
		evidence := fixture.finalityEvidence(first, second, innocent)
		if err := fixture.verify(evidence); !errors.Is(err, finality.ErrValidatorNotInFinalityConflict) {
			t.Fatalf("%s: expected a validator outside the conflict, got %v", innocent, err)
		}
	}
}

func TestVerifyEvidenceRejectsFinalityConflictWithoutTwoChainProofs(t *testing.T) {
	fixture := newEvidenceFixture(t)

	// A certified block is not a final block: certified blocks at one height are normal after a
	// view change, so the missing child link means no fault is proven.
	first, second := fixture.conflictingFinalityProofs()
	first = cloneProof(first)
	first.CommitChain = nil
	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrCommitChainTooShort) {
		t.Fatalf("expected a short commit chain for the first proof, got %v", err)
	}
	first, second = fixture.conflictingFinalityProofs()
	second = cloneProof(second)
	second.CommitChain = nil
	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrCommitChainTooShort) {
		t.Fatalf("expected a short commit chain for the second proof, got %v", err)
	}

	// The child must be certified in the view directly after the view of the block.
	first, second = fixture.conflictingFinalityProofs()
	second = cloneProof(second)
	child := &second.CommitChain[0]
	child.QuorumCert = fixture.cluster.qc(child.Header.Height, second.QuorumCert.Round+2, child.BlockHash, "a", "b", "d")
	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrViewsNotConsecutive) {
		t.Fatalf("expected non-consecutive views, got %v", err)
	}
}

func TestVerifyEvidenceRejectsFinalityConflictWithForgedCertificates(t *testing.T) {
	fixture := newEvidenceFixture(t)
	garbage := types.AggregateSignature("garbage-signature")

	// The certificate names a and b but carries the signatures of b, c and d: a and b are framed.
	first, second := fixture.conflictingFinalityProofs()
	second = cloneProof(second)
	second.QuorumCert.Signature = fixture.cluster.qc(second.Header.Height, second.QuorumCert.Round, second.BlockHash, "b", "c", "d").Signature
	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrMissingQCSignature) {
		t.Fatalf("expected a forged block certificate to be rejected, got %v", err)
	}

	// The child certificate is checked as strictly as the certificate of the block.
	first, second = fixture.conflictingFinalityProofs()
	first = cloneProof(first)
	first.CommitChain[0].QuorumCert.Signature = garbage
	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrMissingQCSignature) {
		t.Fatalf("expected a forged child certificate to be rejected, got %v", err)
	}

	first, second = fixture.conflictingFinalityProofs()
	second = cloneProof(second)
	second.QuorumCert.Signature = nil
	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrMissingQCSignature) {
		t.Fatalf("expected a certificate without a signature to be rejected, got %v", err)
	}
}

func TestVerifyEvidenceRejectsFinalityConflictWithoutAQuorum(t *testing.T) {
	fixture := newEvidenceFixture(t)
	first := fixture.finalityProof(1, types.Hash{1}, 0, "a", "b", "c")
	// Two of four validators are not a quorum, however genuine their signatures are.
	second := fixture.finalityProof(1, types.Hash{2}, 2, "a", "b")

	if err := fixture.verify(fixture.finalityEvidence(first, second, "a")); !errors.Is(err, finality.ErrInsufficientQuorum) {
		t.Fatalf("expected an insufficient quorum, got %v", err)
	}
}

func TestVerifyEvidenceRejectsFinalityProofsOfTheSameBlock(t *testing.T) {
	fixture := newEvidenceFixture(t)
	first, _ := fixture.conflictingFinalityProofs()
	// The same block, certified by another quorum. A block names its view, so a proof from another
	// view is a proof of another block.
	same := fixture.finalityProof(1, types.Hash{1}, 0, "a", "b", "d")

	if err := fixture.verify(fixture.finalityEvidence(first, first, "a")); !errors.Is(err, finality.ErrNoFinalityConflict) {
		t.Fatalf("expected no conflict between a proof and itself, got %v", err)
	}
	if err := fixture.verify(fixture.finalityEvidence(first, same, "a")); !errors.Is(err, finality.ErrNoFinalityConflict) {
		t.Fatalf("expected no conflict between two proofs of one block, got %v", err)
	}
}

func TestVerifyEvidenceIsJudgedAgainstTheValidatorSetOfItsHeight(t *testing.T) {
	fixture := newEvidenceFixture(t)
	evidenceOfA := fixture.conflictingVotes("a", 1, 0)
	evidenceOfD := fixture.conflictingVotes("d", 1, 0)
	finalityOfA := fixture.finalityConflict("a")

	// a left the validator set after the offence. The set of the offence's height still names a,
	// so leaving does not escape the penalty; the set of a later height does not.
	afterLeaving := validator.NewSet(fixture.cluster.validators("b", "c", "d"))
	fixture.mustVerify(evidenceOfA)
	if err := VerifyEvidence(evidenceOfA, afterLeaving, fixture.verifier); !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected the later set not to name the validator, got %v", err)
	}

	// d joined after the offence height: the set of that height has no d, so d cannot be blamed
	// for what happened before it was a validator.
	beforeJoining := validator.NewSet(fixture.cluster.validators("a", "b", "c"))
	if err := VerifyEvidence(evidenceOfD, beforeJoining, fixture.verifier); !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected an earlier set not to name a later validator, got %v", err)
	}

	// A key that replaced the key a signed with does not verify the old signatures.
	rotatedKeys := fixture.cluster.validators("a", "b", "c", "d")
	rotatedKeys[0].PublicKey = rotatedKeys[1].PublicKey
	rotated := validator.NewSet(rotatedKeys)
	if err := VerifyEvidence(evidenceOfA, rotated, fixture.verifier); !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected the rotated key to reject the old signatures, got %v", err)
	}

	// Finality proofs are bound to the hash of the set they were certified by.
	reweighted := fixture.cluster.validators("a", "b", "c", "d")
	reweighted[3].VotingPower = 2
	if err := VerifyEvidence(finalityOfA, validator.NewSet(reweighted), fixture.verifier); !errors.Is(err, finality.ErrValidatorSetMismatch) {
		t.Fatalf("expected a finality proof of another set to be rejected, got %v", err)
	}
}

func TestVerifyEvidenceBindsHeightRoundAndValidatorToTheProof(t *testing.T) {
	fixture := newEvidenceFixture(t)
	// The offence key of evidence is made of these fields, so a mutated copy of genuine evidence
	// would punish a different offence with a proof of the original one.
	cases := []struct {
		name         string
		evidence     slashing.Evidence
		otherMember  types.ValidatorID
		positionErr  error
		validatorErr error
	}{
		{"conflicting vote", fixture.conflictingVotes("a", 2, 1), "b", ErrVotePairMismatch, ErrVotePairMismatch},
		{"conflicting timeout vote", fixture.conflictingTimeoutVotes("a", 2, 1), "b", ErrVotePairMismatch, ErrVotePairMismatch},
		{"invalid proposal", fixture.daMismatch("a"), "b", ErrVotePairMismatch, ErrVotePairMismatch},
		{"unavailable data", fixture.unavailableData("a"), "b", ErrVotePairMismatch, ErrVotePairMismatch},
		{"finality conflict", fixture.finalityConflict("a"), "c", finality.ErrHeightMismatch, finality.ErrValidatorNotInFinalityConflict},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := fixture.verify(testCase.evidence); err != nil {
				t.Fatalf("expected the unmodified evidence to verify, got %v", err)
			}
			later := testCase.evidence
			later.Height++
			if err := fixture.verify(later); !errors.Is(err, testCase.positionErr) {
				t.Fatalf("expected a mutated height to be rejected with %v, got %v", testCase.positionErr, err)
			}
			otherView := testCase.evidence
			otherView.Round++
			if err := fixture.verify(otherView); !errors.Is(err, testCase.positionErr) {
				t.Fatalf("expected a mutated round to be rejected with %v, got %v", testCase.positionErr, err)
			}
			otherValidator := testCase.evidence
			otherValidator.Validator = testCase.otherMember
			if err := fixture.verify(otherValidator); !errors.Is(err, testCase.validatorErr) {
				t.Fatalf("expected a mutated validator to be rejected with %v, got %v", testCase.validatorErr, err)
			}
		})
	}
}

func TestVerifyEvidenceRejectsEvidenceUnderAnotherType(t *testing.T) {
	fixture := newEvidenceFixture(t)
	genuine := []slashing.Evidence{
		fixture.conflictingVotes("a", 1, 0),
		fixture.conflictingTimeoutVotes("a", 1, 0),
		fixture.daMismatch("a"),
		fixture.unavailableData("a"),
		fixture.finalityConflict("a"),
	}
	for _, evidence := range genuine {
		fixture.mustVerify(evidence)
		for _, other := range genuine {
			if other.Type == evidence.Type {
				continue
			}
			relabelled := evidence
			relabelled.Type = other.Type
			if err := fixture.verify(relabelled); err == nil {
				t.Fatalf("expected %s evidence labelled %s to be rejected", evidence.Type, other.Type)
			}
		}
	}
}

func TestVerifyEvidenceFailsClosedWithoutVerifiers(t *testing.T) {
	fixture := newEvidenceFixture(t)
	votes := fixture.conflictingVotes("a", 1, 0)
	timeoutVotes := fixture.conflictingTimeoutVotes("a", 1, 0)
	proposal := fixture.daMismatch("a")
	unavailable := fixture.unavailableData("a")
	conflict := fixture.finalityConflict("a")

	var none EvidenceVerifierBundle
	if err := VerifyEvidence(votes, fixture.set, none); !errors.Is(err, ErrEvidenceSignatureVerifier) {
		t.Fatalf("expected a missing signature verifier for votes, got %v", err)
	}
	if err := VerifyEvidence(timeoutVotes, fixture.set, none); !errors.Is(err, ErrEvidenceSignatureVerifier) {
		t.Fatalf("expected a missing signature verifier for timeout votes, got %v", err)
	}
	if err := VerifyEvidence(proposal, fixture.set, none); err == nil {
		t.Fatal("expected invalid proposal evidence to be rejected without a signature verifier")
	}
	if err := VerifyEvidence(unavailable, fixture.set, none); err == nil {
		t.Fatal("expected unavailable data evidence to be rejected without a signature verifier")
	}
	if err := VerifyEvidence(conflict, fixture.set, none); !errors.Is(err, ErrEvidenceFinalityVerifier) {
		t.Fatalf("expected a missing finality verifier, got %v", err)
	}

	// Each kind needs only the verifier it uses, and no other verifier stands in for it.
	signaturesOnly := NewEvidenceVerifier(vexocrypto.DeterministicSigner{}, nil)
	if err := VerifyEvidence(votes, fixture.set, signaturesOnly); err != nil {
		t.Fatalf("expected vote evidence to verify with signatures only, got %v", err)
	}
	if err := VerifyEvidence(conflict, fixture.set, signaturesOnly); !errors.Is(err, ErrEvidenceFinalityVerifier) {
		t.Fatalf("expected a missing finality verifier, got %v", err)
	}
	aggregatesOnly := NewEvidenceVerifier(nil, vexocrypto.DeterministicAggregateSigner{})
	if err := VerifyEvidence(votes, fixture.set, aggregatesOnly); !errors.Is(err, ErrEvidenceSignatureVerifier) {
		t.Fatalf("expected a missing signature verifier, got %v", err)
	}
	if err := VerifyEvidence(conflict, fixture.set, aggregatesOnly); err != nil {
		t.Fatalf("expected finality evidence to verify with aggregates only, got %v", err)
	}
}

func TestVerifyEvidenceRejectsUnsupportedProofType(t *testing.T) {
	fixture := newEvidenceFixture(t)
	for _, evidenceType := range []slashing.EvidenceType{"opaque", ""} {
		evidence := slashing.Evidence{Type: evidenceType, Validator: "a", Height: 1, Proof: []byte("opaque")}
		if err := fixture.verify(evidence); !errors.Is(err, ErrUnsupportedEvidenceProof) {
			t.Fatalf("%q: expected an unsupported proof, got %v", evidenceType, err)
		}
	}
}

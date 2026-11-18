package consensus

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// verifyCounts is what a countingVerifier saw: the signatures and the aggregate signatures it
// verified, and how many of each ran while the lock of the machine was held.
type verifyCounts struct {
	signatures, aggregates             int
	lockedSignatures, lockedAggregates int
}

// countingVerifier verifies like the deterministic backend and counts every verification. It probes
// the lock of machine with TryLock, so its locked counts are exact for a test whose calls to the
// machine do not overlap.
type countingVerifier struct {
	machine *StateMachine
	// onSignature, if set, runs before each signature verification.
	onSignature func()

	mu     sync.Mutex
	counts verifyCounts
}

func (verifier *countingVerifier) lockHeld() bool {
	if verifier.machine == nil || verifier.machine.mu.TryLock() {
		if verifier.machine != nil {
			verifier.machine.mu.Unlock()
		}
		return false
	}
	return true
}

func (verifier *countingVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	locked := verifier.lockHeld()
	verifier.mu.Lock()
	verifier.counts.signatures++
	if locked {
		verifier.counts.lockedSignatures++
	}
	hook := verifier.onSignature
	verifier.mu.Unlock()
	if hook != nil {
		hook()
	}
	return vexocrypto.DeterministicSigner{}.Verify(publicKey, message, signature)
}

func (verifier *countingVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	locked := verifier.lockHeld()
	verifier.mu.Lock()
	verifier.counts.aggregates++
	if locked {
		verifier.counts.lockedAggregates++
	}
	verifier.mu.Unlock()
	return vexocrypto.DeterministicAggregateSigner{}.VerifyAggregate(publicKeys, message, signature)
}

// take returns the counts since the last call.
func (verifier *countingVerifier) take() verifyCounts {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()

	counts := verifier.counts
	verifier.counts = verifyCounts{}
	return counts
}

func (cluster *testCluster) newCountingMachine() (*StateMachine, *countingVerifier) {
	cluster.t.Helper()
	verifier := &countingVerifier{}
	machine := cluster.newMachineWith(func(config *StateMachineConfig) {
		config.Signatures = verifier
		config.AggregateVerifier = verifier
	})
	verifier.machine = machine
	return machine, verifier
}

func expectCounts(t *testing.T, step string, got verifyCounts, want verifyCounts) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: expected %+v, got %+v", step, want, got)
	}
}

// The cache remembers a signature that verified and only that: a failure may depend on what the
// verifier knew then (the keys it had registered), so the next attempt asks the verifier again.
func TestTheCacheRemembersOnlySuccessfulVerifications(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine, verifier := cluster.newCountingMachine()
	member, _ := cluster.set.Get("a")
	signer, err := cluster.signers.signer("a")
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("statement")
	genuine, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	forged := types.Signature("forged")
	if machine.verifySignature(member.PublicKey, message, forged) || machine.verifySignature(member.PublicKey, message, forged) {
		t.Fatal("expected a forged signature to fail")
	}
	expectCounts(t, "two attempts with a forged signature", verifier.take(), verifyCounts{signatures: 2})

	if !machine.verifySignature(member.PublicKey, message, genuine) || !machine.verifySignature(member.PublicKey, message, genuine) {
		t.Fatal("expected the genuine signature to verify")
	}
	expectCounts(t, "two attempts with the genuine signature", verifier.take(), verifyCounts{signatures: 1})
	if machine.verifySignature(member.PublicKey, []byte("another statement"), genuine) {
		t.Fatal("expected the genuine signature not to vouch for another statement")
	}

	// A statement that failed verifies once the verifier accepts it, which a remembered failure
	// would hide.
	flaky := &flakyVerifier{}
	later := cluster.newMachineWith(func(config *StateMachineConfig) { config.Signatures = flaky })
	if later.verifySignature(member.PublicKey, message, genuine) {
		t.Fatal("expected the first attempt to fail")
	}
	flaky.accept = true
	if !later.verifySignature(member.PublicKey, message, genuine) {
		t.Fatal("expected the second attempt to be judged by the verifier again")
	}
}

// flakyVerifier accepts nothing until accept is set.
type flakyVerifier struct{ accept bool }

func (verifier *flakyVerifier) Verify(types.PublicKey, []byte, types.Signature) bool {
	return verifier.accept
}

// Every field a verification depends on separates its digest from the others, however the bytes of
// neighbouring fields are shifted into each other.
func TestDigestsSeparateEveryField(t *testing.T) {
	key, message, signature := types.PublicKey("ab"), []byte("cd"), types.Signature("ef")
	digests := map[verifiedDigest]string{}
	add := func(name string, digest verifiedDigest) {
		t.Helper()
		if other, found := digests[digest]; found {
			t.Fatalf("%s and %s share a digest", name, other)
		}
		digests[digest] = name
	}
	add("signature", signatureDigest(key, message, signature))
	add("key", signatureDigest(types.PublicKey("ac"), message, signature))
	add("message", signatureDigest(key, []byte("ce"), signature))
	add("signature bytes", signatureDigest(key, message, types.Signature("eg")))
	add("key shifted into the message", signatureDigest(types.PublicKey("a"), []byte("bcd"), signature))
	add("message shifted into the signature", signatureDigest(key, []byte("c"), types.Signature("def")))
	add("no key", signatureDigest(nil, []byte("abcd"), signature))

	set, power, signers, aggregate := types.Hash{1}, types.VotingPower(3), []byte("a,b"), types.AggregateSignature("sig")
	certificates := map[verifiedDigest]string{}
	addCertificate := func(name string, digest verifiedDigest) {
		t.Helper()
		if other, found := certificates[digest]; found {
			t.Fatalf("%s and %s share a digest", name, other)
		}
		certificates[digest] = name
	}
	addCertificate("certificate", certificateDigest(set, message, power, signers, aggregate))
	addCertificate("set", certificateDigest(types.Hash{2}, message, power, signers, aggregate))
	addCertificate("message", certificateDigest(set, []byte("ce"), power, signers, aggregate))
	addCertificate("claimed power", certificateDigest(set, message, power+1, signers, aggregate))
	addCertificate("no claimed power", certificateDigest(set, message, 0, signers, aggregate))
	addCertificate("signers", certificateDigest(set, message, power, []byte("a,c"), aggregate))
	addCertificate("signature", certificateDigest(set, message, power, signers, types.AggregateSignature("sih")))
	addCertificate("signers shifted into the signature", certificateDigest(set, message, power, []byte("a,"), types.AggregateSignature("bsig")))

	if signerListKey([]types.ValidatorID{"a,b"}) == nil || string(signerListKey([]types.ValidatorID{"a,b"})) == string(signerListKey([]types.ValidatorID{"a", "b"})) {
		t.Fatal("expected a signer whose id holds the separator to differ from two signers")
	}
	if string(signerListKey([]types.ValidatorID{"ab", "c"})) == string(signerListKey([]types.ValidatorID{"a", "bc"})) {
		t.Fatal("expected the signer lists to differ")
	}
}

// The cache is bounded: it forgets the oldest entries in bulk, keeps at least the latest half of its
// bound, and an entry in use survives the rotations that forget the others.
func TestTheCacheIsBounded(t *testing.T) {
	const bound = 8
	digest := func(index int) verifiedDigest { return signatureDigest(nil, []byte(fmt.Sprint(index)), nil) }
	set := newVerifiedSet(bound)
	for index := 0; index < 1000; index++ {
		set.add(digest(index))
		if size := set.size(); size > bound {
			t.Fatalf("after %d entries the set holds %d, more than its bound %d", index+1, size, bound)
		}
		for recent := max(0, index-bound/2+1); recent <= index; recent++ {
			if !set.contains(digest(recent)) {
				t.Fatalf("after %d entries the latest %d must be kept, but %d is not", index+1, bound/2, recent)
			}
		}
	}
	if set.contains(digest(0)) || set.contains(digest(900)) {
		t.Fatal("expected the old entries forgotten")
	}

	// An entry that is used again is not forgotten with the generation it was added to.
	used := newVerifiedSet(bound)
	used.add(digest(-1))
	for index := 0; index < 100; index++ {
		used.add(digest(index))
		if !used.contains(digest(-1)) {
			t.Fatalf("expected the entry in use kept after %d more entries", index+1)
		}
	}

	cache := newSigCache(bound, bound)
	cache.certificates.add(digest(1))
	if cache.signatures.contains(digest(1)) || !cache.certificates.contains(digest(1)) {
		t.Fatal("expected the signatures and the certificates kept apart")
	}
}

// A vote is verified before the machine takes its lock and is not verified again under it. A
// repeated vote costs no verification, and a copy of the vote with another signature is judged by
// the verifier, not vouched for by the cache.
func TestAVoteIsVerifiedOnceOutsideTheLock(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine, verifier := cluster.newCountingMachine()
	ctx := context.Background()
	block := cluster.blockAt(1, types.Hash{}, 0)
	hash := HashBlock(block)
	if err := machine.OnProposal(ctx, cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	expectCounts(t, "the proposal", verifier.take(), verifyCounts{signatures: 1})

	vote := cluster.vote("b", 1, 0, hash)
	if err := machine.OnVote(ctx, vote); err != nil {
		t.Fatal(err)
	}
	expectCounts(t, "the vote", verifier.take(), verifyCounts{signatures: 1})

	if err := machine.OnVote(ctx, vote); err != nil {
		t.Fatalf("expected the repeated vote accepted as before, got %v", err)
	}
	expectCounts(t, "the repeated vote", verifier.take(), verifyCounts{})

	forged := vote
	forged.Signature = types.Signature("forged")
	if err := machine.OnVote(ctx, forged); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected the forged copy rejected, got %v", err)
	}
	// Once before the lock and once under it: a failure is not remembered.
	expectCounts(t, "the forged vote", verifier.take(), verifyCounts{signatures: 2, lockedSignatures: 1})
}

// A proposal is verified before the lock: the signature of its proposer, the justify certificate and
// the timeout certificate, each exactly once. A proposal delivered again costs no verification.
func TestAProposalAndItsCertificatesAreVerifiedOutsideTheLock(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine, verifier := cluster.newCountingMachine()
	ctx := context.Background()
	blocks := cluster.chain(2, 0)

	if err := machine.OnProposal(ctx, cluster.proposal("a", blocks[0].block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	expectCounts(t, "the first proposal", verifier.take(), verifyCounts{signatures: 1})

	second := cluster.proposal("b", blocks[1].block, 1, blocks[0].qc, finality.TimeoutCert{})
	if err := machine.OnProposal(ctx, second); err != nil {
		t.Fatal(err)
	}
	expectCounts(t, "the second proposal", verifier.take(), verifyCounts{signatures: 1, aggregates: 1})

	if err := machine.OnProposal(ctx, second); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected the repeated proposal stale, got %v", err)
	}
	expectCounts(t, "the repeated proposal", verifier.take(), verifyCounts{})

	// View 2 timed out: the third block follows a timeout certificate, whose votes quote the
	// certificate of the second block. The certificate is verified, with its signature group, once.
	timeoutCert := cluster.timeoutCert(3, 2, blocks[1].qc, "a", "b", "c")
	third := cluster.proposal("c", cluster.block(3, blocks[1].hash), 3, blocks[1].qc, timeoutCert)
	if err := machine.OnProposal(ctx, third); err != nil {
		t.Fatal(err)
	}
	// The justify certificate and the high certificate of the timeout votes are one certificate.
	expectCounts(t, "the third proposal", verifier.take(), verifyCounts{signatures: 1, aggregates: 2})
}

// A timeout signature group whose aggregate fails is verified again on every attempt, and one that
// verifies is remembered: the cache never turns a rejected aggregate into an accepted one, nor the
// reverse.
func TestTheCacheRemembersOnlyGroupAggregatesThatVerified(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine, verifier := cluster.newCountingMachine()
	timeoutCert := cluster.timeoutCert(1, 1, finality.QuorumCert{}, "a", "b", "c")
	groups, err := decodeTimeoutSignatureGroups(timeoutCert.Signature)
	if err != nil || len(groups) != 1 {
		t.Fatalf("expected one signature group, got %d (%v)", len(groups), err)
	}
	genuine := groups[0]
	publicKeys := make([]types.PublicKey, 0, len(genuine.Signers))
	for _, signer := range genuine.Signers {
		member, _ := cluster.set.Get(signer)
		publicKeys = append(publicKeys, member.PublicKey)
	}
	forged := genuine
	forged.Signature = types.AggregateSignature("forged")

	for attempt := 0; attempt < 2; attempt++ {
		if verified, err := machine.verifyGroupAggregate(cluster.set, timeoutCert.Round, forged, publicKeys); err != nil || verified {
			t.Fatalf("attempt %d: expected the forged aggregate to fail, got %v (%v)", attempt, verified, err)
		}
	}
	expectCounts(t, "two attempts with a forged aggregate", verifier.take(), verifyCounts{aggregates: 2})

	for attempt := 0; attempt < 2; attempt++ {
		if verified, err := machine.verifyGroupAggregate(cluster.set, timeoutCert.Round, genuine, publicKeys); err != nil || !verified {
			t.Fatalf("attempt %d: expected the genuine aggregate to verify, got %v (%v)", attempt, verified, err)
		}
	}
	expectCounts(t, "two attempts with the genuine aggregate", verifier.take(), verifyCounts{aggregates: 1})
}

// A timeout vote is verified before the lock, with the high certificate it carries, which the votes
// of the other validators quote too.
func TestATimeoutVoteIsVerifiedOutsideTheLock(t *testing.T) {
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	machine, verifier := cluster.newCountingMachine()
	ctx := context.Background()
	machine.StartRound(1, 0)
	highQC := cluster.qc(1, 0, types.Hash{7})

	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 2, 1, highQC)); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	expectCounts(t, "the first timeout vote", verifier.take(), verifyCounts{signatures: 1, aggregates: 1})
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("b", 2, 1, highQC)); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	expectCounts(t, "the second timeout vote", verifier.take(), verifyCounts{signatures: 1})

	forged := cluster.timeoutVote("c", 2, 1, highQC)
	forged.Signature = types.Signature("forged")
	if _, err := machine.OnTimeoutVote(ctx, forged); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected the forged timeout vote rejected, got %v", err)
	}
	expectCounts(t, "the forged timeout vote", verifier.take(), verifyCounts{signatures: 2, lockedSignatures: 1})
}

// A message whose validator set cannot be resolved is not verified before the lock, and the locked
// logic judges it exactly as before: a vote of an unknown validator is refused without a
// verification, and a vote for a block whose set is pending is deferred until its set is executed,
// then verified.
func TestAMessageWhoseSetIsUnknownTakesTheLockedPath(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c")
	verifier := &countingVerifier{}
	genesis, err := cluster.registry.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:           testChainID,
		ValidatorSet:      genesis,
		Signatures:        verifier,
		Aggregator:        vexocrypto.DeterministicAggregateSigner{},
		AggregateVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier.machine = machine
	cluster.executed(machine, 1)

	// The block of height 2 names the set after block 2, which this machine has not executed.
	anchored := cluster.block(2, types.Hash{9})
	anchored.Header.ValidatorSetHeight = 3
	if err := machine.AnchorCommittedBlock(anchored); err != nil {
		t.Fatal(err)
	}
	vote := cluster.vote("a", 2, anchored.Header.Round, HashBlock(anchored))
	if err := machine.OnVote(ctx, vote); !errors.Is(err, ErrValidatorSetPending) {
		t.Fatalf("expected the vote for a pending set deferred, got %v", err)
	}
	expectCounts(t, "the vote for a pending set", verifier.take(), verifyCounts{})

	if err := machine.OnVote(ctx, withValidator(vote, "zz")); !errors.Is(err, ErrValidatorSetPending) {
		t.Fatalf("expected the set to be judged before the voter, got %v", err)
	}
	expectCounts(t, "the vote of an unknown validator for a pending set", verifier.take(), verifyCounts{})

	cluster.executed(machine, 3)
	if err := machine.OnVote(ctx, withValidator(vote, "zz")); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected the unknown validator refused, got %v", err)
	}
	expectCounts(t, "the vote of an unknown validator", verifier.take(), verifyCounts{})

	if err := machine.OnVote(ctx, vote); !errors.Is(err, ErrStaleVote) {
		t.Fatalf("expected the vote for the finalized height stale, got %v", err)
	}
	expectCounts(t, "the vote once the set is executed", verifier.take(), verifyCounts{signatures: 1})
}

// A message the locked logic rejects before it reaches a signature costs no verification before
// the lock either: a flood of malformed or unscheduled messages does not make a node pay for
// pairings.
func TestMessagesRejectedBeforeAnySignatureCostNoVerification(t *testing.T) {
	ctx := context.Background()
	cluster := newEqualTestCluster(t, "carol", "alice", "bob")
	verifier := &countingVerifier{}
	machine := cluster.newMachineWith(func(config *StateMachineConfig) {
		config.Signatures = verifier
		config.AggregateVerifier = verifier
		config.EnforceProposerSelection = true
	})
	verifier.machine = machine
	machine.StartRound(1, 0)

	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(cluster.seal(first, 0, 0))

	foreignChain := cluster.proposal("alice", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	foreignChain.Block.Header.ChainID = "another-chain"
	disagreeingViews := cluster.proposal("alice", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	disagreeingViews.Round = 1
	noJustify := cluster.proposal("alice", cluster.block(2, firstHash), 1, finality.QuorumCert{}, finality.TimeoutCert{})
	unknownProposer := cluster.proposal("alice", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	unknownProposer.Proposer = "zz"
	unscheduled := cluster.proposal("carol", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	farHeight := maxHeightWindow + 2
	farParent := types.Hash{7}
	outOfWindow := cluster.proposal("alice", cluster.block(farHeight, farParent), 1, cluster.qc(farHeight-1, 0, farParent), finality.TimeoutCert{})

	proposals := []struct {
		name     string
		proposal Proposal
		expected error
	}{
		{"another chain", foreignChain, ErrInvalidProposal},
		{"views that disagree", disagreeingViews, ErrInvalidProposal},
		{"a height without a justify certificate", noJustify, ErrInvalidProposal},
		{"an unknown proposer", unknownProposer, ErrUnknownValidator},
		{"a proposer that does not lead the view", unscheduled, ErrUnexpectedProposer},
		{"a height out of the window", outOfWindow, ErrOutOfWindow},
	}
	for _, test := range proposals {
		if err := machine.OnProposal(ctx, test.proposal); !errors.Is(err, test.expected) {
			t.Fatalf("%s: expected %v, got %v", test.name, test.expected, err)
		}
		expectCounts(t, test.name, verifier.take(), verifyCounts{})
	}

	vote := cluster.vote("alice", 1, 0, firstHash)
	missingHeight := vote
	missingHeight.Height = 0
	missingBlock := vote
	missingBlock.BlockHash = types.Hash{}
	votes := []struct {
		name     string
		vote     Vote
		expected error
	}{
		{"a vote of an unknown validator", withValidator(vote, "zz"), ErrUnknownValidator},
		{"a vote without a height", missingHeight, ErrInvalidVote},
		{"a vote without a block", missingBlock, ErrInvalidVote},
	}
	for _, test := range votes {
		if err := machine.OnVote(ctx, test.vote); !errors.Is(err, test.expected) {
			t.Fatalf("%s: expected %v, got %v", test.name, test.expected, err)
		}
		expectCounts(t, test.name, verifier.take(), verifyCounts{})
	}

	quoted := cluster.qc(1, 0, firstHash)
	timeoutVote := cluster.timeoutVote("alice", 2, 1, quoted)
	notBelow := cluster.timeoutVote("alice", 2, 0, quoted)
	noHeight := timeoutVote
	noHeight.Height = 0
	timeoutVotes := []struct {
		name     string
		vote     TimeoutVote
		expected error
	}{
		{"a timeout vote of an unknown validator", withTimeoutValidator(timeoutVote, "zz"), ErrUnknownValidator},
		{"a timeout vote without a height", noHeight, ErrInvalidVote},
		{"a timeout vote whose high certificate is not below its view", notBelow, ErrInvalidVote},
	}
	for _, test := range timeoutVotes {
		if _, err := machine.OnTimeoutVote(ctx, test.vote); !errors.Is(err, test.expected) {
			t.Fatalf("%s: expected %v, got %v", test.name, test.expected, err)
		}
		expectCounts(t, test.name, verifier.take(), verifyCounts{})
	}
}

func withTimeoutValidator(vote TimeoutVote, validatorID types.ValidatorID) TimeoutVote {
	vote.ValidatorID = validatorID
	return vote
}

// A proposal whose parent the machine does not hold is verified for its justify certificate only,
// the one signature the locked logic checks before it reports the parent unknown, and then once:
// the proposer is not authenticated before the set of the block is known. A forged certificate is
// reported as forged, not as an unknown parent.
func TestAProposalWithAnUnknownParentIsVerifiedForItsJustifyCertificateOnly(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine, verifier := cluster.newCountingMachine()
	ctx := context.Background()
	blocks := cluster.chain(2, 0)

	orphan := cluster.proposal("b", blocks[1].block, 1, blocks[0].qc, finality.TimeoutCert{})
	if err := machine.OnProposal(ctx, orphan); !errors.Is(err, ErrUnknownParent) {
		t.Fatalf("expected the unknown parent reported, got %v", err)
	}
	expectCounts(t, "the orphan", verifier.take(), verifyCounts{aggregates: 1})

	forged := orphan
	forged.JustifyQC.Signature = types.AggregateSignature("forged")
	if err := machine.OnProposal(ctx, forged); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("expected the forged certificate reported, got %v", err)
	}
	// Once before the lock and once under it: a failure is not remembered.
	expectCounts(t, "the orphan with a forged certificate", verifier.take(), verifyCounts{aggregates: 2, lockedAggregates: 1})
}

// A proposal whose validator set is pending is verified before the lock for its justify
// certificate only, the one signature the locked logic checks before it defers the proposal: the
// proposer is authenticated once the set is executed.
func TestAProposalWhoseSetIsPendingIsVerifiedForItsJustifyCertificateOnly(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c")
	leaderMachine := cluster.newMachine()
	oldSet, err := cluster.registry.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	first := cluster.propose(leaderMachine, 0, "a")
	cluster.certify(leaderMachine, first, "a", "b", "c")
	second := cluster.propose(leaderMachine, 1, "b")
	cluster.certify(leaderMachine, second, "a", "b", "c")
	third := cluster.propose(leaderMachine, 2, "c")
	thirdQC := cluster.certify(leaderMachine, third, "a", "b", "c")

	// The follower sees only the proposals, so the certificate of the third block is new to it.
	verifier := &countingVerifier{}
	follower, err := NewStateMachine(StateMachineConfig{
		ChainID:           testChainID,
		ValidatorSet:      oldSet,
		Signatures:        verifier,
		Aggregator:        vexocrypto.DeterministicAggregateSigner{},
		AggregateVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier.machine = follower
	cluster.executed(follower, 1)
	for _, proposal := range []Proposal{first, second, third} {
		if err := follower.OnProposal(ctx, proposal); err != nil {
			t.Fatal(err)
		}
	}
	verifier.take()

	// The fourth block names the set after the first block, which the follower has not executed.
	if err := cluster.registry.ApplyLeaveAt(ctx, 2, "a"); err != nil {
		t.Fatal(err)
	}
	newSet, err := cluster.registry.ValidatorSet(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	fourth := cluster.block(4, HashBlock(third.Block))
	fourth.Header.ValidatorSetHash = oldSet.Hash()
	fourth.Header.NextValidatorSetHash = newSet.Hash()
	fourth = cluster.seal(fourth, 3, thirdQC.Round)
	pending := cluster.proposal("a", fourth, 3, thirdQC, finality.TimeoutCert{})
	if err := follower.OnProposal(ctx, pending); !errors.Is(err, ErrValidatorSetPending) {
		t.Fatalf("expected the proposal deferred until its set is executed, got %v", err)
	}
	expectCounts(t, "the proposal with a pending set", verifier.take(), verifyCounts{aggregates: 1})

	cluster.executed(follower, 2)
	if err := follower.OnProposal(ctx, pending); err != nil {
		t.Fatalf("expected the proposal accepted once its set is executed, got %v", err)
	}
	expectCounts(t, "the proposal once its set is executed", verifier.take(), verifyCounts{signatures: 1})
}

// rotatingRegistry holds fixed validator sets by registry height.
type rotatingRegistry struct {
	validator.Registry
	sets map[types.Height]validator.Set
}

func (registry rotatingRegistry) ValidatorSet(_ context.Context, height types.Height) (validator.Set, error) {
	return registry.sets[height], nil
}

// A signature verified before the lock vouches for nothing the locked logic resolves differently:
// when the validator's key changes between the two, the vote is verified again against the new key
// under the lock and refused.
func TestAKeyThatChangesBeforeTheLockIsVerifiedAgain(t *testing.T) {
	ctx := context.Background()
	cluster := newEqualTestCluster(t, "a", "b", "c")
	rotated := cluster.validators("a", "b", "c")
	rotated[1].PublicKey = types.PublicKey("another key")
	registry := rotatingRegistry{sets: map[types.Height]validator.Set{
		1: validator.NewSet(cluster.validators("a", "b", "c")),
		2: validator.NewSet(rotated),
	}}
	verifier := &countingVerifier{}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:           testChainID,
		ValidatorSet:      registry.sets[1],
		Signatures:        verifier,
		Aggregator:        vexocrypto.DeterministicAggregateSigner{},
		AggregateVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier.machine = machine
	if err := machine.UpdateValidatorSetFromRegistry(ctx, registry, 1); err != nil {
		t.Fatal(err)
	}

	// The registry moves on while the vote is verified before the lock.
	moved := false
	verifier.onSignature = func() {
		if moved {
			return
		}
		moved = true
		if !machine.mu.TryLock() {
			t.Error("the first verification of the vote ran under the lock")
			return
		}
		machine.mu.Unlock()
		if err := machine.UpdateValidatorSetFromRegistry(ctx, registry, 2); err != nil {
			t.Error(err)
		}
	}
	vote := cluster.vote("b", 5, 0, types.Hash{5})
	if err := machine.OnVote(ctx, vote); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected the vote refused under the key it resolves to under the lock, got %v", err)
	}
	expectCounts(t, "the vote", verifier.take(), verifyCounts{signatures: 2, lockedSignatures: 1})
}

// Votes delivered from many goroutines are judged as they are one by one: the genuine ones count,
// the forged ones are refused, and the certificate forms.
func TestConcurrentMessagesAreJudgedAsSerialOnes(t *testing.T) {
	ids := []types.ValidatorID{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	cluster := newEqualTestCluster(t, ids...)
	machine, _ := cluster.newCountingMachine()
	ctx := context.Background()
	block := cluster.blockAt(1, types.Hash{}, 0)
	hash := HashBlock(block)
	if err := machine.OnProposal(ctx, cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	var group sync.WaitGroup
	failures := make(chan error, 1000)
	for _, id := range ids {
		group.Add(1)
		go func() {
			defer group.Done()
			genuine := cluster.vote(id, 1, 0, hash)
			forged := genuine
			forged.Signature = types.Signature("forged")
			for range 5 {
				if err := machine.OnVote(ctx, genuine); err != nil && !errors.Is(err, ErrStaleVote) {
					failures <- fmt.Errorf("genuine vote of %s: %w", id, err)
				}
				if err := machine.OnVote(ctx, forged); !errors.Is(err, ErrInvalidVote) {
					failures <- fmt.Errorf("forged vote of %s: %v", id, err)
				}
			}
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if qc, err := machine.BuildQuorumCert(1, 0, hash); err != nil {
		t.Fatalf("expected the certificate to form, got %v", err)
	} else if err := machine.VerifyQuorumCert(qc); err != nil {
		t.Fatal(err)
	}
}

// Proposals, votes and timeout votes, genuine and forged, arrive from many goroutines while others
// read the machine and change its validator set, which the race detector watches: the checks before
// the lock read nothing the lock guards. A forged message is refused whatever else is going on.
func TestMixedTrafficFromManyGoroutinesIsRaceFree(t *testing.T) {
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	machine, _ := cluster.newCountingMachine()
	ctx := context.Background()
	blocks := cluster.chain(3, 0)
	forgedQC := blocks[1].qc
	forgedQC.Signature = types.AggregateSignature("forged")
	timeoutCert := cluster.timeoutCert(3, 3, blocks[1].qc, "a", "b", "c")
	afterTimeout := cluster.block(3, blocks[1].hash)

	// Everything is built before the goroutines start: the cluster is not safe for concurrent use.
	type delivery struct {
		forged bool
		send   func() error
	}
	var deliveries []delivery
	proposal := func(forged bool, proposal Proposal) {
		deliveries = append(deliveries, delivery{forged, func() error { return machine.OnProposal(ctx, proposal) }})
	}
	proposal(false, cluster.proposal("a", blocks[0].block, 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	proposal(false, cluster.proposal("b", blocks[1].block, 1, blocks[0].qc, finality.TimeoutCert{}))
	proposal(false, cluster.proposal("c", blocks[2].block, 2, blocks[1].qc, finality.TimeoutCert{}))
	proposal(false, cluster.proposal("d", afterTimeout, 4, blocks[1].qc, timeoutCert))
	proposal(true, cluster.proposal("c", blocks[2].block, 2, forgedQC, finality.TimeoutCert{}))
	forgery := cluster.proposal("a", blocks[0].block, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	forgery.Signature = types.Signature("forged")
	proposal(true, forgery)
	for index, entry := range blocks {
		for _, voter := range cluster.ids {
			genuine := cluster.vote(voter, types.Height(index+1), entry.round, entry.hash)
			forged := genuine
			forged.Signature = types.Signature("forged")
			deliveries = append(deliveries,
				delivery{false, func() error { return machine.OnVote(ctx, genuine) }},
				delivery{true, func() error { return machine.OnVote(ctx, forged) }},
			)
		}
	}
	for _, voter := range cluster.ids {
		genuine := cluster.timeoutVote(voter, 3, 3, blocks[1].qc)
		forged := genuine
		forged.Signature = types.Signature("forged")
		quoting := cluster.timeoutVote(voter, 3, 3, forgedQC)
		deliveries = append(deliveries,
			delivery{false, func() error { _, err := machine.OnTimeoutVote(ctx, genuine); return err }},
			delivery{true, func() error { _, err := machine.OnTimeoutVote(ctx, forged); return err }},
			delivery{true, func() error { _, err := machine.OnTimeoutVote(ctx, quoting); return err }},
		)
	}

	stop := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for round := types.Round(0); ; round++ {
			select {
			case <-stop:
				return
			default:
			}
			machine.Status(ctx)
			machine.HighQC(ctx)
			_, _ = machine.ProposalLeader(round % 8)
			_ = machine.CheckBench(afterTimeout.Header)
			machine.StartRound(1, round%3)
		}
	}()

	const goroutines, passes = 8, 40
	failures := make(chan string, goroutines*passes*len(deliveries))
	var senders sync.WaitGroup
	for worker := 0; worker < goroutines; worker++ {
		senders.Add(1)
		go func() {
			defer senders.Done()
			random := rand.New(rand.NewSource(int64(worker)))
			for range passes {
				for _, index := range random.Perm(len(deliveries)) {
					if err := deliveries[index].send(); err == nil && deliveries[index].forged {
						failures <- fmt.Sprintf("delivery %d: a forged message was accepted", index)
					}
				}
			}
		}()
	}
	senders.Wait()
	close(stop)
	readers.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

// Whether the cache holds a verification never changes what the machine answers: a machine that
// remembers every verification, one that starts with them all remembered and one that remembers
// almost none return the same results, message by message, for genuine, forged, repeated and
// reordered messages alike, and a forged message is refused whatever the cache holds.
func TestResultsDoNotDependOnWhatTheCacheHolds(t *testing.T) {
	cluster := newEqualTestCluster(t, "a", "b", "c", "d")
	ctx := context.Background()
	blocks := cluster.chain(3, 0)
	forged := func(qc finality.QuorumCert) finality.QuorumCert {
		qc.Signature = types.AggregateSignature("forged")
		return qc
	}
	afterTimeout := testBlock{block: cluster.block(3, blocks[1].hash), round: 4}
	timeoutCert := cluster.timeoutCert(3, 3, blocks[1].qc, "a", "b", "c")
	forgedTimeoutCert := reencodeTimeoutCert(t, timeoutCert, func(groups []timeoutSignatureGroup) []timeoutSignatureGroup {
		groups[0].Signature = types.AggregateSignature("forged")
		return groups
	})

	type message struct {
		name string
		send func(machine *StateMachine) error
		// refusal is the error a forged message is refused with, nil for a message that is not forged.
		refusal error
	}
	var messages []message
	proposal := func(name string, refusal error, proposer types.ValidatorID, entry testBlock, justify finality.QuorumCert, timeoutCert finality.TimeoutCert) {
		messages = append(messages, message{name, func(machine *StateMachine) error {
			return machine.OnProposal(ctx, cluster.proposal(proposer, entry.block, entry.round, justify, timeoutCert))
		}, refusal})
	}
	vote := func(name string, refusal error, vote Vote) {
		messages = append(messages, message{name, func(machine *StateMachine) error { return machine.OnVote(ctx, vote) }, refusal})
	}
	timeoutVote := func(name string, refusal error, vote TimeoutVote) {
		messages = append(messages, message{name, func(machine *StateMachine) error {
			_, err := machine.OnTimeoutVote(ctx, vote)
			return err
		}, refusal})
	}
	for pass := 1; pass <= 2; pass++ {
		proposal("first proposal", nil, "a", blocks[0], finality.QuorumCert{}, finality.TimeoutCert{})
		for _, voter := range cluster.ids {
			vote("vote of "+string(voter)+" for the first block", nil, cluster.vote(voter, 1, 0, blocks[0].hash))
			forgery := cluster.vote(voter, 1, 0, blocks[0].hash)
			forgery.Signature = types.Signature("forged")
			vote("forged vote of "+string(voter), ErrInvalidVote, forgery)
		}
		proposal("second proposal with a forged certificate", ErrInvalidQuorumCert, "b", blocks[1], forged(blocks[0].qc), finality.TimeoutCert{})
		proposal("second proposal", nil, "b", blocks[1], blocks[0].qc, finality.TimeoutCert{})
		vote("vote for an unknown block", nil, cluster.vote("a", 9, 9, types.Hash{9}))
		vote("vote of an unknown validator", nil, withValidator(cluster.vote("a", 2, 1, blocks[1].hash), "zz"))
		for _, voter := range cluster.ids {
			vote("vote of "+string(voter)+" for the second block", nil, cluster.vote(voter, 2, 1, blocks[1].hash))
		}
		proposal("third proposal with a forged certificate", ErrInvalidQuorumCert, "c", blocks[2], forged(blocks[1].qc), finality.TimeoutCert{})
		proposal("third proposal", nil, "c", blocks[2], blocks[1].qc, finality.TimeoutCert{})
		for _, voter := range cluster.ids {
			timeoutVote("timeout vote of "+string(voter), nil, cluster.timeoutVote(voter, 3, 3, blocks[1].qc))
		}
		timeoutVote("timeout vote with a forged certificate", ErrInvalidQuorumCert, cluster.timeoutVote("a", 3, 3, forged(blocks[1].qc)))
		proposal("proposal after a timeout with a forged timeout certificate", ErrInvalidTimeoutCert, "d", afterTimeout, blocks[1].qc, forgedTimeoutCert)
		proposal("proposal after a timeout", nil, "d", afterTimeout, blocks[1].qc, timeoutCert)
	}

	run := func(machine *StateMachine) ([]error, string) {
		outcomes := make([]error, 0, len(messages))
		for _, message := range messages {
			outcomes = append(outcomes, message.send(machine))
		}
		return outcomes, fmt.Sprintf("%+v %+v", machine.Status(ctx), machine.HighQC(ctx))
	}
	// The machines see the same messages from the same state. The first starts with an empty cache, the
	// second with the cache the first filled, so that every verification of its is a hit, and the third
	// with a cache that forgets nearly everything.
	empty := cluster.newMachine()
	filled := cluster.newMachine()
	filled.cache = empty.cache
	forgetting := cluster.newMachine()
	forgetting.cache = newSigCache(2, 2)
	want, wantState := run(empty)
	accepted, forgeries := 0, 0
	for index, message := range messages {
		if message.refusal != nil {
			forgeries++
			if !errors.Is(want[index], message.refusal) {
				t.Fatalf("%s: expected the refusal %v, got %v", message.name, message.refusal, want[index])
			}
		}
		if want[index] == nil {
			accepted++
		}
	}
	for name, machine := range map[string]*StateMachine{"filled": filled, "forgetting": forgetting} {
		got, gotState := run(machine)
		for index, message := range messages {
			if fmt.Sprint(want[index]) != fmt.Sprint(got[index]) {
				t.Fatalf("%s: a machine with an empty cache answered %q, one with a %s cache %q", message.name, want[index], name, got[index])
			}
		}
		if wantState != gotState {
			t.Fatalf("the machines ended in different states: %s and, with a %s cache, %s", wantState, name, gotState)
		}
	}
	if accepted < 10 || forgeries < 16 {
		t.Fatalf("expected genuine and forged messages alike, got %d accepted and %d forged of %d", accepted, forgeries, len(messages))
	}
}

package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	// maxVerifiedSignatures bounds the signatures a machine remembers verifying: a view costs one
	// signature per validator and a few more, so the bound covers the traffic of many views of a large
	// validator set.
	maxVerifiedSignatures = 16384
	// maxVerifiedCertificates bounds the certificates a machine remembers verifying.
	maxVerifiedCertificates = 4096
	// maxChildBenches bounds the benched lists a machine remembers.
	maxChildBenches = 256
)

// verifiedDigest identifies one successful verification by the SHA-256 of every input its outcome
// depends on.
type verifiedDigest [sha256.Size]byte

// sigCache holds the signatures and the certificates a machine verified successfully. Verification
// is the dearest step a node takes on a message, and the same statement is judged more than once: a
// vote is verified before the machine takes its lock and again by the logic that runs under it, a
// vote that arrives before its block is verified again when it is replayed, a proposal carries a
// certificate the node usually met before, and a proposal arrives from every peer that relays it.
// The cache turns the repeat into a lookup.
//
// Only successes are remembered, each as the digest of every input the verifier saw, so a hit means
// the verifier already accepted exactly this statement. A failure is never remembered: it may
// depend on context the digest does not capture, such as the keys a verifier has registered or the
// validator set a message resolves to, and its caller rejects it. A hit or a miss so never changes
// what a verification returns, only what it costs; a machine that forgets everything returns the
// same results. The cache belongs to one machine, since a digest is only as good as the verifiers
// that produced it.
//
// It is safe for concurrent use: the machine verifies outside its lock.
type sigCache struct {
	signatures   *verifiedSet
	certificates *verifiedSet
}

func newSigCache(signatureBound, certificateBound int) *sigCache {
	return &sigCache{signatures: newVerifiedSet(signatureBound), certificates: newVerifiedSet(certificateBound)}
}

// verifiedSet is a bounded set of digests that is safe for concurrent use.
type verifiedSet struct {
	mu      sync.Mutex
	entries generations[verifiedDigest, struct{}]
}

func newVerifiedSet(bound int) *verifiedSet {
	return &verifiedSet{entries: newGenerations[verifiedDigest, struct{}](bound)}
}

func (set *verifiedSet) contains(digest verifiedDigest) bool {
	set.mu.Lock()
	defer set.mu.Unlock()

	_, found := set.entries.get(digest)
	return found
}

func (set *verifiedSet) add(digest verifiedDigest) {
	set.mu.Lock()
	defer set.mu.Unlock()

	set.entries.put(digest, struct{}{})
}

func (set *verifiedSet) size() int {
	set.mu.Lock()
	defer set.mu.Unlock()

	return set.entries.size()
}

// generations is a bounded map that forgets its oldest entries in bulk, without ordering them one by
// one. Entries go into the current generation; when it is full it becomes the previous generation
// and the one before is dropped. The map so holds at most bound entries and keeps at least the last
// half of them. An entry still in use survives a rotation: a hit in the previous generation moves it
// to the current one. It is not safe for concurrent use.
type generations[K comparable, V any] struct {
	limit    int
	current  map[K]V
	previous map[K]V
}

func newGenerations[K comparable, V any](bound int) generations[K, V] {
	return generations[K, V]{limit: max(1, bound/2), current: make(map[K]V)}
}

func (entries *generations[K, V]) get(key K) (V, bool) {
	if value, found := entries.current[key]; found {
		return value, true
	}
	value, found := entries.previous[key]
	if found {
		delete(entries.previous, key)
		entries.put(key, value)
	}
	return value, found
}

func (entries *generations[K, V]) put(key K, value V) {
	if _, found := entries.current[key]; !found && len(entries.current) >= entries.limit {
		entries.previous, entries.current = entries.current, make(map[K]V)
	}
	entries.current[key] = value
}

func (entries *generations[K, V]) size() int {
	return len(entries.current) + len(entries.previous)
}

// appendField appends data to buffer behind its length, so that no two lists of fields share an
// encoding.
func appendField(buffer []byte, data []byte) []byte {
	buffer = binary.BigEndian.AppendUint64(buffer, uint64(len(data)))
	return append(buffer, data...)
}

// signatureDigest identifies the verification of signature by publicKey over message, the domain
// message the signer signed.
func signatureDigest(publicKey types.PublicKey, message []byte, signature types.Signature) verifiedDigest {
	buffer := make([]byte, 0, 24+len(publicKey)+len(message)+len(signature))
	buffer = appendField(buffer, publicKey)
	buffer = appendField(buffer, message)
	buffer = appendField(buffer, signature)
	return sha256.Sum256(buffer)
}

// certificateDigest identifies the verification of the aggregate signature of a certificate over
// message, the domain message its signers signed. validatorSetHash stands for the keys and the
// voting powers the verification resolves the signers to, claimedPower is the voting power the
// certificate states (zero where it states none), and signers encodes the signers it names in the
// order it names them. Every one of them takes part in the verification, so a certificate that
// differs in any has another digest.
func certificateDigest(validatorSetHash types.Hash, message []byte, claimedPower types.VotingPower, signers []byte, signature types.AggregateSignature) verifiedDigest {
	buffer := make([]byte, 0, len(validatorSetHash)+32+len(message)+len(signers)+len(signature))
	buffer = append(buffer, validatorSetHash[:]...)
	buffer = appendField(buffer, message)
	buffer = binary.BigEndian.AppendUint64(buffer, uint64(claimedPower))
	buffer = appendField(buffer, signers)
	buffer = appendField(buffer, signature)
	return sha256.Sum256(buffer)
}

// signerListKey encodes the signers of a timeout signature group for certificateDigest. Unlike
// finality.EncodeSigners it is unambiguous: an id may contain the separator that one joins them
// with.
func signerListKey(signers []types.ValidatorID) []byte {
	key := make([]byte, 0, 16*len(signers))
	for _, signer := range signers {
		key = appendField(key, []byte(signer))
	}
	return key
}

// verifySignature verifies signature by publicKey over message, a domain message, unless this
// machine verified exactly that signature before. It reads only the fields NewStateMachine sets, so
// it runs with or without the machine lock.
func (machine *StateMachine) verifySignature(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	digest := signatureDigest(publicKey, message, signature)
	if machine.cache.signatures.contains(digest) {
		return true
	}
	if !machine.signatures.Verify(publicKey, message, signature) {
		return false
	}
	machine.cache.signatures.add(digest)
	return true
}

// verifyGroupAggregate verifies the aggregate signature of group, the timeout votes of its signers
// for view round, by publicKeys, the keys of the signers in validatorSet, unless this machine
// verified exactly that group before. It reads only the fields NewStateMachine sets.
func (machine *StateMachine) verifyGroupAggregate(validatorSet validator.Set, round types.Round, group timeoutSignatureGroup, publicKeys []types.PublicKey) (bool, error) {
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(TimeoutVote{
		Height: group.Height,
		Round:  round,
		HighQC: group.HighQC,
	}))
	if err != nil {
		return false, err
	}
	digest := certificateDigest(validatorSet.Hash(), message, 0, signerListKey(group.Signers), group.Signature)
	if machine.cache.certificates.contains(digest) {
		return true, nil
	}
	if !machine.aggregates.VerifyAggregate(publicKeys, message, group.Signature) {
		return false, nil
	}
	machine.cache.certificates.add(digest)
	return true, nil
}

// Verification before the lock.
//
// OnVote, OnTimeoutVote and OnProposal verify the signatures a message carries before they take the
// machine lock, so that no caller waits for another's pairing. Under the lock a message is judged
// against the state as it is then, and a signature checked here may have been checked against a
// state that has moved on, so nothing is concluded from the early check except the cache entries it
// leaves. Each preverify function resolves, briefly under the lock, the validator sets the
// signatures of the message must verify against, releases the lock, and verifies with the very
// functions the locked logic calls. Those find a signature in the cache when they resolve the same
// keys under the lock, and verify it again when the state moved to others. A signature that fails
// here leaves nothing behind and is judged, and rejected, by the locked logic as before; a message
// whose validator set cannot be resolved yet is not verified at all.
//
// A preverify function verifies a signature only where the locked logic would reach it, in the
// order the locked logic reaches them, and stops at the first that fails, so a message the locked
// logic rejects before it verifies anything costs no signature here either. The steps after the
// first lock read only the fields NewStateMachine sets and no method assigns again, and validator
// sets, which are immutable.

// preverifyVote verifies the signature of vote against the key its validator set resolves.
func (machine *StateMachine) preverifyVote(vote Vote) {
	if vote.Height == 0 || vote.BlockHash == (types.Hash{}) {
		return
	}
	machine.mu.Lock()
	validatorSet, _, err := machine.validatorSetForCert(vote.Height, vote.BlockHash)
	machine.mu.Unlock()
	if err != nil {
		return
	}
	voter, found := validatorSet.Get(vote.ValidatorID)
	if !found {
		return
	}
	_ = machine.verifyVoteSignature(vote, voter)
}

// preverifyTimeoutVote verifies the signature of vote and, if it holds, the high certificate it
// carries.
func (machine *StateMachine) preverifyTimeoutVote(vote TimeoutVote) {
	if vote.Height == 0 || vote.Height > maxChainHeight || vote.Round > maxChainView {
		return
	}
	if vote.HighQC.Height > 0 && vote.HighQC.Round >= vote.Round {
		return
	}
	machine.mu.Lock()
	voterSet := machine.validatorSetForHeight(vote.Height)
	var highQCSet validator.Set
	if vote.HighQC.Height > 0 {
		highQCSet, _ = machine.certificateSet(vote.HighQC)
	}
	machine.mu.Unlock()
	voter, found := voterSet.Get(vote.ValidatorID)
	if !found || machine.verifyTimeoutVoteSignature(vote, voter) != nil || highQCSet == nil {
		return
	}
	_ = machine.verifyQuorumCertWithSet(vote.HighQC, highQCSet)
}

// proposalPlan lists what preverifyProposal verifies, with the validator set each signature
// verifies against.
type proposalPlan struct {
	// proposer is the set of the proposer signature, nil where OnProposal does not reach it.
	proposer validator.Set
	// justify is the set of the justify certificate, nil where OnProposal does not verify it.
	// Without a proposer set OnProposal verifies it and nothing else: it cannot resolve the set of
	// the block yet.
	justify validator.Set
	// timeout holds the sets of the timeout certificate, nil where OnProposal does not reach it.
	timeout *timeoutCertSets
}

// timeoutCertSets are the validator sets the signatures of a timeout certificate verify against.
type timeoutCertSets struct {
	// signers is the set of the signers of every signature group.
	signers validator.Set
	// highQCs are the sets of the high certificates of the groups, nil for a group that carries none
	// and for one whose set cannot be resolved.
	highQCs []validator.Set
}

// planProposalVerification resolves the validator sets proposal is verified against, following the
// steps OnProposal takes to its signatures. groups are the signature groups of its timeout
// certificate, nil without one. The caller holds the machine lock.
func (machine *StateMachine) planProposalVerification(proposal Proposal, groups []timeoutSignatureGroup) proposalPlan {
	var plan proposalPlan
	header := proposal.Block.Header
	if header.Height > machine.floorHeight+maxHeightWindow {
		return plan
	}
	justify := func() validator.Set {
		if proposal.JustifyQC.Height == 0 {
			return nil
		}
		validatorSet, _ := machine.certificateSet(proposal.JustifyQC)
		return validatorSet
	}
	heights, err := machine.expectedChainHeights(header)
	if err != nil {
		if errors.Is(err, ErrUnknownParent) && header.Height > machine.floorHeight {
			// OnProposal verifies the justify certificate of a block whose parent it does not hold, then
			// reports the parent unknown.
			plan.justify = justify()
		}
		return plan
	}
	validatorSet, err := machine.validatorSetAt(heights.set)
	if err == nil && header.Height > machine.floorHeight {
		_, err = machine.validatorSetAt(heights.state + 1)
	}
	if errors.Is(err, ErrValidatorSetPending) {
		// OnProposal verifies the justify certificate of a block whose set is pending, then reports the
		// set pending.
		plan.justify = justify()
		return plan
	}
	if err != nil {
		return plan
	}
	plan.proposer = validatorSet
	plan.justify = justify()
	if len(groups) > 0 {
		plan.timeout = machine.resolveTimeoutCertSets(groups)
	}
	return plan
}

// resolveTimeoutCertSets resolves the validator sets the signature groups of a timeout certificate
// verify against, as verifyTimeoutCert does. The caller holds the machine lock.
func (machine *StateMachine) resolveTimeoutCertSets(groups []timeoutSignatureGroup) *timeoutCertSets {
	sets := &timeoutCertSets{
		signers: machine.validatorSetForHeight(highestGroupHeight(groups)),
		highQCs: make([]validator.Set, len(groups)),
	}
	for index, group := range groups {
		if group.HighQC.Height > 0 {
			sets.highQCs[index], _ = machine.certificateSet(group.HighQC)
		}
	}
	return sets
}

// preverifyProposal verifies the signature of the proposer, the justify certificate and the timeout
// certificate of proposal, the signatures OnProposal verifies, in its order.
func (machine *StateMachine) preverifyProposal(proposal Proposal) {
	if machine.checkProposalShape(proposal) != nil {
		return
	}
	var groups []timeoutSignatureGroup
	if !isZeroTimeoutCert(proposal.TimeoutCert) {
		groups, _ = decodeTimeoutSignatureGroups(proposal.TimeoutCert.Signature)
	}
	machine.mu.Lock()
	plan := machine.planProposalVerification(proposal, groups)
	machine.mu.Unlock()

	if plan.proposer != nil {
		proposer, found := plan.proposer.Get(proposal.Proposer)
		if !found || machine.validateProposer(plan.proposer, proposal.Block.Header, proposal.Proposer) != nil {
			return
		}
		if machine.verifyProposalSignature(proposal, proposer) != nil {
			return
		}
	}
	if plan.justify != nil && machine.verifyQuorumCertWithSet(proposal.JustifyQC, plan.justify) != nil {
		return
	}
	if plan.timeout != nil {
		machine.preverifyTimeoutCert(proposal.TimeoutCert, groups, plan.timeout)
	}
}

// preverifyTimeoutCert verifies the high certificate and the aggregate signature of each signature
// group of timeoutCert, as verifyTimeoutCert does, and stops at the first that fails.
func (machine *StateMachine) preverifyTimeoutCert(timeoutCert finality.TimeoutCert, groups []timeoutSignatureGroup, sets *timeoutCertSets) {
	if timeoutCert.Height == 0 || timeoutCert.Height > maxChainHeight || timeoutCert.Round > maxChainView || timeoutCert.Height != highestGroupHeight(groups) {
		return
	}
	for index, group := range groups {
		if group.Height == 0 || group.Height > maxChainHeight || len(group.Signers) == 0 {
			return
		}
		if group.HighQC.Height > 0 {
			if group.HighQC.Round >= timeoutCert.Round || sets.highQCs[index] == nil {
				return
			}
			if machine.verifyQuorumCertWithSet(group.HighQC, sets.highQCs[index]) != nil {
				return
			}
		} else if !isZeroQC(group.HighQC) {
			return
		}
		publicKeys := make([]types.PublicKey, 0, len(group.Signers))
		for _, signer := range group.Signers {
			member, found := sets.signers.Get(signer)
			if !found {
				return
			}
			publicKeys = append(publicKeys, member.PublicKey)
		}
		if verified, err := machine.verifyGroupAggregate(sets.signers, timeoutCert.Round, group, publicKeys); err != nil || !verified {
			return
		}
	}
}

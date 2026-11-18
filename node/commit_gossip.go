package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidCommitMessage = errors.New("invalid commit message")

// commitMessage announces a final block, or asks for one. A node announces each block it commits
// without its transactions (Bodiless): its peers hold the block already from its proposal, and a
// peer that does not asks the announcer for it (Want), which answers with the whole block.
// Announcing whole blocks would make every validator upload every block to every peer, which
// alone would bound the throughput by a validator's bandwidth.
//
// The finality proof proves Block itself or a descendant of it: a block whose child was certified
// only after a timeout is final as an ancestor of a later block that the 2-chain rule finalizes
// directly, and has no proof of its own. Linkage then holds the headers strictly between Block and
// the proven block, which chain Block to it by hash; it is empty when the proven block is the child.
type commitMessage struct {
	Block         types.Block
	QuorumCert    finality.QuorumCert
	FinalityProof finality.Proof
	Linkage       []types.Header
	Bodiless      bool
	Want          types.Height
}

// maxCommitLinkage bounds the headers that link a block to the descendant its finality proof
// proves: the views that time out in a row before a block is finalized directly again.
const maxCommitLinkage = 64

// A commit message is a kind byte and the fields of its kind in the canonical binary encoding of
// package types, so a message has exactly one encoding and the decoder refuses any other. A block
// message is the Bodiless flag, the block, its quorum certificate, its finality proof and the
// linkage headers. A request is the height it Wants and nothing else.
const (
	commitKindBlock byte = iota + 1
	commitKindWant
)

// commitSlack is room the encoder adds to the length of the block for the certificates and the
// proof behind it. A message that outgrows it costs a reallocation, never correctness.
const commitSlack = 4096

const (
	// recentCommits bounds the whole commits a node keeps in memory to serve the peers that ask,
	// maxCommitServes how often it serves one height to one peer, and maxTrackedServes the
	// (peer, height) pairs it counts serves of.
	recentCommits    = 8
	maxCommitServes  = 2
	maxTrackedServes = 4096
)

// commitCatchUpDelay is how long a node stays at one commit height while its peers announce higher
// ones before it fetches the next block from them. A node that is only a block behind, because it
// executes a little later than its peers, commits that block through consensus first; one that
// missed a block for good would otherwise wait forever, since every validator announces each commit
// once.
const commitCatchUpDelay = time.Second

// servedCommit is a whole commit message this node announced.
type servedCommit struct {
	height types.Height
	data   []byte
}

// commitServe is one height served to one peer.
type commitServe struct {
	peer   p2p.PeerID
	height types.Height
}

// commitServer keeps the recent whole commits of a node and how often it served each height to
// each peer, when it last asked for a block it misses (every validator announces each commit, and
// one copy is enough), and how far ahead its peers are.
type commitServer struct {
	mu      sync.Mutex
	commits []servedCommit
	served  map[commitServe]int
	wanted  types.Height
	asked   time.Time

	// ahead is the highest height a peer announced above the next height this node commits, and
	// aheadPeer the last peer that announced a height above it. lagging is the next height the node
	// commits since lagSince, when it first learned that peers are ahead of it.
	ahead     types.Height
	aheadPeer p2p.PeerID
	lagging   types.Height
	lagSince  time.Time
}

// commitBodyRetry is how long a node waits for a body it asked for before it asks the next
// announcer.
const commitBodyRetry = time.Second

// shouldAsk reports whether to ask for the body of height now.
func (server *commitServer) shouldAsk(height types.Height) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	now := time.Now()
	if server.wanted == height && now.Sub(server.asked) < commitBodyRetry {
		return false
	}
	server.wanted, server.asked = height, now
	return true
}

func (server *commitServer) keep(height types.Height, data []byte) {
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, commit := range server.commits {
		if commit.height == height {
			return
		}
	}
	if len(server.commits) == recentCommits {
		copy(server.commits, server.commits[1:])
		server.commits = server.commits[:recentCommits-1]
	}
	server.commits = append(server.commits, servedCommit{height: height, data: data})
}

// recent returns the whole commit of height the node keeps in memory.
func (server *commitServer) recent(height types.Height) ([]byte, bool) {
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, commit := range server.commits {
		if commit.height == height {
			return commit.data, true
		}
	}
	return nil, false
}

// admitServe counts a serve of height to peer and reports whether the node may serve it: a peer
// gets each height at most maxCommitServes times among the last maxTrackedServes pairs served.
func (server *commitServer) admitServe(peer p2p.PeerID, height types.Height) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	key := commitServe{peer: peer, height: height}
	if server.served[key] >= maxCommitServes {
		return false
	}
	if server.served == nil || len(server.served) >= maxTrackedServes {
		server.served = make(map[commitServe]int)
	}
	server.served[key]++
	return true
}

// observeAhead records that peer announced height, above next, the height this node commits next,
// and reports whether the node has stayed at next for commitCatchUpDelay since it first learned
// that its peers are ahead.
func (server *commitServer) observeAhead(peer p2p.PeerID, next types.Height, height types.Height, now time.Time) bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.ahead = max(server.ahead, height)
	server.aheadPeer = peer
	if server.lagging != next {
		server.lagging, server.lagSince = next, now
		return false
	}
	return now.Sub(server.lagSince) >= commitCatchUpDelay
}

// aheadOf returns the peer that last announced a height above committed, false when no peer did.
func (server *commitServer) aheadOf(committed types.Height) (p2p.PeerID, bool) {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.aheadPeer, server.ahead > committed && server.aheadPeer != ""
}

// appendCommitMessage appends the encoding of message to dst. A message that wants a height is a
// request and carries nothing else.
func appendCommitMessage(dst []byte, message commitMessage) []byte {
	if message.Want != 0 {
		return binary.AppendUvarint(append(dst, commitKindWant), uint64(message.Want))
	}
	dst = slices.Grow(dst, 2+types.BlockSize(message.Block)+commitSlack)
	dst = append(dst, commitKindBlock, 0)
	if message.Bodiless {
		dst[len(dst)-1] = 1
	}
	dst = types.AppendBlock(dst, message.Block)
	dst = finality.AppendQuorumCert(dst, message.QuorumCert)
	dst = finality.AppendProof(dst, message.FinalityProof)
	dst = binary.AppendUvarint(dst, uint64(len(message.Linkage)))
	for _, header := range message.Linkage {
		dst = types.AppendHeader(dst, header)
	}
	return dst
}

func decodeCommitMessage(data []byte) (commitMessage, error) {
	reader := types.NewReader(data)
	var message commitMessage
	switch reader.Byte() {
	case commitKindBlock:
		message.Bodiless = reader.Bool()
		message.Block = reader.Block(consensus.MaxBlockBytes)
		message.QuorumCert = finality.ReadQuorumCert(reader)
		message.FinalityProof = finality.ReadProof(reader)
		if count := reader.Uvarint(); count > maxCommitLinkage {
			reader.Fail("commit linkage too long")
		} else if count > 0 {
			message.Linkage = make([]types.Header, 0, count)
			for range count {
				message.Linkage = append(message.Linkage, reader.Header())
			}
		}
	case commitKindWant:
		message.Want = types.Height(reader.Uvarint())
	default:
		reader.Fail("unknown commit message kind")
	}
	if err := reader.Done(); err != nil {
		return commitMessage{}, fmt.Errorf("%w: %w", ErrInvalidCommitMessage, err)
	}
	if message.Want == 0 && (message.Block.Header.Height == 0 || message.QuorumCert.Height == 0) {
		// A request names a height, and a block message the block and the certificate of one.
		return commitMessage{}, ErrInvalidCommitMessage
	}
	return message, nil
}

func (node *Node) startCommitGossip(ctx context.Context) error {
	if node.wire == nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	events, err := node.wire.Subscribe(runCtx, p2p.TopicCommit)
	if err != nil {
		cancel()
		return err
	}
	done := make(chan struct{})
	node.commitCancel = cancel
	node.commitDone = done
	go node.consumeCommitGossip(runCtx, events, done)
	return nil
}

func (node *Node) consumeCommitGossip(ctx context.Context, events <-chan transport.Envelope, done chan struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			node.acceptCommitMessage(ctx, envelope.From, envelope.Data)
		}
	}
}

// acceptCommitMessage applies a commit gossiped by a peer and scores the peer.
//
// A message the node cannot judge yet, or no longer needs, is a race and counts
// as valid: the block is already committed here, or it is ahead of this node.
// A message for the next height must carry a finality proof that verifies in
// full; every way in which it fails to verify counts as invalid, because the
// sender vouched for a block it could not have finalized. Only a failure of
// the node itself to run the verification (a stopped node, a cancelled
// context, a missing signature verifier) leaves the peer unscored.
func (node *Node) acceptCommitMessage(ctx context.Context, from p2p.PeerID, data []byte) {
	if !node.admitPeerMessage(ctx, from) {
		return
	}
	message, err := decodeCommitMessage(data)
	if err != nil {
		node.observePeerMessage(ctx, from, false)
		return
	}
	if message.Want != 0 {
		node.serveCommit(ctx, from, message.Want)
		node.observePeerMessage(ctx, from, true)
		return
	}
	commitHeight, ok := node.currentCommitHeight(ctx)
	if ok && message.Block.Header.Height > commitHeight+1 {
		// The peer is ahead. The node commits the next block through consensus as a rule, and asks
		// the peer for it only once it stayed behind for a while: it may never receive the block
		// otherwise.
		if node.commits.observeAhead(from, commitHeight+1, message.Block.Header.Height, time.Now()) {
			node.askCommit(ctx, from, commitHeight+1)
		}
		node.observePeerMessage(ctx, from, true)
		return
	}
	if !ok || message.Block.Header.Height <= commitHeight {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if message.Bodiless {
		if len(message.Block.Txs) != 0 || len(message.Linkage) != 0 {
			node.observePeerMessage(ctx, from, false)
			return
		}
		proposal, held := node.pendingProposals()[message.FinalityProof.BlockHash]
		if !held || consensus.HashHeader(proposal.Block.Header) != consensus.HashHeader(message.Block.Header) {
			// The node misses the block: the announcement must prove it final before the node asks
			// the announcer for it.
			node.requestCommitBody(ctx, from, message)
			return
		}
		message.Block = proposal.Block
	}
	if err := node.verifyCommitMessage(ctx, message); err != nil {
		if !commitVerificationUnavailable(err) {
			node.observePeerMessage(ctx, from, false)
		}
		return
	}
	_, err = node.commitBlock(ctx, message.Block, message.QuorumCert, false, false)
	if err != nil && !errors.Is(err, ErrBlockAlreadyCommitted) && !errors.Is(err, ErrNonSequentialCommit) {
		node.logEvent("commit_gossip_apply_failed", map[string]any{
			"height": message.Block.Header.Height,
			"error":  err.Error(),
		})
	}
	node.observePeerMessage(ctx, from, true)
	if err == nil {
		// A node that commits from gossip while its peers are further ahead is catching up: it asks
		// for the next block at once rather than waiting for the next announcement.
		if peer, behind := node.commits.aheadOf(message.Block.Header.Height); behind {
			node.askCommit(ctx, peer, message.Block.Header.Height+1)
		}
	}
}

// askCommit asks peer for the whole commit of height, unless the node asked for it within
// commitBodyRetry.
func (node *Node) askCommit(ctx context.Context, peer p2p.PeerID, height types.Height) {
	wire, running := node.runningTransport()
	if !running || !node.commits.shouldAsk(height) {
		return
	}
	_ = wire.Send(ctx, peer, p2p.TopicCommit, appendCommitMessage(nil, commitMessage{Want: height}))
}

// serveCommit answers a peer that wants the whole commit of height: from the recent commits the node
// keeps, or else from its store, which holds every committed block it retains with its certificate
// and finality proof. The peer verifies the commit like any other.
func (node *Node) serveCommit(ctx context.Context, peer p2p.PeerID, height types.Height) {
	wire, running := node.runningTransport()
	if !running || !node.commits.admitServe(peer, height) {
		return
	}
	data, ok := node.commits.recent(height)
	if !ok {
		if data, ok = node.storedCommit(ctx, height); !ok {
			return
		}
	}
	_ = wire.Send(ctx, peer, p2p.TopicCommit, data)
}

// storedCommit encodes the whole commit of height from the store: the block, its certificate and
// the finality proof of the block or, when its child was certified only after a timeout, of the
// first descendant the 2-chain rule finalizes directly, with the headers that link the two. A
// proof is completed by the child of the proven block, which the node executed or holds certified.
// It reports false when the node lacks any of them.
func (node *Node) storedCommit(ctx context.Context, height types.Height) ([]byte, bool) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, false
	}
	record, err := runtime.BlockByHeight(ctx, height)
	if err != nil || record.QuorumCert == nil {
		return nil, false
	}
	message := commitMessage{Block: record.Block, QuorumCert: finalityQuorumCertFromRecord(*record.QuorumCert)}
	proven := record
	for {
		proof, err := node.buildCommitFinalityProof(ctx, runtime, proven.Block, finalityQuorumCertFromRecord(*proven.QuorumCert))
		if err == nil {
			message.FinalityProof = proof
			return appendCommitMessage(nil, message), true
		}
		if !errors.Is(err, ErrFinalityNotFound) || len(message.Linkage) == maxCommitLinkage {
			return nil, false
		}
		if proven.Block.Header.Height != height {
			message.Linkage = append(message.Linkage, proven.Block.Header)
		}
		next, err := runtime.BlockByHeight(ctx, proven.Block.Header.Height+1)
		if err != nil || next.QuorumCert == nil || next.Block.Header.PreviousBlockHash != proven.Hash {
			return nil, false
		}
		proven = next
	}
}

func (node *Node) broadcastCommit(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, proof finality.Proof) error {
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	if proof.Header.Height == 0 || !proof.HasCommitProof() {
		return nil
	}
	data := appendCommitMessage(nil, commitMessage{Block: block, QuorumCert: quorumCert, FinalityProof: proof})
	if len(block.Txs) > 0 {
		node.commits.keep(block.Header.Height, data)
		bodiless := block
		bodiless.Txs = nil
		data = appendCommitMessage(nil, commitMessage{Block: bodiless, QuorumCert: quorumCert, FinalityProof: proof, Bodiless: true})
	}
	return wire.Publish(ctx, p2p.TopicCommit, data)
}

// requestCommitBody verifies the finality proof of a bodiless announcement and asks the announcer
// for the whole block. A proof that does not verify counts against the announcer.
func (node *Node) requestCommitBody(ctx context.Context, from p2p.PeerID, message commitMessage) {
	proof := message.FinalityProof
	err := error(nil)
	switch {
	case consensus.HashHeader(proof.Header) != consensus.HashHeader(message.Block.Header) || !sameQuorumCert(proof.QuorumCert, message.QuorumCert):
		err = fmt.Errorf("%w: proof does not describe the announced block", ErrInvalidCommitQC)
	default:
		var verifier finality.RegistryVerifier
		if verifier, err = node.commitVerifier(); err == nil {
			err = verifier.VerifyFinalityProofWithContext(ctx, proof)
		}
	}
	if err != nil {
		if !commitVerificationUnavailable(err) {
			node.observePeerMessage(ctx, from, false)
		}
		return
	}
	node.observePeerMessage(ctx, from, true)
	node.askCommit(ctx, from, message.Block.Header.Height)
}

// errCommitVerifierUnavailable marks a failure to set up the verification of a
// commit, as opposed to a commit that failed it.
var errCommitVerifierUnavailable = errors.New("commit verifier unavailable")

// commitVerificationUnavailable reports whether err means the node could not
// run the verification, so the sender of the commit is not at fault.
func commitVerificationUnavailable(err error) bool {
	return errors.Is(err, errCommitVerifierUnavailable) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, finality.ErrMissingSignatureVerifier) ||
		errors.Is(err, consensus.ErrValidatorSetPending)
}

// verifyCommitCertificate checks that quorumCert is a quorum certificate of the
// validator set the block's header names, for exactly this block.
func (node *Node) verifyCommitCertificate(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) error {
	if quorumCert.Height != block.Header.Height {
		return ErrInvalidCommitQC
	}
	blockHash := consensus.HashBlock(block)
	if quorumCert.BlockHash != blockHash {
		return ErrInvalidCommitQC
	}
	verifier, err := node.commitVerifier()
	if err != nil {
		return err
	}
	return verifier.VerifyBlockCertWithContext(ctx, block.Header, quorumCert)
}

// verifyCommitMessage checks that the block of a gossiped commit is final: by its own finality
// proof, or as an ancestor of the block the proof proves, linked to it by the linkage headers.
func (node *Node) verifyCommitMessage(ctx context.Context, message commitMessage) error {
	if message.FinalityProof.Header.Height == message.Block.Header.Height && len(message.Linkage) == 0 {
		return node.verifyCommitFinalityProof(ctx, message.Block, message.QuorumCert, message.FinalityProof)
	}
	return node.verifyLinkedCommit(ctx, message.Block, message.QuorumCert, message.Linkage, message.FinalityProof)
}

// verifyLinkedCommit checks that block is final as an ancestor of the block proof proves: the body
// matches the header, quorumCert certifies block, and linkage chains block by hash, one height at a
// time, to the parent of the proven block, whose finality proof verifies in full. A final block
// finalizes its ancestors, so block is final although no proof of its own exists. The certificate
// is checked on its own, since the proof does not cover it and execution records it.
func (node *Node) verifyLinkedCommit(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, linkage []types.Header, proof finality.Proof) error {
	if err := consensus.VerifyBlockBody(block); err != nil {
		return err
	}
	parent, height := consensus.HashBlock(block), block.Header.Height
	for _, header := range linkage {
		if header.Height != height+1 || header.PreviousBlockHash != parent || header.ChainID != block.Header.ChainID {
			return fmt.Errorf("%w: linkage header at height %d does not extend height %d", ErrInvalidCommitQC, header.Height, height)
		}
		parent, height = consensus.HashHeader(header), header.Height
	}
	if proof.Header.Height != height+1 || proof.Header.PreviousBlockHash != parent || proof.BlockHash != consensus.HashHeader(proof.Header) {
		return fmt.Errorf("%w: the proven block does not extend the linkage", ErrInvalidCommitQC)
	}
	if proof.ValidatorSetHeight != proof.Header.ValidatorSetHeight {
		return fmt.Errorf("%w: validator set height %d for a block naming %d", finality.ErrHeightMismatch, proof.ValidatorSetHeight, proof.Header.ValidatorSetHeight)
	}
	if err := node.verifyCommitCertificate(ctx, block, quorumCert); err != nil {
		return err
	}
	verifier, err := node.commitVerifier()
	if err != nil {
		return err
	}
	return verifier.VerifyFinalityProofWithContext(ctx, proof)
}

// verifyCommitFinalityProof checks that a gossiped commit is final under the
// 2-chain rule: block is the proven block B, quorumCert is QC(B), and the proof
// carries the child C of B with QC(C) formed in the view after QC(B). The
// proof must describe exactly the transmitted block, and the block body must
// match its header, because a quorum certificate vouches for the header hash
// and the transactions are only bound to it through the body checks. The
// validator sets are the ones the headers name, which the quorums certified;
// the proof does not choose them.
func (node *Node) verifyCommitFinalityProof(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, proof finality.Proof) error {
	blockHash := consensus.HashBlock(block)
	if consensus.HashHeader(proof.Header) != consensus.HashHeader(block.Header) || proof.BlockHash != blockHash {
		return fmt.Errorf("%w: proof does not describe the committed block", ErrInvalidCommitQC)
	}
	if !sameQuorumCert(proof.QuorumCert, quorumCert) {
		return fmt.Errorf("%w: proof certificate differs from the commit certificate", ErrInvalidCommitQC)
	}
	if proof.ValidatorSetHeight != block.Header.ValidatorSetHeight {
		return fmt.Errorf("%w: validator set height %d for a block naming %d", finality.ErrHeightMismatch, proof.ValidatorSetHeight, block.Header.ValidatorSetHeight)
	}
	if err := consensus.VerifyBlockBody(block); err != nil {
		return err
	}
	verifier, err := node.commitVerifier()
	if err != nil {
		return err
	}
	return verifier.VerifyFinalityProofWithContext(ctx, proof)
}

// commitVerifier returns the verifier that checks each certified header against the validator
// set the header names.
func (node *Node) commitVerifier() (finality.RegistryVerifier, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return finality.RegistryVerifier{}, fmt.Errorf("%w: %w", errCommitVerifierUnavailable, err)
	}
	return runtime.NewFinalityVerifier(), nil
}

func sameQuorumCert(left finality.QuorumCert, right finality.QuorumCert) bool {
	return left.Height == right.Height &&
		left.Round == right.Round &&
		left.BlockHash == right.BlockHash &&
		left.VotingPower == right.VotingPower &&
		bytes.Equal(left.Signers, right.Signers) &&
		bytes.Equal(left.Signature, right.Signature)
}

func (node *Node) hasCommitted(ctx context.Context, height types.Height) bool {
	commitHeight, ok := node.currentCommitHeight(ctx)
	return ok && commitHeight >= height
}

func (node *Node) currentCommitHeight(ctx context.Context) (types.Height, bool) {
	runtime, err := node.Runtime()
	if err != nil {
		return 0, false
	}
	commit, err := runtime.App.Commit()
	if err != nil {
		return 0, false
	}
	return commit.Height, true
}

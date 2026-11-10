package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
type commitMessage struct {
	Block         types.Block         `json:"block"`
	QuorumCert    finality.QuorumCert `json:"quorum_cert"`
	FinalityProof finality.Proof      `json:"finality_proof,omitempty"`
	Bodiless      bool                `json:"bodiless,omitempty"`
	Want          types.Height        `json:"want,omitempty"`
}

const (
	// recentCommits bounds the whole commits a node keeps to serve the peers that ask, and
	// maxCommitServes how often it serves one to one peer.
	recentCommits   = 8
	maxCommitServes = 2
)

// servedCommit is a whole commit message this node announced, with how often it served it.
type servedCommit struct {
	height types.Height
	data   []byte
	served map[p2p.PeerID]int
}

// commitServer keeps the recent whole commits of a node, and when it last asked for the body of a
// height it misses: every validator announces each commit, and one body is enough.
type commitServer struct {
	mu      sync.Mutex
	commits []servedCommit
	wanted  types.Height
	asked   time.Time
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
	server.commits = append(server.commits, servedCommit{height: height, data: data, served: make(map[p2p.PeerID]int)})
}

// serve returns the whole commit of height for peer, false when the node holds none or served it
// to peer as often as it may.
func (server *commitServer) serve(peer p2p.PeerID, height types.Height) ([]byte, bool) {
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, commit := range server.commits {
		if commit.height != height {
			continue
		}
		if commit.served[peer] >= maxCommitServes {
			return nil, false
		}
		commit.served[peer]++
		return commit.data, true
	}
	return nil, false
}

func encodeCommitMessage(block types.Block, quorumCert finality.QuorumCert, proofs ...finality.Proof) ([]byte, error) {
	var proof finality.Proof
	if len(proofs) > 0 {
		proof = proofs[0]
	}
	return json.Marshal(commitMessage{
		Block:         block,
		QuorumCert:    quorumCert,
		FinalityProof: proof,
	})
}

func decodeCommitMessage(data []byte) (commitMessage, error) {
	var message commitMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return commitMessage{}, err
	}
	if message.Want != 0 {
		// A request names the height it wants and nothing else.
		if message.Block.Header.Height != 0 || message.QuorumCert.Height != 0 || message.Bodiless {
			return commitMessage{}, ErrInvalidCommitMessage
		}
		return message, nil
	}
	if message.Block.Header.Height == 0 || message.QuorumCert.Height == 0 {
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
		if whole, ok := node.commits.serve(from, message.Want); ok {
			if wire, running := node.runningTransport(); running {
				_ = wire.Send(ctx, from, p2p.TopicCommit, whole)
			}
		}
		node.observePeerMessage(ctx, from, true)
		return
	}
	commitHeight, ok := node.currentCommitHeight(ctx)
	if !ok || message.Block.Header.Height <= commitHeight || message.Block.Header.Height > commitHeight+1 {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if message.Bodiless {
		if len(message.Block.Txs) != 0 {
			node.observePeerMessage(ctx, from, false)
			return
		}
		proposal, held := node.pendingProposals()[message.FinalityProof.BlockHash]
		if !held || proposal.Block.Header != message.Block.Header {
			// The node misses the block: the announcement must prove it final before the node asks
			// the announcer for it.
			node.requestCommitBody(ctx, from, message)
			return
		}
		message.Block = proposal.Block
	}
	if err := node.verifyCommitFinalityProof(ctx, message.Block, message.QuorumCert, message.FinalityProof); err != nil {
		if !commitVerificationUnavailable(err) {
			node.observePeerMessage(ctx, from, false)
		}
		return
	}
	if _, err := node.commitBlock(ctx, message.Block, message.QuorumCert, false, false); err != nil &&
		!errors.Is(err, ErrBlockAlreadyCommitted) && !errors.Is(err, ErrNonSequentialCommit) {
		node.logEvent("commit_gossip_apply_failed", map[string]any{
			"height": message.Block.Header.Height,
			"error":  err.Error(),
		})
	}
	node.observePeerMessage(ctx, from, true)
}

func (node *Node) broadcastCommit(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, proof finality.Proof) error {
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	if proof.Header.Height == 0 || !proof.HasCommitProof() {
		return nil
	}
	data, err := encodeCommitMessage(block, quorumCert, proof)
	if err != nil {
		return err
	}
	if len(block.Txs) > 0 {
		node.commits.keep(block.Header.Height, data)
		bodiless := block
		bodiless.Txs = nil
		data, err = json.Marshal(commitMessage{Block: bodiless, QuorumCert: quorumCert, FinalityProof: proof, Bodiless: true})
		if err != nil {
			return err
		}
	}
	return wire.Publish(ctx, p2p.TopicCommit, data)
}

// requestCommitBody verifies the finality proof of a bodiless announcement and asks the announcer
// for the whole block. A proof that does not verify counts against the announcer.
func (node *Node) requestCommitBody(ctx context.Context, from p2p.PeerID, message commitMessage) {
	proof := message.FinalityProof
	err := error(nil)
	switch {
	case proof.Header != message.Block.Header || !sameQuorumCert(proof.QuorumCert, message.QuorumCert):
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
	wire, running := node.runningTransport()
	if !running || !node.commits.shouldAsk(message.Block.Header.Height) {
		return
	}
	if request, err := json.Marshal(commitMessage{Want: message.Block.Header.Height}); err == nil {
		_ = wire.Send(ctx, from, p2p.TopicCommit, request)
	}
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
	if proof.Header != block.Header || proof.BlockHash != blockHash {
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

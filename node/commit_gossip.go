package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidCommitMessage = errors.New("invalid commit message")

type commitMessage struct {
	Block         types.Block         `json:"block"`
	QuorumCert    finality.QuorumCert `json:"quorum_cert"`
	FinalityProof finality.Proof      `json:"finality_proof,omitempty"`
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
	commitHeight, ok := node.currentCommitHeight(ctx)
	if !ok || message.Block.Header.Height <= commitHeight || message.Block.Header.Height > commitHeight+1 {
		node.observePeerMessage(ctx, from, true)
		return
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
	return wire.Publish(ctx, p2p.TopicCommit, data)
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

package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

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
	if !ok {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if message.Block.Header.Height <= commitHeight {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if message.Block.Header.Height > commitHeight+1 {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if message.FinalityProof.Header.Height == 0 {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if err := node.verifyCommitFinalityProof(ctx, message.Block, message.QuorumCert, message.FinalityProof); err != nil {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if _, err := node.commitBlock(ctx, message.Block, message.QuorumCert, false, false); err != nil && !errors.Is(err, ErrBlockAlreadyCommitted) {
		node.observePeerMessage(ctx, from, true)
		return
	}
	node.observePeerMessage(ctx, from, true)
}

func (node *Node) broadcastCommit(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, proof finality.Proof) error {
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	if proof.Header.Height == 0 || !proof.HasThreeChainCommitProof() {
		return nil
	}
	data, err := encodeCommitMessage(block, quorumCert, proof)
	if err != nil {
		return err
	}
	return wire.Publish(ctx, p2p.TopicCommit, data)
}

func (node *Node) verifyCommitCertificate(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) error {
	if quorumCert.Height != block.Header.Height {
		return ErrInvalidCommitQC
	}
	blockHash := consensus.HashBlock(block)
	if quorumCert.BlockHash != blockHash {
		return ErrInvalidCommitQC
	}
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	verifier, err := runtime.NewFinalityVerifier(ctx, block.Header.Height)
	if err != nil {
		return err
	}
	proof := finality.NewProof(block.Header, quorumCert)
	proof.BlockHash = blockHash
	return verifier.VerifyFinalityProofWithContext(ctx, proof)
}

func (node *Node) verifyCommitFinalityProof(ctx context.Context, block types.Block, quorumCert finality.QuorumCert, proof finality.Proof) error {
	blockHash := consensus.HashBlock(block)
	if proof.Header.Height != block.Header.Height || proof.BlockHash != blockHash {
		return ErrInvalidCommitQC
	}
	if proof.Header != block.Header {
		return ErrInvalidCommitQC
	}
	if !sameQuorumCert(proof.QuorumCert, quorumCert) {
		return ErrInvalidCommitQC
	}
	if !proof.HasThreeChainCommitProof() {
		return finality.ErrCommitChainTooShort
	}
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	validatorSetHeight := proof.ValidatorSetHeight
	if validatorSetHeight == 0 {
		validatorSetHeight = block.Header.Height
	}
	verifier, err := runtime.NewFinalityVerifier(ctx, validatorSetHeight)
	if err != nil {
		return err
	}
	return verifier.VerifyFinalityProofWithContext(ctx, proof)
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

package node

import (
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
	Block      types.Block         `json:"block"`
	QuorumCert finality.QuorumCert `json:"quorum_cert"`
}

func encodeCommitMessage(block types.Block, quorumCert finality.QuorumCert) ([]byte, error) {
	return json.Marshal(commitMessage{
		Block:      block,
		QuorumCert: quorumCert,
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
	node.commitCancel = cancel
	go node.consumeCommitGossip(runCtx, events)
	return nil
}

func (node *Node) consumeCommitGossip(ctx context.Context, events <-chan transport.Envelope) {
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
	if node.hasCommitted(ctx, message.Block.Header.Height) {
		node.observePeerMessage(ctx, from, true)
		return
	}
	if err := node.verifyCommitCertificate(ctx, message.Block, message.QuorumCert); err != nil {
		node.observePeerMessage(ctx, from, false)
		return
	}
	if _, err := node.commitBlock(ctx, message.Block, message.QuorumCert, false, false); err != nil {
		node.observePeerMessage(ctx, from, false)
		return
	}
	node.observePeerMessage(ctx, from, true)
}

func (node *Node) broadcastCommit(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) error {
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	data, err := encodeCommitMessage(block, quorumCert)
	if err != nil {
		return err
	}
	return wire.Publish(ctx, p2p.TopicCommit, data)
}

func (node *Node) verifyCommitCertificate(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) error {
	if quorumCert.Height != block.Header.Height {
		return ErrInvalidCommitQC
	}
	if quorumCert.BlockHash != consensus.HashBlock(block) {
		return ErrInvalidCommitQC
	}
	if len(quorumCert.Signature) == 0 {
		return finality.ErrMissingQCSignature
	}
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, block.Header.Height)
	if err != nil {
		return err
	}
	signers, err := finality.ParseSigners(quorumCert.Signers)
	if err != nil {
		return err
	}
	var votingPower types.VotingPower
	for _, signer := range signers {
		validatorInfo, found := validatorSet.Get(signer)
		if !found {
			return finality.ErrUnknownSigner
		}
		votingPower += validatorInfo.VotingPower
	}
	if !finality.HasQuorum(votingPower, validatorSet.TotalVotingPower()) {
		return finality.ErrInsufficientQuorum
	}
	return nil
}

func (node *Node) hasCommitted(ctx context.Context, height types.Height) bool {
	runtime, err := node.Runtime()
	if err != nil {
		return false
	}
	commit, err := runtime.App.Commit()
	if err != nil {
		return false
	}
	return commit.Height >= height
}

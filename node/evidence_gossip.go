package node

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
)

func encodeEvidenceMessage(evidence slashing.Evidence) ([]byte, error) {
	return json.Marshal(evidence)
}

func decodeEvidenceMessage(data []byte) (slashing.Evidence, error) {
	var evidence slashing.Evidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return slashing.Evidence{}, err
	}
	return evidence, nil
}

func (node *Node) SubmitEvidence(ctx context.Context, evidence slashing.Evidence) (consensus.SlashResult, bool, error) {
	result, applied, err := node.applyEvidence(ctx, evidence)
	if err != nil {
		return consensus.SlashResult{}, false, err
	}
	if !applied {
		return result, false, nil
	}
	if err := node.broadcastEvidence(ctx, evidence); err != nil {
		return consensus.SlashResult{}, false, err
	}
	return result, true, nil
}

func (node *Node) applyEvidence(ctx context.Context, evidence slashing.Evidence) (consensus.SlashResult, bool, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return consensus.SlashResult{}, false, err
	}
	result, err := consensus.SubmitEvidenceForSlashing(ctx, runtime.Slashing, runtime.Validators, evidence)
	if errors.Is(err, slashing.ErrDuplicateEvidence) {
		return consensus.SlashResult{}, false, nil
	}
	if err != nil {
		return consensus.SlashResult{}, false, err
	}
	if err := runtime.Store.SaveEvidence(ctx, store.EvidenceRecord{
		Evidence:  evidence,
		Applied:   true,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		return consensus.SlashResult{}, false, err
	}
	machine, err := node.Consensus()
	if err != nil {
		return consensus.SlashResult{}, false, err
	}
	status := machine.Status(ctx)
	height := status.Height
	if height == 0 {
		height = evidence.Height
	}
	if err := machine.UpdateValidatorSetFromRegistry(ctx, runtime.Validators, height); err != nil {
		return consensus.SlashResult{}, false, err
	}
	return result, true, nil
}

func (node *Node) handleLocalEvidence(ctx context.Context, evidence slashing.Evidence) {
	_, _, _ = node.SubmitEvidence(ctx, evidence)
}

func (node *Node) broadcastEvidence(ctx context.Context, evidence slashing.Evidence) error {
	wire, ok := node.runningTransport()
	if !ok {
		return nil
	}
	data, err := encodeEvidenceMessage(evidence)
	if err != nil {
		return err
	}
	return wire.Publish(ctx, p2p.TopicEvidence, data)
}

func (node *Node) startEvidenceGossip(ctx context.Context) error {
	if node.wire == nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	events, err := node.wire.Subscribe(runCtx, p2p.TopicEvidence)
	if err != nil {
		cancel()
		return err
	}
	node.evidenceCancel = cancel
	go node.consumeEvidenceGossip(runCtx, events)
	return nil
}

func (node *Node) consumeEvidenceGossip(ctx context.Context, events <-chan transport.Envelope) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-events:
			if !ok {
				return
			}
			node.acceptEvidenceMessage(ctx, envelope.From, envelope.Data)
		}
	}
}

func (node *Node) acceptEvidenceMessage(ctx context.Context, from p2p.PeerID, data []byte) {
	if !node.admitPeerMessage(ctx, from) {
		return
	}
	evidence, err := decodeEvidenceMessage(data)
	if err != nil {
		node.observePeerMessage(ctx, from, false)
		return
	}
	if _, applied, err := node.applyEvidence(ctx, evidence); err != nil {
		node.observePeerMessage(ctx, from, false)
		return
	} else if !applied {
		node.observePeerMessage(ctx, from, true)
		return
	}
	node.observePeerMessage(ctx, from, true)
}

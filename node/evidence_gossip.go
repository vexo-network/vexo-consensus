package node

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
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
	applyHeight := evidence.Height
	if machine, err := node.Consensus(); err == nil {
		if status := machine.Status(ctx); status.Height > applyHeight {
			applyHeight = status.Height
		}
	}
	if runtime.Store != nil {
		key := store.EvidenceKey(evidence)
		if key == "" {
			return consensus.SlashResult{}, false, store.ErrInvalidKey
		}
		_, err := runtime.Store.EvidenceByKey(ctx, key)
		if err != nil && !errors.Is(err, store.ErrEvidenceNotFound) {
			return consensus.SlashResult{}, false, err
		}
		if errors.Is(err, store.ErrEvidenceNotFound) {
			if err := runtime.Store.SaveEvidence(ctx, store.EvidenceRecord{
				Evidence:  evidence,
				Applied:   false,
				CreatedAt: time.Now().Unix(),
			}); err != nil {
				return consensus.SlashResult{}, false, err
			}
		}
	}
	verifier := consensus.NewEvidenceVerifier(runtime.Crypto.ConsensusVerifier, runtime.Crypto.FinalityVerifier)
	verificationContext, err := node.evidenceVerificationContext(ctx, runtime, evidence)
	if err != nil {
		return consensus.SlashResult{}, false, err
	}
	result, err := consensus.SubmitEvidenceForSlashingWithContext(ctx, runtime.Slashing, runtime.Validators, verifier, applyHeight, evidence, verificationContext)
	if errors.Is(err, slashing.ErrDuplicateEvidence) {
		if runtime.Store != nil {
			key := store.EvidenceKey(evidence)
			if key != "" {
				existing, loadErr := runtime.Store.EvidenceByKey(ctx, key)
				if loadErr == nil && !existing.Applied {
					existing.Applied = true
					if saveErr := runtime.Store.SaveEvidence(ctx, existing); saveErr != nil {
						return consensus.SlashResult{}, false, saveErr
					}
				}
			}
		}
		return consensus.SlashResult{}, false, nil
	}
	if err != nil {
		return consensus.SlashResult{}, false, err
	}
	if err := runtime.ApplyStakingSlashingPenalty(ctx, result.Receipt); err != nil {
		return consensus.SlashResult{}, false, err
	}
	if runtime.Store != nil {
		if err := runtime.Store.SaveEvidence(ctx, store.EvidenceRecord{
			Evidence:  evidence,
			Applied:   true,
			CreatedAt: time.Now().Unix(),
		}); err != nil {
			return consensus.SlashResult{}, false, err
		}
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

func (node *Node) reconcileEvidence(ctx context.Context, runtime *vexoruntime.Runtime) error {
	if runtime.Store == nil {
		return nil
	}
	index, err := runtime.Store.EvidenceIndex(ctx)
	if errors.Is(err, store.ErrEvidenceNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	latestHeight := types.Height(0)
	if state, err := runtime.Store.LatestState(ctx); err == nil {
		latestHeight = state.Height
	} else if !errors.Is(err, store.ErrStateNotFound) {
		return err
	}
	for _, key := range index {
		record, err := runtime.Store.EvidenceByKey(ctx, key)
		if err != nil {
			return err
		}
		applyHeight := record.Evidence.Height
		if latestHeight > applyHeight {
			applyHeight = latestHeight
		}
		verifier := consensus.NewEvidenceVerifier(runtime.Crypto.ConsensusVerifier, runtime.Crypto.FinalityVerifier)
		verificationContext, err := node.evidenceVerificationContext(ctx, runtime, record.Evidence)
		if err != nil {
			return err
		}
		_, err = consensus.SubmitEvidenceForSlashingWithContext(ctx, runtime.Slashing, runtime.Validators, verifier, applyHeight, record.Evidence, verificationContext)
		if err != nil && !errors.Is(err, slashing.ErrDuplicateEvidence) {
			return err
		}
		if err == nil {
			receipt, found, receiptErr := penaltyReceiptForRuntime(ctx, runtime, record.Evidence)
			if receiptErr != nil {
				return receiptErr
			}
			if found {
				if err := runtime.ApplyStakingSlashingPenalty(ctx, receipt); err != nil {
					return err
				}
			}
		}
		record.Applied = true
		if record.CreatedAt == 0 {
			record.CreatedAt = time.Now().Unix()
		}
		if err := runtime.Store.SaveEvidence(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func penaltyReceiptForRuntime(ctx context.Context, runtime *vexoruntime.Runtime, evidence slashing.Evidence) (slashing.PenaltyReceipt, bool, error) {
	reader, ok := runtime.Slashing.(interface {
		PenaltyReceipt(context.Context, slashing.Evidence) (slashing.PenaltyReceipt, bool, error)
	})
	if ok {
		return reader.PenaltyReceipt(ctx, evidence)
	}
	memoryReader, ok := runtime.Slashing.(interface {
		PenaltyReceipt(slashing.Evidence) (slashing.PenaltyReceipt, bool)
	})
	if ok {
		receipt, found := memoryReader.PenaltyReceipt(evidence)
		return receipt, found, nil
	}
	return slashing.PenaltyReceipt{}, false, nil
}

func (node *Node) evidenceVerificationContext(ctx context.Context, runtime *vexoruntime.Runtime, evidence slashing.Evidence) (consensus.EvidenceVerificationContext, error) {
	verificationContext := consensus.EvidenceVerificationContext{}
	if runtime == nil || runtime.Store == nil || evidence.Type != slashing.EvidenceInvalidProposal {
		return verificationContext, nil
	}
	state, err := runtime.Store.StateByHeight(ctx, evidence.Height)
	if errors.Is(err, store.ErrStateNotFound) {
		return verificationContext, nil
	}
	if err != nil {
		return consensus.EvidenceVerificationContext{}, err
	}
	verificationContext.InvalidProposal.ExpectedAppHash = state.AppHash
	return verificationContext, nil
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
	done := make(chan struct{})
	node.evidenceCancel = cancel
	node.evidenceDone = done
	go node.consumeEvidenceGossip(runCtx, events, done)
	return nil
}

func (node *Node) consumeEvidenceGossip(ctx context.Context, events <-chan transport.Envelope, done chan struct{}) {
	defer close(done)
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

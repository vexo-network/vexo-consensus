package node

import (
	"context"
	"errors"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func (node *Node) BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockRecord{}, err
	}
	return runtime.BlockByHeight(ctx, height)
}

func (node *Node) BlockByHash(ctx context.Context, hash types.Hash) (store.BlockRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockRecord{}, err
	}
	return runtime.BlockByHash(ctx, hash)
}

func (node *Node) LatestBlock(ctx context.Context) (store.BlockRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockRecord{}, err
	}
	index, err := runtime.BlockIndex(ctx)
	if err != nil {
		return store.BlockRecord{}, err
	}
	return runtime.BlockByHeight(ctx, index.LatestHeight)
}

func (node *Node) BlockIndex(ctx context.Context) (store.BlockIndex, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.BlockIndex{}, err
	}
	return runtime.BlockIndex(ctx)
}

func (node *Node) LatestState(ctx context.Context) (store.StateRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.StateRecord{}, err
	}
	return runtime.LatestState(ctx)
}

func (node *Node) StateByHeight(ctx context.Context, height types.Height) (store.StateRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.StateRecord{}, err
	}
	return runtime.StateByHeight(ctx, height)
}

func (node *Node) StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.StateRootRecord{}, err
	}
	return runtime.StateRoot(ctx, height, namespace)
}

func (node *Node) QueryEvents(ctx context.Context, key string, value string) ([]events.Record, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.QueryEvents(ctx, key, value)
}

func (node *Node) QueryProof(ctx context.Context, height types.Height, namespace string, key []byte) (queryproof.Proof, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return queryproof.Proof{}, err
	}
	return runtime.QueryProof(ctx, height, namespace, key)
}

func (node *Node) IBCQuery(ctx context.Context, path []string) (vexoapp.QueryResponse, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return vexoapp.QueryResponse{}, err
	}
	return runtime.IBCQuery(ctx, path)
}

func (node *Node) AppQuery(ctx context.Context, path []string, data []byte) (vexoapp.QueryResponse, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return vexoapp.QueryResponse{}, err
	}
	return runtime.AppQuery(ctx, path, data)
}

func (node *Node) AccountSequence(ctx context.Context, address types.Address) (uint64, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return 0, err
	}
	return runtime.AccountSequence(ctx, address)
}

func (node *Node) LatestFinalityProof(ctx context.Context) (finality.Proof, error) {
	if proof, found, err := node.storedLatestFinalityProof(ctx); found || (err != nil && !errors.Is(err, ErrFinalityNotFound)) {
		return proof, err
	}
	return node.finalityProof(ctx, 0)
}

func (node *Node) FinalityProof(ctx context.Context, height types.Height) (finality.Proof, error) {
	if proof, found, err := node.storedFinalityProof(ctx, height); found || (err != nil && !errors.Is(err, ErrFinalityNotFound)) {
		return proof, err
	}
	return node.finalityProof(ctx, height)
}

func (node *Node) storedLatestFinalityProof(ctx context.Context) (finality.Proof, bool, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return finality.Proof{}, false, err
	}
	proofStore, ok := runtime.Store.(store.FinalityProofStore)
	if !ok || proofStore == nil {
		return finality.Proof{}, false, nil
	}
	proof, err := proofStore.LatestFinalityProof(ctx)
	if errors.Is(err, store.ErrFinalityNotFound) {
		return finality.Proof{}, false, nil
	}
	if err != nil {
		return finality.Proof{}, false, err
	}
	loaded := finalityProofFromRecord(proof)
	if loaded.HasThreeChainCommitProof() {
		return loaded, true, nil
	}
	if enriched, err := node.enrichStoredFinalityProof(ctx, loaded); err == nil {
		return enriched, true, nil
	} else if !errors.Is(err, ErrFinalityNotFound) {
		return finality.Proof{}, false, err
	}
	return finality.Proof{}, false, ErrFinalityNotFound
}

func (node *Node) storedFinalityProof(ctx context.Context, height types.Height) (finality.Proof, bool, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return finality.Proof{}, false, err
	}
	proofStore, ok := runtime.Store.(store.FinalityProofStore)
	if !ok || proofStore == nil {
		return finality.Proof{}, false, nil
	}
	proof, err := proofStore.FinalityProof(ctx, height)
	if errors.Is(err, store.ErrFinalityNotFound) {
		return finality.Proof{}, false, nil
	}
	if err != nil {
		return finality.Proof{}, false, err
	}
	loaded := finalityProofFromRecord(proof)
	if loaded.HasThreeChainCommitProof() {
		return loaded, true, nil
	}
	if enriched, err := node.enrichStoredFinalityProof(ctx, loaded); err == nil {
		return enriched, true, nil
	} else if !errors.Is(err, ErrFinalityNotFound) {
		return finality.Proof{}, false, err
	}
	return finality.Proof{}, false, ErrFinalityNotFound
}

func (node *Node) enrichStoredFinalityProof(ctx context.Context, proof finality.Proof) (finality.Proof, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return finality.Proof{}, err
	}
	enriched, err := node.attachCommitChainProof(ctx, runtime, proof)
	if err != nil {
		return finality.Proof{}, err
	}
	if proofStore, ok := runtime.Store.(store.FinalityProofStore); ok && proofStore != nil {
		if err := proofStore.SaveFinalityProof(ctx, finalityProofRecord(enriched)); err != nil {
			return finality.Proof{}, err
		}
	}
	return enriched, nil
}

func (node *Node) finalityProof(ctx context.Context, height types.Height) (finality.Proof, error) {
	machine, err := node.Consensus()
	if err != nil {
		return finality.Proof{}, err
	}
	decisions := machine.CommitDecisions()
	if len(decisions) == 0 {
		return finality.Proof{}, ErrFinalityNotFound
	}
	decision := decisions[len(decisions)-1]
	if height != 0 {
		found := false
		for _, candidate := range decisions {
			if candidate.CommittedHeight == height {
				decision = candidate
				found = true
				break
			}
		}
		if !found {
			return finality.Proof{}, ErrFinalityNotFound
		}
	}
	runtime, err := node.Runtime()
	if err != nil {
		return finality.Proof{}, err
	}
	proof, err := runtime.FinalityProof(ctx, decision.CommittedHeight, decision.CommitQC)
	if err != nil {
		return finality.Proof{}, err
	}
	return node.attachCommitChainProof(ctx, runtime, proof)
}

func finalityProofFromRecord(record store.FinalityProofRecord) finality.Proof {
	commitChain := make([]finality.CommitLink, 0, len(record.CommitChain))
	for _, link := range record.CommitChain {
		commitChain = append(commitChain, finality.CommitLink{
			Header:     link.Header,
			BlockHash:  link.BlockHash,
			QuorumCert: finalityQuorumCertFromRecord(link.QuorumCert),
		})
	}
	return finality.Proof{
		Header:             record.Header,
		BlockHash:          record.BlockHash,
		QuorumCert:         finalityQuorumCertFromRecord(record.QuorumCert),
		CommitChain:        commitChain,
		ValidatorSetHeight: record.ValidatorSetHeight,
		ValidatorSetHash:   record.ValidatorSetHash,
	}
}

func finalityQuorumCertFromRecord(record store.QuorumCertRecord) finality.QuorumCert {
	return finality.QuorumCert{
		Height:      record.Height,
		Round:       record.Round,
		BlockHash:   record.BlockHash,
		Signers:     record.Signers,
		Signature:   record.Signature,
		VotingPower: record.VotingPower,
	}
}

func finalityQuorumCertRecord(quorumCert finality.QuorumCert) store.QuorumCertRecord {
	return store.QuorumCertRecord{
		Height:      quorumCert.Height,
		Round:       quorumCert.Round,
		BlockHash:   quorumCert.BlockHash,
		Signers:     quorumCert.Signers,
		Signature:   quorumCert.Signature,
		VotingPower: quorumCert.VotingPower,
	}
}

func finalityProofRecord(proof finality.Proof) store.FinalityProofRecord {
	commitChain := make([]store.CommitLinkRecord, 0, len(proof.CommitChain))
	for _, link := range proof.CommitChain {
		commitChain = append(commitChain, store.CommitLinkRecord{
			Header:     link.Header,
			BlockHash:  link.BlockHash,
			QuorumCert: finalityQuorumCertRecord(link.QuorumCert),
		})
	}
	return store.FinalityProofRecord{
		Header:             proof.Header,
		BlockHash:          proof.BlockHash,
		QuorumCert:         finalityQuorumCertRecord(proof.QuorumCert),
		CommitChain:        commitChain,
		ValidatorSetHeight: proof.ValidatorSetHeight,
		ValidatorSetHash:   proof.ValidatorSetHash,
	}
}

func (node *Node) PruneBelow(ctx context.Context, retainFrom types.Height) (store.PruneResult, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.PruneResult{}, err
	}
	return runtime.PruneBelow(ctx, retainFrom)
}

func (node *Node) PruneByRetention(ctx context.Context, policy store.RetentionPolicy) (store.PruneResult, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.PruneResult{}, err
	}
	return runtime.PruneByRetention(ctx, policy)
}

func (node *Node) RecoverIndexes(ctx context.Context) (store.RecoverResult, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return store.RecoverResult{}, err
	}
	return runtime.RecoverIndexes(ctx)
}

func (node *Node) Compact(ctx context.Context) error {
	runtime, err := node.Runtime()
	if err != nil {
		return err
	}
	return runtime.Compact(ctx)
}

func (node *Node) Replay(ctx context.Context, from types.Height, to types.Height) (result vexoruntime.ReplayResult, err error) {
	runtime, err := node.Runtime()
	if err != nil {
		return vexoruntime.ReplayResult{}, err
	}
	defer recoverRuntimeAfterReplay(ctx, runtime, &err)
	return runtime.ReplayStrict(ctx, from, to)
}

func (node *Node) ReplayAll(ctx context.Context) (result vexoruntime.ReplayResult, err error) {
	runtime, err := node.Runtime()
	if err != nil {
		return vexoruntime.ReplayResult{}, err
	}
	defer recoverRuntimeAfterReplay(ctx, runtime, &err)
	return runtime.ReplayAllStrict(ctx)
}

func (node *Node) ReplayStrict(ctx context.Context, from types.Height, to types.Height) (result vexoruntime.ReplayResult, err error) {
	runtime, err := node.Runtime()
	if err != nil {
		return vexoruntime.ReplayResult{}, err
	}
	defer recoverRuntimeAfterReplay(ctx, runtime, &err)
	return runtime.ReplayStrict(ctx, from, to)
}

func (node *Node) ReplayAllStrict(ctx context.Context) (result vexoruntime.ReplayResult, err error) {
	runtime, err := node.Runtime()
	if err != nil {
		return vexoruntime.ReplayResult{}, err
	}
	defer recoverRuntimeAfterReplay(ctx, runtime, &err)
	return runtime.ReplayAllStrict(ctx)
}

func recoverRuntimeAfterReplay(ctx context.Context, runtime *vexoruntime.Runtime, err *error) {
	if runtime == nil {
		return
	}
	recoverCtx := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		recoverCtx, cancel = context.WithDeadline(recoverCtx, deadline)
		defer cancel()
	}
	if _, recoverErr := runtime.Recover(recoverCtx); recoverErr != nil && *err == nil {
		*err = recoverErr
	}
}

func (node *Node) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Validators.ValidatorSet(ctx, height)
}

func (node *Node) Committee(ctx context.Context, height types.Height, round types.Round, seed types.Hash) (committee.Committee, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return committee.Committee{}, err
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return committee.Committee{}, err
	}
	epoch := uint64(0)
	if runtime.Config.Committee.EpochLength > 0 && height > 0 {
		epoch = (uint64(height) - 1) / runtime.Config.Committee.EpochLength
	}
	return runtime.Committee.Select(ctx, epoch, round, seed, validatorSet)
}

func ignoreMissingMetricsError(err error) bool {
	return errors.Is(err, store.ErrBlockIndexNotFound) ||
		errors.Is(err, store.ErrStateNotFound) ||
		errors.Is(err, store.ErrBlockNotFound)
}

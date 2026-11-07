package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/mempool"
	appslashing "github.com/vexo-network/vexo-consensus/modules/slashing"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// MaxPendingEvidence bounds the evidence records a node keeps. A full pool refuses new evidence
// instead of evicting old evidence, so flooding it cannot push out evidence that is already queued.
const MaxPendingEvidence = 1024

var (
	ErrEvidenceUnsupported = errors.New("runtime has no slashing module or state store to judge evidence")
	ErrEvidencePoolFull    = errors.New("evidence pool is full")
)

// evidenceModule is what the runtime needs from the slashing module. The module judges evidence
// against replicated state with the rules the block that applies it uses, so a node pools exactly
// the evidence a block can punish.
type evidenceModule interface {
	CheckEvidence(ctx context.Context, store app.StateStore, height types.Height, evidence slashing.Evidence) (slashing.Evidence, error)
	Settled(ctx context.Context, store app.StateStore, height types.Height, evidence slashing.Evidence) (bool, error)
}

// EvidenceAdmission is the outcome of admitting evidence to the pool.
type EvidenceAdmission struct {
	// Evidence is the verified evidence in canonical form.
	Evidence slashing.Evidence
	// Key identifies the offence the evidence proves.
	Key string
	// Tx is the unsigned transaction payload that carries the evidence to a block; a reporter wraps
	// it with a signer, a fee and a nonce.
	Tx types.Tx
	// Hash is the hash of Tx.
	Hash types.Hash
	// New is false when the pool already held evidence of the same offence.
	New bool
}

// EvidenceReconciliation counts what ReconcileEvidence did with the pooled records.
type EvidenceReconciliation struct {
	Kept    int
	Dropped int
}

func (runtime *Runtime) evidenceModule() (evidenceModule, error) {
	if runtime.Store == nil {
		return nil, ErrEvidenceUnsupported
	}
	for _, module := range runtime.AppModules() {
		if judge, ok := module.(evidenceModule); ok {
			return judge, nil
		}
	}
	return nil, ErrEvidenceUnsupported
}

// nextHeight is the height of the block that will execute next.
func (runtime *Runtime) nextHeight(ctx context.Context) (types.Height, error) {
	state, err := runtime.Store.LatestState(ctx)
	if errors.Is(err, store.ErrStateNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return state.Height + 1, nil
}

// AdmitEvidence verifies evidence against the committed state, exactly as the block that applies it
// will, and keeps it in the pool. It never changes consensus state: the penalty is applied only when
// a block carries the evidence transaction. Evidence of an offence the pool already holds is not an
// error but is reported as not new.
func (runtime *Runtime) AdmitEvidence(ctx context.Context, evidence slashing.Evidence) (EvidenceAdmission, error) {
	judge, err := runtime.evidenceModule()
	if err != nil {
		return EvidenceAdmission{}, err
	}
	height, err := runtime.nextHeight(ctx)
	if err != nil {
		return EvidenceAdmission{}, err
	}
	checked, err := judge.CheckEvidence(ctx, runtime.Store, height, evidence)
	if err != nil {
		return EvidenceAdmission{}, err
	}
	key := store.EvidenceKey(checked)
	if key == "" {
		return EvidenceAdmission{}, fmt.Errorf("%w: verified evidence has no offence key", ErrEvidenceUnsupported)
	}
	tx, err := appslashing.EncodeEvidenceTx(checked)
	if err != nil {
		return EvidenceAdmission{}, err
	}
	admission := EvidenceAdmission{Evidence: checked, Key: key, Tx: tx, Hash: mempool.HashTx(tx)}

	runtime.evidenceMu.Lock()
	defer runtime.evidenceMu.Unlock()
	if record, err := runtime.Store.EvidenceByKey(ctx, key); err == nil {
		// The offence has one wire form, that of the evidence pooled first: another proof or label
		// of it is a different transaction, and queueing each would let one offence fill the
		// mempool and blocks with free transactions of which only the first can apply.
		pooledTx, err := appslashing.EncodeEvidenceTx(record.Evidence)
		if err != nil {
			return EvidenceAdmission{}, err
		}
		return EvidenceAdmission{Evidence: record.Evidence, Key: key, Tx: pooledTx, Hash: mempool.HashTx(pooledTx)}, nil
	} else if !errors.Is(err, store.ErrEvidenceNotFound) && !errors.Is(err, store.ErrEvidenceCorrupt) {
		return EvidenceAdmission{}, err
	}
	pooled, err := runtime.Store.EvidenceIndex(ctx)
	if err != nil && !errors.Is(err, store.ErrEvidenceNotFound) {
		return EvidenceAdmission{}, err
	}
	if len(pooled) >= MaxPendingEvidence {
		return EvidenceAdmission{}, ErrEvidencePoolFull
	}
	if err := runtime.Store.SaveEvidence(ctx, store.EvidenceRecord{Evidence: checked}); err != nil {
		return EvidenceAdmission{}, err
	}
	admission.New = true
	return admission, nil
}

// AdmitEvidenceTx admits the evidence an evidence transaction carries.
func (runtime *Runtime) AdmitEvidenceTx(ctx context.Context, tx types.Tx) (EvidenceAdmission, error) {
	evidence, err := appslashing.DecodeEvidenceTx(tx)
	if err != nil {
		return EvidenceAdmission{}, err
	}
	return runtime.AdmitEvidence(ctx, evidence)
}

// ReconcileEvidence re-judges every pooled record against the committed state and deletes the
// records that cannot be applied any more: corrupt, unverifiable, already punished or expired. It
// never fails because of the content of a record, so a bad record cannot stop a node from starting;
// only a cancelled context or a storage failure is returned.
func (runtime *Runtime) ReconcileEvidence(ctx context.Context) (EvidenceReconciliation, error) {
	var result EvidenceReconciliation
	if runtime.Store == nil {
		return result, nil
	}
	runtime.evidenceMu.Lock()
	defer runtime.evidenceMu.Unlock()
	keys, err := runtime.Store.EvidenceIndex(ctx)
	if errors.Is(err, store.ErrEvidenceNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	judge, judgeErr := runtime.evidenceModule()
	height, err := runtime.nextHeight(ctx)
	if err != nil {
		return result, err
	}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		keep, err := runtime.keepEvidence(ctx, judge, judgeErr, height, key)
		if err != nil {
			return result, err
		}
		if keep {
			result.Kept++
			continue
		}
		if err := runtime.Store.DeleteEvidence(ctx, key); err != nil {
			return result, err
		}
		result.Dropped++
	}
	return result, nil
}

// keepEvidence reports whether the record stored under key is still worth pooling. An error is a
// storage failure or a cancelled context; a record that is bad in any way is simply not kept.
func (runtime *Runtime) keepEvidence(ctx context.Context, judge evidenceModule, judgeErr error, height types.Height, key string) (bool, error) {
	record, err := runtime.Store.EvidenceByKey(ctx, key)
	switch {
	case errors.Is(err, store.ErrEvidenceCorrupt):
		return false, nil
	case err != nil:
		return false, err
	case judgeErr != nil:
		return false, nil
	}
	checked, err := judge.CheckEvidence(ctx, runtime.Store, height, record.Evidence)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, ctxErr
	}
	if err != nil {
		return false, nil
	}
	return store.EvidenceKey(checked) == key, nil
}

// pruneEvidence deletes the pooled records the block that just committed settled: offences it
// punished and evidence that grew too old. It verifies no signature, so it is cheap enough to run
// after every block.
func (runtime *Runtime) pruneEvidence(ctx context.Context, height types.Height) error {
	judge, err := runtime.evidenceModule()
	if errors.Is(err, ErrEvidenceUnsupported) {
		return nil
	}
	runtime.evidenceMu.Lock()
	defer runtime.evidenceMu.Unlock()
	keys, err := runtime.Store.EvidenceIndex(ctx)
	if errors.Is(err, store.ErrEvidenceNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, key := range keys {
		record, err := runtime.Store.EvidenceByKey(ctx, key)
		settled := errors.Is(err, store.ErrEvidenceCorrupt)
		if err != nil && !settled {
			return err
		}
		if !settled {
			settled, err = judge.Settled(ctx, runtime.Store, height, record.Evidence)
			if err != nil {
				return err
			}
		}
		if settled {
			if err := runtime.Store.DeleteEvidence(ctx, key); err != nil {
				return err
			}
		}
	}
	return nil
}

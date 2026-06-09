package slashing

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/types"
)

const slashingNamespace = "slashing"

var ErrAtomicPenaltyStoreRequired = errors.New("slashing penalty bundles require atomic batch store")

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
}

type NotFoundClassifier interface {
	IsNotFound(error) bool
}

type StoreKeeper struct {
	store           KVStore
	policy          PenaltyPolicy
	lifecyclePolicy LifecyclePolicy
}

type evidenceDocument struct {
	Evidence  Evidence       `json:"evidence"`
	Status    EvidenceStatus `json:"status"`
	ExpiresAt types.Height   `json:"expires_at,omitempty"`
}

type jailDocument struct {
	Validator types.ValidatorID `json:"validator"`
	Until     types.Height      `json:"until"`
}

type unbondingDocument struct {
	Validator     types.ValidatorID `json:"validator"`
	ReleaseHeight types.Height      `json:"release_height"`
}

func NewStoreKeeper(store KVStore, policy PenaltyPolicy) (*StoreKeeper, error) {
	return NewStoreKeeperWithLifecycle(store, policy, DefaultLifecyclePolicy())
}

func NewStoreKeeperWithLifecycle(store KVStore, policy PenaltyPolicy, lifecyclePolicy LifecyclePolicy) (*StoreKeeper, error) {
	if store == nil {
		return nil, errors.New("slashing store is required")
	}
	if policy == nil {
		policy = DefaultPenaltyPolicy()
	}
	lifecyclePolicy = normalizeLifecyclePolicy(lifecyclePolicy)
	return &StoreKeeper{store: store, policy: policy, lifecyclePolicy: lifecyclePolicy}, nil
}

func (keeper *StoreKeeper) SubmitEvidence(ctx context.Context, evidence Evidence) error {
	if err := keeper.ValidateEvidence(ctx, evidence); err != nil {
		return err
	}
	if _, err := keeper.loadEvidence(ctx, evidence); err == nil {
		return ErrDuplicateEvidence
	} else if !keeper.isNotFound(err) {
		return err
	}
	document := evidenceDocument{Evidence: cloneEvidence(evidence), Status: EvidenceStatusSubmitted}
	if keeper.lifecyclePolicy.EvidenceMaxAge > 0 {
		document.ExpiresAt = evidence.Height + keeper.lifecyclePolicy.EvidenceMaxAge
	}
	return keeper.saveEvidence(ctx, document)
}

func (keeper *StoreKeeper) SubmitWithExpiration(ctx context.Context, evidence Evidence, expiresAt types.Height) error {
	if expiresAt != 0 && evidence.Height >= expiresAt {
		return ErrEvidenceExpired
	}
	if err := keeper.SubmitEvidence(ctx, evidence); err != nil {
		return err
	}
	document, err := keeper.loadEvidence(ctx, evidence)
	if err != nil {
		return err
	}
	document.ExpiresAt = expiresAt
	return keeper.saveEvidence(ctx, document)
}

func (keeper *StoreKeeper) ValidateEvidence(ctx context.Context, evidence Evidence) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if evidence.Validator == "" {
		return ErrMissingValidator
	}
	if evidence.Height == 0 {
		return ErrMissingHeight
	}
	if len(evidence.Proof) == 0 {
		return ErrEmptyProof
	}
	if _, found := keeper.policy[evidence.Type]; !found {
		return ErrUnknownEvidenceType
	}
	return nil
}

func (keeper *StoreKeeper) ApplyPenalty(ctx context.Context, evidence Evidence) (Penalty, error) {
	if err := keeper.ValidateEvidence(ctx, evidence); err != nil {
		return Penalty{}, err
	}
	penalty, found := keeper.policy[evidence.Type]
	if !found {
		return Penalty{}, ErrPenaltyNotConfigured
	}
	return penalty, nil
}

func (keeper *StoreKeeper) ApplyPenaltyWithStake(ctx context.Context, evidence Evidence, currentPower types.VotingPower) (PenaltyReceipt, error) {
	document, err := keeper.loadEvidence(ctx, evidence)
	if err != nil {
		return PenaltyReceipt{}, err
	}
	if receipt, found, err := keeper.PenaltyReceipt(ctx, evidence); err != nil {
		return PenaltyReceipt{}, err
	} else if found {
		document.Status = EvidenceStatusApplied
		if err := keeper.savePenaltyBundle(ctx, receipt, document); err != nil {
			return PenaltyReceipt{}, err
		}
		return receipt, nil
	}
	if document.Status == EvidenceStatusExpired {
		return PenaltyReceipt{}, ErrEvidenceExpired
	}
	if document.Status == EvidenceStatusAppealed {
		return PenaltyReceipt{}, ErrEvidenceAppealed
	}
	penalty, err := keeper.ApplyPenalty(ctx, evidence)
	if err != nil {
		return PenaltyReceipt{}, err
	}
	remaining, err := ApplySlash(currentPower, penalty)
	if err != nil {
		return PenaltyReceipt{}, err
	}
	receipt := PenaltyReceipt{
		Evidence:       cloneEvidence(evidence),
		Penalty:        penalty,
		PreviousPower:  currentPower,
		RemainingPower: remaining,
	}
	document.Status = EvidenceStatusApplied
	if err := keeper.savePenaltyBundle(ctx, receipt, document); err != nil {
		return PenaltyReceipt{}, err
	}
	return receipt, nil
}

func (keeper *StoreKeeper) savePenaltyBundle(ctx context.Context, receipt PenaltyReceipt, document evidenceDocument) error {
	encodedReceipt, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	encodedEvidence, err := json.Marshal(document)
	if err != nil {
		return err
	}
	writes := []kvbatch.KVWrite{
		{Namespace: slashingNamespace, Key: penaltyKey(receipt.Evidence), Value: encodedReceipt},
		{Namespace: slashingNamespace, Key: evidenceDocumentKey(document.Evidence), Value: encodedEvidence},
	}
	if receipt.Penalty.JailDuration > 0 {
		encodedJail, err := json.Marshal(jailDocument{Validator: receipt.Evidence.Validator, Until: receipt.Evidence.Height + types.Height(receipt.Penalty.JailDuration)})
		if err != nil {
			return err
		}
		writes = append(writes, kvbatch.KVWrite{Namespace: slashingNamespace, Key: jailKey(receipt.Evidence.Validator), Value: encodedJail})
	}
	if keeper.lifecyclePolicy.UnbondingDelay > 0 {
		encodedUnbonding, err := json.Marshal(unbondingDocument{Validator: receipt.Evidence.Validator, ReleaseHeight: receipt.Evidence.Height + keeper.lifecyclePolicy.UnbondingDelay})
		if err != nil {
			return err
		}
		writes = append(writes, kvbatch.KVWrite{Namespace: slashingNamespace, Key: unbondingKey(receipt.Evidence.Validator), Value: encodedUnbonding})
	}
	if batchStore, ok := keeper.store.(kvbatch.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	return ErrAtomicPenaltyStoreRequired
}

func (keeper *StoreKeeper) AppealEvidence(ctx context.Context, evidence Evidence) (bool, error) {
	document, err := keeper.loadEvidence(ctx, evidence)
	if err != nil {
		return false, err
	}
	if document.Status == EvidenceStatusApplied {
		return false, nil
	}
	document.Status = EvidenceStatusAppealed
	if err := keeper.saveEvidence(ctx, document); err != nil {
		return false, err
	}
	return true, nil
}

func (keeper *StoreKeeper) ExpireEvidence(ctx context.Context, evidence Evidence, currentHeight types.Height) (bool, error) {
	document, err := keeper.loadEvidence(ctx, evidence)
	if err != nil {
		return false, err
	}
	if document.ExpiresAt == 0 || currentHeight < document.ExpiresAt || document.Status == EvidenceStatusApplied {
		return false, nil
	}
	document.Status = EvidenceStatusExpired
	if err := keeper.saveEvidence(ctx, document); err != nil {
		return false, err
	}
	return true, nil
}

func (keeper *StoreKeeper) EvidenceLifecycle(ctx context.Context, evidence Evidence) (EvidenceStatus, bool, error) {
	document, err := keeper.loadEvidence(ctx, evidence)
	if err != nil {
		if keeper.isNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return document.Status, true, nil
}

func (keeper *StoreKeeper) PenaltyReceipt(ctx context.Context, evidence Evidence) (PenaltyReceipt, bool, error) {
	encoded, err := keeper.store.Get(ctx, slashingNamespace, penaltyKey(evidence))
	if err != nil {
		if keeper.isNotFound(err) {
			return PenaltyReceipt{}, false, nil
		}
		return PenaltyReceipt{}, false, err
	}
	var receipt PenaltyReceipt
	if err := json.Unmarshal(encoded, &receipt); err != nil {
		return PenaltyReceipt{}, false, err
	}
	receipt.Evidence = cloneEvidence(receipt.Evidence)
	return receipt, true, nil
}

func (keeper *StoreKeeper) JailUntil(ctx context.Context, validator types.ValidatorID) (types.Height, bool, error) {
	encoded, err := keeper.store.Get(ctx, slashingNamespace, jailKey(validator))
	if err != nil {
		if keeper.isNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	var document jailDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		return 0, false, err
	}
	return document.Until, true, nil
}

func (keeper *StoreKeeper) IsJailed(ctx context.Context, validator types.ValidatorID, currentHeight types.Height) (bool, error) {
	until, found, err := keeper.JailUntil(ctx, validator)
	if err != nil {
		return false, err
	}
	return found && currentHeight < until, nil
}

func (keeper *StoreKeeper) UnbondingReleaseHeight(ctx context.Context, validator types.ValidatorID) (types.Height, bool, error) {
	encoded, err := keeper.store.Get(ctx, slashingNamespace, unbondingKey(validator))
	if err != nil {
		if keeper.isNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	var document unbondingDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		return 0, false, err
	}
	return document.ReleaseHeight, true, nil
}

func (keeper *StoreKeeper) CanUnbond(ctx context.Context, validator types.ValidatorID, currentHeight types.Height) (bool, error) {
	releaseHeight, found, err := keeper.UnbondingReleaseHeight(ctx, validator)
	if err != nil {
		return false, err
	}
	return !found || currentHeight >= releaseHeight, nil
}

func (keeper *StoreKeeper) loadEvidence(ctx context.Context, evidence Evidence) (evidenceDocument, error) {
	encoded, err := keeper.store.Get(ctx, slashingNamespace, evidenceDocumentKey(evidence))
	if err != nil {
		return evidenceDocument{}, err
	}
	var document evidenceDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		return evidenceDocument{}, err
	}
	document.Evidence = cloneEvidence(document.Evidence)
	return document, nil
}

func (keeper *StoreKeeper) saveEvidence(ctx context.Context, document evidenceDocument) error {
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return keeper.store.Set(ctx, slashingNamespace, evidenceDocumentKey(document.Evidence), encoded)
}

func (keeper *StoreKeeper) saveJail(ctx context.Context, validator types.ValidatorID, until types.Height) error {
	encoded, err := json.Marshal(jailDocument{Validator: validator, Until: until})
	if err != nil {
		return err
	}
	return keeper.store.Set(ctx, slashingNamespace, jailKey(validator), encoded)
}

func (keeper *StoreKeeper) saveUnbonding(ctx context.Context, validator types.ValidatorID, releaseHeight types.Height) error {
	encoded, err := json.Marshal(unbondingDocument{Validator: validator, ReleaseHeight: releaseHeight})
	if err != nil {
		return err
	}
	return keeper.store.Set(ctx, slashingNamespace, unbondingKey(validator), encoded)
}

func (keeper *StoreKeeper) isNotFound(err error) bool {
	if err == nil {
		return false
	}
	classifier, ok := keeper.store.(NotFoundClassifier)
	return ok && classifier.IsNotFound(err)
}

func evidenceDocumentKey(evidence Evidence) []byte {
	return []byte("evidence/" + evidenceKey(evidence))
}

func penaltyKey(evidence Evidence) []byte {
	return []byte("penalty/" + evidenceKey(evidence))
}

func jailKey(validator types.ValidatorID) []byte {
	return []byte("jail/" + string(validator))
}

func unbondingKey(validator types.ValidatorID) []byte {
	return []byte("unbonding/" + string(validator))
}

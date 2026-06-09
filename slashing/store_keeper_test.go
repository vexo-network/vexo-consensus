package slashing

import (
	"context"
	"errors"
	"testing"
	"testing/iotest"

	"github.com/vexo-network/vexo-consensus/kvbatch"
)

func TestStoreKeeperPersistsEvidenceLifecycleAndPenalty(t *testing.T) {
	storage := newMemoryKV()

	keeper, err := NewStoreKeeperWithLifecycle(
		storage,
		PenaltyPolicy{EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30}},
		LifecyclePolicy{EvidenceMaxAge: 100, AppealWindow: 5, UnbondingDelay: 40},
	)
	if err != nil {
		t.Fatal(err)
	}
	evidence := validEvidence(EvidenceConflictingVote)
	if err := keeper.SubmitWithExpiration(context.Background(), evidence, 10); err != nil {
		t.Fatal(err)
	}
	status, found, err := keeper.EvidenceLifecycle(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || status != EvidenceStatusSubmitted {
		t.Fatalf("expected submitted, got %s found=%t", status, found)
	}
	receipt, err := keeper.ApplyPenaltyWithStake(context.Background(), evidence, 100)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PreviousPower != 100 || receipt.RemainingPower != 75 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	jailUntil, found, err := keeper.JailUntil(context.Background(), evidence.Validator)
	if err != nil {
		t.Fatal(err)
	}
	if !found || jailUntil != 31 {
		t.Fatalf("unexpected jail: %d found=%t", jailUntil, found)
	}
	jailed, err := keeper.IsJailed(context.Background(), evidence.Validator, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !jailed {
		t.Fatal("expected validator jailed before jail-until height")
	}
	canUnbond, err := keeper.CanUnbond(context.Background(), evidence.Validator, 40)
	if err != nil {
		t.Fatal(err)
	}
	if canUnbond {
		t.Fatal("expected unbonding blocked before release height")
	}
	releaseHeight, found, err := keeper.UnbondingReleaseHeight(context.Background(), evidence.Validator)
	if err != nil {
		t.Fatal(err)
	}
	if !found || releaseHeight != 41 {
		t.Fatalf("unexpected unbonding release height: %d found=%t", releaseHeight, found)
	}

	reopened, err := NewStoreKeeperWithLifecycle(
		storage,
		PenaltyPolicy{EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30}},
		LifecyclePolicy{EvidenceMaxAge: 100, AppealWindow: 5, UnbondingDelay: 40},
	)
	if err != nil {
		t.Fatal(err)
	}
	stored, found, err := reopened.PenaltyReceipt(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || stored.RemainingPower != 75 {
		t.Fatalf("expected persisted receipt, got %+v found=%t", stored, found)
	}
	reopenedCanUnbond, err := reopened.CanUnbond(context.Background(), evidence.Validator, 41)
	if err != nil {
		t.Fatal(err)
	}
	if !reopenedCanUnbond {
		t.Fatal("expected persisted unbonding release to allow exit")
	}
}

func TestStoreKeeperAppealAndExpire(t *testing.T) {
	storage := newMemoryKV()

	keeper, err := NewStoreKeeper(storage, nil)
	if err != nil {
		t.Fatal(err)
	}
	evidence := validEvidence(EvidenceDoubleSign)
	if err := keeper.SubmitWithExpiration(context.Background(), evidence, 3); err != nil {
		t.Fatal(err)
	}
	appealed, err := keeper.AppealEvidence(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !appealed {
		t.Fatal("expected appeal")
	}
	expired, err := keeper.ExpireEvidence(context.Background(), evidence, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !expired {
		t.Fatal("expected expiration")
	}
	status, found, err := keeper.EvidenceLifecycle(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || status != EvidenceStatusExpired {
		t.Fatalf("expected expired, got %s found=%t", status, found)
	}
}

func TestStoreKeeperRejectsAppealedPenaltyApplication(t *testing.T) {
	storage := newMemoryKV()

	keeper, err := NewStoreKeeper(storage, nil)
	if err != nil {
		t.Fatal(err)
	}
	evidence := validEvidence(EvidenceDoubleSign)
	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if appealed, err := keeper.AppealEvidence(context.Background(), evidence); err != nil || !appealed {
		t.Fatalf("expected appeal, appealed=%t err=%v", appealed, err)
	}
	if _, err := keeper.ApplyPenaltyWithStake(context.Background(), evidence, 100); !errors.Is(err, ErrEvidenceAppealed) {
		t.Fatalf("expected appealed evidence error, got %v", err)
	}
}

func TestStoreKeeperPropagatesCorruptPenaltyReceipt(t *testing.T) {
	storage := newMemoryKV()
	keeper, err := NewStoreKeeper(storage, nil)
	if err != nil {
		t.Fatal(err)
	}
	evidence := validEvidence(EvidenceDoubleSign)
	storage.values[slashingNamespace+"/"+string(penaltyKey(evidence))] = []byte("{")

	if _, _, err := keeper.PenaltyReceipt(context.Background(), evidence); err == nil {
		t.Fatal("expected corrupt receipt decode error")
	}
}

func TestStoreKeeperPropagatesReadErrors(t *testing.T) {
	storage := newMemoryKV()
	storage.readErr = iotest.ErrTimeout
	keeper, err := NewStoreKeeper(storage, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := keeper.PenaltyReceipt(context.Background(), validEvidence(EvidenceDoubleSign)); !errors.Is(err, iotest.ErrTimeout) {
		t.Fatalf("expected read timeout, got %v", err)
	}
	if _, _, err := keeper.JailUntil(context.Background(), "alice"); !errors.Is(err, iotest.ErrTimeout) {
		t.Fatalf("expected jail read timeout, got %v", err)
	}
	if _, _, err := keeper.UnbondingReleaseHeight(context.Background(), "alice"); !errors.Is(err, iotest.ErrTimeout) {
		t.Fatalf("expected unbonding read timeout, got %v", err)
	}
	if _, _, err := keeper.EvidenceLifecycle(context.Background(), validEvidence(EvidenceDoubleSign)); !errors.Is(err, iotest.ErrTimeout) {
		t.Fatalf("expected lifecycle read timeout, got %v", err)
	}
	if err := keeper.SubmitEvidence(context.Background(), validEvidence(EvidenceDoubleSign)); !errors.Is(err, iotest.ErrTimeout) {
		t.Fatalf("expected submit read timeout, got %v", err)
	}
}

type memoryKV struct {
	values  map[string][]byte
	readErr error
}

func newMemoryKV() *memoryKV {
	return &memoryKV{values: make(map[string][]byte)}
}

func (store *memoryKV) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	store.values[namespace+"/"+string(key)] = append([]byte(nil), value...)
	return nil
}

func (store *memoryKV) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	for _, write := range writes {
		if write.Delete {
			_ = store.Delete(ctx, write.Namespace, write.Key)
			continue
		}
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func (store *memoryKV) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	if store.readErr != nil {
		return nil, store.readErr
	}
	value, found := store.values[namespace+"/"+string(key)]
	if !found {
		return nil, errMemoryKVNotFound{}
	}
	return append([]byte(nil), value...), nil
}

func (store *memoryKV) Delete(ctx context.Context, namespace string, key []byte) error {
	delete(store.values, namespace+"/"+string(key))
	return nil
}

type errMemoryKVNotFound struct{}

func (errMemoryKVNotFound) Error() string {
	return "not found"
}

func (store *memoryKV) IsNotFound(err error) bool {
	var notFound errMemoryKVNotFound
	return errors.As(err, &notFound)
}

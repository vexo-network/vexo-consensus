package slashing

import (
	"context"
	"testing"
)

func TestStoreKeeperPersistsEvidenceLifecycleAndPenalty(t *testing.T) {
	storage := newMemoryKV()

	keeper, err := NewStoreKeeper(storage, PenaltyPolicy{
		EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
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

	reopened, err := NewStoreKeeper(storage, PenaltyPolicy{
		EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
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

type memoryKV struct {
	values map[string][]byte
}

func newMemoryKV() *memoryKV {
	return &memoryKV{values: make(map[string][]byte)}
}

func (store *memoryKV) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	store.values[namespace+"/"+string(key)] = append([]byte(nil), value...)
	return nil
}

func (store *memoryKV) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
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

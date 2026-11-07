package app

import (
	"context"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

// mapStateStore is a state store that cannot report namespace roots.
type mapStateStore struct {
	values map[string][]byte
}

func (store *mapStateStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	store.values[namespace+"/"+string(key)] = append([]byte(nil), value...)
	return nil
}

func (store *mapStateStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	return store.values[namespace+"/"+string(key)], nil
}

func (store *mapStateStore) Delete(ctx context.Context, namespace string, key []byte) error {
	delete(store.values, namespace+"/"+string(key))
	return nil
}

// The application hash commits to the state. A store that cannot report the roots of its
// namespaces must fail the hash instead of yielding one that ignores every state change.
func TestAppHashFailsForAStoreWithoutNamespaceRoots(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", nil, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	store := &mapStateStore{values: make(map[string][]byte)}
	hash, err := runtime.computeAppHash(context.Background(), 1, store)
	if err == nil || !strings.Contains(err.Error(), "cannot report namespace roots") {
		t.Fatalf("expected the hash to fail, got hash %x err %v", hash, err)
	}
	if hash != (types.Hash{}) {
		t.Fatalf("a failed hash must be empty, got %x", hash)
	}
}

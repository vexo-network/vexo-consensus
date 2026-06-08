package app

import (
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

type StagedStore struct {
	base    StateStore
	writes  []kvbatch.KVWrite
	overlay map[string]kvbatch.KVWrite
}

type rootWithWritesStore interface {
	RootWithWrites(ctx context.Context, namespace string, writes []kvbatch.KVWrite) (types.Hash, error)
}

func NewStagedStore(base StateStore) *StagedStore {
	return &StagedStore{
		base:    base,
		overlay: make(map[string]kvbatch.KVWrite),
	}
}

func (store *StagedStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	if store == nil || store.base == nil {
		return errors.New("missing staged base store")
	}
	write := kvbatch.KVWrite{
		Namespace: namespace,
		Key:       append([]byte(nil), key...),
		Value:     append([]byte(nil), value...),
	}
	store.writes = append(store.writes, write)
	store.overlay[stagedKey(namespace, key)] = write
	return nil
}

func (store *StagedStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	if store == nil || store.base == nil {
		return nil, errors.New("missing staged base store")
	}
	if write, found := store.overlay[stagedKey(namespace, key)]; found {
		if write.Delete {
			return nil, vexostore.ErrKeyNotFound
		}
		return append([]byte(nil), write.Value...), nil
	}
	return store.base.Get(ctx, namespace, key)
}

func (store *StagedStore) Delete(ctx context.Context, namespace string, key []byte) error {
	if store == nil || store.base == nil {
		return errors.New("missing staged base store")
	}
	write := kvbatch.KVWrite{
		Namespace: namespace,
		Key:       append([]byte(nil), key...),
		Delete:    true,
	}
	store.writes = append(store.writes, write)
	store.overlay[stagedKey(namespace, key)] = write
	return nil
}

func (store *StagedStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	if store == nil || store.base == nil {
		return errors.New("missing staged base store")
	}
	for _, write := range writes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		copied := kvbatch.KVWrite{
			Namespace: write.Namespace,
			Key:       append([]byte(nil), write.Key...),
			Value:     append([]byte(nil), write.Value...),
			Delete:    write.Delete,
		}
		store.writes = append(store.writes, copied)
		store.overlay[stagedKey(write.Namespace, write.Key)] = copied
	}
	return nil
}

func (store *StagedStore) Root(ctx context.Context, namespace string) (types.Hash, error) {
	if store == nil || store.base == nil {
		return types.Hash{}, errors.New("missing staged base store")
	}
	if rooter, ok := store.base.(rootWithWritesStore); ok {
		return rooter.RootWithWrites(ctx, namespace, store.writes)
	}
	rooter, ok := store.base.(StateRootStore)
	if !ok {
		return types.Hash{}, errors.New("staged base store cannot compute roots")
	}
	return rooter.Root(ctx, namespace)
}

func (store *StagedStore) ExportPrefix(ctx context.Context, namespace string, prefix []byte) ([]vexostore.KVPair, error) {
	if store == nil || store.base == nil {
		return nil, errors.New("missing staged base store")
	}
	prefixStore, ok := store.base.(vexostore.PrefixKVStore)
	if !ok {
		return nil, errors.New("staged base store cannot export prefixes")
	}
	pairs, err := prefixStore.ExportPrefix(ctx, namespace, prefix)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]vexostore.KVPair, len(pairs)+len(store.overlay))
	for _, pair := range pairs {
		merged[string(pair.Key)] = vexostore.KVPair{
			Namespace: pair.Namespace,
			Key:       append([]byte(nil), pair.Key...),
			Value:     append([]byte(nil), pair.Value...),
		}
	}
	rawPrefix := string(prefix)
	for _, write := range store.overlay {
		if write.Namespace != namespace || !strings.HasPrefix(string(write.Key), rawPrefix) {
			continue
		}
		key := string(write.Key)
		if write.Delete {
			delete(merged, key)
			continue
		}
		merged[key] = vexostore.KVPair{
			Namespace: write.Namespace,
			Key:       append([]byte(nil), write.Key...),
			Value:     append([]byte(nil), write.Value...),
		}
	}
	result := make([]vexostore.KVPair, 0, len(merged))
	for _, pair := range merged {
		result = append(result, pair)
	}
	sort.Slice(result, func(first int, second int) bool {
		return string(result[first].Key) < string(result[second].Key)
	})
	return result, nil
}

func (store *StagedStore) Writes() []kvbatch.KVWrite {
	if store == nil {
		return nil
	}
	copied := make([]kvbatch.KVWrite, 0, len(store.writes))
	for _, write := range store.writes {
		copied = append(copied, kvbatch.KVWrite{
			Namespace: write.Namespace,
			Key:       append([]byte(nil), write.Key...),
			Value:     append([]byte(nil), write.Value...),
			Delete:    write.Delete,
		})
	}
	return copied
}

func stagedKey(namespace string, key []byte) string {
	return namespace + ":" + hex.EncodeToString(key)
}

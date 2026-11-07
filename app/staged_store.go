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

var errMissingStagedBase = errors.New("missing staged base store")

// StagedStore buffers writes over a base store. Reads see the buffered writes
// first. The ordered write log is what a block commits atomically, and a
// StagedStore may itself be the base of another one: the application runtime
// executes every transaction in a layer over the block's layer, so a failed
// transaction is discarded without touching the block.
type StagedStore struct {
	base    StateStore
	writes  []kvbatch.KVWrite
	overlay map[string]kvbatch.KVWrite
	fault   error
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

// Fault returns the first store error that is not a deterministic outcome of
// the operation itself: an I/O failure or a cancelled context. A module that
// swallows such an error and reports a failed transaction would turn a local
// failure into consensus state, so the runtime aborts the block instead.
func (store *StagedStore) Fault() error {
	if store == nil {
		return nil
	}
	return store.fault
}

// observe latches err when it says something about this node rather than about
// the data.
func (store *StagedStore) observe(err error) error {
	if err == nil || store.fault != nil {
		return err
	}
	switch {
	case errors.Is(err, vexostore.ErrKeyNotFound),
		errors.Is(err, vexostore.ErrInvalidKey),
		errors.Is(err, vexostore.ErrInvalidNamespace):
	default:
		store.fault = err
	}
	return err
}

func (store *StagedStore) ready() error {
	if store == nil || store.base == nil {
		return errMissingStagedBase
	}
	return nil
}

func validateWrite(namespace string, key []byte) error {
	if namespace == "" {
		return vexostore.ErrInvalidNamespace
	}
	if len(key) == 0 {
		return vexostore.ErrInvalidKey
	}
	return nil
}

func (store *StagedStore) stage(write kvbatch.KVWrite) {
	copied := kvbatch.KVWrite{
		Namespace: write.Namespace,
		Key:       append([]byte(nil), write.Key...),
		Delete:    write.Delete,
	}
	if !write.Delete {
		copied.Value = append([]byte(nil), write.Value...)
	}
	store.writes = append(store.writes, copied)
	store.overlay[stagedKey(copied.Namespace, copied.Key)] = copied
}

func (store *StagedStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	if err := store.ready(); err != nil {
		return err
	}
	if err := validateWrite(namespace, key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return store.observe(err)
	}
	store.stage(kvbatch.KVWrite{Namespace: namespace, Key: key, Value: value})
	return nil
}

func (store *StagedStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	if err := store.ready(); err != nil {
		return nil, err
	}
	if write, found := store.overlay[stagedKey(namespace, key)]; found {
		if write.Delete {
			return nil, vexostore.ErrKeyNotFound
		}
		return append([]byte(nil), write.Value...), nil
	}
	value, err := store.base.Get(ctx, namespace, key)
	if err != nil {
		return nil, store.observe(err)
	}
	return value, nil
}

func (store *StagedStore) Delete(ctx context.Context, namespace string, key []byte) error {
	if err := store.ready(); err != nil {
		return err
	}
	if err := validateWrite(namespace, key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return store.observe(err)
	}
	store.stage(kvbatch.KVWrite{Namespace: namespace, Key: key, Delete: true})
	return nil
}

// SetBatch stages writes atomically: a write that is invalid or a cancelled
// context stages none of them.
func (store *StagedStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	if err := store.ready(); err != nil {
		return err
	}
	for _, write := range writes {
		if err := validateWrite(write.Namespace, write.Key); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return store.observe(err)
	}
	for _, write := range writes {
		store.stage(write)
	}
	return nil
}

// ApplyTo replays the ordered write log onto target, atomically when target
// supports batches.
func (store *StagedStore) ApplyTo(ctx context.Context, target StateStore) error {
	if len(store.writes) == 0 {
		return nil
	}
	if batch, ok := target.(kvbatch.BatchKVStore); ok {
		return batch.SetBatch(ctx, store.Writes())
	}
	for _, write := range store.writes {
		var err error
		if write.Delete {
			err = target.Delete(ctx, write.Namespace, write.Key)
		} else {
			err = target.Set(ctx, write.Namespace, write.Key, write.Value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (store *StagedStore) Root(ctx context.Context, namespace string) (types.Hash, error) {
	return store.RootWithWrites(ctx, namespace, nil)
}

// RootWithWrites computes the root of namespace over the base store with the
// staged writes and then extra applied.
func (store *StagedStore) RootWithWrites(ctx context.Context, namespace string, extra []kvbatch.KVWrite) (types.Hash, error) {
	if err := store.ready(); err != nil {
		return types.Hash{}, err
	}
	writes := store.writes
	if len(extra) > 0 {
		writes = append(append(make([]kvbatch.KVWrite, 0, len(store.writes)+len(extra)), store.writes...), extra...)
	}
	if rooter, ok := store.base.(rootWithWritesStore); ok {
		root, err := rooter.RootWithWrites(ctx, namespace, writes)
		return root, store.observe(err)
	}
	rooter, ok := store.base.(StateRootStore)
	if !ok {
		return types.Hash{}, errors.New("staged base store cannot compute roots")
	}
	if len(writes) > 0 {
		return types.Hash{}, errors.New("staged base store cannot compute roots over staged writes")
	}
	root, err := rooter.Root(ctx, namespace)
	return root, store.observe(err)
}

func (store *StagedStore) ExportPrefix(ctx context.Context, namespace string, prefix []byte) ([]vexostore.KVPair, error) {
	if err := store.ready(); err != nil {
		return nil, err
	}
	prefixStore, ok := store.base.(vexostore.PrefixKVStore)
	if !ok {
		return nil, errors.New("staged base store cannot export prefixes")
	}
	pairs, err := prefixStore.ExportPrefix(ctx, namespace, prefix)
	if err != nil {
		return nil, store.observe(err)
	}
	rawPrefix := string(prefix)
	return store.merge(pairs, namespace, func(key string) bool { return strings.HasPrefix(key, rawPrefix) }), nil
}

func (store *StagedStore) ExportNamespace(ctx context.Context, namespace string) ([]vexostore.KVPair, error) {
	if err := store.ready(); err != nil {
		return nil, err
	}
	snapshot, ok := store.base.(vexostore.SnapshotKVStore)
	if !ok {
		return nil, errors.New("staged base store cannot export namespaces")
	}
	pairs, err := snapshot.ExportNamespace(ctx, namespace)
	if err != nil {
		return nil, store.observe(err)
	}
	return store.merge(pairs, namespace, func(string) bool { return true }), nil
}

// merge applies the staged writes of namespace that match to the base pairs and
// returns the result in key order.
func (store *StagedStore) merge(pairs []vexostore.KVPair, namespace string, match func(key string) bool) []vexostore.KVPair {
	merged := make(map[string]vexostore.KVPair, len(pairs)+len(store.overlay))
	for _, pair := range pairs {
		merged[string(pair.Key)] = vexostore.KVPair{
			Namespace: pair.Namespace,
			Key:       append([]byte(nil), pair.Key...),
			Value:     append([]byte(nil), pair.Value...),
		}
	}
	for _, write := range store.overlay {
		key := string(write.Key)
		if write.Namespace != namespace || !match(key) {
			continue
		}
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
	return result
}

func (store *StagedStore) ImportNamespace(ctx context.Context, namespace string, pairs []vexostore.KVPair) error {
	if err := store.ready(); err != nil {
		return err
	}
	existing, err := store.ExportNamespace(ctx, namespace)
	if err != nil {
		return err
	}
	writes := make([]kvbatch.KVWrite, 0, len(existing)+len(pairs))
	for _, pair := range existing {
		writes = append(writes, kvbatch.KVWrite{Namespace: namespace, Key: pair.Key, Delete: true})
	}
	for _, pair := range pairs {
		writes = append(writes, kvbatch.KVWrite{Namespace: namespace, Key: pair.Key, Value: pair.Value})
	}
	return store.SetBatch(ctx, writes)
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

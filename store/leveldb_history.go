package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/vexo-network/vexo-consensus/types"
)

// State history.
//
// Every change to a key after the chain's first committed state is recorded as
// a version of that key at the height whose state it belongs to: a committed
// block's writes at its height, a direct write at the height of the next
// committed state. The first change of a key also records the value the key
// held before it, as its version at height 0. So for every height with a
// committed state record, the value of a key at that height is:
//
//   - the value of its highest version at or below the height, when it has a
//     version there (a deleted version means absent);
//   - absent, when it has versions but none at or below the height (the key was
//     deleted by a pruned version);
//   - its current value, when it has no version at all (it never changed since
//     the base state: genesis, or the namespace an import installed).
//
// The store therefore keeps one copy of the state plus one version per change;
// historical reads never need full copies of a namespace.

// historyBatch records the versions of the writes one batch applies.
type historyBatch struct {
	store    *LevelDBStore
	batch    *leveldb.Batch
	height   types.Height
	baseline map[string]struct{}
}

func newHistoryBatch(store *LevelDBStore, batch *leveldb.Batch, height types.Height) *historyBatch {
	return &historyBatch{store: store, batch: batch, height: height, baseline: make(map[string]struct{})}
}

// record adds the version write makes, and the key's version at height 0 when
// this is its first change.
func (history *historyBatch) record(write KVWrite) error {
	prefix := kvHistoryPrefixKey(write.Namespace, write.Key)
	if _, done := history.baseline[string(prefix)]; !done {
		history.baseline[string(prefix)] = struct{}{}
		versioned, err := history.store.hasVersions(prefix)
		if err != nil {
			return err
		}
		if !versioned {
			prior := kvHistoryRecord{Height: 0, Namespace: write.Namespace, Key: append([]byte(nil), write.Key...)}
			value, err := history.store.db.Get(kvKey(write.Namespace, write.Key), nil)
			switch {
			case errors.Is(err, leveldberrors.ErrNotFound):
				prior.Deleted = true
			case err != nil:
				return err
			default:
				prior.Value = append([]byte(nil), value...)
			}
			if err := history.put(prior); err != nil {
				return err
			}
		}
	}
	return history.put(kvHistoryRecord{
		Height:    history.height,
		Namespace: write.Namespace,
		Key:       append([]byte(nil), write.Key...),
		Value:     append([]byte(nil), write.Value...),
		Deleted:   write.Delete,
	})
}

func (history *historyBatch) put(record kvHistoryRecord) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	history.batch.Put(kvHistoryKey(record.Height, record.Namespace, record.Key), encoded)
	return nil
}

func (store *LevelDBStore) hasVersions(prefix []byte) (bool, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()
	found := iterator.First()
	return found, iterator.Error()
}

// directWriteHeight is the height a direct write takes effect at: the height of
// the next committed state. Before the first committed state there is no
// history; such writes form the base state.
func (store *LevelDBStore) directWriteHeight() (types.Height, bool, error) {
	encoded, err := store.db.Get(stateLatestKey, nil)
	if errors.Is(err, leveldberrors.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var state StateRecord
	if err := json.Unmarshal(encoded, &state); err != nil {
		return 0, false, err
	}
	if state.Height == 0 || state.Height == math.MaxUint64 {
		return 0, false, ErrInvalidStateRecord
	}
	return state.Height + 1, true, nil
}

// applyDirectWrites applies writes outside a block commit, atomically, and
// records their versions.
func (store *LevelDBStore) applyDirectWrites(ctx context.Context, writes []KVWrite) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	for _, write := range writes {
		if write.Namespace == "" {
			return ErrInvalidNamespace
		}
		if len(write.Key) == 0 {
			return ErrInvalidKey
		}
	}
	store.writeMu.Lock()
	defer store.writeMu.Unlock()
	height, versioned, err := store.directWriteHeight()
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	history := newHistoryBatch(store, batch, height)
	for _, write := range writes {
		if versioned {
			if err := history.record(write); err != nil {
				return err
			}
		}
		key := kvKey(write.Namespace, write.Key)
		if write.Delete {
			batch.Delete(key)
			continue
		}
		batch.Put(key, append([]byte(nil), write.Value...))
	}
	return store.writeBatch(batch)
}

// requireCommittedState refuses a historical read at a height whose state the
// store does not hold: never committed, pruned, or before a restored snapshot.
func (store *LevelDBStore) requireCommittedState(height types.Height) error {
	if height == 0 {
		return ErrStateNotFound
	}
	found, err := store.db.Has(stateHeightKey(height), nil)
	if err != nil {
		return err
	}
	if !found {
		return ErrStateNotFound
	}
	return nil
}

func historySeekKey(prefix []byte, height types.Height) ([]byte, bool) {
	if height == math.MaxUint64 {
		return nil, false
	}
	key := append([]byte(nil), prefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height)+1)
	return append(key, buffer[:]...), true
}

// versionAt positions iterator, which ranges over the versions of one or more
// keys, on the highest version of the key with prefix at or below height, and
// reports whether there is one.
func versionAt(iterator interface {
	Seek([]byte) bool
	Prev() bool
	Last() bool
	Key() []byte
}, prefix []byte, height types.Height) bool {
	var positioned bool
	if seek, ok := historySeekKey(prefix, height); ok && iterator.Seek(seek) {
		positioned = iterator.Prev()
	} else {
		positioned = iterator.Last()
	}
	if !positioned {
		return false
	}
	_, ok := kvHistoryHeightFromKey(prefix, iterator.Key())
	return ok
}

func decodeHistoryRecord(prefix []byte, rawKey []byte, rawValue []byte, namespace string) (kvHistoryRecord, error) {
	height, ok := kvHistoryHeightFromKey(prefix, rawKey)
	if !ok {
		return kvHistoryRecord{}, ErrInvalidStateRecord
	}
	var record kvHistoryRecord
	if err := json.Unmarshal(rawValue, &record); err != nil {
		return kvHistoryRecord{}, err
	}
	if record.Height != height || record.Namespace != namespace || len(record.Key) == 0 ||
		!bytes.Equal(kvHistoryPrefixKey(namespace, record.Key), prefix) {
		return kvHistoryRecord{}, ErrInvalidStateRecord
	}
	return record, nil
}

func (store *LevelDBStore) GetAt(ctx context.Context, height types.Height, namespace string, key []byte) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	if len(key) == 0 {
		return nil, ErrInvalidKey
	}
	if err := store.requireCommittedState(height); err != nil {
		return nil, err
	}
	prefix := kvHistoryPrefixKey(namespace, key)
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()
	if !iterator.First() {
		if err := iterator.Error(); err != nil {
			return nil, err
		}
		return store.Get(ctx, namespace, key)
	}
	if !versionAt(iterator, prefix, height) {
		if err := iterator.Error(); err != nil {
			return nil, err
		}
		return nil, ErrKeyNotFound
	}
	record, err := decodeHistoryRecord(prefix, iterator.Key(), iterator.Value(), namespace)
	if err != nil {
		return nil, err
	}
	if record.Deleted {
		return nil, ErrKeyNotFound
	}
	return append([]byte(nil), record.Value...), nil
}

func (store *LevelDBStore) ExportNamespaceAt(ctx context.Context, height types.Height, namespace string) ([]KVPair, error) {
	return store.ExportPrefixAt(ctx, height, namespace, nil)
}

// ExportPrefixAt returns the pairs of namespace whose key starts with keyPrefix
// as they were at height, sorted by key. Its cost is one seek per key that ever
// changed under the prefix plus a scan of the prefix's current pairs, however
// many versions the keys have.
func (store *LevelDBStore) ExportPrefixAt(ctx context.Context, height types.Height, namespace string, keyPrefix []byte) ([]KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	if err := store.requireCommittedState(height); err != nil {
		return nil, err
	}
	namespacePrefix := kvHistoryNamespacePrefix(namespace)
	rangePrefix := append(append([]byte(nil), namespacePrefix...), []byte(hex.EncodeToString(keyPrefix))...)
	iterator := store.db.NewIterator(util.BytesPrefix(rangePrefix), nil)
	defer iterator.Release()

	versioned := make(map[string]struct{})
	pairs := make([]KVPair, 0)
	for valid := iterator.First(); valid; {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		keyPrefixOfVersion, rawKey, ok := historyKeyGroup(namespacePrefix, iterator.Key())
		if !ok {
			return nil, ErrInvalidStateRecord
		}
		versioned[string(rawKey)] = struct{}{}
		if versionAt(iterator, keyPrefixOfVersion, height) {
			record, err := decodeHistoryRecord(keyPrefixOfVersion, iterator.Key(), iterator.Value(), namespace)
			if err != nil {
				return nil, err
			}
			if !record.Deleted {
				pairs = append(pairs, KVPair{Namespace: namespace, Key: record.Key, Value: record.Value})
			}
		}
		valid = iterator.Seek(nextHistoryKeyGroup(keyPrefixOfVersion))
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	current, err := store.ExportPrefix(ctx, namespace, keyPrefix)
	if err != nil {
		return nil, err
	}
	for _, pair := range current {
		if _, found := versioned[string(pair.Key)]; found {
			continue
		}
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(i, j int) bool { return bytes.Compare(pairs[i].Key, pairs[j].Key) < 0 })
	return pairs, nil
}

func kvHistoryNamespacePrefix(namespace string) []byte {
	prefix := append([]byte(nil), kvHistoryPrefix...)
	prefix = append(prefix, []byte(namespace)...)
	return append(prefix, ':')
}

// historyKeyGroup splits the database key of a version into the prefix shared
// by every version of its key, and the key itself.
func historyKeyGroup(namespacePrefix []byte, dbKey []byte) ([]byte, []byte, bool) {
	if len(dbKey) < len(namespacePrefix)+9 || !bytes.HasPrefix(dbKey, namespacePrefix) {
		return nil, nil, false
	}
	groupEnd := len(dbKey) - 8
	if dbKey[groupEnd-1] != ':' {
		return nil, nil, false
	}
	key, err := hex.DecodeString(string(dbKey[len(namespacePrefix) : groupEnd-1]))
	if err != nil || len(key) == 0 {
		return nil, nil, false
	}
	return append([]byte(nil), dbKey[:groupEnd]...), key, true
}

// nextHistoryKeyGroup is the smallest database key after every version sharing
// prefix. A key group ends in ':', which no hex digit of a longer key can
// follow, so ';' bounds the group exactly.
func nextHistoryKeyGroup(prefix []byte) []byte {
	next := append([]byte(nil), prefix...)
	next[len(next)-1] = ';'
	return next
}

// pruneHistoryBelow drops the versions no retained height can read. A key keeps
// its versions from retainFrom on, plus its highest version below retainFrom
// when that is a value and the key has no version at retainFrom itself, so the
// heights from retainFrom to its next version still read it. A key whose
// highest version below retainFrom is a deletion keeps none of them: having no
// version at or below a height reads as absent. A key with no version from
// retainFrom on keeps none at all: its current value is its value at every
// retained height.
func (store *LevelDBStore) pruneHistoryBelow(ctx context.Context, batch *leveldb.Batch, retainFrom types.Height) (uint64, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(kvHistoryPrefix), nil)
	defer iterator.Release()

	var pruned uint64
	var group []byte
	var below [][]byte
	var top []byte
	var topValue []byte
	var retained, replacedAtRetainFrom bool
	flush := func() error {
		if len(below) == 0 {
			return nil
		}
		keepTop := retained && !replacedAtRetainFrom
		if keepTop {
			var record kvHistoryRecord
			if err := json.Unmarshal(topValue, &record); err != nil {
				return err
			}
			keepTop = !record.Deleted
		}
		for _, key := range below {
			if keepTop && bytes.Equal(key, top) {
				continue
			}
			batch.Delete(key)
			pruned++
		}
		return nil
	}
	for iterator.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		dbKey := iterator.Key()
		if len(dbKey) < len(kvHistoryPrefix)+9 {
			return 0, ErrInvalidStateRecord
		}
		keyGroup := dbKey[:len(dbKey)-8]
		if !bytes.Equal(keyGroup, group) {
			if err := flush(); err != nil {
				return 0, err
			}
			group = append(group[:0], keyGroup...)
			below = below[:0]
			top, topValue, retained, replacedAtRetainFrom = nil, nil, false, false
		}
		height := types.Height(binary.BigEndian.Uint64(dbKey[len(dbKey)-8:]))
		if height >= retainFrom {
			retained = true
			replacedAtRetainFrom = replacedAtRetainFrom || height == retainFrom
			continue
		}
		top = append([]byte(nil), dbKey...)
		topValue = append([]byte(nil), iterator.Value()...)
		below = append(below, top)
	}
	if err := iterator.Error(); err != nil {
		return 0, err
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return pruned, nil
}

// deleteNamespaceHistory drops every version of namespace.
func (store *LevelDBStore) deleteNamespaceHistory(ctx context.Context, batch *leveldb.Batch, namespace string) error {
	iterator := store.db.NewIterator(util.BytesPrefix(kvHistoryNamespacePrefix(namespace)), nil)
	defer iterator.Release()
	for iterator.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...))
	}
	return iterator.Error()
}

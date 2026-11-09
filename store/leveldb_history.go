package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"

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
	putVersion(history.batch, kvVersionIndexPrefix, kvHistoryKey(record.Height, record.Namespace, record.Key), record.Height, encoded)
	return nil
}

// Version index.
//
// Every version is also listed under its height in a version index (kvVersionIndexPrefix for the
// versions of keys, stateTreeVersionIndexPrefix for those of tree nodes), so pruning visits the
// versions below the height it retains from, not every version. A version a prune keeps, because
// the heights from retainFrom on still read it, is listed again under the last height it is read
// at, below the version that replaces it, so each version is visited at most twice whatever the
// size of the history. Versions dropped with their namespace (an import) leave entries that the
// next prune drops.
var (
	kvVersionIndexPrefix        = []byte("hidx:")
	stateTreeVersionIndexPrefix = []byte("tidx:")
)

func versionIndexKey(indexPrefix []byte, height types.Height, versionKey []byte) []byte {
	key := make([]byte, 0, len(indexPrefix)+8+len(versionKey))
	key = append(key, indexPrefix...)
	key = binary.BigEndian.AppendUint64(key, uint64(height))
	return append(key, versionKey...)
}

// putVersion adds to batch a version and its entry in the version index under its height.
func putVersion(batch *leveldb.Batch, indexPrefix []byte, versionKey []byte, height types.Height, value []byte) {
	batch.Put(versionKey, value)
	batch.Put(versionIndexKey(indexPrefix, height, versionKey), nil)
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
	trees := newStateTrees(store.db)
	for _, write := range writes {
		if versioned {
			if err := history.record(write); err != nil {
				return err
			}
		}
		if err := trees.apply(write); err != nil {
			return err
		}
		key := kvKey(write.Namespace, write.Key)
		if write.Delete {
			batch.Delete(key)
			continue
		}
		batch.Put(key, append([]byte(nil), write.Value...))
	}
	if err := trees.persist(batch, versioned, height); err != nil {
		return err
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
	return getAt(store.db, height, namespace, key)
}

// getAt reads the value of key at a committed height from reader.
func getAt(reader dbReader, height types.Height, namespace string, key []byte) ([]byte, error) {
	prefix := kvHistoryPrefixKey(namespace, key)
	iterator := reader.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()
	if !iterator.First() {
		if err := iterator.Error(); err != nil {
			return nil, err
		}
		value, err := reader.Get(kvKey(namespace, key), nil)
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), value...), nil
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
	pairs := make([]KVPair, 0)
	err := store.historicalPairs(ctx, height, namespace, keyPrefix, nil, func(pair KVPair) bool {
		pairs = append(pairs, pair)
		return true
	})
	if err != nil {
		return nil, err
	}
	return pairs, nil
}

// ExportRangeAt returns at most limit pairs of namespace as they were at height, sorted by key,
// starting with the first key after after (the first key of the namespace for nil after). more
// reports that pairs follow the last one returned. A page costs a seek per key it returns, so a
// caller can stream a namespace of any size at a past height in bounded memory.
func (store *LevelDBStore) ExportRangeAt(ctx context.Context, height types.Height, namespace string, after []byte, limit int) (pairs []KVPair, more bool, err error) {
	if limit <= 0 {
		return nil, false, ErrInvalidKey
	}
	pairs = make([]KVPair, 0, limit)
	err = store.historicalPairs(ctx, height, namespace, nil, after, func(pair KVPair) bool {
		if len(pairs) == limit {
			more = true
			return false
		}
		pairs = append(pairs, pair)
		return true
	})
	if err != nil {
		return nil, false, err
	}
	return pairs, more, nil
}

// historicalPairs calls emit, in key order, with every pair of namespace under keyPrefix as it
// was at height whose key sorts after after (every key for nil after), until emit returns false.
//
// It merges two streams that are both in key order: the version groups of the keys that ever
// changed, whose value at height is their highest version at or below it, and the current pairs,
// which hold the value of every key that never changed.
func (store *LevelDBStore) historicalPairs(ctx context.Context, height types.Height, namespace string, keyPrefix []byte, after []byte, emit func(KVPair) bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if namespace == "" {
		return ErrInvalidNamespace
	}
	if err := store.requireCommittedState(height); err != nil {
		return err
	}
	snapshot, err := store.db.GetSnapshot()
	if err != nil {
		return err
	}
	defer snapshot.Release()

	namespacePrefix := kvHistoryNamespacePrefix(namespace)
	historyRange := util.BytesPrefix(appendOrderedKeyBody(append([]byte(nil), namespacePrefix...), keyPrefix))
	currentRange := util.BytesPrefix(append(kvNamespacePrefix(namespace), keyPrefix...))
	if after != nil {
		if start := nextHistoryKeyGroup(kvHistoryPrefixKey(namespace, after)); bytes.Compare(start, historyRange.Start) > 0 {
			historyRange.Start = start
		}
		if start := append(kvKey(namespace, after), 0x00); bytes.Compare(start, currentRange.Start) > 0 {
			currentRange.Start = start
		}
	}
	history := snapshot.NewIterator(historyRange, nil)
	defer history.Release()
	current := snapshot.NewIterator(currentRange, nil)
	defer current.Release()
	currentPrefix := len(kvNamespacePrefix(namespace))

	type versioned struct {
		key     []byte
		value   []byte
		present bool
	}
	nextVersioned := func(valid bool) (versioned, bool, error) {
		if !valid {
			return versioned{}, false, history.Error()
		}
		group, key, ok := historyKeyGroup(namespacePrefix, history.Key())
		if !ok {
			return versioned{}, false, ErrInvalidStateRecord
		}
		entry := versioned{key: key}
		if versionAt(history, group, height) {
			record, err := decodeHistoryRecord(group, history.Key(), history.Value(), namespace)
			if err != nil {
				return versioned{}, false, err
			}
			entry.present = !record.Deleted
			entry.value = record.Value
		}
		history.Seek(nextHistoryKeyGroup(group))
		return entry, true, nil
	}

	version, versionOK, err := nextVersioned(history.First())
	if err != nil {
		return err
	}
	currentOK := current.First()
	for versionOK || currentOK {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var currentKey []byte
		if currentOK {
			currentKey = current.Key()[currentPrefix:]
		}
		if versionOK && (!currentOK || bytes.Compare(version.key, currentKey) <= 0) {
			if currentOK && bytes.Equal(version.key, currentKey) {
				currentOK = current.Next()
			}
			if version.present && !emit(KVPair{Namespace: namespace, Key: version.key, Value: version.value}) {
				return nil
			}
			valid := history.Valid()
			if version, versionOK, err = nextVersioned(valid); err != nil {
				return err
			}
			continue
		}
		pair := KVPair{Namespace: namespace, Key: append([]byte(nil), currentKey...), Value: append([]byte(nil), current.Value()...)}
		currentOK = current.Next()
		if !emit(pair) {
			return nil
		}
	}
	if err := current.Error(); err != nil {
		return err
	}
	return history.Error()
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
	key, ok := decodeOrderedKey(dbKey[len(namespacePrefix):groupEnd])
	if !ok {
		return nil, nil, false
	}
	return append([]byte(nil), dbKey[:groupEnd]...), key, true
}

// nextHistoryKeyGroup is the smallest database key after every version sharing
// prefix. A key group ends in the terminator 0x00 0x01 of its ordered key, and
// only heights follow it, so raising the terminator bounds the group exactly.
func nextHistoryKeyGroup(prefix []byte) []byte {
	next := append([]byte(nil), prefix...)
	next[len(next)-1]++
	return next
}

// pruneHistoryBelow drops the versions no retained height can read. A key keeps
// its versions from retainFrom on, plus its highest version below retainFrom
// when that is a value and the key has no version at retainFrom itself, so the
// heights from retainFrom to its next version still read it. A key whose
// highest version below retainFrom is a deletion keeps none of them: having no
// version at or below a height reads as absent. A key with no version from
// retainFrom on keeps none at all: its current value is its value at every
// retained height. Its cost is in the versions below retainFrom (see Version
// index), not in the size of the history.
//
// The versions of the state tree nodes are pruned by the same rule.
func (store *LevelDBStore) pruneHistoryBelow(ctx context.Context, batch *leveldb.Batch, retainFrom types.Height) (uint64, uint64, error) {
	pruned, err := store.pruneVersionsBelow(ctx, batch, kvVersionIndexPrefix, retainFrom, func(value []byte) (bool, error) {
		var record kvHistoryRecord
		if err := json.Unmarshal(value, &record); err != nil {
			return false, err
		}
		return record.Deleted, nil
	})
	if err != nil {
		return 0, 0, err
	}
	prunedNodes, err := store.pruneVersionsBelow(ctx, batch, stateTreeVersionIndexPrefix, retainFrom, func(value []byte) (bool, error) {
		return len(value) == 0, nil
	})
	if err != nil {
		return 0, 0, err
	}
	return pruned, prunedNodes, nil
}

// pruneVersionsBelow prunes the versions the version index under indexPrefix lists below
// retainFrom. A version's database key is the prefix every version of its key shares followed by
// its big-endian height; deleted reports whether a version is a deletion. A version is kept only
// when it is the highest version of its key below retainFrom, a value, and followed by a version
// above retainFrom: the heights from retainFrom up to that version read it. Without a following
// version the current value is read, and a version at retainFrom replaces it.
func (store *LevelDBStore) pruneVersionsBelow(ctx context.Context, batch *leveldb.Batch, indexPrefix []byte, retainFrom types.Height, deleted func([]byte) (bool, error)) (uint64, error) {
	entries := store.db.NewIterator(&util.Range{Start: indexPrefix, Limit: versionIndexKey(indexPrefix, retainFrom, nil)}, nil)
	defer entries.Release()
	var pruned uint64
	for entries.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		entry := entries.Key()
		if len(entry) < len(indexPrefix)+8+9 {
			return 0, ErrInvalidStateRecord
		}
		batch.Delete(append([]byte(nil), entry...))
		versionKey := append([]byte(nil), entry[len(indexPrefix)+8:]...)
		value, err := store.db.Get(versionKey, nil)
		if errors.Is(err, leveldberrors.ErrNotFound) {
			continue
		}
		if err != nil {
			return 0, err
		}
		group, height := versionKey[:len(versionKey)-8], types.Height(binary.BigEndian.Uint64(versionKey[len(versionKey)-8:]))
		next, found, err := store.nextVersionHeight(group, height)
		if err != nil {
			return 0, err
		}
		if found && next > retainFrom {
			isDeleted, err := deleted(value)
			if err != nil {
				return 0, err
			}
			if !isDeleted {
				// Visited again by the first prune that retains from next on.
				batch.Put(versionIndexKey(indexPrefix, next-1, versionKey), nil)
				continue
			}
		}
		batch.Delete(versionKey)
		pruned++
	}
	if err := entries.Error(); err != nil {
		return 0, err
	}
	return pruned, nil
}

// nextVersionHeight returns the height of the version of group that follows the one at height.
func (store *LevelDBStore) nextVersionHeight(group []byte, height types.Height) (types.Height, bool, error) {
	if height == math.MaxUint64 {
		return 0, false, nil
	}
	versions := store.db.NewIterator(util.BytesPrefix(group), nil)
	defer versions.Release()
	if !versions.Seek(binary.BigEndian.AppendUint64(append([]byte(nil), group...), uint64(height)+1)) {
		return 0, false, versions.Error()
	}
	key := versions.Key()
	if len(key) != len(group)+8 {
		return 0, false, ErrInvalidStateRecord
	}
	return types.Height(binary.BigEndian.Uint64(key[len(group):])), true, nil
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

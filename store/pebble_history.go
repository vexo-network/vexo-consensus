package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/cockroachdb/pebble/v2"
	"math"

	"github.com/vexo-network/vexo-consensus/types"
)

// State history.
//
// History is an undo log. When a key changes at a height (a committed block's
// writes at its height, a direct write after the first committed state at the
// height of the next committed state), the store records the value the key held
// before, absent included, as the key's version at that height, once per key and
// height. So for every height with a committed state record, the value of a key
// at that height is:
//
//   - the value its lowest version above the height records, when it has one
//     (a deleted version means absent): the key's first change after the height
//     recorded what it held then;
//   - its current value, when it has none: it has not changed since.
//
// Recording a change reads only the key's current value, never its history, and
// a read is one seek into the key's versions. The store keeps one copy of the
// state plus one version per change; historical reads never need full copies of
// a namespace.

// historyBatch records the versions of the writes one batch applies.
type historyBatch struct {
	store  *PebbleStore
	batch  *pebble.Batch
	height types.Height
	// existing reports that versions at height may already be stored: direct writes at height
	// committed before this batch. A stored version holds the value before the first of them and
	// is kept.
	existing bool
	recorded map[string]struct{}
	versions *keyList
}

func newHistoryBatch(store *PebbleStore, batch *pebble.Batch, height types.Height, existing bool) *historyBatch {
	return &historyBatch{
		store:    store,
		batch:    batch,
		height:   height,
		existing: existing,
		recorded: make(map[string]struct{}),
		versions: newKeyList(kvVersionIndexPrefix, height),
	}
}

// finish lists the recorded versions in the version index.
func (history *historyBatch) finish() {
	history.versions.finish(history.batch)
}

// put adds the version stored at versionKey, a key under kvHistoryPrefix that ends in the batch's
// height, and lists it.
func (history *historyBatch) put(versionKey []byte, value []byte) {
	history.batch.Set(versionKey, value, nil)
	history.versions.add(versionKey[len(kvHistoryPrefix) : len(versionKey)-8])
}

// record adds the version of the key write changes: the value it holds before the batch, once per
// key.
func (history *historyBatch) record(write KVWrite) error {
	prefix := kvHistoryPrefixKey(write.Namespace, write.Key)
	if _, done := history.recorded[string(prefix)]; done {
		return nil
	}
	history.recorded[string(prefix)] = struct{}{}
	versionKey := kvHistoryKey(history.height, write.Namespace, write.Key)
	if history.existing {
		found, err := hasKey(history.store.db, versionKey)
		if err != nil || found {
			return err
		}
	}
	prior := kvHistoryRecord{Height: history.height, Namespace: write.Namespace, Key: append([]byte(nil), write.Key...)}
	dbKey := kvKey(write.Namespace, write.Key)
	// The block's execution usually read the key; its value then is the key's current value.
	if value, found := history.store.reads.value(dbKey); found {
		prior.Value = value
		history.put(versionKey, encodeKVHistoryRecord(prior))
		return nil
	}
	value, err := getValue(history.store.db, dbKey)
	switch {
	case errors.Is(err, pebble.ErrNotFound):
		prior.Deleted = true
	case err != nil:
		return err
	default:
		prior.Value = value
	}
	history.put(versionKey, encodeKVHistoryRecord(prior))
	return nil
}

// historyDirectHeightKey holds the height of the last direct writes that recorded versions, so the
// block that commits that height keeps the versions they recorded.
var historyDirectHeightKey = []byte("hdirect")

// directWritesAt reports whether direct writes recorded versions at height.
func (store *PebbleStore) directWritesAt(height types.Height) (bool, error) {
	encoded, err := getValue(store.db, historyDirectHeightKey)
	if errors.Is(err, pebble.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(encoded) != 8 {
		return false, ErrInvalidStateRecord
	}
	return types.Height(binary.BigEndian.Uint64(encoded)) == height, nil
}

// Version index.
//
// The versions of keys one batch records at one height are listed in the version index
// (kvVersionIndexPrefix) as one key list (see keyList) whose entries are the versions' groups: a
// version key without its history prefix and height. A version is read only by the heights below
// it, so a prune that retains from a height drops exactly the versions listed at or below it,
// visiting each version once whatever the size of the history, and one list per batch keeps the
// index from doubling the entries a block writes. Versions dropped with their namespace (an import)
// leave groups that the next prune skips.
var kvVersionIndexPrefix = []byte("hidx:")

// keyList collects keys a batch lists under a prefix at one height, and adds them to the batch as
// one entry:
//
//	prefix | height | first key  ->  repeated uvarint(len(key)) | key
//
// Two batches at one height never list the same first key: a later batch at a height lists only
// keys the earlier ones did not (a version not recorded yet, a node not orphaned yet).
type keyList struct {
	prefix  []byte
	height  types.Height
	first   []byte
	entries []byte
}

func newKeyList(prefix []byte, height types.Height) *keyList {
	return &keyList{prefix: prefix, height: height}
}

func keyListKey(prefix []byte, height types.Height, first []byte) []byte {
	key := make([]byte, 0, len(prefix)+8+len(first))
	key = append(key, prefix...)
	key = binary.BigEndian.AppendUint64(key, uint64(height))
	return append(key, first...)
}

// add lists key, which the caller does not modify afterwards.
func (list *keyList) add(key []byte) {
	if list.first == nil {
		list.first = key
	}
	list.entries = binary.AppendUvarint(list.entries, uint64(len(key)))
	list.entries = append(list.entries, key...)
}

// finish adds the list, if it holds a key, to batch.
func (list *keyList) finish(batch *pebble.Batch) {
	if list.first == nil {
		return
	}
	batch.Set(keyListKey(list.prefix, list.height, list.first), list.entries, nil)
}

// eachListedKey calls visit with every key of an encoded key list, in order.
func eachListedKey(entries []byte, visit func(key []byte)) error {
	for len(entries) > 0 {
		size, read := binary.Uvarint(entries)
		if read <= 0 || size == 0 || uint64(len(entries)-read) < size {
			return ErrInvalidStateRecord
		}
		visit(entries[read : read+int(size)])
		entries = entries[read+int(size):]
	}
	return nil
}

// directWriteHeight is the height a direct write takes effect at: the height of
// the next committed state. Before the first committed state there is no
// history; such writes form the base state.
func (store *PebbleStore) directWriteHeight() (types.Height, bool, error) {
	encoded, err := getValue(store.db, stateLatestKey)
	if errors.Is(err, pebble.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	state, err := decodeStateRecord(encoded)
	if err != nil {
		return 0, false, err
	}
	if state.Height == 0 || state.Height == math.MaxUint64 {
		return 0, false, ErrInvalidStateRecord
	}
	return state.Height + 1, true, nil
}

// applyDirectWrites applies writes outside a block commit, atomically, and
// records their versions.
func (store *PebbleStore) applyDirectWrites(ctx context.Context, writes []KVWrite) error {
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
	batch := store.db.NewBatch()
	defer batch.Close()
	history := newHistoryBatch(store, batch, height, true)
	trees := newStateTrees(store.db, store.liveTreeView())
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
			batch.Delete(key, nil)
			continue
		}
		batch.Set(key, append([]byte(nil), write.Value...), nil)
	}
	history.finish()
	if err := trees.persist(batch, height); err != nil {
		return err
	}
	if versioned {
		batch.Set(historyDirectHeightKey, binary.BigEndian.AppendUint64(nil, uint64(height)), nil)
	}
	return store.writeBatch(batch)
}

// requireCommittedState refuses a historical read at a height whose state the
// store does not hold: never committed, pruned, or before a restored snapshot.
func (store *PebbleStore) requireCommittedState(height types.Height) error {
	if height == 0 {
		return ErrStateNotFound
	}
	found, err := hasKey(store.db, stateHeightKey(height))
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

// versionAbove positions iterator, which ranges over the versions of one or more
// keys, on the lowest version of the key with prefix above height, and reports
// whether there is one: that version records the key's value at height.
func versionAbove(iterator interface {
	SeekGE([]byte) bool
	Key() []byte
}, prefix []byte, height types.Height) bool {
	seek, ok := historySeekKey(prefix, height)
	if !ok || !iterator.SeekGE(seek) {
		return false
	}
	_, ok = kvHistoryHeightFromKey(prefix, iterator.Key())
	return ok
}

func decodeHistoryRecord(prefix []byte, rawKey []byte, rawValue []byte, namespace string) (kvHistoryRecord, error) {
	height, ok := kvHistoryHeightFromKey(prefix, rawKey)
	if !ok {
		return kvHistoryRecord{}, ErrInvalidStateRecord
	}
	record, err := decodeKVHistoryRecord(rawValue)
	if err != nil {
		return kvHistoryRecord{}, err
	}
	if record.Height != height || record.Namespace != namespace || len(record.Key) == 0 ||
		!bytes.Equal(kvHistoryPrefixKey(namespace, record.Key), prefix) {
		return kvHistoryRecord{}, ErrInvalidStateRecord
	}
	return record, nil
}

func (store *PebbleStore) GetAt(ctx context.Context, height types.Height, namespace string, key []byte) ([]byte, error) {
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
	iterator := newIter(reader, prefixRange(prefix))
	defer iterator.Close()
	if !versionAbove(iterator, prefix, height) {
		if err := iterator.Error(); err != nil {
			return nil, err
		}
		value, err := getValue(reader, kvKey(namespace, key))
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		if err != nil {
			return nil, err
		}
		return value, nil
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

func (store *PebbleStore) ExportNamespaceAt(ctx context.Context, height types.Height, namespace string) ([]KVPair, error) {
	return store.ExportPrefixAt(ctx, height, namespace, nil)
}

// ExportPrefixAt returns the pairs of namespace whose key starts with keyPrefix
// as they were at height, sorted by key. Its cost is one seek per key that ever
// changed under the prefix plus a scan of the prefix's current pairs, however
// many versions the keys have.
func (store *PebbleStore) ExportPrefixAt(ctx context.Context, height types.Height, namespace string, keyPrefix []byte) ([]KVPair, error) {
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
func (store *PebbleStore) ExportRangeAt(ctx context.Context, height types.Height, namespace string, after []byte, limit int) (pairs []KVPair, more bool, err error) {
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
// changed, whose value at height is their lowest version above it when they have one, and the
// current pairs, which hold the value of every other key.
func (store *PebbleStore) historicalPairs(ctx context.Context, height types.Height, namespace string, keyPrefix []byte, after []byte, emit func(KVPair) bool) error {
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
	snapshot := store.db.NewSnapshot()
	defer snapshot.Close()

	namespacePrefix := kvHistoryNamespacePrefix(namespace)
	historyRange := prefixRange(appendOrderedKeyBody(append([]byte(nil), namespacePrefix...), keyPrefix))
	currentRange := prefixRange(append(kvNamespacePrefix(namespace), keyPrefix...))
	if after != nil {
		if start := nextHistoryKeyGroup(kvHistoryPrefixKey(namespace, after)); bytes.Compare(start, historyRange.lower) > 0 {
			historyRange.lower = start
		}
		if start := append(kvKey(namespace, after), 0x00); bytes.Compare(start, currentRange.lower) > 0 {
			currentRange.lower = start
		}
	}
	history := newIter(snapshot, historyRange)
	defer history.Close()
	current := newIter(snapshot, currentRange)
	defer current.Close()
	currentPrefix := len(kvNamespacePrefix(namespace))

	// versioned is the value of a key with versions at height: the one a version records, or the
	// current one when no version is above height.
	type versioned struct {
		key     []byte
		value   []byte
		present bool
		current bool
	}
	nextVersioned := func(valid bool) (versioned, bool, error) {
		if !valid {
			return versioned{}, false, history.Error()
		}
		group, key, ok := historyKeyGroup(namespacePrefix, history.Key())
		if !ok {
			return versioned{}, false, ErrInvalidStateRecord
		}
		entry := versioned{key: key, current: true}
		if versionAbove(history, group, height) {
			record, err := decodeHistoryRecord(group, history.Key(), history.Value(), namespace)
			if err != nil {
				return versioned{}, false, err
			}
			entry = versioned{key: key, value: record.Value, present: !record.Deleted}
		}
		history.SeekGE(nextHistoryKeyGroup(group))
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
				if version.current {
					version.value, version.present = append([]byte(nil), current.Value()...), true
				}
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

// pruneHistoryBelow drops the versions no retained height can read. A version records a key's
// value at the heights below it, so the heights from retainFrom on read only the versions above
// retainFrom, and every version at or below it goes. Its cost is in the versions it drops (see
// Version index), not in the size of the history. It drops the tree nodes no retained height reads
// as well (see pruneTreesBelow).
func (store *PebbleStore) pruneHistoryBelow(ctx context.Context, batch *pebble.Batch, retainFrom types.Height) (uint64, uint64, error) {
	pruned, err := store.pruneVersionsBelow(ctx, batch, retainFrom)
	if err != nil {
		return 0, 0, err
	}
	prunedNodes, err := store.pruneTreesBelow(ctx, batch, retainFrom)
	if err != nil {
		return 0, 0, err
	}
	return pruned, prunedNodes, nil
}

// pruneVersionsBelow drops the versions of keys the version index lists at or below retainFrom,
// with their index entries, and returns how many versions it dropped.
func (store *PebbleStore) pruneVersionsBelow(ctx context.Context, batch *pebble.Batch, retainFrom types.Height) (uint64, error) {
	span := prefixRange(kvVersionIndexPrefix)
	if retainFrom < math.MaxUint64 {
		span.upper = keyListKey(kvVersionIndexPrefix, retainFrom+1, nil)
	}
	entries := newIter(store.db, span)
	defer entries.Close()
	var pruned uint64
	for valid := entries.First(); valid; valid = entries.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		entry := entries.Key()
		if len(entry) < len(kvVersionIndexPrefix)+8+1 {
			return 0, ErrInvalidStateRecord
		}
		batch.Delete(append([]byte(nil), entry...), nil)
		height := entry[len(kvVersionIndexPrefix) : len(kvVersionIndexPrefix)+8]
		var failure error
		if err := eachListedKey(entries.Value(), func(group []byte) {
			if failure != nil {
				return
			}
			versionKey := make([]byte, 0, len(kvHistoryPrefix)+len(group)+8)
			versionKey = append(append(append(versionKey, kvHistoryPrefix...), group...), height...)
			found, err := hasKey(store.db, versionKey)
			if err != nil {
				failure = err
				return
			}
			if !found {
				// Dropped with its namespace by an import.
				return
			}
			batch.Delete(versionKey, nil)
			pruned++
		}); err != nil {
			return 0, err
		}
		if failure != nil {
			return 0, failure
		}
	}
	if err := entries.Error(); err != nil {
		return 0, err
	}
	return pruned, nil
}

// pruneTreesBelow drops the tree nodes orphaned at or below retainFrom, which only heights below it
// read, with their orphan lists, and every root record at or below retainFrom but the last of each
// namespace, which the heights from retainFrom on read. It returns how many nodes it dropped.
func (store *PebbleStore) pruneTreesBelow(ctx context.Context, batch *pebble.Batch, retainFrom types.Height) (uint64, error) {
	span := prefixRange(stateTreeOrphanPrefix)
	if retainFrom < math.MaxUint64 {
		span.upper = keyListKey(stateTreeOrphanPrefix, retainFrom+1, nil)
	}
	lists := newIter(store.db, span)
	defer lists.Close()
	var pruned uint64
	for valid := lists.First(); valid; valid = lists.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), lists.Key()...), nil)
		if err := eachListedKey(lists.Value(), func(entry []byte) {
			nodeKey := make([]byte, 0, len(stateTreePrefix)+len(entry))
			batch.Delete(append(append(nodeKey, stateTreePrefix...), entry...), nil)
			pruned++
		}); err != nil {
			return 0, err
		}
	}
	if err := lists.Error(); err != nil {
		return 0, err
	}

	records := newIter(store.db, prefixRange(stateTreeRootPrefix))
	defer records.Close()
	var namespace, last []byte
	for valid := records.First(); valid; {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		key := records.Key()
		if len(key) < len(stateTreeRootPrefix)+2+8 {
			return 0, ErrInvalidStateRecord
		}
		group := key[:len(key)-8]
		if !bytes.Equal(group, namespace) {
			namespace, last = append(namespace[:0], group...), nil
		}
		if types.Height(binary.BigEndian.Uint64(key[len(key)-8:])) > retainFrom {
			// The namespace's records from here on are retained; go on with the next namespace.
			next := binary.BigEndian.AppendUint64(append([]byte(nil), group...), math.MaxUint64)
			valid = records.SeekGE(next)
			if valid && bytes.Equal(records.Key(), next) {
				valid = records.Next()
			}
			continue
		}
		if last != nil {
			batch.Delete(last, nil)
		}
		last = append([]byte(nil), key...)
		valid = records.Next()
	}
	return pruned, records.Error()
}

// deleteNamespaceHistory drops every version of namespace.
func (store *PebbleStore) deleteNamespaceHistory(ctx context.Context, batch *pebble.Batch, namespace string) error {
	iterator := newIter(store.db, prefixRange(kvHistoryNamespacePrefix(namespace)))
	defer iterator.Close()
	for valid := iterator.First(); valid; valid = iterator.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...), nil)
	}
	return iterator.Error()
}

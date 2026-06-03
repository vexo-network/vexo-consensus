package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

var (
	blockHeightPrefix = []byte("block:height:")
	blockHashPrefix   = []byte("block:hash:")
	blockIndexKey     = []byte("block:index")
	evidencePrefix    = []byte("evidence:")
	evidenceIndexKey  = []byte("evidence:index")
	kvPrefix          = []byte("kv:")
	schemaStateKey    = []byte("schema:state")
	stateLatestKey    = []byte("state:latest")
	stateHeightPrefix = []byte("state:height:")
	stateRootPrefix   = []byte("state:root:")
)

type LevelDBStore struct {
	db *leveldb.DB
}

func OpenLevelDB(path string) (*LevelDBStore, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &LevelDBStore{db: db}, nil
}

func (store *LevelDBStore) SaveBlock(ctx context.Context, record BlockRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Block.Header.Height == 0 || record.Hash == (types.Hash{}) {
		return ErrInvalidBlockRecord
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(blockHeightKey(record.Block.Header.Height), encoded)
	batch.Put(blockHashKey(record.Hash), encoded)
	index, err := store.nextBlockIndex(record.Block.Header.Height)
	if err != nil {
		return err
	}
	encodedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}
	batch.Put(blockIndexKey, encodedIndex)
	return store.db.Write(batch, nil)
}

func (store *LevelDBStore) BlockByHeight(ctx context.Context, height types.Height) (BlockRecord, error) {
	select {
	case <-ctx.Done():
		return BlockRecord{}, ctx.Err()
	default:
	}
	return store.getBlock(blockHeightKey(height))
}

func (store *LevelDBStore) BlockByHash(ctx context.Context, hash types.Hash) (BlockRecord, error) {
	select {
	case <-ctx.Done():
		return BlockRecord{}, ctx.Err()
	default:
	}
	return store.getBlock(blockHashKey(hash))
}

func (store *LevelDBStore) BlockIndex(ctx context.Context) (BlockIndex, error) {
	select {
	case <-ctx.Done():
		return BlockIndex{}, ctx.Err()
	default:
	}

	encoded, err := store.db.Get(blockIndexKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return BlockIndex{}, ErrBlockIndexNotFound
		}
		return BlockIndex{}, err
	}

	var index BlockIndex
	if err := json.Unmarshal(encoded, &index); err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func (store *LevelDBStore) PruneBelow(ctx context.Context, retainFrom types.Height) (PruneResult, error) {
	select {
	case <-ctx.Done():
		return PruneResult{}, ctx.Err()
	default:
	}
	if retainFrom == 0 {
		return PruneResult{}, ErrInvalidPruneHeight
	}

	index, err := store.BlockIndex(ctx)
	if err != nil {
		return PruneResult{}, err
	}
	result := PruneResult{RetainFromHeight: retainFrom}
	batch := new(leveldb.Batch)

	for height := index.EarliestHeight; height < retainFrom && height <= index.LatestHeight; height++ {
		record, err := store.getBlock(blockHeightKey(height))
		if errors.Is(err, ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return PruneResult{}, err
		}
		batch.Delete(blockHeightKey(height))
		batch.Delete(blockHashKey(record.Hash))
		result.PrunedBlocks++
	}

	prunedRoots, err := store.pruneStateRootsBelow(ctx, batch, retainFrom)
	if err != nil {
		return PruneResult{}, err
	}
	prunedStates, err := store.pruneStatesBelow(ctx, batch, retainFrom)
	if err != nil {
		return PruneResult{}, err
	}
	result.PrunedStates = prunedStates
	result.PrunedStateRoots = prunedRoots

	newIndex, err := store.indexAfterPrune(ctx, retainFrom, index.LatestHeight)
	if err != nil {
		return PruneResult{}, err
	}
	if newIndex.TotalBlocks == 0 {
		batch.Delete(blockIndexKey)
	} else {
		encodedIndex, err := json.Marshal(newIndex)
		if err != nil {
			return PruneResult{}, err
		}
		batch.Put(blockIndexKey, encodedIndex)
	}
	if err := store.db.Write(batch, nil); err != nil {
		return PruneResult{}, err
	}
	return result, nil
}

func (store *LevelDBStore) PruneByRetention(ctx context.Context, policy RetentionPolicy) (PruneResult, error) {
	select {
	case <-ctx.Done():
		return PruneResult{}, ctx.Err()
	default:
	}
	if policy.RetainRecent == 0 {
		return PruneResult{}, ErrInvalidRetention
	}
	index, err := store.BlockIndex(ctx)
	if err != nil {
		return PruneResult{}, err
	}
	if index.TotalBlocks <= policy.RetainRecent {
		return PruneResult{RetainFromHeight: index.EarliestHeight}, nil
	}
	retainFrom := index.LatestHeight - types.Height(policy.RetainRecent) + 1
	return store.PruneBelow(ctx, retainFrom)
}

func (store *LevelDBStore) SaveState(ctx context.Context, state StateRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if state.Height == 0 {
		return ErrInvalidStateRecord
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(stateLatestKey, encoded)
	batch.Put(stateHeightKey(state.Height), encoded)
	return store.db.Write(batch, nil)
}

func (store *LevelDBStore) SaveSchemaState(ctx context.Context, state upgrade.State) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return store.db.Put(schemaStateKey, encoded, nil)
}

func (store *LevelDBStore) SchemaState(ctx context.Context) (upgrade.State, error) {
	select {
	case <-ctx.Done():
		return upgrade.State{}, ctx.Err()
	default:
	}
	encoded, err := store.db.Get(schemaStateKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return upgrade.State{}, ErrStateNotFound
		}
		return upgrade.State{}, err
	}
	var state upgrade.State
	if err := json.Unmarshal(encoded, &state); err != nil {
		return upgrade.State{}, err
	}
	return state, nil
}

func (store *LevelDBStore) LatestState(ctx context.Context) (StateRecord, error) {
	select {
	case <-ctx.Done():
		return StateRecord{}, ctx.Err()
	default:
	}

	encoded, err := store.db.Get(stateLatestKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return StateRecord{}, ErrStateNotFound
		}
		return StateRecord{}, err
	}

	var state StateRecord
	if err := json.Unmarshal(encoded, &state); err != nil {
		return StateRecord{}, err
	}
	return state, nil
}

func (store *LevelDBStore) StateByHeight(ctx context.Context, height types.Height) (StateRecord, error) {
	select {
	case <-ctx.Done():
		return StateRecord{}, ctx.Err()
	default:
	}
	if height == 0 {
		return StateRecord{}, ErrInvalidStateRecord
	}
	encoded, err := store.db.Get(stateHeightKey(height), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return StateRecord{}, ErrStateNotFound
		}
		return StateRecord{}, err
	}
	var state StateRecord
	if err := json.Unmarshal(encoded, &state); err != nil {
		return StateRecord{}, err
	}
	return state, nil
}

func (store *LevelDBStore) SaveStateRoot(ctx context.Context, record StateRootRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Height == 0 || record.Namespace == "" {
		return ErrInvalidStateRoot
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return store.db.Put(stateRootKey(record.Height, record.Namespace), encoded, nil)
}

func (store *LevelDBStore) CommitBlockState(ctx context.Context, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if block.Block.Header.Height == 0 || block.Hash == (types.Hash{}) {
		return ErrInvalidBlockRecord
	}
	if state.Height == 0 {
		return ErrInvalidStateRecord
	}
	for _, root := range roots {
		if root.Height == 0 || root.Namespace == "" {
			return ErrInvalidStateRoot
		}
	}

	block.StateRoots = append([]StateRootRecord(nil), roots...)
	encodedBlock, err := json.Marshal(block)
	if err != nil {
		return err
	}
	index, err := store.nextBlockIndex(block.Block.Header.Height)
	if err != nil {
		return err
	}
	encodedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}
	encodedState, err := json.Marshal(state)
	if err != nil {
		return err
	}

	batch := new(leveldb.Batch)
	batch.Put(blockHeightKey(block.Block.Header.Height), encodedBlock)
	batch.Put(blockHashKey(block.Hash), encodedBlock)
	batch.Put(blockIndexKey, encodedIndex)
	batch.Put(stateLatestKey, encodedState)
	batch.Put(stateHeightKey(state.Height), encodedState)
	for _, root := range roots {
		encodedRoot, err := json.Marshal(root)
		if err != nil {
			return err
		}
		batch.Put(stateRootKey(root.Height, root.Namespace), encodedRoot)
	}
	return store.db.Write(batch, nil)
}

func (store *LevelDBStore) StateRoot(ctx context.Context, height types.Height, namespace string) (StateRootRecord, error) {
	select {
	case <-ctx.Done():
		return StateRootRecord{}, ctx.Err()
	default:
	}
	if height == 0 || namespace == "" {
		return StateRootRecord{}, ErrInvalidStateRoot
	}

	encoded, err := store.db.Get(stateRootKey(height, namespace), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return StateRootRecord{}, ErrStateRootNotFound
		}
		return StateRootRecord{}, err
	}

	var record StateRootRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return StateRootRecord{}, err
	}
	return record, nil
}

func (store *LevelDBStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if namespace == "" {
		return ErrInvalidNamespace
	}
	if len(key) == 0 {
		return ErrInvalidKey
	}
	return store.db.Put(kvKey(namespace, key), append([]byte(nil), value...), nil)
}

func (store *LevelDBStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
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

	value, err := store.db.Get(kvKey(namespace, key), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func (store *LevelDBStore) Delete(ctx context.Context, namespace string, key []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if namespace == "" {
		return ErrInvalidNamespace
	}
	if len(key) == 0 {
		return ErrInvalidKey
	}
	return store.db.Delete(kvKey(namespace, key), nil)
}

func (store *LevelDBStore) SetBatch(ctx context.Context, writes []KVWrite) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	batch := new(leveldb.Batch)
	for _, write := range writes {
		if write.Namespace == "" {
			return ErrInvalidNamespace
		}
		if len(write.Key) == 0 {
			return ErrInvalidKey
		}
		key := kvKey(write.Namespace, write.Key)
		if write.Delete {
			batch.Delete(key)
			continue
		}
		batch.Put(key, append([]byte(nil), write.Value...))
	}
	return store.db.Write(batch, nil)
}

func (store *LevelDBStore) Root(ctx context.Context, namespace string) (types.Hash, error) {
	select {
	case <-ctx.Done():
		return types.Hash{}, ctx.Err()
	default:
	}
	if namespace == "" {
		return types.Hash{}, ErrInvalidNamespace
	}

	prefix := kvNamespacePrefix(namespace)
	iterator := store.db.NewIterator(nil, nil)
	defer iterator.Release()

	hasher := sha256.New()
	for ok := iterator.Seek(prefix); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return types.Hash{}, ctx.Err()
		default:
		}
		key := iterator.Key()
		if len(key) < len(prefix) || string(key[:len(prefix)]) != string(prefix) {
			break
		}
		value := iterator.Value()
		writeUint64(hasher, uint64(len(key)))
		hasher.Write(key)
		writeUint64(hasher, uint64(len(value)))
		hasher.Write(value)
	}
	if err := iterator.Error(); err != nil {
		return types.Hash{}, err
	}

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash, nil
}

func (store *LevelDBStore) ExportNamespace(ctx context.Context, namespace string) ([]KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	prefix := kvNamespacePrefix(namespace)
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()

	pairs := make([]KVPair, 0)
	for iterator.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		rawKey := iterator.Key()
		rawValue := iterator.Value()
		pairs = append(pairs, KVPair{
			Namespace: namespace,
			Key:       append([]byte(nil), rawKey[len(prefix):]...),
			Value:     append([]byte(nil), rawValue...),
		})
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return pairs, nil
}

func (store *LevelDBStore) ImportNamespace(ctx context.Context, namespace string, pairs []KVPair) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if namespace == "" {
		return ErrInvalidNamespace
	}
	batch := new(leveldb.Batch)
	prefix := kvNamespacePrefix(namespace)
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	for iterator.Next() {
		select {
		case <-ctx.Done():
			iterator.Release()
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...))
	}
	if err := iterator.Error(); err != nil {
		iterator.Release()
		return err
	}
	iterator.Release()
	for _, pair := range pairs {
		if pair.Namespace != "" && pair.Namespace != namespace {
			return ErrInvalidNamespace
		}
		if len(pair.Key) == 0 {
			return ErrInvalidKey
		}
		batch.Put(kvKey(namespace, pair.Key), append([]byte(nil), pair.Value...))
	}
	return store.db.Write(batch, nil)
}

func (store *LevelDBStore) RecoverIndexes(ctx context.Context) (RecoverResult, error) {
	select {
	case <-ctx.Done():
		return RecoverResult{}, ctx.Err()
	default:
	}
	var result RecoverResult
	blockIndex, err := store.rebuildBlockIndex(ctx)
	if err != nil {
		return RecoverResult{}, err
	}
	evidenceIndex, err := store.rebuildEvidenceIndex(ctx)
	if err != nil {
		return RecoverResult{}, err
	}
	batch := new(leveldb.Batch)
	if blockIndex.TotalBlocks == 0 {
		batch.Delete(blockIndexKey)
	} else {
		encoded, err := json.Marshal(blockIndex)
		if err != nil {
			return RecoverResult{}, err
		}
		batch.Put(blockIndexKey, encoded)
		result.RecoveredIndexes++
	}
	if len(evidenceIndex) == 0 {
		batch.Delete(evidenceIndexKey)
	} else {
		encoded, err := json.Marshal(evidenceIndex)
		if err != nil {
			return RecoverResult{}, err
		}
		batch.Put(evidenceIndexKey, encoded)
		result.RecoveredIndexes++
	}
	if err := store.db.Write(batch, nil); err != nil {
		return RecoverResult{}, err
	}
	result.BlockIndexKeys = blockIndex.TotalBlocks
	result.EvidenceKeys = uint64(len(evidenceIndex))
	result.EarliestHeight = blockIndex.EarliestHeight
	result.LatestHeight = blockIndex.LatestHeight
	return result, nil
}

func (store *LevelDBStore) Compact(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return store.db.CompactRange(util.Range{})
}

func (store *LevelDBStore) Close() error {
	return store.db.Close()
}

func (store *LevelDBStore) getBlock(key []byte) (BlockRecord, error) {
	encoded, err := store.db.Get(key, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return BlockRecord{}, ErrBlockNotFound
		}
		return BlockRecord{}, err
	}

	var record BlockRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return BlockRecord{}, err
	}
	return record, nil
}

func (store *LevelDBStore) pruneStateRootsBelow(ctx context.Context, batch *leveldb.Batch, retainFrom types.Height) (uint64, error) {
	iterator := store.db.NewIterator(nil, nil)
	defer iterator.Release()

	var pruned uint64
	for ok := iterator.Seek(stateRootPrefix); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		key := iterator.Key()
		if len(key) < len(stateRootPrefix) || string(key[:len(stateRootPrefix)]) != string(stateRootPrefix) {
			break
		}
		height, ok := stateRootHeightFromKey(key)
		if !ok {
			continue
		}
		if height < retainFrom {
			batch.Delete(append([]byte(nil), key...))
			pruned++
		}
	}
	if err := iterator.Error(); err != nil {
		return 0, err
	}
	return pruned, nil
}

func (store *LevelDBStore) pruneStatesBelow(ctx context.Context, batch *leveldb.Batch, retainFrom types.Height) (uint64, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(stateHeightPrefix), nil)
	defer iterator.Release()

	var pruned uint64
	for ok := iterator.First(); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		height, ok := stateHeightFromKey(iterator.Key())
		if !ok {
			continue
		}
		if height < retainFrom {
			batch.Delete(append([]byte(nil), iterator.Key()...))
			pruned++
		}
	}
	if err := iterator.Error(); err != nil {
		return 0, err
	}
	return pruned, nil
}

func (store *LevelDBStore) indexAfterPrune(ctx context.Context, retainFrom types.Height, latest types.Height) (BlockIndex, error) {
	var nextIndex BlockIndex
	for height := retainFrom; height <= latest; height++ {
		select {
		case <-ctx.Done():
			return BlockIndex{}, ctx.Err()
		default:
		}
		if _, err := store.getBlock(blockHeightKey(height)); err != nil {
			if errors.Is(err, ErrBlockNotFound) {
				continue
			}
			return BlockIndex{}, err
		}
		if nextIndex.TotalBlocks == 0 {
			nextIndex.EarliestHeight = height
		}
		nextIndex.LatestHeight = height
		nextIndex.TotalBlocks++
	}
	return nextIndex, nil
}

func (store *LevelDBStore) nextBlockIndex(height types.Height) (BlockIndex, error) {
	encoded, err := store.db.Get(blockIndexKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return BlockIndex{EarliestHeight: height, LatestHeight: height, TotalBlocks: 1}, nil
		}
		return BlockIndex{}, err
	}

	var index BlockIndex
	if err := json.Unmarshal(encoded, &index); err != nil {
		return BlockIndex{}, err
	}
	if index.EarliestHeight == 0 || height < index.EarliestHeight {
		index.EarliestHeight = height
	}
	if height > index.LatestHeight {
		index.LatestHeight = height
	}
	if _, err := store.db.Get(blockHeightKey(height), nil); errors.Is(err, leveldberrors.ErrNotFound) {
		index.TotalBlocks++
	} else if err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func (store *LevelDBStore) rebuildBlockIndex(ctx context.Context) (BlockIndex, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(blockHeightPrefix), nil)
	defer iterator.Release()
	var index BlockIndex
	for ok := iterator.First(); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return BlockIndex{}, ctx.Err()
		default:
		}
		var record BlockRecord
		if err := json.Unmarshal(iterator.Value(), &record); err != nil {
			return BlockIndex{}, err
		}
		height := record.Block.Header.Height
		if height == 0 {
			continue
		}
		if index.TotalBlocks == 0 || height < index.EarliestHeight {
			index.EarliestHeight = height
		}
		if height > index.LatestHeight {
			index.LatestHeight = height
		}
		index.TotalBlocks++
	}
	if err := iterator.Error(); err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

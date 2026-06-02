package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	blockHeightPrefix = []byte("block:height:")
	blockHashPrefix   = []byte("block:hash:")
	blockIndexKey     = []byte("block:index")
	evidencePrefix    = []byte("evidence:")
	evidenceIndexKey  = []byte("evidence:index")
	kvPrefix          = []byte("kv:")
	stateLatestKey    = []byte("state:latest")
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
	return store.db.Put(stateLatestKey, encoded, nil)
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

func (store *LevelDBStore) SaveEvidence(ctx context.Context, record EvidenceRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	key := evidenceRecordKey(record.Evidence)
	if key == "" {
		return ErrInvalidKey
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	index, err := store.EvidenceIndex(ctx)
	if err != nil && !errors.Is(err, ErrEvidenceNotFound) {
		return err
	}
	if !stringSliceContains(index, key) {
		index = append(index, key)
	}
	encodedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(evidenceKey(key), encoded)
	batch.Put(evidenceIndexKey, encodedIndex)
	return store.db.Write(batch, nil)
}

func (store *LevelDBStore) EvidenceByKey(ctx context.Context, key string) (EvidenceRecord, error) {
	select {
	case <-ctx.Done():
		return EvidenceRecord{}, ctx.Err()
	default:
	}
	if key == "" {
		return EvidenceRecord{}, ErrInvalidKey
	}
	encoded, err := store.db.Get(evidenceKey(key), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return EvidenceRecord{}, ErrEvidenceNotFound
		}
		return EvidenceRecord{}, err
	}
	var record EvidenceRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func (store *LevelDBStore) EvidenceIndex(ctx context.Context) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	encoded, err := store.db.Get(evidenceIndexKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return nil, ErrEvidenceNotFound
		}
		return nil, err
	}
	var index []string
	if err := json.Unmarshal(encoded, &index); err != nil {
		return nil, err
	}
	return append([]string(nil), index...), nil
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

func blockHeightKey(height types.Height) []byte {
	key := append([]byte(nil), blockHeightPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(key, buffer[:]...)
}

func blockHashKey(hash types.Hash) []byte {
	key := append([]byte(nil), blockHashPrefix...)
	return append(key, hash[:]...)
}

func stateRootKey(height types.Height, namespace string) []byte {
	key := append([]byte(nil), stateRootPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	key = append(key, buffer[:]...)
	key = append(key, ':')
	return append(key, []byte(namespace)...)
}

func stateRootHeightFromKey(key []byte) (types.Height, bool) {
	if len(key) < len(stateRootPrefix)+8 {
		return 0, false
	}
	return types.Height(binary.BigEndian.Uint64(key[len(stateRootPrefix) : len(stateRootPrefix)+8])), true
}

func kvKey(namespace string, key []byte) []byte {
	dbKey := kvNamespacePrefix(namespace)
	return append(dbKey, key...)
}

func kvNamespacePrefix(namespace string) []byte {
	dbKey := append([]byte(nil), kvPrefix...)
	dbKey = append(dbKey, []byte(namespace)...)
	return append(dbKey, ':')
}

func evidenceKey(key string) []byte {
	dbKey := append([]byte(nil), evidencePrefix...)
	return append(dbKey, []byte(key)...)
}

func evidenceRecordKey(evidence slashing.Evidence) string {
	if evidence.Type == "" || evidence.Validator == "" || evidence.Height == 0 || len(evidence.Proof) == 0 {
		return ""
	}
	hash := sha256.Sum256(evidence.Proof)
	return string(evidence.Type) + ":" + string(evidence.Validator) + ":" + strconv.FormatUint(uint64(evidence.Height), 10) + ":" + strconv.FormatUint(uint64(evidence.Round), 10) + ":" + hex.EncodeToString(hash[:])
}

func stringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

type byteWriter interface {
	Write([]byte) (int, error)
}

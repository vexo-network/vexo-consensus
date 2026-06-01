package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	blockHeightPrefix = []byte("block:height:")
	blockHashPrefix   = []byte("block:hash:")
	kvPrefix          = []byte("kv:")
	stateLatestKey    = []byte("state:latest")
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

func kvKey(namespace string, key []byte) []byte {
	dbKey := kvNamespacePrefix(namespace)
	return append(dbKey, key...)
}

func kvNamespacePrefix(namespace string) []byte {
	dbKey := append([]byte(nil), kvPrefix...)
	dbKey = append(dbKey, []byte(namespace)...)
	return append(dbKey, ':')
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

type byteWriter interface {
	Write([]byte) (int, error)
}

package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func (store *LevelDBStore) SaveEvidence(ctx context.Context, record EvidenceRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	key := EvidenceKey(record.Evidence)
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

func (store *LevelDBStore) rebuildEvidenceIndex(ctx context.Context) ([]string, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(evidencePrefix), nil)
	defer iterator.Release()
	index := make([]string, 0)
	for ok := iterator.First(); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		key := evidenceKeyString(iterator.Key())
		if key == "" || key == "index" {
			continue
		}
		index = append(index, key)
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return index, nil
}

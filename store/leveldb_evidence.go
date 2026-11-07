package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Evidence is stored one record per key: saving a record writes only that record, so the cost of a
// save does not grow with the number of stored records, and no index exists that could disagree
// with the records.

// SaveEvidence stores record under the key of the offence its evidence proves, replacing an earlier
// record of the same offence.
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
	return store.put(evidenceKey(key), encoded)
}

// EvidenceByKey returns the record stored under key; ErrEvidenceCorrupt reports a record that does
// not decode.
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
		return EvidenceRecord{}, fmt.Errorf("%w: %v", ErrEvidenceCorrupt, err)
	}
	return record, nil
}

// DeleteEvidence removes the record stored under key; deleting a missing record is not an error.
func (store *LevelDBStore) DeleteEvidence(ctx context.Context, key string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if key == "" {
		return ErrInvalidKey
	}
	return store.delete(evidenceKey(key))
}

// EvidenceIndex returns the keys of every stored record in key order, or ErrEvidenceNotFound when
// there is none.
func (store *LevelDBStore) EvidenceIndex(ctx context.Context) ([]string, error) {
	keys, err := store.evidenceKeys(ctx)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, ErrEvidenceNotFound
	}
	return keys, nil
}

func (store *LevelDBStore) evidenceKeys(ctx context.Context) ([]string, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(evidencePrefix), nil)
	defer iterator.Release()
	keys := make([]string, 0)
	for iterator.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if key := evidenceKeyString(iterator.Key()); key != "" {
			keys = append(keys, key)
		}
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return keys, nil
}

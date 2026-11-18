package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/cockroachdb/pebble/v2"
)

// Evidence is stored one record per key: saving a record writes only that record, so the cost of a
// save does not grow with the number of stored records, and no index exists that could disagree
// with the records.

// SaveEvidence stores record under the key of the offence its evidence proves, replacing an earlier
// record of the same offence.
func (store *PebbleStore) SaveEvidence(ctx context.Context, record EvidenceRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	key := EvidenceKey(record.Evidence)
	if key == "" {
		return ErrInvalidKey
	}
	encoded := encodeEvidenceRecord(record)
	return store.put(evidenceKey(key), encoded)
}

// EvidenceByKey returns the record stored under key; ErrEvidenceCorrupt reports a record that does
// not decode.
func (store *PebbleStore) EvidenceByKey(ctx context.Context, key string) (EvidenceRecord, error) {
	select {
	case <-ctx.Done():
		return EvidenceRecord{}, ctx.Err()
	default:
	}
	if key == "" {
		return EvidenceRecord{}, ErrInvalidKey
	}
	encoded, err := getValue(store.db, evidenceKey(key))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return EvidenceRecord{}, ErrEvidenceNotFound
		}
		return EvidenceRecord{}, err
	}
	record, err := decodeEvidenceRecord(encoded)
	if err != nil {
		return EvidenceRecord{}, fmt.Errorf("%w: %v", ErrEvidenceCorrupt, err)
	}
	return record, nil
}

// DeleteEvidence removes the record stored under key; deleting a missing record is not an error.
func (store *PebbleStore) DeleteEvidence(ctx context.Context, key string) error {
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
func (store *PebbleStore) EvidenceIndex(ctx context.Context) ([]string, error) {
	keys, err := store.evidenceKeys(ctx)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, ErrEvidenceNotFound
	}
	return keys, nil
}

func (store *PebbleStore) evidenceKeys(ctx context.Context) ([]string, error) {
	iterator := newIter(store.db, prefixRange(evidencePrefix))
	defer iterator.Close()
	keys := make([]string, 0)
	for valid := iterator.First(); valid; valid = iterator.Next() {
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

package store

import (
	"bytes"
	"context"
	"errors"

	"github.com/cockroachdb/pebble/v2"
)

// dbReader reads keys from the database or from one of its snapshots.
type dbReader = pebble.Reader

// getValue returns a copy of the value of key, pebble.ErrNotFound when reader holds none.
func getValue(reader dbReader, key []byte) ([]byte, error) {
	value, closer, err := reader.Get(key)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return bytes.Clone(value), nil
}

// hasKey reports whether reader holds key.
func hasKey(reader dbReader, key []byte) (bool, error) {
	_, closer, err := reader.Get(key)
	if errors.Is(err, pebble.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, closer.Close()
}

// keyRange is the keys from lower on and below upper; a nil upper bounds nothing.
type keyRange struct {
	lower []byte
	upper []byte
}

// prefixRange is the keys that start with prefix.
func prefixRange(prefix []byte) keyRange {
	return keyRange{lower: prefix, upper: prefixEnd(prefix)}
}

// prefixEnd returns the first key after every key that starts with prefix, nil when there is none.
func prefixEnd(prefix []byte) []byte {
	end := bytes.Clone(prefix)
	for index := len(end) - 1; index >= 0; index-- {
		if end[index] < 0xff {
			end[index]++
			return end[:index+1]
		}
	}
	return nil
}

// newIter returns an unpositioned iterator over span of reader. A database or snapshot iterator
// reports a read failure through its Error method, not when it is created.
func newIter(reader dbReader, span keyRange) *pebble.Iterator {
	iterator, err := reader.NewIter(&pebble.IterOptions{LowerBound: span.lower, UpperBound: span.upper})
	if err != nil {
		panic(err)
	}
	return iterator
}

// compactAll compacts the whole key space.
func (store *PebbleStore) compactAll(ctx context.Context) error {
	first := newIter(store.db, keyRange{})
	defer first.Close()
	if !first.First() {
		return first.Error()
	}
	lower := bytes.Clone(first.Key())
	if !first.Last() {
		return first.Error()
	}
	upper := append(bytes.Clone(first.Key()), 0)
	return store.db.Compact(ctx, lower, upper, true)
}

// quietLogger drops pebble's informational output and keeps its errors on the standard logger.
type quietLogger struct{}

func (quietLogger) Infof(string, ...any) {}

func (quietLogger) Errorf(format string, args ...any) { pebble.DefaultLogger.Errorf(format, args...) }

func (quietLogger) Fatalf(format string, args ...any) { pebble.DefaultLogger.Fatalf(format, args...) }

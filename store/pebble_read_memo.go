package store

import "sync"

// Read memo.
//
// A block's execution reads the keys it changes before it writes them (a balance before a transfer
// updates it), and the commit then records each changed key's value before the block as its
// version (see State history), which reads the key again. The store therefore remembers the values
// Get read since the last batch committed, and the commit takes a changed key's prior value from
// them instead of reading it.
//
// The memo holds values only, never an absence. A generation counts the batches the store committed:
// every batch goes through writeBatch, which counts it once it committed and counts the batches in
// progress meanwhile. A read is remembered only at the generation it began at, so a read that a
// commit overlapped is dropped, and the memo serves a value only at the generation it was read at
// while no batch is committing, so a memoized value is the key's current value.

// readMemoCapacity bounds the values the memo holds; reads past it are not remembered.
const readMemoCapacity = 1 << 16

type readMemo struct {
	mu sync.Mutex
	// generation counts the batches committed; committing counts those in progress.
	generation uint64
	committing int
	// at is the generation values were read at.
	at     uint64
	values map[string][]byte
}

func newReadMemo() *readMemo {
	return &readMemo{values: make(map[string][]byte)}
}

// begin returns the generation a read starts at.
func (memo *readMemo) begin() uint64 {
	memo.mu.Lock()
	defer memo.mu.Unlock()
	return memo.generation
}

// remember keeps value, read at key from generation on, when no batch committed since. value is
// not modified afterwards.
func (memo *readMemo) remember(generation uint64, key []byte, value []byte) {
	memo.mu.Lock()
	defer memo.mu.Unlock()
	if memo.generation != generation {
		return
	}
	if memo.at != generation {
		clear(memo.values)
		memo.at = generation
	}
	if len(memo.values) < readMemoCapacity {
		memo.values[string(key)] = value
	}
}

// value returns the current value of key when the memo holds it. The caller does not modify it.
func (memo *readMemo) value(key []byte) ([]byte, bool) {
	memo.mu.Lock()
	defer memo.mu.Unlock()
	if memo.committing != 0 || memo.at != memo.generation {
		return nil, false
	}
	value, found := memo.values[string(key)]
	return value, found
}

// commit runs commit, a batch commit, and counts it, so no value read before it is used after it.
func (memo *readMemo) commit(commit func() error) error {
	memo.mu.Lock()
	memo.committing++
	memo.mu.Unlock()
	defer func() {
		memo.mu.Lock()
		memo.generation++
		memo.committing--
		memo.mu.Unlock()
	}()
	return commit()
}

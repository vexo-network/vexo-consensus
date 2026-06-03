package kvbatch

import "context"

type KVWrite struct {
	Namespace string
	Key       []byte
	Value     []byte
	Delete    bool
}

type BatchKVStore interface {
	SetBatch(ctx context.Context, writes []KVWrite) error
}

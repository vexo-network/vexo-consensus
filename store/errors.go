package store

import "errors"

var (
	ErrBlockNotFound      = errors.New("block not found")
	ErrStateNotFound      = errors.New("state not found")
	ErrKeyNotFound        = errors.New("key not found")
	ErrInvalidBlockRecord = errors.New("invalid block record")
	ErrInvalidStateRecord = errors.New("invalid state record")
	ErrInvalidNamespace   = errors.New("namespace is required")
	ErrInvalidKey         = errors.New("key is required")
)

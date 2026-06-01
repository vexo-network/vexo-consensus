package store

import "errors"

var (
	ErrBlockNotFound      = errors.New("block not found")
	ErrStateNotFound      = errors.New("state not found")
	ErrInvalidBlockRecord = errors.New("invalid block record")
	ErrInvalidStateRecord = errors.New("invalid state record")
)

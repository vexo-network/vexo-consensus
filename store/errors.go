package store

import "errors"

var (
	ErrBlockNotFound       = errors.New("block not found")
	ErrBlockIndexNotFound  = errors.New("block index not found")
	ErrStateNotFound       = errors.New("state not found")
	ErrStateRootNotFound   = errors.New("state root not found")
	ErrEvidenceNotFound    = errors.New("evidence not found")
	ErrFinalityNotFound    = errors.New("finality proof not found")
	ErrUpgradePlanNotFound = errors.New("upgrade plan not found")
	ErrKeyNotFound         = errors.New("key not found")
	ErrInvalidBlockRecord  = errors.New("invalid block record")
	ErrInvalidStateRecord  = errors.New("invalid state record")
	ErrInvalidStateRoot    = errors.New("invalid state root record")
	ErrInvalidFinality     = errors.New("invalid finality proof record")
	ErrInvalidPruneHeight  = errors.New("invalid prune height")
	ErrInvalidRetention    = errors.New("invalid retention policy")
	ErrInvalidNamespace    = errors.New("namespace is required")
	ErrInvalidKey          = errors.New("key is required")
)

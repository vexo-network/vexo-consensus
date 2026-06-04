package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

type BlockRecord struct {
	Block      types.Block
	Hash       types.Hash
	AppHash    types.Hash
	StateRoots []StateRootRecord
}

type StateRecord struct {
	Height           types.Height
	AppHash          types.Hash
	LastBlockHash    types.Hash
	ValidatorSetHash types.Hash
	BaseFee          uint64
	NextBaseFee      uint64
}

type StateRootRecord struct {
	Height    types.Height
	Namespace string
	Root      types.Hash
}

type KVPair struct {
	Namespace string
	Key       []byte
	Value     []byte
}

type KVWrite = kvbatch.KVWrite

type BlockIndex struct {
	EarliestHeight types.Height
	LatestHeight   types.Height
	TotalBlocks    uint64
}

type PruneResult struct {
	RetainFromHeight types.Height
	PrunedBlocks     uint64
	PrunedStates     uint64
	PrunedStateRoots uint64
}

type RetentionPolicy struct {
	RetainRecent uint64
}

type RecoverResult struct {
	BlockIndexKeys   uint64
	EvidenceKeys     uint64
	EarliestHeight   types.Height
	LatestHeight     types.Height
	RecoveredIndexes uint64
}

type EvidenceRecord struct {
	Evidence  slashing.Evidence
	Applied   bool
	CreatedAt int64
}

func EvidenceKey(evidence slashing.Evidence) string {
	if evidence.Type == "" || evidence.Validator == "" || evidence.Height == 0 || len(evidence.Proof) == 0 {
		return ""
	}
	hash := sha256.Sum256(evidence.Proof)
	return string(evidence.Type) + ":" + string(evidence.Validator) + ":" + strconv.FormatUint(uint64(evidence.Height), 10) + ":" + strconv.FormatUint(uint64(evidence.Round), 10) + ":" + hex.EncodeToString(hash[:])
}

type Store interface {
	KVStore
	SaveBlock(ctx context.Context, record BlockRecord) error
	BlockByHeight(ctx context.Context, height types.Height) (BlockRecord, error)
	BlockByHash(ctx context.Context, hash types.Hash) (BlockRecord, error)
	BlockIndex(ctx context.Context) (BlockIndex, error)
	PruneBelow(ctx context.Context, retainFrom types.Height) (PruneResult, error)
	PruneByRetention(ctx context.Context, policy RetentionPolicy) (PruneResult, error)
	SaveState(ctx context.Context, state StateRecord) error
	LatestState(ctx context.Context) (StateRecord, error)
	StateByHeight(ctx context.Context, height types.Height) (StateRecord, error)
	SaveStateRoot(ctx context.Context, record StateRootRecord) error
	StateRoot(ctx context.Context, height types.Height, namespace string) (StateRootRecord, error)
	SaveEvidence(ctx context.Context, record EvidenceRecord) error
	EvidenceByKey(ctx context.Context, key string) (EvidenceRecord, error)
	EvidenceIndex(ctx context.Context) ([]string, error)
	RecoverIndexes(ctx context.Context) (RecoverResult, error)
	Compact(ctx context.Context) error
	Close() error
}

type BlockCommitStore interface {
	CommitBlockState(ctx context.Context, block BlockRecord, state StateRecord, roots []StateRootRecord) error
}

type AppBlockCommitStore interface {
	CommitBlockStateWithWrites(ctx context.Context, writes []KVWrite, block BlockRecord, state StateRecord, roots []StateRootRecord) error
}

type RootWithWritesStore interface {
	RootWithWrites(ctx context.Context, namespace string, writes []KVWrite) (types.Hash, error)
}

type SchemaStateStore interface {
	SaveSchemaState(ctx context.Context, state upgrade.State) error
	SchemaState(ctx context.Context) (upgrade.State, error)
}

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
	Root(ctx context.Context, namespace string) (types.Hash, error)
}

type BatchKVStore = kvbatch.BatchKVStore

type SnapshotKVStore interface {
	ExportNamespace(ctx context.Context, namespace string) ([]KVPair, error)
	ImportNamespace(ctx context.Context, namespace string, pairs []KVPair) error
}

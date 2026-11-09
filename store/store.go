package store

import (
	"context"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

type BlockRecord struct {
	Block      types.Block
	Hash       types.Hash
	AppHash    types.Hash
	QuorumCert *QuorumCertRecord
	StateRoots []StateRootRecord
	TxResults  []types.Result
}

type StateRecord struct {
	Height           types.Height
	AppHash          types.Hash
	LastBlockHash    types.Hash
	ValidatorSetHash types.Hash
	BaseFee          uint64
	NextBaseFee      uint64
	BlobBaseFee      uint64
	NextBlobBaseFee  uint64
	BlobGasUsed      uint64
	ExcessBlobGas    uint64
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
	// PrunedStateVersions counts the key versions no retained height reads.
	PrunedStateVersions uint64
	// PrunedStateTreeVersions counts the state tree node versions no retained height reads.
	PrunedStateTreeVersions uint64
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

// EvidenceRecord is a piece of verified evidence a node holds until a block punishes the offence.
type EvidenceRecord struct {
	Evidence slashing.Evidence
}

type FinalityProofRecord struct {
	Header             types.Header
	BlockHash          types.Hash
	QuorumCert         QuorumCertRecord
	CommitChain        []CommitLinkRecord
	ValidatorSetHeight types.Height
	ValidatorSetHash   types.Hash
}

type CommitLinkRecord struct {
	Header     types.Header
	BlockHash  types.Hash
	QuorumCert QuorumCertRecord
}

type QuorumCertRecord struct {
	Height      types.Height
	Round       types.Round
	BlockHash   types.Hash
	Signers     types.Bitmap
	Signature   types.AggregateSignature
	VotingPower types.VotingPower
}

// EvidenceKey identifies the offence evidence proves, so every proof and every label of one offence
// shares one record. It is empty for evidence that has no proof or names no offence.
func EvidenceKey(evidence slashing.Evidence) string {
	if len(evidence.Proof) == 0 {
		return ""
	}
	key, err := slashing.OffenceKey(evidence)
	if err != nil {
		return ""
	}
	return key
}

type Store interface {
	KVStore
	HistoricalSnapshotKVStore
	StateTreeStore
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
	DeleteEvidence(ctx context.Context, key string) error
	EvidenceIndex(ctx context.Context) ([]string, error)
	RecoverIndexes(ctx context.Context) (RecoverResult, error)
	Compact(ctx context.Context) error
	Close() error
}

type BlockCommitStore interface {
	CommitBlockState(ctx context.Context, block BlockRecord, state StateRecord, roots []StateRootRecord) error
}

type FinalityProofStore interface {
	SaveFinalityProof(ctx context.Context, proof FinalityProofRecord) error
	FinalityProof(ctx context.Context, height types.Height) (FinalityProofRecord, error)
	LatestFinalityProof(ctx context.Context) (FinalityProofRecord, error)
}

type AppBlockCommitStore interface {
	CommitBlockStateWithWrites(ctx context.Context, writes []KVWrite, block BlockRecord, state StateRecord, roots []StateRootRecord) error
}

// StateTreeStore keeps the state tree of every namespace, with the versions of its nodes, and
// proves keys from it (see stateproof): of the current state, and of any retained height.
type StateTreeStore interface {
	StateProof(ctx context.Context, namespace string, key []byte) (types.Hash, []byte, bool, stateproof.Proof, error)
	StateProofAt(ctx context.Context, height types.Height, namespace string, key []byte) (types.Hash, []byte, bool, stateproof.Proof, error)
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

type HistoricalKVStore interface {
	GetAt(ctx context.Context, height types.Height, namespace string, key []byte) ([]byte, error)
}

type HistoricalSnapshotKVStore interface {
	ExportNamespaceAt(ctx context.Context, height types.Height, namespace string) ([]KVPair, error)
	// ExportRangeAt pages through a namespace at a past height in key order (see LevelDBStore).
	ExportRangeAt(ctx context.Context, height types.Height, namespace string, after []byte, limit int) ([]KVPair, bool, error)
}

type HistoricalPrefixKVStore interface {
	ExportPrefixAt(ctx context.Context, height types.Height, namespace string, prefix []byte) ([]KVPair, error)
}

type BatchKVStore = kvbatch.BatchKVStore

type SnapshotKVStore interface {
	ExportNamespace(ctx context.Context, namespace string) ([]KVPair, error)
	ImportNamespace(ctx context.Context, namespace string, pairs []KVPair) error
}

type PrefixKVStore interface {
	ExportPrefix(ctx context.Context, namespace string, prefix []byte) ([]KVPair, error)
}

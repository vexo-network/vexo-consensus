package rpc

import (
	"context"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type StatusProvider interface {
	Status(ctx context.Context) node.Status
}

type MetricsProvider interface {
	Metrics(ctx context.Context) (node.Metrics, error)
}

// SnapshotProvider exposes the latest committed state. StateSnapshotHead is cheap; StateSnapshot
// copies every key-value pair of the application state.
type SnapshotProvider interface {
	StateSnapshotHead(ctx context.Context) (node.StateSnapshotHead, error)
	// StateSnapshot exports the state committed block blockHeight anchors (see
	// node.StateSnapshot); 0 selects the latest committed block.
	StateSnapshot(ctx context.Context, blockHeight types.Height) (node.StateSnapshot, error)
}

type RecoveryProvider interface {
	RecoveryReport(ctx context.Context, repairIndexes bool) (node.RecoveryReport, error)
}

type TxSubmitter interface {
	SubmitTx(ctx context.Context, tx types.Tx) error
}

type PendingTxProvider interface {
	PendingTxHashes(ctx context.Context) ([]types.Hash, error)
}

type PendingTxsProvider interface {
	PendingTxProvider
	PendingTxs(ctx context.Context) ([]types.Tx, error)
}

type PendingTxSnapshotProvider interface {
	PendingTxSnapshot(ctx context.Context) ([]types.Tx, error)
}

// EvidenceSubmitter verifies, pools and gossips evidence. It changes no consensus state: the
// returned hash identifies the evidence payload a reporter wraps in a transaction to bring it to a
// block, and the offence is punished only when a block carries that transaction.
type EvidenceSubmitter interface {
	SubmitEvidence(ctx context.Context, evidence slashing.Evidence) (types.Hash, error)
}

type BlockProvider interface {
	BlockByHeight(ctx context.Context, height types.Height) (store.BlockRecord, error)
	BlockByHash(ctx context.Context, hash types.Hash) (store.BlockRecord, error)
	LatestBlock(ctx context.Context) (store.BlockRecord, error)
}

type ChainQueryProvider interface {
	BlockProvider
	BlockIndex(ctx context.Context) (store.BlockIndex, error)
	LatestState(ctx context.Context) (store.StateRecord, error)
	StateRoot(ctx context.Context, height types.Height, namespace string) (store.StateRootRecord, error)
}

type StateByHeightProvider interface {
	StateByHeight(ctx context.Context, height types.Height) (store.StateRecord, error)
}

type EventQueryProvider interface {
	QueryEvents(ctx context.Context, key string, value string) ([]events.Record, error)
}

type QueryProofProvider interface {
	QueryProof(ctx context.Context, height types.Height, namespace string, key []byte) (queryproof.Proof, error)
}

type IBCQueryProvider interface {
	IBCQuery(ctx context.Context, path []string) (vexoapp.QueryResponse, error)
}

type AppQueryProvider interface {
	AppQuery(ctx context.Context, path []string, data []byte) (vexoapp.QueryResponse, error)
}

type AccountQueryProvider interface {
	AccountSequence(ctx context.Context, address types.Address) (uint64, error)
}

type FinalityProvider interface {
	FinalityProof(ctx context.Context, height types.Height) (finality.Proof, error)
	LatestFinalityProof(ctx context.Context) (finality.Proof, error)
}

type PruneProvider interface {
	PruneBelow(ctx context.Context, retainFrom types.Height) (store.PruneResult, error)
}

type ReplayProvider interface {
	Replay(ctx context.Context, from types.Height, to types.Height) (vexoruntime.ReplayResult, error)
	ReplayAll(ctx context.Context) (vexoruntime.ReplayResult, error)
}

type StrictReplayProvider interface {
	ReplayStrict(ctx context.Context, from types.Height, to types.Height) (vexoruntime.ReplayResult, error)
	ReplayAllStrict(ctx context.Context) (vexoruntime.ReplayResult, error)
}

type ConsensusLoopController interface {
	StartConsensusLoop(ctx context.Context, cfg node.ConsensusLoopConfig) error
	StopConsensusLoop(ctx context.Context) error
	ConsensusLoopRunning() bool
}

type ValidatorQueryProvider interface {
	ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error)
	Committee(ctx context.Context, height types.Height, round types.Round, seed types.Hash) (committee.Committee, error)
}

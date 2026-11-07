package node

import (
	"context"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/finality"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// StateSnapshotHead identifies the committed state a snapshot exports: the
// latest state record and the state root of every namespace of the replicated
// state at that height. It reads no key-value pair, so health checks and
// per-step reports can use it without copying the application state.
type StateSnapshotHead struct {
	ChainID string
	// State is the state record of the latest committed block, fee market included.
	State store.StateRecord
	// Namespaces are the namespaces of the replicated state (see
	// runtime.StateNamespaces), each with a root in StateRoots.
	Namespaces []string
	StateRoots []store.StateRootRecord
}

// StateSnapshot is the replicated state at the height a committed block header commits the app
// hash of (finality.AppHashHeight), together with that header and every key-value pair of its
// namespaces. A node restoring it checks the header against a block hash it trusts and the state
// against the header's app hash, so it needs no trust in whoever served the snapshot. Building one
// copies the whole replicated state; a caller that only needs the head uses StateSnapshotHead.
type StateSnapshot struct {
	StateSnapshotHead
	// Header is the header of the committed block the snapshot is anchored to.
	Header types.Header
	KV     []store.KVPair
}

// ErrSnapshotNotAnchored is returned for a block whose header commits the genesis state: there is
// no executed state to export for it yet.
var ErrSnapshotNotAnchored = errors.New("the block header commits no executed state to snapshot")

// StateSnapshotHead returns the latest state record and the state roots of the
// replicated state at that height.
func (node *Node) StateSnapshotHead(ctx context.Context) (StateSnapshotHead, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return StateSnapshotHead{}, err
	}
	return snapshotHead(ctx, runtime)
}

// StateSnapshot exports the state the header of committed block blockHeight commits the app hash
// of; blockHeight 0 selects the latest committed block. The state is read at that height from the
// store's versions, so blocks that commit while the export runs do not change it.
func (node *Node) StateSnapshot(ctx context.Context, blockHeight types.Height) (StateSnapshot, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return StateSnapshot{}, err
	}
	if runtime.Store == nil {
		return StateSnapshot{}, errors.New("state store cannot export a snapshot")
	}
	if blockHeight == 0 {
		latest, err := runtime.LatestState(ctx)
		if err != nil {
			return StateSnapshot{}, err
		}
		blockHeight = latest.Height
	}
	record, err := runtime.BlockByHeight(ctx, blockHeight)
	if err != nil {
		return StateSnapshot{}, err
	}
	header := record.Block.Header
	height, ok := finality.AppHashHeight(header)
	if !ok {
		return StateSnapshot{}, fmt.Errorf("block %d names no validator set height", blockHeight)
	}
	if height == 0 {
		return StateSnapshot{}, fmt.Errorf("%w: block %d", ErrSnapshotNotAnchored, blockHeight)
	}
	head, err := snapshotHeadAt(ctx, runtime, height)
	if err != nil {
		return StateSnapshot{}, err
	}
	headerAppHash, err := runtime.HeaderAppHashAt(ctx, height)
	if err != nil {
		return StateSnapshot{}, err
	}
	if headerAppHash != header.AppHash {
		return StateSnapshot{}, fmt.Errorf("%w: block %d names app hash %x of height %d, the store holds %x", ErrAppHashMismatch, blockHeight, header.AppHash, height, headerAppHash)
	}
	snapshot := StateSnapshot{StateSnapshotHead: head, Header: header}
	for _, namespace := range head.Namespaces {
		pairs, err := runtime.Store.ExportNamespaceAt(ctx, height, namespace)
		if err != nil {
			return StateSnapshot{}, err
		}
		snapshot.KV = append(snapshot.KV, pairs...)
	}
	return snapshot, nil
}

func snapshotHead(ctx context.Context, runtime *vexoruntime.Runtime) (StateSnapshotHead, error) {
	state, err := runtime.LatestState(ctx)
	if err != nil {
		return StateSnapshotHead{}, err
	}
	return snapshotHeadOf(ctx, runtime, state)
}

func snapshotHeadAt(ctx context.Context, runtime *vexoruntime.Runtime, height types.Height) (StateSnapshotHead, error) {
	state, err := runtime.Store.StateByHeight(ctx, height)
	if err != nil {
		return StateSnapshotHead{}, err
	}
	return snapshotHeadOf(ctx, runtime, state)
}

func snapshotHeadOf(ctx context.Context, runtime *vexoruntime.Runtime, state store.StateRecord) (StateSnapshotHead, error) {
	namespaces := runtime.StateNamespaces()
	head := StateSnapshotHead{
		ChainID:    runtime.Config.ChainID,
		State:      state,
		Namespaces: namespaces,
		StateRoots: make([]store.StateRootRecord, 0, len(namespaces)),
	}
	for _, namespace := range namespaces {
		root, err := runtime.StateRoot(ctx, state.Height, namespace)
		if err != nil {
			return StateSnapshotHead{}, err
		}
		head.StateRoots = append(head.StateRoots, root)
	}
	return head, nil
}

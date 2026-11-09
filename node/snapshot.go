package node

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/vexo-network/vexo-consensus/finality"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
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

// StateSnapshotManifest describes the replicated state at the height a committed block header
// commits the app hash of (finality.AppHashHeight): the state record and namespace roots, the
// header of that anchor block, the headers that link the anchor back to the block of the state, and
// the record of that block. A node restoring it checks the anchor against a block hash it trusts and
// the state against the anchor's app hash, and fetches the pairs page by page
// (StateSnapshotPairs), so neither side holds the whole state in memory.
type StateSnapshotManifest struct {
	StateSnapshotHead
	// Header is the header of the committed block the snapshot is anchored to.
	Header types.Header
	// Links are the headers of blocks State.Height+1 .. Header.Height-1, ascending: with the
	// anchor they form the chain of PreviousBlockHash links from the anchor back to Block.
	Links []types.Header
	// Block is the committed block of height State.Height with its quorum certificate, which a
	// restored node continues the chain from.
	Block store.BlockRecord
}

// MaxSnapshotAnchorLinks bounds the headers between a snapshot's state and its anchor. A header
// names the state of its parent's height in normal operation; the gap grows only while views keep
// timing out, and an anchor that far from its state is refused rather than served.
const MaxSnapshotAnchorLinks = 1024

// MaxSnapshotPairsPage bounds the pairs one StateSnapshotPairs page returns.
const MaxSnapshotPairsPage = 10_000

// ErrSnapshotNotAnchored is returned for a block whose header commits the genesis state: there is
// no executed state to export for it yet.
var ErrSnapshotNotAnchored = errors.New("the block header commits no executed state to snapshot")

// ErrSnapshotPruned reports an anchor whose state this node no longer retains.
var ErrSnapshotPruned = errors.New("the state the block commits is pruned")

// ErrSnapshotAnchorTooFar reports an anchor more than MaxSnapshotAnchorLinks blocks above its state.
var ErrSnapshotAnchorTooFar = errors.New("the block is too far above the state it commits")

// ErrSnapshotPage reports an invalid page request.
var ErrSnapshotPage = errors.New("invalid snapshot page request")

// StateSnapshotHead returns the latest state record and the state roots of the
// replicated state at that height.
func (node *Node) StateSnapshotHead(ctx context.Context) (StateSnapshotHead, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return StateSnapshotHead{}, err
	}
	return snapshotHead(ctx, runtime)
}

// SnapshotSource serves the snapshots of the committed states of a store: the manifest of the
// snapshot a committed block anchors and the pages of its pairs, each read from the store's
// versions of that state, so no snapshot is ever held in memory whole.
type SnapshotSource struct {
	Store   store.Store
	ChainID string
	// Namespaces are the namespaces of the replicated state (runtime.StateNamespaces).
	Namespaces []string
}

// SnapshotSourceOf returns the snapshot source of the store of runtime.
func SnapshotSourceOf(runtime *vexoruntime.Runtime) (SnapshotSource, error) {
	if runtime.Store == nil {
		return SnapshotSource{}, errors.New("state store cannot export a snapshot")
	}
	return SnapshotSource{Store: runtime.Store, ChainID: runtime.Config.ChainID, Namespaces: runtime.StateNamespaces()}, nil
}

func (node *Node) snapshotSource() (SnapshotSource, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return SnapshotSource{}, err
	}
	return SnapshotSourceOf(runtime)
}

// StateSnapshotManifest returns the manifest of the snapshot anchored to committed block
// blockHeight (see SnapshotSource.Manifest).
func (node *Node) StateSnapshotManifest(ctx context.Context, blockHeight types.Height) (StateSnapshotManifest, error) {
	source, err := node.snapshotSource()
	if err != nil {
		return StateSnapshotManifest{}, err
	}
	return source.Manifest(ctx, blockHeight)
}

// StateSnapshotPairs returns a page of the snapshot anchored to committed block blockHeight (see
// SnapshotSource.Pairs).
func (node *Node) StateSnapshotPairs(ctx context.Context, blockHeight types.Height, namespace string, after []byte, limit int) ([]store.KVPair, bool, error) {
	source, err := node.snapshotSource()
	if err != nil {
		return nil, false, err
	}
	return source.Pairs(ctx, blockHeight, namespace, after, limit)
}

// EarliestSnapshotHeight returns the lowest committed block this node serves the snapshot of (see
// SnapshotSource.EarliestHeight).
func (node *Node) EarliestSnapshotHeight(ctx context.Context) (types.Height, error) {
	source, err := node.snapshotSource()
	if err != nil {
		return 0, err
	}
	return source.EarliestHeight(ctx)
}

// EarliestHeight returns the lowest committed block whose snapshot the store still serves: the
// lowest anchor whose state and state block are retained.
func (source SnapshotSource) EarliestHeight(ctx context.Context) (types.Height, error) {
	index, err := source.Store.BlockIndex(ctx)
	if err != nil {
		return 0, err
	}
	// The state an anchor commits is at most MaxSnapshotAnchorLinks+1 blocks below it, so the
	// lowest servable anchor is within that distance of the lowest retained block.
	last := min(index.LatestHeight, index.EarliestHeight+MaxSnapshotAnchorLinks+1)
	for height := max(index.EarliestHeight, 1); height <= last; height++ {
		_, _, err := source.anchor(ctx, height)
		if err == nil {
			return height, nil
		}
		if !errors.Is(err, ErrSnapshotNotAnchored) && !errors.Is(err, ErrSnapshotPruned) && !errors.Is(err, store.ErrBlockNotFound) {
			return 0, err
		}
	}
	return 0, ErrSnapshotPruned
}

// Manifest returns the manifest of the snapshot anchored to committed block blockHeight; 0 selects
// the latest committed block. It reads no key-value pair.
func (source SnapshotSource) Manifest(ctx context.Context, blockHeight types.Height) (StateSnapshotManifest, error) {
	if blockHeight == 0 {
		latest, err := source.Store.LatestState(ctx)
		if err != nil {
			return StateSnapshotManifest{}, err
		}
		blockHeight = latest.Height
	}
	header, state, err := source.anchor(ctx, blockHeight)
	if err != nil {
		return StateSnapshotManifest{}, err
	}
	roots, err := source.roots(ctx, state.Height)
	if err != nil {
		return StateSnapshotManifest{}, err
	}
	block, err := source.Store.BlockByHeight(ctx, state.Height)
	if err != nil {
		return StateSnapshotManifest{}, err
	}
	manifest := StateSnapshotManifest{
		StateSnapshotHead: StateSnapshotHead{ChainID: source.ChainID, State: state, Namespaces: slices.Clone(source.Namespaces), StateRoots: roots},
		Header:            header,
		Block:             store.BlockRecord{Block: block.Block, Hash: block.Hash, AppHash: block.AppHash, StateRoots: block.StateRoots, QuorumCert: block.QuorumCert},
	}
	for link := state.Height + 1; link < blockHeight; link++ {
		record, err := source.Store.BlockByHeight(ctx, link)
		if err != nil {
			return StateSnapshotManifest{}, err
		}
		manifest.Links = append(manifest.Links, record.Block.Header)
	}
	return manifest, nil
}

// Pairs returns a page of at most limit pairs of namespace in the state committed block
// blockHeight anchors, in key order after after (from the first key for nil), and whether more
// follow. A page costs a seek per pair, whatever the size of the state.
func (source SnapshotSource) Pairs(ctx context.Context, blockHeight types.Height, namespace string, after []byte, limit int) ([]store.KVPair, bool, error) {
	if blockHeight == 0 || limit <= 0 || limit > MaxSnapshotPairsPage || !slices.Contains(source.Namespaces, namespace) {
		return nil, false, fmt.Errorf("%w: block %d namespace %q limit %d", ErrSnapshotPage, blockHeight, namespace, limit)
	}
	_, state, err := source.anchor(ctx, blockHeight)
	if err != nil {
		return nil, false, err
	}
	history, ok := source.Store.(store.HistoricalSnapshotKVStore)
	if !ok {
		return nil, false, errors.New("state store cannot export a past state")
	}
	return history.ExportRangeAt(ctx, state.Height, namespace, after, limit)
}

// anchor returns the header of committed block blockHeight and the record of the state its app
// hash commits, after checking that the store holds that state, and its block, with that header app
// hash.
func (source SnapshotSource) anchor(ctx context.Context, blockHeight types.Height) (types.Header, store.StateRecord, error) {
	record, err := source.Store.BlockByHeight(ctx, blockHeight)
	if err != nil {
		return types.Header{}, store.StateRecord{}, err
	}
	header := record.Block.Header
	height, ok := finality.AppHashHeight(header)
	if !ok {
		return types.Header{}, store.StateRecord{}, fmt.Errorf("block %d names no validator set height", blockHeight)
	}
	if height == 0 {
		return types.Header{}, store.StateRecord{}, fmt.Errorf("%w: block %d", ErrSnapshotNotAnchored, blockHeight)
	}
	if blockHeight-height-1 > MaxSnapshotAnchorLinks {
		return types.Header{}, store.StateRecord{}, fmt.Errorf("%w: block %d commits the state of height %d", ErrSnapshotAnchorTooFar, blockHeight, height)
	}
	pruned := func(what string) error {
		return fmt.Errorf("%w: the %s of height %d block %d commits is not retained", ErrSnapshotPruned, what, height, blockHeight)
	}
	state, err := source.Store.StateByHeight(ctx, height)
	if errors.Is(err, store.ErrStateNotFound) {
		return types.Header{}, store.StateRecord{}, pruned("state")
	}
	if err != nil {
		return types.Header{}, store.StateRecord{}, err
	}
	registry, err := source.Store.StateRoot(ctx, height, validator.RegistryNamespace)
	if errors.Is(err, store.ErrStateRootNotFound) {
		return types.Header{}, store.StateRecord{}, pruned("validator registry root")
	}
	if err != nil {
		return types.Header{}, store.StateRecord{}, err
	}
	if headerAppHash := finality.HeaderAppHash(state.AppHash, registry.Root); headerAppHash != header.AppHash {
		return types.Header{}, store.StateRecord{}, fmt.Errorf("%w: block %d names app hash %x of height %d, the store holds %x", ErrAppHashMismatch, blockHeight, header.AppHash, height, headerAppHash)
	}
	if _, err := source.Store.BlockByHeight(ctx, height); errors.Is(err, store.ErrBlockNotFound) {
		return types.Header{}, store.StateRecord{}, pruned("block")
	} else if err != nil {
		return types.Header{}, store.StateRecord{}, err
	}
	return header, state, nil
}

func (source SnapshotSource) roots(ctx context.Context, height types.Height) ([]store.StateRootRecord, error) {
	roots := make([]store.StateRootRecord, 0, len(source.Namespaces))
	for _, namespace := range source.Namespaces {
		root, err := source.Store.StateRoot(ctx, height, namespace)
		if errors.Is(err, store.ErrStateRootNotFound) {
			return nil, fmt.Errorf("%w: the %s root of height %d is not retained", ErrSnapshotPruned, namespace, height)
		}
		if err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}
	return roots, nil
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

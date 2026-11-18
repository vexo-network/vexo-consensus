package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/cockroachdb/pebble/v2"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
)

// State trees.
//
// The store keeps the state tree (stateproof) of every namespace next to its
// key-value pairs, as of the current state. Every write that changes a pair
// changes the tree's nodes on that key's path in the same atomic batch, so the
// root of a namespace, and the proof of any of its keys, is read from a few
// nodes instead of being recomputed from the whole namespace. Roots of past
// heights are the state root records; proofs of past heights are rebuilt from
// the namespace's versions.
//
// Tree nodes are versioned like keys (see State history): a change of a node at
// a height records the node as it was before, an empty value for a node that
// did not exist, as the node's version at that height. The tree of any retained
// height is therefore read node by node, so a historical proof costs the nodes
// on one path, not the namespace.

var (
	stateTreePrefix        = []byte("tree:")
	stateTreeHistoryPrefix = []byte("treeh:")
)

// ErrStateTreeCorrupt reports a state tree that does not match the pairs it commits.
var ErrStateTreeCorrupt = errors.New("state tree does not match the stored state")

// stateTreeNamespaceID identifies namespace under the tree prefixes: its length, then its name, so
// no namespace's keys are a prefix of another's.
func stateTreeNamespaceID(prefix []byte, namespace string) []byte {
	key := append([]byte(nil), prefix...)
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(namespace)))
	key = append(key, length[:]...)
	return append(key, []byte(namespace)...)
}

func stateTreeNamespacePrefix(namespace string) []byte {
	return stateTreeNamespaceID(stateTreePrefix, namespace)
}

func stateTreeNodeKey(namespace string, key stateproof.NodeKey) []byte {
	return append(stateTreeNamespacePrefix(namespace), key.Bytes()...)
}

// stateTreeHistoryGroup is the prefix every version of a node shares; encoded node keys are
// prefix-free (their depth fixes their length), so the group is exact.
func stateTreeHistoryGroup(namespace string, key stateproof.NodeKey) []byte {
	return append(stateTreeNamespaceID(stateTreeHistoryPrefix, namespace), key.Bytes()...)
}

func stateTreeHistoryKey(group []byte, height types.Height) []byte {
	key := append([]byte(nil), group...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(key, buffer[:]...)
}

// treeNodes serves the stored nodes of one namespace tree.
type treeNodes struct {
	reader    dbReader
	namespace string
	cache     treeCacheView
}

func (nodes treeNodes) Node(key stateproof.NodeKey) (stateproof.Node, bool, error) {
	encoded, found, err := nodes.encoded(key)
	if err != nil || !found {
		return stateproof.Node{}, false, err
	}
	return decodeStoredNode(encoded)
}

// encoded returns the stored encoding of the node at key, from the cache when it holds it. The
// caller does not modify it.
func (nodes treeNodes) encoded(key stateproof.NodeKey) ([]byte, bool, error) {
	dbKey := stateTreeNodeKey(nodes.namespace, key)
	if encoded, found := nodes.cache.get(dbKey); found {
		return encoded, true, nil
	}
	encoded, err := getValue(nodes.reader, dbKey)
	if errors.Is(err, pebble.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	nodes.cache.add(dbKey, encoded)
	return encoded, true, nil
}

// decodeStoredNode decodes a stored node or node version; an empty version is a removed node.
func decodeStoredNode(encoded []byte) (stateproof.Node, bool, error) {
	if len(encoded) == 0 {
		return stateproof.Node{}, false, nil
	}
	node, err := stateproof.DecodeNode(encoded)
	if err != nil {
		return stateproof.Node{}, false, err
	}
	return node, true, nil
}

// treeNodesAt serves the nodes of one namespace tree as they were at a committed height.
type treeNodesAt struct {
	reader    dbReader
	namespace string
	height    types.Height
	cache     treeCacheView
}

func (nodes treeNodesAt) Node(key stateproof.NodeKey) (stateproof.Node, bool, error) {
	group := stateTreeHistoryGroup(nodes.namespace, key)
	versions := newIter(nodes.reader, prefixRange(group))
	defer versions.Close()
	if !versionAbove(versions, group, nodes.height) {
		if err := versions.Error(); err != nil {
			return stateproof.Node{}, false, err
		}
		return treeNodes{reader: nodes.reader, namespace: nodes.namespace, cache: nodes.cache}.Node(key)
	}
	return decodeStoredNode(versions.Value())
}

// stateTrees applies writes to the trees of the namespaces they touch.
type stateTrees struct {
	reader dbReader
	cache  treeCacheView
	trees  map[string]*stateproof.Tree
	// computed holds the changes of namespaces whose writes an earlier root computation already
	// applied to the same trees.
	computed map[string][]stateproof.NodeChange
}

func newStateTrees(reader dbReader, cache treeCacheView) *stateTrees {
	return &stateTrees{reader: reader, cache: cache, trees: make(map[string]*stateproof.Tree), computed: make(map[string][]stateproof.NodeChange)}
}

// applyNamespaces applies writes to the trees, reusing for each namespace the changes a root
// computation over the same trees and the same writes of that namespace kept.
func (trees *stateTrees) applyNamespaces(computed *computedTrees, writes []KVWrite) error {
	for _, namespace := range writeNamespaces(writes) {
		namespaceWrites, digest := writesOf(namespace, writes)
		if found, ok := computed.get(namespace, trees.cache, digest); ok {
			trees.computed[namespace] = found.changes
			continue
		}
		for _, write := range namespaceWrites {
			if err := trees.apply(write); err != nil {
				return err
			}
		}
	}
	return nil
}

// changes returns the node changes of namespace.
func (trees *stateTrees) changes(namespace string) []stateproof.NodeChange {
	if changes, found := trees.computed[namespace]; found {
		return changes
	}
	return trees.trees[namespace].Changes()
}

func (trees *stateTrees) nodes(namespace string) treeNodes {
	return treeNodes{reader: trees.reader, namespace: namespace, cache: trees.cache}
}

func (trees *stateTrees) tree(namespace string) *stateproof.Tree {
	tree, found := trees.trees[namespace]
	if !found {
		tree = stateproof.NewTree(namespace, trees.nodes(namespace))
		trees.trees[namespace] = tree
	}
	return tree
}

func (trees *stateTrees) apply(write KVWrite) error {
	tree := trees.tree(write.Namespace)
	if write.Delete {
		return tree.Delete(write.Key)
	}
	return tree.Set(write.Key, write.Value)
}

// persist adds the node changes of every tree to batch. With versioned set, each change also
// records the node as it was before as its version at height; with existing set, a version
// already stored at height is kept (see historyBatch).
func (trees *stateTrees) persist(batch *pebble.Batch, versioned bool, height types.Height, existing bool) error {
	namespaces := make([]string, 0, len(trees.trees)+len(trees.computed))
	for namespace := range trees.trees {
		namespaces = append(namespaces, namespace)
	}
	for namespace := range trees.computed {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		for _, change := range trees.changes(namespace) {
			if versioned {
				if err := trees.recordVersion(batch, namespace, change.Key, height, existing); err != nil {
					return err
				}
			}
			key := stateTreeNodeKey(namespace, change.Key)
			if change.Node == nil {
				batch.Delete(key, nil)
				continue
			}
			batch.Set(key, stateproof.EncodeNode(*change.Node), nil)
		}
	}
	return nil
}

func (trees *stateTrees) recordVersion(batch *pebble.Batch, namespace string, key stateproof.NodeKey, height types.Height, existing bool) error {
	versionKey := stateTreeHistoryKey(stateTreeHistoryGroup(namespace, key), height)
	if existing {
		found, err := hasKey(trees.reader, versionKey)
		if err != nil || found {
			return err
		}
	}
	prior, _, err := trees.nodes(namespace).encoded(key)
	if err != nil {
		return err
	}
	putVersion(batch, stateTreeVersionIndexPrefix, versionKey, height, append([]byte{}, prior...))
	return nil
}

// Root returns the current root of namespace.
func (store *PebbleStore) Root(ctx context.Context, namespace string) (types.Hash, error) {
	return store.RootWithWrites(ctx, namespace, nil)
}

// RootWithWrites returns the root namespace would have after writes, without applying them.
func (store *PebbleStore) RootWithWrites(ctx context.Context, namespace string, writes []KVWrite) (types.Hash, error) {
	select {
	case <-ctx.Done():
		return types.Hash{}, ctx.Err()
	default:
	}
	if namespace == "" {
		return types.Hash{}, ErrInvalidNamespace
	}
	namespaceWrites, digest := writesOf(namespace, writes)
	for _, write := range namespaceWrites {
		if len(write.Key) == 0 {
			return types.Hash{}, ErrInvalidKey
		}
	}
	snapshot, cache := store.snapshot()
	defer snapshot.Close()
	if found, ok := store.computed.get(namespace, cache, digest); ok {
		return found.root, nil
	}
	tree := stateproof.NewTree(namespace, treeNodes{reader: snapshot, namespace: namespace, cache: cache})
	for _, write := range namespaceWrites {
		var err error
		if write.Delete {
			err = tree.Delete(write.Key)
		} else {
			err = tree.Set(write.Key, write.Value)
		}
		if err != nil {
			return types.Hash{}, err
		}
	}
	root, err := tree.Root()
	if err != nil {
		return types.Hash{}, err
	}
	store.computed.put(namespace, cache, computedTree{digest: digest, root: root, changes: tree.Changes()})
	return root, nil
}

// Computed trees.
//
// The runtime computes the root of every namespace over a block's writes twice before it commits
// them, for the app hash and for the state roots, and the commit applies the same writes to the
// trees again. The store keeps, for each namespace, the root and node changes of the last
// computation, keyed by the tree node generation it read at (see Tree node cache) and a digest of
// the namespace's writes in order. The same writes applied to the same trees make the same
// changes, so a later root computation and the commit reuse them; anything else computes afresh.

type computedTree struct {
	generation uint64
	digest     types.Hash
	root       types.Hash
	changes    []stateproof.NodeChange
}

type computedTrees struct {
	mu    sync.Mutex
	trees map[string]computedTree
}

func newComputedTrees() *computedTrees {
	return &computedTrees{trees: make(map[string]computedTree)}
}

// get returns the computation of namespace's writes with digest over the trees at the view's
// generation.
func (computed *computedTrees) get(namespace string, view treeCacheView, digest types.Hash) (computedTree, bool) {
	if view.cache == nil {
		return computedTree{}, false
	}
	computed.mu.Lock()
	defer computed.mu.Unlock()
	found, ok := computed.trees[namespace]
	if !ok || found.generation != view.generation || found.digest != digest {
		return computedTree{}, false
	}
	return found, true
}

// put keeps a computation over the trees at the view's generation, in place of the namespace's
// previous one.
func (computed *computedTrees) put(namespace string, view treeCacheView, tree computedTree) {
	if view.cache == nil {
		return
	}
	tree.generation = view.generation
	computed.mu.Lock()
	defer computed.mu.Unlock()
	computed.trees[namespace] = tree
}

// writesOf returns the writes of namespace in order and their digest.
func writesOf(namespace string, writes []KVWrite) ([]KVWrite, types.Hash) {
	hasher := sha256.New()
	var length [binary.MaxVarintLen64]byte
	selected := make([]KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Namespace != namespace {
			continue
		}
		selected = append(selected, write)
		kind := byte(0)
		if write.Delete {
			kind = 1
		}
		hasher.Write([]byte{kind})
		hasher.Write(length[:binary.PutUvarint(length[:], uint64(len(write.Key)))])
		hasher.Write(write.Key)
		if !write.Delete {
			hasher.Write(length[:binary.PutUvarint(length[:], uint64(len(write.Value)))])
			hasher.Write(write.Value)
		}
	}
	var digest types.Hash
	hasher.Sum(digest[:0])
	return selected, digest
}

// writeNamespaces returns the namespaces writes touch, sorted.
func writeNamespaces(writes []KVWrite) []string {
	seen := make(map[string]struct{})
	namespaces := make([]string, 0)
	for _, write := range writes {
		if _, found := seen[write.Namespace]; !found {
			seen[write.Namespace] = struct{}{}
			namespaces = append(namespaces, write.Namespace)
		}
	}
	sort.Strings(namespaces)
	return namespaces
}

// StateProof proves the current value of key in namespace, or its absence, against the
// namespace's current root, read from one consistent view of the store.
func (store *PebbleStore) StateProof(ctx context.Context, namespace string, key []byte) (root types.Hash, value []byte, exists bool, proof stateproof.Proof, err error) {
	select {
	case <-ctx.Done():
		return types.Hash{}, nil, false, stateproof.Proof{}, ctx.Err()
	default:
	}
	if namespace == "" {
		return types.Hash{}, nil, false, stateproof.Proof{}, ErrInvalidNamespace
	}
	if len(key) == 0 {
		return types.Hash{}, nil, false, stateproof.Proof{}, ErrInvalidKey
	}
	snapshot, cache := store.snapshot()
	defer snapshot.Close()
	tree := stateproof.NewTree(namespace, treeNodes{reader: snapshot, namespace: namespace, cache: cache})
	root, err = tree.Root()
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	proof, leaf, err := tree.Prove(key)
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	stored, err := getValue(snapshot, kvKey(namespace, key))
	switch {
	case errors.Is(err, pebble.ErrNotFound):
		if leaf != nil && proof.Neighbor == nil {
			return types.Hash{}, nil, false, stateproof.Proof{}, fmt.Errorf("%w: namespace %q holds a leaf for an absent key", ErrStateTreeCorrupt, namespace)
		}
		return root, nil, false, proof, nil
	case err != nil:
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	if leaf == nil || proof.Neighbor != nil || leaf.ValueHash != stateproof.LeafHash(namespace, key, stored) {
		return types.Hash{}, nil, false, stateproof.Proof{}, fmt.Errorf("%w: namespace %q has no leaf for the value of a key", ErrStateTreeCorrupt, namespace)
	}
	return root, append([]byte(nil), stored...), true, proof, nil
}

// StateProofAt proves the value of key in namespace at committed height, or its absence, against
// the namespace's root at that height, from the tree as it was then: it reads the versions of the
// nodes on the key's path, not the namespace.
func (store *PebbleStore) StateProofAt(ctx context.Context, height types.Height, namespace string, key []byte) (root types.Hash, value []byte, exists bool, proof stateproof.Proof, err error) {
	select {
	case <-ctx.Done():
		return types.Hash{}, nil, false, stateproof.Proof{}, ctx.Err()
	default:
	}
	if namespace == "" {
		return types.Hash{}, nil, false, stateproof.Proof{}, ErrInvalidNamespace
	}
	if len(key) == 0 {
		return types.Hash{}, nil, false, stateproof.Proof{}, ErrInvalidKey
	}
	if err := store.requireCommittedState(height); err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	snapshot, cache := store.snapshot()
	defer snapshot.Close()
	tree := stateproof.NewTree(namespace, treeNodesAt{reader: snapshot, namespace: namespace, height: height, cache: cache})
	root, err = tree.Root()
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	proof, leaf, err := tree.Prove(key)
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	stored, err := getAt(snapshot, height, namespace, key)
	if errors.Is(err, ErrKeyNotFound) {
		if leaf != nil && proof.Neighbor == nil {
			return types.Hash{}, nil, false, stateproof.Proof{}, fmt.Errorf("%w: namespace %q at height %d holds a leaf for an absent key", ErrStateTreeCorrupt, namespace, height)
		}
		return root, nil, false, proof, nil
	}
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	if leaf == nil || proof.Neighbor != nil || leaf.ValueHash != stateproof.LeafHash(namespace, key, stored) {
		return types.Hash{}, nil, false, stateproof.Proof{}, fmt.Errorf("%w: namespace %q at height %d has no leaf for the value of a key", ErrStateTreeCorrupt, namespace, height)
	}
	return root, stored, true, proof, nil
}

// replaceStateTree adds to batch the removal of namespace's tree and its versions, and the nodes
// of the tree of pairs.
func (store *PebbleStore) replaceStateTree(ctx context.Context, batch *pebble.Batch, namespace string, pairs []KVPair) error {
	for _, prefix := range [][]byte{stateTreeNamespacePrefix(namespace), stateTreeNamespaceID(stateTreeHistoryPrefix, namespace)} {
		if err := deletePrefix(ctx, store.db, batch, prefix); err != nil {
			return err
		}
	}
	nodes, err := stateproof.Build(namespace, stateProofPairs(pairs))
	if err != nil {
		return err
	}
	for _, change := range nodes {
		batch.Set(stateTreeNodeKey(namespace, change.Key), stateproof.EncodeNode(*change.Node), nil)
	}
	return nil
}

// deletePrefix adds to batch the deletion of every key under prefix.
func deletePrefix(ctx context.Context, reader dbReader, batch *pebble.Batch, prefix []byte) error {
	iterator := newIter(reader, prefixRange(prefix))
	defer iterator.Close()
	for valid := iterator.First(); valid; valid = iterator.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...), nil)
	}
	return iterator.Error()
}

// NamespaceImport writes a namespace page by page, so a namespace of any size is imported without
// being held in memory: the pairs of each page, in ascending key order after those of the pages
// before, are written with the nodes they change in the namespace's tree in one batch.
type NamespaceImport struct {
	store     *PebbleStore
	namespace string
	last      []byte
}

// ErrImportOrder reports pairs that do not follow the pairs already imported in key order.
var ErrImportOrder = errors.New("imported pairs are not in ascending key order")

// BeginNamespaceImport empties namespace, its versions and its tree, and returns the import that
// fills it. The pairs imported become the namespace's base state.
func (store *PebbleStore) BeginNamespaceImport(ctx context.Context, namespace string) (*NamespaceImport, error) {
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	store.writeMu.Lock()
	defer store.writeMu.Unlock()
	batch := store.db.NewBatch()
	defer batch.Close()
	if err := store.deleteNamespaceHistory(ctx, batch, namespace); err != nil {
		return nil, err
	}
	for _, prefix := range [][]byte{kvNamespacePrefix(namespace), stateTreeNamespacePrefix(namespace), stateTreeNamespaceID(stateTreeHistoryPrefix, namespace)} {
		if err := deletePrefix(ctx, store.db, batch, prefix); err != nil {
			return nil, err
		}
	}
	if err := store.writeBatch(batch); err != nil {
		return nil, err
	}
	return &NamespaceImport{store: store, namespace: namespace}, nil
}

// Add imports pairs, whose keys must ascend strictly after every key imported before.
func (in *NamespaceImport) Add(ctx context.Context, pairs []KVPair) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	in.store.writeMu.Lock()
	defer in.store.writeMu.Unlock()
	trees := newStateTrees(in.store.db, in.store.liveTreeView())
	batch := in.store.db.NewBatch()
	defer batch.Close()
	for _, pair := range pairs {
		if pair.Namespace != "" && pair.Namespace != in.namespace {
			return ErrInvalidNamespace
		}
		if len(pair.Key) == 0 {
			return ErrInvalidKey
		}
		if in.last != nil && bytes.Compare(pair.Key, in.last) <= 0 {
			return fmt.Errorf("%w: %x after %x in namespace %q", ErrImportOrder, pair.Key, in.last, in.namespace)
		}
		in.last = append(in.last[:0], pair.Key...)
		if err := trees.apply(KVWrite{Namespace: in.namespace, Key: pair.Key, Value: pair.Value}); err != nil {
			return err
		}
		batch.Set(kvKey(in.namespace, pair.Key), append([]byte(nil), pair.Value...), nil)
	}
	if err := trees.persist(batch, false, 0, false); err != nil {
		return err
	}
	return in.store.writeBatch(batch)
}

// Root returns the root of the pairs imported so far.
func (in *NamespaceImport) Root(ctx context.Context) (types.Hash, error) {
	return in.store.Root(ctx, in.namespace)
}

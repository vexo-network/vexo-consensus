package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/cockroachdb/pebble/v2"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
)

// State trees.
//
// The store keeps the state tree (stateproof) of every namespace next to its key-value pairs.
// Every write that changes a pair changes the tree's nodes on that key's path in the same atomic
// batch, so the root of a namespace, and the proof of any of its keys, is read from a few nodes
// instead of being recomputed from the whole namespace.
//
// Nodes are versioned and never rewritten by a later height. A batch that changes a tree at a
// height writes each node it changes as a node of that version, under
//
//	tree: | namespace | version | node key
//
// and an internal node stores, after its encoding, the uvarint versions of its two halves (zero
// for an empty half), so a reader finds every node from its parent: a node exists exactly when its
// parent's half is not empty. A root record (troot: | namespace | height) holds, for each height a
// batch changed the namespace's tree at, whether the tree has a root then; the root's version is
// that height. The tree of any retained height is therefore read from its last root record at or
// below the height down, node by node, like the current tree, which is the last record's: a
// historical proof costs the nodes on one path.
//
// The node a change replaces stays for the heights below and is listed as orphaned at the height
// that replaced it (torph:, one entry per batch; see keyList). A prune that retains from a height
// drops the nodes orphaned at or below it and the root records no retained height reads. A batch
// at the version of a node it changes, as direct writes and then the block of the same height, or
// the pages of an import, are, rewrites or deletes that node in place: no height reads it.
//
// Versions only grow, so the nodes, root records and orphan lists a block adds follow the ones
// already stored within each namespace and height range, rather than being spread over the keys
// the history already holds, which keeps compaction small.

var (
	stateTreePrefix       = []byte("tree:")
	stateTreeRootPrefix   = []byte("troot:")
	stateTreeOrphanPrefix = []byte("torph:")
)

// ErrStateTreeCorrupt reports a state tree that does not match the pairs it commits.
var ErrStateTreeCorrupt = errors.New("state tree does not match the stored state")

// stateTreeNamespaceID identifies namespace under the tree prefixes: its length, then its name, so
// no namespace's keys are a prefix of another's.
func stateTreeNamespaceID(prefix []byte, namespace string) []byte {
	return appendStateTreeNamespaceID(make([]byte, 0, len(prefix)+2+len(namespace)+8), prefix, namespace)
}

func appendStateTreeNamespaceID(dst []byte, prefix []byte, namespace string) []byte {
	dst = append(dst, prefix...)
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(namespace)))
	return append(dst, namespace...)
}

func stateTreeNamespacePrefix(namespace string) []byte {
	return stateTreeNamespaceID(stateTreePrefix, namespace)
}

// stateTreeNodeKey is the key of the node at key of version in namespace's tree, built in one
// allocation: a tree read builds one for every node it reads.
func stateTreeNodeKey(namespace string, version types.Height, key stateproof.NodeKey) []byte {
	return appendStateTreeNodeKey(make([]byte, 0, len(stateTreePrefix)+2+len(namespace)+8+key.EncodedLen()), namespace, version, key)
}

// appendStateTreeNodeKey appends the key stateTreeNodeKey returns to dst.
func appendStateTreeNodeKey(dst []byte, namespace string, version types.Height, key stateproof.NodeKey) []byte {
	dst = appendStateTreeNamespaceID(dst, stateTreePrefix, namespace)
	dst = binary.BigEndian.AppendUint64(dst, uint64(version))
	return key.AppendBytes(dst)
}

// stateTreeRootKey is the key of namespace's root record at height.
func stateTreeRootKey(namespace string, height types.Height) []byte {
	return binary.BigEndian.AppendUint64(stateTreeNamespaceID(stateTreeRootPrefix, namespace), uint64(height))
}

// treeRootAt returns the version of namespace's root at height, the last height at or below it a
// batch changed the tree at, and whether the tree has a root then. math.MaxUint64 reads the current
// tree.
func treeRootAt(reader dbReader, namespace string, height types.Height) (types.Height, bool, error) {
	prefix := stateTreeNamespaceID(stateTreeRootPrefix, namespace)
	span := prefixRange(prefix)
	if height < math.MaxUint64 {
		span.upper = stateTreeRootKey(namespace, height+1)
	}
	records := newIter(reader, span)
	defer records.Close()
	if !records.Last() {
		return 0, false, records.Error()
	}
	key, value := records.Key(), records.Value()
	if len(key) != len(prefix)+8 || len(value) != 1 || value[0] > 1 {
		return 0, false, ErrInvalidStateRecord
	}
	return types.Height(binary.BigEndian.Uint64(key[len(prefix):])), value[0] == 1, nil
}

// encodeStoredNode encodes node with the versions of its halves; a leaf has none.
func encodeStoredNode(node stateproof.Node, left types.Height, right types.Height) []byte {
	encoded := stateproof.AppendNode(make([]byte, 0, stateproof.EncodedNodeSize+2*binary.MaxVarintLen64), node)
	if node.Leaf != nil {
		return encoded
	}
	encoded = binary.AppendUvarint(encoded, uint64(left))
	return binary.AppendUvarint(encoded, uint64(right))
}

// decodeStoredNode decodes a stored node and the versions of its halves.
func decodeStoredNode(encoded []byte) (storedNode, error) {
	if len(encoded) < stateproof.EncodedNodeSize {
		return storedNode{}, stateproof.ErrInvalidNode
	}
	node, err := stateproof.DecodeNode(encoded[:stateproof.EncodedNodeSize])
	if err != nil {
		return storedNode{}, err
	}
	rest := encoded[stateproof.EncodedNodeSize:]
	if node.Leaf != nil {
		if len(rest) != 0 {
			return storedNode{}, stateproof.ErrInvalidNode
		}
		return storedNode{node: node}, nil
	}
	left, read := binary.Uvarint(rest)
	if read <= 0 {
		return storedNode{}, stateproof.ErrInvalidNode
	}
	right, more := binary.Uvarint(rest[read:])
	if more <= 0 || read+more != len(rest) {
		return storedNode{}, stateproof.ErrInvalidNode
	}
	return storedNode{node: node, left: types.Height(left), right: types.Height(right)}, nil
}

// nodeVersions holds the versions of the stored nodes of one namespace tree that a reader can
// reach: the root's, and those of the halves of every node it read.
type nodeVersions struct {
	root    types.Height
	hasRoot bool
	halves  map[stateproof.NodeKey]types.Height
	// key is the buffer the reader builds the database key of a node in.
	key []byte
}

// halvesPerWrite is about how many halves a tree learns per write: each write reads the nodes on
// its path below those earlier writes read, about eight of them in a tree of tens of thousands of
// keys, each with two halves. Sizing the map for them up front saves growing it while it fills.
const halvesPerWrite = 8

// newNodeVersions returns the versions of a tree whose root is of version root, sized for writes.
func newNodeVersions(root types.Height, hasRoot bool, writes int) *nodeVersions {
	return &nodeVersions{root: root, hasRoot: hasRoot, halves: make(map[stateproof.NodeKey]types.Height, writes*halvesPerWrite)}
}

// of returns the version of the stored node at key, and false when no node is stored there as far
// as the nodes read so far tell: a node is stored exactly where its parent's half is not empty.
func (versions *nodeVersions) of(key stateproof.NodeKey) (types.Height, bool) {
	if key == (stateproof.NodeKey{}) {
		return versions.root, versions.hasRoot
	}
	version, found := versions.halves[key]
	return version, found
}

// learn keeps the versions of the halves of the node read at key.
func (versions *nodeVersions) learn(key stateproof.NodeKey, stored storedNode) {
	if stored.node.Leaf != nil {
		return
	}
	if stored.node.Left != (types.Hash{}) {
		versions.halves[key.Child(0)] = stored.left
	}
	if stored.node.Right != (types.Hash{}) {
		versions.halves[key.Child(1)] = stored.right
	}
}

// treeNodes serves the stored nodes of one namespace tree, from its root record down.
type treeNodes struct {
	reader    dbReader
	namespace string
	cache     treeCacheView
	// historical reports a tree of an earlier height, whose nodes are not cached: the cache keeps
	// the nodes of the current trees, which such a tree reads from it.
	historical bool
	versions   *nodeVersions
}

// newTreeNodes serves namespace's tree as it was at height, math.MaxUint64 for the current one, to
// a reader that applies about writes writes (zero for a proof).
func newTreeNodes(reader dbReader, namespace string, cache treeCacheView, height types.Height, writes int) (treeNodes, error) {
	root, hasRoot, err := treeRootAt(reader, namespace, height)
	if err != nil {
		return treeNodes{}, err
	}
	return treeNodes{reader: reader, namespace: namespace, cache: cache, historical: height != math.MaxUint64, versions: newNodeVersions(root, hasRoot, writes)}, nil
}

func (nodes treeNodes) Node(key stateproof.NodeKey) (stateproof.Node, bool, error) {
	version, found := nodes.versions.of(key)
	if !found {
		return stateproof.Node{}, false, nil
	}
	dbKey := appendStateTreeNodeKey(nodes.versions.key[:0], nodes.namespace, version, key)
	nodes.versions.key = dbKey
	stored, found := nodes.cache.get(dbKey)
	if !found {
		value, closer, err := nodes.reader.Get(dbKey)
		if errors.Is(err, pebble.ErrNotFound) {
			return stateproof.Node{}, false, fmt.Errorf("%w: namespace %q misses the node of version %d at depth %d", ErrStateTreeCorrupt, nodes.namespace, version, key.Depth)
		}
		if err != nil {
			return stateproof.Node{}, false, err
		}
		stored, err = decodeStoredNode(value)
		closer.Close()
		if err != nil {
			return stateproof.Node{}, false, err
		}
		if !nodes.historical {
			nodes.cache.add(dbKey, stored)
		}
	}
	nodes.versions.learn(key, stored)
	return stored.node, true, nil
}

// stateTrees applies writes to the trees of the namespaces they touch.
type stateTrees struct {
	reader dbReader
	cache  treeCacheView
	trees  map[string]*stateproof.Tree
	// versions are the versions of the stored nodes each tree read.
	versions map[string]*nodeVersions
	// computed holds the computations of namespaces whose writes an earlier root computation
	// already applied to the same trees.
	computed map[string]computedTree
	// writes counts the writes of each namespace the trees apply, when known, to size the trees.
	writes map[string]int
}

func newStateTrees(reader dbReader, cache treeCacheView) *stateTrees {
	return &stateTrees{
		reader:   reader,
		cache:    cache,
		trees:    make(map[string]*stateproof.Tree),
		versions: make(map[string]*nodeVersions),
		computed: make(map[string]computedTree),
		writes:   make(map[string]int),
	}
}

// applyNamespaces applies writes to the trees, reusing for each namespace the computation a root
// computation over the same trees and the same writes of that namespace kept.
func (trees *stateTrees) applyNamespaces(computed *computedTrees, writes []KVWrite) error {
	for _, namespace := range writeNamespaces(writes) {
		namespaceWrites, digest := writesOf(namespace, writes)
		if found, ok := computed.get(namespace, trees.cache, digest); ok {
			trees.computed[namespace] = found
			continue
		}
		trees.writes[namespace] = len(namespaceWrites)
		for _, write := range namespaceWrites {
			if err := trees.apply(write); err != nil {
				return err
			}
		}
	}
	return nil
}

// changes returns the node changes of namespace and the versions of the stored nodes they replace
// or keep.
func (trees *stateTrees) changes(namespace string) ([]stateproof.NodeChange, *nodeVersions, error) {
	if found, ok := trees.computed[namespace]; ok {
		return found.changes, found.versions, nil
	}
	changes, err := trees.trees[namespace].Changes()
	return changes, trees.versions[namespace], err
}

func (trees *stateTrees) tree(namespace string) (*stateproof.Tree, error) {
	if tree, found := trees.trees[namespace]; found {
		return tree, nil
	}
	nodes, err := newTreeNodes(trees.reader, namespace, trees.cache, math.MaxUint64, trees.writes[namespace])
	if err != nil {
		return nil, err
	}
	tree := stateproof.NewTree(namespace, nodes)
	trees.trees[namespace] = tree
	trees.versions[namespace] = nodes.versions
	return tree, nil
}

func (trees *stateTrees) apply(write KVWrite) error {
	tree, err := trees.tree(write.Namespace)
	if err != nil {
		return err
	}
	if write.Delete {
		return tree.Delete(write.Key)
	}
	return tree.Set(write.Key, write.Value)
}

// persist adds the node changes of every tree to batch as nodes of version height, the root
// records of the trees whose roots changed, and the list of the nodes they orphaned.
func (trees *stateTrees) persist(batch *pebble.Batch, height types.Height) error {
	namespaces := make([]string, 0, len(trees.trees)+len(trees.computed))
	for namespace := range trees.trees {
		namespaces = append(namespaces, namespace)
	}
	for namespace := range trees.computed {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	orphans := newKeyList(stateTreeOrphanPrefix, height)
	for _, namespace := range namespaces {
		changes, versions, err := trees.changes(namespace)
		if err != nil {
			return err
		}
		changed := make(map[stateproof.NodeKey]bool, len(changes))
		for _, change := range changes {
			changed[change.Key] = change.Node != nil
		}
		for _, change := range changes {
			old, existed := versions.of(change.Key)
			if existed && old != height {
				orphans.add(stateTreeNodeKey(namespace, old, change.Key)[len(stateTreePrefix):])
			}
			key := stateTreeNodeKey(namespace, height, change.Key)
			if change.Node == nil {
				if existed && old == height {
					batch.Delete(key, nil)
				}
			} else {
				left, right, err := halfVersions(namespace, changed, versions, change, height)
				if err != nil {
					return err
				}
				batch.Set(key, encodeStoredNode(*change.Node, left, right), nil)
			}
			if change.Key == (stateproof.NodeKey{}) {
				present := byte(0)
				if change.Node != nil {
					present = 1
				}
				batch.Set(stateTreeRootKey(namespace, height), []byte{present}, nil)
			}
		}
	}
	orphans.finish(batch)
	return nil
}

// halfVersions returns the versions of the halves of the internal node change sets: height for a
// half the changes rewrote, the stored node's version for one they kept. changed holds the keys the
// changes write (true) or remove (false).
func halfVersions(namespace string, changed map[stateproof.NodeKey]bool, versions *nodeVersions, change stateproof.NodeChange, height types.Height) (types.Height, types.Height, error) {
	node := *change.Node
	if node.Leaf != nil {
		return 0, 0, nil
	}
	versionOf := func(half types.Hash, bit byte) (types.Height, error) {
		if half == (types.Hash{}) {
			return 0, nil
		}
		child := change.Key.Child(bit)
		if written, found := changed[child]; found {
			if !written {
				return 0, fmt.Errorf("%w: namespace %q keeps a half whose node it removes", ErrStateTreeCorrupt, namespace)
			}
			return height, nil
		}
		if version, found := versions.of(child); found {
			return version, nil
		}
		return 0, fmt.Errorf("%w: namespace %q keeps a half whose node it never read", ErrStateTreeCorrupt, namespace)
	}
	left, err := versionOf(node.Left, 0)
	if err != nil {
		return 0, 0, err
	}
	right, err := versionOf(node.Right, 1)
	return left, right, err
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
	nodes, err := newTreeNodes(snapshot, namespace, cache, math.MaxUint64, len(namespaceWrites))
	if err != nil {
		return types.Hash{}, err
	}
	tree := stateproof.NewTree(namespace, nodes)
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
	changes, err := tree.Changes()
	if err != nil {
		return types.Hash{}, err
	}
	store.computed.put(namespace, cache, computedTree{digest: digest, root: root, changes: changes, versions: nodes.versions})
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
	// versions are the versions of the stored nodes the computation read, which the commit of its
	// changes needs.
	versions *nodeVersions
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
	nodes, err := newTreeNodes(snapshot, namespace, cache, math.MaxUint64, 0)
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	tree := stateproof.NewTree(namespace, nodes)
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
// the namespace's root at that height, from the tree as it was then: it reads the nodes on the
// key's path from the root record of the height down, not the namespace.
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
	nodes, err := newTreeNodes(snapshot, namespace, cache, height, 0)
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	tree := stateproof.NewTree(namespace, nodes)
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

// replaceStateTree adds to batch the removal of namespace's tree, its root records and its orphaned
// nodes, and the nodes of the tree of pairs as the namespace's base state: nodes of version zero and
// a root record at height zero, which every height reads.
func (store *PebbleStore) replaceStateTree(ctx context.Context, batch *pebble.Batch, namespace string, pairs []KVPair) error {
	if err := store.dropStateTree(ctx, batch, namespace); err != nil {
		return err
	}
	nodes, err := stateproof.Build(namespace, stateProofPairs(pairs))
	if err != nil {
		return err
	}
	for _, change := range nodes {
		var left, right types.Height
		batch.Set(stateTreeNodeKey(namespace, 0, change.Key), encodeStoredNode(*change.Node, left, right), nil)
	}
	if len(nodes) > 0 {
		batch.Set(stateTreeRootKey(namespace, 0), []byte{1}, nil)
	}
	return nil
}

// dropStateTree adds to batch the removal of namespace's tree: its nodes of every version, its root
// records, and its entries in the orphan lists, which would otherwise name the nodes a new tree of
// the namespace writes at the same versions.
func (store *PebbleStore) dropStateTree(ctx context.Context, batch *pebble.Batch, namespace string) error {
	for _, prefix := range [][]byte{stateTreeNamespacePrefix(namespace), stateTreeNamespaceID(stateTreeRootPrefix, namespace)} {
		if err := deletePrefix(ctx, store.db, batch, prefix); err != nil {
			return err
		}
	}
	own := stateTreeNamespaceID(nil, namespace)
	lists := newIter(store.db, prefixRange(stateTreeOrphanPrefix))
	defer lists.Close()
	for valid := lists.First(); valid; valid = lists.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		key := lists.Key()
		if len(key) < len(stateTreeOrphanPrefix)+8 {
			return ErrInvalidStateRecord
		}
		var kept *keyList
		dropped := false
		if err := eachListedKey(lists.Value(), func(entry []byte) {
			if bytes.HasPrefix(entry, own) {
				dropped = true
				return
			}
			if kept == nil {
				kept = newKeyList(stateTreeOrphanPrefix, types.Height(binary.BigEndian.Uint64(key[len(stateTreeOrphanPrefix):])))
			}
			kept.add(bytes.Clone(entry))
		}); err != nil {
			return err
		}
		if !dropped {
			continue
		}
		// The list is keyed by its first entry, which the remaining entries may no longer start with.
		batch.Delete(append([]byte(nil), key...), nil)
		if kept != nil {
			kept.finish(batch)
		}
	}
	return lists.Error()
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
	if err := deletePrefix(ctx, store.db, batch, kvNamespacePrefix(namespace)); err != nil {
		return nil, err
	}
	if err := store.dropStateTree(ctx, batch, namespace); err != nil {
		return nil, err
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
	// The pairs are the base state: their nodes are of version zero, rewritten in place page by page.
	if err := trees.persist(batch, 0); err != nil {
		return err
	}
	return in.store.writeBatch(batch)
}

// Root returns the root of the pairs imported so far.
func (in *NamespaceImport) Root(ctx context.Context) (types.Hash, error) {
	return in.store.Root(ctx, in.namespace)
}

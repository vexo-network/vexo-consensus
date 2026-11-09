package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
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
// Tree nodes are versioned like keys (see State history): every change of a
// node after the first committed state is recorded at the height of the state
// it belongs to, the first change also records the node as it was before at
// height 0, and a deleted node is a version with an empty value. The tree of
// any retained height is therefore read node by node, so a historical proof
// costs the nodes on one path, not the namespace.

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

// dbReader reads keys from the database or from one of its snapshots.
type dbReader interface {
	Get(key []byte, ro *opt.ReadOptions) ([]byte, error)
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
}

// treeNodes serves the stored nodes of one namespace tree.
type treeNodes struct {
	reader    dbReader
	namespace string
}

func (nodes treeNodes) Node(key stateproof.NodeKey) (stateproof.Node, bool, error) {
	encoded, err := nodes.reader.Get(stateTreeNodeKey(nodes.namespace, key), nil)
	if errors.Is(err, leveldberrors.ErrNotFound) {
		return stateproof.Node{}, false, nil
	}
	if err != nil {
		return stateproof.Node{}, false, err
	}
	return decodeStoredNode(encoded)
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
}

func (nodes treeNodesAt) Node(key stateproof.NodeKey) (stateproof.Node, bool, error) {
	group := stateTreeHistoryGroup(nodes.namespace, key)
	versions := nodes.reader.NewIterator(util.BytesPrefix(group), nil)
	defer versions.Release()
	if !versions.First() {
		if err := versions.Error(); err != nil {
			return stateproof.Node{}, false, err
		}
		return treeNodes{reader: nodes.reader, namespace: nodes.namespace}.Node(key)
	}
	if !versionAt(versions, group, nodes.height) {
		return stateproof.Node{}, false, versions.Error()
	}
	return decodeStoredNode(versions.Value())
}

// stateTrees applies writes to the trees of the namespaces they touch.
type stateTrees struct {
	reader dbReader
	trees  map[string]*stateproof.Tree
}

func newStateTrees(reader dbReader) *stateTrees {
	return &stateTrees{reader: reader, trees: make(map[string]*stateproof.Tree)}
}

func (trees *stateTrees) tree(namespace string) *stateproof.Tree {
	tree, found := trees.trees[namespace]
	if !found {
		tree = stateproof.NewTree(namespace, treeNodes{reader: trees.reader, namespace: namespace})
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

// persist adds the node changes of every tree to batch. With versioned set, each change is also
// recorded as a version of the node at height, after the node's prior form at height 0 when this is
// its first change.
func (trees *stateTrees) persist(batch *leveldb.Batch, versioned bool, height types.Height) error {
	namespaces := make([]string, 0, len(trees.trees))
	for namespace := range trees.trees {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		for _, change := range trees.trees[namespace].Changes() {
			key := stateTreeNodeKey(namespace, change.Key)
			var encoded []byte
			if change.Node != nil {
				encoded = stateproof.EncodeNode(*change.Node)
			}
			if versioned {
				if err := trees.recordVersion(batch, namespace, change.Key, height, encoded); err != nil {
					return err
				}
			}
			if change.Node == nil {
				batch.Delete(key)
				continue
			}
			batch.Put(key, encoded)
		}
	}
	return nil
}

func (trees *stateTrees) recordVersion(batch *leveldb.Batch, namespace string, key stateproof.NodeKey, height types.Height, encoded []byte) error {
	group := stateTreeHistoryGroup(namespace, key)
	versions := trees.reader.NewIterator(util.BytesPrefix(group), nil)
	hasVersions := versions.First()
	err := versions.Error()
	versions.Release()
	if err != nil {
		return err
	}
	if !hasVersions {
		prior, err := trees.reader.Get(stateTreeNodeKey(namespace, key), nil)
		switch {
		case errors.Is(err, leveldberrors.ErrNotFound):
			prior = nil
		case err != nil:
			return err
		}
		putVersion(batch, stateTreeVersionIndexPrefix, stateTreeHistoryKey(group, 0), 0, append([]byte{}, prior...))
	}
	putVersion(batch, stateTreeVersionIndexPrefix, stateTreeHistoryKey(group, height), height, append([]byte{}, encoded...))
	return nil
}

// Root returns the current root of namespace.
func (store *LevelDBStore) Root(ctx context.Context, namespace string) (types.Hash, error) {
	return store.RootWithWrites(ctx, namespace, nil)
}

// RootWithWrites returns the root namespace would have after writes, without applying them.
func (store *LevelDBStore) RootWithWrites(ctx context.Context, namespace string, writes []KVWrite) (types.Hash, error) {
	select {
	case <-ctx.Done():
		return types.Hash{}, ctx.Err()
	default:
	}
	if namespace == "" {
		return types.Hash{}, ErrInvalidNamespace
	}
	snapshot, err := store.db.GetSnapshot()
	if err != nil {
		return types.Hash{}, err
	}
	defer snapshot.Release()
	trees := newStateTrees(snapshot)
	for _, write := range writes {
		if write.Namespace != namespace {
			continue
		}
		if len(write.Key) == 0 {
			return types.Hash{}, ErrInvalidKey
		}
		if err := trees.apply(write); err != nil {
			return types.Hash{}, err
		}
	}
	return trees.tree(namespace).Root()
}

// StateProof proves the current value of key in namespace, or its absence, against the
// namespace's current root, read from one consistent view of the store.
func (store *LevelDBStore) StateProof(ctx context.Context, namespace string, key []byte) (root types.Hash, value []byte, exists bool, proof stateproof.Proof, err error) {
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
	snapshot, err := store.db.GetSnapshot()
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	defer snapshot.Release()
	tree := stateproof.NewTree(namespace, treeNodes{reader: snapshot, namespace: namespace})
	root, err = tree.Root()
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	proof, leaf, err := tree.Prove(key)
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	stored, err := snapshot.Get(kvKey(namespace, key), nil)
	switch {
	case errors.Is(err, leveldberrors.ErrNotFound):
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
func (store *LevelDBStore) StateProofAt(ctx context.Context, height types.Height, namespace string, key []byte) (root types.Hash, value []byte, exists bool, proof stateproof.Proof, err error) {
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
	snapshot, err := store.db.GetSnapshot()
	if err != nil {
		return types.Hash{}, nil, false, stateproof.Proof{}, err
	}
	defer snapshot.Release()
	tree := stateproof.NewTree(namespace, treeNodesAt{reader: snapshot, namespace: namespace, height: height})
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
func (store *LevelDBStore) replaceStateTree(ctx context.Context, batch *leveldb.Batch, namespace string, pairs []KVPair) error {
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
		batch.Put(stateTreeNodeKey(namespace, change.Key), stateproof.EncodeNode(*change.Node))
	}
	return nil
}

// deletePrefix adds to batch the deletion of every key under prefix.
func deletePrefix(ctx context.Context, reader dbReader, batch *leveldb.Batch, prefix []byte) error {
	iterator := reader.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()
	for iterator.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...))
	}
	return iterator.Error()
}

// NamespaceImport writes a namespace page by page, so a namespace of any size is imported without
// being held in memory: the pairs of each page, in ascending key order after those of the pages
// before, are written with the nodes they change in the namespace's tree in one batch.
type NamespaceImport struct {
	store     *LevelDBStore
	namespace string
	last      []byte
}

// ErrImportOrder reports pairs that do not follow the pairs already imported in key order.
var ErrImportOrder = errors.New("imported pairs are not in ascending key order")

// BeginNamespaceImport empties namespace, its versions and its tree, and returns the import that
// fills it. The pairs imported become the namespace's base state.
func (store *LevelDBStore) BeginNamespaceImport(ctx context.Context, namespace string) (*NamespaceImport, error) {
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	store.writeMu.Lock()
	defer store.writeMu.Unlock()
	batch := new(leveldb.Batch)
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
	trees := newStateTrees(in.store.db)
	batch := new(leveldb.Batch)
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
		batch.Put(kvKey(in.namespace, pair.Key), append([]byte(nil), pair.Value...))
	}
	if err := trees.persist(batch, false, 0); err != nil {
		return err
	}
	return in.store.writeBatch(batch)
}

// Root returns the root of the pairs imported so far.
func (in *NamespaceImport) Root(ctx context.Context) (types.Hash, error) {
	return in.store.Root(ctx, in.namespace)
}

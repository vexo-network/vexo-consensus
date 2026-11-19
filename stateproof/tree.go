package stateproof

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"slices"

	"github.com/vexo-network/vexo-consensus/types"
)

// ErrInvalidNode reports a stored node that does not decode.
var ErrInvalidNode = errors.New("invalid state tree node")

// NodeKey locates a tree node: its depth and the first Depth bits of the paths below it; the bits
// past Depth are zero.
type NodeKey struct {
	Depth  int
	Prefix types.Hash
}

// NodeKeyOf returns the key of the node at depth on path.
func NodeKeyOf(path types.Hash, depth int) NodeKey {
	key := NodeKey{Depth: depth}
	full := depth / 8
	copy(key.Prefix[:full], path[:full])
	if rest := depth % 8; rest != 0 {
		key.Prefix[full] = path[full] & byte(0xff<<(8-uint(rest)))
	}
	return key
}

// Bytes encodes key, ordered by depth and then prefix.
func (key NodeKey) Bytes() []byte {
	return key.AppendBytes(make([]byte, 0, key.EncodedLen()))
}

// EncodedLen is the length of the encoding of key.
func (key NodeKey) EncodedLen() int {
	return 2 + (key.Depth+7)/8
}

// AppendBytes appends the encoding of key (see Bytes) to dst.
func (key NodeKey) AppendBytes(dst []byte) []byte {
	dst = binary.BigEndian.AppendUint16(dst, uint16(key.Depth))
	return append(dst, key.Prefix[:(key.Depth+7)/8]...)
}

// Node is a stored tree node: a leaf, or an internal node with the hashes of its halves.
type Node struct {
	Leaf  *Leaf
	Left  types.Hash
	Right types.Hash
}

// Hash returns the hash of node.
func (node Node) Hash() types.Hash {
	if node.Leaf != nil {
		return LeafNodeHash(*node.Leaf)
	}
	return InternalNodeHash(node.Left, node.Right)
}

const (
	nodeKindLeaf     byte = 1
	nodeKindInternal byte = 2
)

// EncodeNode encodes node for storage.
func EncodeNode(node Node) []byte {
	encoded := make([]byte, 0, 1+2*len(types.Hash{}))
	if node.Leaf != nil {
		encoded = append(encoded, nodeKindLeaf)
		encoded = append(encoded, node.Leaf.Path[:]...)
		return append(encoded, node.Leaf.ValueHash[:]...)
	}
	encoded = append(encoded, nodeKindInternal)
	encoded = append(encoded, node.Left[:]...)
	return append(encoded, node.Right[:]...)
}

// DecodeNode decodes a node EncodeNode encoded.
func DecodeNode(encoded []byte) (Node, error) {
	size := len(types.Hash{})
	if len(encoded) != 1+2*size {
		return Node{}, ErrInvalidNode
	}
	var first, second types.Hash
	copy(first[:], encoded[1:1+size])
	copy(second[:], encoded[1+size:])
	switch encoded[0] {
	case nodeKindLeaf:
		return Node{Leaf: &Leaf{Path: first, ValueHash: second}}, nil
	case nodeKindInternal:
		return Node{Left: first, Right: second}, nil
	default:
		return Node{}, ErrInvalidNode
	}
}

// NodeReader reads the stored nodes of one namespace tree.
type NodeReader interface {
	Node(key NodeKey) (Node, bool, error)
}

// NodeChange is a node a Tree wrote (Node set) or removed (Node nil).
type NodeChange struct {
	Key  NodeKey
	Node *Node
}

// Tree is the tree of a namespace as stored, with the writes applied so far kept in memory. A
// write reads and rewrites only the nodes on its path; Changes returns them for the store to persist.
//
// A write does not hash the nodes on its path: it marks the halves it changed as unhashed, and the
// hashes are computed once, bottom-up, when the root, a proof or the changes are asked for. The
// nodes near the root, which every write rewrites, are then hashed once per batch of writes instead
// of once per write.
type Tree struct {
	namespace string
	reader    NodeReader
	changes   map[NodeKey]nodeChange
	// unhashed reports that an internal node in changes holds a half that is not hashed yet.
	unhashed bool
}

// nodeChange is the node a Tree holds at a key in place of the stored one: node, or none when
// removed. stale marks an internal node whose halves hashNodes recomputes.
type nodeChange struct {
	node    Node
	removed bool
	stale   bool
}

// unhashedHalf stands, in an internal node a write changed, for a non-empty half whose hash is not
// computed yet. Only its being non-zero matters until hashNodes replaces it: a half's emptiness is
// all a write reads of it.
var unhashedHalf = types.Hash{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// NewTree returns the tree of namespace whose stored nodes reader serves.
func NewTree(namespace string, reader NodeReader) *Tree {
	return &Tree{namespace: namespace, reader: reader, changes: make(map[NodeKey]nodeChange)}
}

// Set makes key hold value.
func (tree *Tree) Set(key []byte, value []byte) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}
	leaf := Leaf{Path: KeyPath(key), ValueHash: LeafHash(tree.namespace, key, value)}
	_, _, err := tree.update(0, leaf.Path, &leaf)
	return err
}

// Delete removes key.
func (tree *Tree) Delete(key []byte) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}
	_, _, err := tree.update(0, KeyPath(key), nil)
	return err
}

// Root returns the root of the tree.
func (tree *Tree) Root() (types.Hash, error) {
	if err := tree.hashNodes(); err != nil {
		return types.Hash{}, err
	}
	node, found, err := tree.node(NodeKey{})
	if err != nil {
		return types.Hash{}, err
	}
	if !found {
		return EmptyRoot(tree.namespace), nil
	}
	return node.Hash(), nil
}

// Prove returns the proof of the node at key's position and, when that node is a leaf, the leaf:
// key holds a value whose LeafHash is its ValueHash when its Path is key's path, and no value
// otherwise. The proof's Neighbor is set for the leaf of another key.
func (tree *Tree) Prove(key []byte) (Proof, *Leaf, error) {
	if len(key) == 0 {
		return Proof{}, nil, ErrInvalidKey
	}
	if err := tree.hashNodes(); err != nil {
		return Proof{}, nil, err
	}
	path := KeyPath(key)
	var proof Proof
	for depth := 0; depth <= MaxDepth; depth++ {
		node, found, err := tree.node(NodeKeyOf(path, depth))
		if err != nil {
			return Proof{}, nil, err
		}
		if !found {
			return proof, nil, nil
		}
		if node.Leaf != nil {
			leaf := *node.Leaf
			if leaf.Path != path {
				proof.Neighbor = &leaf
			}
			return proof, &leaf, nil
		}
		if Bit(path, depth) == 0 {
			proof.Siblings = append(proof.Siblings, node.Right)
		} else {
			proof.Siblings = append(proof.Siblings, node.Left)
		}
	}
	return Proof{}, nil, ErrInvalidNode
}

// Changes returns the nodes written and removed since the tree was opened, ordered by key.
func (tree *Tree) Changes() ([]NodeChange, error) {
	if err := tree.hashNodes(); err != nil {
		return nil, err
	}
	nodes := make([]Node, 0, len(tree.changes))
	changes := make([]NodeChange, 0, len(tree.changes))
	for key, change := range tree.changes {
		if change.removed {
			changes = append(changes, NodeChange{Key: key})
			continue
		}
		nodes = append(nodes, change.node)
		changes = append(changes, NodeChange{Key: key, Node: &nodes[len(nodes)-1]})
	}
	slices.SortFunc(changes, func(left, right NodeChange) int {
		return left.Key.compare(right.Key)
	})
	return changes, nil
}

// compare orders keys as their Bytes do: by depth, then by prefix, whose bits past the depth are
// zero.
func (key NodeKey) compare(other NodeKey) int {
	if key.Depth != other.Depth {
		return cmp.Compare(key.Depth, other.Depth)
	}
	return bytes.Compare(key.Prefix[:], other.Prefix[:])
}

// hashNodes computes the halves of the internal nodes the writes changed, deepest first, so the
// hash of every child is final before its parent's is taken. A half whose child the writes changed
// is the hash of that child; a half marked unhashed whose child they did not change, as when a
// delete of an absent key leaves its subtree as stored, is the hash of the stored child; any other
// half is as stored.
func (tree *Tree) hashNodes() error {
	if !tree.unhashed {
		return nil
	}
	var byDepth [MaxDepth + 1][]NodeKey
	for key, change := range tree.changes {
		if change.stale {
			byDepth[key.Depth] = append(byDepth[key.Depth], key)
		}
	}
	for depth := MaxDepth; depth >= 0; depth-- {
		for _, key := range byDepth[depth] {
			change := tree.changes[key]
			var err error
			if change.node.Left, err = tree.halfHash(childKey(key, 0), change.node.Left); err != nil {
				return err
			}
			if change.node.Right, err = tree.halfHash(childKey(key, 1), change.node.Right); err != nil {
				return err
			}
			change.stale = false
			tree.changes[key] = change
		}
	}
	tree.unhashed = false
	return nil
}

// halfHash returns the hash of the half of an internal node whose child is at key and which holds
// half; see hashNodes.
func (tree *Tree) halfHash(key NodeKey, half types.Hash) (types.Hash, error) {
	if change, changed := tree.changes[key]; changed {
		if change.removed {
			return types.Hash{}, nil
		}
		return change.node.Hash(), nil
	}
	if half != unhashedHalf {
		return half, nil
	}
	node, found, err := tree.reader.Node(key)
	if err != nil {
		return types.Hash{}, err
	}
	if !found {
		return types.Hash{}, ErrInvalidNode
	}
	return node.Hash(), nil
}

// childKey returns the key of the child of the internal node at key on the side of bit.
func childKey(key NodeKey, bit byte) NodeKey {
	child := NodeKey{Depth: key.Depth + 1, Prefix: key.Prefix}
	if bit == 1 {
		child.Prefix[key.Depth/8] |= 1 << (7 - uint(key.Depth%8))
	}
	return child
}

func (tree *Tree) node(key NodeKey) (Node, bool, error) {
	if change, changed := tree.changes[key]; changed {
		if change.removed {
			return Node{}, false, nil
		}
		return change.node, true, nil
	}
	return tree.reader.Node(key)
}

// put holds node at key; an internal node's halves are hashed by hashNodes.
func (tree *Tree) put(key NodeKey, node Node) {
	stale := node.Leaf == nil
	tree.changes[key] = nodeChange{node: node, stale: stale}
	tree.unhashed = tree.unhashed || stale
}

func (tree *Tree) remove(key NodeKey) {
	tree.changes[key] = nodeChange{removed: true}
}

// update makes the subtree at depth on path hold leaf for path (no value for a nil leaf) and
// returns the subtree's new top node and whether it holds one, false when it is empty.
func (tree *Tree) update(depth int, path types.Hash, leaf *Leaf) (Node, bool, error) {
	key := NodeKeyOf(path, depth)
	current, found, err := tree.node(key)
	if err != nil {
		return Node{}, false, err
	}
	switch {
	case !found:
		if leaf == nil {
			return Node{}, false, nil
		}
		node := Node{Leaf: leaf}
		tree.put(key, node)
		return node, true, nil
	case current.Leaf != nil:
		if current.Leaf.Path == path {
			if leaf == nil {
				tree.remove(key)
				return Node{}, false, nil
			}
			node := Node{Leaf: leaf}
			tree.put(key, node)
			return node, true, nil
		}
		if leaf == nil {
			return current, true, nil
		}
		return tree.split(depth, *current.Leaf, *leaf)
	}
	if depth >= MaxDepth {
		return Node{}, false, ErrInvalidNode
	}
	_, nonEmpty, err := tree.update(depth+1, path, leaf)
	if err != nil {
		return Node{}, false, err
	}
	childHash := types.Hash{}
	if nonEmpty {
		childHash = unhashedHalf
	}
	left, right := current.Left, current.Right
	sibling := path
	if Bit(path, depth) == 0 {
		left = childHash
		sibling[depth/8] |= 1 << (7 - uint(depth%8))
	} else {
		right = childHash
		sibling[depth/8] &^= 1 << (7 - uint(depth%8))
	}
	switch {
	case left == (types.Hash{}) && right == (types.Hash{}):
		tree.remove(key)
		return Node{}, false, nil
	case left == (types.Hash{}) || right == (types.Hash{}):
		// A subtree left with one half holds a single leaf only when that half is a leaf; it then
		// moves up to stand for the whole subtree.
		remainingKey := NodeKeyOf(path, depth+1)
		if childHash == (types.Hash{}) {
			remainingKey = NodeKeyOf(sibling, depth+1)
		}
		remaining, found, err := tree.node(remainingKey)
		if err != nil {
			return Node{}, false, err
		}
		if !found {
			return Node{}, false, ErrInvalidNode
		}
		if remaining.Leaf != nil {
			tree.remove(remainingKey)
			tree.put(key, remaining)
			return remaining, true, nil
		}
	}
	node := Node{Left: left, Right: right}
	tree.put(key, node)
	return node, true, nil
}

// split replaces the leaf existing at depth by the subtree holding it and leaf: internal nodes down
// to the first bit their paths differ at, and the two leaves below it.
func (tree *Tree) split(depth int, existing Leaf, leaf Leaf) (Node, bool, error) {
	differ := depth
	for differ < MaxDepth && Bit(existing.Path, differ) == Bit(leaf.Path, differ) {
		differ++
	}
	if differ == MaxDepth {
		return Node{}, false, ErrPathCollision
	}
	tree.put(NodeKeyOf(existing.Path, differ+1), Node{Leaf: &existing})
	tree.put(NodeKeyOf(leaf.Path, differ+1), Node{Leaf: &leaf})
	node := Node{Left: unhashedHalf, Right: unhashedHalf}
	tree.put(NodeKeyOf(leaf.Path, differ), node)
	for level := differ - 1; level >= depth; level-- {
		if Bit(leaf.Path, level) == 0 {
			node = Node{Left: unhashedHalf}
		} else {
			node = Node{Right: unhashedHalf}
		}
		tree.put(NodeKeyOf(leaf.Path, level), node)
	}
	return node, true, nil
}

// Build returns the nodes of the tree of namespace over pairs, for a store to write in place of a
// namespace it imports.
func Build(namespace string, pairs []Pair) ([]NodeChange, error) {
	leaves, err := sortedLeaves(namespace, pairs)
	if err != nil {
		return nil, err
	}
	changes := make([]NodeChange, 0, 2*len(leaves))
	var build func(leaves []sortedLeaf, depth int, prefix types.Hash) types.Hash
	build = func(leaves []sortedLeaf, depth int, prefix types.Hash) types.Hash {
		switch len(leaves) {
		case 0:
			return types.Hash{}
		case 1:
			node := Node{Leaf: &leaves[0].leaf}
			changes = append(changes, NodeChange{Key: NodeKeyOf(prefix, depth), Node: &node})
			return node.Hash()
		}
		left, right := splitLeaves(leaves, depth)
		rightPrefix := prefix
		rightPrefix[depth/8] |= 1 << (7 - uint(depth%8))
		node := Node{Left: build(left, depth+1, prefix), Right: build(right, depth+1, rightPrefix)}
		changes = append(changes, NodeChange{Key: NodeKeyOf(prefix, depth), Node: &node})
		return node.Hash()
	}
	build(leaves, 0, types.Hash{})
	return changes, nil
}

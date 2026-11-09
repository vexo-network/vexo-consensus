package stateproof

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"

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
	encoded := make([]byte, 2, 2+(key.Depth+7)/8)
	binary.BigEndian.PutUint16(encoded, uint16(key.Depth))
	return append(encoded, key.Prefix[:(key.Depth+7)/8]...)
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
type Tree struct {
	namespace string
	reader    NodeReader
	changes   map[NodeKey]*Node
}

// NewTree returns the tree of namespace whose stored nodes reader serves.
func NewTree(namespace string, reader NodeReader) *Tree {
	return &Tree{namespace: namespace, reader: reader, changes: make(map[NodeKey]*Node)}
}

// Set makes key hold value.
func (tree *Tree) Set(key []byte, value []byte) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}
	leaf := Leaf{Path: KeyPath(key), ValueHash: LeafHash(tree.namespace, key, value)}
	_, err := tree.update(0, leaf.Path, &leaf)
	return err
}

// Delete removes key.
func (tree *Tree) Delete(key []byte) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}
	_, err := tree.update(0, KeyPath(key), nil)
	return err
}

// Root returns the root of the tree.
func (tree *Tree) Root() (types.Hash, error) {
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
func (tree *Tree) Changes() []NodeChange {
	changes := make([]NodeChange, 0, len(tree.changes))
	for key, node := range tree.changes {
		changes = append(changes, NodeChange{Key: key, Node: node})
	}
	sort.Slice(changes, func(left, right int) bool {
		return bytes.Compare(changes[left].Key.Bytes(), changes[right].Key.Bytes()) < 0
	})
	return changes
}

func (tree *Tree) node(key NodeKey) (Node, bool, error) {
	if node, changed := tree.changes[key]; changed {
		if node == nil {
			return Node{}, false, nil
		}
		return *node, true, nil
	}
	return tree.reader.Node(key)
}

func (tree *Tree) put(key NodeKey, node Node) {
	tree.changes[key] = &node
}

func (tree *Tree) remove(key NodeKey) {
	tree.changes[key] = nil
}

// update makes the subtree at depth on path hold leaf for path (no value for a nil leaf) and
// returns the subtree's new top node, nil when it is empty.
func (tree *Tree) update(depth int, path types.Hash, leaf *Leaf) (*Node, error) {
	key := NodeKeyOf(path, depth)
	current, found, err := tree.node(key)
	if err != nil {
		return nil, err
	}
	switch {
	case !found:
		if leaf == nil {
			return nil, nil
		}
		node := Node{Leaf: leaf}
		tree.put(key, node)
		return &node, nil
	case current.Leaf != nil:
		if current.Leaf.Path == path {
			if leaf == nil {
				tree.remove(key)
				return nil, nil
			}
			node := Node{Leaf: leaf}
			tree.put(key, node)
			return &node, nil
		}
		if leaf == nil {
			return &current, nil
		}
		return tree.split(depth, *current.Leaf, *leaf)
	}
	if depth >= MaxDepth {
		return nil, ErrInvalidNode
	}
	child, err := tree.update(depth+1, path, leaf)
	if err != nil {
		return nil, err
	}
	childHash := types.Hash{}
	if child != nil {
		childHash = child.Hash()
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
		return nil, nil
	case left == (types.Hash{}) || right == (types.Hash{}):
		// A subtree left with one half holds a single leaf only when that half is a leaf; it then
		// moves up to stand for the whole subtree.
		remainingKey := NodeKeyOf(path, depth+1)
		if childHash == (types.Hash{}) {
			remainingKey = NodeKeyOf(sibling, depth+1)
		}
		remaining, found, err := tree.node(remainingKey)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ErrInvalidNode
		}
		if remaining.Leaf != nil {
			tree.remove(remainingKey)
			tree.put(key, remaining)
			return &remaining, nil
		}
	}
	node := Node{Left: left, Right: right}
	tree.put(key, node)
	return &node, nil
}

// split replaces the leaf existing at depth by the subtree holding it and leaf: internal nodes down
// to the first bit their paths differ at, and the two leaves below it.
func (tree *Tree) split(depth int, existing Leaf, leaf Leaf) (*Node, error) {
	differ := depth
	for differ < MaxDepth && Bit(existing.Path, differ) == Bit(leaf.Path, differ) {
		differ++
	}
	if differ == MaxDepth {
		return nil, ErrPathCollision
	}
	existingNode, leafNode := Node{Leaf: &existing}, Node{Leaf: &leaf}
	tree.put(NodeKeyOf(existing.Path, differ+1), existingNode)
	tree.put(NodeKeyOf(leaf.Path, differ+1), leafNode)
	var node Node
	if Bit(leaf.Path, differ) == 0 {
		node = Node{Left: leafNode.Hash(), Right: existingNode.Hash()}
	} else {
		node = Node{Left: existingNode.Hash(), Right: leafNode.Hash()}
	}
	tree.put(NodeKeyOf(leaf.Path, differ), node)
	for level := differ - 1; level >= depth; level-- {
		if Bit(leaf.Path, level) == 0 {
			node = Node{Left: node.Hash()}
		} else {
			node = Node{Right: node.Hash()}
		}
		tree.put(NodeKeyOf(leaf.Path, level), node)
	}
	return &node, nil
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

// Package stateproof commits a namespace of key-value state to one root and proves the value of a
// key, or its absence, against that root.
//
// The commitment is a binary Merkle radix tree over the SHA-256 of each key (its path). A subtree
// that holds no key is empty and hashes to the zero hash; a subtree that holds exactly one key is
// that key's leaf node, placed as high as it is alone; any other subtree is the internal node of its
// two halves, split by the next path bit. The shape depends only on the set of keys, so every
// replica derives the same root, and a write changes only the nodes on its path: a store keeps the
// tree and updates it in O(depth) per write instead of rehashing the namespace (see Tree).
//
// A proof is the list of sibling hashes from the root down to the node at the key's position. A key
// is present when that node is its leaf; it is absent when the node is empty or the leaf of another
// key whose path shares the prefix, since a present key would occupy that very subtree.
package stateproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInvalidNamespace = errors.New("namespace is required")
	ErrInvalidKey       = errors.New("key is required")
	ErrDuplicateKey     = errors.New("duplicate state proof key")
	ErrPathCollision    = errors.New("two state keys share one path")
)

// MaxDepth is the depth of the deepest node: one per path bit.
const MaxDepth = 8 * len(types.Hash{})

// Pair is one key-value pair of a namespace.
type Pair struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value,omitempty"`
}

// Leaf identifies the leaf node of a key without the key itself: its path and its value hash.
type Leaf struct {
	Path      types.Hash `json:"path"`
	ValueHash types.Hash `json:"value_hash"`
}

// Proof locates the node at a key's position: Siblings[i] is the hash of the sibling of the path's
// node at depth i+1. Neighbor is set only by an absence proof whose position holds another key.
type Proof struct {
	Siblings []types.Hash `json:"siblings,omitempty"`
	Neighbor *Leaf        `json:"neighbor,omitempty"`
}

// KeyPath returns the path of key in the tree.
func KeyPath(key []byte) types.Hash {
	return sha256.Sum256(key)
}

// LeafHash is the hash of the value of key in namespace, the part of a leaf that binds its content.
func LeafHash(namespace string, key []byte, value []byte) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.leaf.v2"))
	writeBytes(hasher, []byte(namespace))
	writeBytes(hasher, key)
	writeBytes(hasher, value)
	return hashSum(hasher.Sum(nil))
}

// LeafNodeHash is the hash of the tree node of a leaf.
func LeafNodeHash(leaf Leaf) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.leafnode.v2"))
	hasher.Write(leaf.Path[:])
	hasher.Write(leaf.ValueHash[:])
	return hashSum(hasher.Sum(nil))
}

// InternalNodeHash is the hash of an internal node with the given halves; an empty half is zero.
func InternalNodeHash(left types.Hash, right types.Hash) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.node.v2"))
	hasher.Write(left[:])
	hasher.Write(right[:])
	return hashSum(hasher.Sum(nil))
}

// EmptyRoot is the root of a namespace that holds no key.
func EmptyRoot(namespace string) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.empty.v2"))
	hasher.Write([]byte(namespace))
	return hashSum(hasher.Sum(nil))
}

// Bit returns bit index of path, most significant first.
func Bit(path types.Hash, index int) byte {
	return (path[index/8] >> (7 - uint(index%8))) & 1
}

// Root computes the root of namespace over pairs.
func Root(namespace string, pairs []Pair) (types.Hash, error) {
	leaves, err := sortedLeaves(namespace, pairs)
	if err != nil {
		return types.Hash{}, err
	}
	if len(leaves) == 0 {
		return EmptyRoot(namespace), nil
	}
	return subtreeHash(leaves, 0), nil
}

// Prove computes the root of namespace over pairs and the proof of key: of its value when it is
// present, of its absence otherwise.
func Prove(namespace string, pairs []Pair, key []byte) (root types.Hash, value []byte, exists bool, proof Proof, err error) {
	if len(key) == 0 {
		return types.Hash{}, nil, false, Proof{}, ErrInvalidKey
	}
	leaves, err := sortedLeaves(namespace, pairs)
	if err != nil {
		return types.Hash{}, nil, false, Proof{}, err
	}
	if len(leaves) == 0 {
		return EmptyRoot(namespace), nil, false, Proof{}, nil
	}
	path := KeyPath(key)
	subtree := leaves
	for depth := 0; ; depth++ {
		switch len(subtree) {
		case 0:
			return subtreeHash(leaves, 0), nil, false, proof, nil
		case 1:
			if subtree[0].leaf.Path == path {
				return subtreeHash(leaves, 0), append([]byte(nil), subtree[0].value...), true, proof, nil
			}
			neighbor := subtree[0].leaf
			proof.Neighbor = &neighbor
			return subtreeHash(leaves, 0), nil, false, proof, nil
		}
		left, right := splitLeaves(subtree, depth)
		if Bit(path, depth) == 0 {
			proof.Siblings = append(proof.Siblings, subtreeHash(right, depth+1))
			subtree = left
		} else {
			proof.Siblings = append(proof.Siblings, subtreeHash(left, depth+1))
			subtree = right
		}
	}
}

// VerifyMembership reports whether proof shows that key holds value in the namespace with root.
func VerifyMembership(namespace string, key []byte, value []byte, proof Proof, root types.Hash) bool {
	if namespace == "" || len(key) == 0 || proof.Neighbor != nil || len(proof.Siblings) > MaxDepth {
		return false
	}
	path := KeyPath(key)
	node := LeafNodeHash(Leaf{Path: path, ValueHash: LeafHash(namespace, key, value)})
	return foldPath(path, node, proof.Siblings) == root
}

// VerifyNonMembership reports whether proof shows that key holds no value in the namespace with
// root: the node at the key's position is empty, or is the leaf of another key with that prefix.
func VerifyNonMembership(namespace string, key []byte, proof Proof, root types.Hash) bool {
	if namespace == "" || len(key) == 0 || len(proof.Siblings) > MaxDepth {
		return false
	}
	path := KeyPath(key)
	depth := len(proof.Siblings)
	if proof.Neighbor == nil {
		if depth == 0 {
			return root == EmptyRoot(namespace)
		}
		return foldPath(path, types.Hash{}, proof.Siblings) == root
	}
	neighbor := *proof.Neighbor
	if neighbor.Path == path {
		return false
	}
	for index := 0; index < depth; index++ {
		if Bit(neighbor.Path, index) != Bit(path, index) {
			return false
		}
	}
	return foldPath(path, LeafNodeHash(neighbor), proof.Siblings) == root
}

// foldPath hashes node up the path through siblings to the root.
func foldPath(path types.Hash, node types.Hash, siblings []types.Hash) types.Hash {
	for depth := len(siblings) - 1; depth >= 0; depth-- {
		if Bit(path, depth) == 0 {
			node = InternalNodeHash(node, siblings[depth])
		} else {
			node = InternalNodeHash(siblings[depth], node)
		}
	}
	return node
}

type sortedLeaf struct {
	leaf  Leaf
	value []byte
}

// sortedLeaves validates pairs and returns their leaves ordered by path.
func sortedLeaves(namespace string, pairs []Pair) ([]sortedLeaf, error) {
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	leaves := make([]sortedLeaf, 0, len(pairs))
	keys := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		if len(pair.Key) == 0 {
			return nil, ErrInvalidKey
		}
		if _, found := keys[string(pair.Key)]; found {
			return nil, ErrDuplicateKey
		}
		keys[string(pair.Key)] = struct{}{}
		leaves = append(leaves, sortedLeaf{
			leaf:  Leaf{Path: KeyPath(pair.Key), ValueHash: LeafHash(namespace, pair.Key, pair.Value)},
			value: pair.Value,
		})
	}
	sort.Slice(leaves, func(left, right int) bool {
		return bytes.Compare(leaves[left].leaf.Path[:], leaves[right].leaf.Path[:]) < 0
	})
	for index := 1; index < len(leaves); index++ {
		if leaves[index-1].leaf.Path == leaves[index].leaf.Path {
			return nil, ErrPathCollision
		}
	}
	return leaves, nil
}

// splitLeaves splits leaves ordered by path into those whose bit depth is 0 and those whose is 1.
func splitLeaves(leaves []sortedLeaf, depth int) ([]sortedLeaf, []sortedLeaf) {
	index := sort.Search(len(leaves), func(index int) bool { return Bit(leaves[index].leaf.Path, depth) == 1 })
	return leaves[:index], leaves[index:]
}

// subtreeHash is the hash of the subtree at depth that holds leaves, ordered by path.
func subtreeHash(leaves []sortedLeaf, depth int) types.Hash {
	switch len(leaves) {
	case 0:
		return types.Hash{}
	case 1:
		return LeafNodeHash(leaves[0].leaf)
	}
	left, right := splitLeaves(leaves, depth)
	return InternalNodeHash(subtreeHash(left, depth+1), subtreeHash(right, depth+1))
}

func writeBytes(hasher interface{ Write([]byte) (int, error) }, value []byte) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(len(value)))
	_, _ = hasher.Write(buffer[:])
	_, _ = hasher.Write(value)
}

func hashSum(sum []byte) types.Hash {
	var hash types.Hash
	copy(hash[:], sum)
	return hash
}

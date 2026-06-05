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
)

type Pair struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value,omitempty"`
}

type Step struct {
	Side string     `json:"side"`
	Hash types.Hash `json:"hash"`
}

type AbsenceNeighbor struct {
	Key       []byte `json:"key"`
	Value     []byte `json:"value,omitempty"`
	Index     uint64 `json:"index"`
	LeafCount uint64 `json:"leaf_count"`
	Path      []Step `json:"path,omitempty"`
}

func Root(namespace string, pairs []Pair) (types.Hash, error) {
	leaves, err := sortedLeaves(namespace, pairs)
	if err != nil {
		return types.Hash{}, err
	}
	if len(leaves) == 0 {
		return emptyRoot(namespace), nil
	}
	hashes := make([]types.Hash, 0, len(leaves))
	for _, leaf := range leaves {
		hashes = append(hashes, LeafHash(namespace, leaf.Key, leaf.Value))
	}
	return merkleRoot(hashes), nil
}

func BuildMembership(namespace string, pairs []Pair, key []byte) (types.Hash, []byte, bool, []Step, error) {
	if len(key) == 0 {
		return types.Hash{}, nil, false, nil, ErrInvalidKey
	}
	leaves, err := sortedLeaves(namespace, pairs)
	if err != nil {
		return types.Hash{}, nil, false, nil, err
	}
	if len(leaves) == 0 {
		return emptyRoot(namespace), nil, false, nil, nil
	}
	hashes := make([]types.Hash, 0, len(leaves))
	targetIndex := -1
	var value []byte
	for index, leaf := range leaves {
		hashes = append(hashes, LeafHash(namespace, leaf.Key, leaf.Value))
		if bytes.Equal(leaf.Key, key) {
			targetIndex = index
			value = append([]byte(nil), leaf.Value...)
		}
	}
	if targetIndex < 0 {
		return merkleRoot(hashes), nil, false, nil, nil
	}
	return merkleRoot(hashes), value, true, merklePath(hashes, targetIndex), nil
}

func BuildNonMembership(namespace string, pairs []Pair, key []byte) (types.Hash, *AbsenceNeighbor, *AbsenceNeighbor, bool, error) {
	if len(key) == 0 {
		return types.Hash{}, nil, nil, false, ErrInvalidKey
	}
	leaves, err := sortedLeaves(namespace, pairs)
	if err != nil {
		return types.Hash{}, nil, nil, false, err
	}
	if len(leaves) == 0 {
		return emptyRoot(namespace), nil, nil, false, nil
	}
	hashes := make([]types.Hash, 0, len(leaves))
	for _, leaf := range leaves {
		hashes = append(hashes, LeafHash(namespace, leaf.Key, leaf.Value))
	}
	position := sort.Search(len(leaves), func(index int) bool {
		return bytes.Compare(leaves[index].Key, key) >= 0
	})
	root := merkleRoot(hashes)
	if position < len(leaves) && bytes.Equal(leaves[position].Key, key) {
		return root, nil, nil, true, nil
	}
	var left *AbsenceNeighbor
	var right *AbsenceNeighbor
	if position > 0 {
		left = absenceNeighbor(leaves[position-1], position-1, len(leaves), hashes)
	}
	if position < len(leaves) {
		right = absenceNeighbor(leaves[position], position, len(leaves), hashes)
	}
	return root, left, right, false, nil
}

func VerifyMembership(namespace string, key []byte, value []byte, path []Step, expectedRoot types.Hash) bool {
	if namespace == "" || len(key) == 0 || expectedRoot == (types.Hash{}) {
		return false
	}
	current := LeafHash(namespace, key, value)
	for _, step := range path {
		switch step.Side {
		case "left":
			current = nodeHash(step.Hash, current)
		case "right":
			current = nodeHash(current, step.Hash)
		default:
			return false
		}
	}
	return current == expectedRoot
}

func VerifyNonMembership(namespace string, pairs []Pair, key []byte, expectedRoot types.Hash) bool {
	if namespace == "" || len(key) == 0 || expectedRoot == (types.Hash{}) {
		return false
	}
	for _, pair := range pairs {
		if bytes.Equal(pair.Key, key) {
			return false
		}
	}
	root, err := Root(namespace, pairs)
	return err == nil && root == expectedRoot
}

func VerifyNonMembershipCompact(namespace string, key []byte, expectedRoot types.Hash, left *AbsenceNeighbor, right *AbsenceNeighbor) bool {
	if namespace == "" || len(key) == 0 || expectedRoot == (types.Hash{}) {
		return false
	}
	if left == nil && right == nil {
		return expectedRoot == emptyRoot(namespace)
	}
	if left != nil {
		if bytes.Compare(left.Key, key) >= 0 || !verifyNeighbor(namespace, left, expectedRoot) {
			return false
		}
	}
	if right != nil {
		if bytes.Compare(right.Key, key) <= 0 || !verifyNeighbor(namespace, right, expectedRoot) {
			return false
		}
	}
	switch {
	case left == nil:
		return right.Index == 0
	case right == nil:
		return left.Index+1 == left.LeafCount
	default:
		return left.LeafCount == right.LeafCount && left.Index+1 == right.Index
	}
}

func Contains(pairs []Pair, key []byte) bool {
	for _, pair := range pairs {
		if bytes.Equal(pair.Key, key) {
			return true
		}
	}
	return false
}

func absenceNeighbor(pair Pair, index int, leafCount int, hashes []types.Hash) *AbsenceNeighbor {
	return &AbsenceNeighbor{
		Key:       append([]byte(nil), pair.Key...),
		Value:     append([]byte(nil), pair.Value...),
		Index:     uint64(index),
		LeafCount: uint64(leafCount),
		Path:      merklePath(hashes, index),
	}
}

func verifyNeighbor(namespace string, neighbor *AbsenceNeighbor, expectedRoot types.Hash) bool {
	if neighbor == nil || len(neighbor.Key) == 0 || neighbor.LeafCount == 0 || neighbor.Index >= neighbor.LeafCount {
		return false
	}
	return VerifyMembership(namespace, neighbor.Key, neighbor.Value, neighbor.Path, expectedRoot)
}

func LeafHash(namespace string, key []byte, value []byte) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.leaf.v1"))
	hasher.Write([]byte{0})
	hasher.Write([]byte(namespace))
	writeBytes(hasher, key)
	writeBytes(hasher, value)
	return hashSum(hasher.Sum(nil))
}

func sortedLeaves(namespace string, pairs []Pair) ([]Pair, error) {
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	leaves := make([]Pair, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair.Key) == 0 {
			return nil, ErrInvalidKey
		}
		leaves = append(leaves, Pair{
			Key:   append([]byte(nil), pair.Key...),
			Value: append([]byte(nil), pair.Value...),
		})
	}
	sort.Slice(leaves, func(left, right int) bool {
		return bytes.Compare(leaves[left].Key, leaves[right].Key) < 0
	})
	for index := 1; index < len(leaves); index++ {
		if bytes.Equal(leaves[index-1].Key, leaves[index].Key) {
			return nil, ErrDuplicateKey
		}
	}
	return leaves, nil
}

func merkleRoot(hashes []types.Hash) types.Hash {
	if len(hashes) == 1 {
		return hashes[0]
	}
	level := append([]types.Hash(nil), hashes...)
	for len(level) > 1 {
		next := make([]types.Hash, 0, (len(level)+1)/2)
		for index := 0; index < len(level); index += 2 {
			if index+1 >= len(level) {
				next = append(next, level[index])
				continue
			}
			next = append(next, nodeHash(level[index], level[index+1]))
		}
		level = next
	}
	return level[0]
}

func merklePath(hashes []types.Hash, targetIndex int) []Step {
	path := make([]Step, 0)
	index := targetIndex
	level := append([]types.Hash(nil), hashes...)
	for len(level) > 1 {
		if index%2 == 0 {
			if index+1 < len(level) {
				path = append(path, Step{Side: "right", Hash: level[index+1]})
			}
		} else {
			path = append(path, Step{Side: "left", Hash: level[index-1]})
		}
		next := make([]types.Hash, 0, (len(level)+1)/2)
		for pairIndex := 0; pairIndex < len(level); pairIndex += 2 {
			if pairIndex+1 >= len(level) {
				next = append(next, level[pairIndex])
				continue
			}
			next = append(next, nodeHash(level[pairIndex], level[pairIndex+1]))
		}
		index /= 2
		level = next
	}
	return path
}

func nodeHash(left types.Hash, right types.Hash) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.node.v1"))
	hasher.Write(left[:])
	hasher.Write(right[:])
	return hashSum(hasher.Sum(nil))
}

func emptyRoot(namespace string) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo.state.empty.v1"))
	hasher.Write([]byte(namespace))
	return hashSum(hasher.Sum(nil))
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

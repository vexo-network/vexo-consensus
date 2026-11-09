package stateproof

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

type memoryNodes map[NodeKey]Node

func (nodes memoryNodes) Node(key NodeKey) (Node, bool, error) {
	node, found := nodes[key]
	return node, found, nil
}

func (nodes memoryNodes) apply(changes []NodeChange) {
	for _, change := range changes {
		if change.Node == nil {
			delete(nodes, change.Key)
			continue
		}
		nodes[change.Key] = *change.Node
	}
}

func testPairs(count int) []Pair {
	pairs := make([]Pair, 0, count)
	for index := 0; index < count; index++ {
		pairs = append(pairs, Pair{Key: []byte(fmt.Sprintf("account/%04d", index)), Value: []byte(fmt.Sprintf("%d", index*7))})
	}
	return pairs
}

func mustRoot(t *testing.T, namespace string, pairs []Pair) types.Hash {
	t.Helper()
	root, err := Root(namespace, pairs)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRootDependsOnlyOnTheSetOfPairs(t *testing.T) {
	pairs := testPairs(40)
	shuffled := append([]Pair(nil), pairs...)
	rand.New(rand.NewSource(1)).Shuffle(len(shuffled), func(left, right int) { shuffled[left], shuffled[right] = shuffled[right], shuffled[left] })
	if mustRoot(t, "bank", pairs) != mustRoot(t, "bank", shuffled) {
		t.Fatal("the root must not depend on the order of the pairs")
	}
	if mustRoot(t, "bank", pairs) == mustRoot(t, "staking", pairs) {
		t.Fatal("the root must bind the namespace")
	}
	changed := append([]Pair(nil), pairs...)
	changed[3] = Pair{Key: changed[3].Key, Value: []byte("other")}
	if mustRoot(t, "bank", pairs) == mustRoot(t, "bank", changed) {
		t.Fatal("the root must bind every value")
	}
	if mustRoot(t, "bank", nil) != EmptyRoot("bank") {
		t.Fatal("an empty namespace has the empty root")
	}
	if _, err := Root("bank", []Pair{{Key: []byte("a")}, {Key: []byte("a")}}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("expected duplicate keys to be rejected, got %v", err)
	}
	if _, err := Root("bank", []Pair{{}}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected an empty key to be rejected, got %v", err)
	}
}

func TestProofsProveValuesAndAbsence(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3, 17, 64} {
		pairs := testPairs(count)
		root := mustRoot(t, "bank", pairs)
		for _, pair := range pairs {
			proofRoot, value, exists, proof, err := Prove("bank", pairs, pair.Key)
			if err != nil || proofRoot != root || !exists || string(value) != string(pair.Value) {
				t.Fatalf("count %d key %s: root %x exists %v value %q err %v", count, pair.Key, proofRoot, exists, value, err)
			}
			if !VerifyMembership("bank", pair.Key, pair.Value, proof, root) {
				t.Fatalf("count %d: membership of %s must verify", count, pair.Key)
			}
			if VerifyMembership("bank", pair.Key, []byte("forged"), proof, root) {
				t.Fatalf("count %d: a membership proof must bind the value", count)
			}
			if VerifyNonMembership("bank", pair.Key, proof, root) {
				t.Fatalf("count %d: a present key must not be proven absent", count)
			}
		}
		for index := 0; index < 20; index++ {
			key := []byte(fmt.Sprintf("missing/%d", index))
			proofRoot, _, exists, proof, err := Prove("bank", pairs, key)
			if err != nil || proofRoot != root || exists {
				t.Fatalf("count %d: absent key %s: exists %v err %v", count, key, exists, err)
			}
			if !VerifyNonMembership("bank", key, proof, root) {
				t.Fatalf("count %d: absence of %s must verify", count, key)
			}
			if VerifyMembership("bank", key, nil, proof, root) {
				t.Fatalf("count %d: an absent key must not be proven present", count)
			}
		}
	}
}

// Absence is proven by the node at the key's own position, so a proof cannot borrow other leaves of
// the tree to claim that a present key is absent.
func TestAbsenceOfAPresentKeyCannotBeProven(t *testing.T) {
	pairs := testPairs(64)
	root := mustRoot(t, "bank", pairs)
	target := pairs[10].Key
	path := KeyPath(target)
	for _, pair := range pairs {
		if string(pair.Key) == string(target) {
			continue
		}
		_, _, _, proof, err := Prove("bank", pairs, pair.Key)
		if err != nil {
			t.Fatal(err)
		}
		neighbor := Leaf{Path: KeyPath(pair.Key), ValueHash: LeafHash("bank", pair.Key, pair.Value)}
		forged := Proof{Siblings: proof.Siblings, Neighbor: &neighbor}
		if VerifyNonMembership("bank", target, forged, root) {
			t.Fatalf("the leaf of %s must not prove %s absent", pair.Key, target)
		}
		for depth := 0; depth <= len(proof.Siblings); depth++ {
			if VerifyNonMembership("bank", target, Proof{Siblings: proof.Siblings[:depth]}, root) {
				t.Fatalf("an empty position must not be claimed for %s", target)
			}
		}
	}
	_, _, _, own, err := Prove("bank", pairs, target)
	if err != nil {
		t.Fatal(err)
	}
	selfNeighbor := Leaf{Path: path, ValueHash: LeafHash("bank", target, pairs[10].Value)}
	if VerifyNonMembership("bank", target, Proof{Siblings: own.Siblings, Neighbor: &selfNeighbor}, root) {
		t.Fatal("the key's own leaf must not prove it absent")
	}
	if VerifyNonMembership("bank", target, Proof{}, root) {
		t.Fatal("a non-empty namespace must not be proven empty")
	}
}

// A tree that applies writes one at a time, as a store does block after block, has the root and
// the nodes of the tree built from the resulting pairs at once, whatever the order of the writes.
func TestTreeWritesMatchTheTreeBuiltFromThePairs(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	nodes := memoryNodes{}
	state := map[string][]byte{}
	for round := 0; round < 60; round++ {
		tree := NewTree("bank", nodes)
		for write := 0; write < 1+random.Intn(12); write++ {
			key := fmt.Sprintf("k/%d", random.Intn(48))
			if random.Intn(3) == 0 {
				if err := tree.Delete([]byte(key)); err != nil {
					t.Fatal(err)
				}
				delete(state, key)
				continue
			}
			value := []byte(fmt.Sprintf("v%d", random.Intn(1000)))
			if err := tree.Set([]byte(key), value); err != nil {
				t.Fatal(err)
			}
			state[key] = value
		}
		root, err := tree.Root()
		if err != nil {
			t.Fatal(err)
		}
		nodes.apply(tree.Changes())

		pairs := make([]Pair, 0, len(state))
		for key, value := range state {
			pairs = append(pairs, Pair{Key: []byte(key), Value: value})
		}
		if want := mustRoot(t, "bank", pairs); root != want {
			t.Fatalf("round %d: incremental root %x, built root %x", round, root, want)
		}
		built, err := Build("bank", pairs)
		if err != nil {
			t.Fatal(err)
		}
		if len(built) != len(nodes) {
			t.Fatalf("round %d: the tree holds %d nodes, the built tree %d", round, len(nodes), len(built))
		}
		for _, change := range built {
			if stored, found := nodes[change.Key]; !found || stored.Hash() != change.Node.Hash() {
				t.Fatalf("round %d: node %+v differs from the built tree", round, change.Key)
			}
		}
		reader := NewTree("bank", nodes)
		for index := 0; index < 48; index++ {
			key := []byte(fmt.Sprintf("k/%d", index))
			proof, leaf, err := reader.Prove(key)
			if err != nil {
				t.Fatal(err)
			}
			value, present := state[string(key)]
			if present {
				if leaf == nil || leaf.Path != KeyPath(key) || !VerifyMembership("bank", key, value, proof, root) {
					t.Fatalf("round %d: stored proof of %s must verify its value", round, key)
				}
				continue
			}
			if !VerifyNonMembership("bank", key, proof, root) {
				t.Fatalf("round %d: stored proof of absent %s must verify", round, key)
			}
		}
	}
}

func TestNodesRoundTrip(t *testing.T) {
	leaf := Node{Leaf: &Leaf{Path: types.Hash{1}, ValueHash: types.Hash{2}}}
	internal := Node{Left: types.Hash{3}, Right: types.Hash{4}}
	for _, node := range []Node{leaf, internal} {
		decoded, err := DecodeNode(EncodeNode(node))
		if err != nil || decoded.Hash() != node.Hash() {
			t.Fatalf("node %+v did not round trip: %+v %v", node, decoded, err)
		}
	}
	if _, err := DecodeNode([]byte{9}); !errors.Is(err, ErrInvalidNode) {
		t.Fatalf("expected an invalid node to be rejected, got %v", err)
	}
	if NodeKeyOf(types.Hash{0xff, 0xff}, 9).Prefix != (types.Hash{0xff, 0x80}) {
		t.Fatal("a node key keeps only the bits of its depth")
	}
}

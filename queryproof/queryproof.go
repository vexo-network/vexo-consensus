// Package queryproof proves the value of a state key, or its absence, at a height against the state
// root of its namespace at that height.
package queryproof

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// SchemaVersionV2 is the proof schema of the radix state tree (stateproof).
const SchemaVersionV2 = "v2"

var (
	ErrInvalidProof = errors.New("invalid query proof")
	ErrRootMismatch = errors.New("query proof root mismatch")
)

type Proof struct {
	SchemaVersion string           `json:"schema_version"`
	ChainID       string           `json:"chain_id"`
	Height        types.Height     `json:"height"`
	Namespace     string           `json:"namespace"`
	Key           []byte           `json:"key"`
	Value         []byte           `json:"value,omitempty"`
	Exists        bool             `json:"exists"`
	StateRoot     types.Hash       `json:"state_root"`
	Path          stateproof.Proof `json:"path"`
}

// TreeStore serves proofs from the state trees it keeps.
type TreeStore interface {
	StateProof(ctx context.Context, namespace string, key []byte) (types.Hash, []byte, bool, stateproof.Proof, error)
	StateProofAt(ctx context.Context, height types.Height, namespace string, key []byte) (types.Hash, []byte, bool, stateproof.Proof, error)
}

// Build proves key against the current state of namespace, the state of height, from the store's
// state tree: it reads the nodes on the key's path, not the namespace.
func Build(ctx context.Context, trees TreeStore, chainID string, height types.Height, namespace string, key []byte) (Proof, error) {
	if trees == nil || chainID == "" || height == 0 || namespace == "" || len(key) == 0 {
		return Proof{}, ErrInvalidProof
	}
	root, value, exists, path, err := trees.StateProof(ctx, namespace, key)
	if err != nil {
		return Proof{}, err
	}
	return newProof(chainID, height, namespace, key, value, exists, root, path), nil
}

// BuildAt proves key against the state of namespace at committed height, from the versions of
// the store's state tree nodes on the key's path.
func BuildAt(ctx context.Context, trees TreeStore, chainID string, height types.Height, namespace string, key []byte) (Proof, error) {
	if trees == nil || chainID == "" || height == 0 || namespace == "" || len(key) == 0 {
		return Proof{}, ErrInvalidProof
	}
	root, value, exists, path, err := trees.StateProofAt(ctx, height, namespace, key)
	if err != nil {
		return Proof{}, err
	}
	return newProof(chainID, height, namespace, key, value, exists, root, path), nil
}

// BuildFromKVPairs proves key against stateRoot from every pair of a namespace, for a proof built
// outside a store, such as from a snapshot.
func BuildFromKVPairs(chainID string, height types.Height, namespace string, key []byte, pairs []store.KVPair, stateRoot types.Hash) (Proof, error) {
	return BuildFromPairs(chainID, height, namespace, key, storePairsToProofPairs(pairs), stateRoot)
}

func BuildFromPairs(chainID string, height types.Height, namespace string, key []byte, pairs []stateproof.Pair, stateRoot types.Hash) (Proof, error) {
	if chainID == "" || height == 0 || namespace == "" || len(key) == 0 || stateRoot == (types.Hash{}) {
		return Proof{}, ErrInvalidProof
	}
	root, value, exists, path, err := stateproof.Prove(namespace, pairs, key)
	if err != nil {
		return Proof{}, err
	}
	if root != stateRoot {
		return Proof{}, ErrRootMismatch
	}
	return newProof(chainID, height, namespace, key, value, exists, root, path), nil
}

func newProof(chainID string, height types.Height, namespace string, key []byte, value []byte, exists bool, root types.Hash, path stateproof.Proof) Proof {
	return Proof{
		SchemaVersion: SchemaVersionV2,
		ChainID:       chainID,
		Height:        height,
		Namespace:     namespace,
		Key:           append([]byte(nil), key...),
		Value:         append([]byte(nil), value...),
		Exists:        exists,
		StateRoot:     root,
		Path:          path,
	}
}

// Verify checks proof against the chain, the height and the namespace root the verifier trusts;
// an empty expectation is not checked.
func Verify(proof Proof, expectedChainID string, expectedHeight types.Height, expectedRoot types.Hash) error {
	if proof.SchemaVersion != SchemaVersionV2 ||
		proof.ChainID == "" ||
		proof.Height == 0 ||
		proof.Namespace == "" ||
		len(proof.Key) == 0 {
		return ErrInvalidProof
	}
	if expectedChainID != "" && proof.ChainID != expectedChainID {
		return ErrInvalidProof
	}
	if expectedHeight != 0 && proof.Height != expectedHeight {
		return ErrInvalidProof
	}
	if expectedRoot != (types.Hash{}) && proof.StateRoot != expectedRoot {
		return ErrRootMismatch
	}
	if proof.Exists {
		if !stateproof.VerifyMembership(proof.Namespace, proof.Key, proof.Value, proof.Path, proof.StateRoot) {
			return ErrInvalidProof
		}
		return nil
	}
	if len(proof.Value) != 0 || !stateproof.VerifyNonMembership(proof.Namespace, proof.Key, proof.Path, proof.StateRoot) {
		return ErrInvalidProof
	}
	return nil
}

func Encode(proof Proof) ([]byte, error) {
	return json.Marshal(proof)
}

func Decode(data []byte) (Proof, error) {
	var proof Proof
	if err := json.Unmarshal(data, &proof); err != nil {
		return Proof{}, err
	}
	return proof, nil
}

func EqualValue(proof Proof, value []byte) bool {
	return bytes.Equal(proof.Value, value)
}

func storePairsToProofPairs(pairs []store.KVPair) []stateproof.Pair {
	proofPairs := make([]stateproof.Pair, 0, len(pairs))
	for _, pair := range pairs {
		proofPairs = append(proofPairs, stateproof.Pair{
			Key:   append([]byte(nil), pair.Key...),
			Value: append([]byte(nil), pair.Value...),
		})
	}
	return proofPairs
}
